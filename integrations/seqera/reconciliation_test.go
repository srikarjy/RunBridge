package seqera

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/srikarjy/RunBridge/internal/execution"
	"github.com/srikarjy/RunBridge/internal/reconciliation"
)

type reconciliationTransport struct {
	wantRunName string
}

func (transport reconciliationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Path != "/workflow" || request.URL.Query().Get("workspaceId") != "123" || request.URL.Query().Get("search") != transport.wantRunName {
		return nil, io.ErrUnexpectedEOF
	}
	body := `{"workflows":[` +
		`{"workspaceId":123,"workflow":{"id":"wf-match","runName":"` + transport.wantRunName + `","status":"RUNNING"}},` +
		`{"workspaceId":123,"workflow":{"id":"wf-prefix","runName":"` + transport.wantRunName + `-other","status":"RUNNING"}},` +
		`{"workspaceId":456,"workflow":{"id":"wf-other-workspace","runName":"` + transport.wantRunName + `","status":"RUNNING"}}]}`
	return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
}

func TestReconciliationObserverRequiresExactWorkspaceAndRunName(t *testing.T) {
	candidate := reconciliation.Candidate{WorkspaceID: "123", CorrelationID: "corr-1"}
	runName := execution.CorrelationRunName(candidate.CorrelationID)
	client, err := NewClient("https://seqera.example", "secret", &http.Client{Transport: reconciliationTransport{wantRunName: runName}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewReconciliationObserver(client).FindSubmission(context.Background(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Observations) != 1 || result.Observations[0].ExternalID != "wf-match" || result.DefinitiveFailure {
		t.Fatalf("result: %#v", result)
	}
	if _, err := strconv.ParseInt(result.Observations[0].WorkspaceID, 10, 64); err != nil {
		t.Fatal(err)
	}
}
