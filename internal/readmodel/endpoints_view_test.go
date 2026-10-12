package readmodel

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

// The authorization cases of the ready count at the model: the caller's own
// review of the slices, the reader's, a failed read, and what the other
// views never ask for. Each runs over F1 plus the recorded slice of the
// podinfo Service.

// bob is a second caller whose reviews differ from alice's.
var bob = authz.Identity{Username: "bob", Groups: []string{"system:authenticated"}}

const podinfoSvc = readmodeltest.PodinfoServiceName

// instanceWithEndpoints reads an instance as alice with the count.
func (e *env) instanceWithEndpoints(t testing.TB, ns, name string) InstanceDetail {
	t.Helper()
	d, err := e.m.InstanceWithEndpoints(t.Context(), alice, e.grant(t, "get", moduleInstances, ns, name), ns, name)
	if err != nil {
		t.Fatalf("InstanceWithEndpoints(%s/%s) = %v", ns, name, err)
	}
	return d
}

// podinfoNode returns podinfo's Service as the instance view carries it.
func podinfoNode(t *testing.T, d *InstanceDetail) InventoryObject {
	t.Helper()
	return objectNamed(t, d, "Service", podinfoSvc)
}

// endpointReviews returns the reviews of EndpointSlices a checker was asked.
func endpointReviews(r *reviews) []authorizationv1.ResourceAttributes {
	var out []authorizationv1.ResourceAttributes
	asked := r.Asked()
	for i := range asked {
		if asked[i].Resource == "endpointslices" {
			out = append(out, asked[i])
		}
	}
	return out
}

// wantUnread fails unless o says its count could not be read.
func wantUnread(t testing.TB, o InventoryObject) {
	t.Helper()
	if !o.EndpointsUnread || o.EndpointsReady != nil {
		t.Errorf("%s %s: EndpointsUnread = %t, EndpointsReady = %v, want the unread mark and no count",
			o.Ref.Kind, o.Ref.Name, o.EndpointsUnread, o.EndpointsReady)
	}
}

// wantCount fails unless o carries a count of want.
func wantCount(t testing.TB, o InventoryObject, want int64) {
	t.Helper()
	if o.EndpointsReady == nil || *o.EndpointsReady != want || o.EndpointsUnread {
		t.Errorf("%s %s: EndpointsReady = %v, EndpointsUnread = %t, want %d and no mark",
			o.Ref.Kind, o.Ref.Name, o.EndpointsReady, o.EndpointsUnread, want)
	}
}

// neverBoth fails when an object carries the count and the unread mark, or
// when any object other than a Service carries either.
func neverBoth(t testing.TB, objs []InventoryObject) {
	t.Helper()
	for i := range objs {
		o := &objs[i]
		if o.EndpointsReady != nil && o.EndpointsUnread {
			t.Errorf("%s %s carries both the count and the unread mark", o.Ref.Kind, o.Ref.Name)
		}
		if o.Ref.Kind != "Service" && (o.EndpointsReady != nil || o.EndpointsUnread) {
			t.Errorf("%s %s carries an endpoint field", o.Ref.Kind, o.Ref.Name)
		}
	}
}

// (1) A caller allowed on every read gets the count.
func TestEndpointsCountForAnAllowedCaller(t *testing.T) {
	e := newEnv(t, withSlices(t), allowAll, allowAll)
	d := e.instanceWithEndpoints(t, "default", "podinfo")
	wantCount(t, podinfoNode(t, &d), 2)
	neverBoth(t, objectsOf(&d))
	if got := endpointReviews(e.reviews); len(got) != 1 || got[0].Verb != "list" || got[0].Group != "discovery.k8s.io" ||
		got[0].Namespace != "default" || got[0].Name != "" {
		t.Errorf("reviews of endpointslices = %+v, want one list in default with no name", got)
	}
}

// (1b) The count follows the slice: a Service whose Pods are all not ready
// carries 0, a count, and not the unread mark.
func TestEndpointsCountOfZero(t *testing.T) {
	e := newEnv(t, withSlices(t, readmodeltest.WithNoReadyEndpoints()), allowAll, allowAll)
	d := e.instanceWithEndpoints(t, "default", "podinfo")
	wantCount(t, podinfoNode(t, &d), 0)
}

// (2) A caller denied the slices gets the mark, no list is made, and the
// review asked is the slices' own: the Service's review does not stand in.
func TestEndpointsUnreadWhenTheCallerMayNotListSlices(t *testing.T) {
	e := newEnv(t, withSlices(t), denyResources("endpointslices"), allowAll)
	d := e.instanceWithEndpoints(t, "default", "podinfo")
	wantUnread(t, podinfoNode(t, &d))
	neverBoth(t, objectsOf(&d))
	if n := sliceLists(e.client); n != 0 {
		t.Errorf("lists of endpointslices = %d, want none for a caller denied them", n)
	}
	if got := endpointReviews(e.reviews); len(got) != 1 || got[0].Verb != "list" || got[0].Namespace != "default" {
		t.Errorf("reviews of endpointslices = %+v, want one list review in default", got)
	}
	if svc := podinfoNode(t, &d); svc.Access != health.AccessOK {
		t.Errorf("Service access = %q, want ok: the Service is still readable", svc.Access)
	}
}

