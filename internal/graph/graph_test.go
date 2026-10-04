package graph

import (
	"reflect"
	"slices"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

func TestIDs(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{catalogID("opmodel.dev/catalogs/opm@v4"), "cat:opmodel.dev%2Fcatalogs%2Fopm%40v4"},
		{objectID(readmodel.ObjectRef{Kind: "Namespace", Name: "cert-manager"}), "obj:_/Namespace/_/cert-manager"},
		{objectID(readmodel.ObjectRef{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "a:b"}), "obj:rbac.authorization.k8s.io/ClusterRole/_/a%3Ab"},
		{componentID(instanceID("default", "podinfo"), ""), "comp:mi/default/podinfo/_"},
		{componentID(instanceID("default", "podinfo"), "_"), "comp:mi/default/podinfo/%5F"},
		{groupID(GroupConfiguration, instanceID("cert-manager", "cert-manager")), "grp:configuration/mi/cert-manager/cert-manager"},
		{moduleID("opmodel.dev/modules/cert_manager@v2", "v2.0.5"), "mod:opmodel.dev%2Fmodules%2Fcert_manager%40v2/v2.0.5"},
		{registrationID("default.backup-provider"), "treg:default.backup-provider"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("id = %q, want %q", c.got, c.want)
		}
	}
}

// TestIDsSurviveARecreate: a Deployment recreated with a new UID keeps its
// node id, and so does every other node (0030:D4:R6).
func TestIDsSurviveARecreate(t *testing.T) {
	objs := f1(t)
	before := Instance(newCluster(t, objs, readmodeltest.AllowAll).instance(t, "default", "podinfo"), Options{})
	dep := readmodeltest.Find(t, objs, "Deployment", "podinfo-podinfo")
	dep.SetUID(types.UID("recreated"))
	// The recreated Deployment's ReplicaSet points at the new UID.
	rs := readmodeltest.Find(t, objs, "ReplicaSet", "podinfo-podinfo-d9585d794")
	refs := rs.GetOwnerReferences()
	refs[0].UID = dep.GetUID()
	rs.SetOwnerReferences(refs)
	after := Instance(newCluster(t, objs, readmodeltest.AllowAll).instance(t, "default", "podinfo"), Options{})
	ids := func(g Graph) []string {
		out := make([]string, 0, len(g.Nodes))
		for i := range g.Nodes {
			out = append(out, g.Nodes[i].ID)
		}
		return out
	}
	if !hasNode(after, "obj:apps/Deployment/default/podinfo-podinfo") || !slices.Equal(ids(before), ids(after)) {
		t.Fatalf("ids changed with the UID:\nbefore %v\nafter  %v", ids(before), ids(after))
	}
}

func TestPodinfoGraph(t *testing.T) {
	d := newCluster(t, f1(t), readmodeltest.AllowAll).instance(t, "default", "podinfo")
	g := Instance(d, Options{})
	root := nodeByID(t, g, "mi:default/podinfo")
	if g.Root != root.ID || root.Applied.State != health.AppliedStateApplied || root.Health.State != health.Healthy || len(root.RenderContracts) != 7 {
		t.Fatalf("root = %+v, want Applied, Healthy, 7 render contracts", root)
	}
	dep := "obj:apps/Deployment/default/podinfo-podinfo"
	rs := "obj:apps/ReplicaSet/default/podinfo-podinfo-d9585d794"
	for _, want := range []struct {
		kind     EdgeKind
		from, to string
	}{
		{EdgeInstantiates, root.ID, "mod:testing.opmodel.dev%2Fmodules%2Foperator%2Fpodinfo%40v0/v0.1.11"},
		{EdgeHasComponent, root.ID, "comp:mi/default/podinfo/podinfo"},
		{EdgeOwns, "comp:mi/default/podinfo/podinfo", dep},
		{EdgeOwns, "comp:mi/default/podinfo/podinfo", "obj:_/Service/default/podinfo-podinfo"},
		{EdgeControls, dep, rs},
		{EdgeControls, rs, "obj:_/Pod/default/podinfo-podinfo-d9585d794-4lg6h"},
		{EdgeControls, rs, "obj:_/Pod/default/podinfo-podinfo-d9585d794-kp2jx"},
	} {
		if _, ok := edgeBetween(g, want.kind, want.from, want.to); !ok {
			t.Errorf("no %s edge %s -> %s", want.kind, want.from, want.to)
		}
	}
	if len(g.Nodes) != 8 || len(g.Edges) != 7 {
		t.Errorf("%d nodes and %d edges, want 8 and 7", len(g.Nodes), len(g.Edges))
	}
	if n := nodeByID(t, g, rs); n.Replicas == nil || *n.Replicas != 2 || n.Kind != KindRuntime {
		t.Errorf("ReplicaSet node = %+v, want a runtime node with 2 replicas", n)
	}
}

