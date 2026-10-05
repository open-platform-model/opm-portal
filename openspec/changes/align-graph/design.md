## Context

See proposal.md for why. The graph today: `internal/graph` derives the instance and package graphs
from read-model views (`instance.go`, `layout.go`), with one configuration group per owner
(`instance.go:144-187`), columns ordered by barycenter with no notion of a group block
(`layout.go:69-105`), 200 x 44 nodes and cubic routes (`layout.go:9-10`, `route`). `internal/ui`
turns the `Graph` document into SVG (`graph.go` `buildGraph`, `nodeOf`, `edgeOf`; the `graph`
template in `partials.html:79-120`), and `portal.js:704-1200` drives selection, spotlight, cards,
fit, zoom, full screen and Esc. The source node of a package is built from `spec.sourceRef` alone
(`instance.go:87-102`); nothing reads the source object.

Evidence for every claim below: the canvas boards in `docs/design/evidence/03-ui-canvas/`
(`Instance.dc.html`, `Package.dc.html`, `CertManager.dc.html`, `Provider.dc.html`,
`ProviderPackage.dc.html`; line numbers as the gap report cites them), the gap report
`docs/design/evidence/05-canvas-gap-report/gaps.md` section D, the F1 capture under
`testdata/clusters/f1/`, and opm-operator source at 59537b7. The F1 package's own Ready message
shows the "kind not installed" case on the fixture cluster: `getting OCIRepository
pkg/podinfo-release: no matches for kind "OCIRepository" in version "source.toolkit.fluxcd.io/v1"`
(`testdata/clusters/f1/modulepackages.yaml`).

This change starts after `align-shell-and-tokens` merges. It uses that change's tone tokens
(border, ink, background and tint per tone, light and dark), its square badge, and its fix for the
empty details panel (`instance-43`: `openNode` no longer inherits `hx-select="#main"`).

## Goals / Non-Goals

**Goals:**

- Every section D gap is closed or explicitly kept as a recorded divergence; the gap ids are cited
  at the decision that answers them and mapped to tasks at the end of this document.
- The graph stays a pure function of read-model views (Principle II); the one new read lives in
  `internal/readmodel` and is passed to `internal/graph` as input, as the provider lookup already
  is for the platform graph.
- The read API changes only additively (Principle III).
- Every script behaviour is held by `TestBrowserGraph` in Chromium, Firefox and WebKit, on the real
  owner pages over the F1 capture, not on synthetic HTML.

**Non-Goals:**

- Anything outside section D. The panel's generic layout (state, message box, buttons; gaps
  `instance-27`, `instance-28`), the Resources and Events tables and the summary cards are
  `align-owner-pages`.
- Section X items: the hover card's applied age and warning-event count (portal:D17), the
  transformer behind each object (portal:OQ5), per-kind facts (Image, Node, Restarts, Ports) on
  cards and panels, which would need new read-API fields under portal:D8.
- The canvas's "Ready / Not ready" words on the Applied axis: nodes keep today's applied words and
  square (portal:D3:R1; supervisor ruling 2026-10-06).
- Configuration objects without health (`graph-26`): kept as a recorded divergence. Configuration
  objects keep their kstatus health and the group keeps its worst-of health (supervisor ruling
  2026-10-06); the canvas's "configuration has no runtime health" sentence is not used.
- The Module node on instance pages: the graph document keeps it and its `instantiates` edge
  (portal:D4:R1); only the instance page stops drawing it (`instance-17`).
- Widening the `deploy/` role to the Flux source kinds.
- Watching source objects: the source is read on demand, per request.

## Decisions

### Groups per kind family (graph-01, graph-02, graph-26)

A configuration component (no workload kind among its objects, as today) belongs to a family by
the API group of its objects:

| Family id | Label | Every object of the component is in |
| --- | --- | --- |
| `crds` | CRDs | `apiextensions.k8s.io` |
| `rbac` | RBAC | `rbac.authorization.k8s.io` |
| `webhooks` | Webhook configuration | `admissionregistration.k8s.io` |
| `other` | Other configuration | anything else, or a mix |

