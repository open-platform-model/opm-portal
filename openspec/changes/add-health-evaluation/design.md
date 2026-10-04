## Context

See proposal.md for why. The package is the derive layer of Principle II: it takes objects the
read model already holds and returns values; it reads nothing from a cluster, keeps no state and
starts no goroutine. Its callers arrive later (the read model computes instance health on every
watch event; the read API and the UI show both axes).

Evidence used throughout: the live capture of enhancement 0030, experiment 01 (Kubernetes
v1.36.1, opm-operator v1.0.0-beta.5, phases 3 to 6 plus the registration phases 7 to 9), copied
into `internal/health/testdata/`, and opm-operator `main` at 8d34b6b
(`internal/status/conditions.go`, `api/v1alpha1/*_types.go`, `go.mod`).

## Goals / Non-Goals

**Goals:**

- Object, component and instance health per 0030:D3, with the Pod rule the capture forced.
- The Applied axis for the four operator kinds, and registration verdicts per 0030:D4:R4/R7.
- One explanation per operator condition reason, kept in step with the operator by a test.
- Every claim about operator or kstatus output tested against a captured object.

**Non-Goals:**

- Wire types (`api/v1alpha1`) and their JSON names: the read API change owns them and maps
  these Go values onto them.
- Choosing which children to read below a workload, and when: the read model hands the
  children in. Watches versus polling, and the polling interval, are also the read model's.
- Docs URLs on explanations: the docs bundle does not exist yet.

## Research & Decisions

### Package shape

```go
package health

// Health axis.
type State string // Healthy | Progressing | Degraded | Missing | Unknown
type Access string // ok | forbidden | notReadable | withheld

type Ref struct{ Group, Version, Kind, Namespace, Name, Component string }

type Entry struct { // one inventory entry as the read model holds it
    Ref         Ref
    Access      Access                     // how the reader fared reading it
    Object      *unstructured.Unstructured // nil with AccessOK means not found
    EvaluatedAt time.Time
    Live        bool                       // false when refreshed by polling
}

type Input struct {
    Entries  []Entry
    Children []*unstructured.Unstructured // ReplicaSets, Pods, ... below inventory workloads
}

func Object(u *unstructured.Unstructured) ObjectHealth // kstatus plus the Pod rule
func Evaluate(in Input) Result                       // objects, components, instance

// Applied axis.
type AppliedState string // Applied | Reconciling | Failed | Stalled | Suspended | ManagedExternally | Unknown
func ReadApplied(u *unstructured.Unstructured) Applied
func ReadRegistration(u *unstructured.Unstructured) Registration

// Reasons.
func Explain(reason string) (Explanation, bool)
```

Inputs are `unstructured.Unstructured`, not the operator's typed API: the operator module is
not imported (its reason constants are internal anyway), kstatus takes unstructured, and the
read model's dynamic informers yield unstructured objects. Objects MUST be decoded the way
client-go decodes them (integers as `int64`); the spike found kstatus refuses a
`metadata.generation` decoded as `float64`, and the package maps any kstatus error to Unknown
with the error as the message rather than guessing.

### Object health: kstatus plus the Pod rule

**Context**: 0030:D3 names kstatus for per-object health and adds one Pod rule.
**Explored**: kstatus `Compute` run over every captured object (spike, section 1).
**Options considered**:
1. kstatus alone - refuted by the capture: one minute after the image break kstatus says the
   Deployment is `InProgress` ("Updated: 1/2"), and it turns `Failed` only at
   `ProgressDeadlineExceeded` (phase 5, about 600 s).
2. kstatus plus a Pod rule propagated to the owning inventory workload - what 0030:D3 decides.
3. Re-implementing per-kind rules - disagrees with the operator and with Flux, more code.
**Decision**: option 2, kstatus from `github.com/fluxcd/cli-utils` v1.2.2, the version
opm-operator's `go.mod` pins. Mapping: `Current` Healthy, `InProgress` Progressing, `Failed`
Degraded, `Terminating` Progressing, `NotFound` Missing, `Unknown` Unknown. An entry the reader
read and found absent is Missing.
**Rationale**: the decision record and the capture agree; kstatus already covers CRDs
(`Established`), Services, Namespaces, webhooks and RBAC (all `Current` in the cert-manager
capture).

The Pod rule: a Pod with any container or init container whose `state.waiting.reason` is one of
`ErrImagePull`, `ImagePullBackOff`, `CrashLoopBackOff`, `CreateContainerConfigError`,
`InvalidImageName` is Degraded, with the reason and the container's message. `Evaluate` walks
each degraded Pod's controller owner reference (the first owner reference when none is marked
controller) by UID through the children and the inventory objects, at most eight hops, until it
reaches an inventory object, and marks that object Degraded unless it already is. A Pod whose
chain reaches no inventory object changes nothing. Children are otherwise not counted: a
Deployment's own status already reflects its Pods, so counting both would double-count.

