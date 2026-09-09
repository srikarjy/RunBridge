package audit

import (
	"testing"
	"time"
)

func TestAuditEventIdentityRules(t *testing.T) {
	now := time.Now().UTC()
	event := Event{ID: "evt-1", ProjectID: "project-1", ActorKind: External, Type: ExecutionStateChanged, RecordedAt: now, SourceSystem: "seqera", SourceEventID: "source-1"}
	if err := event.Validate(); err != nil {
		t.Fatal(err)
	}
	event.SourceEventID = ""
	if err := event.Validate(); err == nil {
		t.Fatal("expected paired source identity error")
	}
}

func TestHumanEventsRequireActor(t *testing.T) {
	event := Event{ID: "evt-1", ProjectID: "project-1", ActorKind: Human, Type: ProposalCreated, RecordedAt: time.Now()}
	if err := event.Validate(); err == nil {
		t.Fatal("expected actor error")
	}
}
