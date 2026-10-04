## 1. Topics and the ring buffer (internal/stream)

- [x] 1.1 Add `Kind`, `Topic`, `ParseTopic` and `*TopicError` with the grammar in the spec (DNS-1123 namespaces and names, `events:` refs limited to object topics, `log:` parsed and reserved); verify with a table test of accepted and refused names, including wildcards, empty segments, upper case, nested `events:` and over-long names
- [x] 1.2 Add `Item` with its validation and the per-topic ring (fixed capacity, a floor that records the highest evicted sequence, `covers(after)`, `since(after)`); verify with table tests for wrap-around, a floor set at creation, and a gap after overflow, and a table test of valid and invalid items
- [x] 1.3 Add the package documentation (`doc.go`) stating the producer contract and the authorization rules; verify `go doc ./internal/stream` prints it
- [x] 1.4 `task check` green, then commit `chore(stream): add stream topics and the per-topic ring buffer`

## 2. Broker: subscriptions, authorization and delivery (internal/stream)

- [ ] 2.1 Add `Item`, `Producer`, `Session`, `Options`, the errors, `New`, `Publish`, `Open` (fresh streams only), `Subscribe`, `Unsubscribe`, `Close`, and `Stream.Serve` writing `open`, `snapshot`, item and `closed` messages with strictly increasing ids; verify with `synctest` tests over a fake producer and a real local `authz.Checker` on a fake clientset: snapshot then changes, a change racing the snapshot is never lost, topics added and removed on an open stream, `Activate`/`release` refcounting
- [ ] 2.2 Gate subscriptions and deliveries through `authz`: synchronous check before registering a topic, `Covers` then re-`Check` before each delivery and on each heartbeat, per-item gate, `forbidden` and `upstream_unavailable` closes, unauthenticated sessions refused with no review; verify with tests that a namespace-limited subscriber gets zero items of another namespace from snapshots and changes, that a revoked permission closes the topic after the TTL, and that a review error delivers nothing
- [ ] 2.3 `task check` green, then commit `chore(stream): add the broker with per-subscriber authorization`

## 3. Resume, keepalive and bounds (internal/stream)

- [ ] 3.1 Add detached streams, the resume window, `Last-Event-ID` parsing and reattach (same session only, takeover of a still-attached stream, re-authorization, ring replay or fresh snapshot); verify with `synctest` tests for resume within the buffer, past the buffer, before the snapshot was delivered, from another session, from another epoch and after the window
- [ ] 3.2 Add heartbeats, the idle timeout, queue-full eviction, write deadlines, and the session, process and topic caps with discard of the oldest detached stream; verify with `synctest` tests that a quiet stream gets one heartbeat per interval, a stalled reader is evicted without delaying another subscriber and can resume, a third stream in a session is refused while a reloaded tab is not, and the process cap holds
- [ ] 3.3 `task check` green, then commit `chore(stream): resume streams and bound slow consumers`

## 4. Internal HTTP handler (internal/stream)

- [ ] 4.1 Add `NewHandler` and `HandlerOptions` (GET only, session lookup failing closed, `topics` query parsing, `Last-Event-ID` header, SSE headers, error mapping); verify with `httptest.Server` tests for the headers and first messages, 400 on a bad or reserved topic, 401 on an empty identity, 429 on the third stream, and a reconnect with `Last-Event-ID` that resumes over real HTTP
- [ ] 4.2 `task check` green, then commit `chore(stream): serve a stream through an internal handler`
