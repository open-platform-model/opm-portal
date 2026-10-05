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
// configuration components are one closed group in the list, and the
// selected node is marked.
func TestConfigComponentsFoldAsInTheGraph(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	main := mainOf(s.get(t, "/instances/cert-manager/cert-manager?tab=resources").body)
	if !strings.Contains(main, `<details id="config-components">`) || !strings.Contains(main, "17 configuration components") {
		t.Error("the configuration components are not one closed group of 17")
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
