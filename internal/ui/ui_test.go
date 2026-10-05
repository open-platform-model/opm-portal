package ui

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

var update = flag.Bool("update", false, "rewrite the golden HTML fragments")

// TestTheUIImportsOnlyTheWireTypes (0030:D2:R1): among this module's
// packages, the UI's code imports only api/v1alpha1, so every fact a page
// shows is one the read API serves. Its tests may also build the API with
// internal/api/apitest.
func TestTheUIImportsOnlyTheWireTypes(t *testing.T) {
	const module = "github.com/open-platform-model/opm-portal/"
	allowed := map[string]bool{module + "api/v1alpha1": true}
	allowedInTests := map[string]bool{module + "internal/api/apitest": true}
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("listing the package's files: %v", err)
	}
	fset := token.NewFileSet()
	for _, f := range files {
		parsed, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range parsed.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if !strings.HasPrefix(path, module) || allowed[path] || (strings.HasSuffix(f, "_test.go") && allowedInTests[path]) {
				continue
			}
			t.Errorf("%s imports %s: the UI reads through the read API only", f, path)
		}
	}
}

// TestVendoredFilesMatchTheirChecksums: every vendored file is listed
// with the SHA-256 it was checked in with, and nothing else is vendored.
func TestVendoredFilesMatchTheirChecksums(t *testing.T) {
	dir := filepath.Join("static", "vendor")
	f, err := os.Open(filepath.Join(dir, "CHECKSUMS"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	listed := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sum, name, ok := strings.Cut(line, "  ")
		if !ok {
			t.Fatalf("CHECKSUMS line %q is not '<sha256>  <file>'", line)
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		got := sha256.Sum256(b)
		if hex.EncodeToString(got[:]) != sum {
			t.Errorf("%s: sha256 %x, CHECKSUMS says %s", name, got, sum)
		}
		listed[name] = true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		n := e.Name()
		if n == "CHECKSUMS" || strings.HasPrefix(n, "LICENSE.") {
			continue
		}
		if !listed[n] {
			t.Errorf("%s is vendored but not in CHECKSUMS", n)
		}
	}
}

// goldenPage is one page whose main region is compared with
// testdata/golden/<name>.html.
type goldenPage struct {
	name string
	path string
	hx   bool
}

var f1Pages = []goldenPage{
	{"platform", "/", false},
	{"platform-node-registration", "/?node=treg:default.refused-claim-fixture", false},
	{"instances", "/instances", false},
	{"instances-default", "/instances?namespace=default", false},
	{"instance-podinfo", "/instances/default/podinfo", false},
	{"instance-cert-manager", "/instances/cert-manager/cert-manager", false},
	{"instance-cert-manager-expanded", "/instances/cert-manager/cert-manager?expand=grp:configuration/mi/cert-manager/cert-manager", false},
	{"instance-web-cli-owned", "/instances/web/web", false},
	{"packages", "/packages", false},
	{"package-podinfo", "/packages/pkg/podinfo", false},
	{"fragment-object-deployment", "/instances/default/podinfo/object?group=apps&kind=Deployment&namespace=default&name=podinfo-podinfo", true},
	{"fragment-node-deployment", "/instances/default/podinfo/node?id=obj:apps/Deployment/default/podinfo-podinfo", true},
	{"fragment-events-pod", "/instances/default/podinfo/events?kind=Pod&namespace=default&name=podinfo-podinfo-d9585d794-4lg6h", true},
	{"fragment-registration-events", "/platform/registrations/default.refused-claim-fixture/events", true},
}

// TestGoldenPages renders every page over the F1 capture and compares its
// main region, or a fragment, with its golden file.
func TestGoldenPages(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	checkGoldens(t, s, f1Pages)
}

// TestGoldenBrokenRollout: the image-break sample shows Applied and
// Degraded side by side (0030:D3:R1/R2).
func TestGoldenBrokenRollout(t *testing.T) {
	s := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	checkGoldens(t, s, []goldenPage{{"instance-podinfo-broken", "/instances/default/podinfo", false}})
	body := s.get(t, "/instances/default/podinfo").body
	head := between(body, `id="owner-head"`, "</header>")
	if !strings.Contains(head, `class="applied applied-applied"`) || !strings.Contains(head, `class="health health-degraded"`) {
		t.Errorf("the broken rollout's head does not show Applied and Degraded apart:\n%s", head)
	}
}

// TestGoldenLocked: an unreadable kind inside an instance, and a list the
// caller may not read, render locked (0030:D5:R7).
func TestGoldenLocked(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.DenyResources("services"))
	checkGoldens(t, s, []goldenPage{{"instance-podinfo-services-locked", "/instances/default/podinfo", false}})
	main := mainOf(s.get(t, "/instances/default/podinfo").body)
	if !strings.Contains(main, `<li class="object locked">`) || !strings.Contains(main, "Service default/<wbr>podinfo-podinfo") {
		t.Error("the forbidden Service is not rendered locked")
	}
	if !strings.Contains(main, `class="node kind-object locked"`) {
		t.Error("the forbidden Service's graph node is not locked")
	}

	lists := newSite(t, apitest.F1(t), apitest.DenyResources("moduleinstances"))
	checkGoldens(t, lists, []goldenPage{{"instances-locked", "/instances", false}})
	if main := mainOf(lists.get(t, "/instances").body); !strings.Contains(main, "locked-panel") || strings.Contains(main, "<table") {
		t.Errorf("a forbidden list is not a locked panel without rows:\n%s", main)
	}
}

func checkGoldens(t *testing.T, s *site, pages []goldenPage) {
	t.Helper()
	for _, p := range pages {
		t.Run(p.name, func(t *testing.T) {
			var res result
			if p.hx {
				res = s.get(t, p.path, htmxRequest)
			} else {
				res = s.get(t, p.path)
			}
			if res.status != http.StatusOK {
				t.Fatalf("GET %s = %d\n%s", p.path, res.status, res.body)
			}
			got := res.body
			if !p.hx {
				got = mainOf(got)
			}
			got = strings.TrimSpace(collapse(got)) + "\n"
			golden := filepath.Join("testdata", "golden", p.name+".html")
			if *update {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
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
				t.Errorf("GET %s differs from %s (run with -update after checking the diff):\n%s", p.path, golden, got)
			}
		})
	}
}

var blankLines = regexp.MustCompile(`\n[ \t]*\n+`)

// collapse removes the blank lines template actions leave, so goldens
// change only when the markup does.
func collapse(s string) string { return blankLines.ReplaceAllString(s, "\n") }

func between(s, from, to string) string {
	i := strings.Index(s, from)
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], to)
	if j < 0 {
		return s[i:]
	}
	return s[i : i+j]
}

