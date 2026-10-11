package ui

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// Tests of the details panel (design.md decision 5, OP-A3).

const (
	nodeDeployment = "obj:apps/Deployment/default/podinfo-podinfo"
	nodePod        = "obj:_/Pod/default/podinfo-podinfo-d9585d794-4lg6h"
	nodeService    = "obj:_/Service/default/podinfo-podinfo"
	nodeReplicaSet = "obj:apps/ReplicaSet/default/podinfo-podinfo-d9585d794"
)

// panelOf renders the panel fragment of node id on the instance at base.
func panelOf(t *testing.T, s *site, base, id string) string {
	t.Helper()
	res := s.get(t, base+"/node?id="+url.QueryEscape(id), htmxRequest)
	if res.status != http.StatusOK {
		t.Fatalf("panel %s = %d\n%s", id, res.status, res.body)
	}
	return res.body
}

// factsOfPanel returns the label-to-value pairs of the panel's dl, tags
// dropped.
func factsOfPanel(panel string) map[string]string {
	out := map[string]string{}
	for _, m := range regexp.MustCompile(`<dt>([^<]*)</dt><dd[^>]*>(.*?)</dd>`).FindAllStringSubmatch(panel, -1) {
		out[m[1]] = strings.TrimSpace(strings.ReplaceAll(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(m[2], ""), "\n", " "))
	}
	return out
}

// TestPanelFacts (4.4): the per-kind facts of F1's Deployment, ReplicaSet, Pod
// and Service, read from the object document.
func TestPanelFacts(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, tc := range []struct {
		node string
		want map[string]string
		none []string
	}{
		{nodeDeployment, map[string]string{"Replicas": "2 ready of 2 desired", "Image": "ghcr.io/stefanprodan/podinfo:6.7.1", "Strategy": "RollingUpdate"}, []string{"Node", "Type"}},
		{nodeReplicaSet, map[string]string{"Replicas": "2 ready of 2 desired"}, []string{"Image", "Strategy"}},
		{nodePod, map[string]string{"Restarts": "0"}, []string{"Replicas", "Image"}},
		{nodeService, map[string]string{"Type": "ClusterIP"}, []string{"Replicas", "Image"}},
	} {
		got := factsOfPanel(panelOf(t, s, "/instances/default/podinfo", tc.node))
		for k, v := range tc.want {
			if got[k] != v {
				t.Errorf("%s: fact %s = %q, want %q (all: %v)", tc.node, k, got[k], v, got)
			}
		}
		for _, k := range tc.none {
			if _, ok := got[k]; ok {
				t.Errorf("%s: unexpected fact %s", tc.node, k)
			}
		}
	}
	pod := factsOfPanel(panelOf(t, s, "/instances/default/podinfo", nodePod))
	if pod["Node"] == "" || pod["Started"] == "" {
		t.Errorf("a Pod panel lacks its node or start: %v", pod)
	}
	svc := factsOfPanel(panelOf(t, s, "/instances/default/podinfo", nodeService))
	if !regexp.MustCompile(`^\d+/TCP → \S+`).MatchString(svc["Ports"]) {
		t.Errorf("Service ports %q, want port/protocol → targetPort", svc["Ports"])
	}
	for _, id := range []string{nodeDeployment, nodePod} {
		body := panelOf(t, s, "/instances/default/podinfo", id)
		for _, hidden := range []string{"env", "command", "args", "volume"} {
			if regexp.MustCompile(`<dt>[^<]*` + hidden).MatchString(strings.ToLower(body)) {
				t.Errorf("%s: the panel shows %s", id, hidden)
			}
		}
	}
}

