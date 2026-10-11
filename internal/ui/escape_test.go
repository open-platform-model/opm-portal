package ui

import (
	"net/url"
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
	// A hostile query is the viewer's own text: it is shown escaped too.
	q := url.Values{"q": {hostile}, "health": {hostile}, "uses": {hostile}, "pq": {hostile}, "eresource": {hostile}, "reason": {hostile}}.Encode()
	for _, path := range []string{"/installed?" + q, "/?" + q, "/?tab=catalogs&cq=" + url.QueryEscape(hostile),
		"/instances/default/podinfo?tab=resources&" + q, "/instances/default/podinfo?tab=events&resource=" + url.QueryEscape(hostile) + "&" + q} {
		body := s.get(t, path).body
		for _, raw := range []string{"<script>alert", "<img src=x"} {
			if strings.Contains(body, raw) {
				t.Errorf("%s renders the query as markup: %q", path, raw)
			}
		}
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

// ajaxCallsWithoutSelect returns the htmx.ajax calls of js whose options
// do not name select. A call that does not name it inherits the hx-select of
// the element it starts from: the node panel request inherited #app's
// hx-select="#main", found nothing in the panel response and swapped nothing.
// Each call ends at its own closing parenthesis. Its third argument is the
// options: an object literal is read in place, a variable is read from the
// object literal it is assigned in js, and a target string (the short form
// htmx.ajax(verb, path, "#target")) or a missing argument can name no select.
func ajaxCallsWithoutSelect(js string) []string {
	var bad []string
	for from := 0; ; {
		i := strings.Index(js[from:], "htmx.ajax(")
		if i < 0 {
			return bad
		}
		start := from + i
		open := start + len("htmx.ajax")
		end := closing(js, open)
		from = open + 1
		if end < 0 {
			bad = append(bad, js[start:open+1]+" (call never closes)")
			continue
		}
		args := splitArgs(js[open+1 : end])
		options := ""
		if len(args) >= 3 {
			options = strings.TrimSpace(args[2])
			if isIdent(options) {
				options = objectAssignedTo(js, options)
			}
		}
		if !strings.HasPrefix(options, "{") || !strings.Contains(options, "select:") {
			bad = append(bad, strings.SplitN(js[start:end+1], "\n", 2)[0])
		}
	}
}

// closing returns the index of the bracket that closes the one at js[open],
// skipping quoted strings, or -1.
func closing(js string, open int) int {
	depth := 0
	for i := open; i < len(js); i++ {
		switch c := js[i]; c {
		case '"', '\'':
			for i++; i < len(js) && js[i] != c; i++ {
				if js[i] == '\\' {
					i++
				}
			}
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// splitArgs splits a call's argument text at the commas outside any bracket or string.
func splitArgs(text string) []string {
	var args []string
	depth, last := 0, 0
	for i := 0; i < len(text); i++ {
		switch c := text[i]; c {
		case '"', '\'':
			for i++; i < len(text) && text[i] != c; i++ {
				if text[i] == '\\' {
					i++
				}
			}
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, text[last:i])
				last = i + 1
			}
		}
	}
	return append(args, text[last:])
}

func isIdent(s string) bool {
	return regexp.MustCompile(`^[A-Za-z_$][\w$.]*$`).MatchString(s)
}

// objectAssignedTo returns the object literal that name is assigned in js
// ("opts = {...}"), or "" when there is none.
func objectAssignedTo(js, name string) string {
	loc := regexp.MustCompile(`(^|[^\w$.])` + regexp.QuoteMeta(name) + `\s*=\s*\{`).FindStringIndex(js)
	if loc == nil {
		return ""
	}
	open := loc[1] - 1
	end := closing(js, open)
	if end < 0 {
		return ""
	}
	return js[open : end+1]
}

// TestEveryAjaxCallNamesItsSelect: every htmx.ajax call in the page script
// names its swap selection ("unset" for a fragment, "#main" for a page).
func TestEveryAjaxCallNamesItsSelect(t *testing.T) {
	js, err := os.ReadFile("static/portal.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), "htmx.ajax(") {
		t.Fatal("static/portal.js holds no htmx.ajax call; the scan reads nothing")
	}
	for _, call := range ajaxCallsWithoutSelect(string(js)) {
		t.Errorf("static/portal.js: %s does not name select:, so it inherits hx-select from the page around it", call)
	}
	// The scan can fail: each shape of a call without select is flagged, and a later call
	// that names select cannot cover for it.
	named := `htmx.ajax("GET", q, { target: "#a", select: "unset" });`
	for name, bad := range map[string]string{
		"an object without select": `htmx.ajax("GET", p, { target: "#detail", swap: "innerHTML" });`,
		"a target string":          `htmx.ajax("GET", p, "#detail");`,
		"no options":               `htmx.ajax("GET", p);`,
		"an unknown variable":      `htmx.ajax("GET", p, opts);`,
		"a variable without select": `var opts = { target: "#d" };
htmx.ajax("GET", p, opts);`,
	} {
		if got := ajaxCallsWithoutSelect(bad + "\n" + named); len(got) != 1 {
			t.Errorf("%s followed by a call that names select: flagged %d calls, want 1: %v", name, len(got), got)
		}
	}
	for name, good := range map[string]string{
		"an object with select":     `htmx.ajax("GET", p, { target: "#d", select: "#d" });`,
		"a variable with select":    "var opts = { target: \"#d\", select: \"unset\" };\nhtmx.ajax(\"GET\", p, opts);",
		"a path that holds a comma": `htmx.ajax("GET", f(a, b), { select: "x" });`,
	} {
		if got := ajaxCallsWithoutSelect(good); len(got) != 0 {
			t.Errorf("%s: flagged %v, want none", name, got)
		}
	}
}
