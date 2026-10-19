package ui

import (
	"encoding/json"
	"html"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// inClusterPages are the pages and fragments that show controller text in
// local mode, rendered over an in-cluster read API.
var inClusterPages = []goldenPage{
	{"in-cluster/platform", "/", false},
	{"in-cluster/catalog-refused-claim-events", "/catalog?path=testing.opmodel.dev/catalogs/operator/refused-claim-fixture-absent@v0&tab=events", false},
	{"in-cluster/instance-podinfo", "/instances/default/podinfo", false},
	{"in-cluster/package-podinfo", "/packages/pkg/podinfo", false},
	{"in-cluster/fragment-events-pod", "/instances/default/podinfo/events?kind=Pod&namespace=default&name=podinfo-podinfo-d9585d794-4lg6h", true},
}

func TestGoldenPagesInCluster(t *testing.T) {
	s := newInClusterSite(t, apitest.F1(t), apitest.AllowAll)
	checkGoldens(t, s, inClusterPages)
}

func TestGoldenPackageHolderInCluster(t *testing.T) {
	s := newInClusterSite(t, apitest.F1PackageHolder(t), apitest.AllowAll)
	checkGoldens(t, s, []goldenPage{{"in-cluster/package-provider-provider", "/packages/pkg/provider?tab=provider", false}})
}

func TestGoldenBrokenRolloutInCluster(t *testing.T) {
	s := newInClusterSite(t, apitest.F1Broken(t), apitest.AllowAll)
	checkGoldens(t, s, []goldenPage{{"in-cluster/instance-podinfo-broken", "/instances/default/podinfo", false}})
}

// withControllerEventsOnTheRegistration adds two events about the
// TransformerRegistration default.backup-provider, an OPM object that
// backup-provider's inventory reaches and that is not the owner: one the
// controller reports and one that names no reporter. The merged feed that the
// owner pages render carries both, so the walk below hunts for their notes.
func withControllerEventsOnTheRegistration(objs func(testing.TB) []*unstructured.Unstructured) func(testing.TB) []*unstructured.Unstructured {
	return func(t testing.TB) []*unstructured.Unstructured {
		out := objs(t)
		for _, ev := range []struct{ name, reason, note, controller string }{
			{"registration-by-the-controller", "RegistrationControllerSaid", "registration text one", "opm-controller"},
			{"registration-by-nobody-we-know", "RegistrationSomebodySaid", "registration text two", ""},
		} {
			e := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "events.k8s.io/v1",
				"kind":       "Event",
				"metadata": map[string]any{
					"name": ev.name, "namespace": "default", "uid": ev.name + "-uid", "creationTimestamp": "2026-10-04T18:06:00Z",
				},
				"eventTime": "2026-10-04T18:06:00.000000Z",
				"reason":    ev.reason,
				"note":      ev.note,
				"type":      "Normal",
				"regarding": map[string]any{
					"apiVersion": "opmodel.dev/v1alpha1", "kind": "TransformerRegistration", "name": "default.backup-provider",
				},
			}}
			if ev.controller != "" {
				e.Object["reportingController"] = ev.controller
			}
			out = append(out, e)
		}
		return out
	}
}