// TestEdgesNameTheirSource: every edge of every F1 graph has a known kind
// and names that kind's source, and no node is a contract (0030:D4:R1/R3).
func TestEdgesNameTheirSource(t *testing.T) {
	c := newCluster(t, f1(t), readmodeltest.AllowAll)
	graphs := []Graph{
		Instance(c.instance(t, "cert-manager", "cert-manager"), Options{Expand: []string{"grp:configuration/mi/cert-manager/cert-manager"}}),
		Instance(c.instance(t, "web", "web"), Options{}),
		Package(c.pkg(t), Options{}),
		Platform(c.platform(t), Options{}),
	}
	for _, g := range graphs {
		for _, e := range g.Edges {
			if src, ok := Sources[e.Kind]; !ok || e.Source != src {
				t.Errorf("%s: edge %s source %q", g.Root, e.ID, e.Source)
			}
			if !hasNode(g, e.From) || !hasNode(g, e.To) {
				t.Errorf("%s: edge %s has a missing end", g.Root, e.ID)
			}
		}
		for _, n := range g.Nodes {
			if n.Ref != nil && n.Ref.Kind == "Contract" {
				t.Errorf("%s: contract node %s", g.Root, n.ID)
			}
		}
	}
	instanceKinds := []EdgeKind{EdgeInstantiates, EdgeHasComponent, EdgeOwns, EdgeControls}
	for _, e := range graphs[0].Edges {
		if !slices.Contains(instanceKinds, e.Kind) {
			t.Errorf("cert-manager edge %s has kind %s", e.ID, e.Kind)
		}
	}
	cm := nodeByID(t, graphs[0], "mi:cert-manager/cert-manager")
	if len(cm.RenderContracts) != 15 {
		t.Errorf("cert-manager render contracts = %d, want 15", len(cm.RenderContracts))
	}
}

func TestCLIOwnedInstance(t *testing.T) {
	g := Instance(newCluster(t, f1(t), readmodeltest.AllowAll).instance(t, "web", "web"), Options{})
	root := nodeByID(t, g, "mi:web/web")
	if root.Owner != "cli" || root.Applied.State != health.AppliedStateManagedExternally || root.Health.State != health.Healthy {
		t.Fatalf("root = %+v, want cli-owned, managed externally, healthy", root)
	}
	if countKind(g, KindObject) != 2 {
		t.Errorf("objects = %d, want 2", countKind(g, KindObject))
	}
}

func TestCertManagerCollapsed(t *testing.T) {
	g := Instance(newCluster(t, f1(t), readmodeltest.AllowAll).instance(t, "cert-manager", "cert-manager"), Options{})
	group := nodeByID(t, g, "grp:configuration/mi/cert-manager/cert-manager")
	if group.Group == nil || len(group.Group.Members) != 17 || group.Health.State != health.Healthy || group.Label != "17 configuration components" {
		t.Fatalf("group = %+v, want 17 healthy configuration components", group)
	}
	if group.Health.Counts.Healthy != 35 {
		t.Errorf("group counts = %+v, want 35 healthy objects", group.Health.Counts)
	}
	var comps []string
	for _, n := range g.Nodes {
		if n.Kind == KindComponent {
			comps = append(comps, n.Label)
		}
	}
	slices.Sort(comps)
	if !slices.Equal(comps, []string{"cainjector", "controller", "webhook"}) {
		t.Errorf("components = %v, want the three workload components", comps)
	}
	if len(g.Nodes) != 19 {
		t.Errorf("%d nodes, want 19", len(g.Nodes))
	}
	if !slices.IsSorted(group.Group.Members) {
		t.Errorf("members %v are not sorted", group.Group.Members)
	}
}

