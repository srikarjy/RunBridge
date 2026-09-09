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