// (3) A warm cache from one caller does not serve a caller who is denied.
func TestEndpointsWarmCacheDoesNotServeADeniedCaller(t *testing.T) {
	objs := withSlices(t)
	var bobChecker *authz.Checker
	e := newEnv(t, objs, allowAll, allowAll, func(c *Config) {
		bobChecker, _ = newChecker(t, bob, denyResources("endpointslices"), authz.Options{})
		c.Authorizer.(byIdentity)[bob.Username] = bobChecker
	})
	d := e.instanceWithEndpoints(t, "default", "podinfo")
	wantCount(t, podinfoNode(t, &d), 2)
	if n := sliceLists(e.client); n != 1 {
		t.Fatalf("lists after alice = %d, want 1", n)
	}
	g, err := bobChecker.Check(t.Context(), bob, authz.Attributes{Verb: "get", Resource: moduleInstances, Namespace: "default", Name: "podinfo"})
	if err != nil {
		t.Fatal(err)
	}
	bd, err := e.m.InstanceWithEndpoints(t.Context(), bob, g, "default", "podinfo")
	if err != nil {
		t.Fatal(err)
	}
	wantUnread(t, podinfoNode(t, &bd))
	if n := sliceLists(e.client); n != 1 {
		t.Errorf("lists after bob = %d, want still 1: the cache is neither used nor refilled for him", n)
	}
}

// (4) A reader that may not list the slices gives the mark and makes no list.
func TestEndpointsUnreadWhenTheReaderMayNotListSlices(t *testing.T) {
	e := newEnv(t, withSlices(t), allowAll, denyResources("endpointslices"))
	d := e.instanceWithEndpoints(t, "default", "podinfo")
	wantUnread(t, podinfoNode(t, &d))
	if n := sliceLists(e.client); n != 0 {
		t.Errorf("lists of endpointslices = %d, want none", n)
	}
	if got := endpointReviews(e.readerR); len(got) != 1 || got[0].Verb != "list" {
		t.Errorf("the reader's reviews of endpointslices = %+v, want one list", got)
	}
}

// unavailableFor makes the review of one resource fail the way a timeout
// does: an unavailable denial. Every other review goes to inner.
type unavailableFor struct {
	resource string
	inner    authz.Authorizer
}

func (u unavailableFor) Check(ctx context.Context, who authz.Identity, req authz.Attributes) (authz.Grant, error) {
	if req.Resource.Resource == u.resource {
		var none authz.Grant
		return none, &authz.DenialError{Code: authz.CodeUnavailable, Attributes: req}
	}
	return u.inner.Check(ctx, who, req)
}

// (5) A review that errors, a review that times out, and a list that errors,
// times out or has another page each give the mark, never a count of 0.
func TestEndpointsUnreadWhenAReadFails(t *testing.T) {
	t.Run("a review that errors", func(t *testing.T) {
		e := newEnv(t, withSlices(t), allowAll, allowAll)
		// The plain view warms every decision it needs and asks none about
		// slices, so the next review of them is the first.
		e.instance(t, "default", "podinfo")
		e.reviews.SetFail(true)
		d := e.instanceWithEndpoints(t, "default", "podinfo")
		wantUnread(t, podinfoNode(t, &d))
		if n := sliceLists(e.client); n != 0 {
			t.Errorf("lists of endpointslices = %d, want none after a failed review", n)
		}
		if svc := podinfoNode(t, &d); svc.Access != health.AccessOK {
			t.Errorf("Service access = %q, want ok", svc.Access)
		}
	})
	t.Run("a review that times out", func(t *testing.T) {
		e := newEnv(t, withSlices(t), allowAll, allowAll, func(c *Config) {
			c.Authorizer = unavailableFor{resource: "endpointslices", inner: c.Authorizer}
		})
		d := e.instanceWithEndpoints(t, "default", "podinfo")
		wantUnread(t, podinfoNode(t, &d))
		if n := sliceLists(e.client); n != 0 {
			t.Errorf("lists of endpointslices = %d, want none after an undecidable review", n)
		}
	})
	for name, fail := range sliceListFailures() {
		t.Run("a list that fails by "+name, func(t *testing.T) {
			e := newEnv(t, withSlices(t), allowAll, allowAll)
			e.client.PrependReactor("list", "endpointslices", fail)
			d := e.instanceWithEndpoints(t, "default", "podinfo")
			wantUnread(t, podinfoNode(t, &d))
			neverBoth(t, objectsOf(&d))
		})
	}
}

