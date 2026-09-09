package integrity

import (
	"strings"
	"testing"
	"time"
)

func TestExecutionReceiptBindsEvidence(t *testing.T) {
	receipt := ExecutionReceipt{ExecutionID: "execution-1", ApprovalReceiptDigest: strings.Repeat("a", 64), ManifestDigest: strings.Repeat("b", 64), ExternalExecutionID: "wf-1", CompletedAt: time.Unix(1, 0).UTC()}
	if err := receipt.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(receipt.DigestHex()) != 64 {
		t.Fatal("receipt digest length")
	}
}
