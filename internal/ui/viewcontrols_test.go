package ui

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

const (
	cmRBAC     = "grp:configuration/" + certManagerID + "/rbac"
	cmCRDs     = "grp:configuration/" + certManagerID + "/crds"
	cmWebhooks = "grp:configuration/" + certManagerID + "/webhooks"
	cmPage     = "/instances/cert-manager/cert-manager"
)

// expandsOf is the expand values of a link.
func expandsOf(t *testing.T, href string) []string {
	t.Helper()
	u, err := url.Parse(strings.ReplaceAll(href, "&amp;", "&"))
	if err != nil {
		t.Fatalf("link %q: %v", href, err)
	}
	return u.Query()["expand"]
}

// hrefOf is the href of the first element whose opening tag holds marker.
func hrefOf(t *testing.T, page, marker string) string {
	t.Helper()
	m := regexp.MustCompile(`<a [^>]*href="([^"]*)"[^>]*` + regexp.QuoteMeta(marker)).FindStringSubmatch(page)
	if m == nil {
		return ""
	}
	return m[1]
}

// groupNodeHref is the href of the closed group node id.
func groupNodeHref(page, id string) string {
	m := regexp.MustCompile(`<a class="[^"]*" href="([^"]*)" data-panel="[^"]*" data-node="` + regexp.QuoteMeta(id) + `"`).FindStringSubmatch(page)
	if m == nil {
		return ""
	}
	return strings.ReplaceAll(m[1], "&amp;", "&")
}

// TestAccordionOpensOneConfigurationGroup: a closed group's link opens it
// and closes the other configuration groups, and keeps an expanded group of
// another kind. The legend says so, and this is what makes it true.
func TestAccordionOpensOneConfigurationGroup(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	main := mainOf(s.get(t, cmPage+"?expand="+url.QueryEscape(cmCRDs)+"&expand=grp:runtime/elsewhere").body)
	if !strings.Contains(main, "Click to open; one opens at a time") {
		t.Fatal("the legend lost its sentence about groups")
	}
	href := groupNodeHref(main, cmRBAC)
	if href == "" {
		t.Fatal("no closed RBAC group node on the page")
	}
	got := expandsOf(t, href)
	if len(got) != 2 || got[0] != "grp:runtime/elsewhere" || got[1] != cmRBAC {
		t.Errorf("RBAC opened from CRDs links to expand %v, want the other kind kept, CRDs dropped and RBAC added", got)
	}
	if !strings.Contains(href, "fit="+url.QueryEscape(cmRBAC)) {
		t.Errorf("the link %q does not fit the group", href)
	}
	frag := s.get(t, cmPage+"/node?id="+url.QueryEscape(cmRBAC)+"&expand="+url.QueryEscape(cmCRDs), htmxRequest).body
	m := regexp.MustCompile(`href="([^"]*)"[^>]*>Expand group`).FindStringSubmatch(frag)
	if m == nil {
		t.Fatalf("no Expand group link in the panel:\n%s", frag)
	}
	if got := expandsOf(t, m[1]); len(got) != 1 || got[0] != cmRBAC {
		t.Errorf("the panel's Expand links to expand %v, want RBAC alone", got)
	}
}

// TestViewControlsOfTheOverviewAndOfAnOpenGroup: Expand all opens every
// configuration group, Collapse all and Overview close them, and only the
// links that do something are drawn.
func TestViewControlsOfTheOverviewAndOfAnOpenGroup(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	rest := mainOf(s.get(t, cmPage+"?tab=graph").body)
	if strings.Contains(rest, "data-overview") || strings.Contains(rest, "data-collapse-all") {
		t.Error("the overview offers Overview or Collapse all")
	}
	all := hrefOf(t, rest, "data-expand-all")
	if got := expandsOf(t, all); len(got) != 3 {
		t.Errorf("Expand all links to %v, want the three configuration groups", got)
	}
	if !strings.Contains(rest, "22 of 69 boxes") {
		t.Error("the overview does not count 22 of 69 boxes")
	}
	open := mainOf(s.get(t, cmPage+"?tab=graph&expand="+url.QueryEscape(cmRBAC)).body)
	if strings.Contains(open, "data-whole-graph") || !strings.Contains(open, `title="Close RBAC and go back to the overview"`) {
		t.Error("an open RBAC group does not offer an Overview pill titled for it")
	}
	if href := hrefOf(t, open, "data-overview"); len(expandsOf(t, href)) != 0 || !strings.Contains(href, "tab=graph") {
		t.Errorf("Overview links to %q, want the graph with nothing open", href)
	}
	if href := hrefOf(t, open, "data-collapse-all"); len(expandsOf(t, href)) != 0 {
		t.Errorf("Collapse all links to %q", href)
	}
	if got := expandsOf(t, hrefOf(t, open, "data-expand-all")); len(got) != 3 {
		t.Errorf("Expand all with RBAC open links to %v, want all three", got)
	}
	if !strings.Contains(open, "60 of 69 boxes") {
		t.Error("RBAC open does not count 60 of 69 boxes")
	}
	two := mainOf(s.get(t, cmPage+"?tab=graph&expand="+url.QueryEscape(cmRBAC)+"&expand="+url.QueryEscape(cmCRDs)).body)
	if !strings.Contains(two, `title="Close the open groups and go back to the overview"`) {
		t.Error("two open groups: the Overview title does not name them together")
	}
}

