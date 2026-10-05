package auth

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestClaimMapperIdentity(t *testing.T) {
	m := claimMapper{usernameClaim: "sub", usernamePrefix: "oidc:", groupsClaim: "groups", groupsPrefix: "oidc:"}
	tests := []struct {
		name   string
		mapper claimMapper
		claims map[string]any
		user   string
		groups []string
	}{
		{
			name:   "user and groups are prefixed",
			mapper: m,
			claims: map[string]any{"sub": "alice", "groups": []any{"dev", "ops"}},
			user:   "oidc:alice",
			groups: []string{"oidc:dev", "oidc:ops", "system:authenticated"},
		},
		{
			name:   "system, empty and duplicate groups are dropped",
			mapper: m,
			claims: map[string]any{"sub": "alice", "groups": []any{"system:masters", "dev", "", "  ", "dev", "system:authenticated"}},
			user:   "oidc:alice",
			groups: []string{"oidc:dev", "system:authenticated"},
		},
		{
			name:   "a single group as a string",
			mapper: m,
			claims: map[string]any{"sub": "alice", "groups": "dev"},
			user:   "oidc:alice",
			groups: []string{"oidc:dev", "system:authenticated"},
		},
		{
			name:   "no groups claim in the token",
			mapper: m,
			claims: map[string]any{"sub": "alice"},
			user:   "oidc:alice",
			groups: []string{"system:authenticated"},
		},
		{
			name:   "groups are not read without a groups claim",
			mapper: claimMapper{usernameClaim: "sub", usernamePrefix: "oidc:"},
			claims: map[string]any{"sub": "alice", "groups": []any{"dev"}},
			user:   "oidc:alice",
			groups: []string{"system:authenticated"},
		},
		{
			name:   "a mapped group that becomes a system name is dropped",
			mapper: claimMapper{usernameClaim: "sub", usernamePrefix: "u-", groupsClaim: "groups", groupsPrefix: "sys"},
			claims: map[string]any{"sub": "alice", "groups": []any{"tem:masters", "dev"}},
			user:   "u-alice",
			groups: []string{"sysdev", "system:authenticated"},
		},
		{
			name:   "verified email",
			mapper: claimMapper{usernameClaim: "email", usernamePrefix: "oidc:"},
			claims: map[string]any{"email": "alice@example.com", "email_verified": true},
			user:   "oidc:alice@example.com",
			groups: []string{"system:authenticated"},
		},
		{
			name:   "unprefixed when the API server trusts the issuer",
			mapper: claimMapper{usernameClaim: "sub", groupsClaim: "groups"},
			claims: map[string]any{"sub": "alice", "groups": []any{"system:masters", "dev"}},
			user:   "alice",
			groups: []string{"dev", "system:authenticated"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := tt.mapper.identity(tt.claims)
			if err != nil {
				t.Fatalf("identity: %v", err)
			}
			if id.Username != tt.user || !slices.Equal(id.Groups, tt.groups) {
				t.Fatalf("identity = %q %q, want %q %q", id.Username, id.Groups, tt.user, tt.groups)
			}
			if !id.Authenticated() {
				t.Fatal("a mapped identity must be authenticated")
			}
		})
	}
}

// TestClaimMapperRefuses covers the fail-closed rules, including the empty
// claims that fell through to the server's identity in CVE-2026-23990.
func TestClaimMapperRefuses(t *testing.T) {
	m := claimMapper{usernameClaim: "sub", usernamePrefix: "oidc:", groupsClaim: "groups", groupsPrefix: "oidc:"}
	email := claimMapper{usernameClaim: "email", usernamePrefix: "oidc:"}
	tests := []struct {
		name   string
		mapper claimMapper
		claims map[string]any
	}{
		{"missing username claim", m, map[string]any{"groups": []any{"dev"}}},
		{"empty username claim", m, map[string]any{"sub": "", "groups": []any{"dev"}}},
		{"blank username claim", m, map[string]any{"sub": " \t"}},
		{"username claim not a string", m, map[string]any{"sub": 42}},
		{"username claim null", m, map[string]any{"sub": nil}},
		{"empty claims", m, map[string]any{}},
		{"system username", m, map[string]any{"sub": "system:admin"}},
		{"system username without prefix", claimMapper{usernameClaim: "sub"}, map[string]any{"sub": "system:serviceaccount:kube-system:default"}},
		{"anonymous without prefix", claimMapper{usernameClaim: "sub"}, map[string]any{"sub": "system:anonymous"}},
		{"mapped username becomes a system name", claimMapper{usernameClaim: "sub", usernamePrefix: "sys"}, map[string]any{"sub": "tem:admin"}},
		{"groups claim a number", m, map[string]any{"sub": "alice", "groups": 7}},
		{"groups claim holds a non-string", m, map[string]any{"sub": "alice", "groups": []any{"dev", 7}}},
		{"groups claim an object", m, map[string]any{"sub": "alice", "groups": map[string]any{"a": "b"}}},
		{"unverified email", email, map[string]any{"email": "alice@example.com", "email_verified": false}},
		{"email_verified not a bool", email, map[string]any{"email": "alice@example.com", "email_verified": "true"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := tt.mapper.identity(tt.claims)
			if !errors.Is(err, ErrUnmappedIdentity) {
				t.Fatalf("err = %v, want ErrUnmappedIdentity", err)
			}
			if id.Username != "" || id.Groups != nil {
				t.Fatalf("a refusal returned identity %+v", id)
			}
			for _, v := range []string{"alice", "system:admin", "tem:admin"} {
				if strings.Contains(err.Error(), v) {
					t.Fatalf("error %q names a claim value", err)
				}
			}
		})
	}
}

func TestCheckPrefixes(t *testing.T) {
	tests := []struct {
		name    string
		m       claimMapper
		trusted bool
		wantErr string
	}{
		{"both set", claimMapper{usernamePrefix: "oidc:", groupsClaim: "groups", groupsPrefix: "oidc:"}, false, ""},
		{"empty username prefix", claimMapper{groupsClaim: "groups", groupsPrefix: "oidc:"}, false, "username prefix is empty"},
		{"empty groups prefix with a groups claim", claimMapper{usernamePrefix: "oidc:", groupsClaim: "groups"}, false, "groups prefix is empty"},
		{"empty groups prefix without a groups claim", claimMapper{usernamePrefix: "oidc:"}, false, ""},
		{"both empty, API server trusts the issuer", claimMapper{groupsClaim: "groups"}, true, ""},
		{"username prefix inside system:", claimMapper{usernamePrefix: "sys"}, false, "could form a system: name"},
		{"groups prefix starts with system:", claimMapper{usernamePrefix: "oidc:", groupsClaim: "groups", groupsPrefix: "system:oidc:"}, false, "could form a system: name"},
		{"groups prefix system: even when trusted", claimMapper{groupsClaim: "groups", groupsPrefix: "system:"}, true, "could form a system: name"},
		{"groups prefix s", claimMapper{usernamePrefix: "oidc:", groupsClaim: "groups", groupsPrefix: "s"}, false, "could form a system: name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkPrefixes(tt.m, tt.trusted)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("checkPrefixes: %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