// TestNoControllerTextOnAnInClusterPage collects every controller message and
// event note the local read API serves for F1 and the broken sample, the
// merged feed (scope=all) that the owner pages render included, and fails
// when one of them appears on the matching in-cluster page. F1 gains two
// events on a reached object other than the owner, so the walk covers an
// controller note that is not about the owner (portal:D8:R5).
func TestNoControllerTextOnAnInClusterPage(t *testing.T) {
	const apiBase = "/api/v1alpha1/clusters/default"
	sources := map[string][]string{
		"/": {apiBase + "/platform", apiBase + "/platform/events", apiBase + "/platform/registrations/default.refused-claim-fixture/events",
			apiBase + "/platform/registrations/default.backup-provider/events"},
		"/?tab=catalogs":                                {apiBase + "/platform"},
		"/instances/default/podinfo?tab=events":         {apiBase + "/instances/default/podinfo", apiBase + "/instances/default/podinfo/events?scope=all"},
		"/instances/default/backup-provider?tab=events": {apiBase + "/instances/default/backup-provider", apiBase + "/instances/default/backup-provider/events?scope=all"},
		"/packages/pkg/podinfo?tab=events":              {apiBase + "/packages/pkg/podinfo", apiBase + "/packages/pkg/podinfo/events?scope=all"},
		"/catalog?path=testing.opmodel.dev/catalogs/operator/refused-claim-fixture-absent@v0&tab=claims": {apiBase + "/platform"},
		"/instances/default/backup-provider?tab=provider":                                                {apiBase + "/platform", apiBase + "/instances/default/backup-provider"},
		"/catalog?path=testing.opmodel.dev/catalogs/operator/refused-claim-fixture-absent@v0&tab=events": {apiBase + "/platform",
			apiBase + "/platform/events", apiBase + "/platform/registrations/default.refused-claim-fixture/events"},
		"/instances/default/podinfo":         {apiBase + "/instances/default/podinfo", apiBase + "/instances/default/podinfo/events?scope=all"},
		"/instances/default/backup-provider": {apiBase + "/instances/default/backup-provider", apiBase + "/instances/default/backup-provider/events?scope=all"},
		"/packages/pkg/podinfo":              {apiBase + "/packages/pkg/podinfo", apiBase + "/packages/pkg/podinfo/events?scope=all"},
	}
	for name, objs := range map[string]func(testing.TB) []*unstructured.Unstructured{
		"F1": withControllerEventsOnTheRegistration(apitest.F1), "broken": withControllerEventsOnTheRegistration(apitest.F1Broken)} {
		t.Run(name, func(t *testing.T) {
			local := newSite(t, objs(t), apitest.AllowAll)
			in := newInClusterSite(t, objs(t), apitest.AllowAll)
			checked := 0
			for page, docs := range sources {
				var texts []string
				for _, d := range docs {
					texts = append(texts, controllerTexts(t, local.get(t, d).body)...)
				}
				body := in.get(t, page).body
				for _, text := range texts {
					checked++
					if strings.Contains(body, html.EscapeString(text)) {
						t.Errorf("in-cluster %s shows controller text %q", page, text)
					}
				}
			}
			if checked == 0 {
				t.Fatal("the local API served no controller text: the check proves nothing")
			}
		})
	}
}

// controllerTexts returns the controller-written strings of a read API
// document: condition, reconcile, history and registration messages, and
// the notes of events the controller reported. A workload's health message
// and another controller's event note are not the controller's (supervisor
// ruling on the scope of portal:D8:R5).
func controllerTexts(t *testing.T, body string) []string {
	t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("not a JSON document: %v", err)
	}
	var out []string
	var walk func(parentKey string, v any)
	walk = func(parentKey string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				s, isText := child.(string)
				controller := (k == "note" && controllerEvent(x)) || k == "activeMessage" ||
					(k == "message" && parentKey != "health" && parentKey != "")
				if isText && s != "" && controller {
					out = append(out, s)
				}
				walk(k, child)
			}
		case []any:
			for _, child := range x {
				walk(parentKey, child)
			}
		}
	}
	walk("", doc)
	return out
}

// controllerEvent reports whether an event the read API served was reported
// by the controller, or names no reporter and regards an OPM object.
func controllerEvent(ev map[string]any) bool {
	switch ev["reportingController"] {
	case "opm-controller":
		return true
	case nil, "":
		ref, _ := ev["regarding"].(map[string]any)
		return ref["group"] == "opmodel.dev"
	}
	return false
}

// TestInClusterCardsShowReasonsAndMeaningsOnly (align-owner-pages, 3.5): over
// the in-cluster read API the package's Applied block keeps the state word,
// the summary composed from recorded fields, the reason's meaning and the
// attempt dots, and shows none of the controller's message, which the read
// API omits; the Provider block of a refused claim shows the reason and its
// meaning, and no controller text.
func TestInClusterCardsShowReasonsAndMeaningsOnly(t *testing.T) {
	s := newInClusterSite(t, apitest.F1(t), apitest.AllowAll)
	applied := between(mainOf(s.get(t, "/packages/pkg/podinfo").body), `id="applied-card"`, "</section>")
	for _, want := range []string{`<span class="sb-word">Failed, retrying</span>`, "Failed on the last 3 attempts, retrying every 1 min",
		"The package&#39;s source is not ready or does not exist.", "Reconciling: Progressing", "last 3 attempts"} {
		if !strings.Contains(applied, want) {
			t.Errorf("in-cluster Applied block lacks %q:\n%s", want, applied)
		}
	}
	for _, text := range []string{"no matches for kind", `class="sb-note says"`} {
		if strings.Contains(applied, text) {
			t.Errorf("in-cluster Applied block shows the controller's message (%q):\n%s", text, applied)
		}
	}
	held := newInClusterSite(t, apitest.F1PackageHolder(t), apitest.AllowAll)
	provider := between(mainOf(held.get(t, "/packages/pkg/provider").body), `id="provider-card"`, "</section>")
	if !strings.Contains(provider, `<span class="sb-word">Refused</span>`) || !strings.Contains(provider, "ProviderMismatch") ||
		strings.Contains(provider, "which does not exist") {
		t.Errorf("in-cluster Provider block:\n%s", provider)
	}
}

