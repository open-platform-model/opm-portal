## Context

See proposal.md for why. `internal/authz` (the sealed `Grant`, `Checker`, the local
SelfSubjectAccessReview backend) and `internal/health` (kstatus plus the Pod rule, roll-ups, the
Applied axis, registration verdicts) are on `main`. Nothing reads the cluster yet. This change
adds the layer that does, and that every later view (graph, read API, stream, UI) reads from.

Evidence used throughout:

- The F1 capture in `testdata/clusters/f1/` (kind, Kubernetes v1.36.1, opm-operator
  v1.0.0-beta.6): five ModuleInstances (one CLI-owned), one ModulePackage without Flux, the
  Platform, an accepted and a deliberately refused TransformerRegistration, 59 inventory objects
  and their ReplicaSets and Pods, and `events.k8s.io/v1` events.
- The ApplyFailed instance from enhancement 0030 experiment 01, already in
  `internal/health/testdata/mi-apply-failed.yaml`; F1 holds no failed apply.
- The experiment 01 report: per-request graph reads cost 7.4 to 9.4 s at client-go's default
  QPS 5 (observation 12); ReplicaSets and Pods carry `module-instance.opmodel.dev/name` but not
  the uuid label, while every inventory object carries the uuid label (observation 8);
  `regarding.*` field selectors work server-side on 1.36 (observation 7); Platform and
  registration events land in namespace `default` (observation 5); kubelet events have
  `eventTime: null` and count through `deprecatedCount` (observation 4).
- A spike in this worktree (section 1 keeps it as tests): client-go's fake dynamic client
  applies label selectors on list, a dynamic informer's transform runs before the store, and a
  deferred discovery RESTMapper over `memory.NewMemCacheClient` resolves kinds from a fake
  discovery client.

## Goals / Non-Goals

**Goals:**

- The four tiers of the portal design (T1 OPM kinds, T2 inventory kinds, T3 runtime children,
  T4 events) plus the polling fallback, behind one type, `readmodel.Model`.
- Every public read takes the caller's identity and a grant and checks it before reading; every
  read inside a view is authorized for the caller on its own (0030:D7:R2/R3).
- Stripping per 0030:D8 at the point objects enter memory, so no later layer can leak them.
- Views shaped for the portal, built from held state (0030:D3:R9).
- Latency and memory on the F1 set measured and recorded here.

**Non-Goals:**

- Wire types and JSON names: `add-read-api` owns `api/v1alpha1` and maps these Go values onto
  them, as `add-health-evaluation` already left its values to it.
- Change notification to subscribers (the broker), and the events watch while a feed is open:
  `add-stream-broker`. This change keeps a hook point (the informers' handlers) and no more.
- The graph and its node ids: `add-graph-model`.
- The YAML view of an object and the not-in-inventory refusal (0030:D7:R4): `add-read-api`.
- The in-cluster reader. Milestone 2 reads as the portal's ServiceAccount after a
  SubjectAccessReview for the user; how the read model's own background reads are authorized
  there is that milestone's decision (Open Questions).

## Research & Decisions

### Informer library: client-go dynamic informers

**Context**: T1 and T2 both need shared informers with a transform; T2 needs one informer per
kind, started and stopped at run time; inventory kinds are arbitrary, so T2 is unstructured
anyway.
**Explored**: controller-runtime's cache (the operator's choice) and client-go's
`dynamicinformer`; the spike above.
**Options considered**:
1. controller-runtime `cache.Cache` - typed or unstructured informers, `DefaultTransform`, label
   selectors per object, `RemoveInformer` since 0.15. Adds controller-runtime and its
   dependencies to a module that has none of them, and its per-object selector configuration is
   fixed at cache construction, so a lazily added kind with its own selector needs a second
   cache per kind anyway.
2. client-go `dynamicinformer.NewFilteredDynamicSharedInformerFactory` per tier and scope -
   already in the module, one informer per (resource, namespace) with its own stop channel,
   `SetTransform`, and a fake dynamic client that serves list and watch for tests.
**Decision**: Option 2, for all four OPM kinds as well as inventory kinds.
**Rationale**: No new dependency (Principle VII), one code path for T1 and T2, and stopping one
kind is closing one channel. `internal/health` already takes `unstructured.Unstructured`, so the
operator's typed API is not needed either.

### Authorization: who reads, and whose grant covers it

**Context**: 0030:D7 wants every read authorized before lookup; 0030:D5:R7 wants every read in
local mode preceded by an allowed SelfSubjectAccessReview for the exact attributes. The read
model reads twice over: its informers list and watch for everyone, and each caller is served from
that shared state.
**Decision**:

