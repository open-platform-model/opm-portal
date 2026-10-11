package graph

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/open-platform-model/opm-portal/internal/readmodel/readmodeltest"
)

const cmOwner = "grp:configuration/mi/cert-manager/cert-manager/"

var (
	cmCRDs     = cmOwner + "crds"
	cmRBAC     = cmOwner + "rbac"
	cmWebhooks = cmOwner + "webhooks"
)

// rect is a closed rectangle in layout units.
type rect struct {
	id             string
	x0, y0, x1, y1 int
}

func (r rect) meets(o rect) bool {
	return r.x0 <= o.x1 && o.x0 <= r.x1 && r.y0 <= o.y1 && o.y0 <= r.y1
}

// bandMembers returns, for each expanded configuration group, the ids of
// its member components and of every node hanging from them.
func bandMembers(g Graph) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for i := range g.Nodes {
		if n := &g.Nodes[i]; n.Kind == KindComponent && n.MemberOf != "" {
			if out[n.MemberOf] == nil {
				out[n.MemberOf] = map[string]bool{}
			}
			out[n.MemberOf][n.ID] = true
		}
	}
	for _, set := range out {
		for grew := true; grew; {
			grew = false
			for i := range g.Edges {
				if e := &g.Edges[i]; (e.Kind == EdgeOwns || e.Kind == EdgeControls) && set[e.From] && !set[e.To] {
					set[e.To], grew = true, true
				}
			}
		}
	}
	return out
}

// bandRects is the frame a band would be drawn with: from frameOverhang
// left of the components column to frameOverhang right of the last column,
// bandHeader above the first row to bandFooter below the last.
func bandRects(g Graph) []rect {
	out := make([]rect, 0, 4)
	last := g.Layout.Columns[len(g.Layout.Columns)-1].X + g.Layout.NodeWidth
	for id, set := range bandMembers(g) {
		r := rect{id: id, x0: 1 << 30, y0: 1 << 30, x1: last + frameOverhang}
		for i := range g.Nodes {
			n := &g.Nodes[i]
			if !set[n.ID] {
				continue
			}
			r.x0 = min(r.x0, n.X-frameOverhang)
			r.y0 = min(r.y0, n.Y-bandHeader)
			r.y1 = max(r.y1, n.Y+g.Layout.NodeHeight+bandFooter)
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b rect) int { return compareStrings(a.id, b.id) })
	return out
}

// crosses reports whether an elbow route has a run inside the rectangle.
func crosses(r []Point, f rect) bool {
	runs := []rect{
		{x0: min(r[0].X, r[1].X), x1: max(r[0].X, r[1].X), y0: r[0].Y, y1: r[0].Y},
		{x0: r[1].X, x1: r[1].X, y0: min(r[0].Y, r[3].Y), y1: max(r[0].Y, r[3].Y)},
		{x0: min(r[1].X, r[3].X), x1: max(r[1].X, r[3].X), y0: r[3].Y, y1: r[3].Y},
	}
	return slices.ContainsFunc(runs, f.meets)
}

// checkBands holds a laid-out graph to the band invariants: no node but a
// band's own lies inside its frame, no edge between two nodes outside it
// crosses the frame, no two frames overlap, and no two nodes overlap.
func checkBands(t *testing.T, g Graph) {
	t.Helper()
	members := bandMembers(g)
	frames := bandRects(g)
	box := func(n *Node) rect {
		return rect{x0: n.X, y0: n.Y, x1: n.X + g.Layout.NodeWidth, y1: n.Y + g.Layout.NodeHeight}
	}
	for i, f := range frames {
		for j := range g.Nodes {
			if n := &g.Nodes[j]; !members[f.id][n.ID] && box(n).meets(f) {
				t.Errorf("node %s lies inside the frame of %s", n.ID, f.id)
			}
		}
		for _, o := range frames[i+1:] {
			if f.meets(o) {
				t.Errorf("frames %s and %s overlap", f.id, o.id)
			}
		}
		for j := range g.Edges {
			if e := &g.Edges[j]; !members[f.id][e.From] && !members[f.id][e.To] && crosses(e.Route, f) {
				t.Errorf("edge %s crosses the frame of %s", e.ID, f.id)
			}
		}
	}
	for i := range g.Nodes {
		for j := i + 1; j < len(g.Nodes); j++ {
			if box(&g.Nodes[i]).meets(box(&g.Nodes[j])) {
				t.Errorf("nodes %s and %s overlap", g.Nodes[i].ID, g.Nodes[j].ID)
			}
		}
	}
}

