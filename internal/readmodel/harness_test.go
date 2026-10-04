package readmodel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// f1Dir is the committed capture of the e2e fixture cluster.
const f1Dir = "../../testdata/clusters/f1"

// applyFailedSample is the ApplyFailed instance from enhancement 0030
// experiment 01. F1 holds no failed apply, so the health package's copy of
// the live sample is reused.
const applyFailedSample = "../health/testdata/mi-apply-failed.yaml"

// alice is the kubeconfig's identity in these tests: the reader and, unless
// a test says otherwise, the caller.
var alice = authz.Identity{Username: "alice", Groups: []string{"dev", "system:authenticated"}}

// testKind describes one kind the fake cluster serves.
type testKind struct {
	gvk        schema.GroupVersionKind
	resource   string
	namespaced bool
}

func (k testKind) gvr() schema.GroupVersionResource {
	return k.gvk.GroupVersion().WithResource(k.resource)
}

// clusterKinds are the kinds of the F1 capture plus the ones tests add.
var clusterKinds = []testKind{
	{schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}, "namespaces", false},
	{schema.GroupVersionKind{Version: "v1", Kind: "ServiceAccount"}, "serviceaccounts", true},
	{schema.GroupVersionKind{Version: "v1", Kind: "Service"}, "services", true},
	{schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, "configmaps", true},
	{schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, "secrets", true},
	{schema.GroupVersionKind{Version: "v1", Kind: "Pod"}, "pods", true},
	{schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, "deployments", true},
	{schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "ReplicaSet"}, "replicasets", true},
	{schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}, "jobs", true},
	{schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"}, "customresourcedefinitions", false},
	{schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "ValidatingWebhookConfiguration"}, "validatingwebhookconfigurations", false},
	{schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "MutatingWebhookConfiguration"}, "mutatingwebhookconfigurations", false},
	{schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}, "clusterroles", false},
	{schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBinding"}, "clusterrolebindings", false},
	{schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role"}, "roles", true},
	{schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "RoleBinding"}, "rolebindings", true},
	{schema.GroupVersionKind{Group: "events.k8s.io", Version: "v1", Kind: "Event"}, "events", true},
	{schema.GroupVersionKind{Group: opmGroup, Version: "v1alpha1", Kind: "ModuleInstance"}, "moduleinstances", true},
	{schema.GroupVersionKind{Group: opmGroup, Version: "v1alpha1", Kind: "ModulePackage"}, "modulepackages", true},
	{schema.GroupVersionKind{Group: opmGroup, Version: "v1alpha1", Kind: "Platform"}, "platforms", false},
	{schema.GroupVersionKind{Group: opmGroup, Version: "v1alpha1", Kind: "TransformerRegistration"}, "transformerregistrations", false},
}

// loadList decodes a List file the way client-go decodes objects (integers
// as int64), which kstatus requires.
func loadList(t testing.TB, path string) []*unstructured.Unstructured {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	js, err := yaml.YAMLToJSON(raw)
	if err != nil {
		t.Fatalf("converting %s to JSON: %v", path, err)
	}
	var list unstructured.UnstructuredList
	if err := list.UnmarshalJSON(js); err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	out := make([]*unstructured.Unstructured, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, &list.Items[i])
	}
	return out
}

// loadF1 returns every object of the F1 capture: the four OPM kinds, the
// inventory objects with their ReplicaSets and Pods, and the events. The
// TransformerRegistration the backup provider renders appears in both
// transformerregistrations.yaml and objects.yaml, and is returned once.
func loadF1(t testing.TB) []*unstructured.Unstructured {
	t.Helper()
	var out []*unstructured.Unstructured
	seen := map[string]bool{}
	for _, f := range []string{"platforms.yaml", "moduleinstances.yaml", "modulepackages.yaml",
		"transformerregistrations.yaml", "objects.yaml", "events.yaml"} {
		for _, o := range loadList(t, filepath.Join(f1Dir, f)) {
			key := o.GroupVersionKind().String() + "|" + o.GetNamespace() + "/" + o.GetName()
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, o)
		}
	}
	return out
}

