package readmodel

import (
	"strings"
	"sync"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/health"
)

// fakeClock is a settable clock for the idle janitor.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func (e *env) instance(t *testing.T, ns, name string) InstanceDetail {
	t.Helper()
	d, err := e.m.Instance(t.Context(), alice, e.grant(t, "get", moduleInstances, ns, name), ns, name)
	if err != nil {
		t.Fatalf("Instance(%s/%s) = %v", ns, name, err)
	}
	return d
}

// objectsOf flattens a detail's components.
func objectsOf(d *InstanceDetail) []InventoryObject {
	var out []InventoryObject
	for i := range d.Components {
		out = append(out, d.Components[i].Objects...)
	}
	return out
}

func objectNamed(t *testing.T, d *InstanceDetail, kind, name string) InventoryObject {
	t.Helper()
	objs := objectsOf(d)
	for i := range objs {
		if objs[i].Ref.Kind == kind && objs[i].Ref.Name == name {
			return objs[i]
		}
	}
	t.Fatalf("no %s %s in %s", kind, name, d.Ref.Name)
	return InventoryObject{}
}

func TestInstanceHealthFromHeldState(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	cases := []struct {
		ns, name   string
		objects    int
		components []string // first few, in inventory order
	}{
		{"cert-manager", "cert-manager", 42, []string{"namespace", "crds", "controller", "webhook", "cainjector"}},
		{"default", "podinfo", 2, nil},
		{"default", "backup-provider", 1, []string{"registration"}},
		{"web", "web", 2, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := e.instance(t, tc.ns, tc.name)
			objs := objectsOf(&d)
			if len(objs) != tc.objects {
				t.Fatalf("objects = %d, want %d", len(objs), tc.objects)
			}
			for _, o := range objs {
				if o.Access != health.AccessOK || o.Health.State != health.Healthy || !o.Live || o.EvaluatedAt.IsZero() || o.ChildrenUnread {
					t.Errorf("%s %s = %+v, want a live, healthy, readable object", o.Ref.Kind, o.Ref.Name, o)
				}
			}
			if d.Health.State != health.Healthy || d.Health.Partial || !d.Health.Live || d.Health.Counts.Healthy != tc.objects {
				t.Errorf("instance health = %+v", d.Health)
			}
			for i, name := range tc.components {
				if d.Components[i].Name != name {
					t.Errorf("component %d = %s, want %s", i, d.Components[i].Name, name)
				}
			}
		})
	}
}

// TestCLIOwnedInstanceHealth: the CLI-owned instance is managed externally
// on the applied axis, and its health is computed from its inventory
// (portal:D3:R6).
func TestCLIOwnedInstanceHealth(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	d := e.instance(t, "web", "web")
	if d.Owner != OwnerCLI || d.Applied.State != health.AppliedStateManagedExternally {
		t.Fatalf("owner %s applied %s", d.Owner, d.Applied.State)
	}
	if d.Health.State != health.Healthy || d.Health.Counts.Healthy != 2 {
		t.Fatalf("health = %+v, want two healthy objects", d.Health)
	}
}

// TestPodRuleReachesHeldInstances: a Pod waiting on a broken image degrades
// its Deployment through the children the read model hands to health,
// while the operator still says Applied.
func TestPodRuleReachesHeldInstances(t *testing.T) {
	objs := loadF1(t)
	pod := find(t, objs, "Pod", "podinfo-podinfo-d9585d794-4lg6h")
	statuses, _, err := unstructured.NestedSlice(pod.Object, "status", "containerStatuses")
	if err != nil || len(statuses) == 0 {
		t.Fatalf("pod has no container statuses: %v", err)
	}
	first := statuses[0].(map[string]any)
	first["ready"] = false
	first["state"] = map[string]any{"waiting": map[string]any{"reason": "ImagePullBackOff", "message": "Back-off pulling image"}}
	if err := unstructured.SetNestedSlice(pod.Object, statuses, "status", "containerStatuses"); err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, objs, allowAll, allowAll)
	d := e.instance(t, "default", "podinfo")
	dep := objectNamed(t, &d, "Deployment", "podinfo-podinfo")
	if dep.Health.State != health.Degraded || dep.Health.Reason != "ImagePullBackOff" {
		t.Fatalf("Deployment health = %+v, want Degraded ImagePullBackOff", dep.Health)
	}
	if d.Health.State != health.Degraded || d.Applied.State != health.AppliedStateApplied {
		t.Fatalf("instance health %s applied %s, want Degraded and Applied", d.Health.State, d.Applied.State)
	}
}

