package ui

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// The page script refreshes a region by replacing it with the region of
// the same id from one fresh render. These tests hold the markup to what
// that needs, without running the script.

var (
	idAttr     = regexp.MustCompile(`\sid="([^"]*)"`)
	followAttr = regexp.MustCompile(`id="(components|logs)"[^>]*data-follow="([^"]*)"`)
)

// TestALatePodGetsALogPane: a Pod that appears after the page loaded (the
// image-break rollout's new Pod) is listed in the logs region of the next
// render, and that region follows the same topic as the components region,
// so one refresh brings the Pod's row and its log pane together.
func TestALatePodGetsALogPane(t *testing.T) {
	const late = "podinfo-podinfo-794bd8c7fb-cc5ql"
	before := mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/instances/default/podinfo").body)
	if strings.Contains(before, late) {
		t.Fatalf("%s is already on the page before the rollout", late)
	}
	broken := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	after := mainOf(broken.get(t, "/instances/default/podinfo?tab=logs").body)
	resources := mainOf(broken.get(t, "/instances/default/podinfo?tab=resources").body)

	follows := map[string]string{}
	for _, m := range followAttr.FindAllStringSubmatch(after+resources, -1) {
		follows[m[1]] = m[2]
	}
	if follows["logs"] == "" || follows["logs"] != follows["components"] {
		t.Errorf("the logs region follows %q, the components region %q", follows["logs"], follows["components"])
	}

	logs := between(after, `id="logs"`, "</section>")
	// The Pod's row is in the Resources table, and its panel links to a pane
	// the Logs tab's select offers (the Logs link moved from the row to the panel).
	if !strings.Contains(between(resources, `id="components"`, "</section>"), late) {
		t.Errorf("the late Pod %s has no row in the Resources table", late)
	}
	panel := broken.get(t, "/instances/default/podinfo/node?id="+url.QueryEscape("obj:_/Pod/default/"+late), htmxRequest).body
	m := regexp.MustCompile(`href="/instances/default/podinfo\?tab=logs&amp;log=([^"]+)"`).FindStringSubmatch(panel)
	if len(m) < 2 || !strings.Contains(m[1], late) {
		t.Fatalf("the late Pod's panel has no Logs link to its pane:\n%s", panel)
	}
	if !strings.Contains(logs, `<option value="`+m[1]+`"`) {
		t.Errorf("Logs link %q names no option in the Pod / container select", m[1])
	}

	got := strings.TrimSpace(collapse(`<section class="panel panel-wide" `+logs+"</section>")) + "\n"
	golden := filepath.Join("testdata", "golden", "logs-podinfo-late-pod.html")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading %s (run with -update to write it): %v", golden, err)
	}
	if got != string(want) {
		t.Errorf("the late Pod's logs region differs from %s (run with -update after checking the diff):\n%s", golden, got)
	}
}

// TestElementIDsAreUnique: no rendered page repeats an element id, so a
// refresh, a Logs link and a kept-open fold each find the one element they
// name.
func TestElementIDsAreUnique(t *testing.T) {
	f1 := newSite(t, apitest.F1(t), apitest.AllowAll)
	broken := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	type page struct {
		s    *site
		path string
	}
	pages := []page{{broken, "/instances/default/podinfo"}}
	for _, p := range f1Pages {
		if !p.hx {
			pages = append(pages, page{f1, p.path})
		}
	}
	for _, p := range pages {
		seen := map[string]bool{}
		for _, m := range idAttr.FindAllStringSubmatch(p.s.get(t, p.path).body, -1) {
			if seen[m[1]] {
				t.Errorf("GET %s repeats id %q", p.path, m[1])
			}
			seen[m[1]] = true
		}
	}
}

// TestPaneIDsKeepNamesApart: the pane id of Pod a-b's container c is not
// the pane id of Pod a's container b-c.
func TestPaneIDsKeepNamesApart(t *testing.T) {
	if logID("a-b", "c") == logID("a", "b-c") {
		t.Errorf("logID collides: %q", logID("a-b", "c"))
	}
	if logID("a.b", "c") == logID("a", "b.c") {
		t.Errorf("logID collides: %q", logID("a.b", "c"))
	}
}

