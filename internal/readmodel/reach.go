package readmodel

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
)

// ErrNotReachable: no inventory object the caller may read reaches the Pod
// through workload ownership, or the Pod does not exist. Both get this one
// refusal, so it tells the caller nothing about Pods outside OPM.
var ErrNotReachable = errors.New("pod not reachable from an inventory")

// PodReach says how a Pod is reached from an inventory.
type PodReach struct {
	// Owner is the ModuleInstance or ModulePackage whose inventory reaches
	// the Pod.
	Owner ObjectRef
	// Via is the inventory object the Pod is a runtime child of.
	Via ObjectRef
}

// podLogRead is the read a log stream makes, and the read ReachPod's grant
// must cover.
func podLogRead(namespace, pod string) authz.Attributes {
	return authz.Attributes{Verb: "get", Resource: pods, Subresource: "log", Namespace: namespace, Name: pod}
}

// ReachPod answers which inventory object reaches Pod namespace/pod for the
// caller, through the runtime children the instance and package views carry
// (0030:D10:R1). g must cover get pods/log on the Pod; nothing is looked up
// otherwise. Only owners the caller may get, inventory objects the caller may
// read and children the caller may list count, so a Pod reached only through
// objects the caller may not read is ErrNotReachable, as are a Pod no inventory reaches and a Pod
// that does not exist. ErrUnavailable means the answer could not be read.
func (m *Model) ReachPod(ctx context.Context, who authz.Identity, g authz.Grant, namespace, pod string) (PodReach, error) {
	if err := g.Covers(who, podLogRead(namespace, pod)); err != nil {
		return PodReach{}, fmt.Errorf("%w: %w", ErrNotCovered, err)
	}
	ev := m.newEvaluator(who)
	children, access := ev.childrenIn(ctx, namespace)
	switch access {
	case health.AccessOK:
	case health.AccessNotReadable:
		return PodReach{}, ErrUnavailable
	case health.AccessForbidden, health.AccessWithheld:
		return PodReach{}, ErrNotReachable
	}
	instance, ok := podInstanceLabel(children, pod)
	if !ok {
		return PodReach{}, ErrNotReachable
	}
	// The label names the owner but not its namespace (0030:OQ12), so every
	// held owner of that name is a candidate.
	unavailable := false
	for _, resource := range []schema.GroupVersionResource{moduleInstances, modulePackages} {
		owners, err := m.listHeld(ctx, resource, "")
		if err != nil {
			unavailable = true
			continue
		}
		for _, u := range owners {
			if u.GetName() != instance {
				continue
			}
			// The owner is authorized before its inventory is read, so an
			// owner the caller may not get never decides the answer
			// (0030:D7:R1).
			if ev.m.callerAccess(ctx, who, "get", resource, u.GetNamespace(), u.GetName()) != health.AccessOK {
				continue
			}
			if via, ok := reachedVia(ev.inventoryHealth(ctx, u), namespace, pod); ok {
				return PodReach{Owner: refOf(u), Via: via}, nil
			}
		}
	}
	if unavailable {
		return PodReach{}, ErrUnavailable
	}
	return PodReach{}, ErrNotReachable
}

// childrenIn reads the runtime children of namespace for the evaluator's
// caller, once per evaluator.
func (e *evaluator) childrenIn(ctx context.Context, namespace string) ([]*unstructured.Unstructured, health.Access) {
	res, ok := e.children[namespace]
	if !ok {
		res.objects, res.access = e.m.childrenIn(ctx, e.who, namespace)
		e.children[namespace] = res
	}
	return res.objects, res.access
}

// podInstanceLabel returns the instance-name label of the Pod named pod
// among children.
func podInstanceLabel(children []*unstructured.Unstructured, pod string) (string, bool) {
	for _, c := range children {
		if c.GetKind() == kindPod && c.GetName() == pod {
			name := c.GetLabels()[instanceNameLabel]
			return name, name != ""
		}
	}
	return "", false
}

// reachedVia finds the inventory object whose runtime children include Pod
// namespace/pod. The view's children are attached only to objects the caller
// may read.
func reachedVia(ev evaluation, namespace, pod string) (ObjectRef, bool) {
	comps := componentsOf(ev)
	for i := range comps {
		for j := range comps[i].Objects {
			obj := &comps[i].Objects[j]
			for k := range obj.Children {
				if ref := obj.Children[k].Ref; ref.Kind == kindPod && ref.Namespace == namespace && ref.Name == pod {
					return obj.Ref, true
				}
			}
		}
	}
	return ObjectRef{}, false
}