func TestInventoryEntriesTheCallerMayNotRead(t *testing.T) {
	e := newEnv(t, loadF1(t), denyResources("clusterroles"), allowAll)
	d := e.instance(t, "cert-manager", "cert-manager")
	forbidden := 0
	for _, o := range objectsOf(&d) {
		if o.Ref.Kind != "ClusterRole" {
			if o.Access != health.AccessOK {
				t.Errorf("%s %s access %s", o.Ref.Kind, o.Ref.Name, o.Access)
			}
			continue
		}
		forbidden++
		if o.Access != health.AccessForbidden || o.Health != (health.ObjectHealth{}) || !o.EvaluatedAt.IsZero() {
			t.Errorf("ClusterRole %s = %+v, want forbidden with no content", o.Ref.Name, o)
		}
	}
	if forbidden != 10 || !d.Health.Partial || d.Health.Counts.Forbidden != 10 {
		t.Fatalf("forbidden %d, health %+v; want 10 forbidden and a partial health", forbidden, d.Health)
	}
	for _, r := range clusterReads(e.client) {
		if strings.Contains(r, "clusterroles") && !strings.Contains(r, "clusterrolebindings") {
			t.Errorf("read ClusterRoles for a caller who may not: %s", r)
		}
	}
}

// TestExactNameGrantedByRBAC: a caller denied the whole namespace but
// granted one name reads that object.
func TestExactNameGrantedByRBAC(t *testing.T) {
	onlyOne := func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return ra.Resource != "deployments" || ra.Name == "podinfo-podinfo"
	}
	e := newEnv(t, loadF1(t), onlyOne, allowAll)
	d := e.instance(t, "default", "podinfo")
	if o := objectNamed(t, &d, "Deployment", "podinfo-podinfo"); o.Access != health.AccessOK || o.Health.State != health.Healthy {
		t.Fatalf("Deployment = %+v, want readable through its exact-name grant", o)
	}
}

func TestUnreadableChildrenMarkWorkloads(t *testing.T) {
	for name, deny := range map[string]struct{ caller, reader rule }{
		"caller may not list pods": {denyResources("pods"), allowAll},
		"reader may not list pods": {allowAll, denyResources("pods")},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, loadF1(t), deny.caller, deny.reader)
			d := e.instance(t, "default", "podinfo")
			if dep := objectNamed(t, &d, "Deployment", "podinfo-podinfo"); !dep.ChildrenUnread {
				t.Errorf("Deployment = %+v, want children unread", dep)
			}
			if svc := objectNamed(t, &d, "Service", "podinfo-podinfo"); svc.ChildrenUnread {
				t.Errorf("Service marked with unread children")
			}
			if !d.Health.Partial {
				t.Errorf("health %+v, want partial", d.Health)
			}
		})
	}
}

func TestUnresolvableKindIsNotReadable(t *testing.T) {
	var known []testKind
	for _, k := range clusterKinds {
		if k.GVK.Kind != "MutatingWebhookConfiguration" {
			known = append(known, k)
		}
	}
	e := newEnv(t, loadF1(t), allowAll, allowAll, func(c *Config) { c.Discovery = newDiscovery(known...) })
	d := e.instance(t, "cert-manager", "cert-manager")
	if o := objectNamed(t, &d, "MutatingWebhookConfiguration", "cert-manager-webhook"); o.Access != health.AccessNotReadable {
		t.Fatalf("unknown kind = %+v, want not readable", o)
	}
	if !d.Health.Partial || d.Health.Counts.NotReadable != 1 {
		t.Fatalf("health = %+v", d.Health)
	}
}

// TestSecondReadMakesNoClusterRequest: once the kinds are held, the detail
// is answered from memory (portal:D3:R9).
func TestSecondReadMakesNoClusterRequest(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	e.instance(t, "cert-manager", "cert-manager")
	before := clusterReads(e.client)
	reviews := e.reviews.Count() + e.readerR.Count()
	e.instance(t, "cert-manager", "cert-manager")
	if after := clusterReads(e.client); len(after) != len(before) {
		t.Fatalf("second read made cluster requests: %v", after[len(before):])
	}
	if n := e.reviews.Count() + e.readerR.Count(); n != reviews {
		t.Errorf("second read sent %d access reviews; decisions should be cached", n-reviews)
	}
}

