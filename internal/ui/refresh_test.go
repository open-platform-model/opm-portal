package ui

import (
	"net/http"
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
	logLinkRe  = regexp.MustCompile(`data-log-link="([^"]*)"`)
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
	after := mainOf(newSite(t, apitest.F1Broken(t), apitest.AllowAll).get(t, "/instances/default/podinfo").body)

	follows := map[string]string{}
	for _, m := range followAttr.FindAllStringSubmatch(after, -1) {
		follows[m[1]] = m[2]
	}
	if follows["logs"] == "" || follows["logs"] != follows["components"] {
		t.Errorf("the logs region follows %q, the components region %q", follows["logs"], follows["components"])
	}

	logs := between(after, `id="logs"`, "</section>")
	ids := map[string]bool{}
	for _, m := range idAttr.FindAllStringSubmatch(logs, -1) {
		ids[m[1]] = true
	}
	links := logLinkRe.FindAllStringSubmatch(between(after, `id="components"`, "</section>"), -1)
	if len(links) == 0 {
		t.Fatal("no Pod row offers its logs")
	}
	var lateLink bool
	for _, m := range links {
		if !ids[m[1]] {
			t.Errorf("Logs link %q names no pane in the logs region", m[1])
		}
		lateLink = lateLink || strings.Contains(m[1], late)
	}
	if !lateLink {
		t.Errorf("the late Pod %s has no Logs link", late)
	}

	got := strings.TrimSpace(collapse(`<section class="panel panel-wide reveal" `+logs+"</section>")) + "\n"
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
	res := newSite(t, objs, apitest.AllowAll).get(t, "/instances/default/podinfo")
	if res.status != http.StatusOK {
		t.Fatalf("GET = %d\n%s", res.status, res.body)
	}
	fold := regexp.MustCompile(`<details class="old-revs" id="([^"]+)">`).FindStringSubmatch(res.body)
	if fold == nil {
		t.Fatalf("no old-revisions fold with an id:\n%s", between(mainOf(res.body), `id="components"`, "</section>"))
	}
	if want := domID("old", "apps", "Deployment", "default", "podinfo-podinfo"); fold[1] != want {
		t.Errorf("fold id %q, want %q", fold[1], want)
	}
}

// TestStampedLabelsLeaveRoomForTheStamp: a node with an applied stamp
// clips its label shorter, so the stamp does not cover the label's end.
func TestStampedLabelsLeaveRoomForTheStamp(t *testing.T) {
	ready := v1.Reconcile{}
	label := "cert-manager/cert-manager"
	stamped := nodeOf(&v1.GraphNode{ID: "a", Kind: "instance", Label: label, Reconcile: &ready}, 200, 44, "/", "/node")
	plain := nodeOf(&v1.GraphNode{ID: "b", Kind: "instance", Label: label}, 200, 44, "/", "/node")
	if n := len([]rune(stamped.Label)); !stamped.Applied || n > stampedLabel {
		t.Errorf("stamped label %q is %d characters, want at most %d", stamped.Label, n, stampedLabel)
	}
	if n := len([]rune(plain.Label)); n > maxLabel || n <= stampedLabel {
		t.Errorf("unstamped label %q is %d characters, want %d to %d", plain.Label, n, stampedLabel+1, maxLabel)
	}
}

// TestCatalogSubLineShowsTheHost: two catalogs that share their path's
// tail but not its host read apart on the sub-line.
func TestCatalogSubLineShowsTheHost(t *testing.T) {
	for _, tc := range []struct{ label, want string }{
		{"opmodel.dev/catalogs/operator", "Catalog · opmodel.dev/…"},
		{"testing.opmodel.dev/catalogs/operator", "Catalog · testing.opmodel.d…"},
		{"opmodel.dev/operator", "Catalog · opmodel.dev"},
	} {
		_, sub := displayLabel(&v1.GraphNode{Kind: "catalog", Label: tc.label}, "Catalog", false)
		if sub != tc.want {
			t.Errorf("%s: sub-line %q, want %q", tc.label, sub, tc.want)
		}
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
		{f1, "/instances/default/podinfo", true},
		{f1, "/instances/cert-manager/cert-manager", true},
		{f1, "/instances/web/web", true},
		{broken, "/instances/default/podinfo", true},
		{f1, "/packages/pkg/podinfo", false},
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