// TestOldRevisionsFoldHasAnID: the fold of scaled-down ReplicaSets carries
// an id, so a refresh keeps it open. F1 holds no scaled-down ReplicaSet, so
// the test adds a copy of podinfo's, scaled to zero.
func TestOldRevisionsFoldHasAnID(t *testing.T) {
	objs := apitest.F1(t)
	var rs *unstructured.Unstructured
	for _, o := range objs {
		if o.GetKind() == "ReplicaSet" && o.GetNamespace() == "default" && strings.HasPrefix(o.GetName(), "podinfo-podinfo-") {
			rs = o
			break
		}
	}
	if rs == nil {
		t.Fatal("F1 holds no podinfo ReplicaSet")
	}
	old := rs.DeepCopy()
	old.SetName("podinfo-podinfo-0ld0ld0ld")
	old.SetUID("old-revision")
	for _, f := range [][]string{{"spec", "replicas"}, {"status", "replicas"}, {"status", "readyReplicas"}, {"status", "availableReplicas"}} {
		if err := unstructured.SetNestedField(old.Object, int64(0), f...); err != nil {
			t.Fatal(err)
		}
	}
	objs = append(objs, old)
	res := newSite(t, objs, apitest.AllowAll).get(t, "/instances/default/podinfo?tab=resources")
	if res.status != http.StatusOK {
		t.Fatalf("GET = %d\n%s", res.status, res.body)
	}
	fold := regexp.MustCompile(`<details id="(old_[^"]+)"`).FindStringSubmatch(res.body)
	if fold == nil {
		t.Fatalf("no old-revisions fold with an id:\n%s", between(mainOf(res.body), `id="components"`, "</section>"))
	}
	if want := domID("old", "apps", "Deployment", "default", "podinfo-podinfo"); fold[1] != want {
		t.Errorf("fold id %q, want %q", fold[1], want)
	}
}

// TestNodeLinesFitTheirBox: the three lines of a node are clipped to what
// fits a 216 px box, the kind line short enough to leave the applied
// square its corner, and a clipped name keeps its tail, which tells
// siblings apart.
func TestNodeLinesFitTheirBox(t *testing.T) {
	d := &drawing{g: &v1.Graph{Layout: v1.GraphLayout{NodeWidth: 216, NodeHeight: 72}}, byID: map[string]*v1.GraphNode{}}
	ready := v1.Reconcile{}
	n := d.nodeOf(&v1.GraphNode{
		ID: "a", Kind: "object", Label: "a-very-long-name-that-is-not-short-enough-podinfo-7c9d8f",
		Ref:       &v1.ObjectRef{Kind: "MutatingWebhookConfigurationList", Name: "x"},
		Reconcile: &ready,
		Health:    &v1.GraphHealth{State: "Degraded", Reason: "ProgressDeadlineExceededAtTheRolloutOfTheDeployment"},
	})
	for name, got := range map[string]struct {
		text string
		max  int
	}{"kind": {n.Kind, maxKind}, "name": {n.Label, maxLabel}, "status": {n.Status, maxStatus}} {
		if r := len([]rune(got.text)); r > got.max {
			t.Errorf("%s line %q is %d characters, want at most %d", name, got.text, r, got.max)
		}
	}
	if !strings.HasSuffix(n.Label, "8f") || !n.Applied {
		t.Errorf("name %q lost its tail, or the applied square is missing (%v)", n.Label, n.Applied)
	}
}

// TestContractsBannerIsNeutral: unfulfilled contracts on the Platform read
// as neutral information, in the banner as in the informational condition
// tone, never in a state color.
func TestContractsBannerIsNeutral(t *testing.T) {
	css, err := os.ReadFile("static/portal.css")
	if err != nil {
		t.Fatal(err)
	}
	rule := regexp.MustCompile(`(?m)^\.info \{[^}]*\}`).FindString(string(css))
	if !strings.Contains(rule, "var(--neutral)") || regexp.MustCompile(`--(healthy|degraded|progressing|unknown)`).MatchString(rule) {
		t.Errorf("the contracts banner is not neutral: %s", rule)
	}
}

