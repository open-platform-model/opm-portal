package readmodel

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	k8stesting "k8s.io/client-go/testing"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
)

// ownerRefs is the owner and every object and runtime child its view lists
// that the caller may read: the refs a handler passes to EventsAbout.
func ownerRefs(d InstanceDetail) []ObjectRef {
	refs := []ObjectRef{d.Ref}
	for ci := range d.Components {
		for oi := range d.Components[ci].Objects {
			o := &d.Components[ci].Objects[oi]
			if o.Access != health.AccessOK {
				continue
			}
			refs = append(refs, o.Ref)
			for chi := range o.Children {
				refs = append(refs, o.Children[chi].Ref)
			}
		}
	}
	return refs
}

// eventLists returns the events lists the fake cluster served as
// "namespace kind" lines, sorted.
func eventLists(e *env) []string {
	var out []string
	for _, a := range e.client.Actions() {
		l, ok := a.(k8stesting.ListAction)
		if !ok || a.GetResource().Resource != "events" {
			continue
		}
		kind, _ := l.GetListRestrictions().Fields.RequiresExactMatch("regarding.kind")
		out = append(out, a.GetNamespace()+" "+kind)
	}
	sort.Strings(out)
	return out
}

func (e *env) ownerRefs(t *testing.T, namespace, name string) []ObjectRef {
	t.Helper()
	d, err := e.m.Instance(t.Context(), alice, e.grant(t, "get", moduleInstances, namespace, name), namespace, name)
	if err != nil {
		t.Fatal(err)
	}
	return ownerRefs(d)
}

// eventsAboutIn calls EventsAbout with the grant of the owner's event namespace
// taken out of grants; the others stay. An owner namespace with no grant in
// grants gets none, which the model refuses.
func eventsAboutIn(t *testing.T, m *Model, grants map[string]authz.Grant, refs []ObjectRef) ([]Event, int, error) {
	t.Helper()
	owner := EventNamespace(refs[0])
	others := map[string]authz.Grant{}
	for ns, g := range grants {
		if ns != owner {
			others[ns] = g
		}
	}
	return m.EventsAbout(t.Context(), alice, grants[owner], others, refs)
}

func (e *env) eventGrants(t *testing.T, namespaces ...string) map[string]authz.Grant {
	t.Helper()
	grants := map[string]authz.Grant{}
	for _, ns := range namespaces {
		grants[ns] = e.grant(t, "list", events, ns, "")
	}
	return grants
}

// TestEventsAboutPodinfo: the owner's refs on F1 return the instance's
// events, the Deployment's ScalingReplicaSet and the ReplicaSet's and Pods'
// events, newest first, from one list per regarded kind and none per object.
func TestEventsAboutPodinfo(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	refs := e.ownerRefs(t, "default", "podinfo")
	evs, skipped, err := eventsAboutIn(t, e.m, e.eventGrants(t, "default"), refs)
	if err != nil || skipped != 0 {
		t.Fatalf("EventsAbout = %d lines, skipped %d, %v", len(evs), skipped, err)
	}
	kinds := map[string]int{}
	var scaling bool
	for i, ev := range evs {
		kinds[ev.Regarding.Kind]++
		if ev.Regarding.Kind == "Deployment" && ev.Reason == "ScalingReplicaSet" {
			scaling = true
			if ev.Count != 1 {
				t.Errorf("ScalingReplicaSet count = %d, want the one event the capture holds", ev.Count)
			}
		}
		if i > 0 && ev.LastSeen.After(evs[i-1].LastSeen) {
			t.Errorf("line %d (%s %s) is newer than the line before it", i, ev.Regarding.Kind, ev.Reason)
		}
	}
	for _, kind := range []string{"ModuleInstance", "Deployment", "ReplicaSet", "Pod"} {
		if kinds[kind] == 0 {
			t.Errorf("no event about a %s in %v", kind, kinds)
		}
	}
	if !scaling {
		t.Error("the Deployment's ScalingReplicaSet event is missing")
	}
	want := []string{"default Deployment", "default ModuleInstance", "default Pod", "default ReplicaSet", "default Service"}
	if got := eventLists(e); !slices.Equal(got, want) {
		t.Errorf("event lists = %v, want one per kind %v", got, want)
	}
}

