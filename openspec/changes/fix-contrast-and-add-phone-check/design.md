## Context

See proposal.md for the motivation. Current state, read on base `c60cf02` (origin/main, 2026-10-11):

- `internal/ui/static/portal.css` is hand-written, with no build step. Every colour is a custom
  property on `:root`, redefined in two dark blocks that must agree: `@media (prefers-color-scheme:
  dark) { :root:not([data-theme="light"]) { ... } }` (line 84) and `:root[data-theme="dark"]` (line
  132). `--muted` is `#6a6f7a` (light, line 28) and `#8f96a3` (dark). `--line-strong` is `#b9ad94`
  and `#3a4352`, and it draws 14 borders and rules; the only one that is a control's sole boundary
  is `.field input, .field select` (line 446). The other uses are pills, chips, text-labelled
  buttons, folds and rules, where a text label identifies the element.
- `body` has `overflow-x: hidden` (line 199). The masthead nav scrolls inside its own box at 760 px
  and below (line 244). The graph, the YAML view and the log panes scroll inside their own boxes.
- `cmd/opm-portal/browser_test.go` (build tag `browser`) runs Playwright scripts from `test/browser/`
  in the pinned image through `playwright()` and serves the F1 capture through `serveF1Site`.
  `Taskfile.yml:90` lists the test names in a `-run` pattern. The nightly `E2E` workflow runs
  `task test:browser` (`.github/workflows/e2e.yml:172`). Pull-request CI does not.
- The `internal/ui` tests already read `static/portal.css` (`refresh_test.go:198`), so a Go test can
  read it too.
- Thresholds are those of `eng-frontend-build`, subject `accessibility`: 4.5:1 for text (1.4.3), 3:1
  for the boundary of a control (1.4.11), reflow at 320 CSS px without two-dimensional scrolling
  (1.4.10). The portal's own rule is the 360 px line of the web-ui spec.

## Goals / Non-Goals

**Goals:**

- A failing test for the 360 px rule that runs where the other browser tests run.
- Token values that pass AA for muted text and for the boundary of filter inputs and selects, in both
  themes, held by a test that later changes extend.

**Non-Goals:**

- Any token other than `--muted` (light) and the new `--control-border`. The `--*-ink` tokens, the
  tints and the tone pairs belong to `align-shell-and-tokens` (design/T3.4 amendment SH-A1).
- A pull-request CI job for browser tests, a 320 px test, a keyboard or screen reader pass, a
  forced-colours rule.
- A change to layout, copy or components. If a page fails the phone test, the fix is one CSS rule or
  a named exception; see Decision 3.
- The phone states that need another capture or authorizer than `serveF1Site`: the locked page, the
  broken-rollout page and in-cluster mode (their goldens are `ui_test.go:151,168,178` and
  `incluster_test.go:16`). Each needs its own server set-up; a later change can add them to the test
  with that set-up. Locked and degraded panels hold long cluster text, so this is the first gap to
  close.

## Decisions

### Research & Decisions

#### 1. Where the phone check lives and how it finds pages

**Context**: The 360 px rule has no test. The check needs a real layout engine, so it is a browser
test. It needs a page list, and the `f1Pages` list that the golden tests use is in package `ui`
tests, which package `main` cannot import.

**Explored**: `cmd/opm-portal/browser_test.go`, `test/browser/theme.py`, `internal/ui/ui_test.go:109`
(the 28 entries of `f1Pages`, 3 of them fragments). A scratch run on 2026-10-11 (Evidence).

**Options considered**:

1. A list of page paths in `browser_test.go`, passed to a new `test/browser/phone.py` as arguments -
   deterministic, extended by name as design/T3.4 amendments SH-A4, PIC-A2, OP-A5 and AG-A4 assume;
   it mirrors `f1Pages` by hand, so it can drift.
2. The script crawls same-origin links from `/` - finds new pages without a list, but misses
   states reached only by a query (`?tab=logs`, `?expand=...`) and gives a different page set on
   each run.
