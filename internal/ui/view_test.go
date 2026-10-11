package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/api/apitest"
)

// renderMain executes one page template's content over a synthetic view,
// for the states F1 does not hold.
func renderMain(t *testing.T, name string, main any) string {
	t.Helper()
	h := &Handler{cfg: Config{Now: time.Now}}
	pages, err := parsePages(h.templateFuncs())
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := pages[name].ExecuteTemplate(&b, "content", page{Path: "/x", Main: main}); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// TestAcceptedAndActiveApart (portal:D4:R4/R7): an accepted, inactive claim
// shows two pills on their own, and a blocked removal reads as removal
// blocked while accepted and active, never as refused.
func TestAcceptedAndActiveApart(t *testing.T) {
	v := platformView{Tab: tabProviders, Platform: v1.Platform{
		Name:                "cluster",
		RegistrationsAccess: v1.AccessOK,
		Registrations: []v1.Registration{
			{Name: "pending", Catalog: "c@v0", Accepted: true, Active: false, Verdict: "Accepted"},
			{Name: "leaving", Catalog: "d@v0", Accepted: true, Active: true, Verdict: "RemovalBlocked", Reason: "DependentsRemain", Message: "2 instances still demand it"},
		},
	}}
	v.Providers.Access = v1.AccessOK
	v.Providers.Rows, v.Providers.Total = providerRows(&v.Platform, nil)
	v.Providers.Form = newFilterForm(parseFilters(&providerFilters, nil), "Filter providers", nil, nil, nil)
	v.Events.Filters = parseFilters(&filterView{Path: "/"}, nil)
	out := renderMain(t, "platform", v)
	pending := between(out, `<span class="mono">pending</span></th>`, "</tr>")
	if !strings.Contains(pending, ">accepted<") || !strings.Contains(pending, ">inactive<") {
		t.Errorf("accepted, inactive claim:\n%s", pending)
	}
	leaving := between(out, `<span class="mono">leaving</span></th>`, "</tr>")
	if !strings.Contains(leaving, "Removal blocked") || !strings.Contains(leaving, ">accepted<") || !strings.Contains(leaving, ">active<") ||
		strings.Contains(leaving, "Refused") || !strings.Contains(leaving, "DependentsRemain") {
		t.Errorf("removal-blocked claim:\n%s", leaving)
	}
}

// TestSecretsOfferNoYAML (portal:D8:R1): a Secret in an inventory says its
// data is never read and links to nothing.
func TestSecretsOfferNoYAML(t *testing.T) {
	h := &Handler{cfg: Config{Now: time.Now}}
	ref := v1.ObjectRef{Version: "v1", Kind: "Secret", Namespace: "default", Name: "db"}
	v := ownerView{Kind: instanceKind, Namespace: "default", Name: "app", Tab: tabResources, Components: h.components("/instances/default/app", []v1.Component{{
		Name:    "db",
		Objects: []v1.InventoryObject{{Ref: ref, Access: v1.AccessOK, Health: &v1.ObjectHealth{State: "Healthy"}, Live: true}},
	}})}
	out := between(renderMain(t, "owner", v), `id="components"`, "</section>")
	if !strings.Contains(out, "Secret data is never read") || strings.Contains(out, "kind=Secret") {
		t.Errorf("Secret row:\n%s", out)
	}
}

// TestPolledObjectsSayWhenTheyWereRead (portal:D3:R5): a polled object and
// a health that is not live show when they were evaluated.
func TestPolledObjectsSayWhenTheyWereRead(t *testing.T) {
	h := &Handler{cfg: Config{Now: func() time.Time { return time.Date(2026, 10, 5, 12, 1, 0, 0, time.UTC) }}}
	read := time.Date(2026, 10, 5, 12, 0, 30, 0, time.UTC)
	ref := v1.ObjectRef{Version: "v1", Kind: "Service", Namespace: "default", Name: "web"}
	v := ownerView{
		Kind: instanceKind, Namespace: "default", Name: "app", Tab: tabResources,
		Health: v1.Health{State: "Healthy", Live: false, EvaluatedAt: &read},
		Components: h.components("/instances/default/app", []v1.Component{{
			Name:    "web",
			Objects: []v1.InventoryObject{{Ref: ref, Access: v1.AccessOK, Health: &v1.ObjectHealth{State: "Healthy"}, EvaluatedAt: &read}},
		}}),
	}
	pages, err := parsePages(h.templateFuncs())
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := pages["owner"].ExecuteTemplate(&b, "content", page{Path: "/x", Main: v}); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "not live · read <time datetime=\"2026-10-05T12:00:30Z\"") || !strings.Contains(out, "evaluated <time") || !strings.Contains(out, "Healthy (not live)") {
		t.Errorf("polled object without its read time:\n%s", between(out, `id="health-card"`, `id="detail"`))
	}
}

