package readmodel

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
)

// childrenResult is one namespace's runtime children as one view read them.
type childrenResult struct {
	objects []*unstructured.Unstructured
	access  health.Access
}

// evaluator computes the health of the inventories one request reads. It
// remembers each namespace's children, so a list of instances reads them
// once per namespace.
type evaluator struct {
	m        *Model
	who      authz.Identity
	children map[string]childrenResult
}

func (m *Model) newEvaluator(who authz.Identity) *evaluator {
	return &evaluator{m: m, who: who, children: map[string]childrenResult{}}
}

// inventoryHealth reads every entry of owner's inventory the caller may
// read, from held state, and evaluates the instance's health (0030:D3). An
// entry the caller may not read is forbidden and never looked up; a Secret
// is withheld and never read (0030:D7:R3, 0030:D8:R1).
func (e *evaluator) inventoryHealth(ctx context.Context, owner *unstructured.Unstructured) health.Result {
	refs := inventory(owner)
	entries := make([]health.Entry, len(refs))
	byKind := map[resolvedKind][]int{}
	for i, ref := range refs {
		entries[i].Ref = ref
		kind, access := e.authorizeEntry(ctx, ref)
		if access != health.AccessOK {
			entries[i].Access = access
			continue
		}
		byKind[kind] = append(byKind[kind], i)
	}
	held := make(map[resolvedKind]*inventoryKind, len(byKind))
	var fresh []*watch
	for kind, idx := range byKind {
		k, started := e.m.acquire(ctx, kind, entryNamespaces(refs, idx))
		held[kind] = k
		fresh = append(fresh, started...)
	}
	waitSynced(ctx, e.m.cfg.SyncTimeout, fresh...)
	for kind, idx := range byKind {
		k := held[kind]
		for _, i := range idx {
			held := e.m.lookup(ctx, k, refs[i])
			entries[i].Object = held.object
			entries[i].Access = held.access
			entries[i].EvaluatedAt = held.evaluatedAt
			entries[i].Live = held.live
		}
	}
	children, access := e.childrenOf(ctx, owner.GetName(), entries)
	return health.Evaluate(health.Input{Entries: entries, Children: children, ChildrenAccess: access})
}

// authorizeEntry resolves an entry's kind and checks that the caller may
// get it. A Secret is withheld before any check.
func (e *evaluator) authorizeEntry(ctx context.Context, ref health.Ref) (resolvedKind, health.Access) {
	if ref.Group == "" && ref.Kind == "Secret" {
		return resolvedKind{}, health.AccessWithheld
	}
	kind, err := e.m.kinds.resolve(schema.GroupVersionKind{Group: ref.Group, Version: ref.Version, Kind: ref.Kind})
	if err != nil {
		return resolvedKind{}, health.AccessNotReadable
	}
	if isSecret(kind.Resource) {
		return resolvedKind{}, health.AccessWithheld
	}
	return kind, e.m.callerAccess(ctx, e.who, "get", kind.Resource, ref.Namespace, ref.Name)
}

func entryNamespaces(refs []health.Ref, idx []int) []string {
	seen := map[string]bool{}
	var out []string
	for _, i := range idx {
		ns := refs[i].Namespace
		if ns != "" && !seen[ns] {
			seen[ns] = true
			out = append(out, ns)
		}
	}
	return out
}

// childrenOf returns the runtime children labeled with the instance's name
// in every namespace holding a readable inventory object, and the worst
// access among those namespaces. With no such namespace there is nothing to
// read, which is not a failure.
func (e *evaluator) childrenOf(ctx context.Context, instance string, entries []health.Entry) ([]*unstructured.Unstructured, health.Access) {
	access := health.AccessOK
	var out []*unstructured.Unstructured
	seen := map[string]bool{}
	for i := range entries {
		obj := entries[i].Object
		if entries[i].Access != health.AccessOK || obj == nil || obj.GetNamespace() == "" || seen[obj.GetNamespace()] {
			continue
		}
		ns := obj.GetNamespace()
		seen[ns] = true
		res, ok := e.children[ns]
		if !ok {
			res.objects, res.access = e.m.childrenIn(ctx, e.who, ns)
			e.children[ns] = res
		}
		if res.access != health.AccessOK {
			access = res.access
			continue
		}
		for _, c := range res.objects {
			if c.GetLabels()[instanceNameLabel] == instance {
				out = append(out, c)
			}
		}
	}
	return out, access
}

// componentsOf groups an evaluated inventory by component, in the order the
// components first appear in the inventory.
func componentsOf(res health.Result) []Component {
	summaries := make(map[string]health.Summary, len(res.Components))
	for i := range res.Components {
		summaries[res.Components[i].Name] = res.Components[i].Summary
	}
	var out []Component
	index := map[string]int{}
	for i := range res.Objects {
		o := &res.Objects[i]
		at, ok := index[o.Ref.Component]
		if !ok {
			at = len(out)
			index[o.Ref.Component] = at
			out = append(out, Component{Name: o.Ref.Component, Health: summaries[o.Ref.Component]})
		}
		out[at].Objects = append(out[at].Objects, inventoryObject(o))
	}
	return out
}

// inventoryObject copies an evaluated entry into the view. An entry the
// caller may not read keeps only its reference and access.
func inventoryObject(o *health.ObjectResult) InventoryObject {
	io := InventoryObject{
		Ref:    ObjectRef{Group: o.Ref.Group, Version: o.Ref.Version, Kind: o.Ref.Kind, Namespace: o.Ref.Namespace, Name: o.Ref.Name},
		Access: o.Access,
	}
	if o.Access != health.AccessOK {
		return io
	}
	io.Health = o.Health
	io.ChildrenUnread = o.ChildrenUnread
	io.EvaluatedAt = o.EvaluatedAt
	io.Live = o.Live
	return io
}