3. Export `f1Pages` from `internal/ui` - one list, but it adds exported test data to a production
   package.

**Decision**: Option 1. The list holds the 25 non-fragment paths of `f1Pages` and the not-found
path `/nope`, which the same F1 site serves and which `TestEveryPageCarriesThePagePolicy` counts as a
page (`ui_test.go:260`): 26 paths. A later change that adds a page adds its path.
**Rationale**: A stable list makes a failure name a page and keeps the test reviewable. The drift
risk is small and is the same one the golden list already carries.

#### 2. What the phone check measures

**Decision**: At a 360 by 640 viewport (the one height of the test; task 1.4 confirms the Evidence below at this
height), `max(document.documentElement.scrollWidth,
document.body.scrollWidth)` must be at most 360 after `load` and a 300 ms settle. On failure the
script lists up to six elements whose right edge is past 360 and that have no scrolling ancestor.
The script first injects a 500 px wide element into the first page and requires the measure to rise
above 360, so a layout change that makes the measure blind fails the test. Light scheme only: the
themes change colours, not layout. The script reports all failing pages, then exits 1.
**Rationale**: `body` has `overflow-x: hidden`, so a scrollbar test would pass while content is
clipped. `scrollWidth` still counts clipped overflow, so the measure catches content lost by clipping
(1.4.10) as well as sideways scroll. Boxes inside their own scroller (graph, YAML, log panes, the
masthead nav) do not count: they are the intended way to show wide content.
**Alternatives**: Measuring element rectangles alone would flag the intended scrollers.
`is_mobile` emulation would change the viewport meta handling, which the pages do not set; plain
viewport resizing is what the spec states.

#### 3. A page that fails today

**Context**: The brief asks: if a page fails and the fix is more than one CSS rule, list it as a
named exception that the owning PR 40 change removes.
**Decision**: No page fails, so the test has no exception list and the proposal says so. If the
implementation run measures a failing page: a fix of one CSS rule goes into section 1 with the
test; a larger fix stops the work and is reported, because a changed task list is not the
implementer's to decide.
**Rationale**: An exception mechanism with no entries is dead code (Principle VII).

#### 4. Muted text

**Decision**: Light `--muted` becomes `#5f6672`, the candidate of design/T2.3 section 10.1. Dark
`--muted` stays `#8f96a3`.
**Alternatives**: `#5c6370` or `#5a606c` raise the margin on the page background from 4.78 to 5.00
or 5.22. The candidate is also the board's gray and the value of `--neutral`, so the palette gains
no new colour. The margin is thin on the page background (4.782) and on the neutral fill (4.517); the browser pass in task 2.3 decides
whether to darken it. A page background is not flat (a 6 percent grid and a corner glow, `portal.css`
body rule), and the pass samples the rendered pixels (Evidence).

#### 5. The control-border token

**Decision**: Add `--control-border`: `#8c8269` light, `#6a7587` dark, in the three places that
define the colour tokens. Use it only for `.field input, .field select`. `--line-strong` keeps its
value and its other uses.
**Rationale**: 1.4.11 asks 3:1 for a boundary that is needed to identify a control. A pill, a chip
or a button with a text label is identified by its text. Darkening `--line-strong` everywhere would
change the look of every rule that reads it and break "must not change layout, copy or components". The
filled state (`border-color: var(--accent)` on `--accent-field`, line 447) is unchanged.
**Alternative**: Re-point `--line-strong` and add a lighter token for decoration. Rejected: it moves
every decorative border.

#### 6. The contrast test

**Decision**: `internal/ui/contrast_test.go`, package `ui`, no browser, runs in `task test`.

```go
type contrastPair struct {
	name  string  // "muted on page"
	fg    string  // custom property, "--muted"
	bg    string  // "--bg"
	min   float64 // 4.5 or 3
	theme string  // "", "light" or "dark"; "" checks both
}
var contrastPairs = []contrastPair{ /* the list below */ }
var contrastPending = []pendingPair{ /* pairs that fail today; see below */ }

type pendingPair struct {
	contrastPair             // the pair that fails today
	closedByFg, closedByBg string // the replacement tokens, "--healthy-ink", "--healthy-bg"
}
```

