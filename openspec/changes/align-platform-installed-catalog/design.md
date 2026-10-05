## Context

`redesign-web-ui` (PR 39, main ad8b218) built the Platform page, Installed and the Catalog page.
The gap report `docs/design/evidence/05-canvas-gap-report/gaps.md`, section B, lists 46
verified differences between those pages and the owner's canvas
(`docs/design/evidence/03-ui-canvas/Main.dc.html`, `Instances.dc.html`, `Catalog.dc.html`).
This document says how each one is closed and cites its gap id, so every id can be traced to a
decision here and to a task.

The change starts after `align-shell-and-tokens` (A) merges, and uses what A provides:

- the flat look and the 1840 px column (platform-01, platform-28),
- the tone ink tokens `--<tone>-ink` and the dark tone values (platform-31),
- the kind chip tokens (installed-10, platform-39),
- square uppercase Applied and Health badges with no `APPLY` prefix (platform-38),
- underline tabs with counts on the Platform and Catalog pages (platform-09, catalog-16),
- sans field labels (`.facts dt`, catalog-04), kickers and table heads (platform-40),
- hidden panel headings inside tabbed panels (catalog-27),
- and the shared **state block** partial.

A defines the state block (its design.md, "The state block"): a `stateBlock` struct with
`ID`, `Label`, `Eyebrow`, `When *stamp`, `WhenText`, `Hue`, `State`, `Summary`,
`Reasons []countLink`, `None`, `Follow` and `Problem`, drawn by the `state-block` partial. The
icon follows the hue: a check for applied, healthy and neutral, a bang for degraded, turning arrows
for progressing, a clock for unknown, the lock for locked. This change fills the struct and does
not edit A's partial. The two forms it needs ship in A's partial from the start (integrator ruling
2026-10-06, so that B and C, running in parallel, do not both edit it): a reason anchor takes its
`countLink.Class`, so each reason takes its own hue as the canvas does (a red refusal next to an
amber contracts reason), and a reason with `N` zero shows only its reason, with no count.

`align-owner-pages` (C) also starts after A, and runs in parallel with this change. Both changes
draw events as a table and both depend on filter-form behaviour. The ownership split is in
"Shared with align-owner-pages" below.

Nothing here reads anything new. Every fact comes from documents the three pages already fetch:
`Platform` (conditions, registrations with their conditions, reasons and holders, catalogs,
subscriptions), `InstanceList` and `PackageList` (including `sourceArtifact`), and the Platform
and registration `EventList`s. Claims about what the controller writes are those of the archived
redesign design.md, checked against opm-operator 277ca18 and the F1 capture
(`testdata/clusters/f1/`).

## Goals / Non-Goals

**Goals:**

- Close every section-B gap except platform-41, following the canvas where no decision says
  otherwise.
- Keep URLs, the remembered-filter contract and locked behaviour exactly as they are
  (portal:D14, portal:D7).
- Script stays optional: every filter, link and tooltip works without script. Script only adds
  apply-as-you-type and hides the Apply button.

**Non-Goals:**

- **platform-41**, the Applied words "Ready / Not ready / Managed externally". The supervisor
  ruled on 2026-10-06 that the Applied axis keeps today's words (Applied, Reconciling, Failed,
  Stalled, Suspended, Managed externally, Unknown), because "Not ready" merges three states the
  user must tell apart. The Platform state block therefore says "Applied", not "Ready". The owner
  may overturn this in review; doing so changes only the text column of `appliedStates`.
