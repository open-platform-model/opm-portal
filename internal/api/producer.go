package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

var podsGVR = schema.GroupVersionResource{Version: "v1", Resource: "pods"}

// producer serves the broker's topics from the read model. Every topic
// carries one document, the one its GET returns, rendered for each
// subscriber by the GET's own code path (portal:D2:R5). Changes come from the
// read model's change feed, coalesced; topics the feed cannot see change
// (events, polled objects) are rendered again every Refresh.
type producer struct {
	s *Server
	b *stream.Broker

	mu      sync.Mutex
	active  map[stream.Topic]int
	dirty   map[stream.Topic]bool
	deleted map[stream.Topic]readmodel.ObjectRef
	// removed holds the topics whose last published item was a delete. A
	// refresh skips them until a change that is not a delete arrives, so a
	// followed object that is gone is announced once, not every Refresh.
	removed map[stream.Topic]bool

	stopFeed func()
	stopCh   chan struct{}
	done     chan struct{}
	once     sync.Once
}

var _ stream.Producer = (*producer)(nil)

func newProducer(s *Server) *producer {
	return &producer{
		s:       s,
		active:  map[stream.Topic]int{},
		dirty:   map[stream.Topic]bool{},
		deleted: map[stream.Topic]readmodel.ObjectRef{},
		removed: map[stream.Topic]bool{},
		stopCh:  make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// start subscribes to the change feed and runs the publisher.
func (p *producer) start(b *stream.Broker) {
	p.b = b
	p.stopFeed = p.s.cfg.Model.OnChange(p.changed)
	go p.run()
}

func (p *producer) stop() {
	p.once.Do(func() {
		p.stopFeed()
		close(p.stopCh)
	})
	<-p.done
}

// target is the object a topic, or the topic an events topic follows, is
// about.
type target struct {
	kind      stream.Kind
	namespace string
	name      string
}

func (tg target) owner() owner {
	if tg.kind == stream.KindPackage {
		return packageOwner(tg.namespace, tg.name)
	}
	return instanceOwner(tg.namespace, tg.name)
}

func (tg target) ref() readmodel.ObjectRef {
	switch tg.kind {
	case stream.KindPlatform:
		return readmodel.ObjectRef{Group: opmGroup, Version: opmVersion, Kind: kindPlatform, Name: platformName}
	case stream.KindRegistration:
		return readmodel.ObjectRef{Group: opmGroup, Version: opmVersion, Kind: kindRegistration, Name: tg.name}
	case stream.KindInstance, stream.KindPackage:
		return tg.owner().ref()
	case stream.KindInstances, stream.KindEvents, stream.KindLog:
	}
	return readmodel.ObjectRef{}
}

// get is the read a subscriber needs on the target.
func (tg target) get() authz.Attributes {
	switch tg.kind {
	case stream.KindPlatform:
		return platformGet()
	case stream.KindRegistration:
		return authz.Attributes{Verb: verbGet, Resource: registrationsGVR, Name: tg.name}
	case stream.KindInstance, stream.KindPackage:
		return tg.owner().get()
	case stream.KindInstances, stream.KindEvents, stream.KindLog:
	}
	return authz.Attributes{}
}

func targetOf(t stream.Topic) target {
	return target{kind: t.Kind(), namespace: t.Namespace(), name: t.Name()}
}

// Attributes names the reads a topic's GET authorizes: get on the object,
// list in the list topic's scope, and for events, get on the object plus
// list of events where they live. A lone registration has no GET, so its
// topic is not served; its events topic is.
func (p *producer) Attributes(t stream.Topic) ([]authz.Attributes, bool) {
	switch t.Kind() {
	case stream.KindPlatform, stream.KindInstance, stream.KindPackage:
		return []authz.Attributes{targetOf(t).get()}, true
	case stream.KindInstances:
		return []authz.Attributes{{Verb: verbList, Resource: instancesGVR, Namespace: t.Namespace()}}, true
	case stream.KindEvents:
		ref, ok := t.Ref()
		if !ok {
			return nil, false
		}
		tg := targetOf(ref)
		return []authz.Attributes{tg.get(), eventsList(tg.ref())}, true
	case stream.KindRegistration, stream.KindLog:
	}
	return nil, false
}

// Snapshot returns the topic's one document, rendered per subscriber.
func (p *producer) Snapshot(_ context.Context, t stream.Topic) ([]stream.Item, error) {
	it, ok := p.item(t)
	if !ok {
		return nil, fmt.Errorf("topic %s is not served", t)
	}
	return []stream.Item{it}, nil
}

// item is the topic's document as an item: under the topic's first read
// (its list read for a list topic, the events list for an events topic),
// rendered for whoever receives it.
func (p *producer) item(t stream.Topic) (stream.Item, bool) {
	attrs, ok := p.Attributes(t)
	if !ok {
		return stream.Item{}, false
	}
	it := stream.Item{Event: stream.EventUpsert, Attrs: attrs[0]}
	if t.Kind() == stream.KindEvents {
		it.Event, it.Attrs = stream.EventK8sEvent, attrs[1]
	}
	it.Render = func(ctx context.Context, who authz.Identity) (json.RawMessage, error) {
		doc, err := p.render(ctx, who, t)
		if err == nil {
			doc, err = p.s.forMode(doc)
		}
		if err != nil {
			return nil, err
		}
		return json.Marshal(doc)
	}
	return it, true
}

// render runs the topic's GET for who. An object that does not exist
// renders as Removed.
func (p *producer) render(ctx context.Context, who authz.Identity, t stream.Topic) (any, error) {
	s := p.s
	tg := targetOf(t)
	var doc any
	var err error
	switch t.Kind() {
	case stream.KindInstances:
		return s.instanceListDoc(ctx, who, t.Namespace())
	case stream.KindPlatform:
		var view readmodel.PlatformView
		view, err = s.platformView(ctx, who)
		doc = platformDoc(view)
	case stream.KindInstance:
		var d readmodel.InstanceDetail
		d, err = s.instanceDetail(ctx, who, tg.owner())
		doc = instanceDoc(d)
	case stream.KindPackage:
		var d readmodel.PackageDetail
		d, err = s.packageDetail(ctx, who, tg.owner())
		doc = packageDoc(d)
	case stream.KindEvents:
		ref, _ := t.Ref()
		tg = targetOf(ref)
		doc, err = p.renderEvents(ctx, who, tg)
	case stream.KindRegistration, stream.KindLog:
		return nil, fmt.Errorf("topic %s is not served", t)
	}
	if errors.Is(err, readmodel.ErrNotFound) {
		return removed(tg.ref()), nil
	}
	return doc, err
}

func (p *producer) renderEvents(ctx context.Context, who authz.Identity, tg target) (v1.EventList, error) {
	switch tg.kind {
	case stream.KindPlatform:
		return p.s.platformEventsDoc(ctx, who)
	case stream.KindRegistration:
		return p.s.registrationEventsDoc(ctx, who, tg.name)
	case stream.KindInstance, stream.KindPackage, stream.KindInstances, stream.KindEvents, stream.KindLog:
	}
	return p.s.ownerEventsDoc(ctx, who, tg.owner(), readmodel.ObjectRef{}, false)
}

func removed(ref readmodel.ObjectRef) v1.Removed {
	return v1.Removed{TypeMeta: meta(v1.KindRemoved), Ref: objectRef(ref)}
}

// Activate records that t is followed. An instance or package topic also
// holds the runtime-children watch of its namespace, as the reader, so a
// Pod's waiting reason reaches it within the coalescing delay.
func (p *producer) Activate(t stream.Topic) func() {
	p.mu.Lock()
	p.active[t]++
	p.mu.Unlock()
	h := &hold{}
	if k := t.Kind(); k == stream.KindInstance || k == stream.KindPackage {
		go h.start(p, t.Namespace())
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			p.mu.Lock()
			p.active[t]--
			if p.active[t] <= 0 {
				delete(p.active, t)
				delete(p.dirty, t)
				delete(p.deleted, t)
				delete(p.removed, t)
			}
			p.mu.Unlock()
			h.release()
		})
	}
}

