package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// TestGraphKeepsTheAxesApart (portal:D3:R1): on the image-break sample the
// instance node's outline says Degraded and its stamp says Applied; no
// node draws an applied state through its outline class.
func TestGraphKeepsTheAxesApart(t *testing.T) {
	s := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	main := mainOf(s.get(t, "/instances/default/podinfo").body)
	node := regexp.MustCompile(`<a class="node kind-instance ([^"]*)"[^>]*>(?s:.*?)</a>`).FindStringSubmatch(main)
	if node == nil {
		t.Fatal("no instance node")
	}
	if node[1] != "health-degraded ap-applied" || !strings.Contains(node[0], `class="node-stamp"`) {
		t.Errorf("instance node classes %q, stamp present %v", node[1], strings.Contains(node[0], "node-stamp"))
	}
	if strings.Contains(main, `class="node kind-`) && regexp.MustCompile(`class="node [^"]*applied-`).MatchString(main) {
		t.Error("a node carries an applied-* class, which the badge styles paint")
	}
}

// TestConfigComponentsFoldAsInTheGraph (portal:D4:R5): cert-manager's
// configuration components the graph folds (its CRDs, RBAC and webhook
// groups, 16 of 17: the lone Namespace stays a component) are one closed
// group in the list for each family group of the graph, and the selected
// node is marked.
func TestConfigComponentsFoldAsInTheGraph(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	main := mainOf(s.get(t, "/instances/cert-manager/cert-manager?tab=resources").body)
	for _, want := range []string{
		`<details id="config-components">`, "CRDs · 1 component",
		`<details id="config-components-2">`, "RBAC · 13 components",
		`<details id="config-components-3">`, "Webhook configuration · 2 components",
	} {
		if !strings.Contains(main, want) {
			t.Errorf("the family groups of the graph are not three closed groups in the list: %q is missing", want)
		}
	}
	comps := between(main, `id="components"`, `id="config-components"`)
	if strings.Contains(comps, `<h3 class="component-name">crds</h3>`) {
		t.Error("a configuration component is listed outside the group")
	}
	graph := mainOf(s.get(t, "/instances/cert-manager/cert-manager?tab=graph&focus=mi%3Acert-manager%2Fcert-manager").body)
	if !regexp.MustCompile(`data-node="mi:cert-manager/cert-manager"[^>]*aria-current="true"`).MatchString(graph) {
		t.Error("the selected node is not marked")
	}
}

// TestConfigFoldsNameTheirOwnFamily: each configuration fold of the Resources
// tab says what its own family holds, not one sentence for all of them.
func TestConfigFoldsNameTheirOwnFamily(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	main := mainOf(s.get(t, "/instances/cert-manager/cert-manager?tab=resources").body)
	notes := map[string]string{
		`<details id="config-components">`:   "Custom resource definitions, which add kinds to the cluster",
		`<details id="config-components-2">`: "Roles, bindings and other access rules, with no workload",
		`<details id="config-components-3">`: "Webhook configurations, which route API requests to a service",
	}
	for id, want := range notes {
		head := between(main, id, "</summary>")
		if !strings.Contains(head, want) {
			t.Errorf("the fold %s does not name its family: want %q in %q", id, want, head)
		}
		for _, other := range notes {
			if other != want && strings.Contains(head, other) {
				t.Errorf("the fold %s carries another family's note %q", id, other)
			}
		}
	}
}
