package ui

import (
	"net/http"
	"strings"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// TestBackupProvidersProviderTab (portal:D16:R3): the backup trait lists
// backup-consumer and links to Installed filtered by it.
func TestBackupProvidersProviderTab(t *testing.T) {
	main := mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/instances/default/backup-provider?tab=provider").body)
	tab := between(main, `id="provider"`, "</section>")
	for _, want := range []string{
		`href="/instances/default/backup-consumer"`,
		`href="/installed?uses=opmodel.dev%2Fcatalogs%2Fopm%2Ftraits%2Fbackup%40v1alpha1" title="Open Installed filtered to everything that uses opmodel.dev/catalogs/opm/traits/backup@v1alpha1">Show in Installed →</a>`,
		"<dt>Contracts</dt><dd>1 provided, 1 in use</dd>", "This instance is also a provider: it registers a catalog with the platform",
		`<h4 class="sec-h">Registration conditions</h4>`, `<span class="cond-badge tone-normal">True</span>`,
		`<span class="contract-pill" title="opmodel.dev/catalogs/opm/traits/backup@v1alpha1">traits/backup@v1alpha1</span>`,
		`href="/catalog?path=testing.opmodel.dev%2Fcatalogs%2Fcontroller%2Fbackup%40v0"`,
		"In the Platform's resolved registry, contributed by this registration.",
		">Ready</th>", ">Accepted</td>",
	} {
		if !strings.Contains(tab, want) {
			t.Errorf("Provider tab lacks %s", want)
		}
	}
	if !strings.Contains(main, `href="/instances/default/backup-provider?tab=provider" aria-current="page">Provider</a>`) {
		t.Error("no current Provider tab")
	}
	// The standing is said once (portal:D4:R4): two marks, no success reason beside them.
	if strings.Count(tab, ">accepted</span>") != 1 || strings.Count(tab, ">active</span>") != 1 || strings.Contains(tab, `<span class="mono">Accepted</span>`) {
		t.Errorf("the standing is not said once:\n%s", between(tab, `class="pills"`, "</p>"))
	}
	if strings.Contains(tab, "prov-active") || strings.Contains(tab, "claim-badge") {
		t.Error("the claim badge is still on the tab")
	}
	if strings.Contains(mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/instances/default/podinfo").body), "?tab=provider") {
		t.Error("an instance that holds no registration offers a Provider tab")
	}
}

// TestProviderTabWithThePlatformForbidden: the tab keeps the name and the
// standing; conditions and catalog are locked; the page is no error.
func TestProviderTabWithThePlatformForbidden(t *testing.T) {
	res := newSite(t, apitest.F1(t), apitest.DenyResources("platforms")).get(t, "/instances/default/backup-provider?tab=provider")
	tab := between(mainOf(res.body), `id="provider"`, "</section>")
	if res.status != http.StatusOK || !strings.Contains(tab, "default.<wbr>backup-provider") || !strings.Contains(tab, ">accepted</span>") || !strings.Contains(tab, ">active</span>") ||
		!strings.Contains(tab, "locked-panel") || strings.Contains(tab, "/catalog?path=") {
		t.Errorf("Platform forbidden: %d\n%s", res.status, tab)
	}
}

// TestPackageHolderProviderTab (portal:D15:R3): a package holding a refused
// claim shows Refused with the controller's reason, the providerRef
// beside it, and nothing saying the refusal is wrong.
func TestPackageHolderProviderTab(t *testing.T) {
	s := newSite(t, apitest.F1PackageHolder(t), apitest.AllowAll)
	main := mainOf(s.get(t, "/packages/pkg/provider?tab=provider").body)
	card := between(main, `id="provider-card"`, "</section>")
	if !strings.Contains(card, `<span class="sb-word">Refused</span>`) || !strings.Contains(card, "ProviderMismatch") {
		t.Errorf("Provider card:\n%s", card)
	}
	tab := between(main, `id="provider"`, "</section>")
	if !strings.Contains(tab, "names ModuleInstance <span class=\"mono\">pkg/provider</span>, not this package") ||
		!strings.Contains(tab, "Claim names provider ModuleInstance pkg/provider, which does not exist") {
		t.Errorf("Provider tab:\n%s", tab)
	}
	if !strings.Contains(between(main, `id="owner-head"`, "</h1>"), `Provider</span><span class="vh">standing: refused</span>`) {
		t.Error("no Provider pill on the package")
	}
}

const (
	backupCatalog  = "/catalog?path=testing.opmodel.dev/catalogs/controller/backup@v0"
	refusedCatalog = "/catalog?path=testing.opmodel.dev/catalogs/controller/refused-claim-fixture-absent@v0"
	opmCatalog     = "/catalog?path=opmodel.dev/catalogs/opm@v4"
)

func TestCatalogPage(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	backup := mainOf(s.get(t, backupCatalog).body)
	for _, want := range []string{`<span class="origin origin-registration">From a provider</span>`, `<dt>Enabled</dt><dd>yes</dd>`,
		`<a class="mono" href="/instances/default/backup-provider?tab=provider">default.<wbr>backup-provider</a></dd>`,
		`<span class="sb-word">Resolved</span>`, "does not record a catalog's definitions"} {
		if !strings.Contains(backup, want) {
			t.Errorf("backup catalog lacks %s", want)
		}
	}
	refused := mainOf(s.get(t, refusedCatalog).body)
	if !strings.Contains(refused, "origin-claim\">Claimed only") || !strings.Contains(refused, `<span class="sb-word">Unresolved</span>`) ||
		!strings.Contains(refused, `<span class="mono">CatalogUnresolved</span>`) {
		t.Errorf("refused claim's catalog:\n%s", between(refused, `id="catalog-resolved"`, "</section>"))
	}
	if res := s.get(t, "/catalog?path=example.com/nothing@v0"); res.status != http.StatusNotFound {
		t.Errorf("unknown catalog: %d; want 404", res.status)
	}
	locked := newSite(t, apitest.F1(t), apitest.DenyResources("platforms"))
	a := locked.get(t, backupCatalog)
	b := locked.get(t, "/catalog?path=example.com/nothing@v0")
	if a.status != http.StatusForbidden || b.status != http.StatusForbidden || mainOf(a.body) != mainOf(b.body) {
		t.Errorf("Platform forbidden: %d and %d, bodies alike %v; want the same 403", a.status, b.status, mainOf(a.body) == mainOf(b.body))
	}
}

// TestCatalogHeadlineIsTheShortName (catalog-01, catalog-02, catalog-24): the
// h1 is the short name in the display font, the full path and version sit on
// a line below, the Version row is gone, the breadcrumb ends with the short
// name in ink, and the origin is a pill beside the "Catalog" eyebrow.
func TestCatalogHeadlineIsTheShortName(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, c := range []struct{ path, short, origin, class string }{
		{backupCatalog, "backup", "From a provider", "origin-registration"},
		{opmCatalog, "opm", "Subscribed", "origin-subscription"},
		{refusedCatalog, "refused-claim-fixture-absent", "Claimed only", "origin-claim"},
	} {
		main := mainOf(s.get(t, c.path).body)
		head := between(main, `id="catalog-head"`, "</section>")
		for _, want := range []string{"<h1>" + c.short + "</h1>", `<p class="kicker">Catalog <span class="origin ` + c.class + `">` + c.origin + `</span></p>`, `<p class="catalog-path mono">`} {
			if !strings.Contains(head, want) {
				t.Errorf("%s: identity card lacks %s:\n%s", c.path, want, head)
			}
		}
		if strings.Contains(head, "<dt>Version</dt>") || strings.Contains(head, `<h1 class="mono"`) {
			t.Errorf("%s: the Version row or the mono headline is back:\n%s", c.path, head)
		}
		if !strings.Contains(main, `<span class="crumb-here" aria-current="page">`+c.short+`</span>`) {
			t.Errorf("%s: the last breadcrumb is not the short name", c.path)
		}
	}
	if line := between(mainOf(s.get(t, opmCatalog).body), `class="catalog-path mono"`, "</p>"); !strings.Contains(line, "· 4.") {
		t.Errorf("the path line lacks the version: %s", line)
	}
}

// TestCatalogShortFallsBackWhenTwoShareTheSegment: two catalogs with one last
// segment show the longer form contractShort gives, not the bare segment.
func TestCatalogShortFallsBackWhenTwoShareTheSegment(t *testing.T) {
	all := []string{"a.dev/catalogs/x/backup@v0", "b.dev/catalogs/y/backup@v1", "a.dev/catalogs/opm@v4"}
	for path, want := range map[string]string{
		"a.dev/catalogs/x/backup@v0": "x/backup@v0",
		"b.dev/catalogs/y/backup@v1": "y/backup@v1",
		"a.dev/catalogs/opm@v4":      "opm",
	} {
		if got := catalogShort(path, all); got != want {
			t.Errorf("catalogShort(%s) = %q; want %q", path, got, want)
		}
	}
}

// TestCatalogSourceLinksItsRegistration (catalog-05, catalog-25): a
// subscription reads "Platform subscription"; a contributed catalog names its
// registration, linked to the one holder; a claimed-only catalog lists its
// claimants; several holders follow the name as a muted "held by"; none leave
// the name plain. Nothing reads "contributed, contributed by".
func TestCatalogSourceLinksItsRegistration(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	if d := between(mainOf(s.get(t, opmCatalog).body), "<dt>Source</dt>", "</dd>"); !strings.Contains(d, "Platform subscription") {
		t.Errorf("subscription source: %s", d)
	}
	d := between(mainOf(s.get(t, backupCatalog).body), "<dt>Source</dt>", "</dd>")
	if !strings.Contains(d, `<a class="mono" href="/instances/default/backup-provider?tab=provider">`) || strings.Contains(d, "contributed") {
		t.Errorf("contributed source: %s", d)
	}
	if d := between(mainOf(s.get(t, refusedCatalog).body), "<dt>Source</dt>", "</dd>"); !strings.Contains(d, `<span class="mono">default.<wbr>refused-claim-fixture</span>`) || strings.Contains(d, "<a ") {
		t.Errorf("claimed-only source with no holder: %s", d)
	}

	p := v1.Platform{RegistrationsAccess: v1.AccessOK, Registrations: []v1.Registration{
		{Name: "many", Catalog: "c.dev/x@v1", Accepted: true, Active: true, Verdict: "Accepted",
			HeldBy: []v1.ObjectRef{{Kind: "ModuleInstance", Namespace: "a", Name: "one"}, {Kind: "ModuleInstance", Namespace: "b", Name: "two"}}},
		{Name: "single", Catalog: "c.dev/x@v1", Verdict: "Pending", HeldBy: []v1.ObjectRef{{Kind: "ModuleInstance", Namespace: "a", Name: "one"}}},
	}}
	rows, _ := catalogRows(&p, nil)
	v := catalogViewOf(&p, rows[0], rows)
	if len(v.SourceLinks) != 2 || v.SourceLinks[0].Href != "" || len(v.SourceLinks[0].Holders) != 2 || v.SourceLinks[1].Href == "" {
		t.Fatalf("source links: %+v", v.SourceLinks)
	}
	got := renderPartial(t, "claim-link", v.SourceLinks[0])
	if !strings.Contains(got, `<span class="muted">held by <a class="mono" href="/instances/a/one?tab=provider">a/one</a>, <a class="mono" href="/instances/b/two?tab=provider">b/two</a></span>`) || strings.Contains(got, `href="/instances/a/one?tab=provider">`+"many") {
		t.Errorf("a name with several holders:\n%s", got)
	}
}

// TestCatalogEnabledForACatalogTheRegistryLacks (catalog-07): a catalog the
// registry does not hold reads "no: not in the registry", not "not recorded".
func TestCatalogEnabledForACatalogTheRegistryLacks(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	if d := between(mainOf(s.get(t, refusedCatalog).body), "<dt>Enabled</dt>", "</dd>"); !strings.Contains(d, "no: not in the registry") || strings.Contains(d, "not recorded") {
		t.Errorf("unresolved Enabled: %s", d)
	}
}

// TestCatalogBlockSaysSinceWhenTheClaimWasRefused (catalog-08, catalog-26):
// the F1 refused claim reads "refused since" the time its Ready condition
// turned False, names its reason with a count linking to the Events tab, and
// says who claims it; a resolved catalog shows no time and "No resolution
// errors".
func TestCatalogBlockSaysSinceWhenTheClaimWasRefused(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	refusedMain := mainOf(s.get(t, refusedCatalog).body)
	if !strings.Contains(refusedMain, `<section class="state-block hue-degraded" id="catalog-resolved"`) {
		t.Error("the refused block is not in the degraded hue")
	}
	block := between(refusedMain, `id="catalog-resolved"`, "</section>")
	for _, want := range []string{`refused since <time datetime="2026-10-21T16:04:50Z"`, `<span class="sb-word">Unresolved</span>`,
		"Not in the resolved registry. Claimed by default.refused-claim-fixture",
		`reason=CatalogUnresolved&amp;tab=events" class="hue-degraded"><strong>1</strong> <span class="mono">CatalogUnresolved</span>`} {
		if !strings.Contains(block, want) {
			t.Errorf("refused block lacks %s:\n%s", want, block)
		}
	}
	for _, c := range []struct{ path, summary string }{
		{backupCatalog, "In the resolved registry at 0.1.0, contributed by its provider&#39;s registration"},
		{opmCatalog, "from the Platform&#39;s subscription"},
	} {
		main := mainOf(s.get(t, c.path).body)
		block := between(main, `id="catalog-resolved"`, "</section>")
		if !strings.Contains(block, c.summary) || !strings.Contains(block, "No resolution errors") || strings.Contains(block, "sb-when") ||
			!strings.Contains(main, `<section class="state-block hue-applied" id="catalog-resolved"`) {
			t.Errorf("%s: resolved block:\n%s", c.path, block)
		}
	}
}

// TestCatalogBlockOfAnUnresolvedSubscription: a subscription the registry
// lacks shows the Platform's Ready reason with no count and "since" the time
// that condition changed; a claim whose registration has no Ready condition
// falls back to the Platform's own "since".
func TestCatalogBlockOfAnUnresolvedSubscription(t *testing.T) {
	ready := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	since := time.Date(2026, 10, 4, 7, 0, 0, 0, time.UTC)
	p := v1.Platform{RegistrationsAccess: v1.AccessOK, Reconcile: v1.Reconcile{Since: &since},
		Subscriptions: []v1.Subscription{{Catalog: "c.dev/sub@v1", Version: "1.0.0"}},
		Conditions:    []v1.Condition{{Type: "Ready", Status: "False", Reason: "CatalogsUnresolved", LastTransitionTime: &ready}},
		Registrations: []v1.Registration{{Name: "cl", Catalog: "c.dev/claim@v1", Verdict: "Refused", Reason: "CatalogUnresolved"}}}
	rows, _ := catalogRows(&p, nil)
	sub, claim := catalogViewOf(&p, rows[0], rows).Block, catalogViewOf(&p, rows[1], rows).Block
	if sub.State != "Unresolved" || sub.WhenText != "since" || sub.When == nil || !sub.When.Equal(ready) ||
		len(sub.Reasons) != 1 || sub.Reasons[0].Text != "CatalogsUnresolved" || sub.Reasons[0].N != 0 || !strings.Contains(sub.Reasons[0].Href, "reason=CatalogsUnresolved&tab=events") {
		t.Errorf("subscription block: %+v", sub)
	}
	if claim.WhenText != "refused since" || claim.When == nil || !claim.When.Equal(since) || len(claim.Reasons) != 1 || claim.Reasons[0].N != 1 {
		t.Errorf("claim block with no Ready condition: %+v", claim)
	}
}

// TestPlatformLinksToTheNewPages: provider holders open their Provider
// tab; catalogs open the Catalog page.
func TestPlatformLinksToTheNewPages(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	if !strings.Contains(mainOf(s.get(t, "/").body), `<a class="mono" href="/instances/default/backup-provider?tab=provider">default/backup-provider</a>`) {
		t.Error("the provider row's holder does not open its Provider tab")
	}
	if !strings.Contains(mainOf(s.get(t, "/?tab=catalogs").body), `href="/catalog?path=opmodel.dev%2Fcatalogs%2Fopm%40v4"`) {
		t.Error("the catalog row does not open the Catalog page")
	}
}

// TestUsedByForANamespaceReader: a caller who may list instances only in
// default still sees backup-consumer use the backup trait, marked as
// possibly incomplete; one who may list none sees the list locked, with no
// incomplete note under it.
func TestUsedByForANamespaceReader(t *testing.T) {
	onlyDefault := func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return ra.Resource != "moduleinstances" || ra.Verb != "list" || ra.Namespace == "default"
	}
	tab := between(mainOf(newSite(t, apitest.F1(t), onlyDefault).get(t, "/instances/default/backup-provider?tab=provider").body), `id="provider"`, "</section>")
	if !strings.Contains(tab, `href="/instances/default/backup-consumer"`) || !strings.Contains(tab, "may be incomplete") {
		t.Errorf("namespace reader's Used by:\n%s", tab)
	}
	noList := func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return ra.Resource != "moduleinstances" || ra.Verb != "list"
	}
	locked := between(mainOf(newSite(t, apitest.F1(t), noList).get(t, "/instances/default/backup-provider?tab=provider").body), `id="provider"`, "</section>")
	if !strings.Contains(locked, "you may not list instances") || strings.Contains(locked, "may be incomplete") {
		t.Errorf("no instance listable: Used by:\n%s", locked)
	}
}