// (6) With no slice recorded a Service carries neither field.
func TestEndpointsAbsentWhenNoSliceIsRecorded(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	for _, in := range [][2]string{{"default", "podinfo"}, {"cert-manager", "cert-manager"}} {
		d := e.instanceWithEndpoints(t, in[0], in[1])
		for _, o := range objectsOf(&d) {
			if o.EndpointsReady != nil || o.EndpointsUnread {
				t.Errorf("%s/%s %s %s carries an endpoint field with no slice recorded", in[0], in[1], o.Ref.Kind, o.Ref.Name)
			}
		}
	}
}

// (7) With the slices denied the Deployment still carries its ReplicaSet and
// Pods: the unread mark costs nothing else.
func TestEndpointsDeniedLeavesTheWorkloadChildren(t *testing.T) {
	e := newEnv(t, withSlices(t), denyResources("endpointslices"), allowAll)
	d := e.instanceWithEndpoints(t, "default", "podinfo")
	dep := objectNamed(t, &d, "Deployment", "podinfo-podinfo")
	if len(dep.Children) != 3 || dep.ChildrenUnread {
		t.Errorf("Deployment children = %d, ChildrenUnread = %t, want a ReplicaSet and two Pods", len(dep.Children), dep.ChildrenUnread)
	}
}

// sameHealth fails when two reads of one instance differ in anything but the
// endpoint fields and the read time.
func sameHealth(t testing.TB, got, want *InstanceDetail) {
	t.Helper()
	gh, wh := got.Health, want.Health
	gh.EvaluatedAt, wh.EvaluatedAt = time.Time{}, time.Time{}
	if !reflect.DeepEqual(got.Applied, want.Applied) || !reflect.DeepEqual(gh, wh) {
		t.Errorf("instance Applied/Health = %+v/%+v, want %+v/%+v", got.Applied, gh, want.Applied, wh)
	}
	g, w := objectsOf(got), objectsOf(want)
	if len(g) != len(w) {
		t.Fatalf("objects = %d, want %d", len(g), len(w))
	}
	for i := range g {
		if g[i].Ref != w[i].Ref || g[i].Access != w[i].Access || !reflect.DeepEqual(g[i].Health, w[i].Health) ||
			g[i].ChildrenUnread != w[i].ChildrenUnread || !reflect.DeepEqual(g[i].Children, w[i].Children) {
			t.Errorf("%s %s differs from the read without slices:\n got %+v\nwant %+v", g[i].Ref.Kind, g[i].Ref.Name, g[i], w[i])
		}
	}
}

// (8) The Applied and Health of the Service and of every other object equal
// those of F1 without slices, whether the count is served or unread.
func TestEndpointsLeaveHealthAlone(t *testing.T) {
	plain := newEnv(t, loadF1(t), allowAll, allowAll)
	want := plain.instance(t, "default", "podinfo")
	for name, caller := range map[string]rule{"counted": allowAll, "unread": denyResources("endpointslices")} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, withSlices(t), caller, allowAll)
			got := e.instanceWithEndpoints(t, "default", "podinfo")
			sameHealth(t, &got, &want)
		})
	}
}

// (9) The two fields are never both set, and only a Service carries either,
// in every state above.
func TestEndpointsFieldsNeverMeet(t *testing.T) {
	cases := map[string]*env{
		"counted":       newEnv(t, withSlices(t), allowAll, allowAll),
		"caller denied": newEnv(t, withSlices(t), denyResources("endpointslices"), allowAll),
		"reader denied": newEnv(t, withSlices(t), allowAll, denyResources("endpointslices")),
		"no slice":      newEnv(t, loadF1(t), allowAll, allowAll),
	}
	for name, e := range cases {
		t.Run(name, func(t *testing.T) {
			for _, in := range [][2]string{{"default", "podinfo"}, {"cert-manager", "cert-manager"}} {
				d := e.instanceWithEndpoints(t, in[0], in[1])
				neverBoth(t, objectsOf(&d))
			}
		})
	}
}

// packageObjects flattens a package detail's components.
func packageObjects(d *PackageDetail) []InventoryObject {
	var out []InventoryObject
	for i := range d.Components {
		out = append(out, d.Components[i].Objects...)
	}
	return out
}

