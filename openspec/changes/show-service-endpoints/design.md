## Context

See proposal.md for why. The current state, read from the base `origin/main` (888edd4):

- **Where the panel gets a Service's facts.** `objectFacts` (`internal/ui/panelfacts.go:43`)
  reads the Service's object document through the read API and `factsOf` (`panelfacts.go:59`)
  lists its type and ports (`panelfacts.go:79`). The panel receives a `*v1.GraphNode`
  (`panelfacts.go:43`), and the UI reads only the read API (`AGENTS.md`, "internal/ui reads only
  through the read API"; a test fails if it imports `internal/readmodel`). So the count must reach
  the UI on a wire type, and the graph node is the one the panel already holds.
- **How the runtime children are read.** `childResources` is ReplicaSets, Pods and Jobs
  (`internal/readmodel/children.go:23`). `childrenIn` (`children.go:106`) asks the caller for
  `list` on all three kinds and returns nothing for the namespace if any one is denied. They are
  listed with the label `module-instance.opmodel.dev/name` (`children.go:144`) and attached to an
  inventory object by `runtimeChildren` (`internal/readmodel/evaluate.go:92`), which follows
  controller owner references with `reachInventory` (`evaluate.go:153`). A child whose chain
  reaches no inventory object is dropped. The openspec requirement "Instance views carry the
  runtime children below each inventory object" states the same rule.
- **How a caller's access is decided.** `callerAccess` (`internal/readmodel/access.go:30`) asks the
  authorizer for the caller; `readerMay` (`access.go:59`) asks for the portal's own reader. Every
  read is preceded by both (`children.go:152` to `159`). `internal/authz/attributes.go` accepts any
  read verb on any resource except Secrets, so `endpointslices` needs no authorizer change.
- **The role.** `deploy/clusterrole.yaml` lists every rule by hand; `checkRule` and `checkResource`
  (`deploy/manifest_test.go:147`) refuse a write verb, a wildcard, a Secret and a subresource but
  `pods/log`. The test does not name the rules the role must hold, so a new rule needs its own
  assertion.
- **The wire.** `v1.GraphNode` (`api/v1alpha1/types.go`) carries `Replicas` and `ChildrenUnread` for
  workloads; `internal/api/convert.go:388` copies them from `internal/graph`'s `Node`
  (`graph.go:303`), which `internal/graph/instance.go:352` fills from `readmodel.InventoryObject`.
- **Evidence the plan rests on.** `agent-swarm/opm-portal-v1/design/T2.2.md` G14,
  `design/T2.4.md` slice S16, section 8 "G14, EndpointSlices", SR5 and SR10. Claims about general
  Kubernetes behaviour below carry the mark *(K8s, to check)*: they come from Kubernetes
  documentation and source known to the author, not from a file here, and task 1.2 checks each on a
  live cluster.

## Goals / Non-Goals

**Goals:**

- Show "Endpoints N ready" in the Service panel, counted from the cluster's EndpointSlices, for a
  caller who passes a separate review, and nothing in its place for a caller who does not.
- Keep `portal:D7` true without an amendment, if the spike confirms the assumption below.
- Cross into the wire and the page as one integer. No name, address, port, node or zone.

**Non-Goals:**

- A watch on EndpointSlices, so the count does not follow a Pod's readiness live (see "Reading").
- The count on the hover card, the Resources tab or the Instance document.
- A second health signal: the count changes no Applied or Health badge and no rollup (portal:D3).
- Showing the endpoints themselves, or per-address-type counts.

## Research & Decisions

### D7: slices as runtime children

**Context**: T2.4 SR5 asks to read the slices "through the runtime-children path or amend
`portal:D7`". D7 says the portal serves "only objects reachable from an OPM inventory" and that a
request for an object no inventory reaches gets the same `forbidden` document as a denied read
(R4). An amendment goes to the owner before implement; a reading that needs none does not.
**Explored**: `children.go`, `evaluate.go` (above); the openspec requirement "Instance views carry
the runtime children below each inventory object"; D11, which lists the child kinds the role grants.
**Options considered**:

1. **A fourth entry in `childResources`.** Pros: the literal "children path". Cons: `childrenIn`
   denies the whole namespace's children when one kind is denied, so a caller without
   `endpointslices` would lose Pods, ReplicaSets and Jobs, a regression of every existing view; the
   children are attached to the wire as `RuntimeChild` with a `ref` that carries the slice name,
   which D8 and OD12 forbid; and the Pod watches (`HoldChildren`) would gain a fourth informer and
   a `watch` verb (SR10).
2. **A separate read keyed on the inventory Service, with the children rule for reach.** The read
   starts only from an inventory Service the caller may already read. It lists the slices of that
   Service in its namespace by the label `kubernetes.io/service-name` and counts only a slice whose
   controller owner reference reaches that Service by `reachInventory`, the rule that attaches a
   ReplicaSet to a Deployment. It has its own review and its own failure state, and it leaves
   `childrenIn` untouched. Pros: no regression, no name on the wire, no new verb but `list`. Cons: a
   second small read path in `internal/readmodel`.
3. **Amend D7** so that a Service's slices are served by the label alone. Needed only if the owner
   reference is not reliable (the assumption below). Cons: the portal then counts a slice that
   anyone with `create` on `endpointslices` labelled with the Service's name, with no tie to the
   Service, and the amendment goes to the owner first.

**Decision**: Option 2, on one assumption that section 1 verifies. **D7 needs no amendment.**
**Rationale**: D7 binds what the portal *serves* and to whom. Under option 2 the portal serves no
slice: there is no request that names one (`/object` refuses it as it refuses any object no inventory
reaches, D7:R4, and a test says so), and the one integer reaches the page only through an inventory
Service the caller may read and a slice namespace the caller may list (D7:R1 to R3 hold, D5 holds).
The count is derived from objects whose controller chain reaches the inventory, which is the
meaning of "reachable from an OPM inventory" that `runtimeChildren` already implements. D11 does
change: its decision text lists the child kinds the role grants. That edit records the owner's OD12
answer and adds D11:R7; it changes no policy and does not renumber (task 5.5). The supervisor
confirms that reading in "Questions" of the report.
**Assumption (checked in task 1.2, *K8s, to check*)**: the EndpointSlice controller creates every
slice of a Service with a selector with a controller owner reference to that Service, and with the
label `kubernetes.io/service-name`; it also creates one slice with no endpoints when the Service
selects no Pod. A slice a person made by hand for a Service with no selector has the label and no
owner reference, and is not counted. **If the spike shows the owner reference is absent, the change
stops at task 1.4**: the choice becomes option 3, `portal:D7` needs an amendment, and the owner
decides it before any code. Reversibility: cheap before section 2, moderate after.

### The review

**Context**: SR5 and the threat statement: a caller who may read a Service must not see readiness
through the Service's own review.
**Options considered**:

1. **Reuse the review for the Service** (`get` on `services`). Rejected: it is the threat.
2. **`list` on `endpointslices` in the Service's namespace, no name** (`callerAccess` with
   `name == ""`). The read is a label-selected list, and the review asks for exactly that verb
   and scope (D5:R7). A caller who holds `get` on named slices only cannot list them and sees
   nothing; that is the conservative answer.