// mainOf returns the page's main element.
func mainOf(page string) string {
	i := strings.Index(page, "<main ")
	j := strings.LastIndex(page, "</main>")
	if i < 0 || j < i {
		return ""
	}
	return page[i : j+len("</main>")]
}

// TestEveryPageCarriesThePagePolicy: every page and fragment is served
// with the page policy, and no element carries an inline style, an inline
// script or an event-handler attribute.
func TestEveryPageCarriesThePagePolicy(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	pages := append([]goldenPage{{"not-found", "/nope", false}}, f1Pages...)
	for _, p := range pages {
		res := s.get(t, p.path)
		if got := res.header.Get("Content-Security-Policy"); got != PagePolicy {
			t.Errorf("%s: Content-Security-Policy %q", p.path, got)
		}
		checkNoInlineCode(t, p.path, res.body)
	}
	if res := s.get(t, "/", anonymous); res.header.Get("Content-Security-Policy") != PagePolicy {
		t.Error("the sign-in page lacks the page policy")
	}
}

func checkNoInlineCode(t *testing.T, path, body string) {
	t.Helper()
	z := html.NewTokenizer(strings.NewReader(body))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		tok := z.Token()
		hasSrc := false
		for _, a := range tok.Attr {
			if msg := inlineCode(a); msg != "" {
				t.Errorf("%s: <%s> %s", path, tok.Data, msg)
			}
			hasSrc = hasSrc || a.Key == "src"
		}
		if (tok.Data == "script" && !hasSrc) || tok.Data == "style" {
			t.Errorf("%s: inline <%s>", path, tok.Data)
		}
	}
}

