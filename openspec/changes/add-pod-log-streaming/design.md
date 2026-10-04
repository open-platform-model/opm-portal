## Context

`main` has the stream broker (`internal/stream`): one `Producer` per broker, topics authorized on
the reads `Producer.Attributes` names, every snapshot and item written through one send funnel
that re-validates every grant it used. The broker parses `log:<ns>/<pod>/<container>` and refuses
it. The read model (`internal/readmodel`) computes runtime children (ReplicaSets, Pods, Jobs whose
controller chain reaches an inventory object) inside its instance and package views, for the
caller. The read API and its object-topic producer are built in parallel (P7) and are not
imported here: this change integrates only through `stream.Producer`.

Sources: 0030:D10 (contract), the V1 architecture's logs section (defaults: tail 200 lines, max
2000; 16 KiB line cap; 256 KiB/s per topic; 1 MiB initial-tail cap; 4 log topics per session, 50
per process). Where they disagree, 0030 wins.

## Goals / Non-Goals

**Goals:**

- A log line reaches a subscriber only through the broker's send funnel, so topic gating,
  revalidation, resume and eviction apply to logs exactly as to every other topic.
- Reachability and `pods/log` access both checked before any upstream byte is read.
- Bounded memory and bandwidth per topic, independent of what a container writes.
- Deterministic tests with a fake clientset, a fake log source and the real broker.

**Non-Goals:**

