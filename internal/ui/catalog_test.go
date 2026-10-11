package ui

import (
	"net/http"
	"strings"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// TestBackupProvidersProviderTab (portal:D16:R3): the backup trait lists
// backup-consumer and links to Installed filtered by it.
func TestBackupProvidersProviderTab(t *testing.T) {
	main := mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/instances/default/backup-provider?tab=provider").body)
	tab := between(main, `id="provider"`, "</section>")
	for _, want := range []string{
		`href="/instances/default/backup-consumer"`,
		`href="/installed?uses=opmodel.dev%2Fcatalogs%2Fopm%2Ftraits%2Fbackup%40v1alpha1">All 1 in Installed</a>`,
		`href="/catalog?path=testing.opmodel.dev%2Fcatalogs%2Foperator%2Fbackup%40v0"`,
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
	if strings.Contains(mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/instances/default/podinfo").body), "?tab=provider") {
		t.Error("an instance that holds no registration offers a Provider tab")
	}
}

// TestProviderTabWithThePlatformForbidden: the tab keeps the name and the
// standing; conditions and catalog are locked; the page is no error.
func TestProviderTabWithThePlatformForbidden(t *testing.T) {
	res := newSite(t, apitest.F1(t), apitest.DenyResources("platforms")).get(t, "/instances/default/backup-provider?tab=provider")
	tab := between(mainOf(res.body), `id="provider"`, "</section>")
	if res.status != http.StatusOK || !strings.Contains(tab, "default.<wbr>backup-provider") || !strings.Contains(tab, ">Active</span>") ||
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
	if !strings.Contains(card, ">Refused</span>") || !strings.Contains(card, "ProviderMismatch") {
		t.Errorf("Provider card:\n%s", card)
	}
	tab := between(main, `id="provider"`, "</section>")
	if !strings.Contains(tab, "names ModuleInstance <span class=\"mono\">pkg/provider</span>, not this package") ||
		!strings.Contains(tab, "Claim names provider ModuleInstance pkg/provider, which does not exist") {
		t.Errorf("Provider tab:\n%s", tab)
	}
	if !strings.Contains(between(main, `id="owner-head"`, "</h1>"), "Provider, refused") {
		t.Error("no Provider pill on the package")
	}
}

func TestCatalogPage(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	backup := mainOf(s.get(t, "/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0").body)
	for _, want := range []string{"Catalog · From a provider", `<span class="mono">0.1.0</span>`, "contributed by <span class=\"mono\">default.backup-provider</span>",
		">Resolved</span>", `href="/instances/default/backup-provider?tab=provider"`, "does not record a catalog's definitions"} {
		if !strings.Contains(backup, want) {
			t.Errorf("backup catalog lacks %s", want)
		}
	}
	refused := mainOf(s.get(t, "/catalog?path=testing.opmodel.dev/catalogs/operator/refused-claim-fixture-absent@v0").body)
	if !strings.Contains(refused, "Claimed only") || !strings.Contains(refused, "Not resolved</span> <span class=\"mono\">CatalogUnresolved</span>") {
		t.Errorf("refused claim's catalog:\n%s", between(refused, `id="catalog-resolved"`, "</section>"))
	}
	if res := s.get(t, "/catalog?path=example.com/nothing@v0"); res.status != http.StatusNotFound {
		t.Errorf("unknown catalog: %d; want 404", res.status)
	}
	locked := newSite(t, apitest.F1(t), apitest.DenyResources("platforms"))
	a := locked.get(t, "/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0")
	b := locked.get(t, "/catalog?path=example.com/nothing@v0")
	if a.status != http.StatusForbidden || b.status != http.StatusForbidden || mainOf(a.body) != mainOf(b.body) {
		t.Errorf("Platform forbidden: %d and %d, bodies alike %v; want the same 403", a.status, b.status, mainOf(a.body) == mainOf(b.body))
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
	const path = "/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0"
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
