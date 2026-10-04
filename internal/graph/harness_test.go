package graph

import (
	"errors"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

// f1Dir is the committed capture of the e2e fixture cluster.
const f1Dir = "../../testdata/clusters/f1"

// The image-break samples from enhancement 0030 experiment 01: podinfo
// applied with an image tag that does not exist, one minute in. F1 holds
// no broken rollout.
const (
	brokenInstanceSample = "../health/testdata/mi-podinfo-image-broken.yaml"
	brokenObjectsSample  = "../health/testdata/objects-podinfo-phase4-broken-1min.yaml"
)

var (
	alice  = authz.Identity{Username: "alice", Groups: []string{"system:authenticated"}}
	reader = authz.Identity{Username: "portal-reader", Groups: []string{"system:authenticated"}}

	moduleInstances = schema.GroupVersionResource{Group: opmGroup, Version: "v1alpha1", Resource: "moduleinstances"}
	modulePackages  = schema.GroupVersionResource{Group: opmGroup, Version: "v1alpha1", Resource: "modulepackages"}
	platforms       = schema.GroupVersionResource{Group: opmGroup, Version: "v1alpha1", Resource: "platforms"}
)

// cluster is a started read model over a fake cluster. The reader may read
// everything; the caller, alice, follows a rule.
type cluster struct {
	m      *readmodel.Model
	caller *authz.Checker
}

func newCluster(t testing.TB, objs []*unstructured.Unstructured, callerRule readmodeltest.Rule) *cluster {
	t.Helper()
	caller, _ := readmodeltest.NewChecker(t, alice, callerRule, authz.Options{})
	readerChecker, _ := readmodeltest.NewChecker(t, reader, readmodeltest.AllowAll, authz.Options{})
	m, err := readmodel.New(readmodel.Config{
		Dynamic:     readmodeltest.Dynamic(objs...),
		Discovery:   readmodeltest.Discovery(readmodeltest.Kinds...),
		Authorizer:  readmodeltest.ByIdentity{alice.Username: caller, reader.Username: readerChecker},
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
	return &cluster{m: m, caller: caller}
}

func f1(t testing.TB) []*unstructured.Unstructured {
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
		drop[o.GetKind()+"|"+o.GetNamespace()+"/"+o.GetName()] = true
	}
	var out []*unstructured.Unstructured
	for _, o := range f1(t) {
		if drop[o.GetKind()+"|"+o.GetNamespace()+"/"+o.GetName()] {
			continue
		}
		// The F1 ReplicaSet and Pods of the healthy podinfo go too: the
		// samples carry their own.
		if o.GetNamespace() == "default" && (o.GetKind() == "ReplicaSet" || o.GetKind() == "Pod") &&
			o.GetLabels()["module-instance.opmodel.dev/name"] == "podinfo" {
			continue
		}
		out = append(out, o)
	}
	return append(out, replace...)
}

// get asks the caller's checker for a get grant.
func (c *cluster) get(t testing.TB, r schema.GroupVersionResource, ns, name string) (authz.Grant, error) {
	t.Helper()
	return c.caller.Check(t.Context(), alice, authz.Attributes{Verb: "get", Resource: r, Namespace: ns, Name: name})
}

func (c *cluster) instance(t testing.TB, ns, name string) readmodel.InstanceDetail {
	t.Helper()
	g, err := c.get(t, moduleInstances, ns, name)
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	d, err := c.m.Instance(t.Context(), alice, g, ns, name)
	if err != nil {
		t.Fatalf("Instance(%s/%s): %v", ns, name, err)
	}
	return d
}

func (c *cluster) pkg(t testing.TB, ns, name string) readmodel.PackageDetail {
	t.Helper()
	g, err := c.get(t, modulePackages, ns, name)
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	d, err := c.m.Package(t.Context(), alice, g, ns, name)
	if err != nil {
		t.Fatalf("Package(%s/%s): %v", ns, name, err)
	}
	return d
}

// platform reads the platform view and every provider its registrations
// name, the way the read API will: one grant per provider, a denial
// recorded as forbidden, a missing instance as read and absent.
func (c *cluster) platform(t testing.TB) PlatformInput {
	t.Helper()
	g, err := c.get(t, platforms, "", "cluster")
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	view, err := c.m.Platform(t.Context(), alice, g)
	if err != nil {
		t.Fatalf("Platform: %v", err)
	}
	in := PlatformInput{Platform: view}
	seen := map[readmodel.ObjectRef]bool{}
	for i := range view.Registrations {
		r := &view.Registrations[i]
		if r.Provider.Name == "" || seen[r.Provider] {
			continue
		}
		seen[r.Provider] = true
		l := ProviderLookup{Ref: r.Provider, Access: health.AccessOK}
		g, err := c.get(t, moduleInstances, r.Provider.Namespace, r.Provider.Name)
		if err != nil {
			l.Access = health.AccessForbidden
			in.Providers = append(in.Providers, l)
			continue
		}
		d, err := c.m.Instance(t.Context(), alice, g, r.Provider.Namespace, r.Provider.Name)
		switch {
		case errors.Is(err, readmodel.ErrNotFound):
		case err != nil:
			l.Access = health.AccessNotReadable
		default:
			l.Instance = &d
		}
		in.Providers = append(in.Providers, l)
	}
	return in
}

func nodeByID(t testing.TB, g Graph, id string) Node {
	t.Helper()
	for i := range g.Nodes {
		if g.Nodes[i].ID == id {
			return g.Nodes[i]
		}
	}
	t.Fatalf("no node %s", id)
	return Node{}
}

func hasNode(g Graph, id string) bool {
	for i := range g.Nodes {
		if g.Nodes[i].ID == id {
			return true
		}
	}
	return false
}

func edgeBetween(g Graph, kind EdgeKind, from, to string) (Edge, bool) {
	for i := range g.Edges {
		if e := g.Edges[i]; e.Kind == kind && e.From == from && e.To == to {
			return e, true
		}
	}
	return Edge{}, false
}

func countKind(g Graph, kind NodeKind) int {
	n := 0
	for i := range g.Nodes {
		if g.Nodes[i].Kind == kind {
			n++
		}
	}
	return n
}
