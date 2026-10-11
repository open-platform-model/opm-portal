package ui

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/net/html"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// Tests of the owner tabs (order, selection params, counts) and of the
// Resources table (design.md decisions 4 and 5).

var tabLabelRe = regexp.MustCompile(`<a href="[^"]*"[^>]*>([A-Za-z]+)`)

// tabLabels lists the labels of the tab strip in page order.
func tabLabels(main string) []string {
	matches := tabLabelRe.FindAllStringSubmatch(tabsOf(main), -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

// TestOwnerTabOrder (4.1): Graph, Provider when the item holds a registration,
// Resources, Events, Logs and YAML, with the counts of F1's backup-provider.
func TestOwnerTabOrder(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	main := mainOf(s.get(t, "/instances/default/backup-provider").body)
	if got, want := strings.Join(tabLabels(main), ","), "Graph,Provider,Resources,Events,Logs,YAML"; got != want {
		t.Errorf("backup-provider tabs %s, want %s", got, want)
	}
	for _, want := range []string{`Resources <span class="tab-n" id="tab-n-resources"`, `>1</span></a><a href="/instances/default/backup-provider?tab=events">Events <span class="tab-n" id="tab-n-events"`, `>3</span>`} {
		if !strings.Contains(tabsOf(main), want) {
			t.Errorf("backup-provider tabs lack %q:\n%s", want, tabsOf(main))
		}
	}
	if got, want := strings.Join(tabLabels(mainOf(s.get(t, "/instances/default/podinfo").body)), ","), "Graph,Resources,Events,Logs,YAML"; got != want {
		t.Errorf("podinfo tabs %s, want %s", got, want)
	}
}

// TestEventsCountIsTheMergedCount (4.1): the Events tab counts the lines of
// the merged feed on every tab, and none when the feed left objects out (a
// caller who may not read the Pods gets a partial feed).
func TestEventsCountIsTheMergedCount(t *testing.T) {
	const follow = `instance:default/podinfo events:instance:default/podinfo`
	for _, tab := range []string{"graph", "resources", "logs", "yaml"} {
		main := mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/instances/default/podinfo?tab="+tab).body)
		if !strings.Contains(main, `id="tab-n-events" data-follow="`+follow+`">18</span>`) {
			t.Errorf("tab %s: the Events count is not the merged 18:\n%s", tab, tabsOf(main))
		}
	}
	partial := mainOf(newSite(t, apitest.F1(t), apitest.DenyResources("pods")).get(t, "/instances/default/podinfo").body)
	if want := `id="tab-n-events" data-follow="` + follow + `"></span>`; !strings.Contains(partial, want) {
		t.Errorf("a partial feed still carries an Events count (want an empty, followed span):\n%s", tabsOf(partial))
	}
}

// TestTabLinksKeepNodeAndFocus (4.1): every tab link carries node and focus
// when the page has them, and neither when it has none.
func TestTabLinksKeepNodeAndFocus(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	const node, focus = "obj:apps/Deployment/default/podinfo-podinfo", "obj:_/Service/default/podinfo-podinfo"
	q := url.Values{"tab": {"resources"}, "node": {node}, "focus": {focus}}.Encode()
	tabs := tabsOf(mainOf(s.get(t, "/instances/default/podinfo?"+q).body))
	for _, tab := range []string{"graph", "resources", "events", "logs", "yaml"} {
		want := `href="/instances/default/podinfo?` + strings.ReplaceAll(url.Values{"tab": {tab}, "node": {node}, "focus": {focus}}.Encode(), "&", "&amp;") + `"`
		if !strings.Contains(tabs, want) {
			t.Errorf("tab %s lost node or focus (want %s):\n%s", tab, want, tabs)
		}
	}
	plain := tabsOf(mainOf(s.get(t, "/instances/default/podinfo").body))
	if strings.Contains(plain, "node=") || strings.Contains(plain, "focus=") {
		t.Errorf("tab links invent a selection:\n%s", plain)
	}
}

// rowsOf lists the <tr class="res ..."> rows of the Resources table.
func rowsOf(main string) []string {
	parts := strings.Split(between(main, `id="components"`, "</section>"), `<tr class="res`)[1:]
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, `<tr class="res`+between(part, "", "</tr>"))
	}
	return out
}

// cell is the text of the cell of row labeled label, tags dropped.
func cell(row, label string) string {
	c := between(row, `data-label="`+label+`"`, "</t")
	c = c[strings.Index(c, ">")+1:]
	return strings.TrimSpace(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(c, ""))
}

