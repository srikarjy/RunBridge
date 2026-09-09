package security

import (
	"errors"
	"testing"
	"time"
)

func TestWebhookVerifierSignsAndVerifiesRawBody(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	verifier, err := NewWebhookVerifier("test-secret", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"id":"evt-1","status":"RUNNING"}`)
	signature := verifier.Sign(now.Unix(), body)
	if err := verifier.Verify(now, now.Unix(), signature, body); err != nil {
		t.Fatal(err)
	}
	if err := verifier.Verify(now, now.Unix(), signature, []byte("tampered")); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("tampered body: %v", err)
	}
}

func TestWebhookVerifierRejectsReplayAndMalformedSignatures(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	verifier, _ := NewWebhookVerifier("secret", time.Minute)
	signature := verifier.Sign(now.Add(-2*time.Minute).Unix(), []byte("body"))
	if err := verifier.Verify(now, now.Add(-2*time.Minute).Unix(), signature, []byte("body")); !errors.Is(err, ErrTimestampInvalid) {
		t.Fatalf("replay: %v", err)
	}
	if err := verifier.Verify(now, now.Unix(), "not-hex", []byte("body")); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("malformed: %v", err)
	}
}
