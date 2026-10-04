package readmodel

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// opmKind is one OPM kind held for the process: one informer per scope.
type opmKind struct {
	resource   schema.GroupVersionResource
	namespaced bool
	scopes     []opmScope
}

// opmScope is one namespace ("" for all) of an OPM kind. A nil watch means
// the reader may not list and watch the kind there.
type opmScope struct {
	namespace string
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
// (0030:D5:R5). Each starts only after the reader's list and watch grants
// for its scope; a scope that is denied, or has not synced within
// SyncTimeout, leaves its kind unavailable there, and reads of it return
// ErrUnavailable until it syncs. Start returns an error only when called
// twice or after Stop.
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
			scope := opmScope{namespace: ns}
			if m.readerMayWatch(ctx, k.resource, ns) {
				scope.watch = m.startWatch(k.resource, ns, "")
				started = append(started, scope.watch)
			}
			kind.scopes = append(kind.scopes, scope)
		}
		m.mu.Lock()
		m.opm[k.resource] = kind
		m.mu.Unlock()
	}
	waitSynced(ctx, m.cfg.SyncTimeout, started...)
	go m.janitor()
	return nil
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
		for _, s := range k.scopes {
			s.watch.close()
		}
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
// needs is denied to the reader, not synced, or not configured.
func (m *Model) heldIn(resource schema.GroupVersionResource, namespace string) ([]*watch, error) {
	m.mu.Lock()
	kind := m.opm[resource]
	stopped := m.stopped
	m.mu.Unlock()
	if kind == nil || stopped {
		return nil, ErrUnavailable
	}
	var out []*watch
	for _, s := range kind.scopes {
		if s.namespace != "" && namespace != "" && s.namespace != namespace {
			continue
		}
		if !s.watch.synced() {
			return nil, ErrUnavailable
		}
		out = append(out, s.watch)
	}
	if len(out) == 0 {
		return nil, ErrUnavailable
	}
	return out, nil
}

// listHeld returns the held objects of kind in namespace, sorted.
func (m *Model) listHeld(resource schema.GroupVersionResource, namespace string) ([]*unstructured.Unstructured, error) {
	ws, err := m.heldIn(resource, namespace)
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
func (m *Model) getHeld(resource schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	ws, err := m.heldIn(resource, namespace)
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
