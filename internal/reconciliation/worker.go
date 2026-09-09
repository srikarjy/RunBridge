package reconciliation

import (
	"context"
	"fmt"
	"time"
)

type Worker struct {
	service  *Service
	interval time.Duration
	limit    int
	onError  func(error)
}

func NewWorker(service *Service, interval time.Duration, limit int) (*Worker, error) {
	if service == nil {
		return nil, fmt.Errorf("reconciliation worker requires service")
	}
	if interval <= 0 {
		return nil, fmt.Errorf("reconciliation worker requires positive interval")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	return &Worker{service: service, interval: interval, limit: limit}, nil
}

func (worker *Worker) WithErrorHandler(handler func(error)) *Worker {
	worker.onError = handler
	return worker
}

// Run performs an immediate recovery pass and then polls at the configured
// interval. Cancellation is the only normal exit; individual pass failures
// are reported and do not terminate the worker, allowing transient Seqera or
// database outages to recover on the next tick.
func (worker *Worker) Run(ctx context.Context) error {
	if worker == nil || worker.service == nil {
		return fmt.Errorf("reconciliation worker is not configured")
	}
	worker.runPass(ctx)
	ticker := time.NewTicker(worker.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			worker.runPass(ctx)
		}
	}
}

func (worker *Worker) runPass(ctx context.Context) {
	if err := worker.service.ReconcileOnce(ctx, worker.limit); err != nil && worker.onError != nil {
		worker.onError(err)
	}
}
