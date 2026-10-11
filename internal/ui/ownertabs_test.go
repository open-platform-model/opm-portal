package ui

import (
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// The Events, Logs and YAML tabs (align-owner-pages, section 5).

const (
	podinfoOwner = "opmodel.dev/ModuleInstance/default/podinfo"
	eventsTabURL = "/instances/default/podinfo?tab=events"
)

var (
	optionTag   = regexp.MustCompile(`<option value="([^"]*)"( selected| disabled)*>([^<]*)</option>`)
	resourceRef = regexp.MustCompile(`href="[^"]*[?&;]resource=([^"&]+)`)
	allCount    = regexp.MustCompile(`All (?:readable )?resources \((\d+)\)`)
	showing     = regexp.MustCompile(`Showing <strong>(\d+)</strong> of (\d+)`)
)

// eventsRegion is the Events tab's section of a page.
func eventsRegion(page string) string { return between(mainOf(page), `id="events"`, "</section>") }

// TestEventsTabOpensOnTheMergedFeed (5.1): with no resource filter the tab
// lists the Deployment's ScalingReplicaSet event beside the owner's own, and
// "All resources" counts the lines the tab count counts.
func TestEventsTabOpensOnTheMergedFeed(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	page := s.get(t, eventsTabURL).body
	region := eventsRegion(page)
	if !strings.Contains(region, "ScalingReplicaSet") {
		t.Errorf("the Deployment's ScalingReplicaSet event is not in the default list:\n%s", region)
	}
	m := allCount.FindStringSubmatch(region)
	count := regexp.MustCompile(`<span class="tab-n" id="tab-n-events"[^>]*>(\d+)</span>`).FindStringSubmatch(between(page, `class="tabs"`, "</nav>"))
	sh := showing.FindStringSubmatch(region)
	if m == nil || sh == nil || sh[1] != sh[2] || sh[1] != m[1] {
		t.Fatalf("All resources %v, showing %v: want one number", m, sh)
	}
	if len(count) > 1 && count[1] != m[1] {
		t.Errorf("the tab counts %s lines, the choice says %s", count[1], m[1])
	}
	if !strings.Contains(region, `<optgroup label="Everything">`) || !strings.Contains(region, "This instance (") {
		t.Errorf("the choice lacks the Everything group with the owner:\n%s", region)
	}
}

// TestEventsChoiceOffersOnlyObjectsWithEvents (5.1, portal:D14:R7): every
// option but the owner is an object the shown rows regard, and the owner is
// offered whether or not it has events.
func TestEventsChoiceOffersOnlyObjectsWithEvents(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, path := range []string{eventsTabURL, "/instances/web/web?tab=events", "/instances/cert-manager/cert-manager?tab=events", "/packages/pkg/podinfo?tab=events"} {
		region := eventsRegion(s.get(t, path).body)
		sel := between(region, `<select id="f-resource"`, "</select>")
		linked := map[string]bool{}
		for _, m := range resourceRef.FindAllStringSubmatch(region, -1) {
			v, _ := url.QueryUnescape(html.UnescapeString(m[1]))
			linked[v] = true
		}
		owner := false
		for _, o := range optionTag.FindAllStringSubmatch(sel, -1) {
			val := html.UnescapeString(o[1])
			if val == "" {
				continue
			}
			if strings.HasPrefix(val, "opmodel.dev/Module") {
				owner = true
				continue
			}
			if !linked[val] {
				t.Errorf("%s: option %q is offered but no row regards it", path, val)
			}
		}
		if !owner {
			t.Errorf("%s: the owner is not among the options:\n%s", path, sel)
		}
	}
}

// TestEventsTypeFilter (5.1): type=Warning lists Warnings only, and the type
// options count the whole merged feed.
func TestEventsTypeFilter(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	all := eventsRegion(s.get(t, eventsTabURL).body)
	warn := eventsRegion(s.get(t, eventsTabURL+"&type=Warning").body)
	if strings.Contains(warn, "ev-normal") || !strings.Contains(warn, "ev-warning") {
		t.Errorf("type=Warning does not list Warnings only:\n%s", warn)
	}
	n := strings.Count(warn, `class="ev-row ev-warning"`)
	if !strings.Contains(all, "Warning ("+strconv.Itoa(n)+")") || !strings.Contains(warn, "Warning ("+strconv.Itoa(n)+")") {
		t.Errorf("the Warning option does not count the %d Warning lines of the whole feed", n)
	}
	if sh := showing.FindStringSubmatch(warn); len(sh) < 2 || sh[1] != strconv.Itoa(n) {
		t.Errorf("showing %v, want %d of the feed", sh, n)
	}
	if !strings.Contains(warn, `href="/instances/default/podinfo?tab=events"`) {
		t.Errorf("no Clear filters link back to the whole feed:\n%s", warn)
	}
}

// TestEventsResourceNarrowsInThePage (5.1): resource= the owner shows the owner's
// lines, resource= a Pod its own, each marked selected; an object the feed does not
// hold is ignored and named.
func TestEventsResourceNarrowsInThePage(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	owner := eventsRegion(s.get(t, eventsTabURL+"&resource="+url.QueryEscape(podinfoOwner)).body)
	if rows := resourceRef.FindAllStringSubmatch(between(owner, "<tbody>", "</tbody>"), -1); len(rows) == 0 {
		t.Fatal("no row for the owner")
	} else {
		for _, r := range rows {
			if v, _ := url.QueryUnescape(r[1]); v != podinfoOwner {
				t.Errorf("a row regards %q under resource=%s", v, podinfoOwner)
			}
		}
	}
	if !strings.Contains(owner, `<option value="`+podinfoOwner+`" selected>This instance (`) {
		t.Errorf("This instance is not selected:\n%s", owner)
	}
	pod := eventsRegion(s.get(t, eventsTabURL+"&resource=/Pod/default/podinfo-podinfo-d9585d794-4lg6h").body)
	if !strings.Contains(pod, "Ignored filter") && !strings.Contains(pod, "podinfo-podinfo-d9585d794-4lg6h") {
		t.Errorf("the Pod's lines are neither shown nor named as ignored:\n%s", pod)
	}
	if !strings.Contains(eventsRegion(s.get(t, eventsTabURL+"&resource=/Pod/default/nope").body), "Ignored filter") {
		t.Error("an unknown resource is not ignored")
	}
}

// TestEventsEmptyTextFollowsTheFilter (5.2).
func TestEventsEmptyTextFollowsTheFilter(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	if r := eventsRegion(s.get(t, eventsTabURL+"&reason=NoSuchReason").body); !strings.Contains(r, "No events match these filters.") || strings.Contains(r, "No events in the last hour") {
		t.Errorf("a reason with no match:\n%s", r)
	}
	if r := eventsRegion(s.get(t, "/instances/web/web?tab=events").body); strings.Contains(r, "No events match") {
		t.Errorf("an unfiltered empty feed reads as filtered:\n%s", r)
	}
}

// TestEventsPartialFeedSaysSo (5.1): a caller who may not list the Pods gets no
// Pod in the choice or the table, the note, and "All readable resources".
func TestEventsPartialFeedSaysSo(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.DenyResources("pods"))
	page := s.get(t, eventsTabURL).body
	region := eventsRegion(page)
	for _, want := range []string{"Some objects' events could not be read and are left out.", "All readable resources ("} {
		if !strings.Contains(region, want) {
			t.Errorf("the partial feed lacks %q:\n%s", want, region)
		}
	}
	if strings.Contains(region, `label="Pod"`) || strings.Contains(region, "/Pod/") {
		t.Errorf("a Pod is offered to a caller who may not list Pods:\n%s", region)
	}
	if regexp.MustCompile(`id="tab-n-events"[^>]*>\d`).MatchString(page) {
		t.Error("the Events tab counts a feed the caller could not read in full")
	}
}

