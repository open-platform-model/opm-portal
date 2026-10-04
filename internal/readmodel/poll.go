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

// pollTimeout bounds one poll.
const pollTimeout = 10 * time.Second

// poller refreshes, by reading them one at a time, the objects of a kind the
// reader may get but not list and watch. Every result says it is not live
// and when it was read (0030:D3:R5).
type poller struct {
	resource schema.GroupVersionResource

	mu      sync.Mutex
	objects map[string]heldObject
	names   map[string][2]string // key -> namespace, name
	stop    chan struct{}
	once    sync.Once
}

func newPoller(resource schema.GroupVersionResource) *poller {
	return &poller{
		resource: resource,
		objects:  map[string]heldObject{},
		names:    map[string][2]string{},
		stop:     make(chan struct{}),
	}
}

func (p *poller) close() { p.once.Do(func() { close(p.stop) }) }

// read returns the last poll of the object, reading it now when it has never
// been read; from then on the poller refreshes it every PollInterval.
func (p *poller) read(ctx context.Context, m *Model, namespace, name string) heldObject {
	key := namespace + "/" + name
	p.mu.Lock()
	held, ok := p.objects[key]
	p.mu.Unlock()
	if ok {
		return held
	}
	held = p.fetch(ctx, m, namespace, name)
	p.mu.Lock()
	p.objects[key] = held
	p.names[key] = [2]string{namespace, name}
	p.mu.Unlock()
	return held
}

// fetch reads one object as the reader, after the reader's get grant.
func (p *poller) fetch(ctx context.Context, m *Model, namespace, name string) heldObject {
	now := m.cfg.Now()
	if !m.readerMay(ctx, "get", p.resource, namespace, name) {
		return heldObject{access: health.AccessNotReadable, evaluatedAt: now}
	}
	ctx, cancel := context.WithTimeout(ctx, pollTimeout)
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

func (p *poller) refresh(ctx context.Context, m *Model) {
	p.mu.Lock()
	targets := make([][2]string, 0, len(p.names))
	for _, n := range p.names {
		targets = append(targets, n)
	}
	p.mu.Unlock()

	slots := make(chan struct{}, m.cfg.PollWorkers)
	var wg sync.WaitGroup
	for _, n := range targets {
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			held := p.fetch(ctx, m, n[0], n[1])
			p.mu.Lock()
			p.objects[n[0]+"/"+n[1]] = held
			p.mu.Unlock()
		})
	}
	wg.Wait()
}