func TestSecretsAreWithheldAndNeverRead(t *testing.T) {
	objs := loadF1(t)
	podinfo := find(t, objs, "ModuleInstance", "podinfo")
	entries, _, err := unstructured.NestedSlice(podinfo.Object, "status", "inventory", "entries")
	if err != nil {
		t.Fatal(err)
	}
	entries = append(entries, map[string]any{"component": "app", "kind": "Secret", "v": "v1", "namespace": "default", "name": "db-password"})
	if err := unstructured.SetNestedSlice(podinfo.Object, entries, "status", "inventory", "entries"); err != nil {
		t.Fatal(err)
	}
	secret := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]any{"name": "db-password", "namespace": "default",
			"labels": map[string]any{instanceUUIDLabel: "e6d8d1d8-77f4-5cfe-9011-9ff6c38c651c"}},
		"data": map[string]any{"password": "aHVudGVyMg=="},
	}}
	e := newEnv(t, append(objs, secret), allowAll, allowAll)
	d := e.instance(t, "default", "podinfo")
	if o := objectNamed(t, &d, "Secret", "db-password"); o.Access != health.AccessWithheld {
		t.Fatalf("Secret entry = %+v, want withheld", o)
	}
	if d.Health.Partial || d.Health.Counts.Withheld != 1 || d.Health.State != health.Healthy {
		t.Fatalf("health = %+v, want healthy, not partial, one withheld", d.Health)
	}
	for _, r := range clusterReads(e.client) {
		if strings.Contains(r, "secrets") {
			t.Errorf("read Secrets: %s", r)
		}
	}
	for _, ra := range e.reviews.Asked() {
		if ra.Resource == "secrets" {
			t.Errorf("asked to read Secrets: %+v", ra)
		}
	}
}

func TestInventoryKindsIdleOutAndRestart(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	e := newEnv(t, loadF1(t), allowAll, allowAll, func(c *Config) {
		c.Now = clock.now
		c.IdleTimeout = time.Minute
	})
	e.instance(t, "default", "podinfo")
	e.m.mu.Lock()
	dep := e.m.inventory[clusterKinds[6].GVR()]
	e.m.mu.Unlock()
	if dep == nil || dep.cluster == nil || !dep.cluster.synced() {
		t.Fatalf("no running Deployment watch after the first read")
	}

	clock.advance(59 * time.Second)
	e.m.sweep()
	e.m.mu.Lock()
	kept := e.m.inventory[clusterKinds[6].GVR()] == dep
	e.m.mu.Unlock()
	if !kept {
		t.Fatal("Deployment watch stopped before the idle timeout")
	}

	clock.advance(2 * time.Second)
	e.m.sweep()
	select {
	case <-dep.cluster.stop:
	default:
		t.Fatal("Deployment watch still running after the idle timeout")
	}
	e.m.mu.Lock()
	n := len(e.m.inventory)
	e.m.mu.Unlock()
	if n != 0 {
		t.Fatalf("%d inventory kinds held after sweeping them all", n)
	}

	d := e.instance(t, "default", "podinfo")
	if o := objectNamed(t, &d, "Deployment", "podinfo-podinfo"); o.Access != health.AccessOK || o.Health.State != health.Healthy {
		t.Fatalf("Deployment after restart = %+v", o)
	}
}

