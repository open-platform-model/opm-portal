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
  Links are brass.
- **Badges, tabs and the live mark** (section 3): Applied and Health share one square, uppercase
  badge in their tone. The `APPLY |` prefix and the health dot go; each badge still names its axis
  to assistive technology. Tabs on every page become underline tabs and carry counts the page
  already holds. A region opened by a tab no longer shows its own heading, which stays for screen
  readers. The live mark reads "Live", "Not live", "Offline" and so on, with a steady dot.
- **The state block** (section 4): one shared component for the summary cards: eyebrow, a
  recorded time, a tone icon, a big uppercase state word, a summary line and reason count links,
  with locked and degraded forms. This change builds and tests it. `align-platform-installed-catalog`
  and `align-owner-pages` put it on their pages.
- **Docs and evidence** (section 5): screenshots of every page in light, dark and at 360 px;
  `docs/site/` pages that describe the old badge shapes or wording; `ROADMAP.md`.

The planning PR records in `docs/DESIGN.md` the owner's decision as portal:D19 (the canvas's
flat look, with the badge ruling under it). It also opens one question and widens another, both
docs only. The new question, portal:OQ26, is whether the controller can record a per-catalog
resolve time and digest (gap `catalog-09`, opm-operator#230). The widened one is portal:OQ25,
which now also covers a catalog's own description (gap `catalog-03`).

Not in this change: anything one page owns, which the three follow-on changes rebuild. That
includes the Platform status card, the Installed card, the Installed filters, the catalog
blocks, the owner summary cards, the Resources and Events tables, the conditions fold and the
graph's drawing. The canvas's "Ready / Not ready" words for the Applied axis also stay out: the
Applied axis keeps the controller's state words (portal:D3:R1; supervisor ruling 2026-10-06).

## Gate

`align-platform-installed-catalog`, `align-owner-pages` and `align-graph` each depend on
align-shell-and-tokens. They use its tone classes, its tab component and its state block, and
none of them starts before this change merges. This change edits `owner.html`, `catalog.html`,
`platform.html`, `partials.html`, `panel-body.html` and `portal.js` only for shell-level markup:
tab counts, region headings, badge markup and the panel request. That edit lands before the
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
  block view type and partial. Tests and goldens change too. `test/browser/graph.py` and
  `test/browser/expired.py` change. `api/v1alpha1`, `internal/api`, `internal/readmodel`,
  `internal/graph` and `internal/health` do not change.
- API: none. `task api:breaking` is unaffected.
- Pages: every page, through the shell and the shared CSS; tab strips on the Platform, Catalog,
  instance and package pages.
- Principle V: no new read, no new kind, no write, no Secret, no values. Every tab count comes
  from a document the page already fetches. A count over a list the caller may not read in full
  is not shown.
- Principle VII: no dependency and no build step. The state block lands one section before any
  page uses it. Two follow-on changes run in parallel and both need it, so one copy here beats
  two copies there. A view test exercises every tone and every form.
- SemVer: MINOR after 1.0. Pages change, and no API or flag changes. On the 0.x line it ships as
  one PR titled `feat(ui): follow the canvas's flat look, tokens, tabs and state block`, which
  cuts a minor release. The section 1 bug fix rides in the same PR.
- Decisions: implements portal:D19 (the canvas's flat look) and
  keeps portal:D3 (two axes, the Applied words), portal:D14 (nothing new stored) and portal:D17
  as they are. portal:OQ25 is widened and portal:OQ26 is added, both docs only.
- Main-spec requirements touched, so the parallel changes can avoid them: `web-ui`'s "Applied and
  health are two badges, never one" and "An expired session stops the page's stream" (MODIFIED).
  Five requirements are ADDED under new names.
