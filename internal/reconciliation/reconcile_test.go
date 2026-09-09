package reconciliation

import "testing"

func TestDecideConservatively(t *testing.T) {
	if got := Decide(nil, false); got.Kind != RemainUnknown {
		t.Fatalf("empty eventual-consistency result: %#v", got)
	}
	if got := Decide(nil, true); got.Kind != RetryAllowed {
		t.Fatalf("definitive failure: %#v", got)
	}
	if got := Decide([]Observation{{ExternalID: "wf-1"}}, false); got.Kind != AdoptExecution || got.ExternalID != "wf-1" {
		t.Fatalf("single match: %#v", got)
	}
	if got := Decide([]Observation{{ExternalID: "wf-1"}, {ExternalID: "wf-2"}}, false); got.Kind != ManualReview {
		t.Fatalf("multiple matches: %#v", got)
	}
}

func TestRetryPolicyIsBounded(t *testing.T) {
	if err := CanRetry(1, 3, true); err != nil {
		t.Fatal(err)
	}
	if err := CanRetry(3, 3, true); err == nil {
		t.Fatal("retry limit ignored")
	}
	if err := CanRetry(1, 3, false); err == nil {
		t.Fatal("uncertain result allowed blind retry")
	}
}

func TestFindMatchesUsesStableCorrelation(t *testing.T) {
	observations := []Observation{{ExternalID: "wf-1", WorkspaceID: "ws-1", CorrelationID: "corr-1"}, {ExternalID: "wf-2", WorkspaceID: "ws-1", CorrelationID: "other"}}
	matches := FindMatches(observations, "ws-1", "corr-1")
	if len(matches) != 1 || matches[0].ExternalID != "wf-1" {
		t.Fatalf("matches: %#v", matches)
	}
}
