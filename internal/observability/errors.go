package observability

import (
	"context"
	"errors"
	"net"
)

type ErrorCategory string

const (
	CategoryUnknown     ErrorCategory = "unknown"
	CategoryTimeout     ErrorCategory = "timeout"
	CategoryTransport   ErrorCategory = "transport"
	CategoryConflict    ErrorCategory = "conflict"
	CategoryPersistence ErrorCategory = "persistence"
)

// Classify provides a conservative baseline; domain packages can wrap their
// sentinel errors and add richer labels at the application boundary.
func Classify(err error) ErrorCategory {
	if err == nil {
		return CategoryUnknown
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return CategoryTimeout
	}
	var netError net.Error
	if errors.As(err, &netError) {
		if netError.Timeout() {
			return CategoryTimeout
		}
		return CategoryTransport
	}
	return CategoryUnknown
}
