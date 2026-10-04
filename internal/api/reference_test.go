package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/open-platform-model/opm-portal/internal/stream"
)

const readAPIReferencePath = "../../docs/site/reference/portal/read-api.md"

// referenceRow matches a resource row of the reference page's table:
// | `GET /clusters/{cluster}/instances` | ... |
var referenceRow = regexp.MustCompile("(?m)^\\| `GET (/[^`]*)` \\|")

// TestReadAPIReferenceListsEveryPath: the published read API reference names
// exactly the OpenAPI document's paths, so a route added to or removed from
// the contract fails here until the page follows.
func TestReadAPIReferenceListsEveryPath(t *testing.T) {
	doc := loadOpenAPI(t)
	raw, err := os.ReadFile(readAPIReferencePath)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, m := range referenceRow.FindAllStringSubmatch(string(raw), -1) {
		if listed[m[1]] {
			t.Errorf("%s lists %s twice", readAPIReferencePath, m[1])
		}
		listed[m[1]] = true
	}
	if len(listed) == 0 {
		t.Fatalf("%s has no resource rows (| `GET <path>` | ...)", readAPIReferencePath)
	}
	for _, p := range slices.Sorted(maps.Keys(doc.Paths)) {
		if !listed[p] {
			t.Errorf("%s does not list %s, which openapi/v1alpha1.yaml has", readAPIReferencePath, p)
		}
	}
	for _, p := range slices.Sorted(maps.Keys(listed)) {
		if _, ok := doc.Paths[p]; !ok {
			t.Errorf("%s lists %s, which openapi/v1alpha1.yaml does not have", readAPIReferencePath, p)
		}
	}
}

// referenceSection returns the body of the reference page's "## heading"
// section, up to the next section.
func referenceSection(t *testing.T, heading string) string {
	t.Helper()
	raw, err := os.ReadFile(readAPIReferencePath)
	if err != nil {
		t.Fatal(err)
	}
	_, body, ok := strings.Cut(string(raw), "\n## "+heading+"\n")
	if !ok {
		t.Fatalf("%s has no %q section", readAPIReferencePath, heading)
	}
	body, _, _ = strings.Cut(body, "\n## ")
	return body
}

// codeRow matches a row of the problem code table: | 400 | `bad_request` | ...
var codeRow = regexp.MustCompile("(?m)^\\| \\d{3} \\| `([a-z_]+)` \\|")

// TestReadAPIReferenceListsEveryProblemCode: the reference page's problem
// code table names exactly the Code constants of api/v1alpha1, read from the
// package's source so a constant added there fails here until the page
// follows, and exactly the codes of the OpenAPI document's Problem.code enum.
func TestReadAPIReferenceListsEveryProblemCode(t *testing.T) {
	listed := map[string]bool{}
	for _, m := range codeRow.FindAllStringSubmatch(referenceSection(t, "Problem codes"), -1) {
		listed[m[1]] = true
	}
	for source, want := range map[string][]string{
		"api/v1alpha1":          problemCodeConstants(t),
		"openapi/v1alpha1.yaml": openAPIProblemCodes(t),
	} {
		for _, c := range want {
			if !listed[c] {
				t.Errorf("%s does not list the problem code %s, which %s has", readAPIReferencePath, c, source)
			}
		}
		for _, c := range slices.Sorted(maps.Keys(listed)) {
			if !slices.Contains(want, c) {
				t.Errorf("%s lists the problem code %s, which %s does not have", readAPIReferencePath, c, source)
			}
		}
	}
}

// openAPIProblemCodes returns the OpenAPI document's Problem.code enum.
func openAPIProblemCodes(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(openAPIPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Components struct {
			Schemas struct {
				Problem struct {
					Properties struct {
						Code struct {
							Enum []string `json:"x-extensible-enum"`
						} `json:"code"`
					} `json:"properties"`
				} `json:"Problem"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	codes := doc.Components.Schemas.Problem.Properties.Code.Enum
	if len(codes) == 0 {
		t.Fatal("openapi/v1alpha1.yaml has no Problem.code enum")
	}
	return codes
}

// problemCodeConstants returns the values of the constants named Code* in
// api/v1alpha1's non-test source files.
func problemCodeConstants(t *testing.T) []string {
	t.Helper()
	const dir = "../../api/v1alpha1"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if !strings.HasPrefix(name.Name, "Code") {
						continue
					}
					if i >= len(vs.Values) {
						t.Fatalf("the constant %s in %s has no value of its own", name.Name, e.Name())
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						t.Fatalf("the constant %s in %s is not a string literal", name.Name, e.Name())
					}
					s, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatal(err)
					}
					codes = append(codes, s)
				}
			}
		}
	}
	if len(codes) == 0 {
		t.Fatalf("%s declares no Code constants", dir)
	}
	return codes
}

// topicRow matches a row of the topic table: | `instance:<namespace>/<name>` | ...
var topicRow = regexp.MustCompile("(?m)^\\| `([a-z][^`]*)` \\|")

// placeholder matches a <name> in a documented topic.
var placeholder = regexp.MustCompile(`<[a-z]+>`)

// unservedTopicKinds are the stream.Kind values the stream refuses: a lone
// registration has no GET, so only its events topic is served.
var unservedTopicKinds = map[stream.Kind]bool{stream.KindRegistration: true}

// TestReadAPIReferenceListsEveryTopicKind: every topic row of the reference
// page parses, and every topic kind the stream serves has a row. The kinds
// are read from the Kind constants in internal/stream, so a kind added there
// fails here until the page or unservedTopicKinds names it.
func TestReadAPIReferenceListsEveryTopicKind(t *testing.T) {
	covered := map[stream.Kind]bool{}
	for _, m := range topicRow.FindAllStringSubmatch(referenceSection(t, "Change stream topics"), -1) {
		topic, err := stream.ParseTopic(placeholder.ReplaceAllString(m[1], "x"))
		if err != nil {
			t.Errorf("%s lists the topic %s, which does not parse: %v", readAPIReferencePath, m[1], err)
			continue
		}
		covered[topic.Kind()] = true
	}
	for _, k := range streamKinds(t) {
		switch {
		case unservedTopicKinds[k] && covered[k]:
			t.Errorf("%s lists a %s topic, which the stream does not serve", readAPIReferencePath, k)
		case !unservedTopicKinds[k] && !covered[k]:
			t.Errorf("%s lists no %s topic", readAPIReferencePath, k)
		}
	}
}

// streamKinds returns the values of the Kind constants in internal/stream.
func streamKinds(t *testing.T) []stream.Kind {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "../stream/topic.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []stream.Kind
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "Kind" {
				continue
			}
			for _, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("a Kind constant in internal/stream/topic.go is not a string literal")
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				kinds = append(kinds, stream.Kind(s))
			}
		}
	}
	if len(kinds) == 0 {
		t.Fatal("internal/stream/topic.go declares no Kind constants")
	}
	return kinds
}