// TestLogsRegionShape holds the owner page's logs region to the shape the
// page script's refresh and TestBrowserLogs' page rely on: the region's
// direct child .logs holds every pane as its direct child, and each pane
// carries its summary, its tools and its pre. A pane moved off .logs is
// replaced on refresh instead of kept, which resets its scroll.
func TestLogsRegionShape(t *testing.T) {
	f1 := newSite(t, apitest.F1(t), apitest.AllowAll)
	broken := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	for _, c := range []struct {
		s     *site
		path  string
		panes bool
	}{
		{f1, "/instances/default/podinfo?tab=logs", true},
		{f1, "/instances/cert-manager/cert-manager?tab=logs", true},
		{f1, "/instances/web/web?tab=logs", true},
		{broken, "/instances/default/podinfo?tab=logs", true},
		{f1, "/packages/pkg/podinfo?tab=logs", false},
	} {
		doc, err := html.Parse(strings.NewReader(c.s.get(t, c.path).body))
		if err != nil {
			t.Fatalf("GET %s: %v", c.path, err)
		}
		region := findByID(doc, "logs")
		if region == nil || region.Data != "section" {
			t.Errorf("GET %s: no section#logs", c.path)
			continue
		}
		lists := children(region, func(n *html.Node) bool { return hasClass(n, "logs") })
		if len(lists) != 1 {
			t.Errorf("GET %s: #logs has %d .logs direct children; want 1", c.path, len(lists))
			continue
		}
		direct := children(lists[0], isPane)
		for _, d := range direct {
			checkPane(t, c.path, d)
		}
		all := descendants(region, isPane)
		if len(all) != len(direct) || (len(direct) > 0) != c.panes {
			t.Errorf("GET %s: %d log panes, %d of them direct children of .logs; want all direct, panes %t", c.path, len(all), len(direct), c.panes)
		}
	}
}

func isPane(n *html.Node) bool { return n.Data == "details" && hasClass(n, "log") }

// checkPane: a pane has an id and a log topic, and its summary, its tools
// (the previous-instance box and the state) and its pre as direct children.
func checkPane(t *testing.T, path string, d *html.Node) {
	t.Helper()
	if attr(d, "id") == "" || attr(d, "data-log-topic") == "" {
		t.Errorf("GET %s: a log pane lacks its id or data-log-topic", path)
	}
	summary := children(d, func(n *html.Node) bool { return n.Data == "summary" })
	pre := children(d, func(n *html.Node) bool { return n.Data == "pre" && hasClass(n, "log-pane") })
	var tools bool
	for _, k := range children(d, func(n *html.Node) bool { return n.Data == "div" && hasClass(n, "log-tools") }) {
		previous := descendants(k, func(n *html.Node) bool { return n.Data == "input" && attr(n, "data-log-previous") != "" })
		state := descendants(k, func(n *html.Node) bool { return hasClass(n, "log-state") })
		tools = tools || (len(previous) == 1 && len(state) == 1)
	}
	if len(summary) != 1 || !tools || len(pre) != 1 {
		t.Errorf("GET %s: pane %s: %d summaries, tools %t, %d pres; want one of each", path, attr(d, "id"), len(summary), tools, len(pre))
	}
}

// children returns n's element children that match.
func children(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	for k := range n.ChildNodes() {
		if k.Type == html.ElementNode && match(k) {
			out = append(out, k)
		}
	}
	return out
}

// descendants returns n's element descendants that match.
func descendants(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	for d := range n.Descendants() {
		if d.Type == html.ElementNode && match(d) {
			out = append(out, d)
		}
	}
	return out
}

func findByID(n *html.Node, id string) *html.Node {
	if found := descendants(n, func(d *html.Node) bool { return attr(d, "id") == id }); len(found) > 0 {
		return found[0]
	}
	return nil
}