A component that holds an `opmodel.dev` TransformerRegistration MUST NOT be folded: it is the
claim the Provider card and the resting panel point at. A family forms a group node when its
components hold two or more inventory objects in all; otherwise its components are drawn as
ordinary component nodes. The group's health is the worst of its members', partial and not live
when any member is (unchanged rule, now per family).

On F1's cert-manager (`moduleinstances.yaml`, 20 components): `crds` (1 component, 6
CustomResourceDefinitions) groups; `rbac` (13 components, 26 objects) groups; `webhooks`
(`webhook-mutating`, `webhook-validating`, 2 objects) groups; `other` holds only `namespace`
(1 Namespace) and stays a plain component. The default graph has 23 nodes: instance, module,
the components `cainjector`, `controller`, `webhook` and `namespace`, three group nodes, 8
inventory objects and 6 runtime children. This matches the canvas, which leaves `namespace`
ungrouped (`CertManager.dc.html:438`).

Group ids add the family as a last part: `grp:configuration/mi/cert-manager/cert-manager/rbac`.
Ids stay free of UIDs (portal:D4:R6). An old `expand=grp:configuration/mi/...` names no node and is
ignored, so the graph opens collapsed.

```go
// internal/graph
type Group struct {
	Kind    GroupKind   `json:"kind"`
	Family  string      `json:"family,omitempty"`  // configuration groups only
	Members []string    `json:"members,omitempty"`
	// ObjectKinds counts the inventory objects the members hold, by kind,
	// sorted by kind: the group's "N objects: K kinds".
	ObjectKinds []KindCount `json:"objectKinds,omitempty"`
	Hidden      []KindCount `json:"hidden,omitempty"`
}
```

Wire: `GraphGroup` gains `family` (`x-extensible-enum: [crds, rbac, webhooks, other]`) and
`objectKinds` (`GraphKindCount[]`). The UI maps the family to its label; an unknown family renders
as "Configuration".

### Expanded groups as a band (graph-21, graph-06)

`orderColumns` orders each column by barycenter, so an expanded group's members interleave with
workload components (gap report: `controller` between `cainjector-rbac` and
`controller-approve-rbac`). A frame is a rectangle across columns, so contiguity in one column is
not enough: a non-member object in the objects column can still fall inside the frame's height.

`layout` therefore places each expanded configuration group as a band: a run of rows reserved
across the component column and every column to its right, holding the members (sorted by
member name, as `Members` is), then their objects in member order, and nothing else. Every other
node of those columns is placed above or below the band in its barycenter order. The band starts
half a row pitch below the previous row and ends half a pitch above the next, leaving room for
the frame's header. The band's height is the larger of its member count and its object count.
Layout stays deterministic: the band's position is the barycenter of the members, and ties fall
back to ids. Invariant (tested): no node that is neither a member nor an object of a member lies
inside a band's rectangle.

The frame itself is drawn by `internal/ui` from the member and object node boxes plus the half-row
margins (`svgGraph.Frames`).

### Columns, node size and left-to-right reading (instance-14, graph-13, instance-17)

- Nodes are 216 x 72 (canvas 216 x 72, `CertManager.dc.html:451`), row pitch 96, column pitch 264.
- Instance titles: Module, Instance, Components, Objects, ReplicaSets, Pods (the two "Runtime"
  titles become what sits there: depth 1 is a ReplicaSet, depth 2 a Pod).
- Package graphs start at the package. Its source and its `dependsOn` packages sit in the
  components column, above the components, titled "Source and components". Edges from the
  package then all run rightward. This does not invent the canvas's source-to-component edge,
  which no field records (portal:D4:R1): the package owns its components, and the source is what
  it reads.
- The instance page does not draw the module node: `buildGraph` drops nodes of kind `module` and
  their edges, and when column 0 then holds nothing, shifts every x left by one column pitch and
  narrows the width. The module path and version stay on the identity card and in the document.

### Registration standing on instance and package graphs (provider-14)

