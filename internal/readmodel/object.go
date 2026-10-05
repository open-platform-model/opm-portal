package readmodel

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/open-platform-model/opm-portal/internal/authz"
)

// ErrWithheld: the read names a kind the portal never reads, a core
// Secret. It is refused before any review (portal:D8:R1).
var ErrWithheld = errors.New("kind withheld from every read")

// Object reads one object on demand, for a YAML view, and returns it with
// what the portal never serves removed: managed fields, the last-applied
// annotation and, on a ModuleInstance or ModulePackage, spec.values
// (portal:D8:R2/R3). Fields the held copies drop for memory, such as a
// Deployment's pod template, are kept. g must cover get on the object, and
// the reader must be allowed the same get. Reaching the object from an
// inventory is the caller's check, made before this read.
func (m *Model) Object(ctx context.Context, who authz.Identity, g authz.Grant, kind ResolvedKind, ref ObjectRef) (map[string]any, error) {
	if isSecret(kind.Resource) {
		return nil, ErrWithheld
	}
	namespace := ref.Namespace
	if !kind.Namespaced {
		namespace = ""
	}
	if err := covers(who, g, "get", kind.Resource, namespace, ref.Name); err != nil {
		return nil, err
	}
	if !m.readerMay(ctx, "get", kind.Resource, namespace, ref.Name) {
		return nil, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	obj, err := m.cfg.Dynamic.Resource(kind.Resource).Namespace(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("%w: reading an object: %w", ErrUnavailable, err)
	}
	stripWithheld(obj)
	return obj.Object, nil
}
