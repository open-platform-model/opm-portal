## Why

The redesign (`redesign-web-ui`, PR 39) gave the instance and package pages a graph that works
but does not read like the owner's canvas. The verified gap report
(`docs/design/evidence/05-canvas-gap-report/gaps.md`, section D, 40 gaps) shows the problem at a
glance: at 1920 px the cert-manager graph renders at about 58 % in a 250 px strip with 7 px labels;
every configuration component folds into one "17 config components" node, so RBAC, CRDs and
webhook configuration cannot be opened apart; an expanded group's members interleave with
workloads; nodes are two-line boxes with a rail where the canvas has three lines and a tone; edges
are curves with arrowheads; a package's source node is drawn without reading the source, so a
package whose Flux kind is not installed looks the same as one whose artifact is ready; and the
details panel sits empty until the user picks a node.

The owner answered on 2026-10-06: "Groups per kind (Recommended)" (amend portal:D4:R5: groups per
kind family, an accordion, a dotted frame, Collapse and Overview, zoom-to-group) and "Read the
source (Recommended)" (an authorized get of the package's `spec.sourceRef` object, showing its
Ready state, its revision, and "kind not installed" when the CRD is missing).

## What Changes

- **Graph model** (`internal/graph`): configuration components are grouped per kind family (CRDs,
  RBAC, Webhook configuration, Other configuration) instead of into one node; a family groups
  when its components hold two or more objects; a component holding a TransformerRegistration is
  never folded. An expanded group's members and the objects below them are laid out as one band,
  so a frame drawn around them encloses nothing else. A package graph reads left to right from the
  package, with its source and dependencies ahead of its components. Nodes grow to 216 x 72 for a
  third line. Registration object nodes on instance and package graphs carry the standing the
  controller wrote, as platform-graph registration nodes already do.
- **Read API, additive**: `GraphGroup` gains `family` and `objectKinds`; the package document and
  the package graph's source node gain `sourceState` (access, state, Ready reason and message,
  artifact revision); the instance and package graphs' TransformerRegistration nodes carry
  `registration`; the edge `route` description says its four points are the corners of an
  orthogonal elbow as well as a Bézier's control points (the points do not change).
- **Read model**: one new on-demand read, a `get` of the Flux source a package's `spec.sourceRef`
  names, made for the caller after its access review, limited to OCIRepository, GitRepository and
  Bucket, chosen by kind alone and always read in `source.toolkit.fluxcd.io`, as the controller
  does. A Ready source with no artifact is shown as having none, as the controller treats it. The
  server re-checks discovery for those kinds on a timer, so a Flux installed after the portal
  starts stops reading "Kind not installed" within minutes. The package graph's source node takes
  a health from the source's state, so the Resources tab's source row and the node's fill agree.
- **Web UI, drawing** (`internal/ui`): three-line nodes (kind, name, status line) filled and
  outlined in their health tone, cluster-made nodes dashed, the applied square kept; elbow edges
  without arrowheads; group nodes hatched and stacked with "N objects: K kinds"; a dotted frame
  with a Collapse pill around an open group; a legend under the pane; a ghost box when nothing was
  applied; source and registration nodes toned by their own state; hover cards beside the node
  with the message, a parent-aware origin and a hint. The instance page no longer draws the module
  node (its path and version are on the identity card).
- **Web UI, exploring**: a click selects (`node=`) without dimming; hover, keyboard focus and a
  hand-off `focus=` spotlight the whole upstream and downstream chain (two directed walks); a
  hand-off scrolls the focused node into the centre; one group open at a time, with Expand all,
  Collapse all, a Group configuration switch (`group=off` shows the components ungrouped), a
  "shown of total" count and an Overview pill, all swapping in place without jumping the page;
  full screen is a fixed overlay on the page, not the browser's full-screen API, so it survives
  opening and closing groups and Esc reaches the page; fit uses width and
  height, scales up to 1.5x (2x in full screen), and the pane height follows the graph; zoom 30 to
  200 %, wheel zoom anchored at the cursor; an animated expand that honours reduced motion; Esc
  steps back through card, selection, open group, full screen; Clear selection as a pill in the
  details panel.
- **Details panel**: at rest it shows the owner's node, or the registration node when the claim
  is refused, pending, accepted but not active, or removal blocked; panel variants for a group
  (object kinds and counts), a registration (standing, controller message, catalog, provider) and
  a source (Ready state, revision, or "kind not installed").
- `docs/site/` pages that describe the graph, and `ROADMAP.md`.

Gate: depends on `align-shell-and-tokens`. This change uses its tone tokens (border, ink,
background, tint), its square badge and its underline tabs, and the node-selection fix
(`instance-43`) that change ships. It does not start before that change merges.

Runs in parallel with `align-owner-pages`. Ownership: this change owns `internal/graph`,
`internal/ui/graph.go`, the `graph` template in `partials.html`, the graph part of `portal.js` and
`portal.css`, and the read-model and API work for the source read. `align-owner-pages` owns the
summary cards, the tab contents, the Resources and Events tables, Logs and YAML. Two files are
shared and say so in their tasks: `owner.go` (this change edits only the graph hand-off link
helper call in `annotateResources`, and in `ownerTabsOf` the resting-node choice and the selection
lines the shared-file table names) and
`panel.go` / `panel-body.html` (this change adds the group, registration and source variants and
the Clear selection pill; `align-owner-pages` restyles the generic panel). Gap `graph-23` (the
panel at rest on the Graph tab) is the same behaviour as `align-owner-pages`'s `instance-27`: this
change decides which node the panel rests on and resets the script to it; whichever change lands
second rebases onto the other's markup. No main-spec requirement is modified by both changes.
`align-shell-and-tokens`' "Selecting a graph node fills the details panel" already says a
no-script node link opens the page with the node selected, which covers this change's move from
`focus` to `node` for a plain selection, so this change does not modify it.

