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

	// lastUsed is guarded by the Model's mu, so the janitor decides idleness
	// and acquire records a use without waiting on mu below, which is held
	// only for bookkeeping, never across an access review.
	lastUsed time.Time

	mu      sync.Mutex
	stopped bool              // set by stop; nothing starts afterwards
	decided bool              // whether the reader was allowed or denied cluster-wide
	cluster *watch            // the cluster-wide informer, when allowed
	perNS   map[string]*watch // per-namespace informers; nil when not allowed there
	poller  *poller           // objects the reader may get but not watch
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
// idling out. It returns the informers the view reads from, which the caller
// waits for (once for every kind a view needs); one still syncing reads as
// not readable. The reader's access reviews run without holding the kind's
// lock, so a slow review delays only the view that asked.
func (m *Model) acquire(ctx context.Context, kind resolvedKind, namespaces []string) (*inventoryKind, []*watch) {
	k := m.useKind(kind)
	askCluster, askNS, ok := k.undecided(namespaces)
	if !ok {
		return k, nil
	}
	// A review that could not be made decides nothing: the next view asks
	// again instead of settling for a narrower scope.
	cluster := health.AccessNotReadable
	if askCluster {
		cluster = m.readerWatchAccess(ctx, k.resource, "")
	}
	perNS := map[string]health.Access{}
	if cluster != health.AccessOK {
		for _, ns := range askNS {
			perNS[ns] = m.readerWatchAccess(ctx, k.resource, ns)
		}
	}
	return k, m.install(k, askCluster, cluster, perNS, namespaces)
}

// useKind returns the held kind, creating it on first use, and records the
// use under the Model's lock, where the janitor decides idleness. After
// Stop it returns a stopped kind that is not held.
func (m *Model) useKind(kind resolvedKind) *inventoryKind {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.inventory[kind.Resource]
	if k == nil {
		k = &inventoryKind{resource: kind.Resource, namespaced: kind.Namespaced, perNS: map[string]*watch{}}
		if m.stopped {
			k.stopped = true
		} else {
			m.inventory[kind.Resource] = k
		}
	}
	k.lastUsed = m.cfg.Now()
	return k
}

// undecided returns whether the cluster-wide scope still needs a review,
// and which of namespaces do. ok is false once the kind has stopped.
func (k *inventoryKind) undecided(namespaces []string) (askCluster bool, askNS []string, ok bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.stopped {
		return false, nil, false
	}
	if k.cluster == nil && k.namespaced {
		for _, ns := range namespaces {
			if _, asked := k.perNS[ns]; !asked {
				askNS = append(askNS, ns)
			}
		}
	}
	return !k.decided, askNS, true
}

// install records the reviews' answers, starting the informers they allow
// unless another view already decided the scope or the kind has stopped,
// and returns the informers serving namespaces.
func (m *Model) install(k *inventoryKind, askedCluster bool, cluster health.Access, perNS map[string]health.Access, namespaces []string) []*watch {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.stopped {
		return nil
	}
	if askedCluster && !k.decided {
		switch cluster {
		case health.AccessOK:
			k.decided = true
			k.cluster = m.startWatch(k.resource, "", instanceUUIDLabel)
			// The cluster-wide informer replaces any per-namespace ones
			// started while the cluster-wide review could not be made.
			for ns, w := range k.perNS {
				w.close()
				delete(k.perNS, ns)
			}
		case health.AccessForbidden, health.AccessWithheld:
			k.decided = true
		case health.AccessNotReadable:
		}
	}
	if k.cluster != nil {
		return []*watch{k.cluster}
	}
	var reads []*watch
	for _, ns := range namespaces {
		if _, asked := k.perNS[ns]; !asked {
			switch perNS[ns] {
			case health.AccessOK:
				k.perNS[ns] = m.startWatch(k.resource, ns, instanceUUIDLabel)
			case health.AccessForbidden, health.AccessWithheld:
				k.perNS[ns] = nil
			case health.AccessNotReadable:
			}
		}
		if w := k.perNS[ns]; w != nil {
			reads = append(reads, w)
		}
	}
	return reads
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
	p := m.pollerFor(k)
	if p == nil {
		return heldObject{access: health.AccessNotReadable, evaluatedAt: m.cfg.Now()}
	}
	return p.read(ctx, m, ref.Namespace, ref.Name)
}

// pollerFor returns the kind's poller, starting it on first use, or nil
// once the kind has stopped.
func (m *Model) pollerFor(k *inventoryKind) *poller {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.stopped {
		return nil
	}
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
	k.stopped = true
	k.cluster.close()
	for _, w := range k.perNS {
		w.close()
	}
	if k.poller != nil {
		k.poller.close()
	}
}

// sweep stops every inventory kind no view has used for IdleTimeout, and
// drops expired on-demand lists of runtime children. The next view that
// needs one starts or lists it again.
func (m *Model) sweep() {
	cutoff := m.cfg.Now().Add(-m.cfg.IdleTimeout)
	m.mu.Lock()
	var idle []*inventoryKind
	for gvr, k := range m.inventory {
		if k.lastUsed.Before(cutoff) {
			idle = append(idle, k)
			delete(m.inventory, gvr)
		}
	}
	now := m.cfg.Now()
	for ns, listing := range m.childList {
		if !now.Before(listing.at.Add(m.cfg.ChildrenTTL)) {
			delete(m.childList, ns)
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