// renderPartial executes one named partial over data.
func renderPartial(t *testing.T, name string, data any) string {
	t.Helper()
	h := &Handler{cfg: Config{Now: time.Now}}
	pages, err := parsePages(h.templateFuncs())
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := pages["platform"].ExecuteTemplate(&b, name, data); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// TestBadgesAreOneShapeAndNameTheirAxis (portal:D19:R3): the three badge
// partials draw no dot, and each carries its axis in its accessible name,
// the "state" partial included.
func TestBadgesAreOneShapeAndNameTheirAxis(t *testing.T) {
	for name, tc := range map[string]struct {
		out  string
		want string
	}{
		"applied": {renderPartial(t, "applied", v1.Reconcile{State: "Failed", Retrying: true}), `<span class="applied applied-failed"><span class="axis">Applied axis</span>Failed, retrying</span>`},
		"health":  {renderPartial(t, "health", v1.Health{State: "Degraded", Live: true}), `<span class="health health-degraded"><span class="axis">Health axis</span>Degraded</span>`},
		"state":   {renderPartial(t, "state", "Healthy"), `<span class="health health-healthy"><span class="axis">Health axis</span>Healthy</span>`},
	} {
		if !strings.Contains(tc.out, tc.want) {
			t.Errorf("%s partial: got %q, want it to hold %q", name, tc.out, tc.want)
		}
		if strings.Contains(tc.out, `class="dot"`) {
			t.Errorf("%s partial still draws a dot: %q", name, tc.out)
		}
	}
	partial := renderPartial(t, "health", v1.Health{State: "Healthy", Partial: true, Live: false})
	if !strings.Contains(partial, "health-partial") || !strings.Contains(partial, "Healthy (partial, not live)") {
		t.Errorf("a partial health keeps its class and words: %q", partial)
	}
}

// TestHoverCardAndPanelNameTheirAxes (portal:D19:R3): where an applied and a
// health badge stand together, visible text names each axis.
func TestHoverCardAndPanelNameTheirAxes(t *testing.T) {
	s := newSite(t, apitest.F1(t), apitest.AllowAll)
	page := s.get(t, "/instances/default/podinfo").body
	card := between(page, `data-card-for="mi:default/podinfo"`, "</div>")
	for _, want := range []string{`<span class="label">Health</span> <span class="health health-healthy">Healthy</span>`, `<span class="label">Applied</span> <span class="applied applied-applied">Applied</span>`} {
		if !strings.Contains(card, want) {
			t.Errorf("the root node's hover card lacks %q:\n%s", want, card)
		}
	}
	component := between(page, `data-card-for="comp:mi/default/podinfo/podinfo"`, "</div>")
	if !strings.Contains(component, `<span class="label">Health</span>`) || strings.Contains(component, `<span class="label">Applied</span>`) {
		t.Errorf("a component's hover card names Health only:\n%s", component)
	}
	if strings.Contains(page, `class="dot"`) {
		t.Error("the instance page still draws a badge dot")
	}
	root := s.get(t, "/instances/default/podinfo/node?id=mi:default/podinfo").body
	if !strings.Contains(root, `<span class="label">Applied</span> <span class="applied applied-applied">`) || !strings.Contains(root, `<span class="label">Health</span> <span class="health health-healthy">`) {
		t.Errorf("the root node's panel lacks the visible Applied and Health labels:\n%s", root)
	}
	deploy := s.get(t, "/instances/default/podinfo/node?id=obj:apps/Deployment/default/podinfo-podinfo").body
	if !strings.Contains(deploy, `<span class="label">Health</span> <span class="health health-healthy">`) || strings.Contains(deploy, `<span class="label">Applied</span>`) {
		t.Errorf("the Deployment's panel names Health only:\n%s", deploy)
	}
}

// sbNow is the clock of the state block tests, so a golden holds no
// time-dependent text.
var sbNow = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

// renderPartialAt executes one named partial over data at a fixed clock.
func renderPartialAt(t *testing.T, name string, data any) string {
	t.Helper()
	h := &Handler{cfg: Config{Now: func() time.Time { return sbNow }}}
	pages, err := parsePages(h.templateFuncs())
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := pages["platform"].ExecuteTemplate(&b, name, data); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// goldenFragment compares got with testdata/golden/<name>.html, or writes it
// under -update.
func goldenFragment(t *testing.T, name, got string) {
	t.Helper()
	got = strings.TrimSpace(collapse(got)) + "\n"
	golden := filepath.Join("testdata", "golden", name+".html")
	if *update {
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
		t.Errorf("%s differs from %s (run with -update after checking the diff):\n%s", name, golden, got)
	}
}

// iconSignature is a piece of each icon's drawing, so a test can tell which
// icon a hue got.
var iconSignature = map[string]string{
	"check":  `d="M10.5 18.5l5.5 5.5`,
	"bang":   `d="M18 10v9M18 25v.5"`,
	"arrows": `d="M11 17a7.5 7.5`,
	"clock":  `d="M18 10.5V18l5 3"`,
	"lock":   `<rect x="12" y="16"`,
}

// TestStateBlockDrawsEveryHue (portal:D19:R3): each hue gets its class, its
// icon and its state word; the icon is decoration and the word names the
// state.
func TestStateBlockDrawsEveryHue(t *testing.T) {
	for hue, icon := range map[string]string{
		"applied": "check", "healthy": "check", "neutral": "check",
		"degraded": "bang", "missing": "bang",
		"progressing": "arrows", "unknown": "clock", "locked": "lock",
	} {
		out := renderPartialAt(t, "state-block", newStateBlock(stateBlock{
			ID: "b-" + hue, Label: "Status", Eyebrow: "Eyebrow", Hue: hue, State: "Word " + hue, Summary: "A summary.",
		}))
		if !strings.Contains(out, `class="state-block hue-`+hue+`"`) {
			t.Errorf("%s: no hue class:\n%s", hue, out)
		}
		if !strings.Contains(out, `<svg class="sb-icon" viewBox="0 0 36 36" aria-hidden="true"`) {
			t.Errorf("%s: the icon is missing or not hidden from assistive technology:\n%s", hue, out)
		}
		for name, sig := range iconSignature {
			if has := strings.Contains(out, sig); has != (name == icon) {
				t.Errorf("%s: icon %s present = %v, want the %s icon only", hue, name, has, icon)
			}
		}
		if !strings.Contains(out, `<span class="sb-word">Word `+hue+`</span>`) || !strings.Contains(out, `<p class="sb-summary">A summary.</p>`) {
			t.Errorf("%s: no state word or summary:\n%s", hue, out)
		}
		if !strings.Contains(out, `id="b-`+hue+`" aria-label="Status"`) || strings.Contains(out, "style=") {
			t.Errorf("%s: wrong section attributes or a style attribute:\n%s", hue, out)
		}
	}
	out := renderPartialAt(t, "state-block", newStateBlock(stateBlock{ID: "b", Label: "Status", Hue: "not-a-hue", State: "Odd"}))
	if !strings.Contains(out, "hue-unknown") || strings.Contains(out, "not-a-hue") || strings.Contains(out, "hue-degraded") {
		t.Errorf("an unknown hue reads unknown, never an error hue:\n%s", out)
	}
}

// TestStateBlockNeverInventsATime (portal:D19:R3): a recorded time is drawn
// with its words; with no time the words stand alone and no time appears.
func TestStateBlockNeverInventsATime(t *testing.T) {
	at := sbNow.Add(-2 * time.Hour)
	with := renderPartialAt(t, "state-block", newStateBlock(stateBlock{ID: "b", Hue: "healthy", State: "Healthy", When: &at, WhenText: "since"}))
	if !strings.Contains(with, `<span class="sb-when">since <time datetime="2026-10-11T10:00:00Z"`) {
		t.Errorf("a recorded time is drawn after its words:\n%s", with)
	}
	words := renderPartialAt(t, "state-block", newStateBlock(stateBlock{ID: "b", Hue: "healthy", State: "Healthy", WhenText: "checked live"}))
	if !strings.Contains(words, `<span class="sb-when">checked live</span>`) || strings.Contains(words, "<time") || strings.Contains(words, "never") {
		t.Errorf("words without a time stand alone:\n%s", words)
	}
	none := renderPartialAt(t, "state-block", newStateBlock(stateBlock{ID: "b", Hue: "healthy", State: "Healthy"}))
	if strings.Contains(none, "sb-when") || strings.Contains(none, "<time") {
		t.Errorf("a block with no time and no words draws neither:\n%s", none)
	}
}

// fullStateBlock is a block that uses every optional form.
func fullStateBlock() stateBlock {
	at := sbNow.Add(-3 * time.Minute)
	return newStateBlock(stateBlock{
		ID: "applied-status", Label: "Applied status", Eyebrow: "Applied · the controller",
		When: &at, WhenText: "reconciled", Hue: "degraded", State: "Failed", Summary: "The last attempt failed.",
		Notes: []sbNote{
			{Text: "Retrying after a backoff."},
			{Text: "RenderFailed", Class: "partial", Mono: true},
			{Text: "Some objects are locked.", Class: "locked"},
		},
		Caption: "Warning events, last hour",
		Reasons: []countLink{
			{Text: "ImagePullBackOff", N: 3, Href: "/installed?reason=ImagePullBackOff"},
			{Text: "Refused", N: 1, Class: "hue-degraded", Href: "/installed?reason=Refused"},
			{Text: "Ready", Href: "/installed?reason=Ready"},
		},
		Links:  []link{{Text: "Open the Provider tab", Href: "/instances/default/x?tab=provider"}},
		Follow: "platform",
	})
}

// TestStateBlockDrawsItsOptionalForms (portal:D19:R3): notes with their own
// class, a caption, reasons with their own hue and with no count, the none
// text, footer links and the follow topics.
func TestStateBlockDrawsItsOptionalForms(t *testing.T) {
	out := renderPartialAt(t, "state-block", fullStateBlock())
	for _, want := range []string{
		`data-follow="platform"`,
		`<p class="sb-note">Retrying after a backoff.</p>`,
		`<p class="sb-note partial"><span class="mono">RenderFailed</span></p>`,
		`<p class="sb-note locked">Some objects are locked.</p>`,
		`<p class="sb-caption">Warning events, last hour</p>`,
		`<a href="/installed?reason=ImagePullBackOff"><strong>3</strong> <span class="mono">ImagePullBackOff</span></a>`,
		`<a href="/installed?reason=Refused" class="hue-degraded"><strong>1</strong> <span class="mono">Refused</span></a>`,
		`<a href="/installed?reason=Ready"><span class="mono">Ready</span></a>`,
		`<p class="sb-links"><a href="/instances/default/x?tab=provider">Open the Provider tab</a></p>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the block lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "<strong>0</strong>") {
		t.Errorf("a reason with a zero count is drawn without a count:\n%s", out)
	}
	if strings.Contains(out, `class="muted"`) {
		t.Errorf("the none text shows only when there are no reasons:\n%s", out)
	}
	empty := renderPartialAt(t, "state-block", newStateBlock(stateBlock{ID: "b", Hue: "healthy", State: "Healthy", None: "No unhealthy resources"}))
	if !strings.Contains(empty, `<span class="muted">No unhealthy resources</span>`) {
		t.Errorf("the none text is missing:\n%s", empty)
	}
	if bare := renderPartialAt(t, "state-block", newStateBlock(stateBlock{ID: "b", Hue: "healthy", State: "Healthy"})); strings.Contains(bare, "sb-note") || strings.Contains(bare, "sb-caption") || strings.Contains(bare, "sb-links") || strings.Contains(bare, "data-follow") {
		t.Errorf("an optional form that is not set draws nothing:\n%s", bare)
	}
}

// TestStateBlockProblemForms (portal:D19:R3, portal:D2:R3): a forbidden read
// is locked and any other problem is neutral; neither draws a state word, an
// icon or a reason.
func TestStateBlockProblemForms(t *testing.T) {
	for code, hue := range map[string]string{v1.CodeForbidden: "locked", "not_readable_by_portal": "neutral", "internal": "neutral"} {
		b := newStateBlock(stateBlock{ID: "b", Label: "Status", Eyebrow: "Platform status", Hue: "healthy", State: "Healthy", Summary: "s", Problem: &v1.Problem{Code: code}})
		if b.Hue != hue {
			t.Errorf("problem %s: hue %s, want %s", code, b.Hue, hue)
		}
		out := renderPartialAt(t, "state-block", b)
		if !strings.Contains(out, "hue-"+hue) || !strings.Contains(out, "Platform status") {
			t.Errorf("problem %s: no hue or eyebrow:\n%s", code, out)
		}
		for _, bad := range []string{"sb-word", "sb-icon", "sb-summary", "sb-reasons", "Healthy"} {
			if strings.Contains(out, bad) {
				t.Errorf("problem %s still draws %q:\n%s", code, bad, out)
			}
		}
	}
}

// TestStateBlockEscapesClusterText (SR11): hostile text in a reason, a note,
// the caption, a link, the summary, the eyebrow and the none text is escaped
// in every place the block draws it.
func TestStateBlockEscapesClusterText(t *testing.T) {
	hostileBlock := newStateBlock(stateBlock{
		ID: "b", Label: "Status", Eyebrow: hostile, WhenText: hostile, Hue: "degraded", State: hostile, Summary: hostile,
		Notes:   []sbNote{{Text: hostile}, {Text: hostile, Mono: true}},
		Caption: hostile,
		Reasons: []countLink{{Text: hostile, N: 2, Href: "/x?r=" + hostile}},
		Links:   []link{{Text: hostile, Href: "/y?l=" + hostile}},
	})
	out := renderPartialAt(t, "state-block", hostileBlock)
	if strings.Contains(out, "<script") || strings.Contains(out, "<img") {
		t.Errorf("hostile text reached the block as markup:\n%s", out)
	}
	if got := strings.Count(out, "&lt;script&gt;alert(1)&lt;/script&gt;"); got != 9 {
		t.Errorf("hostile text is escaped in %d places, want 9:\n%s", got, out)
	}
	none := renderPartialAt(t, "state-block", newStateBlock(stateBlock{ID: "b", Hue: "healthy", State: "Healthy", None: hostile}))
	if strings.Contains(none, "<script") || !strings.Contains(none, "&lt;script&gt;") {
		t.Errorf("the none text is not escaped:\n%s", none)
	}
}

// TestStateBlockGoldens: the fragments the follow-on changes copy from.
func TestStateBlockGoldens(t *testing.T) {
	at := sbNow.Add(-26 * time.Hour)
	goldenFragment(t, "fragment-state-block-full", renderPartialAt(t, "state-block", fullStateBlock()))
	goldenFragment(t, "fragment-state-block-words", renderPartialAt(t, "state-block", newStateBlock(stateBlock{
		ID: "health-status", Label: "Health status", Eyebrow: "Health · live", WhenText: "checked live",
		Hue: "healthy", State: "Healthy", Summary: "Every object is ready.", None: "No unhealthy resources",
	})))
	goldenFragment(t, "fragment-state-block-since", renderPartialAt(t, "state-block", newStateBlock(stateBlock{
		ID: "platform-status", Label: "Platform status", Eyebrow: "Platform", When: &at, WhenText: "since",
		Hue: "progressing", State: "Reconciling", Summary: "Waiting for a catalog.",
	})))
	goldenFragment(t, "fragment-state-block-problem-forbidden", renderPartialAt(t, "state-block", newStateBlock(stateBlock{
		ID: "platform-status", Label: "Platform status", Eyebrow: "Platform", Problem: &v1.Problem{Code: v1.CodeForbidden},
	})))
	goldenFragment(t, "fragment-state-block-problem-degraded", renderPartialAt(t, "state-block", newStateBlock(stateBlock{
		ID: "platform-status", Label: "Platform status", Eyebrow: "Platform", Problem: &v1.Problem{Code: "not_readable_by_portal"},
	})))
}

// TestTipPartial (WCAG 2.2 1.4.13, 4.1.2): the trigger is a native button, which
// takes focus and names its box; the box is the last child of span.tip, a
// tooltip with text only.
func TestTipPartial(t *testing.T) {
	out := renderPartialAt(t, "tip", tipData{ID: "tip-provider", Trigger: "Provider", Text: "Who installed this."})
	want := `<span class="tip"><button type="button" class="tip-t" aria-describedby="tip-provider">Provider</button><span class="tipbox" id="tip-provider" role="tooltip">Who installed this.</span></span>`
	if strings.TrimSpace(out) != want {
		t.Errorf("tip markup:\n got %s\nwant %s", out, want)
	}
	goldenFragment(t, "fragment-tip", out)
	end := renderPartialAt(t, "tip", tipData{ID: "t", Trigger: "x", Text: "y", End: true})
	if !strings.Contains(end, `class="tip tip-end"`) {
		t.Errorf("an end tip carries tip-end: %s", end)
	}
	bad := renderPartialAt(t, "tip", tipData{ID: "t", Trigger: hostile, Text: hostile})
	if strings.Contains(bad, "<script") || strings.Contains(bad, "<img") || strings.Count(bad, "&lt;script&gt;") != 2 {
		t.Errorf("hostile tip text is not escaped:\n%s", bad)
	}
	if strings.Contains(out, "tabindex") || strings.Contains(out, "style=") {
		t.Errorf("a tip trigger is a button, not a scripted span, and has no style attribute: %s", out)
	}
	box := out[strings.Index(out, `<span class="tipbox"`):]
	if strings.Contains(box, "<a ") || strings.Contains(box, "<button") || !strings.HasSuffix(strings.TrimSpace(out), "</span></span>") {
		t.Errorf("the box holds text only and is the last child of span.tip: %s", out)
	}
}