3. **`get` per slice.** Needs the slice names first, which is the read the review guards.

**Decision**: Option 2. The reviewed attributes are verb `list`, group `discovery.k8s.io`, version
`v1`, resource `endpointslices`, the Service's namespace, no name. In local mode this is a
`SelfSubjectAccessReview`; in-cluster, a `SubjectAccessReview` for the signed-in user (D6:R9), by
the same `callerAccess`. A review that fails, times out or is denied serves no count (fail closed,
AGENTS.md "Security Rules").
**Not covered by the eng skills read for this proposal**: asking the API server for a decision from
backend code (SubjectAccessReview and its variants) is listed as not covered by the
`eng-backend-security` guide `sign-in/kubernetes.md`. The decision above follows the portal's
existing, reviewed mechanism (`internal/authz`, `portal:D5/D6`) and adds no mechanism of its own.

### Reading

**Context**: the count must be per Service, cheap, per caller at serve time, and hold no slice.
**Options considered**:

1. **A watch (informer) per namespace.** Live, but needs `watch` (a second SR10 widening, and a
   fourth held informer in `HoldChildren`). Not taken now; recorded as a follow-up the owner may
   ask for.
2. **One list per Service per evaluation, no cache.** Simple; a page load makes several
   evaluations, so it multiplies API-server calls by the number of Services.