// inlineCode says what is wrong with an attribute under the page policy,
// or "".
func inlineCode(a html.Attribute) string {
	switch {
	case a.Key == "style":
		return "has an inline style"
	case strings.HasPrefix(a.Key, "hx-on"):
		return "has " + a.Key + ", which needs eval"
	case strings.HasPrefix(a.Key, "on"):
		return "has the event handler " + a.Key
	case a.Key == "hx-trigger" && strings.Contains(a.Val, "["):
		return "has a trigger filter, which needs eval"
	case a.Key == "src" && !strings.HasPrefix(a.Val, "/static/"):
		return "loads " + a.Val + " from outside the portal"
	}
	return ""
}

// TestAccessibilitySmoke: one h1 per page; every form control has a
// label; every button and graph node has an accessible name; no positive
// tabindex reorders focus; graph nodes come in column order, then row.
func TestAccessibilitySmoke(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, p := range f1Pages {
		if p.hx {
			continue
		}
		checkAccessible(t, p.path, s.get(t, p.path).body)
	}
}

func checkAccessible(t *testing.T, path, body string) {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	c := &a11y{t: t, path: path, labels: map[string]bool{}}
	c.walk(doc)
	c.finish()
}

// a11y collects what the accessibility smoke check needs from one page.
type a11y struct {
	t            *testing.T
	path         string
	labels       map[string]bool
	h1           int
	controls     []*html.Node
	nodeX, nodeY []int
}

func (c *a11y) walk(n *html.Node) {
	if n.Type == html.ElementNode {
		c.element(n)
	}
	for k := n.FirstChild; k != nil; k = k.NextSibling {
		c.walk(k)
	}
}

func (c *a11y) element(n *html.Node) {
	switch n.Data {
	case "h1":
		c.h1++
	case "label":
		c.labels[attr(n, "for")] = true
	case "input", "select", "textarea":
		if attr(n, "type") != "hidden" {
			c.controls = append(c.controls, n)
		}
	case "button":
		if strings.TrimSpace(text(n)) == "" && attr(n, "aria-label") == "" {
			c.t.Errorf("%s: a button has no accessible name", c.path)
		}
	case "a":
		if strings.Contains(attr(n, "class"), "node") {
			c.node(n)
		}
	}
	if ti := attr(n, "tabindex"); ti != "" && ti != "0" && ti != "-1" {
		c.t.Errorf("%s: tabindex %s reorders focus", c.path, ti)
	}
}

func (c *a11y) node(n *html.Node) {
	if attr(n, "aria-label") == "" {
		c.t.Errorf("%s: graph node %s has no accessible name", c.path, attr(n, "href"))
	}
	for k := n.FirstChild; k != nil; k = k.NextSibling {
		if k.Type == html.ElementNode && k.Data == "rect" && attr(k, "class") == "node-box" {
			x, _ := strconv.Atoi(attr(k, "x"))
			y, _ := strconv.Atoi(attr(k, "y"))
			c.nodeX, c.nodeY = append(c.nodeX, x), append(c.nodeY, y)
		}
	}
}