// TestGroupConfigurationSwitch: it reads Grouped or Ungrouped in text, with
// no aria-pressed, and turns grouping off with group=off; under group=off
// the page expands every configuration group and draws no frame, no group
// node and no Expand all.
func TestGroupConfigurationSwitch(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	rest := mainOf(s.get(t, cmPage+"?tab=graph").body)
	if !strings.Contains(rest, `<span class="sw-state">Grouped</span>`) || strings.Contains(rest, "aria-pressed=\"true\" data-group-switch") {
		t.Error("the switch does not read Grouped as text")
	}
	if href := hrefOf(t, rest, "data-group-switch"); !strings.Contains(href, "group=off") {
		t.Errorf("the switch links to %q, want group=off", href)
	}
	off := mainOf(s.get(t, cmPage+"?tab=graph&group=off").body)
	if !strings.Contains(off, `<span class="sw-state">Ungrouped</span>`) {
		t.Error("the switch does not read Ungrouped")
	}
	if strings.Contains(off, "group-frame") || strings.Contains(off, "data-group ") || strings.Contains(off, "data-expand-all") {
		t.Error("the ungrouped view draws a frame, a group node or Expand all")
	}
	if n := strings.Count(off, `data-member-of=`); n != 16 {
		t.Errorf("the ungrouped view draws %d members, want the 16 components of the three groups", n)
	}
	if !strings.Contains(off, "69 of 69 boxes") {
		t.Error("the ungrouped view does not count 69 of 69 boxes")
	}
	if href := hrefOf(t, off, "data-group-switch"); strings.Contains(href, "group=off") {
		t.Errorf("turning grouping on links to %q", href)
	}
}

// TestPanelLinkOfAMemberCarriesTheOpenGroups: a member of an open group is
// not in the graph of the page without it, so its panel link names the
// groups, and the panel it fetches is the member's.
func TestPanelLinkOfAMemberCarriesTheOpenGroups(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	main := mainOf(s.get(t, cmPage+"?tab=graph&expand="+url.QueryEscape(cmRBAC)).body)
	m := regexp.MustCompile(`data-panel="([^"]*)" data-node="([^"]*)" data-member-of=`).FindStringSubmatch(main)
	if m == nil {
		t.Fatal("no member node in the open RBAC group")
	}
	panel := strings.ReplaceAll(m[1], "&amp;", "&")
	if !strings.Contains(panel, "expand="+url.QueryEscape(cmRBAC)) {
		t.Fatalf("the member's panel link %q lacks the open group", panel)
	}
	frag := s.get(t, panel, htmxRequest).body
	if !strings.Contains(frag, `class="frag-h`) || strings.Contains(frag, "Missing") {
		t.Errorf("the member's panel is empty or missing:\n%s", frag)
	}
}

// TestToolbarHoldsTheFittedZoomControls: Fit is pressed until the user zooms,
// the slider runs 30 to 200 in steps of 5, its label tells of Ctrl and
// scroll, and the full screen button is an icon with a name.
func TestToolbarHoldsTheFittedZoomControls(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	main := mainOf(s.get(t, cmPage+"?tab=graph").body)
	for _, want := range []string{
		`data-zoom="fit" aria-pressed="true">Fit<`,
		`type="range" min="30" max="200" step="5"`,
		`title="Or hold Ctrl and scroll over the graph"`,
		`aria-label="Show the graph full screen" title="Show the graph full screen"><svg class="ic ic-enter"`,
	} {
		if !strings.Contains(main, want) {
			t.Errorf("the toolbar lacks %s", want)
		}
	}
	if strings.Contains(main, ">Full screen<") {
		t.Error("the full screen button still has a word")
	}
}