// TestGroupIgnoresInventoryOrder: the operator may reorder
// status.inventory; the graph stays the same.
func TestGroupIgnoresInventoryOrder(t *testing.T) {
	d := newCluster(t, f1(t), readmodeltest.AllowAll).instance(t, "cert-manager", "cert-manager")
	want := Instance(d, Options{})
	slices.Reverse(d.Components)
	if got := Instance(d, Options{}); !reflect.DeepEqual(got, want) {
		t.Error("reversing the components changed the graph")
	}
}

func TestCertManagerExpanded(t *testing.T) {
	gid := "grp:configuration/mi/cert-manager/cert-manager"
	g := Instance(newCluster(t, f1(t), readmodeltest.AllowAll).instance(t, "cert-manager", "cert-manager"), Options{Expand: []string{gid}})
	if hasNode(g, gid) || countKind(g, KindComponent) != 20 || countKind(g, KindObject) != 42 {
		t.Fatalf("components %d, objects %d, group shown %v; want 20, 42, no group", countKind(g, KindComponent), countKind(g, KindObject), hasNode(g, gid))
	}
	members := 0
	for _, n := range g.Nodes {
		if n.MemberOf == gid {
			members++
		}
	}
	if members != 17 {
		t.Errorf("%d nodes say they belong to the group, want 17", members)
	}
}

func TestForbiddenObjects(t *testing.T) {
	c := newCluster(t, f1(t), readmodeltest.DenyResources("clusterroles"))
	gid := "grp:configuration/mi/cert-manager/cert-manager"
	d := c.instance(t, "cert-manager", "cert-manager")
	if group := nodeByID(t, Instance(d, Options{}), gid); !group.Health.Partial || group.Health.Counts.Forbidden != 10 {
		t.Errorf("group = %+v, want partial with 10 forbidden", group.Health)
	}
	n := nodeByID(t, Instance(d, Options{Expand: []string{gid}}), "obj:rbac.authorization.k8s.io/ClusterRole/_/cert-manager-cainjector")
	if n.Access != health.AccessForbidden || n.Health != nil {
		t.Errorf("ClusterRole = %+v, want forbidden with no health", n)
	}
}

// TestBrokenRollout: experiment 01's image break, one minute in.
func TestBrokenRollout(t *testing.T) {
	g := Instance(newCluster(t, f1Broken(t), readmodeltest.AllowAll).instance(t, "default", "podinfo"), Options{})
	root := nodeByID(t, g, "mi:default/podinfo")
	if root.Health.State != health.Degraded || root.Applied.State != health.AppliedStateApplied {
		t.Fatalf("root health %s applied %s, want Degraded and Applied", root.Health.State, root.Applied.State)
	}
	dep := "obj:apps/Deployment/default/podinfo-podinfo"
	newRS := "obj:apps/ReplicaSet/default/podinfo-podinfo-794bd8c7fb"
	broken := "obj:_/Pod/default/podinfo-podinfo-794bd8c7fb-cc5ql"
	for _, rs := range []string{newRS, "obj:apps/ReplicaSet/default/podinfo-podinfo-d9585d794"} {
		if _, ok := edgeBetween(g, EdgeControls, dep, rs); !ok {
			t.Errorf("Deployment does not control %s", rs)
		}
	}
	if _, ok := edgeBetween(g, EdgeControls, newRS, broken); !ok {
		t.Errorf("new ReplicaSet does not control the broken Pod")
	}
	if p := nodeByID(t, g, broken); p.Health.State != health.Degraded || p.Health.Reason != "ImagePullBackOff" {
		t.Errorf("broken Pod = %+v, want Degraded ImagePullBackOff", p.Health)
	}
	if n := nodeByID(t, g, dep); n.Health.State != health.Degraded {
		t.Errorf("Deployment = %+v, want Degraded", n.Health)
	}
}

