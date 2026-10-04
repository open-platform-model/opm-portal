package readmodel

import (
	"context"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
)

// The runtime children below inventory workloads (tier 3), which the Pod
// rule reads (0030:D3:R2).
var (
	pods        = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	replicaSets = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	jobs        = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}

	childResources = []schema.GroupVersionResource{replicaSets, pods, jobs}
)

// childWatch holds the runtime-children informers of one namespace while
// anyone holds interest in it.
type childWatch struct {
	holders int
	ready   chan struct{} // closed once the watches below are set
	watches []*watch      // one per child resource; nil where not allowed
}

// childListing is an on-demand list of one namespace's runtime children.
type childListing struct {
	at      time.Time
	objects []*unstructured.Unstructured
	access  health.Access
}

// HoldChildren watches the runtime children (ReplicaSets, Pods, Jobs) in
// namespace until the returned release is called; a stream subscriber holds
// it while an instance page is open. g must cover watch pods in namespace,
// which is the gate for the whole hold: the reader also watches
// ReplicaSets and Jobs there, but no view shows a child kind the caller may
// not list, because childrenIn authorizes each kind for the caller on every
// read. Interest is counted: the watches stop when the last holder
// releases. Release is safe to call more than once.
func (m *Model) HoldChildren(ctx context.Context, who authz.Identity, g authz.Grant, namespace string) (release func(), err error) {
	if err := covers(who, g, "watch", pods, namespace, ""); err != nil {
		return nil, err
	}
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return nil, ErrUnavailable
	}
	cw := m.children[namespace]
	first := cw == nil
	if first {
		cw = &childWatch{ready: make(chan struct{})}
		m.children[namespace] = cw
	}
	cw.holders++
	m.mu.Unlock()

	if first {
		// The watches serve every holder, so the first holder going away
		// must not leave them unreviewed; the authorizer bounds each review.
		review := context.WithoutCancel(ctx)
		watches := make([]*watch, len(childResources))
		for i, r := range childResources {
			if m.readerMayWatch(review, r, namespace) {
				watches[i] = m.startWatch(r, namespace, instanceNameLabel)
			}
		}
		cw.watches = watches
		close(cw.ready)
		waitSynced(ctx, m.cfg.SyncTimeout, watches...)
	}
	var once sync.Once
	return func() { once.Do(func() { m.releaseChildren(namespace, cw) }) }, nil
}

func (m *Model) releaseChildren(namespace string, cw *childWatch) {
	m.mu.Lock()
	cw.holders--
	last := cw.holders == 0
	if last && m.children[namespace] == cw {
		delete(m.children, namespace)
	}
	m.mu.Unlock()
	if !last {
		return
	}
	<-cw.ready
	for _, w := range cw.watches {
		w.close()
	}
}

// childrenIn returns the runtime children in namespace for a view, and how
// reading them went. The caller must be allowed to list each child kind
// there. They come from the namespace's watches while someone holds
// interest, and otherwise from an on-demand list reused for ChildrenTTL.
func (m *Model) childrenIn(ctx context.Context, who authz.Identity, namespace string) ([]*unstructured.Unstructured, health.Access) {
	for _, r := range childResources {
		if access := m.callerAccess(ctx, who, "list", r, namespace, ""); access != health.AccessOK {
			return nil, access
		}
	}
	if objs, ok := m.watchedChildren(namespace); ok {
		return objs, health.AccessOK
	}
	return m.listedChildren(ctx, namespace)
}

// watchedChildren returns the children from the namespace's watches when
// every one of them runs and has synced.
func (m *Model) watchedChildren(namespace string) ([]*unstructured.Unstructured, bool) {
	m.mu.Lock()
	cw := m.children[namespace]
	m.mu.Unlock()
	if cw == nil {
		return nil, false
	}
	select {
	case <-cw.ready:
	default:
		return nil, false
	}
	var out []*unstructured.Unstructured
	for _, w := range cw.watches {
		if !w.synced() {
			return nil, false
		}
		out = append(out, w.list(namespace)...)
	}
	return out, true
}

// listedChildren lists the namespace's children as the reader, at most once
// per ChildrenTTL.
func (m *Model) listedChildren(ctx context.Context, namespace string) ([]*unstructured.Unstructured, health.Access) {
	now := m.cfg.Now()
	m.mu.Lock()
	cached, ok := m.childList[namespace]
	m.mu.Unlock()
	if ok && now.Before(cached.at.Add(m.cfg.ChildrenTTL)) {
		return cached.objects, cached.access
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	listing := childListing{at: now, access: health.AccessOK}
	for _, r := range childResources {
		if !m.readerMay(ctx, "list", r, namespace, "") {
			listing = childListing{at: now, access: health.AccessNotReadable}
			break
		}
		list, err := m.cfg.Dynamic.Resource(r).Namespace(namespace).List(ctx, metav1.ListOptions{LabelSelector: instanceNameLabel})
		if err != nil {
			listing = childListing{at: now, access: health.AccessNotReadable}
			break
		}
		for i := range list.Items {
			strip(&list.Items[i])
			listing.objects = append(listing.objects, &list.Items[i])
		}
	}
	m.mu.Lock()
	m.childList[namespace] = listing
	m.mu.Unlock()
	return listing.objects, listing.access
}