// TestPanelBodyAndLinks (4.4): the health message sits in a box toned by the
// node's health, the buttons keep their links, and a Pod's Logs button is the
// first and the dark one.
func TestPanelBodyAndLinks(t *testing.T) {
	broken := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	const waiting = "obj:_/Pod/default/podinfo-podinfo-794bd8c7fb-cc5ql"
	pod := panelOf(t, broken, "/instances/default/podinfo", waiting)
	if m := regexp.MustCompile(`<p class="panel-msg hue-degraded">([^<]+)</p>`).FindStringSubmatch(pod); m == nil {
		t.Errorf("the waiting Pod's message is not in a degraded box:\n%s", pod)
	}
	links := regexp.MustCompile(`<a class="btn( btn-dark)?" href="([^"]*)"[^>]*>([^<]*)</a>`).FindAllStringSubmatch(between(pod, `<p class="panel-links">`, "</p>"), -1)
	names := make([]string, 0, len(links))
	for _, l := range links {
		names = append(names, l[3])
	}
	if strings.Join(names, ",") != "Logs,Events,YAML" || links[0][1] == "" || links[1][1] != "" || links[2][1] != "" {
		t.Errorf("Pod buttons %v (dark %q), want Logs dark first, then Events and YAML", names, links[0][1])
	}
	if !strings.Contains(links[0][2], "?tab=logs&amp;log=log_") {
		t.Errorf("the Logs button %q does not open the pane", links[0][2])
	}
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	dep := panelOf(t, s, "/instances/default/podinfo", nodeDeployment)
	if !strings.Contains(dep, `<p class="panel-msg hue-healthy">`) || strings.Contains(dep, "btn-dark") {
		t.Errorf("Deployment panel:\n%s", dep)
	}
	for _, name := range []string{">Events</a>", ">YAML</a>"} {
		if !strings.Contains(dep, name) {
			t.Errorf("the Deployment panel lost %s", name)
		}
	}
}

// TestPanelOfAComponent (4.4): a component is not a Kubernetes object; the
// panel says so and names what it renders, with no YAML and no Events link.
func TestPanelOfAComponent(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	panel := panelOf(t, s, "/instances/default/podinfo", "comp:mi/default/podinfo/podinfo")
	for _, want := range []string{"<strong>Rendered.</strong> A Component is not a Kubernetes object: it is a part of the module, so it has no events or YAML of its own.",
		`<dt>Renders</dt><dd><span class="mono">`, "Deployment podinfo-podinfo", "Service podinfo-podinfo"} {
		if !strings.Contains(panel, want) {
			t.Errorf("component panel lacks %q:\n%s", want, panel)
		}
	}
	if strings.Contains(panel, ">YAML</a>") || strings.Contains(panel, ">Events</a>") {
		t.Errorf("a component offers YAML or Events:\n%s", panel)
	}
}

// TestPanelOfTheOwnersNode (4.4): the owner's own node has no Open link, an
// Events link to the tab and the identity facts, for an instance and a package.
func TestPanelOfTheOwnersNode(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	inst := panelOf(t, s, "/instances/default/podinfo", "mi:default/podinfo")
	if strings.Contains(inst, "Open page") || !strings.Contains(inst, `<a class="btn" href="/instances/default/podinfo?tab=events">Events</a>`) || strings.Contains(inst, ">YAML</a>") {
		t.Errorf("instance root links:\n%s", between(inst, "panel-links", "</p>"))
	}
	facts := factsOfPanel(inst)
	if facts["Module"] != "testing.opmodel.dev/modules/operator/podinfo@v0" || facts["Version"] != "v0.1.11" || facts["Owner"] != "the controller" ||
		facts["Applier"] != "ServiceAccount podinfo-applier" {
		t.Errorf("instance facts: %v", facts)
	}
	pkg := panelOf(t, s, "/packages/pkg/podinfo", "mp:pkg/podinfo")
	pf := factsOfPanel(pkg)
	if pf["Source"] != "OCIRepository pkg/podinfo-release" || pf["Interval"] != "every 1 min" || pf["Revision"] != "not recorded" || pf["Owner"] != "the controller" || pf["Path"] == "" {
		t.Errorf("package facts: %v", pf)
	}
	if strings.Contains(pkg, "Open page") {
		t.Error("the package's own panel links to the page already open")
	}
}

