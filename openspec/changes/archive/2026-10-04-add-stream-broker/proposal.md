## Why

The portal promises that a broken rollout looks broken within seconds (0030:D3), and its read API
carries a change stream with the same document shapes as its `GET`s, resumable or answered with a
re-read (0030:D2:R5). In local mode the browser speaks HTTP/1.1 to loopback, so every tab shares
six connections per host: one stream per tab, carrying every topic the page needs, is the only
shape that leaves connections for ordinary requests. The broker also sits exactly where a shared
cache meets many readers, so it must deliver to each subscriber only what that subscriber may
read (0030:D7). The read model (change `add-read-model`) and the read API (`add-read-api`) are
built next to and after this change; both need the broker's contract fixed first.

## What Changes

- New package `internal/stream`: a server-sent-events broker.
- Topics with a fixed grammar: `platform`, `instances`, `instances:<ns>`, `instance:<ns>/<name>`,
  `package:<ns>/<name>`, `registration:<name>` and `events:<object topic>`. The `log:` prefix is
  parsed and reserved for the pod log change; subscribing to it is refused until then.
- One multiplexed stream per browser tab: topics are given when the stream opens and are added
  and removed while it stays open.
- Per-principal filtering: a topic is authorized through `internal/authz` before it is attached,
  the topic's grants are re-checked before each delivery, and every item is delivered only when
  the subscriber may read the object it reveals. The instance list topics follow the `GET` list
  rule: they attach without a topic-wide read and are filtered per item. A denial or an
  authorization error is never a delivery.
- A per-topic ring buffer and `Last-Event-ID` resume: a stream that reconnects within the resume
  window continues where it stopped; when the buffer no longer covers the gap the topic is sent a
  fresh snapshot.
- Heartbeats, a bounded per-stream queue that evicts a slow consumer, an idle timeout, and caps on
  streams per session (two by default, for the six-connection limit), streams per process and
  topics per stream.
- A `Producer` interface the read model implements (topic access, snapshots, activation for
  refcounted watches) and `Broker.Publish` for changes; a fake producer for tests.
- An internal `http.Handler` constructor that serves one stream, tested through `httptest`. No
  route is mounted: `/api/v1alpha1/watch` and the topic-change request arrive with the read API.

## Capabilities

### New Capabilities

- `change-stream`: how a client receives live changes over one server-sent-events stream: topics,
  subscription and per-item authorization, snapshots, resume, heartbeats, eviction and caps.

### Modified Capabilities

None.

## Impact

- Packages: new `internal/stream`, which imports `internal/authz` only. No API route, UI page or
  binary behaviour changes; the package has no production caller until `add-read-api`.
- Principle V: the broker reads nothing from the cluster. Its only cluster effect is through
  `authz.Authorizer.Check`, which in local mode creates `selfsubjectaccessreviews` and nothing
  else. It fails closed: an unauthenticated session opens no stream, and a denial or an
  authorization error delivers nothing.
- Principle VII: standard library only (`net/http`, `log/slog`, `testing/synctest` for the fake
  clock). The ring buffer, resume window and slow-consumer eviction exist because the stream must
  survive reconnects and must not let one stalled tab hold memory or block the publisher.
- SemVer: MINOR after 1.0 (a new internal capability). Nothing user-visible changes, so the PR
  title is `chore(stream)` and the 0.x line cuts no release for it.
- Enhancement link: builds toward 0030:D2:R5 and the stream half of 0030:D7:R2; claims no decision
  (`enhancement.yaml`), because neither is observable until the read API serves the stream.

Not in this change: the `/api/v1alpha1/watch` route and its problem documents, the topic-change
request and its CSRF protection (`add-read-api`), the producer implementation (`add-read-model`),
the HTML rendering of stream items (`add-htmx-ui`), and log topics with their per-session caps
(`add-pod-log-streaming`).
