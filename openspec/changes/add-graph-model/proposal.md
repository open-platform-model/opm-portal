## Why

The owner asked for "a generated DAG showing the relationships". The read model on `main` holds
every fact such a graph needs, but nothing turns it into nodes, edges and a layout yet, and the
read API (`add-read-api`) and the UI's graph pages cannot start without that. 0030:D4 fixes what a
graph may say: every edge kind has exactly one source of truth written by the operator or the API
server, a disagreement between two sources is shown rather than resolved, no "requires" edge is
drawn because `status.requiredContracts` lists every contract the render used, registrations show
acceptance and activation apart, and configuration-only components are grouped by default. The
live capture measured cert-manager's ungrouped graph at 86 nodes, which is unreadable; grouping
and a node cap are part of the model, not of a renderer.

## What Changes

- New package `internal/graph`, a pure function of read-model views (Principle II):
  - **Node kinds**: platform, catalog, registration, instance, package, module, source,
    component, object, runtime child and group (configuration components, Pods, the over-cap
    summary). Ids are `<prefix>:<parts>` with percent-encoded parts and no UID, so they survive
    a restart and a delete-and-recreate (0030:D4:R6).
  - **Edge kinds**, each with one named source: Platform `status.registry` (platform to
    catalog); the registry entries a registration contributed, joined on its `spec.catalog`
    (registration to catalog); `spec.providerRef`, drawn verified only when the provider's
    inventory holds the registration and marked unverified otherwise (registration to provider
    instance, 0030:D4:R2); `spec.module` (instance to module); `spec.sourceRef` and
    `spec.dependsOn` (package to source and to package); `status.inventory` (owner to component,
    component to object); controller `ownerReferences` below inventory objects only (object to
    runtime child). No edge from an instance to a contract (0030:D4:R3); render contracts are a
    text field on the instance node.
  - Per-node applied state, health, access and registration standing, copied from the views
    `internal/health` already filled.
  - **Collapse rules**: configuration-only components grouped into one node (0030:D4:R5),
    ReplicaSets scaled to zero hidden behind a count on their parent, more than five Pods under
    one parent grouped, and a node cap that replaces what it drops with one summary node.
  - **Layout data**: a column per node kind, barycenter ordering with a stable tie-break, x/y
    and an edge route per edge. Two scopes: an instance (or package) graph and the platform graph.
- `internal/readmodel`: an inventory object carries the runtime children below it (controller
  ownerReferences, with each child's health and a ReplicaSet's replica count), and a package
  item its `spec.dependsOn`. The F1 test harness moves to `internal/readmodel/readmodeltest` so
  the graph's golden tests build views through a real `Model`.
- Golden JSON and golden SVG for the F1 fixtures (cert-manager, podinfo broken, the CLI-owned
  web, backup provider and consumer, the platform with its accepted and refused claims), written
  by a test-only SVG writer so a reviewer can look at each layout. Build time and node counts
  for cert-manager are recorded in design.md.

## Capabilities

### New Capabilities

- `graph-model`: the nodes, edges, ids, collapse rules and layout of the instance and platform
  graphs, and the source each edge is drawn from.

### Modified Capabilities

- `read-model`: instance views carry the runtime children below each inventory object, and
  package views their dependencies.

## Impact

- Packages: new `internal/graph`; `internal/readmodel` gains two view fields and a test-support
  package. No API resource, UI page or binary behaviour changes; the API change maps the graph
  onto wire types.
- Dependencies: none new (Principle VII). The layout is a fixed column per kind plus barycenter
  sweeps, so no layout library is needed.
- Principle V: no new read. Runtime children were already read (tier 3) for the Pod rule; the
  view now keeps what it read. The graph reads nothing itself.
- SemVer: MINOR after 1.0 (new internal capability). Nothing user-visible, so the PR title is
  `chore(graph)` and the 0.x line cuts no release.
- Enhancement link: lands 0030:D4 at the graph layer (R1, R2, R5, R6 here; R3, R4 and R7 need
  the pages that show the fields); no decision completes, so `enhancement.yaml` claims none.

Not in this change: wire types and the `/graph` resources (`add-read-api`), the SVG renderer,
pan and zoom and expansion clicks (`add-htmx-ui`), requires edges (0030:D4:R3 defers them to an
operator field), and a graph for consumers of provider contracts.
