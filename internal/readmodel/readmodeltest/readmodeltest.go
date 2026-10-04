// Package readmodeltest serves committed cluster captures through fake
// clients, so tests can build read-model views from real operator output.
// It does not import readmodel, so readmodel's own tests use it too.
package readmodeltest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

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

// F1Files are the files of a capture directory, in load order.
var F1Files = []string{"platforms.yaml", "moduleinstances.yaml", "modulepackages.yaml",
	"transformerregistrations.yaml", "objects.yaml", "events.yaml"}

// Kind describes one kind the fake cluster serves.
type Kind struct {
	GVK        schema.GroupVersionKind
	Resource   string
	Namespaced bool
}

// GVR returns the kind's resource.
func (k Kind) GVR() schema.GroupVersionResource {
	return k.GVK.GroupVersion().WithResource(k.Resource)
}

const opmGroup = "opmodel.dev"

// Kinds are the kinds of the F1 capture plus the ones tests add.
var Kinds = []Kind{
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

// LoadList decodes a List file the way client-go decodes objects (integers
// as int64), which kstatus requires.
func LoadList(t testing.TB, path string) []*unstructured.Unstructured {
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

// LoadCapture returns every object of the capture in dir: the four OPM
// kinds, the inventory objects with their ReplicaSets and Pods, and the
// events. An object listed in two files (the TransformerRegistration a
// provider renders is both an OPM kind and an inventory object) is
// returned once.
func LoadCapture(t testing.TB, dir string) []*unstructured.Unstructured {
	t.Helper()
	var out []*unstructured.Unstructured
	seen := map[string]bool{}
	for _, f := range F1Files {
		for _, o := range LoadList(t, filepath.Join(dir, f)) {
			key := Key(o)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, o)
		}
	}
	return out
}

// Key identifies an object by kind, namespace and name.
func Key(o *unstructured.Unstructured) string {
	return o.GroupVersionKind().String() + "|" + o.GetNamespace() + "/" + o.GetName()
}

// Find returns the object with the given kind and name.
func Find(t testing.TB, objs []*unstructured.Unstructured, kind, name string) *unstructured.Unstructured {
	t.Helper()
	for _, o := range objs {
		if o.GetKind() == kind && o.GetName() == name {
			return o
		}
	}
	t.Fatalf("no %s %s", kind, name)
	return nil
}

// Dynamic returns a fake dynamic client serving objs.
func Dynamic(objs ...*unstructured.Unstructured) *dynfake.FakeDynamicClient {
	listKinds := map[schema.GroupVersionResource]string{}
	for _, k := range Kinds {
		listKinds[k.GVR()] = k.GVK.Kind + "List"
	}
	runtimeObjs := make([]runtime.Object, 0, len(objs))
	for _, o := range objs {
		runtimeObjs = append(runtimeObjs, o.DeepCopy())
	}
	return dynfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, runtimeObjs...)
}

// Discovery returns a fake discovery client that knows kinds.
func Discovery(kinds ...Kind) *fakediscovery.FakeDiscovery {
	d := &fakediscovery.FakeDiscovery{Fake: &k8stesting.Fake{}}
	SetDiscovery(d, kinds...)
	return d
}

// SetDiscovery replaces what d knows with kinds.
func SetDiscovery(d *fakediscovery.FakeDiscovery, kinds ...Kind) {
	byGV := map[string]*metav1.APIResourceList{}
	var order []string
	for _, k := range kinds {
		gv := k.GVK.GroupVersion().String()
		if byGV[gv] == nil {
			byGV[gv] = &metav1.APIResourceList{GroupVersion: gv}
			order = append(order, gv)
		}
		byGV[gv].APIResources = append(byGV[gv].APIResources, metav1.APIResource{
			Name: k.Resource, Kind: k.GVK.Kind, Namespaced: k.Namespaced,
			Verbs: metav1.Verbs{"get", "list", "watch"},
		})
	}
	d.Resources = nil
	for _, gv := range order {
		d.Resources = append(d.Resources, byGV[gv])
	}
}

// Rule answers one access review: true allows it.
type Rule func(who string, ra authorizationv1.ResourceAttributes) bool

// AllowAll allows every review.
func AllowAll(string, authorizationv1.ResourceAttributes) bool { return true }

// DenyResources returns a rule that denies every read of the named
// resources and allows the rest.
func DenyResources(resources ...string) Rule {
	return func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return !slices.Contains(resources, ra.Resource)
	}
}

// Reviews is a fake cluster answering SelfSubjectAccessReviews by a rule,
// recording what it is asked.
type Reviews struct {
	mu    sync.Mutex
	rule  Rule
	fail  bool // every review fails, as an unreachable API server would
	asked []authorizationv1.ResourceAttributes
}

// SetFail makes every later review fail, or work again.
func (r *Reviews) SetFail(fail bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fail = fail
}

// Count returns how many reviews were asked.
func (r *Reviews) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.asked)
}

// Asked returns a copy of every review asked, in order.
func (r *Reviews) Asked() []authorizationv1.ResourceAttributes {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.asked)
}

// NewChecker returns a local Checker for self whose reviews follow fn.
func NewChecker(t testing.TB, self authz.Identity, fn Rule, opts authz.Options) (*authz.Checker, *Reviews) {
	t.Helper()
	r := &Reviews{rule: fn}
	cs := fake.NewClientset()
	cs.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		create, ok := action.(k8stesting.CreateAction)
		if !ok {
			return true, nil, errors.New("not a create action")
		}
		review, ok := create.GetObject().(*authorizationv1.SelfSubjectAccessReview)
		if !ok {
			return true, nil, errors.New("not a SelfSubjectAccessReview")
		}
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

// ByIdentity routes each check to the Checker serving that identity, so
// every grant a test uses is issued by a real Checker.
type ByIdentity map[string]*authz.Checker

// Check implements authz.Authorizer.
func (b ByIdentity) Check(ctx context.Context, who authz.Identity, req authz.Attributes) (authz.Grant, error) {
	c, ok := b[who.Username]
	if !ok {
		var none authz.Grant
		return none, &authz.DenialError{Code: authz.CodeUnauthenticated, Attributes: req}
	}
	return c.Check(ctx, who, req)
}
