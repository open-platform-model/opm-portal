## Why

The redesign (`redesign-web-ui`, PR 39) built the instance and package pages to recorded data,
but a verified comparison with the owner's canvas
(`docs/design/evidence/05-canvas-gap-report/gaps.md`, section C, 38 gaps) shows the pages still
read differently from the boards the owner reviewed. The summary cards are small badges in plain
boxes with no state word, summary line or time. The Resources tab is a nested list. The Events
tab opens on the owner's own events only, so a Pod that cannot pull its image says nothing until
the user picks that Pod. The details panel is empty until a node is chosen. Logs hide behind
collapsed folds. YAML opens empty, and the Provider tab repeats one standing four times. The
owner answered on 2026-10-06 that the Events tab merges every reached object's events ("Merge
all"), and that the global look follows the canvas. This change brings the owner pages onto the
canvas within what the cluster records.

Depends on `align-shell-and-tokens`: it uses that change's shared state block, tone tokens,
underline tabs with a count slot, square badges, and its fix of the empty details panel
(`instance-43`). Runs beside `align-graph`; the overlap is listed under Impact.

## What Changes

- **Read API, additive** (section 1):
  - `instances/{ns}/{name}/events` and `packages/{ns}/{name}/events` accept `scope=all`. With
    it, the feed holds the owner's events and those of every object its inventory reaches that
    the caller may read, merged newest first and folded as today. One list per event namespace
    and kind, not one read per object.
  - `EventList` gains `scope` and `partial`.
  - `PackageSummary` gains `prune`, read from `spec.prune` of the package object the portal
    already reads.
  - Implements the owner's "Merge all" answer, recorded as portal:D9:R6.
  - Every event note the API serves is capped at 512 Unicode code points (see "Length cap" below).
