package readmodel

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/health"
)

// covers checks that the caller's grant covers the read a public method is
// about to make, before anything is looked up (0030:D7:R1).
func covers(who authz.Identity, g authz.Grant, verb string, resource schema.GroupVersionResource, namespace, name string) error {
	err := g.Covers(who, authz.Attributes{Verb: verb, Resource: resource, Namespace: namespace, Name: name})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNotCovered, err)
	}
	return nil
}

// callerAccess decides whether the caller may make a read inside a view
// (an inventory entry, the registrations, runtime children). It first asks
// for the whole namespace, which a warm view answers from the authorizer's
// cache, and asks for the exact name only when that is denied, because RBAC
// may grant single names. A denial is forbidden; a review that could not be
// made is not readable (0030:D7:R3).
func (m *Model) callerAccess(ctx context.Context, who authz.Identity, verb string, resource schema.GroupVersionResource, namespace, name string) health.Access {
	exact := authz.Attributes{Verb: verb, Resource: resource, Namespace: namespace, Name: name}
	wide := exact
	wide.Name = ""
	access := m.decide(ctx, who, wide, exact)
	if access != health.AccessForbidden || name == "" {
		return access
	}
	return m.decide(ctx, who, exact, exact)
}

// decide asks the authorizer for ask and requires the grant to cover read.
func (m *Model) decide(ctx context.Context, who authz.Identity, ask, read authz.Attributes) health.Access {
	g, err := m.cfg.Authorizer.Check(ctx, who, ask)
	if err != nil {
		if d, ok := errors.AsType[*authz.DenialError](err); ok && d.Code == authz.CodeUnavailable {
			return health.AccessNotReadable
		}
		return health.AccessForbidden
	}
	if g.Covers(who, read) != nil {
		return health.AccessForbidden
	}
	return health.AccessOK
}

// readerMay reports whether the reader identity may make a read. Every
// informer, poll and on-demand list is preceded by it; core Secrets are
// refused here as well as by the authorizer (0030:D8:R1).
func (m *Model) readerMay(ctx context.Context, verb string, resource schema.GroupVersionResource, namespace, name string) bool {
	if isSecret(resource) {
		return false
	}
	read := authz.Attributes{Verb: verb, Resource: resource, Namespace: namespace, Name: name}
	return m.decide(ctx, m.cfg.Reader, read, read) == health.AccessOK
}

// readerMayWatch reports whether the reader may run an informer on resource
// in namespace: list and watch both.
func (m *Model) readerMayWatch(ctx context.Context, resource schema.GroupVersionResource, namespace string) bool {
	return m.readerMay(ctx, "list", resource, namespace, "") && m.readerMay(ctx, "watch", resource, namespace, "")
}