- Everything section X of the gap report lists for these pages, decided out or needing a
  source: catalog definitions, contracts, Used by and transformers (catalog-11, -14, -15, -22;
  portal:D17, portal:OQ25), description (catalog-03, portal:OQ25), registry and digest
  (catalog-06), a per-catalog resolve time (catalog-09, the open question A adds for
  opm-operator#230), remembered catalog tabs (catalog-17, portal:D14), Ready-shaped Applied and
  Health badges (installed-12, -13, portal:D3:R1), packages in the Uses filter (installed-20,
  portal:D16:R2), the Kubernetes and Context identity rows (platform-03, portal:D18), Platform
  health (platform-06, portal:D17:R4, portal:OQ22), the Provides popover (platform-14), and the
  live-only verdict stripe, Conditions panel and footer (platform-16, -22, -44).
- Owner pages, the details panel, the graph and its script. Those belong to C and D.
- Live counts in select options. Option counts are computed when the page renders. A live
  refresh updates the rows and the "Showing" count but not the option labels (see Risks).

## Decisions

### Filter forms (installed-01, -02, -03, -04, -05, -06, -21, -22; platform-10, -11, -43)

`filters.go` changes as follows. The URL and the remembered-filter spec (`filterSpec`) keep their
meaning.

```go
type filterParam struct {
    Name, Label string
    Values []string          // fixed enumerated values; nil = free text or dynamic
    Text   map[string]string // value -> option and chip text
    Any    string            // first option's text; "" means "Any"
    // Choices: a free-text parameter drawn as a select of the values the
    // view supplies (namespace, uses, module, provides). It is still
    // validated as free text, so prefs.js and parseFilters treat it as today.
    Choices bool
    // Chip formats the chip text of a value (module: last path segment;
    // uses and provides: contractShort). Its full value goes in the chip's title.
    Chip func(string) string
}

type field struct { // one control, in the view's parameter order
    ID, Name, Label string
    Search          bool           // the wide search box with its magnifier
    Select          []selectOption // nil for a text input
    Value, Placeholder string
}

type filterForm struct {
    Action, Aria string
    Fields       []field // replaces formFields{Selects, Texts}
    Filters      filters
    Hidden       []hiddenField
    // Shown/Total/Scope draw the always-present count row; Total < 0 hides it.
    Shown, Total int
    Scope        string // "" or e.g. "in default" when the total is one namespace
    ShownID      string // id of the followed count region, see below
    ClearText    string // "Clear all" on list views
}

// newFilterForm takes per-parameter choices and counts.
func newFilterForm(f filters, aria string, hidden []hiddenField,
    choices map[string][]string, counts map[string]map[string]int,
    placeholders map[string]string) filterForm
```

- **Order and row** (installed-02, platform-10): one loop over `Params` renders the search box
  first only because it is first in `Params`. Each view's `Params` order follows the canvas.
  Installed: q, kind, provider, uses, namespace, health, applied, owner, module. The form is a
  flat strip: no inset box, a bottom rule, and the Showing count right-aligned in the field
  row.
- **Choices** (installed-01, platform-11): `namespace`, `uses` and `module` on Installed, and
  `provides` on Providers, become selects of the values found in the rows the caller may read,
  sorted. A `uses` or `provides` option shows `contractShort` text, and its value is the full
  contract. Module options are the module paths, plus each package's source string (see
  "Installed"). A URL value that is not among the choices is still applied: it renders as an
  extra selected option, so the form never silently drops a filter. **Locked namespace:** when
  the instance or the package list is locked cluster-wide, the namespace field stays a text input
  with its datalist, so a namespace-only reader can still type a namespace they may read. The
  locked panel and `run-the-portal-locally.md` tell that reader to do exactly that.
- **Counts** (installed-21, platform-11, -43): every option of every select reads
  `Text (n)`. `n` is counted over the rows before filtering, from the same loop that collects
  the choices. When a list behind the counts is locked or failed, the counts cover only the
  readable rows, and the form says "counts cover what you may read" (portal:D7:R2: a count never
  stands for a list the caller may not read). The Platform tabs count over the registrations or
  catalogs the Platform document carries.
- **Any words** (installed-03, platform-11): Installed uses "Any kind", "Anything" (provider,
  uses), "All namespaces", "Any health", "Any state" (applied), "Anyone" (owner), "Any module".
  Providers uses "Any status" and "Anything" (provides). Catalogs uses "Any source" and
  "Either" (claimed).
- **Value words** (platform-11, -43): `pstatus` gains the value `inactive` (accepted and not
  active), and its texts become Accepted, Active, Refused, Inactive, Removal blocked, Pending.
  `csource` texts become Subscription, Contributed, Claimed only. `claimed` gains
  `{yes: "Claimed", no: "Unclaimed"}`. The values themselves are unchanged except the new
  `inactive`, which `filterSpec` publishes to `prefs.js` like any other value.