- **Summary cards** (section 2):
  - Applied, Health and Provider are drawn as the shared state block: eyebrow, a "when" stamp,
    tone icon, big state word, a one-line summary from recorded facts, and reason counts (count
    first, in the card's tone) or a "no problems" line.
  - Applied keeps its attempt history as a strip of marked dots with a Show/Hide history pill.
    The opened rows carry an outcome badge.
  - While `Reconciling=True` on an applied owner, Applied shows Reconciling as its state.
  - The identity card gets a mono subtitle (module path and version, or the package source),
    info tips on Owner and Applier, the resolved source namespace, a readable interval and
    prune.
  - Owner pages widen to the 1840 px column, with 380 px summary columns.
  - The Conditions fold moves below the tab panels.
- **Tabs, Resources and the details panel** (section 3):
  - Tab order is Graph, Provider, Resources N, Events N, Logs, YAML.
  - Resources becomes a table: Kind, Name, Component, Origin, Health, Details. Degraded rows are
    tinted, each row links to Graph focused on its node, configuration groups fold as in the
    graph, and a package's source is the first row.
  - Tab links keep `node` (the selection a graph click writes, `align-graph`) and `focus` (the
    hand-off spotlight); YAML opens on the `node` object, else the `focus` object.
  - The details panel opens on the owner at rest and goes back to it on Clear selection. It shows
    a message box tinted by tone, 40 px buttons, per-kind facts read from the object document,
    the component explanation, and an Events link on the root.
- **Events, Logs and YAML** (section 4):
  - Events opens on "All resources (N)" from the merged feed.
  - The filter bar shows counts on every option and a Reason chip only when one is set. "Showing
    N of M" sits on the filter row, and no Apply button shows when script runs.
  - Events render as a table whose resource cell filters to that object. The notes fold into one
    line, and the empty text depends on whether a filter is set.
  - Logs pick a Pod and container with a select and show one dark pane that follows at once,
    with a hint and an Events link when the container has not started, and a "restarting" line
    for a crash loop.
  - YAML picks with a select and opens on the focused node or the first object.
- **Provider tab** (section 5):
  - An intro per kind and a facts grid (Registration, Claims catalog, Registry, Contracts,
    providerRef, Controller says).
  - "Registration conditions" with tone badges.
  - "What it provides" as a table with round contract pills, and a "Show in Installed →" or "All
    N in Installed →" hand-off that is hidden when nothing uses the contract.
  - The standing appears once per claim.
- **Floors this change holds** (amendments of 2026-10-11 from the portal's plan, each traced in
  design.md "Amendments"):
  - A two-user test for the merged feed: a caller who may list events but may not list the Pods
    sees no event of a ReplicaSet, Pod or Job (the runtime children are shown all or none for each
    namespace) and no count that includes one (OP-A1).
  - A length cap on served event notes, and the in-cluster cases for both writers of an operator
    note (OP-A2).
  - Panel facts for a node only when the caller's access to it is `ok`, and an image reference
    shown without userinfo (OP-A3).
  - A text for every Provider standing in the Provider card and the Contracts fact, so none falls
    through to "N provided, M in use" when the claim provides nothing (OP-A4).
  - The owner pages in `TestBrowserPhone` at 360 px, and the tap and Escape cases for the Owner and
    Applier tips (OP-A5).
  - Branches take the other change's work by merging `main` into them, never by a history rewrite
    (OP-A6).
- Docs pages that describe the owner pages, and `ROADMAP.md`.

Not in this change: the per-gap exclusions in design.md Non-Goals (Age columns, Service
endpoints, a version-transition sentence, the owner's own YAML), and the graph drawing,
grouping, source-node read and registration node standing, which belong to `align-graph`.

**Shared-file rule with `align-graph`** (integrator, 2026-10-06; the same text stands in both
proposals). The two changes run in parallel after `align-shell-and-tokens` and share files,
never defines or requirements, and one function only (`ownerTabsOf`, at the lines named below):

| File | `align-graph` edits only | `align-owner-pages` edits only |
| --- | --- | --- |
| `internal/graph/*` | everything | nothing |
| `internal/ui/graph.go` | everything | only a minimal `restingNode` returning the owner's node, if it lands first; `align-graph` replaces its body |
| `internal/ui/owner.go` | the `graphHandoff` call in `annotateResources`; in `ownerTabsOf`, the `restingNode` call and the selection lines: parsing `node` beside `focus`, the Clear selection and Overview links (`linkWithout`, today's "Whole graph"), and `node` in `keepHidden` | the rest of `ownerTabsOf` (tab order, the `scope=all` fetch, counts, `node` and `focus` kept on tab links through `linkWith`, YAML's default object, the marked Resources row) and the summary cards |
| `internal/ui/panel.go`, `panel-body.html` | the group, registration and source variant blocks and the Clear selection pill | the generic panel (message box, buttons, per-kind object facts, root facts) |
| `internal/ui/pages.go` | nothing | `foldConfig` (returns one group per graph group node) |
| `partials.html` | the `graph` define | `component`, `child`, `history`, `events-list`, `events-table` and the `state-block` `History` field |
| `static/portal.js` | the graph functions and `resetDetail` | the log pane code (and the minimal `resetDetail` only if it lands first) |
| `static/portal.css` | `.graph*`, `.gn*`, `.ge*`, the legend and the group frame | owner cards, tables, panel, logs and YAML rules |

Both changes read `node` and `focus` the same way: `node` is the selection a click writes, `focus`
the hand-off spotlight; tab links keep both, YAML opens on the `node` object, else the `focus`
object, the Resources row of either is marked, and the panel rests only when neither is set. The
selection lines in `ownerTabsOf` are the one place both changes edit the same function; whichever
lands second merges `main` into its branch and reconciles them (never a history rewrite).

The resting node of the details panel is one behaviour (`graph-23`, `provider-21`, `instance-27`):
`align-graph`'s ADDED requirement "The details panel rests on the node that needs attention" picks
the node; `align-owner-pages`' tabs requirement only says the panel shows that node, never an empty
placeholder. The per-kind object facts on the panel (Replicas, Image, Strategy, Node, Restarts,
Started, Type, Ports; `instance-28`) are `align-owner-pages`', read from the object document in
`panel.go`. Whichever change lands second merges `main` into its branch and takes the other's
markup.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `web-ui`: the owner page's summary cards, tabs, Resources table, details panel at rest,
  merged Events, Logs, YAML and Provider tab requirements change.
- `read-api`: owner events gain `scope=all` and the `scope` and `partial` fields; packages carry
  `prune`; every served event note is capped at 512 Unicode code points.
- `read-model`: events can be read merged over an inventory, one list per namespace and kind.

## Impact

- Packages:
  - `internal/readmodel`: the merged events read.
  - `api/v1alpha1` and `internal/api`: the `scope` parameter, the `EventList` and
    `PackageSummary` fields, OpenAPI, the API goldens, and in-cluster omission over merged
    feeds.
  - `internal/ui`: `owner.go`, `panel.go`, `provider.go`, `view.go`, `pages.go`, the templates
    `owner.html`, `panel-body.html` and `partials.html` (the `component`, `child`, `history` and
    `events-list` defines), `portal.css`, and in `portal.js` only the log pane code (and `resetDetail`
    when this change lands before `align-graph`).
  - `cmd/opm-portal` browser tests, `Taskfile.yml`, `AGENTS.md` (the `test:browser` line),
    `docs/site/`.
  - `internal/authz`, `internal/graph` and `internal/health` do not change.
- API: additive only; `task api:breaking` stays green. No new path, so
  `docs/site/reference/portal/read-api.md` only describes the parameter and fields.
- Principle V: no new kind and no new verb. The merged feed lists `events.k8s.io` events, which
  the portal already lists, in each namespace where a reached object's events live (`default` for
  cluster-scoped objects, per portal:D9:R4). Each list is reviewed for the caller before it runs.
  An object the caller may not read, or whose namespace's events the caller may not list, is left
  out and the feed says `partial`; Secrets stay excluded. `prune` comes from the ModulePackage
  the portal already reads. Per-kind panel facts come from the object resource the YAML view
  already serves, stripped as today (portal:D8). No write and no Secret data. The `deploy/` role
  needs no new rule. Two floors from the portal's plan hold on top of that. A caller who may list
  events but may not read an object's kind sees none of that object's events; ReplicaSets, Pods and
  Jobs go together, so a caller who may not list one of them in a namespace sees none of the three
  there. The test `TestMergedEventsHideWhatTheCallerCannotGet` names both cases. The panel's per-kind facts are
  a fixed list, shown only for a node whose access is `ok`, and an image reference loses any
  userinfo before it is shown.
- Length cap: the API caps every event note it serves at 512 Unicode code points, an ellipsis
  ending a longer note, through one exported helper, `CapText(s string, limit int) string` in
  `internal/readmodel/cap.go`, called in `eventListDoc` (`internal/api/convert.go:309`) after the
  read model's fold of events by note. `align-graph` calls the same helper for the Flux condition
  message (its own limit, set in its own change). Either change, whichever merges first, adds
  `CapText` and its tests; the other adopts the file and adds no second helper. The helper sits in
  `internal/readmodel` so that both changes can import it: `internal/api` imports the read model,
  never the reverse, and `internal/ui` cannot be imported from `internal/api` without an import
  cycle. The call stays in `eventListDoc` because a cap in the read model, where the note is read
  (`events.go:113`), would cut notes before the fold at `events.go:120` (design.md "Where the
  length cap lives"). The cap is a documented maximum on an existing field (`Event.note`), so it is
  additive in the sense of `task api:breaking`; it shortens only notes over 512 code points, and
  design.md records it as a limit, not as a decision in `docs/DESIGN.md`.
- Principle VII: no dependency and no build step. The UI takes on one complexity: the panel
  reads an object document to show Replicas, Image, Strategy, Node, Restarts, Started, Type and
  Ports. The alternative was widening `GraphNode` in `internal/graph`, which `align-graph` owns.
- Overlap with `align-graph` (it runs in parallel):
  - Main-spec requirements: this change modifies "The instance page shows the record, the feed
    and the logs", "Instance and package pages summarize Applied, Health and Provider standing",
    "Instance and package pages are organised in tabs" and "The Provider tab shows the claim and
    who uses what it provides". `align-graph` must not modify them. It owns the two graph
    requirements and `graph-model`.
  - Default panel: the panel at rest (`instance-27`) is the same behaviour as `align-graph`'s
    `graph-23`. `align-graph` owns which node the panel rests on and the `resetDetail` reset. This
    change builds the minimal form (the owner's node) only if it lands first; the second change
    merges `main` into its branch.
  - Per-kind panel facts: `instance-28` is this change's gap, and design.md decision 5 reads them
    from the object document in `panel.go`, with no `internal/graph` change (both proposals now
    say so).
  - Shared files, in different functions except one:
    - `owner.go` `ownerTabsOf`, where `align-graph` adds the `restingNode` call and the selection
      lines (`node` beside `focus`, the Clear and Overview links, `keepHidden`), and this change
      keeps `node` and `focus` on tab links and reads them for YAML and the Resources row.
    - `graph.go`, where this change adds only a minimal `restingNode` if it lands first.
    - `panel.go` and `panel-body.html`, where `align-graph` adds the registration and group
      panel variants on top of this change's restyle.
    - `pages.go` `foldConfig`, which this change makes return a list of groups so per-kind groups
      flow through unchanged.
    - `partials.html`, where the `graph` define is `align-graph`'s.
    - `portal.js`, where the graph functions are `align-graph`'s.
  - The package source row shows the source node's health, which `align-graph` sets from the
    source's state (ready Healthy, not ready Degraded, and so on); until then it says the source
    is not read.
- Overlap with `align-platform-installed-catalog`:
  - That change owns the generic filter code (`filters.go`, the `filter-form` partial, and the
    filter code in `portal.js` and `prefs.js`) and the `opm-js` class. This change does not edit
    them.
  - The `events-table` partial is shared, with the input shape that change's design.md gives.
    Whichever change lands first adds it.
  - That change leaves `providerBadge` unchanged and adds `providerPill`; this change moves the
    kicker and `claimBadge` to `providerPill` (adding it as that change's design.md gives it, if
    this change lands first). That change keeps `newFilterForm`'s signature, so the Resources
    reason form compiles until this change removes it.
  - The tooltip (`tip` partial, `.tipbox`) ships in `align-shell-and-tokens`, so this change's
    Owner and Applier tips do not wait on that change.
  - Only this change edits `align-shell-and-tokens`' `state-block` partial (a `History` field). The
    per-reason class, the count-less reason, words-only `WhenText`, note lines, the caption and
    footer links ship in that change's partial, so `align-platform-installed-catalog` does not
    edit it.
- Live refresh: the merged Events tab refreshes when the owner or its own events change, not on
  an event about a child alone; the docs say so, and `ROADMAP.md` keeps a follow-up.
- Sections: six, one more than the usual five, because the merged feed's cost is an unverified
  assumption and section 1 must be the spike that measures it (openspec/config.yaml).
- SemVer: MINOR after 1.0 (additive API, changed pages). On the 0.x line it ships as one PR
  titled `feat(ui): align the instance and package pages with the canvas`.
- Decisions: implements portal:D9:R6 (new, owner answer 2026-10-06) and keeps portal:D3,
  portal:D8, portal:D9:R1 to R5, portal:D10, portal:D15 and portal:D16 as they are. The owner's
  answers of 2026-10-11 confirm this change's choices over the plan's: the plan's Provider table
  and claim heading, a ReplicaSet owner fact and a revision tip are not added, age, the `cause`
  join and the hover warning line stay unscheduled, and Resources stays a table.
