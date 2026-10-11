package graph

import (
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

func titles(g Graph) []string {
	out := make([]string, len(g.Layout.Columns))
	for i, c := range g.Layout.Columns {
		out[i] = c.Title
	}
	return out
}

// TestGeometry: nodes are 216 x 72 on a 96 row pitch, and an elbow turns in
// the middle of the gutter.
func TestGeometry(t *testing.T) {
	g := Instance(newCluster(t, f1(t), readmodeltest.AllowAll).instance(t, "default", "podinfo"), Options{})
	if g.Layout.NodeWidth != 216 || g.Layout.NodeHeight != 72 {
		t.Errorf("node size = %d x %d, want 216 x 72", g.Layout.NodeWidth, g.Layout.NodeHeight)
	}
	for i := 1; i < len(g.Nodes); i++ {
		a, b := g.Nodes[i-1], g.Nodes[i]
		if a.Column == b.Column && b.Y-a.Y != rowPitch {
			t.Errorf("%s and %s are %d apart in one column, want %d", a.ID, b.ID, b.Y-a.Y, rowPitch)
		}
	}
	for _, e := range g.Edges {
		r := e.Route
		if d := r[1].X - r[0].X; d != (colPitch-nodeWidth)/2 && d != -(colPitch-nodeWidth)/2 {
			t.Errorf("edge %s corner is %d from its start, want %d, the middle of the gutter", e.ID, d, (colPitch-nodeWidth)/2)
		}
		if r[1].X != r[2].X || r[1].Y != r[0].Y || r[2].Y != r[3].Y {
			t.Errorf("edge %s route %v is not an elbow", e.ID, r)
		}
	}
}

// TestInstanceColumnTitles: runtime columns are titled from the kinds they
// hold.
func TestInstanceColumnTitles(t *testing.T) {
	g := Instance(newCluster(t, f1(t), readmodeltest.AllowAll).instance(t, "default", "podinfo"), Options{})
	if want := []string{"Module", "Instance", "Components", "Objects", "ReplicaSets", "Pods"}; !slices.Equal(titles(g), want) {
		t.Errorf("podinfo columns = %v, want %v", titles(g), want)
	}
}

// ref is a constructed object reference.
func ref(group, kind, name string) readmodel.ObjectRef {
	return readmodel.ObjectRef{Group: group, Version: "v1", Kind: kind, Namespace: "ns", Name: name}
}

func object(r readmodel.ObjectRef, children ...readmodel.RuntimeChild) readmodel.InventoryObject {
	return readmodel.InventoryObject{Ref: r, Access: health.AccessOK, Health: health.ObjectHealth{State: health.Healthy}, Children: children}
}

func detail(components ...readmodel.Component) readmodel.InstanceDetail {
	return readmodel.InstanceDetail{
		InstanceItem: readmodel.InstanceItem{Ref: ref("opmodel.dev", "ModuleInstance", "x")},
		Components:   components,
	}
}

func child(r, owner readmodel.ObjectRef) readmodel.RuntimeChild {
	return readmodel.RuntimeChild{Ref: r, Owner: owner, Health: health.ObjectHealth{State: health.Healthy}}
}

// TestRuntimeColumnTitles: a StatefulSet's Pods are depth 1, so that column
// reads Pods; a Deployment beside it makes the column mixed; a CronJob's
// Jobs read Jobs.
func TestRuntimeColumnTitles(t *testing.T) {
	sts := ref("apps", "StatefulSet", "db")
	dep := ref("apps", "Deployment", "web")
	cron := ref("batch", "CronJob", "nightly")
	rs := ref("apps", "ReplicaSet", "web-1")
	for _, tc := range []struct {
		name string
		in   readmodel.InstanceDetail
		want []string
	}{
		{"statefulset", detail(readmodel.Component{Name: "db", Objects: []readmodel.InventoryObject{
			object(sts, child(ref("", "Pod", "db-0"), sts), child(ref("", "Pod", "db-1"), sts)),
		}}), []string{"Module", "Instance", "Components", "Objects", "Pods"}},
		{"mixed", detail(readmodel.Component{Name: "db", Objects: []readmodel.InventoryObject{
			object(sts, child(ref("", "Pod", "db-0"), sts)),
		}}, readmodel.Component{Name: "web", Objects: []readmodel.InventoryObject{
			object(dep, child(rs, dep), child(ref("", "Pod", "web-1-a"), rs)),
		}}), []string{"Module", "Instance", "Components", "Objects", "Runtime", "Pods"}},
		{"cronjob", detail(readmodel.Component{Name: "job", Objects: []readmodel.InventoryObject{
			object(cron, child(ref("batch", "Job", "nightly-1"), cron)),
		}}), []string{"Module", "Instance", "Components", "Objects", "Jobs"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.in.Module = readmodel.ModuleRef{Path: "example.com/m@v0", Version: "v1"}
			if got := titles(Instance(tc.in, Options{})); !slices.Equal(got, tc.want) {
				t.Errorf("columns = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPackageColumns: a package graph starts at the package, its source and
// the packages it depends on come first in the components column, and every
// edge runs rightward.
func TestPackageColumns(t *testing.T) {
	g := Package(newCluster(t, f1(t), readmodeltest.AllowAll).pkg(t), Options{})
	if want := []string{"Package", "Source and components"}; !slices.Equal(titles(g), want) {
		t.Errorf("F1 package columns = %v, want %v", titles(g), want)
	}
	pkg := nodeByID(t, g, "mp:pkg/podinfo")
	src := nodeByID(t, g, "src:source.toolkit.fluxcd.io/OCIRepository/pkg/podinfo-release")
	if pkg.Column != 0 || src.Column != 1 {
		t.Errorf("package in column %d and source in column %d, want 0 and 1", pkg.Column, src.Column)
	}
	if e, ok := edgeBetween(g, EdgeSourcedFrom, pkg.ID, src.ID); !ok || e.Route[0].X >= e.Route[3].X {
		t.Errorf("sourcedFrom edge = %+v, want one that runs left to right", e)
	}
}

// TestPackageColumnsOrder: with a source, dependencies and a holder's
// components, the components column reads source, dependencies by id, then
// components, and every edge runs rightward.
func TestPackageColumnsOrder(t *testing.T) {
	objs := readmodeltest.WithPackageHolder(t, f1(t))
	pkgObj := readmodeltest.Find(t, objs, "ModulePackage", "provider")
	if err := setDependsOn(pkgObj, "base", "alpha"); err != nil {
		t.Fatal(err)
	}
	g := Package(newCluster(t, objs, readmodeltest.AllowAll).pkgNamed(t, "pkg", "provider"), Options{})
	var col []string
	for i := range g.Nodes {
		if g.Nodes[i].Column == 1 {
			col = append(col, g.Nodes[i].ID)
		}
	}
	if len(col) < 4 || col[0][:4] != "src:" || col[1] != "mp:pkg/alpha" || col[2] != "mp:pkg/base" || col[3][:5] != "comp:" {
		t.Errorf("components column = %v, want source, dependencies by id, then components", col)
	}
	for i := range g.Edges {
		if e := &g.Edges[i]; e.Route[0].X >= e.Route[3].X {
			t.Errorf("edge %s runs %d to %d, want rightward", e.ID, e.Route[0].X, e.Route[3].X)
		}
	}
}

func setDependsOn(pkg *unstructured.Unstructured, names ...string) error {
	deps := make([]any, 0, len(names))
	for _, n := range names {
		deps = append(deps, map[string]any{"name": n})
	}
	return unstructured.SetNestedSlice(pkg.Object, deps, "spec", "dependsOn")
}