// TestObjectsThatCannotBeWatchedArePolled: the reader may get Services but
// not list or watch them; the Service is read directly, marked not live,
// and refreshed every poll interval.
func TestObjectsThatCannotBeWatchedArePolled(t *testing.T) {
	getOnly := func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return ra.Resource != "services" || ra.Verb == "get"
	}
	e := newEnv(t, loadF1(t), allowAll, getOnly, func(c *Config) { c.PollInterval = 20 * time.Millisecond })
	d := e.instance(t, "default", "podinfo")
	svc := objectNamed(t, &d, "Service", "podinfo-podinfo")
	if svc.Access != health.AccessOK || svc.Live || svc.EvaluatedAt.IsZero() || svc.Health.State != health.Healthy {
		t.Fatalf("Service = %+v, want readable, healthy, not live, with an evaluation time", svc)
	}
	if d.Health.Live {
		t.Errorf("instance health says live with a polled object")
	}
	gets := func() int { return countReads(e, "get services default/podinfo-podinfo") }
	if gets() != 1 {
		t.Fatalf("Service gets = %d, want 1 synchronous read", gets())
	}
	deadline := time.Now().Add(5 * time.Second)
	for gets() < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if gets() < 3 {
		t.Fatalf("poller did not refresh the Service: %d gets", gets())
	}
	for _, r := range clusterReads(e.client) {
		if strings.HasPrefix(r, "list services") || strings.HasPrefix(r, "watch services") {
			t.Errorf("listed or watched a kind the reader may only get: %s", r)
		}
	}
}

func TestHoldChildrenWatchesWhileHeld(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	g := e.grant(t, "watch", pods, "default", "")
	release1, err := e.m.HoldChildren(t.Context(), alice, g, "default")
	if err != nil {
		t.Fatal(err)
	}
	release2, err := e.m.HoldChildren(t.Context(), alice, g, "default")
	if err != nil {
		t.Fatal(err)
	}
	e.m.mu.Lock()
	cw := e.m.children["default"]
	e.m.mu.Unlock()
	if cw == nil || cw.holders != 2 || len(cw.watches) != 3 {
		t.Fatalf("children watch = %+v, want two holders and three watches", cw)
	}

	// With interest held, a view reads children from the watches.
	before := clusterReads(e.client)
	e.instance(t, "default", "podinfo")
	for _, r := range clusterReads(e.client)[len(before):] {
		if strings.HasPrefix(r, "list pods") || strings.HasPrefix(r, "list replicasets") {
			t.Errorf("listed children on demand while they are watched: %s", r)
		}
	}

	release1()
	release1()
	if cw.watches[1].informer.IsStopped() {
		t.Fatal("watches stopped while one holder remains")
	}
	release2()
	for _, w := range cw.watches {
		select {
		case <-w.stop:
		default:
			t.Fatal("a children watch still runs after the last release")
		}
	}
	e.m.mu.Lock()
	_, held := e.m.children["default"]
	e.m.mu.Unlock()
	if held {
		t.Fatal("namespace still held after the last release")
	}
}

func TestHoldChildrenRefusesAnUncoveredGrant(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	for name, g := range map[string]struct{ verb, ns string }{
		"another namespace": {"watch", "web"},
		"list, not watch":   {"list", "default"},
	} {
		t.Run(name, func(t *testing.T) {
			release, err := e.m.HoldChildren(t.Context(), alice, e.grant(t, g.verb, pods, g.ns, ""), "default")
			if err == nil || release != nil {
				t.Fatalf("HoldChildren = %v, want a refusal", err)
			}
		})
	}
}

