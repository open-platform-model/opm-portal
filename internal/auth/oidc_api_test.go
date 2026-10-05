package auth_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-platform-model/opm-portal/internal/api"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
	"github.com/open-platform-model/opm-portal/internal/auth"
	"github.com/open-platform-model/opm-portal/internal/auth/oidctest"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

// TestEmptyClaimsNeverReachTheCluster is the regression test for
// CVE-2026-23990, where requests with empty OIDC claims ran as the server's
// own identity. A correctly signed token for the configured audience whose
// claims map to no user must be refused by the read API with no access
// review and no read, for the user or the portal's own reader.
func TestEmptyClaimsNeverReachTheCluster(t *testing.T) {
	iss := oidctest.New(t, oidctest.Options{})
	o, err := auth.NewOIDC(t.Context(), auth.OIDCConfig{
		IssuerURL:      iss.URL,
		ClientID:       iss.ClientID(),
		RedirectURL:    "https://portal.example.com" + auth.CallbackPath,
		UsernamePrefix: "oidc:",
		GroupsClaim:    "groups",
		GroupsPrefix:   "oidc:",
		HTTPClient:     iss.Client(),
	})
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	alice := authz.Identity{Username: "oidc:alice", Groups: []string{"oidc:dev", "system:authenticated"}}
	reader := authz.Identity{Username: "portal-reader", Groups: []string{"system:authenticated"}}
	aliceChecker, aliceReviews := readmodeltest.NewChecker(t, alice, readmodeltest.AllowAll, authz.Options{})
	readerChecker, readerReviews := readmodeltest.NewChecker(t, reader, readmodeltest.AllowAll, authz.Options{})
	az := &recordingAuthorizer{next: readmodeltest.ByIdentity{alice.Username: aliceChecker, reader.Username: readerChecker}}
	dyn := readmodeltest.Dynamic(apitest.F1(t)...)
	m, err := readmodel.New(readmodel.Config{
		Dynamic:     dyn,
		Discovery:   readmodeltest.Discovery(readmodeltest.Kinds...),
		Authorizer:  az,
		Reader:      reader,
		SyncTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("readmodel.New: %v", err)
	}
	if err := m.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(m.Stop)
	srv, err := api.New(api.Config{
		Mode:       api.ModeInCluster,
		Model:      m,
		Authorizer: az,
		Reader:     reader,
		Authenticate: func(r *http.Request) (api.Principal, error) {
			id, session, err := o.Authenticate(r)
			return api.Principal{Identity: id, Session: session}, err
		},
	})
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	t.Cleanup(srv.Close)

	get := func(t *testing.T, claims map[string]any) (int, string) {
		t.Helper()
		token := iss.Sign(claims)
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, api.Prefix+"/clusters/"+api.DefaultCluster+"/instances", http.NoBody)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, r)
		body, _ := io.ReadAll(w.Result().Body)
		return w.Code, string(body)
	}
	claims := func(edit func(map[string]any)) map[string]any {
		c := iss.Claims("alice", iss.ClientID())
		edit(c)
		return c
	}
	refused := map[string]map[string]any{
		"empty username":            claims(func(c map[string]any) { c["sub"] = "" }),
		"missing username":          claims(func(c map[string]any) { delete(c, "sub") }),
		"blank username":            claims(func(c map[string]any) { c["sub"] = "   " }),
		"empty username and groups": claims(func(c map[string]any) { c["sub"] = ""; c["groups"] = []string{"dev", "system:masters"} }),
		"only system groups":        claims(func(c map[string]any) { delete(c, "sub"); c["groups"] = []string{"system:masters"} }),
		"username not a string":     claims(func(c map[string]any) { c["sub"] = []string{"alice"} }),
		"system username":           claims(func(c map[string]any) { c["sub"] = "system:admin" }),
	}
	reads := func() []string {
		return slices.DeleteFunc(readmodeltest.Actions(dyn), func(a string) bool { return strings.HasPrefix(a, "watch ") })
	}
	before := struct {
		alice, reader, checks int
		reads                 []string
	}{aliceReviews.Count(), readerReviews.Count(), len(az.asked()), reads()}
	for name, c := range refused {
		t.Run(name, func(t *testing.T) {
			status, body := get(t, c)
			if status != http.StatusUnauthorized || !strings.Contains(body, "unauthenticated") {
				t.Fatalf("status %d, body %s; want 401 unauthenticated", status, body)
			}
			if asked := az.asked(); len(asked) != before.checks {
				t.Fatalf("authorization was asked for %q", asked[before.checks:])
			}
			if aliceReviews.Count() != before.alice || readerReviews.Count() != before.reader {
				t.Fatalf("reviews were sent: alice %d->%d, reader %d->%d",
					before.alice, aliceReviews.Count(), before.reader, readerReviews.Count())
			}
			if got := reads(); !slices.Equal(got, before.reads) {
				t.Fatalf("reads were made: %v", got[len(before.reads):])
			}
		})
	}

	// The same token shape with a username reaches the cluster as that user.
	status, body := get(t, claims(func(c map[string]any) { c["groups"] = []string{"dev"} }))
	if status != http.StatusOK {
		t.Fatalf("a mapped user: status %d, body %s", status, body)
	}
	if aliceReviews.Count() == before.alice {
		t.Fatal("a mapped user's read sent no review")
	}
}

// recordingAuthorizer records every identity an authorization is asked
// for, so a check for an identity no fake checker serves still shows.
type recordingAuthorizer struct {
	next authz.Authorizer
	mu   sync.Mutex
	who  []string
}

func (a *recordingAuthorizer) Check(ctx context.Context, who authz.Identity, req authz.Attributes) (authz.Grant, error) {
	a.mu.Lock()
	a.who = append(a.who, who.Username)
	a.mu.Unlock()
	return a.next.Check(ctx, who, req)
}

func (a *recordingAuthorizer) asked() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.who)
}
