## Context

`main` has `internal/authz` (an `Authorizer` that answers with a sealed `Grant` or a
`*DenialError`, decision cache with a 30 s TTL, grants that expire with their decision) and
`internal/health`. The read model (`add-read-model`) is built in parallel and will feed the broker;
the read API (`add-read-api`) will mount the stream at `/api/v1alpha1/watch` with a companion
topic-change request. This change fixes the contract both depend on, without importing either.

The design draws on the portal architecture's stream section (one multiplexed stream per tab,
2 streams per session in local mode, 500 per process, per-topic ring of about 1000 entries,
heartbeat every 15 s, idle close after 30 min). Where that sketch and 0030 differ, 0030 wins; the
one deviation from the sketch (in-band snapshot instead of a bare `resync`) is argued below.

## Goals / Non-Goals

**Goals:**

- One stream per tab, topics added and removed while it stays open.
- No subscriber ever receives an item it may not read; a denial or an authorization error is
  never a delivery.
- No change is lost between a snapshot and the live feed; a short disconnect resumes.
- One stalled client cannot block publishers or hold unbounded memory.
- Deterministic tests, with the race detector, and no sleeps.

**Non-Goals:**

- HTTP routing, problem documents, CSRF (the read API mounts the handler).
- The producer (the read model) and HTML rendering (the UI).
- Log topics: the `log:` prefix is parsed and refused until `add-pod-log-streaming`.

## Decisions

### Package shape

```go
package stream

type Kind string // platform | instances | instance | package | registration | events | log
type Topic struct{ /* comparable, unexported */ }
func ParseTopic(s string) (Topic, error)          // *TopicError on a bad name
func (Topic) String() string
func (Topic) Kind() Kind
func (Topic) Namespace() string
func (Topic) Name() string
func (Topic) Ref() (Topic, bool)                  // events:<ref>

type Item struct {
    Event  string                // EventUpsert | EventDelete | EventK8sEvent
    Attrs  authz.Attributes      // the read this item reveals; gated per subscriber
    Data   json.RawMessage       // payload when it is the same for every allowed reader
    Render func(ctx context.Context, who authz.Identity) (json.RawMessage, error) // or per reader
}

type Producer interface {
    // Attributes are the reads a subscriber must be allowed before t attaches; ok=false
    // means the producer does not serve t. An empty slice is refused (fail closed).
    Attributes(t Topic) (attrs []authz.Attributes, ok bool)
    // Snapshot returns t's current items. Called after t is registered, so a change the
    // producer publishes after updating its state is never missed.
    Snapshot(ctx context.Context, t Topic) ([]Item, error)
    // Activate is called when t gets its first subscriber; release when it loses its last.
    Activate(t Topic) (release func())
}

type Session struct { Key string; Identity authz.Identity } // Key is never logged

type Options struct {
    MaxStreamsPerSession int           // 2
    MaxStreams           int           // 500
    MaxTopicsPerStream   int           // 32
    RingSize             int           // 1000 per topic
    QueueSize            int           // 256 per stream
    HeartbeatInterval    time.Duration // 15 s
    IdleTimeout          time.Duration // 30 min with no topics
    ResumeWindow         time.Duration // 1 min after a disconnect
    WriteTimeout         time.Duration // 10 s per write
    Logger               *slog.Logger
}

func New(p Producer, az authz.Authorizer, opts Options) *Broker
func (*Broker) Publish(t Topic, items ...Item) error
func (*Broker) Open(ctx context.Context, s Session, topics []Topic, lastEventID string) (*Stream, error)
func (*Broker) Subscribe(ctx context.Context, s Session, streamID string, topics ...Topic) error
func (*Broker) Unsubscribe(s Session, streamID string, topics ...Topic) error
func (*Broker) Close()
func (*Stream) ID() string
func (*Stream) Serve(ctx context.Context, w http.ResponseWriter) error

func NewHandler(b *Broker, session func(*http.Request) (Session, error), opts HandlerOptions) http.Handler
```

Errors: `ErrUnauthenticated`, `ErrTooManyStreams`, `ErrTooManyTopics`, `ErrNoStream`,
`ErrTopicNotServed`, `ErrClosed`, plus `*TopicError` from `ParseTopic`. The handler maps them to
401, 429, 400, 404, 400 and 503 through `HandlerOptions.Error`, which the read API replaces with
problem documents.

### Wire format

```
retry: 3000

event: open
data: {"stream":"<id>"}

id: <epoch>.<stream>.<seq>
event: snapshot
data: {"topic":"instance:apps/blog","items":[{...},{...}]}

id: <epoch>.<stream>.<seq>
event: upsert                     (or delete, k8sevent)
data: {"topic":"instance:apps/blog","item":{...}}

event: closed
data: {"topic":"instance:apps/blog","code":"forbidden"}   (or unauthenticated, upstream_unavailable)

event: heartbeat
data: {}
```

Data is compacted JSON, so a payload never spans lines. `open`, `closed` and `heartbeat` carry no
`id`, so they never move the client's `Last-Event-ID`. Codes reuse the read API's problem codes.

### Authorization

Verbs and resources: the broker reads nothing. It calls `Authorizer.Check` for the attributes the
producer names for a topic (for example `get moduleinstances` in `apps` named `blog`, or `list
events` in `apps`) and for each item's `Attrs`.

