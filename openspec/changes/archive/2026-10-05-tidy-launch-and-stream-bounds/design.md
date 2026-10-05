## Context

Issue 21 (items 2 and 5) and issue 13 collect follow-ups from the reviews of local mode (PR 20)
and the stream broker (PR 9). Each is small; together they bound three lifetimes: the launch
page's, an open stream's after its session, and one message's re-validation.

## Goals / Non-Goals

**Goals:** the launch page leaves the disk when its token is spent; an expired session gets no
more data on an open stream and the page says so; one message cannot keep a stream silent for
longer than one decision lifetime; the write-path guard pins what control events carry; the
revocation bound is stated as it is.

**Non-Goals:** renewing a session, warning before expiry, the meta-refresh hand-off page (issue
21, item 3), the nightly e2e and browser runs (item 1, change H1).

## Decisions

### The launch page goes when the token is spent

`auth.Local` gains `Launched() <-chan struct{}`, closed once, by `launch`, when the token is
spent. `runLocal` starts a goroutine after `openLaunch` that waits for it (or for shutdown) and
runs the cleanup. `openLaunch`'s cleanup is wrapped in `sync.Once`, so the goroutine and the
deferred shutdown call may both run it. The page is read by the browser before the request that
spends the token is sent, so removing it then cannot break the launch; the printed link spends
the same token, so the page goes then too.

### A stream ends at session expiry

`stream.Session` gains `Expires time.Time` (zero: no expiry). `auth.Local.Authenticate` returns
an `auth.Session{Identity, Key, Expires}`; `api.Principal` carries `Expires` to the stream
handler. The broker:

- `Open` refuses a session whose `Expires` has passed with `ErrUnauthenticated`, before any
  review.
- `streamState.expires` holds the session's expiry; a reconnect of the same session sets it
  again (the session is the same, so a later expiry can only come from a renewed session).
- `Stream.run` holds a timer for the expiry. When it fires the writer writes
  `event: expired`, `data: {"code":"unauthenticated"}` and returns `ErrSessionExpired`, which
  `disconnect` treats like `ErrIdle`: the stream is deleted, not kept for resume.

```text
event: expired
data: {"code":"unauthenticated"}
```

The page sets `sse-close="expired"` on the element that holds the `EventSource`, so the htmx SSE
extension closes it on that event and does not reconnect, and the script's listener sets the
live indicator to "session expired, reload". A reconnect would only get `401` and retry with
back-off forever.

API: additive. The stream description in `openapi/v1alpha1.yaml` and the reference page name the
event.

### Re-validation is bounded by time

Options considered:

1. A cap on reviews per message: deterministic, but a large snapshot whose parts all expired
   during a slow render needs one review per part and would be closed for being large, not for
   failing to settle.
2. A total deadline of one decision lifetime: bounds the silence a message can cause directly,
   whatever its size.

**Decision:** option 2. `revalidate` runs under `context.WithTimeout(ctx,
opts.RevalidateTimeout)` (default 30 s, the authorizer's default decision lifetime). Each pass
first checks the in-memory `covered`; then, if the deadline has passed, it closes the topic with
`upstream_unavailable`; otherwise it reviews. `gateTopic` and `recheckParts` check the context
before every review, so no review starts after the deadline, and a review running at the
deadline is cancelled by the context. A fresh grant from `Check` covers its own request, so each
pass that does not confirm makes progress or hits the deadline.

Pinned in `TestAMessageWhoseGrantsNeverSettleClosesTheTopic`: eight parts with 4.5 s reviews
after a 31 s render. Re-validation starts at t=31 with every decision expired, deadline t=61: the
topic's review, then the parts' reviews starting at t=31, 35.5, ..., 58 (seven), and none after.
Nine reviews at open, eight in re-validation: exactly 17. The old three-round cap sent 36
(nine at open, nine in each round, measured); a two-round cap would send 27.

### Control events pinned in the guard

`TestOnlySendWritesTopicData` compares every argument of each control call to `wr.event` against
its expected source text: `run` writes `"retry: "+strconv.Itoa(retryMillis)+"\n"`, `""`,
`EventOpen`, `open`; `heartbeat` writes `""`, `""`, `EventHeartbeat`, `[]byte("{}")`; `closed`
and `expired` write `""`, `""`, their event, `body`. It also pins that `body` in `closed` and
`expired`, and `open` in `run`, are each assigned once from `json.Marshal` of a struct literal
with exactly the expected fields, so no item data can reach a control event.

### The revocation bound, stated precisely

A decision's lifetime starts when its review answers (`decisionCache.put` after `decide`). A
permission revoked just after the API server evaluated the review is therefore honoured for the
lifetime plus the review's duration, at most the review timeout: 30 s + 5 s, about 35 s with the
defaults. `internal/authz` (`Check`, package doc) and `internal/stream` (package doc,
`revalidate`) say so. The change-stream scenario for a revoked permission adds one review to its
bound.
