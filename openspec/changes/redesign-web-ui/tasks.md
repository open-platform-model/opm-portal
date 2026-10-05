## 1. Read API additions (internal/authz, internal/readmodel, api/v1alpha1, internal/api, cmd/opm-portal, openapi, docs/site)

- [ ] 1.1 Spike, `/version`: on a throwaway fixture cluster (`task e2e:up`), send a SelfSubjectAccessReview with non-resource attributes `{path: /version, verb: get}` and read `/version`, once as the kubeconfig user and once as the `deploy/` ServiceAccount (`kubectl auth can-i get /version --as=system:serviceaccount:opm-portal:opm-portal`); record both answers in design.md, Research & Decisions; if the ServiceAccount is denied, stop and ask the supervisor before adding a rule to `deploy/clusterrole.yaml`; `task e2e:down`
- [ ] 1.2 Spike, boosted filter restore: in htmx 2.0.11, rewrite a boosted request's path in `htmx:configRequest` and check which URL is pushed; if not the rewritten one, use an `HX-Push-Url` response header from `internal/ui`; record the finding in design.md
- [ ] 1.3 `internal/authz`: `Attributes.NonResourcePath`; refuse without asking every non-resource read but `get /version`, and any request naming both a path and a resource; local review sends `nonResourceAttributes`, in-cluster review the SubjectAccessReview equivalent; `Covers` compares the path; table tests across both shapes; the grant seal scan stays green
- [ ] 1.4 `internal/readmodel`: the holder join (`Holders`, `ProviderOf`) over held inventories, kind-agnostic, per-caller and marked incomplete; package `interval` and source artifact (revision, digest; never the URL); the server version read at `Start` after the reader's review; history `outcome` from phase and message; tests over F1 (backup-provider holds its claim, the refused claim has no holder, the package's three entries are `Failed`) and a package-holder fixture derived from F1 and labelled constructed
- [ ] 1.5 `api/v1alpha1`, `internal/api`: the `Cluster` document and route; `providerOf` on instance and package items; `renderContracts` moved into `InstanceSummary`; `interval` and `sourceArtifact` on packages; `conditions`, `heldBy`, `heldByPartial` on registrations; `outcome` on history entries; the in-cluster omission names registration condition messages and keeps outcomes and standings; OpenAPI with `x-extensible-enum` for `outcome`, `source` and `kubernetesVersionAccess`; API goldens updated with `-update` and reviewed; `task api:breaking` green
- [ ] 1.6 `cmd/opm-portal`: pass the context name and cluster entry name, or the in-cluster source, to `api.Config`; table test over `configSource` cases
- [ ] 1.7 `docs/site/reference/portal/read-api.md`: the cluster path and the new fields (`TestReadAPIReferenceListsEveryPath`); `task docs:bundle:check` green
- [ ] 1.8 `task check` green, then commit `feat(api): serve the cluster, provider holders and package sources`

## 2. Shell, theme and Installed (internal/ui)

- [ ] 2.1 `portal.css`: every colour a custom property on `:root`, redefined for dark under `prefers-color-scheme` with `:root:not([data-theme="light"])` and under `:root[data-theme="dark"]`; status colours per the canvas, red kept to failures (the refine-htmx-ui rule)
- [ ] 2.2 `static/prefs.js` (theme before first paint, filter restore on a full load, every storage access in `try`/`catch`) loaded without `defer` before the stylesheet; theme menu (Light, Dark, System) in the header
- [ ] 2.3 Header: sticky, slimming past 48 px through an `IntersectionObserver` sentinel, no transition under reduced motion; nav Platform and Installed; cluster chip, "reading as", Kubernetes version locked when not readable, from the `Cluster` document
- [ ] 2.4 `/installed`: instances and packages merged, columns per design.md, provider badge from `providerOf`, owner words; locked group per forbidden kind; `/instances` and `/packages` answer `308` to `/installed` with `kind` and `namespace`
- [ ] 2.5 Filters: server-side from the query, chips that drop one parameter, a `GET` filter form; `portal.js` restores on boosted navigation per spike 1.2 and stores each view's own parameters after render; "Clear filters" removes the stored entry
- [ ] 2.6 Page text says controller, Providers and Installed everywhere (wire `operator` reads "controller")
- [ ] 2.7 Tests: no `style` attribute and no `setAttribute("style"` in page scripts; the storage-key test (only the theme and filter keys are written); filter parsing and chip links; redirects; UI goldens updated and reviewed; a browser test (`TestBrowserTheme`, build tag `browser`) that a stored Dark paints dark first and a stored filter opens filtered, run with `task test:browser`
- [ ] 2.8 `task check` green, then commit `feat(ui): add Installed, a theme choice and remembered filters`

