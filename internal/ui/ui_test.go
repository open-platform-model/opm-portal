package ui

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

var update = flag.Bool("update", false, "rewrite the golden HTML fragments")

// TestTheUIImportsOnlyTheWireTypes (portal:D2:R1): among this module's
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
	{"platform-catalogs", "/?tab=catalogs", false},
	{"platform-providers-refused", "/?pstatus=refused&eresource=registration:default.refused-claim-fixture", false},
	{"installed", "/installed", false},
	{"installed-default", "/installed?namespace=default", false},
	{"installed-packages", "/installed?kind=package", false},
	{"installed-uses", "/installed?uses=opmodel.dev/catalogs/opm/traits/backup@v1alpha1", false},
	{"installed-ignored", "/installed?health=Bogus&q=pod", false},
	{"instance-podinfo", "/instances/default/podinfo", false},
	{"instance-podinfo-focus", "/instances/default/podinfo?tab=graph&focus=obj:apps/Deployment/default/podinfo-podinfo", false},
	{"instance-podinfo-resources", "/instances/default/podinfo?tab=resources", false},
	{"instance-podinfo-events", "/instances/default/podinfo?tab=events&type=Normal", false},
	{"instance-podinfo-logs", "/instances/default/podinfo?tab=logs", false},
	{"instance-podinfo-yaml", "/instances/default/podinfo?tab=yaml&object=apps/Deployment/default/podinfo-podinfo", false},
	{"instance-backup-provider", "/instances/default/backup-provider", false},
	{"instance-backup-provider-provider", "/instances/default/backup-provider?tab=provider", false},
	{"catalog-backup", "/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0", false},
	{"catalog-refused-claim", "/catalog?path=testing.opmodel.dev/catalogs/operator/refused-claim-fixture-absent@v0", false},
	{"catalog-opm", "/catalog?path=opmodel.dev/catalogs/opm@v4", false},
	{"catalog-opm-events", "/catalog?path=opmodel.dev/catalogs/opm@v4&tab=events", false},
	{"catalog-backup-events", "/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0&tab=events", false},
	{"catalog-refused-claim-events", "/catalog?path=testing.opmodel.dev/catalogs/operator/refused-claim-fixture-absent@v0&tab=events&reason=CatalogUnresolved", false},
	{"instance-cert-manager-resources", "/instances/cert-manager/cert-manager?tab=resources", false},
	{"instance-cert-manager", "/instances/cert-manager/cert-manager", false},
	{"instance-cert-manager-expanded", "/instances/cert-manager/cert-manager?expand=grp:configuration/mi/cert-manager/cert-manager/rbac", false},
	{"instance-web-cli-owned", "/instances/web/web", false},
	{"package-podinfo", "/packages/pkg/podinfo", false},
	{"package-podinfo-resources", "/packages/pkg/podinfo?tab=resources", false},
	{"fragment-object-deployment", "/instances/default/podinfo/object?group=apps&kind=Deployment&namespace=default&name=podinfo-podinfo", true},
	{"fragment-node-deployment", "/instances/default/podinfo/node?id=obj:apps/Deployment/default/podinfo-podinfo", true},
	{"fragment-events-pod", "/instances/default/podinfo/events?kind=Pod&namespace=default&name=podinfo-podinfo-d9585d794-4lg6h", true},
}

// TestGoldenPages renders every page over the F1 capture and compares its
// main region, or a fragment, with its golden file.
func TestGoldenPages(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	checkGoldens(t, s, f1Pages)
}

