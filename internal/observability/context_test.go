package observability

import (
	"context"
	"testing"
)

func TestCorrelationContext(t *testing.T) {
	requestID, err := NewID()
	if err != nil || len(requestID) != 32 {
		t.Fatalf("request ID: %q %v", requestID, err)
	}
	runID, _ := NewID()
	ctx := WithRunID(WithRequestID(context.Background(), requestID), runID)
	if RequestID(ctx) != requestID || RunID(ctx) != runID {
		t.Fatalf("context IDs not retained")
	}
}
