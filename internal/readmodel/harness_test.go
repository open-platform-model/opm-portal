package readmodel

import (
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

// f1Dir is the committed capture of the e2e fixture cluster.
const f1Dir = "../../testdata/clusters/f1"

// applyFailedSample is the ApplyFailed instance from design evidence 01.
// F1 holds no failed apply, so the health package's copy of
// the live sample is reused.
const applyFailedSample = "../health/testdata/mi-apply-failed.yaml"

// alice is the kubeconfig's identity in these tests: the reader and, unless
// a test says otherwise, the caller.
var alice = authz.Identity{Username: "alice", Groups: []string{"dev", "system:authenticated"}}

// The cluster-side harness lives in readmodeltest; these names keep the
// tests short.
type (
	testKind = readmodeltest.Kind
	rule     = readmodeltest.Rule
	reviews  = readmodeltest.Reviews
)

var (
	clusterKinds  = readmodeltest.Kinds
	allowAll      = readmodeltest.AllowAll
	denyResources = readmodeltest.DenyResources
	newDynamic    = readmodeltest.Dynamic
	newDiscovery  = readmodeltest.Discovery
	setDiscovery  = readmodeltest.SetDiscovery
	newChecker    = readmodeltest.NewChecker
	loadList      = readmodeltest.LoadList
	find          = readmodeltest.Find
)

// loadF1 returns every object of the F1 capture.
func loadF1(t testing.TB) []*unstructured.Unstructured {
	t.Helper()
	return readmodeltest.LoadCapture(t, f1Dir)
}

// reader is the identity the fake cluster's informers read as. Tests keep
// it apart from the caller so each side's permissions can differ; in local
// mode both are the kubeconfig's identity.
var reader = authz.Identity{Username: "portal-reader", Groups: []string{"system:authenticated"}}

// byIdentity routes each check to the local Checker serving that identity.
type byIdentity = readmodeltest.ByIdentity

// env is a Model over a fake cluster, with the checkers behind it.
type env struct {
	m       *Model
	client  *dynfake.FakeDynamicClient
	caller  *authz.Checker
	reviews *reviews // the caller's
	readerR *reviews // the reader's
}

// newEnv starts a Model over objs. callerRule answers alice's reviews and
// readerRule the reader's; mutate adjusts the config before New.
func newEnv(t testing.TB, objs []*unstructured.Unstructured, callerRule, readerRule rule, mutate ...func(*Config)) *env {
	t.Helper()
	e := newUnstartedEnv(t, objs, callerRule, readerRule, mutate...)
	if err := e.m.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return e
}

// newUnstartedEnv is newEnv without Start, for tests that change the fake
// cluster before the Model starts.
func newUnstartedEnv(t testing.TB, objs []*unstructured.Unstructured, callerRule, readerRule rule, mutate ...func(*Config)) *env {
	t.Helper()
	caller, callerReviews := newChecker(t, alice, callerRule, authz.Options{})
	readerChecker, readerReviews := newChecker(t, reader, readerRule, authz.Options{})
	client := newDynamic(objs...)
	cfg := Config{
		Dynamic:     client,
		Discovery:   newDiscovery(clusterKinds...),
		Authorizer:  byIdentity{alice.Username: caller, reader.Username: readerChecker},
		Reader:      reader,
		SyncTimeout: 5 * time.Second,
	}
	for _, fn := range mutate {
		fn(&cfg)
	}
	m, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(m.Stop)
	return &env{m: m, client: client, caller: caller, reviews: callerReviews, readerR: readerReviews}
}

// grant asks alice's checker for a read and fails the test if it is denied.
func (e *env) grant(t testing.TB, verb string, resource schema.GroupVersionResource, namespace, name string) authz.Grant {
	t.Helper()
	g, err := e.caller.Check(t.Context(), alice, authz.Attributes{Verb: verb, Resource: resource, Namespace: namespace, Name: name})
	if err != nil {
		t.Fatalf("grant %s %s %s/%s: %v", verb, resource.Resource, namespace, name, err)
	}
	return g
}

// clusterReads returns the dynamic client's get, list and watch actions, as
// "verb resource namespace/name" lines.
func clusterReads(c *dynfake.FakeDynamicClient) []string {
	actions := c.Actions()
	out := make([]string, 0, len(actions))
	for _, a := range actions {
		switch a.GetVerb() {
		case "get", "list", "watch":
		default:
			continue
		}
		name := ""
		if g, ok := a.(k8stesting.GetAction); ok {
			name = g.GetName()
		}
		out = append(out, strings.Join([]string{a.GetVerb(), a.GetResource().Resource, a.GetNamespace() + "/" + name}, " "))
	}
	return out
}
