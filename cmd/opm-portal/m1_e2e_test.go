//go:build e2e

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// The milestone 1 e2e test (task e2e:m1) reads the F1 fixture cluster
// through the built binary in local mode, over the JSON API and the SSE
// stream, and checks what milestone 1 promises: the Platform shows the
// accepted and the refused claim, the instance list keeps Applied and
// Health apart, an image break turns Health Degraded within seconds while
// Applied stays, a CLI-owned instance reads Managed externally, and a
// namespace-scoped identity sees its namespace and locked items only.

const (
	acceptedClaim = "default.backup-provider"
	refusedClaim  = "default.refused-claim-fixture"
	backupCatalog = "testing.opmodel.dev/catalogs/operator/backup@v0"
	// brokenTag is an image tag the podinfo image does not have.
	brokenTag = "opm-portal-e2e-absent"
	// degradedLag is how long after the cluster first reports the broken
	// Pod the stream may take to say Degraded.
	degradedLag = 10 * time.Second
)

var instanceGVR = schema.GroupVersionResource{Group: "opmodel.dev", Version: "v1alpha1", Resource: "moduleinstances"}

// TestM1 reads the fixture cluster as the kubeconfig's user, who may read
// everything. The image break runs last and is reverted when it ends.
func TestM1(t *testing.T) {
	kubeconfig, kubeContext := fixtureCluster(t)
	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Minute)
	defer cancel()
	p := startPortal(ctx, t, buildPortal(ctx, t), "serve", "--kubeconfig", kubeconfig, "--context", kubeContext, "--addr", "127.0.0.1:0")
	defer p.stop(t)
	browser, _ := launch(ctx, t, p.launch)
	s := m1Session{ctx: ctx, browser: browser, base: p.launch.Scheme + "://" + p.launch.Host}

	t.Run("platform shows the accepted and the refused claim", s.platformClaims)
	t.Run("instance list keeps applied and health apart", s.instanceAxes)
	t.Run("CLI-owned instance reads managed externally", s.cliOwned)
	t.Run("image break turns health degraded while applied stays", func(t *testing.T) {
		s.imageBreak(t, restConfig(t, kubeconfig, kubeContext))
	})
}

// TestM1NamespaceReader reads as a ServiceAccount that may read the OPM
// kinds only in default, with --namespaces default: it sees default's
// instances and nothing else, and every object it may not read is locked.
func TestM1NamespaceReader(t *testing.T) {
	kubeconfig, kubeContext := fixtureCluster(t)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	scoped := namespaceReader(ctx, t, kubeconfig, kubeContext)
	p := startPortal(ctx, t, buildPortal(ctx, t), "serve", "--kubeconfig", scoped, "--namespaces", "default", "--addr", "127.0.0.1:0")
	defer p.stop(t)
	browser, _ := launch(ctx, t, p.launch)
	s := m1Session{ctx: ctx, browser: browser, base: p.launch.Scheme + "://" + p.launch.Host}

	t.Run("sees its namespace only", s.onlyDefault)
	t.Run("everything else is forbidden", s.restForbidden)
	t.Run("objects it may not read are locked", s.lockedObjects)
	t.Run("the stream follows its namespace only", s.streamScoped)
}

// m1Session is one signed-in browser session on a running portal.
type m1Session struct {
	ctx     context.Context
	browser *http.Client
	base    string
}

func (s m1Session) api(path string) string { return s.base + "/api/v1alpha1/clusters/default" + path }

