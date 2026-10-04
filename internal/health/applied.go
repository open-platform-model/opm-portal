package health

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// AppliedState is what the operator says it applied. It is its own axis and
// never a health value: Ready=True means every apply succeeded, not that the
// workload runs (0030:D3).
type AppliedState string

// Applied states.
const (
	AppliedStateApplied           AppliedState = "Applied"
	AppliedStateReconciling       AppliedState = "Reconciling"
	AppliedStateFailed            AppliedState = "Failed"
	AppliedStateStalled           AppliedState = "Stalled"
	AppliedStateSuspended         AppliedState = "Suspended"
	AppliedStateManagedExternally AppliedState = "ManagedExternally"
	AppliedStateUnknown           AppliedState = "Unknown"
)

// IsFault reports whether the state is a failure a person should act on.
// ManagedExternally and Suspended are deliberate and neutral.
func (s AppliedState) IsFault() bool {
	return s == AppliedStateFailed || s == AppliedStateStalled
}

// Condition types and reasons the applied axis reads.
const (
	conditionReady              = "Ready"
	conditionReconciling        = "Reconciling"
	conditionStalled            = "Stalled"
	conditionActive             = "Active"
	conditionDrifted            = "Drifted"
	conditionContractsFulfilled = "ContractsFulfilled"

	reasonManagedExternally = "ManagedExternally"
	reasonSuspended         = "Suspended"
	reasonDependentsRemain  = "DependentsRemain"

	operatorGroup = "opmodel.dev"
)

// operatorKinds are the kinds whose conditions the applied axis knows.
var operatorKinds = map[string]bool{
	"ModuleInstance":          true,
	"ModulePackage":           true,
	"Platform":                true,
	"TransformerRegistration": true,
}

// Condition is one status condition as the operator wrote it.
type Condition struct {
	Type               string
	Status             metav1.ConditionStatus
	Reason             string
	Message            string
	LastTransitionTime time.Time
	ObservedGeneration int64
}

// Note is an informational condition shown beside the applied state. It
// never changes the state.
type Note = Condition

// Applied is an operator object's applied state and the condition that
// decided it.
type Applied struct {
	State   AppliedState
	Reason  string
	Message string
	// Since is the deciding condition's last transition.
	Since time.Time
	// Retrying is set on a Failed state the operator is still retrying
	// (Reconciling=True beside Ready=False).
	Retrying bool
	// Notes are informational conditions: ContractsFulfilled=False on a
	// Platform, which is normal on a fresh install, and Drifted=True.
	Notes []Note
}

// ReadApplied reads the applied state of a ModuleInstance, ModulePackage,
// Platform or TransformerRegistration from its conditions and spec. The
// first match wins: a CLI owner, suspension, Stalled, Ready=False,
// Reconciling or Ready=Unknown, Ready=True. Failure counters are never read:
// the drift counter climbs on healthy instances.
func ReadApplied(u *unstructured.Unstructured) Applied {
	if u == nil || u.GroupVersionKind().Group != operatorGroup || !operatorKinds[u.GetKind()] {
		return Applied{State: AppliedStateUnknown, Message: "not an OPM operator kind"}
	}
	conds := conditions(u)
	ready, hasReady := conds[conditionReady]
	stalled := conds[conditionStalled]
	reconciling := conds[conditionReconciling]
	notes := appliedNotes(conds)

	owner := stringAt(u.Object, "spec", "owner")
	suspend := boolAt(u.Object, "spec", "suspend")

	switch {
	case owner == "cli" || ready.Reason == reasonManagedExternally:
		return fromCondition(AppliedStateManagedExternally, ready, notes)
	case suspend || ready.Reason == reasonSuspended:
		return fromCondition(AppliedStateSuspended, ready, notes)
	case stalled.Status == metav1.ConditionTrue:
		return fromCondition(AppliedStateStalled, stalled, notes)
	case ready.Status == metav1.ConditionFalse:
		a := fromCondition(AppliedStateFailed, ready, notes)
		a.Retrying = reconciling.Status == metav1.ConditionTrue
		return a
	case reconciling.Status == metav1.ConditionTrue:
		return fromCondition(AppliedStateReconciling, reconciling, notes)
	case ready.Status == metav1.ConditionUnknown:
		return fromCondition(AppliedStateReconciling, ready, notes)
	case ready.Status == metav1.ConditionTrue:
		return fromCondition(AppliedStateApplied, ready, notes)
	case !hasReady:
		return Applied{State: AppliedStateUnknown, Message: "no Ready condition yet", Notes: notes}
	default:
		return fromCondition(AppliedStateUnknown, ready, notes)
	}
}