// TestCatalogClaimsLockedWithRegistrations: the Claims tab is locked, not
// empty, when the caller may not list registrations.
func TestCatalogClaimsLockedWithRegistrations(t *testing.T) {
	main := mainOf(newSite(t, apitest.F1(t), apitest.DenyResources("transformerregistrations")).get(t, "/catalog?path=opmodel.dev/catalogs/opm@v4").body)
	claims := between(main, `id="claims"`, "</section>")
	if !strings.Contains(claims, "locked-panel") || strings.Contains(claims, "No registration you may read") {
		t.Errorf("Claims tab:\n%s", claims)
	}
}

// TestCatalogTabsCarryTheirCounts (portal:D19:R3, portal:D7): Claims shows
// the claiming registrations, and an empty count span when they are locked;
// Events never carries a count, because the page reads events only on that tab.
func TestCatalogTabsCarryTheirCounts(t *testing.T) {
	const path = "/catalog?path=testing.opmodel.dev/catalogs/controller/backup@v0"
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, p := range []string{path, path + "&tab=events"} {
		tabs := tabsOf(mainOf(s.get(t, p).body))
		if !strings.Contains(tabs, `Claims <span class="tab-n" id="tab-n-claims" data-follow="platform">1</span>`) || !strings.Contains(tabs, `>Events</a>`) {
			t.Errorf("%s: tabs:\n%s", p, tabs)
		}
	}
	locked := newSite(t, apitest.F1(t), apitest.DenyResources("transformerregistrations"))
	tabs := tabsOf(mainOf(locked.get(t, "/catalog?path=opmodel.dev/catalogs/opm@v4").body))
	if !strings.Contains(tabs, `Claims<span class="tab-n" id="tab-n-claims" data-follow="platform"></span></a>`) || strings.Contains(tabs, "tab-n-events") {
		t.Errorf("the locked claims keep an empty followed count span and Events none:\n%s", tabs)
	}
}