3. **One label-selected list per Service, cached for `ChildrenTTL` (10 s,
   `internal/readmodel/model.go:41`).** Matches how the runtime children are read when nobody holds
   interest (`children.go:144`), and adds no setting.

**Decision**: Option 3. The cache key is namespace and Service name, and it holds the integer and
whether any slice was found, never a slice (D8). The caller's review and the reader's review run
before each serve, never when the cache is filled, so one caller's allow never serves another
(read-authorization, "Decisions are cached briefly per identity and request"). Entries leave the cache in the same sweep that drops expired child lists
(`internal/readmodel/tier2.go`). The cost is that the count can lag a Pod's readiness by up to ten
seconds plus the control plane's own delay *(K8s, to check)*, and nothing triggers a refresh when
only a slice changes. The panel shows the count the page read; a reload reads again after the TTL.
The spike (task 1.2) measures the real delay on the fixture cluster and writes it here.
Reversibility: cheap (a watch replaces the list behind the same function).

### What is counted

**Decision**: the number of endpoints with `conditions.ready` true or unset, summed over the slices
of one address type, and the largest sum over the address types of that Service. *(K8s, to check)*:
the API documents an unset `ready` as "unknown, interpret as ready"; a terminating endpoint has
`ready` false; a dual-stack Service has an IPv4 and an IPv6 slice that list the same Pods, so
summing the types would double the number. The count is of endpoints, not of addresses within an
endpoint. A Service with no slice found carries no count; a Service with slices and no ready
endpoint carries `0`.
**Rationale**: "N ready" must read as the number of Pods that take traffic, the thing a reader
compares with the Replicas fact next to it. The unset-is-ready rule is the documented one, not a
guess; if task 1.2 finds the API text says otherwise, this paragraph changes before code.
**Alternatives**: count `serving` (differs from `ready` while terminating; not what a reader asks);
derive from the selector and the Pods (OD12 option 2, an invention, Principle IV); show per-slice
numbers (a topology disclosure, SR5).

### Absent, not locked

**Context**: three states give no number: the caller lacks the review, the portal's reader lacks
the verb (an install that predates the rule), or the cluster records no slice. Principle IV asks
that a degraded read is not shown as empty; the brief asks that a caller without the review sees
nothing that implies a fact.
**Options considered**:

1. **A distinct mark for each state** ("Endpoints: locked", "not readable"). For the caller without
   the review, "locked" tells that slices exist in that namespace; the brief rules it out.
2. **No line in all three states.** The panel makes no claim: neither a zero nor a lock. The
   existing facts follow the same shape (`appendFact` drops an empty value, `panelfacts.go:86`).
3. **A line only for the portal-side failure.** Splits the three states, so a reader can infer the
   caller's state from the other two.

**Decision**: Option 2: the wire field is absent and the panel shows no Endpoints line in all
three states. Neither state is rendered as `0 ready`. The cost is that a reader cannot tell "not
allowed" from "not recorded"; the docs page says what a missing line can mean. Reversibility:
cheap.

## Decisions

### The ClusterRole rule

One rule, placed beside the runtime children, with a comment in the existing style (no `portal:`
citation, `AGENTS.md` on `deploy/`):

```yaml
  # The ready count of a Service's endpoints; the portal lists the slices and keeps one number.
  - apiGroups: ["discovery.k8s.io"]
    resources: [endpointslices]
    verbs: [list]
```

`list` alone, like `events` (`deploy/clusterrole.yaml:22`): the read is a list, so no `get` and no
`watch`. `checkRole` already refuses any other verb, a wildcard and a Secret on it. The new test
`TestRoleGrantsEndpointSlicesListOnly` finds the rule by group and resource and fails unless its
verbs are exactly `list`, so a later widening is a reviewed test change. `TestCheckRoleRefuses`
gains rows for a `create` verb on `endpointslices` and for a wildcard group beside it.
D11's role list and the openspec requirement "The role only reads, and never Secrets" name the
rule; the README, which says how to extend the role, names it too.