`graph.Instance` and `graph.Package` receive the owner's provider claims (`ProviderOf`, already on
`InstanceItem` and `PackageItem`). For each TransformerRegistration object node whose name matches
a claim read with access `ok`, the node carries `registration` with the claim's standing,
catalog, version and provides, exactly as a platform-graph registration node does. The read
model's `ProviderClaim` gains the claim's `spec.catalog`, `spec.version` and `spec.provides` (read
from the same held object `claims.of` already reads). A claim the caller may not read adds no
`registration`; the node shows the claim as locked in the UI. In-cluster, the existing omission
drops `registration.message` and `activeMessage` (`internal/api/mode.go` `omitGraphNode`), which a
test now pins for an instance graph.

### The package's source state (instance-25)

The owner chose "Read the source". Shape:

```go
// internal/readmodel
type SourceReadiness string // Ready, NotReady, Unknown, NotFound, KindNotServed, UnsupportedKind

type SourceState struct {
	Access    health.Access   // ok, forbidden, notReadable; empty when nothing was asked
	Readiness SourceReadiness
	Reason    string // the Ready condition's
	Message   string // the Ready condition's, as source-controller wrote it
	Revision  string // status.artifact.revision
}

// Source reads the Flux source a package names, for the caller. g must
// cover get on it. Only OCIRepository, GitRepository and Bucket in
// source.toolkit.fluxcd.io are read; a core Secret is refused first.
func (m *Model) Source(ctx context.Context, who authz.Identity, g authz.Grant,
	kind ResolvedKind, ref ObjectRef) (SourceState, error)
```

`internal/api` looks the source up for `GET .../packages/{ns}/{name}` and `.../graph`, after the
package itself is authorized and read, the way `lookupProvider` does for the platform graph:

1. Kind not in the allowlist: `UnsupportedKind`, no review, no read (the controller refuses such a
   package anyway: `ErrUnsupportedSourceKind`, `internal/source/resolve.go:80-90`).
2. `ResolveKind(group, kind)` fails (discovery serves no such kind): `KindNotServed`, no review.
   Discovery is read without a review today (portal:D18:R3).
3. Authorize `get` on the resolved resource, namespace (sourceRef's, else the package's, as the
   controller does, `resolve.go:41-44`) and name. Refused: access `forbidden`, nothing read.
4. `Model.Source`: `covers`, then `readerMay("get", ...)` (false: access `notReadable`), then one
   `Get`. NotFound: `NotFound`. Otherwise the Ready condition (`status.conditions[type=Ready]`) as
   `Ready` (`True`), `NotReady` (`False`) or `Unknown` (`Unknown` or absent), with its reason and
   message, and `status.artifact.revision`. `status.artifact.url` is never read into the state.
   This mirrors the controller's own test (`resolve.go:60-73`).

A failed lookup never fails the package document or its graph; it is the state. Wire:

```json
"sourceState": {
  "access": "ok",
  "state": "kindNotServed",
  "reason": "",
  "message": "",
  "revision": ""
}
```

`state` is `x-extensible-enum: [ready, notReady, unknown, notFound, kindNotServed,
unsupportedKind]`. It is on the `Package` document (not on list items, which would cost one read
per row) and on the package graph's `source` node, whose `access` and `missing` follow it
(`missing` for `notFound`). In-cluster the message is served as written: source-controller wrote
it, not the operator, which the in-cluster omission already allows for other writers' text.

Authorization: `get` on `source.toolkit.fluxcd.io` `ocirepositories`, `gitrepositories` or
`buckets` in one namespace, by name, for the caller (SelfSubjectAccessReview locally,
SubjectAccessReview in-cluster), and the reader's own `get` review. No list, no watch.

Liveness: the source is not watched. The package's topic fires when the controller re-reconciles
the package after a source change (the controller watches the three kinds,
`modulepackage_controller.go:178-206`), and the page refetches the document then.

### Drawing nodes (instance-14, graph-14, provider-14, instance-25, instance-16, graph-24)

Node anatomy (template `graph`, classes only, no `style` attribute):

