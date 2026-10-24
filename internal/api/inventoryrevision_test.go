package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

// revisionObjects is the F1 capture with podinfo at revision 3 and
// backup-provider with no recorded revision.
func revisionObjects(t *testing.T) []*unstructured.Unstructured {
	t.Helper()
	objs := loadF1(t)
	podinfo := readmodeltest.Find(t, objs, "ModuleInstance", "podinfo")
	if err := unstructured.SetNestedField(podinfo.Object, int64(3), "status", "inventory", "revision"); err != nil {
		t.Fatal(err)
	}
	provider := readmodeltest.Find(t, objs, "ModuleInstance", "backup-provider")
	unstructured.RemoveNestedField(provider.Object, "status", "inventory", "revision")
	return objs
}

// TestInstancesServeTheInventoryRevision: the revision the controller
// recorded is served on the document and the list item, in both modes, and
// an instance with none carries no field (portal:D2:R1).
func TestInstancesServeTheInventoryRevision(t *testing.T) {
	modes := map[string][]func(*Config){"local": nil, "in-cluster": {inCluster}}
	for name, opts := range modes {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, revisionObjects(t), readmodeltest.AllowAll, opts...)
			podinfo := decode[v1.Instance](t, e.get(t, base+"/instances/default/podinfo"))
			if podinfo.InventoryRevision != 3 || podinfo.InventoryCount != 2 {
				t.Errorf("podinfo revision %d count %d, want 3 and 2", podinfo.InventoryRevision, podinfo.InventoryCount)
			}
			none := e.get(t, base+"/instances/default/backup-provider")
			if bytes.Contains(none.body, []byte("inventoryRevision")) {
				t.Errorf("backup-provider records no revision but its document names one: %s", none.body)
			}
			if got := decode[v1.Instance](t, none); got.InventoryCount != 1 {
				t.Errorf("backup-provider count = %d, want 1", got.InventoryCount)
			}

			list := decode[v1.InstanceList](t, e.get(t, base+"/instances"))
			revisions := map[string]int64{}
			for _, it := range list.Items {
				revisions[it.Ref.Name] = it.InventoryRevision
			}
			want := map[string]int64{"cert-manager": 1, "podinfo": 3, "backup-provider": 0, "backup-consumer": 1, "web": 1}
			for n, w := range want {
				if got, ok := revisions[n]; !ok || got != w {
					t.Errorf("list item %s revision = %d (listed %v), want %d", n, got, ok, w)
				}
			}
		})
	}
}

// TestPackagesServeNoInventoryRevision: a package document and its list
// item carry no revision, whatever its status records.
func TestPackagesServeNoInventoryRevision(t *testing.T) {
	objs := loadF1(t)
	pkg := readmodeltest.Find(t, objs, "ModulePackage", "podinfo")
	if err := unstructured.SetNestedField(pkg.Object, int64(4), "status", "inventory", "revision"); err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, objs, readmodeltest.AllowAll)
	for _, path := range []string{base + "/packages/pkg/podinfo", base + "/packages"} {
		res := e.get(t, path)
		if res.status != http.StatusOK || bytes.Contains(res.body, []byte("inventoryRevision")) {
			t.Errorf("GET %s = %d, want 200 with no inventoryRevision: %s", path, res.status, res.body)
		}
	}
}

// TestForbiddenInstanceNamesNoRevision: a caller who may not read the
// instance gets the same 403 as before, which names no revision.
func TestForbiddenInstanceNamesNoRevision(t *testing.T) {
	plain := newEnv(t, loadF1(t), denyAll).get(t, base+"/instances/default/podinfo")
	withRevision := newEnv(t, revisionObjects(t), denyAll).get(t, base+"/instances/default/podinfo")
	if withRevision.status != http.StatusForbidden || !bytes.Equal(withRevision.body, plain.body) {
		t.Errorf("GET = %d %s, want the forbidden problem %s", withRevision.status, withRevision.body, plain.body)
	}
	if bytes.Contains(withRevision.body, []byte("inventoryRevision")) {
		t.Errorf("the forbidden document names a revision: %s", withRevision.body)
	}
}

// TestInstanceTopicFollowsTheRevision: an update that changes only the
// inventory revision reaches the instance topic as an upsert whose document
// carries it.
func TestInstanceTopicFollowsTheRevision(t *testing.T) {
	e := newEnv(t, loadF1(t), readmodeltest.AllowAll, fastStream)
	ts := newHTTPServer(t, e)
	c, res := openStream(t, ts, "instance:default/podinfo")
	if c == nil {
		t.Fatalf("stream refused: %d %s", res.status, res.body)
	}
	first := c.next(t, func(ev sse, _ message) bool { return ev.event == stream.EventSnapshot })
	var before v1.Instance
	if err := json.Unmarshal(first.Items[0], &before); err != nil || before.InventoryRevision != 1 {
		t.Fatalf("snapshot revision = %d (%v), want 1", before.InventoryRevision, err)
	}

	updateObject(t, e, instancesGVR, "default", "podinfo", func(u *unstructured.Unstructured) {
		_ = unstructured.SetNestedField(u.Object, int64(2), "status", "inventory", "revision")
	})
	m := c.next(t, func(ev sse, m message) bool {
		var d v1.Instance
		return ev.event == stream.EventUpsert && m.Topic == "instance:default/podinfo" &&
			json.Unmarshal(m.Item, &d) == nil && d.InventoryRevision == 2
	})
	var after v1.Instance
	if err := json.Unmarshal(m.Item, &after); err != nil {
		t.Fatal(err)
	}
	if after.InventoryCount != before.InventoryCount || after.Reconcile.State != before.Reconcile.State {
		t.Errorf("the update changed more than the revision: before %+v after %+v", before.InstanceSummary, after.InstanceSummary)
	}
}