// TestEventsTableShape (5.2): the table has the five columns with the Resource
// cell as a filter link that keeps the other parameters, and the panel's Events view
// has no Resource column.
func TestEventsTableShape(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	region := eventsRegion(s.get(t, eventsTabURL+"&type=Normal").body)
	head := between(region, "<thead>", "</thead>")
	for _, col := range []string{">Type<", ">Reason<", ">Resource<", ">Message<", ">Age<"} {
		if !strings.Contains(head, col) {
			t.Errorf("the table lacks the column %s", col)
		}
	}
	if !strings.Contains(region, `title="Show only this resource's events"`) || !strings.Contains(region, "type=Normal") || !strings.Contains(region, "?resource=") {
		t.Errorf("the resource cell is not a filter link that keeps the type:\n%s", region)
	}
	if !strings.Contains(region, "<time datetime=") || strings.Contains(between(region, "<tbody>", "</tbody>"), " ago") {
		t.Errorf("the age is not compact, with its time in a title:\n%s", region)
	}
	frag := s.get(t, "/instances/default/podinfo/events?kind=Pod&namespace=default&name=podinfo-podinfo-d9585d794-4lg6h", htmxRequest).body
	if !strings.Contains(frag, `class="table rows events-table"`) || strings.Contains(between(frag, "<thead>", "</thead>"), "Resource") {
		t.Errorf("the panel's Events view is not the table without the Resource column:\n%s", frag)
	}
}

