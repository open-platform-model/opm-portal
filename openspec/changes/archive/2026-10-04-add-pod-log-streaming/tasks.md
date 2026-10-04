## 1. Stream: serve log topics

- [x] 1.1 `internal/stream/topic.go`: accept the optional `/previous` segment on log topics (`Topic.Previous()`), round-trip it in `String`; topic tests for the new and malformed forms
- [x] 1.2 `internal/stream/item.go`: add the `log` and `logend` item events
- [x] 1.3 `internal/stream/broker.go`: drop the log-topic refusal; add `Admitter`, `ErrNotAdmitted` and call `Admit` in `authorize` after a topic's reads are allowed; add `MaxLogTopicsPerSession` (4) and `MaxLogTopics` (50) checked before any review
- [x] 1.4 `internal/stream/mux.go`: `Mux` routing kinds to producers, delegating `Admit`
- [x] 1.5 Broker tests: admission refusal closes with `forbidden` and starts no activation, admission error closes with `upstream_unavailable`, reattach admits again, log caps per session and per process, `Mux` routing; update `doc.go` and the reserved-topic test
- [x] 1.6 `task check` green, then commit `chore(stream): serve log topics behind per-identity admission`

## 2. Read model: Pod reachability

- [x] 2.1 `internal/readmodel/reach.go`: `ReachPod`, `PodReach`, `ErrNotReachable` per design
- [x] 2.2 Tests on the F1 capture: a podinfo Pod reaches its Deployment; a stray Pod, a missing Pod and a Pod behind a forbidden Deployment get `ErrNotReachable`; an uncovered grant reads nothing; an unavailable kind is `ErrUnavailable`
- [x] 2.3 `task check` green, then commit `chore(readmodel): answer whether an inventory reaches a pod`

## 3. Logs: bounded reader and producer

- [x] 3.1 `internal/logs/source.go`: `Source`, `ClientSource` over a typed clientset (Get pod, `GetLogs(...).Stream`, never `LimitBytes`)
- [x] 3.2 `internal/logs/reader.go`: bounded line reader (line cap with chunked discard, timestamp split, tail skip), token buckets with an injectable clock, message shapes
- [x] 3.3 `internal/logs/producer.go`: `Producer` (Attributes, Admit through a `Reacher`, Snapshot from the buffer, Activate with one upstream per topic and a release that closes it), container selection, previous logs, end reasons, reader-identity checks, `SetPublisher`
- [x] 3.4 Unit tests: truncation, rate-limited marker with count, tail skip marker, end reasons, container selection, previous options, no `LimitBytes`, release closes the upstream, no line text in portal logs, `ClientSource` against a fake clientset
- [x] 3.5 Broker integration tests with the real broker, a fake clientset authorizer and a fake source: denied `pods/log` opens no upstream; unreachable Pod gets the same `forbidden` closing; lines arrive through the send funnel; revocation closes the topic and stops lines; two sessions share one upstream; last unsubscribe closes it
- [x] 3.6 `task check` green, then commit `chore(logs): stream pod logs`

## 4. Review fixes

- [x] 4.1 `internal/stream`: `Follower`, called after a new subscription to an active topic attaches on Open or Subscribe, never on reconnect; `Mux` delegates it; tests for open, reconnect, re-follow and a cap re-check refusal
- [x] 4.2 `internal/stream`: `LogRingBytes` bounds log topics' resume rings only; a test that an object topic replays while a log topic snapshots
- [x] 4.3 `internal/logs`: restart an ended read in `Follow`, not `Admit`; tests that Admit and a plain reconnect start no read and a re-follow does
- [x] 4.4 `internal/logs`: a closed activation leaves the producer before its reader is waited for and has an empty snapshot; test
- [x] 4.5 `internal/logs`: an oversize initial tail keeps its newest lines after the `skipped` marker; a quiet tail is sent after `MarkerDelay`; tests for live, quiet and previous tails
- [x] 4.6 `internal/readmodel`: an owner check the authorizer cannot decide makes `ReachPod` `ErrUnavailable`; test
- [x] 4.7 Specs and design updated; `task check` green
