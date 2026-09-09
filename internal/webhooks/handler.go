// Package webhooks exposes the vendor-neutral inbound event boundary.
package webhooks

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/srikarjy/RunBridge/internal/events"
	"github.com/srikarjy/RunBridge/internal/observability"
	"github.com/srikarjy/RunBridge/internal/security"
)

type Sink func(*http.Request, events.Event) error

type Handler struct {
	verifier *security.WebhookVerifier
	sink     Sink
	now      func() time.Time
	metrics  *observability.Metrics
}

func NewHandler(verifier *security.WebhookVerifier, sink Sink) *Handler {
	return &Handler{verifier: verifier, sink: sink, now: time.Now}
}

func (handler *Handler) WithMetrics(metrics *observability.Metrics) *Handler {
	handler.metrics = metrics
	return handler
}

func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if handler == nil || handler.verifier == nil || handler.sink == nil {
		http.Error(writer, "webhook unavailable", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
	if err != nil {
		http.Error(writer, "invalid body", http.StatusBadRequest)
		return
	}
	timestamp, err := strconv.ParseInt(request.Header.Get("X-RunBridge-Timestamp"), 10, 64)
	if err != nil || handler.verifier.Verify(handler.now(), timestamp, request.Header.Get("X-RunBridge-Signature"), body) != nil {
		http.Error(writer, "invalid webhook signature", http.StatusUnauthorized)
		return
	}
	var event events.Event
	if err := json.Unmarshal(body, &event); err != nil || event.Validate() != nil {
		http.Error(writer, "invalid webhook event", http.StatusBadRequest)
		return
	}
	if err := handler.sink(request, event); err != nil {
		http.Error(writer, "event unavailable", http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}
