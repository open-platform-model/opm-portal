## Context

The instance and package pages come from `redesign-web-ui` (archived 2026-10-05). They live in
`internal/ui/owner.go`, `provider.go`, `panel.go`, `pages.go` (Resources rows, `foldConfig`, log
panes) and the templates `owner.html`, `panel-body.html` and `partials.html`. The gap report
`docs/design/evidence/05-canvas-gap-report/gaps.md`, section C, lists 38 verified gaps between
these pages and the canvas boards `Instance.dc.html`, `Package.dc.html`, `Provider.dc.html`,
`ProviderPackage.dc.html` and `CertManager.dc.html` in `docs/design/evidence/03-ui-canvas/`.
Each decision below names the gap ids it closes; the table at the end maps every id to its
tasks.

Binding inputs:

- Owner answers of 2026-10-06:
  - "Follow the canvas (Recommended)" for the global look, built by `align-shell-and-tokens`.
  - "Merge all (Recommended)" for the Events tab, recorded as portal:D9:R6.
- Supervisor rulings of 2026-10-06, which the owner may overturn in review:
  - Resources and Events become tables.
  - The Applied axis keeps today's words.
  - Configuration objects keep kstatus health.
  - The Conditions fold moves below the tab panels.
  - Everything else follows the canvas.

Every claim about controller output names the F1 capture (`testdata/clusters/f1/`, opm-operator
v1.0.0-beta.6), as the redesign did.

### What this change takes from its sibling changes

This change starts after `align-shell-and-tokens` merges. It uses these pieces from that change
(its design.md, "The state block" and "Underline tabs with counts"):

- **The `state-block` partial and its `stateBlock` value**: `ID`, `Label`, `Eyebrow`, `When`
  (`*stamp`), `WhenText`, `Hue` (applied, healthy, progressing, degraded, unknown, neutral,
  missing, locked), `State`, `Summary`, `Reasons`, `None`, `Follow` and `Problem`. The icon
  follows the hue. The partial already shows `WhenText` as plain words when `When` is nil (moved
  into `align-shell-and-tokens` by the integrator on 2026-10-06, so that this change and
  `align-platform-installed-catalog` do not both edit the partial); only the Health block uses it,
  for "checked live". This change makes one additive edit to the partial, and blocks that do not
  set the field render as before:
  - A `History *historyStrip` field draws the Applied card's attempt strip inside the block's
    section, after the reasons. This change is the only follow-on change that edits the partial.
- **The tone tokens** (border, ink, background, tint) in light and dark.
- **Underline tabs and `tabLink.Count`.** The owner tabs' Resources and Events counts already come
  from that change: inventory objects plus runtime children, and the folded lines of the feed the
  Events tab opens with. A list the caller may not read in full gets no count.
- **The square badges and the visually hidden region headings.**
- **The `openNode` select fix (`instance-43`).**

From `align-platform-installed-catalog`, which runs in parallel and owns the generic filter code:

- **The `opm-js` class** that `prefs.js` sets, which hides an Apply button.
- **The `.tipbox` tooltip CSS.**
- **`providerBadge`'s pill**: the word "Provider", with the standing in hidden text and in a
  tooltip.
- **The `events-table` partial** with its `eventsTable` and `eventRow` input. Whichever of the two
  changes lands first adds it, and the other adopts it.

This change edits neither `filters.go`, the `filter-form` partial, nor the filter code in
`portal.js` and `prefs.js`. Its Events, Logs and YAML forms carry `data-filters` keys (none of
them a remembered view), so the existing select auto-submit applies them. If this change lands
before `align-platform-installed-catalog`, the owner-page Apply and Show buttons stay visible until
`opm-js` exists. That is today's behaviour, not a regression.

`align-graph` runs in parallel and decides the node the details panel rests on and the
`resetDetail` script (its `graph-23` and `provider-21`). This change's `instance-27` is the same
behaviour: decision 5 builds it only if `align-graph` has not landed it, and whichever change
lands second rebases.

## Goals / Non-Goals

**Goals:**

- The owner pages read like the canvas boards: state blocks, tables, a details panel that is
  never empty, one open log pane, and a Provider tab that states each fact once.
- The Events tab answers "what is going wrong" on arrival (portal:D9:R6) without one read per
  object.
- Every number and sentence on a card comes from a recorded field. A sentence the controller did
  not write is never phrased as its verdict.

**Non-Goals** (each a part of a section C gap left out, with its reason):

- **Age columns in Resources** (`instance-30`, `graph-10`). No time per inventory entry or child
  is served (`instance-31`, decided out).
