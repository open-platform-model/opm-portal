package ui

import (
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// TestPackageCardsOnF1: the package's identity names its interval and an
// unrecorded revision; the Applied card reads Failed, SourceNotReady, with
// the Reconciling mark and three failed dots and no other (portal:D9:R1).
func TestPackageCardsOnF1(t *testing.T) {
	main := mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/packages/pkg/podinfo").body)
	head := between(main, `id="owner-head"`, "</section>")
	for _, want := range []string{`<dt>Interval</dt><dd><span class="mono">1m</span></dd>`, `<dt>Revision</dt><dd><span class="muted">not recorded</span></dd>`, "OCIRepository podinfo-release"} {
		if !strings.Contains(head, want) {
			t.Errorf("identity lacks %s", want)
		}
	}
	applied := between(main, `id="applied-card"`, "</section>")
	if !strings.Contains(applied, "applied-failed") || !strings.Contains(applied, ">SourceNotReady<") ||
		!strings.Contains(applied, `<span class="label">Reconciling</span> <span class="mono">Progressing</span>`) {
		t.Errorf("Applied card:\n%s", applied)
	}
	dots := regexp.MustCompile(`class="adot ([a-z-]+)"`).FindAllStringSubmatch(applied, -1)
	if len(dots) != 3 {
		t.Fatalf("%d dots; want three", len(dots))
	}
	for _, d := range dots {
		if d[1] != "dot-failed" {
			t.Errorf("dot %s; want failed", d[1])
		}
	}
	if !strings.Contains(applied, "At most ten attempts are kept") || !strings.Contains(applied, "Show history") {
		t.Error("the Applied card lacks the ten-entry note or Show history")
	}
}

// TestBrokenRolloutHandsOffToResources: the image-break sample's Health
// card counts the Pod's waiting reason, and its link lists that Pod.
func TestBrokenRolloutHandsOffToResources(t *testing.T) {
	s := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	health := between(mainOf(s.get(t, "/instances/default/podinfo").body), `id="health-card"`, "</section>")
	if !strings.Contains(health, `<span class="mono">ImagePullBackOff</span> <span class="count-n">1</span>`) {
		t.Errorf("the waiting reason is not counted once, at the Pod:\n%s", health)
	}
	m := regexp.MustCompile(`href="(/instances/default/podinfo\?reason=ImagePullBackOff&amp;tab=resources)"`).FindStringSubmatch(health)
	if m == nil {
		t.Fatalf("no ImagePullBackOff count:\n%s", health)
	}
	res := mainOf(s.get(t, strings.ReplaceAll(m[1], "&amp;", "&")).body)
	rows := between(res, `id="components"`, "</section>")
	if !strings.Contains(rows, "podinfo-podinfo-794bd8c7fb-cc5ql") || strings.Contains(rows, "Service default/") {
		t.Errorf("the reason filter does not list just the waiting Pod's branch:\n%s", rows)
	}
}

// TestProviderCardOnF1 (portal:D15:R2): backup-provider's Provider card
// reads Active, its pill shows, and a caller who may not list
// registrations sees a locked standing.
func TestProviderCardOnF1(t *testing.T) {
	main := mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/instances/default/backup-provider").body)
	card := between(main, `id="provider-card"`, "</section>")
	if !strings.Contains(card, ">Active</span>") || !strings.Contains(card, "default.<wbr>backup-provider") {
		t.Errorf("Provider card:\n%s", card)
	}
	if !strings.Contains(between(main, `id="owner-head"`, "</h1>"), `class="prov prov-active"`) {
		t.Error("no Provider pill")
	}
	locked := mainOf(newSite(t, apitest.F1(t), apitest.DenyResources("transformerregistrations")).get(t, "/instances/default/backup-provider").body)
	if card := between(locked, `id="provider-card"`, "</section>"); !strings.Contains(card, ">Locked</span>") {
		t.Errorf("forbidden registrations: Provider card:\n%s", card)
	}
	if strings.Contains(mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/instances/default/podinfo").body), `id="provider-card"`) {
		t.Error("an instance that holds no registration shows a Provider card")
	}
}

func TestEventsTabFilters(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	warn := between(mainOf(s.get(t, "/instances/default/podinfo?tab=events&type=Warning").body), `<ol class="events">`, "</ol>")
	if strings.Contains(warn, "ev-normal") {
		t.Errorf("type=Warning lists Normal events:\n%s", warn)
	}
	pod := mainOf(s.get(t, "/instances/default/podinfo?tab=events&resource=/Pod/default/podinfo-podinfo-d9585d794-4lg6h").body)
	if !strings.Contains(pod, "Pod default/podinfo-podinfo-d9585d794-4lg6h") || !strings.Contains(pod, "<optgroup label=\"Pod\">") {
		t.Errorf("the resource filter does not read the Pod's events or group by kind")
	}
	if !strings.Contains(mainOf(s.get(t, "/instances/default/podinfo?tab=events&resource=/Pod/default/nope").body), "Ignored filter") {
		t.Error("an unknown resource is not ignored")
	}
}

// TestTabsShowTheirOwnRegions (portal:D9): each tab renders the cards and
// its own regions only, and the details panel only on Graph and Resources.
func TestTabsShowTheirOwnRegions(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	regions := map[string][]string{
		"graph":     {`id="graph"`, `id="detail"`},
		"resources": {`id="components"`, `id="detail"`},
		"events":    {`id="events"`},
		"logs":      {`id="logs"`},
		"yaml":      {`id="yaml-picker"`, `id="yaml"`},
	}
	for tab, own := range regions {
		main := mainOf(s.get(t, "/instances/default/podinfo?tab="+tab).body)
		for _, card := range []string{`id="owner-head"`, `id="applied-card"`, `id="health-card"`, `id="conditions"`} {
			if !strings.Contains(main, card) {
				t.Errorf("tab %s lacks %s", tab, card)
			}
		}
		for other, ids := range regions {
			for _, id := range ids {
				has := strings.Contains(main, id)
				mine := false
				for _, o := range own {
					mine = mine || o == id
				}
				if has != mine {
					t.Errorf("tab %s: %s present %v (region of %s)", tab, id, has, other)
				}
			}
		}
		if !strings.Contains(main, `href="/instances/default/podinfo?tab=`+tab+`" aria-current="page"`) {
			t.Errorf("tab %s is not marked current", tab)
		}
	}
}

// TestYAMLTabListsASecretAsNeverRead (portal:D8:R1).
func TestYAMLTabListsASecretAsNeverRead(t *testing.T) {
	h := &Handler{}
	v := ownerView{Kind: instanceKind, Base: "/instances/default/app", Components: h.components("/instances/default/app", []v1.Component{{
		Name: "db", Objects: []v1.InventoryObject{{Ref: v1.ObjectRef{Version: "v1", Kind: "Secret", Namespace: "default", Name: "db"}, Access: v1.AccessOK}},
	}})}
	h.ownerYAMLTab(httptestRequest(t, "/instances/default/app?tab=yaml"), &v)
	v.Tab = tabYAML
	out := between(renderMain(t, "owner", v), `id="yaml-picker"`, "</section>")
	if !strings.Contains(out, "Secret data is never read") || strings.Contains(out, "object=") {
		t.Errorf("Secret in the YAML picker:\n%s", out)
	}
}

// TestOwnerCardsEscapeClusterText: hostile operator text on the cards,
// the dots' titles and every tab is shown as text.
func TestOwnerCardsEscapeClusterText(t *testing.T) {
	s := newSite(t, hostileF1(t), apitest.AllowAll)
	for _, tab := range []string{"graph", "resources", "events", "yaml&object=apps/Deployment/default/podinfo-podinfo"} {
		main := mainOf(s.get(t, "/instances/default/podinfo?tab="+tab).body)
		if strings.Contains(main, "<script>alert") || strings.Contains(main, "<img src=x") {
			t.Errorf("tab %s renders cluster text as markup", tab)
		}
	}
	pkg := mainOf(s.get(t, "/packages/pkg/podinfo").body)
	dots := between(pkg, `<span class="dots">`, "</div>")
	if !strings.Contains(dots, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("the dots' titles do not carry the hostile message escaped:\n%s", dots)
	}
}

func httptestRequest(t *testing.T, target string) *http.Request {
	t.Helper()
	return httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
}

// TestFoldedObjectsFocusTheirOwnNode: every Graph link of cert-manager's
// folded configuration objects opens the group expanded and focuses a node
// the expanded graph draws.
func TestFoldedObjectsFocusTheirOwnNode(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	res := mainOf(s.get(t, "/instances/cert-manager/cert-manager?tab=resources").body)
	links := regexp.MustCompile(`href="(/instances/cert-manager/cert-manager\?expand=[^"]+)">Graph</a>`).FindAllStringSubmatch(res, -1)
	if len(links) == 0 {
		t.Fatal("no folded object links to the graph")
	}
	expanded := mainOf(s.get(t, "/instances/cert-manager/cert-manager?tab=graph&expand=grp:configuration/mi/cert-manager/cert-manager").body)
	for _, l := range links {
		u, err := url.Parse(strings.ReplaceAll(l[1], "&amp;", "&"))
		if err != nil {
			t.Fatal(err)
		}
		focus := u.Query().Get("focus")
		if !strings.Contains(expanded, `data-node="`+html.EscapeString(focus)+`"`) {
			t.Errorf("focus %q names no node of the expanded graph", focus)
		}
	}
}

// TestPodPanelLogsOpenItsPane: a Pod's panel links to its first
// container's pane on the Logs tab, on the page and as a fragment.
func TestPodPanelLogsOpenItsPane(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	const pod = "obj:_/Pod/default/podinfo-podinfo-d9585d794-4lg6h"
	want := `href="/instances/default/podinfo?tab=logs#log_podinfo-podinfo-d9585d794-4lg6h_podinfo"`
	if page := mainOf(s.get(t, "/instances/default/podinfo?tab=graph&focus="+url.QueryEscape(pod)).body); !strings.Contains(page, want) {
		t.Error("the page's Pod panel lacks its Logs link to the pane")
	}
	if frag := s.get(t, "/instances/default/podinfo/node?id="+url.QueryEscape(pod), htmxRequest).body; !strings.Contains(frag, want) {
		t.Error("the Pod panel fragment lacks its Logs link to the pane")
	}
}

// TestOwnerTabsCarryTheirCounts (portal:D19:R3): Resources counts the
// objects and the runtime children below them, Events the folded lines of
// the owner's feed; both count the whole list when a filter narrows the tab,
// and neither shows when the owner document is a problem.
func TestOwnerTabsCarryTheirCounts(t *testing.T) {
	follow := `instance:default/podinfo events:instance:default/podinfo`
	count := func(name string, n int) string {
		return `<span class="tab-n" id="tab-n-` + name + `" data-follow="` + follow + `">` + strconv.Itoa(n) + `</span>`
	}
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	// The Service and the Deployment, the ReplicaSet and its two Pods.
	tabs := tabsOf(mainOf(s.get(t, "/instances/default/podinfo").body))
	for _, want := range []string{"Resources " + count("resources", 5), "Events " + count("events", 3), ">Graph</a>", ">Logs</a>", ">YAML</a>"} {
		if !strings.Contains(tabs, want) {
			t.Errorf("the instance tabs lack %s:\n%s", want, tabs)
		}
	}
	// A package with an empty inventory counts zero resources.
	pkg := tabsOf(mainOf(s.get(t, "/packages/pkg/podinfo").body))
	if !strings.Contains(pkg, `id="tab-n-resources" data-follow="package:pkg/podinfo events:package:pkg/podinfo">0</span>`) {
		t.Errorf("the package tabs:\n%s", pkg)
	}
	// A filter narrows the list, not the tab: the unfiltered counts stay.
	broken := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	unfiltered := tabsOf(mainOf(broken.get(t, "/instances/default/podinfo").body))
	for _, path := range []string{"/instances/default/podinfo?tab=events&type=Warning", "/instances/default/podinfo?tab=resources&reason=ImagePullBackOff"} {
		got := tabsOf(mainOf(broken.get(t, path).body))
		for _, name := range []string{"tab-n-resources", "tab-n-events"} {
			if between(got, `id="`+name+`"`, "</span>") != between(unfiltered, `id="`+name+`"`, "</span>") {
				t.Errorf("%s: %s changed under a filter:\n%s\nunfiltered:\n%s", path, name, got, unfiltered)
			}
		}
	}
	if !strings.Contains(unfiltered, count("resources", 7)) {
		t.Errorf("the broken rollout lists seven rows:\n%s", unfiltered)
	}
	// No count over a source that is a problem: the owner document itself.
	denied := newSite(t, apitest.F1(t), apitest.DenyResources("moduleinstances")).get(t, "/instances/default/podinfo").body
	if strings.Contains(denied, "tab-n") {
		t.Error("a tab count shows while the owner document is a problem")
	}
}

// TestATabOpensWithoutScript (portal:D19:R3): ?tab=resources renders the
// Resources tab as the current one, from the server alone, and the region it
// opens is named by a heading only a screen reader sees.
func TestATabOpensWithoutScript(t *testing.T) {
	main := mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/instances/default/podinfo?tab=resources").body)
	if !strings.Contains(tabsOf(main), `<a href="/instances/default/podinfo?tab=resources" aria-current="page">Resources`) {
		t.Errorf("Resources is not the current tab:\n%s", tabsOf(main))
	}
	if strings.Contains(main, "<script") {
		t.Error("the main region carries a script; the tab opens without one")
	}
	if !strings.Contains(main, `<h2 class="vh" id="components-h">Resources`) || !strings.Contains(main, `aria-labelledby="components-h"`) {
		t.Error("the Resources region lost its hidden heading or its aria-labelledby")
	}
}

// TestTabRegionsKeepTheirHeadingForScreenReaders (portal:D19:R3): every
// region a tab opens keeps its h2 and its id, which the region names itself
// by, and hides the heading from sight.
func TestTabRegionsKeepTheirHeadingForScreenReaders(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for path, ids := range map[string][]string{
		"/instances/default/podinfo?tab=graph":                                     {"graph-h"},
		"/instances/default/podinfo?tab=resources":                                 {"components-h"},
		"/instances/default/podinfo?tab=events":                                    {"events-h"},
		"/instances/default/podinfo?tab=logs":                                      {"logs-h"},
		"/instances/default/podinfo?tab=yaml":                                      {"yaml-picker-h", "yaml-h"},
		"/instances/default/backup-provider?tab=provider":                          {"provider-tab-h"},
		"/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0":            {"claims-h"},
		"/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0&tab=events": {"events-h"},
	} {
		main := mainOf(s.get(t, path).body)
		for _, id := range ids {
			if !strings.Contains(main, `<h2 class="vh" id="`+id+`">`) || !strings.Contains(main, `aria-labelledby="`+id+`"`) {
				t.Errorf("%s: the region named by %s lost its hidden heading", path, id)
			}
		}
	}
}