// TestInClusterPlatformPageShowsReasonsOnly (platform-21, platform-23): in
// cluster mode the Platform page keeps every reason and no controller message:
// the events table carries no controller note (the reporter's name stays), the contracts note has no
// "Controller message" fold, and a provider row shows its reason with no
// message after it. The local site proves the same pieces exist there.
func TestInClusterPlatformPageShowsReasonsOnly(t *testing.T) {
	local := newSite(t, apitest.F1(t), apitest.AllowAll)
	in := newInClusterSite(t, apitest.F1(t), apitest.AllowAll)

	cats := between(mainOf(in.get(t, "/?tab=catalogs").body), `id="contracts"`, "</div>")
	if cats == "" || strings.Contains(cats, "<details") || strings.Contains(cats, "Controller message") {
		t.Errorf("the in-cluster contracts note is missing or carries the controller message:\n%s", cats)
	}
	if !strings.Contains(between(mainOf(local.get(t, "/?tab=catalogs").body), `id="contracts"`, "</div>"), "<details") {
		t.Error("the local contracts note has no fold, so the in-cluster check proves nothing")
	}

	main := mainOf(in.get(t, "/").body)
	events := between(main, `<table class="table rows events-table">`, "</table>")
	if !strings.Contains(events, "CatalogUnresolved") || strings.Contains(events, `class="ev-note"`) {
		t.Errorf("the in-cluster events table shows a controller note:\n%s", events)
	}
	if !strings.Contains(between(mainOf(local.get(t, "/").body), `<table class="table rows events-table">`, "</table>"), `class="ev-note"`) {
		t.Error("the local events table has no note, so the in-cluster check proves nothing")
	}
	providers := between(main, `<div id="providers"`, "</table>")
	if !strings.Contains(providers, `<span class="mono">CatalogUnresolved</span>`) || strings.Contains(providers, `class="msg"`) {
		t.Errorf("the in-cluster provider rows show a message, or lose the reason:\n%s", providers)
	}
	status := between(main, `id="platform-status"`, "</section>")
	if !strings.Contains(status, "CatalogUnresolved") || strings.Contains(status, "Generated") {
		t.Errorf("the in-cluster status block does not read from reasons:\n%s", status)
	}
}

// TestInClusterCatalogEventsShowReasonsOnly: the Catalog page's events table
// keeps the reason and drops the controller's note in-cluster, and the
// resolved block and claim rows show no controller message (portal:D8:R5).
func TestInClusterCatalogEventsShowReasonsOnly(t *testing.T) {
	const path = "/catalog?path=testing.opmodel.dev/catalogs/operator/refused-claim-fixture-absent@v0"
	local := newSite(t, apitest.F1(t), apitest.AllowAll)
	in := newInClusterSite(t, apitest.F1(t), apitest.AllowAll)
	events := between(mainOf(in.get(t, path+"&tab=events").body), `<table class="table rows events-table">`, "</table>")
	if !strings.Contains(events, "CatalogUnresolved") || strings.Contains(events, `class="ev-note"`) {
		t.Errorf("the in-cluster Catalog events show a controller note, or lose the reason:\n%s", events)
	}
	if !strings.Contains(between(mainOf(local.get(t, path+"&tab=events").body), `<table class="table rows events-table">`, "</table>"), `class="ev-note"`) {
		t.Error("the local Catalog events have no note, so the in-cluster check proves nothing")
	}
	claims := between(mainOf(in.get(t, path).body), `id="claims"`, "</section>")
	if !strings.Contains(claims, `<span class="mono">CatalogUnresolved</span>`) || strings.Contains(claims, `class="msg"`) {
		t.Errorf("the in-cluster claim row shows a message, or loses the reason:\n%s", claims)
	}
}
