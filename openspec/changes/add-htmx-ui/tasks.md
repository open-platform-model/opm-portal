## 1. Read API additions (internal/readmodel, internal/api, api/v1alpha1, openapi)

- [x] 1.1 `internal/readmodel`: `Model.Object` (caller grant covers `get`, Secret refused first, reader `get` review, on-demand get, strip without the memory drops); Pod `Containers` on `RuntimeChild`
- [x] 1.2 `api/v1alpha1`: `Object` document and `KindObject`; `RuntimeChild.Containers`
- [x] 1.3 `internal/api`: `…/object` for instances and packages (owner get, Secret refusal, kind resolution, object get, inventory reach); `POST stream/{stream}/topics` (JSON only, `Subscribe` then `Unsubscribe`, problems); route table and method handling
- [x] 1.4 `openapi/v1alpha1.yaml`: the two resources, `Object`, `containers` (additive); `internal/api/apitest` helper serving a capture for other packages' tests
- [x] 1.5 Tests: object goldens and refusals (not reached, Secret without review, unknown kind, forbidden), topic change (add/remove, other session, form body, method), containers in goldens, contract test covers the new routes
- [x] 1.6 `task check` green, then commit `feat(api): serve one inventory object and change a stream's topics`

## 2. The pages (internal/ui)

- [x] 2.1 Vendor htmx 2.0.11, htmx-ext-sse 2.2.4 and two OFL variable fonts under `internal/ui/static/vendor` with `CHECKSUMS` and licences; `go:embed`; checksum test
- [x] 2.2 `internal/ui`: `New(Config)`, in-process `fetch` through the API handler, page CSP, layout, tokens CSS (light, dark, phone width), problem regions (locked, degraded, not found, sign-in), badges for both axes with unknown fallback
- [x] 2.3 Platform page: subscriptions, registry, registrations with accepted and active pills, verdicts and reasons, contracts as information, conditions with explanations, platform graph, events feed
- [x] 2.4 Instances and packages lists (namespace filter, two badges, locked list); instance and package pages (graph, components with children, conditions, render contracts as text, history, events with the expiry and render-warning notes, logs per container); YAML, node panel and events fragments; group expansion
- [x] 2.5 `portal.js`: stream topic sync on open and after boosted swaps, region refresh by topic, log panes as text, closed topics, node activation, zoom and drag-to-scroll
- [x] 2.6 Tests: import rule, page policy and no inline code on every page, goldens per page on F1, the image-break sample and locked cases, accessibility smoke, topics per page, sign-in, problems as regions, YAML without withheld fields, unknown enums, static assets served
- [x] 2.7 `task check` green, then commit `feat(ui): serve the portal pages`

## 3. Launch onto the pages (internal/auth, cmd/opm-portal)

- [x] 3.1 `internal/auth`: the launch serves the landing page in place under the new session; `portal.js` moves once to the page's own address (Firefox and WebKit withhold the cookie on a reload otherwise)
- [x] 3.2 `cmd/opm-portal`: mount the read API under its prefix and the UI everywhere else; landing `/`
- [x] 3.3 Tests: auth launch serves the landing under the session; browser test stand-in page loads `portal.js` under the page policy; e2e launch and instance page
- [x] 3.4 Run `task test:browser` (Chromium, Firefox, WebKit, both launch paths)
- [x] 3.5 `task check` green, then commit `feat(local): land a launch on the platform page`

## 4. End to end, screenshots and docs (test, README, AGENTS.md)

- [x] 4.1 Throwaway cluster `opm-portal-e2e-ui`: `task e2e:local`, serve, screenshot every page light, dark and at phone width with the Playwright image, run the scripted image break and record when the UI shows Degraded, delete the cluster
- [x] 4.2 README (pages, what the YAML view shows per 0030:D8:R4) and `AGENTS.md` (layout, `internal/ui`, UI goldens, the dev server)
- [x] 4.3 Fix from the e2e run: a namespace-scoped user's launch landed on a 403 (the Platform is forbidden to them); a forbidden page now renders its locked region with 200, and htmx swaps 4xx and 5xx fragments so their regions show; commit `fix(ui): serve a locked page as 200 and show error fragments`
- [x] 4.4 `task check` green, then commit `docs(ui): document the portal pages`
