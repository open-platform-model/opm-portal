package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// The page script refreshes a region by replacing it with the region of
// the same id from one fresh render. These tests hold the markup to what
// that needs, without running the script.

var (
	idAttr     = regexp.MustCompile(`\sid="([^"]*)"`)
	logLinkRe  = regexp.MustCompile(`data-log-link="([^"]*)"`)
	followAttr = regexp.MustCompile(`id="(components|logs)"[^>]*data-follow="([^"]*)"`)
)

// TestALatePodGetsALogPane: a Pod that appears after the page loaded (the
// image-break rollout's new Pod) is listed in the logs region of the next
// render, and that region follows the same topic as the components region,
// so one refresh brings the Pod's row and its log pane together.
func TestALatePodGetsALogPane(t *testing.T) {
	const late = "podinfo-podinfo-794bd8c7fb-cc5ql"
	before := mainOf(newSite(t, apitest.F1(t), apitest.AllowAll).get(t, "/instances/default/podinfo").body)
	if strings.Contains(before, late) {
		t.Fatalf("%s is already on the page before the rollout", late)
	}
	after := mainOf(newSite(t, apitest.F1Broken(t), apitest.AllowAll).get(t, "/instances/default/podinfo").body)

	follows := map[string]string{}
	for _, m := range followAttr.FindAllStringSubmatch(after, -1) {
		follows[m[1]] = m[2]
	}
	if follows["logs"] == "" || follows["logs"] != follows["components"] {
		t.Errorf("the logs region follows %q, the components region %q", follows["logs"], follows["components"])
	}

	logs := between(after, `id="logs"`, "</section>")
	ids := map[string]bool{}
	for _, m := range idAttr.FindAllStringSubmatch(logs, -1) {
		ids[m[1]] = true
	}
	links := logLinkRe.FindAllStringSubmatch(between(after, `id="components"`, "</section>"), -1)
	if len(links) == 0 {
		t.Fatal("no Pod row offers its logs")
	}
	var lateLink bool
	for _, m := range links {
		if !ids[m[1]] {
			t.Errorf("Logs link %q names no pane in the logs region", m[1])
		}
		lateLink = lateLink || strings.Contains(m[1], late)
	}
	if !lateLink {
		t.Errorf("the late Pod %s has no Logs link", late)
	}

	got := strings.TrimSpace(collapse(`<section class="panel panel-wide reveal" `+logs+"</section>")) + "\n"
	golden := filepath.Join("testdata", "golden", "logs-podinfo-late-pod.html")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading %s (run with -update to write it): %v", golden, err)
	}
	if got != string(want) {
		t.Errorf("the late Pod's logs region differs from %s (run with -update after checking the diff):\n%s", golden, got)
	}
}

// TestElementIDsAreUnique: no rendered page repeats an element id, so a
// refresh, a Logs link and a kept-open fold each find the one element they
// name.
func TestElementIDsAreUnique(t *testing.T) {
	f1 := newSite(t, apitest.F1(t), apitest.AllowAll)
	broken := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	type page struct {
		s    *site
		path string
	}
	pages := []page{{broken, "/instances/default/podinfo"}}
	for _, p := range f1Pages {
		if !p.hx {
			pages = append(pages, page{f1, p.path})
		}
	}
	for _, p := range pages {
		seen := map[string]bool{}
		for _, m := range idAttr.FindAllStringSubmatch(p.s.get(t, p.path).body, -1) {
			if seen[m[1]] {
				t.Errorf("GET %s repeats id %q", p.path, m[1])
			}
			seen[m[1]] = true
		}
	}
}

// TestPaneIDsKeepNamesApart: the pane id of Pod a-b's container c is not
// the pane id of Pod a's container b-c.
func TestPaneIDsKeepNamesApart(t *testing.T) {
	if logID("a-b", "c") == logID("a", "b-c") {
		t.Errorf("logID collides: %q", logID("a-b", "c"))
	}
	if logID("a.b", "c") == logID("a", "b.c") {
		t.Errorf("logID collides: %q", logID("a.b", "c"))
	}
}

// TestOldRevisionsFoldHasAnID: the fold of scaled-down ReplicaSets carries
// an id, so a refresh keeps it open.
func TestOldRevisionsFoldHasAnID(t *testing.T) {
	main := mainOf(newSite(t, apitest.F1Broken(t), apitest.AllowAll).get(t, "/instances/default/podinfo").body)
	if strings.Contains(main, `<details class="old-revs">`) {
		t.Error("an old-revisions fold has no id")
	}
}

// TestStampedLabelsLeaveRoomForTheStamp: a node with an applied stamp
// clips its label shorter, so the stamp does not cover the label's end.
func TestStampedLabelsLeaveRoomForTheStamp(t *testing.T) {
	ready := v1.Reconcile{}
	label := "cert-manager/cert-manager"
	stamped := nodeOf(&v1.GraphNode{ID: "a", Kind: "instance", Label: label, Reconcile: &ready}, 200, 44, "/", "/node")
	plain := nodeOf(&v1.GraphNode{ID: "b", Kind: "instance", Label: label}, 200, 44, "/", "/node")
	if n := len([]rune(stamped.Label)); !stamped.Applied || n > stampedLabel {
		t.Errorf("stamped label %q is %d characters, want at most %d", stamped.Label, n, stampedLabel)
	}
	if n := len([]rune(plain.Label)); n > maxLabel || n <= stampedLabel {
		t.Errorf("unstamped label %q is %d characters, want %d to %d", plain.Label, n, stampedLabel+1, maxLabel)
	}
}

// TestCatalogSubLineShowsTheHost: two catalogs that share their path's
// tail but not its host read apart on the sub-line.
func TestCatalogSubLineShowsTheHost(t *testing.T) {
	for _, tc := range []struct{ label, want string }{
		{"opmodel.dev/catalogs/operator", "Catalog · opmodel.dev/…"},
		{"testing.opmodel.dev/catalogs/operator", "Catalog · testing.opmodel.d…"},
		{"opmodel.dev/operator", "Catalog · opmodel.dev"},
	} {
		_, sub := displayLabel(&v1.GraphNode{Kind: "catalog", Label: tc.label}, "Catalog", false)
		if sub != tc.want {
			t.Errorf("%s: sub-line %q, want %q", tc.label, sub, tc.want)
		}
	}
}

// TestContractsBannerIsNeutral: unfulfilled contracts on the Platform read
// as neutral information, in the banner as in the informational condition
// tone, never in a state color.
func TestContractsBannerIsNeutral(t *testing.T) {
	css, err := os.ReadFile("static/portal.css")
	if err != nil {
		t.Fatal(err)
	}
	rule := regexp.MustCompile(`(?m)^\.info \{[^}]*\}`).FindString(string(css))
	if !strings.Contains(rule, "var(--neutral)") || regexp.MustCompile(`--(healthy|degraded|progressing|unknown)`).MatchString(rule) {
		t.Errorf("the contracts banner is not neutral: %s", rule)
	}
}
