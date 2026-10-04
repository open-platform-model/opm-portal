## 1. Read-model inputs the graph needs (internal/readmodel)

- [x] 1.1 Move the cluster-side test harness (kinds, list loaders, fake dynamic and discovery clients, the rule-answered local checker) to `internal/readmodel/readmodeltest`, which does not import `readmodel`; `readmodel`'s tests use it; verify `go test ./internal/readmodel/...`
- [x] 1.2 Add `RuntimeChild` and `InventoryObject.Children`: children whose controller chain reaches an inventory object, each with its direct controller owner, health and a ReplicaSet's replicas, attached only when the children read succeeded; verify podinfo's Deployment carries one ReplicaSet (2 replicas) and two Healthy Pods, and that unreadable Pods leave no children
- [x] 1.3 Add `PackageItem.DependsOn` from `spec.dependsOn`; verify `pkg/podinfo` has none
- [x] 1.4 `task check` green, then commit `chore(readmodel): keep runtime children and package dependencies in the views`

## 2. Graph model: ids, nodes, edges and collapse (internal/graph)

- [x] 2.1 Add the id grammar (`<prefix>:<parts>`, escaped parts, `_` for empty) and the node and edge types with JSON tags; verify ids for catalog paths, empty groups and cluster scope, and that no id changes with a UID
- [x] 2.2 Add `Instance` and `Package`: module or source and dependency nodes, components, objects with access and health, runtime children below inventory objects, the edge kinds with their sources, render contracts as text; verify on F1 podinfo and the CLI-owned web
- [x] 2.3 Add the collapse rules: configuration group (two or more), `Options.Expand`, scaled-down ReplicaSets hidden with a count, Pod groups above five, the node cap with a summary node; verify cert-manager collapsed (19 nodes) and expanded (all 20 components, 42 objects), a scaled-to-zero ReplicaSet, seven Pods, and a cap of 30
- [x] 2.4 Add `Platform`: platform, catalogs, registrations with acceptance, activation and verdict, provider instances with the verified or unverified `providedBy` edge; verify on F1 (accepted claim verified, refused claim unverified `ProviderNotFound` with no `contributes` edge), a forbidden provider lookup, and registrations forbidden
- [x] 2.5 Add the layout (moved from section 3, since every builder ends in it): columns per scope and kind, the downward and upward barycenter sweeps with stable tie-breaks, integer coordinates and edge routes; verify ordering on a small hand-built graph and that two builds are equal
- [x] 2.6 `task check` green, then commit `chore(graph): derive instance, package and platform graphs from the read model`

## 3. Layout, goldens and measurement (internal/graph, design.md)

- [x] 3.1 (moved to 2.5)
- [ ] 3.2 Add the test-only SVG writer and the golden suite (`-update` rewrites): instance graphs of cert-manager (collapsed and expanded), podinfo broken (experiment 01 samples), web, backup-provider and backup-consumer, package `pkg/podinfo`, and the F1 platform graph, each as JSON and SVG
- [ ] 3.3 Add `TestMeasureCertManager`: build time and node and edge counts for cert-manager collapsed and expanded; record them in design.md, Measurements on F1
- [ ] 3.4 Add the package documentation (`doc.go`); verify `go doc ./internal/graph` prints it
- [ ] 3.5 `task check` green, then commit `chore(graph): lay out graphs deterministically and add F1 goldens`
