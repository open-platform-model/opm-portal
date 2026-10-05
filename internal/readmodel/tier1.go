package readmodel

import (
	"context"
	"errors"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-platform-model/opm-portal/internal/health"
)

// opmKind is one OPM kind held for the process: one informer per scope.
type opmKind struct {
	resource   schema.GroupVersionResource
	namespaced bool

	mu      sync.Mutex
	stopped bool // set by Stop; nothing starts afterwards
	scopes  []*opmScope
}

// opmScope is one namespace ("" for all) of an OPM kind. A nil watch means
// the reader may not list and watch the kind there, or that its review
// could not be made, which leaves the scope undecided: a later read asks
// again.
type opmScope struct {
	namespace string
	decided   bool
	watch     *watch
}

// opmKinds lists the four OPM kinds and whether each is namespaced.
var opmKinds = []struct {
	resource   schema.GroupVersionResource
	namespaced bool
}{
	{moduleInstances, true},
	{modulePackages, true},
	{platforms, false},
	{registrations, false},
}

// Start starts the informers on the four OPM kinds (tier 1): cluster-wide,
// or one per configured namespace for ModuleInstances and ModulePackages
// (portal:D5:R5). Each starts only after the reader's list and watch grants
// for its scope; a scope that is denied, or has not synced within
// SyncTimeout, leaves its kind unavailable there, and reads of it return
// ErrUnavailable until it syncs. A scope whose review could not be made is
// reviewed again by the next read that needs it. Start also reads the API
// server's version once (ServerVersion). Start returns an error only when
// called twice or after Stop.
func (m *Model) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started || m.stopped {
		m.mu.Unlock()
		return errors.New("read model: already started")
	}
	m.started = true
	m.mu.Unlock()

	var started []*watch
	for _, k := range opmKinds {
		kind := &opmKind{resource: k.resource, namespaced: k.namespaced}
		for _, ns := range m.scopesFor(k.namespaced) {
			scope := &opmScope{namespace: ns}
			switch m.readerWatchAccess(ctx, k.resource, ns) {
			case health.AccessOK:
				scope.decided = true
				scope.watch = m.startWatch(k.resource, ns, "")
				started = append(started, scope.watch)
			case health.AccessForbidden, health.AccessWithheld:
				scope.decided = true
			case health.AccessNotReadable:
			}
			kind.scopes = append(kind.scopes, scope)
		}
		m.mu.Lock()
		m.opm[k.resource] = kind
		m.mu.Unlock()
	}
	versionRead := m.readServerVersion(ctx)
	waitSynced(ctx, m.cfg.SyncTimeout, started...)
	select {
	case <-versionRead:
	case <-ctx.Done():
	}
	go m.janitor()
	return nil
}

// DeniedScope is a scope of an OPM kind that Start found the reader may
// not list and watch, so the kind's reads there answer forbidden.
type DeniedScope struct {
	// Resource is the kind's resource name, such as "moduleinstances".
	Resource string
	// Namespaced reports whether the kind is namespaced, so --namespaces
	// could narrow it to namespaces the reader may list.
	Namespaced bool
	// Namespace is the scope's namespace, "" for cluster-wide.
	Namespace string
}

// Denied returns the scopes Start found denied to the reader, in the order
// of the OPM kinds and their namespaces. A scope whose review could not be
// made is not denied: a later read reviews it again.
func (m *Model) Denied() []DeniedScope {
	var denied []DeniedScope
	for _, k := range opmKinds {
		kind := m.opmKind(k.resource)
		if kind == nil {
			continue
		}
		kind.mu.Lock()
		for _, s := range kind.scopes {
			if s.decided && s.watch == nil {
				denied = append(denied, DeniedScope{Resource: k.resource.Resource, Namespaced: k.namespaced, Namespace: s.namespace})
			}
		}
		kind.mu.Unlock()
	}
	return denied
}