// TestPanelAtRest (4.3): the Graph and Resources tabs with neither node nor
// focus show the owner's panel, spotlight nothing, and name the panel Clear
// selection brings back; a Clear link without script drops node and focus.
func TestPanelAtRest(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, tab := range []string{"graph", "resources"} {
		main := mainOf(s.get(t, "/instances/default/podinfo?tab="+tab).body)
		detail := between(main, `id="detail"`, "</aside>")
		if !strings.Contains(detail, `data-root-panel="/instances/default/podinfo/node?id=mi%3Adefault%2Fpodinfo"`) ||
			!strings.Contains(detail, `<p class="kicker">Instance</p>`) || !strings.Contains(detail, "default/<wbr>podinfo") ||
			strings.Contains(detail, "Select a graph node") {
			t.Errorf("tab %s: the panel at rest:\n%s", tab, detail)
		}
		if strings.Contains(main, `aria-current="true"`) && tab == "graph" {
			t.Errorf("tab %s: a node is current with none selected", tab)
		}
	}
	graph := mainOf(s.get(t, "/instances/default/podinfo?tab=graph").body)
	if strings.Contains(graph, `class="node kind-object health-healthy ap-`) && regexp.MustCompile(`class="node [^"]* dim"`).MatchString(graph) {
		t.Error("a node is dimmed at rest")
	}
	if !regexp.MustCompile(`<a class="tool-link" href="/instances/default/podinfo\?tab=graph" hx-boost="false" data-clear-selection hidden>`).MatchString(graph) {
		t.Errorf("Clear selection at rest:\n%s", between(graph, "data-clear-selection", ">"))
	}
}

// TestPanelOfASelection (4.3): with a node or focus the Clear link is shown and
// plain, the panel is the selected object's, and node wins over focus.
func TestPanelOfASelection(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	sel := mainOf(s.get(t, "/instances/default/podinfo?tab=graph&focus="+url.QueryEscape(nodeDeployment)+"&node="+url.QueryEscape(nodeDeployment)).body)
	if !regexp.MustCompile(`<a class="tool-link" href="/instances/default/podinfo\?tab=graph" hx-boost="false" data-clear-selection>`).MatchString(sel) {
		t.Errorf("Clear selection with a selection (a plain link without focus and node, shown):\n%s", between(sel, "data-clear-selection", ">"))
	}
	if d := between(sel, `id="detail"`, "</aside>"); !strings.Contains(d, "podinfo-podinfo") || !strings.Contains(d, `<p class="kicker">Object</p>`) {
		t.Errorf("the selected panel:\n%s", d)
	}
	// node wins over focus for the panel.
	both := mainOf(s.get(t, "/instances/default/podinfo?tab=resources&node="+url.QueryEscape(nodeService)+"&focus="+url.QueryEscape(nodeDeployment)).body)
	if d := between(both, `id="detail"`, "</aside>"); !strings.Contains(d, "Service") || strings.Contains(d, "Strategy") {
		t.Errorf("node does not win over focus:\n%s", d)
	}
}

// TestPanelWhenTheGraphIsUnreadable (4.3): the panel is never an empty
// placeholder; a graph that cannot be read says so in its place.
func TestPanelWhenTheGraphIsUnreadable(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.DenyResources("moduleinstances")).get(t, "/instances/default/podinfo?tab=graph")
	if strings.Contains(s.body, "Select a graph node, an object's YAML or its events to see them here.") {
		t.Error("the placeholder is back")
	}
}

// requestCounter wraps an API handler and counts requests to the object
// resource, so a test can tell whether the panel read an object.
type requestCounter struct {
	h       http.Handler
	objects atomic.Int64
	fail    bool
}

