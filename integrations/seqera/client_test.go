package seqera

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/srikarjy/RunBridge/internal/runs"
)

type fakeTransport struct{ calls int }

func (t *fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.calls++
	if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("Accept-Version") != "1" {
		return nil, io.ErrUnexpectedEOF
	}
	var status int
	var body string
	switch r.Method + " " + r.URL.Path {
	case "POST /workflow/launch":
		if r.URL.Query().Get("workspaceId") != "123" {
			return nil, io.ErrUnexpectedEOF
		}
		data, _ := io.ReadAll(r.Body)
		text := string(data)
		if !strings.Contains(text, `"pipeline":"nf-core/rnaseq"`) || !strings.Contains(text, `"revision":"3.18.0"`) {
			return nil, io.ErrUnexpectedEOF
		}
		status, body = http.StatusOK, `{"workflowId":"wf-1"}`
	case "GET /workflow/wf-1":
		status, body = http.StatusOK, `{"id":"wf-1","status":"RUNNING"}`
	case "GET /workflow":
		if r.URL.Query().Get("workspaceId") != "123" || r.URL.Query().Get("search") != "runbridge-key" || r.URL.Query().Get("max") != "25" {
			return nil, io.ErrUnexpectedEOF
		}
		status, body = http.StatusOK, `{"hasMore":false,"totalSize":1,"workflows":[{"workspaceId":123,"workflow":{"id":"wf-1","runName":"runbridge-key","status":"RUNNING"}}]}`
	case "POST /workflow/wf-1/cancel":
		status = http.StatusNoContent
	default:
		status = http.StatusNotFound
	}
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

func TestClientListsWorkspaceWorkflows(t *testing.T) {
	client, err := NewClient("https://seqera.example", "secret", &http.Client{Transport: &fakeTransport{}})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.ListWorkflows(context.Background(), "123", "runbridge-key", 25)
	if err != nil {
		t.Fatal(err)
	}
	if response.TotalSize != 1 || len(response.Workflows) != 1 || response.Workflows[0].Workflow.RunName != "runbridge-key" {
		t.Fatalf("response: %#v", response)
	}
}

func TestWorkflowStatusMapsExternalState(t *testing.T) {
	client, err := NewClient("https://seqera.example", "secret", &http.Client{Transport: &fakeTransport{}})
	if err != nil {
		t.Fatal(err)
	}
	status, err := client.WorkflowStatus(context.Background(), "wf-1")
	if err != nil || status != runs.StatusRunning {
		t.Fatalf("status: %s %v", status, err)
	}
}

func TestClientUsesDocumentedWorkflowEndpoints(t *testing.T) {
	transport := &fakeTransport{}
	client, err := NewClient("https://seqera.example", "secret", &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := client.Submit(context.Background(), LaunchRequest{WorkspaceID: "123", Pipeline: "nf-core/rnaseq", Revision: "3.18.0", ParamsText: `{"input":"samples.csv"}`})
	if err != nil || launch.WorkflowID != "wf-1" {
		t.Fatalf("launch: %#v %v", launch, err)
	}
	workflow, err := client.GetWorkflow(context.Background(), launch.WorkflowID)
	if err != nil || workflow.Status != "RUNNING" {
		t.Fatalf("workflow: %#v %v", workflow, err)
	}
	if err := client.Cancel(context.Background(), launch.WorkflowID); err != nil {
		t.Fatal(err)
	}
	if transport.calls != 3 {
		t.Fatalf("calls = %d", transport.calls)
	}
}