// TestEventsAboutKeepsToItsRefs: a Pod the caller is not given is not in the
// result, even though its events share the list of its kind.
func TestEventsAboutKeepsToItsRefs(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	refs := e.ownerRefs(t, "default", "podinfo")
	var kept []ObjectRef
	var dropped ObjectRef
	for _, r := range refs {
		if r.Kind == "Pod" && dropped.Name == "" {
			dropped = r
			continue
		}
		kept = append(kept, r)
	}
	evs, _, err := eventsAboutIn(t, e.m, e.eventGrants(t, "default"), kept)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range evs {
		if ev.Regarding.Name == dropped.Name {
			t.Errorf("event %s about %s/%s, which was not asked for", ev.Reason, ev.Regarding.Kind, ev.Regarding.Name)
		}
	}
	if len(evs) == 0 {
		t.Fatal("no events at all")
	}
}

// TestEventsAboutClusterScopedObjectsReadFromDefault: cert-manager's
// cluster-scoped objects are listed in namespace default, and the lists of
// its own namespace stay one per kind.
func TestEventsAboutClusterScopedObjectsReadFromDefault(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	refs := e.ownerRefs(t, "cert-manager", "cert-manager")
	clusterScoped := map[string]bool{}
	distinct := map[string]bool{}
	for _, r := range refs {
		distinct[EventNamespace(r)+" "+r.Kind] = true
		if r.Namespace == "" {
			clusterScoped[r.Kind] = true
		}
	}
	if len(clusterScoped) == 0 {
		t.Fatal("cert-manager holds no cluster-scoped object in F1")
	}
	if _, _, err := eventsAboutIn(t, e.m, e.eventGrants(t, "cert-manager", "default"), refs); err != nil {
		t.Fatal(err)
	}
	got := eventLists(e)
	for kind := range clusterScoped {
		if !slices.Contains(got, "default "+kind) {
			t.Errorf("no list of %s events in default: %v", kind, got)
		}
	}
	if len(got) != len(distinct) || len(got) >= len(refs) {
		t.Errorf("%d lists for %d distinct (namespace, kind) pairs and %d refs; want one per pair", len(got), len(distinct), len(refs))
	}
}

// TestEventsAboutSkipsANamespaceWithoutAGrant: refs whose event namespace has
// no grant are left out and counted, and no list runs there.
func TestEventsAboutSkipsANamespaceWithoutAGrant(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	refs := e.ownerRefs(t, "cert-manager", "cert-manager")
	inDefault := 0
	for _, r := range refs {
		if EventNamespace(r) == "default" {
			inDefault++
		}
	}
	evs, skipped, err := eventsAboutIn(t, e.m, e.eventGrants(t, "cert-manager"), refs)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != inDefault || skipped == 0 {
		t.Errorf("skipped = %d, want the %d refs whose events live in default", skipped, inDefault)
	}
	for _, l := range eventLists(e) {
		if strings.HasPrefix(l, "default ") {
			t.Errorf("a list ran in default without a grant: %s", l)
		}
	}
	if len(evs) == 0 {
		t.Error("the events of cert-manager's own namespace are missing")
	}
}

// TestEventsAboutNamespaceTheReaderMayNotList: the portal's reader denied in
// a namespace is a skip there, and ErrUnavailable in the owner's own.
func TestEventsAboutNamespaceTheReaderMayNotList(t *testing.T) {
	t.Run("owner namespace", func(t *testing.T) {
		e := newEnv(t, loadF1(t), allowAll, allowAll)
		refs := e.ownerRefs(t, "default", "podinfo")
		denied := newEnv(t, loadF1(t), allowAll, denyResources("events"))
		if _, _, err := eventsAboutIn(t, denied.m, denied.eventGrants(t, "default"), refs); !errors.Is(err, ErrUnavailable) {
			t.Errorf("err = %v, want ErrUnavailable", err)
		}
		if lists := eventLists(denied); len(lists) != 0 {
			t.Errorf("lists ran with the reader denied: %v", lists)
		}
	})
	t.Run("another namespace", func(t *testing.T) {
		noDefault := func(_ string, ra authorizationv1.ResourceAttributes) bool {
			return ra.Resource != "events" || ra.Namespace != "default"
		}
		e := newEnv(t, loadF1(t), allowAll, noDefault)
		refs := e.ownerRefs(t, "cert-manager", "cert-manager")
		evs, skipped, err := eventsAboutIn(t, e.m, e.eventGrants(t, "cert-manager", "default"), refs)
		if err != nil || skipped == 0 || len(evs) == 0 {
			t.Errorf("EventsAbout = %d lines, skipped %d, %v; want events from cert-manager and a skip count", len(evs), skipped, err)
		}
		for _, l := range eventLists(e) {
			if strings.HasPrefix(l, "default ") {
				t.Errorf("a list ran in default with the reader denied: %s", l)
			}
		}
	})
}

