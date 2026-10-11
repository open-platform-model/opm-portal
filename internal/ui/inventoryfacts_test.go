package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// Tests of the "Inventory" and "Inventory revision" facts on the panel of an
// instance's own node (add-inventory-revision).

// TestInventoryFacts: the two facts of one instance, by what the API serves.
func TestInventoryFacts(t *testing.T) {
	for _, tc := range []struct {
		name      string
		count     int64
		revision  int64
		inventory string
		rev       string
	}{
		{"2 objects, revision 1", 2, 1, "2 objects", "1"},
		{"1 object, revision 1", 1, 1, "1 object", "1"},
		{"0 objects, revision 2", 0, 2, "0 objects", "2"},
		{"0 objects, none recorded", 0, 0, "", ""},
		{"5 objects, no revision", 5, 0, "5 objects", ""},
		{"42 objects, revision 12", 42, 12, "42 objects", "12"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := map[string]string{}
			for _, f := range inventoryFacts(&v1.Instance{InstanceSummary: v1.InstanceSummary{InventoryCount: tc.count, InventoryRevision: tc.revision}}) {
				got[f.Label] = f.Value
			}
			if got["Inventory"] != tc.inventory || got["Inventory revision"] != tc.rev || len(got) != btoi(tc.inventory != "")+btoi(tc.rev != "") {
				t.Errorf("facts = %v, want Inventory %q and Inventory revision %q", got, tc.inventory, tc.rev)
			}
		})
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// instanceNamed finds the ModuleInstance ns/name in objs.
func instanceNamed(t *testing.T, objs []*unstructured.Unstructured, ns, name string) *unstructured.Unstructured {
	t.Helper()
	for _, u := range objs {
		if u.GetKind() == "ModuleInstance" && u.GetNamespace() == ns && u.GetName() == name {
			return u
		}
	}
	t.Fatalf("no ModuleInstance %s/%s in the capture", ns, name)
	return nil
}

// TestRootPanelShowsTheInventory: podinfo's panel at rest and as a fragment
// reads its object count and revision; backup-provider's reads one object.
func TestRootPanelShowsTheInventory(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	frag := factsOfPanel(panelOf(t, s, "/instances/default/podinfo", "mi:default/podinfo"))
	if frag["Inventory"] != "2 objects" || frag["Inventory revision"] != "1" {
		t.Errorf("podinfo root facts: %v", frag)
	}
	// The resting panel the page renders in place carries the same facts.
	detail := between(mainOf(s.get(t, "/instances/default/podinfo?tab=graph").body), `id="detail"`, "</aside>")
	if rest := factsOfPanel(detail); rest["Inventory"] != "2 objects" || rest["Inventory revision"] != "1" {
		t.Errorf("the panel at rest: %v", rest)
	}
	one := factsOfPanel(panelOf(t, s, "/instances/default/backup-provider", "mi:default/backup-provider"))
	if one["Inventory"] != "1 object" || one["Inventory revision"] != "1" {
		t.Errorf("backup-provider root facts: %v", one)
	}
	cm := factsOfPanel(panelOf(t, s, "/instances/cert-manager/cert-manager", "mi:cert-manager/cert-manager"))
	if cm["Inventory"] != "42 objects" {
		t.Errorf("cert-manager root facts: %v", cm)
	}
}

// TestRootPanelInventoryByWhatIsRecorded: an applied instance with an empty
// inventory shows both facts, an instance never applied shows neither, and
// an inventory with no revision shows the count only.
func TestRootPanelInventoryByWhatIsRecorded(t *testing.T) {
	for _, tc := range []struct {
		name      string
		change    func(*unstructured.Unstructured)
		inventory string
		rev       string
	}{
		{"applied, nothing rendered", func(u *unstructured.Unstructured) {
			_ = unstructured.SetNestedField(u.Object, int64(0), "status", "inventory", "count")
			_ = unstructured.SetNestedSlice(u.Object, []any{}, "status", "inventory", "entries")
			_ = unstructured.SetNestedField(u.Object, int64(2), "status", "inventory", "revision")
		}, "0 objects", "2"},
		{"never applied", func(u *unstructured.Unstructured) {
			unstructured.RemoveNestedField(u.Object, "status", "inventory")
		}, "", ""},
		{"no revision recorded", func(u *unstructured.Unstructured) {
			unstructured.RemoveNestedField(u.Object, "status", "inventory", "revision")
		}, "2 objects", ""},
		{"a revision the operator does not write", func(u *unstructured.Unstructured) {
			_ = unstructured.SetNestedField(u.Object, "3", "status", "inventory", "revision")
		}, "2 objects", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			objs := apitest.F1(t)
			tc.change(instanceNamed(t, objs, "default", "podinfo"))
			for _, site := range []*site{newSite(t, objs, apitest.AllowAll), newInClusterSite(t, objs, apitest.AllowAll)} {
				got := factsOfPanel(panelOf(t, site, "/instances/default/podinfo", "mi:default/podinfo"))
				if got["Inventory"] != tc.inventory || got["Inventory revision"] != tc.rev {
					t.Errorf("facts = %v, want Inventory %q and Inventory revision %q", got, tc.inventory, tc.rev)
				}
			}
		})
	}
}