// (10) Instance, Package, ListInstances, ListPackages and ReachPod ask no
// review of EndpointSlices and list none, over the fixture where the count
// is available; InstanceWithEndpoints and PackageWithEndpoints ask one.
func TestEndpointsOnlyTheGraphReadsAsk(t *testing.T) {
	objs := readmodeltest.WithServicePackage(t, withSlices(t))
	e := newEnv(t, objs, allowAll, allowAll)
	ctx := t.Context()

	e.instance(t, "default", "podinfo")
	if _, err := e.m.Package(ctx, alice, e.grant(t, "get", modulePackages, readmodeltest.ServicePackageNamespace, readmodeltest.ServicePackageName), readmodeltest.ServicePackageNamespace, readmodeltest.ServicePackageName); err != nil {
		t.Fatal(err)
	}
	if _, err := e.m.ListInstances(ctx, alice, e.grant(t, "list", moduleInstances, "", ""), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := e.m.ListPackages(ctx, alice, e.grant(t, "list", modulePackages, "", ""), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := e.m.ReachPod(ctx, alice, e.logGrant(t, f1Pod), "default", f1Pod); err != nil {
		t.Fatal(err)
	}
	if got := endpointReviews(e.reviews); len(got) != 0 {
		t.Errorf("reviews of endpointslices by the plain reads = %+v, want none", got)
	}
	if n := sliceLists(e.client); n != 0 {
		t.Errorf("lists of endpointslices by the plain reads = %d, want none", n)
	}

	d := e.instanceWithEndpoints(t, "default", "podinfo")
	wantCount(t, podinfoNode(t, &d), 2)
	if got := endpointReviews(e.reviews); len(got) != 1 {
		t.Errorf("reviews of endpointslices after InstanceWithEndpoints = %+v, want one", got)
	}

	// The package read asks too, with a fresh caller so that its review is not
	// answered from the decision above.
	e2 := newEnv(t, objs, allowAll, allowAll)
	pd, err := e2.m.PackageWithEndpoints(ctx, alice, e2.grant(t, "get", modulePackages, readmodeltest.ServicePackageNamespace, readmodeltest.ServicePackageName), readmodeltest.ServicePackageNamespace, readmodeltest.ServicePackageName)
	if err != nil {
		t.Fatal(err)
	}
	pobjs := packageObjects(&pd)
	if len(pobjs) != 1 {
		t.Fatalf("package objects = %d, want the Service", len(pobjs))
	}
	wantCount(t, pobjs[0], 2)
	if got := endpointReviews(e2.reviews); len(got) != 1 {
		t.Errorf("reviews of endpointslices after PackageWithEndpoints = %+v, want one", got)
	}
}

// sentinels are written into the recorded slice by (11).
var sentinels = readmodeltest.Sentinels{
	Address: "198.51.100.77", Hostname: "sentinel-hostname", NodeName: "sentinel-node", Zone: "sentinel-zone",
	PodName: "sentinel-pod", PortName: "sentinel-port", LabelValue: "sentinel-label",
}

// captureLog sends the default slog logger to a buffer for the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// (11) With the sentinel copies, the JSON of every view and the captured log
// hold none of the sentinels, counted, unread and failed alike.
func TestEndpointsLeakNothingOfTheSlices(t *testing.T) {
	logs := captureLog(t)
	views := make([]any, 0, 9)
	objs := readmodeltest.WithServicePackage(t, withSlices(t, readmodeltest.WithSentinels(sentinels)))
	for name, caller := range map[string]rule{"counted": allowAll, "unread": denyResources("endpointslices")} {
		e := newEnv(t, objs, caller, allowAll)
		d := e.instanceWithEndpoints(t, "default", "podinfo")
		views = append(views, d, e.instance(t, "default", "podinfo"))
		pd, err := e.m.PackageWithEndpoints(t.Context(), alice, e.grant(t, "get", modulePackages, readmodeltest.ServicePackageNamespace, readmodeltest.ServicePackageName), readmodeltest.ServicePackageNamespace, readmodeltest.ServicePackageName)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		list, err := e.m.ListInstances(t.Context(), alice, e.grant(t, "list", moduleInstances, "", ""), "")
		if err != nil {
			t.Fatal(err)
		}
		views = append(views, pd, list)
	}
	failing := newEnv(t, objs, allowAll, allowAll)
	failing.client.PrependReactor("list", "endpointslices", sliceListFailures()["an error"])
	views = append(views, failing.instanceWithEndpoints(t, "default", "podinfo"))

	raw, err := json.Marshal(views)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sentinels.All() {
		if bytes.Contains(raw, []byte(s)) {
			t.Errorf("a view holds the sentinel %q", s)
		}
		if strings.Contains(logs.String(), s) {
			t.Errorf("the log holds the sentinel %q", s)
		}
	}
	if !strings.Contains(logs.String(), "endpoint count not served") {
		t.Errorf("the log has no line for the unread states: %q", logs.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		for _, want := range []string{"namespace=default", "service=" + podinfoSvc, "reason="} {
			if !strings.Contains(line, want) {
				t.Errorf("log line %q lacks %q", line, want)
			}
		}
	}
}