| Line | y | Text |
| --- | --- | --- |
| kind | +20 | `Ref.Kind` (or the node kind word), 10 px uppercase, tone ink |
| name | +42 | label; mono 12 px for objects, runtime children and sources, sans 600 otherwise |
| status | +60 | a dot and a short status, tone ink |

Status line, first match wins: locked, "Locked"; missing, "Missing"; a registration with a
standing, "Accepted · active", "Accepted · not active", "Removal blocked" or the refusal or
pending reason; a source, "Artifact ready" plus a short revision, "Kind not installed", "Not
found" or the Ready reason; a configuration group, "N objects: K kinds"; a Pod group, "N Pods" and
the worst state; a ReplicaSet, "N desired" (with the health reason when not healthy); a
component, "N objects"; anything with a health reason, that reason; otherwise the health word.

Fill and outline carry health in the tone's tint and border; the left rail goes. Runtime children
(`kind-runtime`) are dashed (made by the cluster). The applied square stays in the top-right
corner and is the only applied mark, so the two axes stay apart (portal:D3:R1). Registration
nodes take the standing's tone (`prov-active` healthy, `prov-inactive` progressing,
`prov-refused` degraded); source nodes the readiness tone (`ready` healthy, `notReady` degraded,
`unknown` unknown, `notFound` and `kindNotServed` missing). The legend says which mark means what.