// withScaledDownReplicaSet adds an old ReplicaSet of podinfo's Deployment with zero
// replicas, as a finished rollout leaves it.
func withScaledDownReplicaSet(t *testing.T) []*unstructured.Unstructured {
	objs := f1(t)
	old := readmodeltest.Find(t, objs, "ReplicaSet", "podinfo-podinfo-d9585d794").DeepCopy()
	old.SetName("podinfo-podinfo-old")
	old.SetUID("old-rs")
	if err := unstructured.SetNestedField(old.Object, int64(0), "spec", "replicas"); err != nil {
		t.Fatal(err)
	}
	return append(objs, old)
}

func TestScaledDownReplicaSetHidden(t *testing.T) {
	d := newCluster(t, withScaledDownReplicaSet(t), readmodeltest.AllowAll).instance(t, "default", "podinfo")
	old := "obj:apps/ReplicaSet/default/podinfo-podinfo-old"
	dep := "obj:apps/Deployment/default/podinfo-podinfo"
	g := Instance(d, Options{})
	if hasNode(g, old) || nodeByID(t, g, dep).HiddenScaledDown != 1 {
		t.Fatalf("scaled-down ReplicaSet shown %v, hidden count %d; want hidden, 1", hasNode(g, old), nodeByID(t, g, dep).HiddenScaledDown)
	}
	g = Instance(d, Options{ShowScaledDown: true})
	if _, ok := edgeBetween(g, EdgeControls, dep, old); !ok || nodeByID(t, g, dep).HiddenScaledDown != 0 {
		t.Fatal("ShowScaledDown did not show the ReplicaSet")
	}
}

func TestManyPodsGrouped(t *testing.T) {
	objs := f1(t)
	pod := readmodeltest.Find(t, objs, "Pod", "podinfo-podinfo-d9585d794-4lg6h")
	for _, s := range []string{"a", "b", "c", "d", "e"} {
		extra := pod.DeepCopy()
		extra.SetName("podinfo-podinfo-d9585d794-" + s)
		extra.SetUID(types.UID("extra-" + s))
		objs = append(objs, extra)
	}
	d := newCluster(t, objs, readmodeltest.AllowAll).instance(t, "default", "podinfo")
	rs := "obj:apps/ReplicaSet/default/podinfo-podinfo-d9585d794"
	gid := "grp:pods/obj/apps/ReplicaSet/default/podinfo-podinfo-d9585d794"
	g := Instance(d, Options{})
	group := nodeByID(t, g, gid)
	if group.Label != "7 Pods" || group.Health.Counts.Healthy != 7 || countKind(g, KindRuntime) != 1 {
		t.Fatalf("group = %+v, runtime nodes %d; want 7 healthy Pods grouped beside the ReplicaSet", group, countKind(g, KindRuntime))
	}
	if _, ok := edgeBetween(g, EdgeControls, rs, gid); !ok {
		t.Error("ReplicaSet does not control the group")
	}
	g = Instance(d, Options{Expand: []string{gid}})
	if hasNode(g, gid) || countKind(g, KindRuntime) != 8 {
		t.Fatalf("expanded: group shown %v, runtime nodes %d; want 8 runtime nodes", hasNode(g, gid), countKind(g, KindRuntime))
	}
}

