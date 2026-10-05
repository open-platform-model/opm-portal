package health

import (
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// Access says how reading an inventory object went.
type Access string

// Access values.
const (
	// AccessOK: the object was read, or read and found absent.
	AccessOK Access = "ok"
	// AccessForbidden: the reader may not read the object.
	AccessForbidden Access = "forbidden"
	// AccessNotReadable: the read failed for another reason (an unreachable
	// API server, an unserved kind).
	AccessNotReadable Access = "notReadable"
	// AccessWithheld: the portal never reads this kind (Secrets).
	AccessWithheld Access = "withheld"
)

// Ref identifies one inventory entry.
type Ref struct {
	Group     string
	Version   string
	Kind      string
	Namespace string
	Name      string
	Component string
}

// isSecret reports whether the entry names a core Secret, which the portal
// never reads.
func (r Ref) isSecret() bool {
	return r.Group == "" && r.Kind == "Secret"
}

// Entry is one inventory entry as the read model holds it.
type Entry struct {
	Ref    Ref
	Access Access
	// Object is the live object. Nil with AccessOK means read and not found.
	Object *unstructured.Unstructured
	// EvaluatedAt is when the object was last read.
	EvaluatedAt time.Time
	// Live is false when the object is refreshed by polling instead of a
	// watch.
	Live bool
}

// Input is everything one instance's health is computed from.
type Input struct {
	Entries []Entry
	// Children are runtime objects read below inventory workloads
	// (ReplicaSets, Pods, Jobs). They feed health only through the Pod rule.
	Children []*unstructured.Unstructured
	// ChildrenAccess says how reading the children went. Anything but
	// AccessOK, the zero value included, means the Pod rule could not run:
	// every readable workload that can own Pods is marked ChildrenUnread and
	// its component and the instance are partial (portal:D3:R4).
	ChildrenAccess Access
}

// ObjectResult is one inventory entry's evaluated health.
type ObjectResult struct {
	Ref         Ref
	Access      Access
	Health      ObjectHealth
	EvaluatedAt time.Time
	Live        bool
	// ChildrenUnread is set on a readable workload that can own Pods when
	// the children below it could not be read, so the Pod rule could not
	// run: its health is kstatus alone and may be better than the truth.
	ChildrenUnread bool
}

// Counts tallies a summary's entries by health state and by access.
type Counts struct {
	Healthy     int
	Progressing int
	Degraded    int
	Missing     int
	Unknown     int
	Forbidden   int
	NotReadable int
	Withheld    int
}

// Summary is the rolled-up health of a component or an instance.
type Summary struct {
	State  State
	Counts Counts
	// Partial is true when an entry the reader could not read was left out,
	// or when a counted workload's children could not be read. A withheld
	// Secret never makes a summary partial.
	Partial bool
	// EvaluatedAt is the oldest evaluation among the counted entries; zero
	// when nothing was counted.
	EvaluatedAt time.Time
	// Live is false when any counted entry is refreshed by polling.
	Live bool
}

// ComponentResult is one component's rolled-up health.
type ComponentResult struct {
	Name string
	Summary
}

// Result is an instance's health: per entry, per component, and overall.
type Result struct {
	Objects    []ObjectResult
	Components []ComponentResult
	Instance   Summary
}

// maxOwnerHops bounds the owner walk from a Pod up to an inventory object
// (Pod, ReplicaSet, Deployment is two hops; a CronJob chain is three).
const maxOwnerHops = 8

// Evaluate computes an instance's health from its inventory entries and
// the runtime children below them (portal:D3): per-object health, the Pod
// rule carried up to the inventory workload that owns the Pod, and a
// worst-of roll-up to components and the instance.
func Evaluate(in Input) Result {
	objects := make([]ObjectResult, len(in.Entries))
	byUID := make(map[types.UID]int, len(in.Entries))
	for i := range in.Entries {
		objects[i] = evaluateEntry(&in.Entries[i])
		if obj := in.Entries[i].Object; objects[i].Access == AccessOK && obj != nil {
			byUID[obj.GetUID()] = i
		}
	}
	if in.ChildrenAccess == AccessOK {
		propagatePodRule(objects, byUID, in)
	} else {
		markChildrenUnread(objects, in.Entries)
	}

	return Result{
		Objects:    objects,
		Components: components(objects),
		Instance:   summarize(objects),
	}
}

func evaluateEntry(e *Entry) ObjectResult {
	res := ObjectResult{Ref: e.Ref, Access: e.Access, EvaluatedAt: e.EvaluatedAt, Live: e.Live}
	switch {
	case e.Ref.isSecret():
		res.Access = AccessWithheld
		res.Health = ObjectHealth{State: Unknown, Message: "Secrets are never read"}
	case e.Access != AccessOK:
		res.Health = ObjectHealth{State: Unknown, Message: "object could not be read"}
	default:
		res.Health = Object(e.Object)
	}
	return res
}

// propagatePodRule marks the inventory object owning a broken Pod Degraded.
// The owning workload's own status can report it available for the whole
// progress deadline while its new Pod cannot start (portal:D3:R2).
func propagatePodRule(objects []ObjectResult, inventoryByUID map[types.UID]int, in Input) {
	childByUID := make(map[types.UID]*unstructured.Unstructured, len(in.Children))
	for _, c := range in.Children {
		childByUID[c.GetUID()] = c
	}
	for _, pod := range in.Children {
		broken, ok := brokenPod(pod)
		if !ok {
			continue
		}
		i, found := owningInventoryObject(pod, childByUID, inventoryByUID)
		if !found || objects[i].Health.State == Degraded {
			continue
		}
		objects[i].Health = broken
	}
}

// podOwnerKinds are the inventory kinds whose health the Pod rule can
// change: the built-in workloads that own Pods, directly or through a
// ReplicaSet or Job.
var podOwnerKinds = map[string]bool{
	"apps/Deployment":  true,
	"apps/ReplicaSet":  true,
	"apps/StatefulSet": true,
	"apps/DaemonSet":   true,
	"batch/Job":        true,
	"batch/CronJob":    true,
}

// markChildrenUnread flags every readable, present workload that can own
// Pods, because the Pod rule could not look below it.
func markChildrenUnread(objects []ObjectResult, entries []Entry) {
	for i := range objects {
		if objects[i].Access != AccessOK || entries[i].Object == nil {
			continue
		}
		if podOwnerKinds[objects[i].Ref.Group+"/"+objects[i].Ref.Kind] {
			objects[i].ChildrenUnread = true
		}
	}
}

// owningInventoryObject follows controller owner references by UID from obj
// until it reaches an inventory object.
func owningInventoryObject(obj *unstructured.Unstructured, children map[types.UID]*unstructured.Unstructured, inventory map[types.UID]int) (int, bool) {
	current := obj
	for range maxOwnerHops {
		owner, ok := controllerOwner(current)
		if !ok {
			return 0, false
		}
		if i, ok := inventory[owner]; ok {
			return i, true
		}
		next, ok := children[owner]
		if !ok {
			return 0, false
		}
		current = next
	}
	return 0, false
}

// controllerOwner returns the UID of the owner reference marked controller.
// An object with no controller reference has no owner the walk follows: a
// plain owner reference does not make its holder responsible for the Pod.
func controllerOwner(obj *unstructured.Unstructured) (types.UID, bool) {
	for _, r := range obj.GetOwnerReferences() {
		if r.Controller != nil && *r.Controller {
			return r.UID, true
		}
	}
	return "", false
}

func components(objects []ObjectResult) []ComponentResult {
	grouped := map[string][]ObjectResult{}
	for i := range objects {
		name := objects[i].Ref.Component
		grouped[name] = append(grouped[name], objects[i])
	}
	out := make([]ComponentResult, 0, len(grouped))
	for name, objs := range grouped {
		out = append(out, ComponentResult{Name: name, Summary: summarize(objs)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// summarize rolls entries up worst-of. Only readable entries count; an
// unreadable one is left out and makes the summary partial. With nothing
// counted the state is Unknown.
func summarize(objects []ObjectResult) Summary {
	s := Summary{State: Healthy, Live: true}
	counted := 0
	for i := range objects {
		o := &objects[i]
		switch o.Access {
		case AccessWithheld:
			s.Counts.Withheld++
			continue
		case AccessForbidden:
			s.Counts.Forbidden++
			s.Partial = true
			continue
		case AccessNotReadable:
			s.Counts.NotReadable++
			s.Partial = true
			continue
		case AccessOK:
		default:
			s.Counts.NotReadable++
			s.Partial = true
			continue
		}
		counted++
		if o.ChildrenUnread {
			s.Partial = true
		}
		s.Counts.add(o.Health.State)
		if o.Health.State.worse(s.State) {
			s.State = o.Health.State
		}
		if !o.Live {
			s.Live = false
		}
		if s.EvaluatedAt.IsZero() || o.EvaluatedAt.Before(s.EvaluatedAt) {
			s.EvaluatedAt = o.EvaluatedAt
		}
	}
	if counted == 0 {
		s.State = Unknown
		s.Live = false
	}
	return s
}

func (c *Counts) add(s State) {
	switch s {
	case Healthy:
		c.Healthy++
	case Progressing:
		c.Progressing++
	case Degraded:
		c.Degraded++
	case Missing:
		c.Missing++
	case Unknown:
		c.Unknown++
	default:
		c.Unknown++
	}
}