- A **caller read** is a public method. It takes `who authz.Identity` and `g authz.Grant` and
  calls `g.Covers(who, attrs)` for its own read first. The method signatures are the
  enforcement: there is no read method without a `Grant` parameter, and a test walks the
  package's exported methods to keep it so.

  | Method | Read the grant must cover |
  | --- | --- |
  | `ListInstances(ctx, who, g, ns)` | `list opmodel.dev/v1alpha1 moduleinstances` in `ns` (`""` = all) |
  | `Instance(ctx, who, g, ns, name)` | `get ... moduleinstances` `ns/name` |
  | `ListPackages`, `Package` | the same on `modulepackages` |
  | `Platform(ctx, who, g)` | `get ... platforms` `cluster` |
  | `Events(ctx, who, g, about)` | `list events.k8s.io/v1 events` in the event namespace |
  | `HoldChildren(ctx, who, g, ns)` | `watch v1 pods` in `ns` |

- **Reads inside a view** (each inventory entry, the registrations on the platform view, the
  runtime children) are authorized for the same caller through the model's `authz.Authorizer`:
  first `get` with an empty name in the object's namespace, which covers every object there;
  only if that is denied, `get` on the exact name, because RBAC may grant single names. A
  denial marks the item `forbidden`; an `unavailable` denial marks it `notReadable`. Decisions
  are cached by the authorizer for 30 s, so a warm view costs no review.
