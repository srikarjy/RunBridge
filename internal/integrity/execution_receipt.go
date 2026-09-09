package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidExecutionReceipt = errors.New("invalid execution receipt")

type ExecutionReceipt struct {
	ExecutionID           string    `json:"execution_id"`
	ApprovalReceiptDigest string    `json:"approval_receipt_digest"`
	ManifestDigest        string    `json:"manifest_digest"`
	ExternalExecutionID   string    `json:"external_execution_id"`
	CompletedAt           time.Time `json:"completed_at"`
}

func (receipt ExecutionReceipt) Validate() error {
	if strings.TrimSpace(receipt.ExecutionID) == "" || strings.TrimSpace(receipt.ApprovalReceiptDigest) == "" || strings.TrimSpace(receipt.ManifestDigest) == "" || strings.TrimSpace(receipt.ExternalExecutionID) == "" || receipt.CompletedAt.IsZero() {
		return ErrInvalidExecutionReceipt
	}
	if len(receipt.ApprovalReceiptDigest) != 64 || len(receipt.ManifestDigest) != 64 {
		return ErrInvalidExecutionReceipt
	}
	return nil
}

func (receipt ExecutionReceipt) CanonicalBytes() []byte {
	bytes, _ := json.Marshal(receipt)
	return bytes
}
func (receipt ExecutionReceipt) DigestHex() string {
	digest := sha256.Sum256(receipt.CanonicalBytes())
	return hex.EncodeToString(digest[:])
}
