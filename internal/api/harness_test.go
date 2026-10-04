package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dynfake "k8s.io/client-go/dynamic/fake"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

// f1Dir is the committed capture of the e2e fixture cluster.
const f1Dir = "../../testdata/clusters/f1"

// The image-break samples from enhancement 0030 experiment 01: podinfo
// applied with an image tag that does not exist. F1 holds no broken rollout.
const (
	brokenInstanceSample = "../health/testdata/mi-podinfo-image-broken.yaml"
	brokenObjectsSample  = "../health/testdata/objects-podinfo-phase4-broken-1min.yaml"
)

var (
	// alice is the caller in these tests.
	alice = authz.Identity{Username: "alice", Groups: []string{"system:authenticated"}}
	// reader is the identity the fake cluster's informers read as.
	reader = authz.Identity{Username: "portal-reader", Groups: []string{"system:authenticated"}}
	// now is the read model's clock, so evaluation times are stable.
	now = time.Date(2026, 10, 4, 18, 10, 0, 0, time.UTC)
)

// recorder is the fake authorizer: it passes each check to the
// SSAR-backed checkers on the fake cluster, so every grant is real, records
// what it was asked, and can fail checks as an unreachable API server would.
type recorder struct {
	inner authz.Authorizer

	mu    sync.Mutex
	asked []authz.Attributes
	fail  func(authz.Identity, authz.Attributes) bool
}

func (r *recorder) Check(ctx context.Context, who authz.Identity, req authz.Attributes) (authz.Grant, error) {
	r.mu.Lock()
	if who.Username == alice.Username {
		r.asked = append(r.asked, req)
	}
	fail := r.fail
	r.mu.Unlock()
	if fail != nil && fail(who, req) {
		var none authz.Grant
		return none, &authz.DenialError{Code: authz.CodeUnavailable, Attributes: req}
	}
	return r.inner.Check(ctx, who, req)
}

// callerChecks returns how many checks were asked for alice.
func (r *recorder) callerChecks() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.asked)
}

func (r *recorder) callerAsked() []authz.Attributes {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.asked)
}

func (r *recorder) setFail(fn func(authz.Identity, authz.Attributes) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fail = fn
}

// env is a Server over a started read model on a fake cluster.
type env struct {
	srv    *Server
	model  *readmodel.Model
	client *dynfake.FakeDynamicClient
	az     *recorder
	// principal is what Authenticate returns.
	principal Principal
}

// newEnv serves objs. alice's reviews follow callerRule; the reader may
// read everything.
func newEnv(t testing.TB, objs []*unstructured.Unstructured, callerRule readmodeltest.Rule, opts ...func(*Config)) *env {
	t.Helper()
	return newEnvWithReader(t, objs, callerRule, readmodeltest.AllowAll, opts...)
}

// newEnvWithReader is newEnv with the reader's reviews following
// readerRule.
func newEnvWithReader(t testing.TB, objs []*unstructured.Unstructured, callerRule, readerRule readmodeltest.Rule, opts ...func(*Config)) *env {
	t.Helper()
	caller, _ := readmodeltest.NewChecker(t, alice, callerRule, authz.Options{})
	readerChecker, _ := readmodeltest.NewChecker(t, reader, readerRule, authz.Options{})
	az := &recorder{inner: readmodeltest.ByIdentity{alice.Username: caller, reader.Username: readerChecker}}
	client := readmodeltest.Dynamic(objs...)
	m, err := readmodel.New(readmodel.Config{
		Dynamic:     client,
		Discovery:   readmodeltest.Discovery(readmodeltest.Kinds...),
		Authorizer:  az,
		Reader:      reader,
		SyncTimeout: 5 * time.Second,
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("readmodel.New: %v", err)
	}
	if err := m.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(m.Stop)
	e := &env{model: m, client: client, az: az, principal: Principal{Identity: alice, Session: "session-alice"}}
	cfg := Config{
		Model:      m,
		Authorizer: az,
		Reader:     reader,
		Authenticate: func(*http.Request) (Principal, error) {
			if e.principal.Session == "" && e.principal.Identity.Username == "" {
				return Principal{}, errors.New("no session")
			}
			return e.principal, nil
		},
	}
	for _, o := range opts {
		o(&cfg)
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(srv.Close)
	e.srv = srv
	return e
}

// response is one request's outcome.
type response struct {
	status int
	header http.Header
	body   []byte
}

// get sends GET path to the server.
func (e *env) get(t testing.TB, path string) response {
	t.Helper()
	return e.do(t, http.MethodGet, path)
}

func (e *env) do(t testing.TB, method, path string) response {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response{status: res.StatusCode, header: res.Header, body: body}
}

func loadF1(t testing.TB) []*unstructured.Unstructured {
	t.Helper()
	return readmodeltest.LoadCapture(t, f1Dir)
}

// f1Broken is F1 with default/podinfo and its objects replaced by the
// experiment 01 image-break samples.
func f1Broken(t testing.TB) []*unstructured.Unstructured {
	t.Helper()
	replace := readmodeltest.LoadList(t, brokenInstanceSample)
	replace = append(replace, readmodeltest.LoadList(t, brokenObjectsSample)...)
	drop := map[string]bool{}
	for _, o := range replace {
		drop[readmodeltest.Key(o)] = true
	}
	var out []*unstructured.Unstructured
	for _, o := range loadF1(t) {
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

const base = Prefix + "/clusters/default"
