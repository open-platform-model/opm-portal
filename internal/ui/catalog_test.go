package ui

import (
	"net/http"
	"strings"
	"testing"

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