// TestEventsAboutAListFails: a failing list in another namespace leaves its
// refs out and counted; one in the owner's namespace is ErrUnavailable.
func TestEventsAboutAListFails(t *testing.T) {
	fail := func(namespace string) func(k8stesting.Action) (bool, runtime.Object, error) {
		return func(a k8stesting.Action) (bool, runtime.Object, error) {
			if a.GetNamespace() == namespace {
				return true, nil, errors.New("boom")
			}
			return false, nil, nil
		}
	}
	t.Run("other namespace", func(t *testing.T) {
		e := newEnv(t, loadF1(t), allowAll, allowAll)
		refs := e.ownerRefs(t, "cert-manager", "cert-manager")
		e.client.PrependReactor("list", "events", fail("default"))
		evs, skipped, err := eventsAboutIn(t, e.m, e.eventGrants(t, "cert-manager", "default"), refs)
		if err != nil || skipped == 0 || len(evs) == 0 {
			t.Errorf("EventsAbout = %d lines, skipped %d, %v; want events from cert-manager and a skip count", len(evs), skipped, err)
		}
	})
	t.Run("owner namespace", func(t *testing.T) {
		e := newEnv(t, loadF1(t), allowAll, allowAll)
		refs := e.ownerRefs(t, "cert-manager", "cert-manager")
		e.client.PrependReactor("list", "events", fail("cert-manager"))
		if _, _, err := eventsAboutIn(t, e.m, e.eventGrants(t, "cert-manager", "default"), refs); !errors.Is(err, ErrUnavailable) {
			t.Errorf("err = %v, want ErrUnavailable", err)
		}
	})
}

// TestEventsAboutRunsAtMostFourListsAtOnce: the lists overlap, and never more
// than four are in flight.
func TestEventsAboutRunsAtMostFourListsAtOnce(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	refs := e.ownerRefs(t, "cert-manager", "cert-manager")
	probe := &inFlight{Interface: e.m.cfg.Dynamic}
	e.m.cfg.Dynamic = probe
	if _, _, err := eventsAboutIn(t, e.m, e.eventGrants(t, "cert-manager", "default"), refs); err != nil {
		t.Fatal(err)
	}
	if peak := probe.peak.Load(); peak < 2 || peak > 4 {
		t.Errorf("peak lists in flight = %d, want between 2 and 4", peak)
	}
}

// inFlight counts the events lists running at once and holds each for a
// moment so they overlap. Reactors of the fake client run under its lock,
// so the delay sits here instead.
type inFlight struct {
	dynamic.Interface
	now, peak atomic.Int64
}

func (f *inFlight) Resource(r schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	if r != events {
		return f.Interface.Resource(r)
	}
	return flightResource{f.Interface.Resource(r), f}
}

type flightResource struct {
	dynamic.NamespaceableResourceInterface
	f *inFlight
}

func (r flightResource) Namespace(ns string) dynamic.ResourceInterface {
	return flightNamespace{r.NamespaceableResourceInterface.Namespace(ns), r.f}
}

type flightNamespace struct {
	dynamic.ResourceInterface
	f *inFlight
}

func (n flightNamespace) List(ctx context.Context, o metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	now := n.f.now.Add(1)
	defer n.f.now.Add(-1)
	for {
		peak := n.f.peak.Load()
		if now <= peak || n.f.peak.CompareAndSwap(peak, now) {
			break
		}
	}
	time.Sleep(20 * time.Millisecond)
	return n.ResourceInterface.List(ctx, o)
}