func fromCondition(state AppliedState, c Condition, notes []Note) Applied {
	return Applied{State: state, Reason: c.Reason, Message: c.Message, Since: c.LastTransitionTime, Notes: notes}
}

func appliedNotes(conds map[string]Condition) []Note {
	var notes []Note
	if c, ok := conds[conditionContractsFulfilled]; ok && c.Status == metav1.ConditionFalse {
		notes = append(notes, c)
	}
	if c, ok := conds[conditionDrifted]; ok && c.Status == metav1.ConditionTrue {
		notes = append(notes, c)
	}
	return notes
}

// conditions decodes status.conditions by type. A malformed entry is
// skipped: the axis then reports what is left, down to Unknown.
func conditions(u *unstructured.Unstructured) map[string]Condition {
	raw, found, err := unstructured.NestedSlice(u.Object, "status", "conditions")
	if err != nil || !found {
		return map[string]Condition{}
	}
	out := make(map[string]Condition, len(raw))
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		c := Condition{
			Type:    stringAt(m, "type"),
			Status:  metav1.ConditionStatus(stringAt(m, "status")),
			Reason:  stringAt(m, "reason"),
			Message: stringAt(m, "message"),
		}
		if c.Type == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, stringAt(m, "lastTransitionTime")); err == nil {
			c.LastTransitionTime = t
		}
		if g, found, err := unstructured.NestedInt64(m, "observedGeneration"); err == nil && found {
			c.ObservedGeneration = g
		}
		out[c.Type] = c
	}
	return out
}

// Verdict is a TransformerRegistration's standing.
type Verdict string

// Registration verdicts.
const (
	VerdictAccepted       Verdict = "Accepted"
	VerdictRefused        Verdict = "Refused"
	VerdictPending        Verdict = "Pending"
	VerdictRemovalBlocked Verdict = "RemovalBlocked"
	VerdictUnknown        Verdict = "Unknown"
)

// Registration is a TransformerRegistration's acceptance, activation and
// verdict.
type Registration struct {
	// Accepted and Active come from status.accepted and status.active; an
	// absent field is false.
	Accepted bool
	Active   bool
	Verdict  Verdict
	// Reason and Message are the Ready condition's.
	Reason  string
	Message string
	// ActiveReason and ActiveMessage are the Active condition's, which
	// explain an accepted claim that is not active yet.
	ActiveReason  string
	ActiveMessage string
}

// ReadRegistration reads a TransformerRegistration's standing from its status
// fields, never from the Stalled and Ready pair alone: the same pair marks a
// refused claim and an accepted, active claim whose removal is blocked by
// dependents (0030:D4:R4/R7).
func ReadRegistration(u *unstructured.Unstructured) Registration {
	if u == nil || u.GroupVersionKind().Group != operatorGroup || u.GetKind() != "TransformerRegistration" {
		return Registration{Verdict: VerdictUnknown, Message: "not a TransformerRegistration"}
	}
	conds := conditions(u)
	ready, hasReady := conds[conditionReady]
	active := conds[conditionActive]
	r := Registration{
		Accepted:      boolAt(u.Object, "status", "accepted"),
		Active:        boolAt(u.Object, "status", "active"),
		Reason:        ready.Reason,
		Message:       ready.Message,
		ActiveReason:  active.Reason,
		ActiveMessage: active.Message,
	}
	switch {
	case ready.Reason == reasonDependentsRemain:
		r.Verdict = VerdictRemovalBlocked
	case r.Accepted:
		r.Verdict = VerdictAccepted
	case ready.Status == metav1.ConditionFalse:
		r.Verdict = VerdictRefused
	case ready.Status == metav1.ConditionUnknown:
		r.Verdict = VerdictPending
	case !hasReady:
		r.Verdict = VerdictUnknown
	default:
		// Ready=True without status.accepted contradicts the operator's
		// contract; say so instead of choosing.
		r.Verdict = VerdictUnknown
	}
	return r
}

// boolAt returns the bool at the field path, false when absent or not a bool.
func boolAt(obj map[string]any, fields ...string) bool {
	v, found, err := unstructured.NestedBool(obj, fields...)
	if err != nil || !found {
		return false
	}
	return v
}
