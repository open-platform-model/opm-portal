package ui

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// hostile is cluster text that would run as markup if a page did not
// escape it.
const hostile = `<script>alert(1)</script><img src=x onerror=alert(2)>`

// poison puts hostile into every free-text field below v: messages,
// reasons, notes and actions, at any depth, and into an annotation.
func poison(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			if s, ok := x.(string); ok && s != "" {
				switch k {
				case "message", "reason", "note", "action":
					t[k] = s + " " + hostile
				}
				continue
			}
			poison(x)
		}
	case []any:
		for _, x := range t {
			poison(x)
		}
	}
}

func hostileF1(t *testing.T) []*unstructured.Unstructured {
	objs := apitest.F1(t)
	for _, o := range objs {
		poison(o.Object)
		a := o.GetAnnotations()
		if a == nil {
			a = map[string]string{}
		}
		a["example.com/note"] = hostile
		o.SetAnnotations(a)
	}
	return objs
}

// TestClusterTextIsEscapedEverywhere: every F1 page and fragment, rendered
// over a capture whose free text is hostile, shows that text escaped and
// never as markup.
func TestClusterTextIsEscapedEverywhere(t *testing.T) {
	s := newSite(t, hostileF1(t), apitest.AllowAll)
	escaped := 0
	for _, p := range f1Pages {
		res := s.get(t, p.path)
		if p.hx {
			res = s.get(t, p.path, htmxRequest)
		}
		if res.status >= 500 {
			t.Errorf("%s: status %d", p.path, res.status)
		}
		for _, raw := range []string{"<script>alert", "<img src=x"} {
			if strings.Contains(res.body, raw) {
				t.Errorf("%s renders cluster text as markup: %q", p.path, raw)
			}
		}
		escaped += strings.Count(res.body, "&lt;script&gt;alert(1)&lt;/script&gt;")
	}
	if escaped == 0 {
		t.Fatal("no page showed the hostile text at all, so the test checks nothing")
	}
}

// TestNoTrustedMarkupInTheUI: the UI never turns a string into a trusted
// template type, and the page script never assigns markup, so escaping
// cannot be switched off for one field without this test failing.
func TestNoTrustedMarkupInTheUI(t *testing.T) {
	goConv := regexp.MustCompile(`template\.(HTML|HTMLAttr|JS|JSStr|URL|CSS|Srcset)\b`)
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		name := f.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if m := goConv.FindString(string(b)); m != "" {
			t.Errorf("%s uses %s, which bypasses escaping", name, m)
		}
	}
	markup := regexp.MustCompile(`\.(innerHTML|outerHTML)\s*\+?=|insertAdjacentHTML\s*\(|document\.write(ln)?\s*\(`)
	style := regexp.MustCompile(`setAttribute\(\s*["']style["']`)
	for _, name := range []string{"static/portal.js", "static/prefs.js"} {
		js, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if m := markup.FindString(string(js)); m != "" {
			t.Errorf("%s uses %s; untrusted text goes in as textContent only", name, m)
		}
		if m := style.FindString(string(js)); m != "" {
			t.Errorf("%s uses %s; the page policy forbids style attributes, so positions go through CSSOM", name, m)
		}
	}
}
