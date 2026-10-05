package ui

import (
	"net/url"
	"strings"
	"testing"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

func TestParseFilters(t *testing.T) {
	q := url.Values{"health": {"Degraded"}, "kind": {"thing"}, "q": {" pod "}, "tab": {"x"}, "namespace": {""}}
	f := parseFilters(&installedFilters, q)
	if f.Values["health"] != "Degraded" || f.Values["q"] != "pod" || f.Values["kind"] != "" || f.Values["namespace"] != "" {
		t.Errorf("values = %v", f.Values)
	}
	if len(f.Ignored) != 1 || f.Ignored[0].Name != "kind" || f.Ignored[0].Remove != "/installed?health=Degraded&q=+pod+&tab=x" {
		t.Errorf("ignored = %+v", f.Ignored)
	}
	chips := make([]string, 0, len(f.Active))
	for _, c := range f.Active {
		chips = append(chips, c.Label+"="+c.Text+" -> "+c.Remove)
	}
	// A chip drops its own parameter and the ignored ones, and keeps tab.
	want := []string{"Search=pod -> /installed?health=Degraded&tab=x", "Health=Degraded -> /installed?q=+pod+&tab=x"}
	if strings.Join(chips, "|") != strings.Join(want, "|") {
		t.Errorf("chips = %q; want %q", chips, want)
	}
	if f.Clear != "/installed?tab=x" {
		t.Errorf("clear = %q; tab is not a filter and stays", f.Clear)
	}
}

// TestInstalledFilters filters the F1 list on the server, so each view is
// a plain link (portal:D14:R2, portal:D16:R2).
func TestInstalledFilters(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for query, want := range map[string][]string{
		"kind=package":                          {"/packages/pkg/podinfo"},
		"kind=instance&owner=cli":               {"/instances/web/web"},
		"provider=yes":                          {"/instances/default/backup-provider"},
		"applied=Failed":                        {"/packages/pkg/podinfo"},
		"module=opmodel.dev/modules/web_app@v1": {"/instances/web/web"},
		"namespace=Not_A_Label":                 nil,
		"uses=opmodel.dev/catalogs/opm/traits/backup@v1alpha1": {"/instances/default/backup-consumer"},
	} {
		main := mainOf(s.get(t, "/installed?"+query).body)
		got := rowLinks(main)
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("/installed?%s lists %v; want %v", query, got, want)
		}
	}
	main := mainOf(s.get(t, "/installed?health=Bogus").body)
	if len(rowLinks(main)) != 6 || !strings.Contains(main, "Ignored filter <span class=\"mono\">health=Bogus</span>") {
		t.Errorf("an unknown health value is not ignored with a note:\n%s", main)
	}
	if main := mainOf(s.get(t, "/installed?uses=x").body); !strings.Contains(main, "Packages are not included") {
		t.Error("the uses filter does not say packages are left out")
	}
}

// rowLinks returns the Installed rows' links, in order.
func rowLinks(main string) []string {
	parts := strings.Split(main, `<th scope="row" data-label="Name"><a href="`)[1:]
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		link, _, _ := strings.Cut(part, `"`)
		out = append(out, link)
	}
	return out
}

// TestInstalledPackagesForbidden: one kind locked, the other listed.
func TestInstalledPackagesForbidden(t *testing.T) {
	main := mainOf(newSite(t, apitest.F1(t), apitest.DenyResources("modulepackages")).get(t, "/installed").body)
	if !strings.Contains(main, "Module packages</strong>: locked") || len(rowLinks(main)) != 5 {
		t.Errorf("packages forbidden: want the five instances and a locked package group:\n%s", main)
	}
}