- **Service Endpoints in the panel** (`instance-28`). They would need a read of EndpointSlices, a
  kind the portal does not read; a new kind needs its own decision under Principle V. Type and
  Ports come from the Service object.
- **The text "Applying v0.1.12 over v0.1.11"** (`instance-40`). No read API field carries the
  module version the controller applied last. The block shows the condition reason instead.
- **The owner's own YAML and the package source's YAML in the picker** (`instance-36`). Raw YAML
  is served only for inventory objects (portal:D8, `instance-46` decided out).
- **Catalog contents on the Provider tab** (`provider-09`): `#Definition` names, a provided
  contract's kind and status, and transformers (portal:D17, portal:OQ25; `provider-10`,
  `provider-13` and `instance-37` decided out). The contract pill shows the recorded path in
  short form.
- **Remembered Events filters** (`instance-42`, portal:D14). The Events tab stays a URL-only
  view.
- **Graph drawing, grouping, the source node read and registration node standing.** These belong
  to `align-graph`; the package source row here only shows what the graph document carries.

## Decisions

### 1. Merged owner events in the read API (`instance-44`; portal:D9:R6)

Request:

```text
GET /api/v1alpha1/clusters/{cluster}/instances/{ns}/{name}/events?scope=all
GET /api/v1alpha1/clusters/{cluster}/packages/{ns}/{name}/events?scope=all
```

