package ui

import (
	"net/http"
	"strings"
	"testing"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// TestPlatformStatusLinksItsReasons: the status card names Ready's and
// ContractsFulfilled's own reasons, each linking to its condition row, and
// the page draws no graph and no Platform health (portal:D17).
func TestPlatformStatusLinksItsReasons(t *testing.T) {
	main := mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/").body)
	for _, want := range []string{
		`<a href="#cond-Ready"><span class="label">Ready</span> <span class="mono">Generated</span></a>`,
		`<a href="#cond-ContractsFulfilled"><span class="label">Contracts</span> <span class="mono">UnfulfilledContracts</span></a>`,
		`id="cond-Ready"`, `id="cond-ContractsFulfilled"`, `<dt>Controller version</dt><dd class="mono">v1.0.0-beta.6</dd>`,
	} {
		if !strings.Contains(main, want) {
			t.Errorf("the Platform page lacks %s", want)
		}
	}
	for _, never := range []string{"<svg viewBox", "graph-frame", "Platform health"} {
		if strings.Contains(main, never) {
			t.Errorf("the Platform page carries %s", never)
		}
	}
}

// TestPlatformCountsHandOff: each count is a link to Installed filtered by
// it, and a list the caller may not read is locked, not zero.
func TestPlatformCountsHandOff(t *testing.T) {
	main := mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/").body)
	for _, want := range []string{`href="/installed?health=Healthy">Healthy <span class="count-n">5</span>`,
		`href="/installed?applied=Failed">Failed <span class="count-n">1</span>`,
		"6 installed: 5 instances, 1 package; 1 of them holds a registration."} {
		if !strings.Contains(main, want) {
			t.Errorf("the Installed card lacks %s", want)
		}
	}
	locked := mainOf(newSite(t, apitest.F1(t), apitest.DenyResources("moduleinstances")).get(t, "/").body)
	card := between(locked, `id="installed-card"`, "</section>")
	if !strings.Contains(card, "instances locked") || !strings.Contains(card, "locked-panel") || strings.Contains(card, "0 instances") {
		t.Errorf("forbidden instances do not count as locked:\n%s", card)
	}
}

func TestPlatformTabsAndFilters(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for path, want := range map[string][]string{
		"/?pstatus=refused":            {"default.<wbr>refused-claim-fixture"},
		"/?pstatus=active":             {"default.<wbr>backup-provider"},
		"/?pq=backup&tab=providers":    {"default.<wbr>backup-provider"},
		"/?tab=catalogs&csource=claim": {"refused-claim-fixture-absent@v0"},
		"/?tab=catalogs&claimed=no":    {"opm@v4"},
	} {
		main := mainOf(s.get(t, path).body)
		rows := between(main, "<tbody>", "</tbody>")
		if strings.Count(rows, "<tr") != len(want) {
			t.Errorf("%s: %d rows; want %d:\n%s", path, strings.Count(rows, "<tr"), len(want), rows)
		}
		for _, w := range want {
			if !strings.Contains(rows, w) {
				t.Errorf("%s: no row for %s", path, w)
			}
		}
	}
	// A tab link keeps every filter.
	if main := mainOf(s.get(t, "/?pstatus=refused&cq=opm").body); !strings.Contains(main, `href="/?cq=opm&amp;pstatus=refused&amp;tab=catalogs"`) {
		t.Error("the Catalogs tab link drops a filter")
	}
}

func TestPlatformEventsMergeAndFilter(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	all := between(mainOf(s.get(t, "/").body), `<div id="events"`, "</section>")
	if !strings.Contains(all, "CatalogUnresolved") || !strings.Contains(all, "Generated") {
		t.Errorf("the merged feed lacks the Platform's or the registration's events:\n%s", all)
	}
	if strings.Index(all, "CatalogUnresolved") > strings.Index(all, ">Generated<") {
		t.Error("the merged feed is not newest first")
	}
	only := between(mainOf(s.get(t, "/?eresource=platform").body), `<div id="events"`, "</section>")
	if strings.Contains(only, "CatalogUnresolved") || !strings.Contains(only, "Generated") {
		t.Errorf("eresource=platform keeps another resource's events:\n%s", only)
	}
	bogus := mainOf(s.get(t, "/?eresource=registration:nope").body)
	if !strings.Contains(bogus, "Ignored filter <span class=\"mono\">eresource=registration:nope</span>") {
		t.Error("an unknown resource is not ignored with a note")
	}
}

// TestPlatformForbidden: a caller who may not read the Platform gets a page
// that says so and still shows what it may read.
func TestPlatformForbidden(t *testing.T) {
	res := newSite(t, apitest.F1(t), apitest.DenyResources("platforms")).get(t, "/")
	main := mainOf(res.body)
	if res.status != http.StatusOK || !strings.Contains(main, "locked-panel") || !strings.Contains(main, "6 installed") {
		t.Errorf("forbidden Platform: %d\n%s", res.status, main)
	}
}

// TestPlatformRegionsEscapeClusterText: the provider rows, the conditions
// and the merged feed show hostile cluster text as text.
func TestPlatformRegionsEscapeClusterText(t *testing.T) {
	s := newSite(t, hostileF1(t), apitest.AllowAll)
	for path, regions := range map[string][]string{
		"/":              {`<div id="providers"`, `id="conditions"`, `<div id="events"`},
		"/?tab=catalogs": {`<div id="catalogs"`},
	} {
		main := mainOf(s.get(t, path).body)
		for _, from := range regions {
			region := between(main, from, "</section>")
			if !strings.Contains(region, "&lt;script&gt;alert(1)&lt;/script&gt;") || strings.Contains(region, "<script>alert") {
				t.Errorf("%s %s does not show the hostile text escaped", path, from)
			}
		}
	}
}

// TestLockedRegistrationsLockTheirClaims (portal:D7:R2/R3): when the
// caller may not list registrations, which catalog is claimed is locked,
// not "no provider", the claimed filter is set aside, and no count of
// providers is shown.
func TestLockedRegistrationsLockTheirClaims(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.DenyResources("transformerregistrations"))
	cats := between(mainOf(s.get(t, "/?tab=catalogs&claimed=no").body), `<div id="catalogs"`, "</section>")
	if strings.Contains(cats, ">no provider<") || !strings.Contains(cats, `class="locked-inline"`) {
		t.Errorf("claims are not locked:\n%s", cats)
	}
	if strings.Count(between(cats, "<tbody>", "</tbody>"), "<tr") != 2 {
		t.Error("the claimed filter was applied to locked claims")
	}
	form := between(mainOf(s.get(t, "/?tab=catalogs&claimed=no").body), `aria-label="Filter catalogs"`, "</form>")
	if !strings.Contains(form, "Ignored filter <span class=\"mono\">claimed=no</span>") {
		t.Errorf("the set-aside filter is not named:\n%s", form)
	}
	if prov := between(mainOf(s.get(t, "/").body), `<div id="providers"`, "</div>"); strings.Contains(prov, "Showing") {
		t.Errorf("a count of providers shows while they are locked:\n%s", prov)
	}
	events := between(mainOf(newSite(t, apitest.F1(t), apitest.DenyResources("platforms")).get(t, "/").body), `<div id="events"`, "</section>")
	if strings.Contains(events, "Showing") {
		t.Errorf("a count of events shows while the feed is locked:\n%s", events)
	}
}

