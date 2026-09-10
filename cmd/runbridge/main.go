package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	seqeraintegration "github.com/srikarjy/RunBridge/integrations/seqera"
	"github.com/srikarjy/RunBridge/internal/auth"
	"github.com/srikarjy/RunBridge/internal/authorization"
	"github.com/srikarjy/RunBridge/internal/events"
	"github.com/srikarjy/RunBridge/internal/httpapi"
	"github.com/srikarjy/RunBridge/internal/observability"
	"github.com/srikarjy/RunBridge/internal/postgres"
	"github.com/srikarjy/RunBridge/internal/reconciliation"
	"github.com/srikarjy/RunBridge/internal/security"
	"github.com/srikarjy/RunBridge/internal/webhooks"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	serviceHandler, workers, closeDB, configurationErr := configuredHandler(logger)
	if configurationErr != nil {
		logger.Error("service configuration failed", "error", configurationErr)
		os.Exit(1)
	}
	defer closeDB()
	workerContext, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	var workerGroup sync.WaitGroup
	for _, worker := range workers {
		workerGroup.Add(1)
		go func(worker backgroundWorker) {
			defer workerGroup.Done()
			if err := worker.Run(workerContext); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("background worker stopped", "error", err)
			}
		}(worker)
	}
	server := &http.Server{
		Addr:              env("RUNBRIDGE_ADDR", ":8080"),
		Handler:           serviceHandler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
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
	stopWorker()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}
	workerDone := make(chan struct{})
	go func() { workerGroup.Wait(); close(workerDone) }()
	select {
	case <-workerDone:
	case <-ctx.Done():
		logger.Error("background worker shutdown timed out")
	}
}

func handler() http.Handler {
	return handlerWithAudit(nil, nil)
}

func handlerWithAudit(store *postgres.Store, resolver httpapi.PrincipalResolver) http.Handler {
	return handlerWithDependencies(store, resolver, nil)
}

func handlerWithDependencies(store *postgres.Store, resolver httpapi.PrincipalResolver, webhook http.Handler) http.Handler {
	return handlerWithRuntimeDependencies(store, resolver, webhook, &observability.Metrics{})
}

func handlerWithRuntimeDependencies(store *postgres.Store, resolver httpapi.PrincipalResolver, webhook http.Handler, metrics *observability.Metrics) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /readyz", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if store == nil {
			writer.WriteHeader(http.StatusServiceUnavailable)
			_, _ = writer.Write([]byte(`{"status":"not_ready"}`))
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
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
		_, _ = fmt.Fprintf(writer, "runbridge_http_requests_total %d\nrunbridge_http_errors_total %d\nrunbridge_submission_failures_total %d\nrunbridge_reconciliation_attempts_total %d\nrunbridge_webhook_duplicates_total %d\nrunbridge_transition_conflicts_total %d\n", snapshot.HTTPRequests, snapshot.HTTPErrors, snapshot.SubmissionFailures, snapshot.ReconciliationAttempts, snapshot.WebhookDuplicates, snapshot.TransitionConflicts)
	})
	if store != nil && resolver != nil {
		mux.Handle("GET /projects/{projectID}/audit", &httpapi.AuditHandler{Store: store, Authorizer: authorization.NewAuthorizer(store), ResolvePrincipal: resolver})
		mux.Handle("GET /projects/{projectID}/executions/{executionID}", &httpapi.ExecutionHandler{Store: store, Authorizer: authorization.NewAuthorizer(store), ResolvePrincipal: resolver})
	}
	if webhook != nil {
		mux.Handle("POST /webhooks/seqera", webhook)
	}
	return security.Headers(observability.MiddlewareWithMetrics(mux, slog.Default(), metrics))
}

type backgroundWorker interface {
	Run(context.Context) error
}