func (c *requestCounter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/object") {
		c.objects.Add(1)
		if c.fail {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"code":"upstream_unavailable","status":503}`))
			return
		}
	}
	c.h.ServeHTTP(w, r)
}

func countingSite(t *testing.T, objs []*unstructured.Unstructured, rule apitest.Rule, fail bool) (*site, *requestCounter) {
	t.Helper()
	c := &requestCounter{h: apitest.New(t, objs, rule), fail: fail}
	return mount(t, c), c
}

// TestPanelReadsNoObjectForAnUnreadableNode (4.4, OP-A3): a node that is
// forbidden carries no fact and the panel makes no request for its object; a
// readable one makes exactly one.
func TestPanelReadsNoObjectForAnUnreadableNode(t *testing.T) {
	s, c := countingSite(t, apitest.F1(t), apitest.DenyResources("deployments"), false)
	panel := panelOf(t, s, "/instances/default/podinfo", nodeDeployment)
	if n := c.objects.Load(); n != 0 {
		t.Errorf("the panel of a forbidden node made %d object requests", n)
	}
	if got := factsOfPanel(panel); got["Replicas"] != "" || got["Image"] != "" || got["Strategy"] != "" {
		t.Errorf("a forbidden node carries facts: %v", got)
	}
	if !strings.Contains(panel, "locked-panel") {
		t.Errorf("a forbidden node's panel is not locked:\n%s", panel)
	}
	ok, c2 := countingSite(t, apitest.F1(t), apitest.AllowAll, false)
	panelOf(t, ok, "/instances/default/podinfo", nodeDeployment)
	if n := c2.objects.Load(); n != 1 {
		t.Errorf("a readable node made %d object requests, want 1", n)
	}
	// A component and the owner's node are not objects: no request.
	panelOf(t, ok, "/instances/default/podinfo", "comp:mi/default/podinfo/podinfo")
	panelOf(t, ok, "/instances/default/podinfo", "mi:default/podinfo")
	if n := c2.objects.Load(); n != 1 {
		t.Errorf("a component or the owner made an object request (%d in all)", n)
	}
}

// TestPanelWhenTheObjectCannotBeRead (4.4): a refused object read leaves the
// graph's own facts, says the details could not be read, and is not an error
// page.
func TestPanelWhenTheObjectCannotBeRead(t *testing.T) {
	s, _ := countingSite(t, apitest.F1(t), apitest.AllowAll, true)
	res := s.get(t, "/instances/default/podinfo/node?id="+url.QueryEscape(nodeDeployment), htmxRequest)
	if res.status != http.StatusOK || !strings.Contains(res.body, "Details from the object could not be read.") {
		t.Fatalf("status %d:\n%s", res.status, res.body)
	}
	got := factsOfPanel(res.body)
	if got["Replicas"] != "" || got["Image"] != "" || got["Object"] == "" || got["Health reason"] == "" {
		t.Errorf("facts when the object is unreadable: %v", got)
	}
	if !strings.Contains(res.body, "panel-msg") {
		t.Error("the graph's own health message is gone")
	}
}

// withImage sets the image of the first container of the podinfo Deployment.
func withImage(t *testing.T, image string) []*unstructured.Unstructured {
	t.Helper()
	objs := apitest.F1(t)
	for _, o := range objs {
		if o.GetKind() == "Deployment" && o.GetName() == "podinfo-podinfo" {
			cs, found, err := unstructured.NestedSlice(o.Object, "spec", "template", "spec", "containers")
			if err != nil || !found || len(cs) == 0 {
				t.Fatalf("the Deployment has no containers: %v", err)
			}
			cs[0].(map[string]any)["image"] = image
			if err := unstructured.SetNestedSlice(o.Object, cs, "spec", "template", "spec", "containers"); err != nil {
				t.Fatal(err)
			}
		}
	}
	return objs
}

// TestPanelImageWithUserinfo (4.5, OP-A3): an image reference holding userinfo
// shows without it, and neither the user nor the password is anywhere in the
// panel fragment or the page that opens on the node.
func TestPanelImageWithUserinfo(t *testing.T) {
	const digest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	for _, tc := range []struct{ image, want string }{
		{"admin:hunter2@registry.example/app:1", "registry.example/app:1"},
		{"admin:hu/nter2@registry.example/app:1", "registry.example/app:1"},
		{"admin:hu@nter2@registry.example/app:1", "registry.example/app:1"},
		{"admin:hu/n@ter2@registry.example/app:1", "registry.example/app:1"},
		{"admin:hunter2@registry.example/app@" + digest, "registry.example/app@" + digest},
		{"registry.example/app@" + digest, "registry.example/app@" + digest},
	} {
		s := newSite(t, withImage(t, tc.image), apitest.AllowAll)
		frag := panelOf(t, s, "/instances/default/podinfo", nodeDeployment)
		page := s.get(t, "/instances/default/podinfo?tab=graph&focus="+url.QueryEscape(nodeDeployment)).body
		if got := factsOfPanel(frag)["Image"]; got != tc.want {
			t.Errorf("image %q shows %q, want %q", tc.image, got, tc.want)
		}
		for _, secret := range []string{"admin", "hunter2", "hu/nter2", "nter2", "ter2"} {
			if strings.Contains(tc.image, secret) && (strings.Contains(frag, secret) || strings.Contains(page, secret)) {
				t.Errorf("image %q: %q appears in the page or the panel", tc.image, secret)
			}
		}
	}
	// A reference holding markup is text.
	hostileImage := `<script>alert(1)</script><img src=x onerror=alert(2)>`
	frag := panelOf(t, newSite(t, withImage(t, hostileImage), apitest.AllowAll), "/instances/default/podinfo", nodeDeployment)
	if strings.Contains(frag, "<script>") || strings.Contains(frag, "<img src=x") || !strings.Contains(frag, "&lt;script&gt;alert(1)&lt;/<wbr>script&gt;") {
		t.Errorf("an image holding markup is not escaped:\n%s", frag)
	}
}

// TestImageText (4.5, OP-A3): the userinfo cut, case by case.
func TestImageText(t *testing.T) {
	const d256 = "sha256:0123456789abcdef"
	for _, tc := range []struct{ name, in, want string }{
		{"empty", "", ""},
		{"tag only", "app:1", "app:1"},
		{"repository and tag", "registry.example/app:1", "registry.example/app:1"},
		{"user and password", "user:secret@registry.example/app:1", "registry.example/app:1"},
		{"user only", "user@registry.example/app:1", "registry.example/app:1"},
		{"scheme kept", "https://user:secret@registry.example/app:1", "https://registry.example/app:1"},
		{"scheme without userinfo", "https://registry.example/app:1", "https://registry.example/app:1"},
		{"slash in the password", "user:ab/cd@registry.example/app:1", "registry.example/app:1"},
		{"at sign in the password", "user:a@b@registry.example/app:1", "registry.example/app:1"},
		{"slash and at sign in the password", "user:a/b@c/d@registry.example/app:1", "registry.example/app:1"},
		{"no slash after the userinfo", "user:secret@app:1", "app:1"},
		{"digest", "registry.example/app@" + d256, "registry.example/app@" + d256},
		{"digest without a repository host", "app@" + d256, "app@" + d256},
		{"digest with userinfo", "user:secret@registry.example/app@" + d256, "registry.example/app@" + d256},
		{"sha384", "registry.example/app@sha384:00ff", "registry.example/app@sha384:00ff"},
		{"sha512 with userinfo", "u:p@registry.example/app@sha512:00ff", "registry.example/app@sha512:00ff"},
		{"a git sha tag is not a digest", "user:pw@app:0123456789abcdef0123456789abcdef01234567", "app:0123456789abcdef0123456789abcdef01234567"},
		{"an unknown algorithm is not a digest, so its at sign cuts", "user:pw@app@md5:00ff", "md5:00ff"},
		{"colon before the scheme separator is not a scheme", "user:pw://x@registry.example/app:1", "registry.example/app:1"},
	} {
		if got := imageText(tc.in); got != tc.want {
			t.Errorf("%s: imageText(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

// TestPanelIsPolicyClean: the panel adds no inline style or handler.
func TestPanelIsPolicyClean(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, id := range []string{nodeDeployment, nodePod, nodeService, "comp:mi/default/podinfo/podinfo", "mi:default/podinfo"} {
		body := panelOf(t, s, "/instances/default/podinfo", id)
		for _, bad := range []string{` style=`, ` onclick=`, `<script`, `<style`} {
			if strings.Contains(body, bad) {
				t.Errorf("%s: the panel carries %q", id, bad)
			}
		}
	}
}