// TestInventoryFactsOnlyOnTheOwnersNode: no other node carries either fact,
// nor does a package's root panel, whose Revision stays the source's.
func TestInventoryFactsOnlyOnTheOwnersNode(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, node := range []string{nodeDeployment, nodeService, nodePod, "comp:mi/default/podinfo/podinfo"} {
		got := factsOfPanel(panelOf(t, s, "/instances/default/podinfo", node))
		if _, ok := got["Inventory"]; ok {
			t.Errorf("node %s shows Inventory: %v", node, got)
		}
		if _, ok := got["Inventory revision"]; ok {
			t.Errorf("node %s shows Inventory revision: %v", node, got)
		}
	}
	pkg := factsOfPanel(panelOf(t, s, "/packages/pkg/podinfo", "mp:pkg/podinfo"))
	if _, ok := pkg["Inventory"]; ok || pkg["Inventory revision"] != "" || pkg["Revision"] != "not recorded" {
		t.Errorf("package root facts: %v", pkg)
	}
}

// TestLockedRootPanelShowsNoInventory: a root node whose access is not ok
// renders in the locked form, which has no facts (SR7).
func TestLockedRootPanelShowsNoInventory(t *testing.T) {
	h, err := New(Config{API: apitest.New(t, apitest.F1(t), apitest.AllowAll), Now: func() time.Time { return apitest.Now }})
	if err != nil {
		t.Fatal(err)
	}
	ref := &v1.ObjectRef{Group: "opmodel.dev", Kind: "ModuleInstance", Namespace: "default", Name: "podinfo"}
	g := &v1.Graph{Root: "mi:default/podinfo", Nodes: []v1.GraphNode{{
		ID: "mi:default/podinfo", Kind: instanceKind.Topic, Label: "default/podinfo", Ref: ref, Access: v1.AccessForbidden,
	}}}
	self := &ownerView{Kind: instanceKind, Base: "/instances/default/podinfo", Instance: &v1.Instance{
		InstanceSummary: v1.InstanceSummary{InventoryCount: 2, InventoryRevision: 1},
	}}
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/instances/default/podinfo/node?id=x", http.NoBody)
	p := h.panel(r, g, "mi:default/podinfo", panelContext{owner: instanceKind, base: self.Base, self: self})
	if !p.Locked || len(p.Facts) != 0 {
		t.Fatalf("panel locked = %v with facts %v, want locked with none", p.Locked, p.Facts)
	}
	w := httptest.NewRecorder()
	h.renderPanel(w, r, p)
	if body := w.Body.String(); !strings.Contains(body, "locked-panel") || strings.Contains(body, "Inventory") {
		t.Errorf("the locked root panel:\n%s", body)
	}
}
