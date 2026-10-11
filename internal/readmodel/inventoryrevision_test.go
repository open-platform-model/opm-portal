package readmodel

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestInventoryRevision(t *testing.T) {
	withInventory := func(inv map[string]any) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{"status": map[string]any{"inventory": inv}}}
	}
	cases := []struct {
		name string
		u    *unstructured.Unstructured
		want int64
	}{
		{"revision 3", withInventory(map[string]any{"revision": int64(3), "count": int64(2)}), 3},
		{"revision 1", withInventory(map[string]any{"revision": int64(1)}), 1},
		{"no status.inventory", &unstructured.Unstructured{Object: map[string]any{"status": map[string]any{}}}, 0},
		{"no status", &unstructured.Unstructured{Object: map[string]any{}}, 0},
		{"an inventory with no revision", withInventory(map[string]any{"count": int64(2)}), 0},
		{"revision 0", withInventory(map[string]any{"revision": int64(0)}), 0},
		{"revision -2", withInventory(map[string]any{"revision": int64(-2)}), 0},
		{"revision as a string", withInventory(map[string]any{"revision": "3"}), 0},
		{"revision as a float", withInventory(map[string]any{"revision": 3.5}), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := inventoryRevision(tc.u); got != tc.want {
				t.Errorf("inventoryRevision = %d, want %d", got, tc.want)
			}
			if got := instanceItem(tc.u).InventoryRevision; got != tc.want {
				t.Errorf("instanceItem revision = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestInstanceRevisionOverF1 reads the F1 capture: all five instances record
// revision 1, beside the counts the capture holds.
func TestInstanceRevisionOverF1(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	list, err := e.m.ListInstances(t.Context(), alice, e.grant(t, "list", moduleInstances, "", ""), "")
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int64{
		"cert-manager/cert-manager": 42, "default/podinfo": 2, "default/backup-provider": 1,
		"default/backup-consumer": 1, "web/web": 2,
	}
	if len(list) != len(counts) {
		t.Fatalf("ListInstances = %d items, want %d", len(list), len(counts))
	}
	for _, it := range list {
		key := it.Ref.Namespace + "/" + it.Ref.Name
		want, ok := counts[key]
		if !ok {
			t.Errorf("unexpected instance %s", key)
			continue
		}
		if it.InventoryCount != want || it.InventoryRevision != 1 {
			t.Errorf("%s: count %d revision %d, want %d and 1", key, it.InventoryCount, it.InventoryRevision, want)
		}
	}
	d, err := e.m.Instance(t.Context(), alice, e.grant(t, "get", moduleInstances, "default", "podinfo"), "default", "podinfo")
	if err != nil {
		t.Fatal(err)
	}
	if d.InventoryRevision != 1 {
		t.Errorf("podinfo detail revision = %d, want 1", d.InventoryRevision)
	}
}

// TestInstanceNeverApplied: an instance whose apply failed before it recorded
// an inventory has no revision.
func TestInstanceNeverApplied(t *testing.T) {
	e := newEnv(t, loadList(t, applyFailedSample), allowAll, allowAll)
	d, err := e.m.Instance(t.Context(), alice, e.grant(t, "get", moduleInstances, "cert-manager", "cert-manager"), "cert-manager", "cert-manager")
	if err != nil {
		t.Fatal(err)
	}
	if d.InventoryRevision != 0 || d.InventoryCount != 0 {
		t.Errorf("revision %d count %d, want none", d.InventoryRevision, d.InventoryCount)
	}
}