// resourceRowWant is what a row of podinfo's Resources table shows.
type resourceRowWant struct{ kind, name, origin, child string }

// checkResourceRow holds row i to its want: the cells, the child mark, the
// Graph link on its name and its health.
func checkResourceRow(t *testing.T, i int, r string, w resourceRowWant) {
	t.Helper()
	if cell(r, "Kind") != w.kind || cell(r, "Name") != w.name || cell(r, "Origin") != w.origin || cell(r, "Component") != "podinfo" {
		t.Errorf("row %d: %s %s %s %s; want %v", i, cell(r, "Kind"), cell(r, "Name"), cell(r, "Component"), cell(r, "Origin"), w)
	}
	if !strings.HasPrefix(r, `<tr class="res`+w.child+`"`) {
		t.Errorf("row %d: class %q, want a child mark %q", i, between(r, `class="`, `"`), w.child)
	}
	if !regexp.MustCompile(`<a class="row-link" href="/instances/default/podinfo\?focus=[^"]+&amp;tab=graph">`).MatchString(r) {
		t.Errorf("row %d does not link to the Graph tab focused on its node:\n%s", i, r)
	}
	if !strings.Contains(cell(r, "Health"), "Healthy") {
		t.Errorf("row %d health %q", i, cell(r, "Health"))
	}
}

// TestResourcesTableRowsOnF1 (4.2): podinfo's rows, in order, with the
// columns of the table, children indented, a Graph link on each name and
// none of the old per-row YAML, Events and Logs links.
func TestResourcesTableRowsOnF1(t *testing.T) {
	main := mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/instances/default/podinfo?tab=resources").body)
	head := between(main, "<thead>", "</thead>")
	for _, col := range []string{"Kind", "Name", "Component", "Origin", "Health", "Details"} {
		if !strings.Contains(head, `scope="col">`+col+"</th>") {
			t.Errorf("the header lacks %s:\n%s", col, head)
		}
	}
	rows := rowsOf(main)
	want := []resourceRowWant{
		{"Deployment", "default/podinfo-podinfo", "applied", ""},
		{"Pod", "default/podinfo-podinfo-d9585d794-4lg6h", "made by the cluster", " row-child"},
		{"Pod", "default/podinfo-podinfo-d9585d794-kp2jx", "made by the cluster", " row-child"},
		{"ReplicaSet", "default/podinfo-podinfo-d9585d794", "made by the cluster", " row-child"},
		{"Service", "default/podinfo-podinfo", "applied", ""},
	}
	if len(rows) != len(want) {
		t.Fatalf("%d rows, want %d:\n%s", len(rows), len(want), main)
	}
	for i, w := range want {
		checkResourceRow(t, i, rows[i], w)
	}
	if !strings.Contains(cell(rows[3], "Details"), "2 desired") {
		t.Errorf("the ReplicaSet row lacks its desired count: %q", cell(rows[3], "Details"))
	}
	if !strings.Contains(rows[0], `title="Deployment is available. Replicas: 2"`) {
		t.Errorf("the message is not in full in its title:\n%s", rows[0])
	}
	for _, gone := range []string{`>YAML</a>`, `>Events</a>`, `>Logs</a>`, `data-log-link`, `name="reason"`} {
		if strings.Contains(between(main, `id="components"`, "</section>"), gone) {
			t.Errorf("the Resources tab still draws %q", gone)
		}
	}
}

// TestResourcesTableMarksTheSelectedRow (4.2): the row of the node object, else
// the focus object, is current; with neither, none is.
func TestResourcesTableMarksTheSelectedRow(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	const dep, svc = "obj:apps/Deployment/default/podinfo-podinfo", "obj:_/Service/default/podinfo-podinfo"
	current := func(path string) []string {
		var out []string
		for _, r := range rowsOf(mainOf(s.get(t, path).body)) {
			if strings.Contains(r, `aria-current="true"`) {
				out = append(out, cell(r, "Kind"))
			}
		}
		return out
	}
	for path, want := range map[string]string{
		"/instances/default/podinfo?tab=resources":                                                "",
		"/instances/default/podinfo?tab=resources&node=" + url.QueryEscape(dep):                   "Deployment",
		"/instances/default/podinfo?tab=resources&focus=" + url.QueryEscape(svc):                  "Service",
		"/instances/default/podinfo?tab=resources&node=" + url.QueryEscape(dep) + "&focus=" + svc: "Deployment",
	} {
		got := current(path)
		if (want == "" && len(got) != 0) || (want != "" && (len(got) != 1 || got[0] != want)) {
			t.Errorf("%s: current rows %v, want %q", path, got, want)
		}
	}
}

