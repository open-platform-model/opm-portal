package ui

import (
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// The details panel follows its page while it shows a graph node, and a YAML
// or Events view in it says when it was read. These tests hold the markup to
// what the page script needs; the script itself is run by TestBrowserGraph
// and TestBrowserStale.

const (
	podinfoTopic = "instance:default/podinfo"
	yamlPath     = "/instances/default/podinfo/object?group=apps&kind=Deployment&namespace=default&name=podinfo-podinfo"
	eventsPath   = "/instances/default/podinfo/events?kind=Pod&namespace=default&name=podinfo-podinfo-d9585d794-4lg6h"
)

var (
	detailTag   = regexp.MustCompile(`<aside class="panel detail"[^>]*>`)
	staleNoteRe = regexp.MustCompile(`<p class="stale-note" hidden>Read <time datetime="([^"]*)" title="([^"]*)" data-relative aria-live="off">([^<]*)</time>\. The object may have changed since\. <a href="([^"]*)" hx-get="([^"]*)" hx-target="#detail" hx-select="unset" hx-swap="innerHTML" data-focus-key="stale-reload">Reload this view</a></p>`)
)

// TestTheDetailPanelFollowsItsPage: a node panel, the resting one included,
// names the topic of its page twice: data-topics for the note of a view, and
// data-follow for the refresh. The empty placeholder names neither.
func TestTheDetailPanelFollowsItsPage(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	want := `data-topics="` + podinfoTopic + `" data-follow="` + podinfoTopic + `"`
	for _, path := range []string{
		"/instances/default/podinfo?tab=graph",
		"/instances/default/podinfo?tab=graph&node=" + url.QueryEscape("obj:apps/Deployment/default/podinfo-podinfo"),
		"/instances/default/podinfo?tab=resources",
		"/instances/default/podinfo?tab=resources&focus=" + url.QueryEscape("obj:apps/Deployment/default/podinfo-podinfo"),
	} {
		tag := detailTag.FindString(s.get(t, path).body)
		if !strings.Contains(tag, want) {
			t.Errorf("%s: the panel does not follow its page:\n%s", path, tag)
		}
	}
	pkg := detailTag.FindString(s.get(t, "/packages/pkg/podinfo?tab=graph").body)
	if !strings.Contains(pkg, `data-topics="package:pkg/podinfo" data-follow="package:pkg/podinfo"`) {
		t.Errorf("a package's panel does not follow its page:\n%s", pkg)
	}

	h := &Handler{cfg: Config{Now: func() time.Time { return apitest.Now }}}
	pages, err := parsePages(h.templateFuncs())
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := pages["owner"].ExecuteTemplate(&b, "detail", ownerView{Topic: podinfoTopic}); err != nil {
		t.Fatal(err)
	}
	if got := b.String(); !strings.Contains(got, "no node to show") || strings.Contains(got, "data-topics") || strings.Contains(got, "data-follow") {
		t.Errorf("the empty placeholder follows the page:\n%s", got)
	}
}

// TestAViewSaysWhenItWasRead: the YAML and Events fragments mark their root
// and carry a hidden note whose time is the handler's clock, in the form of
// the time partial, and a link that renders the same address again.
func TestAViewSaysWhenItWasRead(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, c := range []struct{ view, path string }{{"yaml", yamlPath}, {"events", eventsPath}} {
		res := s.get(t, c.path, htmxRequest)
		if res.status != 200 {
			t.Fatalf("%s = %d", c.path, res.status)
		}
		if !strings.Contains(res.body, `<div class="fragment" data-view="`+c.view+`">`) {
			t.Errorf("%s: the fragment root is not marked %q", c.view, c.view)
		}
		m := staleNoteRe.FindStringSubmatch(res.body)
		if m == nil {
			t.Fatalf("%s: no hidden note in the form of the time partial:\n%s", c.view, between(res.body, "stale-note", "</p>"))
		}
		if m[1] != "2026-10-04T18:10:00Z" || m[2] != "2026-10-04 18:10:00 UTC" || m[3] != "0s ago" {
			t.Errorf("%s: the note's time is %q %q %q, want the handler's clock", c.view, m[1], m[2], m[3])
		}
		// The link renders the address that was asked, as the browser sent it.
		asked, err := url.Parse(c.path)
		if err != nil {
			t.Fatal(err)
		}
		href, err := url.Parse(strings.ReplaceAll(m[4], "&amp;", "&"))
		if err != nil {
			t.Fatal(err)
		}
		if href.Path != asked.Path || href.Query().Encode() != asked.Query().Encode() || m[4] != m[5] {
			t.Errorf("%s: the reload link is %q, want the address %q", c.view, m[4], c.path)
		}
	}
}

// TestAFailedViewHasNoNote: a view that could not be read is not a view of
// anything, so it is neither marked nor offered a reload.
func TestAFailedViewHasNoNote(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	res := s.get(t, "/instances/default/podinfo/object?group=apps&kind=Deployment&namespace=default&name=nothing-here", htmxRequest)
	if !strings.Contains(res.body, "locked-panel") {
		t.Fatalf("the view of an object outside the instance is not a problem:\n%s", res.body)
	}
	if strings.Contains(res.body, "data-view") || strings.Contains(res.body, "stale-note") {
		t.Errorf("a failed view is marked or has a note:\n%s", res.body)
	}
}

// TestTheNoteEscapesTheAddress: the reload link is the request's own address,
// escaped like every attribute; a hostile query reaches neither markup nor a
// second attribute.
func TestTheNoteEscapesTheAddress(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	// The API ignores a parameter it does not know, so the view still renders.
	hostile := `x"><script>alert(1)</script>`
	path := eventsPath + "&zzz=" + url.QueryEscape(hostile)
	res := s.get(t, path, htmxRequest)
	if res.status != 200 {
		t.Fatalf("%s = %d", path, res.status)
	}
	if strings.Contains(res.body, "<script>alert") || strings.Contains(res.body, `x">`) {
		t.Errorf("a hostile name reaches the markup:\n%s", between(res.body, "stale-note", "</p>"))
	}
	m := staleNoteRe.FindStringSubmatch(res.body)
	if m == nil {
		t.Fatalf("no note for a hostile name:\n%s", between(res.body, "stale-note", "</p>"))
	}
	if !strings.HasPrefix(m[4], "/instances/default/podinfo/events?") || strings.ContainsAny(m[4], `"<>`) {
		t.Errorf("the reload link is not the escaped address: %q", m[4])
	}
	// The value reaches the link as the viewer sent it, no more and no less.
	href, err := url.Parse(html.UnescapeString(m[4]))
	if err != nil || href.Query().Get("zzz") != hostile {
		t.Errorf("the reload link does not carry the query back unchanged: %q (%v)", m[4], err)
	}
}

// TestThePanelLinksCarryTheirFocusKey: each link of a node panel and the
// Clear selection controls carry the key the script refocuses by, since none
// has an id and the refresh replaces the panel.
func TestThePanelLinksCarryTheirFocusKey(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	dep := panelOf(t, s, "/instances/default/podinfo", "obj:apps/Deployment/default/podinfo-podinfo")
	for text, key := range map[string]string{"Events": "panel-events", "YAML": "panel-yaml"} {
		if !regexp.MustCompile(`<a class="btn" href="[^"]*" data-focus-key="` + key + `"[^>]*>` + text + `</a>`).MatchString(dep) {
			t.Errorf("the %s link has no key %q:\n%s", text, key, between(dep, "panel-links", "</p>"))
		}
	}
	// No F1 node offers Open page or a group switch, so the links are built directly.
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/instances/default/podinfo?tab=graph", nil)
	ref := &v1.ObjectRef{Namespace: "other", Name: "web"}
	open := panelLinks(r, &v1.GraphNode{Kind: instanceKind.Topic, Ref: ref}, panelContext{base: "/instances/default/podinfo"})
	if len(open) != 1 || open[0].Text != "Open page" || open[0].Key != "panel-open" {
		t.Errorf("an instance node's links = %+v, want Open page keyed panel-open", open)
	}
	group := panelLinks(r, &v1.GraphNode{ID: "grp", Group: &v1.GraphGroup{}}, panelContext{base: "/instances/default/podinfo"})
	if len(group) == 0 || group[len(group)-1].Key != "panel-expand" {
		t.Errorf("a group node's links = %+v, want the switch keyed panel-expand", group)
	}
	pod := panelOf(t, s, "/instances/default/podinfo", nodePod)
	if !strings.Contains(pod, `data-focus-key="panel-logs"`) {
		t.Error("the Logs link of a Pod has no key")
	}
	sel := mainOf(s.get(t, "/instances/default/podinfo?tab=graph&node="+url.QueryEscape("obj:apps/Deployment/default/podinfo-podinfo")).body)
	if !strings.Contains(sel, `data-clear-selection data-focus-key="panel-clear"`) {
		t.Error("the Clear selection pill of the panel has no key")
	}
	if !strings.Contains(sel, `data-clear-selection data-focus-key="clear-instance-graph"`) {
		t.Error("the Clear selection link of the graph has no key")
	}
}

// TestALockedPanelOffersNoView: a node the caller may not read has no YAML or
// Events link, no view and no note, but it still follows its page.
func TestALockedPanelOffersNoView(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.DenyResources("services"))
	svc := panelOf(t, s, "/instances/default/podinfo", nodeService)
	for _, bad := range []string{">YAML<", ">Events<", "data-view", "stale-note", "data-focus-key=\"panel-yaml\""} {
		if strings.Contains(svc, bad) {
			t.Errorf("a locked panel carries %q:\n%s", bad, svc)
		}
	}
	page := s.get(t, "/instances/default/podinfo?tab=graph&node="+url.QueryEscape(nodeService)).body
	if tag := detailTag.FindString(page); !strings.Contains(tag, `data-follow="`+podinfoTopic+`"`) {
		t.Errorf("a locked panel does not follow its page:\n%s", tag)
	}
}

// TestAChangeOnTheOwnerTopicReplacesThePanelAndSparesAView: the panel follows
// the topic of the graph beside it, so one refresh brings both from one fetch;
// a view fragment follows nothing and has no id the refresh could replace.
func TestAChangeOnTheOwnerTopicReplacesThePanelAndSparesAView(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	page := s.get(t, "/instances/default/podinfo?tab=graph").body
	graph := regexp.MustCompile(`id="graph"[^>]*data-follow="([^"]*)"`).FindStringSubmatch(page)
	panel := regexp.MustCompile(`id="detail"[^>]*data-follow="([^"]*)"`).FindStringSubmatch(page)
	if graph == nil || panel == nil || graph[1] != panel[1] || graph[1] != podinfoTopic {
		t.Fatalf("the graph and the panel do not follow one topic: %v %v", graph, panel)
	}
	for _, path := range []string{yamlPath, eventsPath} {
		view := s.get(t, path, htmxRequest).body
		if strings.Contains(view, "data-follow") || strings.Contains(view, "data-topics") || strings.Contains(view, ` id="detail"`) {
			t.Errorf("%s: a view could be refreshed in place:\n%s", path, view)
		}
	}
}
