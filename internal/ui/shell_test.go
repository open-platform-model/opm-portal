package ui

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// TestHeaderNamesTheConnection (portal:D18): every page's header names the
// context, the reader and the Kubernetes version from the Cluster
// document, and the root carries the context the browser keys its
// filters by.
func TestHeaderNamesTheConnection(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, path := range []string{"/", "/installed", "/instances/default/podinfo", "/nope"} {
		body := s.get(t, path).body
		head := between(body, `<header class="masthead"`, "</header>")
		for _, want := range []string{
			`href="/"`, ">Platform</a>", `href="/installed"`, ">Installed</a>",
			`<span class="conn-cluster mono" title="Context ` + apitest.Context + `">` + apitest.Context + `</span>`,
			`reading as <span class="mono">alice</span>`, `Kubernetes <span class="mono">v1.36.1</span>`,
			`id="live"`, `data-theme-choice="light"`, `data-theme-choice="dark"`, `data-theme-choice="system"`,
		} {
			if !strings.Contains(head, want) {
				t.Errorf("%s: the header lacks %s:\n%s", path, want, head)
			}
		}
		if !strings.Contains(body, `<html lang="en" data-context="`+apitest.Context+`">`) {
			t.Errorf("%s: the root does not carry the context", path)
		}
	}
	if head := between(s.get(t, "/installed").body, `<nav`, "</nav>"); !strings.Contains(head, `href="/installed" aria-current="page"`) {
		t.Errorf("Installed is not the current entry on /installed:\n%s", head)
	}
}

func TestHeaderInACluster(t *testing.T) {
	s := newInClusterSite(t, apitest.F1(t), apitest.AllowAll)
	body := s.get(t, "/").body
	if !strings.Contains(body, `<span class="conn-cluster mono" title="In cluster: the credentials of the Pod">in cluster</span>`) ||
		!strings.Contains(body, `data-context="in-cluster"`) {
		t.Errorf("an in-cluster header does not read in cluster:\n%s", between(body, "<header", "</header>"))
	}
}

// clusterAs answers the Cluster document with status and body, and passes
// every other request to the read API.
func clusterAs(api http.Handler, status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == defaultAPIBase {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
			return
		}
		api.ServeHTTP(w, r)
	})
}

