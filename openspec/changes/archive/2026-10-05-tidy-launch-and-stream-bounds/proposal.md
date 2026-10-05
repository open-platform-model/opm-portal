## Why

Three reviews left follow-ups that bound how long things live past their purpose. The `--open`
launch page keeps the spent token on disk until shutdown (issue 21, item 2). A session that
expires does not end a stream it already opened, so a tab keeps receiving data past the session's
end; only new requests get `401` (issue 21, item 5). The stream broker's re-validation is capped
at three rounds of reviews, but each round can review every part of a message under a 5 s
timeout, so one message that never settles can keep a stream silent for about ten minutes, and a
cap of two passes the suite (issue 13, items 1 and 2). The write-path guard checks the event name
of the control events but not what they carry (issue 13, item 3), and the documented revocation
bound of 30 s leaves out the review the decision's lifetime starts after (issue 13, item 4).

## What Changes

- Local mode: the `--open` launch page and its directory are removed as soon as the launch token
  is spent, by either link, and still at shutdown when it never is.
- Stream: a stream ends when its session expires. It writes a final `expired` event,
  `{"code":"unauthenticated"}`, and is discarded, not kept for resume. A stream does not open for
  a session already expired. The read API's stream description and the reference page name the
  event; the page script stops the stream on it and shows "session expired, reload".
- Stream: re-validation of one message is bounded by time, one decision lifetime (30 s by
  default, `Options.RevalidateTimeout`), instead of a number of rounds; a review still running at
  the deadline is cancelled and the topic closes with `upstream_unavailable`. A test pins the
  bound by the exact number of reviews sent.
- Stream: `TestOnlySendWritesTopicData` pins every argument of each control event write (the
  open event's retry prefix and body, the heartbeat's literal `{}`, the closed event's
  topic-and-code body, the expired event's body), so no topic data can ride a control event.
- Docs: `internal/authz` and `internal/stream` state the revocation bound as one decision
  lifetime plus one review, about 35 s with the defaults.

## Capabilities

### Modified Capabilities

- `local-mode`: the launch page is removed once the token is spent.
- `change-stream`: a stream ends at session expiry; re-validation is bounded by time; the
  revocation bound is stated as lifetime plus one review.
- `read-authorization`: a revocation is seen within one lifetime plus one review.
- `web-ui`: the page shows an expired session and stops its stream.

## Impact

- Packages: `cmd/opm-portal` (launch page cleanup, session wiring), `internal/auth` (spent-token
  signal, `Authenticate` returns the session's expiry), `internal/api` (`Principal.Expires`, the
  stream's session), `internal/stream` (expiry, revalidation deadline, guard test, docs),
  `internal/authz` (docs), `internal/ui` (layout and script), `openapi/v1alpha1.yaml` and
  `docs/site` (the new event). No new read, no new verb, no new dependency.
- API: additive, one new stream event; `task api:breaking` stays green.
- Principle V: strengthened. An expired session stops receiving data on an open stream; nothing
  is read for it.
- SemVer: PATCH after 1.0 (a fix, plus an additive stream event); on the 0.x line it ships as
  `fix(stream)`.
