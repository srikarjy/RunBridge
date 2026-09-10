// Package seqera contains the narrow external boundary for Seqera Platform.
// It does not own RunBridge lifecycle state; callers persist that state locally.
package seqera

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/srikarjy/RunBridge/internal/execution"
	"github.com/srikarjy/RunBridge/internal/runs"
)

const DefaultBaseURL = "https://api.cloud.seqera.io"

var (
	ErrWorkspaceRequired  = errors.New("workspace ID is required")
	ErrWorkflowIDRequired = errors.New("workflow ID is required")
	ErrPipelineRequired   = errors.New("pipeline is required")
	ErrRevisionRequired   = errors.New("revision is required")
	ErrUnexpectedStatus   = errors.New("unexpected Seqera response status")
)

type LaunchRequest struct {
	WorkspaceID    string
	ComputeEnvID   string
	RunName        string
	Pipeline       string
	WorkDir        string
	Revision       string
	ConfigProfiles []string
	ParamsText     string
}

type LaunchResponse struct {
	WorkflowID string `json:"workflowId"`
}

type Workflow struct {
	ID      string `json:"id"`
	RunName string `json:"runName"`
	Status  string `json:"status"`
}

type WorkflowListItem struct {
	WorkspaceID int64    `json:"workspaceId"`
	Workflow    Workflow `json:"workflow"`
}

type ListWorkflowsResponse struct {
	HasMore   bool               `json:"hasMore"`
	TotalSize int64              `json:"totalSize"`
	Workflows []WorkflowListItem `json:"workflows"`
}

type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string, httpClient *http.Client) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse Seqera base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("parse Seqera base URL: invalid URL")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{baseURL: parsed, token: strings.TrimSpace(token), httpClient: httpClient}, nil
}

func (c *Client) Submit(ctx context.Context, request LaunchRequest) (LaunchResponse, error) {
	if strings.TrimSpace(request.WorkspaceID) == "" {
		return LaunchResponse{}, ErrWorkspaceRequired
	}
	if strings.TrimSpace(request.Pipeline) == "" {
		return LaunchResponse{}, ErrPipelineRequired
	}
	if strings.TrimSpace(request.Revision) == "" {
		return LaunchResponse{}, ErrRevisionRequired
	}
	body := struct {
		Launch map[string]any `json:"launch"`
	}{Launch: map[string]any{
		"pipeline": request.Pipeline, "revision": request.Revision,
	}}
	optional := map[string]any{"computeEnvId": request.ComputeEnvID, "runName": request.RunName, "workDir": request.WorkDir, "configProfiles": request.ConfigProfiles, "paramsText": request.ParamsText}
	for key, value := range optional {
		if value != nil && value != "" && !(key == "configProfiles" && len(request.ConfigProfiles) == 0) {
			body.Launch[key] = value
		}
	}
	var response struct {
		WorkflowID string   `json:"workflowId"`
		Workflow   Workflow `json:"workflow"`
	}
	err := c.doJSON(ctx, http.MethodPost, "/workflow/launch", request.WorkspaceID, body, &response)
	if err != nil {
		return LaunchResponse{}, err
	}
	id := response.WorkflowID
	if id == "" {
		id = response.Workflow.ID
	}
	if id == "" {
		return LaunchResponse{}, fmt.Errorf("%w: launch response did not include workflow ID", ErrUnexpectedStatus)
	}
	return LaunchResponse{WorkflowID: id}, nil
}

func (c *Client) GetWorkflow(ctx context.Context, workflowID string) (Workflow, error) {
	if strings.TrimSpace(workflowID) == "" {
		return Workflow{}, ErrWorkflowIDRequired
	}
	var workflow Workflow
	if err := c.doJSON(ctx, http.MethodGet, "/workflow/"+url.PathEscape(workflowID), "", nil, &workflow); err != nil {
		return Workflow{}, err
	}
	if workflow.ID == "" {
		workflow.ID = workflowID
	}
	return workflow, nil
}

// ListWorkflows uses Seqera's workspace-scoped workflow search. Search is
// treated as a candidate filter; callers must still compare exact run names.
func (c *Client) ListWorkflows(ctx context.Context, workspaceID, search string, max int) (ListWorkflowsResponse, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return ListWorkflowsResponse{}, ErrWorkspaceRequired
	}
	if max <= 0 || max > 100 {
		max = 100
	}
	query := url.Values{}
	query.Set("workspaceId", workspaceID)
	query.Set("search", search)
	query.Set("max", strconv.Itoa(max))
	var response ListWorkflowsResponse
	if err := c.doJSONQuery(ctx, http.MethodGet, "/workflow", query, nil, &response); err != nil {
		return ListWorkflowsResponse{}, err
	}
	return response, nil
}

func (c *Client) WorkflowStatus(ctx context.Context, workflowID string) (runs.Status, error) {
	workflow, err := c.GetWorkflow(ctx, workflowID)
	if err != nil {
		return "", err
	}
	return execution.NormalizeExternalStatus(workflow.Status)
}

func (c *Client) Cancel(ctx context.Context, workflowID string) error {
	if strings.TrimSpace(workflowID) == "" {
		return ErrWorkflowIDRequired
	}
	return c.doJSON(ctx, http.MethodPost, "/workflow/"+url.PathEscape(workflowID)+"/cancel", "", struct{}{}, nil)
}

func (c *Client) doJSON(ctx context.Context, method, path, workspaceID string, payload, result any) error {
	query := url.Values{}
	if workspaceID != "" {
		query.Set("workspaceId", workspaceID)
	}
	return c.doJSONQuery(ctx, method, path, query, payload, result)
}

func (c *Client) doJSONQuery(ctx context.Context, method, path string, query url.Values, payload, result any) error {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(c.baseURL.Path, "/") + path
	endpoint.RawQuery = query.Encode()
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode Seqera request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return fmt.Errorf("create Seqera request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Version", "1")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("Seqera request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%w: HTTP %s", ErrUnexpectedStatus, response.Status)
	}
	if result == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(result); err != nil {
		return fmt.Errorf("decode Seqera response: %w", err)
	}
	return nil
}

// ParseWorkspaceID accepts the numeric IDs used by Platform while keeping the
// adapter's public request shape string-based for compatibility with API JSON.
func ParseWorkspaceID(value string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(value), 10, 64)
}
