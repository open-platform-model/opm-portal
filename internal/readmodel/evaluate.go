package readmodel

import (
	"context"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

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

// evaluation is an evaluated inventory with, per entry, the runtime
// children below it.
type evaluation struct {
	health.Result
	children [][]RuntimeChild
}

// inventoryHealth reads every entry of owner's inventory the caller may
// read, from held state, and evaluates the instance's health (portal:D3). An
// entry the caller may not read is forbidden and never looked up; a Secret
// is withheld and never read (portal:D7:R3, portal:D8:R1).
func (e *evaluator) inventoryHealth(ctx context.Context, owner *unstructured.Unstructured) evaluation {
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
	res := health.Evaluate(health.Input{Entries: entries, Children: children, ChildrenAccess: access})
	if access != health.AccessOK {
		return evaluation{Result: res}
	}
	return evaluation{Result: res, children: runtimeChildren(entries, children)}
}

// maxOwnerHops bounds the controller walk from a child up to an inventory
// object; a CronJob, Job, Pod chain is the longest built-in one.
const maxOwnerHops = 8

// runtimeChildren returns, per entry, the children whose chain of
// controller owner references reaches it, sorted by kind and name. A child
// whose chain reaches no inventory object is left out.
func runtimeChildren(entries []health.Entry, children []*unstructured.Unstructured) [][]RuntimeChild {
	out := make([][]RuntimeChild, len(entries))
	inventoryByUID := make(map[types.UID]int, len(entries))
	for i := range entries {
		if obj := entries[i].Object; entries[i].Access == health.AccessOK && obj != nil {
			inventoryByUID[obj.GetUID()] = i
		}
	}
	childByUID := make(map[types.UID]*unstructured.Unstructured, len(children))
	for _, c := range children {
		childByUID[c.GetUID()] = c
	}
	for _, c := range children {
		owner, ok := controllerOf(c)
		if !ok {
			continue
		}
		i, ok := reachInventory(owner, childByUID, inventoryByUID)
		if !ok {
			continue
		}
		gvk := schema.FromAPIVersionAndKind(owner.APIVersion, owner.Kind)
		out[i] = append(out[i], RuntimeChild{
			Ref:        refOf(c),
			Owner:      ObjectRef{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind, Namespace: c.GetNamespace(), Name: owner.Name},
			Health:     health.Object(c),
			Replicas:   replicasOf(c),
			Containers: containersOf(c),
		})
	}
	for i := range out {
		sort.Slice(out[i], func(a, b int) bool {
			x, y := out[i][a].Ref, out[i][b].Ref
			if x.Kind != y.Kind {
				return x.Kind < y.Kind
			}
			return x.Name < y.Name
		})
	}
	return out
}

// containersOf returns a Pod's init container names, then its container
// names, in spec order, and nil for any other kind.
func containersOf(u *unstructured.Unstructured) []string {
	if u.GetAPIVersion() != "v1" || u.GetKind() != kindPod {
		return nil
	}
	var names []string
	for _, field := range []string{"initContainers", "containers"} {
		for _, c := range maps(u.Object, "spec", field) {
			if n := str(c, "name"); n != "" {
				names = append(names, n)
			}
		}
	}
	return names
}

// reachInventory follows controllers from owner until it reaches an
// inventory object.
func reachInventory(owner metav1.OwnerReference, children map[types.UID]*unstructured.Unstructured, inventory map[types.UID]int) (int, bool) {
	for range maxOwnerHops {
		if i, ok := inventory[owner.UID]; ok {
			return i, true
		}
		next, ok := children[owner.UID]
		if !ok {
			return 0, false
		}
		if owner, ok = controllerOf(next); !ok {
			return 0, false
		}
	}
	return 0, false
}

// controllerOf returns the owner reference marked controller.
func controllerOf(obj *unstructured.Unstructured) (metav1.OwnerReference, bool) {
	for _, r := range obj.GetOwnerReferences() {
		if r.Controller != nil && *r.Controller {
			return r, true
		}
	}
	return metav1.OwnerReference{}, false
}

// replicasOf returns a ReplicaSet's spec.replicas.
func replicasOf(u *unstructured.Unstructured) *int64 {
	if u.GetKind() != kindReplicaSet {
		return nil
	}
	n, ok := i64(u.Object, "spec", "replicas")
	if !ok {
		return nil
	}
	return &n
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
		objects, nsAccess := e.childrenIn(ctx, ns)
		if nsAccess != health.AccessOK {
			access = nsAccess
			continue
		}
		for _, c := range objects {
			if c.GetLabels()[instanceNameLabel] == instance {
				out = append(out, c)
			}
		}
	}
	return out, access
}

// componentsOf groups an evaluated inventory by component, in the order the
// components first appear in the inventory.
func componentsOf(ev evaluation) []Component {
	res := ev.Result
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
		obj := inventoryObject(o)
		if obj.Access == health.AccessOK && i < len(ev.children) {
			obj.Children = ev.children[i]
		}
		out[at].Objects = append(out[at].Objects, obj)
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
