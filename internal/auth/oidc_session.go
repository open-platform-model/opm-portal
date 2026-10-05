package auth

import (
	"net/http"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// session authenticates a request by its browser session.
func (o *OIDC) session(*http.Request) (authz.Identity, string, error) {
	return authz.Identity{}, "", ErrNoSession
}