- **Subscribe** (`Open`, `Subscribe`, reattach): every topic attribute is checked synchronously,
  before the topic is registered and before `Activate`, so a caller cannot make the read model
  start watches for topics it may not read. A denied topic is not registered; the stream gets a
  `closed` message for it.
- **Before each delivery** the writer checks the topic's held grants with `Grant.Covers`; an
  expired grant is re-checked with `Check`. A denial or error closes the topic. The same check
  runs on each heartbeat, so a revocation closes a quiet topic within one decision TTL plus one
  heartbeat.
- **Per item**: the item is delivered when a topic grant covers `Item.Attrs`, otherwise after its
  own `Check` (cached by `authz`). Forbidden, unauthenticated or invalid: the item is skipped.
  Unavailable: the topic is closed with `upstream_unavailable`, because a silently skipped update
  would leave the client stale (Principle IV).
- An unauthenticated `Session.Identity` is refused at `Open` with no `Check` made.

### Ordering, snapshot and resume

One broker-wide sequence numbers every published item. Under the broker lock, `Publish` assigns
the next number, appends to the topic's ring and enqueues to each attached stream subscribed to
the topic. Attaching a topic allocates its own number for the snapshot marker under the same lock,
so every stream's queue is in strictly increasing order and the marker sits between what came
before registration and what came after. The writer calls `Producer.Snapshot` when it reaches the
marker; the producer updates its state before it publishes, so a change is in the snapshot, after
it, or both.

Event ids are `<epoch>.<stream id>.<seq>`; the epoch is random per broker, so ids from a previous
process never resume. A disconnected stream stays registered, detached, for `ResumeWindow`, keeping
its topic subscriptions (and so the topics' rings) alive. `Open` with a `Last-Event-ID` naming a
detached or still-attached stream of the same session reattaches it (a still-attached one is
taken over: its old writer ends). For each topic: a sequence below the topic's snapshot marker, or
a gap the ring no longer covers, gets a fresh snapshot; otherwise the ring's items after the id are
replayed. Anything else opens a fresh stream with the URL's topics.

### Bounds

- Per-stream queue (`QueueSize`): a full queue at `Publish` evicts the stream's connection without
  blocking; the stream detaches and stays resumable.
- `WriteTimeout` through `http.ResponseController.SetWriteDeadline` before each write, so a client
  that stops reading at the TCP level also ends its stream (unsupported writers are tolerated).
- Caps count attached and detached streams. At a cap, `Open` first discards the oldest detached
  stream in that scope (session or process), then refuses with `ErrTooManyStreams`.
- A topic is dropped, and its `release` called, when no stream (attached or detached) holds it.

## Research & Decisions

### How a reconnect learns it missed too much

**Context**: 0030:D2:R5 asks that a reconnect "resumes from its last event or is told to
re-read"; the architecture sketch sends a `resync` event and the client re-GETs.
**Explored**: the architecture's stream section; the ordering of a re-GET against the live stream.
**Options considered**:
1. `resync`, then the client re-GETs. A live upsert that arrives after the GET returns but was
   produced before the GET's read overwrites newer state with older; the client cannot order a GET
   response against stream ids.
2. A fresh `snapshot` in-band, at its own sequence number. Ordered against every later change by
   construction; the client replaces the topic's state.
**Decision**: option 2. A snapshot is the re-read the decision asks for, delivered where it can be
ordered.
**Rationale**: correctness of ordering; one fewer event type for clients.

### Where resume state lives

**Context**: `EventSource` reconnects to the URL it was opened with, so topics added later by the
topic-change request are not in that URL.
**Options considered**:
1. Resume per topic from the URL's topics only. Loses later-added topics silently.
2. Keep the stream, detached, for a resume window and name it in the event id.
**Decision**: option 2, bound to the session, with a 1 minute default window.
**Rationale**: the tab gets back exactly what it had; the id is opaque to clients anyway.

### Fake clock

**Context**: heartbeats, idle timeouts, the resume window and grant expiry are all time-driven,
and grant expiry is inside `authz`, which reads `time.Now`.
**Options considered**:
1. An injected clock interface in `stream`. Cannot move `authz`'s grant expiry.
2. `testing/synctest` (standard library since Go 1.25): every goroutine in the bubble sees a fake
   clock that advances only when all are blocked, `authz`'s included.
**Decision**: option 2 for every time-dependent test; plain `httptest.Server` tests cover the
wire with no timing.
**Rationale**: one fake clock for the broker and the grant it holds, no clock parameter in
production code, deterministic under `-race`.

## Risks / Trade-offs

- [A producer that publishes before updating its state can lose a change] → stated as the
  `Producer` contract, and the fake producer's race test asserts the documented order.
- [Per-item `Check` on every delivery] → `authz` caches decisions per identity and attributes for
  30 s; items covered by a topic grant skip the call.
- [Takeover by id within a session] → bounded to the same session key and identity; another
  session's id opens a fresh stream.
- [A topic denied by `Subscribe` while its stream is detached is closed without a message, since
  the closing has no sequence number to replay] → the topic is simply absent after the reconnect;
  the read API's topic-change request returns before the stream reconnects, so a client that
  needs certainty re-subscribes.
- [Per-reader `Render` runs in the writer] → it runs outside the broker lock, so a slow render
  delays only that stream; its error closes the topic.

## Open Questions

None for the owner. The read API change decides the route, the topic-change request and its
CSRF, and whether bearer clients reopen instead.
