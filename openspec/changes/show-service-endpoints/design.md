## Context

See proposal.md for why. The current state, read from the base `origin/main` (888edd4):

- **Where the panel gets a Service's facts.** `objectFacts` (`internal/ui/panelfacts.go:43`)
  reads the Service's object document through the read API and `factsOf` (`panelfacts.go:59`)
  lists its type and ports (`panelfacts.go:79`). The panel receives a `*v1.GraphNode`
  (`panelfacts.go:43`), and the UI reads only the read API (`AGENTS.md`, "internal/ui reads only
  through the read API"; a test fails if it imports `internal/readmodel`). So the count must reach
  the UI on a wire type, and the graph node is the one the panel already holds.
- **How the panel shows a read it could not make.** The Pods line reads "Could not be read; health
  is the object's own status." whenever the children read is not OK, a caller's own denial
  included (`internal/ui/templates/panel-body.html:25`, fed by `n.ChildrenUnread`,
  `internal/ui/panel.go:154`; set by `markChildrenUnread`, `internal/health/rollup.go:199`). The
  new fact follows that precedent.
- **How the runtime children are read.** `childResources` is ReplicaSets, Pods and Jobs
  (`internal/readmodel/children.go:23`). `childrenIn` (`children.go:106`) asks the caller for
  `list` on all three kinds and returns nothing for the namespace if any one is denied. They are
  listed with the label `module-instance.opmodel.dev/name` (`children.go:144`) and attached to an
  inventory object by `runtimeChildren` (`internal/readmodel/evaluate.go:92`), which follows
  controller owner references with `reachInventory` (`evaluate.go:153`). A child whose chain
  reaches no inventory object is dropped. The openspec requirement "Instance views carry the
  runtime children below each inventory object" states the same rule.
- **Who evaluates an inventory.** `inventoryHealth` (`evaluate.go:46`) is called by `ListInstances`
  (`internal/readmodel/instances.go:29`), the package list (`packages.go:28`), `Instance`
  (`instances.go:46`), `Package` (`packages.go:45`) and `ReachPod` (`reach.go:86`). `componentsOf`
  (`evaluate.go:251`) is a pure function of an evaluation. It is called by `Instance`
  (`instances.go:56`), `Package` (`packages.go:55`) and by `reachedVia` (`reach.go:124`) for
  `ReachPod`. `Instance` and `Package` are themselves called by more paths than the panel's graph:
  the graph handlers `instanceGraph` (`internal/api/handlers.go:192`) and `packageGraph` (`:261`,
  through `packageWithSource`); the Instance and Package documents and their stream topics
  (`handlers.go:168`, `source.go:26`, `producer.go:192,196`); the owner events endpoints and the
  named-object check (`ownerComponents`, `handlers.go:598-604`, called at `:467` and `:572`); and the
  provider lookup of the Platform graph (`lookupProvider`, `handlers.go:324`). Only the two graph
  handlers return the Service graph nodes the panel reads. The Instance and Package documents carry
  no field for the count (`convert.go:128-151`, `inventoryObject`), the stream topics render those
  documents, and the Platform graph keeps only an instance's own node and its registration
  (`internal/graph/platform.go:145,177-182`). A count computed in `inventoryHealth` would run for every
  list row and every log-topic admission; a count computed in `Instance` and `Package` would still
  run for the documents, the stream, the events endpoints and the provider lookup, none of which
  shows it.
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
  Kubernetes behaviour below carry the mark *(K8s, to check)*: they are not in a file of this
  repository. Where the pinned `k8s.io/api` v0.37.1 type documentation states a fact, the text cites
  its file and line in the module cache (`discovery/v1/types.go`); task 1.2 checks the rest on the
  live fixture cluster. There is no fetch grant, so no claim rests on a web page.

## Goals / Non-Goals

**Goals:**

- Show "Endpoints N ready" in the Service panel, counted from the cluster's EndpointSlices, for a
  caller who passes a separate review; show "Could not be read" for a read that was not made or
  failed; show nothing only when the cluster records no slice.
- Keep `portal:D7` true without an amendment, if the spike confirms the assumption below.
- Cross into the wire and the page as one integer (or one flag). No name, address, port, node or
  zone.

**Non-Goals:**

- A watch on EndpointSlices, so the count does not follow a Pod's readiness live (see "Reading").
- The count on the hover card, the Resources tab, the Instance and Package documents and their
  stream topics, the owner events endpoints, the provider lookup, the instance and package lists or
  `ReachPod`.
- A second health signal: the count changes no Applied or Health badge and no rollup (portal:D3).
- Showing the endpoints themselves, or per-address-type counts.

## Research & Decisions

### D7: slices owned by an inventory Service

**Context**: T2.4 SR5 asks to read the slices "through the runtime-children path or amend
`portal:D7`". D7 says the portal serves "only objects reachable from an OPM inventory" and that a
request for an object no inventory reaches gets the same `forbidden` document as a denied read
(R4). An amendment goes to the owner before implement; a reading that needs none does not. One
word needs care: D4, D10 and D11 use "runtime children" for objects owned by *workloads* (Pods,
ReplicaSets, Jobs). A Service is not a workload, so this change calls the slices "owned by an
inventory Service", reads them by the same ownership rule, and has D11 name them apart (task 5.5).
**Explored**: `children.go`, `evaluate.go` (above); the openspec requirement "Instance views carry
the runtime children below each inventory object"; D11, which lists the child kinds the role grants.
**Options considered**:

1. **A fourth entry in `childResources`.** Pros: the literal "children path". Cons: `childrenIn`
   denies the whole namespace's children when one kind is denied, so a caller without
   `endpointslices` would lose Pods, ReplicaSets and Jobs, a regression of every existing view; the
   children are attached to the wire as `RuntimeChild` with a `ref` that carries the slice name,
   which OD12 forbids and D11:R7 will forbid; and the Pod watches (`HoldChildren`) would gain a
   fourth informer and a `watch` verb (SR10).
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
meaning of "reachable from an OPM inventory" that `runtimeChildren` already implements. Both
proposal-gate lenses agree with this reading.
**D11 does change, and the change is to the role contract.** D11 is a `contract` decision about what
the portal's role grants. This change adds a kind to it (`list` on `endpointslices`) and, in
`deploy/` mode, widens what every launch-token holder sees. The owner's OD12 answer ("Add the read",
2026-10-12) is the source, so the edit is allowed in place (`docs/DESIGN.md:35-37`). The edit is not
a clarification: task 5.5 states the rule in the decision text, adds D11:R7 and moves the three
positions it turns down to "Alternatives considered" with their reasons (omit the read, OD12 option
3; derive the count from the selector, OD12 option 2, refused by Principle IV; a watch, refused to
avoid a second widening). It names the slices as owned by a Service, not among the workload
children. It reuses no number.
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
   and scope (D5:R7). A caller who holds `get` on named slices only cannot list them and gets no
   count; that is the conservative answer.