// TestEventsTableShowsACappedNoteOnce (5.2, OP-A2): a note the API capped
// renders whole and once; the template clips nothing, and hostile text is text.
func TestEventsTableShowsACappedNoteOnce(t *testing.T) {
	long := strings.Repeat("x", 511) + "\u2026"
	evil := `<script>alert(1)</script>`
	out := renderPartial(t, "events-table", eventsTable{ShowResource: true, Rows: []eventRow{
		eventRowOf(v1.Event{Type: "Warning", Reason: "Failed", Note: long, Regarding: v1.ObjectRef{Kind: "Pod", Name: "p"}, Count: 1}, "/x?resource=a"),
		eventRowOf(v1.Event{Type: "Normal", Reason: evil, Note: evil, ReportingController: evil, Regarding: v1.ObjectRef{Kind: "Pod", Name: evil}, Count: 3}, ""),
	}})
	if strings.Count(out, long) != 1 {
		t.Errorf("the capped note shows %d times, want once", strings.Count(out, long))
	}
	if strings.Contains(out, "<script>") || strings.Count(out, "&lt;script&gt;") < 4 {
		t.Errorf("hostile event text is not escaped:\n%s", out)
	}
	if got := renderPartial(t, "events-table", eventsTable{Empty: noEventsMatch}); !strings.Contains(got, noEventsMatch) || strings.Contains(got, "<table") {
		t.Errorf("empty table:\n%s", got)
	}
}

// TestInClusterEventsTableHasNoOperatorNote (5.2, OP-A2): over the in-cluster
// read API the Events tab of the instance shows the note of neither writer of an
// operator note: an event from opm-controller, and one with no reporter that
// regards an OPM object. The local API serves both, built here from F1's events.
func TestInClusterEventsTableHasNoOperatorNote(t *testing.T) {
	const (
		feed        = "/api/v1alpha1/clusters/default/instances/default/podinfo/events?scope=all"
		anonymousOP = "A note with no reporter, about an OPM object."
	)
	objs := withAnonymousOPMEvent(t, apitest.F1(t), anonymousOP)
	local := newSite(t, objs, apitest.AllowAll)
	var doc v1.EventList
	if err := json.Unmarshal([]byte(local.get(t, feed).body), &doc); err != nil {
		t.Fatal(err)
	}
	var byController, byOPMObject string
	for _, e := range doc.Items {
		switch {
		case e.ReportingController == "opm-controller" && e.Note != "":
			byController = e.Note
		case e.ReportingController == "" && e.Regarding.Group == "opmodel.dev" && e.Note != "":
			byOPMObject = e.Note
		}
	}
	if byController == "" || byOPMObject == "" {
		t.Fatalf("the local API serves no event of both operator writers (controller %q, no reporter %q)", byController, byOPMObject)
	}
	if !strings.Contains(eventsRegion(local.get(t, eventsTabURL).body), html.EscapeString(byOPMObject)) {
		t.Fatal("the local Events tab does not show the operator notes: the in-cluster check proves nothing")
	}
	in := newInClusterSite(t, objs, apitest.AllowAll)
	region := eventsRegion(in.get(t, eventsTabURL).body)
	for _, note := range []string{byController, byOPMObject} {
		if strings.Contains(region, html.EscapeString(note)) {
			t.Errorf("the in-cluster Events tab shows the operator note %q", note)
		}
	}
	if !strings.Contains(region, "ScalingReplicaSet") {
		t.Errorf("the in-cluster tab lost the kubelet-side lines:\n%s", region)
	}
}

