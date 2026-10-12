package ui

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// Tests of the Service panel's Endpoints fact (show-service-endpoints, S16).

const podinfo = "/instances/default/podinfo"

// factLabels lists the labels of the panel's facts in page order.
func factLabels(panel string) []string {
	found := regexp.MustCompile(`<dt>([^<]*)</dt>`).FindAllStringSubmatch(panel, -1)
	out := make([]string, 0, len(found))
	for _, m := range found {
		out = append(out, m[1])
	}
	return out
}

// noEndpointsWords fails when text names the kind of the slices or says what is
// denied, which the panel must not say about the read.
func noEndpointsWords(t *testing.T, what, text string) {
	t.Helper()
	for _, word := range []string{"endpointslice", "endpoint slice", "forbidden", "denied", "permission"} {
		if strings.Contains(strings.ToLower(text), word) {
			t.Errorf("%s names %q:\n%s", what, word, text)
		}
	}
}

// TestPanelEndpointsFact: the Endpoints fact sits right after the Service's
// Ports, reads "N ready" in plain text, and carries no tone and no monospace.
func TestPanelEndpointsFact(t *testing.T) {
	s := newSite(t, apitest.F1WithEndpointSlices(t), apitest.AllowAll)
	panel := panelOf(t, s, podinfo, nodeService)
	labels := factLabels(panel)
	var facts []string
	for _, l := range labels {
		switch l {
		case "Type", "Ports", "Endpoints":
			facts = append(facts, l)
		}
	}
	if strings.Join(facts, ",") != "Type,Ports,Endpoints" {
		t.Errorf("the Service's fact labels %v, want Type, Ports, Endpoints in that order", labels)
	}
	if !strings.Contains(panel, "<dt>Endpoints</dt><dd>2 ready</dd>") {
		t.Errorf("the Endpoints fact is not plain text with no class:\n%s", between(panel, `<dl class="facts">`, "</dl>"))
	}
	// The Service's panel inside the page, selected by node, shows it too.
	page := mainOf(s.get(t, podinfo+"?tab=graph&node="+url.QueryEscape(nodeService)).body)
	if !strings.Contains(between(page, `id="detail"`, "</aside>"), "<dt>Endpoints</dt><dd>2 ready</dd>") {
		t.Errorf("the page's panel lacks the Endpoints fact:\n%s", between(page, `id="detail"`, "</aside>"))
	}
}

// TestPanelEndpointsZero: a Service whose endpoints are all not ready reads
// "0 ready", in the same words, never "Could not be read".
func TestPanelEndpointsZero(t *testing.T) {
	s := newSite(t, apitest.F1WithEndpointSlices(t, apitest.WithNoReadyEndpoints()), apitest.AllowAll)
	panel := panelOf(t, s, podinfo, nodeService)
	if !strings.Contains(panel, "<dt>Endpoints</dt><dd>0 ready</dd>") || strings.Contains(panel, "Could not be read") {
		t.Errorf("the all-not-ready Service:\n%s", between(panel, `<dl class="facts">`, "</dl>"))
	}
}

// TestPanelEndpointsUnread: a read that was not made or failed reads "Could
// not be read" beside the Service's other facts, never "0 ready", and the page
// does not name slices or a denial. The caller's own refusal and the portal
// reader's refusal read the same.
func TestPanelEndpointsUnread(t *testing.T) {
	for name, s := range map[string]*site{
		"the caller may not list slices": newSite(t, apitest.F1WithEndpointSlices(t), apitest.DenyResources("endpointslices")),
		"the reader may not list slices": mount(t, apitest.NewWithReaderRule(t, apitest.F1WithEndpointSlices(t), apitest.AllowAll, apitest.DenyResources("endpointslices"))),
	} {
		t.Run(name, func(t *testing.T) {
			panel := panelOf(t, s, podinfo, nodeService)
			got := factsOfPanel(panel)
			if got["Endpoints"] != "Could not be read" || got["Type"] != "ClusterIP" || got["Ports"] == "" {
				t.Errorf("facts %v, want the Service's Type and Ports and Endpoints = Could not be read", got)
			}
			if strings.Contains(panel, "0 ready") || strings.Contains(panel, "<dt>Endpoints</dt><dd class=") {
				t.Errorf("the unread form shows a count or a class:\n%s", between(panel, `<dl class="facts">`, "</dl>"))
			}
			noEndpointsWords(t, "the panel", panel)
			page := s.get(t, podinfo+"?tab=graph&node="+url.QueryEscape(nodeService)).body
			noEndpointsWords(t, "the page", page)
		})
	}
}

// TestPanelEndpointsAbsent: a Service whose cluster records no slice shows no
// Endpoints line.
func TestPanelEndpointsAbsent(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	panel := panelOf(t, s, podinfo, nodeService)
	if strings.Contains(panel, "Endpoints") {
		t.Errorf("a Service with no recorded slice shows an Endpoints line:\n%s", between(panel, `<dl class="facts">`, "</dl>"))
	}
}