// TestResourcesTableDegradedRowAndReasonChip (4.2): a Degraded row carries the
// tint class, a reason filter keeps its branch and shows as a removable chip,
// and the old free-text input is gone.
func TestResourcesTableDegradedRowAndReasonChip(t *testing.T) {
	s := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	main := mainOf(s.get(t, "/instances/default/podinfo?tab=resources&reason=ImagePullBackOff").body)
	rows := rowsOf(main)
	if len(rows) != 2 {
		t.Fatalf("the reason filter keeps %d rows, want the Deployment and its waiting Pod:\n%s", len(rows), main)
	}
	if !strings.HasPrefix(rows[1], `<tr class="res row-child row-degraded"`) {
		t.Errorf("the waiting Pod's row is not marked degraded: %s", between(rows[1], "", ">"))
	}
	chip := between(main, `<div class="filter-chips"`, "</div>")
	if !strings.Contains(chip, `href="/instances/default/podinfo?tab=resources"`) || !strings.Contains(chip, "ImagePullBackOff") ||
		!strings.Contains(chip, `aria-label="Remove the filter Health reason: ImagePullBackOff"`) {
		t.Errorf("the reason is not a removable chip:\n%s", chip)
	}
	if strings.Contains(main, `<input`) {
		t.Error("the Resources tab keeps a free-text input")
	}
	none := mainOf(s.get(t, "/instances/default/podinfo?tab=resources&reason=NoSuchReason").body)
	if !strings.Contains(none, "No object or child reports this reason.") || strings.Contains(none, "<table") {
		t.Errorf("an unmatched reason:\n%s", between(none, `id="components"`, "</section>"))
	}
}