// withAnonymousOPMEvent adds to objs an event with no reporter that regards the
// podinfo ModuleInstance, built from the operator's own: the second writer of an
// operator note (OP-A2).
func withAnonymousOPMEvent(t *testing.T, objs []*unstructured.Unstructured, note string) []*unstructured.Unstructured {
	t.Helper()
	for _, o := range slices.Clone(objs) {
		about, _, _ := unstructured.NestedString(o.Object, "regarding", "name")
		if o.GetKind() == "Event" && o.GetNamespace() == "default" && about == "podinfo" && o.Object["reportingController"] == "opm-controller" {
			twin := o.DeepCopy()
			twin.SetName("anonymous-opm-event")
			twin.SetUID("00000000-0000-4000-8000-000000000001")
			delete(twin.Object, "reportingController")
			delete(twin.Object, "reportingInstance")
			twin.Object["note"] = note
			return append(objs, twin)
		}
	}
	t.Fatal("F1 holds no operator event of podinfo to copy")
	return nil
}

// --- Logs ---

func logsRegion(page string) string { return between(mainOf(page), `id="logs"`, "</section>") }

// TestLogsTabOpensOneOpenPane (5.3): one pane, open, for the first container of the
// first Pod when none is picked; the log parameter picks another; the select
// lists every container grouped by Pod.
func TestLogsTabOpensOneOpenPane(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	region := logsRegion(s.get(t, "/instances/default/podinfo?tab=logs").body)
	if n := strings.Count(region, `<details class="log"`); n != 1 || !strings.Contains(region, `data-log-topic="log:default/`) || !regexp.MustCompile(`<details class="log"[^>]*\sopen>`).MatchString(region) {
		t.Fatalf("want one open pane, got %d:\n%s", n, region)
	}
	sel := between(region, `<select id="f-log"`, "</select>")
	opts := optionTag.FindAllStringSubmatch(sel, -1)
	if len(opts) < 2 || !strings.Contains(sel, "<optgroup label=\"podinfo-podinfo-") || !strings.Contains(opts[0][0], "selected") {
		t.Fatalf("the select lists %d containers and does not select the first:\n%s", len(opts), sel)
	}
	last := html.UnescapeString(opts[len(opts)-1][1])
	region = logsRegion(s.get(t, "/instances/default/podinfo?tab=logs&log="+url.QueryEscape(last)).body)
	if !regexp.MustCompile(`<details class="log" id="`+regexp.QuoteMeta(last)+`"`).MatchString(region) || !strings.Contains(region, `value="`+last+`" selected`) {
		t.Errorf("log=%s does not open that pane:\n%s", last, region)
	}
	region = logsRegion(s.get(t, "/instances/default/podinfo?tab=logs&log=log_nope_nope").body)
	if n := strings.Count(region, `<details class="log"`); n != 1 {
		t.Errorf("an unknown log= leaves %d panes, want the first one", n)
	}
	if !strings.Contains(region, `name="log"`) || !strings.Contains(region, `data-filters="owner-logs"`) || !strings.Contains(region, ">Show</button>") {
		t.Errorf("the select form lacks its name, key or Show button:\n%s", region)
	}
}

