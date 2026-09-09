package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/authorization"
	"github.com/srikarjy/RunBridge/internal/events"
	"github.com/srikarjy/RunBridge/internal/httpapi"
	"github.com/srikarjy/RunBridge/internal/observability"
	"github.com/srikarjy/RunBridge/internal/postgres"
	"github.com/srikarjy/RunBridge/internal/security"
	"github.com/srikarjy/RunBridge/internal/webhooks"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	serviceHandler, closeDB, configurationErr := configuredHandler(logger)
	if configurationErr != nil {
		logger.Error("service configuration failed", "error", configurationErr)
		os.Exit(1)
	}
	defer closeDB()
	server := &http.Server{Addr: env("RUNBRIDGE_ADDR", ":8080"), Handler: serviceHandler}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		logger.Info("runbridge server started", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}
}

func handler() http.Handler {
	return handlerWithAudit(nil, nil)
}

func handlerWithAudit(store *postgres.Store, resolver httpapi.PrincipalResolver) http.Handler {
	return handlerWithDependencies(store, resolver, nil)
}

func handlerWithDependencies(store *postgres.Store, resolver httpapi.PrincipalResolver, webhook http.Handler) http.Handler {
	metrics := &observability.Metrics{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /readyz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if store == nil {
			writer.WriteHeader(http.StatusServiceUnavailable)
			_, _ = writer.Write([]byte(`{"status":"not_ready"}`))
			return
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"status":"ready"}`))
	})
	mux.HandleFunc("GET /metrics", func(writer http.ResponseWriter, _ *http.Request) {
		snapshot := metrics.Snapshot()
		writer.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = fmt.Fprintf(writer, "runbridge_submission_failures_total %d\nrunbridge_reconciliation_attempts_total %d\nrunbridge_webhook_duplicates_total %d\nrunbridge_transition_conflicts_total %d\n", snapshot.SubmissionFailures, snapshot.ReconciliationAttempts, snapshot.WebhookDuplicates, snapshot.TransitionConflicts)
	})
	if store != nil && resolver != nil {
		mux.Handle("GET /projects/{projectID}/audit", &httpapi.AuditHandler{Store: store, Authorizer: authorization.NewAuthorizer(store), ResolvePrincipal: resolver})
		mux.Handle("GET /projects/{projectID}/executions/{executionID}", &httpapi.ExecutionHandler{Store: store, Authorizer: authorization.NewAuthorizer(store), ResolvePrincipal: resolver})
	}
	if webhook != nil {
		mux.Handle("POST /webhooks/seqera", webhook)
	}
	return mux
}

func configuredHandler(logger *slog.Logger) (http.Handler, func(), error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return handler(), func() {}, nil
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return handler(), func() {}, fmt.Errorf("open database: %w", err)
	}
	if err := postgres.Migrate(context.Background(), database); err != nil {
		_ = database.Close()
		return handler(), func() {}, fmt.Errorf("migrate database: %w", err)
	}
	actorID, actorErr := auth.NewActorID(env("RUNBRIDGE_ACTOR_ID", "service"))
	actor, actorBuildErr := auth.NewActor(actorID, env("RUNBRIDGE_ACTOR_NAME", "RunBridge service"), auth.ActorKindHuman)
	token := os.Getenv("RUNBRIDGE_API_TOKEN")
	if actorErr != nil || actorBuildErr != nil || token == "" {
		logger.Warn("audit API disabled: actor or token configuration is missing")
		return handler(), func() { _ = database.Close() }, nil
	}
	authenticator, err := httpapi.NewStaticBearerAuthenticator(token, actor)
	if err != nil {
		return handler(), func() { _ = database.Close() }, fmt.Errorf("configure authenticator: %w", err)
	}
	store := postgres.NewStore(database)
	var webhookHandler http.Handler
	if secret := os.Getenv("RUNBRIDGE_WEBHOOK_SECRET"); secret != "" {
		verifier, verifierErr := security.NewWebhookVerifier(secret, 5*time.Minute)
		if verifierErr != nil {
			_ = database.Close()
			return handler(), func() {}, fmt.Errorf("configure webhook verifier: %w", verifierErr)
		}
		processor := events.NewProcessor(store, store)
		webhookHandler = webhooks.NewHandler(verifier, func(ctxRequest *http.Request, event events.Event) error {
			_, err := processor.Process(ctxRequest.Context(), event)
			return err
		})
	}
	return handlerWithDependencies(store, authenticator.Resolve, webhookHandler), func() { _ = database.Close() }, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
