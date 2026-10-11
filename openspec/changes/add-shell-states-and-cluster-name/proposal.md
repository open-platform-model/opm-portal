## Why

The portal's pages say "live" and then go quiet when something is wrong. Four gaps remain after
the canvas alignment (`align-shell-and-tokens`, `align-platform-installed-catalog`,
`align-owner-pages`, `align-graph` and `fix-contrast-and-add-phone-check`, all merged and built on
`main`):

- A relative time such as "4m ago" is written once at render and freezes. An operator who leaves a
  page open reads a time that was true when the page loaded.
- A slow request shows nothing. A click on a graph node or a tab can take seconds on a large
  cluster, and the page looks dead.
- A lost stream or an expired session is only a few words in the header's live mark. The page
  keeps its last render with no visible sign that it is old, and no way to get out except to know
  that a reload fixes it. After the stream reconnects, the page also keeps showing the regions it
  had before the outage until something changes (design.md, "Reconnecting").
- The details panel is not part of the live refresh. A reader who selects the Deployment node sees
  "Progressing" in the panel while the graph beside it already shows "Healthy". This is the
  portal showing two answers for one object (Principle IV: status is read, never inferred).

Separately, portal:D18 says the header names "the kubeconfig context and the name of its cluster
entry". The header shows only the context. The read API already serves the entry name.

Source of the work: `agent-swarm/opm-portal-v1/design/T3.4.md` section 3.8 (E5), from plan
slice S3 and `design/T2.3.md` sections 11 and 12 (UI states S3, S4, S6, S9). The `Tip` of S3 is
not here: `align-shell-and-tokens` owns it.

## What Changes

- **Relative times keep counting.** Every time the pages print as relative text ("12s ago",
  "4m ago", "3h ago", "2d ago") is rewritten by the page script every 30 seconds from its
  `datetime` attribute, as a text node, in the same words the server uses. A time inside a live
  region is marked so the rewrite is not announced.
- **A slow request shows a loading state after 1 second.** A 2 px bar under the header, and, when
  the slow request fills the details panel, `aria-busy="true"` and a visible "Loading..." line in
  it. The bar is static under `prefers-reduced-motion`. The count of open requests, not a flag,
  decides when it ends. Background refreshes from the stream show nothing.
- **A lost stream and an expired session show a banner.** A visible line in the sticky header, so
  a reader scrolled down a long page still sees it, says "Showing data read 4m ago. Reconnecting."
  or "Session expired. Showing data read 4m ago." with a Reload link. The time has the form of the
  existing `time` partial (relative text, the absolute UTC time in `title`) and keeps counting. The
  banner is not a live region and has no role: the existing live mark `#live` stays the one
  announcer. When the stream comes back, the page re-renders every region it follows once, then
  the banner goes; until that fetch succeeds the live mark keeps the word of the loss, and the
  banner keeps the time of the data on screen, which the reopened stream does not move. A failed
  fetch is tried again on the next stream event. The page keeps clear of the taller header:
  `html` carries `scroll-padding-top` equal to the header's height.
- **The header names the cluster entry.** When the `Cluster` document carries an entry name that
  differs from the context name, the cluster pill shows both, each with its word. It wraps at
  360 px with the longest name. No read, no API change (design.md, "The cluster entry name").
- **The details panel follows the page, or says it is old.** A panel that shows a graph node (or
  the resting node) is replaced when the page's topic changes, from the same one fetch as the
  other regions. A YAML or Events view is not re-fetched: it shows a note, "Read 4m ago. The
  object may have changed since.", with a link that reloads the view. A keyboard reader on one of
  the panel's links keeps focus through the replacement.
- **The leftover `reveal` classes go.** `align-shell-and-tokens` removes the animation CSS; this
  change, which lands last, removes the `reveal` class from the templates.

Section 1 is a spike. The facts the stale panel, the banner and the header's height rest on are
read from source and not yet run in a browser (design.md, "Assumptions"). The spike runs them and writes the finding
into design.md before any code.

**Builds on changes now merged and built on `main`** (PRs 43, 44, 47, 48 and 49). The table names
the files each touched and what this change relies on; section 1 re-read them and design.md carries
the current line numbers:

| Change | Expected to touch first | What this change relies on |
| --- | --- | --- |
| `fix-contrast-and-add-phone-check` (E0) | `test/browser/` (a phone test), `cmd/opm-portal/browser_test.go`, the `-run` pattern at `Taskfile.yml:90`, a contrast pair test in `internal/ui` | `TestBrowserPhone` and its page list, which this change extends; the pair list, which gets the loading bar and the banner |
| `align-shell-and-tokens` (E1) | `internal/ui/static/portal.css` (tokens, no `pulse`), `static/portal.js` (`setLive` words, `openNode` with `select: "unset"`), `templates/layout.html` (initial "Not live"), `templates/partials.html` (`state-block`, `tip`), `test/browser/expired.py` | The live mark's capitalised words ("Reconnecting", "Offline", "Session expired, reload"), which the banner repeats and extends; the `time` partial unchanged under the state block; the tone tokens the banner uses |
| `align-owner-pages` (E3) | `internal/ui/owner.go` (`ownerTabsOf`), `templates/owner.html` (`detail` define, `data-root-panel`), `internal/ui/panel.go`, `templates/panel-body.html`, `static/portal.js` (`resetDetail`), `Taskfile.yml`, `AGENTS.md` | The details panel at rest, and `#detail` carrying `data-root-panel`; the panel body this change marks |
| `align-graph` (E4) | `internal/ui/graph.go` (`restingNode`), `internal/ui/owner.go` (`node` and `focus` parsing), `static/portal.js` (graph selection, `markSelected`, `resetDetail`), `panel.go`, `panel-body.html` | A click writes `node` to the address with `replaceState`, so the page render at that address holds the selected panel; `data-resting` on the resting node |
| `align-platform-installed-catalog` (E2) | `internal/ui/filters.go`, `partials.html` (`filter-form`, `events-table`), `static/portal.js` and `prefs.js` (filter code), `Taskfile.yml` | Nothing. It shares `portal.js` and the `-run` pattern, so its merge order matters (design.md, "Shared files") |

## Capabilities

### New Capabilities

None. The behaviour belongs to the existing `web-ui` capability.

### Modified Capabilities

- `web-ui`: modifies "The header names the cluster, the reader and the version" (adds the
  cluster entry name) and "An expired session stops the page's stream" (adds the banner on top of
  `align-shell-and-tokens`' wording); adds "Relative times are rewritten as they age", "A rewritten
  time changes only its text", "A request open for a second shows a loading bar", "A slow panel
  request marks the details panel busy", "A lost stream shows a banner", "The banner and the stale
  note print their time like every other time", "A stream that returns re-renders before the
  banner goes", "A resync that has not succeeded keeps the banner", "The banner's time is the time of the data on screen", "A failed resync is tried again", "The sticky header never covers a focused element", "A node panel is re-rendered with its page" and "A YAML or
  Events view says when it is old".

## Impact

- **Packages.** `internal/ui` (view types for the header and the panel marker, templates,
  `static/portal.js`, `static/portal.css`, tests and goldens); `internal/api/apitest` (a test
  helper only, so a fixture can serve a cluster entry that differs from the context);
  `test/browser` and `cmd/opm-portal` (browser tests); `Taskfile.yml` (the `-run` pattern);
  `AGENTS.md` (the browser test lines); `docs/site`, `ROADMAP.md`. No change to `api/v1alpha1`,
  the handlers of `internal/api`, `internal/readmodel`, `internal/stream` or `deploy/`.
- **Pages.** All pages (the header, the banner, the loading bar, times); the instance and package
  pages (the details panel). The Catalog page has no panel and takes the first three behaviours.
- **SemVer.** MINOR after 1.0: pages change and nothing else does. On the 0.x line it ships as one
  PR titled `feat(ui): show a lost stream and an expired session, keep times and the panel current and name the cluster entry`.
- **Principle V (read-only, least privilege).** No new read, no new kind, no write, no review, no
  identity change. The cluster entry name is already in the `Cluster` document the header reads
  (`api/v1alpha1/types.go:381-384`, `internal/api/handlers.go:130`), which sends no review and
  reads nothing from the cluster (`internal/api/handlers.go:116-119`). The panel refresh calls the
  page render the portal already serves and a stale note adds no request; both go through the
  existing authorization (portal:D7). Principle V is unchanged.
- **Principle VII (no build step, browser keeps only display preferences).** No dependency. The
  script adds timers and DOM text; it stores nothing in `localStorage` or elsewhere. The server
  time the script needs for clock skew is one `meta` value in the page.
- **Security.** Every text the script writes comes from the page's own words or from numbers it
  formats, and goes in with `textContent`. The banner, the loading line and the stale note carry
  no inline handler, no `style` attribute and no HTML from the cluster. The strict policy
  (`script-src 'self'`) is unchanged. The stale note and the panel marker are template text.
- **Decisions.** Implements portal:D18:R1 (the entry name) and the live mark of portal:D19.
  Changes no decision and edits no number in `docs/DESIGN.md`; no `enhancement.yaml`.
- **Not in this change.** The `Tip` and its keyboard rules (`align-shell-and-tokens`); a visible
  absolute time beside every relative time (not scheduled in track 1; a later change on the owner's ask); a banner or
  "No longer live" text on a region whose topic the server closed (`sse:closed`, unchanged);
  the Kubernetes version and Context facts on the Platform card (NQ5: header only); a count of
  data age on each region; any change to what the stream sends.
