package readmodel

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"
)

const opmGroup = "opmodel.dev"

// resolvedKind is an inventory kind resolved to the resource it is served
// as.
type resolvedKind struct {
	Resource   schema.GroupVersionResource
	Namespaced bool
}

// kindResolver maps kinds to resources through discovery, fetched once and
// kept for the process. A kind discovery did not know is looked up once more
// after a refresh, so a CRD installed later resolves.
type kindResolver struct {
	mapper *restmapper.DeferredDiscoveryRESTMapper
}

func newKindResolver(d discovery.DiscoveryInterface) *kindResolver {
	return &kindResolver{mapper: restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(d))}
}

func (k *kindResolver) resolve(gvk schema.GroupVersionKind) (resolvedKind, error) {
	m, err := k.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if meta.IsNoMatchError(err) {
		k.mapper.Reset()
		m, err = k.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	}
	if err != nil {
		return resolvedKind{}, fmt.Errorf("resolving kind %s: %w", gvk, err)
	}
	return resolvedKind{Resource: m.Resource, Namespaced: m.Scope.Name() == meta.RESTScopeNameNamespace}, nil
}