3. **`get` per slice.** Needs the slice names first, which is the read the review guards.

**Decision**: Option 2. The reviewed attributes are verb `list`, group `discovery.k8s.io`, version
`v1`, resource `endpointslices`, the Service's namespace, no name. In local mode this is a
`SelfSubjectAccessReview`; in-cluster, a `SubjectAccessReview` for the signed-in user (D6:R9), by
the same `callerAccess`. A review that fails, times out or is denied serves no count and no read
(fail closed, AGENTS.md "Security Rules"); the panel shows "Could not be read" (see "Could not be
read, not absent").
**Not covered by the eng skills read for this proposal**: asking the API server for a decision from
backend code (SubjectAccessReview and its variants) is listed as not covered by the
`eng-backend-security` guide `sign-in/kubernetes.md`. The decision above follows the portal's
existing, reviewed mechanism (`internal/authz`, `portal:D5/D6`) and adds no mechanism of its own.

### Reading

**Context**: the count must be per Service, cheap, per caller at serve time, and hold no slice. It
is needed only by the instance graph and the package graph documents (see "Who evaluates an inventory").
**Options considered**:

1. **A watch (informer) per namespace.** Live, but needs `watch` (a second SR10 widening, and a
   fourth held informer in `HoldChildren`). Refused by the supervisor to avoid that second widening;
   recorded as a follow-up the owner may ask for, and as an alternative in D11 (task 5.5).
2. **One list per Service per evaluation, no cache.** Simple; a page load makes several
   evaluations, so it multiplies API-server calls by the number of Services.
3. **One label-selected list per Service, cached for `ChildrenTTL` (10 s,
   `internal/readmodel/model.go:41`).** Matches how the runtime children are read when nobody holds
   interest (`children.go:144`), and adds no setting.

**Decision**: Option 3, run only on the two paths whose response carries the Service graph nodes the
panel reads: `GET .../instances/{namespace}/{name}/graph` (`instanceGraph`) and
`GET .../packages/{namespace}/{name}/graph` (`packageGraph`). They ask the model for the count
through two new methods, `Model.InstanceWithEndpoints` and `Model.PackageWithEndpoints`, which run
`Instance` and `Package` and then a new step (see "The read, in Go"). `Instance` and `Package`
themselves, `inventoryHealth`, `componentsOf`, `ListInstances`, the package list and `ReachPod` make
no review and no list for the count. So these paths do not read: the Instance and Package documents
and their stream topics, the owner events endpoints and their named-object check, the provider
lookup of the Platform graph, the lists and the log-topic admission. A list view that needs the
count later is a separate change. The cache key is namespace, Service name and uid (a Service made again under
the same name does not inherit the count), and it holds the
integer and whether any slice was found, never a slice. A failed read is not cached, so the next
graph read tries again; it is bounded by the one graph read that asks. The caller's review and the reader's review run before each serve, never when the cache is
filled, so one caller's allow never serves another (read-authorization, "Decisions are cached
briefly per identity and request"). Entries leave the cache in the same sweep that drops expired
child lists (`internal/readmodel/tier2.go`). The cost is that the count can lag a Pod's readiness by
up to ten seconds plus the control plane's own delay *(K8s, to check)*, and nothing triggers a
refresh when only a slice changes. The panel shows the count the page read; a reload reads again
after the TTL. The spike (task 1.2) measures the real delay on the fixture cluster and writes it
here. Reversibility: cheap (a watch replaces the list behind the same function).

### What is counted

**Decision**: an endpoint is ready when `conditions.ready` is not false. The count keeps only slices
of address type IPv4 and IPv6, counts each endpoint once within an address type, and is the largest
sum over the two types.

- **Ready.** The pinned `k8s.io/api` v0.37.1 documents an unset `ready` as "true"
  (`discovery/v1/types.go:146-153`: "A nil value should be interpreted as \"true\". In general, an
  endpoint should be marked ready if it is serving and not terminating, though this can be
  overridden in some cases, such as when the associated Service has set the
  publishNotReadyAddresses flag."). So a terminating endpoint is `ready` false in general, and on a
  Service with `publishNotReadyAddresses` set the controller can list a Pod that is not ready with
  `ready` true. The count follows the slice and does not read the Service's flag, so on such a Service
  (common for a headless StatefulSet Service) "N ready" can exceed the Pods that pass their
  readiness probe. Task 1.2 (c) observes it on the cluster if it can; the docs page says it (task
  5.6).
- **Address types.** The type documentation says the EndpointSlice controller "only generates, and
  kube-proxy only processes, slices of addressType IPv4 and IPv6" and that "no semantics are
  defined for the FQDN type" (`types.go:41-50`). A slice of type FQDN (or any other type) is always
  hand-made, so the count skips it. One FQDN slice holds up to 1000 endpoints (`types.go:56-57`) and
  would otherwise set the number alone.
- **Once within an address type.** The documentation says endpoints are "unique" within one slice
  (`types.go:56-57`) and that a Service's slices "must be joined to produce the full set"
  (`types.go:28-33`). It does not say that an endpoint cannot sit in two slices of the same type,
  for example while the controller moves it during an update *(K8s, to check, task 1.2 (c))*. The
  count therefore keys each endpoint within its address type: the `targetRef.uid` when the endpoint
  has a target reference, otherwise its first address (`types.go:92-102`: a controller-made endpoint
  has exactly one address). An endpoint with neither key counts on its own. An endpoint that appears
  in several slices counts once, and counts as ready only if no copy says false, so a conflict never
  claims readiness. The key is used in memory for the count and kept nowhere.
- **Dual stack.** A dual-stack Service has an IPv4 slice and an IPv6 slice. The two sums are
  separate key spaces, and the count takes the larger, so the same Pod seen on both addresses is
  not counted twice. That the two slices list the same Pods is *(K8s, to check, task 1.2 (d))*; the
  rule gives the right number whether they do or not.

The count is of endpoints, not of addresses within an endpoint. A Service with no kept slice
carries no count; a Service with kept slices and no ready endpoint carries `0`.
**Rationale**: "N ready" must read as the number of Pods that take traffic, the thing a reader
compares with the Replicas fact next to it. The unset-is-ready rule is the documented one, not a
guess; if task 1.2 finds the documentation says otherwise, this section changes before code.
**Alternatives**: count `serving` (differs from `ready` while terminating; not what a reader asks);
derive from the selector and the Pods (OD12 option 2, an invention, Principle IV); show per-slice
numbers (a topology disclosure, SR5); sum every address type (double counts a dual-stack Service).

### Could not be read, not absent

**Context**: four states give no number. The caller lacks the review; the portal's reader lacks the
verb (an install that predates the rule); the list fails, times out or returns a continue token; or
the cluster records no slice. Principle IV (`openspec/config.yaml:45`) says a degraded read (missing
permission, unreachable API) "is shown as such, never as empty". The panel already does this for
the Pods line (`panel-body.html:25`), a caller's own denial included. The first draft of this change
showed no line in the first three states, to honour the plan's line that a caller without the review
sees "nothing that implies" a fact. That line was the supervisor's, not the owner's, and the
proposal gate ruled that the principle wins.
**What a mark can disclose.** For a caller denied the review the portal makes no read, so a mark
shown to that caller is the same whether or not slices exist: it tells the caller's own access, which
`kubectl auth can-i` also tells. For the reader-denied and failed-list states the mark tells the
portal's state. Neither discloses a slice. (The first draft's claim that a "locked" mark would reveal
that slices exist was wrong for this reason.)
**Options considered**:

1. **A distinct mark for each state** ("locked", "not readable", "failed"). Splits states a reader
   cannot act on differently, and "locked" copies a word the panel uses for a node. More states to
   test.
2. **No line in every state.** The first draft. Breaks Principle IV: an operator whose role predates
   the rule, or whose list timed out, sees the same panel as a Service with no slices.
3. **A mark only for the portal-side states.** Leaves the denied caller with silence, the same
   break for that caller.
4. **One mark, "Could not be read", for the denied caller, the denied reader and the failed list;
   no line only when no slice is recorded.** Matches the Pods line in words and in the cases it
   covers.

**Decision**: Option 4. The node carries `endpointsUnread: true` (and no `endpointsReady`) in the
three states, as `childrenUnread` does for Pods; it carries `endpointsReady` when a count was made,
including `0`; it carries neither when no slice is recorded. The two fields are never both set; a
test says so. The panel shows `Endpoints` with the value `Could not be read`, and never `0 ready`
for a read it could not make. The cost is that a reader cannot tell a denied caller from a denied
reader from a failed list; the docs page says what the mark can mean (task 5.6). The server log
names the reason (namespace, Service name and reason, no slice content). Reversibility: cheap.

### Live checks

Run on 2026-10-12 on the throwaway fixture cluster `opm-portal-e2e-show-service-endpoints` (kind
v1.36.1 on podman, fixture set F1, `task e2e:up`), with `kubectl --kubeconfig
.e2e/clusters/opm-portal-e2e-show-service-endpoints/kubeconfig --context
kind-opm-portal-e2e-show-service-endpoints`. Extra scratch objects (namespace `t55`: Services
`nopods`, `nosel`, `roll`, `bad`, `badpub`, a Deployment `roll` of three Pods, a Pod `bad1` whose
readiness probe fails) were made on that cluster and are deleted with it. The Kubernetes
type facts come from `k8s.io/api` v0.37.1 in the module cache.

- **(a) Owner reference, label, managed-by label.** `kubectl -n default get endpointslice
  podinfo-podinfo-smkg5 -o jsonpath=...` gives one controller owner reference `kind: Service`,
  `controller: true`, `name: podinfo-podinfo`, `uid: f1b384bd-...`, equal to the Service's own
  `metadata.uid` (`kubectl -n default get svc podinfo-podinfo -o jsonpath='{.metadata.uid}'`). The
  slice carries `kubernetes.io/service-name: podinfo-podinfo` and
  `endpointslice.kubernetes.io/managed-by: endpointslice-controller.k8s.io` (`LabelManagedBy`,
  `well_known_labels.go:22`). The cert-manager Service's slice carries the same owner
  reference. **The assumption holds**, so D7 needs no amendment. One counter-example the count
  already handles: the slice `default/kubernetes`, which the API server makes for the Service
  `kubernetes`, has the service-name label and **no** owner reference, so a slice with the label and
  no owner is real and is not counted.
- **(b) A Service with a selector and no Pod.** Service `t55/nopods`: one slice, `nopods-m9kn2`,
  `addressType: IPv4`, `endpoints: null`, `ports: null`, controller owner reference to the Service.
  So a placeholder slice with no endpoints exists and the count is `0` with `found` true. A Service
  with no selector (`t55/nosel`) has no slice at all (0 slices with its label).
- **(c) The ready rule.** Documented: an unset `ready` is "true" (`discovery/v1/types.go:146-153`),
  the controller makes only IPv4 and IPv6 slices (`types.go:41-50`), endpoints are unique within one
  slice (`types.go:56-57`). Observed: a script polled `kubectl get pods,endpointslice -o json` in
  `t55` for 110 s while `kubectl rollout restart deploy/roll` ran three times and one Pod was
  deleted: 8763 observations of endpoints of the Service `roll`, **none with `ready` unset**; `ready`
  true for a Pod that passed its probe; `ready` false for a Pod whose probe had not passed yet (with
  `serving` false) and for a Pod that is terminating (10 terminating endpoints seen, every one
  `ready` false, `serving` true for part of the time). The failing Pod `t55/bad1` (probe path
  `/nope`) is `ready: false, serving: false` in the slice of Service `bad`. **The unset-is-ready rule
  of this design stands.** The **duplicate case**: during the three restarts the Service had exactly
  one slice at every poll and no endpoint (same `targetRef.uid`) appeared twice: not seen, and the
  documentation does not rule it out, so the once-per-type key stays. The **override** the same text
  allows: Service `t55/badpub` (`publishNotReadyAddresses: true`, same selector as `bad`) has the
  same Pod `bad1` as `ready: true, serving: false`. So on such a Service the count includes a Pod
  that is not ready; the design and the docs say so (task 5.6).
- **(d) Dual stack.** Documented: one address type for each slice (`types.go:41-47`); a Service's
  `ipFamilyPolicy` of `PreferDualStack` or `RequireDualStack` gives it an IPv4 and an IPv6 slice
  (`k8s.io/api` `core/v1/types.go:6234-6255`). Not run: the cluster is single stack (Service
  `ipFamilies: ["IPv4"]`, `ipFamilyPolicy: SingleStack`, node `podCIDRs: ["10.244.0.0/24"]`), so the
  dual-stack case is "documented, not run". The rule (larger of the two sums) is right whether the
  two slices list the same Pods or not.
- **(e) The `view` role.** `kubectl get clusterrole view -o yaml` lists `discovery.k8s.io`
  `endpointslices` with verbs `get`, `list`, `watch`. A namespace reader that holds `view` therefore
  holds the review the count asks (`list` on `endpointslices`).
- **(f) The delay.** In the same run, for 13 Pods that turned ready, the endpoint was `ready` in the
  same poll as the Pod's `Ready` condition: the delay is below the poll step (about 40 ms, from 8763
  endpoint observations in 110 s; not measured more finely). So the control plane's own delay is
  small on this cluster; the lag the panel shows is the ten-second cache of "Reading", not the
  controller. A busy cluster can differ; this is one kind cluster with one node.

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

