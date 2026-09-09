package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/srikarjy/RunBridge/internal/auth"
)

func TestStaticBearerAuthenticator(t *testing.T) {
	actorID, _ := auth.NewActorID("actor-1")
	actor, _ := auth.NewActor(actorID, "Researcher", auth.ActorKindHuman)
	authenticator, _ := NewStaticBearerAuthenticator("secret", actor)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer secret")
	got, err := authenticator.Resolve(request)
	if err != nil || got.ID() != actorID {
		t.Fatalf("valid bearer: %v", err)
	}
	request.Header.Set("Authorization", "Bearer wrong")
	if _, err := authenticator.Resolve(request); err == nil {
		t.Fatal("invalid bearer accepted")
	}
}
