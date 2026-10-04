## 1. Read API additions (internal/readmodel, internal/api, api/v1alpha1, openapi)

- [x] 1.1 `internal/readmodel`: `Model.Object` (caller grant covers `get`, Secret refused first, reader `get` review, on-demand get, strip without the memory drops); Pod `Containers` on `RuntimeChild`
- [x] 1.2 `api/v1alpha1`: `Object` document and `KindObject`; `RuntimeChild.Containers`
- [x] 1.3 `internal/api`: `…/object` for instances and packages (owner get, Secret refusal, kind resolution, object get, inventory reach); `POST stream/{stream}/topics` (JSON only, `Subscribe` then `Unsubscribe`, problems); route table and method handling
- [x] 1.4 `openapi/v1alpha1.yaml`: the two resources, `Object`, `containers` (additive); `internal/api/apitest` helper serving a capture for other packages' tests
- [x] 1.5 Tests: object goldens and refusals (not reached, Secret without review, unknown kind, forbidden), topic change (add/remove, other session, form body, method), containers in goldens, contract test covers the new routes
- [x] 1.6 `task check` green, then commit `feat(api): serve one inventory object and change a stream's topics`

## 2. UI foundation and the Platform page (internal/ui, internal/auth, cmd/opm-portal)

- [ ] 2.1 Vendor htmx 2.0.11, htmx-ext-sse 2.2.4 and two OFL variable fonts under `internal/ui/static/vendor` with `CHECKSUMS` and licences; `go:embed`; checksum test
- [ ] 2.2 `internal/ui`: `New(Config)`, in-process `fetch` through the API handler, page CSP, layout template, tokens CSS (light, dark, phone width), problem regions (locked, degraded, not found, sign-in), badges for both axes with unknown fallback
- [ ] 2.3 Platform page (`/`): subscriptions, registry, registrations with accepted and active pills, verdicts and reasons, conditions with explanations, platform graph SVG, events feed
- [ ] 2.4 `internal/auth`: launch serves the landing in place under the new session; `cmd/opm-portal`: mount `/api/` and the UI, landing `/`; browser test front door updated
- [ ] 2.5 Tests: import rule, CSP header and no inline style or script in any page, Platform golden fragment, sign-in page, launch in place
- [ ] 2.6 `task check` green, then commit `feat(ui): serve the platform page`

## 3. Instances, packages, objects and locked rendering (internal/ui)

- [ ] 3.1 Instances list with namespace filter, two badges, locked list and rows
- [ ] 3.2 Instance page: graph, components and objects with children, conditions with meanings, history timeline, render contracts as text, events feed with the expiry and render-warning notes, logs panel per container, YAML view fragment, node panel fragment, group expansion
- [ ] 3.3 Packages list and Package page, sharing the instance renderers
- [ ] 3.4 Tests: goldens per page on F1 and the image-break sample, locked rendering with a forbidden kind and a forbidden list, YAML without withheld fields, unknown enum, accessibility smoke (labels on controls, nodes focusable in column order, headings)
- [ ] 3.5 `task check` green, then commit `feat(ui): show instances, packages and their objects`

## 4. Live updates and graph interaction (internal/ui static)

- [ ] 4.1 `portal.js`: stream topic sync on open and after boosted swaps, region refresh by topic, log panes, closed topics, node activation, pan and zoom, address replacement after launch
- [ ] 4.2 Pages declare `data-topics` and regions `data-follow`; body carries the boosted stream
- [ ] 4.3 Tests: every page's topics, region markup, script served under the page policy
- [ ] 4.4 `task check` green, then commit `feat(ui): follow changes and logs live`

## 5. End to end, screenshots and docs (test, README, AGENTS.md)

- [ ] 5.1 Run `task test:browser` (launch in place in Chromium, Firefox and WebKit)
- [ ] 5.2 Throwaway cluster `opm-portal-e2e-ui`: serve, screenshot every page light, dark and at phone width with the Playwright image, run the scripted image break and record when the UI shows Degraded, delete the cluster
- [ ] 5.3 README (pages, what the YAML view shows per 0030:D8:R4) and `AGENTS.md` (layout, `internal/ui`, UI goldens)
- [ ] 5.4 `task check` green, then commit `docs(ui): document the portal pages`