// TestHeaderWithoutAContext: a kubeconfig source that names no context
// reads unknown, and the browser keys no filters by it.
func TestHeaderWithoutAContext(t *testing.T) {
	doc, err := json.Marshal(map[string]any{
		"apiVersion": "portal.opmodel.dev/v1alpha1", "kind": "Cluster", "name": "default", "mode": "local",
		"source": "kubeconfig", "readingAs": map[string]string{"username": "alice"},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := mount(t, clusterAs(apitest.New(t, apitest.F1(t), apitest.AllowAll), http.StatusOK, string(doc))).get(t, "/").body
	if !strings.Contains(between(body, "<header", "</header>"), `title="Context unknown">unknown</span>`) ||
		strings.Contains(body, "data-context=") {
		t.Errorf("a header without a context:\n%s", between(body, "<html", "</header>"))
	}
}

// TestThemeMenuIsAGroupOfPressedButtons: three buttons, the current one
// pressed, and the summary names the choice.
func TestThemeMenuIsAGroupOfPressedButtons(t *testing.T) {
	head := between(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/").body, `<details class="theme"`, "</details>")
	if !strings.Contains(head, `<summary aria-label="Theme: System" data-theme-summary>`) || strings.Contains(head, "menuitem") ||
		strings.Count(head, "aria-pressed=") != 3 || !strings.Contains(head, `aria-pressed="true" data-theme-choice="system"`) {
		t.Errorf("theme menu:\n%s", head)
	}
}

func TestHeaderWithoutAVersion(t *testing.T) {
	doc, err := json.Marshal(map[string]any{
		"apiVersion": "portal.opmodel.dev/v1alpha1", "kind": "Cluster", "name": "default", "mode": "local",
		"source": "kubeconfig", "context": "dev", "readingAs": map[string]string{"username": "alice"},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := mount(t, clusterAs(apitest.New(t, apitest.F1(t), apitest.AllowAll), http.StatusOK, string(doc)))
	if head := between(s.get(t, "/").body, "<header", "</header>"); !strings.Contains(head, `Kubernetes <span class="mono">unknown</span>`) {
		t.Errorf("a missing version does not read unknown:\n%s", head)
	}
}

// TestHeaderWhenTheClusterDocumentFails: the page still renders, with its
// navigation and theme menu, and the connection reads unknown, degraded.
func TestHeaderWhenTheClusterDocumentFails(t *testing.T) {
	s := mount(t, clusterAs(apitest.New(t, apitest.F1(t), apitest.AllowAll), http.StatusServiceUnavailable,
		`{"type":"about:blank","title":"Unavailable","status":503,"code":"upstream_unavailable"}`))
	res := s.get(t, "/installed")
	if res.status != http.StatusOK || !strings.Contains(mainOf(res.body), "/instances/default/podinfo") {
		t.Fatalf("the page did not render: %d", res.status)
	}
	head := between(res.body, "<header", "</header>")
	for _, want := range []string{`class="conn conn-degraded"`, `reading as <span class="mono">unknown</span>`,
		`Kubernetes <span class="mono">unknown</span>`, ">Installed</a>", `data-theme-choice="dark"`} {
		if !strings.Contains(head, want) {
			t.Errorf("the degraded header lacks %s:\n%s", want, head)
		}
	}
	if strings.Contains(res.body, "data-context=") {
		t.Error("a page without the Cluster document names a context, so the browser would key filters by nothing known")
	}
}

// TestThePrefsScriptRunsBeforeTheStylesheet: the theme is applied before
// first paint by a script in the head without defer, ahead of the
// stylesheet, and after the filter definitions it reads (portal:D14:R4).
func TestThePrefsScriptRunsBeforeTheStylesheet(t *testing.T) {
	head := between(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/").body, "<head>", "</head>")
	spec := strings.Index(head, `<meta name="opm-portal-filters"`)
	prefs := strings.Index(head, `<script src="/static/prefs.js"></script>`)
	css := strings.Index(head, `<link rel="stylesheet" href="/static/portal.css">`)
	if spec < 0 || prefs < 0 || css < 0 || spec > prefs || prefs > css {
		t.Errorf("head order: filters %d, prefs.js %d, stylesheet %d; want them in that order:\n%s", spec, prefs, css, head)
	}
}

// TestTheBrowserStoresOnlyPreferences (portal:D14:R1): the page scripts
// write browser storage only through prefs.js, and prefs.js writes only the
// theme key and the per-context filter key of a remembered view, which are
// Installed, Providers and Catalogs.
func TestTheBrowserStoresOnlyPreferences(t *testing.T) {
	portal, err := os.ReadFile("static/portal.js")
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`localStorage|sessionStorage|indexedDB|document\.cookie`).Match(portal) {
		t.Error("portal.js touches browser storage itself; only prefs.js may")
	}
	prefs, err := os.ReadFile("static/prefs.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(prefs)
	if n := strings.Count(src, "localStorage."); n != 3 {
		t.Errorf("prefs.js uses localStorage %d times; want read, set and remove, once each in read and write", n)
	}
	if !strings.Contains(src, `var THEME_KEY = "opm-portal.theme";`) ||
		!strings.Contains(src, `return "opm-portal.filters." + view + ":" + context();`) {
		t.Error("prefs.js does not name the theme key and the per-context filter key as expected")
	}
	for _, call := range regexp.MustCompile(`(function )?\bwrite\(((?:[^,()]|\([^)]*\))*)`).FindAllStringSubmatch(src, -1) {
		if call[1] != "" {
			continue // the definition
		}
		if arg := strings.TrimSpace(call[2]); arg != "THEME_KEY" && arg != "filterKey(view)" {
			t.Errorf("prefs.js writes storage under %s, which is neither the theme key nor a filter key", arg)
		}
	}
	var spec map[string]any
	if err := json.Unmarshal([]byte(filterSpec()), &spec); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(spec))
	for k := range spec {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, []string{"catalogs", "installed", "providers"}) {
		t.Errorf("remembered views %v; want catalogs, installed, providers", keys)
	}
}

// TestOldListsRedirectToInstalled (portal:D17:R1).
func TestOldListsRedirectToInstalled(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for path, want := range map[string]string{
		"/instances?namespace=team-a": "/installed?kind=instance&namespace=team-a",
		"/instances":                  "/installed?kind=instance",
		"/packages?namespace=pkg":     "/installed?kind=package&namespace=pkg",
		"/packages":                   "/installed?kind=package",
	} {
		res := s.get(t, path)
		if res.status != http.StatusPermanentRedirect || res.header.Get("Location") != want {
			t.Errorf("GET %s = %d to %q; want 308 to %q", path, res.status, res.header.Get("Location"), want)
		}
	}
	if res := s.get(t, "/instances/default/podinfo"); res.status != http.StatusOK {
		t.Errorf("an instance page answers %d; only the lists redirect", res.status)
	}
}

// TestTheLiveMarkIsWorded (portal:D19:R3): the mark starts as "Not live", the
// script words every state with a capital, and the dot does not pulse.
func TestTheLiveMarkIsWorded(t *testing.T) {
	body := newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/").body
	live := between(body, `<p class="live"`, "</p>")
	if !strings.Contains(live, `<span class="live-text">Not live</span>`) {
		t.Errorf("the live mark does not start as Not live:\n%s", live)
	}
	script, err := os.ReadFile("static/portal.js")
	if err != nil {
		t.Fatal(err)
	}
	calls := regexp.MustCompile(`(?:setLive\((?:true|false|ok)|setStream\("[a-z]+"), "([^"]*)"\)`).FindAllStringSubmatch(string(script), -1)
	want := []string{"Live", "Not live", "Reconnecting", "Offline", "Signed out", "Topics refused", "Session expired, reload"}
	seen := map[string]bool{}
	for _, c := range calls {
		seen[c[1]] = true
		if !slices.Contains(want, c[1]) {
			t.Errorf("portal.js sets the live mark to %q, which is not one of the worded states", c[1])
		}
	}
	for _, w := range []string{"Live", "Reconnecting", "Offline", "Signed out", "Topics refused", "Session expired, reload"} {
		if !seen[w] {
			t.Errorf("portal.js never sets the live mark to %q", w)
		}
	}
	if strings.Contains(string(script), "pulse") || strings.Contains(body, "pulse") {
		t.Error("the script or the page still names pulse")
	}
}

// clusterDoc is a Cluster document with source, context and entry; an empty
// value is left out.
func clusterDoc(t *testing.T, source, context, entry string) string {
	t.Helper()
	doc := map[string]any{
		"apiVersion": "portal.opmodel.dev/v1alpha1", "kind": "Cluster", "name": "default", "mode": "local",
		"source": source, "readingAs": map[string]string{"username": "alice"}, "kubernetesVersion": "v1.36.1",
	}
	if context != "" {
		doc["context"] = context
	}
	if entry != "" {
		doc["clusterEntry"] = entry
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// pill is the cluster pill of a page's header.
func pill(t *testing.T, s *site) string {
	t.Helper()
	return between(between(s.get(t, "/").body, "<header", "</header>"), `<span class="conn-cluster`, `<span class="conn-reader">`)
}

// TestHeaderNamesTheClusterEntry (portal:D18:R1): a kubeconfig whose cluster
// entry is named other than its context shows both, the second with the
// word "cluster", and the root's data-context stays the context alone.
func TestHeaderNamesTheClusterEntry(t *testing.T) {
	s := mount(t, apitest.NewWithCluster(t, apitest.F1(t), apitest.AllowAll, apitest.Context, "prod-east"))
	body := s.get(t, "/").body
	want := `<span class="conn-cluster conn-two" title="Context ` + apitest.Context + `, cluster prod-east"><span class="conn-name mono">` + apitest.Context +
		`</span> <span class="conn-entry"><span class="conn-k">cluster</span> <span class="conn-name mono">prod-east</span></span></span>`
	if got := pill(t, s); !strings.Contains(got, want) {
		t.Errorf("the pill does not name the entry:\n%s", got)
	}
	if !strings.Contains(body, `<html lang="en" data-context="`+apitest.Context+`">`) {
		t.Error("the root's data-context is not the context alone")
	}
}

// TestHeaderWithAnEqualClusterEntry: the common case keeps today's markup.
func TestHeaderWithAnEqualClusterEntry(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	got := pill(t, s)
	if strings.Contains(got, "conn-entry") || strings.Contains(got, "cluster</span>") ||
		!strings.Contains(got, `<span class="conn-cluster mono" title="Context `+apitest.Context+`">`+apitest.Context+`</span>`) {
		t.Errorf("an equal entry changed the pill:\n%s", got)
	}
}

// TestHeaderNamesAnEntryOnlyWhereItAddsSomething: no entry in a Pod, none
// when the document carries none, one beside "unknown" when the context is
// missing, and the name is escaped.
func TestHeaderNamesAnEntryOnlyWhereItAddsSomething(t *testing.T) {
	for name, tc := range map[string]struct {
		source, context, entry string
		wantEntry              string
		wantFirst              string
	}{
		"in cluster":    {"in-cluster", "", "leftover", "", "in cluster"},
		"no entry":      {"kubeconfig", "dev", "", "", "dev"},
		"no context":    {"kubeconfig", "", "prod-east", "prod-east", "unknown"},
		"hostile entry": {"kubeconfig", "dev", `<img src=x onerror=alert(1)>`, `&lt;img src=x onerror=alert(1)&gt;`, "dev"},
	} {
		t.Run(name, func(t *testing.T) {
			s := mount(t, clusterAs(apitest.New(t, apitest.F1(t), apitest.AllowAll), http.StatusOK, clusterDoc(t, tc.source, tc.context, tc.entry)))
			got := pill(t, s)
			if !strings.Contains(got, ">"+tc.wantFirst+"</span>") {
				t.Errorf("the pill does not read %q:\n%s", tc.wantFirst, got)
			}
			if tc.wantEntry == "" {
				if strings.Contains(got, "conn-entry") {
					t.Errorf("the pill names an entry it should not:\n%s", got)
				}
			} else if !strings.Contains(got, `<span class="conn-k">cluster</span> <span class="conn-name mono">`+tc.wantEntry+`</span>`) {
				t.Errorf("the pill does not name the entry %q:\n%s", tc.wantEntry, got)
			}
			if strings.Contains(got, "<img") {
				t.Errorf("the entry was not escaped:\n%s", got)
			}
		})
	}
}

// TestHeaderWithoutTheClusterDocumentNamesNoEntry: a failed document leaves
// the degraded pill and no entry.
func TestHeaderWithoutTheClusterDocumentNamesNoEntry(t *testing.T) {
	s := mount(t, clusterAs(apitest.NewWithCluster(t, apitest.F1(t), apitest.AllowAll, apitest.Context, "prod-east"), http.StatusServiceUnavailable,
		`{"type":"about:blank","title":"Unavailable","status":503,"code":"upstream_unavailable"}`))
	if got := pill(t, s); strings.Contains(got, "conn-entry") || strings.Contains(got, "prod-east") {
		t.Errorf("the degraded pill names an entry:\n%s", got)
	}
}

// TestEveryPageHoldsAHiddenBanner (portal:D18): the banner is the last row of
// the sticky header, inside #app and outside #main so a boosted navigation
// keeps it, hidden without script, with no role and no live region of its
// own, and its Reload link is a full page load. The stream hook lists the
// heartbeat, which the page records as the time of the data.
func TestEveryPageHoldsAHiddenBanner(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, path := range []string{"/", "/installed", "/instances/default/podinfo", "/nope"} {
		body := s.get(t, path).body
		head := between(body, `<header class="masthead"`, "</header>")
		i := strings.Index(head, `<div class="region-banner"`)
		if i < 0 {
			t.Errorf("%s: the header holds no banner", path)
			continue
		}
		banner := head[i:]
		if !strings.HasPrefix(banner, `<div class="region-banner" id="region-banner" hidden>`) {
			t.Errorf("%s: the banner is not hidden at render:\n%s", path, banner)
		}
		if strings.Count(head[i:], "<div") != 1 || strings.Contains(head[:i], `class="region-banner"`) {
			t.Errorf("%s: the banner is not the last row of the header", path)
		}
		for _, bad := range []string{"role=", "aria-live=\"polite\"", "aria-live=\"assertive\"", "aria-atomic"} {
			if strings.Contains(banner, bad) {
				t.Errorf("%s: the banner carries %s", path, bad)
			}
		}
		for _, want := range []string{`<a class="rb-reload" href="" hx-boost="false">Reload</a>`, `<time class="rb-time" data-relative aria-live="off" datetime="" title=""></time>`} {
			if !strings.Contains(banner, want) {
				t.Errorf("%s: the banner lacks %s", path, want)
			}
		}
		if strings.Index(body, `id="region-banner"`) > strings.Index(body, `<main id="main"`) || strings.Index(body, `id="app"`) > strings.Index(body, `id="region-banner"`) {
			t.Errorf("%s: the banner is not inside #app and outside #main", path)
		}
		if !strings.Contains(body, "sse:expired, sse:heartbeat") {
			t.Errorf("%s: the stream hook does not list the heartbeat", path)
		}
	}
}