func TestNodeCap(t *testing.T) {
	gid := "grp:configuration/mi/cert-manager/cert-manager"
	d := newCluster(t, f1(t), readmodeltest.AllowAll).instance(t, "cert-manager", "cert-manager")
	full := Instance(d, Options{Expand: []string{gid}})
	g := Instance(d, Options{Expand: []string{gid}, NodeCap: 30})
	if len(g.Nodes) != 30 {
		t.Fatalf("%d nodes, want 30", len(g.Nodes))
	}
	more := nodeByID(t, g, "grp:more/mi/cert-manager/cert-manager")
	hidden := 0
	for _, k := range more.Group.Hidden {
		hidden += k.Count
	}
	if hidden != len(full.Nodes)-29 {
		t.Errorf("summary hides %d, want %d", hidden, len(full.Nodes)-29)
	}
	for _, e := range g.Edges {
		if !hasNode(g, e.From) || !hasNode(g, e.To) {
			t.Errorf("edge %s points at a dropped node", e.ID)
		}
	}
	if !hasNode(g, "mi:cert-manager/cert-manager") {
		t.Error("the cap dropped the root")
	}
	for _, c := range []int{1, 2} {
		if tiny := Instance(d, Options{NodeCap: c}); len(tiny.Nodes) != 2 || !hasNode(tiny, "mi:cert-manager/cert-manager") {
			t.Errorf("cap %d kept %d nodes, root kept %v; want the root and the summary", c, len(tiny.Nodes), hasNode(tiny, "mi:cert-manager/cert-manager"))
		}
	}
}

// TestNodeCapKeepsHealth: a cap that drops the broken rollout's Pods shows
// their worst state on the summary, so the graph still says where the
// instance is Degraded (constitution principle IV).
func TestNodeCapKeepsHealth(t *testing.T) {
	d := newCluster(t, f1Broken(t), readmodeltest.AllowAll).instance(t, "default", "podinfo")
	full := Instance(d, Options{})
	last, pods := 0, 0
	for i := range full.Nodes {
		last = max(last, full.Nodes[i].Column)
	}
	for i := range full.Nodes {
		if full.Nodes[i].Column == last {
			if full.Nodes[i].Ref == nil || full.Nodes[i].Ref.Kind != kindPod {
				t.Fatalf("last column holds %s, want only Pods", full.Nodes[i].ID)
			}
			pods++
		}
	}
	g := Instance(d, Options{NodeCap: len(full.Nodes) - pods + 1})
	more := nodeByID(t, g, "grp:more/mi/default/podinfo")
	if hasNode(g, "obj:_/Pod/default/podinfo-podinfo-794bd8c7fb-cc5ql") {
		t.Fatal("the cap kept the broken Pod")
	}
	if h := more.Health; h == nil || h.State != health.Degraded || h.Counts.Degraded != 1 || h.Counts.Healthy != pods-1 {
		t.Errorf("summary health = %+v, want Degraded with 1 Degraded and %d Healthy", h, pods-1)
	}
}

func TestPackageGraph(t *testing.T) {
	g := Package(newCluster(t, f1(t), readmodeltest.AllowAll).pkg(t), Options{})
	root := nodeByID(t, g, "mp:pkg/podinfo")
	if root.Applied.State != health.AppliedStateFailed || root.Applied.Reason != "SourceNotReady" {
		t.Errorf("root applied = %+v, want Failed SourceNotReady", root.Applied)
	}
	src := "src:source.toolkit.fluxcd.io/OCIRepository/pkg/podinfo-release"
	if _, ok := edgeBetween(g, EdgeSourcedFrom, root.ID, src); !ok || len(g.Nodes) != 2 {
		t.Fatalf("nodes %+v, want the package and its source", g.Nodes)
	}
}

// TestForeignNamespaceDependency: the operator refuses a dependsOn entry in
// another namespace, so its edge is drawn unverified; a same-namespace one
// is not marked.
func TestForeignNamespaceDependency(t *testing.T) {
	objs := f1(t)
	pkg := readmodeltest.Find(t, objs, "ModulePackage", "podinfo")
	deps := []any{
		map[string]any{"name": "base"},
		map[string]any{"name": "shared", "namespace": "other"},
	}
	if err := unstructured.SetNestedSlice(pkg.Object, deps, "spec", "dependsOn"); err != nil {
		t.Fatal(err)
	}
	g := Package(newCluster(t, objs, readmodeltest.AllowAll).pkg(t), Options{})
	if e, ok := edgeBetween(g, EdgeDependsOn, "mp:pkg/podinfo", "mp:pkg/base"); !ok || e.Verified != nil {
		t.Errorf("same-namespace edge = %+v, want drawn and unmarked", e)
	}
	e, ok := edgeBetween(g, EdgeDependsOn, "mp:pkg/podinfo", "mp:other/shared")
	if !ok || e.Verified == nil || *e.Verified || e.Reason != ReasonForeignNamespace {
		t.Errorf("foreign edge = %+v, want unverified ForeignNamespace", e)
	}
}

