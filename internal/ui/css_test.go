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

// svgTextSelector matches a selector part that draws SVG text: the graph's
// node labels, sublabels and titles. SVG text takes its color from fill.
var svgTextSelector = regexp.MustCompile(`-(label|sub|title|text)\b|\btext\b|\btspan\b`)

// borderTokenText returns the rules of css that draw text in a border token
// where the token's own fill sits behind it (portal:D19:R7). Text takes the
// -ink token. Three shapes are named:
//   - color is var(--<tone>) while the background is var(--<tone>-bg) or
//     var(--<tone>-tint) in the same rule;
//   - color is var(--c), the hue variable that holds the border token, in any
//     rule: every hue sets --c-bg and --c-tint, so text on one needs --c-ink;
//   - an SVG text rule (a label or sublabel) sets fill to a tone's border token
//     or to var(--c): the node box it sits on is filled in the -bg token.
func borderTokenText(css string) []string {
	var bad []string
	toneList := append(append([]string{}, tones...), "locked")
	for _, r := range parseCSS(css) {
		fg := r.decls["color"]
		bg := r.decls["background"] + " " + r.decls["background-color"]
		if fg == "var(--c)" {
			bad = append(bad, r.selector+" { color: var(--c) } on a hue fill")
		}
		for _, tone := range toneList {
			if fg == "var(--"+tone+")" && (strings.Contains(bg, "var(--"+tone+"-bg)") || strings.Contains(bg, "var(--"+tone+"-tint)")) {
				bad = append(bad, r.selector+" { color: "+fg+" } on its "+tone+" fill")
			}
		}
		fill := r.decls["fill"]
		if fill == "" || !svgTextSelector.MatchString(r.selector) {
			continue
		}
		if fill == "var(--c)" {
			bad = append(bad, r.selector+" { fill: var(--c) } draws SVG text in the hue's border token")
		}
		for _, tone := range toneList {
			if fill == "var(--"+tone+")" {
				bad = append(bad, r.selector+" { fill: "+fill+" } draws SVG text in the "+tone+" border token")
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

// shadowAllowed are the selectors that may carry a box-shadow: the compact header's
// lift, the theme menu and the graph hover card (floating layers), the count's hover
// ring, and the two steps of the settle ring that portal:D19 keeps. Any other rule
// with a shadow is a card, panel or block that is no longer flat.
var shadowAllowed = map[string]bool{
	".masthead.compact": true, ".theme-list": true, ".node-card": true, ".count:hover": true, "0%": true, "100%": true,
}

// flatProblems returns what breaks the flat look (portal:D19:R1) in rules: a page
// background that is anything but a flat color (a gradient or an image, in any
// background property including the shorthand), a corner mark, or a box-shadow
// outside the allowlist.
func flatProblems(rules []cssRuleOf) []string {
	var bad []string
	for _, r := range rules {
		if r.selector == "body" {
			for _, prop := range []string{"background", "background-image", "background-size"} {
				v, ok := r.decls[prop]
				if !ok {
					continue
				}
				if prop != "background" || strings.Contains(v, "gradient(") || strings.Contains(v, "url(") {
					bad = append(bad, "body sets "+prop+": "+v+"; the page background is one flat color")
				}
			}
		}
		if r.selector == ".panel::after" || r.selector == "h2::before" {
			bad = append(bad, "portal.css has a rule for "+r.selector+"; panels carry no corner mark and headings no marker")
		}
		if v, ok := r.decls["box-shadow"]; ok && !shadowAllowed[r.selector] {
			bad = append(bad, r.selector+" sets box-shadow: "+v+"; panels, cards and blocks are flat")
		}
	}
	return bad
}

func TestPortalCSSFlatCheckCanFail(t *testing.T) {
	for name, css := range map[string]string{
		"a grid in the background shorthand": "body { background: linear-gradient(var(--line) 1px, transparent 1px) 0 0 / 24px 24px, var(--bg); }",
		"an image in the shorthand":          "body { background: url(grid.svg) var(--bg); }",
		"a background-image":                 "body { background-image: linear-gradient(red, blue); }",
		"a shadow on .card":                  ".card { box-shadow: 0 2px 4px #0003; }",
		"a shadow on .stat":                  ".stat { box-shadow: 0 2px 4px #0003; }",
		"a shadow on .entry":                 ".entry { box-shadow: 0 2px 4px #0003; }",
		"a shadow on .state-block":           ".state-block { box-shadow: 0 2px 4px #0003; }",
		"a shadow on .panel":                 ".panel { box-shadow: 0 2px 4px #0003; }",
		"a corner mark":                      ".panel::after { content: ''; }",
	} {
		if got := flatProblems(parseCSS(css)); len(got) != 1 {
			t.Errorf("the flat check did not name %s once: %v", name, got)
		}
	}
	if got := flatProblems(parseCSS("body { background: var(--bg); } .theme-list { box-shadow: 0 12px 28px -10px #0004; }")); len(got) != 0 {
		t.Errorf("the flat check flagged a flat body and a floating menu: %v", got)
	}
}

func TestPortalCSSIsFlat(t *testing.T) {
	css := readPortalCSS(t)
	rules := parseCSS(css)
	for _, line := range flatProblems(rules) {
		t.Error(line)
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
	// The check can fail: each shape of the defect is named, and ink is not.
	for name, css := range map[string]string{
		"text in a tone's token on its fill": ".prov-active { border-color: var(--healthy); color: var(--healthy); background: var(--healthy-bg); }",
		"text in the hue variable":           ".sb-reasons a, .sb-links a { color: var(--c); }",
		"SVG text in the locked token":       ".node.locked .node-label, .node.locked .node-sub { fill: var(--locked); }",
		"SVG text in the hue variable":       ".node-sub { fill: var(--c); }",
	} {
		if got := borderTokenText(css); len(got) < 1 {
			t.Errorf("the check did not name %s: %v", name, got)
		}
	}
	for name, css := range map[string]string{
		"an ink rule":         ".ok { color: var(--healthy-ink); background: var(--healthy-bg); border-color: var(--healthy); }",
		"a hue ink rule":      ".sb-word { color: var(--c-ink); }",
		"SVG text in ink":     ".node.locked .node-label { fill: var(--locked-ink); }",
		"a filled node shape": ".node.locked .node-box { fill: var(--locked-bg); stroke: var(--locked); } .node.locked .node-rail { fill: var(--locked); }",
	} {
		if got := borderTokenText(css); len(got) != 0 {
			t.Errorf("the check flagged %s: %v", name, got)
		}
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
		for _, sel := range []string{".applied", ".health", ".health-partial", ".state-block", ".tipbox"} {
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
	// The check can fail: a block without the partial, state block and tip rules is named.
	if got := check("@media (forced-colors: active) { .applied, .health { border-color: CanvasText; } }"); len(got) != 4 {
		t.Errorf("the check did not name the missing partial, state block and tip rules: %v", got)
	}
	if got := check("a { color: red; }"); len(got) != 6 {
		t.Errorf("the check did not name a missing block: %v", got)
	}
}

// Forced colors paint a transparent border, so without this block every tab shows the 3 px
// underline and only the font weight marks the current one. A page-color border would instead
// cut the strip's 1 px baseline under each tab (the tabs overlap it by 1 px): the other tabs
// draw no bottom border and take its 3 px as padding.
func TestPortalCSSMarksOnlyTheCurrentTabInForcedColours(t *testing.T) {
	check := func(css string) []string {
		var bad []string
		rules := forcedColorsRules(css)
		if d, ok := ruleFor(rules, `.tabs a:not([aria-current="page"])`); !ok || d["border-bottom-style"] != "none" || d["padding-bottom"] != "13px" {
			bad = append(bad, "a tab that is not current keeps its bottom border or loses its 3 px of padding in the forced-colors block")
		}
		if d, ok := ruleFor(rules, `.tabs-lg a:not([aria-current="page"])`); !ok || d["padding-bottom"] != "15px" {
			bad = append(bad, "a large tab that is not current loses its 3 px of padding in the forced-colors block")
		}
		if d, ok := ruleFor(rules, `.tabs a[aria-current="page"]`); !ok || d["border-bottom-color"] != "CanvasText" {
			bad = append(bad, "the current tab keeps no CanvasText underline in the forced-colors block")
		}
		if d, ok := ruleFor(rules, ".tabs a"); ok && d["border-bottom-color"] != "" {
			bad = append(bad, ".tabs a paints a page-color border over the strip's baseline in the forced-colors block")
		}
		return bad
	}
	for _, line := range check(readPortalCSS(t)) {
		t.Error(line)
	}
	if got := check("@media (forced-colors: active) { .applied { border-color: CanvasText; } }"); len(got) != 3 {
		t.Errorf("the check did not name the three missing tab rules: %v", got)
	}
	if got := check("@media (forced-colors: active) { .tabs a { border-bottom-color: Canvas; } .tabs a[aria-current=\"page\"] { border-bottom-color: CanvasText; } }"); len(got) < 3 {
		t.Errorf("the check did not name the old page-color border: %v", got)
	}
}

// The page column is 1840 px at most (portal:D19), on a 1920 px viewport as on a wider one:
// the main grid and the footer share the width, so the footer stays under the content.
func TestPortalCSSHoldsTheWideColumn(t *testing.T) {
	rules := parseCSS(readPortalCSS(t))
	for _, sel := range []string{"main", ".foot"} {
		d, ok := ruleFor(rules, sel)
		if !ok || d["max-width"] != "1840px" {
			t.Errorf("%s has max-width %q, want 1840px", sel, d["max-width"])
		}
		if d["margin"] != "0 auto" && sel == "main" {
			t.Errorf("main has margin %q, want 0 auto, so the column is centred", d["margin"])
		}
	}
	if d, _ := ruleFor(parseCSS("main { max-width: 1200px; margin: 0 auto; }"), "main"); d["max-width"] == "1840px" {
		t.Error("the column check cannot tell 1200px from 1840px")
	}
}

// --line-soft is the canvas's hairline between table rows; a token with no reader is dead.
func TestPortalCSSReadsLineSoft(t *testing.T) {
	d := cssDeclsWith(parseCSS(readPortalCSS(t)), ".table td", "border-bottom-color")
	if d["border-bottom-color"] != "var(--line-soft)" {
		t.Errorf(".table td border-bottom-color = %q, want var(--line-soft)", d["border-bottom-color"])
	}
}

// A tab with no count keeps its count span, so a live refresh can empty it; CSS hides it.
func TestPortalCSSHidesAnEmptyTabCount(t *testing.T) {
	d, ok := ruleFor(parseCSS(readPortalCSS(t)), ".tab-n:empty")
	if !ok || d["display"] != "none" {
		t.Errorf(".tab-n:empty must set display: none, got %v", d)
	}
}

func TestPortalCSSHasNoPulse(t *testing.T) {
	css := readPortalCSS(t)
	if strings.Contains(css, "pulse") {
		t.Error("portal.css still names pulse; the live dot is steady")
	}
}

func TestPortalCSSDrawsTheStateBlock(t *testing.T) {
	rules := parseCSS(readPortalCSS(t))
	d, ok := ruleFor(rules, ".state-block")
	if !ok {
		t.Fatal("portal.css has no .state-block rule")
	}
	for prop, want := range map[string]string{"border": "2px solid var(--c)", "background": "var(--c-tint)", "border-radius": "12px"} {
		if d[prop] != want {
			t.Errorf(".state-block %s = %q, want %q", prop, d[prop], want)
		}
	}
	if w, _ := ruleFor(rules, ".sb-word"); w["color"] != "var(--c-ink)" || w["font-size"] != "26px" {
		t.Errorf(".sb-word draws the state in the tone's ink at 26px, got %v", w)
	}
	minHeight := ""
	for _, r := range rules {
		for _, sel := range selectorList(r.selector) {
			if sel == ".sb-reasons a" && r.decls["min-block-size"] != "" {
				minHeight = r.decls["min-block-size"]
			}
		}
	}
	if minHeight != "28px" {
		t.Errorf(".sb-reasons a keeps a 28px minimum height, got %q", minHeight)
	}
}

// tipRuleProblems returns what is wrong with the tip rules of css: the box
// shows on hover, on focus and when opened, hides when dismissed with that
// hide rule last, touches its trigger, and holds no style the script would
// need to set.
func tipRuleProblems(css string) []string {
	rules := parseCSS(css)
	return append(append(tipShowProblems(rules), tipBoxProblems(rules)...), tipTriggerProblems(rules)...)
}

// tipTriggerProblems checks that the trigger button reads as the words around it: the
// global button rules (a solid ink pill) would otherwise draw it.
func tipTriggerProblems(rules []cssRuleOf) []string {
	d, ok := ruleFor(rules, ".tip .tip-t")
	if !ok {
		return []string{"no .tip .tip-t rule, so the trigger button takes the global button look"}
	}
	var bad []string
	if d["background"] != "none" || d["border"] != "0" || d["color"] != "inherit" {
		bad = append(bad, ".tip .tip-t does not reset the global button look (background none, border 0, color inherit)")
	}
	if _, ok := ruleFor(rules, ".tip .tip-t:hover"); !ok {
		bad = append(bad, ".tip .tip-t:hover is not in the reset, so button:hover paints the trigger")
	}
	return bad
}

// tipShowProblems checks the rules that show and hide the box.
func tipShowProblems(rules []cssRuleOf) []string {
	var bad []string
	shows := map[string]bool{".tip:hover .tipbox": false, ".tip:focus-within .tipbox": false, ".tip.is-open .tipbox": false}
	lastDisplay, lastValue := "", ""
	for _, r := range rules {
		if !strings.Contains(r.selector, ".tipbox") || r.decls["display"] == "" {
			continue
		}
		for _, sel := range selectorList(r.selector) {
			if _, ok := shows[sel]; ok && r.decls["display"] == "block" {
				shows[sel] = true
			}
		}
		lastDisplay, lastValue = r.selector, r.decls["display"]
	}
	for sel, ok := range shows {
		if !ok {
			bad = append(bad, "no rule shows the box with "+sel)
		}
	}
	if lastDisplay != ".tip.is-dismissed .tipbox" || lastValue != "none" {
		bad = append(bad, "the last rule that sets the box's display is "+lastDisplay+" { display: "+lastValue+" }, not .tip.is-dismissed .tipbox { display: none }")
	}
	return bad
}

// tipBoxProblems checks that the box touches its trigger.
func tipBoxProblems(rules []cssRuleOf) []string {
	box, ok := ruleFor(rules, ".tipbox")
	if !ok {
		return []string{"no .tipbox rule"}
	}
	if box["inset-block-start"] != "100%" {
		return []string{".tipbox does not start at the end of its trigger (inset-block-start: 100%)"}
	}
	if m := box["margin-block-start"]; m != "" && m != "0" {
		if br, ok := ruleFor(rules, ".tipbox::before"); !ok || br["inset-block-end"] != "100%" || br["content"] != `""` {
			return []string{".tipbox leaves a gap above it with no ::before bridge, so the pointer drops :hover on the way"}
		}
	}
	return nil
}

func TestPortalCSSDrawsADismissableHoverableTip(t *testing.T) {
	for _, line := range tipRuleProblems(readPortalCSS(t)) {
		t.Error(line)
	}
	// The check can fail: a dismiss rule that is not last, a gap with no bridge and a trigger
	// with no reset are named.
	bad := tipRuleProblems(`.tipbox { display: none; inset-block-start: 100%; margin-block-start: 6px; }
.tip.is-dismissed .tipbox { display: none; }
.tip:hover .tipbox, .tip:focus-within .tipbox, .tip.is-open .tipbox { display: block; }`)
	if len(bad) != 3 {
		t.Errorf("the check did not name the late show rule, the missing bridge and the missing trigger reset: %v", bad)
	}
}
