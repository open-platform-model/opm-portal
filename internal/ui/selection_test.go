package ui

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

const (
	brokenDeployment = "obj:apps/Deployment/default/podinfo-podinfo"
	brokenService    = "obj:_/Service/default/podinfo-podinfo"
	brokenNewPod     = "obj:_/Pod/default/podinfo-podinfo-794bd8c7fb-cc5ql"
	podinfoRoot      = "mi:default/podinfo"
	backupReg        = "obj:opmodel.dev/TransformerRegistration/_/default.backup-provider"
)

var nodeTag = regexp.MustCompile(`<a class="([^"]*)" href="[^"]*" data-panel="[^"]*" data-node="([^"]*)"([^>]*)>`)

// nodeClassesOf maps each drawn node's id to its classes and the rest of
// its tag (aria-current, data-spotlight, data-resting).
func nodeClassesOf(t *testing.T, page string) (classes, rest map[string]string) {
	t.Helper()
	classes, rest = map[string]string{}, map[string]string{}
	for _, m := range nodeTag.FindAllStringSubmatch(page, -1) {
		id := strings.ReplaceAll(m[2], "&#43;", "+")
		classes[id], rest[id] = m[1], m[3]
	}
	if len(classes) == 0 {
		t.Fatal("the page draws no nodes")
	}
	return classes, rest
}