func (s m1Session) platformClaims(t *testing.T) {
	var pf v1.Platform
	getJSON(s.ctx, t, s.browser, s.api("/platform"), &pf)
	if pf.RegistrationsAccess != v1.AccessOK {
		t.Fatalf("registrationsAccess = %q; want ok", pf.RegistrationsAccess)
	}
	checkClaims(t, pf.Registrations)
	var contributed bool
	for _, c := range pf.Catalogs {
		contributed = contributed || (c.Catalog == backupCatalog && c.Source == "Registration" && c.ContributedBy == acceptedClaim)
	}
	if !contributed {
		t.Errorf("no catalog %s contributed by %s in %+v", backupCatalog, acceptedClaim, pf.Catalogs)
	}
	// The landing page shows both claims with their verdicts.
	page := get(s.ctx, t, s.browser, s.base+"/", nil)
	for _, want := range []string{acceptedClaim, refusedClaim, "Refused", "CatalogUnresolved"} {
		if !strings.Contains(page.body, want) {
			t.Errorf("the Platform page does not show %q", want)
		}
	}
}

// checkClaims: the backup provider's claim is accepted and active, and the
// deliberate refusal fixture is refused with its reason and message.
func checkClaims(t *testing.T, registrations []v1.Registration) {
	t.Helper()
	regs := map[string]*v1.Registration{}
	for i := range registrations {
		regs[registrations[i].Name] = &registrations[i]
	}
	if r := regs[acceptedClaim]; r == nil || r.Verdict != "Accepted" || !r.Accepted || !r.Active {
		t.Errorf("%s: %+v; want verdict Accepted, accepted and active", acceptedClaim, r)
	}
	if r := regs[refusedClaim]; r == nil || r.Verdict != "Refused" || r.Accepted || r.Active || r.Reason != "CatalogUnresolved" || r.Message == "" {
		t.Errorf("%s: %+v; want verdict Refused, reason CatalogUnresolved with a message, neither accepted nor active", refusedClaim, r)
	}
}

func (s m1Session) instanceAxes(t *testing.T) {
	var list v1.InstanceList
	getJSON(s.ctx, t, s.browser, s.api("/instances"), &list)
	if list.Access != v1.AccessOK {
		t.Fatalf("access = %q; want ok", list.Access)
	}
	want := map[string]struct{ owner, applied, health string }{
		"cert-manager/cert-manager": {"operator", "Applied", "Healthy"},
		"default/podinfo":           {"operator", "Applied", "Healthy"},
		"default/backup-provider":   {"operator", "Applied", "Healthy"},
		"default/backup-consumer":   {"operator", "Applied", "Healthy"},
		"web/web":                   {"cli", "ManagedExternally", "Healthy"},
	}
	got := map[string]*v1.InstanceSummary{}
	for i := range list.Items {
		it := &list.Items[i]
		got[it.Ref.Namespace+"/"+it.Ref.Name] = it
	}
	for key, w := range want {
		it, ok := got[key]
		if !ok {
			t.Errorf("%s is not listed", key)
			continue
		}
		if it.Owner != w.owner || it.Reconcile.State != w.applied || it.Health.State != w.health {
			t.Errorf("%s: owner %q, applied %q, health %q; want %q, %q, %q",
				key, it.Owner, it.Reconcile.State, it.Health.State, w.owner, w.applied, w.health)
		}
	}
}

func (s m1Session) cliOwned(t *testing.T) {
	var mi v1.Instance
	getJSON(s.ctx, t, s.browser, s.api("/instances/web/web"), &mi)
	if mi.Owner != "cli" || mi.Reconcile.State != "ManagedExternally" {
		t.Errorf("web/web: owner %q, applied %q; want cli, ManagedExternally", mi.Owner, mi.Reconcile.State)
	}
	if mi.Health.State != "Healthy" || len(mi.Components) == 0 {
		t.Errorf("web/web: health %q over %d components; want a computed Healthy", mi.Health.State, len(mi.Components))
	}
	page := get(s.ctx, t, s.browser, s.base+"/instances/web/web", nil)
	if page.status != http.StatusOK || !strings.Contains(page.body, "Managed externally") {
		t.Errorf("the web/web page: %d, does not read Managed externally", page.status)
	}
}