- **Chips** (installed-05): the chip text is `Label: text`, its `title` holds the full value,
  and its `aria-label` is "Remove Label filter". Module chips show the last path segment. Uses
  and provides chips show `contractShort`.
- **Count row** (installed-06): the row is always rendered. It holds "Showing **n** of M"
  (plus the scope when there is one), then the chips, then "Clear all" when a filter is active.
  The Installed empty state keeps "Clear filters". A live refresh swaps `#list`, not the form, so
  the count lives in its own small region `#shown` inside the form. That region carries the same
  `data-follow` topic as `#list`, and a refresh updates it without re-rendering the fields the
  user may be typing in.
- **Search icon** (installed-04): the search input sits in `span.search-box`, which holds an
  inline, `aria-hidden` SVG magnifier. The box carries the border and background. Inline SVG is
  allowed under the page policy (`img-src`/`style-src` are not involved).
- **Apply as you type** (installed-22): in `portal.js`, a delegated `input` listener on
  `form[data-filters] input[type=search]` and a `change` listener on the form's other text
  inputs call `form.requestSubmit()` after 300 ms without input. The form carries
  `hx-sync="this:replace"`, so a newer submit aborts an older one in flight. The boosted `GET`
  keeps the URL as the source of truth, and the URL then stores the filters as today
  (portal:D14:R1). Selects keep their immediate submit (`portal.js:161-166`). Focus and caret
  must survive the swap; section 1's spike settles how (see Research & Decisions).
- **Hide Apply when script runs** (installed-02, platform-10): `prefs.js`, which already runs in
  the head before paint, adds the class `opm-js` to `<html>`. The rule
  `.opm-js form[data-filters] .field-submit` hides the Apply button visually and keeps it
  reachable for Enter (the clip pattern, not `display:none`, so implicit submission still has a
  submit button). Without script, the button shows as today. The Platform events form
  (`data-filters="events"`) and the Catalog events form get the same rule, which replaces their
  "Show" buttons.

### Installed (installed-07, -08, -09, -11, -15, -17, -19, -23, -26)

- **Counting every namespace** (installed-23): `installedPage` fetches `/instances` and
  `/packages` unscoped. When a list comes back `access: forbidden` and a `namespace` filter is
  set, the page refetches that kind scoped to the namespace, which is exactly the read the page
  makes today. In that case the total is labelled with its scope ("of 4 in default") and the
  namespace field is a text input. Otherwise the namespace filter is applied in
  `matchesInstalled`, like every other filter, so "of M", the counts and the choices cover
  everything the caller may read. The stream topic stays `instances:<ns>` only for the
  scoped fallback, and is `instances` otherwise.
- **Module matches a package's source** (installed-26): a package's module string is
  `<Source.Kind> <namespace>/<Source.Name>`, where the namespace defaults to the package's own when
  `Source.Namespace` is empty. The string goes into the module choices, and `module=<that string>`
  matches the package. The note "Packages are not included: a package records a source, not a
  module" is removed. This is not portal:D16:R2: that requirement covers `uses`, which a package
  does not record. A package's source is recorded (`PackageSummary.Source`).
- **Search** (installed-19): the haystack adds the row kind (`instance` or `package`) and the
  package's source string.
- **Rows** (installed-07): the `<tr>` gets `row-<health state>`, and
  `tr.row-degraded` takes a `--degraded-row` token (#fdf3f0 light, #24110e dark, defined in all
  three theme blocks). The whole row is a link, through a CSS stretched link: the name
  anchor's `::after` covers the row. The provider tooltip trigger sits above it (`z-index:1`),
  so it stays reachable. At phone width (`tr` is a block card) the same overlay covers the card.
- **Table geometry** (installed-08): from 761 px up, `table.installed` uses
  `table-layout:fixed; min-width:940px` inside `.table-wrap` (horizontal scroll). A `<colgroup>`
  sets Kind 130 px, Applied 150 px, Health 150 px, Objects 80 px and Owner 90 px, and Name and
  Module or source share the rest about 1.2 : 2. Cells pad 14 px 20 px. Names do not break at
  hyphens (`white-space:nowrap`, ellipsis, full name in `title`). The phone card layout is
  untouched.
