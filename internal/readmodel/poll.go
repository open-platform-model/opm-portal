package readmodel

import (
	"context"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-platform-model/opm-portal/internal/health"
)

// readTimeout bounds one direct read: a poll, an on-demand list of runtime
// children, or an events list.
const readTimeout = 10 * time.Second

// poller refreshes, by reading them one at a time, the objects of a kind the
// reader may get but not list and watch. Every result says it is not live
// and when it was read (0030:D3:R5). An object no view has read for
// IdleTimeout is dropped instead of refreshed.
type poller struct {
	resource schema.GroupVersionResource

	mu      sync.Mutex
	entries map[string]*polled // by namespace/name
	stop    chan struct{}
	once    sync.Once
}

// polled is one object a poller refreshes.
type polled struct {
	namespace, name string
	held            heldObject
	lastRead        time.Time // when a view last read it
}

func newPoller(resource schema.GroupVersionResource) *poller {
	return &poller{
		resource: resource,
		entries:  map[string]*polled{},
		stop:     make(chan struct{}),
	}
}

func (p *poller) close() { p.once.Do(func() { close(p.stop) }) }

// read returns the last poll of the object, reading it now when it has never
// been read; from then on the poller refreshes it every PollInterval.
func (p *poller) read(ctx context.Context, m *Model, namespace, name string) heldObject {
	key := namespace + "/" + name
	p.mu.Lock()
	if e, ok := p.entries[key]; ok {
		e.lastRead = m.cfg.Now()
		held := e.held
		p.mu.Unlock()
		return held
	}
	p.mu.Unlock()
	held := p.fetch(ctx, m, namespace, name)
	p.mu.Lock()
	p.entries[key] = &polled{namespace: namespace, name: name, held: held, lastRead: m.cfg.Now()}
	p.mu.Unlock()
	return held
}

// fetch reads one object as the reader, after the reader's get grant.
func (p *poller) fetch(ctx context.Context, m *Model, namespace, name string) heldObject {
	now := m.cfg.Now()
	if !m.readerMay(ctx, "get", p.resource, namespace, name) {
		return heldObject{access: health.AccessNotReadable, evaluatedAt: now}
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	obj, err := m.cfg.Dynamic.Resource(p.resource).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		return heldObject{access: health.AccessOK, evaluatedAt: now}
	case err != nil:
		return heldObject{access: health.AccessNotReadable, evaluatedAt: now}
	}
	strip(obj)
	return heldObject{object: obj, access: health.AccessOK, evaluatedAt: now}
}

// run refreshes every object read so far each PollInterval, at most
// PollWorkers at a time, until the poller or the Model stops.
func (p *poller) run(m *Model) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t := time.NewTicker(m.cfg.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-m.done:
			return
		case <-t.C:
			p.refresh(ctx, m)
		}
	}
}

// refresh reads again every object a view has read within IdleTimeout, and
// drops the others.
func (p *poller) refresh(ctx context.Context, m *Model) {
	cutoff := m.cfg.Now().Add(-m.cfg.IdleTimeout)
	p.mu.Lock()
	targets := make([]*polled, 0, len(p.entries))
	for key, e := range p.entries {
		if e.lastRead.Before(cutoff) {
			delete(p.entries, key)
			continue
		}
		targets = append(targets, e)
	}
	p.mu.Unlock()

	slots := make(chan struct{}, m.cfg.PollWorkers)
	var wg sync.WaitGroup
	for _, e := range targets {
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			held := p.fetch(ctx, m, e.namespace, e.name)
			p.mu.Lock()
			e.held = held
			p.mu.Unlock()
		})
	}
	wg.Wait()
}
