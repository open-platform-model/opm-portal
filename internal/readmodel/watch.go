package readmodel

import (
	"context"
	"sort"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// watch is one running informer: one resource in one scope, stripped before
// it stores anything, with its own stop channel so it can be stopped alone.
type watch struct {
	informer cache.SharedIndexInformer
	stop     chan struct{}
	once     sync.Once
}

// startWatch starts an informer on resource in namespace ("" for every
// namespace or a cluster-scoped resource), limited to objects matching
// selector. Callers have checked the reader's list and watch grants.
func (m *Model) startWatch(resource schema.GroupVersionResource, namespace, selector string) *watch {
	inf := dynamicinformer.NewFilteredDynamicInformer(m.cfg.Dynamic, resource, namespace, 0,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		func(o *metav1.ListOptions) { o.LabelSelector = selector })
	informer := inf.Informer()
	// SetTransform fails only on a started informer; this one is new.
	if err := informer.SetTransform(stripTransform); err != nil {
		panic("readmodel: transform on a new informer: " + err.Error())
	}
	w := &watch{informer: informer, stop: make(chan struct{})}
	go informer.Run(w.stop)
	return w
}

func (w *watch) synced() bool { return w != nil && w.informer.HasSynced() }

func (w *watch) close() {
	if w == nil {
		return
	}
	w.once.Do(func() { close(w.stop) })
}

// get returns the stored object, which callers must not modify.
func (w *watch) get(namespace, name string) (*unstructured.Unstructured, bool) {
	key := name
	if namespace != "" {
		key = namespace + "/" + name
	}
	obj, ok, err := w.informer.GetStore().GetByKey(key)
	if err != nil || !ok {
		return nil, false
	}
	u, ok := obj.(*unstructured.Unstructured)
	return u, ok
}

// list returns the stored objects in namespace ("" for all), which callers
// must not modify.
func (w *watch) list(namespace string) []*unstructured.Unstructured {
	var raw []any
	if namespace == "" {
		raw = w.informer.GetStore().List()
	} else {
		var err error
		raw, err = w.informer.GetIndexer().ByIndex(cache.NamespaceIndex, namespace)
		if err != nil {
			return nil
		}
	}
	out := make([]*unstructured.Unstructured, 0, len(raw))
	for _, o := range raw {
		if u, ok := o.(*unstructured.Unstructured); ok {
			out = append(out, u)
		}
	}
	return out
}

// syncPoll is how often waitSynced looks at the informers. client-go's own
// wait polls every 100 ms, which a cold view would pay once per kind.
const syncPoll = 5 * time.Millisecond

// waitSynced waits until every watch has synced, for at most timeout or
// until ctx ends. A watch still syncing afterwards reads as unavailable or
// not readable, never as empty.
func waitSynced(ctx context.Context, timeout time.Duration, ws ...*watch) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	t := time.NewTicker(syncPoll)
	defer t.Stop()
	for {
		if allSynced(ws) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func allSynced(ws []*watch) bool {
	for _, w := range ws {
		if w != nil && !w.synced() {
			return false
		}
	}
	return true
}

// sortObjects orders objects by namespace, then name.
func sortObjects(objs []*unstructured.Unstructured) {
	sort.Slice(objs, func(i, j int) bool {
		if objs[i].GetNamespace() != objs[j].GetNamespace() {
			return objs[i].GetNamespace() < objs[j].GetNamespace()
		}
		return objs[i].GetName() < objs[j].GetName()
	})
}
