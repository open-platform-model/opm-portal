package authz

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
)

type accessLine struct {
	Msg         string `json:"msg"`
	User        string `json:"user"`
	Verb        string `json:"verb"`
	Group       string `json:"group"`
	Version     string `json:"version"`
	Resource    string `json:"resource"`
	Subresource string `json:"subresource"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Decision    string `json:"decision"`
	Code        string `json:"code"`
	Cached      bool   `json:"cached"`
}

func accessLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, nil)), &buf
}

func accessLines(t *testing.T, buf *bytes.Buffer) []accessLine {
	t.Helper()
	var out []accessLine
	for raw := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if raw == "" {
			continue
		}
		var l accessLine
		if err := json.Unmarshal([]byte(raw), &l); err != nil {
			t.Fatalf("access log line %q: %v", raw, err)
		}
		if l.Msg != "access" {
			t.Fatalf("access log line %q has message %q", raw, l.Msg)
		}
		out = append(out, l)
	}
	return out
}

func TestAccessLogRecordsEachDecisionAboutAPerson(t *testing.T) {
	sc := newSARCluster(t)
	sc.answer = func(s authorizationv1.SubjectAccessReviewSpec) authorizationv1.SubjectAccessReviewStatus {
		return authorizationv1.SubjectAccessReviewStatus{Allowed: s.ResourceAttributes.Namespace == "team-a"}
	}
	log, buf := accessLogger()
	c := sc.checker(t, Options{AccessLog: log})
	allowed := Attributes{Verb: "get", Resource: pods, Subresource: "log", Namespace: "team-a", Name: "web-0"}
	denied := getDeployment("team-b", "api")

	_, _ = c.Check(t.Context(), person, allowed)
	_, _ = c.Check(t.Context(), person, denied)
	_, _ = c.Check(t.Context(), person, allowed)
	_, _ = c.Check(t.Context(), portalReader(t), allowed)
	_, _ = c.Check(t.Context(), Identity{Groups: []string{"oidc:team-a"}}, allowed)
	_, _ = c.Check(t.Context(), Identity{Username: "system:admin"}, allowed)
	_, _ = c.Check(t.Context(), person, Attributes{Verb: "get", Resource: secrets, Namespace: "team-a", Name: "db"})

	want := []accessLine{
		{User: "oidc:alice", Verb: "get", Version: "v1", Resource: "pods", Subresource: "log", Namespace: "team-a", Name: "web-0", Decision: "allow"},
		{User: "oidc:alice", Verb: "get", Group: "apps", Version: "v1", Resource: "deployments", Namespace: "team-b", Name: "api", Decision: "deny", Code: "forbidden"},
		{User: "oidc:alice", Verb: "get", Version: "v1", Resource: "pods", Subresource: "log", Namespace: "team-a", Name: "web-0", Decision: "allow", Cached: true},
		// The reader's own decision is not logged.
		{User: "", Verb: "get", Version: "v1", Resource: "pods", Subresource: "log", Namespace: "team-a", Name: "web-0", Decision: "deny", Code: "unauthenticated"},
		{User: "system:admin", Verb: "get", Version: "v1", Resource: "pods", Subresource: "log", Namespace: "team-a", Name: "web-0", Decision: "deny", Code: "unauthenticated"},
		{User: "oidc:alice", Verb: "get", Version: "v1", Resource: "secrets", Namespace: "team-a", Name: "db", Decision: "deny", Code: "forbidden"},
	}
	got := accessLines(t, buf)
	if len(got) != len(want) {
		t.Fatalf("access log has %d lines, want %d:\n%s", len(got), len(want), buf)
	}
	for i := range want {
		want[i].Msg = "access"
		if got[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestAccessLogCarriesNoGroupsExtraOrCause(t *testing.T) {
	sc := newSARCluster(t)
	sc.err = errors.New("webhook refused: oidc:bob is in group admins")
	log, buf := accessLogger()
	c := sc.checker(t, Options{AccessLog: log})
	_, err := c.Check(t.Context(), person, getDeployment("team-a", "web"))
	requireDenial(t, Grant{}, err, CodeUnavailable)

	got := accessLines(t, buf)
	if len(got) != 1 || got[0].Decision != "deny" || got[0].Code != "unavailable" || got[0].Cached {
		t.Fatalf("access log = %+v, want one uncached unavailable deny", got)
	}
	for _, leak := range []string{"bob", "admins", "webhook", "oidc:team-a", "system:authenticated", "mfa", "a1b2"} {
		if strings.Contains(buf.String(), leak) {
			t.Errorf("access log carries %q:\n%s", leak, buf)
		}
	}
}

func TestNoAccessLogWithoutALogger(t *testing.T) {
	sc := newSARCluster(t)
	if _, err := sc.checker(t, Options{}).Check(t.Context(), person, getDeployment("team-a", "web")); err != nil {
		t.Fatalf("Check = %v", err)
	}
	var nilChecker *Checker
	if _, err := nilChecker.Check(t.Context(), person, getDeployment("team-a", "web")); err == nil {
		t.Fatal("a nil Checker granted")
	}
}
