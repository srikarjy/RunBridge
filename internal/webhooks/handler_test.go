package webhooks

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/srikarjy/RunBridge/internal/events"
	"github.com/srikarjy/RunBridge/internal/runs"
	"github.com/srikarjy/RunBridge/internal/security"
)

func TestHandlerVerifiesBeforeSinking(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	verifier, _ := security.NewWebhookVerifier("secret", time.Minute)
	body := []byte(`{"Source":"seqera","ID":"evt-1","ExternalExecutionID":"wf-1","Status":"RUNNING","OccurredAt":"2023-11-14T22:13:20Z"}`)
	called := false
	handler := NewHandler(verifier, func(_ *http.Request, event events.Event) error {
		called = true
		if event.Status != runs.StatusRunning {
			t.Fatal("status not decoded")
		}
		return nil
	})
	handler.now = func() time.Time { return now }
	request := httptest.NewRequest(http.MethodPost, "/webhooks/seqera", bytesReader(body))
	request.Header.Set("X-RunBridge-Timestamp", "1700000000")
	request.Header.Set("X-RunBridge-Signature", verifier.Sign(now.Unix(), body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || !called {
		t.Fatalf("accepted webhook: %d called=%v", response.Code, called)
	}
	request = httptest.NewRequest(http.MethodPost, "/webhooks/seqera", bytesReader([]byte("tampered")))
	request.Header.Set("X-RunBridge-Timestamp", "1700000000")
	request.Header.Set("X-RunBridge-Signature", verifier.Sign(now.Unix(), body))
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("tampered webhook: %d", response.Code)
	}
}

func bytesReader(value []byte) *byteReader { return &byteReader{value: value} }

type byteReader struct{ value []byte }

func (reader *byteReader) Read(buffer []byte) (int, error) {
	if len(reader.value) == 0 {
		return 0, io.EOF
	}
	count := copy(buffer, reader.value)
	reader.value = reader.value[count:]
	return count, nil
}
func (reader *byteReader) Close() error { return nil }