### The read, in Go

```go
// in internal/readmodel/endpoints.go
var endpointSlices = schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"}

const serviceNameLabel = "kubernetes.io/service-name"

// endpointsOf returns, per entry, the ready endpoints of an inventory Service for the caller, or
// nil: the entry is not a readable Service, the caller or the reader may not list slices there, the
// read failed, or no slice reached the Service.
func (e *evaluator) endpointsOf(ctx context.Context, entries []health.Entry) []*int64

// readyEndpoints lists svc's slices (cached for ChildrenTTL) and counts them.
func (m *Model) readyEndpoints(ctx context.Context, svc *unstructured.Unstructured) (ready int64, found bool)

// countReady keeps a slice only if its controller owner reference reaches svc by reachInventory,
// groups the kept slices by addressType, sums endpoints whose conditions.ready is not false, and
// returns the largest sum. found is false when no slice was kept.
func countReady(slices []unstructured.Unstructured, svc types.UID) (ready int64, found bool)
```

Order inside `endpointsOf` for one Service entry: skip unless `Access == AccessOK`, group `""`,
kind `Service`, object non-nil; `callerAccess(list, endpointSlices, ns, "")` must be `AccessOK`; then
`readerMay(list, endpointSlices, ns, "")`; then the cache; then a list with
`LabelSelector: serviceNameLabel + "=" + name` under `readTimeout`. Any error, timeout or refusal
yields nil and logs the namespace, the Service name and the reason, no slice content
(`log/slog`, AGENTS.md "Error Handling And Logging"). A slice is counted and dropped, never
stored. The result is stored in
`InventoryObject.EndpointsReady *int64` for an entry whose access is OK (`componentsOf`,
`evaluate.go:251`), and `internal/graph` copies it to `Node.EndpointsReady`
(`instance.go:352`).

### The wire

```json
{ "id": "obj:_/Service/default/podinfo-podinfo", "kind": "object", "label": "podinfo-podinfo",
  "access": "ok", "endpointsReady": 2 }
```

`GraphNode.EndpointsReady *int64 \`json:"endpointsReady,omitempty"\``; `openapi/v1alpha1.yaml`
`GraphNode` gains `endpointsReady: {type: integer, format: int64, minimum: 0}` with a description
that says "the ready endpoints the cluster records for a Service; absent when the caller may not
list its EndpointSlices, the portal could not, or none are recorded". It is a pointer so that `0`
differs from absent. No new path, no new error: a node the caller may not read still carries
`access` only, and the `InventoryObject` of the Instance document is not changed.

### The panel

`factsOf` is a function of the object document, so the count joins it from the node:
`objectFacts` (`panelfacts.go:43`) appends `fact{Label: "Endpoints", Value: "N ready"}` after the
Service's Ports when `n.EndpointsReady != nil`. Value format: `fmt.Sprintf("%d ready", n)`. Plain
text through `html/template`, not monospace, no tone, no icon: it states a count and judges
nothing, in the words of the Replicas fact ("2 ready of 2 desired"). The `0 ready` case reads the
same way. The fixed list of facts grows by this one entry; nothing else changes. States of the
panel, all existing: loading is the existing `aria-busy` and "Loading..." line; the object document
unreadable keeps today's note and drops the fact with the other facts; a locked, forbidden or
not-readable node has no facts and no request (the node carries no count). Empty (no count) and
permission-denied (no count) are the same: no line (see "Absent, not locked").

At 360 px the facts grid is one column (archived change `align-owner-pages`, task 6.6); one short line adds no
width, and `TestBrowserPhone` holds it. The text is not colour-coded, so WCAG 2.2 AA contrast and
non-colour cues need nothing new. The label and value stand in the same `dl` as the other facts, so
a screen reader reads "Endpoints, 2 ready" in the order of the list.

### Tests, and the new authorization case