func dimmedNodes(t *testing.T, page string) []string {
	t.Helper()
	classes, _ := nodeClassesOf(t, page)
	var out []string
	for id, c := range classes {
		if slices.Contains(strings.Fields(c), "dim") {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

func podinfoGraph(t *testing.T, query string) string {
	t.Helper()
	s := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	return mainOf(s.get(t, "/instances/default/podinfo?tab=graph&"+query).body)
}

// TestChainIsTwoDirectedWalks: the chain of a node is its ancestors and its
// descendants and nothing joined to an ancestor by another path.
func TestChainIsTwoDirectedWalks(t *testing.T) {
	edge := func(from, to string) v1.GraphEdge { return v1.GraphEdge{From: from, To: to} }
	edges := []v1.GraphEdge{edge("i", "c"), edge("c", "svc"), edge("c", "dep"), edge("dep", "rs"), edge("rs", "pod"), edge("x", "svc")}
	got := chain(edges, "dep")
	for _, id := range []string{"dep", "c", "i", "rs", "pod"} {
		if !got[id] {
			t.Errorf("chain of dep lacks %s", id)
		}
	}
	for _, id := range []string{"svc", "x"} {
		if got[id] {
			t.Errorf("chain of dep holds %s, a sibling reached through a shared ancestor", id)
		}
	}
	if len(chain(edges, "pod")) != 5 || !chain(edges, "pod")["i"] {
		t.Errorf("chain of pod = %v, want pod, rs, dep, c, i", chain(edges, "pod"))
	}
	if loop := chain([]v1.GraphEdge{edge("a", "b"), edge("b", "a")}, "a"); len(loop) != 2 {
		t.Errorf("a cycle gave %v", loop)
	}
}

// TestFocusingThePodLightsItsChain: the image-break Pod's spotlight leaves
// the instance, its component, the Deployment, the new ReplicaSet and the
// Pod lit, and dims the Service, the old ReplicaSet and its Pods.
func TestFocusingThePodLightsItsChain(t *testing.T) {
	page := podinfoGraph(t, "focus="+url.QueryEscape(brokenNewPod))
	dim := dimmedNodes(t, page)
	want := []string{
		"obj:_/Pod/default/podinfo-podinfo-d9585d794-5m9tx", "obj:_/Pod/default/podinfo-podinfo-d9585d794-j6jr9",
		brokenService, "obj:apps/ReplicaSet/default/podinfo-podinfo-d9585d794",
	}
	slices.Sort(want)
	if !slices.Equal(dim, want) {
		t.Errorf("dimmed %v, want %v", dim, want)
	}
	_, rest := nodeClassesOf(t, page)
	if !strings.Contains(rest[brokenNewPod], `aria-current="true"`) || !strings.Contains(rest[brokenNewPod], "data-spotlight") {
		t.Errorf("the Pod's tag %q is not ringed and spotlit", rest[brokenNewPod])
	}
	if !strings.Contains(page, `data-focus="`+brokenNewPod+`"`) {
		t.Error("the graph does not name its spotlit node, so the script cannot center it")
	}
	// The edge between the component and the Service has an end outside the
	// chain, so it fades; the edge from the Deployment to the new
	// ReplicaSet does not.
	edgeClass := func(from, to string) string {
		m := regexp.MustCompile(`<path class="([^"]*)" d="[^"]*" data-from="` + regexp.QuoteMeta(from) + `" data-to="` + regexp.QuoteMeta(to) + `"`).FindStringSubmatch(page)
		if m == nil {
			t.Fatalf("no edge %s to %s", from, to)
		}
		return m[1]
	}
	if c := edgeClass("comp:mi/default/podinfo/podinfo", brokenService); !strings.Contains(c, "dim") {
		t.Errorf("the component's edge to the Service is %q, want dim", c)
	}
	if c := edgeClass(brokenDeployment, "obj:apps/ReplicaSet/default/podinfo-podinfo-794bd8c7fb"); strings.Contains(c, "dim") {
		t.Errorf("the Deployment's edge to the new ReplicaSet is %q, want lit", c)
	}
}

// TestFocusingTheDeploymentDoesNotLightTheServiceSibling: the Service shares
// the component with the Deployment and stays dimmed; everything below the
// Deployment is lit.
func TestFocusingTheDeploymentDoesNotLightTheServiceSibling(t *testing.T) {
	dim := dimmedNodes(t, podinfoGraph(t, "focus="+url.QueryEscape(brokenDeployment)))
	if !slices.Equal(dim, []string{brokenService}) {
		t.Errorf("dimmed %v, want only the Service", dim)
	}
}

// TestSelectionDimsNothing: node= rings the node and shows its panel with no
// dimming and no halo; focus= adds the halo and the dimming.
func TestSelectionDimsNothing(t *testing.T) {
	page := podinfoGraph(t, "node="+url.QueryEscape(brokenService))
	if dim := dimmedNodes(t, page); len(dim) != 0 {
		t.Errorf("a selection dimmed %v", dim)
	}
	if strings.Contains(page, `class="edge`) && regexp.MustCompile(`<path class="[^"]*\bdim\b`).MatchString(page) {
		t.Error("a selection dimmed an edge")
	}
	_, rest := nodeClassesOf(t, page)
	if !strings.Contains(rest[brokenService], `aria-current="true"`) || strings.Contains(rest[brokenService], "data-spotlight") || strings.Contains(rest[brokenService], "data-resting") {
		t.Errorf("the Service's tag %q should be selected only", rest[brokenService])
	}
	if strings.Contains(page, "data-focus=") {
		t.Error("a selection names a spotlit node")
	}
	if !strings.Contains(page, `<p class="kicker">Object</p>`) && !strings.Contains(page, "podinfo-podinfo") {
		t.Error("the details panel does not show the Service")
	}
	if !strings.Contains(page, `class="graph has-selection"`) {
		t.Error("the graph is not marked as holding a selection")
	}
}

// TestNodeLinksSelectAndKeepTheOpenGroups: a node's link is the no-script
// form of a click: node=, not focus=, and the groups the page has open.
func TestNodeLinksSelectAndKeepTheOpenGroups(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	const rbac = "grp:configuration/" + certManagerID + "/rbac"
	page := mainOf(s.get(t, "/instances/cert-manager/cert-manager?tab=graph&group=off&expand="+rbac).body)
	hrefs := regexp.MustCompile(`<a class="node [^"]*" href="([^"]*)"`).FindAllStringSubmatch(page, -1)
	if len(hrefs) == 0 {
		t.Fatal("no node links")
	}
	checked := 0
	for _, m := range hrefs {
		u, err := url.Parse(strings.ReplaceAll(m[1], "&amp;", "&"))
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		if q.Has("fit") { // a closed group's link opens it instead of selecting it
			continue
		}
		checked++
		if q.Get("node") == "" || q.Has("focus") || q.Get("tab") != "graph" || q.Get("group") != "off" || !slices.Contains(q["expand"], rbac) {
			t.Errorf("node link %q lacks node, the tab, group=off or the open group", m[1])
		}
	}
	if checked == 0 {
		t.Error("no link of an ordinary node was checked")
	}
}

// TestResourcesHandOffOpensAndFitsAFoldedGroup: the Graph link of a folded
// RBAC object carries expand, fit and focus; one of a plain object, focus
// alone.
func TestResourcesHandOffOpensAndFitsAFoldedGroup(t *testing.T) {
	if got := graphHandoff("/instances/a/b", "obj:x", ""); got != "/instances/a/b?focus=obj%3Ax&tab=graph" {
		t.Errorf("a plain hand-off is %q", got)
	}
	got := graphHandoff("/instances/a/b", "obj:x", "grp:g")
	q := mustQuery(t, got)
	if q.Get("expand") != "grp:g" || q.Get("fit") != "grp:g" || q.Get("focus") != "obj:x" || q.Get("tab") != "graph" {
		t.Errorf("a folded hand-off is %q", got)
	}
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	res := mainOf(s.get(t, "/instances/cert-manager/cert-manager?tab=resources").body)
	links := regexp.MustCompile(`class="row-link" href="(/instances/cert-manager/cert-manager\?expand=[^"]+)"`).FindAllStringSubmatch(res, -1)
	if len(links) == 0 {
		t.Fatal("no folded object links to the graph")
	}
	for _, l := range links {
		q := mustQuery(t, strings.ReplaceAll(l[1], "&amp;", "&"))
		if q.Get("expand") == "" || q.Get("fit") != q.Get("expand") || q.Get("focus") == "" {
			t.Errorf("folded link %q lacks expand, fit or focus", l[1])
		}
	}
}

func mustRequest(t *testing.T, target string) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
}

func mustQuery(t *testing.T, href string) url.Values {
	t.Helper()
	u, err := url.Parse(href)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}

// restingOf is the id of the node a page rests on, from its data-resting
// attribute, and of the panel it shows.
func restingOf(t *testing.T, page string) (node, panelName string) {
	t.Helper()
	m := regexp.MustCompile(`data-node="([^"]*)"[^>]*data-resting`).FindStringSubmatch(page)
	if m == nil {
		t.Fatal("no node is marked as the resting node")
	}
	d := regexp.MustCompile(`(?s)<aside[^>]*id="detail"[^>]*>(.*?)</aside>`).FindStringSubmatch(page)
	if d == nil {
		t.Fatal("no details panel")
	}
	h := regexp.MustCompile(`<h2 class="frag-h mono">(.*?)</h2>`).FindStringSubmatch(d[1])
	if h == nil {
		return m[1], d[1]
	}
	return m[1], strings.ReplaceAll(h[1], "<wbr>", "")
}

// TestRestingPanel: with nothing selected the panel shows the owner's own
// node, or the claim that needs attention; the resting node wears the plain
// ring and is not a selection.
func TestRestingPanel(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	cm := mainOf(s.get(t, "/instances/cert-manager/cert-manager?tab=graph").body)
	node, name := restingOf(t, cm)
	if node != "mi:cert-manager/cert-manager" || !strings.Contains(name, "cert-manager") {
		t.Errorf("cert-manager rests on %q showing %q, want its instance node", node, name)
	}
	_, rest := nodeClassesOf(t, cm)
	if strings.Contains(rest[node], "aria-current") {
		t.Error("the resting node is marked as a selection")
	}
	if strings.Contains(cm, "data-clear-selection") && !strings.Contains(cm, `data-clear-selection hidden`) {
		t.Error("the resting panel offers Clear selection")
	}
	if !strings.Contains(cm, `data-resting-panel="/instances/cert-manager/cert-manager/node?id=mi%3Acert-manager%2Fcert-manager"`) {
		t.Error("the graph does not name the resting panel the script returns to")
	}
	// The Resources tab rests the same way.
	res := mainOf(s.get(t, "/instances/cert-manager/cert-manager?tab=resources").body)
	if d := regexp.MustCompile(`(?s)id="detail".*?<h2 class="frag-h mono">(.*?)</h2>`).FindStringSubmatch(res); len(d) < 2 || !strings.Contains(d[1], "cert-manager") {
		t.Error("the Resources tab does not rest on the instance")
	}
	// A selection moves the ring and the panel; the resting mark goes.
	sel := mainOf(s.get(t, "/instances/cert-manager/cert-manager?tab=graph&node=obj:_/Namespace/_/cert-manager").body)
	if regexp.MustCompile(` data-resting[ >]`).MatchString(sel) {
		t.Error("a selection leaves the resting mark on another node")
	}

	// A package whose held claim was refused rests on that claim.
	hold := newSite(t, apitest.F1PackageHolder(t), apitest.AllowAll)
	pkg := mainOf(hold.get(t, "/packages/pkg/provider?tab=graph").body)
	node, name = restingOf(t, pkg)
	if !strings.Contains(node, "TransformerRegistration") || !strings.Contains(name, "provider") {
		t.Errorf("the refused holder rests on %q showing %q, want its registration", node, name)
	}
	if !strings.Contains(pkg, "ProviderMismatch") || len(dimmedNodes(t, pkg)) != 0 {
		t.Error("the refused claim's reason is not shown or a node is dimmed")
	}
}

// TestRestingNodeChoice: the first attention-needing claim by name wins, an
// active one never does, and a claim the caller may not read does not move
// the rest.
func TestRestingNodeChoice(t *testing.T) {
	reg := func(id, name, verdict string, active bool, access string) v1.GraphNode {
		return v1.GraphNode{ID: id, Kind: "object", Label: name, Access: access, Ref: &v1.ObjectRef{Name: name},
			Registration: &v1.GraphRegistration{Verdict: verdict, Accepted: verdict == "Accepted", Active: active}}
	}
	g := &v1.Graph{Root: "mi", Nodes: []v1.GraphNode{
		{ID: "mi", Kind: "instance", Label: "ns/i"},
		reg("r-b", "b", "Refused", false, "ok"),
		reg("r-a", "a", "Accepted", true, "ok"),
		reg("r-c", "c", "Pending", false, ""),
		reg("r-0", "0", "Refused", false, v1.AccessForbidden),
		reg("r-1", "1", verdictRemovalBlocked, false, "ok"),
	}}
	if got := restingNode(g); got == nil || got.ID != "r-1" {
		t.Errorf("rests on %v, want r-1 (first by name of the readable, attention-needing claims)", got)
	}
	g.Nodes = slices.DeleteFunc(g.Nodes, func(n v1.GraphNode) bool { return n.ID == "r-1" || n.ID == "r-b" || n.ID == "r-c" })
	if got := restingNode(g); got == nil || got.ID != "mi" {
		t.Errorf("with only an active and a forbidden claim it rests on %v, want the owner", got)
	}
	g.Nodes[2].Registration.Verdict, g.Nodes[2].Registration.Active = "Accepted", false
	g.Nodes[2].Access = "ok"
	if got := restingNode(g); got == nil || got.ID != "r-0" {
		t.Errorf("an accepted claim that is not active rests on %v, want r-0", got)
	}
}

// TestForbiddenClaimDoesNotMoveTheRest: when the caller may not read
// registrations, a refused holder's page rests on the package.
func TestForbiddenClaimDoesNotMoveTheRest(t *testing.T) {
	s := newSite(t, apitest.F1PackageHolder(t), apitest.DenyResources("transformerregistrations"))
	pkg := mainOf(s.get(t, "/packages/pkg/provider?tab=graph").body)
	node, name := restingOf(t, pkg)
	if node != "mp:pkg/provider" {
		t.Errorf("rests on %q (%q), want the package node", node, name)
	}
	if strings.Contains(name, "backup-provider") {
		t.Errorf("the panel shows the registration the caller may not read: %q", name)
	}
}

// TestClearSelectionPill: a panel that is not the resting node's carries the
// pill, titled for the other ways out; the resting panel does not.
func TestClearSelectionPill(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	const title = `title="Or click an empty part of the graph, or press Esc"`
	page := mainOf(s.get(t, "/instances/default/podinfo?tab=graph&node="+url.QueryEscape("obj:apps/Deployment/default/podinfo-podinfo")).body)
	detail := regexp.MustCompile(`(?s)id="detail".*?</aside>`).FindString(page)
	if !strings.Contains(detail, "clear-pill") || !strings.Contains(detail, title) || !strings.Contains(detail, `href="/instances/default/podinfo?tab=graph"`) {
		t.Errorf("a selected node's panel lacks the pill: %q", detail)
	}
	rest := mainOf(s.get(t, "/instances/default/podinfo?tab=graph").body)
	if d := regexp.MustCompile(`(?s)id="detail".*?</aside>`).FindString(rest); strings.Contains(d, "clear-pill") {
		t.Error("the resting panel carries Clear selection")
	}
	frag := s.get(t, "/instances/default/podinfo/node?id="+url.QueryEscape(podinfoRoot), htmxRequest).body
	if strings.Contains(frag, "clear-pill") {
		t.Error("the resting node's fragment carries Clear selection")
	}
	frag = s.get(t, "/instances/default/podinfo/node?id="+url.QueryEscape(brokenService), htmxRequest).body
	if !strings.Contains(frag, "clear-pill") || !strings.Contains(frag, "data-clear-selection") {
		t.Error("a selected node's fragment lacks Clear selection")
	}
}

// TestGroupPanel: a group's panel keeps its health, says it is a display
// group, counts its objects per kind and offers Expand, or Collapse where
// the page has it open.
func TestGroupPanel(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	const rbac = "grp:configuration/" + certManagerID + "/rbac"
	frag := s.get(t, "/instances/cert-manager/cert-manager/node?id="+url.QueryEscape(rbac), htmxRequest).body
	for _, want := range []string{
		"A display group, not a Kubernetes object. It folds 13 configuration components so the workloads stay readable.",
		"<dt>ClusterRole</dt>", "Expand group", `class="health health-`,
	} {
		if !strings.Contains(frag, want) {
			t.Errorf("the group panel lacks %q:\n%s", want, frag)
		}
	}
	if strings.Contains(frag, "Collapse group") {
		t.Error("a closed group offers Collapse")
	}
	if strings.Contains(frag, "Edges") && strings.Contains(frag, "Stands for") {
		t.Error("a configuration group still says Stands for")
	}
	r := mustRequest(t, "/instances/cert-manager/cert-manager?tab=graph&expand="+url.QueryEscape(rbac))
	group := &v1.GraphNode{ID: rbac, Kind: "group", Group: &v1.GraphGroup{Kind: groupConfiguration, Family: "rbac", Members: []string{"a"}}}
	links := panelLinks(r, group, panelContext{base: "/instances/cert-manager/cert-manager"})
	if len(links) != 1 || links[0].Text != "Collapse group" || !strings.Contains(links[0].Href, "node=") || strings.Contains(links[0].Href, "expand=") {
		t.Errorf("an open group's links are %+v: want one Collapse group link that selects the group and drops it from expand", links)
	}
}

// TestRegistrationPanel: the standing, the controller's words in local mode
// only, the role, the catalog and the provider.
func TestRegistrationPanel(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	frag := s.get(t, "/instances/default/backup-provider/node?id="+url.QueryEscape(backupReg), htmxRequest).body
	for _, want := range []string{
		`<span class="prov prov-active">Accepted · active</span>`,
		"Claims a catalog for this provider; the controller decides whether it is accepted and active.",
		`href="/catalog?path=testing.opmodel.dev%2fcatalogs%2foperator%2fbackup%40v0"`,
		"default/backup-provider", "spec.providerRef names it",
	} {
		if !strings.Contains(strings.ToLower(frag), strings.ToLower(want)) {
			t.Errorf("the registration panel lacks %q:\n%s", want, frag)
		}
	}
	hold := newSite(t, apitest.F1PackageHolder(t), apitest.AllowAll)
	pkg := mainOf(hold.get(t, "/packages/pkg/provider?tab=graph").body)
	id := regexp.MustCompile(`data-node="(obj:opmodel.dev/TransformerRegistration[^"]*)"`).FindStringSubmatch(pkg)
	if id == nil {
		t.Fatal("the refused holder draws no registration node")
	}
	local := hold.get(t, "/packages/pkg/provider/node?id="+url.QueryEscape(id[1]), htmxRequest).body
	if !strings.Contains(local, "prov-refused") || !strings.Contains(local, "ProviderMismatch") || !strings.Contains(local, `class="tone-box"`) || !strings.Contains(local, "spec.providerRef does not name it") {
		t.Errorf("the refused claim's panel lacks its reason or the controller's message:\n%s", local)
	}
	cluster := newInClusterSite(t, apitest.F1PackageHolder(t), apitest.AllowAll)
	in := cluster.get(t, "/packages/pkg/provider/node?id="+url.QueryEscape(id[1]), htmxRequest).body
	if !strings.Contains(in, "prov-refused") || strings.Contains(in, "tone-box") {
		t.Errorf("an in-cluster registration panel must show the standing and no controller message:\n%s", in)
	}
	// Locked: a caller who may not read registrations sees the lock.
	deny := newSite(t, apitest.F1(t), apitest.DenyResources("transformerregistrations"))
	locked := deny.get(t, "/instances/default/backup-provider/node?id="+url.QueryEscape(backupReg), htmxRequest).body
	if !strings.Contains(locked, "Locked") || strings.Contains(locked, "prov-") {
		t.Errorf("a denied registration panel is not locked:\n%s", locked)
	}
}

// TestSourcePanel: the source's readiness words, the sentences for the
// states that are not readings, and the lock.
func TestSourcePanel(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	src := mainOf(s.get(t, "/packages/pkg/podinfo?tab=graph").body)
	id := regexp.MustCompile(`data-node="(src:[^"]*|[^"]*source[^"]*)"`).FindStringSubmatch(src)
	if id == nil {
		t.Fatal("the F1 package draws no source node")
	}
	frag := s.get(t, "/packages/pkg/podinfo/node?id="+url.QueryEscape(id[1]), htmxRequest).body
	if !strings.Contains(frag, "Kind not installed") || !strings.Contains(frag, "The cluster does not serve this kind. Install Flux source-controller, or point the package at a source kind the cluster has.") {
		t.Errorf("the not-installed source panel:\n%s", frag)
	}
	for _, c := range []struct{ state, want string }{
		{"unsupportedKind", "The controller reads only OCIRepository, GitRepository and Bucket."},
		{"noArtifact", "No artifact yet"},
		{"ready", "Artifact ready"},
		{"notReady", "ImagePullFailed"},
	} {
		p := sourcePanel(&v1.SourceState{Access: "ok", State: c.state, Reason: "ImagePullFailed", Message: "m", Revision: "sha256:0123456789abcdef"})
		if !strings.Contains(p.Status+p.Sentence, c.want) {
			t.Errorf("state %s reads %+v, want %q", c.state, p, c.want)
		}
	}
	// A source the caller may not read is a locked panel with no reading.
	h := &Handler{}
	g := &v1.Graph{Root: "p", Nodes: []v1.GraphNode{
		{ID: "p", Kind: "package", Label: "pkg/p"},
		{ID: "s", Kind: "source", Label: "OCIRepository s", Access: v1.AccessForbidden, SourceState: &v1.SourceState{Access: v1.AccessForbidden}},
	}}
	if lp := h.panel(mustRequest(t, "/"), g, "s", panelContext{base: "/b"}); !lp.Locked || lp.Src != nil || len(lp.Links) != 0 {
		t.Errorf("a forbidden source's panel %+v is not just locked", lp)
	}
}

// TestHostileNodeLabelAndTitleAreEscaped: a node's label is cluster text, in
// its line and in its title; both come out as text. The label is short, so
// the clip cannot hide a failure to escape it.
func TestHostileNodeLabelAndTitleAreEscaped(t *testing.T) {
	const label = `<b>x</b>&"'`
	d := testDrawing(v1.GraphNode{ID: "n", Kind: "object", Label: label, Ref: &v1.ObjectRef{Kind: "ConfigMap", Name: "x"}})
	n := d.nodeOf(&d.g.Nodes[0])
	n.Card.ID = "c"
	html := renderGraph(t, svgGraph{ID: "g", Width: 300, Height: 100, Nodes: []svgNode{n}})
	want := template.HTMLEscapeString(label)
	if !strings.Contains(html, `>`+want+`</text>`) {
		t.Errorf("the node's name line is not the escaped label %q", want)
	}
	if !strings.Contains(html, `<title>`+want+`</title>`) {
		t.Errorf("the node's title is not the escaped label %q", want)
	}
	if strings.Contains(html, "<b>x</b>") {
		t.Error("the label reached the markup as a tag")
	}
}
