## Why

The redesign (PR 39, archived change `redesign-web-ui`) built the Platform page, the Installed
list and the Catalog page to recorded data, but a side-by-side check against the owner's canvas
(`docs/design/evidence/05-canvas-gap-report/`, section B, 46 verified gaps) shows they still read
differently from what the owner reviewed. The large ones:

- The Platform status and the catalog's Resolved block are small pills. The canvas gives each a
  toned state block with a big state word, a summary and reason counts that link to the cause.
  Refused registrations do not show on the Platform status card at all.
- Installed filters are free-text boxes that apply only on Enter or a button. Their totals shrink
  to one namespace when a namespace filter is set, and packages cannot be filtered by their
  source. On the canvas, every filter is a select listing the values present, with counts, and
  it applies as the user types or picks.
- The Catalog page's headline renders at body size because of a CSS bug (`<h1 class="mono">`),
  its Events tab has no filter, and its source facts are run-on sentences.
- Events on the Platform and Catalog pages are stacked cards. The canvas draws a table whose
  Resource cell filters the feed.

The owner chose on 2026-10-06 to follow the canvas's look (decision 1, portal:D19). The supervisor
ruled that everything not decided otherwise follows the canvas, that events become tables, and that
the Applied axis keeps today's words (portal:D19:R3). This change applies that to the three
list-and-summary pages.

**Gate:** depends on `align-shell-and-tokens`. That change lands the flat look, the 1840 px
column, the tone ink and kind chip tokens, the square badges, the underline tabs with counts, the
shared state block partial and the shared tooltip, and this change uses all of them. Start this change only after
`align-shell-and-tokens` has merged.

## What Changes

- **Filter forms** (all `form[data-filters]`): fields in the view's parameter order in one
  wrapping row. A filter whose values are facts of the rows (namespace, uses, module, provides)
  becomes a select of the values present. Every select option carries its count over every row
  the caller may read, and the first option has a per-filter "any" word ("Any kind", "Anyone").
  The search box gets a magnifier icon. Search and text fields apply as the user types (debounced,
  keeping focus, caret and text typed meanwhile), a filter submit replaces the history entry
  instead of pushing one, and the Apply button is hidden when script runs until it takes keyboard
  focus. An option over a list the caller may not read carries no count. A row that is always shown holds
  "Showing N of M", chips reading "Label: value" and "Clear all". The URL contract is unchanged
  (portal:D14).
- **Installed**: when the caller may list everywhere, the list is fetched unscoped and filtered by
  namespace on the server, so the total and the counts cover every readable namespace. A reader
  who may not list everywhere keeps today's scoped read and a typed namespace. The module filter
  also matches a package by its source. Search matches the kind word. Whole rows are links, and
  Degraded rows are tinted. The table has fixed column widths. Kind chips get icons. The provider
  badge reads "Provider" with a keyboard-reachable tooltip naming its standing. The module or
  source cell has a version line, or for a package its short revision digest. A styled empty
  state is shown when nothing matches.
- **Platform page**: the status card becomes a state block: the applied state word, "since",
  the summary line, and reason counts for refused or removal-blocked registrations and unfulfilled
  contracts, each linking to the cause. The Installed card shows Health and Applied as two
  columns of count rows. The identity facts read "Controller". Provider rows show square
  acceptance and activation badges without the duplicate verdict, link the provider name, and
  show contract pills. Catalog rows read "subscription" or "contributed", show an Enabled badge,
  and link claimants. The contracts note is unboxed. Recent events become a table: the
  resource select sits in the heading row, its options carry counts, each row's Resource
  filters the feed, and ages are compact. The theme menu's check becomes an SVG.
- **Catalog page**: the headline is the short name in the display font, with a "path · version"
  line under it. An origin pill sits under the eyebrow. The source is "Platform subscription" or
  the contributing or claiming registration, linked to its Provider tab. A catalog not in the
  registry reads "no: not in the registry". The breadcrumb uses the short name. The Resolved
  block is a state block with a summary, a "refused since" time for an unresolved catalog and a
  reason count that opens the Events tab filtered by that reason. The Events tab gains a Type
  filter, "Showing X of N" and the events table.
