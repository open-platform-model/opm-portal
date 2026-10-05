# OPM portal design

The portal's design record: the problem it solves, how it is built, the decisions that bind every
change, and the questions still open. Plan and progress live in [ROADMAP.md](../ROADMAP.md);
mechanism lives in the OpenSpec changes under `openspec/`.

**Numbering.** Decisions D1 to D11, their requirements and the open questions OQ1 to OQ20 were
enhancement 0030's numbers until that entry was withdrawn on 2026-10-05; the numbers are unchanged,
so `0030:D7:R4` and `portal:D7:R4` name the same requirement. Numbers added here start at D12 and
OQ21. The 2026-10-05 answers for the in-cluster milestone added D2:R7, D6:R10, D8:R5, D8:R6,
D11:R6, D12 and OQ21, and closed OQ2, OQ6, OQ8 and OQ20. The never-merged enhancements PR 101 drafted
the same answers under other numbers (OQ20 as 0030:D7:R5, the floor as 0030:D9:R6/R7); those
numbers were never adopted, and this file's are the only ones.

## Citing this record

Cite a portal decision as `portal:D7`, a requirement as `portal:D7:R4`, several decisions as
`portal:D3/D4`, several requirements of one decision as `portal:D6:R2/R8`, and a question as
`portal:OQ20`. Next to an enhancement, repeat each head: `portal:D9, 0027:D1`. A bare `D7` names
nothing outside this file. Inside this file, decisions refer to one another without the `portal:`
head.

Archived OpenSpec changes keep their `0030:Dn` citations as history; read each as `portal:Dn`.
Everything else cites `portal:Dn`, and no new text may cite the withdrawn entry.

## How this file is maintained

- **Numbers are permanent.** A decision, requirement or question number is never reused or
  renumbered. A withdrawn one keeps a one-line tombstone.
- **Decision text states what is true now.** A changed decision is edited in place, in the PR that
  changes the behavior, and the position it replaces moves to *Alternatives considered* with the
  evidence that retired it. The file never holds two conflicting decisions.
- **A decision binds a from-scratch rewrite.** Contract, policy and scope belong here; how the
  portal caches, which libraries it uses and how packages are laid out belong in OpenSpec changes.
- **Every decision names its source:** an owner decision, a measurement in
  [docs/design/evidence/](design/evidence/), or a supervisor ruling, which says so and waits for the
  owner.
- **A question closes with a status:** `resolved-by-Dn`, `answered`, `deferred-to-<entry or
  milestone>`, or `withdrawn`. An open one says whether it blocks a milestone.
- **Writes of any kind need a new decision first.** V2 writes build on enhancement 0027; a portal
  change that implements an enhancement decision cites it and carries `enhancement.yaml`.

## Summary

**V1 only reads (D1).** It creates, edits and deletes no stored object; the only requests it
creates are access reviews the API server never stores. Writes arrive with the marketplace (V2).

**The read API is the product (D2).** A versioned JSON and event-stream API at `/api/v1alpha1`
serves portal-shaped documents. The web UI renders only from that API, so adapters see what the UI
sees.

**Applied and healthy are two values (D3).** The operator's `Ready=True` shows as "Applied". Health
is computed from live objects, plus a Pod rule: a measured image-pull failure stayed Ready and
Available for ten minutes.

**Graphs come only from recorded state (D4).** No module is re-rendered. V1 draws no "requires"
edges, because the recorded contracts are everything a render used, not what it demands.

**Your access is the boundary (D5, D6, D7).** Locally, the portal reads with your kubeconfig.
In-cluster, a future plan, every read is checked for the signed-in user first and empty identity
is refused. Either way a missing object looks like a forbidden one.

**Nothing secret is read (D8, D10, D11).** No Secret data, no instance values, a read-only role,
logs only for OPM Pods. In-cluster, operator message text is hidden. Events are an expiring feed,
never state (D9). Kubernetes 1.34 is the floor (D12).

## Problem

OPM has no way to see, in one place, what it runs in a cluster. The opm-operator writes everything
a viewer needs into the Kubernetes API, spread over four kinds in `opmodel.dev/v1alpha1`:

- **Platform** (cluster-scoped singleton `cluster`): subscribed catalogs, the resolved registry,
  and conditions such as `ContractsFulfilled`.
- **ModuleInstance**: one deployed module, with conditions (`Ready`, `Reconciling`, `Stalled`), an
  inventory of every object the render applied, the contracts the render used, a short reconcile
  history, apply digests and failure counters.
- **ModulePackage**: a pre-rendered module from a Flux source, with a similar status.
- **TransformerRegistration** (cluster-scoped): a provider's claim to implement contracts, with
  verdicts in `status.accepted` and `status.active` and refusal and blocked-removal reasons in its
  conditions.

Rendered objects carry the `module-instance.opmodel.dev/name` and `.../uuid` labels but no
ownerReference back to their instance, so the instance-to-object relation exists only in the
inventory. Events are recorded through events.k8s.io/v1 by `opm-controller`.

**Nothing shows the whole picture.** The Platform, its catalogs, the registered providers, the
instances and their objects are four lists and a join the reader does in their head; generic
Kubernetes UIs cannot draw the most important relation, because no ownerReference exists.

**The obvious status is misleading.** `Ready=True` means the render applied, not that the workload
runs. On a live cluster, an instance whose new Pod could not pull its image stayed `Ready=True` for
the whole ten-minute progress deadline, and the Deployment stayed `Available=True` with it
([evidence 01](design/evidence/01-live-cluster-capture/), observation 3).

**Generic dashboards do not know OPM.** Headlamp, the Kubernetes Dashboard and Argo CD's tree draw
ownerReferences and label selectors. They cannot draw module to component to object, tell a refused
registration from a pending one, or read the inventory.

**Access is all or nothing.** The operator's only viewer role covers ModuleInstances; a non-admin
cannot read the Platform, ModulePackages or TransformerRegistrations, and no role aggregates into
the built-in `view` role.

User stories:

- As a **platform team operator**, I want to see the Platform's catalogs, the providers that
  registered and whether each registration was accepted, so that I can tell what this cluster
  offers.
- As an **application developer**, I want to open my instance and see its components, objects and
  Pods with honest health and recent events, so that I find a broken rollout in seconds.
- As a **tool author** (Headlamp plugin, Backstage adapter, MCP server), I want a stable,
  documented read API over OPM state, so that I can build on it without re-deriving the joins.

## Design

`opm-portal` reads what the opm-operator and the API server already record and serves it twice: as
a versioned JSON and server-sent-events read API, and as a server-rendered web UI that is that
API's first consumer.

### Goals

- **One place shows what OPM runs**, from the Platform's catalogs and registrations down to each
  instance's components, objects and Pods, with events and logs beside them.
- **Status never lies by omission.** Applied state and workload health are always shown apart, and
  a broken rollout shows as broken within seconds.