Not in this change: gaps outside section D; the hover card's applied age and warning count and
every other item section X of the report lists (portal:D17); per-kind facts on the panel (Image,
Node, Restarts), which `align-owner-pages` adds (`instance-28`); health for the Platform (portal:OQ22).
Configuration objects keep their kstatus health (supervisor ruling 2026-10-06, gap `graph-26`).

**Shared-file rule with `align-owner-pages`** (integrator, 2026-10-06; the same text stands in both
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
lands second rebases them.

The resting node of the details panel is one behaviour (`graph-23`, `provider-21`, `instance-27`):
`align-graph`'s ADDED requirement "The details panel rests on the node that needs attention" picks
the node; `align-owner-pages`' tabs requirement only says the panel shows that node, never an empty
placeholder. The per-kind object facts on the panel (Replicas, Image, Strategy, Node, Restarts,
Started, Type, Ports; `instance-28`) are `align-owner-pages`', read from the object document in
`panel.go`. Whichever change lands second rebases onto the other's markup.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `graph-model`: configuration grouping per kind family (replaces the one-group requirement); an
  expanded group laid out as a band; package graphs read left to right; registration standing on
  instance and package graphs; the source node carries the source's state.
- `read-api`: `GraphGroup.family` and `objectKinds`; `sourceState` on the package document and the
  source node.
- `read-model`: the on-demand source read for the caller.
- `web-ui`: graph drawing and interactions (two requirements modified), group frames and the
  accordion, the legend and the ghost, registration and source nodes, the resting panel.

## Impact

- Packages: `internal/graph` (grouping, layout, registration standing, source input),
  `internal/readmodel` (source read, claim catalog fields), `api/v1alpha1` and `internal/api`
  (fields, the source lookup, OpenAPI, goldens), `internal/ui` (`graph.go`, `panel.go`,
  `owner.go` at two named call sites, templates, `portal.css`, `portal.js`, tests),
  `cmd/opm-portal` (`TestBrowserGraph`), `test/browser/graph.py`, `docs/site/`.
  `internal/authz` and `internal/health` do not change.
- API: additive only; `task api:breaking` stays green. Node ids of configuration groups change
  from `grp:configuration/<owner>` to `grp:configuration/<owner>/<family>`; ids are stable across
  restarts as before, and an old `expand=` link opens the graph collapsed. Layout numbers change
  (node size, package columns), which the goldens record.
- Principle V: one new kind of read. The portal `get`s the one Flux source object a ModulePackage's
  `spec.sourceRef` names, only when its kind is OCIRepository, GitRepository or Bucket in
  `source.toolkit.fluxcd.io` (the kinds the controller reads, opm-operator
  `internal/source/resolve.go:18-22`), after the caller's access review and the reader's own, never
  with list or watch, and never following the source's `secretRef`. It serves the Ready
  condition's status, reason and message and `status.artifact.revision`; never the artifact URL.
  A Secret is still refused before any review. A caller who may not get the source sees it
  locked; a kind the cluster does not serve is read from discovery without a review, as kinds are
  resolved today. The `deploy/` role does not gain the Flux kinds: in a Pod run from `deploy/`
  the ServiceAccount is the caller, so the source shows locked; in-cluster, a user the cluster
  allows sees it as not readable by the portal (portal:D11:R3, portal:D13). No write, no new create, no values.
- Principle VII: no dependency, no build step. The band layout and the frame are the one new
  layout idea; the gap report shows a frame without it encloses non-members (`graph-21`).
- SemVer: MINOR after 1.0 (additive API, changed pages). On the 0.x line it ships as one PR titled
  `feat(ui): draw and explore the graph as the canvas does`, which cuts a minor release.
- Decisions: amends portal:D4 (R5: groups per kind family; opening one closes the others, and
  Expand all opens every family) and portal:D1 (the source read), adds portal:D20 for the source
  read, and records under portal:D3 the supervisor ruling that configuration objects keep kstatus
  health and that a registration node's fill and outline carry its standing. Keeps portal:D3:R1 (outline
  and fill carry health, a separate square carries applied), portal:D4:R1/R3, portal:D8 and
  portal:D17.
- Main-spec requirements touched: `graph-model` "Configuration components are grouped by default"
  (REMOVED, replaced by an ADDED requirement) and "Layout is deterministic" (MODIFIED: the band is
  the one exception to barycenter order); `web-ui` "Graphs are server-rendered, accessible SVG"
  and "Graphs can be explored without leaving the page" (MODIFIED). Everything else is ADDED under
  new names.
- Sections: six, one more than the usual five, because the design carries three unverified
  assumptions (the band layout, the source fixtures, SVG motion) and openspec/config.yaml makes
  section 1 the spike that checks them; folding it into section 2 would land layout code before
  the band is known to hold.
