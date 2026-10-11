package ui

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// TestPlatformStatusLinksItsReasons: the status card names Ready's and
// ContractsFulfilled's own reasons, each linking to its condition row, and
// the page draws no graph and no Platform health (portal:D17).
func TestPlatformStatusLinksItsReasons(t *testing.T) {
	main := mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/").body)
	for _, want := range []string{
		`<section class="state-block hue-applied" id="platform-status" aria-label="Platform status"`,
		`<span class="sb-eyebrow">Platform · the controller</span><span class="sb-when">since `,
		`<span class="sb-word">Applied</span>`,
		`<a href="/?tab=providers&amp;pstatus=refused#parts" class="hue-degraded"><strong>1</strong> <span class="mono">CatalogUnresolved</span></a>`,
		`<a href="/?tab=catalogs#contracts" class="hue-unknown"><span class="mono">UnfulfilledContracts</span></a>`,
		`id="cond-Ready"`, `id="cond-ContractsFulfilled"`, `<dt>Controller</dt><dd class="mono">v1.0.0-beta.6</dd>`,
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
	for _, want := range []string{`href="/installed?health=Healthy" title="Show the 5 installed with health Healthy"><span class="count-badge">Healthy</span><span class="count-n">5</span>`,
		`href="/installed?applied=Failed" title="Show the 1 installed with applied state Failed"><span class="count-badge">Failed</span><span class="count-n">1</span>`,
		"6 installed: 5 instances, 1 package · 1 of them is a provider"} {
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
		"/":              {`<div id="providers"`, `id="platform-status"`, `id="conditions"`, `<div id="events"`},
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

// TestPlatformReasonLinksAndEventCellsEscapeClusterText (4.9, SR11): over a
// capture whose reasons and notes are hostile, the status block's reason
// links and the events table's reason and message cells show the text
// escaped, the reason links' addresses carry none of it, and no attribute
// anywhere on the page holds an unescaped angle bracket or quote.
func TestPlatformReasonLinksAndEventCellsEscapeClusterText(t *testing.T) {
	const escaped = "&lt;script&gt;alert(1)&lt;/script&gt;"
	main := mainOf(newSite(t, hostileF1(t), apitest.AllowAll).get(t, "/").body)

	reasons := between(main, `<div class="sb-reasons">`, "</div>")
	links := regexp.MustCompile(`(?s)<a href="([^"]*)"[^>]*>(.*?)</a>`).FindAllStringSubmatch(reasons, -1)
	if len(links) == 0 {
		t.Fatalf("the status block has no reason link:\n%s", main)
	}
	for _, l := range links {
		if strings.Contains(l[1], "script") || strings.Contains(l[1], "%3C") {
			t.Errorf("a reason link's address carries cluster text: %s", l[1])
		}
		if !strings.Contains(l[2], escaped) || strings.Contains(l[2], "<script") {
			t.Errorf("a reason link does not show its hostile reason escaped: %s", l[2])
		}
	}

	events := between(main, `<table class="table rows events-table">`, "</table>")
	for _, cell := range []string{`class="mono ev-reason">`, `<span class="ev-note">`} {
		part := between(events, cell, "</")
		if !strings.Contains(part, escaped) || strings.Contains(part, "<script") {
			t.Errorf("an events cell (%s) does not show the hostile text escaped: %q", cell, part)
		}
	}

	if bad := regexp.MustCompile(`="[^"]*[<>][^"]*"`).FindAllString(main, -1); len(bad) != 0 {
		t.Errorf("attributes hold an unescaped angle bracket: %q", bad)
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

// platformWith is a Platform with the registrations and conditions a status
// block test needs, whose registrations are readable unless said.
func platformWith(access string, regs []v1.Registration, conds ...v1.Condition) *v1.Platform {
	return &v1.Platform{Name: "cluster", Reconcile: v1.Reconcile{State: "Applied"}, Registrations: regs,
		RegistrationsAccess: access, Conditions: conds}
}

// TestPlatformStatusBlockCountsItsReasons (platform-04, platform-05): a count per
// refusal reason linking to the Providers tab filtered to refused, one per
// removal-blocked reason, the Ready reason when Ready is not True and the
// ContractsFulfilled reason when it is False, both without a count.
func TestPlatformStatusBlockCountsItsReasons(t *testing.T) {
	regs := []v1.Registration{
		{Name: "a", Verdict: verdictRefused, Reason: "CatalogUnresolved"},
		{Name: "b", Verdict: verdictRefused, Reason: "CatalogUnresolved"},
		{Name: "c", Verdict: verdictRemovalBlocked, Reason: "DependentsRemain", Accepted: true, Active: true},
		{Name: "d", Accepted: true, Active: true, Verdict: "Accepted"},
	}
	b := platformStatusBlock(platformWith(v1.AccessOK, regs,
		v1.Condition{Type: conditionReady, Status: "False", Reason: "BuildFailed"},
		v1.Condition{Type: conditionContracts, Status: "False", Reason: "UnfulfilledContracts"}))
	got := make([]string, 0, len(b.Reasons))
	for _, r := range b.Reasons {
		got = append(got, fmt.Sprintf("%d %s %s %s", r.N, r.Text, r.Class, r.Href))
	}
	want := []string{
		"2 CatalogUnresolved hue-degraded /?tab=providers&pstatus=refused#parts",
		"1 DependentsRemain hue-unknown /?tab=providers&pstatus=blocked#parts",
		"0 BuildFailed hue-degraded #cond-Ready",
		"0 UnfulfilledContracts hue-unknown /?tab=catalogs#contracts",
	}
	if !slices.Equal(got, want) {
		t.Errorf("reasons:\n got %q\nwant %q", got, want)
	}
	if !strings.Contains(b.Summary, "3 claims active") && !strings.Contains(b.Summary, "2 claims active") {
		t.Errorf("the summary does not count active claims: %q", b.Summary)
	}
	quiet := platformStatusBlock(platformWith(v1.AccessOK, regs[3:]))
	if len(quiet.Reasons) != 0 || quiet.None != "Nothing refused or unfulfilled" {
		t.Errorf("a clean Platform shows reasons %v", quiet.Reasons)
	}
}

// TestPlatformStatusBlockLocksWhatItCannotCount (portal:D7:R2): with
// registrations the caller may not list, the claims part reads "claims
// locked", no refusal count shows, a locked note line says so, and a Ready
// reason still shows beside it.
func TestPlatformStatusBlockLocksWhatItCannotCount(t *testing.T) {
	regs := []v1.Registration{{Name: "a", Verdict: verdictRefused, Reason: "CatalogUnresolved"}}
	for name, conds := range map[string][]v1.Condition{
		"ready":         nil,
		"ready False":   {{Type: conditionReady, Status: "False", Reason: "BuildFailed"}},
		"contracts off": {{Type: conditionContracts, Status: "False", Reason: "UnfulfilledContracts"}},
	} {
		b := platformStatusBlock(platformWith(v1.AccessForbidden, regs, conds...))
		if !strings.Contains(b.Summary, "claims locked") || strings.Contains(b.Summary, "0 claim") {
			t.Errorf("%s: the summary is %q", name, b.Summary)
		}
		if len(b.Notes) != 1 || b.Notes[0].Class != "locked" || !strings.Contains(b.Notes[0].Text, "Refusals not shown") {
			t.Errorf("%s: no locked note: %+v", name, b.Notes)
		}
		for _, r := range b.Reasons {
			if r.Text == "CatalogUnresolved" {
				t.Errorf("%s: a refusal count shows for registrations the caller may not list", name)
			}
		}
	}
	b := platformStatusBlock(platformWith(v1.AccessForbidden, regs, v1.Condition{Type: conditionReady, Status: "False", Reason: "BuildFailed"}))
	out := renderMain(t, "platform", platformView{Block: b, Platform: v1.Platform{RegistrationsAccess: v1.AccessForbidden}})
	status := between(out, `id="platform-status"`, "</section>")
	if !strings.Contains(status, `class="sb-note locked">Refusals not shown: you may not list TransformerRegistrations`) || !strings.Contains(status, "BuildFailed") {
		t.Errorf("the locked note and the Ready reason do not show together:\n%s", status)
	}
}

// TestPlatformFormsCountOverTheDocument (platform-11, platform-43): Provides is
// a select of the registrations' contracts and every option carries its
// count; locked registrations leave Status and Provides without counts and
// the Claimed select out.
func TestPlatformFormsCountOverTheDocument(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	form := between(mainOf(s.get(t, "/").body), `aria-label="Filter providers"`, "</form>")
	for _, want := range []string{`<select id="f-provides"`, `Active (1)`, `Refused (1)`, `Inactive (0)`} {
		if !strings.Contains(form, want) {
			t.Errorf("the Providers form lacks %s:\n%s", want, form)
		}
	}
	cats := between(mainOf(s.get(t, "/?tab=catalogs").body), `aria-label="Filter catalogs"`, "</form>")
	for _, want := range []string{`Subscription (`, `Claimed only (1)`, `Unclaimed (`} {
		if !strings.Contains(cats, want) {
			t.Errorf("the Catalogs form lacks %s:\n%s", want, cats)
		}
	}
	locked := newSite(t, apitest.F1(t), apitest.DenyResources("transformerregistrations"))
	form = between(mainOf(locked.get(t, "/").body), `aria-label="Filter providers"`, "</form>")
	if strings.Contains(form, " (") && strings.Contains(between(form, `<select id="f-pstatus"`, "</select>"), " (") {
		t.Errorf("a Status option carries a count over locked registrations:\n%s", form)
	}
	cats = between(mainOf(locked.get(t, "/?tab=catalogs").body), `aria-label="Filter catalogs"`, "</form>")
	if strings.Contains(cats, `id="f-claimed"`) || strings.Contains(cats, "Claimed only (") {
		t.Errorf("the Claimed select or a claimed-only count shows over locked registrations:\n%s", cats)
	}
}

// TestPlatformEventsTableLinksItsResources (platform-23 to platform-26): the
// heading row holds the Resource select with counts and the count in its own
// followed region; a resource cell filters the feed and keeps the other
// parameters.
func TestPlatformEventsTableLinksItsResources(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	main := mainOf(s.get(t, "/?tab=catalogs").body)
	panel := between(main, `id="events-panel"`, "</section>")
	for _, want := range []string{
		`<optgroup label="Everything"><option value="" selected>All resources (`,
		`<optgroup label="Providers (TransformerRegistration)">`,
		`<p class="shown" id="events-shown" data-follow="`,
		`<table class="table rows events-table">`,
		`href="/?eresource=platform&amp;tab=catalogs" title="Show only this resource's events"`,
	} {
		if !strings.Contains(panel, want) {
			t.Errorf("the events panel lacks %s", want)
		}
	}
	region := regexp.MustCompile(`id="events-shown" data-follow="([^"]*)"`).FindStringSubmatch(panel)
	feed := regexp.MustCompile(`<div id="events" class="region" data-follow="([^"]*)"`).FindStringSubmatch(panel)
	if region == nil || feed == nil || region[1] != feed[1] {
		t.Errorf("the count region follows %v, the feed %v", region, feed)
	}
	only := between(mainOf(s.get(t, "/?eresource=platform").body), `id="events-panel"`, "</section>")
	if !strings.Contains(only, `<option value="platform" selected>`) || strings.Contains(only, "CatalogUnresolved") {
		t.Errorf("eresource=platform does not filter the table:\n%s", only)
	}
}
