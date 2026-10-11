package ui

import (
	"regexp"
	"strings"
	"testing"
)

// This file holds the structure of static/portal.css that the flat look and
// the tokens of portal:D19 depend on: every tone and kind token in all three
// theme blocks, no page grid or glow, no shadow or corner mark on a panel, no
// entrance animation, and no text drawn in a border token on its fill.

var (
	cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssRule    = regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`)
	cssDecl    = regexp.MustCompile(`([a-z-]+)\s*:\s*([^;]+)`)
)

// cssRuleOf is one rule of a stylesheet: its selector and its declarations.
type cssRuleOf struct {
	selector string
	decls    map[string]string
}

// parseCSS returns the flat rules of css, those inside a media query
// included. It reads only what portal.css holds: no nested rules.
func parseCSS(css string) []cssRuleOf {
	css = cssComment.ReplaceAllString(css, "")
	matches := cssRule.FindAllStringSubmatch(css, -1)
	rules := make([]cssRuleOf, 0, len(matches))
	for _, m := range matches {
		r := cssRuleOf{selector: strings.Join(strings.Fields(m[1]), " "), decls: map[string]string{}}
		for _, d := range cssDecl.FindAllStringSubmatch(m[2], -1) {
			r.decls[d[1]] = strings.TrimSpace(d[2])
		}
		rules = append(rules, r)
	}
	return rules
}

// borderTokenText returns the rules of css that draw text in a tone's border
// token on that tone's fill: color is var(--<tone>) while the background is
// var(--<tone>-bg) or var(--<tone>-tint). Text takes the -ink token.
func borderTokenText(css string) []string {
	var bad []string
	for _, r := range parseCSS(css) {
		fg := r.decls["color"]
		bg := r.decls["background"] + " " + r.decls["background-color"]
		for _, tone := range append(append([]string{}, tones...), "locked") {
			if fg != "var(--"+tone+")" {
				continue
			}
			if strings.Contains(bg, "var(--"+tone+"-bg)") || strings.Contains(bg, "var(--"+tone+"-tint)") {
				bad = append(bad, r.selector+" { color: "+fg+" } on its "+tone+" fill")
			}
		}
	}
	return bad
}

func TestPortalCSSHoldsEveryToneAndKindToken(t *testing.T) {
	blocks, err := tokenBlocks(readPortalCSS(t))
	if err != nil {
		t.Fatal(err)
	}
	want := make([]string, 0, 4*len(tones)+len(lockedTokens)+len(kindTokens)+1)
	for _, tone := range tones {
		for _, suffix := range []string{"", "-ink", "-bg", "-tint"} {
			want = append(want, "--"+tone+suffix)
		}
	}
	want = append(want, lockedTokens...)
	want = append(want, kindTokens...)
	want = append(want, "--line-soft")
	for _, theme := range []string{"light", "dark", "dark-chosen"} {
		for _, token := range want {
			if _, ok := blocks[theme][token]; !ok {
				t.Errorf("token %s is missing from the %s block of portal.css", token, theme)
			}
		}
	}
}

func TestPortalCSSIsFlat(t *testing.T) {
	css := readPortalCSS(t)
	rules := parseCSS(css)
	for _, r := range rules {
		if r.selector == "body" {
			for _, prop := range []string{"background-image", "background-size"} {
				if v, ok := r.decls[prop]; ok {
					t.Errorf("body sets %s: %s; the page background is one flat color", prop, v)
				}
			}
		}
		if r.selector == ".panel::after" || r.selector == "h2::before" {
			t.Errorf("portal.css has a rule for %s; panels carry no corner mark and headings no marker", r.selector)
		}
		for _, sel := range []string{".panel", ".summary > .card", ".pf-summary > .card"} {
			if r.selector == sel {
				if v, ok := r.decls["box-shadow"]; ok {
					t.Errorf("%s sets box-shadow: %s; panels and cards are flat", sel, v)
				}
			}
		}
	}
	if strings.Contains(css, "--shadow") {
		t.Error("portal.css still names --shadow, which the flat look removed")
	}
	if strings.Contains(css, "@keyframes rise") {
		t.Error("portal.css has @keyframes rise; nothing animates in on page load")
	}
	for _, r := range rules {
		if strings.Contains(r.selector, ".reveal") {
			t.Errorf("portal.css has a rule on .reveal (%s); the entrance animation is gone", r.selector)
		}
	}
}

func TestPortalCSSDrawsNoTextInABorderToken(t *testing.T) {
	for _, line := range borderTokenText(readPortalCSS(t)) {
		t.Errorf("%s; text takes the -ink token", line)
	}
	// The check can fail: a rule that does this is named.
	bad := borderTokenText(".prov-active { border-color: var(--healthy); color: var(--healthy); background: var(--healthy-bg); }")
	if len(bad) != 1 || !strings.Contains(bad[0], ".prov-active") {
		t.Errorf("the check did not name a text-in-border-token rule: %v", bad)
	}
	if got := borderTokenText(".ok { color: var(--healthy-ink); background: var(--healthy-bg); border-color: var(--healthy); }"); len(got) != 0 {
		t.Errorf("the check flagged an ink rule: %v", got)
	}
}

// selectorList splits a rule's selector into its comma-separated parts.
func selectorList(selector string) []string {
	parts := strings.Split(selector, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// ruleFor returns the declarations of the first rule whose selector list
// holds sel exactly.
func ruleFor(rules []cssRuleOf, sel string) (map[string]string, bool) {
	for _, r := range rules {
		for _, s := range selectorList(r.selector) {
			if s == sel {
				return r.decls, true
			}
		}
	}
	return nil, false
}

// forcedColorsRules returns the rules inside the @media (forced-colors:
// active) blocks of css.
func forcedColorsRules(css string) []cssRuleOf {
	css = cssComment.ReplaceAllString(css, "")
	var out []cssRuleOf
	const open = "@media (forced-colors: active)"
	for {
		i := strings.Index(css, open)
		if i < 0 {
			return out
		}
		start := strings.Index(css[i:], "{")
		if start < 0 {
			return out
		}
		start += i + 1
		depth, end := 1, start
		for ; end < len(css) && depth > 0; end++ {
			switch css[end] {
			case '{':
				depth++
			case '}':
				depth--
			}
		}
		out = append(out, parseCSS(css[start:end-1])...)
		css = css[end:]
	}
}

// hueOf is the hue each state class draws (portal:D19:R3): the Applied axis
// keeps the controller's words, and an unknown Applied value reads neutral.
var (
	appliedHue = map[string]string{
		"applied": "applied", "reconciling": "progressing", "failed": "degraded", "stalled": "degraded",
		"suspended": "neutral", "managedexternally": "neutral", "unknown": "neutral",
	}
	healthHue = map[string]string{
		"healthy": "healthy", "progressing": "progressing", "degraded": "degraded", "missing": "missing", "unknown": "unknown",
	}
)

func TestPortalCSSMapsEveryStateClassToAHue(t *testing.T) {
	rules := parseCSS(readPortalCSS(t))
	check := func(class, hue string) {
		t.Helper()
		d, ok := ruleFor(rules, class)
		if !ok {
			t.Errorf("portal.css has no rule for %s, so its badge takes no hue", class)
			return
		}
		for variable, want := range map[string]string{
			"--c": "var(--" + hue + ")", "--c-ink": "var(--" + hue + "-ink)", "--c-bg": "var(--" + hue + "-bg)", "--c-tint": "var(--" + hue + "-tint)",
		} {
			if d[variable] != want {
				t.Errorf("%s sets %s to %q, want %q", class, variable, d[variable], want)
			}
		}
	}
	for _, v := range appliedValues {
		slug := strings.ToLower(v)
		check(".applied-"+slug, appliedHue[slug])
	}
	for _, v := range healthValues {
		slug := strings.ToLower(v)
		check(".health-"+slug, healthHue[slug])
	}
	for _, hue := range append(append([]string{}, tones...), "locked") {
		check(".hue-"+hue, hue)
	}
}

// TestPortalCSSDrawsOneBadgeShape (portal:D19:R3): both axes draw the one
// badge shape from the hue variables, with no stamp prefix and no dot.
func TestPortalCSSDrawsOneBadgeShape(t *testing.T) {
	rules := parseCSS(readPortalCSS(t))
	for _, class := range []string{".applied", ".health"} {
		d := cssDeclsWith(rules, class, "border")
		if d["border"] != "1.5px solid var(--c)" || d["color"] != "var(--c-ink)" || d["background"] != "var(--c-bg)" || d["border-radius"] != "5px" {
			t.Errorf("%s does not draw the one badge shape from the hue variables: %v", class, d)
		}
	}
	for _, r := range rules {
		for _, s := range selectorList(r.selector) {
			if s == ".applied::before" || s == ".health .dot" || s == ".health-partial .dot" {
				t.Errorf("portal.css still has a rule for %s; the badges carry no stamp prefix or dot", s)
			}
		}
	}
}

// cssDeclsWith returns the declarations of the first rule whose selector list
// holds sel and that sets prop.
func cssDeclsWith(rules []cssRuleOf, sel, prop string) map[string]string {
	for _, r := range rules {
		for _, s := range selectorList(r.selector) {
			if s == sel {
				if _, ok := r.decls[prop]; ok {
					return r.decls
				}
			}
		}
	}
	return nil
}

func TestPortalCSSDrawsUnderlineTabs(t *testing.T) {
	rules := parseCSS(readPortalCSS(t))
	cur, ok := ruleFor(rules, `.tabs a[aria-current="page"]`)
	if !ok {
		t.Fatal("portal.css has no rule for the current tab")
	}
	if cur["border-bottom"] != "3px solid var(--accent)" || cur["color"] != "var(--ink)" {
		t.Errorf("the current tab draws %v, want a 3px accent underline in ink", cur)
	}
	base, _ := ruleFor(rules, ".tabs a")
	if base["min-height"] != "44px" || base["color"] != "var(--muted)" || base["border-bottom"] != "3px solid transparent" {
		t.Errorf("a tab draws %v, want 44px high, muted, with a 3px transparent underline", base)
	}
	large, _ := ruleFor(rules, ".tabs-lg a")
	if large["font-size"] != "18px" || large["min-height"] != "48px" {
		t.Errorf("a large tab draws %v, want 18px and 48px", large)
	}
	if _, ok := ruleFor(rules, ".vh"); !ok {
		t.Error("portal.css has no .vh utility for the tab regions' headings")
	}
}

func TestPortalCSSHoldsAForcedColoursBorder(t *testing.T) {
	check := func(css string) []string {
		var bad []string
		rules := forcedColorsRules(css)
		system := func(v string) bool {
			switch v {
			case "CanvasText", "ButtonText", "Highlight", "LinkText", "GrayText":
				return true
			}
			return false
		}
		for _, sel := range []string{".applied", ".health", ".health-partial"} {
			d, ok := ruleFor(rules, sel)
			if !ok || !system(d["border-color"]) {
				bad = append(bad, sel+" sets no border color from a system color in the forced-colors block")
			}
		}
		if d, _ := ruleFor(rules, ".health-partial"); d["border-style"] != "dashed" {
			bad = append(bad, ".health-partial keeps no dashed border in the forced-colors block")
		}
		return bad
	}
	for _, line := range check(readPortalCSS(t)) {
		t.Error(line)
	}
	// The check can fail: a block without the partial rule is named.
	if got := check("@media (forced-colors: active) { .applied, .health { border-color: CanvasText; } }"); len(got) != 2 {
		t.Errorf("the check did not name the missing partial rule: %v", got)
	}
	if got := check("a { color: red; }"); len(got) != 4 {
		t.Errorf("the check did not name a missing block: %v", got)
	}
}

func TestPortalCSSHasNoPulse(t *testing.T) {
	css := readPortalCSS(t)
	if strings.Contains(css, "pulse") {
		t.Error("portal.css still names pulse; the live dot is steady")
	}
}
