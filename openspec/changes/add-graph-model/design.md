## Context

See proposal.md for why. `internal/readmodel` (views built from held state, every read covered by
a grant) and `internal/health` (object health, roll-ups, applied state, registration verdicts) are
on `main`. This change derives graphs from those views and nothing else.

Evidence used throughout:

- The F1 capture in `testdata/clusters/f1/` (opm-operator v1.0.0-beta.6): cert-manager with 42
  inventory entries in 20 components and three Deployments, podinfo with two replicas, the
  CLI-owned `web/web`, `default/backup-provider` whose inventory holds the accepted claim
  `default.backup-provider`, `default/backup-consumer` (one ConfigMap), the package `pkg/podinfo`
  stuck at `SourceNotReady`, and the deliberately refused claim `default.refused-claim-fixture`
  whose `spec.providerRef` names an instance that does not exist and whose `spec.catalog` is in
  no registry.
- The image-break samples from enhancement 0030 experiment 01, already in
  `internal/health/testdata/` (`mi-podinfo-image-broken.yaml`,
  `objects-podinfo-phase4-broken-1min.yaml`): two ReplicaSets, one Pod waiting on
  `ErrImagePull`. F1 holds no broken rollout.
- The experiment 01 report, observations 2 and 13: `status.requiredContracts` lists 15 contracts
  for cert-manager and 7 for podinfo, nearly all fulfilled by the catalog; the ungrouped
  cert-manager graph had 86 nodes and 85 edges, 20 of them components, most holding one RBAC or
  configuration object.
- opm-operator source: `spec.sourceRef.namespace` defaults to the package's namespace
  (`internal/controller/modulepackage_controller.go:241`); `spec.dependsOn` entries are
  same-namespace only and a foreign namespace is refused (`api/v1alpha1/modulepackage_types.go:50-56`,
  `internal/reconcile/modulepackage.go:588`).

## Goals / Non-Goals

**Goals:**

- Instance, package and platform graphs as pure functions of read-model views.
- One named source per edge kind; a disagreement between two sources shown, not resolved.
- Ids stable across restarts and across a delete and recreate (0030:D4:R6).
- Collapse rules that keep cert-manager readable by default, with expansion by group id.
- Layout data deterministic to the byte, so golden JSON and SVG tests are stable.

**Non-Goals:**

- Wire types, JSON names on `/api/v1alpha1` and the `/graph` resources: `add-read-api` maps
  `graph.Graph` onto them.
- The SVG the UI serves, CSS, pan and zoom, and expansion clicks: `add-htmx-ui`. This change's
  SVG writer lives in a test file and exists so a reviewer can look at a golden layout.
- Assembling the platform graph's provider lookups from the read model (which grants to ask
  for): the API handler does it, because grants are its job (0030:D7).
- Requires edges, and a column of consumer instances: 0030:D4:R3 defers them until the operator
  records provider demand.

## Decisions

### Package shape

```go
package graph

func Instance(d readmodel.InstanceDetail, opts Options) Graph
func Package(d readmodel.PackageDetail, opts Options) Graph
func Platform(in PlatformInput, opts Options) Graph

type PlatformInput struct {
	Platform  readmodel.PlatformView
	Providers []ProviderLookup // one per distinct spec.providerRef
}

type ProviderLookup struct {
	Ref      readmodel.ObjectRef
	Access   health.Access             // how the read went
	Instance *readmodel.InstanceDetail // nil with AccessOK: read and not found
}

type Options struct {
	Expand         []string // group ids to show expanded
	ShowScaledDown bool     // show ReplicaSets scaled to zero
	NodeCap        int      // 0 means DefaultNodeCap (150)
}
```

The graph package imports `readmodel` and `health` and is imported by nothing yet. It reads no
cluster state: every value comes from the views, which were authorized for the caller.

### Ids

`<prefix>:<part>[/<part>...]`, each part escaped with `url.PathEscape` (which escapes `/`) plus `:`
and `@`, so
a part holding a catalog path stays one part. An empty part (core group, cluster scope, empty
component) is `_`, and a literal `_` is written `%5F`. No id holds a UID or a version that
changes on upgrade.

| Kind | Id |
| --- | --- |
| platform | `plat:<name>` |
| catalog | `cat:<catalog path with major>` |
| registration | `treg:<name>` |
| instance | `mi:<ns>/<name>` |
| package | `mp:<ns>/<name>` |
| module | `mod:<path>/<version>` |
| source | `src:<group>/<kind>/<ns>/<name>` |
| component | `comp:<mi or mp>/<ns>/<name>/<component>` |
| object, runtime child | `obj:<group>/<kind>/<ns>/<name>` |
| configuration group | `grp:configuration/<mi or mp>/<ns>/<name>` |
| Pod group | `grp:pods/obj/<parent object parts>` |
| over-cap summary | `grp:more/<root prefix>/<root parts>` |

### Edge kinds and their single source

