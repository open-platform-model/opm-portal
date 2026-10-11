package ui

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"testing"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
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

// fieldOptions returns the option texts of the named select field, "" for a
// control that is not a select.
func fieldOptions(f filterForm, name string) []string {
	for i := range f.Fields {
		if fld := &f.Fields[i]; fld.Name == name {
			out := make([]string, 0, len(fld.Select))
			for _, o := range fld.Select {
				out = append(out, o.Text)
			}
			return out
		}
	}
	return nil
}

func fieldNamed(f filterForm, name string) *field {
	for i := range f.Fields {
		if f.Fields[i].Name == name {
			return &f.Fields[i]
		}
	}
	return nil
}

// TestFilterFieldsFollowTheViewsOrderAndNameTheirEmptyChoice: one ordered
// list of fields, in the view's parameter order, each select's first option
// naming the empty choice in words (installed-02, installed-03).
func TestFilterFieldsFollowTheViewsOrderAndNameTheirEmptyChoice(t *testing.T) {
	form := newChoiceForm(parseFilters(&installedFilters, nil), "x", nil, formInput{Total: 0})
	names := make([]string, 0, len(form.Fields))
	for _, f := range form.Fields {
		names = append(names, f.Name)
	}
	want := "q kind provider uses namespace health applied owner module"
	if strings.Join(names, " ") != want {
		t.Errorf("fields = %v; want %s", names, want)
	}
	for name, first := range map[string]string{"kind": "Any kind", "provider": "Anything", "health": "Any health", "applied": "Any state", "owner": "Anyone"} {
		if got := fieldOptions(form, name); len(got) == 0 || got[0] != first {
			t.Errorf("%s offers %q first; want %q", name, got, first)
		}
	}
	for _, name := range []string{"uses", "namespace", "module"} {
		if fld := fieldNamed(form, name); fld == nil || fld.IsSelect {
			t.Errorf("%s is %+v; with no choices supplied it stays a text input", name, fld)
		}
	}
	prov := newChoiceForm(parseFilters(&providerFilters, nil), "x", nil, formInput{})
	if got := fieldOptions(prov, "pstatus"); strings.Join(got, "|") != "Any status|Accepted|Active|Refused|Inactive|Removal blocked|Pending" {
		t.Errorf("status options = %q", got)
	}
	cat := newChoiceForm(parseFilters(&catalogFilters, nil), "x", nil, formInput{})
	if got := fieldOptions(cat, "csource"); strings.Join(got, "|") != "Any source|Subscription|Contributed|Claimed only" {
		t.Errorf("source options = %q", got)
	}
	if got := fieldOptions(cat, "claimed"); strings.Join(got, "|") != "Either|Claimed|Unclaimed" {
		t.Errorf("claimed options = %q", got)
	}
}

// TestFilterOptionsReadTextAndCount: an option reads "Text (n)" when its
// count is known, and plain when it is not, never with a zero in place of
// an unknown count (portal:D7:R2); a parameter in NoCount shows none; a
// contract shows by its short name with the full value; a value in the URL
// among no choice is an extra selected option.
func TestFilterOptionsReadTextAndCount(t *testing.T) {
	const backup = "opmodel.dev/catalogs/opm/traits/backup@v1alpha1"
	q := url.Values{"namespace": {"team-z"}, "uses": {backup}, "kind": {"package"}}
	form := newChoiceForm(parseFilters(&installedFilters, q), "x", nil, formInput{
		Choices: map[string][]string{"namespace": {"cert-manager", "default"}, "uses": {backup, "opmodel.dev/catalogs/opm/crds/x@v1"}},
		Counts: map[string]map[string]int{
			"namespace": {"cert-manager": 1, "default": 3},
			"kind":      {"instance": 5}, // package: its list is locked, so no count
			"health":    {"Healthy": 4},
		},
		NoCount: map[string]bool{"health": true},
	})
	if got := fieldOptions(form, "namespace"); strings.Join(got, "|") != "All namespaces|cert-manager (1)|default (3)|team-z" {
		t.Errorf("namespace options = %q", got)
	}
	if fld := fieldNamed(form, "namespace"); !fld.IsSelect || !fld.Select[3].Selected || fld.Select[3].Value != "team-z" {
		t.Errorf("the URL value is not the selected extra option: %+v", fld.Select)
	}
	uses := fieldNamed(form, "uses")
	if uses.Select[1].Value != backup || uses.Select[1].Text != "traits/backup@v1alpha1" || !uses.Select[1].Selected {
		t.Errorf("uses option = %+v; want the full value, the short text, selected", uses.Select[1])
	}
	if got := fieldOptions(form, "kind"); strings.Join(got, "|") != "Any kind|Instance (5)|Package" {
		t.Errorf("kind options = %q; the option without a count must carry none", got)
	}
	for _, o := range fieldOptions(form, "health") {
		if strings.Contains(o, "(") {
			t.Errorf("health option %q carries a count although its parameter is in NoCount", o)
		}
	}
}

// TestFilterChipsReadLabelAndKeepTheFullValue (installed-05): the chip text
// is the value's words, a contract and a module path shortened, and the
// chip keeps the full value.
func TestFilterChipsReadLabelAndKeepTheFullValue(t *testing.T) {
	const backup = "opmodel.dev/catalogs/opm/traits/backup@v1alpha1"
	f := parseFilters(&installedFilters, url.Values{"uses": {backup}, "module": {"opmodel.dev/modules/web_app@v1"}, "applied": {"Failed"}})
	got := map[string][2]string{}
	for _, c := range f.Active {
		got[c.Label] = [2]string{c.Text, c.Value}
	}
	want := map[string][2]string{
		"Uses":    {"traits/backup@v1alpha1", backup},
		"Module":  {"web_app@v1", "opmodel.dev/modules/web_app@v1"},
		"Applied": {"Failed", "Failed"},
	}
	for label, w := range want {
		if got[label] != w {
			t.Errorf("chip %s = %v; want %v", label, got[label], w)
		}
	}
	if c := moduleChip("OCIRepository pkg/podinfo-release"); c != "OCIRepository pkg/podinfo-release" {
		t.Errorf("a package's source string is shortened to %q", c)
	}
}

