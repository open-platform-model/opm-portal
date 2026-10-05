package api_test

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
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

// TestEmptyIdentityNeverReachesTheClusterInCluster guards the class of bug
// where a request whose identity names no one runs as the server's own
// identity. In in-cluster mode, an identity with an empty, blank or
// anonymous username must be refused by the read API with no access review
// and no read, for the caller or the portal's own reader.
func TestEmptyIdentityNeverReachesTheClusterInCluster(t *testing.T) {
	alice := authz.Identity{Username: "alice", Groups: []string{"dev", "system:authenticated"}}
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

	var (
		mu      sync.Mutex
		current authz.Identity
	)
	srv, err := api.New(api.Config{
		Mode:       api.ModeInCluster,
		Model:      m,
		Authorizer: az,
		Reader:     reader,
		Connection: api.Connection{Source: "in-cluster"},
		Authenticate: func(*http.Request) (api.Principal, error) {
			mu.Lock()
			defer mu.Unlock()
			return api.Principal{Identity: current, Session: "session"}, nil
		},
	})
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	t.Cleanup(srv.Close)

	get := func(t *testing.T, id authz.Identity) (int, string) {
		t.Helper()
		mu.Lock()
		current = id
		mu.Unlock()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, api.Prefix+"/clusters/"+api.DefaultCluster+"/instances", http.NoBody)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, r)
		body, _ := io.ReadAll(w.Result().Body)
		return w.Code, string(body)
	}
	refused := map[string]authz.Identity{
		"empty username":            {Groups: []string{"dev", "system:authenticated"}},
		"blank username":            {Username: "   ", Groups: []string{"system:authenticated"}},
		"empty username and groups": {Groups: []string{"dev", "system:masters"}},
		"anonymous username":        {Username: "system:anonymous", Groups: []string{"system:masters"}},
	}
	reads := func() []string {
		return slices.DeleteFunc(readmodeltest.Actions(dyn), func(a string) bool { return strings.HasPrefix(a, "watch ") })
	}
	before := struct {
		alice, reader, checks int
		reads                 []string
	}{aliceReviews.Count(), readerReviews.Count(), len(az.asked()), reads()}
	for name, id := range refused {
		t.Run(name, func(t *testing.T) {
			status, body := get(t, id)
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

	// A named user reaches the cluster as that user.
	status, body := get(t, alice)
	if status != http.StatusOK {
		t.Fatalf("a named user: status %d, body %s", status, body)
	}
	if aliceReviews.Count() == before.alice {
		t.Fatal("a named user's read sent no review")
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
