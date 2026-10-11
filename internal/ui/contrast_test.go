package ui

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// This file holds the color tokens of static/portal.css to WCAG 2.2 AA
// (1.4.3 text, 1.4.11 control boundaries) without a browser. It reads the
// hex values of the light block and the two dark blocks, and checks the pairs
// in contrastPairs in both themes. It cannot see translucent layers or the
// page's grid and glow; the rendered pages are measured in a browser when a
// token changes (portal:D19:R6).
//
// A UI change that draws text or a control boundary in a new token pair adds
// the pair to contrastPairs.

// contrastPair is one foreground token on one background token.
type contrastPair struct {
	name  string  // "muted on page"
	fg    string  // custom property, "--muted"
	bg    string  // "--bg"
	min   float64 // 4.5 for text, 3 for a control boundary
	theme string  // "light", "dark", or "" for both
}

// pendingPair is a pair that fails today and that a named replacement pair
// closes. The test logs it, and fails when the replacement tokens exist in
// portal.css, so the change that adds them has to move the replacement pair
// into contrastPairs and delete the entry.
type pendingPair struct {
	contrastPair
	closedByFg, closedByBg string
}

const (
	textFloor     = 4.5
	boundaryFloor = 3.0
)

var contrastPairs = buildContrastPairs()

// tones are the status tones of portal.css. Each has a border token
// (--<tone>), a text ink (--<tone>-ink), a badge background (--<tone>-bg) and
// a block tint (--<tone>-tint); locked has the same four, with the ink and the
// tint named in lockedTokens.
var tones = []string{"applied", "healthy", "progressing", "degraded", "unknown", "neutral", "missing"}

// lockedTokens are the locked tone's tokens, which share their values with
// --locked and --locked-bg.
var lockedTokens = []string{"--locked", "--locked-ink", "--locked-bg", "--locked-tint"}

// kindTokens are the Instance and Package chip colors.
var kindTokens = []string{"--kind-instance-bg", "--kind-instance-ink", "--kind-package-ink"}

func buildContrastPairs() []contrastPair {
	fixed := []contrastPair{
		{"muted on page", "--muted", "--bg", textFloor, ""},
		{"muted on card", "--muted", "--surface", textFloor, ""},
		{"muted on secondary card", "--muted", "--surface-2", textFloor, ""},
		{"muted on field", "--muted", "--field", textFloor, ""},
		{"muted on neutral fill", "--muted", "--neutral-bg", textFloor, ""},
		{"muted on degraded fill", "--muted", "--degraded-bg", textFloor, ""},
		{"empty field border on field", "--control-border", "--field", boundaryFloor, ""},
		{"empty field border on card", "--control-border", "--surface", boundaryFloor, ""},
		{"empty field border on secondary card", "--control-border", "--surface-2", boundaryFloor, ""},
		{"empty field border on page", "--control-border", "--bg", boundaryFloor, ""},
		{"filled field border on filled field", "--accent", "--accent-field", boundaryFloor, ""},
		{"filled field border on card", "--accent", "--surface", boundaryFloor, ""},
		{"filled field border on secondary card", "--accent", "--surface-2", boundaryFloor, ""},
		{"locked ink on locked tint", "--locked-ink", "--locked-tint", textFloor, ""},
		{"locked ink on locked fill", "--locked-ink", "--locked-bg", textFloor, ""},
		{"muted on locked tint", "--muted", "--locked-tint", textFloor, ""},
		{"secondary ink on locked tint", "--ink-2", "--locked-tint", textFloor, ""},
		{"accent on page", "--accent", "--bg", textFloor, ""},
		{"accent on card", "--accent", "--surface", textFloor, ""},
		{"accent on secondary card", "--accent", "--surface-2", textFloor, ""},
		{"deep accent on page", "--accent-deep", "--bg", textFloor, ""},
		{"deep accent on card", "--accent-deep", "--surface", textFloor, ""},
		{"deep accent on secondary card", "--accent-deep", "--surface-2", textFloor, ""},
		{"instance chip text on its fill", "--kind-instance-ink", "--kind-instance-bg", textFloor, ""},
		{"package chip text on its fill", "--kind-package-ink", "--accent-field", textFloor, ""},
	}
	pairs := make([]contrastPair, 0, len(fixed)+5*len(tones))
	pairs = append(pairs, fixed...)
	for _, tone := range tones {
		pairs = append(pairs,
			contrastPair{tone + " ink on its fill", "--" + tone + "-ink", "--" + tone + "-bg", textFloor, ""},
			contrastPair{tone + " ink on its tint", "--" + tone + "-ink", "--" + tone + "-tint", textFloor, ""},
			contrastPair{"muted on " + tone + " tint", "--muted", "--" + tone + "-tint", textFloor, ""},
			contrastPair{"secondary ink on " + tone + " tint", "--ink-2", "--" + tone + "-tint", textFloor, ""},
		)
	}
	return pairs
}