func TestPlatformGraph(t *testing.T) {
	g := Platform(newCluster(t, f1(t), readmodeltest.AllowAll).platform(t), Options{})
	plat := "plat:cluster"
	opm := "cat:opmodel.dev%2Fcatalogs%2Fopm%40v4"
	backup := "cat:testing.opmodel.dev%2Fcatalogs%2Foperator%2Fbackup%40v0"
	accepted := "treg:default.backup-provider"
	refused := "treg:default.refused-claim-fixture"
	for _, cat := range []string{opm, backup} {
		if _, ok := edgeBetween(g, EdgeResolves, plat, cat); !ok {
			t.Errorf("platform does not resolve %s", cat)
		}
	}
	if _, ok := edgeBetween(g, EdgeContributes, accepted, backup); !ok {
		t.Error("the accepted claim does not contribute its catalog")
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeContributes && e.From == refused {
			t.Errorf("the refused claim contributes: %s", e.ID)
		}
	}
	if len(g.Nodes) != 7 {
		t.Errorf("%d nodes, want 7", len(g.Nodes))
	}
}

// TestContributesNeedsTheContributingClaim: the registry entry does not
// name its registration, so a claim on an already contributed catalog that
// is refused as a duplicate, accepted but not active, or at another version
// gets no contributes edge (0030:D4).
func TestContributesNeedsTheContributingClaim(t *testing.T) {
	backup := "cat:testing.opmodel.dev%2Fcatalogs%2Foperator%2Fbackup%40v0"
	for _, tt := range []struct {
		name             string
		accepted, active bool
		version          string
	}{
		{name: "duplicate", version: "0.1.0"},
		{name: "pending", accepted: true, version: "0.1.0"},
		{name: "other-version", accepted: true, active: true, version: "0.2.0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			objs := f1(t)
			claim := readmodeltest.Find(t, objs, "TransformerRegistration", "default.backup-provider").DeepCopy()
			claim.SetName("default." + tt.name)
			claim.SetUID(types.UID(tt.name))
			for _, set := range []struct {
				v    any
				path []string
			}{
				{tt.accepted, []string{"status", "accepted"}},
				{tt.active, []string{"status", "active"}},
				{tt.version, []string{"spec", "version"}},
			} {
				if err := unstructured.SetNestedField(claim.Object, set.v, set.path...); err != nil {
					t.Fatal(err)
				}
			}
			g := Platform(newCluster(t, append(objs, claim), readmodeltest.AllowAll).platform(t), Options{})
			if e, ok := edgeBetween(g, EdgeContributes, "treg:default."+tt.name, backup); ok {
				t.Errorf("%s claim contributes: %s", tt.name, e.ID)
			}
			if _, ok := edgeBetween(g, EdgeContributes, "treg:default.backup-provider", backup); !ok {
				t.Error("the contributing claim lost its edge")
			}
		})
	}
}

// TestProviderEdges: the accepted claim's provider holds it; the refused
// claim names an instance that does not exist (0030:D4:R2).
func TestProviderEdges(t *testing.T) {
	g := Platform(newCluster(t, f1(t), readmodeltest.AllowAll).platform(t), Options{})
	accepted := "treg:default.backup-provider"
	refused := "treg:default.refused-claim-fixture"
	if e, ok := edgeBetween(g, EdgeProvidedBy, accepted, "mi:default/backup-provider"); !ok || e.Verified == nil || !*e.Verified {
		t.Errorf("accepted claim's provider edge = %+v, want verified", e)
	}
	if e, ok := edgeBetween(g, EdgeProvidedBy, refused, "mi:default/refused-claim-fixture"); !ok || e.Verified == nil || *e.Verified || e.Reason != ReasonProviderNotFound {
		t.Errorf("refused claim's provider edge = %+v, want unverified ProviderNotFound", e)
	}
	if n := nodeByID(t, g, "mi:default/refused-claim-fixture"); !n.Missing || n.Health != nil {
		t.Errorf("missing provider = %+v, want missing with no health", n)
	}
}