func configuredHandler(logger *slog.Logger) (http.Handler, []backgroundWorker, func(), error) {
	databaseURL, databaseConfigErr := databaseConnectionString()
	if databaseConfigErr != nil {
		return handler(), nil, func() {}, databaseConfigErr
	}
	if databaseURL == "" {
		return handler(), nil, func() {}, nil
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return handler(), nil, func() {}, fmt.Errorf("open database: %w", err)
	}
	if err := postgres.Migrate(context.Background(), database); err != nil {
		_ = database.Close()
		return handler(), nil, func() {}, fmt.Errorf("migrate database: %w", err)
	}
	actorID, actorErr := auth.NewActorID(env("RUNBRIDGE_ACTOR_ID", "service"))
	actor, actorBuildErr := auth.NewActor(actorID, env("RUNBRIDGE_ACTOR_NAME", "RunBridge service"), auth.ActorKindHuman)
	token := os.Getenv("RUNBRIDGE_API_TOKEN")
	if actorErr != nil || actorBuildErr != nil || token == "" {
		_ = database.Close()
		return handler(), nil, func() {}, fmt.Errorf("authorization configuration is required when DATABASE_URL is set")
	}
	authenticator, err := httpapi.NewStaticBearerAuthenticator(token, actor)
	if err != nil {
		return handler(), nil, func() { _ = database.Close() }, fmt.Errorf("configure authenticator: %w", err)
	}
	store := postgres.NewStore(database)
	seqeraToken := os.Getenv("SEQERA_TOKEN")
	if seqeraToken == "" {
		_ = database.Close()
		return handler(), nil, func() {}, fmt.Errorf("SEQERA_TOKEN is required when DATABASE_URL is set")
	}
	seqeraClient, err := seqeraintegration.NewClient(env("SEQERA_BASE_URL", seqeraintegration.DefaultBaseURL), seqeraToken, nil)
	if err != nil {
		_ = database.Close()
		return handler(), nil, func() {}, fmt.Errorf("configure Seqera client: %w", err)
	}
	metrics := &observability.Metrics{}
	reconciliationService := reconciliation.NewService(store, seqeraintegration.NewReconciliationObserver(seqeraClient), store).WithMetrics(metrics)
	worker, err := reconciliation.NewWorker(reconciliationService, 30*time.Second, 100)
	if err != nil {
		_ = database.Close()
		return handler(), nil, func() {}, fmt.Errorf("configure reconciliation worker: %w", err)
	}
	worker.WithErrorHandler(func(err error) { logger.Error("reconciliation pass failed", "error", err) })
	eventProcessor := events.NewProcessor(store, store).WithMetrics(metrics)
	poller, err := events.NewPoller(store, seqeraClient, eventProcessor, 30*time.Second, 100)
	if err != nil {
		_ = database.Close()
		return handler(), nil, func() {}, fmt.Errorf("configure execution poller: %w", err)
	}
	poller.WithErrorHandler(func(err error) { logger.Error("execution poll failed", "error", err) })
	var webhookHandler http.Handler
	if secret := os.Getenv("RUNBRIDGE_WEBHOOK_SECRET"); secret != "" {
		verifier, verifierErr := security.NewWebhookVerifier(secret, 5*time.Minute)
		if verifierErr != nil {
			_ = database.Close()
			return handler(), nil, func() {}, fmt.Errorf("configure webhook verifier: %w", verifierErr)
		}
		processor := eventProcessor
		webhookHandler = webhooks.NewHandler(verifier, func(ctxRequest *http.Request, event events.Event) error {
			_, err := processor.Process(ctxRequest.Context(), event)
			return err
		})
	}
	return handlerWithRuntimeDependencies(store, authenticator.Resolve, webhookHandler, metrics), []backgroundWorker{worker, poller}, func() { _ = database.Close() }, nil
}

func databaseConnectionString() (string, error) {
	if value := os.Getenv("DATABASE_URL"); value != "" {
		return value, nil
	}
	host, password := os.Getenv("DB_HOST"), os.Getenv("DB_PASSWORD")
	if host == "" && password == "" {
		return "", nil
	}
	user, name := os.Getenv("DB_USER"), os.Getenv("DB_NAME")
	if host == "" || password == "" || user == "" || name == "" {
		return "", fmt.Errorf("DB_HOST, DB_USER, DB_PASSWORD, and DB_NAME must be configured together")
	}
	port := env("DB_PORT", "5432")
	connection := &url.URL{Scheme: "postgres", User: url.UserPassword(user, password), Host: net.JoinHostPort(host, port), Path: "/" + name}
	query := connection.Query()
	query.Set("sslmode", env("DB_SSLMODE", "require"))
	connection.RawQuery = query.Encode()
	return connection.String(), nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
