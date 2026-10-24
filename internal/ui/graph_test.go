package ui

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// testDrawing is a drawing over nodes laid out as the read API does:
// 216 x 72 boxes.
func testDrawing(nodes ...v1.GraphNode) *drawing {
	g := &v1.Graph{Layout: v1.GraphLayout{NodeWidth: 216, NodeHeight: 72}, Nodes: nodes}
	d := &drawing{g: g, byID: map[string]*v1.GraphNode{}, objects: map[string]int{}, controller: map[string]string{}}
	for i := range g.Nodes {
		d.byID[g.Nodes[i].ID] = &g.Nodes[i]
	}
	return d
}

func i64(n int64) *int64 { return &n }

func hp(state, reason string) *v1.GraphHealth { return &v1.GraphHealth{State: state, Reason: reason} }

// renderGraph executes the graph template on g.
func renderGraph(t *testing.T, g svgGraph) string {
	t.Helper()
	h, err := New(Config{API: http.NotFoundHandler()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var b strings.Builder
	if err := h.pages["owner"].ExecuteTemplate(&b, "graph", g); err != nil {
		t.Fatalf("graph template: %v", err)
	}
	return b.String()
}

// TestNodeStatusLines: the third line of a node, first match wins, for
// every case design.md "Drawing nodes" names (instance-14, provider-14,
// instance-25).
func TestNodeStatusLines(t *testing.T) {
	d := testDrawing()
	d.objects["comp"] = 3
	for _, tc := range []struct {
		name string
		n    v1.GraphNode
		want string
	}{
		{"locked", v1.GraphNode{Kind: "object", Access: v1.AccessForbidden, Health: hp("Healthy", "")}, "Locked"},
		{"missing", v1.GraphNode{Kind: "object", Access: v1.AccessOK, Missing: true}, "Missing"},
		{"source ready", v1.GraphNode{Kind: "source", Access: v1.AccessOK, Health: hp("Healthy", ""),
			SourceState: &v1.SourceState{Access: v1.AccessOK, State: "ready", Revision: "latest@sha256:0123456789abcdef"}}, "Artifact ready · 0123456"},
		{"source not ready", v1.GraphNode{Kind: "source", Access: v1.AccessOK, Health: hp("Degraded", "AuthenticationFailed"),
			SourceState: &v1.SourceState{Access: v1.AccessOK, State: "notReady", Reason: "AuthenticationFailed"}}, "AuthenticationFailed"},
		{"source no artifact", v1.GraphNode{Kind: "source", Access: v1.AccessOK, Health: hp("Unknown", ""),
			SourceState: &v1.SourceState{Access: v1.AccessOK, State: "noArtifact"}}, "No artifact yet"},
		{"source kind not installed", v1.GraphNode{Kind: "source", Access: v1.AccessOK, Missing: true,
			SourceState: &v1.SourceState{Access: v1.AccessOK, State: "kindNotServed"}}, "Kind not installed"},
		{"source not found", v1.GraphNode{Kind: "source", Access: v1.AccessOK, Missing: true,
			SourceState: &v1.SourceState{Access: v1.AccessOK, State: "notFound"}}, "Not found"},
		{"source unsupported", v1.GraphNode{Kind: "source", Access: v1.AccessOK,
			SourceState: &v1.SourceState{Access: v1.AccessOK, State: "unsupportedKind"}}, "Not read by the portal"},
		{"source not looked up", v1.GraphNode{Kind: "source", Health: hp("Healthy", "")}, "Healthy"},
		{"registration active", v1.GraphNode{Kind: "object", Health: hp("Healthy", ""),
			Registration: &v1.GraphRegistration{Verdict: "Accepted", Active: true}}, "Accepted · active"},
		{"registration not active", v1.GraphNode{Kind: "object", Registration: &v1.GraphRegistration{Verdict: "Accepted"}}, "Accepted · not active"},
		{"registration blocked", v1.GraphNode{Kind: "object", Registration: &v1.GraphRegistration{Verdict: "RemovalBlocked"}}, "Removal blocked"},
		{"registration refused", v1.GraphNode{Kind: "object", Registration: &v1.GraphRegistration{Verdict: "Refused", Reason: "ProviderMismatch"}}, "ProviderMismatch"},
		{"registration pending", v1.GraphNode{Kind: "object", Registration: &v1.GraphRegistration{Verdict: "Pending"}}, "Pending"},
		{"configuration group", v1.GraphNode{Kind: "group", Group: &v1.GraphGroup{Kind: "configuration", Family: "rbac",
			ObjectKinds: []v1.GraphKindCount{{Kind: "ClusterRole", Count: 5}, {Kind: "RoleBinding", Count: 3}}}}, "8 objects: 2 kinds"},
		{"configuration group of one", v1.GraphNode{Kind: "group", Group: &v1.GraphGroup{Kind: "configuration",
			ObjectKinds: []v1.GraphKindCount{{Kind: "CustomResourceDefinition", Count: 1}}}}, "1 object: 1 kind"},
		{"pod group", v1.GraphNode{Kind: "group", Health: hp("Degraded", ""), Group: &v1.GraphGroup{Kind: "pods", Members: []string{"a", "b", "c"}}}, "3 Pods · Degraded"},
		{"replica set", v1.GraphNode{Kind: "runtime", Ref: &v1.ObjectRef{Kind: "ReplicaSet"}, Replicas: i64(3), Health: hp("Healthy", "")}, "3 desired"},
		{"replica set not healthy", v1.GraphNode{Kind: "runtime", Ref: &v1.ObjectRef{Kind: "ReplicaSet"}, Replicas: i64(2), Health: hp("Degraded", "ReplicaFailure")}, "2 desired · ReplicaFailure"},
		{"component", v1.GraphNode{ID: "comp", Kind: "component", Health: hp("Healthy", "")}, "3 objects"},
		{"health reason", v1.GraphNode{Kind: "object", Health: hp("Degraded", "ImagePullBackOff")}, "ImagePullBackOff"},
		{"health word", v1.GraphNode{Kind: "object", Health: hp("Progressing", "")}, "Progressing"},
		{"nothing read", v1.GraphNode{Kind: "object"}, "Not read"},
	} {
		if got := d.status(&tc.n, tc.n.Access != "" && tc.n.Access != v1.AccessOK); got != tc.want {
			t.Errorf("%s: status %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestGraphNodeFillAndOutlineCarryHealth (portal:D3:R1): a node's fill and
// outline are one tone, set by its health class, and the applied state is
// a separate square. On the image-break sample the instance node is
// degraded and applied: the class names both axes, the markup draws the
// square, and the stylesheet gives the box the degraded tone and the square
// the applied one. The rail is gone.
func TestGraphNodeFillAndOutlineCarryHealth(t *testing.T) {
	s := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	main := mainOf(s.get(t, "/instances/default/podinfo").body)
	node := regexp.MustCompile(`<a class="node kind-instance ([^"]*)"[^>]*>(?s:.*?)</a>`).FindStringSubmatch(main)
	if node == nil {
		t.Fatal("no instance node")
	}
	if node[1] != "health-degraded ap-applied" || !strings.Contains(node[0], `class="node-stamp"`) {
		t.Errorf("instance node classes %q, want health-degraded ap-applied with a stamp", node[1])
	}
	if strings.Contains(main, "node-rail") {
		t.Error("the page still draws a node rail")
	}
	rules := parseCSS(readPortalCSS(t))
	box, _ := ruleFor(rules, ".node-box")
	if box["fill"] != "var(--c-tint)" || box["stroke"] != "var(--c)" {
		t.Errorf(".node-box fill %q stroke %q, want the hue's tint and border", box["fill"], box["stroke"])
	}
	tone, ok := ruleFor(rules, ".node.health-degraded")
	if !ok || tone["--c"] != "var(--degraded)" || tone["--c-tint"] != "var(--degraded-tint)" || tone["--c-ink"] != "var(--degraded-ink)" {
		t.Errorf(".node.health-degraded sets %v, want the degraded border, ink and tint", tone)
	}
	stamp, _ := ruleFor(rules, ".node-stamp")
	if stamp["fill"] != "var(--applied)" {
		t.Errorf(".node-stamp fill %q, want the applied token", stamp["fill"])
	}
	if _, ok := ruleFor(rules, ".node-rail"); ok {
		t.Error("portal.css still styles .node-rail")
	}
}

// TestRegistrationNodeIsTonedByStanding (portal:D3, provider-14): a
// registration node's fill and outline carry the claim's standing. The
// accepted, active claim of backup-provider is tinted healthy by
// standing-active; the refused claim of the package holder is tinted
// degraded by standing-refused, and says why; neither carries a health
// class.
func TestRegistrationNodeIsTonedByStanding(t *testing.T) {
	rules := parseCSS(readPortalCSS(t))
	for standing, tone := range map[string]string{
		"active": "healthy", "inactive": "progressing", "pending": "progressing", "blocked": "unknown", "refused": "degraded",
	} {
		r, ok := ruleFor(rules, ".node.standing-"+standing)
		if !ok || r["--c-tint"] != "var(--"+tone+"-tint)" || r["--c"] != "var(--"+tone+")" {
			t.Errorf("standing-%s sets %v, want the %s tone", standing, r, tone)
		}
	}
	reg := func(page string, objs bool) string {
		var s *site
		if objs {
			s = newSite(t, apitest.F1PackageHolder(t), apitest.AllowAll)
		} else {
			s = newSite(t, apitest.F1(t), apitest.AllowAll)
		}
		return mainOf(s.get(t, page).body)
	}
	ok := reg("/instances/default/backup-provider", false)
	if m := regexp.MustCompile(`<a class="node kind-object (standing-[a-z]+)[ "]`).FindStringSubmatch(ok); len(m) < 2 || m[1] != "standing-active" {
		t.Errorf("backup-provider's registration node classes %v, want standing-active", m)
	}
	if !strings.Contains(ok, ">Accepted · active</text>") {
		t.Error("backup-provider's registration node does not say Accepted · active")
	}
	refused := reg("/packages/pkg/provider", true)
	m := regexp.MustCompile(`<a class="node kind-object (standing-[a-z]+)[ "][^>]*>(?s:.*?)</a>`).FindStringSubmatch(refused)
	if len(m) < 2 || m[1] != "standing-refused" || !strings.Contains(m[0], ">ProviderMismatch</text>") {
		t.Errorf("the refused holder's registration node: %v", m)
	}
	if strings.Contains(m[0], "health-") {
		t.Error("a registration node carries a health class beside its standing")
	}
}

// TestRuntimeChildrenAreDashedAndNothingElseIs: dashed means made by the
// cluster, and only a runtime child's outline is dashed.
func TestRuntimeChildrenAreDashedAndNothingElseIs(t *testing.T) {
	rules := parseCSS(readPortalCSS(t))
	r, ok := ruleFor(rules, ".node.kind-runtime .node-box")
	if !ok || r["stroke-dasharray"] == "" {
		t.Errorf("a runtime child's box is not dashed: %v", r)
	}
	for _, rule := range rules {
		if !strings.Contains(rule.selector, ".node") || strings.Contains(rule.selector, "kind-runtime") ||
			strings.Contains(rule.selector, "-stamp") || strings.Contains(rule.selector, "pill") {
			continue
		}
		if rule.decls["stroke-dasharray"] != "" {
			t.Errorf("%s is dashed, but only a runtime child may be", rule.selector)
		}
	}
}

// TestEdgeIsAnElbowWithoutAMarker: the route's four points are the
// elbow's corners; the path is M p0 H p1.x V p3.y H p3.x, and no edge
// carries an arrow marker.
func TestEdgeIsAnElbowWithoutAMarker(t *testing.T) {
	e := v1.GraphEdge{ID: "e", Kind: "owns", From: "a", To: "b", Route: []v1.GraphPoint{{X: 232, Y: 80}, {X: 256, Y: 80}, {X: 256, Y: 176}, {X: 280, Y: 176}}}
	got, ok := edgeOf(&e, nil, 0)
	if !ok || got.D != "M232 80 H256 V176 H280" {
		t.Errorf("path %q, want M232 80 H256 V176 H280", got.D)
	}
	moved, _ := edgeOf(&e, nil, 264)
	if moved.D != "M-32 80 H-8 V176 H16" {
		t.Errorf("a path moved one column left reads %q", moved.D)
	}
	e.Route = e.Route[:3]
	if _, ok := edgeOf(&e, nil, 0); ok {
		t.Error("an edge without four route points was drawn")
	}
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	main := mainOf(s.get(t, "/instances/default/podinfo").body)
	if strings.Contains(main, "marker") {
		t.Error("the graph still draws an arrow marker")
	}
	if rule, _ := ruleFor(parseCSS(readPortalCSS(t)), ".edge"); rule["stroke-width"] != "2" {
		t.Errorf(".edge stroke-width %q, want 2", rule["stroke-width"])
	}
	for _, d := range regexp.MustCompile(`<path class="edge [^"]*" d="([^"]*)"`).FindAllStringSubmatch(main, -1) {
		if !regexp.MustCompile(`^M-?\d+ -?\d+ H-?\d+ V-?\d+ H-?\d+$`).MatchString(d[1]) {
			t.Errorf("edge path %q is not an elbow", d[1])
		}
	}
}

// TestGroupNodeAtRest: cert-manager at rest draws its three groups as a
// stack under a hatch, with the kind line, family and counts of
// design.md "Group nodes"; an unknown family reads "Configuration".
func TestGroupNodeAtRest(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	main := mainOf(s.get(t, "/instances/cert-manager/cert-manager").body)
	for _, want := range []string{
		">Group · 13 components</text>", ">RBAC</text>", ">Group · 1 component</text>", ">CRDs</text>",
		`<rect class="node-stack"`, `fill="url(#instance-graph-ghatch)"`, `id="instance-graph-ghatch"`,
	} {
		if !strings.Contains(main, want) {
			t.Errorf("cert-manager at rest lacks %s", want)
		}
	}
	if !regexp.MustCompile(`>\d+ objects: \d+ kinds</text>`).MatchString(main) {
		t.Error("no group node says its object and kind counts")
	}
	d := testDrawing()
	n := v1.GraphNode{Kind: "group", ID: "g", Label: "x", Group: &v1.GraphGroup{Kind: "configuration", Family: "nonsense", Members: []string{"a", "b"}}}
	got := d.nodeOf(&n)
	if got.Label != "Configuration" || got.Kind != "Group · 2 components" || !strings.Contains(got.Class, "kind-group") {
		t.Errorf("an unknown family draws %q on %q (%s), want Configuration", got.Label, got.Kind, got.Class)
	}
}

const certManagerID = "mi/cert-manager/cert-manager"

// collapseHref is the href of the Collapse link labeled label on page.
func collapseHref(page, label string) string {
	m := regexp.MustCompile(`<a class="group-collapse" href="([^"]*)"[^>]*aria-label="` + label + `"`).FindStringSubmatch(page)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// TestFrameOfAnExpandedGroup: the open RBAC group has one frame from the
// components column to the last column across its band, with its header
// and a Collapse link that drops the group and selects it.
func TestFrameOfAnExpandedGroup(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	const rbac = "grp:configuration/" + certManagerID + "/rbac"
	main := mainOf(s.get(t, "/instances/cert-manager/cert-manager?expand="+rbac).body)
	if n := strings.Count(main, `class="group-frame"`); n != 1 {
		t.Fatalf("%d frames, want 1", n)
	}
	frame := regexp.MustCompile(`<rect class="group-frame" x="(\d+)" y="(\d+)" width="(\d+)" height="(\d+)"`).FindStringSubmatch(main)
	// Components column x 280 less 12; last column (Pods) x 1072 plus 216
	// plus 12; the band is 26 rows (13 members, 26 objects) with 48 px
	// above its first node and 24 px below its last.
	if len(frame) < 5 || frame[1] != "268" || frame[3] != "1032" || frame[4] != "2544" {
		t.Fatalf("frame %v, want x 268, width 1032 and height 2544", frame)
	}
	for _, want := range []string{">RBAC · 13 components · 26 objects</text>", `aria-label="Collapse RBAC"`} {
		if !strings.Contains(main, want) {
			t.Errorf("the frame lacks %s", want)
		}
	}
	href := collapseHref(main, "Collapse RBAC")
	if strings.Contains(href, "expand=") || !strings.Contains(href, "tab=graph") || !strings.Contains(href, "node=grp%3Aconfiguration%2Fmi%2Fcert-manager%2Fcert-manager%2Frbac") {
		t.Errorf("Collapse link %q: want the graph tab, no expand, and node=<group>", href)
	}
}

// TestCollapseKeepsTheOtherGroupsOpen: with two groups open there are two
// frames, and the Collapse link of one still expands the other.
func TestCollapseKeepsTheOtherGroupsOpen(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	const rbac = "grp:configuration/" + certManagerID + "/rbac"
	const crds = "grp:configuration/" + certManagerID + "/crds"
	two := mainOf(s.get(t, "/instances/cert-manager/cert-manager?expand="+rbac+"&expand="+crds).body)
	if n := strings.Count(two, `class="group-frame"`); n != 2 {
		t.Errorf("%d frames with two groups open, want 2", n)
	}
	href := collapseHref(two, "Collapse RBAC")
	if !strings.Contains(href, "expand=grp%3Aconfiguration%2Fmi%2Fcert-manager%2Fcert-manager%2Fcrds") || strings.Contains(strings.Split(href, "node=")[0], "rbac") {
		t.Errorf("Collapse of RBAC with CRDs open: %q", href)
	}
}

// TestNoFrameUnderGroupOff: the ungrouped view draws no frame.
func TestNoFrameUnderGroupOff(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	off := mainOf(s.get(t, "/instances/cert-manager/cert-manager?group=off&expand=grp:configuration/"+certManagerID+"/rbac").body)
	if strings.Contains(off, "group-frame") {
		t.Error("a frame is drawn under group=off")
	}
}

func moduleGraph() v1.Graph {
	return v1.Graph{
		Root: "mi",
		Layout: v1.GraphLayout{Width: 800, Height: 200, NodeWidth: 216, NodeHeight: 72,
			Columns: []v1.GraphColumn{{Title: "Module", X: 16}, {Title: "Instance", X: 280}, {Title: "Components", X: 544}}},
		Nodes: []v1.GraphNode{
			{ID: "mod", Kind: "module", Label: "m", Column: 0, X: 16, Y: 44},
			{ID: "mi", Kind: "instance", Label: "ns/i", Column: 1, X: 280, Y: 44},
			{ID: "c", Kind: "component", Label: "web", Column: 2, X: 544, Y: 44},
		},
		Edges: []v1.GraphEdge{
			{ID: "e1", Kind: "instantiates", From: "mod", To: "mi", Route: []v1.GraphPoint{{X: 232, Y: 80}, {X: 256, Y: 80}, {X: 256, Y: 80}, {X: 280, Y: 80}}},
			{ID: "e2", Kind: "hasComponent", From: "mi", To: "c", Route: []v1.GraphPoint{{X: 496, Y: 80}, {X: 520, Y: 80}, {X: 520, Y: 80}, {X: 544, Y: 80}}},
		},
	}
}

// TestBuildGraphDropsTheModuleNode: the module node, its edge and its
// column go, and the rest moves left by one column pitch; the document is
// not changed, and a graph whose first column still holds a node stays.
func TestBuildGraphDropsTheModuleNode(t *testing.T) {
	g := moduleGraph()
	out := buildGraph(g, graphParams{ID: "instance-graph", Page: "/instances/ns/i", Panel: "/instances/ns/i/node"})
	if len(out.Nodes) != 2 || len(out.Edges) != 1 || out.Width != 800-264 {
		t.Fatalf("%d nodes, %d edges, width %d; want 2, 1, 536", len(out.Nodes), len(out.Edges), out.Width)
	}
	if out.Columns[0].Title != "Instance" || out.Columns[0].X != 16 || out.Nodes[0].X != 16 || out.Nodes[1].X != 280 {
		t.Errorf("columns %v, node x %d and %d: the instance should start at 16", out.Columns, out.Nodes[0].X, out.Nodes[1].X)
	}
	if out.Edges[0].D != "M232 80 H256 V80 H280" {
		t.Errorf("edge path %q was not moved with the nodes", out.Edges[0].D)
	}
	if g.Nodes[1].X != 280 || g.Layout.Columns[0].Title != "Module" {
		t.Error("buildGraph changed the document it was given")
	}
	g.Nodes[0].Kind = "object"
	if kept := buildGraph(g, graphParams{ID: "instance-graph", Page: "/", Panel: "/node"}); kept.Width != 800 || kept.Columns[0].Title != "Module" {
		t.Errorf("a graph with a node in its first column was moved: width %d, columns %v", kept.Width, kept.Columns)
	}
}

// TestInstancePageStartsAtTheInstance: podinfo's graph page draws no module
// node and no Module column.
func TestInstancePageStartsAtTheInstance(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	podinfo := mainOf(s.get(t, "/instances/default/podinfo").body)
	if strings.Contains(podinfo, "kind-module") || strings.Contains(podinfo, ">Module</text>") {
		t.Error("podinfo's graph draws a module node or its column")
	}
	if !strings.Contains(podinfo, `<text class="col-title" x="16" y="20">Instance</text>`) {
		t.Error("podinfo's graph does not start at the instance")
	}
}

// TestLegendNamesTheThirdOriginOnlyOnAPackageGraph (AG-A3): a graph that
// draws a source node adds the entry for its origin and says that
// components and dependency packages are drawn the same way; an instance
// graph has neither. The toolbar hint is the canvas's.
func TestLegendNamesTheThirdOriginOnlyOnAPackageGraph(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	pkg := mainOf(s.get(t, "/packages/pkg/podinfo").body)
	inst := mainOf(s.get(t, "/instances/default/podinfo").body)
	const third = "Read by the controller, owned by Flux. Components and dependency packages are drawn the same way"
	if !strings.Contains(between(pkg, `class="graph-legend"`, "</ul>"), third) {
		t.Error("the package graph's legend lacks the third origin and its sentence")
	}
	if strings.Contains(between(inst, `class="graph-legend"`, "</ul>"), "Flux") || strings.Contains(inst, "dependency packages") {
		t.Error("the instance graph's legend names the source origin")
	}
	for name, page := range map[string]string{"package": pkg, "instance": inst} {
		legend := between(page, `class="graph-legend"`, "</ul>")
		for _, want := range []string{"Applied by the controller", "Made by the cluster", "Group of configuration components", "Fill and outline: health", "<kbd>Ctrl</kbd> + scroll to zoom", "<kbd>Esc</kbd> to step back"} {
			if !strings.Contains(legend, want) {
				t.Errorf("the %s graph's legend lacks %q", name, want)
			}
		}
		if !strings.Contains(page, `<span class="hint">Select a box to see its details on the right.</span>`) {
			t.Errorf("the %s graph's toolbar hint is not the canvas's", name)
		}
	}
	// The package's source card uses the legend's words.
	if !strings.Contains(pkg, `<span class="nc-origin">Read by the controller, owned by Flux</span>`) {
		t.Error("the source node's card does not use the legend's origin words")
	}
}

// TestGhostBoxOfAnInstance: an instance that applied nothing draws a
// dashed 216 x 72 box and edge right of the root, its text broken into
// tspans, and the viewBox widens to hold it.
func TestGhostBoxOfAnInstance(t *testing.T) {
	root := svgNode{ID: "mi", X: 16, Y: 44, W: 216, H: 72}
	inst := svgGraph{RootID: "mi", Width: 280, Height: 132, Columns: []v1.GraphColumn{{X: 16}, {X: 280}}, Nodes: []svgNode{root}}
	inst.addGhost(false)
	g := inst.Ghost
	if g == nil || g.X != 280 || g.Y != 44 || g.H != 72 || len(g.Lines) != 3 || inst.Width != 280+216+16 {
		t.Fatalf("instance ghost %+v, width %d: want x 280, y 44, 3 lines in 72 px, width 512", g, inst.Width)
	}
	for _, l := range g.Lines {
		if len([]rune(l.Text)) > ghostWidth {
			t.Errorf("ghost line %q is longer than %d", l.Text, ghostWidth)
		}
	}
	if g.D != "M232 80 H256 V80 H280" {
		t.Errorf("ghost edge %q", g.D)
	}
}

// TestGhostBoxOfAPackage: a package that never fetched its source says so
// in a taller box, below the source it draws in that column.
func TestGhostBoxOfAPackage(t *testing.T) {
	root := svgNode{ID: "mi", X: 16, Y: 44, W: 216, H: 72}
	src := svgNode{ID: "src", X: 280, Y: 44, W: 216, H: 72}
	pkg := svgGraph{RootID: "mi", Width: 512, Height: 132, Columns: []v1.GraphColumn{{X: 16}, {X: 280}}, Nodes: []svgNode{root, src}}
	pkg.addGhost(true)
	p := pkg.Ghost
	if p == nil || p.Y != 140 || len(p.Lines) != 5 || p.H != 104 || pkg.Height != 140+104+16 {
		t.Fatalf("package ghost %+v, height %d: want y 140 below the source, 5 lines in 104 px", p, pkg.Height)
	}
	if !strings.Contains(p.Lines[3].Text+" "+p.Lines[4].Text, "never fetched its source") {
		t.Errorf("package ghost lines %+v lack the never-fetched sentence", p.Lines)
	}
}

// TestGhostOnThePages: the F1 package page draws its ghost; an instance
// with an inventory does not.
func TestGhostOnThePages(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	if page := mainOf(s.get(t, "/packages/pkg/podinfo").body); !strings.Contains(page, `class="ghost-box"`) || !strings.Contains(page, "never fetched") {
		t.Error("the F1 package page does not draw its ghost")
	}
	if page := mainOf(s.get(t, "/instances/default/podinfo").body); strings.Contains(page, "ghost") {
		t.Error("an instance with an inventory draws a ghost")
	}
}

// cardNodes are the nodes the card tests read: a ReplicaSet and its Pod, a
// Deployment, a registration, a group, a source and the owner.
func cardNodes() *drawing {
	long := strings.Repeat("é", 300)
	d := testDrawing(
		v1.GraphNode{ID: "rs", Kind: "runtime", Label: "podinfo-7c9d", Ref: &v1.ObjectRef{Kind: "ReplicaSet", Namespace: "default", Name: "podinfo-7c9d"}, Replicas: i64(3),
			Health: &v1.GraphHealth{State: "Degraded", Reason: "ReplicaFailure", Message: long}},
		v1.GraphNode{ID: "pod", Kind: "runtime", Label: "podinfo-7c9d-x", Ref: &v1.ObjectRef{Kind: "Pod", Namespace: "default", Name: "podinfo-7c9d-x"}},
		v1.GraphNode{ID: "dep", Kind: "object", Label: "podinfo", Ref: &v1.ObjectRef{Kind: "Deployment", Namespace: "default", Name: "podinfo"}},
		v1.GraphNode{ID: "reg", Kind: "object", Label: "backup", Ref: &v1.ObjectRef{Kind: "TransformerRegistration", Name: "backup"},
			Registration: &v1.GraphRegistration{Catalog: "testing.opmodel.dev/catalogs/controller/backup@v0", Version: "0.1.0", Verdict: "Accepted", Active: true}},
		v1.GraphNode{ID: "grp", Kind: "group", Label: "x", Group: &v1.GraphGroup{Kind: "configuration", Family: "rbac"}},
		v1.GraphNode{ID: "src", Kind: "source", Label: "OCIRepository podinfo-release", Ref: &v1.ObjectRef{Kind: "OCIRepository", Namespace: "pkg", Name: "podinfo-release"}},
		v1.GraphNode{ID: "mi", Kind: "instance", Label: "default/backup-provider"},
	)
	d.g.Root = "mi"
	d.controller["pod"] = "rs"
	return d
}

// TestHoverCardFacts: the card says namespace/name, the health message cut
// to 120 characters, the replicas and the hint.
func TestHoverCardFacts(t *testing.T) {
	d := cardNodes()
	card := d.cardOf(&d.g.Nodes[0], "Runtime", false, "Degraded")
	if card.Name != "default/podinfo-7c9d" || card.Fact != "Replicas: 3" || card.Hint != "Click for details" || card.Reason != "ReplicaFailure" {
		t.Errorf("replica set card %+v", card)
	}
	if n := len([]rune(card.Message)); n != 120 || !strings.HasSuffix(card.Message, "…") {
		t.Errorf("message is %d characters ending %q, want 120 ending with an ellipsis", n, card.Message[len(card.Message)-3:])
	}
	g := d.cardOf(&d.g.Nodes[4], "Group", false, "")
	if g.Hint != "Click to expand" {
		t.Errorf("a closed group's hint is %q", g.Hint)
	}
	d.expanded = []string{"grp"}
	if h := d.cardOf(&d.g.Nodes[4], "Group", false, "").Hint; h != "Click for details" {
		t.Errorf("an expanded group's hint is %q", h)
	}
	locked := v1.GraphNode{ID: "l", Kind: "object", Access: v1.AccessForbidden, Label: "secret-thing", Health: hp("Healthy", "ok")}
	if c := d.cardOf(&locked, "Object", true, "Healthy"); c.Access == "" || c.Health != nil || c.Hint == "" {
		t.Errorf("locked card %+v: want the access words, no health, and the hint", c)
	}
}

// TestHoverCardOrigins: the canvas's origin words, and a runtime child
// naming the object that controls it.
func TestHoverCardOrigins(t *testing.T) {
	d := cardNodes()
	for i, want := range map[int]string{
		1: "Created by the cluster from ReplicaSet podinfo-7c9d",
		2: "Applied by the controller",
		4: "Display group",
		5: "Read by the controller, owned by Flux",
		6: "Reconciled by the controller",
	} {
		if got := d.cardOf(&d.g.Nodes[i], "x", false, "").Origin; got != want {
			t.Errorf("%s: origin %q, want %q", d.g.Nodes[i].ID, got, want)
		}
	}
}

// TestHoverCardOfARegistration: its standing, its catalog and the owner
// that provides it.
func TestHoverCardOfARegistration(t *testing.T) {
	d := cardNodes()
	r := d.cardOf(&d.g.Nodes[3], "Object", false, "")
	if r.Standing == nil || r.Standing.Text != "Accepted · active" || r.Catalog != "Catalog: testing.opmodel.dev/catalogs/controller/backup@v0 0.1.0 · Provider: default/backup-provider" {
		t.Errorf("registration card %+v", r)
	}
}

// TestHostileTextInTheDrawingIsEscaped: cluster text in a node's lines, a
// card, the frame header and the ghost leaves the template escaped.
func TestHostileTextInTheDrawingIsEscaped(t *testing.T) {
	d := testDrawing(
		v1.GraphNode{ID: hostile, Kind: "object", Label: hostile, Ref: &v1.ObjectRef{Kind: hostile, Namespace: hostile, Name: hostile},
			Health: &v1.GraphHealth{State: "Degraded", Reason: hostile, Message: hostile}, Replicas: i64(1),
			Registration: &v1.GraphRegistration{Catalog: hostile, Version: hostile, Verdict: "Refused", Reason: hostile}},
		v1.GraphNode{ID: "src", Kind: "source", Label: hostile, Ref: &v1.ObjectRef{Kind: hostile, Name: hostile},
			SourceState: &v1.SourceState{Access: "ok", State: "notReady", Reason: hostile, Message: hostile}},
		v1.GraphNode{ID: "m", Kind: "component", Label: hostile, MemberOf: configGroupPrefix + hostile + "/" + hostile},
	)
	d.g.Root = hostile
	d.g.Layout.Columns = []v1.GraphColumn{{Title: hostile, X: 16}}
	out := svgGraph{ID: "g", RootID: hostile, Width: 600, Height: 200, Columns: d.columns(), Frames: d.frames(d.columns())}
	for i := range d.g.Nodes {
		out.Nodes = append(out.Nodes, d.nodeOf(&d.g.Nodes[i]))
		out.Nodes[i].Card.ID = "c" + strings.Repeat("x", i)
	}
	out.addGhost(true)
	if len(out.Frames) != 1 {
		t.Fatalf("%d frames, want 1", len(out.Frames))
	}
	html := renderGraph(t, out)
	for _, raw := range []string{"<script>alert", "<img src=x"} {
		if strings.Contains(html, raw) {
			t.Errorf("the graph renders cluster text as markup: %q", raw)
		}
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("no hostile text reached the graph, so the test checks nothing")
	}
	if strings.Contains(html, " style=") {
		t.Error("the graph carries a style attribute")
	}
}
