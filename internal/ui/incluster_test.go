package ui

import (
	"encoding/json"
	"html"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// inClusterPages are the pages and fragments that show operator text in
// local mode, rendered over an in-cluster read API.
var inClusterPages = []goldenPage{
	{"in-cluster/platform", "/", false},
	{"in-cluster/instance-podinfo", "/instances/default/podinfo", false},
	{"in-cluster/package-podinfo", "/packages/pkg/podinfo", false},
	{"in-cluster/fragment-events-pod", "/instances/default/podinfo/events?kind=Pod&namespace=default&name=podinfo-podinfo-d9585d794-4lg6h", true},
}

func TestGoldenPagesInCluster(t *testing.T) {
	s := newInClusterSite(t, apitest.F1(t), apitest.AllowAll)
	checkGoldens(t, s, inClusterPages)
}

func TestGoldenBrokenRolloutInCluster(t *testing.T) {
	s := newInClusterSite(t, apitest.F1Broken(t), apitest.AllowAll)
	checkGoldens(t, s, []goldenPage{{"in-cluster/instance-podinfo-broken", "/instances/default/podinfo", false}})
}

// TestNoOperatorTextOnAnInClusterPage collects every operator message and
// event note the local read API serves for F1 and the broken sample, and
// fails when one of them appears on the matching in-cluster page.
func TestNoOperatorTextOnAnInClusterPage(t *testing.T) {
	const apiBase = "/api/v1alpha1/clusters/default"
	sources := map[string][]string{
		"/": {apiBase + "/platform", apiBase + "/platform/events", apiBase + "/platform/registrations/default.refused-claim-fixture/events",
			apiBase + "/platform/registrations/default.backup-provider/events"},
		"/?tab=catalogs":                                  {apiBase + "/platform"},
		"/instances/default/podinfo?tab=events":           {apiBase + "/instances/default/podinfo", apiBase + "/instances/default/podinfo/events"},
		"/instances/default/backup-provider?tab=events":   {apiBase + "/instances/default/backup-provider", apiBase + "/instances/default/backup-provider/events"},
		"/packages/pkg/podinfo?tab=events":                {apiBase + "/packages/pkg/podinfo", apiBase + "/packages/pkg/podinfo/events"},
		"/instances/default/backup-provider?tab=provider": {apiBase + "/platform", apiBase + "/instances/default/backup-provider"},
		"/catalog?path=testing.opmodel.dev/catalogs/operator/refused-claim-fixture-absent@v0&tab=events": {apiBase + "/platform",
			apiBase + "/platform/events", apiBase + "/platform/registrations/default.refused-claim-fixture/events"},
		"/instances/default/podinfo":         {apiBase + "/instances/default/podinfo", apiBase + "/instances/default/podinfo/events"},
		"/instances/default/backup-provider": {apiBase + "/instances/default/backup-provider", apiBase + "/instances/default/backup-provider/events"},
		"/packages/pkg/podinfo":              {apiBase + "/packages/pkg/podinfo", apiBase + "/packages/pkg/podinfo/events"},
	}
	for name, objs := range map[string]func(testing.TB) []*unstructured.Unstructured{"F1": apitest.F1, "broken": apitest.F1Broken} {
		t.Run(name, func(t *testing.T) {
			local := newSite(t, objs(t), apitest.AllowAll)
			in := newInClusterSite(t, objs(t), apitest.AllowAll)
			checked := 0
			for page, docs := range sources {
				var texts []string
				for _, d := range docs {
					texts = append(texts, operatorTexts(t, local.get(t, d).body)...)
				}
				body := in.get(t, page).body
				for _, text := range texts {
					checked++
					if strings.Contains(body, html.EscapeString(text)) {
						t.Errorf("in-cluster %s shows operator text %q", page, text)
					}
				}
			}
			if checked == 0 {
				t.Fatal("the local API served no operator text: the check proves nothing")
			}
		})
	}
}

// operatorTexts returns the operator-written strings of a read API
// document: condition, reconcile, history and registration messages, and
// the notes of events the operator reported. A workload's health message
// and another controller's event note are not the operator's (supervisor
// ruling on the scope of portal:D8:R5).
func operatorTexts(t *testing.T, body string) []string {
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
				operator := (k == "note" && operatorEvent(x)) || k == "activeMessage" ||
					(k == "message" && parentKey != "health" && parentKey != "")
				if isText && s != "" && operator {
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

// operatorEvent reports whether an event the read API served was reported
// by the operator, or names no reporter and regards an OPM object.
func operatorEvent(ev map[string]any) bool {
	switch ev["reportingController"] {
	case "opm-controller":
		return true
	case nil, "":
		ref, _ := ev["regarding"].(map[string]any)
		return ref["group"] == "opmodel.dev"
	}
	return false
}
