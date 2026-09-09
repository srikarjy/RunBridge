package observability

import (
	"context"
	"testing"
)

func TestClassifyContextFailures(t *testing.T) {
	if Classify(context.DeadlineExceeded) != CategoryTimeout {
		t.Fatal("deadline was not classified as timeout")
	}
	if Classify(nil) != CategoryUnknown {
		t.Fatal("nil error classification")
	}
}
