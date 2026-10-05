## Context

The portal's milestone 1 (local mode) is done. Before milestone 2 (in-cluster, many users behind
one process) two answers from the portal's question register have to be built into the
read path: portal:OQ8 (the owner: hide operator message text in-cluster until the kernel redacts)
and portal:OQ20 (the supervisor: write a subscriber only a change to its own rendered document, in
every mode). Neither needs a new read, a new verb or a new identity.

## Goals / Non-Goals

**Goals:** no event reaches a subscriber whose rendered document did not change; an in-cluster
server serves no operator-written message text anywhere (JSON, stream, pages); both modes held
by goldens; the OpenAPI document says which fields are absent in-cluster.

**Non-Goals:** running the portal in-cluster (OIDC, SubjectAccessReview, the `serve` flag for
the mode): milestone 2 builds that. Redacting messages instead of dropping them. A page notice
explaining the missing text.

## Decisions

### The stream compares per subscription

`subscription` gains `sent []byte`, guarded by `Broker.mu`: the item data last written for that
subscription on its current connection. `Stream.eventID`, which already runs under the lock
right before every topic write, now takes the message and its document and decides:

```go
switch {
case m.snapshot:            // always written; sets the baseline
    sub.sent = doc          // the only item, or nil for 0 or 2+ items
case doc == nil:            // a log line: a record, never compared
case m.event != EventDelete && bytes.Equal(sub.sent, doc) && sub.sent != nil:
    return "", false        // nothing written, no id taken
default:
    sub.sent = doc          // a delete is always written, and sets it too
}
```

`doc` is the item's data for `upsert`, `delete` and `k8sevent` events. A `delete` is never left
out: an upsert may already have carried the object's `Removed` document, and the client drops the
object on the delete event itself (when the object went is its own, visible to every subscriber).
A `closed` event never passes through `eventID`, so it is never compared either. An item that is not
written takes no event id, so ids stay consecutive and no gap reveals it (as for a forbidden
item today). Each subscription also records `sentID`, the
stream event id `sent` was written under. `reattach` keeps `sent` when the client's
`Last-Event-ID` is at or after `sentID`, since the client then holds that document, so a replayed
item equal to it is not written and the reconnect does not reveal when a change the client cannot
see happened. Otherwise it sets `sent = nil`: the client may not hold what the old connection last
wrote (a write can succeed on the server and be lost on the way), so the first item is written.
When the resume cannot replay (the ring lost part of the gap, or the stream no longer remembers
the client's id), it writes a fresh snapshot, which sets `sent` like any snapshot, so later items
are compared with it.

The comparison is on the rendered bytes. Every read API document is `json.Marshal` output of a
struct with no wall-clock field (evaluation times come from the read model's clock and change
only when an object is evaluated again), so an unchanged view renders to the same bytes.

### Mode in the API config

```go
type Mode string

const (
    ModeLocal     Mode = "local"
    ModeInCluster Mode = "in-cluster"
)

type Config struct {
    // Mode is where the portal runs. Required.
    Mode Mode
    ...
}
```

`New` refuses an empty or unknown mode, so a caller that forgets it does not silently get the
mode that shows more. `opm-portal serve` passes `ModeLocal`.

### Omission at the edge of the server

One function, `omitOperatorText(doc any) (any, error)`, takes every wire document the server
writes and returns it without operator text. It runs in two places, the only two where a
document leaves the server: `Server.document` (every `GET`) and the stream producer's render.
The UI reads through `Server.document` in process, so it is covered by the first.

It switches on the document type (`Instance`, `InstanceList`, `Package`, `PackageList`,
`Platform`, `EventList`, `Graph`, `Object`, `Removed`) and returns an error for any other type, so
a document type added later fails its first request in-cluster instead of leaking; a test runs
it over every route's document type. It omits:

| Where | Field |
| --- | --- |
| `Condition` (conditions, reconcile notes) | `message` |
| `Reconcile` (summaries, details, graph nodes) | `message` |
| `HistoryEntry` | `message` |
| `Registration`, `GraphRegistration` | `message`, `activeMessage` |
| `Event` the operator reported (`reportingController` `opm-controller`, or none on an event about an `opmodel.dev` object) | `note` |
| `InventoryObject.health`, `GraphNode.health`, of an `opmodel.dev` object | `message` |
| `Object` of an `opmodel.dev` kind | `status.conditions[].message`, `status.history[].message` |

`reason`, `state`, `tone`, `meaning`, `nextStep` and every time stay. A non-OPM object's health
message (a Pod's waiting reason, a Deployment's progress) is the API server's and kubelet's text
about a workload, not the operator's, and stays; so do the notes of events the kubelet or another
controller reported. The owner's question was about the operator's messages, and the supervisor
ruled the omission to that scope (recorded under portal:D8): other writers' text is not a kernel
diagnostic, and it carries the remediation a user needs.

### Research & Decisions

#### Which fields carry operator text

**Context**: the owner's answer names condition messages, status history messages and event
notes. Other fields copy those.
**Explored**: `internal/health/applied.go` (`fromCondition` copies the deciding condition's
message into `Applied.Message`; `ReadRegistration` copies the Ready and Active messages);
`internal/health/object.go` (`status.Compute` from kstatus returns the `Ready` condition's
message for a generic resource, so an `opmodel.dev` inventory object's health message is the
operator's); `internal/api/objects.go` (an inventory can reach an OPM object: F1's
`default/backup-provider` instance reaches the TransformerRegistration
`default.backup-provider`, and its raw `status` holds condition messages).
**Options considered**:
1. Omit only `Condition.message`, `HistoryEntry.message`, `Event.note`. Leaves the same text in
   the reconcile message, the registration messages, kstatus health and the YAML view.
2. Omit every field that carries or copies operator text, as tabled above.
**Decision**: option 2.
**Rationale**: a copy leaks what the original would.

#### The reconcile message the portal writes itself

**Context**: `Reconcile.message` is the deciding condition's message, except in two cases where
the portal writes its own sentence ("no Ready condition yet", and the generation note on a stale
`Ready=True`).
**Options considered**:
1. Mark in `health.Applied` whether the message is the operator's, and keep the portal's own.
   A new field in a derived view for two sentences whose meaning the state already carries.
2. Omit `Reconcile.message` whole in-cluster.
**Decision**: option 2.
**Rationale**: fail safe and simple (Principle VII); the state (`Unknown`, `Reconciling`) says
what the two sentences say.

#### Omit or redact

**Options considered**:
1. Redact secret-looking substrings. The portal does not know which paths are secret (the
   markers live in the module schema, portal:D8), so any pattern is a guess.
2. Omit the text. The reason and the portal's explanation remain.
**Decision**: option 2, per the owner's answer.
**Rationale**: the portal cannot tell a secret from other text; the kernel can, and will.

#### Where the omission runs

**Options considered**:
1. Thread the mode into every conversion function in `convert.go`.
2. One function over the finished wire document, at the two exits.
**Decision**: option 2.
**Rationale**: one place to read and test; the conversions stay copies that decide nothing.

## Authorization

No new read. The mode changes what a document carries, not what is read or for whom; every
grant is unchanged.

## Risks / Trade-offs

- [In-cluster remediation is harder: "RenderFailed" without the kernel's error] → `meaning` and
  `nextStep` explain known reasons; the owner accepted this until the kernel redacts.
- [The stream holds one document per subscription] → bounded by the topic cap (32 per stream) and
  the stream cap; a document is what the stream already renders per subscriber.
- [A subscriber that missed a write keeps a stale view until the next difference] → a write
  error ends the connection, and the reconnect resets the comparison.