## 3. Platform page (internal/ui)

- [ ] 3.1 Identity card (cluster, type, controller version, Kubernetes version, context) and status card: `Ready` reasons and `ContractsFulfilled` as information, each reason linking to its condition row
- [ ] 3.2 Installed card: counts per health and applied state linking to `/installed` filtered, the instances, packages and holders line, locked counts for a forbidden list
- [ ] 3.3 Providers and Catalogs tabs with their filters: provider rows with holder links from `heldBy`, two pills, refusal or blocked reason; catalog rows with the path as text (section 5 links it to the Catalog page)
- [ ] 3.4 Recent events: the Platform's feed and each readable registration's, merged newest first, with the resource filter; follow `platform`, `instances`, `events:platform` and `events:registration:<name>`
- [ ] 3.5 Remove the platform graph and the `/platform/node` route from the UI; `platform/graph` stays in the read API
- [ ] 3.6 UI goldens updated and reviewed; the hostile-text suite covers the new regions
- [ ] 3.7 `task check` green, then commit `feat(ui): rebuild the Platform page around providers and catalogs`

## 4. Instance and package pages (internal/ui)

- [ ] 4.1 Identity card (applier ServiceAccount, package interval "not set" and revision "not recorded" when absent); Applied card with feed reason counts linking to Events, attempt dots by `outcome`, a running dot only while `Reconciling=True`, the ten-entry note and "Show history"; Health card with reason counts linking to Resources; Provider card when `providerOf` is not empty
- [ ] 4.2 Tabs Graph, Resources, Events, Logs, YAML as `tab=` links; the details panel on Graph and Resources only; Resources rows link to Graph with `focus`; Events filters by resource, type and reason; Logs and YAML pickers from today's regions
- [ ] 4.3 `portal.js` graph interactions: hover and focus cards placed through CSSOM properties, focus spotlight and "Clear selection", full screen with a class fallback, zoom slider bound to the existing scale beside Fit, group expand through `expand` with fit-to-members and "Whole graph"
- [ ] 4.4 Live refresh keeps the tab, focus, zoom and open cards (extend `refresh_test.go`); `TestBrowserLogs` and `TestBrowserExpired` still pass under `task test:browser`
- [ ] 4.5 UI goldens updated and reviewed; the hostile-text suite covers cards, dots and tabs
- [ ] 4.6 `task check` green, then commit `feat(ui): split instance and package pages into summary cards and tabs`

## 5. Provider tab, Catalog page, live check and docs (internal/ui, test/e2e, docs/site, ROADMAP.md)

- [ ] 5.1 Provider tab: registration, catalog link and registry state, conditions table, per provided contract "Used by" (three, the rest counted, "All N in Installed"), incomplete marked
- [ ] 5.2 Catalog page `/catalog?path=`, and the Platform page's catalog paths and provider catalogs linked to it: identity, origin, source and holder, resolved block with the right reason, platform-wide `ContractsFulfilled`, Claims and Events tabs, the not-recorded note; `404` for a path nothing names
- [ ] 5.3 `cmd/opm-portal` e2e: `TestM1` reads the `Cluster` document and `/installed`; `TestPod` checks the `Cluster` document names `source: in-cluster` and a Kubernetes version as the ServiceAccount
- [ ] 5.4 Throwaway cluster: `task e2e:up`, `task e2e:local`, `task e2e:m1`, `task e2e:pod`; screenshots of every page in light, dark and at 360 px; re-run the scripted image break on an open instance page; record what was seen in design.md; `task e2e:down`
- [ ] 5.5 `docs/site/operating/portal/run-the-portal-locally.md` (the Platform page, `/installed` links) and `portal-security.md` (what the browser stores, portal:D14); `task docs:bundle:check` green
- [ ] 5.6 `ROADMAP.md`: the change moves to in review with its PR number; under Next in V1, the `packages` stream topic and the controller follow-ups portal:OQ23 and portal:OQ24; "Last updated"
- [ ] 5.7 `task check` green, then commit `feat(ui): add the Provider tab and the Catalog page`
