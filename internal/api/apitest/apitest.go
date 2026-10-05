// Package apitest serves the read API over a committed cluster capture, for
// the tests of packages that consume the API, such as the web UI, without
// importing the read model themselves.
package apitest

import (
	"errors"
	"net/http"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

// SessionCookie is the cookie a request carries to be Caller; its value is
// SessionValue. Anything else is unauthenticated.
const (
	SessionCookie = "opm-portal-test"
	SessionValue  = "session-alice"
)

// Context is the kubeconfig context, and the name of its cluster entry, a
// local-mode server says it reads with: the e2e fixture cluster's.
const Context = "kind-opm-portal-e2e"

var (
	// Caller is who an authenticated request reads as.
	Caller = authz.Identity{Username: "alice", Groups: []string{"system:authenticated"}}
	reader = authz.Identity{Username: "portal-reader", Groups: []string{"system:authenticated"}}
	// Now is the read model's clock, so evaluation times are stable.
	Now = time.Date(2026, 10, 4, 18, 10, 0, 0, time.UTC)
)

// Rule answers the caller's access reviews.
type Rule = readmodeltest.Rule

// AllowAll allows every review.
var AllowAll Rule = readmodeltest.AllowAll

// DenyResources denies every review of the named resources.
func DenyResources(resources ...string) Rule { return readmodeltest.DenyResources(resources...) }

// root is the repository root, found from this file so callers in any
// package resolve the same captures.
func root() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// F1 returns every object of the F1 capture.
func F1(t testing.TB) []*unstructured.Unstructured {
	t.Helper()
	return readmodeltest.LoadCapture(t, filepath.Join(root(), "testdata", "clusters", "f1"))
}

// F1Broken is F1 with default/podinfo and its objects replaced by the
// design evidence 01 image-break samples: Applied and Degraded.
func F1Broken(t testing.TB) []*unstructured.Unstructured {
	t.Helper()
	dir := filepath.Join(root(), "internal", "health", "testdata")
	replace := readmodeltest.LoadList(t, filepath.Join(dir, "mi-podinfo-image-broken.yaml"))
	replace = append(replace, readmodeltest.LoadList(t, filepath.Join(dir, "objects-podinfo-phase4-broken-1min.yaml"))...)
	drop := map[string]bool{}
	for _, o := range replace {
		drop[readmodeltest.Key(o)] = true
	}
	var out []*unstructured.Unstructured
	for _, o := range F1(t) {
		if drop[readmodeltest.Key(o)] {
			continue
		}
		if o.GetNamespace() == "default" && (o.GetKind() == "ReplicaSet" || o.GetKind() == "Pod") &&
			o.GetLabels()["module-instance.opmodel.dev/name"] == "podinfo" {
			continue
		}
		out = append(out, o)
	}
	return append(out, replace...)
}

// New serves the read API in local mode over objs. Reviews for Caller
// follow rule; the reader may read everything. A request is Caller's when
// it carries SessionCookie with SessionValue.
func New(t testing.TB, objs []*unstructured.Unstructured, rule Rule) *api.Server {
	t.Helper()
	return newServer(t, objs, rule, api.ModeLocal)
}

// NewInCluster is New in in-cluster mode: no document carries text the
// operator wrote.
func NewInCluster(t testing.TB, objs []*unstructured.Unstructured, rule Rule) *api.Server {
	t.Helper()
	return newServer(t, objs, rule, api.ModeInCluster)
}

func newServer(t testing.TB, objs []*unstructured.Unstructured, rule Rule, mode api.Mode) *api.Server {
	t.Helper()
	caller, _ := readmodeltest.NewChecker(t, Caller, rule, authz.Options{})
	readerChecker, _ := readmodeltest.NewChecker(t, reader, readmodeltest.AllowAll, authz.Options{})
	az := readmodeltest.ByIdentity{Caller.Username: caller, reader.Username: readerChecker}
	m, err := readmodel.New(readmodel.Config{
		Dynamic:     readmodeltest.Dynamic(objs...),
		Discovery:   readmodeltest.Discovery(readmodeltest.Kinds...),
		Authorizer:  az,
		Reader:      reader,
		SyncTimeout: 5 * time.Second,
		Now:         func() time.Time { return Now },
	})
	if err != nil {
		t.Fatalf("readmodel.New: %v", err)
	}
	if err := m.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(m.Stop)
	conn := api.Connection{Source: v1.SourceKubeconfig, Context: Context, ClusterEntry: Context}
	if mode == api.ModeInCluster {
		conn = api.Connection{Source: v1.SourceInCluster}
	}
	srv, err := api.New(api.Config{
		Mode:       mode,
		Model:      m,
		Authorizer: az,
		Reader:     reader,
		Connection: conn,
		Authenticate: func(r *http.Request) (api.Principal, error) {
			c, err := r.Cookie(SessionCookie)
			if err != nil || c.Value != SessionValue {
				return api.Principal{}, errors.New("no session")
			}
			return api.Principal{Identity: Caller, Session: SessionValue}, nil
		},
	})
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	t.Cleanup(srv.Close)
	return srv
}
