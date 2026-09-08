// Package auth defines identities used by the RunBridge domain.
package auth

import (
	"errors"
	"strings"
)

var ErrActorIDRequired = errors.New("actor ID is required")
var ErrActorKindInvalid = errors.New("actor kind is invalid")

type ActorID string

func NewActorID(value string) (ActorID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrActorIDRequired
	}
	return ActorID(value), nil
}

func (id ActorID) String() string { return string(id) }

type ActorKind string

const (
	ActorKindHuman ActorKind = "human"
)

func (kind ActorKind) Valid() bool {
	return kind == ActorKindHuman
}

// Actor is the minimal authenticated identity known to the domain.
type Actor struct {
	id          ActorID
	displayName string
	kind        ActorKind
}

func NewActor(id ActorID, displayName string, kind ActorKind) (Actor, error) {
	if strings.TrimSpace(id.String()) == "" {
		return Actor{}, ErrActorIDRequired
	}
	if !kind.Valid() {
		return Actor{}, ErrActorKindInvalid
	}
	return Actor{id: id, displayName: strings.TrimSpace(displayName), kind: kind}, nil
}

func (actor Actor) ID() ActorID         { return actor.id }
func (actor Actor) DisplayName() string { return actor.displayName }
func (actor Actor) Kind() ActorKind     { return actor.kind }