- A **reader read** is an informer's list and watch, or a poll. It runs as the reader identity
  (the kubeconfig's in local mode, the same identity the `Checker` serves) and is started only
  after `Check(reader, list ...)` and `Check(reader, watch ...)` both allow it, at the scope it
  will use. A poll needs `Check(reader, get ...)`.
- Before any informer or poll starts, the resource is refused if it is core `secrets`, in
  addition to the authorizer's own refusal.

### Tier 1: the OPM kinds

One informer per kind and scope: cluster-wide when `Config.Namespaces` is empty, else one per
listed namespace (0030:D5:R5); Platform and TransformerRegistration are cluster-scoped and always
cluster-wide. `Start` checks the reader's grants and starts what it may; a kind whose grant is
denied, or whose list fails, is recorded **unavailable**, and every read of it returns
`ErrUnavailable` instead of an empty answer (Principle IV). `Start` waits for the started
informers to sync, bounded by `Config.SyncTimeout` (default 10 s).

GVRs are fixed: `opmodel.dev/v1alpha1` `moduleinstances`, `modulepackages`, `platforms`,
`transformerregistrations`.

### Tier 2: inventory kinds, on demand

- An entry's kind is resolved to a resource through `restmapper.NewDeferredDiscoveryRESTMapper`
  over `memory.NewMemCacheClient` (discovery fetched once; on a no-match the cache is reset and
  the lookup retried once, so a CRD added later resolves). An unresolvable kind marks its entries
  `notReadable`.
- `acquire(gvr, namespaces)` starts the kind's informer on the first view that needs it and
  records the time of every use. Scope, in order: cluster-wide if the reader may list and watch
  cluster-wide; else one informer per namespace the entries live in that the reader may list and
  watch; else the **poller** for the objects the reader may get; else the entries are
  `notReadable`. Every informer is label-selected on `module-instance.opmodel.dev/uuid` (exists).
- A janitor stops a kind's informers (and poller) once no view has used them for
  `Config.IdleTimeout` (default 5 min).
- `acquire` starts informers without waiting; a view starts every kind it needs, then waits once
  for all of them, bounded by the request context and `SyncTimeout`, polling every 5 ms
  (client-go's own wait polls every 100 ms, which a cold cert-manager read paid eleven times in a
  first cut). Entries whose informer has not synced are `notReadable`, never `Missing`.
- A synced informer that does not hold an entry's object means the object is absent: the entry
  is `Missing` (read, not found).

### Polling fallback

A poller per kind, holding the entries views asked for, GETs each one every `PollInterval`
(default 30 s) with at most `PollWorkers` (default 8) in flight. Results carry `EvaluatedAt` and
`Live: false`, which `health.Summary` already rolls up. A view that finds an entry the poller has
never read reads it once synchronously. A 404 is `Missing`, a 403 `forbidden` for the reader
(shown `notReadable`), anything else `notReadable`.

### Tier 3: runtime children

- `HoldChildren(ctx, who, g, ns)` returns a release function. While its refcount is above zero
  the namespace has informers on `apps/v1 replicasets`, `v1 pods` and `batch/v1 jobs`, selected
  on `module-instance.opmodel.dev/name` (exists): the capture found that label, not the uuid, on
  ReplicaSets and Pods. The last release stops them.
- Without interest, a view lists the three kinds in the namespace on demand with the same
  selector, cached for `ChildrenTTL` (default 10 s) so an instance list over one namespace lists
  once. The caller must be allowed to list each kind in the namespace; otherwise
  `ChildrenAccess` is not ok and `health.Evaluate` marks the workloads `ChildrenUnread`.

### Tier 4: events

`Events(ctx, who, g, about)` lists `events.k8s.io/v1` events in the object's namespace, or in
`default` for a cluster-scoped object (observation 5; exported as `EventNamespace`, so the read
API knows which namespace the caller's list grant must name), with the field selector
`regarding.kind=K,regarding.name=N[,regarding.namespace=NS]`, and filters the result on the same
fields client-side (a fake client ignores field selectors; a real one has filtered already).
Lines are folded on (regarding uid, type, reason, note): the count sums each event's
`series.count`, else `deprecatedCount`, else 1; the time is the latest of `series.lastObservedTime`,
`eventTime`, `deprecatedLastTimestamp` and `metadata.creationTimestamp` (0030:D9:R3). Lines are
returned newest first. No cache: the feed is read when a page opens it, and the stream broker
adds the watch.

### Stripping

One transform, set on every informer and applied to every polled or listed object before it is
kept or returned:

- always: delete `metadata.managedFields` and the last-applied annotation;
- `opmodel.dev` ModuleInstance and ModulePackage: delete `spec.values`;
- T2 kinds, to bound memory without changing health: delete `spec.template` on Deployments,
  StatefulSets, DaemonSets, ReplicaSets and Jobs, `spec.jobTemplate` on CronJobs,
  `spec.versions[].schema` on CustomResourceDefinitions, and `data` and `binaryData` on
  ConfigMaps. A table test runs `health.Object` on every F1 object before and after the
  transform and requires identical results.

### Views

Go values in `internal/readmodel`, reusing `health`'s value types where they fit:

```go
type ObjectRef struct{ Group, Version, Kind, Namespace, Name string }

type InstanceItem struct {
    Ref            ObjectRef
    Module         ModuleRef      // spec.module.path, .version
    Owner          Owner          // OwnerOperator | OwnerCLI
    Applied        health.Applied // the operator's axis
    Health         health.Summary // the portal's axis
    InventoryCount int
    LastAppliedAt  time.Time
}

type InstanceDetail struct {
    InstanceItem
    ServiceAccountName string
    Conditions         []health.Condition
    History            []HistoryEntry  // action, phase, sequence, times, digests, message
    LastApplied        Digests         // source, config, render
    RenderContracts    []string        // status.requiredContracts: the render's, not demand
    Components         []Component     // inventory grouped by component, in inventory order
}

type Component struct {
    Name    string
    Health  health.Summary
    Objects []InventoryObject // Ref, Access, Health, ChildrenUnread, EvaluatedAt, Live
}

type PackageItem / PackageDetail // the same shape on ModulePackage, with SourceRef and Path

type PlatformView struct {
    Name, Type, OperatorVersion string
    Applied             health.Applied
    Conditions          []health.Condition
    Subscriptions       []Subscription      // spec.registry: catalog, version, enable
    Catalogs            []Catalog           // status.registry joined with registrations
    Registrations       []RegistrationView
    RegistrationsAccess health.Access
}

type RegistrationView struct {
    Name, Catalog, Version string
    Provides               []string
    Provider               ObjectRef        // spec.providerRef
    Standing               health.Registration
    Applied                health.Applied
}

type Event struct {
    Type, Reason, Note, ReportingController string
    Regarding ObjectRef; FieldPath string
    Count     int
    LastSeen  time.Time
}
```

No view carries an `unstructured.Unstructured`, a raw status, `spec.values` or an annotation map.
`Owner` is `cli` when `spec.owner` is `cli`. An instance with no `status.inventory` (the
ApplyFailed sample) has no components and a health of Unknown with nothing counted, not partial.

Errors: `ErrNotCovered` (wraps `authz.ErrNoGrant`), `ErrNotFound`, `ErrUnavailable`; none names
the object or identity.

### Client tuning

`TuneConfig(cfg *rest.Config)` sets QPS 50 and burst 100 when the config carries client-go's
defaults (zero), and leaves explicit values alone. The capture's 7-9 s came from QPS 5 over ~45
GETs; with T2 the steady state makes no per-entry reads, and a cold instance read makes one list
per kind (11 for cert-manager), so burst 100 covers a cold start without throttling.

### Measurements on F1

Two tests record them. `TestMeasureF1` runs on every `task test` against the fake dynamic client
fed from `testdata/clusters/f1`, so it measures the portal's own work with no network.
`TestMeasureLive` runs the same reads against the kind fixture cluster when
`OPM_PORTAL_MEASURE_KUBECONFIG` names its kubeconfig; it was run on 2026-10-04 on a fresh
`task e2e:up` cluster (podman kind, Kubernetes v1.36.1, opm-operator v1.0.0-beta.6, the F1
fixtures), as the kubeconfig's own identity (`kubernetes-admin`), reading through the local
SelfSubjectAccessReview `Checker`. Each figure is the range over two or three runs.

| Measure | Fake cluster (F1) | Live, client-go defaults (QPS 5, burst 10) | Live, `TuneConfig` (QPS 50, burst 100) |
| --- | --- | --- | --- |
| `Start` (four OPM kinds listed and synced) | 43-55 ms | 11-13 ms | 11-13 ms |
| `Instance` cert-manager, cold (42 entries, 11 kinds) | 6.4-7.7 ms | **7.0 s** | **43-57 ms** |
| cluster requests on that cold read | 25 (11 lists, 11 watches, 3 child lists) | 16 reads + 36 access reviews | 16 reads + 36 access reviews |
| `Instance` cert-manager, warm | 0.34-0.41 ms | 0.48-0.65 ms | 0.45-0.52 ms |
| cluster requests on a warm read | 0 | 0 reads, 0 reviews (20 calls) | 0 reads, 0 reviews (20 calls) |
| `ListInstances` (5 instances), first after cert-manager | 12 ms | 2.8 s | 33-35 ms |
| `ListInstances`, warm | 0.40-0.48 ms | n/a | n/a |
| heap held by a warm model over F1 | 1.0-1.1 MiB | not measured | not measured |

What the numbers say:

- The capture's 7.4 to 9.4 s per graph (0030:D3:R9) is reproduced at client-go's default
  rate limit even with the tiers in place: a cold read sends 52 requests, and most of them are
  access reviews (two per kind for the reader, one per kind and namespace for the caller, three
  for the children). The rate limit, not the reads, is the cost. `TuneConfig` brings the cold read
  to tens of milliseconds, and the wiring change (`add-local-mode`) MUST apply it to both the
  dynamic client and the review client.
- A warm read sends nothing: the inventory comes from the informers and every decision from the
  authorizer's 30 s cache. After 30 s the decisions are asked again (about 36 reviews for
  cert-manager), which at the tuned rate is a few tens of milliseconds.
- Memory over F1 is about 1 MiB. The F1 capture has its CRD schemas removed already; live CRDs
  carry them, and the strip transform drops them before they are stored, so the live figure for
  cert-manager's six CRDs is expected in the same range. A budget at scale is later work.

## Risks / Trade-offs

- [Grant per view is only as good as the methods that check it] -> a test lists every exported
  method of `Model` whose name is a read and requires an `authz.Grant` parameter, and each
  method's tests include the not-covered case.
- [Per-namespace `get` decision covers every object in the namespace] -> it is the RBAC answer
  for an empty name, which is what `Grant.Covers` already accepts; a name-restricted role falls
  through to the exact check.
- [Label-selected informers miss inventory objects without the uuid label] -> such an object
  shows as Missing. Every inventory object in both captures carries it; the CLI-owned instance's
  two objects included (checked in section 3's tests).
- [Fake clients ignore field selectors and watch label filtering] -> the read model filters
  events client-side as well, and T2 tests add objects before the informer starts.
- [An events list error carries the API server's text] -> it is returned only after the caller's
  grant was checked and holds no credential; the read API logs it and sends the client a
  problem document without it, as it does with `DenialError`'s cause.
- [Memory] -> the numbers in Measurements on F1 are for F1 only; a budget at scale is later work.

## Open Questions

- Milestone 2: the read model's reader reads (informers, polls) need a grant for the reader
  identity. Locally that is the kubeconfig's identity and the self review answers for it.
  In-cluster the reader is the portal's ServiceAccount and the only review allowed is
  `subjectaccessreviews` (0030:D6:R9); whether the portal reviews its own ServiceAccount that
  way or trusts its role is for `add-sar-authorizer` to decide. No owner input is needed now.