// TestFilterSpecListsFixedValuesOnly: the remembered-filter spec names the
// fixed values of the enumerated parameters, the new Inactive among them, and
// leaves every parameter drawn from the rows as free text.
func TestFilterSpecListsFixedValuesOnly(t *testing.T) {
	var spec map[string]struct {
		Params map[string][]string `json:"params"`
	}
	if err := json.Unmarshal([]byte(filterSpec()), &spec); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(spec["providers"].Params["pstatus"], " "); got != "accepted active refused inactive blocked pending" {
		t.Errorf("pstatus values = %q", got)
	}
	for _, p := range []string{"uses", "namespace", "module", "q"} {
		if v, ok := spec["installed"].Params[p]; !ok || v != nil {
			t.Errorf("installed %s = %v (present %v); want free text", p, v, ok)
		}
	}
	if v, ok := spec["providers"].Params["provides"]; !ok || v != nil {
		t.Errorf("provides = %v (present %v); want free text", v, ok)
	}
}

// TestInactiveProviderStatus: Inactive is accepted and not active.
func TestInactiveProviderStatus(t *testing.T) {
	accepted := providerRow{Registration: v1.Registration{Accepted: true, Active: false}}
	active := providerRow{Registration: v1.Registration{Accepted: true, Active: true}}
	refused := providerRow{Registration: v1.Registration{Accepted: false, Verdict: verdictRefused}}
	f := map[string]string{"pstatus": "inactive"}
	if !matchesProvider(&accepted, f) || matchesProvider(&active, f) || matchesProvider(&refused, f) {
		t.Error("pstatus=inactive must match an accepted, not active registration only")
	}
}

// TestFilterFormsRenderTheCountRowAndTheChips: the count row is always
// there, with "Showing n of M" in its own followed region; a chip reads
// "Label: value", names its removal for assistive technology and holds the
// full value; "Clear all" shows when a filter is active; the form applies by
// replacing its history entry and aborting an older request (installed-05,
// installed-06, installed-22).
func TestFilterFormsRenderTheCountRowAndTheChips(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	all := mainOf(s.get(t, "/installed").body)
	for _, want := range []string{
		`<p class="shown" id="shown" data-follow="instances">Showing <strong>6</strong> of 6</p>`,
		`data-filters="installed" hx-sync="this:replace" hx-replace-url="true"`,
		`<span class="search-box"><svg class="search-icon" viewBox="0 0 24 24" aria-hidden="true">`,
	} {
		if !strings.Contains(all, want) {
			t.Errorf("/installed lacks %s", want)
		}
	}
	if strings.Contains(all, "Clear all") {
		t.Error("/installed shows Clear all with no filter active")
	}
	list := between(all, `id="list"`, "</section>")
	if strings.Contains(list, `class="shown"`) {
		t.Error("the count is still in the list region, which a refresh replaces whole")
	}
	filtered := mainOf(s.get(t, "/installed?namespace=default&uses=opmodel.dev/catalogs/opm/traits/backup@v1alpha1").body)
	for _, want := range []string{
		`title="default" data-filters-explicit aria-label="Remove Namespace filter"><span class="fchip-label">Namespace:</span> default`,
		`title="opmodel.dev/catalogs/opm/traits/backup@v1alpha1" data-filters-explicit aria-label="Remove Uses filter"><span class="fchip-label">Uses:</span> traits/backup@v1alpha1`,
		`data-clear-filters="installed">Clear all</a>`,
	} {
		if !strings.Contains(filtered, want) {
			t.Errorf("the filtered /installed lacks %s", want)
		}
	}
	// The count region follows the list's topic, so one refresh updates both.
	if f := regexp.MustCompile(`id="(shown|list)"[^>]*data-follow="([^"]*)"`).FindAllStringSubmatch(all, -1); len(f) != 2 || f[0][2] != f[1][2] {
		t.Errorf("the count and the list follow %v", f)
	}
	// The owner page's Resources reason form is not a filter form of the
	// list views: no count row, no apply-as-you-type, its own clear link.
	owner := mainOf(s.get(t, "/instances/default/podinfo?tab=resources&reason=ImagePullBackOff").body)
	form := between(owner, `aria-label="Filter resources"`, "</form>")
	if strings.Contains(form, "data-filters=") || strings.Contains(form, `class="shown"`) || !strings.Contains(form, "Clear filters") {
		t.Errorf("the Resources reason form changed:\n%s", form)
	}
	// The Platform tabs draw the strip, the count at the end of the field row,
	// and no count for providers the caller may not list.
	prov := mainOf(s.get(t, "/").body)
	if !strings.Contains(prov, `class="filters filters-strip"`) || !strings.Contains(prov, `<p class="shown" id="shown" data-follow="platform">Showing <strong>2</strong> of 2</p>`) {
		t.Error("the Providers tab lacks its strip or its count")
	}
	locked := mainOf(newSite(t, apitest.F1(t), apitest.DenyResources("transformerregistrations")).get(t, "/").body)
	if strings.Contains(between(locked, `id="parts"`, "</section>"), `class="shown"`) {
		t.Error("the Providers tab counts providers the caller may not list")
	}
}