// Stop stops every informer and poller. The Model serves nothing afterwards.
func (m *Model) Stop() {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	close(m.done)
	for _, k := range m.opm {
		k.mu.Lock()
		k.stopped = true
		for _, s := range k.scopes {
			s.watch.close()
		}
		k.mu.Unlock()
	}
	kinds := make([]*inventoryKind, 0, len(m.inventory))
	for _, k := range m.inventory {
		kinds = append(kinds, k)
	}
	held := make([]*childWatch, 0, len(m.children))
	for _, cw := range m.children {
		held = append(held, cw)
	}
	m.mu.Unlock()
	for _, k := range kinds {
		k.stop()
	}
	for _, cw := range held {
		<-cw.ready
		for _, w := range cw.watches {
			w.close()
		}
	}
}

func (m *Model) scopesFor(namespaced bool) []string {
	if !namespaced || len(m.cfg.Namespaces) == 0 {
		return []string{""}
	}
	return m.cfg.Namespaces
}

// heldIn returns the watches that hold kind in namespace ("" for every
// namespace the Model covers), or ErrUnavailable when any scope the read
// needs is denied to the reader, not synced, or not configured. A scope
// whose reader review could not be made at Start is reviewed again here,
// and its informer started and waited for when the reader is allowed.
func (m *Model) heldIn(ctx context.Context, resource schema.GroupVersionResource, namespace string) ([]*watch, error) {
	m.mu.Lock()
	kind := m.opm[resource]
	stopped := m.stopped
	m.mu.Unlock()
	if kind == nil || stopped {
		return nil, ErrUnavailable
	}
	scopes, fresh := m.decideScopes(ctx, kind, namespace)
	waitSynced(ctx, m.cfg.SyncTimeout, fresh...)
	var out []*watch
	for _, w := range scopes {
		if !w.synced() {
			return nil, ErrUnavailable
		}
		out = append(out, w)
	}
	if len(out) == 0 {
		return nil, ErrUnavailable
	}
	return out, nil
}

// decideScopes returns the watch of every scope of kind a read in namespace
// needs (nil where the reader may not watch), after reviewing again each
// undecided one. The reviews run without holding the kind's lock. It also
// returns the informers it started.
func (m *Model) decideScopes(ctx context.Context, kind *opmKind, namespace string) (scopes, fresh []*watch) {
	needed := func(s *opmScope) bool {
		return s.namespace == "" || namespace == "" || s.namespace == namespace
	}
	kind.mu.Lock()
	var undecided []string
	for _, s := range kind.scopes {
		if needed(s) && !s.decided {
			undecided = append(undecided, s.namespace)
		}
	}
	kind.mu.Unlock()

	access := make(map[string]health.Access, len(undecided))
	for _, ns := range undecided {
		access[ns] = m.readerWatchAccess(ctx, kind.resource, ns)
	}

	kind.mu.Lock()
	defer kind.mu.Unlock()
	for _, s := range kind.scopes {
		if !needed(s) {
			continue
		}
		if a, asked := access[s.namespace]; asked && !s.decided && !kind.stopped {
			switch a {
			case health.AccessOK:
				s.decided = true
				s.watch = m.startWatch(kind.resource, s.namespace, "")
				fresh = append(fresh, s.watch)
			case health.AccessForbidden, health.AccessWithheld:
				s.decided = true
			case health.AccessNotReadable:
			}
		}
		scopes = append(scopes, s.watch)
	}
	return scopes, fresh
}

// listHeld returns the held objects of kind in namespace, sorted.
func (m *Model) listHeld(ctx context.Context, resource schema.GroupVersionResource, namespace string) ([]*unstructured.Unstructured, error) {
	ws, err := m.heldIn(ctx, resource, namespace)
	if err != nil {
		return nil, err
	}
	var out []*unstructured.Unstructured
	for _, w := range ws {
		out = append(out, w.list(namespace)...)
	}
	sortObjects(out)
	return out, nil
}

// getHeld returns one held object of kind, or ErrNotFound.
func (m *Model) getHeld(ctx context.Context, resource schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	ws, err := m.heldIn(ctx, resource, namespace)
	if err != nil {
		return nil, err
	}
	for _, w := range ws {
		if u, ok := w.get(namespace, name); ok {
			return u, nil
		}
	}
	return nil, ErrNotFound
}
