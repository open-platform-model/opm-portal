package readmodel

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-platform-model/opm-portal/internal/health"
)

// inventoryKind is one inventory kind held on demand (tier 2). It is watched
// cluster-wide when the reader may, else per namespace where the reader
// may, else its objects are polled.
type inventoryKind struct {
	resource   schema.GroupVersionResource
	namespaced bool

	mu       sync.Mutex
	lastUsed time.Time
	decided  bool              // whether the cluster-wide grant was asked
	cluster  *watch            // the cluster-wide informer, when allowed
	perNS    map[string]*watch // per-namespace informers; nil when not allowed there
	poller   *poller           // objects the reader may get but not watch
}

// heldObject is one inventory object as a view reads it.
type heldObject struct {
	object      *unstructured.Unstructured // nil with AccessOK: read, not found
	access      health.Access
	evaluatedAt time.Time
	live        bool
}

// acquire returns the held kind, starting its informers for the namespaces
// a view needs on first use and recording the use, which keeps the kind from
// idling out. It returns the informers it started, which the caller waits
// for (once for every kind a view needs); one still syncing reads as not
// readable.
func (m *Model) acquire(ctx context.Context, kind resolvedKind, namespaces []string) (*inventoryKind, []*watch) {
	m.mu.Lock()
	k := m.inventory[kind.Resource]
	if k == nil {
		k = &inventoryKind{resource: kind.Resource, namespaced: kind.Namespaced, perNS: map[string]*watch{}}
		m.inventory[kind.Resource] = k
	}
	stopped := m.stopped
	m.mu.Unlock()
	if stopped {
		return k, nil
	}

	k.mu.Lock()
	k.lastUsed = m.cfg.Now()
	var fresh []*watch
	if !k.decided {
		k.decided = true
		if m.readerMayWatch(ctx, k.resource, "") {
			k.cluster = m.startWatch(k.resource, "", instanceUUIDLabel)
			fresh = append(fresh, k.cluster)
		}
	}
	if k.cluster == nil && k.namespaced {
		for _, ns := range namespaces {
			if _, asked := k.perNS[ns]; asked {
				continue
			}
			var w *watch
			if m.readerMayWatch(ctx, k.resource, ns) {
				w = m.startWatch(k.resource, ns, instanceUUIDLabel)
				fresh = append(fresh, w)
			}
			k.perNS[ns] = w
		}
	}
	k.mu.Unlock()
	return k, fresh
}

// watchFor returns the informer holding objects in namespace, or nil when
// the kind is not watched there.
func (k *inventoryKind) watchFor(namespace string) *watch {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.cluster != nil {
		return k.cluster
	}
	return k.perNS[namespace]
}

// lookup reads one inventory object from the kind's informer, or from its
// poller when the kind is not watched in the object's namespace.
func (m *Model) lookup(ctx context.Context, k *inventoryKind, ref health.Ref) heldObject {
	if w := k.watchFor(ref.Namespace); w != nil {
		if !w.synced() {
			return heldObject{access: health.AccessNotReadable}
		}
		obj, _ := w.get(ref.Namespace, ref.Name)
		return heldObject{object: obj, access: health.AccessOK, evaluatedAt: m.cfg.Now(), live: true}
	}
	return m.pollerFor(k).read(ctx, m, ref.Namespace, ref.Name)
}

// pollerFor returns the kind's poller, starting it on first use.
func (m *Model) pollerFor(k *inventoryKind) *poller {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.poller == nil {
		k.poller = newPoller(k.resource)
		go k.poller.run(m)
	}
	return k.poller
}

// stop stops the kind's informers and poller.
func (k *inventoryKind) stop() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.cluster.close()
	for _, w := range k.perNS {
		w.close()
	}
	if k.poller != nil {
		k.poller.close()
	}
}

func (k *inventoryKind) idleSince(cutoff time.Time) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.lastUsed.Before(cutoff)
}

// sweep stops every inventory kind no view has used for IdleTimeout. The
// next view that needs one starts it again.
func (m *Model) sweep() {
	cutoff := m.cfg.Now().Add(-m.cfg.IdleTimeout)
	m.mu.Lock()
	var idle []*inventoryKind
	for gvr, k := range m.inventory {
		if k.idleSince(cutoff) {
			idle = append(idle, k)
			delete(m.inventory, gvr)
		}
	}
	m.mu.Unlock()
	for _, k := range idle {
		k.stop()
	}
}

// janitor sweeps idle inventory kinds until the Model stops.
func (m *Model) janitor() {
	interval := m.cfg.IdleTimeout / 2
	if interval < time.Second {
		interval = time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-m.done:
			return
		case <-t.C:
			m.sweep()
		}
	}
}