// TestPlatformEventsFilterShowsItsChip: the events form names the filter
// it applies, like every other filter form.
func TestPlatformEventsFilterShowsItsChip(t *testing.T) {
	form := between(mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/?eresource=platform").body), `aria-label="Filter recent events"`, "</form>")
	if !strings.Contains(form, `<span class="fchip-label">Resource</span> the Platform`) {
		t.Errorf("no chip for the events filter:\n%s", form)
	}
}

// tabsOf returns the first tab strip of a page's main region.
func tabsOf(main string) string { return between(main, `<nav class="tabs`, "</nav>") }

// TestPlatformTabsCarryTheirCounts (portal:D19:R3, portal:D7): Providers and
// Catalogs show the registrations and catalog rows the page holds, the whole
// list whatever a filter says. When the registrations are locked or the
// Platform is not readable, each keeps an empty count span for a live refresh
// to empty.
func TestPlatformTabsCarryTheirCounts(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, path := range []string{"/", "/?tab=catalogs", "/?pstatus=refused", "/?tab=catalogs&csource=claim"} {
		tabs := tabsOf(mainOf(s.get(t, path).body))
		for _, want := range []string{
			`Providers <span class="tab-n" id="tab-n-providers" data-follow="platform">2</span>`,
			`Catalogs <span class="tab-n" id="tab-n-catalogs" data-follow="platform">3</span>`,
		} {
			if !strings.Contains(tabs, want) {
				t.Errorf("%s: the tabs lack %s:\n%s", path, want, tabs)
			}
		}
		if !strings.Contains(mainOf(s.get(t, path).body), `class="tabs tabs-lg"`) {
			t.Errorf("%s: the Platform strip is not the large one", path)
		}
	}
	for name, rule := range map[string]apitest.Rule{
		"registrations locked": apitest.DenyResources("transformerregistrations"),
		"Platform forbidden":   apitest.DenyResources("platforms"),
	} {
		tabs := tabsOf(mainOf(newSite(t, apitest.F1(t), rule).get(t, "/").body))
		for _, want := range []string{
			`Providers<span class="tab-n" id="tab-n-providers" data-follow="platform"></span></a>`,
			`Catalogs<span class="tab-n" id="tab-n-catalogs" data-follow="platform"></span></a>`,
		} {
			if !strings.Contains(tabs, want) {
				t.Errorf("%s: a count shows over a list the caller may not read in full, or its empty span is gone (want %s):\n%s", name, want, tabs)
			}
		}
	}
}