The case that must be able to fail: over F1 plus the recorded slice fixture, the caller `alice`
with `AllowAll` reads `endpointsReady: 2` on podinfo's Service node; the same read with
`DenyResources("endpointslices")` (the Service, Pods and the rest still allowed) carries no
`endpointsReady`, and the panel shows no Endpoints line. A test that reused the Service's review
would pass the first half and fail the second. Beside it:

- **No read for a denied caller.** `readmodeltest.Actions` on the fake dynamic client shows no
  `list` of `endpointslices` when the caller's review is denied, and `Reviews.Asked()` shows a
  review with verb `list` and resource `endpointslices` in the Service's namespace when it is allowed.
- **Per caller at serve time.** Caller A (allowed) reads and fills the cache; caller B (denied)
  reads next and gets no count; the cache is warm for A.
- **Fail closed.** A review that errors, and a reader that may not list, give no count and no panic.
- **Numbers only.** The fixture slices carry sentinel strings in every address, hostname, node name,
  zone, target Pod name and label value; the instance, graph, object, events and stream documents
  and every page contain none of them.
- **Reach.** A slice with the label and no owner reference, a slice owned by another Service, and a
  slice owned by a Service outside the inventory add nothing.
- **Count rules.** Table test of `countReady`: unset ready, false, terminating, two slices, a
  dual-stack pair, only a placeholder slice (`0`), no slice (absent).
- **D7:R4.** `GET .../object` for an EndpointSlice ref is refused like any object no inventory
  reaches.
- **Other kinds unchanged.** The health rollup and the Applied badge of the Service are equal with
  and without slices.

The UI test reads the node from the API in process, as every panel test does. The browser check
extends `TestBrowserOwner` (the details panel of an owner page) to select podinfo's Service node
over F1 plus the slice fixture and read the Endpoints line, and `TestBrowserPhone` holds 360 px.
The live check, `task e2e:m1` against the fixture cluster, gains an assertion that podinfo's
Service node reports `endpointsReady` equal to its ready Pods. The implementing agent runs e2e
only if its brief allows; otherwise the nightly `E2E` workflow runs it and the report says so.

### The recorded slice

F1 holds no EndpointSlice and is not recaptured by this change (a recapture would move every
golden for one kind). Section 1 records a live slice of the fixture cluster's podinfo Service, with
`managedFields` and the last-applied annotation removed, as
`internal/readmodel/testdata/endpointslices-podinfo.yaml`, and a helper in `readmodeltest` adds it
to F1 the way `F1Broken` adds its samples. The capture check
(`task e2e:capture:check`) covers only `testdata/clusters/`; the file holds Pod IPs and a node name of
a throwaway kind cluster, nothing else, and carries no Secret. Recapturing F1 with slices is a
follow-up the supervisor may want.

## Risks / Trade-offs

- **The owner-reference assumption fails** (a controller version or a custom controller does not set
  it) → the spike catches it for the fixture's Kubernetes version; the design then changes to
  option 3 and stops for the owner. A Service whose slices are made by hand shows no line: a
  missing fact, not a wrong one.
- **The count lags** (up to the TTL plus the control plane's delay) → stated in the docs page as the
  cluster's record at the last read; a watch is the follow-up, with its own `watch` verb.
- **Writers can forge a count.** Anyone with `create` or `update` on `endpointslices` in the
  namespace can create a slice with the Service's label and owner reference (the UID is readable
  with `get` on the Service) → the proposal and the docs page call the number the cluster's record,
  not proof of health, and the panel claims nothing more (Principle IV).
- **`deploy/` mode widens what every token holder sees** → stated in the proposal, the README and the
  portal-security page; the rule is `list` only, one integer reaches the page.
- **A label-selected list is bounded by the Service, not by the namespace** → the call sets no page
  limit, so the answer is whole, and `readTimeout` bounds it. A Service has one slice for every
  hundred endpoints or so (*K8s, to check*). If a response ever carried a continue token, the code
  treats it as a failed read (no count) and never as an undercount.
- **A second read path** → one file, one function the evaluator calls, tested beside the children
  tests; `childrenIn` is not touched.