func hasClass(n *html.Node, class string) bool {
	return slices.Contains(strings.Fields(attr(n, "class")), class)
}

// TestARefreshKeepsTheTabAndTheFocus: a refresh fetches the page's own
// address, which carries the tab and the focus, so the fresh render holds
// every region the open page follows, with the same ids, the same tab and
// the same spotlight; regions of other tabs are not in it.
func TestARefreshKeepsTheTabAndTheFocus(t *testing.T) {
	s := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	followed := regexp.MustCompile(`<[a-z]+ [^>]*id="([^"]+)"[^>]*data-follow="([^"]+)"`)
	for _, path := range []string{
		"/instances/default/podinfo?tab=graph&focus=obj:apps/Deployment/default/podinfo-podinfo",
		"/instances/default/podinfo?tab=resources&reason=ImagePullBackOff",
		"/instances/default/podinfo?tab=events&type=Warning",
		"/instances/default/podinfo?tab=logs",
	} {
		page := mainOf(s.get(t, path).body)
		fresh := mainOf(s.get(t, path).body)
		regions := followed.FindAllStringSubmatch(page, -1)
		if len(regions) < 4 {
			t.Errorf("%s: %d followed regions; want the cards and the tab's", path, len(regions))
		}
		for _, m := range regions {
			if !strings.Contains(fresh, `id="`+m[1]+`"`) {
				t.Errorf("%s: the fresh render lacks region %s", path, m[1])
			}
		}
		for _, card := range []string{"applied-card", "health-card"} {
			if !strings.Contains(page, `id="`+card+`"`) {
				t.Errorf("%s: no %s to refresh", path, card)
			}
		}
	}
	focused := mainOf(s.get(t, "/instances/default/podinfo?tab=graph&focus=obj:apps/Deployment/default/podinfo-podinfo").body)
	if !regexp.MustCompile(`data-node="obj:apps/Deployment/default/podinfo-podinfo"[^>]*aria-current="true"`).MatchString(focused) ||
		!strings.Contains(focused, `class="node kind-object health-degraded dim"`) && !strings.Contains(focused, " dim\"") {
		t.Error("the focused render does not mark the node or dim the rest")
	}
	if !strings.Contains(focused, `>Clear selection</a>`) {
		t.Error("the focused render offers no Clear selection")
	}
}

// TestANewEventReplacesTheTabCountAndNothingElse (portal:D19:R3): the live
// refresh replaces only elements that carry an id and data-follow, so a new
// event on events:<topic> swaps the Events count and leaves the strip's
// links, and their attributes, as they were.
func TestANewEventReplacesTheTabCountAndNothingElse(t *testing.T) {
	const path = "/instances/default/podinfo?tab=resources"
	objs := apitest.F1(t)
	more := slices.Clone(objs)
	for _, o := range objs {
		// An event of the ModuleInstance itself: the feed the Events tab opens with.
		if name, _, _ := unstructured.NestedString(o.Object, "regarding", "name"); o.GetKind() != "Event" || o.GetNamespace() != "default" || name != "podinfo" {
			continue
		}
		extra := o.DeepCopy()
		extra.SetName("podinfo.new-event")
		extra.SetUID("00000000-0000-0000-0000-00000000e1e1")
		if err := unstructured.SetNestedField(extra.Object, "BackOff", "reason"); err != nil {
			t.Fatal(err)
		}
		if err := unstructured.SetNestedField(extra.Object, "Back-off restarting failed container", "note"); err != nil {
			t.Fatal(err)
		}
		more = append(more, extra)
		break
	}
	if len(more) == len(objs) {
		t.Fatal("F1 holds no event of podinfo to copy")
	}
	before := mainOf(newSite(t, objs, apitest.AllowAll).get(t, path).body)
	after := mainOf(newSite(t, more, apitest.AllowAll).get(t, path).body)

	count := regexp.MustCompile(`<span class="tab-n" id="tab-n-events" data-follow="([^"]*)">(\d+)</span>`)
	b, a := count.FindStringSubmatch(before), count.FindStringSubmatch(after)
	if b == nil || a == nil {
		t.Fatalf("no Events count in the strips:\n%s\n%s", tabsOf(before), tabsOf(after))
	}
	if !slices.Contains(strings.Fields(b[1]), "events:instance:default/podinfo") {
		t.Errorf("the Events count follows %q, not events:instance:default/podinfo", b[1])
	}
	if b[2] == a[2] {
		t.Errorf("the Events count stayed %s after a new event", b[2])
	}
	// The strip itself carries no id and no data-follow, so a refresh cannot
	// replace it; with the counts taken out the two strips are identical.
	nav := regexp.MustCompile(`<nav class="tabs[^>]*>`).FindString(before)
	if strings.Contains(nav, "id=") || strings.Contains(nav, "data-follow") {
		t.Errorf("the strip is a followed region: %s", nav)
	}
	strip := func(s string) string { return count.ReplaceAllString(tabsOf(s), "") }
	if strip(before) != strip(after) {
		t.Errorf("the strips differ beyond the count:\n%s\n%s", strip(before), strip(after))
	}
}

