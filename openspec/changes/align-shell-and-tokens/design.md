## Context

The redesign (`redesign-web-ui`, archived 2026-10-05, PR 39) built the canvas's views
(`docs/design/evidence/03-ui-canvas/`) down to recorded data, but kept the look of the pages
before it. That look was a drafting table: a blueprint grid behind every page, shadowed panels with
registration marks, diamond `h2` markers, mono labels, a 1500 px column and folder tabs. A gap
report compared the canvas with the live pages at main `ad8b218`, and a second reader verified
every claim
(`docs/design/evidence/05-canvas-gap-report/gaps.md` and `gaps.json`; the screenshots it cites
are named there). Its section A lists twenty gaps every page shares, nineteen visual and one
shipped bug.

The owner answered on 2026-10-06 (verbatim): global look, "Follow the canvas (Recommended)": flat
panels, no grid background or shadows, plain headings, 1840 px column. The supervisor ruled the
same day, pending the owner, that the Applied axis keeps today's words (Applied, Failed, Stalled,
Reconciling, ...), because the canvas's "Ready / Not ready" loses the difference between a failed
and a stalled apply. The planning PR records both in `docs/DESIGN.md` as one new decision, portal:D19 ("The pages
follow the reviewed canvas's flat look").

This change is the gate for three follow-on changes: `align-platform-installed-catalog`,
`align-owner-pages` and `align-graph`. They rebuild the pages and the graph on the tokens, tab
component and state block this change lands.

## Goals / Non-Goals

**Goals:**

- Fix the shipped bug: with script on, a selected graph node fills the details panel
  (`instance-43`).
- One set of tone tokens (border, ink, background, tint) in light and dark with the canvas's
  values. Kind chip tokens. Flat page, panels and headings. An 1840 px column. The canvas's type
  scale, label style and link colour.
- One square badge for both axes; underline tabs with counts on every page; a worded live mark.
- One state block component, built and tested here, for the follow-on changes to use.
- Every gap in `gaps.md` section A mapped to a design point and a task (table below).

**Non-Goals:**

- Page-specific rebuilds, each owned by a follow-on change. These include the Platform status
  card and Installed card, the Installed filters and Catalog blocks
  (`align-platform-installed-catalog`), and the owner summary cards on the state block. Also the
  Resources and Events tables, the details panel at rest, the conditions fold moving below the
  tab panels, and Logs and YAML (`align-owner-pages`). The graph's drawing, groups and source node
  belong to `align-graph`. This change touches those templates only for shell-level markup (tab
  counts, region headings, badge markup).
- Panel inner padding. The canvas pads sections, not panels. Moving the padding means rebuilding
  each panel's content, so each follow-on change does it for its pages. Panels keep `20px` here.
- The canvas's "Ready / Not ready" words for the Applied axis (`installed-12`, still decided out
  for the words): the Applied badge keeps the controller's state words (portal:D3:R1, supervisor
  ruling 2026-10-06).
- An Events count on the Catalog page's tab (`catalog-16`). The page reads events only on the
  Events tab (`internal/ui/catalog.go:92-96`), and a count would cost a read on every tab, so the
  Events tab stays uncounted there.