Edges: `edgeOf` draws the route's four points as an orthogonal polyline, `M p0 H p1.x V p3.y H
p3.x`, stroke 2 in the edge token, with no marker. The route's points are already the elbow's
corners (`layout.go` `route`: start, (mid, sy), (mid, ey), end), so the document does not change;
the OpenAPI description of `route` says so. Unverified and unconfirmed edges keep their dashes.

### Group nodes, frames and the accordion (graph-02, graph-06, graph-04, graph-16)

A collapsed configuration group draws a stacked card (a second rect offset +4,+4 behind the box)
and a hatched fill (a second SVG pattern, `<id>-ghatch`), with the lines "Group · N components",
the family label, and "N objects: K kinds".

An expanded configuration group draws, before the edges, `rect.group-frame` (rx 12, 1.5 px dashed
accent stroke, accent-tint fill) around its band, a header "RBAC · 13 components · 26 objects" in
the band's top margin, and a Collapse pill: an SVG `<a class="group-collapse">` with a chevron
path, linking to the Graph tab without that group expanded and with `node=<group id>`, so the
group's panel opens on the collapsed group (`graph-16`).

Accordion: a group node's link carries `expand=<that group>` and `fit=<that group>` plus every
expanded id that is not a configuration group (an expanded Pod group stays open). The toolbar
adds Expand all (every configuration group id in `expand`, a frame per group), Collapse all (no
configuration group in `expand`) and a Group configuration toggle (`aria-pressed="true"` while no
configuration group is expanded; it links to Expand all when pressed and to Collapse all when
not). The read API already takes `expand` repeatably, so no new parameter is needed.

### The legend and the ghost (graph-03, instance-18, instance-24)

Under the pane, `div.graph-legend` with CSS swatches: "Applied by the controller" (solid box with
the applied square), "Made by the cluster" (dashed box), "Group of configuration components (CRDs,
RBAC, webhooks, other). Click to open; one opens at a time" (hatched stacked box), "Fill and
outline: health", `<kbd>Ctrl</kbd> + scroll to zoom`, `<kbd>Esc</kbd> to step back`. The toolbar
hint becomes "Select a box to see its details on the right."

When the owner's `inventoryCount` is 0 and `lastAppliedAt` is absent, `buildGraph` adds a ghost: a
dashed box one column right of the rightmost root-side node, joined by a dashed edge, reading
"Nothing applied yet. The graph fills in once an apply succeeds." For a package with no
`sourceArtifact` it adds "The package has never fetched its source." The ghost is not a node: it
has no id, no link and no panel, and the viewBox widens to hold it.

### Hover cards (instance-22, graph-17, provider-16)

`nodeCard` adds: the name as `namespace/name` from `Ref`; the health message clipped to 120 runes;
a parent-aware origin for runtime children from the incoming `controls` edge ("Created by the
cluster from ReplicaSet podinfo-7c9d"); the canvas's origin words ("Display group", "Applied by
the controller", "Reconciled by the controller" for the owner, "Part of the module" for a
component); a fact line with `Replicas` when the document has it; and an accent hint, "Click to
expand" for a collapsed group and "Click for details" otherwise. A registration card adds its
standing badge and "Catalog: <path> <version> · Provider: <this owner>". `showCard` places the card
at the node's right edge plus 12 px, or left of the node when it does not fit, clamped
vertically, through CSSOM properties.

### Selection and spotlight (graph-22, instance-21, graph-11)

Two states, as on the canvas (`CertManager.dc.html:529-554`):

- **Selection** (`node=<id>`): a click, Enter or Space on a node, or the no-script node link. It
  shows the node's panel and a 3 px accent ring with a surface-coloured gap. It dims nothing.
  `portal.js` writes it with `history.replaceState`; a refresh keeps it.
- **Spotlight** (`focus=<id>`): the hand-off from Resources and Health reasons. It selects the node
  as well, draws the ring with a 10 px tint halo, scrolls the frame so the node is centred
  (smooth unless reduced motion), and dims every node and edge outside the node's whole chain at
  0.35 opacity. Hover and keyboard focus apply the same chain dimming as a transient class,
  cleared on leave.

The chain is the transitive closure over the drawn edges in both directions: everything upstream
of the node and everything downstream of it. Server-side, `buildGraph`'s `Dim` uses the same walk
for `focus` (no-script).

```go
// chain returns the ids reachable from id along edges, forward and backward.
func chain(edges []v1.GraphEdge, id string) map[string]bool
```

Hand-off links come from one helper in `graph.go`, which `annotateResources` (`owner.go`, shared
with `align-owner-pages`) calls:

```go
// graphHandoff is the Graph tab spotlighting node id, with group expanded
// and fitted when the node is folded into one.
func graphHandoff(base, id, group string) string
```

It adds `fit=<group>` when a group is expanded (today the link has none, so the group is never
fitted, `owner.go:253-266`).

### The panel at rest (graph-23, provider-21)

With no `focus` and no `node`, the Graph and Resources tabs show the resting node's panel without
spotlight. `restingNode` (in `graph.go`) picks the TransformerRegistration object node of the first
held claim, by name, that the caller may read and whose standing is refused, pending, accepted but
not active, or removal blocked (`Provider.dc.html:621, 644, 680`); otherwise the owner's own node
(`CertManager.dc.html:351, 536`). The resting node carries the plain selection ring. Clearing a
selection (Esc, the Clear selection pill, a click on empty pane) returns to the resting node's
panel: `resetDetail` fetches its panel URL (with the explicit `select` `align-shell-and-tokens`
added to `openNode`) instead of writing the placeholder. Esc does nothing at the resting node and
moves on to the next step.

`ownerTabsOf` (`owner.go`, shared) calls `restingNode` and renders its panel; `align-owner-pages`
restyles the generic panel and owns the Resources tab around it.

### Panel variants (graph-16, provider-15, instance-25, graph-19, instance-20)

`panel.go` and `panel-body.html` (shared with `align-owner-pages`, which restyles the generic
layout) gain three variants, each a block selected by node kind:

- **Group**: the family label, the group's health badge (`graph-26` kept), "A display group, not
  a Kubernetes object. It folds N configuration components so the workloads stay readable.", one
  fact per object kind with its count (`objectKinds`), and Expand group or Collapse group.
- **Registration**: the standing badge (Active, Accepted · not active, Refused, Removal blocked,
  Pending), the controller's message in a tone box in local mode (absent in-cluster, portal:D8:R5),
  the role sentence "Claims a catalog for this provider; the controller decides whether it is
  accepted and active.", the facts Catalog (path and version, linking to `/catalog?path=`) and
  Provider (this owner, and whether `spec.providerRef` names it, portal:D4:R2/D15:R2). Locked
  when the caller may not read registrations (portal:D15:R4).
- **Source**: kind and `namespace/name`, the readiness badge, reason, message and revision; for
  `kindNotServed` the sentence "The cluster does not serve this kind. Install Flux
  source-controller, or point the package at a source kind the cluster has."; for
  `unsupportedKind` "The controller reads only OCIRepository, GitRepository and Bucket."; locked
  when forbidden or not readable.

A Clear selection pill (an × icon, title "Or click an empty part of the graph, or press Esc") sits
at the top right of any panel that is not the resting node's, linking to the page without `node`
and `focus`; the toolbar's text link stays only in full screen, where the aside is hidden. The
click handler finds the page's graph when the pill is outside it.

### Fit, zoom and full screen (instance-15, graph-13, instance-26, graph-08, graph-09, graph-20, graph-15, instance-19)

```js
// overview fit
var w = (frame.clientWidth - 2) / vb.width, h = (paneMax - 2) / vb.height;
var s = Math.min(1.5, w, h);            // 2 in full screen
if (s < 0.6) s = Math.max(0.6, Math.min(1.5, w)); // tall graph: width fit, frame scrolls
frame.style.height = clamp(320, vb.height * s, 760) + "px"; // CSSOM, not an attribute
```

A graph narrower than its frame is centred (`margin: 0 auto` on the SVG). Group fit uses the frame
rectangle: `s = max(0.6, min(1.5, wFit, (frame.clientHeight - 32) / frameH))`, then a smooth
`scrollTo` that centres the frame horizontally and, when it fits, vertically; a taller frame is
shown from its top minus 12 px. Full screen makes the stage a flex column with the frame
`flex: 1`, clears the user zoom and refits with the 2x cap, on entering and leaving.

Zoom range 30 to 200 %, step 5, a 180 px slider, a mono readout; Fit carries `aria-pressed="true"`
while no user zoom is set. Ctrl/Cmd + wheel zooms continuously by `exp(-deltaY * 0.0015)`, clamped
to 0.3 to 2, keeping the SVG point under the cursor fixed:
`frame.scrollLeft = cx * s - mx` after `apply`. The full-screen control is an icon button, 36 x 32,
whose two inline SVG icons swap on `aria-pressed`, with `aria-label` and `title` "Show the graph
full screen" or "Exit full screen (Esc)". A count "N of M boxes" shows how many boxes are drawn of
all there are with every group expanded (folded members and their `objectKinds` counts, Pod group
members, the node-cap summary's hidden counts).

### Motion and the grid (graph-05, graph-25)

After an expand swap, every node with `data-member-of` the expanded group starts at the group
node's old position (a CSSOM `transform` on the SVG `<a>`, recorded before the swap) and
transitions to its place over 420 ms `cubic-bezier(0.2, 0.8, 0.2, 1)`, fading in over 260 ms; the
frame and edges fade in. Zoom changes transition the SVG's size the same way. Under
`prefers-reduced-motion: reduce` nothing animates. This is lighter than the canvas's per-box
morph and is accepted as such.

The pane's grid moves from `.graph` to `.graph-frame` with `background-attachment: local`;
`apply()` sets `--grid-size` to `24 * scale` px through CSSOM, so the grid scales and scrolls with
the graph. `align-shell-and-tokens` removes the page's blueprint grid; this grid is the graph
pane's own, as on the canvas (`CertManager.dc.html:155, 770`).

### Esc and the Overview pill (graph-07, instance-49)

Esc steps back one thing per press: the theme menu, an open card, a selection that is not the
resting node, an open configuration group (as Overview does), then full screen. When a
configuration group is expanded, the toolbar shows an Overview pill (a back chevron, "Overview"
and a muted "Esc"), titled "Close RBAC and go back to the overview", linking to the Graph tab with
no configuration group expanded; it replaces "Whole graph". The legend's Esc line matches.

## Research & Decisions

### Where the family comes from

**Context**: The owner chose groups per kind family; the canvas groups by component-name pattern
(`CertManager.dc.html:436`, `-rbac$|-role$`, `webhook-*`).
**Explored**: F1 cert-manager inventory (20 components, 42 objects) and the canvas script.
**Options considered**:
1. Component-name patterns, as the canvas - matches the mock, but names are module authors'
   choices and say nothing recorded about kinds.
2. The API group of the component's objects - read from the inventory (portal:D4:R1), works for
   any module; a mixed component goes to Other.
3. Per object instead of per component - splits a component across groups, breaking the
   component edge.
**Decision**: Option 2, per component.
**Rationale**: It groups F1 exactly as the canvas draws it, and every input is a recorded field.

### What makes a family a group

**Context**: Today two or more configuration components form the group. Per family, CRDs is one
component with six objects, and Other on cert-manager is one Namespace.
**Options considered**:
1. Two or more components per family - CRDs would show six object nodes at rest, unlike the canvas.
2. Two or more objects per family - CRDs and webhooks group, the lone Namespace does not, as on the
   canvas.
**Decision**: Option 2.
**Rationale**: Grouping exists to hide objects; the object count is the measure of what it hides.

### Keeping the frame honest

**Context**: A frame around members encloses non-members when the layout interleaves them
(`graph-21`, `light-i4-group-open.png`).
**Options considered**:
1. Contiguous members in their column only - objects of other components can still fall inside the
   frame's rectangle.
2. A band across columns reserved for the group - the frame encloses only the group.
3. Separate frames per column - no longer reads as one group.
**Decision**: Option 2, verified by the spike (task 1.1) and pinned by an invariant test.
**Rationale**: The frame says "these belong together"; it must not enclose anything that does
not.

### Selection versus spotlight

**Context**: Live dims one hop on any click and on `focus`; the canvas dims the whole chain on
hover and hand-off focus and never on a plain click.
**Options considered**:
1. One parameter (`focus`) for both, dimming on both - a click hides most of the graph.
2. `focus` for the hand-off spotlight, a new `node` for a plain selection - matches the canvas, and
   a shared link still says which it is.
**Decision**: Option 2; the existing "Graphs can be explored" requirement is amended.
**Rationale**: Clicking to read a node's details should not hide its surroundings.

### Reading the source

**Context**: Owner decision 2026-10-06 ("Read the source"). The source node carries no state, and
the F1 package's controller message is the only sign its kind is not installed.
**Explored**: opm-operator 59537b7 `internal/source/resolve.go:17-90` (the three kinds, the
namespace default, the Ready test, the artifact) and `modulepackage_controller.go:178-241` (the
watches, guarded by a REST mapping check, so a missing CRD is expected).
**Options considered**:
1. Show the controller's reason only (`SourceNotReady`) - already on the Applied card; says nothing
   about the source itself.
2. Read any kind `spec.sourceRef` names - a spec field would steer what the portal reads.
3. Read only the three kinds the controller reads, as the caller, get only - the controller's own
   test, under the user's RBAC.
**Decision**: Option 3; a new portal decision records it (proposal, Impact).
**Rationale**: It shows what the cluster says about the source, and nothing a package author could
point the portal at beyond what the controller itself would read.

### Module node on instance pages

**Context**: The canvas has no Module node; live draws one with a leftward edge (`instance-17`).
**Options considered**:
1. Drop it from the graph document - removes a D4 edge from the API (Principle III).
2. Keep it drawn - the gap stays.
3. Keep it in the document, skip it in the instance page's drawing - the identity card already
   shows path and version.
**Decision**: Option 3.

## Risks / Trade-offs

- [Group ids change] An `expand=` link saved before this change opens the graph collapsed. →
  Ids were never documented as permanent across releases beyond restarts; the API keeps ignoring
  unknown ids.
- [The band layout makes tall graphs taller] RBAC's 13 members and 26 objects reserve 26 rows. →
  Group fit floors at 0.6 and shows the band from its top, as the canvas does.
- [Source read cost] One more `get` and access review per package page load and per package topic
  refresh. → One object per package; list items do not read it.
- [Source state goes stale between package reconciles] A source that turns Ready without the
  package re-reconciling is shown as before until the next refresh. → The controller watches the
  three kinds and re-reconciles on change, which fires the package topic.
- [Two changes edit `owner.go`, `panel.go`, `panel-body.html`] → This change touches named call
  sites and kind-specific blocks only; whichever change lands second rebases.
- [Animation under CSP] CSSOM transforms on SVG `<a>` elements are allowed under the policy, but
  engine support for CSS transforms on SVG links is the spike's to confirm (task 1.3). → If an
  engine refuses, members fade in without moving.

## Migration Plan

None beyond the release. Rollback is the previous release; the API fields are additive.

## Gap map

| Gap | Decision above | Tasks |
| --- | --- | --- |
| graph-01 | Groups per kind family | 2.1, 2.5 |
| graph-02 | Groups per kind family; Group nodes, frames and the accordion | 2.1, 4.3 |
| graph-03 | The legend and the ghost | 4.6 |
| graph-04 | Group nodes, frames and the accordion | 6.2 |
| graph-05 | Motion and the grid | 1.3, 6.4 |
| graph-06 | Expanded groups as a band; Group nodes, frames and the accordion | 2.2, 4.4 |
| graph-07 | Esc and the Overview pill | 6.2, 6.5 |
| graph-08 | Fit, zoom and full screen | 6.2 |
| graph-09 | Fit, zoom and full screen | 6.1, 6.2 |
| graph-11 | Selection and spotlight | 5.2, 5.3 |
| graph-13 | Columns, node size and left-to-right reading; Fit, zoom and full screen | 2.3, 4.5, 6.1 |
| graph-14 | Drawing nodes | 4.1 |
| graph-15 | Fit, zoom and full screen; Group nodes, frames and the accordion | 6.2, 6.3 |
| graph-16 | Panel variants; Group nodes, frames and the accordion | 4.4, 5.5 |
| graph-17 | Hover cards | 4.8 |
| graph-19 | Panel variants | 5.6 |
| graph-20 | Fit, zoom and full screen | 6.3 |
| graph-21 | Expanded groups as a band | 1.1, 2.2 |
| graph-22 | Selection and spotlight | 5.1, 5.2 |
| graph-23 | The panel at rest | 5.4 |
| graph-24 | Drawing nodes (edges) | 4.2 |
| graph-25 | Motion and the grid | 6.4 |
| graph-26 | Kept divergence (Non-Goals); Groups per kind family | 2.1, 5.5, 6.7 |
| instance-14 | Columns, node size; Drawing nodes | 2.3, 4.1 |
| instance-15 | Fit, zoom and full screen | 6.1 |
| instance-16 | Drawing nodes (edges) | 4.2 |
| instance-17 | Columns, node size and left-to-right reading | 2.3, 4.5 |
| instance-18 | The legend and the ghost | 4.6 |
| instance-19 | Fit, zoom and full screen | 6.3 |
| instance-20 | Panel variants | 5.6 |
| instance-21 | Selection and spotlight | 5.1, 5.2 |
| instance-22 | Hover cards | 4.8 |
| instance-24 | The legend and the ghost | 4.7 |
| instance-25 | The package's source state; Drawing nodes; Panel variants | 1.2, 3.1-3.4, 4.1, 5.5 |
| instance-26 | Fit, zoom and full screen | 6.1 |
| instance-49 | Esc and the Overview pill | 6.5 |
| provider-14 | Registration standing on instance and package graphs; Drawing nodes | 2.4, 4.1 |
| provider-15 | Panel variants | 5.5 |
| provider-16 | Hover cards | 4.8 |
| provider-21 | The panel at rest | 5.4 |

## Open Questions

None that change the specs or the tasks. The spike (section 1) confirms three assumptions and
records its findings here: the band layout's invariant on cert-manager, the constructed Flux source
fixtures through the read-model test harness, and CSS transforms on SVG links in the three
engines.
