package seqera

import (
	"context"
	"testing"

	"github.com/srikarjy/RunBridge/internal/execution"
)

func TestLauncherAdapterRequiresClient(t *testing.T) {
	adapter := NewLauncherAdapter(nil)
	if _, err := adapter.Submit(context.Background(), execution.LaunchRequest{}); err == nil {
		t.Fatal("expected missing client error")
	}
}
