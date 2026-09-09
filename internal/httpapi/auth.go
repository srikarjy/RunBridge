package httpapi

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/srikarjy/RunBridge/internal/auth"
)

var ErrAuthenticationFailed = errors.New("authentication failed")

// StaticBearerAuthenticator is suitable for local development and controlled
// deployments. Production can replace it with an OIDC or workload identity
// resolver without changing handlers.
type StaticBearerAuthenticator struct {
	token string
	actor auth.Actor
}

func NewStaticBearerAuthenticator(token string, actor auth.Actor) (*StaticBearerAuthenticator, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrAuthenticationFailed
	}
	return &StaticBearerAuthenticator{token: token, actor: actor}, nil
}

func (authenticator *StaticBearerAuthenticator) Resolve(request *http.Request) (auth.Actor, error) {
	if authenticator == nil {
		return auth.Actor{}, ErrAuthenticationFailed
	}
	prefix, token, ok := strings.Cut(request.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(prefix, "Bearer") || subtle.ConstantTimeCompare([]byte(token), []byte(authenticator.token)) != 1 {
		return auth.Actor{}, ErrAuthenticationFailed
	}
	return authenticator.actor, nil
}
