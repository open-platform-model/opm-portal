## 1. Foundation: clients, kinds, stripping and the F1 harness (internal/readmodel)

- [x] 1.1 Add `Config`, `New`, `TuneConfig` (QPS 50, burst 100 over client-go's zero defaults), the cached RESTMapper (deferred discovery over the memory cache, one reset and retry on a no-match) and the errors `ErrNotCovered`, `ErrNotFound`, `ErrUnavailable`; verify `go build ./...`
- [x] 1.2 Add the strip transform (managedFields, the last-applied annotation, `spec.values` on ModuleInstance and ModulePackage, the T2 per-kind drop list); verify with a test that runs `health.Object` on every F1 object before and after and requires identical results, and that an instance carrying values, managed fields and the annotation leaves none of them
- [x] 1.3 Add the test harness: an F1 loader into a fake dynamic client with list kinds, a fake discovery client for the F1 kinds, and a local `authz.Checker` over a fake clientset whose review answers come from a rule table; verify the spike's findings as tests (label-selected list, transform before store, mapper resolution and reset)
- [x] 1.4 `task check` green, then commit `chore(readmodel): add the read model foundation and the F1 test harness`

## 2. Tier 1 and the OPM-kind views (internal/readmodel)

- [x] 2.1 Add `Start` and `Stop`: one informer per OPM kind and scope (cluster-wide, or per configured namespace for the namespaced kinds), each started only after the reader's `list` and `watch` grants, a kind denied or failing recorded unavailable; verify a denied ModulePackage grant makes `ListPackages` return `ErrUnavailable`
- [x] 2.2 Add `Platform`, `ListPackages`, `Package`, and the instance item and detail fields that come from the custom resource (module, owner, applied state, conditions, history, digests, render contracts); every method calls `Grant.Covers` first; registrations authorized per caller and marked forbidden when denied; verify with F1 (accepted and refused claims, both catalogs, the CLI-owned instance managed externally, the package `SourceNotReady`, the ApplyFailed sample Failed and retrying with no inventory) and with zero, foreign, expired and mismatched grants
- [x] 2.3 Add namespace-scoped listing (a list grant for one namespace returns only that namespace) and identical refusals for existing and missing objects; verify against F1
- [x] 2.4 `task check` green, then commit `chore(readmodel): hold the OPM kinds and build their views`

## 3. Inventory, polling, runtime children and health (internal/readmodel)

- [x] 3.1 Add tier 2: `acquire` per resolved kind with the scope order (cluster-wide, per namespace, poller, not readable), label-selected on the uuid label, sync bounded by `SyncTimeout`, Secrets refused before any informer, and the idle janitor; verify start on first use, stop after `IdleTimeout` with a fake clock, restart on the next use, and that no Secret request reaches the fake cluster
- [x] 3.2 Add the poller (`PollInterval`, `PollWorkers`, synchronous first read, `EvaluatedAt`, not live); verify with a reader that may get but not list Services in `default/podinfo`
- [x] 3.3 Add tier 3: `HoldChildren` with a refcount and release, on-demand lists cached for `ChildrenTTL` without interest, per-caller list checks setting `ChildrenAccess`; verify the watches stop on the last release and that unreadable children mark the workloads `ChildrenUnread`
- [x] 3.4 Join inventory and children into `health.Evaluate` for the instance detail (components in inventory order) and the list item; per-entry caller checks (namespace-wide `get`, then the exact name) mark forbidden and not-readable entries; verify cert-manager with ClusterRoles forbidden is partial, the CLI-owned instance's two objects are Healthy, podinfo's Pods reach the Pod rule, and a second read makes no list, get or watch request
- [x] 3.5 `task check` green, then commit `chore(readmodel): watch inventory kinds on demand and evaluate instance health`

## 4. Tier 4: events (internal/readmodel)

- [ ] 4.1 Add `Events`: `events.k8s.io/v1` lists in the object's namespace or `default` for the Platform and registrations, `regarding.*` field selectors plus the same filter client-side, folding on (regarding uid, type, reason, note) with series, deprecated and plain counts and the latest time, newest first; verify with F1 (Platform `Generated` events from `default`, kubelet lines with `eventTime: null`, the CLI-owned instance's `ManagedExternally`) and a not-covered grant
- [ ] 4.2 `task check` green, then commit `chore(readmodel): read events about one object on demand`

## 5. Grant signatures, measurements and the record (internal/readmodel, design.md)

- [ ] 5.1 Add the signature test: every exported read method of `Model` takes an `authz.Identity` and an `authz.Grant`; verify it fails when a method without a grant is added
- [ ] 5.2 Add `TestMeasureF1`: cold and warm latency of `Instance` for cert-manager and of `ListInstances`, the cluster requests each makes, and the heap held by a warm model over F1; record the numbers in design.md, Measurements on F1
- [ ] 5.3 Add the package documentation (`doc.go`); verify `go doc ./internal/readmodel` prints it
- [ ] 5.4 `task check` green, then commit `chore(readmodel): measure the read model on F1 and document it`
