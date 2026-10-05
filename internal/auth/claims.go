package auth

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// systemPrefix starts every name Kubernetes reserves for itself.
const systemPrefix = "system:"

// groupAuthenticated is the group Kubernetes gives every authenticated
// principal.
const groupAuthenticated = "system:authenticated"

// ErrUnmappedIdentity is returned for a token whose claims map to no
// portal user. Its wrapped text names the rule, never a claim value.
var ErrUnmappedIdentity = errors.New("the token's claims map to no user")

// claimMapper turns a verified token's claims into the identity a
// SubjectAccessReview is sent for.
type claimMapper struct {
	usernameClaim  string
	usernamePrefix string
	groupsClaim    string
	groupsPrefix   string
}

// identity maps claims, failing closed (0030:D6:R2/R3). The username claim
// is checked before its prefix is applied: a prefix would turn an empty
// claim into a non-empty name such as "oidc:", the shape of the bug where
// empty claims fell through to the server's own identity (CVE-2026-23990).
func (m claimMapper) identity(claims map[string]any) (authz.Identity, error) {
	username, err := m.username(claims)
	if err != nil {
		return authz.Identity{}, err
	}
	groups, err := m.groups(claims)
	if err != nil {
		return authz.Identity{}, err
	}
	id := authz.Identity{Username: username, Groups: groups}
	if !id.Authenticated() {
		return authz.Identity{}, fmt.Errorf("%w: the username names no principal", ErrUnmappedIdentity)
	}
	return id, nil
}

// username returns the prefixed username, refusing an empty or system one.
func (m claimMapper) username(claims map[string]any) (string, error) {
	raw, ok := claims[m.usernameClaim].(string)
	if !ok || strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("%w: the username claim is missing, empty or not a string", ErrUnmappedIdentity)
	}
	if m.usernameClaim == "email" {
		if v, present := claims["email_verified"]; present {
			if verified, ok := v.(bool); !ok || !verified {
				return "", fmt.Errorf("%w: the email is not verified", ErrUnmappedIdentity)
			}
		}
	}
	username := m.usernamePrefix + raw
	if strings.HasPrefix(raw, systemPrefix) || strings.HasPrefix(username, systemPrefix) {
		return "", fmt.Errorf("%w: the username is a system name", ErrUnmappedIdentity)
	}
	return username, nil
}

// groups returns the prefixed groups with every system: group dropped,
// before and after the prefix, and system:authenticated added.
func (m claimMapper) groups(claims map[string]any) ([]string, error) {
	rawGroups, err := m.rawGroups(claims)
	if err != nil {
		return nil, err
	}
	groups := make([]string, 0, len(rawGroups)+1)
	for _, g := range rawGroups {
		if strings.TrimSpace(g) == "" || strings.HasPrefix(g, systemPrefix) {
			continue
		}
		mapped := m.groupsPrefix + g
		if strings.HasPrefix(mapped, systemPrefix) || slices.Contains(groups, mapped) {
			continue
		}
		groups = append(groups, mapped)
	}
	return append(groups, groupAuthenticated), nil
}

// rawGroups reads the groups claim: absent, a string, or a list of strings.
func (m claimMapper) rawGroups(claims map[string]any) ([]string, error) {
	if m.groupsClaim == "" {
		return nil, nil
	}
	switch v := claims[m.groupsClaim].(type) {
	case nil:
		return nil, nil
	case string:
		return []string{v}, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, g := range v {
			s, ok := g.(string)
			if !ok {
				return nil, fmt.Errorf("%w: the groups claim holds a value that is not a string", ErrUnmappedIdentity)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%w: the groups claim is not a string or a list of strings", ErrUnmappedIdentity)
	}
}

// checkPrefixes refuses prefixes that would let an identity provider
// choose a name the API server trusts on its own. A prefix may be empty
// only when the API server trusts the same issuer with the same prefixes
// (0030:D6:R6); the groups prefix matters only when groups are read. A
// prefix that is a prefix of "system:", or starts with it, is refused in
// any case, so "sys" and a group "tem:masters" cannot form
// "system:masters".
func checkPrefixes(m claimMapper, apiServerTrustsIssuer bool) error {
	if m.usernamePrefix == "" && !apiServerTrustsIssuer {
		return errors.New("the username prefix is empty and the API server is not declared to trust the issuer")
	}
	if m.groupsClaim != "" && m.groupsPrefix == "" && !apiServerTrustsIssuer {
		return errors.New("the groups prefix is empty and the API server is not declared to trust the issuer")
	}
	for _, p := range [...]struct{ name, value string }{{"username", m.usernamePrefix}, {"groups", m.groupsPrefix}} {
		if p.value != "" && (strings.HasPrefix(systemPrefix, p.value) || strings.HasPrefix(p.value, systemPrefix)) {
			return fmt.Errorf("the %s prefix %q could form a system: name", p.name, p.value)
		}
	}
	return nil
}
