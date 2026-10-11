## Context

See proposal.md, Why. Three facts about the current code and the operator shape the approach.

- **The operator records the revision as an integer, starting at 1.** `Inventory.Revision` is an
  optional `int64` (opm-operator `api/v1alpha1/common_types.go:176-178`, main at `cb97ffb0`,
  read 2026-10-11). `nextInventory` writes `prev.Revision + 1`, or `1` when there is no previous
  inventory (`internal/reconcile/moduleinstance.go:1187-1196`), and the instance reconcile calls
  it only when the apply and prune succeeded (`moduleinstance.go:370`). So the number counts
  successful applies that wrote a new inventory; it is not a spec generation and not the module
  version. A ModulePackage records it the same way (`modulepackage.go:264`).
- **The F1 capture holds it for all five instances, always `1`.** `testdata/clusters/f1/moduleinstances.yaml`
  has `status.inventory.revision: 1` on `cert-manager/cert-manager`, `default/podinfo`,
  `default/backup-provider`, `default/backup-consumer` and the CLI-owned `web/web`, with counts 42,
  2, 1, 1 and 2. `pkg/podinfo` records no inventory. No capture instance has a revision above 1 or
  none, so those two cases need unit-test fixtures.
- **The portal drops it today.** `inventoryCount` reads `status.inventory.count`, falling back to
  the entry count (`internal/readmodel/parse.go:186-192`); `instanceItem` builds the summary that
  both the list and the instance document use (`internal/readmodel/instances.go:70-82`); the wire
  type is `InstanceSummary`, which `Instance` embeds (`api/v1alpha1/types.go:187`); the
  OpenAPI document spells the two schemas out separately (`openapi/v1alpha1.yaml:734` and `:807`),
  so both need the property. `Instance` and `InstanceSummary` carry `inventoryCount` as a
  required integer; the history entries name theirs `inventoryCount` and `inventoryDigest`.

The panel side is merged: `align-owner-pages` and `align-graph` landed on `main` as PRs 48 and 49
(base `360382a`, 2026-10-11). The panel builds the owner's identity facts from the owner document
that `panelContext` carries; this design adds two facts to those and leaves the variants alone.

## Goals / Non-Goals

**Goals:**

- Serve the operator's inventory revision on the instance document and list item, additively.
- Show "Inventory" and "Inventory revision" on the panel of an instance's own node, only where the
  caller may read the node, with no unrecorded value drawn.

**Non-Goals:**

- The revision on packages, and any display of it on a package page. The operator writes it there
  too (`modulepackage.go:264`), but the package root panel's "Revision" fact already means the
  source revision, and no page shows an inventory revision for a package. The owner ruled on
  2026-10-11 that packages do not serve it yet. Adding the field later is additive. Trigger to
  revisit: a page shows the package's inventory revision.
- "in N components" after the object count. The canvas shows "42 objects in 20 components"; the
  component count comes from the graph document, varies with how the graph folds configuration
  components, and the approved item names `InventoryCount` only.
- Any explanation of what a revision is, in the panel (no tip, no `title`). The docs row in the
  read-API reference says it.
- Changing the identity card's "Objects" fact, which `align-owner-pages` keeps. The same count then
  shows twice on a page, as "Objects" on the card and "Inventory" on the panel. The owner accepted
  that on 2026-10-11: the panel shows the root node's facts, the card the page's.
- The graph document (`GraphNode`) gains nothing. The count and the revision are facts of the
  owner, not of a node, and the UI reads only the API (portal:D2:R1).

## Research & Decisions

### Where the wire field lives, and its name