// endpointsState is what the read gives for one Service.
type endpointsState struct {
	Ready  *int64 // set when a count was made, 0 included
	Unread bool   // set when the read was not made or failed; never with Ready
}

// withEndpoints fills EndpointsReady or EndpointsUnread on the Service objects of comps. Only
// InstanceWithEndpoints and PackageWithEndpoints call it, after componentsOf. Instance, Package,
// inventoryHealth, componentsOf, the list views and ReachPod do not.
func (e *evaluator) withEndpoints(ctx context.Context, ev evaluation, comps []Component)

// endpointsOf decides one Service entry: the state, or the zero value for "show nothing".
func (e *evaluator) endpointsOf(ctx context.Context, entry health.Entry) endpointsState

// readyEndpoints lists svc's slices (cached for ChildrenTTL) and counts them.
func (m *Model) readyEndpoints(ctx context.Context, svc *unstructured.Unstructured) (ready int64, found bool, err error)

// countReady keeps a slice only if its addressType is IPv4 or IPv6 and its controller owner
// reference reaches svc by reachInventory. Within one address type it counts each endpoint once
// (key: targetRef.uid, else the first address), as ready only if no copy has ready false. It
// returns the larger of the two sums. found is false when no slice was kept.
func countReady(slices []unstructured.Unstructured, svc types.UID) (ready int64, found bool)
```

`evaluation` keeps the `entries` that `inventoryHealth` already builds (a slice of references), so
`withEndpoints` can reach each Service's object and UID; it matches an entry to its
`InventoryObject` in `comps` by component name and reference. `Instance` and `Package` become thin
wrappers over unexported `instance` and `pkg` functions that take a `withEndpoints` flag; they pass
`false`. `InstanceWithEndpoints` and `PackageWithEndpoints` pass `true`. The shared body creates the
evaluator once (`ev := m.newEvaluator(who)`), calls `ev.inventoryHealth`, `componentsOf`, then
`ev.withEndpoints` only when the flag is set. In `internal/api`, `instanceGraph` and `packageGraph`
ask for the flag (`instanceDetail` and `packageWithSource` gain a `withEndpoints` argument); every
other caller passes `false`.

Order inside `endpointsOf` for one Service entry: skip (the zero value) unless `Access ==
AccessOK`, group `""`, kind `Service`, object non-nil. Then:

1. `callerAccess(list, endpointSlices, ns, "")` not `AccessOK` (denied or undecidable): `Unread`,
   no read.
2. `readerMay(list, endpointSlices, ns, "")` false: `Unread`, no read.
3. The cache, then a list with `LabelSelector: serviceNameLabel + "=" + name` under `readTimeout`,
   no page limit. An error, a timeout or a non-empty continue token: `Unread`. A failure is never an
   undercount.
4. `found` false: the zero value. Otherwise `Ready` set.

Each `Unread` logs the namespace, the Service name and the reason, no slice content (`log/slog`,
AGENTS.md "Error Handling And Logging"). A slice is counted and dropped, never stored. The result
is stored in `InventoryObject.EndpointsReady *int64` and `InventoryObject.EndpointsUnread bool`
(`views.go`) for an entry whose access is OK, and `internal/graph` copies both to the `Node`
(`instance.go:352`).

### The wire

```json
{ "id": "obj:_/Service/default/podinfo-podinfo", "kind": "object", "label": "podinfo-podinfo",
  "access": "ok", "endpointsReady": 2 }
