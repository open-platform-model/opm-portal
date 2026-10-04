package stream

import (
	"context"
	"errors"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// Mux is a Producer that routes each topic to the producer serving its
// kind, so the read model's object topics and the log reader's topics share
// one broker. A kind with no producer is not served. Admit is delegated to a
// routed producer that is an Admitter; one that is not admits every topic
// whose reads are allowed.
type Mux map[Kind]Producer

var (
	_ Producer = Mux(nil)
	_ Admitter = Mux(nil)
)

// errNotRouted is returned for a topic whose kind has no producer. The
// broker never asks for one: Attributes refuses it first.
var errNotRouted = errors.New("stream: no producer serves this topic kind")

// Attributes implements Producer.
func (m Mux) Attributes(t Topic) ([]authz.Attributes, bool) {
	p := m[t.Kind()]
	if p == nil {
		return nil, false
	}
	return p.Attributes(t)
}

// Snapshot implements Producer.
func (m Mux) Snapshot(ctx context.Context, t Topic) ([]Item, error) {
	p := m[t.Kind()]
	if p == nil {
		return nil, errNotRouted
	}
	return p.Snapshot(ctx, t)
}

// Activate implements Producer.
func (m Mux) Activate(t Topic) func() {
	p := m[t.Kind()]
	if p == nil {
		return func() {}
	}
	return p.Activate(t)
}

// Admit implements Admitter.
func (m Mux) Admit(ctx context.Context, who authz.Identity, t Topic, grants []authz.Grant) error {
	ad, ok := m[t.Kind()].(Admitter)
	if !ok {
		return nil
	}
	return ad.Admit(ctx, who, t, grants)
}
