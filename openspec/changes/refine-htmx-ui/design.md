## Context

PR 22 (`add-htmx-ui`, archived in the same branch) serves the pages. Its review found one
blocking defect (an open page that does not show a broken workload) and a set of should-fix
items. This change fixes them before the PR merges.

## Goals / Non-Goals

**Goals:** the open page shows the image break in every region; the UI reads only the read API;
the two axes stay on separate visual channels everywhere, the graph included; red means broken
and nothing else; escaping is held by a test.

**Non-Goals:** new pages, new reads, a client-side layout, theming beyond the tokens.

## Decisions

### Live refresh: one fetch per changed topic

A followed topic's change marks it dirty; 400 ms after the first mark the script fetches the
page once (`fetch(location.pathname + location.search)`), parses it with `DOMParser`, and
replaces every region whose `data-follow` names a dirty topic with the region of the same `id`
from that document, then calls `htmx.process` on it. All regions of one refresh come from one
render. Open `<details>` groups keep their state by id, and the selected graph node and zoom are
restored. The detail panel and log panes are not followed and are never replaced.

### Conditions read from the API

`v1.Condition` gains `tone` (required; `x-extensible-enum`
`[normal, abnormal, progressing, informational, unknown]`) and the optional `meaning` and
`nextStep`. `internal/health.ConditionTone` decides the tone from the type and status:

| Type | True | False |
| --- | --- | --- |
| Ready, ModuleResolved | normal | abnormal |
| Active | normal | informational |
| Stalled | abnormal | normal |
| Reconciling | progressing | normal |
| Drifted | informational | normal |
| ContractsFulfilled | normal | informational |

Any other type, and any `Unknown` status, is `unknown`: the portal does not guess a polarity it
does not know (Principle IV). `meaning` and `nextStep` come from `health.Explain`, the table the
UI rendered before.

```json
{"type":"Stalled","status":"True","reason":"RenderFailed","message":"...","tone":"abnormal",
 "meaning":"The module could not be rendered.","nextStep":"Read the message; ..."}
```

### Graph marks

The node box stroke carries health (degraded red, missing dashed), and a small square stamp in
the node's top-right corner carries the applied state, in the applied badge's colours (failed and
stalled red, reconciling blue, neutral dashed). An edge whose `verified` is false is drawn as
broken only when the node at its far end is readable; when that node is locked, or the reason is
`ProviderUnreadable` or `ProviderNotLookedUp`, it is drawn dashed in the locked colour. Labels
longer than the node keep their head and tail with a middle ellipsis; a catalog or module label
keeps its last path segment and version. Every node carries a `<title>` with its full label.

### Research & Decisions

#### Where the image break was lost

**Context**: the review suspected the producer missed Pod and ReplicaSet changes.
**Explored**: a Playwright trace on a throwaway cluster logged, every 0.5 s after the patch, the
header badge, the component rows, the graph's runtime nodes and the read API's instance document.
**Options considered**:
1. Mark owner topics dirty on child changes in the producer. The read model already turns a
   labelled child's change into an owner change (`readmodel.changesFor`), and the trace showed the
   API document holding the new ReplicaSet and the `ErrImagePull` Pod 0.67 s after the patch.
2. Fix the page script. The trace showed two page GETs per change for five followed regions:
   `htmx.ajax` without `source` synchronises on one element, so the first request runs, the last
   is queued and the rest are dropped; the graph and components were never refreshed.
**Decision**: option 2, with one fetch per change.
**Rationale**: the evidence locates the defect in the script; the producer needs no change.

#### Condition tone in the API or in the UI

**Options considered**:
1. A UI-side table of condition types. Cheap, but it is interpretation an API client would need
   too, and it is the same kind of fact as the reason table the review moved into the API.
2. A `tone` field on the API's `Condition`. Additive, one table in `internal/health` beside the
   axis that already knows these types.
**Decision**: option 2.

## Risks / Trade-offs

- [A full page per refresh] → one per change instead of one per region; the API answers from held
  state (0030:D3:R9).
- [A fetched page whose session ended] → the response is the sign-in page; it has no matching
  regions, so nothing is swapped and the live indicator says offline.