// TestPackageSourceRowOnAnEmptyInventory (4.2): pkg/podinfo has applied
// nothing; its first row is its source and the tab says it never fetched one.
func TestPackageSourceRowOnAnEmptyInventory(t *testing.T) {
	main := mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/packages/pkg/podinfo?tab=resources").body)
	rows := rowsOf(main)
	if len(rows) != 1 {
		t.Fatalf("%d rows, want just the source:\n%s", len(rows), main)
	}
	if cell(rows[0], "Kind") != "OCIRepository" || cell(rows[0], "Name") != "pkg/podinfo-release" || cell(rows[0], "Origin") != "source" {
		t.Errorf("source row: %s %s %s", cell(rows[0], "Kind"), cell(rows[0], "Name"), cell(rows[0], "Origin"))
	}
	if cell(rows[0], "Health") != "not read" {
		t.Errorf("a source node with no health reads %q, want not read", cell(rows[0], "Health"))
	}
	if !strings.Contains(rows[0], `href="/packages/pkg/podinfo?focus=src%3Asource.toolkit.fluxcd.io%2FOCIRepository%2Fpkg%2Fpodinfo-release&amp;tab=graph"`) {
		t.Errorf("the source row does not focus the source node:\n%s", rows[0])
	}
	if !strings.Contains(main, "Nothing applied yet. The package has never fetched its source.") {
		t.Errorf("the empty text:\n%s", between(main, `id="components"`, "</section>"))
	}
	if n := strings.Count(mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/packages/pkg/podinfo?tab=resources&reason=X").body), `<tr class="res`); n != 0 {
		t.Errorf("a reason filter keeps %d source rows", n)
	}
}

// TestEmptyInstanceReadsNothingApplied (4.2): an instance with no inventory
// draws no table and says an apply fills the graph.
func TestEmptyInstanceReadsNothingApplied(t *testing.T) {
	v := ownerView{Kind: instanceKind, Namespace: "default", Name: "app", Tab: tabResources}
	got := resourcesOf(nil, &v, &v1.Graph{})
	if got.Empty != "Nothing applied yet. The graph fills in once an apply succeeds." || len(got.Bodies) != 0 {
		t.Errorf("empty instance: %+v", got)
	}
	out := between(renderMain(t, "owner", ownerView{Kind: instanceKind, Namespace: "default", Name: "app", Tab: tabResources, Resources: got}), `id="components"`, "</section>")
	if strings.Contains(out, "<table") || !strings.Contains(out, "Nothing applied yet.") {
		t.Errorf("empty instance region:\n%s", out)
	}
}

// parseMain parses the page and returns the tbody elements of the Resources
// table with their rows; it fails the test when the markup is not valid enough
// for the parser to keep each group's rows in its tbody.
func tbodiesOf(t *testing.T, page string) []*html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tbody" {
			for p := n.Parent; p != nil; p = p.Parent {
				if p.Type == html.ElementNode && p.Data == "table" {
					if attr(p, "class") != "" && strings.Contains(attr(p, "class"), "resources-table") {
						out = append(out, n)
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out
}

func childrenNamed(n *html.Node, name string) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == name {
			out = append(out, c)
		}
	}
	return out
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

// firstElement finds the first element named name below n.
func firstElement(n *html.Node, name string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == name {
			return c
		}
		if f := firstElement(c, name); f != nil {
			return f
		}
	}
	return nil
}

// TestConfigurationGroupFoldsInATbody (4.2): cert-manager's configuration
// objects are one tbody whose header row holds a closed details (the group is
// healthy) and whose member rows are its siblings, so the stylesheet alone can
// hide them. The page parses with every member row inside that tbody.
func TestConfigurationGroupFoldsInATbody(t *testing.T) {
	page := mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/instances/cert-manager/cert-manager?tab=resources").body)
	var groups []*html.Node
	for _, tb := range tbodiesOf(t, page) {
		if attr(tb, "class") == "group" {
			groups = append(groups, tb)
		}
	}
	if len(groups) != 1 {
		t.Fatalf("%d group tbody elements, want the one configuration group", len(groups))
	}
	rows := childrenNamed(groups[0], "tr")
	if len(rows) < 2 || attr(rows[0], "class") != "group-head" {
		t.Fatalf("the group's first row is not its header: %d rows", len(rows))
	}
	d := firstElement(rows[0], "details")
	if d == nil || hasAttr(d, "open") || attr(d, "id") != "config-components" {
		t.Fatalf("the group's details: %+v (want closed, id config-components)", d)
	}
	for _, r := range rows[1:] {
		if !strings.Contains(attr(r, "class"), "member") {
			t.Errorf("a row of the group is not a member: %q", attr(r, "class"))
		}
	}
	if !strings.Contains(page, "17 configuration components") {
		t.Error("the group's summary lacks the node's label")
	}
	// The count in the tab is the whole list, folded or not.
	if n := len(rows) - 1; !strings.Contains(page, strconvItoa(n)+" rows") {
		t.Errorf("the group's summary does not count its %d member rows", n)
	}
}

func strconvItoa(n int) string { return itoa(int64(n)) }

// TestGroupOpensForAnUnhealthyMemberOrAReason (4.2): a group is open when its
// graph health is not Healthy, when a member row is not healthy, or when a
// reason filter is set; a workload's old revisions open on an unhealthy one.
func TestGroupOpensForAnUnhealthyMemberOrAReason(t *testing.T) {
	healthy := &v1.ObjectHealth{State: "Healthy", Reason: "Current"}
	bad := &v1.ObjectHealth{State: "Degraded", Reason: "Failed"}
	mk := func(h *v1.ObjectHealth) []componentView {
		return []componentView{{Name: "cfg", Objects: []objectView{{
			InventoryObject: v1.InventoryObject{Ref: v1.ObjectRef{Kind: "ConfigMap", Namespace: "n", Name: "c"}, Access: v1.AccessOK, Health: h, Live: true},
			Focus:           "/x?tab=graph", NodeID: "n1",
		}}}}
	}
	open := func(h *v1.ObjectHealth, graphHealth, reason string) bool {
		v := ownerView{Kind: instanceKind, Base: "/instances/n/app", Configs: []configGroup{{ID: "config-components", Label: "1 configuration components", Health: graphHealth, Open: graphHealth != "Healthy", Components: mk(h)}}}
		q := url.Values{}
		if reason != "" {
			q.Set("reason", reason)
		}
		got := resourcesOf(q, &v, &v1.Graph{})
		for _, b := range got.Bodies {
			if b.Head != nil {
				return b.Head.Open
			}
		}
		t.Fatal("no group body")
		return false
	}
	if open(healthy, "Healthy", "") {
		t.Error("a healthy group opens")
	}
	if !open(healthy, "Degraded", "") {
		t.Error("a group whose graph health is Degraded stays closed")
	}
	if !open(bad, "Healthy", "") {
		t.Error("a group with an unhealthy member stays closed")
	}
	if !open(healthy, "Healthy", "Current") {
		t.Error("a group stays closed under a reason filter")
	}

	// Old revisions.
	rs := func(state string) resourcesTable {
		o := objectView{InventoryObject: v1.InventoryObject{Ref: v1.ObjectRef{Group: "apps", Kind: "Deployment", Namespace: "n", Name: "d"}, Access: v1.AccessOK, Live: true}, OldID: "old_x"}
		zero := int64(0)
		o.Old = []childView{{RuntimeChild: v1.RuntimeChild{Ref: v1.ObjectRef{Group: "apps", Kind: "ReplicaSet", Namespace: "n", Name: "d-1"}, Health: v1.ObjectHealth{State: state}, Replicas: &zero}}}
		v := ownerView{Kind: instanceKind, Base: "/instances/n/app", Components: []componentView{{Name: "c", Objects: []objectView{o}}}}
		return resourcesOf(nil, &v, &v1.Graph{})
	}
	for state, want := range map[string]bool{"Healthy": false, "Degraded": true} {
		got := rs(state)
		var head *resHead
		for _, b := range got.Bodies {
			if b.Head != nil {
				head = b.Head
			}
		}
		if head == nil || head.Open != want || head.Label != "1 old revision" || head.ID != "old_x" {
			t.Errorf("old revision %s: head %+v, want open=%v", state, head, want)
		}
	}
}

// TestLockedResourceRowHasNoLink (4.2): an object the caller may not read is a
// row with the lock and the reason, and nothing to follow.
func TestLockedResourceRowHasNoLink(t *testing.T) {
	main := mainOf(newSite(t, apitest.F1(t), apitest.DenyResources("services")).get(t, "/instances/default/podinfo?tab=resources").body)
	var locked []string
	for _, r := range rowsOf(main) {
		if strings.Contains(r, "row-locked") {
			locked = append(locked, r)
		}
	}
	if len(locked) != 1 {
		t.Fatalf("%d locked rows, want the Service's", len(locked))
	}
	r := locked[0]
	if strings.Contains(r, "<a ") || !strings.Contains(r, "lock-mark") || !strings.Contains(cell(r, "Health"), "Locked") ||
		!strings.Contains(cell(r, "Details"), "Locked: you may not read this.") || cell(r, "Kind") != "Service" {
		t.Errorf("locked row:\n%s", r)
	}
}

// TestResourcesCSSHidesAClosedGroupWithoutScript (4.2): the stylesheet hides
// the member rows of a group whose details is closed, by CSS alone.
func TestResourcesCSSHidesAClosedGroupWithoutScript(t *testing.T) {
	css := readPortalCSS(t)
	for _, r := range parseCSS(css) {
		if r.selector == "tbody.group:has(details:not([open])) > tr.member" {
			if r.decls["display"] != "none" {
				t.Errorf("the rule sets display %q, want none", r.decls["display"])
			}
			return
		}
	}
	t.Error("portal.css has no rule hiding the members of a closed group")
}

// TestYAMLTabOpensOnTheSelectedObject (4.6): after a graph click or a
// Resources hand-off the YAML tab link carries node or focus, and the tab
// opens on that object's YAML without a pick; a Secret or an object the
// picker does not list opens nothing.
func TestYAMLTabOpensOnTheSelectedObject(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, q := range []string{"node=" + url.QueryEscape(nodeDeployment), "focus=" + url.QueryEscape(nodeDeployment)} {
		main := mainOf(s.get(t, "/instances/default/podinfo?tab=yaml&"+q).body)
		yaml := between(main, `id="yaml"`, "</section>")
		if !strings.Contains(yaml, `<p class="mono muted">Deployment.apps default/podinfo-podinfo</p>`) || !strings.Contains(yaml, "kind: Deployment") {
			t.Errorf("%s: the YAML tab did not open on the Deployment:\n%s", q, yaml)
		}
		if !strings.Contains(between(main, `id="yaml-picker"`, "</section>"), `aria-current="true"`) {
			t.Errorf("%s: the picker does not mark the open object", q)
		}
	}
	none := mainOf(s.get(t, "/instances/default/podinfo?tab=yaml&node="+url.QueryEscape("mi:default/podinfo")).body)
	if strings.Contains(between(none, `id="yaml"`, "</section>"), "kind: ") {
		t.Error("the owner's own node opens a YAML it does not serve")
	}
	picked := mainOf(s.get(t, "/instances/default/podinfo?tab=yaml&node="+url.QueryEscape(nodeDeployment)+"&object="+url.QueryEscape("/Service/default/podinfo-podinfo")).body)
	if !strings.Contains(between(picked, `id="yaml"`, "</section>"), "kind: Service") {
		t.Error("an explicit pick does not win over the selection")
	}
}