// TestCatalogEventsTab (catalog-18, catalog-19): the Events tab lists the
// events of the Platform and the claimants as a table without the Resource
// column, with a Type select whose options carry their counts, and "Showing x
// of N" in its own followed region.
func TestCatalogEventsTab(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for path, shown := range map[string]string{backupCatalog: "Showing <strong>1</strong> of 1", refusedCatalog: "Showing <strong>2</strong> of 2"} {
		main := mainOf(s.get(t, path+"&tab=events").body)
		panel := between(main, `id="events-panel"`, "</section>")
		for _, want := range []string{`<table class="table rows events-table">`, `<option value="" selected>Any type</option>`, `<option value="Normal">Normal (`, `<option value="Warning">Warning (`,
			`<p class="shown" id="events-shown" data-follow="events:platform events:registration:`, shown} {
			if !strings.Contains(panel, want) {
				t.Errorf("%s: Events tab lacks %s:\n%s", path, want, panel)
			}
		}
		if strings.Contains(panel, "c-resource") || strings.Contains(panel, ">Resource<") {
			t.Errorf("%s: the Events table keeps a Resource column", path)
		}
	}
}

// TestCatalogEventsReasonDeepLink: the reason link of the Resolved block opens
// the Events tab filtered to it; the form keeps path, tab and reason as hidden
// fields, so a Type choice keeps the reason; the Type field is not hidden; and
// a Type that leaves no event says so.
func TestCatalogEventsReasonDeepLink(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	link := strings.TrimPrefix(between(mainOf(s.get(t, refusedCatalog).body), `class="sb-reasons"><a href="`, `" class=`), `class="sb-reasons"><a href="`)
	if !strings.HasSuffix(link, "reason=CatalogUnresolved&amp;tab=events") {
		t.Fatalf("the reason link: %s", link)
	}
	main := mainOf(s.get(t, refusedCatalog+"&tab=events&reason=CatalogUnresolved").body)
	panel := between(main, `id="events-panel"`, "</section>")
	for _, want := range []string{`<input type="hidden" name="path" value="testing.opmodel.dev/catalogs/controller/refused-claim-fixture-absent@v0">`,
		`<input type="hidden" name="reason" value="CatalogUnresolved">`, `<input type="hidden" name="tab" value="events">`,
		`aria-label="Remove Reason filter"><span class="fchip-label">Reason:</span> CatalogUnresolved`, "Showing <strong>1</strong> of 2", `<td data-label="Reason" class="mono ev-reason">CatalogUnresolved</td>`} {
		if !strings.Contains(panel, want) {
			t.Errorf("deep link lacks %s:\n%s", want, panel)
		}
	}
	if strings.Contains(panel, `type="hidden" name="type"`) || strings.Contains(panel, "Normal</td>") {
		t.Error("the Type field is hidden too, or a Normal event passes the reason filter")
	}
	// Choosing a Type submits the form: its fields are the hidden ones and the select.
	warn := mainOf(s.get(t, refusedCatalog+"&tab=events&reason=CatalogUnresolved&type=Warning").body)
	if p := between(warn, `id="events-panel"`, "</section>"); !strings.Contains(p, `name="reason" value="CatalogUnresolved"`) || !strings.Contains(p, `<option value="Warning" selected>`) || !strings.Contains(p, "Showing <strong>1</strong> of 2") {
		t.Errorf("Type=Warning with the reason:\n%s", p)
	}
	none := mainOf(s.get(t, refusedCatalog+"&tab=events&reason=CatalogUnresolved&type=Normal").body)
	if p := between(none, `id="events-panel"`, "</section>"); !strings.Contains(p, "No events match this filter.") || !strings.Contains(p, "Showing <strong>0</strong> of 2") {
		t.Errorf("Type=Normal with the reason:\n%s", p)
	}
}

// TestCatalogEventsWithRegistrationsForbidden: a caller who may not list
// registrations sees the Platform's events only, the Claims tab locked and no
// error.
func TestCatalogEventsWithRegistrationsForbidden(t *testing.T) {
	res := newSite(t, apitest.F1(t), apitest.DenyResources("transformerregistrations")).get(t, opmCatalog+"&tab=events")
	main := mainOf(res.body)
	if res.status != http.StatusOK || !strings.Contains(main, `<table class="table rows events-table">`) || strings.Contains(main, "events:registration:") ||
		!strings.Contains(main, `data-follow="events:platform"`) {
		t.Errorf("registrations forbidden: %d\n%s", res.status, between(main, `id="events-panel"`, "</section>"))
	}
}