// TestRegistrationStanding: acceptance, activation and verdict are
// separate values, read from the status fields (0030:D4:R4).
func TestRegistrationStanding(t *testing.T) {
	g := Platform(newCluster(t, f1(t), readmodeltest.AllowAll).platform(t), Options{})
	accepted := "treg:default.backup-provider"
	refused := "treg:default.refused-claim-fixture"
	a := nodeByID(t, g, accepted).Registration
	if !a.Accepted || !a.Active || a.Verdict != health.VerdictAccepted {
		t.Errorf("accepted claim = %+v", a)
	}
	r := nodeByID(t, g, refused).Registration
	if r.Accepted || r.Active || r.Verdict != health.VerdictRefused || r.Reason != "CatalogUnresolved" || r.Message == "" ||
		r.Catalog != "testing.opmodel.dev/catalogs/operator/refused-claim-fixture-absent@v0" {
		t.Errorf("refused claim = %+v", r)
	}
}

// TestRemovalBlockedIsNotRefused: experiment 01's accepted, active claim
// whose deletion is blocked by dependents keeps showing as accepted and
// active, with verdict RemovalBlocked (0030:D4:R7).
func TestRemovalBlockedIsNotRefused(t *testing.T) {
	blocked := readmodeltest.LoadList(t, "../health/testdata/treg-removal-blocked.yaml")
	var objs []*unstructured.Unstructured
	for _, o := range f1(t) {
		if o.GetKind() != "TransformerRegistration" || o.GetName() != "default.backup-provider" {
			objs = append(objs, o)
		}
	}
	g := Platform(newCluster(t, append(objs, blocked...), readmodeltest.AllowAll).platform(t), Options{})
	r := nodeByID(t, g, "treg:default.backup-provider").Registration
	if !r.Accepted || !r.Active || r.Verdict != health.VerdictRemovalBlocked || r.Reason != "DependentsRemain" {
		t.Errorf("blocked claim = %+v, want accepted, active, RemovalBlocked", r)
	}
}

func TestPlatformProviderForbidden(t *testing.T) {
	deny := func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return ra.Resource != "moduleinstances" || ra.Name != "backup-provider"
	}
	g := Platform(newCluster(t, f1(t), deny).platform(t), Options{})
	e, ok := edgeBetween(g, EdgeProvidedBy, "treg:default.backup-provider", "mi:default/backup-provider")
	if !ok || e.Verified == nil || *e.Verified || e.Reason != ReasonProviderUnreadable {
		t.Errorf("edge = %+v, want unverified ProviderUnreadable", e)
	}
	if n := nodeByID(t, g, "mi:default/backup-provider"); n.Access != health.AccessForbidden || n.Health != nil {
		t.Errorf("provider = %+v, want forbidden", n)
	}
}

// TestProviderNotLookedUp: without a lookup the graph says so instead of
// guessing the provider unreadable.
func TestProviderNotLookedUp(t *testing.T) {
	in := newCluster(t, f1(t), readmodeltest.AllowAll).platform(t)
	in.Providers = nil
	g := Platform(in, Options{})
	e, ok := edgeBetween(g, EdgeProvidedBy, "treg:default.backup-provider", "mi:default/backup-provider")
	if !ok || e.Verified == nil || *e.Verified || e.Reason != ReasonProviderNotLookedUp {
		t.Errorf("edge = %+v, want unverified ProviderNotLookedUp", e)
	}
	if n := nodeByID(t, g, "mi:default/backup-provider"); n.Access != "" || n.Missing || n.Health != nil {
		t.Errorf("provider = %+v, want no access, not missing, no health", n)
	}
}