func (s m1Session) onlyDefault(t *testing.T) {
	var list v1.InstanceList
	getJSON(s.ctx, t, s.browser, s.api("/instances?namespace=default"), &list)
	names := map[string]bool{}
	for i := range list.Items {
		ref := list.Items[i].Ref
		if ref.Namespace != "default" {
			t.Errorf("the default list holds %s/%s", ref.Namespace, ref.Name)
		}
		names[ref.Name] = true
	}
	if list.Access != v1.AccessOK || !names["podinfo"] || !names["backup-provider"] || !names["backup-consumer"] {
		t.Errorf("default list: access %q, items %v; want ok with podinfo, backup-provider and backup-consumer", list.Access, names)
	}
}

// restForbidden: lists and objects outside default are forbidden, never
// empty or failed, and the cluster-wide list page is a locked list.
func (s m1Session) restForbidden(t *testing.T) {
	for _, path := range []string{"/instances", "/instances?namespace=web", "/instances?namespace=cert-manager"} {
		var other v1.InstanceList
		getJSON(s.ctx, t, s.browser, s.api(path), &other)
		if other.Access != v1.AccessForbidden || len(other.Items) != 0 {
			t.Errorf("%s: access %q with %d items; want forbidden with none", path, other.Access, len(other.Items))
		}
	}
	for _, path := range []string{"/instances/web/web", "/instances/cert-manager/cert-manager", "/platform"} {
		res := get(s.ctx, t, s.browser, s.api(path), nil)
		if res.status != http.StatusForbidden || !strings.Contains(res.body, `"code":"forbidden"`) {
			t.Errorf("%s: %d %.200s; want 403 forbidden", path, res.status, res.body)
		}
	}
	if page := get(s.ctx, t, s.browser, s.base+"/instances", nil); !strings.Contains(page.body, "Locked: you may not list") {
		t.Errorf("the cluster-wide instance page is not a locked list")
	}
}

// lockedObjects: podinfo is readable, but the reader may not read its
// Deployment or Service, so they are listed by reference only, locked.
func (s m1Session) lockedObjects(t *testing.T) {
	var mi v1.Instance
	getJSON(s.ctx, t, s.browser, s.api("/instances/default/podinfo"), &mi)
	var objects int
	for i := range mi.Components {
		for j := range mi.Components[i].Objects {
			o := &mi.Components[i].Objects[j]
			objects++
			if o.Access != v1.AccessForbidden || o.Health != nil {
				t.Errorf("%s %s: access %q, health %v; want forbidden with no health", o.Ref.Kind, o.Ref.Name, o.Access, o.Health)
			}
		}
	}
	if objects == 0 || !mi.Health.Partial {
		t.Errorf("podinfo: %d objects, partial %t; want its objects listed and its health partial", objects, mi.Health.Partial)
	}
	page := get(s.ctx, t, s.browser, s.base+"/instances/default/podinfo", nil)
	if rows := strings.Count(page.body, `<li class="object locked">`); page.status != http.StatusOK || rows != objects {
		t.Errorf("the podinfo page: %d with %d locked object rows; want 200 with %d", page.status, rows, objects)
	}
}

// streamScoped: on the stream, default's list is followed and the
// cluster-wide list is closed as forbidden.
func (s m1Session) streamScoped(t *testing.T) {
	events := openStream(s.ctx, t, s.browser, s.base, "instances:default,instances")
	seen := map[string]string{}
	for len(seen) < 2 {
		e := events.next(t, 30*time.Second)
		var env struct {
			Topic string `json:"topic"`
			Code  string `json:"code"`
		}
		if json.Unmarshal([]byte(e.data), &env) != nil || env.Topic == "" {
			continue
		}
		switch e.name {
		case "snapshot":
			seen[env.Topic] = "snapshot"
		case "closed":
			seen[env.Topic] = "closed " + env.Code
		}
	}
	if seen["instances:default"] != "snapshot" || seen["instances"] != "closed "+v1.CodeForbidden {
		t.Errorf("stream topics: %v; want a snapshot of instances:default and instances closed forbidden", seen)
	}
}

