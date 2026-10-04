## 1. Read-model and graph inputs the API needs (internal/readmodel, internal/graph)

- [ ] 1.1 Add `Change` and `Model.OnChange`: an event handler on every informer `startWatch` builds maps OPM kinds to themselves (deletions marked), runtime children to held owners by name label, other objects to the held owner whose UID is their uuid label, and registrations also to the Platform; verify with F1 that updating `default/podinfo-podinfo` reports `ModuleInstance default/podinfo` and deleting the instance reports a deletion
- [ ] 1.2 Add `Model.ResolveKind(group, kind)`; verify `apps/Deployment` resolves namespaced and an unknown kind errors
- [ ] 1.3 Add `graph.Contributor` and use it in the platform builder; verify the platform goldens are unchanged and F1's backup catalog names `default.backup-provider`
- [ ] 1.4 `task check` green, then commit `feat(readmodel): report changed OPM objects and resolve kinds`

## 2. Wire types and the OpenAPI contract (api/v1alpha1, openapi)

- [ ] 2.1 Add `api/v1alpha1`: documents, list items, components, inventory objects, children, conditions, history, platform, catalogs, registrations, events, graph, `Removed`, `Problem`, and the problem codes; no functions
- [ ] 2.2 Write `openapi/v1alpha1.yaml` (OpenAPI 3.1, open enums as `x-extensible-enum`, the verbatim-messages caveat in plain words); verify `oasdiff breaking` parses it against itself
- [ ] 2.3 `task check` green, then commit `feat(api): add the v1alpha1 wire types and OpenAPI document`

## 3. Handlers (internal/api)

- [ ] 3.1 Add the server: the `Authenticate` seam (401 before routing), the route table, cluster check, path validation, 404 and 405 as problems, the error mapping and the single forbidden document
- [ ] 3.2 Add the view-to-wire mapping and the handlers for instances, packages, platform (claimants, contributedBy), the three graphs (provider lookups authorized per provider) and the events resources with the reach gate
- [ ] 3.3 Add the contract test: every route in the OpenAPI document with its 200 schema, every wire type matching its schema
- [ ] 3.4 Add golden JSON tests from F1 through `httptest` with SSAR-backed checkers on the fake cluster (`-update` rewrites), and the denial tests with a fake authorizer: forbidden existing and missing byte-identical, list denial empty with no count, partial access, unauthenticated without review, unavailable review, unknown cluster, bad names, an unreached Lease's events refused like a forbidden read, a forbidden provider in the platform graph, and no `values` or last-applied annotation anywhere
- [ ] 3.5 `task check` green, then commit `feat(api): serve instances, packages, the platform, graphs and events`

## 4. The change stream (internal/api)

- [ ] 4.1 Add the producer: topics' attributes as the GETs authorize them, one rendered document per topic, `Removed` for a missing object, active topics, coalesced publishing from `OnChange`, deletes, the periodic refresh, and children holds for instance and package topics
- [ ] 4.2 Mount `stream` with problem documents for the broker's refusals; close the producer and broker with the server
- [ ] 4.3 Test through `httptest`: a snapshot equals the GET body, a Pod moved to `ErrImagePull` in the fake cluster delivers a `Degraded` upsert, a deleted instance delivers `Removed`, a forbidden topic closes, too many streams is 429
- [ ] 4.4 `task check` green, then commit `feat(api): serve the change stream over the read model`

## 5. The breaking-change gate (hack, .github)

- [ ] 5.1 Add `hack/api-breaking.sh` (base copy via `git show`, no base copy passes, `!` in `PR_TITLE` lets a breaking diff pass) and a `task api:breaking`; verify locally that removing a response property fails without `!` and passes with it
- [ ] 5.2 Add the step to the `Lint` job on pull requests (oasdiff pinned, title through `env`); `actionlint` clean; update `AGENTS.md` (layout, commands)
- [ ] 5.3 `task check` green, then commit `ci(api): fail a breaking OpenAPI change without a breaking title`