- No read API change, no new read, no new route.

Not in this change: the Applied axis words "Ready / Not ready" (platform-41; supervisor ruling,
portal:D19:R3: the canvas's words lose Failed, Stalled and Reconciling); per-catalog resolve time,
registry and digest (catalog-06, catalog-09; portal:OQ26, opm-operator#230); the gaps assigned to the other three
changes; and every section X gap (decided out, or needing the controller or the registry).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-ui`: the Platform page, the Installed list, the Catalog page and page vocabulary change
  as above. New requirements cover filter forms (choices with counts, applying as the user
  types), counting Installed across namespaces, and matching a package's source with the module
  filter.

## Impact

- Packages: `internal/ui` only (`filters.go`, `pages.go`, `platform.go`, `catalog.go`, `view.go`,
  templates `installed.html`, `platform.html`, `catalog.html`, `partials.html`, `layout.html`
  (theme menu check only), `static/portal.css`, `static/portal.js`, `static/prefs.js`, tests and
  UI goldens), `test/browser/` (a new `filters.py`), `cmd/opm-portal/browser_test.go`
  (`TestBrowserFilters`), and `Taskfile.yml` (the `test:browser` pattern). `api/v1alpha1`,
  `internal/api`, `internal/readmodel`, `internal/graph` and `internal/health` do not change.
- Pages: `/`, `/installed`, `/catalog`.
- Shared with `align-owner-pages` (it runs after `align-shell-and-tokens`, in parallel with this
  change): both changes render events as a table and both use the filter-form behaviour. This
  change owns the generic filter code: the `filter-form` partial, `filters.go`, the filter
  handlers in `portal.js`, and the `opm-js` class `prefs.js` sets. The `events-table` partial in
  `partials.html` belongs to whichever of the two changes lands first, and the second one adopts
  it rather than redefining it. design.md gives the partial's shape so both build the same one.
  The two changes modify different `web-ui` requirements. This change does not touch owner
  pages, the details panel, the graph or `internal/ui/graph.go`.
- Three edits reach pieces `align-owner-pages` owns or uses, each made so that its files keep
  working whichever change lands first (design.md, "the pieces this change changes under C"):
  `newFilterForm` keeps its signature as a wrapper, so `owner.go:588` is not edited here;
  `providerBadge` stays as it is and the Installed pill is a new `providerPill`, so the owner
  kicker keeps its standing word; the tooltip CSS comes from `align-shell-and-tokens`, not from
  this change.
- `align-shell-and-tokens`' `state-block` partial is used as it ships: it already gives a reason
  its own class, lets a reason show without a count, and carries note lines (the locked refusals
  line). This change does not edit it.
- Principle V: no new kind is read, no new verb, no new identity. Installed makes one extra
  `list` call per kind only when the unscoped read comes back forbidden and a namespace filter
  is set. That call is the scoped read the page makes today. Every new fact (registration
  reasons and condition times, package revision digests, event counts) comes from documents the
  page already fetches.
- Principle VII: no dependency and no build step. The script additions are a debounced submit and
  focus handling in `portal.js`, plus one class set in `prefs.js`. Tooltips are pure CSS.
- SemVer: MINOR after 1.0 (UI behaviour and look, no API change). On the 0.x line it ships as one
  PR titled `feat(ui): align the Platform, Installed and Catalog pages with the canvas`, which
  cuts a minor release.
- In-cluster: the events tables, the contracts note's controller message and the provider rows'
  messages are new places for operator-written text; the in-cluster suite covers each.
- Decisions: implements portal:D17 (the canvas is the target, cut to recorded data) and portal:D19
  (R1 the flat look, R3 the Applied words, R5 recorded times only), defers catalog-06 and
  catalog-09 to portal:OQ26, and keeps
  portal:D3:R1/R8, portal:D4:R4/R7, portal:D7:R2, portal:D9:R2/R4, portal:D14:R1-R3 and
  portal:D16:R2 as they are. It adds portal:D14:R7 (filter choices come only from
  rows the caller may read), recorded by the planning PR.
