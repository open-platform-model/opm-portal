## Why

The review of the web UI (PR 22, change `add-htmx-ui`) found that the open page did not show a
broken workload: after an image break the header read Degraded while the graph and the
components still showed the old, healthy ReplicaSet. A live trace showed the read API served the
new Pod within 0.7 s, but the page sent only two of its five region refreshes: every region
refreshed through `htmx.ajax` without a source element, so htmx synchronised them on one element
and dropped the rest. The review also found that the page showed condition meanings the read
API does not serve, so the UI read something an API client cannot (0030:D2:R1); that the graph
drew both axes on one stroke (0030:D3:R1); that the accent colour sat in the degraded red; that
condition stripes were coloured by status only; and a set of graph, phone-width and repetition
problems visible in the screenshots. No test failed if a page stopped escaping cluster text.

## What Changes

- Read API, additive: a `Condition` carries `tone` (how it reads for its type: normal,
  abnormal, progressing, informational, unknown) and the portal's `meaning` and `nextStep` for
  its reason. The UI renders conditions from these fields and no longer imports
  `internal/health`; the import test allows only `api/v1alpha1` among this module's packages.
- Live updates: a change fetches the page once per changed topic and swaps every region that
  follows the topic from that one response, so the regions always come from one moment.
- Graph: the node box stroke carries health only, and a square stamp on the node carries the
  applied state; the selected node is marked; labels keep their distinguishing tail with a
  middle ellipsis and carry the full name as an SVG title; a graph is fitted to its frame on
  load and after a swap; an edge that cannot be confirmed because its end is locked is drawn in
  the locked style, not as broken; edges have their own token; the toolbar wraps on a phone.
- Colour: the accent leaves the red family, so red means degraded, failed or refused only.
  Condition stripes follow `tone`.
- Pages: configuration components fold into one group with a count and their worst health,
  open when any member is not healthy; ReplicaSets at zero replicas fold under "old revisions";
  object references break at `.` and `/`; the masthead is opaque and compact on a phone; log
  rows show a disclosure mark and each Pod row links to its log; a partial health is marked
  visually; the detail panel scrolls into view at any width; repeated messages, the duplicate
  verdict badge and the contracts condition already shown in the banner are dropped; list
  kickers and stat labels read as words.
- Tests: every F1 page and fragment rendered over a capture whose free text is hostile shows no
  raw markup, and a source check fails on a `template.HTML`, `JS`, `URL`, `HTMLAttr` or `CSS`
  conversion in `internal/ui` or an `innerHTML`, `outerHTML` or `insertAdjacentHTML` in the page
  script.

## Capabilities

### Modified Capabilities

- `web-ui`: reads only `api/v1alpha1`; conditions, graph marks, live refresh, grouping and
  escaping as above.
- `read-api`: conditions carry tone, meaning and next step.

## Impact

- Packages: `internal/health` (condition tone), `api/v1alpha1` and `internal/api` (`Condition` gains
  `tone` and the optional `meaning` and `nextStep`; OpenAPI, goldens), `internal/ui` (templates, view, graph, CSS,
  script, tests). No new dependency, no new read and no new verb.
- API: additive; `task api:breaking` stays green.
- Principle V: unchanged. The new fields are the portal's own text and a function of the
  condition's type and status.
- SemVer: MINOR after 1.0 (additive API fields); on the 0.x line it rides PR 22's `feat(ui)`.
- Enhancement link: refines 0030:D2:R1, 0030:D3:R1/R3/R8, 0030:D4 and 0030:D5:R7 as shown on the
  pages. See `enhancement.yaml`.