func (c *a11y) finish() {
	if c.h1 != 1 {
		c.t.Errorf("%s: %d h1 elements, want 1", c.path, c.h1)
	}
	for _, n := range c.controls {
		hasLabel := c.labels[attr(n, "id")] && attr(n, "id") != ""
		if !hasLabel && attr(n, "aria-label") == "" && (n.Parent == nil || n.Parent.Data != "label") {
			c.t.Errorf("%s: <%s name=%q> has no label", c.path, n.Data, attr(n, "name"))
		}
	}
	for i := 1; i < len(c.nodeX); i++ {
		if c.nodeX[i] < c.nodeX[i-1] || (c.nodeX[i] == c.nodeX[i-1] && c.nodeY[i] < c.nodeY[i-1]) {
			c.t.Errorf("%s: graph node %d at (%d,%d) is focused after (%d,%d)", c.path, i, c.nodeX[i], c.nodeY[i], c.nodeX[i-1], c.nodeY[i-1])
		}
	}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// TestPagesDeclareTheirTopics: each page names the stream topics its
// regions follow.
func TestPagesDeclareTheirTopics(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for path, want := range map[string]string{
		"/":                          "platform events:platform",
		"/instances":                 "instances",
		"/instances?namespace=web":   "instances:web",
		"/instances/default/podinfo": "instance:default/podinfo events:instance:default/podinfo",
		"/packages/pkg/podinfo":      "package:pkg/podinfo events:package:pkg/podinfo",
		"/packages":                  "",
	} {
		body := s.get(t, path).body
		main := between(body, "<main ", ">")
		if got := between(main, `data-topics="`, `" `); got != `data-topics="`+want {
			t.Errorf("%s: %s, want topics %q", path, got, want)
		}
		if !strings.Contains(body, `sse-connect="/api/v1alpha1/clusters/default/stream?topics=`) {
			t.Errorf("%s: no stream on the body", path)
		}
	}
}

// TestSignInWithoutASession: a page without the session is the sign-in
// page with 401, and asks for no review.
func TestSignInWithoutASession(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, path := range []string{"/", "/instances", "/instances/default/podinfo", "/packages/pkg/podinfo/object?kind=Pod&name=x&namespace=pkg"} {
		res := s.get(t, path, anonymous)
		if res.status != http.StatusUnauthorized || !strings.Contains(res.body, "You are not signed in") {
			t.Errorf("GET %s without a session = %d", path, res.status)
		}
	}
}

// TestProblemsRenderAsRegions: a missing instance is a not-found page and
// a refused object a locked fragment.
func TestProblemsRenderAsRegions(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	if res := s.get(t, "/instances/default/nope"); res.status != http.StatusNotFound || !strings.Contains(res.body, "Not found.") {
		t.Errorf("missing instance = %d", res.status)
	}
	res := s.get(t, "/instances/default/podinfo/object?kind=Secret&namespace=default&name=podinfo", htmxRequest)
	if res.status != http.StatusOK || !strings.Contains(res.body, "locked-panel") {
		t.Errorf("Secret object = %d\n%s", res.status, res.body)
	}
	if res := s.get(t, "/nope"); res.status != http.StatusNotFound {
		t.Errorf("unknown page = %d", res.status)
	}
	if res := s.get(t, "/", func(r *http.Request) { r.Method = http.MethodPost }); res.status != http.StatusMethodNotAllowed {
		t.Errorf("POST / = %d", res.status)
	}
}

// TestYAMLHidesWhatIsNeverServed: the YAML view carries the pod template
// but no managed fields, last-applied annotation or values.
func TestYAMLHidesWhatIsNeverServed(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	body := s.get(t, "/instances/default/podinfo/object?group=apps&kind=Deployment&namespace=default&name=podinfo-podinfo", htmxRequest).body
	if !strings.Contains(body, "template:") {
		t.Error("the YAML lacks the pod template")
	}
	for _, never := range []string{"managedFields", "last-applied-configuration", "values:"} {
		if strings.Contains(body, never) {
			t.Errorf("the YAML carries %s", never)
		}
	}
}

// TestUnknownValuesReadUnknown (0030:D2:R3).
func TestUnknownValuesReadUnknown(t *testing.T) {
	cases := []badge{
		appliedBadge(v1Reconcile("Exploded")),
		stateBadge("Glowing"),
		verdictBadge("Maybe"),
	}
	for _, b := range cases {
		if b.Text != "unknown" || !strings.HasSuffix(b.Class, "-unknown") || b.Title == "" {
			t.Errorf("badge %+v, want unknown with the raw value as its title", b)
		}
	}
}

// TestStaticAssetsAreServed: the page's assets come from the binary.
func TestStaticAssetsAreServed(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	err := fs.WalkDir(staticFS, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if res := s.get(t, "/"+p, anonymous); res.status != http.StatusOK {
			t.Errorf("GET /%s = %d", p, res.status)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func v1Reconcile(state string) v1.Reconcile { return v1.Reconcile{State: state} }