- The `/api/v1alpha1` routes (log topic endpoints, a Pod's container list as a document), the UI
  log panel, ANSI stripping (the UI's rendering concern).
- Multi-Pod merge, search, persistence.

## Decisions

### Reachability is a per-identity admission step in the broker

**Context**: The topic's read (`get pods/log`) is identity-free and static, but reachability
depends on the caller ("an inventory object the caller may read") and needs a lookup. 0030:D7
says authorize before lookup and give one denial for missing and forbidden.

**Options considered**:

1. Look reachability up inside `Producer.Attributes`. Runs before the caller is authorized
   (lookup before authorize) and `ok=false` surfaces as `ErrTopicNotServed`, a bad request that
   differs from `forbidden`: it would tell a caller without access which Pods exist.
2. Carry the inventory object's `get` as each log item's `Attrs`. A caller who may not read it
   gets an attached topic that silently never delivers: a degraded read shown as empty
   (Principle IV), and an unreachable Pod still opens an upstream stream.
3. An optional `stream.Admitter` interface the broker calls after a topic's reads are allowed
   and before it attaches, in the same `authorize` step Open, Subscribe and reattach share.

**Decision**: Option 3.

```go
// Admitter is an optional Producer extension.
type Admitter interface {
    // Admit decides whether who may follow t once t's reads are allowed; grants are those
    // reads' grants, in Attributes order. ErrNotAdmitted closes t with "forbidden", the code a
    // denial gives; a *authz.DenialError maps as a denial does; any other error closes t with
    // "upstream_unavailable".
    Admit(ctx context.Context, who authz.Identity, t Topic, grants []authz.Grant) error
}
var ErrNotAdmitted = errors.New("stream: topic not admitted")
```

**Rationale**: It keeps authorize-before-lookup (Admit runs only after the caller's `pods/log`
grant), one denial for every refusal, and reconnects re-run it (0030:D10:R2) because `reattach`
goes through `authorize` too. Delivery-time gating needs nothing new: log items carry the
topic's own read, so they ride the topic's grants through `send`.

### The read model answers reachability

```go
// ErrNotReachable: no inventory object the caller may read reaches the Pod, or the Pod does not
// exist. One refusal for both.
var ErrNotReachable = errors.New("pod not reachable from an inventory")

type PodReach struct {
    Owner ObjectRef // the ModuleInstance or ModulePackage
    Via   ObjectRef // the inventory object the Pod is a runtime child of
}

// g must cover get pods/log namespace/pod.
func (m *Model) ReachPod(ctx context.Context, who authz.Identity, g authz.Grant, namespace, pod string) (PodReach, error)
```

The Pod is found among the namespace's runtime children as the caller may list them; its
`module-instance.opmodel.dev/name` label names candidate instances and packages (any namespace,
from held state, since the label carries no namespace, 0030:OQ12); each candidate's inventory is
evaluated for the caller and the Pod must appear among an entry's runtime children. Not reached
and some kind unavailable is `ErrUnavailable`, not `ErrNotReachable` (Principle IV). Evidence: the
F1 capture's podinfo Deployment, ReplicaSet and Pods (`testdata/clusters/f1`).

### One producer per broker: a `Mux`

`stream.New` takes one `Producer`. `stream.Mux` maps `Kind` to `Producer` and delegates
`Attributes`, `Snapshot`, `Activate` and `Admit` (an unrouted kind is not served). The cmd wiring
passes `Mux{KindLog: logs, …object kinds: readmodel producer}`. **Alternative**: let `logs` wrap
the other producer; that couples `logs` to P7's code, which the task forbids.

### Topic grammar keeps `log:`, adds `/previous`

The broker, the main spec and the architecture all use `log:<ns>/<pod>/<container>`. The
supervisor's task text says `logs:`; renaming would change a published spec for no gain, so the
existing name stays. Previous-container logs are `log:<ns>/<pod>/<container>/previous`: a
separate topic, because its output is finite and differs from the live one.

### Upstream read, bounds and markers

```go
type Source interface {
    Pod(ctx context.Context, namespace, name string) (*corev1.Pod, error)
    Logs(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error)
}
func ClientSource(c kubernetes.Interface) Source // CoreV1().Pods(ns).Get / GetLogs(...).Stream
```

`PodLogOptions{Container, Follow: !previous, Previous: previous, Timestamps: true, TailLines:
cap}`. **`LimitBytes` is never set** (0030:D10: it ends a followed stream outright). The task text
asked for `limitBytes`; 0030:D10 is the contract and wins. Bounds in the reader instead:

| Bound | Default | Effect |
| --- | --- | --- |
| `TailLines` | 200, capped at 2000 | initial tail requested |
| `MaxLineBytes` | 16 KiB | longer line cut, `marker: "truncated"`, `cut: <bytes>` |
| `MaxTailBytes` | 1 MiB | tail lines (timestamp before the stream started) past it skipped; `skipped` marker with count before the first live line |
| `LinesPerSecond` / `LineBurst` | 200 / 500 | token bucket; dropped lines counted |
| `BytesPerSecond` / `ByteBurst` | 256 KiB / 1 MiB | token bucket; dropped lines counted |
| `Buffer` | 500 messages | recent messages a late subscriber's snapshot carries |

A line longer than the cap is read in bounded chunks and the rest discarded, so a newline-free
writer cannot grow memory. The `rate-limited` marker is emitted before the next admitted line,
and at the end of the stream when lines were still pending.

Wire shape (the payload of `log` and `logend` items; the read API's types will mirror it):

```jsonc
{"seq": 41, "type": "line", "time": "2026-10-04T10:00:00.123Z", "text": "GET /healthz 200"}
{"seq": 42, "type": "line", "time": "…", "text": "<16 KiB>", "marker": "truncated", "cut": 20480}
{"seq": 43, "type": "marker", "marker": "rate-limited", "dropped": 120}
{"seq": 44, "type": "marker", "marker": "skipped", "dropped": 900}
{"seq": 45, "type": "end", "reason": "container_stopped"}
```

End reasons: `container_stopped`, `completed` (previous output read), `upstream_closed` (EOF
while the container still runs), `container_not_found`, `pod_not_found`, `unavailable` (the
reader may not read, or the upstream failed). `seq` increases per activation; a snapshot and a
later message may repeat a line, and clients drop a `seq` they have.

### Authorization: verbs and grants

| Read | Who | When |
| --- | --- | --- |
| `get pods/log` ns/pod | caller | topic attach, every delivery after expiry, reconnect (broker) |
| `get pods/log` ns/pod (grant) | caller | `ReachPod` covers it before any lookup |
| `get` inventory objects, `list` pods, replicasets, jobs in ns | caller | inside `ReachPod` (view rules) |
| `get pods/log`, `get pods` ns/pod | reader | before the upstream `Pod` and `Logs` calls |

### Activation and close

`Activate` starts one goroutine per topic with its own context; the release cancels it, which
closes the upstream body, and waits for the goroutine to finish. The goroutine updates its buffer
before `Publish` (producer contract). It publishes through a `Publisher` interface set after the
broker exists (`SetPublisher`), so tests can also capture items. Portal logs carry the topic and
an error class, never line text.

### Log topic caps in the broker

`Options.MaxLogTopicsPerSession` (4) counts log subscriptions across a session's streams and
`Options.MaxLogTopics` (50) counts distinct active log topics; both are checked with the existing
topic cap, before any review, and refused with `ErrTooManyTopics`.

## Risks / Trade-offs

- [Reachability is checked at attach and reconnect, not on every delivery] → the `pods/log`
  grant, the actual RBAC boundary, is re-checked on every delivery; a Pod that leaves its
  inventory is replaced or deleted, which ends the upstream stream.
- [`ReachPod` evaluates every candidate inventory] → one evaluation per attach, from held state;
  label candidates are usually one.
- [Tail/live split uses timestamps] → a node clock skewed ahead classifies tail lines as live;
  the cost is that the skip cap is not applied, never lost data beyond the rate bound.
- [Mux and Admitter touch `internal/stream`, which P7 also consumes] → both are additive; a
  producer that implements neither behaves as before.

## Migration Plan

None: nothing mounts the stream yet.