- **Kind chip icons** (installed-09): the template partials `icon-instance` (hexagon) and
  `icon-package` (box) are inline `aria-hidden` SVGs with the canvas paths, sized by
  `.kind svg`. They are used in every kind chip on the three pages: Installed, Platform
  "Installed as", and Catalog claims.
- **Provider badge** (installed-11): `span.prov-tip` (`tabindex=0`, `aria-describedby`) wraps
  `span.prov`, which holds a plug SVG and the word "Provider", 11 px/700 uppercase. Its border is
  coloured by standing: green when accepted and active, red when refused, amber when removal is
  blocked, the progressing blue when pending, neutral when accepted and inactive, the lock style
  when locked. Next
  to it is `span.tipbox role=tooltip`, shown on `:hover` and `:focus-within` by CSS alone. The
  tooltip is composed in `view.go` from `ProviderClaim`: "Registration default.x is accepted and
  active.", "... is refused: <Reason>.", "... is pending: <Reason>.", "... is accepted, removal
  blocked: <Reason>.", "... is accepted and not active.", or "Holds a registration whose standing
  you may not read." (portal:D15:R2/R4). The standing word is also in visually hidden text, so
  colour is never the only carrier. "Provides #X" is not added: it would need a provides list on
  `ProviderClaim`.
- **Module or source cell** (installed-15): two block lines. An instance shows the module path,
  then its version (`span.ver.block`). A package shows `<Kind> <ns>/<name>`, then the first 8 hex
  characters of `sourceArtifact.digest` (with the revision in `title`). When `sourceArtifact` is
  absent, the second line says "no revision recorded", the recorded fact, where the canvas says
  "no revision fetched". "path <p>" shows only when the path is neither empty nor `.`.
  `installedRow` gains `SourceArtifact`.
- **Empty state** (installed-17): a centred block with the 17 px/600 line "Nothing matches these
  filters" (no full stop) and "Clear filters" as `a.btn`, a solid ink button. It keeps
  `data-clear-filters`.

### Platform page (platform-04, -05, -07, -12, -13, -15, -18, -21, -23, -24, -25, -26, -32, -36, -42)

- **Status state block** (platform-04): `#platform-status` renders A's state block.
  - Eyebrow: "Platform · the controller".
  - When: "since <relative time>" from `Platform.Reconcile.Since`. Nothing records a
    last-reconcile time, and Since is the deciding condition's `lastTransitionTime`.
  - Word: the applied badge text ("Applied", "Reconciling", "Failed", ...; platform-41 ruling).
  - Hue follows the reconcile state, and the icon follows the hue: Applied takes `applied` (navy,
    check), Reconciling `progressing`, Failed and Stalled `degraded`, Suspended and
    ManagedExternally `neutral`, and Unknown or a state the UI does not know `unknown`.
  - Summary: today's line, "N catalogs resolved from M subscriptions · K claims active".
  - No h2. The block's `aria-label` is "Platform status".