- Per-catalog resolve time and digest (`catalog-09`) and a catalog's description (`catalog-03`):
  no source exists. The planning PR records them in `docs/DESIGN.md` as portal:OQ26
  (opm-operator#230) and the widened portal:OQ25; this change shows nothing for them.

## Gap map

Each section A gap id, the design point that covers it, and the section of `tasks.md` that builds
it.

| Gap | What | Design point | Section |
| --- | --- | --- | --- |
| instance-43 | Details panel empty after selecting a node | The node panel request | 1 |
| installed-10 | Instance chip cream in dark; padding | Kind chip tokens | 2 |
| platform-39 | Instance chip in dark (same as installed-10) | Kind chip tokens | 2 |
| platform-31 | Tone ink shades and dark values | Tone tokens | 2 |
| platform-28 | Shadows, corner marks, diamond `h2` | The flat look | 2 |
| installed-25 | Page background grid (Installed) | The flat look | 2 |
| platform-29 | Page background grid (Platform) | The flat look | 2 |
| platform-01 | 1500 px column | The column | 2 |
| platform-02 | Summary row proportions | The column | 2 |
| platform-37 | Link colour | Type and links | 2 |
| platform-30 | Type scale (body, `h1`, mono) | Type and links | 2 |
| installed-24 | `h1` size and lede width | Type and links | 2 |
| platform-40 | Kicker, label and table head type | Type and links | 2 |
| catalog-04 | Fact label style | Type and links | 2 |
| platform-38 | Applied and Health badge shapes | One badge shape | 3 |
| platform-09 | Platform tabs: style and counts | Underline tabs with counts | 3 |
| catalog-16 | Catalog tabs: style and counts | Underline tabs with counts | 3 |
| instance-13 | Owner tabs: style and counts | Underline tabs with counts | 3 |
| catalog-27 | Headings inside tab panels | Tab regions without visible headings | 3 |
| platform-27 | Live mark wording and pulse | The live mark | 3 |

Docs only (section X items the supervisor assigned here): `catalog-09` becomes portal:OQ26, a new open
question; `catalog-03` widens portal:OQ25. Both are `docs/DESIGN.md` text, which the planning
PR writes.

Side effects on items outside section A, claimed by nobody else: the page-wide grid half of
`instance-50` goes with the flat look; its region-heading half goes with the tab regions. The
shape half of `installed-12` and `installed-13` (decided out under the old badge rule) is
reversed by portal:D19:R3. Their word half stays out.

## Decisions

### The node panel request (section 1, instance-43)

`openNode` (`internal/ui/static/portal.js:786-802`) calls
`htmx.ajax("GET", panel, {source: a, target: "#detail", swap: "innerHTML"})`. In htmx 2.0.11 the
swap's select is `select === "unset" ? null : select || hx-select inherited from the source`
(read from `internal/ui/static/vendor/htmx-2.0.11.min.js`, the response handler). The source node
sits inside `#app`, which carries `hx-select="#main"` (`templates/layout.html:24`). The panel
response is `<div class="fragment">` with no `#main` (`templates/panel.html`), so nothing is
swapped.

The call MUST pass `select: "unset"`, the convention every other `#detail` loader already follows
with `hx-select="unset"` (`templates/panel-body.html:27-28`, `templates/partials.html:145,164`).
The other `htmx.ajax` call, `expandGroup` (`portal.js:826`), already passes `select: "#main"`.
Every `htmx.ajax` call in `portal.js` MUST name its `select`. `TestNoTrustedMarkupInTheUI`'s
neighbour in `escape_test.go` gains a scan that fails on an `htmx.ajax(` call without `select:`.

```js
htmx.ajax("GET", panel, { source: a, target: "#detail", swap: "innerHTML", select: "unset" })
```

`test/browser/graph.py` gains a check: on `/instances/default/podinfo`, activating the
`podinfo-podinfo` Deployment node fills `#detail` with the kicker `Deployment` and the name. It
drives the real F1 page under the page policy, so an inherited attribute shows as it would for a
user.

### Tone tokens (section 2, platform-31)

Every tone has four tokens on `:root`: the border (`--<tone>`), the text ink (`--<tone>-ink`), the
badge background (`--<tone>-bg`) and the block tint (`--<tone>-tint`). Each is redefined in the
`prefers-color-scheme: dark` block under `:root:not([data-theme="light"])` and again under
`:root[data-theme="dark"]`, as `portal.css` does today. Values from the canvas's token sheet
(`Main.dc.html:29-32`, `Instance.dc.html` `TONE`, `:644-651`):

| Tone (canvas) | Light border / ink / bg / tint | Dark border / ink / bg / tint |
| --- | --- | --- |
| applied (navy) | #1b2a44 / #1b2a44 / #dfe5ef / #eef1f7 | #9db3e0 / #9db3e0 / #1a2436 / #141c2b |
| healthy (green) | #157a52 / #0f5c3d / #dcefe5 / #eaf5ee | #4cc38a / #6fd3a2 / #12261c / #102019 |
| progressing (blue) | #2458b8 / #1d4796 / #dde6f7 / #edf2fb | #7ea6f2 / #9dbcf7 / #17233d / #121b2e |
| degraded (red) | #c3311b / #a3260f / #f8e0da / #fcefeb | #f0806b / #ff9d88 / #3a1712 / #2a1310 |
| unknown (amber) | #b07814 / #6e4700 / #f6ead0 / #fbf3e2 | #e3b45c / #f0c879 / #33270f / #261d0b |
| neutral (gray) | #5f6672 / #3a4150 / #e7e3da / #fffdf8 | #8f96a3 / #c4c0b5 / #262c36 / #161c26 |
| missing (live only) | #8a3a9c / #6e2c7e / #efe0f3 / #f7eefa | #d69af0 / #e2b6f5 / #2a1733 / #1f1226 |

`--line` in dark becomes `#2a3240` (canvas `--pt-d6ccb8`). A new `--line-soft` (#e5dccb, dark
#222a35) is the canvas's row divider. `missing` has no canvas value; it keeps today's border and
background and gains an ink and tint in the same family. `locked` keeps its tokens.

Components read tones through one set of classes, so the badge and the state block share them:

```css
.hue-applied     { --c: var(--applied);     --c-ink: var(--applied-ink);     --c-bg: var(--applied-bg);     --c-tint: var(--applied-tint); }
/* likewise .hue-healthy, .hue-progressing, .hue-degraded, .hue-unknown, .hue-neutral, .hue-missing, .hue-locked */
```

The prefix is `hue-`, not `tone-`: `tone-*` already names a condition's tone
(`internal/ui/view.go:207-213`) and keeps that meaning.

### Kind chip tokens (section 2, installed-10, platform-39)

`--kind-instance-bg` (#141820, dark #3a4352) and `--kind-instance-ink` (#efe9dc, dark #ebe6da);
`--kind-package-ink` (#6b4500, dark #f0c879). The package chip keeps `--accent` as its dashed
border and `--accent-field` as its fill. `.kind` padding becomes `4px 10px`. Today
`.kind-instance` fills with `--ink`, which turns cream in dark mode (`portal.css:468`).

### The flat look (section 2, platform-28, installed-25, platform-29)

- `body` keeps `background-color: var(--bg)` and loses `background-image` and `background-size`.
  The graph keeps its own grid inside `.graph` (`portal.css:654-657`); `align-graph` owns it.
- `.panel` and the summary cards lose `box-shadow` and `.panel::after`. Panels keep a 1px
  `--line` border and radius 10 px; summary cards (`.summary > .card`, `.pf-summary > .card`)
  take radius 12 px. The `--shadow` token is removed. The `settle` flash after a live refresh
  becomes a ring that fades (`0 0 0 3px var(--accent-soft)` to transparent), with no shadow under
  it.
- `h2` is 20 px with no `::before` marker.

### The column (section 2, platform-01, platform-02)

`main` and `.foot` take `max-width: 1840px`; `main` padding becomes `28px clamp(16px, 4vw, 40px)
56px`. The 12-column grid stays. Summary rows (`.summary`, `.pf-summary`) become
`repeat(auto-fit, minmax(min(380px, 100%), 1fr))` with a 14 px gap. The `min()` keeps a 360 px
viewport from scrolling sideways, which the policy requirement forbids. The Installed card stays
full width, as on the canvas.

### Type and links (section 2, platform-30, installed-24, platform-40, catalog-04, platform-37)

| Element | Today | After |
| --- | --- | --- |
| `body` | 15 px | 16 px |
| `h1` | `clamp(2rem, 5vw, 3.4rem)`, -0.03em | `clamp(2rem, 4vw, 40px)`, line-height 1.05, -0.02em |
| `.mono`, `code`, `.ver`, `.chip` | 0.86em | 13 px |
| `.lede` | max 72ch | 15 px, `--ink-2`, no width cap |
| `.kicker` | mono 0.74rem, brass, 0.14em | sans 12 px uppercase, 0.08em, `--ink-2` |
| `.label`, `.table thead th`, `.stat-l` | mono 0.7rem, 0.1em | sans 12 px uppercase, 0.07em, `--muted` |
| `.facts dt` | mono 0.7rem uppercase | sans 14 px, sentence case, `--muted`; label column `minmax(96px, max-content)`, row gap 6 px |
| `a` | ink, 2px brass underline, brass on hover | `--accent`, 1px underline in the current colour, offset 3 px; hover `--accent-deep` |

The `.facts dt` change is global: the Instance, Package, Provider and Catalog boards all draw
sentence-case labels (`Instance.dc.html:80-84`, `Catalog.dc.html:80-90`), and every `dt` in the
templates is already written in sentence case. The header sets its own link colours (`.brand`,
`.nav a`, `.conn`) and keeps them. `.kicker a` stays without underline.

`h1.mono` on the Catalog page (`catalog-01`, section B) renders at 13 px once `.mono` is fixed at
13 px, as it renders at 0.86em today: `align-platform-installed-catalog` fixes that heading, and
this change does not.

### One badge shape (section 3, platform-38)

`.applied` and `.health` both become the canvas's badge (`Main.dc.html:297`): `inline-flex`,
`padding: 3px 9px`, `border: 1.5px solid var(--c)`, `border-radius: 5px`, `color: var(--c-ink)`,
`background: var(--c-bg)`, sans 11 px, weight 600, uppercase, letter-spacing 0.05em. The tone
comes from a `hue-*` class that `appliedBadge` and `healthBadge` (`internal/ui/view.go`) add:

| Applied state | Hue | Health state | Hue |
| --- | --- | --- | --- |
| Applied | applied | Healthy | healthy |
| Reconciling | progressing | Progressing | progressing |
| Failed, Stalled | degraded | Degraded | degraded |
| Suspended, ManagedExternally, Unknown, unknown value | neutral | Unknown, unknown value | unknown |
| | | Missing | missing |

The `APPLY` prefix (`.applied::before`) and the health dot (`<span class="dot">` in the
`health` and `state` partials and the literal Missing badge in `panel-body.html`) are removed.
What tells the axes apart is now:

- the accessible name each badge already carries (`<span class="axis">Applied axis</span>`,
  `Health axis`, `templates/partials.html:1-7`), kept;
- the place a badge stands, which names its axis in visible text: the Installed table's column
  heads, the Installed card's group labels, the summary cards' headings (eyebrows after the
  follow-on changes), and the details panel's fact labels. The details panel shows the two
  badges side by side in `.axes`; it gains visible "Applied" and "Health" labels before each
  badge, so no badge stands without a named axis. The graph's node stamp and outline are
  `align-graph`'s and are untouched.

A partial health keeps its hatch and gains a dashed border, so it stays visibly apart from a full
one without colour. Its text still says "(partial)". "(not live)" is unchanged. Managed
externally loses its dashed border: the canvas draws it solid gray, and its words and neutral hue
already keep it from reading as an error. The red family is still kept to degraded, failed,
stalled and refused.

### Underline tabs with counts (section 3, platform-09, catalog-16, instance-13)

One tab component for every strip (`.tabs`): no box, a 1px `--line` bottom rule, links with
`border-bottom: 3px solid transparent` and `margin-bottom: -1px`. The current link
(`aria-current="page"`) takes `border-bottom-color: var(--accent)` and `color: var(--ink)`; the
others take `--muted`. Base size from the owner and catalog boards (`Instance.dc.html:900`,
`Catalog.dc.html:506`): 15 px, `min-height: 44px`, `padding: 10px 18px`, current weight 600,
others 400. The Platform strip adds `.tabs-lg` (`Main.dc.html:368`): 18 px, `min-height: 48px`,
`padding: 12px 18px`, current 700, others 500. A strip that does not fit wraps; it never scrolls
the page sideways.

`tabLink` (`internal/ui/owner.go:38-43`) gains a count:

```go
// tabLink is one tab. Count is shown after the label when set; nil means
// the page holds no count it may show.
type tabLink struct {
	Label   string
	Href    string
	Current bool
	Count   *int
}
```

```html
<a href="{{.Href}}"{{if .Current}} aria-current="page"{{end}}>{{.Label}}{{with .Count}} <span class="tab-n">{{.}}</span>{{end}}</a>
```

A count is shown only when the page already holds it and it covers everything the tab lists:

| Page | Tab | Count | Source (already fetched) | No count when |
| --- | --- | --- | --- | --- |
| Platform | Providers | registrations | `Platform.Registrations` (`platform.go:428`) | the Platform is not readable, or `registrationsAccess` is not `ok` |
| Platform | Catalogs | catalog rows | `catalogRows` total (`platform.go:435`) | the Platform is not readable |
| Catalog | Claims | claiming registrations | `catalogView.Claims` | claims are locked |
| Catalog | Events | none | read only on its tab | always |
| Instance, package | Resources | inventory objects plus runtime children, configuration group included | the owner document's components | the owner document is a problem |
| Instance, package | Events | folded lines in the feed the Events tab opens with | `v.Events`, read on every tab (`owner.go:576`) | the feed is a problem |
| Instance, package | Graph, Logs, YAML, Provider | none | | always |

The total is the unfiltered total. A filter narrows the list, not the tab. The owner page builds
its tabs before `ownerTabsOf` reads the feed (`owner.go:533-535`), so the counts are set after it.
When `align-owner-pages` changes what the Events tab opens with (all reached objects, merged), the
same rule gives the merged count. The tab's count changes with its default scope, and that change
owns it.

### Tab regions without visible headings (section 3, catalog-27)

A region a tab opens is named by the selected tab. The region keeps its `h2`, which carries the
`aria-labelledby` the region uses, but the heading takes `.vh`, a visually-hidden utility:

```css
.vh { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
```

This holds for the owner page's Graph, Resources, Events, Logs, YAML (both regions) and Provider
regions, and for the Catalog page's Claims and Events regions. The Platform's `#parts` has no
heading today. Its `h-note` text stays in the heading, so a screen reader still hears it. Where it
said something a sighted reader needs, the region already has a note line (`events-note`, the
logs note). `.axis` becomes an alias of `.vh`, so there is one utility.

### The live mark (section 3, platform-27)

`setLive` texts are capitalised: "Live", "Not live", "Offline", "Reconnecting", "Signed out",
"Topics refused", "Session expired, reload". The initial text in `layout.html` is "Not live".
The dot is steady: the `pulse` animation and its keyframes are removed. The dot's colours stay
(`--live-ok`, `--live-off`, muted). `test/browser/expired.py` waits for the new wording.

### The state block (section 4)

The canvas's summary block (`Main.dc.html:82-96`, `Instance.dc.html:88-107`,
`Catalog.dc.html:94-112`) is one component:

- an eyebrow (12 px uppercase, `--ink-2`) naming the axis and its source;
- a "when" at the right, which reads a recorded time only;
- a 36 px round icon in the tone's border colour: a check for applied, healthy and neutral, a
  bang for degraded, turning arrows for progressing, a clock for unknown, the lock for locked;
- the state word, 26 px, weight 700, uppercase, in the tone's ink;
- a summary line, 15 px, `--ink-2`;
- reason count links (a bold count and a mono reason, underlined, in the tone's ink), or a muted
  "none" line.

The block has a 2 px border in `var(--c)`, `var(--c-tint)` as background, and radius 12 px.

```go
// stateBlock is one summary block: the state of one axis or one standing,
// as a recorded source reports it. Pages build it; the "state-block"
// partial draws it.
type stateBlock struct {
	ID      string // the section's id, for links and live refresh
	Label   string // the accessible name, e.g. "Applied status"
	Eyebrow string // e.g. "Applied · the controller"
	When    *stamp // a recorded time, or nil: never invented
	WhenText string // the words before the time, e.g. "since", "reconciled"; with When nil, shown alone as words (e.g. "checked live"), never as a time
	Hue     string // applied, healthy, progressing, degraded, unknown, neutral, missing
	State   string // the word, e.g. "Applied", "Degraded"
	Summary string
	Reasons []countLink // reuses countLink{Text, N, Href, Class}: Class gives a reason its own hue, N 0 shows the reason without a count
	None    string      // shown when Reasons is empty, e.g. "No unhealthy resources"
	Follow  string      // data-follow topics
	Problem *v1.Problem // set: the block renders the problem region (locked or degraded) in place of the state
}
```

```html
{{define "state-block"}}
<section class="state-block hue-{{.Hue}}" id="{{.ID}}" aria-label="{{.Label}}"{{with .Follow}} data-follow="{{.}}"{{end}}>
  <div class="sb-head"><span class="sb-eyebrow">{{.Eyebrow}}</span>{{with .When}}<span class="sb-when">{{$.WhenText}} {{template "time" .}}</span>{{else}}{{with .WhenText}}<span class="sb-when">{{.}}</span>{{end}}{{end}}</div>
  {{if .Problem}}{{template "problem-region" .Problem}}{{else}}
  <div class="sb-state">{{template "sb-icon" .Hue}}<span class="sb-word">{{.State}}</span></div>
  {{with .Summary}}<p class="sb-summary">{{.}}</p>{{end}}
  <div class="sb-reasons">{{range .Reasons}}<a href="{{.Href}}"{{with .Class}} class="{{.}}"{{end}}>{{if .N}}<strong>{{.N}}</strong> {{end}}<span class="mono">{{.Text}}</span></a>{{else}}<span class="muted">{{.None}}</span>{{end}}</div>
  {{end}}
</section>
{{end}}
```

Unknown hue values map to `unknown`, never to an error hue (portal:D2:R3). A block with a
`Problem` takes `hue-locked` for `forbidden` and `hue-neutral` for a degraded read, and shows no
state word, so a locked source never reads as a state. Every text field is cluster text or page
text through `html/template`. The icon partial is inline SVG with `aria-hidden="true"`; the state
word is the non-colour cue. No `style` attribute is used.

The partial carries, from the start, the three generic forms the two follow-on changes need, so
that only one change edits it while they run in parallel: a reason with its own hue (`Class`, a
red refusal beside an amber contracts reason on the Platform page), a reason with no count (`N`
zero, for the Platform's `Ready` and `ContractsFulfilled` reasons), and words in place of a time
(`WhenText` with `When` nil, for the owner Health block's "checked live"). Words are not a time,
so the rule that a block never borrows a time holds. The one block-specific addition, the Applied
card's attempt strip, stays with `align-owner-pages`, which is the only change that edits the
partial after this one.

No page renders a state block in this change. A view test renders the partial for every hue,
with and without `When`, reasons and a problem, and checks the escaping. The follow-on changes put
it on their pages: the Platform status and Catalog resolved blocks
(`align-platform-installed-catalog`), and the Applied, Health and Provider blocks
(`align-owner-pages`).

### Authorization

No new read. The change reads no new kind, makes no new request and stores nothing new in the
browser (portal:D14:R1 unchanged). Every tab count comes from a document the page already
fetches, with the caller's grants. A count over a list the caller may not read in full is not
shown (the table above), so a count never hints at what is locked (portal:D7).

## Research & Decisions

### Where the bug comes from

**Context**: `light-i2-selected-panel.png` shows an empty details panel after a node is selected
with script on.
**Explored**: `portal.js:786-802`, `layout.html:24`, `panel.html`, the htmx 2.0.11 response
handler (vendored, minified; the `select` expression quoted above), and `test/browser/graph.py`,
which activates nodes but never reads `#detail`.
**Options considered**:
1. `select: "unset"` on the call: matches the `hx-select="unset"` convention of every other
   `#detail` loader; one line.
2. `select: ".fragment"`: works, but ties the script to the fragment's class name.
3. Drop `hx-select` from `#app` and set it per link: touches every boosted link's behaviour.
**Decision**: 1, plus a scan that every `htmx.ajax` call names `select`, and a browser check on
the real page.
**Rationale**: smallest fix at the cause; the scan stops the next call from inheriting the same
attribute.

### Badge shape against the old rule

**Context**: the main spec required two badges "of different shapes". The canvas draws one shape
for both. The owner chose to follow the canvas.
**Explored**: `installed-12` and `installed-13` in `gaps.md` section X, `portal:D3:R1` (which asks
for two separate values, not two shapes), and the axis texts already in the partials.
**Options considered**:
1. Keep two shapes: contradicts the owner's answer.
2. One shape, and the canvas's "Ready / Not ready" words: loses Failed against Stalled and
   Reconciling.
3. One shape, the controller's state words, the axis named by place and accessible name.
**Decision**: 3.
**Rationale**: portal:D3:R1 holds (two values, never derived from each other). The shape was a
spec choice, not a decision. Placement names the axis everywhere a badge stands, and the details
panel gains visible axis labels where it had none.

### Which counts a tab may show

**Context**: the canvas shows a count on most tabs; the live pages fetch some lists only on
their own tab.
**Explored**: `platform.go:406-460`, `catalog.go:80-100`, `owner.go:515-611`.
**Options considered**:
1. Count every tab, adding reads: the Catalog Events tab would read events on every tab.
2. Count only what the page already holds, and show no count over a partly readable list.
**Decision**: 2.
**Rationale**: no new read, no count that hints at locked items, and the counts the canvas puts
first (Providers, Catalogs, Resources, Events) are all held already.

### One state block now, used later

**Context**: two follow-on changes run in parallel and both draw the canvas's summary block;
`gaps.md` notes no such component exists live (`catalog-08`).
**Explored**: the block on four boards (`Main.dc.html:82-96`, `Instance.dc.html:88-107` and
`:771-780`, `Catalog.dc.html:94-112`, `Provider.dc.html:782-790`): one template, with the tone,
icon, word, summary and reasons as data.
**Options considered**:
1. Each follow-on change builds its own: two copies to reconcile.
2. Build it here, unused for one release, tested by a view test.
3. Build it here and convert one page: that page belongs to a follow-on change.
**Decision**: 2.
**Rationale**: Principle VII asks for justified complexity. The justification is that the two
parallel changes need one component, and putting it in the gate is the only order without a
copy.

## Risks / Trade-offs

- **The look changes on every page at once.** Goldens change wholesale in sections 2 and 3.
  Mitigation: CSS-only steps where possible, the goldens reviewed per page, and screenshots in
  light, dark and at 360 px compared with the canvas (section 5).
- **Same-shape badges are easier to confuse.** Mitigation: a badge never stands without a named
  axis, and each carries its axis in its accessible name. The words of the two axes do not
  overlap except "Unknown", and that one is named by place.
- **An unused component for one release.** Mitigation: the view test holds its contract, and the
  follow-on changes cannot start before it lands.
- **Shared templates edited here and in the follow-on changes.** Mitigation: this change edits
  them only for tab counts, region headings and badge markup, and lands first. The follow-on
  changes start from it.
- **Main-spec overlap.** This change MODIFIES two `web-ui` requirements ("Applied and health are
  two badges, never one"; "An expired session stops the page's stream") and adds five under new
  names. The follow-on changes must not modify those two, or must rebase on this change's
  archive.

## Migration Plan

None. Nothing is stored or served differently. A browser's stored theme and filters keep
working. Rollback is a revert.

## Open Questions

None for this change. The docs-only questions are in `docs/DESIGN.md`: portal:OQ26 (per-catalog
resolve time and digest, opm-operator#230) and portal:OQ25 (now also a catalog's description).
