package integrity

import (
	"strings"
	"testing"
	"time"
)

func TestApprovalReceiptIdentity(t *testing.T) {
	receipt := ApprovalReceipt{ApprovalID: "approval-1", SpecificationDigest: strings.Repeat("a", 64), Decision: "approved", DecidedAt: time.Unix(1, 0).UTC()}
	if err := receipt.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(receipt.DigestHex()) != 64 {
		t.Fatal("receipt digest length")
	}
}
