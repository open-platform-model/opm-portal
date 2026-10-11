## Why

The redesign (`redesign-web-ui`, PR 39) built the canvas's views, but not its look. A verified
comparison of the canvas and the live pages
(`docs/design/evidence/05-canvas-gap-report/gaps.md`, section A) found twenty gaps that every page
shares: a blueprint grid behind every page, shadowed panels with corner marks and diamond
headings, a 1500 px column against the canvas's 1840 px, folder tabs without counts, Applied and
Health badges drawn in two shapes, labels in mono where the canvas uses sans, links in ink, tone
colours with one shade where the canvas has three, and an Instance chip that turns cream in dark
mode. It also found one shipped bug: with script on, selecting a graph node leaves the details
panel empty (gap `instance-43`).

The owner answered on 2026-10-06: "Follow the canvas (Recommended)": flat panels, no grid
background or shadows, plain headings, a 1840 px column. Three follow-on changes rebuild the
Platform, Installed and Catalog pages (`align-platform-installed-catalog`), the instance, package
and provider pages (`align-owner-pages`), and the graph (`align-graph`). All three draw on the
same tokens, tabs and summary block, so this change lands them first, once.

## What Changes

- **Bug fix** (section 1): selecting a graph node with script on fills the details panel again.
  The node's panel request no longer inherits the page's `hx-select="#main"`. A browser test
  drives the real instance page, so the bug cannot come back unseen.
- **Tokens and the flat look** (section 2): every tone gets a border, an ink, a background and a
  tint, in light and dark, with the canvas's values. The Instance and Package kind chips get their
  own tokens, so the Instance chip is slate in dark mode. The page background is flat, panels lose
  their shadow and corner mark, and `h2` loses its diamond and grows to 20 px. The page column and
  footer grow to 1840 px. Summary rows use equal auto-fit columns. Body text is 16 px, `h1` is at
  most 40 px, mono text is 13 px, and kickers, labels, table heads and fact labels are sans.
  Links are brass. No element animates in on page load: the staggered entrance goes. A contrast
  test covers every new colour pair, and `docs/DESIGN.md` records the floor as a new D19 requirement.
- **Badges, tabs and the live mark** (section 3): Applied and Health share one square, uppercase
  badge in their tone, taken from the state class every badge already carries. The `APPLY |`
  prefix and the health dot go; each badge still names its axis to assistive technology, and
  where both axes stand together each is named in visible text. Tabs on every page become
  underline tabs and carry counts the page already holds, refreshed live with the page. A region opened by a tab no longer shows its own heading, which stays for screen
  readers. The live mark reads "Live", "Not live", "Offline" and so on, with a steady dot. In
  forced-colours mode the badges keep a system-colour border, and the partial health keeps its
  dashed one. The 360 px phone test also covers the details panel after a node selection.
- **The state block and the tooltip** (section 4): one shared component for the summary cards:
  eyebrow, a recorded time, a tone icon, a big uppercase state word, a summary line, note lines, a
  caption, reason count links and footer links, with locked and degraded forms; and one info
  tooltip. This change builds and tests both. `align-platform-installed-catalog` and
  `align-owner-pages` put them on their pages. The tip can be dismissed with Escape, hovered and
  opened by a tap, with a small script over its CSS-only base (WCAG 2.2 1.4.13).
- **Docs and evidence** (section 5): screenshots of every page in light, dark and at 360 px;
  `docs/site/` pages that describe the old badge shapes or wording; `ROADMAP.md`.

