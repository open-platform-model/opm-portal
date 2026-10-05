package ui

import (
	"net/http"
	"net/http/httptest"
	"regexp"
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
