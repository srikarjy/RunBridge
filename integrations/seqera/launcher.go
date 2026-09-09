package seqera

import (
	"context"

	"github.com/srikarjy/RunBridge/internal/execution"
)

// LauncherAdapter bridges the lifecycle coordinator to the Seqera transport
// without exposing Seqera response types to the execution package.
type LauncherAdapter struct{ client *Client }

func NewLauncherAdapter(client *Client) *LauncherAdapter { return &LauncherAdapter{client: client} }

func (adapter *LauncherAdapter) Submit(ctx context.Context, request execution.LaunchRequest) (execution.LaunchResponse, error) {
	if adapter == nil || adapter.client == nil {
		return execution.LaunchResponse{}, ErrUnexpectedStatus
	}
	response, err := adapter.client.Submit(ctx, LaunchRequest{WorkspaceID: request.WorkspaceID, Pipeline: request.Pipeline, Revision: request.Revision, ParamsText: request.ParamsText})
	if err != nil {
		return execution.LaunchResponse{}, err
	}
	return execution.LaunchResponse{ExternalExecutionID: response.WorkflowID}, nil
}
