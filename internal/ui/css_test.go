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