// TestProviderNotHoldingTheClaim: a registration naming an instance whose
// inventory does not hold it is drawn unverified (0030:D4:R2).
func TestProviderNotHoldingTheClaim(t *testing.T) {
	objs := f1(t)
	claim := readmodeltest.Find(t, objs, "TransformerRegistration", "default.refused-claim-fixture")
	if err := unstructured.SetNestedField(claim.Object, "podinfo", "spec", "providerRef", "name"); err != nil {
		t.Fatal(err)
	}
	g := Platform(newCluster(t, objs, readmodeltest.AllowAll).platform(t), Options{})
	e, ok := edgeBetween(g, EdgeProvidedBy, "treg:default.refused-claim-fixture", "mi:default/podinfo")
	if !ok || e.Verified == nil || *e.Verified || e.Reason != ReasonNotInProviderInventory {
		t.Errorf("edge = %+v, want unverified NotInProviderInventory", e)
	}
}

func TestPlatformRegistrationsForbidden(t *testing.T) {
	g := Platform(newCluster(t, f1(t), readmodeltest.DenyResources("transformerregistrations")).platform(t), Options{})
	if countKind(g, KindRegistration) != 0 || countKind(g, KindCatalog) != 2 {
		t.Fatalf("registrations %d, catalogs %d; want 0 and 2", countKind(g, KindRegistration), countKind(g, KindCatalog))
	}
	if p := nodeByID(t, g, "plat:cluster"); p.Platform.RegistrationsAccess != health.AccessForbidden {
		t.Errorf("platform = %+v, want registrations forbidden", p.Platform)
	}
}

// TestLayoutOrdersByBarycenter: a node follows its neighbors' rows, ties
// break by id, and two builds are identical.
func TestLayoutOrdersByBarycenter(t *testing.T) {
	build := func() Graph {
		b := newBuilder(ScopeInstance, instanceTitles, Options{})
		b.root = "a:1"
		for _, id := range []string{"a:1", "a:2"} {
			b.add(Node{ID: id}, 0)
		}
		for _, id := range []string{"b:x", "b:y", "b:z"} {
			b.add(Node{ID: id}, 2)
		}
		b.edge(EdgeOwns, "a:2", "b:x")
		b.edge(EdgeOwns, "a:1", "b:y")
		b.edge(EdgeOwns, "a:1", "b:z")
		return b.finish()
	}
	g := build()
	order := make([]string, 0, len(g.Nodes))
	for i := range g.Nodes {
		order = append(order, g.Nodes[i].ID)
	}
	if want := []string{"a:1", "a:2", "b:y", "b:z", "b:x"}; !slices.Equal(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	if len(g.Layout.Columns) != 2 || g.Layout.Columns[1].X != pad+colPitch || g.Nodes[2].Column != 1 {
		t.Errorf("columns = %+v, want two, the empty one compacted", g.Layout.Columns)
	}
	e, _ := edgeBetween(g, EdgeOwns, "a:2", "b:x")
	if len(e.Route) != 4 || e.Route[0].X != pad+nodeWidth || e.Route[3].X != pad+colPitch {
		t.Errorf("route = %+v", e.Route)
	}
	if !reflect.DeepEqual(g, build()) {
		t.Error("two builds differ")
	}
}

// TestContributor: the read API names the contributing registration by the
// rule the contributes edge follows.
func TestContributor(t *testing.T) {
	p := newCluster(t, f1(t), readmodeltest.AllowAll).platform(t).Platform
	got := map[string]string{}
	for _, c := range p.Catalogs {
		got[c.Catalog] = Contributor(c, p.Registrations)
	}
	want := map[string]string{
		"opmodel.dev/catalogs/opm@v4":                     "",
		"testing.opmodel.dev/catalogs/operator/backup@v0": "default.backup-provider",
	}
	if len(got) != len(want) {
		t.Fatalf("catalogs = %v", got)
	}
	for c, reg := range want {
		if got[c] != reg {
			t.Errorf("Contributor(%s) = %q, want %q", c, got[c], reg)
		}
	}
}
