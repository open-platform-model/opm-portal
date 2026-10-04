// Package health derives the two status axes the portal shows side by side
// (0030:D3): workload health, computed from live objects, and applied state,
// read from the operator's conditions. Every function is pure: it evaluates
// the objects it is handed and reads nothing from a cluster.
package health

import (
	"fmt"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// State is the workload health of an object, a component or an instance.
type State string

// Health states. The roll-up orders them Degraded, Missing, Progressing,
// Unknown, Healthy, worst first.
const (
	Healthy     State = "Healthy"
	Progressing State = "Progressing"
	Degraded    State = "Degraded"
	Missing     State = "Missing"
	Unknown     State = "Unknown"
)

// rank orders states for the worst-of roll-up: a higher rank is worse.
func (s State) rank() int {
	switch s {
	case Degraded:
		return 4
	case Missing:
		return 3
	case Progressing:
		return 2
	case Unknown:
		return 1
	case Healthy:
		return 0
	default:
		return 1
	}
}

// worse reports whether s is worse than other.
func (s State) worse(other State) bool {
	return s.rank() > other.rank()
}

// Worst returns the worse of two states in the roll-up order, so callers
// grouping summaries roll them up the same way.
func Worst(a, b State) State {
	if b.worse(a) {
		return b
	}
	return a
}

// ObjectHealth is one object's health and what decided it.
type ObjectHealth struct {
	State State
	// Reason is a short machine-readable cause: a kstatus status or a
	// container waiting reason.
	Reason  string
	Message string
}

// brokenWaitingReasons are the container waiting reasons that mark a Pod
// Degraded at once. The capture showed an image-pull failure stays
// InProgress under kstatus until the progress deadline, about ten minutes,
// so these reasons are read directly (0030:D3:R2).
var brokenWaitingReasons = map[string]bool{
	"ErrImagePull":               true,
	"ImagePullBackOff":           true,
	"CrashLoopBackOff":           true,
	"CreateContainerConfigError": true,
	"InvalidImageName":           true,
}

// Object computes one object's health: the Pod rule for a Pod with a broken
// container, and the standard Kubernetes status rules (kstatus) otherwise.
func Object(u *unstructured.Unstructured) ObjectHealth {
	if u == nil {
		return ObjectHealth{State: Missing, Reason: string(status.NotFoundStatus), Message: "object not found"}
	}
	if broken, ok := brokenPod(u); ok {
		return broken
	}
	res, err := status.Compute(u)
	if err != nil {
		return ObjectHealth{State: Unknown, Reason: string(status.UnknownStatus), Message: fmt.Sprintf("computing status: %v", err)}
	}
	return ObjectHealth{State: fromKstatus(res.Status), Reason: string(res.Status), Message: res.Message}
}

func fromKstatus(s status.Status) State {
	switch s {
	case status.CurrentStatus:
		return Healthy
	case status.InProgressStatus, status.TerminatingStatus:
		return Progressing
	case status.FailedStatus:
		return Degraded
	case status.NotFoundStatus:
		return Missing
	case status.UnknownStatus:
		return Unknown
	default:
		return Unknown
	}
}

// brokenPod applies the Pod rule. It returns false for anything that is not
// a Pod with a container (or init container) waiting on a broken reason.
func brokenPod(u *unstructured.Unstructured) (ObjectHealth, bool) {
	if u.GetAPIVersion() != "v1" || u.GetKind() != "Pod" {
		return ObjectHealth{}, false
	}
	for _, field := range []string{"initContainerStatuses", "containerStatuses"} {
		statuses, _, err := unstructured.NestedSlice(u.Object, "status", field)
		if err != nil {
			continue
		}
		for _, raw := range statuses {
			cs, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			reason := stringAt(cs, "state", "waiting", "reason")
			if !brokenWaitingReasons[reason] {
				continue
			}
			name := stringAt(cs, "name")
			msg := stringAt(cs, "state", "waiting", "message")
			message := fmt.Sprintf("Pod %s container %s is waiting: %s", u.GetName(), name, reason)
			if msg != "" {
				message += ": " + msg
			}
			return ObjectHealth{State: Degraded, Reason: reason, Message: message}, true
		}
	}
	return ObjectHealth{}, false
}

// stringAt returns the string at the field path, or "" when the path is
// absent or holds something other than a string.
func stringAt(obj map[string]any, fields ...string) string {
	v, found, err := unstructured.NestedString(obj, fields...)
	if err != nil || !found {
		return ""
	}
	return v
}
