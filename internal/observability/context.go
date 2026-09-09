// Package observability provides correlation primitives shared by future
// HTTP, execution, and reconciliation components.
package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
)

type contextKey string

const requestIDKey contextKey = "runbridge.request_id"
const runIDKey contextKey = "runbridge.run_id"

var ErrIDGeneration = errors.New("could not generate correlation ID")

func NewID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("%w: %v", ErrIDGeneration, err)
	}
	return hex.EncodeToString(bytes[:]), nil
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}
func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}
func WithRunID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, runIDKey, id)
}
func RunID(ctx context.Context) string { value, _ := ctx.Value(runIDKey).(string); return value }

func Logger(base *slog.Logger, ctx context.Context) *slog.Logger {
	if base == nil {
		base = slog.Default()
	}
	attrs := make([]any, 0, 4)
	if id := RequestID(ctx); id != "" {
		attrs = append(attrs, "request_id", id)
	}
	if id := RunID(ctx); id != "" {
		attrs = append(attrs, "run_id", id)
	}
	return base.With(attrs...)
}