// TestARefreshKeepsTheOpenGroupAndTheSelection: the refresh fetches the
// page's own address, so the fresh render holds the same open group, the
// same frame and the same selected node as the page it replaces.
func TestARefreshKeepsTheOpenGroupAndTheSelection(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	path := "/instances/cert-manager/cert-manager?tab=graph&expand=" + url.QueryEscape(cmRBAC)
	page := mainOf(s.get(t, path).body)
	member := regexp.MustCompile(`data-node="([^"]*)" data-member-of=`).FindStringSubmatch(page)
	if member == nil {
		t.Fatal("no member in the open group")
	}
	path += "&node=" + url.QueryEscape(member[1])
	for i, body := range []string{mainOf(s.get(t, path).body), mainOf(s.get(t, path).body)} {
		if strings.Count(body, `class="group-frame-box"`) != 1 || !strings.Contains(body, `data-frame-for="`+cmRBAC+`"`) {
			t.Errorf("render %d: the open group has no frame", i)
		}
		if !regexp.MustCompile(`data-node="` + regexp.QuoteMeta(member[1]) + `"[^>]*aria-current="true"`).MatchString(body) {
			t.Errorf("render %d: the selected member is not marked", i)
		}
	}
}

// TestTheCountIsAFollowedRegionAndTheLiveNoteIsNot (installed-06, PIC-L1): the
// live refresh replaces only elements that carry an id and data-follow, so
// "Showing n of M" in a filter form refreshes with its list and leaves the
// fields beside it alone, and the hidden count note in the header's status
// region has neither, so no refresh can write it. The header's region is the
// one status region on the page, not atomic, and holds the note beside the
// stream's text.
func TestTheCountIsAFollowedRegionAndTheLiveNoteIsNot(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	region := regexp.MustCompile(`<p class="shown" id="shown" data-follow="([^"]+)">`)
	for path, list := range map[string]string{
		"/installed":              `id="list"`,
		"/installed?q=zzz":        `id="list"`,
		"/":                       `id="providers"`,
		"/?tab=catalogs":          `id="catalogs"`,
		"/installed?kind=package": `id="list"`,
	} {
		page := s.get(t, path).body
		m := region.FindStringSubmatch(mainOf(page))
		if m == nil {
			t.Errorf("%s: no followed count region", path)
			continue
		}
		after := between(mainOf(page), list, ">")
		if !strings.Contains(after, `data-follow="`+m[1]+`"`) {
			t.Errorf("%s: the count follows %q, the list %s", path, m[1], after)
		}
		if n := strings.Count(page, `role="status"`); n != 1 {
			t.Errorf("%s: %d status regions; the header's is the only one", path, n)
		}
		live := between(page, `<p class="live"`, "</p>")
		if !strings.Contains(live, `id="live" role="status" aria-live="polite" aria-atomic="false"`) ||
			!strings.Contains(live, `<span class="live-text">Not live</span><span class="live-note"></span>`) {
			t.Errorf("%s: the status region is not the atomic-free text and note: %s", path, live)
		}
	}
}

