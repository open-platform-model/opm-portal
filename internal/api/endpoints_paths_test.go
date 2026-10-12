package api

import (
	"net/http"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

// withEndpointFixture is F1 plus the recorded EndpointSlice of the podinfo
// Service, a package whose inventory is that Service, and a provider that
// names it too.
func withEndpointFixture(t testing.TB) []*unstructured.Unstructured {
	t.Helper()
	return withServiceInProvider(t, readmodeltest.WithServicePackage(t, readmodeltest.WithEndpointSlices(t, loadF1(t))))
}

// withServiceInProvider makes the provider instance default/backup-provider
// name podinfo's Service in its inventory, so that the provider lookup of the
// Platform graph meets a Service.
//
// CONSTRUCTED, not captured: the captured provider deploys no Service. The
// entry is the shape of podinfo's own entry for the Service.
func withServiceInProvider(t testing.TB, objs []*unstructured.Unstructured) []*unstructured.Unstructured {
	t.Helper()
	prov := readmodeltest.Find(t, objs, "ModuleInstance", "backup-provider")
	entries, _, err := unstructured.NestedSlice(prov.Object, "status", "inventory", "entries")
	if err != nil {
		t.Fatal(err)
	}
	entries = append(entries, map[string]any{
		"component": "podinfo", "kind": "Service", "name": readmodeltest.PodinfoServiceName,
		"namespace": readmodeltest.PodinfoServiceNamespace, "v": "v1",
	})
	if err := unstructured.SetNestedSlice(prov.Object, entries, "status", "inventory", "entries"); err != nil {
		t.Fatal(err)
	}
	return objs
}

// endpointSliceAsks counts the checks for alice that name EndpointSlices.
func endpointSliceAsks(e *env) int {
	n := 0
	for _, a := range e.az.callerAsked() {
		if a.Resource.Resource == "endpointslices" {
			n++
		}
	}
	return n
}

// endpointSliceLists counts the lists of EndpointSlices the cluster served.
func endpointSliceLists(e *env) int {
	n := 0
	for _, a := range e.client.Actions() {
		if a.GetVerb() == "list" && a.GetResource().Resource == "endpointslices" {
			n++
		}
	}
	return n
}

// TestOnlyTheGraphPathsAskForTheEndpointCount: the Instance and Package
// documents and their stream snapshots, the owner events and the named-object
// check, the provider lookup of the Platform graph and both lists ask no
// review of EndpointSlices and list none. The two graphs ask, once each for
// podinfo's Service, and list once (the package's Service is the same one,
// so its count is read from the cache).
func TestOnlyTheGraphPathsAskForTheEndpointCount(t *testing.T) {
	const pkg = base + "/packages/pkg/" + readmodeltest.ServicePackageName
	e := newEnv(t, withEndpointFixture(t), readmodeltest.AllowAll, fastStream)

	for _, path := range []string{
		base + "/instances/default/podinfo",
		pkg,
		base + "/instances",
		base + "/instances?namespace=default",
		base + "/packages",
		base + "/instances/default/podinfo/events",
		base + "/instances/default/podinfo/events?scope=all",
		pkg + "/events",
		pkg + "/events?scope=all",
		base + "/instances/default/podinfo/events?kind=Service&namespace=default&name=" + readmodeltest.PodinfoServiceName,
		base + "/instances/default/podinfo/object?kind=Service&namespace=default&name=" + readmodeltest.PodinfoServiceName,
		pkg + "/object?kind=Service&namespace=default&name=" + readmodeltest.PodinfoServiceName,
		base + "/platform",
		base + "/platform/graph",
		base + "/instances/default/backup-provider",
	} {
		if res := e.get(t, path); res.status != http.StatusOK {
			t.Errorf("GET %s = %d %s", path, res.status, res.body)
		}
	}

	ts := newHTTPServer(t, e)
	topics := "instance:default/podinfo,package:pkg/" + readmodeltest.ServicePackageName
	c, res := openStream(t, ts, topics)
	if c == nil {
		t.Fatalf("stream refused: %d %s", res.status, res.body)
	}
	for range 2 {
		m := c.next(t, func(ev sse, _ message) bool { return ev.event == stream.EventSnapshot })
		if len(m.Items) != 1 {
			t.Errorf("snapshot of %s has %d items, want 1", m.Topic, len(m.Items))
		}
	}

	if n := endpointSliceAsks(e); n != 0 {
		t.Errorf("checks for endpointslices by the non-graph paths = %d, want 0", n)
	}
	if n := endpointSliceLists(e); n != 0 {
		t.Errorf("lists of endpointslices by the non-graph paths = %d, want 0", n)
	}

	if res := e.get(t, base+"/instances/default/podinfo/graph"); res.status != http.StatusOK {
		t.Fatalf("instance graph = %d %s", res.status, res.body)
	}
	if n := endpointSliceAsks(e); n != 1 {
		t.Errorf("checks for endpointslices after the instance graph = %d, want 1 (one Service)", n)
	}
	if res := e.get(t, pkg+"/graph"); res.status != http.StatusOK {
		t.Fatalf("package graph = %d %s", res.status, res.body)
	}
	if n := endpointSliceAsks(e); n != 2 {
		t.Errorf("checks for endpointslices after the package graph = %d, want 2", n)
	}
	if n := endpointSliceLists(e); n != 1 {
		t.Errorf("lists of endpointslices after both graphs = %d, want 1 (the second is served from the cache)", n)
	}
}