// TestLogsHintPerReason (5.3): the hint appears only for a container that has
// not started, with the link to the Pod's events, and CrashLoopBackOff reads as
// restarting and never as not started.
func TestLogsHintPerReason(t *testing.T) {
	v := &ownerView{Base: "/instances/default/podinfo"}
	for reason, want := range map[string]string{
		"ContainerCreating": hintNotStarted, "ErrImagePull": hintNotStarted, "ImagePullBackOff": hintNotStarted,
		"InvalidImageName": hintNotStarted, "CreateContainerConfigError": hintNotStarted, "Pending": hintNotStarted,
		"CrashLoopBackOff": hintRestarting, "Current": "", "InProgress": "", "": "",
	} {
		h := hintOf(&logPane{PodValue: "/Pod/default/p", Reason: reason}, v)
		if h.Text != want {
			t.Errorf("reason %q: hint %q, want %q", reason, h.Text, want)
		}
		if want != "" && h.Href != "/instances/default/podinfo?resource=%2FPod%2Fdefault%2Fp&tab=events" {
			t.Errorf("reason %q: link %q", reason, h.Href)
		}
	}
	broken := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	waiting := logsRegion(broken.get(t, "/instances/default/podinfo?tab=logs&log=log_podinfo-podinfo-794bd8c7fb-cc5ql_podinfo").body)
	if !strings.Contains(waiting, hintNotStarted) || !strings.Contains(waiting, `href="/instances/default/podinfo?resource=%2FPod%2Fdefault%2Fpodinfo-podinfo-794bd8c7fb-cc5ql&amp;tab=events"`) {
		t.Errorf("the ImagePullBackOff Pod has no hint with its Events link:\n%s", waiting)
	}
	running := logsRegion(broken.get(t, "/instances/default/podinfo?tab=logs&log=log_podinfo-podinfo-d9585d794-5m9tx_podinfo").body)
	if strings.Contains(running, "log-hint") {
		t.Errorf("a running container gets a hint:\n%s", running)
	}
}

// --- YAML ---

func yamlRegion(page string) string { return between(mainOf(page), `id="yaml"`, "</section>") }

// TestYAMLTabDefaults (5.4): with neither node nor focus the tab opens on the
// first readable object and shows its YAML; the choice is grouped by kind.
func TestYAMLTabDefaults(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	region := yamlRegion(s.get(t, "/instances/default/podinfo?tab=yaml").body)
	opts := optionTag.FindAllStringSubmatch(between(region, `<select id="f-object"`, "</select>"), -1)
	if len(opts) == 0 || !strings.Contains(opts[0][0], "selected") || !strings.Contains(region, "kind: ") {
		t.Fatalf("the tab does not open on its first object (%d options):\n%s", len(opts), region)
	}
	if !strings.Contains(region, "<optgroup label=\"Deployment\">") || !strings.Contains(region, `data-filters="owner-yaml"`) || !strings.Contains(region, "Secrets and values are never shown.") {
		t.Errorf("the form lacks its kind groups, key or Secret line:\n%s", region)
	}
}

// TestASecretCannotBePicked (5.4, portal:D8:R1): a URL that names a Secret gets
// the default, and a Secret is a disabled option.
func TestASecretCannotBePicked(t *testing.T) {
	secret := v1.ObjectRef{Version: "v1", Kind: "Secret", Namespace: "default", Name: "db"}
	svc := v1.ObjectRef{Version: "v1", Kind: "Service", Namespace: "default", Name: "web"}
	v := ownerView{Kind: instanceKind, Base: "/instances/default/app", Components: (&Handler{}).components("/instances/default/app", []v1.Component{{
		Name: "db", Objects: []v1.InventoryObject{{Ref: secret, Access: v1.AccessOK}, {Ref: svc, Access: v1.AccessOK}},
	}})}
	entries := yamlEntries(&v)
	if got := yamlPick(resourceValue(secret), &v, &v1.Graph{}, entries); got != resourceValue(svc) {
		t.Errorf("object=Secret picks %q, want the Service", got)
	}
	if got := yamlPick("", &ownerView{}, &v1.Graph{}, []yamlEntry{{ref: secret, secret: true}}); got != "" {
		t.Errorf("a Secret alone is picked as %q", got)
	}
}

// TestEventsRegionFollowsOnlyTheOwnersFeed (5.1): the stream's events topic is the
// owner's own feed, so an event about a child alone sends no refresh. The tab's
// region and the page follow that one events topic and no child's; a child's
// event shows at the next render (design.md, "Scale and refresh").
func TestEventsRegionFollowsOnlyTheOwnersFeed(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	page := s.get(t, eventsTabURL).body
	const own = "events:instance:default/podinfo"
	if m := regexp.MustCompile(`id="events"[^>]*data-follow="([^"]*)"`).FindStringSubmatch(page); len(m) < 2 || m[1] != own {
		t.Errorf("the Events region follows %v, want %q alone", m, own)
	}
	for _, m := range regexp.MustCompile(`events:[^\s"]+`).FindAllString(page, -1) {
		if m != own {
			t.Errorf("the page names the events topic %q, which is not the owner's", m)
		}
	}
}