// imageBreak patches podinfo's image tag to one that does not exist and
// follows podinfo on the stream: the new Pod cannot pull, so health must
// turn Degraded within degradedLag of the cluster reporting it, while the
// operator's applied state stays Applied. The patch is reverted, and the
// rollout waited for, when the test ends.
func (s m1Session) imageBreak(t *testing.T, cfg *rest.Config) {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	events := openStream(s.ctx, t, s.browser, s.base, "instance:default/podinfo")
	first := events.instance(t, 30*time.Second)
	if first.Reconcile.State != "Applied" || first.Health.State != "Healthy" {
		t.Fatalf("podinfo before the break: applied %q, health %q; want Applied and Healthy (is the fixture settled?)", first.Reconcile.State, first.Health.State)
	}

	mis := dyn.Resource(instanceGVR).Namespace("default")
	revert := podinfoImageRevert(s.ctx, t, mis)
	t.Cleanup(func() { restorePodinfo(context.WithoutCancel(s.ctx), t, mis, cs, revert) })
	patched := time.Now()
	if _, err := mis.Patch(s.ctx, "podinfo", types.MergePatchType,
		[]byte(`{"spec":{"values":{"image":{"tag":"`+brokenTag+`"}}}}`), metav1.PatchOptions{}); err != nil {
		t.Fatalf("patching podinfo's image tag: %v", err)
	}
	broken := brokenPodSeen(s.ctx, cs)
	degraded := events.untilDegraded(t, patched)

	var clusterAt time.Time
	select {
	case clusterAt = <-broken:
	case <-time.After(30 * time.Second):
		t.Fatal("the stream said Degraded, but the cluster never reported a Pod failing to pull")
	}
	lag := degraded.Sub(clusterAt)
	t.Logf("the cluster reported the broken Pod %s after the patch; the stream said Degraded %s after it (%s after the cluster)",
		clusterAt.Sub(patched).Round(time.Millisecond), degraded.Sub(patched).Round(time.Millisecond), lag.Round(time.Millisecond))
	if lag > degradedLag {
		t.Errorf("the stream said Degraded %s after the cluster reported the broken Pod; want within %s", lag.Round(time.Millisecond), degradedLag)
	}

	// Applied stays: the operator neither fails nor stalls on a workload it
	// applied, and Degraded stays while the Pod cannot pull.
	events.holdApplied(t, 5*time.Second)
	var list v1.InstanceList
	getJSON(s.ctx, t, s.browser, s.api("/instances?namespace=default"), &list)
	for i := range list.Items {
		it := &list.Items[i]
		if it.Ref.Name == "podinfo" && (it.Reconcile.State != "Applied" || it.Health.State != "Degraded") {
			t.Errorf("podinfo in the list: applied %q, health %q; want Applied and Degraded", it.Reconcile.State, it.Health.State)
		}
	}
}

type sseEvent struct{ name, data string }

type sseStream struct{ events <-chan sseEvent }

// untilDegraded reads Instance documents until one says Degraded, and
// returns when it arrived. That document must still say Applied; a passing
// Reconciling while the operator applies the patch is logged, not failed.
func (s *sseStream) untilDegraded(t *testing.T, patched time.Time) time.Time {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for {
		mi := s.instance(t, time.Until(deadline))
		if mi.Health.State != "Degraded" {
			if mi.Reconcile.State != "Applied" {
				t.Logf("%s after the patch: applied %q, health %q", time.Since(patched).Round(time.Millisecond), mi.Reconcile.State, mi.Health.State)
			}
			continue
		}
		if mi.Reconcile.State != "Applied" {
			t.Errorf("health turned Degraded with applied %q; want Applied", mi.Reconcile.State)
		}
		return time.Now()
	}
}