| Edge | From -> To | Source |
| --- | --- | --- |
| `resolves` | platform -> catalog | Platform `status.registry` |
| `contributes` | registration -> catalog | Platform `status.registry` entry with source `Registration`, joined on the registration's `spec.catalog` |
| `providedBy` | registration -> instance | registration `spec.providerRef`, verified against the provider's `status.inventory` |
| `instantiates` | instance -> module | ModuleInstance `spec.module` |
| `sourcedFrom` | package -> source | ModulePackage `spec.sourceRef` |
| `dependsOn` | package -> package | ModulePackage `spec.dependsOn` |
| `hasComponent` | instance or package -> component or configuration group | `status.inventory` entries' `component` |
| `owns` | component -> object | `status.inventory` entries |
| `controls` | object -> runtime child or Pod group | controller `metadata.ownerReferences` below an inventory object |

Each edge carries its source string, so the API can show where an edge came from (0030:D4:R1).

### Collapse rules

1. Configuration components (no inventory entry of a workload kind: Deployment, StatefulSet,
   DaemonSet, ReplicaSet, Job, CronJob, Pod) are grouped into one node with their objects
   hidden when an owner has two or more of them. The group's health is the worst of its
   components', partial when any is; its counts are their sum. `Options.Expand` holding the
   group's id shows them.
2. A ReplicaSet with `spec.replicas: 0` and its descendants are hidden; the parent carries
   `hiddenScaledDown`. `Options.ShowScaledDown` shows them.
3. More than five Pods under one parent are one Pod group with counts by health state.
   `Options.Expand` holding the group's id shows them.
4. With more than `NodeCap` nodes after rules 1 to 3, nodes are dropped from the last column
   back, within a column from the end of the id order, until `NodeCap - 1` remain; one summary
   node in the last kept column counts them by kind. Edges to dropped nodes are dropped.

### Layout

Columns are fixed per scope and kind; an empty column takes no space.

- Instance scope: module | instance | component, configuration group | object | first runtime
  level (ReplicaSet, Job, or a Pod directly below a StatefulSet or DaemonSet) | second runtime
  level (Pods below a ReplicaSet or Job).
- Package scope: source, dependency packages | package | then as the instance scope.
- Platform scope: platform | catalog | registration | instance.

Ordering: the first column by id; each later column by the barycenter of its neighbours' rows in
the column before (a node without one goes last), ties by id; then one upward sweep, each column
by the barycenter of its neighbours' rows in the column after, ties by the current row. Integer
coordinates: node 200 x 44, column pitch 264, row pitch 56, columns centred vertically on the
tallest. An edge route is a cubic Bézier from the right middle of the left node to the left middle
of the right node, control points at the horizontal midpoint.

## Research & Decisions

### Registration to catalog: spec or registry

**Context**: The task brief says the edge comes from the registration's spec; 0030:D4 says from
"the registry entries a registration contributed". F1's refused claim names a catalog that is in
no registry.
**Options considered**:
1. Draw from `spec.catalog` alone - draws the refused claim into a catalog node that does not
   exist on the platform, implying a contribution the operator refused.
2. Draw only where `status.registry` holds an entry with source `Registration` for that catalog -
   the platform's resolved record decides, `spec.catalog` is the join key.
**Decision**: Option 2; the registration node keeps `spec.catalog` as text either way.
**Rationale**: 0030 wins over the brief, and an edge the record does not support is worse than
none (0030:D4 rationale).

### Provider cross-check

**Context**: 0030:D4:R2 wants the disagreement shown when the reference and the provider's
inventory disagree. In F1 the refused claim's provider does not exist.
**Decision**: The `providedBy` edge is always drawn and carries `verified`. It is verified only
when the provider instance was read and its inventory holds a TransformerRegistration of that
name; otherwise `reason` says why: `ProviderNotFound`, `ProviderUnreadable` (forbidden or not
readable, never guessed) or `NotInProviderInventory`. A provider node that could not be read is
still drawn, with its access, so the edge has an end.

### Runtime children in the view

**Context**: The read model already reads ReplicaSets, Pods and Jobs for the Pod rule but hands
`health.Evaluate` the objects and keeps only the result.
**Options considered**:
1. The graph lists children itself - a second read path, and the graph would need the Model.
2. The instance view keeps, per inventory object, the children its controller chain reaches,
   each with its direct parent, health and replica count.
**Decision**: Option 2. Children are attached only when the children read succeeded; a child
whose chain reaches no inventory object is not shown.

### Golden data for the broken rollout

**Context**: The brief asks for "podinfo broken" from F1; F1's podinfo is healthy.
**Decision**: The golden replaces F1's `default/podinfo` instance and its Deployment, Service,
ReplicaSet and Pods with experiment 01's image-break samples, through the same `Model`. Both are
live captures; nothing is hand-written.

### Test harness location

**Context**: The graph's goldens need views built by a real `Model` over the F1 fake cluster, and
the harness lived in `readmodel`'s own test files.
**Decision**: Move the cluster-side harness (kinds, loaders, fake dynamic and discovery clients,
the rule-answered local checker) to `internal/readmodel/readmodeltest`, which does not import
`readmodel`, so both packages' tests use it.

## Risks / Trade-offs

- [Barycenter sweeps leave crossings] → acceptable at the sizes collapse leaves; the graph JSON
  is there for a client-side layout later.
- [The cap drops runtime children first] → the deepest, most numerous nodes go first; the
  summary node says how many of each kind are hidden.
- [Default cap of 150] → about half the size at which the spike's SVG stopped being readable;
  revisit with the UI.

## Measurements on F1

Recorded by `TestMeasureCertManager` (section 3).