- It reads `static/portal.css`, takes the light tokens from the `:root` block and the dark ones from
  the two dark blocks, and fails if a listed token is missing or is not a six-digit hex.
- `contrastRatio(a, b)` uses the WCAG 2 relative luminance formula and is not rounded. The test
  checks the function on black and white (21) and on `#6a6f7a` over `#efe9dc` (4.165).
- For every token in the lists, the two dark blocks must hold the same value.
- A pair under its floor fails with its name, theme, tokens and ratio.
- A pending entry names the pair that closes it (`closedBy`: the replacement foreground and background
  tokens). The test logs the entry, and fails when both tokens of the replacement pair exist in
  `portal.css`. The change that adds those tokens then has to delete the entry and put the
  replacement pair into `contrastPairs`, so the list cannot go stale, whatever the ratio of the old
  pair does.

The list for this change (ratios by formula on the candidate values; Evidence has the method):

| Pair | Floor | Light | Dark |
| --- | --- | --- | --- |
| `--muted` on `--bg` | 4.5 | 4.782 | 6.452 |
| `--muted` on `--surface` | 4.5 | 5.454 | 5.987 |
| `--muted` on `--surface-2` | 4.5 | 5.045 | 5.624 |
| `--muted` on `--field` | 4.5 | 5.691 | 5.748 |
| `--muted` on `--neutral-bg` | 4.5 | 4.517 | 4.968 |
| `--muted` on `--degraded-bg` | 4.5 | 4.591 | 5.602 |
| `--control-border` on `--field` | 3 | 3.748 | 3.670 |
| `--control-border` on `--surface` | 3 | 3.592 | 3.822 |
| `--control-border` on `--surface-2` | 3 | 3.322 | 3.591 |
| `--control-border` on `--bg` | 3 | 3.149 | 4.120 |
| `--accent` on `--accent-field` (filled field) | 3 | 5.516 | 8.203 |
| `--accent` on `--surface` (filled field) | 3 | 5.588 | 9.285 |
| `--accent` on `--surface-2` (filled field) | 3 | 5.169 | 8.723 |
| Tone text on its fill: `--progressing`, `--unknown`, `--neutral`, `--missing`, `--applied`, `--locked` | 4.5 | 4.517 to 11.354 | 6.404 to 10.864 |
| `--healthy` and `--degraded` on their fills, dark | 4.5 | n/a | 8.185 and 5.941 |

Pending (light only): `--healthy` on `--healthy-bg` 4.450, closed by `--healthy-ink` on `--healthy-bg`,
and `--degraded` on `--degraded-bg` 4.414, closed by `--degraded-ink` on `--degraded-bg`. The
`--<tone>-ink` tokens come from `align-shell-and-tokens`, which keeps `--<tone>` as the border
token (its design, "Tone tokens"); the old pairs stay at 4.450 and 4.414 after that change, and
only the replacement pairs pass. This change may not change another token. The thinnest margin of
the list is 4.517, for `--muted` and `--neutral` on `--neutral-bg` in the light theme.

**Rationale**: A pure Go function over hex values is fast, runs on every `task check`, and needs no
browser. It cannot see translucent layers or the page's grid and glow, which is why the browser pass
of task 2.3 exists. The pending list records known failures in code instead of in a note.
**Alternatives**: Leaving the two failing pairs out hides them. Putting them in the main list makes
the test red. A "must still fail" assertion would force the next change to edit the test to turn
green. The pending list with a stale check avoids all three.
The pending list is an addition to the brief; the supervisor accepted it at the proposal gate.

#### 7. Recording the floor

**Decision**: The floor is a requirement of the web-ui spec delta (above), and this change records
it in `docs/DESIGN.md` as `portal:D19:R6`, the next free requirement of D19, keeping every existing
number. PR 40 merged on 2026-10-11, so D19 is on the base, and the owner answered yes to
design/T3.4 NQ8 the same day. `align-shell-and-tokens` extends R6 and does not state it again.
**Rationale**: D19 says the tokens carry the canvas's values. The canvas's `--muted` and control
border fail AA (T2.3 section 10.1), so the decision text should state the floor. Nothing on `main`
may cite a decision that `main` lacks.