// hold is one topic's hold on a namespace's runtime children. A hold that
// arrives after the topic was released is let go at once.
type hold struct {
	mu       sync.Mutex
	unwatch  func()
	released bool
}

func (h *hold) start(p *producer, namespace string) {
	reader := p.s.cfg.Reader
	var unwatch func()
	if reader.Authenticated() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		watch := authz.Attributes{Verb: "watch", Resource: podsGVR, Namespace: namespace}
		if g, err := p.s.cfg.Authorizer.Check(ctx, reader, watch); err == nil {
			if r, err := p.s.cfg.Model.HoldChildren(ctx, reader, g, namespace); err == nil {
				unwatch = r
			}
		}
	}
	h.mu.Lock()
	if h.released && unwatch != nil {
		h.mu.Unlock()
		unwatch()
		return
	}
	h.unwatch = unwatch
	h.mu.Unlock()
}

func (h *hold) release() {
	h.mu.Lock()
	h.released = true
	unwatch := h.unwatch
	h.unwatch = nil
	h.mu.Unlock()
	if unwatch != nil {
		unwatch()
	}
}

// changed marks the followed topics a change affects. It runs on an
// informer's goroutine and only records. A deletion marks the object's own
// topic and its events topic deleted; any other change marks them dirty and
// cancels a deletion still waiting in this window, so an object deleted and
// recreated within one Coalesce is published as it now is.
func (p *producer) changed(c readmodel.Change) {
	var own, lists []string
	var ref readmodel.ObjectRef
	switch c.Kind {
	case readmodel.ChangeInstance:
		own = []string{"instance:" + c.Namespace + "/" + c.Name, "events:instance:" + c.Namespace + "/" + c.Name}
		lists = []string{"instances", "instances:" + c.Namespace}
		ref = instanceOwner(c.Namespace, c.Name).ref()
	case readmodel.ChangePackage:
		own = []string{"package:" + c.Namespace + "/" + c.Name, "events:package:" + c.Namespace + "/" + c.Name}
		ref = packageOwner(c.Namespace, c.Name).ref()
	case readmodel.ChangePlatform:
		own = []string{"platform", "events:platform"}
		ref = target{kind: stream.KindPlatform}.ref()
	case readmodel.ChangeRegistration:
		own = []string{"events:registration:" + c.Name}
		ref = target{kind: stream.KindRegistration, name: c.Name}.ref()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, name := range own {
		t, ok := p.followed(name)
		if !ok {
			continue
		}
		if c.Deleted {
			p.deleted[t] = ref
			delete(p.dirty, t)
			continue
		}
		delete(p.deleted, t)
		delete(p.removed, t)
		p.dirty[t] = true
	}
	for _, name := range lists {
		if t, ok := p.followed(name); ok {
			p.dirty[t] = true
		}
	}
}

// followed parses name and reports whether its topic is followed. Names
// come from objects the cluster accepted; one that does not parse follows
// no topic. p.mu is held.
func (p *producer) followed(name string) (stream.Topic, bool) {
	t, err := stream.ParseTopic(name)
	if err != nil || p.active[t] == 0 {
		return stream.Topic{}, false
	}
	return t, true
}

// run publishes what changed every Coalesce, and everything followed every
// Refresh, until stop.
func (p *producer) run() {
	defer close(p.done)
	flush := time.NewTicker(p.s.cfg.Coalesce)
	defer flush.Stop()
	refresh := time.NewTicker(p.s.cfg.Refresh)
	defer refresh.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-refresh.C:
			p.mu.Lock()
			for t := range p.active {
				if !p.removed[t] {
					p.dirty[t] = true
				}
			}
			p.mu.Unlock()
		case <-flush.C:
			p.flush()
		}
	}
}

func (p *producer) flush() {
	p.mu.Lock()
	dirty, deleted := p.dirty, p.deleted
	p.dirty, p.deleted = map[stream.Topic]bool{}, map[stream.Topic]readmodel.ObjectRef{}
	p.mu.Unlock()
	for t, ref := range deleted {
		data, err := json.Marshal(removed(ref))
		if err != nil {
			continue
		}
		attrs, _ := p.Attributes(t)
		p.publish(t, stream.Item{Event: stream.EventDelete, Attrs: attrs[0], Data: data})
		delete(dirty, t)
		// A change that arrived since this flush began cancels the mark.
		p.mu.Lock()
		if _, ok := p.active[t]; ok && !p.dirty[t] {
			p.removed[t] = true
		}
		p.mu.Unlock()
	}
	for t := range dirty {
		if it, ok := p.item(t); ok {
			p.publish(t, it)
		}
	}
}

func (p *producer) publish(t stream.Topic, it stream.Item) {
	if err := p.b.Publish(t, it); err != nil {
		p.s.log.Warn("publishing a change", "topic", t.String(), "error", err)
	}
}