// TestANewPlatformEventReplacesTheEventsCount (platform-24): the Platform
// page's "Showing n of M" for Recent events is its own followed region,
// `#events-shown`, with the topics of the feed beside it, so a new event
// changes its text and the refresh that replaces it needs nothing else.
func TestANewPlatformEventReplacesTheEventsCount(t *testing.T) {
	objs := apitest.F1(t)
	more := slices.Clone(objs)
	for _, o := range objs {
		if kind, _, _ := unstructured.NestedString(o.Object, "regarding", "kind"); o.GetKind() != "Event" || kind != "Platform" {
			continue
		}
		extra := o.DeepCopy()
		extra.SetName("platform.new-event")
		extra.SetUID("00000000-0000-0000-0000-00000000e2e2")
		if err := unstructured.SetNestedField(extra.Object, "NewReason", "reason"); err != nil {
			t.Fatal(err)
		}
		more = append(more, extra)
		break
	}
	if len(more) == len(objs) {
		t.Fatal("F1 holds no event of the Platform to copy")
	}
	count := regexp.MustCompile(`(?s)<p class="shown" id="events-shown" data-follow="([^"]*)">(.*?)</p>`)
	before := count.FindStringSubmatch(mainOf(newSite(t, objs, apitest.AllowAll).get(t, "/").body))
	after := count.FindStringSubmatch(mainOf(newSite(t, more, apitest.AllowAll).get(t, "/").body))
	if before == nil || after == nil {
		t.Fatal("the Platform page has no followed #events-shown region")
	}
	if !slices.Contains(strings.Fields(before[1]), "events:platform") {
		t.Errorf("the count follows %q, not events:platform", before[1])
	}
	if before[2] == after[2] || !strings.Contains(after[2], "<strong>3</strong> of 3") {
		t.Errorf("the count did not follow the new event:\n%s\n%s", before[2], after[2])
	}
}

// TestANewEventReplacesTheCatalogEventsCount: the Catalog page's Events count
// is a followed region, and a new Platform event changes it.
func TestANewEventReplacesTheCatalogEventsCount(t *testing.T) {
	objs := apitest.F1(t)
	more := slices.Clone(objs)
	for _, o := range objs {
		if kind, _, _ := unstructured.NestedString(o.Object, "regarding", "kind"); o.GetKind() != "Event" || kind != "Platform" {
			continue
		}
		extra := o.DeepCopy()
		extra.SetName("platform.new-catalog-event")
		extra.SetUID("00000000-0000-0000-0000-00000000e2e3")
		if err := unstructured.SetNestedField(extra.Object, "NewReason", "reason"); err != nil {
			t.Fatal(err)
		}
		more = append(more, extra)
		break
	}
	if len(more) == len(objs) {
		t.Fatal("F1 holds no event of the Platform to copy")
	}
	const path = "/catalog?path=testing.opmodel.dev/catalogs/operator/refused-claim-fixture-absent@v0&tab=events"
	count := regexp.MustCompile(`(?s)<p class="shown" id="events-shown" data-follow="([^"]*)">(.*?)</p>`)
	before := count.FindStringSubmatch(mainOf(newSite(t, objs, apitest.AllowAll).get(t, path).body))
	after := count.FindStringSubmatch(mainOf(newSite(t, more, apitest.AllowAll).get(t, path).body))
	if before == nil || after == nil {
		t.Fatal("the Catalog Events tab has no followed #events-shown region")
	}
	if !slices.Contains(strings.Fields(before[1]), "events:platform") {
		t.Errorf("the count follows %q, not events:platform", before[1])
	}
	if before[2] == after[2] || !strings.Contains(after[2], "<strong>3</strong> of 3") {
		t.Errorf("the count did not follow the new event:\n%s\n%s", before[2], after[2])
	}
}
