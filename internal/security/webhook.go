// Package security contains small, vendor-neutral security primitives used at
// integration boundaries.
package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrSignatureInvalid = errors.New("webhook signature is invalid")
	ErrTimestampInvalid = errors.New("webhook timestamp is invalid")
)

// WebhookVerifier verifies signatures over timestamp + "." + raw body. The
// timestamp window limits replay, while constant-time comparison avoids
// leaking signature information. Header names and vendor prefixes stay at the
// HTTP adapter boundary.
type WebhookVerifier struct {
	secret []byte
	MaxAge time.Duration
}

func NewWebhookVerifier(secret string, maxAge time.Duration) (*WebhookVerifier, error) {
	if strings.TrimSpace(secret) == "" || maxAge <= 0 {
		return nil, fmt.Errorf("webhook verifier requires secret and positive max age")
	}
	return &WebhookVerifier{secret: []byte(secret), MaxAge: maxAge}, nil
}

func (verifier *WebhookVerifier) Sign(timestamp int64, body []byte) string {
	digest := hmac.New(sha256.New, verifier.secret)
	digest.Write([]byte(strconv.FormatInt(timestamp, 10)))
	digest.Write([]byte("."))
	digest.Write(body)
	return hex.EncodeToString(digest.Sum(nil))
}

func (verifier *WebhookVerifier) Verify(now time.Time, timestamp int64, signature string, body []byte) error {
	when := time.Unix(timestamp, 0)
	if now.IsZero() || timestamp <= 0 || when.After(now.Add(verifier.MaxAge)) || when.Before(now.Add(-verifier.MaxAge)) {
		return ErrTimestampInvalid
	}
	want := verifier.Sign(timestamp, body)
	provided, err := hex.DecodeString(strings.TrimSpace(signature))
	if err != nil || len(provided) != sha256.Size {
		return ErrSignatureInvalid
	}
	expected, _ := hex.DecodeString(want)
	if subtle.ConstantTimeCompare(provided, expected) != 1 {
		return ErrSignatureInvalid
	}
	return nil
}