// TestPanelEndpointsOfAnUnreadableNode: a Service the caller may not read has
// no fact and the panel makes no request for its object; a Service whose
// object document cannot be read drops the Endpoints fact with the others.
func TestPanelEndpointsOfAnUnreadableNode(t *testing.T) {
	forbidden, c := countingSite(t, apitest.F1WithEndpointSlices(t), apitest.DenyResources("services"), false)
	panel := panelOf(t, forbidden, podinfo, nodeService)
	if n := c.objects.Load(); n != 0 {
		t.Errorf("the panel of a forbidden Service made %d object requests", n)
	}
	if strings.Contains(panel, "Endpoints") || !strings.Contains(panel, "locked-panel") {
		t.Errorf("a forbidden Service's panel:\n%s", panel)
	}

	failing, _ := countingSite(t, apitest.F1WithEndpointSlices(t), apitest.AllowAll, true)
	res := failing.get(t, podinfo+"/node?id="+url.QueryEscape(nodeService), htmxRequest)
	if !strings.Contains(res.body, "Details from the object could not be read.") || strings.Contains(res.body, "Endpoints") {
		t.Errorf("a Service whose object cannot be read:\n%s", res.body)
	}
}

// TestPanelEndpointsLeakNothingOfTheSlices: the slices carry sentinel strings
// in every address, hostname, node name, zone, Pod name, port name and label
// value. The page and the panel show the count and none of them.
func TestPanelEndpointsLeakNothingOfTheSlices(t *testing.T) {
	sent := apitest.Sentinels{
		Address: "sentinel-address", Hostname: "sentinel-hostname", NodeName: "sentinel-node",
		Zone: "sentinel-zone", PodName: "sentinel-pod", PortName: "sentinel-port", LabelValue: "sentinel-label",
	}
	s := newSite(t, apitest.F1WithEndpointSlices(t, apitest.WithSentinels(sent)), apitest.AllowAll)
	panel := panelOf(t, s, podinfo, nodeService)
	if factsOfPanel(panel)["Endpoints"] != "2 ready" {
		t.Fatalf("the count is not shown, so the leak check proves nothing:\n%s", between(panel, `<dl class="facts">`, "</dl>"))
	}
	pages := map[string]string{
		"the panel": panel,
		"the graph": s.get(t, podinfo+"?tab=graph&node="+url.QueryEscape(nodeService)).body,
		"resources": s.get(t, podinfo+"?tab=resources&node="+url.QueryEscape(nodeService)).body,
		"events":    s.get(t, podinfo+"?tab=events").body,
		"yaml":      s.get(t, podinfo+"?tab=yaml&object=_/Service/default/podinfo-podinfo").body,
	}
	for name, body := range pages {
		for _, marker := range sent.All() {
			if strings.Contains(body, marker) {
				t.Errorf("%s holds the sentinel %q", name, marker)
			}
		}
	}
}

// TestPanelIsPolicyCleanWithEndpoints: the fact adds no inline style, handler,
// script or style element, in either form.
func TestPanelIsPolicyCleanWithEndpoints(t *testing.T) {
	for name, s := range map[string]*site{
		"counted": newSite(t, apitest.F1WithEndpointSlices(t), apitest.AllowAll),
		"unread":  newSite(t, apitest.F1WithEndpointSlices(t), apitest.DenyResources("endpointslices")),
	} {
		body := panelOf(t, s, podinfo, nodeService)
		for _, bad := range []string{` style=`, ` onclick=`, `<script`, `<style`} {
			if strings.Contains(body, bad) {
				t.Errorf("%s: the panel carries %q", name, bad)
			}
		}
	}
}

// TestEndpointFacts: the fact is a function of a Service node alone. A count of
// 0 reads "0 ready"; the unread mark never reads as a count; a node that is not
// a Service, or carries neither field, has no fact.
func TestEndpointFacts(t *testing.T) {
	n := func(kind string, ready int64, setReady, unread bool) *v1.GraphNode {
		node := &v1.GraphNode{Ref: &v1.ObjectRef{Kind: kind, Name: "x"}, EndpointsUnread: unread}
		if setReady {
			node.EndpointsReady = &ready
		}
		return node
	}
	for _, tc := range []struct {
		name string
		node *v1.GraphNode
		want []fact
	}{
		{"a count", n("Service", 2, true, false), []fact{{Label: "Endpoints", Value: "2 ready"}}},
		{"a count of zero", n("Service", 0, true, false), []fact{{Label: "Endpoints", Value: "0 ready"}}},
		{"one endpoint", n("Service", 1, true, false), []fact{{Label: "Endpoints", Value: "1 ready"}}},
		{"unread", n("Service", 0, false, true), []fact{{Label: "Endpoints", Value: "Could not be read"}}},
		{"neither", n("Service", 0, false, false), nil},
		{"a Pod that carries a count", n("Pod", 2, true, false), nil},
		{"a Pod that carries the mark", n("Pod", 0, false, true), nil},
		{"no reference", &v1.GraphNode{}, nil},
	} {
		got := endpointFacts(tc.node)
		if len(got) != len(tc.want) || (len(got) == 1 && got[0] != tc.want[0]) {
			t.Errorf("%s: endpointFacts = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}