- **The read API is the product.** Anything the UI shows, a script or adapter can read through a
  versioned, documented API under the same identity.
- **A person sees what their RBAC allows, and nothing else.**
- **No new source of truth.** Every fact comes from a Kubernetes object the operator or the API
  server wrote. The portal does not render modules, store state, or invent edges.
- **Secrets stay unread**, and instance values are not shown in V1.

### Non-goals

- **Writes of any kind** in V1: no create, edit or delete of a stored object, no scale, restart or
  order. The only creates are access reviews the API server evaluates and never stores (D5:R6,
  D6:R9).
- **A general Kubernetes dashboard.** Objects are reachable only through an OPM inventory.
- **Contract demand and transformer provenance**, until the operator records them.
- **Persisted history.** No event store, no metrics backend; the portal is stateless apart from
  sessions.
- **Multi-cluster.** One portal reads one cluster.
- **Module presentation** (icons, cards, forms) belongs to V2.

### Approach

```
 Browser (HTMX pages)          Scripts, adapters, MCP
          |                             |
          v                             v
 +--------------------------------------------------+
 | opm-portal                                       |
 |   web UI  --in-process-->  read API (JSON + SSE) |
 |                               |                  |
 |   authorize as the user  -----+                  |
 |                               v                  |
 |   watched view of OPM kinds, inventory objects,  |
 |   runtime children; events and logs on demand    |
 +--------------------------------------------------+
          | get / list / watch (never write)
          v
 Kubernetes API: Platform, ModuleInstance, ModulePackage,
 TransformerRegistration, inventory objects, Pods, events
```

**Two milestones, one design.** Milestone 1 is a binary on the user's machine, bound to loopback,
reading with the user's kubeconfig; each read is checked first with a SelfSubjectAccessReview, so a
node the user may not read shows as locked before it is read. Milestone 2, a future plan (D6,
Status), runs the same binary in-cluster: users sign in with OIDC, every read is authorized by a
SubjectAccessReview for that user, and the portal then reads with its own narrow, read-only
ServiceAccount.

**The portal keeps a watched view.** The four OPM kinds are watched. Inventory objects are watched
per kind, selected by the OPM instance label, once an inventory names that kind. Pods and
ReplicaSets below an instance are watched only while someone has that instance open. Events and
logs are read on demand. Where the reading identity can only `get` a kind, the portal polls it and
says how old the answer is. Serving one instance's graph with request-time reads took 7.4 to 9.4 s
([evidence 01](design/evidence/01-live-cluster-capture/), observation 12), so the watched view is
required.

**Two status axes, computed apart.** `Ready=True` shows as **Applied**. Health is the portal's own
computation: per object by kstatus, plus the Pod rule of D3:R2, then worst-of through components to
the instance. An object the reader cannot see makes the result partial, not healthy.

**Graphs come from status, not from a render.** The Platform view joins the Platform's registry,
the TransformerRegistrations and the instances. The instance view runs module, instance,
components, inventory objects, then Pods and ReplicaSets found through ownerReferences below
inventory workloads. Every edge kind has exactly one source.

### API surface

V1 adds no CUE and changes no OPM schema. Its contract is the read API, described by what a client
relies on; `openapi/v1alpha1.yaml` holds the route table.

- **Versioned path.** Every resource, the change stream included, lives under `/api/v1alpha1`.
- **Portal-shaped documents** with a `kind` and the portal's own `apiVersion`
  (`portal.opmodel.dev/v1alpha1`). Raw objects appear only in explicit YAML views, stripped as D8
  requires.
- **An instance document carries both axes**: a `reconcile` block and a `health` block (state,
  per-state counts, `partial`, `evaluatedAt`, `live`), never one merged status.
- **Graph documents**: nodes with stable ids that never embed a UID, an access state (`ok`,
  `forbidden`, `notReadable`) and an optional collapsed summary; edges with a kind and their source.
- **One change stream per client**: server-sent events carrying the same document shapes as the
  GETs, plus Kubernetes events and log lines as topics, resumable or answered with a resync.
- **Problem documents**: RFC 9457, with a closed but extensible `code` set (unauthenticated,
  forbidden, not readable by the portal, not found, bad request, too many streams, upstream
  unavailable).

### Affected repos

- **opm-portal**: the read API and change stream, the web UI, local mode, in-cluster mode,
  binaries and an image, later an install manifest.
- **opm-operator**: viewer roles a cluster administrator can bind (D11:R4). Two status additions
  are follow-ups, not V1: provider-contract demand (OQ18) and a transformer per inventory entry
  (OQ5).
- **opmodel.dev**: the portal's documentation bundle, published from `docs/site/`.

## Decisions

Each decision carries a **Kind** (`contract`, `policy` or `scope`), the decision, its requirements,
the alternatives considered, the rationale and the source. **Depends** names an enhancement
decision it rests on.

### D1: V1 is read-only

**Kind:** scope

**Decision:** The V1 portal creates, edits and deletes no stored object, in either milestone. The
only requests it creates are access reviews the API server evaluates and never stores: the self
reviews of D5:R6 locally and the SubjectAccessReview of D6:R9 in-cluster, which D11 grants. It reads
the four OPM kinds, the objects their inventories name, the runtime children below those objects,
events and Pod logs. Writes, including ordering a module, arrive with the marketplace (V2), which
builds on the self-service kinds of enhancement 0027. A CLI-owned ModuleInstance is shown like any
other instance and is equally read-only.

