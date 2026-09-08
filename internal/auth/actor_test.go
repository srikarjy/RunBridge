package auth_test

import (
	"errors"
	"testing"

	"github.com/srikarjy/RunBridge/internal/auth"
)

func TestNewActorIDRejectsBlankValue(t *testing.T) {
	_, err := auth.NewActorID("  ")
	if !errors.Is(err, auth.ErrActorIDRequired) {
		t.Fatalf("expected ErrActorIDRequired, got %v", err)
	}
}

func TestNewActorRequiresKnownKind(t *testing.T) {
	id, err := auth.NewActorID("actor-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.NewActor(id, "Researcher", auth.ActorKind("robot")); err == nil {
		t.Fatal("expected invalid actor kind to fail")
	}
}