- **Reason counts** (platform-05): `statusOf` builds the reason links. Each one is a bold count
  plus a mono reason.
  - For registrations whose verdict is `Refused`, a count per `Reason`, in red, linking to
    `/?tab=providers&pstatus=refused#parts`.
  - For `RemovalBlocked`, the same in amber, linking to `pstatus=blocked`.
  - When `ContractsFulfilled` is `False`: its reason, in amber with no count (`N` 0), linking to
    `/?tab=catalogs#contracts`. The number of unfulfilled contracts is only in the condition
    message. Parsing it would be inference, and the contract inventory is portal:OQ4.
  - When `Ready` is not `True`: the Ready reason, in red with no count, linking to `#cond-Ready`.
  - Each reason's `Class` carries its hue: `hue-degraded` for refusals and the Ready reason, and
    `hue-unknown` (A's amber) for removal blocked and the contracts reason, which is information
    (portal:D3:R8).
  - With no reasons, `None` reads "Nothing refused or unfulfilled".
  - When the caller may not list registrations, `None` reads "Refusals not shown: you may not
    list TransformerRegistrations", and no refusal count is drawn, never a zero.
- **Installed card** (platform-07): `.count-groups` becomes two columns, Health and Applied. Each
  column is a stack of 40 px rows with a bottom rule: the badge on the left and a 22 px/700 count
  in the tone ink on the right. Each row has the `title` "Show the N installed with health X" (or
  "applied state X"). `countLink` gains `Title`. The closing line reads "· N of them are
  providers" (portal:D17 vocabulary). Locked and failed lists keep their locked wording.
- **Identity facts** (platform-36): the label becomes "Controller". The facts grid is
  `110px minmax(0,1fr)` at 14 px. The label style itself comes from A.
- **Provider rows** (platform-12, -13, -15, -42):
  - The provider name is a mono link to the first holder's Provider tab when there is a
    holder, and plain text otherwise.
  - Provided contracts are `.chip-contract` pills (999 px radius, accent border, accent field
    background, accent-deep text, 600). The text stays `contractShort` and the `title` the full
    path.
  - Status is one row of square badges (A's badge style). The first badge is acceptance from
    `accepted`: "Accepted" in green, or, when not accepted, the verdict ("Refused" in red,
    "Pending" in the progressing blue, otherwise "Not accepted" in neutral). The second is activation from
    `active`: "Active" in green or "Inactive" in neutral. A third badge, "Removal blocked" in
    amber, shows only for that verdict, beside Accepted and Active (portal:D4:R4/R7). The
    duplicate verdict badge goes.
  - The reason line reads `<mono>Reason</mono>: message`, in the degraded ink for Refused, the amber ink
    for Removal blocked and the progressing ink for Pending. The message stays (portal:D15:R3). The activation
    reason line stays.
  - Row-heading links on the Providers and Catalogs tables are 13 px mono at weight 500.
- **Catalog rows** (platform-18): Source reads "subscription", "contributed" ("contributed by
  <name>" kept as a muted second line), or "claimed only". Claimed by is the registration name as
  a mono link to its first holder's Provider tab, or plain text with no holder, or "no
  registration" in muted text. Enabled is a square badge: "Enabled" in green, "Disabled" in
  neutral, or muted "not recorded".
- **Contracts note** (platform-21): `#contracts` is unboxed: an amber sans "Contracts" kicker over
  one 14 px sentence. The controller's message moves into a `<details>` labelled "Controller
  message", because the Conditions panel also carries it.
- **Recent events** (platform-23, -24, -25, -26):
  - The heading row holds the h2, the Resource select (min 260 px) and the right-aligned
    "Showing n of M, newest first, repeats folded".
  - The one-hour note becomes a footnote under the table (portal:D9:R2 keeps the label).
  - The feed renders through the `events-table` partial (below), with the Resource column.
  - Each row's Resource is a link to `/` with the current query and `eresource` set to that
    object's value (`platform` or `registration:<name>`). It has the title "Show only this
    resource's events" and shows the Kind without its group over the name.
  - Options: an "Everything" optgroup holding "All resources (N)"; a "Platform" group whose option
    is the Platform's name with its count; and "Providers (TransformerRegistration)" with each
    registration and its count. The counts come from the feeds the page already reads.
  - Age is compact ("5m"), with `datetime` and the absolute time in `title`.
- **Theme menu** (platform-32): each theme button holds an inline SVG check, shown only when
  `aria-pressed="true"` and drawn in `--hdr-strong`. It replaces the brass `::after` glyph. This
  edits the theme menu markup in `layout.html`, which A also edits. This change starts after A
  merges, so the two edits do not conflict.

### Catalog page (catalog-01, -02, -05, -07, -08, -18, -19, -24, -25, -26)

- **Headline** (catalog-01, catalog-24): the template func `catalogShort(path, all)` returns the
  last path segment without `@vN`, falling back to `contractShort` when another catalog in the
  Platform document has the same last segment. The h1 is the short name in the display font, with
  no `mono` class, which also fixes the 14 px headline bug. Under it is
  `p.catalog-path.mono` with "path · version" ("not set" when there is no version). The Version
  fact row goes. The last breadcrumb is the same short name, in ink.
- **Eyebrow and origin** (catalog-02): `p.kicker` "Catalog", then a separate `span.origin`
  pill: `origin-subscription` navy "Subscribed", `origin-registration` amber "From a provider",
  `origin-claim` dashed neutral "Claimed only".
- **Source** (catalog-05, catalog-25):
  - A subscription reads "Platform subscription".
  - A contributed catalog shows the contributing registration's name as a mono link to its
    holder's Provider tab when there is one holder. With several holders, the name is followed by
    a muted "held by A, B". With none, it is plain text.
  - A claim-only catalog lists the claiming registrations the same way. `catalogView` gains
    `ClaimLinks []holderLink`, built from `Claims`.
- **Enabled** (catalog-07): when the catalog is not resolved and `Enabled` is nil, the page reads
  "no: not in the registry". Enablement is a registry field, and absence from
  `status.registry` is the recorded fact. "not recorded" stays only for a resolved catalog with no
  value.
- **Resolved state block** (catalog-08, catalog-26):
  - Eyebrow: "Resolved · the controller".
  - Hue and word: `applied` (navy, check) "Resolved", or `degraded` (red, bang) "Unresolved".
  - Summary: "In the resolved registry at <version>, from the Platform's subscription", or
    "..., contributed by its provider's registration", or "Not in the resolved registry". The
    canvas says "Pulled <version> from the registry", but nothing the portal reads records a
    pull, only the registry entry (Principle IV). For a claim, the summary adds "Claimed by <name>",
    replacing today's separate "From the claim" line.
  - Reasons: for an unresolved catalog, a count per refusal reason among the claiming
    registrations (or 1 for the Platform's `Ready` reason when the catalog is a subscription). Each
    count links to `?path=<path>&tab=events&reason=<Reason>`. A resolved catalog shows the quiet
    line "No resolution errors".
  - When: for an unresolved catalog, "refused since <time>" from the claim's `Accepted`
    condition's `lastTransitionTime` (falling back to `Reconcile.Since`), or "since <time>" from
    the Platform's `Ready` for a subscription. A resolved catalog has no time (catalog-09 needs
    the controller).
  - The platform-wide `ContractsFulfilled` line stays below the block.
- **Events tab** (catalog-18, catalog-19): `catalogEvents` takes the view
  `filterView{Path:"/catalog", Params: type (Normal, Warning; Any "Any type"), reason}`, which
  is not remembered (portal:D14). The type options carry counts. `Total` stays the unfiltered
  length. The tab renders a `filters-inline` form (hidden `path` and `tab`), the chips, "Showing X
  of N" in the heading row, the `events-table` without the Resource column (as on the canvas), and
  the filtered empty state "No events match this filter."

### Shared with align-owner-pages: the events table

The `events-table` partial replaces `events-list` wherever an events feed is a page region. It is
a `<table class="table rows events-table">` with these columns: Type (88 px, 12 px uppercase,
red ink for Warning), Reason (mono), an optional Resource (kind over a linked name), Message
(note; then "×N" when folded and the reporting controller, both muted), and Age (compact,
right-aligned). At phone width it falls back to the card layout `table.rows` already has. Its
input is:

```go
type eventsTable struct {
    Problem      *v1.Problem
    Rows         []eventRow
    ShowResource bool
    Empty        string // "No events in the last hour or so." or the filtered text
}
type eventRow struct {
    v1.Event
    Kind, Name   string // Regarding, kind without group
    ResourceHref string // a filter link, "" when not a filter
}
```

Whichever of this change and `align-owner-pages` lands first adds the partial, and the other
adopts it. `events-list` is removed by the change that moves its last caller (the details
panel's Events view is C's). This change does not edit `owner.html`, `panel*.html`, `owner.go`,
`graph.go` or the graph script. C does not edit `filters.go`, the `filter-form` partial or the
filter code in `portal.js` and `prefs.js`, and its owner-page forms pick up the new behaviour
through `data-filters`.

### Spec deltas

`web-ui` only. MODIFIED: "The Platform page is the landing page", "Installed lists instances and
packages together", "The Catalog page shows one catalog as the cluster records it", "Pages use the
words a platform team uses". ADDED: "Filter forms offer the values present and apply as the user
types", "Installed counts every namespace the caller may list", "The module filter matches a
package by its source". The MODIFIED requirements keep every scenario of the main spec, and none
of them is touched by `align-owner-pages` or `align-graph`. "List filters live in the URL and are
remembered per browser" is left as it is: the new behaviour is additive and sits in the ADDED
requirements.

### Design record

One requirement is added to portal:D14 in `docs/DESIGN.md` by the planning PR, as portal:D14:R7: filter choices and their counts come only
from rows the caller may read; a list the caller may not read leaves its namespace filter as free
text, and no count stands for an unreadable list. It applies portal:D7 to the filter forms this
change adds.
No other decision changes. The module filter for packages (installed-26) and the recorded-fact
wording below are supervisor rulings recorded here, not decisions.

### Authorization

No new verb or resource. Installed: `list moduleinstances` and `list modulepackages`
cluster-wide. When that is forbidden and a namespace filter is set, the page falls back to `list` in
that namespace, which is today's read. Platform and Catalog: `get platforms` and `list
transformerregistrations` through the Platform document, plus the events resources the pages
already read (`get` on the owner and `list events` in its namespace, portal:D7:R4). None of these
changes. `internal/ui` still reads only through the read API.

## Research & Decisions

### Focus and caret across an apply-as-you-type swap (spike, section 1)

**Context**: a boosted `GET` of the filter form swaps `#main`, which replaces the search input
while the user types. If focus or caret are lost, apply-as-you-type is worse than Enter.
**Explored**: htmx 2.0.11 swap code (`internal/ui/static/vendor/htmx*.js`, its focus
preservation for an active element with an `id`, and `hx-sync`), and `portal.js:161-166`.
**Options considered**:
1. Rely on htmx's own restore. htmx 2 records the focused element's `id` and selection range
   before a swap and restores both after settle. No code, if it holds in all three engines.
2. Restore in `portal.js` on `htmx:afterSettle`: refocus the element with the saved `id` and
   reset `selectionStart` and `selectionEnd`. A few lines, under our control.
3. Keep the form out of the swap (target `#list` and `#shown` with `hx-select`). The form is
   stable, but chips and the "Clear all" link would go stale.
**Decision**: Option 1 if the spike's browser run passes in Chromium, Firefox and WebKit,
including characters typed while a request is in flight (`hx-sync="this:replace"`). Otherwise
option 2. The finding goes here before section 2 starts.
**Rationale**: the least code that keeps the URL as the only state.

### Installed totals across namespaces (installed-23, installed-01)

**Context**: the namespace filter scopes the API read, so "of M", the counts and the namespace
choices shrink to one namespace.
**Options considered**:
1. Keep the scoped read and label the total. Honest, but the namespace select would list only
   the current namespace.
2. Read unscoped when allowed and filter on the server. Fall back to the scoped read only when
   the unscoped one is forbidden.
**Decision**: Option 2.
**Rationale**: it is the canvas behaviour, and it costs one read in the common case, the same
read the unfiltered page already makes. The locked fallback is today's read and wording.

### Module filter for packages (installed-26)

**Context**: the canvas puts a package's source into the Module select. Live leaves packages out,
and no decision records that.
**Options considered**:
1. Keep packages out and record it as a decision.
2. Match a package by its recorded source string.
**Decision**: Option 2 (supervisor ruling 2026-10-06: what no decision rules out follows the
canvas).
**Rationale**: the source is recorded, so matching it guesses nothing. portal:D16:R2 is about
`uses` only.

### Canvas copy the portal cannot back

**Context**: two canvas phrases claim things nothing records: "Pulled <version>" on the Resolved
block, and "no revision fetched" on a package row.
**Decision**: say what is recorded: "In the resolved registry at <version>" and "no revision
recorded". The owner may prefer the canvas words in review.
**Rationale**: Principle IV: status is read, never inferred.

## Risks / Trade-offs

- **Option counts go stale on a live refresh.** Rows and "Showing" refresh, but option labels
  update only on navigation or a filter change. Re-rendering the form during a refresh would
  steal focus from someone typing. Accepted, because the counts are a guide, not a status.
- **Unscoped Installed read for a namespace filter.** A cluster with many instances pays for the
  full list, then filters. That is the same read the unfiltered page makes, and both lists are
  answered from held state (portal:D3:R9).
- **Typing triggers requests.** Debounced to 300 ms, with older ones aborted. Every request is an
  ordinary read through the API, already rate-limited by the read model's held state.
- **Whole-row links can capture clicks** meant for the tooltip or a holder link. Those elements
  sit above the stretched link (`position:relative; z-index:1`), and the browser test clicks the
  tooltip trigger to check it.
- **Parallel work with align-owner-pages** on `partials.html` and `portal.css`. The split is
  above. The second change to land rebases and adopts the first's `events-table`.

## Migration Plan

None. URLs, stored filter keys and values stay valid. `pstatus=inactive` is new, and an old
stored query never holds it. Goldens are regenerated and reviewed in each section.

## Open Questions

None that blocks this change. The per-catalog resolve time is the open question A adds
(opm-operator#230). The Applied words follow the supervisor ruling unless the owner overturns it.

## Gap map

| Gap | Where above | Task |
| --- | --- | --- |
| installed-01 | Filter forms: Choices; Installed totals | 2.2, 3.2 |
| installed-02 | Filter forms: Order and row; Hide Apply | 2.1, 2.5 |
| installed-03 | Filter forms: Any words | 2.2 |
| installed-04 | Filter forms: Search icon | 2.3 |
| installed-05 | Filter forms: Chips | 2.3 |
| installed-06 | Filter forms: Count row | 2.3 |
| installed-07 | Installed: Rows | 3.4 |
| installed-08 | Installed: Table geometry | 3.4 |
| installed-09 | Installed: Kind chip icons | 3.5 |
| installed-11 | Installed: Provider badge | 3.5 |
| installed-15 | Installed: Module or source cell | 3.3 |
| installed-17 | Installed: Empty state | 3.4 |
| installed-19 | Installed: Search | 3.3 |
| installed-21 | Filter forms: Counts | 2.2, 3.2 |
| installed-22 | Filter forms: Apply as you type; spike | 1.2, 2.4 |
| installed-23 | Installed: Counting every namespace | 3.1 |
| installed-26 | Installed: Module matches a package's source | 3.3 |
| platform-04 | Platform: Status state block | 4.1 |
| platform-05 | Platform: Reason counts | 4.1 |
| platform-07 | Platform: Installed card | 4.2 |
| platform-10 | Filter forms: Order and row; Hide Apply | 2.1, 2.5 |
| platform-11 | Filter forms: Choices, Counts, Any words, Value words | 2.2, 4.3 |
| platform-12 | Platform: Provider rows | 4.4 |
| platform-13 | Platform: Provider rows | 4.4 |
| platform-15 | Platform: Provider rows | 4.4 |
| platform-18 | Platform: Catalog rows | 4.5 |
| platform-21 | Platform: Contracts note | 4.5 |
| platform-23 | Platform: Recent events; events table | 4.6 |
| platform-24 | Platform: Recent events | 4.6 |
| platform-25 | Platform: Recent events | 4.6 |
| platform-26 | Platform: Recent events | 4.6 |
| platform-32 | Platform: Theme menu | 4.7 |
| platform-36 | Platform: Identity facts | 4.2 |
| platform-41 | Non-Goals (supervisor ruling) | none |
| platform-42 | Platform: Provider rows | 4.4 |
| platform-43 | Filter forms: Value words, Counts | 2.2, 4.3 |
| catalog-01 | Catalog: Headline | 5.1 |
| catalog-02 | Catalog: Eyebrow and origin | 5.1 |
| catalog-05 | Catalog: Source | 5.2 |
| catalog-07 | Catalog: Enabled | 5.2 |
| catalog-08 | Catalog: Resolved state block | 5.3 |
| catalog-18 | Catalog: Events tab | 5.4 |
| catalog-19 | Catalog: Events tab; events table | 5.4 |
| catalog-24 | Catalog: Headline | 5.1 |
| catalog-25 | Catalog: Source | 5.2 |
| catalog-26 | Catalog: Resolved state block | 5.3 |
