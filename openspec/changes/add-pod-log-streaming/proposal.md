## Why

The owner asked for "status + events logs" next to each module's resources. Enhancement 0030:D10
fixes the rules: logs stream only for Pods an OPM inventory reaches through workload ownership,
the user's `pods/log` access is confirmed before a stream starts and again on every reconnect,
and the portal bounds each stream itself instead of using the API server's byte limit, which ends
a followed stream outright. The stream broker (`add-stream-broker`) reserved the
`log:<namespace>/<pod>/<container>` topic and refuses it until this change serves it.

## What Changes

- New package `internal/logs`: a bounded Pod log reader and a `stream.Producer` for log topics.
  - Reachability: a Pod is served only when it is a runtime child of an inventory object the
    caller may read, decided by the read model's runtime-children join. An unreachable Pod is
    refused with the same `forbidden` closing as a missing permission (0030:D10:R1, 0030:D7).
  - `get pods/log` is the topic's read, so the broker authorizes it before the upstream stream
    opens, re-checks it before every delivery through its single send funnel, and authorizes it
    again on reconnect (0030:D10:R2). The reading identity is checked too before any upstream read.
  - Follow mode through client-go `GetLogs` with `timestamps` and a capped `tailLines`; never
    `limitBytes` (0030:D10). Bounds enforced in the portal: an oversize line is cut and marked
    `truncated` with the bytes cut, lines over the per-topic line and byte rate are dropped and
    reported by a `rate-limited` marker with a count, and an oversize initial tail skips ahead to
    live output with a `skipped` marker and a count (0030:D10:R3).
  - Container selection against the Pod's containers, init containers and ephemeral containers,
    and the previous container's logs for a crash loop (`log:<ns>/<pod>/<container>/previous`).
  - A container that stops ends its topic with an explicit `logend` message (0030:D10:R4).
  - One upstream stream per topic, shared by its subscribers and closed when the last one leaves.
  - Log content never reaches the portal's own logs.
- `internal/stream`: serves log topics. The grammar gains the optional `/previous` segment, items
  gain the `log` and `logend` events, a producer may admit a topic per identity after its reads
  are allowed (`Admitter`), a `Mux` routes topic kinds to producers, and log topics are capped per
  session (4) and per process (50).
- `internal/readmodel`: `ReachPod` answers which inventory object a Pod is reachable from, for
  the caller, through the runtime children join.

## Capabilities

### New Capabilities

- `pod-logs`: which Pods' logs a caller may follow, how a log topic is authorized, and how its
  output is bounded, marked and ended.

### Modified Capabilities

- `change-stream`: the topic grammar serves `log:` topics (with an optional `previous` segment)
  instead of refusing them; log topics are capped per session and per process.
- `read-model`: a Pod's reachability from an inventory, for the caller, is a read-model query.

## Impact

- Packages: new `internal/logs` (imports `internal/authz`, `internal/readmodel`,
  `internal/stream`, client-go's typed core client); `internal/stream` and `internal/readmodel`
  gain the pieces above. No API route or UI page yet: the read API mounts the stream and its
  per-request log endpoints in its own change, which also lists a Pod's containers.
- Principle V: the change reads a new kind of data, Pod logs (`get pods/log`) and the Pod itself
  (`get pods`, for container selection), both as the reading identity after the caller's grant.
  Nothing is written, no Secret is read, and an empty identity still opens no stream. Log lines
  are untrusted text: they are carried as JSON strings and never logged by the portal.
- Principle VII: no new dependency. The rate limiter is a small token bucket with an injectable
  clock; `golang.org/x/time/rate` is not imported for one use.
- SemVer: MINOR after 1.0 (a new capability). Nothing user-visible until the read API mounts the
  stream, so the PR title is `chore(logs)` and the 0.x line cuts no release for it.
- Enhancement link: implements 0030:D10 whole (R1 to R4) at the stream layer. No decision is
  claimed in `enhancement.yaml`: D10 becomes observable only once the read API serves the stream.