// contrastPending is the list of known failures. It is empty: the --<tone>-ink
// tokens of align-shell-and-tokens closed the two light pairs that
// fix-contrast-and-add-phone-check left pending (--healthy-ink on
// --healthy-bg and --degraded-ink on --degraded-bg, now in contrastPairs).
// A change that cannot meet a floor yet adds an entry here with the pair
// that closes it.
var contrastPending = []pendingPair{}

// The three blocks of portal.css that define the color tokens, by the text
// that opens each: the dark block under the OS preference sits inside a
// media query, so it is indented.
var themeBlocks = []struct{ theme, opening string }{
	{"light", "\n:root {"},
	{"dark", "\n  :root:not([data-theme=\"light\"]) {"},
	{"dark-chosen", "\n:root[data-theme=\"dark\"] {"},
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

var declaration = regexp.MustCompile(`(--[a-z0-9-]+):\s*([^;]+);`)

// tokenBlocks returns the custom properties of each theme block of css.
func tokenBlocks(css string) (map[string]map[string]string, error) {
	blocks := make(map[string]map[string]string)
	for _, b := range themeBlocks {
		open := strings.Index(css, b.opening)
		if open < 0 {
			return nil, fmt.Errorf("the %s block %q is not in portal.css", b.theme, strings.TrimSpace(b.opening))
		}
		body := css[open:]
		end := strings.Index(body, "}")
		if end < 0 {
			return nil, fmt.Errorf("the %s block %q does not end", b.theme, strings.TrimSpace(b.opening))
		}
		tokens := make(map[string]string)
		for _, m := range declaration.FindAllStringSubmatch(body[:end], -1) {
			tokens[m[1]] = strings.TrimSpace(m[2])
		}
		blocks[b.theme] = tokens
	}
	return blocks, nil
}

// color returns the value of token in the block of theme as RGB, or an
// error that names the token when it is missing or not a six-digit hex.
func color(blocks map[string]map[string]string, theme, token string) ([3]float64, error) {
	value, ok := blocks[theme][token]
	if !ok {
		return [3]float64{}, fmt.Errorf("token %s is missing from the %s block", token, theme)
	}
	if !hexColor.MatchString(value) {
		return [3]float64{}, fmt.Errorf("token %s holds %q in the %s block, not a six-digit hex color", token, value, theme)
	}
	var rgb [3]float64
	for i := range rgb {
		n, err := strconv.ParseUint(value[1+2*i:3+2*i], 16, 8)
		if err != nil {
			return rgb, fmt.Errorf("token %s: %w", token, err)
		}
		rgb[i] = float64(n)
	}
	return rgb, nil
}

// luminance is the WCAG 2 relative luminance of an sRGB color.
func luminance(rgb [3]float64) float64 {
	var lin [3]float64
	for i, c := range rgb {
		c /= 255
		if c <= 0.04045 {
			lin[i] = c / 12.92
		} else {
			lin[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*lin[0] + 0.7152*lin[1] + 0.0722*lin[2]
}

// contrastRatio is the WCAG 2 contrast ratio of two colors, not rounded.
func contrastRatio(a, b [3]float64) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func readPortalCSS(t *testing.T) string {
	t.Helper()
	css, err := os.ReadFile("static/portal.css")
	if err != nil {
		t.Fatal(err)
	}
	return string(css)
}

func themesOf(p contrastPair) []string {
	if p.theme == "" {
		return []string{"light", "dark"}
	}
	return []string{p.theme}
}

// TestContrastRatio checks the formula on two known pairs, so a wrong
// formula cannot pass every pair.
func TestContrastRatio(t *testing.T) {
	if got := contrastRatio([3]float64{0, 0, 0}, [3]float64{255, 255, 255}); math.Abs(got-21) > 1e-9 {
		t.Errorf("black on white = %v, want 21", got)
	}
	// The pre-fix light muted text on the page: 4.165 by the WCAG formula.
	muted, bg := [3]float64{0x6a, 0x6f, 0x7a}, [3]float64{0xef, 0xe9, 0xdc}
	if got := contrastRatio(muted, bg); math.Abs(got-4.165) > 0.001 {
		t.Errorf("#6a6f7a on #efe9dc = %.4f, want 4.165", got)
	}
}

// TestContrast holds every pair of contrastPairs to its floor in its themes,
// and fails, naming the token, when a token is missing or is not a hex color.
func TestContrast(t *testing.T) {
	blocks, err := tokenBlocks(readPortalCSS(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range contrastPairs {
		for _, theme := range themesOf(p) {
			t.Run(p.name+"/"+theme, func(t *testing.T) {
				fg, err := color(blocks, theme, p.fg)
				if err != nil {
					t.Fatal(err)
				}
				bg, err := color(blocks, theme, p.bg)
				if err != nil {
					t.Fatal(err)
				}
				if got := contrastRatio(fg, bg); got < p.min {
					t.Errorf("%s in the %s theme: %s on %s is %.3f:1, below %.1f:1", p.name, theme, p.fg, p.bg, got, p.min)
				}
			})
		}
	}
}

// TestContrastDarkBlocksAgree: the dark tokens are set twice, under the OS
// preference and under the stored Dark choice. Every token the pair lists
// use, and every tone and kind token, holds one value in both.
func TestContrastDarkBlocksAgree(t *testing.T) {
	blocks, err := tokenBlocks(readPortalCSS(t))
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	use := func(p contrastPair) {
		for _, token := range []string{p.fg, p.bg} {
			seen[token] = true
		}
	}
	for _, p := range contrastPairs {
		use(p)
	}
	for _, p := range contrastPending {
		use(p.contrastPair)
	}
	for _, tone := range tones {
		for _, suffix := range []string{"", "-ink", "-bg", "-tint"} {
			seen["--"+tone+suffix] = true
		}
	}
	for _, token := range append(append([]string{}, lockedTokens...), kindTokens...) {
		seen[token] = true
	}
	for token := range seen {
		byOS, hasOS := blocks["dark"][token]
		chosen, hasChosen := blocks["dark-chosen"][token]
		if hasOS != hasChosen || byOS != chosen {
			t.Errorf("token %s differs between the two dark blocks: %q under the OS preference, %q under the Dark choice", token, byOS, chosen)
		}
	}
}

// TestContrastPending logs the known failures. It fails when a pending pair
// passes, or when the pair that closes it now exists in portal.css: the
// change that adds the replacement tokens moves the replacement pair into
// contrastPairs and deletes the entry.
func TestContrastPending(t *testing.T) {
	blocks, err := tokenBlocks(readPortalCSS(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range contrastPending {
		theme := p.theme
		fg, err := color(blocks, theme, p.fg)
		if err != nil {
			t.Fatal(err)
		}
		bg, err := color(blocks, theme, p.bg)
		if err != nil {
			t.Fatal(err)
		}
		got := contrastRatio(fg, bg)
		t.Logf("pending: %s: %s on %s is %.3f:1 in the %s theme, below %.1f:1; closed by %s on %s", p.name, p.fg, p.bg, got, theme, p.min, p.closedByFg, p.closedByBg)
		if got >= p.min {
			t.Errorf("pending pair %q passes (%.3f:1): delete it from contrastPending", p.name, got)
		}
		_, hasFg := blocks[theme][p.closedByFg]
		_, hasBg := blocks[theme][p.closedByBg]
		if hasFg && hasBg {
			t.Errorf("pending pair %q is closed by %s on %s, which now exist in portal.css: add that pair to contrastPairs and delete the entry", p.name, p.closedByFg, p.closedByBg)
		}
	}
}

// TestContrastNamesWhatItCannotRead: a missing token and a value that is not
// a six-digit hex color fail with the token's name, not a skip.
func TestContrastNamesWhatItCannotRead(t *testing.T) {
	css := "\n:root {\n  --muted: rgb(1 2 3);\n}\n  :root:not([data-theme=\"light\"]) {\n  --muted: #8f96a3;\n}\n:root[data-theme=\"dark\"] {\n  --muted: #8f96a3;\n}\n"
	blocks, err := tokenBlocks(css)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := color(blocks, "light", "--muted"); err == nil || !strings.Contains(err.Error(), "--muted") {
		t.Errorf("an rgb() value gave %v, want an error that names --muted", err)
	}
	if _, err := color(blocks, "dark", "--control-border"); err == nil || !strings.Contains(err.Error(), "--control-border") {
		t.Errorf("a missing token gave %v, want an error that names --control-border", err)
	}
	if _, err := color(blocks, "dark", "--muted"); err != nil {
		t.Errorf("a hex value gave %v", err)
	}
}
