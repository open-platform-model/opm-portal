package readmodel

import (
	"slices"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// ChangeKind is the OPM kind a Change names.
type ChangeKind string

// Change kinds: the four OPM kinds.
const (
	ChangeInstance     ChangeKind = "ModuleInstance"
	ChangePackage      ChangeKind = "ModulePackage"
	ChangePlatform     ChangeKind = "Platform"
	ChangeRegistration ChangeKind = "TransformerRegistration"
)

// Change names an OPM object whose view may have changed. It carries no
// object content: a listener learns only which view to render again, and
// rendering it takes the caller's grant like any other read.
type Change struct {
	Kind      ChangeKind
	Namespace string
	Name      string
	// Deleted: the OPM object itself was deleted.
	Deleted bool
	// Joined: the object's view changed through a provider join with
	// another object (a registration it holds, or an owner that holds or
	// held a registration); the object itself, and its events, did not
	// change. Never set with Deleted.
	Joined bool
}

// changeFeed holds the listeners OnChange registered.
type changeFeed struct {
	mu        sync.Mutex
	next      int
	listeners map[int]func(Change)
}

// OnChange registers fn to be told of every Change and returns a function
// that removes it. fn runs on an informer's goroutine and must not block;
// a listener that renders a view does so elsewhere.
func (m *Model) OnChange(fn func(Change)) (remove func()) {
	f := &m.feed
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listeners == nil {
		f.listeners = map[int]func(Change){}
	}
	id := f.next
	f.next++
	f.listeners[id] = fn
	var once sync.Once
	return func() {
		once.Do(func() {
			f.mu.Lock()
			defer f.mu.Unlock()
			delete(f.listeners, id)
		})
	}
}

func (m *Model) emit(changes []Change) {
	if len(changes) == 0 {
		return
	}
	m.feed.mu.Lock()
	fns := make([]func(Change), 0, len(m.feed.listeners))
	for _, fn := range m.feed.listeners {
		fns = append(fns, fn)
	}
	m.feed.mu.Unlock()
	for _, fn := range fns {
		for _, c := range changes {
			fn(c)
		}
	}
}

// changeHandler is registered on every informer startWatch builds. It maps
// each event to the OPM objects whose views the object feeds.
func (m *Model) changeHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { m.emit(m.changesFor(nil, obj, false)) },
		UpdateFunc: func(old, obj any) { m.emit(m.changesFor(old, obj, false)) },
		DeleteFunc: func(obj any) {
			if tomb, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = tomb.Obj
			}
			m.emit(m.changesFor(nil, obj, true))
		},
	}
}

// opmChangeKinds maps the OPM kinds to their change kinds.
var opmChangeKinds = map[string]ChangeKind{
	"ModuleInstance":          ChangeInstance,
	"ModulePackage":           ChangePackage,
	"Platform":                ChangePlatform,
	"TransformerRegistration": ChangeRegistration,
}

// changesFor returns the changes one informer event means; old is the
// object before an update, nil otherwise:
//
//   - an OPM object is itself changed (deleted is passed through), and a
//     registration also changes the Platform, whose view lists them;
//   - a registration also changes, joined, every held instance and package
//     whose inventory holds it, whose provider claims show its standing;
//   - an instance or package whose inventory holds a registration, before
//     or after the event, also changes the Platform, joined, whose
//     registrations name their holders;
//   - an object carrying the uuid label (an inventory object, tier 2)
//     changes every held instance or package whose status.inventory names
//     it; the label's value is not recorded on operator-owned instances, so
//     the inventory is the join;
//   - a runtime child, which carries the instance name label but no uuid
//     (capture, observation 8), changes every held instance or package of
//     that name.
func (m *Model) changesFor(old, obj any, deleted bool) []Change {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}
	var out []Change
	if kind, ok := opmChangeKinds[u.GetKind()]; ok && u.GroupVersionKind().Group == opmGroup {
		out = append(out, Change{Kind: kind, Namespace: u.GetNamespace(), Name: u.GetName(), Deleted: deleted})
		switch kind {
		case ChangeRegistration:
			out = append(out, Change{Kind: ChangePlatform, Name: platformName})
			name := u.GetName()
			for _, c := range m.heldOwners(func(o *unstructured.Unstructured) bool {
				return slices.Contains(heldRegistrations(o), name)
			}) {
				c.Joined = true
				out = append(out, c)
			}
		case ChangeInstance, ChangePackage:
			before, ok := old.(*unstructured.Unstructured)
			if holdsRegistration(u) || (ok && holdsRegistration(before)) {
				out = append(out, Change{Kind: ChangePlatform, Name: platformName, Joined: true})
			}
		case ChangePlatform:
		}
	}
	labels := u.GetLabels()
	if labels[instanceUUIDLabel] != "" {
		ref := refOf(u)
		out = append(out, m.heldOwners(func(o *unstructured.Unstructured) bool { return inventoryNames(o, ref) })...)
	} else if name := labels[instanceNameLabel]; name != "" {
		out = append(out, m.heldOwners(func(o *unstructured.Unstructured) bool { return o.GetName() == name })...)
	}
	return out
}

// heldOwners returns a change for every held ModuleInstance and
// ModulePackage that match accepts.
func (m *Model) heldOwners(match func(*unstructured.Unstructured) bool) []Change {
	var out []Change
	for _, k := range []struct {
		kind ChangeKind
		opm  *opmKind
	}{{ChangeInstance, m.opmKind(moduleInstances)}, {ChangePackage, m.opmKind(modulePackages)}} {
		if k.opm == nil {
			continue
		}
		k.opm.mu.Lock()
		watches := make([]*watch, 0, len(k.opm.scopes))
		for _, s := range k.opm.scopes {
			if s.watch != nil {
				watches = append(watches, s.watch)
			}
		}
		k.opm.mu.Unlock()
		for _, w := range watches {
			for _, o := range w.list("") {
				if match(o) {
					out = append(out, Change{Kind: k.kind, Namespace: o.GetNamespace(), Name: o.GetName()})
				}
			}
		}
	}
	return out
}

func (m *Model) opmKind(resource schema.GroupVersionResource) *opmKind {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.opm[resource]
}

// inventoryNames reports whether owner's status.inventory names ref.
func inventoryNames(owner *unstructured.Unstructured, ref ObjectRef) bool {
	for _, e := range maps(owner.Object, "status", "inventory", "entries") {
		if str(e, "group") == ref.Group && str(e, "kind") == ref.Kind &&
			str(e, "namespace") == ref.Namespace && str(e, "name") == ref.Name {
			return true
		}
	}
	return false
}