The planning PR records in `docs/DESIGN.md` the owner's decision as portal:D19 (the canvas's
flat look, with the badge ruling under it). It also opens one question and widens another, both
docs only. The new question, portal:OQ26, is whether the controller can record a per-catalog
resolve time and digest (gap `catalog-09`, opm-operator#230). The widened one is portal:OQ25,
which now also covers a catalog's own description (gap `catalog-03`).

Open deviation for the owner's review: the Catalog page's Events tab carries no count, where the
canvas shows "Events 2". The page reads events only on that tab, and a count would add a read on
every tab. This follows the planner's reading of portal:D19:R4, not a supervisor ruling.

Not in this change: anything one page owns, which the three follow-on changes rebuild. That
includes the Platform status card, the Installed card, the Installed filters, the catalog
blocks, the owner summary cards, the Resources and Events tables, the conditions fold and the
graph's drawing. The canvas's "Ready / Not ready" words for the Applied axis also stay out: the
Applied axis keeps the controller's state words (portal:D3:R1; supervisor ruling 2026-10-06).

## Gate

This change needs `fix-contrast-and-add-phone-check` merged first: that change adds the
phone-width test and the contrast test that sections 2, 3 and 4 extend, and the sections that
extend them start after `main` is merged into the branch.

`align-platform-installed-catalog`, `align-owner-pages` and `align-graph` each depend on
align-shell-and-tokens. They use its tone classes, its tab component and its state block, and
none of them starts before this change merges. This change edits `owner.html`, `catalog.html`,
`platform.html`, `partials.html`, `panel-body.html` and `portal.js` only for shell-level markup:
tab counts, region headings, badge markup (the hover card's marks in the `graph` define included)
and the panel request. That edit lands before the
follow-on changes start.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-ui`: the shared look (tokens, flat panels, column, typography, links, kind chips), one
  badge shape for both axes, underline tabs with counts, the worded live mark, the state block,
  and a node selection that fills the details panel with script on.

## Impact

- Packages: `internal/ui` only. That covers `static/portal.css`, `static/portal.js`, the
  templates, `view.go`, `owner.go`, `catalog.go` and `platform.go` (tab counts), and a new state
  block view type and partial, and the tooltip partial. Tests and goldens change too.
  `test/browser/graph.py`, `test/browser/expired.py`, `test/browser/theme.py` and the
  `TestBrowserGraph` doc comment in `cmd/opm-portal/browser_test.go` change. The floors add
  `internal/ui/contrast_test.go` (extended), `test/browser/phone.py` (extended), a new
  `test/browser/tip.py` with `TestBrowserTip` and its name in the `-run` pattern of `Taskfile.yml`,
  a check in `test/browser/theme.py`, a requirement in `docs/DESIGN.md`, and a small script in
  `portal.js` for the tip. `api/v1alpha1`, `internal/api`, `internal/readmodel`,
  `internal/graph` and `internal/health` do not change.
- API: none. `task api:breaking` is unaffected.
- Pages: every page, through the shell and the shared CSS; tab strips on the Platform, Catalog,
  instance and package pages.
- Principle V: no new read, no new kind, no write, no Secret, no values. Every tab count comes
  from a document the page already fetches. A count over a list the caller may not read in full
  is not shown.
- Principle VII: no dependency and no build step. The state block lands one section before any
  page uses it. Two follow-on changes run in parallel and both need it, so one copy here beats
  two copies there. A view test exercises every tone and every form. The tip gains a small script (Escape, tap) over
  its CSS-only base, because WCAG 2.2 1.4.13 asks for a tip that can be dismissed and CSS alone
  cannot close a tip that still has the pointer or focus.
- SemVer: MINOR after 1.0. Pages change, and no API or flag changes. On the 0.x line it ships as
  one PR titled `feat(ui): follow the canvas's flat look, tokens, tabs and state block`, which
  cuts a minor release. The section 1 bug fix rides in the same PR.
- Decisions: implements portal:D19 (the canvas's flat look), adds one requirement to it (the
  contrast floor, next free number, every existing number kept), and
  keeps portal:D3 (two axes, the Applied words), portal:D14 (nothing new stored) and portal:D17
  as they are. portal:OQ25 is widened and portal:OQ26 is added, both docs only.
- Shared pieces the follow-on changes take as they ship: the `stateBlock` fields (`When` is a
  `*time.Time`), the `tip` partial, `tabLink.Count` and `Follow`, and the badge colours from the
  state classes.
- Main-spec requirements touched, so the parallel changes can avoid them: `web-ui`'s "Applied and
  health are two badges, never one" and "An expired session stops the page's stream" (MODIFIED).
  The other requirements are ADDED under new names. Each stays under 500 characters, since
  `openspec validate --strict` warns above that: the five requirements of the first draft were
  split without changing a sentence, and the floors of the plan add their own.
