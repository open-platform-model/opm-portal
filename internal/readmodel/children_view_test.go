package readmodel

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/health"
)

// TestInventoryObjectsCarryTheirChildren: podinfo's Deployment carries its
// ReplicaSet and, through it, its two Pods, each with its direct owner.
func TestInventoryObjectsCarryTheirChildren(t *testing.T) {
	e := newEnv(t, loadF1(t), allowAll, allowAll)
	d := e.instance(t, "default", "podinfo")
	dep := objectNamed(t, &d, "Deployment", "podinfo-podinfo")
	if len(dep.Children) != 3 {
		t.Fatalf("Deployment children = %+v, want a ReplicaSet and two Pods", dep.Children)
	}
	pods, rs := dep.Children[:2], dep.Children[2]
	if rs.Ref.Kind != "ReplicaSet" || rs.Owner.Kind != "Deployment" || rs.Owner.Group != "apps" ||
		rs.Owner.Name != "podinfo-podinfo" || rs.Replicas == nil || *rs.Replicas != 2 {
		t.Errorf("ReplicaSet = %+v, want owned by the Deployment with 2 replicas", rs)
	}
	for _, p := range pods {
		if p.Ref.Kind != "Pod" || p.Owner.Kind != "ReplicaSet" || p.Owner.Name != rs.Ref.Name ||
			p.Health.State != health.Healthy || p.Replicas != nil {
			t.Errorf("Pod = %+v, want a Healthy Pod owned by %s", p, rs.Ref.Name)
		}
	}
	if svc := objectNamed(t, &d, "Service", "podinfo-podinfo"); len(svc.Children) != 0 {
		t.Errorf("Service children = %+v, want none", svc.Children)
	}
}

// TestChildrenUnreadLeaveNone: with Pods unreadable no object carries
// children and the Deployment says its children are unread.
func TestChildrenUnreadLeaveNone(t *testing.T) {
	e := newEnv(t, loadF1(t), denyResources("pods"), allowAll)
	d := e.instance(t, "default", "podinfo")
	for _, o := range objectsOf(&d) {
		if len(o.Children) != 0 {
			t.Errorf("%s %s children = %+v, want none", o.Ref.Kind, o.Ref.Name, o.Children)
		}
	}
	if dep := objectNamed(t, &d, "Deployment", "podinfo-podinfo"); !dep.ChildrenUnread {
		t.Error("Deployment does not say its children are unread")
	}
}

// TestChildOutsideTheChainIsLeftOut: a Pod labeled with the instance's
// name whose controller is no inventory object is attached to nothing.
func TestChildOutsideTheChainIsLeftOut(t *testing.T) {
	objs := loadF1(t)
	stray := find(t, objs, "Pod", "podinfo-podinfo-d9585d794-4lg6h").DeepCopy()
	stray.SetName("stray")
	stray.SetUID("stray-uid")
	refs := stray.GetOwnerReferences()
	refs[0].UID = "no-such-owner"
	stray.SetOwnerReferences(refs)
	e := newEnv(t, append(objs, stray), allowAll, allowAll)
	d := e.instance(t, "default", "podinfo")
	for _, o := range objectsOf(&d) {
		for _, c := range o.Children {
			if c.Ref.Name == "stray" {
				t.Fatalf("stray Pod attached to %s %s", o.Ref.Kind, o.Ref.Name)
			}
		}
	}
}

func TestPackageDependsOn(t *testing.T) {
	objs := loadF1(t)
	e := newEnv(t, objs, allowAll, allowAll)
	p, err := e.m.Package(t.Context(), alice, e.grant(t, "get", modulePackages, "pkg", "podinfo"), "pkg", "podinfo")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.DependsOn) != 0 || p.Source.Kind != "OCIRepository" || p.Source.Name != "podinfo-release" {
		t.Fatalf("package = %+v, want no dependencies and source OCIRepository podinfo-release", p.PackageItem)
	}

	withDeps := find(t, objs, "ModulePackage", "podinfo").DeepCopy()
	deps := []any{map[string]any{"name": "base"}, map[string]any{"name": "other", "namespace": "elsewhere"}}
	if err := unstructured.SetNestedSlice(withDeps.Object, deps, "spec", "dependsOn"); err != nil {
		t.Fatal(err)
	}
	got := packageItem(withDeps).DependsOn
	want := []ObjectRef{
		{Group: opmGroup, Version: opmVersion, Kind: "ModulePackage", Name: "base"},
		{Group: opmGroup, Version: opmVersion, Kind: "ModulePackage", Namespace: "elsewhere", Name: "other"},
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("DependsOn = %+v, want %+v", got, want)
	}
}
