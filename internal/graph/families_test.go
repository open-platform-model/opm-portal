package graph

import (
	"slices"
	"testing"

	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

func config(name string, objs ...readmodel.InventoryObject) readmodel.Component {
	return readmodel.Component{Name: name, Objects: objs, Health: health.Summary{State: health.Healthy, Live: true, Counts: health.Counts{Healthy: len(objs)}}}
}

var (
	cm   = func(n string) readmodel.InventoryObject { return object(ref("", "ConfigMap", n)) }
	role = func(n string) readmodel.InventoryObject {
		return object(ref("rbac.authorization.k8s.io", "Role", n))
	}
	crd = func(n string) readmodel.InventoryObject {
		return object(ref("apiextensions.k8s.io", "CustomResourceDefinition", n))
	}
	reg = func(n string) readmodel.InventoryObject {
		return object(ref("opmodel.dev", "TransformerRegistration", n))
	}
)

// TestFamiliesByAPIGroup: a component joins the family its objects' API
// group names when all of them agree, and Other otherwise; a family folds
// only when its components hold two or more objects.
func TestFamiliesByAPIGroup(t *testing.T) {
	in := detail(
		config("crd", crd("a")),            // CRDs: 1 object, alone
		config("role-a", role("r1")),       // RBAC: 2 objects across 2 components
		config("role-b", role("r2")),       //
		config("mixed", crd("b"), cm("c")), // mixed: Other
		config("map", cm("d")),             // Other: with mixed, 3 objects
		config("empty"),                    // no objects: Other, nothing to hide
	)
	g := Instance(in, Options{})
	groups := map[Family]Node{}
	for _, n := range g.Nodes {
		if n.Kind == KindGroup {
			groups[n.Group.Family] = n
		}
	}
	if len(groups) != 2 {
		t.Fatalf("groups = %v, want RBAC and Other only", groups)
	}
	if rbac := groups[FamilyRBAC]; !slices.Equal(rbac.Group.Members, []string{"role-a", "role-b"}) || rbac.ID != "grp:configuration/mi/ns/x/rbac" {
		t.Errorf("RBAC group = %+v", rbac)
	}
	if other := groups[FamilyOther]; !slices.Equal(other.Group.Members, []string{"empty", "map", "mixed"}) ||
		!slices.Equal(other.Group.ObjectKinds, []KindCount{{"ConfigMap", 2}, {"CustomResourceDefinition", 1}}) {
		t.Errorf("Other group = %+v", other.Group)
	}
	if !hasNode(g, "comp:mi/ns/x/crd") {
		t.Error("a lone CRD component was folded into a group of one object")
	}
}

// TestClaimIsNeverFolded: the component holding a TransformerRegistration
// stays an ordinary node; the other forms its group alone.
func TestClaimIsNeverFolded(t *testing.T) {
	in := detail(
		config("claim", reg("p.q"), cm("a")),
		config("two", cm("b"), cm("c")),
	)
	g := Instance(in, Options{})
	if !hasNode(g, "comp:mi/ns/x/claim") || nodeByID(t, g, "comp:mi/ns/x/claim").MemberOf != "" {
		t.Error("the component holding the TransformerRegistration was folded")
	}
	other := nodeByID(t, g, "grp:configuration/mi/ns/x/other")
	if !slices.Equal(other.Group.Members, []string{"two"}) {
		t.Errorf("Other group = %+v, want the other component alone", other.Group)
	}
	one := detail(config("claim", reg("p.q"), cm("a")), config("one", cm("b")))
	if g := Instance(one, Options{}); countKind(g, KindGroup) != 0 {
		t.Errorf("a family of one object formed a group: %d groups", countKind(g, KindGroup))
	}
}

// TestGroupHealthIsWorstOfMembers: a Degraded member makes the group
// Degraded and a member that is not live makes it not live.
func TestGroupHealthIsWorstOfMembers(t *testing.T) {
	bad := config("role-b", role("r2"))
	bad.Health = health.Summary{State: health.Degraded, Partial: true, Counts: health.Counts{Degraded: 1}}
	g := Instance(detail(config("role-a", role("r1")), bad), Options{})
	h := nodeByID(t, g, "grp:configuration/mi/ns/x/rbac").Health
	if h.State != health.Degraded || !h.Partial || !h.NotLive || h.Counts.Degraded != 1 || h.Counts.Healthy != 1 {
		t.Errorf("group health = %+v, want Degraded, partial, not live, 1 Degraded and 1 Healthy", h)
	}
}

// TestFamilyGroupsIgnoreInventoryOrder: the same groups from any order.
func TestFamilyGroupsIgnoreInventoryOrder(t *testing.T) {
	d := newCluster(t, f1(t), readmodeltest.AllowAll).instance(t, "cert-manager", "cert-manager")
	want := Instance(d, Options{})
	for _, g := range want.Nodes {
		if g.Kind == KindGroup && (!slices.IsSorted(g.Group.Members) || !slices.IsSortedFunc(g.Group.ObjectKinds, func(a, b KindCount) int { return compareStrings(a.Kind, b.Kind) })) {
			t.Errorf("group %s is not sorted: %+v", g.ID, g.Group)
		}
	}
	slices.Reverse(d.Components)
	got := Instance(d, Options{})
	if len(got.Nodes) != len(want.Nodes) {
		t.Fatalf("reversed inventory: %d nodes, want %d", len(got.Nodes), len(want.Nodes))
	}
	for i := range want.Nodes {
		if want.Nodes[i].ID != got.Nodes[i].ID || want.Nodes[i].Y != got.Nodes[i].Y {
			t.Errorf("node %d: %s at %d, want %s at %d", i, got.Nodes[i].ID, got.Nodes[i].Y, want.Nodes[i].ID, want.Nodes[i].Y)
		}
	}
}