// TestGoldenEndpoints (S16): the Service panel over F1 plus the recorded
// slices in the three forms of the Endpoints fact: a count, the unread mark
// and no line.
func TestGoldenEndpoints(t *testing.T) {
	const node = "/instances/default/podinfo/node?id=obj:_/Service/default/podinfo-podinfo"
	t.Run("counted", func(t *testing.T) {
		s := newSite(t, apitest.F1WithEndpointSlices(t), apitest.AllowAll)
		checkGoldens(t, s, []goldenPage{{"fragment-node-service-endpoints", node, true}})
	})
	t.Run("unread", func(t *testing.T) {
		s := newSite(t, apitest.F1WithEndpointSlices(t), apitest.DenyResources("endpointslices"))
		checkGoldens(t, s, []goldenPage{{"fragment-node-service-endpoints-unread", node, true}})
	})
	t.Run("none", func(t *testing.T) {
		s := newSite(t, apitest.F1(t), apitest.AllowAll)
		checkGoldens(t, s, []goldenPage{{"fragment-node-service", node, true}})
	})
}

// TestGoldenBrokenRollout: the image-break sample shows Applied and
// Degraded side by side (portal:D3:R1/R2).
func TestGoldenBrokenRollout(t *testing.T) {
	s := newSite(t, apitest.F1Broken(t), apitest.AllowAll)
	checkGoldens(t, s, []goldenPage{{"instance-podinfo-broken", "/instances/default/podinfo", false}})
	body := s.get(t, "/instances/default/podinfo").body
	applied := between(body, `id="applied-card"`, "</section>")
	health := between(body, `id="health-card"`, "</section>")
	if !strings.Contains(body, `class="state-block hue-applied" id="applied-card"`) || !strings.Contains(body, `class="state-block hue-degraded" id="health-card"`) ||
		!strings.Contains(applied, `<span class="sb-word">Applied</span>`) || !strings.Contains(health, `<span class="sb-word">Degraded</span>`) {
		t.Errorf("the broken rollout's cards do not show Applied and Degraded apart:\n%s\n%s", applied, health)
	}
	if !strings.Contains(health, `href="/instances/default/podinfo?reason=ImagePullBackOff&amp;tab=resources"`) &&
		!strings.Contains(health, `reason=ErrImagePull`) {
		t.Errorf("the Health card does not count the Pod's waiting reason:\n%s", health)
	}
}

// TestGoldenPackageHolder: a package holding a refused claim shows its
// Provider tab (align-owner-pages, 6.2).
func TestGoldenPackageHolder(t *testing.T) {
	s := newSite(t, apitest.F1PackageHolder(t), apitest.AllowAll)
	checkGoldens(t, s, []goldenPage{{"package-provider-provider", "/packages/pkg/provider?tab=provider", false}})
}

// TestGoldenLocked: an unreadable kind inside an instance, and a list the
// caller may not read, render locked (portal:D5:R7).
func TestGoldenLocked(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.DenyResources("services"))
	checkGoldens(t, s, []goldenPage{{"instance-podinfo-services-locked", "/instances/default/podinfo?tab=resources", false}})
	main := mainOf(s.get(t, "/instances/default/podinfo?tab=resources").body)
	if !strings.Contains(main, `row-locked`) || !strings.Contains(main, "default/<wbr>podinfo-podinfo") {
		t.Error("the forbidden Service is not rendered locked")
	}
	if main := mainOf(s.get(t, "/instances/default/podinfo").body); !strings.Contains(main, `class="node kind-object locked"`) {
		t.Error("the forbidden Service's graph node is not locked")
	}

	lists := newSite(t, apitest.F1(t), apitest.DenyResources("moduleinstances"))
	checkGoldens(t, lists, []goldenPage{{"installed-instances-locked", "/installed", false}})
	main = mainOf(lists.get(t, "/installed").body)
	if !strings.Contains(main, "Module instances</strong>: locked") || strings.Contains(main, "/instances/default/podinfo") {
		t.Errorf("a forbidden instance list is not a locked group without its rows:\n%s", main)
	}
	if !strings.Contains(main, "/packages/pkg/podinfo") || strings.Contains(main, "Module packages</strong>: locked") {
		t.Errorf("the readable package list does not show beside the locked instances:\n%s", main)
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
		"/":                                "platform instances events:platform events:registration:default.backup-provider events:registration:default.refused-claim-fixture",
		"/installed":                       "instances",
		"/installed?namespace=web":         "instances", // the lists are read for every namespace (installed-23)
		"/installed?namespace=Not_A_Label": "instances",
		"/instances/default/backup-provider?tab=provider": "instance:default/backup-provider events:instance:default/backup-provider platform instances",
		"/instances/default/podinfo":                      "instance:default/podinfo events:instance:default/podinfo",
		"/packages/pkg/podinfo":                           "package:pkg/podinfo events:package:pkg/podinfo",
	} {
		body := s.get(t, path).body
		main := between(body, "<main ", ">")
		if got := between(main, `data-topics="`, `" `); got != `data-topics="`+want {
			t.Errorf("%s: %s, want topics %q", path, got, want)
		}
		if !strings.Contains(body, `sse-connect="/api/v1alpha1/clusters/default/stream?topics=`) {
			t.Errorf("%s: no stream on the body", path)
		}
		// The stream's expired event closes it for good and is shown.
		if !strings.Contains(body, `sse-close="expired"`) || !strings.Contains(body, "sse:expired") {
			t.Errorf("%s: the page does not stop its stream on expired", path)
		}
	}
}

