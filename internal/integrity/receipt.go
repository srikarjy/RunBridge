package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidReceipt = errors.New("invalid approval receipt")

type ApprovalReceipt struct {
	ApprovalID          string    `json:"approval_id"`
	SpecificationDigest string    `json:"specification_digest"`
	Decision            string    `json:"decision"`
	DecidedAt           time.Time `json:"decided_at"`
}

func (receipt ApprovalReceipt) Validate() error {
	if strings.TrimSpace(receipt.ApprovalID) == "" || strings.TrimSpace(receipt.SpecificationDigest) == "" || strings.TrimSpace(receipt.Decision) == "" || receipt.DecidedAt.IsZero() {
		return ErrInvalidReceipt
	}
	if len(receipt.SpecificationDigest) != 64 {
		return ErrInvalidReceipt
	}
	return nil
}

func (receipt ApprovalReceipt) CanonicalBytes() []byte {
	bytes, _ := json.Marshal(receipt)
	return bytes
}
func (receipt ApprovalReceipt) DigestHex() string {
	digest := sha256.Sum256(receipt.CanonicalBytes())
	return hex.EncodeToString(digest[:])
}