- `scope` takes `owner` (the default, today's behaviour) or `all`.
- `scope=all` together with the `group`, `kind`, `namespace` and `name` object parameters is
  `400 bad_request`: one request is either one object or everything.
- Any other `scope` value is also `400 bad_request`.

Response: the `EventList` of today with two additive fields.

```go
type EventList struct {
	TypeMeta
	Regarding ObjectRef `json:"regarding"`
	Items     []Event   `json:"items"`
	// Scope is "all" when the list merges the owner's events with those of
	// every object its inventory reaches; absent for one object's feed.
	Scope string `json:"scope,omitempty"`
	// Partial: some reached objects were left out because the caller may
	// not read them or may not list events where their events live.
	Partial bool `json:"partial,omitempty"`
}

type PackageSummary struct {
	// ... unchanged fields ...
	// Prune is spec.prune as written; absent when the package does not set it.
	Prune *bool `json:"prune,omitempty"`
}
```

```json
{
  "apiVersion": "portal.opmodel.dev/v1alpha1",
  "kind": "EventList",
  "regarding": {"group": "opmodel.dev", "version": "v1alpha1", "kind": "ModuleInstance", "namespace": "default", "name": "podinfo"},
  "scope": "all",
  "items": [
    {"type": "Normal", "reason": "SuccessfulCreate", "reportingController": "replicaset-controller",
     "regarding": {"group": "apps", "version": "v1", "kind": "ReplicaSet", "namespace": "default", "name": "podinfo-podinfo-d9585d794"},
     "count": 1, "lastSeen": "2026-10-04T18:04:06Z"},
    {"type": "Normal", "reason": "ScalingReplicaSet", "...": "..."}
  ]
}
```

Problems:

| Case | Response |
| --- | --- |
| Not signed in | `401 unauthenticated`, as today |
| Get on the owner denied, or the owner missing | `403 forbidden`, the same document (portal:D7) |
| List of events denied in the owner's own namespace | `403 forbidden`, as today's owner feed |
| List denied in another namespace | `200` with those objects left out and `partial: true` |
| A list in some namespace fails | `200` with those objects left out and `partial: true` |
| `scope` unknown, or `scope=all` with object parameters | `400 bad_request` |
| The read model not yet synced for the owner | `503 not_readable_by_portal`, as today |

Authorization, in order, each review before the lookup it covers:

1. `get` on the ModuleInstance or ModulePackage, as today.
2. The owner's inventory, read from held state under that grant (portal:D3:R9). Each object
   carries the caller's `access`. Objects that are not `ok`, and every core Secret, are dropped.
   Runtime children are kept as the inventory document carries them.
3. The event namespace of every remaining object, from `readmodel.EventNamespace`: the object's
   own namespace, or `default` for a cluster-scoped object (portal:D9:R4).
4. `list` on `events.k8s.io` `events` in each of those namespaces, reviewed one namespace at a
   time so that one denial does not fail the whole feed. The owner's own namespace must be
   allowed. A denial elsewhere sets `partial`.
5. One list per (event namespace, regarded kind), with the field selector `regarding.kind=<Kind>`.
   The results are filtered in memory to the reached set by group, kind, namespace and name, as
   `regards` does today. They are folded per portal:D9:R3 and merged newest first.

No new kind, verb or namespace pattern is read beyond what today's per-object feed reads. The
in-cluster omission (portal:D8:R5) applies per item, exactly as for a one-object feed. The
`events:` stream topic keeps carrying the owner's own feed. The page re-reads the merged feed on
each refresh, and the stream's periodic refresh covers events about children.

`internal/readmodel` gains:

```go
// EventsAbout returns the folded events about each of refs, merged newest
// first: one list per event namespace and regarded kind. grants maps an
// event namespace to the caller's list grant there; refs whose namespace
// has no grant are skipped and reported in skipped.
func (m *Model) EventsAbout(ctx context.Context, who authz.Identity,
	grants map[string]authz.Grant, refs []ObjectRef) (evs []Event, skipped int, err error)
```

The lists run concurrently, at most four at a time, under the existing `readTimeout`.

### 2. Applied, Health and Provider as state blocks (`instance-01`, `instance-02`, `instance-03`, `instance-04`, `instance-06`, `instance-07`, `instance-40`, `instance-48`, `instance-39`, `provider-01`, `provider-02`, `provider-03`, `provider-22`)

`owner.go` builds one `stateBlock` per card. Hues (the icon follows the hue) and words:

| Card | State | Hue | Word |
| --- | --- | --- | --- |
| Applied | Applied, with `Reconciling=True` | progressing | Reconciling (`instance-40`) |
| Applied | Applied | applied | Applied |
| Applied | Failed, Stalled | degraded | Failed, Failed, retrying, Stalled |
| Applied | Reconciling | progressing | Reconciling |
| Applied | ManagedExternally, Suspended | neutral | Managed externally, Suspended |
| Applied | Unknown, or a value the UI does not know | unknown | Unknown |
| Health | Healthy | healthy | Healthy |
| Health | Degraded | degraded | Degraded |
| Health | Missing | missing | Missing |
| Health | Progressing | progressing | Progressing |
| Health | Unknown | unknown | Unknown |
| Provider | Accepted and active | healthy | Active |
| Provider | Accepted and not active, or Pending | progressing | Accepted, Pending |
| Provider | Refused | degraded | Refused |
| Provider | Removal blocked | unknown | Removal blocked |
| Provider | Claim not readable | `Problem` set to the claim's access | none (the block renders locked, per `align-shell-and-tokens`) |

The Applied words are today's, by supervisor ruling: the canvas's "Ready / Not ready" merges
Failed, Stalled and Reconciling. The Reconciling word applies only while the applied state is
Applied. A package keeps `Reconciling=True` beside `Ready=False` across retries (F1 `pkg/podinfo`,
`modulepackages.yaml:23-35`), so a Failed package stays Failed and keeps the Reconciling mark as a
secondary line.

**Applied**:

- Eyebrow "Applied · the controller".
- When: "since <Reconcile.Since>".
- Summary, composed in `appliedSummary` from recorded fields only:

  | Recorded facts | Summary |
  | --- | --- |
  | Applied, instance | "<InventoryCount> objects applied" |
  | Applied, package with a `SourceArtifact` | "Fetched <short revision or digest>, <InventoryCount> objects applied" |
  | Failed or Stalled | "Failed on the last <N> attempts", where N is the trailing run of `History` entries with outcome Failed, plus ", retrying every <interval>" when `Retrying` and a package interval are set |
  | Reconciling | the Reconciling condition's reason |
  | Managed externally | "Applied by the opm CLI; the controller does not manage it" |
  | No history, no inventory | "Nothing applied yet" |

- The reason's meaning and, in local mode, the controller's message stay visible on a smaller
  line beneath the summary (portal:D8:R5/R6).
- Reasons: Warning counts of the owner's own events, count first, under the caption "Warning
  events, last hour" (portal:D9:R1/R2). They are filtered from the merged feed by `Regarding` ==
  owner, so the counts mean what they meant before. With no counts: "No reconcile errors".
- History strip (`instance-06`), inside the card's section:
  - A "History" label.
  - 18 px dots marked ✓, ✕ or … by outcome, each focusable with a tooltip.
  - "last N attempts".
  - A `<details>` whose `<summary>` is the right-aligned pill "Show history ▾ / Hide history ▴",
    the label swapped with CSS on `details[open]`.
  - "At most ten attempts are kept; reconciles that change nothing are not recorded" inside the
    opened body.
- Opened rows (`instance-07`): `#seq`, an outcome badge (Applied, Failed or Unknown), the
  message, and the age, plus the existing digest line.

**Health**:

- Eyebrow "Health · everything it runs".
- When: "checked live" (words, through `WhenText` with `When` nil) when `Health.Live`; otherwise
  "not live, evaluated" and the `EvaluatedAt` stamp (portal:D3:R5).
- A partial health adds "Partial: some objects could not be read and are left out." under the
  summary (portal:D3:R4). The state word stays the bare state.
- Summary from `Health.Counts` (`instance-02`). With total = Healthy + Progressing + Degraded +
  Missing + Unknown:

  | Counts | Summary |
  | --- | --- |
  | Degraded + Missing > 0 | "N of M resources unhealthy" |
  | Progressing > 0 | "N of M resources rolling out" |
  | total = 0 | "Nothing applied yet, so nothing to check" |
  | otherwise | "All M resources healthy" |

- Reasons: today's counts, count first, in the tone's ink (`instance-48`), still linking to
  `tab=resources&reason=` (`instance-05` decided out). With none: "No unhealthy resources".
- `healthBadge` drops the "not live" note when the counts total zero (`instance-39`): nothing was
  polled, so there is nothing to warn about, and portal:D3:R5 holds.

**Provider**: shown when `providerOf` is not empty.

- Eyebrow "Provider · its registration".
- The first claim is the headline. Other claims are compact rows (badge and name) below it.
- On an owner holding a claim, the page reads `/platform` on every tab, not only the Provider
  tab, and follows the `platform` topic. One fetch, already authorized as the Provider tab's.
- When (`provider-01`): from the registration's own conditions in the Platform document. "active
  since <Active.lastTransitionTime>" when active. Otherwise "accepted", "refused", "pending" or
  "removal blocked" followed by <Ready.lastTransitionTime>. F1 `transformerregistrations.yaml:35-46`
  carries both conditions with times. With the Platform unreadable there is no when.
- Summary (`provider-02`):
  - Active: "Provides <contract short names> for the platform", from `Registration.Provides`.
  - Otherwise, in local mode: `Registration.ActiveMessage` when the claim is accepted and not
    active, else `Registration.Message`, verbatim (portal:D15:R2/R3).
  - Otherwise, in-cluster: the reason's meaning, or the reason alone.
  - The portal composes no verdict of its own.
- Reasons (`provider-03`, `provider-22`): a non-success reason renders as "1 <Reason>", linking
  to `?tab=provider`. Success shows "No registration problems". The card carries one "Open the
  Provider tab" link instead of linking each registration name.

### 3. Identity card and page frame (`instance-09`, `instance-10`, `instance-11`, `provider-04`, and `instance-12` from section X)

- **Subtitle** under the h1, in mono: "<module path> · <version>" for an instance; "from <Source
  Kind> <ns>/<name>" for a package, with the namespace resolved to the package's own when
  `sourceRef` names none (as `internal/graph/instance.go:89-94` does). The Module fact row goes.
- **Facts**: Namespace; Owner ("controller" or "the opm CLI"); Applier ("ServiceAccount <name>",
  or the controller's own). Owner and Applier each carry an info tip: a focusable span with an
  inline SVG and a `.tipbox`, using the canvas wording. A package adds Source, Path, Interval
  ("every 1 min", humanized from the Go duration; "not set" when absent), Prune ("on" or "off"
  from `prune`; "not set" when absent), Revision ("not recorded" when absent) and Depends on.
  Objects and Last applied stay.
- **Provider pill** in the kicker (`provider-04`): round, 11 px, 700, uppercase, body font. It
  reads "Provider", with the standing in a visually hidden span and the title, so colour is not
  the only carrier. It uses `providerBadge` as `align-platform-installed-catalog` leaves it. The
  kind word stays "Instance" or "Package" (portal:D17:R2).
- **Frame** (`instance-11`): a `page-owner` class on `<main>` uses the 1840 px column. The summary
  grid is `repeat(auto-fit, minmax(380px, 1fr))`. On Graph and Resources the tab panel and the
  details panel sit in a flex row (`999 1 900px` and `1 1 420px`) that wraps below 1320 px.
- **Conditions** (`instance-12`, supervisor ruling): the "Conditions and render contracts" fold
  moves below the tab panels, so the tabs sit right under the cards. The fold is the record
  (portal:D9:R1), so it stays on the page.

### 4. Tabs and Resources table (`provider-05`, `graph-12`, `instance-30`, `graph-10`, `instance-45`, `instance-39`)

- **Tab order**: Graph, Provider (when held), Resources, Events, Logs, YAML (`provider-05`).
- **Counts** (`graph-12`): `align-shell-and-tokens` already sets them through `tabLink.Count`
  (Resources: inventory objects plus runtime children; Events: the folded lines of the feed the
  tab opens with). This change keeps that rule as the feed changes:
  - The merged feed (`scope=all`) replaces today's owner-only fetch on every tab, so the Events
    count becomes the merged count. There is still one events fetch per render.
  - A `partial` merged feed gets no Events count, because the caller could not read the whole
    list. That is that change's rule.
  - The package source row is not an inventory object and is not counted.
- **Resources table** (`instance-30`, `graph-10`): one table. The columns combine the two
  boards:

  | Column | Content |
  | --- | --- |
  | Kind | `Ref.Kind` |
  | Name | `ns/name`, mono |
  | Component | the component's name |
  | Origin | "applied", "made by the cluster" or "source" |
  | Health | square badge |
  | Details | the health reason, or "N desired" for a ReplicaSet, with the message clipped to one line and in full in its `title` |

  - The table is a `<table class="table rows resources-table">` with a header row, so the
    phone-width card fallback of `table.rows` applies.
  - Children follow their object, indented.
  - Old ReplicaSets fold in a `<details>` row as today.
  - A Degraded or Missing row carries `row-degraded` and the tint token from
    `align-shell-and-tokens`.
  - The row's name is the link to `?tab=graph&focus=<node>` (and `expand` when folded), with a
    CSS stretched link so the whole row is a target and no script is needed.
  - The focused row, when `focus` is set, is marked `aria-current` and tinted.
  - YAML, Events and Logs leave the row; the panel carries them.
  - The free-text reason input goes. `reason` stays a URL parameter, set by the Health card's
    hand-off and shown as a removable chip.
- **Configuration groups**: `foldConfig` returns `[]configGroup`, one per group node in the graph
  document, each labelled by its node label, with count and worst health. Today that is one
  group; when `align-graph` adds per-kind groups they flow through with no template change.
- **Package source row** (`instance-45`): the first row on a package page comes from the graph
  document's `source` node: Kind and `ns/name` from its `Ref`, origin "source", and its health
  when the node carries one (`align-graph`). Without that health it reads "not read". Details
  show the revision when recorded. The row links to the source node's focus. It is present even
  when the inventory is empty.
- **Empty text** (`instance-39`): with no inventory, "Nothing applied yet. The graph fills in
  once an apply succeeds." for an instance. For a package with no source artifact: "Nothing
  applied yet. The package has never fetched its source." With a filter set, the text stays
  "No object or child reports this reason."

### 5. The details panel (`instance-27`, `instance-28`, `instance-29`, `instance-47`)

- **At rest** (`instance-27`):
  - With no `focus`, the Graph and Resources tabs render the panel of a resting node, with no
    spotlight and no `aria-current`, so the panel is never an empty placeholder. Clear selection
    returns the panel to that node.
  - `align-graph` owns the choice of resting node: the owner's node by default, or the
    registration node when a claim needs attention (`graph-23`, `provider-21`). It also owns the
    reset in `resetDetail`.
  - When this change lands first, it builds the minimal form so that `instance-27` is closed
    either way:
    - `defaultPanelNode(v *ownerView, g *v1.Graph) string` returns `g.Root`.
    - `#detail` carries `data-root-panel="<panel URL>"`.
    - `resetDetail` fetches that URL with `select: 'unset'`.
    - Without script, Clear selection is a link without `focus`, so the server renders the
      resting panel.
    `align-graph` then extends `defaultPanelNode` and rebases.
  - When `align-graph` lands first, this change only renders the resting panel on the Resources
    tab through that change's function, and adds its tests.
- **Body** (`instance-28`):
  - Kicker kind, mono name, and the badges.
  - The health message in a `.msg` box tinted by the node's health tone.
  - The facts.
  - The links as 40 px buttons: Logs dark and first for a Pod, Events and YAML outlined, Open
    and Expand outlined. All the links are kept, per the redesign.
- **Per-kind facts**, read from the object document the YAML view already serves
  (`<base>/object?group=&kind=&namespace=&name=`), only when the panel opens on a readable object
  of one of these kinds:

  | Kind | Facts and source fields |
  | --- | --- |
  | Deployment, StatefulSet | Replicas "R ready of D desired" (`status.readyReplicas`, `spec.replicas`), Image (each container's `image`), Strategy (`spec.strategy.type` or `spec.updateStrategy.type`) |
  | DaemonSet | Image, Strategy |
  | ReplicaSet | Replicas |
  | Pod | Node (`spec.nodeName`), Restarts (sum of `status.containerStatuses[].restartCount`), Started (`status.startTime`) |
  | Service | Type (`spec.type`), Ports ("port/protocol → targetPort") |

  - A failed object read leaves those facts out, and the panel says "Details from the object
    could not be read". It is never an error page.
  - No `env`, `args`, `command` or volume is shown: the panel shows what the YAML view shows,
    narrowed.
- **Component** (`instance-29`): "Rendered" as the state word, the canvas's sentence ("A
  Component is not a Kubernetes object: it is a part of the module, so it has no events or YAML of
  its own."), and a Renders fact listing the kinds and names of its outgoing edges' targets.
  There is no Catalog fact; the graph does not record one.
- **Root node** (`instance-47`): when the node's `Ref` is the page's owner, `links()` returns no
  Open link (it is the open page) and an Events link to `<base>?tab=events`. There is no YAML
  link (`instance-46` decided out). The facts gain Module and Version, or Source, Path, Interval
  and Revision, plus Owner and Applier, from the owner document through `panelContext`.

### 6. Events, Logs and YAML tabs (`instance-44`, `instance-32`, `instance-33`, `instance-34`, `instance-35`, `instance-36`)

- **Default scope** (`instance-44`):
  - The Events tab shows the merged feed. The empty `resource` value means "All resources".
  - `resource=<group/Kind/ns/name>` narrows the merged feed in the UI, with no second read,
    including to the owner itself: "This instance" or "This package" is an explicit option.
  - A `partial` feed adds the note "Some objects' events could not be read and are left out."
- **Filter bar** (`instance-32`):
  - Resource: an "Everything" optgroup holding "All resources (N)" and "This <kind> (n)", then
    one optgroup per kind with "<name> (n)".
  - Type: "Any type", "Warning (n)" and "Normal (n)".
  - Every count is taken over the merged feed.
  - The Reason text input goes. `reason` stays a URL parameter, set by the Applied card's
    hand-off and shown as a removable chip.
  - "Showing N of M" sits on the filter row, right-aligned, with "Clear filters".
  - The form carries `data-filters="owner-events"`, a key no view remembers (portal:D14). The
    existing select handler applies a change at once, and the `opm-js` class from
    `align-platform-installed-catalog` hides the Apply button.
- **Table** (`instance-33`): the shared `events-table` partial, with `ShowResource` set. Its input
  is `eventsTable{Problem, Rows []eventRow, ShowResource, Empty}`, as
  `align-platform-installed-catalog`'s design.md defines it. Whichever change lands first adds
  it.
  - Columns: Type (uppercase, red ink for Warning); Reason (mono); Resource (the kind without its
    group, in small caps, over the name); Message (the note, then a muted ×N and the reporting
    controller); and Age (compact, without "ago", with the absolute time in `title`).
  - `eventRow.ResourceHref` is `?tab=events&resource=<value>` (other parameters kept) when the
    regarded object is a reached object or the owner. It is titled "Show only this resource's
    events".
  - The details panel's Events view (`events.html`) moves to the same partial without the Resource
    column. `events-list` is removed when its last caller goes.
- **Notes and empty text** (`instance-34`):
  - The expiry note and the render-warnings note become one muted line: "Recent activity:
    Kubernetes keeps events for about an hour, and render warnings exist only as events, so older
    ones are gone." It keeps portal:D9:R2/R5.
  - With a filter set and no match: "No events match these filters." Otherwise: "No events in
    the last hour or so."
- **Logs** (`instance-35`):
  - A GET form (`data-filters="owner-logs"`, not remembered) holds one select, "Pod / container"
    in mono, grouped by Pod, with the option value `log=<logID>`. The existing select handler
    submits it; a Show button appears without script.
  - One `details.log` is rendered `open` for the selected container, or the first container of
    the first Pod. The existing script follows an open pane on load, so the pane streams at once.
  - The Previous instance checkbox stays.
  - `.log-pane` uses a `--logbg` token that is dark in both themes.
  - When the selected Pod's health reason is a waiting reason (`ImagePullBackOff`,
    `ErrImagePull`, `ContainerCreating`, `CrashLoopBackOff`, `CreateContainerConfigError`,
    `InvalidImageName`, or a Pod `Pending`), the pane is preceded by "The container has not
    started, so there are no logs yet." and "See Events for why." This links to
    `?tab=events&resource=<the Pod>`. The pane is still offered, since a crash loop has
    previous-instance logs.
  - Pod row and panel Logs links become `?tab=logs&log=<id>`.
- **YAML** (`instance-36`):
  - A GET form (`data-filters="owner-yaml"`, not remembered) holds one "Resource" select grouped
    by kind. Secrets are listed as disabled
    options reading "<name> (Secret data is never read)".
  - The line "Secrets and values are never shown." sits beside it.
  - With no `object` parameter the tab preselects the focused node's object (`focus` maps to the
    node's `Ref`), else the first readable inventory object, and shows its YAML on arrival.
  - Tab links carry `focus` across tabs (`linkWith`), so YAML follows the graph selection.
  - The panel is one wide region.

### 7. Provider tab (`provider-06`, `provider-07`, `provider-08`, `provider-09`, `provider-11`, `provider-20`)

- **Intro** (`provider-06`): "This <instance|package> is also a provider: it registers a catalog
  with the platform, and modules that use the contracts below render through it." A refused
  claim adds "The controller refused the claim, so it provides nothing." Both come from the
  claim's own verdict.
- **Facts grid** (`provider-07`): `.facts-grid`, `repeat(auto-fit, minmax(260px, 1fr))`, label
  above value.

  | Fact | Content |
  | --- | --- |
  | Registration | "TransformerRegistration <name>", mono |
  | Claims catalog | link and version |
  | Registry | today's sentence |
  | Contracts | "N provided, M in use", where M counts contracts whose Used by total > 0; or "none: the claim is refused" |
  | providerRef | as today (portal:D15) |
  | Controller says | local mode only |

- **Standing once** (`provider-20`): the h3 stays as the per-claim heading. One badge row shows
  the accepted and active pills (portal:D4:R4), plus the reason only when it is not a success
  reason. The claim badge leaves the tab.
- **"Registration conditions"** (`provider-08`): an h3 heading. Status is a square badge toned by
  `Condition.Tone`. Reason is mono. The Meaning column stays (live-only).
- **"What it provides"** (`provider-09`): a table with columns Contract and Used by, styled as a
  table header.
  - The contract is a round amber pill (`.contract-pill`) with `contractShort` text and the full
    path in `title`.
- **Used by** (`provider-11`):
  - Up to three mono links, then "and N more".
  - A right-aligned pill button: "Show in Installed →" when Total ≤ 3, else "All N in Installed
    →", titled "Open Installed filtered to everything that uses <contract>".
  - Total 0 on a complete list: "No installed module uses it yet", and no button.
  - The permission wording stays when the list is incomplete or locked.

### Scale and refresh

Every owner page render now reads the merged feed. For cert-manager (42 entries; F1) that is one
list per (namespace, kind) pair rather than one per object. The pages already re-render on
refresh from one fetch of the page. Section 5 measures the read on the e2e cluster's
cert-manager. If it adds more than about 200 ms at the 95th percentile, the Events count shows
only on the Events tab and the other tabs read the owner's own feed, as today. design.md records
the measurement.

## Research & Decisions

### Where the merged feed is built

**Context**: The owner chose "Merge all" (portal:D9:R6). The Platform page already merges feeds
in `internal/ui` (archived design, "Merged Platform events").
**Explored**: `internal/api/handlers.go:406-445` (`ownerEventsDoc`: owner get, object get,
events list, then `ownerReaches` walks the inventory for every named object);
`internal/readmodel/events.go:58-90` (one field-selected list per object);
`testdata/clusters/f1/events.yaml` (63 events about 8 kinds).
**Options considered**:
1. Merge in `internal/ui` from per-object reads. This needs no API change. For cert-manager it
   means about 50 in-process requests per render, each with its own reviews, an inventory walk
   and a cluster list. That is the per-object cost portal:D3:R9 rules out for the page.
2. `scope=all` in the API, with one unselected list per event namespace. This takes few calls,
   but a busy `default` namespace returns every event in it.
3. `scope=all` in the API, with one list per (event namespace, regarded kind) using a
   `regarding.kind` field selector, filtered in memory. Calls are bounded by kinds, not objects,
   and each list is narrow.
**Decision**: Option 3.
**Rationale**: It keeps the page cost independent of inventory size, keeps every list reviewed
for the caller, and gives any API client the same merged feed (Principle III), so the UI still
reads only what the API serves.

### Per-kind panel facts

**Context**: The canvas panel shows Replicas, Image, Strategy, Node, Restarts, Started, Type,
Ports and Endpoints (`instance-28`).
**Options considered**:
1. Add the fields to `GraphNode`. That puts the graph builder in `internal/graph` (owned by
   `align-graph`) in charge of object specs, and widens every graph document.
2. Read the object document the YAML view already serves, only for the selected node. That is one
   authorized read per panel, with the fields extracted in `internal/ui`.
3. Leave the facts to the YAML view. The panel then stays thinner than the canvas.
**Decision**: Option 2, without Endpoints.
**Rationale**: No new kind and no API change. The facts are a subset of the YAML the user can
already open (portal:D8:R4 applies the same way). Endpoints would need EndpointSlices, a new kind.

### Logs: a select or the folds

**Context**: The canvas shows one always-visible pane picked by a select; live shows one closed
`<details>` per container (`instance-35`).
**Options considered**: open the first fold by default; a select and one open fold.
**Decision**: A select and one open fold.
**Rationale**: It matches the canvas and keeps the log script's contract (it follows open
`details.log` panes; `TestBrowserLogs` holds scroll, focus and tail). Without script the select
form still works.

### Provider card "when"

**Context**: `ProviderClaim` has no time (`api/v1alpha1/types.go:210-225`).
**Options considered**: add a `since` to `ProviderClaim` (an API change in the read model's
holder join); read `/platform` on owner pages that hold a claim and take the registration's
condition times.
**Decision**: Read `/platform`.
**Rationale**: The Provider tab already reads it. One more fetch on pages that hold a claim is
cheaper than a wire change, and a Platform the caller may not read degrades to "no time".

## Risks / Trade-offs

- [Merged feed slower than one list] → Concurrent lists, at most four. The section 1
  measurement can fall back to counting only on the Events tab.
- [`partial` hides why] → The note says objects were left out. The per-object option still
  returns the precise `403` for one object.
- [Shared files with `align-graph` and `align-platform-installed-catalog`] → The proposal lists
  each overlap. Each change rebases on the other's merge and re-runs `task check` and
  `task test:browser`.
- [The Applied summary looks like a verdict] → It uses only recorded counts and the condition
  reason. The controller's own message stays visible beneath it in local mode.
- [Panel object read fails or is slow] → The facts are optional: the panel renders from the graph
  document first and says when object details are missing.

## Migration Plan

Additive API; nothing to migrate. Rollback is reverting the PR. Clients that never send `scope`
see no change. The PR updates the API goldens (local and in-cluster) and the UI goldens, and
reviews them.

## Visual checks

Filled in by sections 2 to 5: screenshots of each owner tab over the F1 capture
(`TestDevServe`) in light, dark and at 360 px, compared with the canvas boards. Record each
remaining difference here and say why it stays.

## Gap map (section C of gaps.md)

| Gap | Decision | Tasks |
| --- | --- | --- |
| instance-01 | 2 | 2.1, 2.3, 2.4 |
| instance-02 | 2 | 2.1, 2.3 |
| instance-03 | 2 | 2.1, 2.3 |
| instance-04 | 2 | 2.1, 2.3 |
| instance-06 | 2 | 2.3, 2.4 |
| instance-07 | 2 | 2.3 |
| instance-09 | 3 | 2.3 |
| instance-10 | 3, 1 (`prune`) | 1.2, 2.3, 2.4 |
| instance-11 | 3 | 2.3, 2.4, 3.2 |
| instance-27 | 5 | 3.3, 3.6 |
| instance-28 | 5 | 3.4 |
| instance-29 | 5 | 3.4 |
| instance-30 | 4 | 3.2 |
| instance-32 | 6 | 4.1 |
| instance-33 | 6 | 4.2 |
| instance-34 | 6 | 4.2 |
| instance-35 | 6 | 4.3 |
| instance-36 | 6 | 4.4 |
| instance-39 | 2, 4 | 2.2, 3.2 |
| instance-40 | 2 | 2.1 |
| instance-44 | 1, 6 | 1.1, 1.3, 4.1, 5.4 |
| instance-45 | 4 | 3.2 |
| instance-47 | 5 | 3.4 |
| instance-48 | 2 | 2.1, 2.3 |
| graph-10 | 4 | 3.2 |
| graph-12 | 4 | 3.1 |
| provider-01 | 2 | 2.1, 2.3 |
| provider-02 | 2 | 2.1 |
| provider-03 | 2 | 2.1 |
| provider-04 | 3 | 2.3, 2.4 |
| provider-05 | 4 | 3.1 |
| provider-06 | 7 | 5.1 |
| provider-07 | 7 | 5.1 |
| provider-08 | 7 | 5.1 |
| provider-09 | 7 | 5.1 |
| provider-11 | 7 | 5.1 |
| provider-20 | 7 | 5.1 |
| provider-22 | 2 | 2.1 |
| instance-12 (section X, supervisor ruling) | 3 | 2.3 |