```

```json
{ "id": "obj:_/Service/default/podinfo-podinfo", "kind": "object", "label": "podinfo-podinfo",
  "access": "ok", "endpointsUnread": true }
```

`GraphNode.EndpointsReady *int64 \`json:"endpointsReady,omitempty"\`` and
`GraphNode.EndpointsUnread bool \`json:"endpointsUnread,omitempty"\``. `openapi/v1alpha1.yaml`
`GraphNode` gains `endpointsReady: {type: integer, format: int64, minimum: 0}` with a description
that says "the ready endpoints the cluster records for a Service; absent when the read was not made
or failed, or when none are recorded", and `endpointsUnread: {type: boolean}` with "true when the
ready endpoints of a Service could not be read: the caller or the portal may not list its
EndpointSlices, or the list failed; never present with endpointsReady". `endpointsReady` is a pointer
so that `0` differs from absent. No new path, no new error: a node the caller may not read still
carries `access` only, and the `InventoryObject` of the Instance document is not changed.

### The panel

`factsOf` is a function of the object document, so the fact joins it from the node: `objectFacts`
(`panelfacts.go:43`) appends, after the Service's Ports, `fact{Label: "Endpoints", Value: "N ready"}`
when `n.EndpointsReady != nil` (value `fmt.Sprintf("%d ready", n)`), and `fact{Label: "Endpoints",
Value: "Could not be read"}` when `n.EndpointsUnread`. Plain text through `html/template`, not
monospace, no tone, no icon: it states a count and judges nothing, in the words of the Replicas fact
("2 ready of 2 desired") and of the Pods line. The `0 ready` case reads the same way. The fixed list
of facts grows by this one entry; nothing else changes. States of the panel:

- **Loading:** the existing `aria-busy` and "Loading..." line.
- **Empty (no slice recorded):** no Endpoints line.
- **Error (the read failed, or the portal's reader is denied):** `Endpoints: Could not be read`.
- **Permission denied (the caller lacks the review):** the same `Endpoints: Could not be read`, with
  the Service's other facts shown.
- **Object document unreadable:** today's note, and the facts of the object, the Endpoints fact
  with them, are dropped.
- **Node locked, forbidden or not readable:** no facts and no request, as now; the node carries no
  count and no flag.

At 360 px the facts grid is one column (archived change `align-owner-pages`, task 6.6); one short
line adds no width, and `TestBrowserPhone` holds it. The text carries the state, not colour, so
WCAG 2.2 AA contrast and non-colour cues need nothing new. The label and value stand in the same
`dl` as the other facts, so a screen reader reads "Endpoints, 2 ready" or "Endpoints, Could not be
read" in the order of the list. No element is focusable or new, so no keyboard path changes.

### Tests, and the new authorization case

The case that must be able to fail: over F1 plus the recorded slice fixture, the caller `alice`
with `AllowAll` reads `endpointsReady: 2` on podinfo's Service node; the same read with
`DenyResources("endpointslices")` (the Service, Pods and the rest still allowed) carries
`endpointsUnread: true` and no `endpointsReady`, and the panel shows `Endpoints: Could not be read`.
A test that reused the Service's review would pass the first half and fail the second. Beside it:

- **No read for a denied caller.** `readmodeltest.Actions` on the fake dynamic client shows no
  `list` of `endpointslices` when the caller's review is denied, and `Reviews.Asked()` shows a
  review with verb `list` and resource `endpointslices` in the Service's namespace when it is allowed.
- **Per caller at serve time.** Caller A (allowed) reads and fills the cache; caller B (denied)
  reads next and gets `Unread` and no count; the cache is warm for A.
- **Fail closed, shown as such.** A review that errors, a review that times out, a reader that may
  not list, a list that errors, a list that times out and a list that returns a continue token each
  give `Unread`, no count and no panic, and never `0`.
- **No slice is not unread.** A Service with no slice carries neither field and the panel shows no
  line.
- **The two fields never meet.** No node carries both `endpointsReady` and `endpointsUnread`.
- **Graph paths only.** `Instance`, `Package`, `ListInstances`, the package list and `ReachPod` make
  no review of `endpointslices` and no list of it (`Reviews.Asked()` and `Actions` are empty for it).
  At the API, the Instance and Package documents and their stream snapshot, the owner events
  endpoints (and the named-object check), the provider lookup of the Platform graph and both lists
  ask none either; the instance graph and the package graph ask one review for each Service.
- **Numbers only.** The fixture slices carry sentinel strings in every address, hostname, node name,
  zone, target Pod name and label value; the instance, graph, object, events and stream documents
  and every page contain none of them.
- **Reach.** A slice with the label and no owner reference, a slice owned by another Service, and a
  slice owned by a Service outside the inventory add nothing.
- **Count rules.** Table test of `countReady`: unset ready, false, terminating, two slices, a
  dual-stack pair, an FQDN slice (skipped), an endpoint in two slices of one type (counted once), the
  same endpoint with ready true in one copy and false in the other (not counted), an endpoint without
  `targetRef` (keyed by its address), only a placeholder slice (`0`), no slice (absent).
- **D7:R4.** `GET .../object` for an EndpointSlice ref is refused like any object no inventory
  reaches.
- **Other kinds unchanged.** The health rollup and the Applied badge of the Service are equal with
  and without slices.

The UI test reads the node from the API in process, as every panel test does. The browser check
extends `TestBrowserOwner` (the details panel of an owner page) to select podinfo's Service node
over F1 plus the slice fixture and read the Endpoints line, and `TestBrowserPhone` holds 360 px.
The live check, `task e2e:m1` against the throwaway fixture cluster
`E2E_CLUSTER=opm-portal-e2e-show-service-endpoints`, gains an assertion that podinfo's Service node
reports `endpointsReady` equal to its ready Pods.

### The recorded slice

F1 holds no EndpointSlice and is not recaptured by this change (a recapture would move every
golden for one kind). Section 1 records a live slice of the fixture cluster's podinfo Service, with
`managedFields` and the last-applied annotation removed, as
`internal/readmodel/testdata/endpointslices-podinfo.yaml`, and a helper in `readmodeltest` adds it
to F1 the way `F1Broken` adds its samples. The capture check (`task e2e:capture:check`,
`Taskfile.yml:151-155`) scans only `testdata/clusters/`, so the helper's own test does the same
refusal for this file: it fails on a Secret, on `managedFields` and on the last-applied annotation,
and so a later recapture cannot keep them unnoticed. (The other option, moving the file under
`testdata/clusters/`, would put a `List` file among F1-shaped captures; the test is the smaller
change.) The file holds Pod IPs and a node name of a throwaway kind cluster, nothing else, and
carries no Secret. Recapturing F1 with slices is a follow-up the supervisor may want.

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
  portal-security page; the rule is `list` only, one integer reaches the page. D11 is edited as a
  role-contract change (task 5.5).
- **A label-selected list is bounded by the Service, not by the namespace** → the call sets no page
  limit, so the answer is whole, and `readTimeout` bounds it. A Service has one slice for every
  hundred endpoints or so (*K8s, to check*). If a response ever carried a continue token, the code
  treats it as a failed read (`Unread`) and never as an undercount.
- **A graph read waits for one list per Service** → a cold graph read of an instance with many
  Services lists once for each, each bounded by `readTimeout`; the documents, the stream, the events
  endpoints, the provider lookup, the lists and `ReachPod` are not affected. If the wait shows in practice, the lists of one namespace can run together; that is an
  implementation detail inside `withEndpoints`.
- **The mark does not say why** → `Could not be read` covers three causes; the log and the docs page
  name them. A distinct mark for each stays a cheap follow-up.
- **A second read path** → one file, one function the evaluator calls, tested beside the children
  tests; `childrenIn` is not touched.