// TestSignInWithoutASession: a page without the session is the sign-in
// page with 401, and asks for no review.
func TestSignInWithoutASession(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	for _, path := range []string{"/", "/installed", "/instances/default/podinfo", "/packages/pkg/podinfo/object?kind=Pod&name=x&namespace=pkg"} {
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

// TestUnknownValuesReadUnknown (portal:D2:R3).
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

// onlyIn is a caller who may use the named resources in one namespace only
// (a RoleBinding there) and everything else everywhere.
func onlyIn(ns string, resources ...string) apitest.Rule {
	return func(_ string, ra authorizationv1.ResourceAttributes) bool {
		return !slices.Contains(resources, ra.Resource) || ra.Namespace == ns
	}
}

var (
	selectRE = regexp.MustCompile(`(?s)<select id="f-([a-z]+)"[^>]*>(.*?)</select>`)
	optionRE = regexp.MustCompile(`<option value="([^"]*)"[^>]*>([^<]*)</option>`)
	listRE   = regexp.MustCompile(`(?s)<datalist id="f-([a-z]+)-list">(.*?)</datalist>`)
	countRE  = regexp.MustCompile(` \((\d+)\)$`)
)

// installedForm reads the Installed filter form of a page: each select's
// options as "value|text", each datalist's values, and the count row.
func installedForm(t *testing.T, body string) (selects, lists map[string][]string, shown string) {
	t.Helper()
	main := mainOf(body)
	selects, lists = map[string][]string{}, map[string][]string{}
	for _, m := range selectRE.FindAllStringSubmatch(main, -1) {
		for _, o := range optionRE.FindAllStringSubmatch(m[2], -1) {
			selects[m[1]] = append(selects[m[1]], o[1]+"|"+o[2])
		}
	}
	for _, m := range listRE.FindAllStringSubmatch(main, -1) {
		for _, o := range regexp.MustCompile(`<option value="([^"]*)">`).FindAllStringSubmatch(m[2], -1) {
			lists[m[1]] = append(lists[m[1]], o[1])
		}
	}
	return selects, lists, collapse(between(main, `class="shown"`, "</p>"))
}

// optionValues are the values of "value|text" options.
func optionValues(opts []string) []string {
	out := make([]string, 0, len(opts))
	for _, o := range opts {
		v, _, _ := strings.Cut(o, "|")
		out = append(out, v)
	}
	return out
}

// checkCountsInside fails for any option count above total: a count that
// large includes a row outside the namespace.
func checkCountsInside(t *testing.T, selects map[string][]string, total int) {
	t.Helper()
	for name, opts := range selects {
		for _, o := range opts {
			if m := countRE.FindStringSubmatch(o); m != nil {
				if n, _ := strconv.Atoi(m[1]); n > total {
					t.Errorf("%s option %q counts %d rows, more than the %d in default", name, o, n, total)
				}
			}
		}
	}
}

// checkDefaultOnly asserts what a reader who may list only in default sees
// on /installed?namespace=default (cases 1 and 2).
func checkDefaultOnly(t *testing.T, s *site) {
	t.Helper()
	res := s.get(t, "/installed?namespace=default")
	selects, lists, shown := installedForm(t, res.body)
	if shown != `class="shown" id="shown" data-follow="instances:default">Showing <strong>3</strong> of 3 in default` {
		t.Errorf("count row = %q; want 3 of 3 in default", shown)
	}
	checkCountsInside(t, selects, 3)
	if got, want := selects["kind"], []string{"|Any kind", "instance|Instance (3)", "package|Package (0)"}; !slices.Equal(got, want) {
		t.Errorf("kind options = %q; want %q", got, want)
	}
	if got := lists["namespace"]; !slices.Equal(got, []string{"default"}) {
		t.Errorf("namespace choices = %q; want default only", got)
	}
	if _, isSelect := selects["namespace"]; isSelect {
		t.Error("the namespace is a select; a reader who may list one namespace must be able to type another")
	}
	uses := optionValues(selects["uses"])
	for _, outside := range []string{"resources/crds", "resources/namespaces", "resources/mutating-webhooks", "resources/role@", "traits/workload-identity"} {
		if slices.ContainsFunc(uses, func(u string) bool { return strings.Contains(u, outside) }) {
			t.Errorf("the Uses options hold %s, which only rows outside default use: %q", outside, uses)
		}
	}
	for _, row := range rowLinks(mainOf(res.body)) {
		if !strings.Contains(row, "/default/") {
			t.Errorf("the list holds %s, outside default", row)
		}
	}
}

// checkUsesAreTheRowsContracts is case 4: the Uses choices are the union of
// the readable instances' renderContracts, so no blueprint name appears.
func checkUsesAreTheRowsContracts(t *testing.T, s *site) {
	t.Helper()
	var doc v1.InstanceList
	if err := json.Unmarshal([]byte(s.get(t, defaultAPIBase+"/instances").body), &doc); err != nil {
		t.Fatal(err)
	}
	var want []string
	for i := range doc.Items {
		for _, c := range doc.Items[i].RenderContracts {
			if !slices.Contains(want, c) {
				want = append(want, c)
			}
		}
	}
	slices.Sort(want)
	selects, _, _ := installedForm(t, s.get(t, "/installed").body)
	got := optionValues(selects["uses"])[1:] // after the empty choice
	if !slices.Equal(got, want) {
		t.Errorf("Uses choices = %q; want the union of the rows' renderContracts %q", got, want)
	}
	for _, c := range got {
		if strings.Contains(c, "/blueprints/") {
			t.Errorf("the Uses options hold a blueprint: %s", c)
		}
	}
}

// TestInstalledCountsFollowTheCallersNamespaces (SR8, portal:D14:R7,
// portal:D7:R2): a number on the Installed form is a count over rows the
// caller may list. The read API returns nothing outside a granted
// namespace, so no choice or count can include a row outside it; the page
// keeps both kinds to the filtered namespace where one kind is read there.
func TestInstalledCountsFollowTheCallersNamespaces(t *testing.T) {
	f1 := apitest.F1(t)
	t.Run("case 1: instances and packages in default only", func(t *testing.T) {
		checkDefaultOnly(t, newSite(t, f1, onlyIn("default", "moduleinstances", "modulepackages")))
	})
	t.Run("case 2: instances everywhere, packages in default only", func(t *testing.T) {
		checkDefaultOnly(t, newSite(t, f1, onlyIn("default", "modulepackages")))
	})
	t.Run("case 3: a reader who may list everywhere", func(t *testing.T) {
		s := newSite(t, f1, apitest.AllowAll)
		selects, _, shown := installedForm(t, s.get(t, "/installed?namespace=default").body)
		if shown != `class="shown" id="shown" data-follow="instances">Showing <strong>3</strong> of 6` {
			t.Errorf("count row = %q; want 3 of 6", shown)
		}
		want := []string{"|All namespaces", "cert-manager|cert-manager (1)", "default|default (3)", "pkg|pkg (1)", "web|web (1)"}
		if !slices.Equal(selects["namespace"], want) {
			t.Errorf("namespace options = %q; want %q", selects["namespace"], want)
		}
		if !slices.ContainsFunc(selects["uses"], func(u string) bool { return strings.Contains(u, "resources/crds") }) {
			t.Error("the Uses options leave out a contract that a row outside default uses")
		}
	})
	t.Run("case 4: the Uses choices are the contracts of the readable rows", func(t *testing.T) {
		checkUsesAreTheRowsContracts(t, newSite(t, f1, apitest.AllowAll))
	})
}

// TestInstalledTotalCoversEveryNamespace (installed-23): with the caller
// allowed everywhere, a namespace filter narrows the rows and not the total.
func TestInstalledTotalCoversEveryNamespace(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	_, _, shown := installedForm(t, s.get(t, "/installed?namespace=default").body)
	if !strings.HasSuffix(shown, "Showing <strong>3</strong> of 6") {
		t.Errorf("count row = %q; want Showing 3 of 6", shown)
	}
	// The namespace-only reader reads each kind in the namespace asked and
	// follows its stream; the others follow every namespace.
	ns := newSite(t, apitest.F1(t), onlyIn("default", "moduleinstances", "modulepackages"))
	if body := ns.get(t, "/installed?namespace=default").body; !strings.Contains(body, `data-topics="instances:default"`) {
		t.Error("the scoped fallback does not follow instances:default")
	}
	// Both lists locked for every namespace and no namespace named: both
	// render locked, the namespace is a field to type into, and no option
	// count stands for a locked list.
	res := ns.get(t, "/installed")
	selects, lists, _ := installedForm(t, res.body)
	if _, isSelect := selects["namespace"]; isSelect || len(lists["namespace"]) != 0 {
		t.Errorf("namespace = select %v, datalist %v; want a text field and no names read", selects["namespace"], lists["namespace"])
	}
	if got := selects["kind"]; !slices.Equal(got, []string{"|Any kind", "instance|Instance (locked)", "package|Package (locked)"}) {
		t.Errorf("kind options = %q; want both kinds locked with no count", got)
	}
	// Nor does a count stand for health, applied, provider or owner: with
	// neither list read, "(0)" would say nothing is unhealthy (portal:D14:R7).
	for _, name := range []string{"health", "applied", "provider", "owner"} {
		for _, o := range selects[name] {
			if strings.Contains(o, "(0)") {
				t.Errorf("%s option %q carries a zero count for a list the caller may not read", name, o)
			}
		}
	}
	main := mainOf(res.body)
	if strings.Count(main, "locked-panel") != 2 {
		t.Errorf("want both kinds rendered locked:\n%s", between(main, `id="list"`, "</section>"))
	}
}

// TestInstalledPackageSourceIsAModule (installed-26, installed-19): the
// module filter offers a package's source string and matches it, no note
// says packages are excluded, and search matches the kind word and the
// source.
func TestInstalledPackageSourceIsAModule(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	const source = "OCIRepository pkg/podinfo-release"
	res := s.get(t, "/installed?module="+url.QueryEscape(source))
	main := mainOf(res.body)
	if got := rowLinks(main); !slices.Equal(got, []string{"/packages/pkg/podinfo"}) {
		t.Errorf("module=%s lists %v; want the package only", source, got)
	}
	if strings.Contains(main, "records a source, not a module") || strings.Contains(main, "Packages are not included") {
		t.Error("a note still says packages are left out of the module filter")
	}
	selects, _, _ := installedForm(t, res.body)
	if !slices.Contains(selects["module"], source+"|"+source+" (1)") {
		t.Errorf("module options = %q; want the package's source with its count", selects["module"])
	}
	for q, want := range map[string][]string{
		"package":        {"/packages/pkg/podinfo"},
		"ocirepository":  {"/packages/pkg/podinfo"},
		"pkg/podinfo-re": {"/packages/pkg/podinfo"},
	} {
		if got := rowLinks(mainOf(s.get(t, "/installed?q="+url.QueryEscape(q)).body)); !slices.Equal(got, want) {
			t.Errorf("q=%s lists %v; want %v", q, got, want)
		}
	}
}

// TestInstalledRowsMatchTheCanvas (installed-07, -09, -11, -15, -17): the
// F1 rows carry the health class, the kind icons, the provider tip, the two
// lines of the source cell, and no "path ."; the empty state offers a
// button.
func TestInstalledRowsMatchTheCanvas(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	main := mainOf(s.get(t, "/installed").body)
	row := func(name string) string {
		return between(main, `<a class="row-link" href="`+name, "</tr>")
	}
	pkg := row("/packages/pkg/podinfo")
	for _, want := range []string{
		`<span class="src mono">OCIRepository pkg/podinfo-release</span><span class="ver block">no revision recorded</span>`,
		`kind-icon`, `>Package</span>`,
	} {
		if !strings.Contains(pkg, want) {
			t.Errorf("the package row lacks %s:\n%s", want, pkg)
		}
	}
	if strings.Contains(pkg, "path .") || strings.Contains(pkg, "path") && strings.Contains(pkg, "· path") {
		t.Errorf("the package row shows an empty path:\n%s", pkg)
	}
	prov := row("/instances/default/backup-provider")
	for _, want := range []string{
		`<span class="prov prov-active">`, `<span class="vh">standing: active</span>`,
		`role="tooltip">Registration default.backup-provider is accepted and active.`,
	} {
		if !strings.Contains(prov, want) {
			t.Errorf("the provider row lacks %s:\n%s", want, prov)
		}
	}
	// The F1 instances are healthy: no row is tinted as degraded.
	if strings.Contains(main, "row-degraded") {
		t.Error("a healthy F1 row is tinted as degraded")
	}
	if !strings.Contains(row("/instances/web/web"), `<td data-label="Owner">cli</td>`) {
		t.Error("web/web is not owned by the cli")
	}
	empty := mainOf(s.get(t, "/installed?q=zzz").body)
	if !strings.Contains(empty, `<p class="empty-title">Nothing matches these filters</p>`) ||
		!strings.Contains(empty, `<a class="btn" href="/installed" data-filters-explicit data-clear-filters="installed">Clear filters</a>`) {
		t.Errorf("the empty state lacks its title or its button:\n%s", between(empty, `class="empty"`, "</div>"))
	}
}

// TestInstalledRowOfAPackageWithARevision (installed-15): a package that
// recorded a source artifact shows the digest as sha256 and its first eight
// hex characters, the revision in the title, and its path when it has one.
func TestInstalledRowOfAPackageWithARevision(t *testing.T) {
	row := packageRow(&v1.PackageSummary{
		Ref: v1.ObjectRef{Namespace: "pkg", Name: "x"}, Path: "./deploy",
		Source:         v1.SourceRef{Kind: "OCIRepository", Name: "x-release"},
		SourceArtifact: &v1.SourceArtifact{Revision: "v1.2.3@sha256:abcdef0123456789", Digest: "sha256:abcdef0123456789"},
	})
	if got := row.Revision(); got.Text != "sha256:abcdef01" || got.Title != "v1.2.3@sha256:abcdef0123456789" {
		t.Errorf("revision line = %+v", got)
	}
	if !row.ShowPath() {
		t.Error("a path other than . is not shown")
	}
	row.Path = "."
	if row.ShowPath() {
		t.Error("the path . is shown")
	}
	row.SourceArtifact = &v1.SourceArtifact{Revision: "main@sha1:aa"}
	if got := row.Revision(); got.Text != "main@sha1:aa" {
		t.Errorf("a revision with no digest reads %+v", got)
	}
	row.Health.State = "Degraded"
	if got := row.RowClass(); got != "row-degraded" {
		t.Errorf("a Degraded row has class %q", got)
	}
	row.Health.State = "Odd"
	if got := row.RowClass(); got != "row-unknown" {
		t.Errorf("a health the portal does not know has class %q", got)
	}
}

// TestPlatformNamesTheControllerVersion: the version the Platform's
// status.controllerVersion records reaches the API document, the platform
// graph's platform node and the page. The F1 capture predates the rename (its
// Platform wrote status.operatorVersion), so the goldens carry no version.
func TestPlatformNamesTheControllerVersion(t *testing.T) {
	const want = "v1.0.0-beta.9"
	objs := apitest.F1(t)
	for _, o := range objs {
		if o.GetKind() == "Platform" {
			if err := unstructured.SetNestedField(o.Object, want, "status", "controllerVersion"); err != nil {
				t.Fatal(err)
			}
		}
	}
	s := newSite(t, objs, apitest.AllowAll)
	res := s.get(t, defaultAPIBase+"/platform")
	var doc v1.Platform
	if err := json.Unmarshal([]byte(res.body), &doc); res.status != http.StatusOK || err != nil {
		t.Fatalf("platform document: status %d, %v", res.status, err)
	}
	if doc.ControllerVersion != want {
		t.Errorf("platform document controllerVersion = %q, want %s", doc.ControllerVersion, want)
	}
	res = s.get(t, defaultAPIBase+"/platform/graph")
	var g v1.Graph
	if err := json.Unmarshal([]byte(res.body), &g); res.status != http.StatusOK || err != nil {
		t.Fatalf("platform graph: status %d, %v", res.status, err)
	}
	var got []string
	for _, n := range g.Nodes {
		if n.Platform != nil {
			got = append(got, n.Platform.ControllerVersion)
		}
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("platform graph platform controllerVersion = %q, want one node with %s", got, want)
	}
	if main := mainOf(s.get(t, "/").body); !strings.Contains(main, `<dt>Controller</dt><dd class="mono">`+want+`</dd>`) {
		t.Errorf("the platform page does not name the controller version:\n%s", main)
	}
}

// TestUnknownOwnerIsShownAsWritten: a spec.owner the portal does not know,
// here the value the CRD of an earlier release allowed, reaches the API
// document as written and the pages mark it unknown. It never reads as the
// controller.
func TestUnknownOwnerIsShownAsWritten(t *testing.T) {
	const old = "operator"
	objs := apitest.F1(t)
	found := false
	for _, o := range objs {
		if o.GetKind() == "ModuleInstance" && o.GetNamespace() == "default" && o.GetName() == "podinfo" {
			if err := unstructured.SetNestedField(o.Object, old, "spec", "owner"); err != nil {
				t.Fatal(err)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("F1 holds no ModuleInstance default/podinfo")
	}
	s := newSite(t, objs, apitest.AllowAll)
	res := s.get(t, defaultAPIBase+"/instances/default/podinfo")
	var doc v1.Instance
	if err := json.Unmarshal([]byte(res.body), &doc); res.status != http.StatusOK || err != nil {
		t.Fatalf("instance document: status %d, %v", res.status, err)
	}
	if doc.Owner != old {
		t.Errorf("instance document owner = %q, want %q as written", doc.Owner, old)
	}
	marked := `<span class="mono">` + old + `</span> <span class="muted">(unknown)</span>`
	if main := mainOf(s.get(t, "/installed?namespace=default").body); !strings.Contains(main, `<td data-label="Owner">`+marked+`</td>`) {
		t.Errorf("the Installed list does not mark the owner unknown:\n%s", main)
	}
	main := mainOf(s.get(t, "/instances/default/podinfo").body)
	if !strings.Contains(main, `<dd>`+old+` (unknown)</dd>`) {
		t.Errorf("the instance page does not mark the owner unknown:\n%s", main)
	}
	if strings.Contains(main, `<dd>the controller</dd>`) {
		t.Error("the instance page reads an unknown owner as the controller")
	}
}