**Context**: The UI may read only what the API serves (portal:D2:R1). The panel's root facts come
from the owner document (`align-owner-pages` design.md, Root node).
**Explored**: The operator type above; the wire types at `types.go:145,196,277`; the way
`align-owner-pages` adds `prune` (`PackageSummary`, optional, "when set, nothing when not, never a
default": its read-api spec).
**Options considered**:
1. `inventoryRevision` on `InstanceSummary` (so on the list item and the document), `omitempty`
   integer. Sits beside `inventoryCount`, follows the history entries' `inventoryCount` and
   `inventoryDigest` naming, one edit point (`instanceItem`).
2. A nested `inventory: {count, revision}` object. Duplicates `inventoryCount`, which cannot be
   removed within `v1alpha1` (portal:D2:R2), so two homes for one fact.
3. On `Instance` only, not the list item. Saves one property; the list and the document would then
   disagree about what an instance carries, and `prune` set the other precedent.
4. On `InstanceSummary` and `PackageSummary`. Symmetric, but it serves a value nothing shows yet.
**Decision**: Option 1.
**Rationale**: One flat name beside its sibling, one parse site, no second home to keep in step.
Packages wait for a consumer (Non-Goals).

### What the field holds when the operator wrote nothing, or something odd

**Context**: Principle IV: unknown is unknown, never a guess. The operator omits a zero revision
(`omitempty`), and starts at 1.
**Options considered**:
1. Serve any integer found, `0` included. A `0` would print "Inventory revision 0" for an instance
   whose revision field is absent but whose parse yields the zero value: `omitempty` on the wire
   type hides it, but the UI would have to guess which.
2. Serve the value only when it is found and at least 1; otherwise absent. A negative value, or a
   string where an integer belongs, is treated as not recorded. The operator never writes either
   (`moduleinstance.go:1189-1196`), so this only guards a hand-edited or foreign object.
**Decision**: Option 2. The parser is one function in `internal/readmodel/parse.go` beside
`inventoryCount`, using the existing `i64` helper (a wrong type is already "not found").
**Rationale**: The wire field means "the revision the operator recorded". An absent field is the
only honest answer for an instance never applied, and the UI needs no second rule to tell absent
from zero.

### The label of the second fact

**Context**: The entry leaves this open: the package page's "Revision" is the source's revision
(`align-owner-pages` design.md, Root node; `internal/ui/templates/owner.html:24`), and one name
must name one thing.
**Explored**: The canvas's root panel pushes the facts `Inventory` = "42 objects in 20 components"
and `Revision` = "1" (`docs/design/evidence/03-ui-canvas/CertManager.dc.html:621`); no other board
has either. The portal already uses "Applied" for the controller's reconcile state (portal:D19:R3),
and "Version" for the module version.
**Options considered**:
1. "Revision". The canvas word, but a user who sees "Revision" on a package panel and on an instance
   panel reads one concept; the two are not (source artifact versus inventory writes).
2. "Inventory revision". Names the thing, matches the wire name `inventoryRevision`, pairs with the
   "Inventory" fact above it, and cannot be mistaken for the module's "Version" or a source
   revision.
3. "Applied revision". Reuses a word the portal reserves for the reconcile axis (portal:D19:R3), so
   rejected.
4. One combined row, "Inventory: 42 objects, revision 3". Removes the label question; but it
   merges two facts the approved item names apart, and a missing revision would change the row's
   shape.
**Decision**: Option 2. The facts read `Inventory` / `42 objects` (`1 object` for one) and
`Inventory revision` / `3`.
**Rationale**: One name per thing. It departs from the canvas word by one qualifier, for a reason
the owner can see on the page. The owner settled the label on 2026-10-11: the package page's
"Revision" already names the source revision.

### When each fact shows

**Context**: portal:D17 shows what the cluster records. `inventoryCount` is always present on the
wire; it is the entry count when the operator wrote none (`parse.go:186-192`), so `0` can mean "never
applied" or "applied and empty".
**Options considered**:
1. Always show both. An instance never applied would read "0 objects" with no revision: a fact the
   cluster did not record.
2. Show "Inventory" when the count is above zero or a revision is recorded; show "Inventory
   revision" when recorded.
**Decision**: Option 2.
**Rationale**: A never-applied instance shows neither fact (its graph already shows the empty
inventory, which `align-graph` draws as the ghost node); an applied instance that renders nothing
shows "Inventory 0 objects" and its revision. The rule is one boolean in the view model.

### Which nodes carry the facts, and where the data comes from

**Context**: The panel is generic: any node, on any owner page or the Platform page, opens in it.
`align-owner-pages` gives the node whose `Ref` is the page's own owner its identity facts through
`panelContext`.
**Options considered**:
1. Put the two facts on `GraphNode`. Widens the graph document and every graph golden, and the
   facts would be on a node, not on the owner.
2. Read them from the owner document that `panelContext` already carries for the root node. No new
   fetch, no graph change.
**Decision**: Option 2. The facts show only when the panel's node is the page's own ModuleInstance
(the `Ref` equals the owner and the owner kind is instance). A ModuleInstance node seen from another
page (a provider's instance on the Platform graph, an instance on a registration panel) is not
the owner there and shows neither fact. A node whose `Access` is not `ok` renders in the locked
form of the template, which has no facts block, so it carries neither.
**Rationale**: The data is already in the page's own request; the locked branch of the template
is the same gate every other fact passes.

### The shape of the code to extend

Read on merged `main` (`360382a`), 2026-10-11. The owner document reaches the root panel:

- `panelContext.self` is the `*ownerView` of the page (`internal/ui/panel.go:11`). The page sets it
  when it builds the resting panel (`owner.go:461`), and the panel fragment route sets it when the
  requested node is the owner's own node and the owner document read without a problem
  (`pages.go:720-731`).
- `panel()` sets `p.Root` through `isOwnerNode` (a node of kind instance or package whose page path
  equals the page's own) and then `p.Facts = ownerFacts(ctx.self)` (`panel.go:178-180`). A node
  that is locked returns before this (`panel.go:157-159`).
- `ownerFacts` returns a `[]fact` (`Label`, `Value`, `Mono`) for the instance or the package; the
  template renders `.Facts` in the not-locked `<dl class="facts">` (`panel-body.html:15`). The
  instance's document is `ownerView.Instance`, a `*v1.Instance`, so it carries the two values once
  `InstanceSummary` has the field.

So the change adds two entries to the instance branch of `ownerFacts`, with no field, no template
edit and no new type. `ownerFacts` is the only place the two facts are written; a package, a
non-owner node and a locked node never reach its instance branch.

## Interfaces

The wire type (`api/v1alpha1/types.go`), after `InventoryCount`:

```go
// InventoryRevision is the operator's status.inventory.revision: the count
// of successful applies that wrote an inventory, starting at 1. Absent when
// the operator recorded none.
InventoryRevision int64 `json:"inventoryRevision,omitempty"`
```

OpenAPI, on `InstanceSummary` and `Instance` (not in `required`):

```yaml
inventoryRevision:
  description: "The operator's status.inventory.revision, an integer from 1; absent when it recorded none."
  type: integer
  minimum: 1
```

An instance document from the F1 capture, after the change (other fields as today):

```json
{ "kind": "ModuleInstance", "ref": { "namespace": "default", "name": "podinfo" },
  "inventoryCount": 2, "inventoryRevision": 1 }
```

Errors: none new. The field is part of a document that already exists; the `403`, `404` and `503`
cases of the instance routes are unchanged and run before the lookup (portal:D7).

The parser, in `internal/readmodel/parse.go`:

```go
// inventoryRevision is status.inventory.revision when the operator wrote an
// integer of at least 1, else 0 (not recorded).
func inventoryRevision(u *unstructured.Unstructured) int64
```

The instance branch of `ownerFacts` (`internal/ui/panel.go`) gains two facts after "Version",
before "Owner":

```go
if n := v.Instance.InventoryCount; n > 0 || v.Instance.InventoryRevision > 0 {
	out = append(out, fact{Label: "Inventory", Value: fmt.Sprintf("%d %s", n, plural(int(n), "object", "objects"))})
}
if r := v.Instance.InventoryRevision; r > 0 {
	out = append(out, fact{Label: "Inventory revision", Value: strconv.FormatInt(r, 10)})
}
```

The template renders them through the existing `.Facts` loop, as escaped text (`plural` is
`internal/ui/view.go:358`; it takes an `int`, so the call converts).

Reads and grants: the change adds no read. It serves the field from the `get moduleinstances` and
`list moduleinstances` reads the portal already makes, under the grants the caller already holds
(portal:D7). No Kubernetes verb or resource is added, and no ClusterRole changes.

## States of the new facts

| State | What the panel shows |
| --- | --- |
| Loading | Unchanged: the panel is a server-rendered fragment; E5's busy state belongs to `add-shell-states-and-cluster-name`. |
| Empty: instance never applied (count 0, no revision) | Neither fact. The graph shows the ghost node. |
| Empty: applied, nothing rendered (count 0, revision 2) | "Inventory 0 objects", "Inventory revision 2". |
| Normal | "Inventory 42 objects", "Inventory revision 1". |
| Error: the document could not be read | The page is the `403`, `404` or `503` page; the panel does not render. A failed read of an inventory object (the per-kind facts of `align-owner-pages`) does not touch these two facts, which come from the owner document. |
| Permission denied: root node locked | The locked form of the template: kind, name, a lock, no facts. |
| Not the owner's node (package root, another page's instance node, a Pod, a component) | Neither fact. |

## Accessibility and layout

- The facts are two more `dt`/`dd` pairs of the panel's existing definition list. No colour, icon,
  hover or script carries them; the text is the only carrier. No new CSS, no new focus stop.
- At 360 px the facts list is the panel's own; the label "Inventory revision" is the longest label
  of the root facts. Task 2.4 checks it at 360 px in light and dark and records the result; if the
  label wraps, it wraps inside its cell (the list's grid already allows it) and the page does not
  scroll sideways.
- Contrast and target size: the facts use the panel's existing text tokens, so they inherit the
  floor that `fix-contrast-and-add-phone-check` sets. Not measured here; no new colour is added.

## Risks / Trade-offs

- [The owner document does not reach the root panel] -> Task 1.1 checks it first and stops the
  change if it does not (done on `360382a`: it does, see "The shape of the code to extend").
- [The same count shows as "Objects" on the identity card and "Inventory" on the panel] -> Accepted
  as the owner-approved canvas; recorded in Non-Goals.
- [An `int64` revision that the operator later stops incrementing, or resets on recreate] -> The
  portal shows the number as recorded. It explains nothing about it, so no wording goes stale.
- [`inventoryRevision` served on instances but not packages] -> Additive to fix later (Non-Goals);
  a client that needs the package value waits for the page that shows it (Non-Goals).
- [Local `task check` stops at `openspec:check`] -> The local openspec 1.14.1 warns on
  requirement text over 500 characters in 13 main specs; CI pins 1.12.0 and `main` is green there.
  The requirements of this change stay under 500 characters. `openspec validate
  add-inventory-revision --strict` is the gate for the change; report `openspec:check` as run and
  failing on the same 13 specs only.
- [Golden churn] -> Every instance golden gains one line and nothing else. Task 1.4 checks that
  the diff is only that.

## Migration Plan

None: an optional property and two panel rows. Rollback is the revert of the two section commits.
Clients that ignore unknown fields (portal:D2:R3) are not affected.

## Open Questions

None. The owner settled the package revision, the label and the count shown twice on 2026-10-11
(Non-Goals, "Research & Decisions").
