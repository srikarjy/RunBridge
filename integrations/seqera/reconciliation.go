package seqera

import (
	"context"
	"strconv"

	"github.com/srikarjy/RunBridge/internal/execution"
	"github.com/srikarjy/RunBridge/internal/reconciliation"
)

// ReconciliationObserver looks up uncertain attempts by their deterministic
// Seqera run name. An empty search result is deliberately non-definitive
// because a newly accepted workflow may not yet be visible to list queries.
type ReconciliationObserver struct{ client *Client }

func NewReconciliationObserver(client *Client) *ReconciliationObserver {
	return &ReconciliationObserver{client: client}
}

func (observer *ReconciliationObserver) FindSubmission(ctx context.Context, candidate reconciliation.Candidate) (reconciliation.ObservationResult, error) {
	if observer == nil || observer.client == nil {
		return reconciliation.ObservationResult{}, ErrUnexpectedStatus
	}
	runName := execution.CorrelationRunName(candidate.CorrelationID)
	response, err := observer.client.ListWorkflows(ctx, candidate.WorkspaceID, runName, 100)
	if err != nil {
		return reconciliation.ObservationResult{}, err
	}
	workspaceID, err := strconv.ParseInt(candidate.WorkspaceID, 10, 64)
	if err != nil {
		return reconciliation.ObservationResult{}, ErrWorkspaceRequired
	}
	result := reconciliation.ObservationResult{}
	for _, item := range response.Workflows {
		if item.WorkspaceID != workspaceID || item.Workflow.RunName != runName || item.Workflow.ID == "" {
			continue
		}
		result.Observations = append(result.Observations, reconciliation.Observation{
			ExternalID:    item.Workflow.ID,
			WorkspaceID:   candidate.WorkspaceID,
			CorrelationID: candidate.CorrelationID,
		})
	}
	return result, nil
}