// holdApplied reads the Instance documents the stream carries for d: none
// may say Failed or Stalled, or leave Degraded.
func (s *sseStream) holdApplied(t *testing.T, d time.Duration) {
	t.Helper()
	until := time.After(d)
	for {
		select {
		case <-until:
			return
		case e, ok := <-s.events:
			if !ok {
				t.Fatal("the stream ended")
			}
			if e.name == "closed" {
				t.Fatalf("the topic was closed: %s", e.data)
			}
			if e.name != "upsert" {
				continue
			}
			var m struct {
				Item v1.Instance `json:"item"`
			}
			if err := json.Unmarshal([]byte(e.data), &m); err != nil {
				t.Fatalf("upsert: %v", err)
			}
			if st := m.Item.Reconcile.State; st == "Failed" || st == "Stalled" || m.Item.Health.State != "Degraded" {
				t.Errorf("after the break: applied %q, health %q; want Applied to stay and Degraded", st, m.Item.Health.State)
			}
		}
	}
}

// brokenPodSeen polls podinfo's Pods and sends the time it first sees a
// container waiting on an image it cannot pull.
func brokenPodSeen(ctx context.Context, cs kubernetes.Interface) <-chan time.Time {
	out := make(chan time.Time, 1)
	go func() {
		tick := time.NewTicker(250 * time.Millisecond)
		defer tick.Stop()
		for {
			pods, err := cs.CoreV1().Pods("default").List(ctx, metav1.ListOptions{LabelSelector: "module-instance.opmodel.dev/name=podinfo"})
			if err == nil && anyPullFailure(pods.Items) {
				out <- time.Now()
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	return out
}

func anyPullFailure(pods []corev1.Pod) bool {
	for i := range pods {
		for j := range pods[i].Status.ContainerStatuses {
			if w := pods[i].Status.ContainerStatuses[j].State.Waiting; w != nil && (w.Reason == "ErrImagePull" || w.Reason == "ImagePullBackOff") {
				return true
			}
		}
	}
	return false
}

// podinfoImageRevert returns the JSON patch that puts podinfo's
// spec.values.image back as it is now: the same value when the fixture
// sets one, removed when it does not.
func podinfoImageRevert(ctx context.Context, t *testing.T, mis dynamic.ResourceInterface) []byte {
	t.Helper()
	mi, err := mis.Get(ctx, "podinfo", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("reading podinfo before the break: %v", err)
	}
	image, found, err := unstructured.NestedFieldCopy(mi.Object, "spec", "values", "image")
	if err != nil {
		t.Fatalf("reading podinfo's spec.values.image: %v", err)
	}
	op := map[string]any{"op": "remove", "path": "/spec/values/image"}
	if found {
		op = map[string]any{"op": "replace", "path": "/spec/values/image", "value": image}
	}
	revert, err := json.Marshal([]any{op})
	if err != nil {
		t.Fatal(err)
	}
	return revert
}

// restorePodinfo applies revert, which puts podinfo's image value back, and
// waits until podinfo's Deployment has rolled back and only ready Pods
// remain, so the tests that follow read a settled fixture.
func restorePodinfo(ctx context.Context, t *testing.T, mis dynamic.ResourceInterface, cs kubernetes.Interface, revert []byte) {
	if _, err := mis.Patch(ctx, "podinfo", types.JSONPatchType, revert, metav1.PatchOptions{}); err != nil {
		t.Errorf("reverting podinfo's image tag: %v", err)
		return
	}
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		if podinfoSettled(ctx, cs) {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Errorf("podinfo did not settle within 5 minutes of reverting the image tag")
}

func podinfoSettled(ctx context.Context, cs kubernetes.Interface) bool {
	sel := metav1.ListOptions{LabelSelector: "module-instance.opmodel.dev/name=podinfo"}
	deps, err := cs.AppsV1().Deployments("default").List(ctx, sel)
	if err != nil || len(deps.Items) == 0 {
		return false
	}
	for i := range deps.Items {
		if !rolledOut(&deps.Items[i]) {
			return false
		}
	}
	pods, err := cs.CoreV1().Pods("default").List(ctx, sel)
	if err != nil || len(pods.Items) == 0 {
		return false
	}
	for i := range pods.Items {
		p := &pods.Items[i]
		if p.DeletionTimestamp != nil || p.Status.Phase != corev1.PodRunning || anyPullFailure(pods.Items[i:i+1]) {
			return false
		}
	}
	return true
}

func rolledOut(d *appsv1.Deployment) bool {
	want := int32(1)
	if d.Spec.Replicas != nil {
		want = *d.Spec.Replicas
	}
	s := d.Status
	return s.ObservedGeneration >= d.Generation && s.UpdatedReplicas == want && s.Replicas == want && s.AvailableReplicas == want
}

// getJSON reads a 200 JSON document into v.
func getJSON(ctx context.Context, t *testing.T, c *http.Client, target string, v any) {
	t.Helper()
	res := get(ctx, t, c, target, nil)
	if res.status != http.StatusOK {
		t.Fatalf("GET %s: %d %.300s; want 200", target, res.status, res.body)
	}
	if err := json.Unmarshal([]byte(res.body), v); err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
}

// openStream opens a stream on topics and delivers its events until the
// test ends.
func openStream(ctx context.Context, t *testing.T, browser *http.Client, base, topics string) *sseStream {
	t.Helper()
	sctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(sctx, http.MethodGet, base+"/api/v1alpha1/clusters/default/stream?topics="+url.QueryEscape(topics), http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	res, err := (&http.Client{Jar: browser.Jar}).Do(req) //nolint:bodyclose // the reader goroutine closes it
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		t.Fatalf("stream %s: %d", topics, res.StatusCode)
	}
	out := make(chan sseEvent, 64)
	go func() {
		defer close(out)
		defer func() { _ = res.Body.Close() }()
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		var name string
		for sc.Scan() {
			line := sc.Text()
			if n, ok := strings.CutPrefix(line, "event: "); ok {
				name = n
				continue
			}
			if d, ok := strings.CutPrefix(line, "data: "); ok {
				select {
				case out <- sseEvent{name: name, data: d}:
				case <-sctx.Done():
					return
				}
			}
		}
	}()
	return &sseStream{events: out}
}

// next returns the stream's next event, failing the test after wait.
func (s *sseStream) next(t *testing.T, wait time.Duration) sseEvent {
	t.Helper()
	select {
	case e, ok := <-s.events:
		if !ok {
			t.Fatal("the stream ended")
		}
		return e
	case <-time.After(wait):
		t.Fatalf("no stream event within %s", wait)
	}
	return sseEvent{}
}

// instance returns the next Instance document the stream carries, from a
// snapshot or an upsert, failing the test on a closed topic or after wait.
func (s *sseStream) instance(t *testing.T, wait time.Duration) v1.Instance {
	t.Helper()
	deadline := time.Now().Add(wait)
	for {
		e := s.next(t, time.Until(deadline))
		switch e.name {
		case "closed":
			t.Fatalf("the topic was closed: %s", e.data)
		case "snapshot":
			var m struct {
				Items []v1.Instance `json:"items"`
			}
			if err := json.Unmarshal([]byte(e.data), &m); err != nil || len(m.Items) != 1 || m.Items[0].Kind != v1.KindInstance {
				t.Fatalf("snapshot: %v %.300s; want one Instance", err, e.data)
			}
			return m.Items[0]
		case "upsert":
			var m struct {
				Item v1.Instance `json:"item"`
			}
			if err := json.Unmarshal([]byte(e.data), &m); err != nil || m.Item.Kind != v1.KindInstance {
				t.Fatalf("upsert: %v %.300s; want an Instance", err, e.data)
			}
			return m.Item
		}
	}
}