func TestListInstancesCarryHealth(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	items, err := e.m.ListInstances(t.Context(), alice, e.grant(t, "list", moduleInstances, "", ""), "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]health.State{}
	for _, i := range items {
		got[i.Ref.Name] = i.Health.State
	}
	want := map[string]health.State{
		"cert-manager": health.Healthy, "backup-consumer": health.Healthy,
		"backup-provider": health.Healthy, "podinfo": health.Healthy, "web": health.Healthy,
	}
	for name, state := range want {
		if got[name] != state {
			t.Errorf("%s health = %s, want %s", name, got[name], state)
		}
	}
	// One list over the namespace serves every instance in it.
	if lists := countReads(e, "list pods default/"); lists != 1 {
		t.Errorf("listed pods in default %d times, want once", lists)
	}
}

// countReads counts the cluster reads equal to line.
func countReads(e *env, line string) int {
	n := 0
	for _, r := range clusterReads(e.client) {
		if r == line {
			n++
		}
	}
	return n
}

// TestFailedReaderReviewDecidesNothing: a reader review that could not be
// made leaves the entries not readable, and the next read, once reviews
// work, watches the kind cluster-wide instead of settling for polling.
func TestFailedReaderReviewDecidesNothing(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	e.readerR.SetFail(true)
	d := e.instance(t, "default", "podinfo")
	if o := objectNamed(t, &d, "Deployment", "podinfo-podinfo"); o.Access != health.AccessNotReadable {
		t.Fatalf("Deployment while reviews fail = %+v, want not readable", o)
	}
	e.readerR.SetFail(false)
	d = e.instance(t, "default", "podinfo")
	if o := objectNamed(t, &d, "Deployment", "podinfo-podinfo"); o.Access != health.AccessOK || !o.Live {
		t.Fatalf("Deployment after reviews recover = %+v, want live and readable", o)
	}
}

// gate holds every reader review of one resource until it is opened, and
// says when the first one is waiting.
type gate struct {
	resource string
	waiting  chan struct{}
	open     chan struct{}
	once     sync.Once
}

func newGate(resource string) *gate {
	return &gate{resource: resource, waiting: make(chan struct{}), open: make(chan struct{})}
}

func (g *gate) rule(_ string, ra authorizationv1.ResourceAttributes) bool {
	if ra.Resource == g.resource {
		g.once.Do(func() { close(g.waiting) })
		<-g.open
	}
	return true
}

// TestSlowReaderReviewStallsOnlyItsView: while the reader's review for
// Deployments hangs in a cold instance read, the janitor and a tier-1 read
// still finish at once.
func TestSlowReaderReviewStallsOnlyItsView(t *testing.T) {
	g := newGate("deployments")
	e := newEnv(t, loadF1(t), allowAll, g.rule)
	done := make(chan struct{})
	go func() {
		defer close(done)
		e.instance(t, "default", "podinfo")
	}()
	<-g.waiting
	defer func() { close(g.open); <-done }()

	quick := make(chan error, 1)
	go func() {
		e.m.sweep()
		_, err := e.m.ListPackages(t.Context(), alice, e.grant(t, "list", modulePackages, "", ""), "")
		quick <- err
	}()
	select {
	case err := <-quick:
		if err != nil {
			t.Fatalf("ListPackages = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("sweep and a tier-1 read waited on a pending reader review")
	}
}

// TestKindSweptDuringItsReviewStartsNothing: a kind the janitor stops while
// its first view waits on the reader's review starts no informer once the
// review returns, so nothing runs that Stop cannot reach.
func TestKindSweptDuringItsReviewStartsNothing(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	g := newGate("deployments")
	e := newEnv(t, loadF1(t), allowAll, g.rule, func(c *Config) {
		c.Now = clock.now
		c.IdleTimeout = time.Minute
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		e.instance(t, "default", "podinfo")
	}()
	<-g.waiting
	e.m.mu.Lock()
	dep := e.m.inventory[clusterKinds[6].GVR()]
	e.m.mu.Unlock()
	clock.advance(2 * time.Minute)
	e.m.sweep()
	close(g.open)
	<-done

	dep.mu.Lock()
	defer dep.mu.Unlock()
	if !dep.stopped || dep.cluster != nil || len(dep.perNS) != 0 || dep.poller != nil {
		t.Fatalf("swept Deployment kind started something: cluster=%v perNS=%v poller=%v", dep.cluster, dep.perNS, dep.poller)
	}
}

// TestPolledObjectsNoViewReadsAreDropped: the poller stops refreshing an
// object once no view has read it for the idle period.
func TestPolledObjectsNoViewReadsAreDropped(t *testing.T) {
	clock := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	getOnly := func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return ra.Resource != "services" || ra.Verb == "get"
	}
	e := newEnv(t, loadF1(t), allowAll, getOnly, func(c *Config) {
		c.Now = clock.now
		c.IdleTimeout = time.Minute
		c.PollInterval = 10 * time.Millisecond
	})
	e.instance(t, "default", "podinfo")
	e.m.mu.Lock()
	svc := e.m.inventory[clusterKinds[2].GVR()]
	e.m.mu.Unlock()
	p := svc.poller
	size := func() int {
		p.mu.Lock()
		defer p.mu.Unlock()
		return len(p.entries)
	}
	if size() != 1 {
		t.Fatalf("poller holds %d objects, want the one Service", size())
	}
	clock.advance(2 * time.Minute)
	deadline := time.Now().Add(5 * time.Second)
	for size() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if size() != 0 {
		t.Fatal("poller still refreshes a Service no view has read for the idle period")
	}
}
