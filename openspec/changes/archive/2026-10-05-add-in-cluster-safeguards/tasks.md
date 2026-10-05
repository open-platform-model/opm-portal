## 1. Per-subscriber comparison on the stream (internal/stream)

- [x] 1.1 `subscription.sent`; `Stream.eventID` writes an upsert or k8sevent item only when it differs from the last one written to the subscription, a snapshot or delete is always written and sets it, a log line is never compared; `reattach` keeps it only when the client holds it
- [x] 1.2 Tests: a change only bob may see sends alice nothing and leaves her ids consecutive; an unchanged re-render sends nobody anything; a reconnect from the last event received writes no document the client holds, and one from before it writes the first item; a delete after an upsert carrying its Removed document is written; a resume that falls back to a snapshot (ring gap, forgotten id) compares later items with it; equal log lines are both written; a re-added topic starts from its snapshot without touching another subscriber
- [x] 1.3 Package documentation describes the comparison
- [x] 1.4 `task check` green, then commit `feat(stream): write a subscriber only a change to its own document`

## 2. Server mode and in-cluster omission (internal/api, cmd/opm-portal, openapi)

- [x] 2.1 `api.Mode` (`local`, `in-cluster`), required in `Config`; `New` refuses an empty or unknown mode; `serve`, `apitest` and the API test harness pass it
- [x] 2.2 `omitOperatorText` over every wire document, applied in `Server.document` and the stream producer's render in `in-cluster` mode; only operator-reported event notes are dropped, other writers' notes and workload health messages stay; an unknown document type is an error
- [x] 2.3 OpenAPI: each omitted field's description says it is absent in-cluster; the info description says so too
- [x] 2.4 Tests: in-cluster goldens for the documents that carry operator text (instance, broken instance, package, lists, platform, graphs, events, the backup-provider registration's `Object`), a walk that fails on any operator-text field in an in-cluster document, the stream carrying the omitted document, every route's document type known to the omission, `New` refusing a missing mode; the kubelet's notes and workload health messages served in-cluster as in local mode; local goldens unchanged
- [x] 2.5 `task check` green, then commit `feat(api): hide operator messages in-cluster`

## 3. Pages in both modes (internal/ui)

- [x] 3.1 `apitest.NewInCluster`; UI goldens of the broken podinfo page and the platform page over an in-cluster API, and a test that no operator message of the capture appears on any in-cluster page
- [x] 3.2 `task check` green, then commit `test(ui): render the pages over an in-cluster read API`

## 4. Documentation (docs/site)

- [x] 4.1 `portal-security.md` and `reference/portal/read-api.md`: messages are shown as written in local mode; the in-cluster mode, when built, shows only the reasons of the operator's text and other writers' text as written
- [x] 4.2 `task docs:bundle:check` and `task check` green, then commit `docs(api): say where operator messages are shown`