// TestBandInvariants: for each family expanded alone, for pairs and for all
// three, the band holds only its group and nothing crosses it; two builds
// are identical, and so is a build from a reversed inventory.
func TestBandInvariants(t *testing.T) {
	d := newCluster(t, f1(t), readmodeltest.AllowAll).instance(t, "cert-manager", "cert-manager")
	reversed := d
	reversed.Components = slices.Clone(d.Components)
	slices.Reverse(reversed.Components)
	for _, expand := range [][]string{
		{cmCRDs}, {cmRBAC}, {cmWebhooks},
		{cmRBAC, cmCRDs}, {cmCRDs, cmWebhooks}, {cmCRDs, cmRBAC, cmWebhooks},
	} {
		t.Run(strings.ReplaceAll(strings.Join(expand, ","), cmOwner, ""), func(t *testing.T) {
			g := Instance(d, Options{Expand: expand})
			if got := len(bandRects(g)); got != len(expand) {
				t.Fatalf("%d bands, want %d", got, len(expand))
			}
			checkBands(t, g)
			if !reflect.DeepEqual(g, Instance(d, Options{Expand: expand})) {
				t.Error("two builds differ")
			}
			if !reflect.DeepEqual(g, Instance(reversed, Options{Expand: slices.Clone(expand)})) {
				t.Error("a reversed inventory changed the layout")
			}
		})
	}
}

// TestBandRowsLineUp: once a graph holds a band, every column is on one row
// grid, so a band's members and its objects start on the same row, members
// fill consecutive rows, and the 26 objects of the 13 RBAC members fill 26.
func TestBandRowsLineUp(t *testing.T) {
	d := newCluster(t, f1(t), readmodeltest.AllowAll).instance(t, "cert-manager", "cert-manager")
	g := Instance(d, Options{Expand: []string{cmRBAC}})
	members := bandMembers(g)[cmRBAC]
	var memberYs, objectYs []int
	for i := range g.Nodes {
		n := &g.Nodes[i]
		switch {
		case members[n.ID] && n.Kind == KindComponent:
			memberYs = append(memberYs, n.Y)
		case members[n.ID] && n.Kind == KindObject:
			objectYs = append(objectYs, n.Y)
		}
	}
	if len(memberYs) != 13 || len(objectYs) != 26 {
		t.Fatalf("band holds %d members and %d objects, want 13 and 26", len(memberYs), len(objectYs))
	}
	if memberYs[0] != objectYs[0] {
		t.Errorf("members start at y=%d and objects at y=%d, want one row", memberYs[0], objectYs[0])
	}
	for i := 1; i < len(memberYs); i++ {
		if memberYs[i]-memberYs[i-1] != rowPitch {
			t.Errorf("members %d and %d are %d apart, want consecutive rows of %d", i-1, i, memberYs[i]-memberYs[i-1], rowPitch)
		}
	}
	if span := objectYs[len(objectYs)-1] - objectYs[0]; span != 25*rowPitch {
		t.Errorf("objects span %d, want 25 rows of %d", span, rowPitch)
	}
}

// TestBandKeepsWorkloadsOutside: cert-manager's controller and everything
// below it sit outside the RBAC band, above or below it, never between its
// members, and the members are in name order.
func TestBandKeepsWorkloadsOutside(t *testing.T) {
	d := newCluster(t, f1(t), readmodeltest.AllowAll).instance(t, "cert-manager", "cert-manager")
	g := Instance(d, Options{Expand: []string{cmRBAC}})
	frame := bandRects(g)[0]
	controller := nodeByID(t, g, "comp:mi/cert-manager/cert-manager/controller")
	if controller.Y+g.Layout.NodeHeight >= frame.y0 && controller.Y <= frame.y1 {
		t.Errorf("controller at y=%d lies in the band's rows %d to %d", controller.Y, frame.y0, frame.y1)
	}
	var names []string
	for i := range g.Nodes {
		if n := &g.Nodes[i]; n.Kind == KindComponent && n.MemberOf == cmRBAC {
			names = append(names, n.Label)
		}
	}
	if len(names) != 13 || !slices.IsSorted(names) {
		t.Errorf("members %v, want 13 in name order, top to bottom", names)
	}
}