**Requirements:** none (scope boundary; the observable consequences are D5's and D6's review-only
creates and D11's read-only role)

**Alternatives considered:**

- **Simple writes in V1** (restart a workload, suspend an instance, delete). Each needs a write
  identity, which in-cluster means impersonation or token passthrough. Both reverse the posture D6
  takes for reads and widen the blast radius of the empty-identity class of bug. Deferred to the
  marketplace (V2), where the write identity is an open question in its own right.
- **Ordering modules in V1.** Needs the 0027 definition kind, a draft, plus a presentation contract
  that does not exist yet.

**Rationale:** A read-only V1 ships on today's operator with no dependency on unaccepted
enhancements, and removes every write-side security question from the first release.

**Source:** Owner decision 2026-10-04 (V1 read-only, writes in V2 through 0027 kinds); see
[ROADMAP](../ROADMAP.md). Owner decision 2026-10-04 ("Keep the seam": the only allowed creates are the non-persisted review APIs).

### D2: The versioned read API is the durable contract, and the web UI is its first consumer

**Kind:** contract

**Decision:** The portal's contract is a read API under a versioned path, `/api/v1alpha1`, serving
JSON documents over `GET` and a server-sent-events change stream. The web UI renders from that
API's documents, in-process, under the caller's identity: it has no read path of its own. The
documents are shaped for the portal (instance, package, platform, registration, contract, graph,
event), never a passthrough of a custom resource's status. The API stays at `v1alpha1` until a
declared stability point (OQ10); within the version, changes are additive and clients ignore what
they do not know. The change stream writes a subscriber only what changed in the documents rendered
for that subscriber, so one subscriber cannot time a change only another may see.

**Requirements:**

- R1: Every fact the web UI shows is available from the read API to a client holding the same
  identity, with the same filtering.
- R2: The API version is part of every path; within one version, a field or enum value may be added
  but not removed or renamed without the release notes marking the change as breaking.
- R3: Clients are told to ignore unknown fields and to treat every enum as open; the portal's own UI
  renders an unknown enum value as "unknown", never as an error.
- R4: No API document embeds a raw custom-resource status; raw objects are served only by explicit
  YAML views, under D8's stripping rules.
- R5: The change stream carries the same document shapes as the corresponding `GET`, and a client
  that reconnects either resumes from its last event or is told to re-read.
- R6: Errors are RFC 9457 problem documents carrying a `code` from a closed but extensible set, so a
  client can tell "you may not read this" from "the portal may not read this" from "not found".
- R7: A subscriber receives a change event only when the document rendered for it differs from the
  last one sent to it on that stream; a change it cannot see produces no event, no event id and no
  other observable signal. A delete, a snapshot and a topic's closing are always sent. Log lines
  are exempt: they are only ever sent to a reader of that Pod.

**Alternatives considered:**

- **HTMX pages with no public API** (the API added later). Fastest V1, but every adapter later
  re-derives the joins. Rejected by the owner.
- **A JavaScript single-page app over the JSON API.** Costs a client build and client templates.
- **UI and API as two presenters over a shared service layer.** The API then has no consumer in V1
  except tests, so nothing keeps it complete.
- **Version `v1` from day one.** Everything upstream is `opmodel.dev/v1alpha1` and may still change.
- **Change detection per topic only** (R7's earlier absence). Every subscriber of a topic re-renders
  on any change, so in-cluster a user learns when something they cannot see changed, though not
  what.

**Rationale:** Standalone Kubernetes UIs have a poor survival record (Kubernetes Dashboard,
Kubeapps and Glasskube are archived), and the portals that last expose a neutral read model that
adapters build on ([research](design/evidence/prior-art-and-access.md)). Making the UI a real
consumer keeps the API complete.

**Source:** Owner decision 2026-10-04 ("Go API + HTMX on top"); [research](design/evidence/prior-art-and-access.md),
portals and dashboards. R7: supervisor ruling 2026-10-05 answering OQ20, pending the owner.

### D3: Applied state and workload health are two axes, never merged

**Kind:** contract

**Depends:** 0015:D14, 0015:D18

**Decision:** Every instance and package shows two values side by side. **Applied** state comes
from the operator's conditions: `Ready=True` is shown as "Applied" and never as healthy, because the
operator's Ready means every apply succeeded, with no wait on workload health. **Health** is the
portal's own computation from live objects: each object's status by kstatus, plus one Pod rule,
rolled up worst-of through components to the instance, with unreadable objects excluded and the
result marked partial. The Pod rule exists because an image-pull failure stays invisible to both
the operator and kstatus for the whole progress deadline: a container waiting with `ErrImagePull`,
`ImagePullBackOff`, `CrashLoopBackOff`, `CreateContainerConfigError` or `InvalidImageName` marks
the Pod's owning workload Degraded. A CLI-owned instance shows its applied state as managed
externally, in a neutral style, and its health is still computed from the inventory the CLI wrote.
The operator's failure counters and drift flag are diagnostics, never health.

**Requirements:**

- R1: An instance's applied state and its workload health are shown as two separate values; neither
  is derived from the other, and a `Ready=True` instance is never labelled healthy.
- R2: A Pod whose container waits with an image-pull, crash-loop, container-config or invalid-image
  reason marks its owning workload, and so its component and instance, Degraded, even while the
  workload's own conditions report it available.
- R3: In the scripted image break (the instance's image set to a tag that does not exist, so the
  new Pod cannot pull while the old replicas keep serving), the instance's health is Degraded within
  seconds of the new Pod reporting its waiting reason, while its applied state stays Applied.
- R4: An object the portal or the user cannot read is excluded from the roll-up and the result says
  it is partial; a Secret is never read and does not make a result partial.
- R5: An object whose health is refreshed by polling rather than by a watch shows when it was last
  evaluated, and the instance's health says it is not live.
- R6: A CLI-owned instance shows its applied state as managed externally, without an error style,
  and shows workload health computed from its inventory.
- R7: The operator's failure counters and drift flag never change either axis.
- R8: A Platform reporting unfulfilled provider contracts is shown as information, not as a
  failure.
- R9: An instance page and its graph are answered from state the portal already holds; their
  response time does not grow with one Kubernetes read per inventory object.

**Alternatives considered:**

- **One merged status badge.** With Ready meaning applied, it shows green for the whole ten-minute
  window in which a rollout is broken (evidence 01, observation 3).
- **kstatus alone for health** (the design before the capture). The Deployment stayed
  `Available=True` and kstatus said InProgress until `ProgressDeadlineExceeded`, about 600 s after
  the break.
- **Failure counters as a health hint.** The drift counter climbs on healthy instances because the
  operator's drift check ignores the instance's ServiceAccount (opm-operator issue 209).
- **Wait for an operator `Healthy` condition.** No health wait exists or is planned (0015:D14), and
  health in the operator is OQ3, not a V1 prerequisite.
- **Health computed per request** (one read per inventory object). Measured at 7.4 to 9.4 s for
  cert-manager's graph at client-go's default rate limit.

**Rationale:** The portal's main promise is that a broken rollout looks broken. Ready is correct for
what it says, apply success, so the portal keeps it and labels it honestly, and computes health from
the objects that show the failure.

**Source:** [Evidence 01](design/evidence/01-live-cluster-capture/), observations 3, 6, 11 and 12,
and the CLI-owned instance addendum. 0015:D14 (readiness means apply success); 0015:D18 (an
unfulfilled contract is reported, never refused), which R8 follows. Owner decision 2026-10-04 to
capture a CLI-owned instance on the throwaway cluster.

### D4: Graphs derive only from operator- and API-server-written state

**Kind:** contract

**Depends:** 0015:D3

**Decision:** The portal never renders a module. Every node and edge comes from a field a Kubernetes
object already carries, and each edge kind has exactly one source: subscriptions from the Platform's
resolved registry; registration-to-catalog from the registry entries a registration contributed;
registration-to-instance from the registration's provider reference, cross-checked against the
provider's inventory; instance-to-module from the instance spec; component and object edges from
the inventory; runtime children from ownerReferences walked below inventory workloads;
package-to-package from the package's dependencies. **V1 draws no "requires" edge from an instance
to a provider contract**: the instance's recorded contracts are every contract its render used,
most fulfilled by the catalog itself, not the provider contracts it demands, so V1 lists them as
text, and requires edges wait for the operator to record provider demand. Registrations show acceptance and
activation as separate states, read from `status.accepted` and `status.active`, never inferred from
the `Stalled` and `Ready` conditions: the same condition pair marks both a refused claim and an
accepted, active claim whose removal is blocked by dependents. A blocked removal is its own state.
Configuration-only components are grouped into one expandable node by default.

**Requirements:**

- R1: Every node and edge in a portal graph is traceable to a field of a Kubernetes object; no graph
  is produced by rendering a module.
- R2: Where two sources for the same relation disagree (a registration's provider reference and the
  provider's inventory), the graph shows the disagreement instead of picking one.
- R3: V1 shows no edge from an instance to a contract; the contracts an instance's render used are
  listed on the instance as plain text, labelled as the render's contracts, not its provider demand.
- R4: A registration shows whether it was accepted and whether it is active as two separate states,
  taken from its `status.accepted` and `status.active`; a refused registration shows its refusal
  reason and message.
- R5: Components that own no workload are grouped into one expandable node by default.
- R6: Graph node identifiers are stable across portal restarts and do not change when the
  underlying object is deleted and recreated.
- R7: A registration that is being deleted while instances still demand its contracts is shown as
  removal blocked, naming the reason and message, and keeps showing as accepted and active; it is
  never shown as refused.

**Alternatives considered:**

- **Requires edges from `status.requiredContracts`** (the design before the capture). cert-manager
  lists 15 contracts and podinfo 7, nearly all catalog-fulfilled, so the edges would claim every
  instance depends on every core resource. Waiting for an operator field that records provider
  demand (OQ18) is the honest path.
- **Re-render modules in the portal** for provenance and demand. Tens of megabytes per module,
  registry access, and a second render path that can disagree with the operator's.
- **Instance-to-object edges from labels.** The uuid label is not on Pods or ReplicaSets, and labels
  are a cross-check, not the record.
- **Acceptance read from the conditions.** Phase 9 of the capture showed `Stalled=True` plus
  `Ready=False`, reason `DependentsRemain`, on a claim that stayed accepted and active.
- **Every component as its own node.** cert-manager's graph had 86 nodes and 85 edges, 20 of them
  components (observation 13).

**Rationale:** One source per edge kind makes every edge explainable and every disagreement
visible. Drawing an edge the data does not support is worse than drawing none, because a platform
team would act on it.

**Source:** [Evidence 01](design/evidence/01-live-cluster-capture/), observations 2, 8, 9 and 13,
phases 7 to 9; [evidence 02](design/evidence/02-live-graph-spike/). Registration verdicts as
0015:D3 defines them.

### D5: Milestone 1 runs locally, and the user's kubeconfig is the boundary

**Kind:** contract

**Decision:** In local mode the portal is a binary on the user's machine that reads the cluster
only as the kubeconfig's identity. It binds to loopback only, refuses a request whose `Host` is not
that loopback address, and admits a browser only through a one-time launch token exchanged for a
session cookie. There is no login code against the cluster and nothing to install in it. Before
each read the portal asks the API server whether the kubeconfig's identity may make it, with a
SelfSubjectAccessReview for the exact verb, resource, namespace and name, and learns who that
identity is with a SelfSubjectReview. A node the user may not read is shown locked up front, and
in-cluster mode swaps the review for D6's SubjectAccessReview behind the same check. These two
reviews are the only objects local mode creates; the API server evaluates them and stores nothing.

**Requirements:**

- R1: In local mode every cluster read is made as the kubeconfig's identity; the portal adds no
  credential and installs nothing in the cluster.
- R2: Local mode refuses to listen on a non-loopback address.
- R3: A request that does not carry the session established from the launch token is refused, so
  another local user or process cannot read the cluster through the portal.
- R4: A request whose `Host` header names anything other than the loopback address and port is
  refused.
- R5: A user who cannot list the OPM kinds cluster-wide can still use the portal on the namespaces
  they can read.
- R6: In local mode the only create requests the portal sends are `selfsubjectaccessreviews` and
  `selfsubjectreviews`; it sends no other create, update, patch or delete.
- R7: In local mode every read is preceded by an allowed SelfSubjectAccessReview for the
  kubeconfig's identity and the exact request attributes, and an object the identity may not read
  is shown as locked without being read.

**Alternatives considered:**

- **In-cluster first.** Needs OIDC, a session store, RBAC and an install manifest before anyone can
  see anything.
- **Loopback without a launch token.** Any local process or user can reach loopback; the token is
  the same defence notebook servers use.
- **A container image as the only artifact.** A container cannot reach a kind API server on host
  loopback or serve a browser on loopback without host networking.
- **No access review in local mode** (read, and map the API server's refusal). The UI learns a node
  is locked only after a failed read, and in-cluster mode would need a second authorization path.

**Rationale:** The user's own RBAC is already the right boundary on their machine. Local mode
exercises every read path and the whole API before any authentication code exists. A
SelfSubjectAccessReview asks about whoever sends it, which locally is the user, so R6 allows the
self reviews here and D6:R9 allows only the SubjectAccessReview in-cluster.

**Source:** Owner decision 2026-10-04 ("Local first, then in-cluster"). Owner decision 2026-10-04
("Keep the seam": the only allowed creates are `subjectaccessreviews`, `selfsubjectaccessreviews`
and `selfsubjectreviews`; milestone 1 shows locked nodes up front, and milestone 2 swaps the
backend without rewriting the seam).

### D6: Milestone 2 authorizes every read as the signed-in user, then reads as the portal

**Kind:** contract

**Status:** in-cluster mode is a future plan, and its OIDC half is not implemented. Owner decision
2026-10-05, asked about the merged OIDC sign-in after writing "I thought you only would add local
auth, and not oidc?": "Remove OIDC, but keet it as future plans". The OIDC text below (sign-in with
the authorization code flow and PKCE, bearer JWTs, the identity mapping and its prefixes) and R5,
R6 and R8, with the claim-mapping parts of R2 and R3, are a future plan, kept with their numbers.
Their implementation (PR 28) was removed by the change `remove-oidc-sessions`; it is preserved in
the archived change `2026-10-05-add-oidc-sessions` and in git history at f5a8eaa. The review half
stays on main, dormant: the SubjectAccessReview authorizer (R1, R3's review side, R4, R7, R9, R10)
and the read API's refusal of an empty identity before any review (R2). No command constructs
in-cluster mode yet.

**Decision:** In-cluster, users sign in through OIDC (authorization code with PKCE); programmatic
clients present a bearer JWT from the same issuer, naming the portal's configured audience. For
every read the portal sends a SubjectAccessReview carrying the user's mapped name and groups for
the exact verb, resource, namespace and name, and reads with its own ServiceAccount only on allow.
Identity mapping fails closed: an empty mapped username is refused before any Kubernetes call, a
`system:` username is refused, every `system:` group from the identity provider is stripped, and
`system:authenticated` is added for every authenticated principal. Mapped names carry a non-empty
prefix unless the deployment declares that the API server trusts the same issuer with the same
prefixes. A review that errors or times out is a denial. The reads that keep the watched view
current are not a person's: they are authorized by a SubjectAccessReview for the portal's own
ServiceAccount, with exactly the groups the API server gives it, sent at startup and renewed at a
bounded interval. A denied, failed or expired review stops that read, and no person's request is
ever answered with that review. The portal holds no impersonate permission and no write verb other than
`create` on the review API of R9; the only object it creates is the SubjectAccessReview, which the
API server evaluates and does not store. It keeps a per-user log of reads because the API server's
audit log sees only the portal's ServiceAccount.

**Requirements:**

- R1: In-cluster, every read is preceded by an allowed SubjectAccessReview for the signed-in user's
  mapped identity and the exact request attributes.
- R2: A principal whose mapped username is empty is refused with no Kubernetes call made on its
  behalf.
- R3: No `system:` group asserted by the identity provider reaches a SubjectAccessReview, a
  `system:` username is refused, and `system:authenticated` is present for every authenticated
  principal.
- R4: A SubjectAccessReview that errors, times out or returns an evaluation error without allowing
  is treated as a denial and is never cached.
- R5: A bearer-token client sees exactly what the same user sees in the browser.
- R6: Unless the deployment declares that the API server trusts the portal's issuer, the portal
  refuses to start with an empty username or groups prefix.
- R7: The portal records, per authenticated person, each read it authorized and each it denied.
- R8: A bearer token is accepted only when it is signed by the configured issuer and names the
  configured audience; any other token is refused with no Kubernetes call made on its behalf.
- R9: In-cluster, the only create request the portal sends is `subjectaccessreviews`; it sends no
  other create, update, patch or delete.
- R10: In-cluster, a background read the portal makes for no single user is preceded by an allowed
  SubjectAccessReview naming the portal's own ServiceAccount for that read's attributes, never by a
  self review, sent at startup and renewed at a bounded interval; a denied, failed or expired review
  stops that read, and what it would have read is shown as not readable. No person's request is
  answered with that ServiceAccount's review.

**Alternatives considered:**

- **Impersonation** (the Flux Operator UI's model). Requires the impersonate verb, the blast radius
  of the empty-identity bug class (CVE-2026-23990: empty claims fell through to the server's own
  identity). Revisiting it after V1 is OQ1.
- **Token passthrough.** Works only where the API server trusts the portal's issuer; it stays an
  option for writes in V2.
- **A shared portal identity with no per-user check.** Rejected outright.
- **Background reads with no review** (the read model trusts its own role). Leaves the background
  path as the one read that skips the seam every other read goes through.

**Rationale:** Reads need the user's authorization, not the user's credential. Checking with a
SubjectAccessReview and reading with a narrow ServiceAccount keeps the user's RBAC as the boundary
without granting the portal the power to become anyone. The cost, users being invisible in the API
server's audit log, is paid by R7. In-cluster a self review would describe the portal's own
ServiceAccount, not the user, so R9 forbids it: a check that answers for the portal would make it a
confused deputy.

**Source:** Owner decision 2026-10-04 (milestone 2: in-cluster Deployment, OIDC login,
SubjectAccessReview-as-user, fail closed on empty identity). Owner decision 2026-10-04 ("Keep the
seam"). Owner decision 2026-10-05 ("Remove OIDC, but keet it as future plans"; see Status). R10: owner answer 2026-10-05 ("SAR for its own SA (Recommended)": at startup and per TTL
the portal sends a SubjectAccessReview naming its own ServiceAccount, background reads hold that
grant, and every user-facing answer is still gated by a review as the user). [Research](design/evidence/prior-art-and-access.md),
access model.

### D7: Authorize before lookup, and never reveal existence

**Kind:** contract

**Decision:** Every request is authorized on its attributes (verb, resource, namespace, name)
before anything is looked up, in both milestones. A caller who may not read a kind in a namespace
gets the same refusal whether or not the object exists. Lists return only what the caller may read.
Partial access inside a response is reported per item as an access state, not as a failed request.
Only objects reachable from an OPM inventory are served: the portal is not a general cluster
browser.

**Requirements:**

- R1: A caller without read access to a kind in a namespace receives the same refusal for an
  existing and a non-existing object of that kind.
- R2: A list contains only items the caller may read, and a caller with no access receives an empty
  list with no count or name of hidden items.
- R3: Objects inside a readable response that the caller cannot read are marked forbidden, and
  objects the portal itself cannot read are marked not readable; neither fails the response.
- R4: A request for an object no OPM inventory reaches, including an events request about such an
  object, is refused with the same `forbidden` problem document a forbidden read gets (R1); there
  is no distinct not-in-inventory code, so a caller cannot learn whether such an object exists.

**Alternatives considered:**

- **Look up first, then authorize.** A missing-versus-forbidden difference leaks which objects
  exist.
- **Serve any object the caller may read.** Turns the portal into a general cluster browser and
  widens its role.
- **A distinct not-in-inventory refusal** (the earlier R4). Tells a caller who passed authorization
  that the object is outside every inventory, and an events request would answer whether a named
  object exists.

**Rationale:** A shared cache serving many users is exactly where a cross-tenant leak happens.
Making authorization the first step of every read path, and testing that a caller without access
sees nothing, is cheaper than auditing each handler.

**Source:** Portal architecture, error model and access sections;
[research](design/evidence/prior-art-and-access.md), access model. R4: supervisor ruling 2026-10-04,
pending the owner.

### D8: Secret data is never read, and values are not shown in V1

**Kind:** contract

**Depends:** 0013:D28

**Decision:** The portal never reads a Secret's data, in any mode, so it can never serve one. In V1
it shows no instance's `spec.values`: users supply plain values that unification marks as secrets
only inside the module's schema, and modules still take plain-string passwords, so no marker in the
stored values identifies what to hide. The `kubectl.kubernetes.io/last-applied-configuration`
annotation is stripped from every object the portal serves, because a client-side apply copies the
full values into it. Hiding `spec.values` does not hide what the values became: a value a module
renders into a non-Secret object is shown in that object's YAML view to any user whose RBAC lets
them read it, exactly as `kubectl` would show it. Keeping a value out of reach means rendering it
into a Secret. The operator's embedded kernel does not redact secret paths in the text it writes,
so in-cluster the portal shows the reason of every condition, history entry, registration verdict
and event the operator writes, and never the operator's message text; local mode shows that text
verbatim, because it reveals nothing the user's kubeconfig cannot read. Text other writers wrote,
the kubelet's and other controllers' event notes and the health messages of rendered workloads,
is shown in both modes.

**Requirements:**

- R1: The portal reads no Secret's data in any mode, and its in-cluster role grants no access to
  Secrets.
- R2: No API document, YAML view or page in V1 contains an instance's or package's `spec.values`.
- R3: No object the portal serves carries the `kubectl.kubernetes.io/last-applied-configuration`
  annotation.
- R4: The portal's documentation states that values rendered into non-Secret objects are visible
  to anyone who may read those objects, in the portal as in `kubectl`.
- R5: In-cluster, no API document, stream event or page carries message text the operator or the
  kernel wrote (condition, history and registration messages, the notes of events whose
  `reportingController` is `opm-controller`, the reconcile message built from them, and the
  messages in an OPM object's health and raw status); reasons are shown. An event that names no
  reporting controller and regards an OPM object is treated as the operator's. The kubelet's and
  other controllers' event notes and the kstatus health messages of rendered workloads are shown.
  This holds until the kernel redacts that text.
- R6: In local mode, condition, event and history messages are shown verbatim.

**Alternatives considered:**

- **Show values with secret leaves masked.** There is nothing to mask on: secret markers are added
  by unification inside the module, not stored in `spec.values`.
- **Show values to users who may read Secrets in every namespace the inventory renders a Secret
  into.** Still cannot find secret leaves without the module's schema walker. Left for later (OQ7).
- **Show operator messages in-cluster** (OQ8's other answer). A message can carry a value the user
  could not otherwise read, and the portal cannot tell which.
- **Hide every message and event note in-cluster, whoever wrote it.** Removes the kubelet's
  image-pull and probe notes, the remediation users need most, to guard against a kernel that did
  not write them. Rejected by the supervisor's scope ruling under R5.

**Rationale:** A client-side apply copies every value into the annotation (observation 14). Hiding
values and stripping the annotation is the only rule V1 can keep without reading module schemas.
Messages are hidden in-cluster for the same reason: without kernel redaction there is nothing to
mask on.

**Source:** [Evidence 01](design/evidence/01-live-cluster-capture/), observation 14. Core's secret
marker shape and the plain-string password fields in the first-party modules, read from source. R5:
owner answer 2026-10-05 to OQ8 ("Hide messages in-cluster"). R6: the local-mode half of the same
answer, which the question's text recorded (local mode reveals nothing the kubeconfig cannot read).
R5's scope: supervisor ruling 2026-10-05, not an owner decision. The question asked about operator
messages, so in-cluster hides the text the operator writes (OPM resource condition messages,
`status.history` messages, events whose `reportingController` is `opm-controller`, and the
reconcile message built from them); the kubelet's and other controllers' event notes and the
kstatus health messages of rendered workloads stay visible, because they are not kernel
diagnostics and carry the remediation users need.

### D9: Conditions and history are the record; events are an expiring feed

**Kind:** contract

**Decision:** Status badges and the durable part of the timeline come from conditions and
`status.history`. Kubernetes events are a recent-activity feed with the API server's roughly
one-hour lifetime, labelled as such, and no displayed state is inferred from them. The portal
deduplicates events itself: the recorder folds a repeat into `series` only when it regards the same
object version, so repeats after the object's status changed arrive as separate events, and kubelet
events count through the deprecated count and timestamp fields, so
repeated events about the same object with the same reason and message become one line with a count
and the latest time, however each repeat was recorded. Events about the cluster-scoped Platform and
TransformerRegistrations, which Kubernetes records in namespace `default`, appear on those objects'
pages. Render warnings the operator records only as events are labelled on the instance page as
expiring with the feed.

**Requirements:**

- R1: Every status value the portal shows comes from conditions or status history; none is
  inferred from an event.
- R2: The events feed is labelled as recent activity that expires.
- R3: Repeated events about the same object with the same reason and message appear once, with a
  count and the latest occurrence time, for both operator and kubelet events, whether each repeat
  was recorded as a separate event, in an event's `series`, or in its deprecated count.
- R4: Events about the Platform and about each TransformerRegistration appear on that object's
  page.
- R5: The instance page states that render warnings are kept only as events and that older ones are
  gone.

**Alternatives considered:**

- **Rely on event `series` for collapsing.** In the capture only three operator events carried
  `series`, all from an unreleased build; cert-manager's four identical `ApplyFailed` repeats were
  four separate events, and kubelet events have `eventTime: null` (observation 4).
- **Look for Platform events in the Platform's namespace.** It has none; they are in `default`
  (observation 5).
- **Persist events in the portal.** Makes the portal stateful and a second record of history (OQ9).

**Rationale:** Events are transition-only and expire, so they cannot carry state. Labelling them
honestly and folding repeats keeps the feed useful without pretending it is history.

**Source:** [Evidence 01](design/evidence/01-live-cluster-capture/), observations 4, 5, 7 and 10.

### D10: Logs stream only for Pods an inventory reaches, bounded by the portal

**Kind:** contract

**Decision:** Pod logs can be streamed only for a Pod reachable from an instance's or package's
inventory through workload ownership: Deployment to ReplicaSet to Pod, StatefulSet or DaemonSet to
Pod, Job to Pod, and CronJob to Job to Pod. The user's `pods/log` access is checked before a stream
starts and again on reconnect. The portal bounds each stream itself: an oversize line is truncated
with a marker, lines beyond a per-stream rate are dropped and counted, and an oversize initial tail
skips ahead to live output with a marker. It never uses the Kubernetes byte limit, which ends a
followed stream outright.

**Requirements:**

- R1: A log stream is refused for any Pod no OPM inventory reaches through workload ownership.
- R2: A log stream starts only after the user's `pods/log` access is confirmed, and is re-checked
  on every reconnect.
- R3: A stream never ends because of a byte limit; truncated and dropped lines are marked in the
  stream, with a count of what was dropped.
- R4: A container that stops ends its stream with an explicit end-of-stream message.

**Alternatives considered:**

- **Logs for any Pod the user can read.** Makes the portal a general log viewer and widens its role.
- **The API server's `limitBytes`.** Ends the whole stream, followed output included.

**Rationale:** Keeping logs inside the OPM view keeps the role narrow. Bounding in the portal
protects the browser and the portal without cutting off the live tail a developer is watching.

**Source:** Portal architecture, logs section. The Deployment to ReplicaSet to Pod chain was
measured ([evidence 01](design/evidence/01-live-cluster-capture/), observation 8); the capture held no StatefulSet, DaemonSet, Job or CronJob, so those chains are
design, read from the Kubernetes controllers' ownerReference behaviour.

### D11: The portal's role is read-only, follows the catalog, and the operator ships viewer roles

**Kind:** contract

**Decision:** The in-cluster portal's ClusterRole grants `get`, `list` and `watch` on the four OPM
kinds and their status, on events, on every non-Secret kind the pinned OPM catalog's transformers
can render, and on the runtime children those workloads own (Pods, `apps` ReplicaSets, and `batch`
Jobs a CronJob creates); `create` on `subjectaccessreviews`; and `get` on `pods/log`. The role
grants no other create, no update, patch or delete, no impersonate and no Secrets. The kind list is
checked against the pinned catalog, so a catalog bump that adds a kind fails before release. Kinds
that provider modules define (cert-manager's Certificate, for example) are not covered in V1 and
show as not readable. On the user side, the opm-operator ships viewer roles a cluster administrator
binds so non-admins may read Platforms, ModulePackages and TransformerRegistrations. They do not
aggregate into the built-in `view`, `edit` or `admin` roles: platform state is shown only to whom
an administrator grants it.

**Requirements:**

- R1: The in-cluster portal's role contains no impersonate verb, no access to Secrets, and no verb
  that stores or changes an object; its only `create` is on `subjectaccessreviews`.
- R2: Every non-Secret kind the pinned OPM catalog can render, and every runtime child kind below
  those workloads (Pods, ReplicaSets, Jobs), is readable by the portal's role, and a catalog that
  adds a kind the role does not cover is caught before the portal releases.
- R3: An inventory object of a kind the portal's role does not cover is shown as not readable, with
  the reason, never omitted.
- R4: A cluster administrator can grant a non-admin read access to Platforms, ModulePackages and
  TransformerRegistrations using a role the operator ships.
- R5: A user without read access to the Platform sees that it is hidden by their access, not an
  empty Platform.
- R6: No viewer role the operator ships carries an aggregation label for the built-in `view`,
  `edit` or `admin` roles.

**Alternatives considered:**

- **A wildcard read role.** Covers Secrets and every other tenant's objects; rejected.
- **Aggregate provider kinds into the portal's role through a label in V1.** Needs a decision on
  which provider kinds a portal may read; moved to a follow-up.
- **No operator change, and document hand-written roles.** Leaves every installation to rediscover
  the same role.
- **Aggregate the viewer roles into `view`** (OQ6's other answer). Every namespace viewer would see
  platform state, and in `kubectl` the `spec.values` of every ModuleInstance in reach.

**Rationale:** A read-only role is the in-cluster form of D1. Checking it against the catalog keeps
"read-only" from silently turning into "cannot see half the objects" after a catalog bump.

**Source:** Owner decision 2026-10-04 ("Keep the seam"). opm-operator's shipped RBAC, read from
source; kind list from the opm catalog's transformer outputs, read from source;
[research](design/evidence/prior-art-and-access.md). R6: owner answer 2026-10-05 to OQ6 ("No
aggregation").

### D12: Kubernetes 1.34 is the supported floor

**Kind:** policy

**Decision:** The portal supports Kubernetes 1.34 and later, the oldest version OPM is tested on.
Every Kubernetes feature the portal relies on (the events.k8s.io/v1 field selectors of D9, the
review APIs of D5 and D6) must work at that floor, and features introduced later (such as
ConstrainedImpersonation, OQ1) are not used until the floor moves.

**Requirements:**

- R1: The portal's documentation states 1.34 as the minimum supported version.
- R2: The events field selectors the portal sends (`regarding.*`, `reason`, `type`) are verified
  against a 1.34 API server before the in-cluster release.
- R3: The portal's CI keeps at least one job on a Kubernetes 1.34 cluster, and that job exercises
  the events field selectors the portal relies on.

**Alternatives considered:**

- **1.36, the only version the event selectors were measured on.** Excludes clusters OPM itself
  supports.
- **No declared floor.** Every installer would guess.

**Rationale:** The portal reads what OPM writes, so it should run where OPM runs. A floor is only a
promise if something tests it, which is why R3 keeps a job there. The owner's answer also puts the
opm-operator on the same floor with its own CI job; that half is tracked in the roadmap, because
this record binds only the portal.

**Source:** Owner answer 2026-10-05 to OQ2 ("1.34 (Recommended)": the oldest version tested today;
CI keeps one job on the floor and event field selectors are checked there). OPM's tested range (1.34 to 1.36) and the
event selector measurement at 1.36 ([evidence 01](design/evidence/01-live-cluster-capture/),
observation 7).

## Open questions

Each question carries a status. An open one carries **Blocking:** `milestone 2` (must be answered
before the in-cluster release), `V2`, a named release or event it must precede, or `deferrable`.

### Access and identity

- **OQ1: Which identity model follows V1 in-cluster: SubjectAccessReview-then-read, impersonation,
  or token passthrough?** Status: open. Blocking: V2. D6 settles V1 on
  SubjectAccessReview-then-read. Impersonation (classic, then ConstrainedImpersonation once the
  floor is 1.36 or later) puts users in the audit log at the cost of an impersonate grant; token
  passthrough works only where the API server trusts the portal's issuer. Binding when V2 adds
  writes (see 0027:OQ26).
- **OQ2: What is the minimum Kubernetes version?** Status: resolved-by-D12 (owner answer
  2026-10-05: 1.34). The selector check at 1.34 is D12:R2, and the CI job that keeps it is D12:R3.
- **OQ6: Who may read the cluster-scoped Platform and TransformerRegistrations, and do the
  operator's viewer roles aggregate into `view`?** Status: resolved-by-D11 (owner answer
  2026-10-05: no aggregation; D11:R6).
- **OQ8: Does the operator's embedded kernel redact marked secret paths in condition and event
  messages?** Status: resolved-by-D8 (owner answer 2026-10-05: "Hide messages in-cluster"; the
  kernel was found not to redact; D8:R5). Local mode shows messages verbatim (D8:R6).
- **OQ20: Does change detection in the change stream leak a timing signal across access boundaries
  in-cluster?** Status: resolved-by-D2 (supervisor ruling 2026-10-05, pending the owner: compare each
  subscriber's rendered document with the last one sent to it and send only on a difference;
  D2:R7).
- **OQ21: How are background reads authorized in-cluster?** Status: resolved-by-D6 (owner answer
  2026-10-05: a SubjectAccessReview for the portal's own ServiceAccount; D6:R10).

### Status and history

- **OQ3: Should the operator report workload health, or does health stay a consumer concern?**
  Status: open. Blocking: deferrable. D3 computes health in the portal. An operator condition would
  let the CLI and other consumers share one answer; it would also be a health wait the operator has
  deliberately not built.
- **OQ9: Is a one-hour event feed enough, or should the portal or the operator persist events or
  extend status history?** Status: open. Blocking: deferrable. The operator keeps at most ten
  history entries, so a run of failed retries pushes every earlier entry out.
- **OQ19: Should the operator record render warnings durably in status, not only as events?**
  Status: open. Blocking: deferrable. D9:R5 labels the gap on the instance page.

### Graph and data the operator does not record

- **OQ4: What shape should the Platform's contract inventory take in its status?** Status: open.
  Blocking: deferrable. It would let the Platform view draw which catalog defines which contract and
  which provider fulfils it.
- **OQ5: Should each inventory entry record the transformer that produced it?** Status: open.
  Blocking: deferrable. It would give a transformer-to-object provenance edge.
- **OQ12: Should rendered objects and pod templates carry the instance uuid and namespace labels?**
  Status: open. Blocking: deferrable. V1 walks ownerReferences instead (D10), so this is a
  convenience for other consumers.
- **OQ16: Should the portal draw Flux Kustomization to ModuleInstance edges from Flux labels?**
  Status: open. Blocking: deferrable. Not observed on a live cluster; the capture had no Flux.
- **OQ18: How should provider-contract demand be recorded, and for which owners?** Status: open.
  Blocking: deferrable. `status.requiredContracts` lists every contract the render used, so it is
  not demand. Candidates: per-entry fulfilment, or a separate provider-contract list. Until one
  exists, D4:R3 holds.

### Scope of what is shown

- **OQ7: Should a later version show values, and how?** Status: open. Blocking: V2. Options: show
  them to users who may read Secrets in every namespace the inventory renders a Secret into, or have
  the kernel expose a secret-aware projection of values. Either needs the module's schema.
- **OQ11: Should CLI-owned instances be shown?** Status: resolved-by-D3 (shown read-only, with
  health from the CLI's inventory; D3:R6).

### Product and distribution

- **OQ10: When does the read API leave `v1alpha1`, and who may depend on it before then?** Status:
  open. Blocking: before the first release outside `0.x`, and before any adapter is published as
  depending on the API. In 0030 it blocked acceptance; with no acceptance stage left, it binds where
  the promise does. Adapters need to know what the additive-only promise of D2 is worth.
  Candidate: a declared stability point after which a removal needs a new version served beside the
  old one for at least one minor release.
- **OQ13: Is the portal published as an OPM module?** Status: open. Blocking: deferrable. V1 ships
  binaries, an image and an install manifest.
- **OQ14: Does opm-portal join the release cascade as a consumer of opm-operator, or stay a leaf?**
  Status: open. Blocking: deferrable. It starts as a leaf with hand-moved pins.
- **OQ15: Is a multi-cluster hub a goal, or never?** Status: open. Blocking: deferrable. V1 has no
  cluster parameter in its paths; a hub would add a parallel route set.
- **OQ17:** withdrawn: it asked about repo vocabulary outside this design.

## Risks

**Highest: a security failure in a shared, multi-user service.**

- **A shared cache leaks across tenants.** One handler that skips the per-user check shows one team
  another team's objects. *Mitigation:* D7 makes authorization the first step of every read and
  requires the same refusal for existing and missing objects; tests assert a principal without
  access gets zero items from every list.
- **Empty or forged identity reaches the cluster** (CVE-2026-23990 in the Flux Operator UI).
  *Mitigation:* D6 refuses an empty username before any Kubernetes call, strips every `system:`
  group, requires prefixes, and holds no impersonate grant.
- **Values leak through a side door**: the instance spec, the last-applied annotation, and operator
  messages. *Mitigation:* D8 strips the first two everywhere and hides messages in-cluster.

**Next: status that misleads.**

- **Health is wrong in a way the capture did not cover** (a stuck volume attach, a Pod with no
  node). *Mitigation:* health never claims more than kstatus plus D3:R2; unknown and progressing
  are shown as such, and the rule list can grow.
- **Users read "Applied" as "healthy".** *Mitigation:* D3 shows both axes everywhere; the docs
  explain the difference.
- **Operator status is stale or wrong** (issue 209's drift counter, contracts that are not demand,
  ten-entry history). *Mitigation:* D3:R7, D4:R3; OQ9 and OQ18 carry the gaps.

**Then: operational.**

- **The portal breaks on an operator upgrade.** *Mitigation:* the portal pins the operator version
  it reads and moves the pin by hand (OQ14), tested against a captured cluster for that version.
- **Many watched kinds on a provider-heavy cluster.** *Mitigation:* watches are label-selected and
  keep only metadata and status; a scale budget is follow-up work.
- **The project joins the archived dashboards.** *Mitigation:* D2 makes the read API the product,
  so adapters can outlive the UI.

**Drawbacks.** A new repo and release line; no writes and no values in V1; two status values where
users expect one; provider-defined kinds show as not readable; in milestone 2 users are invisible
in the API server's audit log, so the portal's access log is the only record of who read what.

**Alternatives not taken.**

- **A Headlamp plugin instead of a portal.** Plugins run only in the browser, so per-user filtering
  of a shared cache, server-side health and log bounds cannot live there. A plugin over this API
  stays a follow-up.
- **Embed the UI in the operator** (the Flux Operator model). The operator holds cluster-wide
  impersonate and write on ModuleInstances; a UI bug there has the operator's blast radius.
- **Backstage as the base.** A heavy platform with a shared server-side identity by default; it
  stays an adapter target.
- **Re-render modules in the portal.** A second render path that can disagree with the operator's.

## Operations

- **Observability.** RFC 9457 problem documents distinguish `forbidden` (the user's RBAC) from
  `not_readable_by_portal` (the portal's role). Items carry access states and health results say
  when they are partial or polled (`evaluatedAt`, `live`). In-cluster, one structured access-log
  line per authorized or denied read (D6:R7). Liveness, readiness and version endpoints sit outside
  the versioned API.
- **Versioning.** Releases start at `0.1.0` and stay `0.x` until the read API declares stability
  (OQ10). Leaving alpha adds a new path version served beside the old one.
- **Rollback.** The portal writes nothing and keeps no state beyond in-memory sessions: stop the
  binary, or delete the Deployment, its ServiceAccount, role and binding. The operator's viewer
  roles are unbound by default and harmless to leave.
- **Cross-repo ordering.** The operator's viewer roles (D11:R4) must be released before milestone
  2 can show a non-admin platform state; milestone 1 needs nothing from the operator. Any requires
  edge waits on OQ18, provenance on OQ5, a contract inventory on OQ4. The portal pins one operator
  version: the API module it imports and the operator its e2e tests install move together.

## Related enhancements

| Entry | Why |
| --- | --- |
| 0015 (archived) | Catalog contracts and transformer registration: the registration verdicts the Platform view reads, and the readiness rule behind "Applied" (D3, D4) |
| 0027 | Self-service kinds, which V2's writes build on; its OQ17 to OQ29 hold the marketplace contract questions |
| 0013 | Secrets, and the diagnostics redaction rule behind D8:R5 |
| 0030 (withdrawn 2026-10-05) | This design's former home |
| 0031 (withdrawn 2026-10-05) | The module presentation contract draft; its evidence informs V2 |