## Evidence (specify, 2026-10-11)

Method and limits: a scratch run outside the repo, on a worktree of base `c60cf02`, with
`OPM_PORTAL_UI_DEV=127.0.0.1:18080 go test ./internal/ui -run TestDevServe`, and the pinned image
`mcr.microsoft.com/playwright/python:v1.63.0-noble` with the repo's hash-locked `playwright`
requirements. Nothing in the repo was changed. The scripts are in the swarm scratchpad, not
committed.

- **Phone width on base.** 25 pages (every non-fragment entry of `f1Pages`) at 360 by 800, in
  Chromium, Firefox and WebKit, light and dark: 150 of 150 loads report `scrollWidth` 360 for the
  document and the body. No element reaches past 360 outside a scrolling ancestor. The scratch run
  used a height of 800 and did not open `/nope`; the test uses 360 by 640 and adds `/nope`, and
  task 1.4 is the first measure of that height and that page.
- **The measure can fail.** Appending a 500 px wide `div` to `main` on `/installed` raises
  `scrollWidth` to 516 in all three engines, although `body` computes `overflow-x: hidden`.
- **Formula ratios** (WCAG 2, not rounded; token hex values): table in Decision 6. On the current
  values, `--muted` is 4.165 on `--bg` and 4.394 on `--surface-2`; `--line-strong` is 2.182 on
  `--field` (light) and 1.714 (dark). They match design/T2.3 section 10.1.
- **Rendered pixels, muted text.** For each element whose text colour is the muted token, on 12 pages
  in the three engines, text hidden and the box screenshotted, then the median pixel contrast against
  the muted colour: with `#6a6f7a` (light) 141 of 448 elements fall under 4.5; with `#5f6672` 13 of
  453 fall under 4.5 on the median pixel. Those 13 are `time` and digest elements inside closed
  `details` folds, whose boxes sample other content, so they are not measured text. The dark theme
  has 0 (Chromium, Firefox) and 1 (WebKit, also a closed-fold element). Limits: the median pixel,
  not a per-glyph measure; anti-aliasing and the 1 px grid lines pull the minimum and the 5th
  percentile to nothing useful, so only the median is used. Task 2.3 repeats the pass on the final
  files and may adjust the hex.
- **Not measured**: the border colours in a browser (formula only), forced colours, 200 percent text,
  a real device. Task 2.3 takes the border screenshots.

## Risks / Trade-offs

- [`task test:browser` runs nightly only, so a width regression is caught the night after it merges]
  -> The same holds for every browser test today. A pull-request job is out of scope; the change
  that adds one can use the same task.
- [The page list drifts from `f1Pages`] -> Each UI change lists its pages (the spec requirement); a
  reviewer compares the two lists.
- [`#5f6672` is thin on the neutral fill (4.517 against 4.5) and on the page background (4.78)] ->
  Task 2.3 measures rendered pixels before the value ships and may darken it; the contrast test
  then holds whatever value ships, on both fills.
- [A darker light `--muted` lowers the hierarchy between `--ink-2` and `--muted`] -> `--ink-2` is
  `#3a4150` (8.46:1 on the page), so the steps stay apart; the screenshots in task 2.3 show
  light and dark.
- [A rule that reads `--line-strong` for a control later] -> The contrast pairs name
  `--control-border`; a new control uses it, and a reviewer checks.
- [The pending list is seen as permission to leave a failure] -> Each entry names the replacement
  pair that closes it, and the test fails when that pair's tokens exist in `portal.css`;
  design/T3.4 amendment SH-A1 names the change that adds them.

## Migration Plan

Not applicable: CSS values and tests, no data or API. Rollback is a revert of the two section
commits. Section 1 alone leaves `main` releasable; section 2 changes only token values and adds a
test.