// find returns the object with the given kind and name.
func find(t testing.TB, objs []*unstructured.Unstructured, kind, name string) *unstructured.Unstructured {
	t.Helper()
	for _, o := range objs {
		if o.GetKind() == kind && o.GetName() == name {
			return o
		}
	}
	t.Fatalf("no %s %s", kind, name)
	return nil
}

// newDynamic returns a fake dynamic client serving objs.
func newDynamic(objs ...*unstructured.Unstructured) *dynfake.FakeDynamicClient {
	listKinds := map[schema.GroupVersionResource]string{}
	for _, k := range clusterKinds {
		listKinds[k.gvr()] = k.gvk.Kind + "List"
	}
	runtimeObjs := make([]runtime.Object, 0, len(objs))
	for _, o := range objs {
		runtimeObjs = append(runtimeObjs, o.DeepCopy())
	}
	return dynfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, runtimeObjs...)
}

// newDiscovery returns a fake discovery client that knows kinds.
func newDiscovery(kinds ...testKind) *fakediscovery.FakeDiscovery {
	d := &fakediscovery.FakeDiscovery{Fake: &k8stesting.Fake{}}
	setDiscovery(d, kinds...)
	return d
}

func setDiscovery(d *fakediscovery.FakeDiscovery, kinds ...testKind) {
	byGV := map[string]*metav1.APIResourceList{}
	var order []string
	for _, k := range kinds {
		gv := k.gvk.GroupVersion().String()
		if byGV[gv] == nil {
			byGV[gv] = &metav1.APIResourceList{GroupVersion: gv}
			order = append(order, gv)
		}
		byGV[gv].APIResources = append(byGV[gv].APIResources, metav1.APIResource{
			Name: k.resource, Kind: k.gvk.Kind, Namespaced: k.namespaced,
			Verbs: metav1.Verbs{"get", "list", "watch"},
		})
	}
	d.Resources = nil
	for _, gv := range order {
		d.Resources = append(d.Resources, byGV[gv])
	}
}

// rule answers one access review: true allows it.
type rule func(who string, ra authorizationv1.ResourceAttributes) bool

func allowAll(string, authorizationv1.ResourceAttributes) bool { return true }

// reviews is a fake cluster answering SelfSubjectAccessReviews by a rule,
// counting what it is asked.
type reviews struct {
	mu    sync.Mutex
	rule  rule
	fail  bool // every review fails, as an unreachable API server would
	asked []authorizationv1.ResourceAttributes
}

func (r *reviews) setFail(fail bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fail = fail
}

// newChecker returns a local Checker for self whose reviews follow fn.
func newChecker(t testing.TB, self authz.Identity, fn rule, opts authz.Options) (*authz.Checker, *reviews) {
	t.Helper()
	r := &reviews{rule: fn}
	cs := fake.NewClientset()
	cs.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		review := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		ra := *review.Spec.ResourceAttributes
		r.mu.Lock()
		r.asked = append(r.asked, ra)
		allowed, fail := r.rule(self.Username, ra), r.fail
		r.mu.Unlock()
		if fail {
			return true, nil, errors.New("connection refused")
		}
		out := review.DeepCopy()
		out.Status.Allowed = allowed
		out.Status.Denied = !allowed
		return true, out, nil
	})
	c, err := authz.NewLocal(cs.AuthorizationV1().SelfSubjectAccessReviews(), self, opts)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	return c, r
}

// reader is the identity the fake cluster's informers read as. Tests keep
// it apart from the caller so each side's permissions can differ; in local
// mode both are the kubeconfig's identity.
var reader = authz.Identity{Username: "portal-reader", Groups: []string{"system:authenticated"}}

// byIdentity routes each check to the local Checker serving that identity,
// so every grant a test uses is issued by a real Checker.
type byIdentity map[string]*authz.Checker

func (b byIdentity) Check(ctx context.Context, who authz.Identity, req authz.Attributes) (authz.Grant, error) {
	c, ok := b[who.Username]
	if !ok {
		var none authz.Grant
		return none, &authz.DenialError{Code: authz.CodeUnauthenticated, Attributes: req}
	}
	return c.Check(ctx, who, req)
}

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

// denyResources returns a rule that denies every read of the named
// resources and allows the rest.
func denyResources(resources ...string) rule {
	return func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return !slices.Contains(resources, ra.Resource)
	}
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

// count returns how many reviews were asked.
func (r *reviews) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.asked)
}
