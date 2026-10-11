## Why

The owner's canvas shows two facts on the details panel of an instance's own node: the inventory
("42 objects") and its revision ("Revision 1"). The panel the canvas alignment builds
(`align-owner-pages`, `align-graph`) has neither. The count is already in the read API
(`inventoryCount`). The revision is not: the operator records it in `status.inventory.revision` and
the instance document drops it. The owner approved both facts on 2026-10-11 as part of the
canvas-alignment plan; the sibling changes left them out because the revision needs a wire field
and a golden pass of its own.

Depends on `align-owner-pages` and `align-graph`: they own the details panel, its root-node facts
and the owner document the panel reads. This change adds two facts to what they build and touches
no other panel behaviour.

## What Changes

- **Read API, additive** (section 1):
  - `Instance` and `InstanceSummary` gain `inventoryRevision`, the operator's
    `status.inventory.revision`, an integer. It is absent when the operator recorded none (an
    instance never applied), never a default and never a guess.
  - `openapi/v1alpha1.yaml` gains the optional property on both schemas; the read-API reference
    page gains its row.
  - The change stream already carries the whole instance document, so a revision change reaches
    `instance:` subscribers with no stream change.
- **Details panel, root node of an instance** (section 2):
  - Two facts join the identity facts that `align-owner-pages` builds: "Inventory" ("42 objects")
    and "Inventory revision" ("3").
  - Neither shows for a node the caller may not read, for a package, or for any node that is not
    the page's own instance. "Inventory revision" is absent when the API serves none.
  - The label "Inventory revision" differs on purpose from the package page's "Revision", which is
    the package source's revision (`align-owner-pages` design.md, Root node).
- `ROADMAP.md` records the change.

Not in this change: the revision on `ModulePackage` (the operator writes it there too; nothing
shows it yet), "in N components" after the object count (the canvas shows it; it comes from the
graph document, not from `InventoryCount`), the revision on the identity card, the Installed list
or the history strip, and any text that explains what a revision means.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `read-model`: the instance view carries the operator's inventory revision.
- `read-api`: instance documents and list items serve `inventoryRevision`.
- `web-ui`: the details panel of an instance's own node shows the inventory and its revision.

## Impact

- Packages: `api/v1alpha1` (one field on `InstanceSummary`, which `Instance` embeds),
  `internal/readmodel` (parse and carry), `internal/api` (convert), `internal/ui` (the root-node
  facts in `panel.go` and `panel-body.html`), `openapi/v1alpha1.yaml`, `docs/site/reference/portal`,
  `ROADMAP.md`.
- API resources: `GET instances/{namespace}/{name}` and the instance list items, and the same
  documents on the change stream. Packages, the Platform, graphs and events are unchanged.
- UI pages: the details panel on the Graph and Resources tabs of every instance page.
- Tests: API goldens, UI goldens for the instance pages (local and in-cluster), the read-model
  parse table, `TestBrowserOwner` (one assertion) and the `TestBrowserPhone` page list.
- SemVer: MINOR after 1.0 (an additive API field, one changed page area). On the 0.x line it ships
  as one PR titled `feat(api): serve the inventory revision and show it on the instance panel`;
  section commits are `feat(api)` and `feat(ui)`, so the PR title carries `feat`. `task
  api:breaking` stays green (an optional response property is added).
- Principle V (read-only, no Secret data, fail closed): no new kind, no new verb, no new identity.
  The revision is a number from `status` of the ModuleInstance the portal already holds and
  already authorizes (`get moduleinstances`, portal:D7). It is not operator text, so the
  in-cluster omission of operator messages (portal:D8:R5) does not apply and it is served in both
  modes.
- Principle VII (simplicity): no dependency, no JavaScript, no CSS rule, no template define. The
  two facts are two rows of the existing facts list.
- Decisions: implements portal:D2:R1/R2/R3 (every fact the UI shows is in the API; additive
  change; clients ignore unknown fields) and portal:D17 (the UI shows what the cluster records).
  It changes no decision and adds none, so `docs/DESIGN.md` is not edited and there is no
  `enhancement.yaml`.
- Shared-file rule: this change edits only the root-facts block of `internal/ui/panel.go` and
  `panel-body.html`, after `align-owner-pages` and `align-graph` have both merged. It brings in
  their `main` by merge, never by history rewrite.
- Sections: two. The first ends with a releasable API, the second with the panel. No spike: the
  one thing this change cannot check today, the shape of the root-facts code that the sibling
  changes will merge, is read in task 2.1 and stops the section if it differs (design.md Risks).
