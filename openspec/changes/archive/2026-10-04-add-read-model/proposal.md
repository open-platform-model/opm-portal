## Why

Every portal view (the read API, the graph, the change stream, the UI) needs the same thing
underneath: the four OPM kinds, the objects their inventories name, the runtime children below
those objects and their events, held in memory so a page does not cost one Kubernetes read per
inventory object. The live capture measured 7.4 to 9.4 s for one cert-manager graph built from
per-request reads at client-go's default rate limit (0030:D3:R9), and found that a broken rollout
shows only on the Pods below a Deployment (0030:D3:R2). The authorization seam (`internal/authz`)
and the health derivation (`internal/health`) are on `main`; the read model is the layer that
joins them to the cluster, and the read API (`add-read-api`) cannot start without it.

## What Changes

- New package `internal/readmodel`, the cache, watches, on-demand reads and joins of Principle II:
  - **T1**: shared informers on ModuleInstance, ModulePackage, Platform and
    TransformerRegistration, cluster-wide or per configured namespace, started at `Start` and
    kept for the process.
  - **T2**: one label-selected informer per inventory kind, started on the first read that needs
    it and stopped when no read has used it for an idle period. Secrets are never informed.
    Kinds are resolved through a RESTMapper over cached discovery.
  - **T3**: informers on ReplicaSets, Pods and Jobs in a namespace only while a caller holds
    interest in it; otherwise a bounded on-demand list.
  - **T4**: `events.k8s.io/v1` events listed on demand with `regarding.*` field selectors, in
    namespace `default` for the cluster-scoped Platform and TransformerRegistrations, with
    repeats folded into one line with a count.
  - A **30 s polling fallback** for inventory objects the reader may get but not list and watch,
    marked not live, with the time each was last evaluated.
- Every object the read model stores loses `metadata.managedFields` and the
  `kubectl.kubernetes.io/last-applied-configuration` annotation; ModuleInstances and
  ModulePackages lose `spec.values` (0030:D8).
- Portal-shaped views (Go values, not wire types): instance list item, instance detail with its
  inventory grouped by component and both status axes, platform view with its catalogs and
  registrations, package view, and the event line. None carries a raw custom-resource status,
  `spec.values` or the last-applied annotation.
- Every public read method takes the caller's `authz.Identity` and an `authz.Grant` and calls
  `Grant.Covers` before it reads; reads inside a view (inventory objects, registrations,
  children) are authorized one by one through the same `Authorizer`, and an item the caller
  may not read is marked forbidden instead of failing the view (0030:D7:R2/R3).
- A helper that raises client-go's QPS and burst for the reading client.
- Tests against fake dynamic clients fed from the F1 capture, plus the captured ApplyFailed
  instance; latency and memory measured on the F1 set and recorded in design.md.

## Capabilities

### New Capabilities

- `read-model`: what the portal holds about a cluster, how it keeps it fresh, what it strips
  before holding it, and how each view is authorized and degraded per item.

### Modified Capabilities

None.

## Impact

- Packages: new `internal/readmodel`, which calls `internal/authz` (grants) and
  `internal/health` (pure derivations). No API resource, UI page or binary behaviour changes;
  `cmd/opm-portal` does not wire it yet (`add-local-mode` does).
- Dependencies: none new. client-go's dynamic informers, discovery and `restmapper` are in the
  module already (Principle VII; design.md says why not controller-runtime).
- Principle V: the read model reads the four OPM kinds, the non-Secret kinds inventories name,
  ReplicaSets, Pods, Jobs and events, with `get`, `list` and `watch` only, each after an allowed
  `Check`. It never reads Secrets (refused at `Check` and again before any informer starts), never
  writes, and holds no `spec.values`. Reading as the kubeconfig's identity is unchanged from
  milestone 1.
- SemVer: MINOR after 1.0 (new internal capability). Nothing user-visible, so the PR title is
  `chore(readmodel)` and the 0.x line cuts no release.
- Enhancement link: contributes to 0030:D3:R9, 0030:D7:R2/R3, 0030:D8:R1/R2/R3 and 0030:D9:R3/R4
  at the read-model layer; no decision completes here, so `enhancement.yaml` claims none.

Not in this change: wire types and JSON names (`add-read-api`), the graph (`add-graph-model`),
change streams and the T4 watch (`add-stream-broker`), YAML views, the not-in-inventory refusal
(0030:D7:R4), wiring into the binary and resolving the kubeconfig identity (`add-local-mode`),
and the in-cluster reader identity (milestone 2).