### Roll-up

Worst-of by severity `Degraded > Missing > Progressing > Unknown > Healthy`, objects to
components (the inventory entry's `component`; an entry without one goes under the empty
name) to the instance. Only entries with `Access=ok` count. `forbidden` and `notReadable`
entries are excluded and set `Partial`. A core `Secret` entry is `withheld` whatever the
caller passed: the package never evaluates one, and it neither counts nor makes the result
partial (0030:D3:R4). A summary with nothing counted is Unknown. `EvaluatedAt` is the oldest
evaluation among counted entries, and `Live` is false when any counted entry is not live
(0030:D3:R5). Counts per state and per non-ok access ride along for the list pages.

### Applied axis

**Context**: the operator's conditions say what it applied; 0030:D3 shows them as their own
axis. Evidence: `internal/status/conditions.go` (`MarkReconciling` sets `Reconciling=True` and
`Ready=Unknown`; `MarkStalled` sets `Stalled=True` and `Ready=False`; `MarkManagedExternally`
sets `Ready=Unknown/ManagedExternally`) and the captured samples.
**Options considered**:
1. Reconciling before Ready=False - would show the captured cert-manager `ApplyFailed`
   instance (`Reconciling=True` plus `Ready=False/ApplyFailed`, retrying) as merely
   reconciling, hiding the failure.
2. First match, in this order - chosen.
**Decision**: first match wins:
1. `spec.owner: cli`, or `Ready` reason `ManagedExternally`: ManagedExternally (neutral).
2. `spec.suspend: true`, or `Ready` reason `Suspended`: Suspended.
3. `Stalled=True`: Stalled, with the Stalled reason and message.
4. `Ready=False`: Failed, with `Retrying` set when `Reconciling=True`.
5. `Reconciling=True`, or `Ready=Unknown`: Reconciling.
6. `Ready=True`: Applied.
7. Otherwise (no Ready condition, an unknown kind): Unknown.
`Since` is the deciding condition's `lastTransitionTime`. Informational notes, which never move
the state: `ContractsFulfilled=False` on a Platform (normal on a fresh install, observation 11)
and `Drifted=True`. `status.failureCounters` is never read (observation 6, 0030:D3:R7).
**Rationale**: Stalled and Failed carry the reason a person acts on; ManagedExternally and
Suspended are deliberate states that would otherwise read as faults.

### Registration verdicts

From `status.accepted` and `status.active` (absent means false, observation 9), never from the
condition pair (0030:D4:R4). Verdict: `Ready` reason `DependentsRemain` is RemovalBlocked
(phase 9: accepted and active stay true, 0030:D4:R7); otherwise `accepted` is Accepted; otherwise
`Ready=False` is Refused with its reason; `Ready=Unknown` is Pending
(`ProviderInventoryPending` or reconciling); no `Ready` is Unknown. The `Active` condition's
reason and message ride along, because `ProviderNotReady` explains an accepted, inactive claim.

### Reason explanations

**Context**: a person seeing `ProvidesMismatch` needs to know what it means and what to do.
The operator's reason constants live in its `internal/status` package and cannot be imported.
**Options considered**:
1. Import the operator module - impossible for `internal/`, and the reasons are not in `api/`.
2. Parse the operator's diagnostics page - an outline today, every cell empty.
3. A copied list of the reason strings with a pointer to the source, plus an opt-in test that
   parses the source when a checkout is named by `OPM_OPERATOR_SRC` - chosen.
**Decision**: `reasons.go` holds `map[string]Explanation{Meaning, NextStep}`, keyed by the
reason string. A test fails when a copied operator reason has no row or a row names a reason
not in the copy. Event-only reasons (`Applied`, `Pruned`, `Resumed`, `NoOp`, `RenderWarning`)
are excluded: they are never a condition reason. Two reasons the operator writes as literals
(`Progressing` in `MarkReconciling` calls, `ModuleResolved`) are in the copy with a comment.
Success reasons carry a meaning and no next step. User-facing text cites no enhancement.

### Authorization

The package reads no Kubernetes resource and needs no grant. The read model decides what is
read, as whom; this package only records how a read went (`Access`) and never evaluates a
Secret it is handed.

## Risks / Trade-offs

- [The copied reason list drifts from the operator] → the opt-in parity test, run locally
  against a checkout; moving the constants into the operator's `api/` package (a proposed operator change)
  would turn the copy into an import.
- [Applied stays Applied while the operator works on a newer generation whose `Ready` was not
  rewritten yet] → not handled here; the operator rewrites `Ready=Unknown` when it starts
  reconciling, and the read API can expose `observedGeneration` beside the state.
- [The Pod rule's reason list is closed] → it is exactly the list 0030:D3 names; a new reason
  needs a decision, not a code edit.
- [A Pod mid-deletion with a waiting reason marks its workload Degraded for its grace period]
  → accepted: the rule as decided does not look at deletion, and the window is seconds.
