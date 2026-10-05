package readmodel

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"
)

// The four OPM kinds the read model holds for the process (tier 1).
var (
	moduleInstances = schema.GroupVersionResource{Group: opmGroup, Version: opmVersion, Resource: "moduleinstances"}
	modulePackages  = schema.GroupVersionResource{Group: opmGroup, Version: opmVersion, Resource: "modulepackages"}
	platforms       = schema.GroupVersionResource{Group: opmGroup, Version: opmVersion, Resource: "platforms"}
	registrations   = schema.GroupVersionResource{Group: opmGroup, Version: opmVersion, Resource: "transformerregistrations"}
)

const (
	opmGroup   = "opmodel.dev"
	opmVersion = "v1alpha1"

	// platformName is the only name the Platform CRD admits.
	platformName = "cluster"

	// instanceUUIDLabel is on every object an instance's inventory names;
	// tier 2 selects on it, so only OPM-rendered objects are held.
	instanceUUIDLabel = "module-instance.opmodel.dev/uuid"
	// instanceNameLabel is on the ReplicaSets and Pods below inventory
	// workloads, which do not carry the uuid label (capture, observation 8).
	instanceNameLabel = "module-instance.opmodel.dev/name"

	// kindReplicaSet is the apps ReplicaSet kind, read for its replicas and
	// stripped of its template.
	kindReplicaSet = "ReplicaSet"
	// kindPod is the core Pod kind, whose logs a stream reads.
	kindPod = "Pod"
)

// isSecret reports whether resource is core Secrets, which the read model
// never reads (portal:D8:R1).
func isSecret(resource schema.GroupVersionResource) bool {
	return resource.Group == "" && resource.Resource == "secrets"
}

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

// lookup resolves gvk from what discovery already holds and never refreshes
// it, so a kind named by a request cannot invalidate the shared cache. The
// first lookup of the process fetches discovery once, as any first use does.
func (k *kindResolver) lookup(gvk schema.GroupVersionKind) (resolvedKind, error) {
	m, err := k.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return resolvedKind{}, fmt.Errorf("looking up kind %s: %w", gvk, err)
	}
	return resolvedKind{Resource: m.Resource, Namespaced: m.Scope.Name() == meta.RESTScopeNameNamespace}, nil
}

// ResolvedKind is a kind as the cluster serves it.
type ResolvedKind struct {
	Resource   schema.GroupVersionResource
	Namespaced bool
}

// ResolveKind resolves group and kind to the resource they are served as,
// at the preferred version, through the Model's cached discovery, so a
// caller can authorize a read of the kind before looking anything up. A
// kind discovery does not know is an error, never a guess, and never
// refreshes discovery: group and kind may come from a request, and a
// request must not be able to invalidate the cache every reader shares. A
// kind an inventory reaches is already known, because evaluating that
// inventory resolved it (refreshing once if it was new).
func (m *Model) ResolveKind(group, kind string) (ResolvedKind, error) {
	k, err := m.kinds.lookup(schema.GroupVersionKind{Group: group, Kind: kind})
	if err != nil {
		return ResolvedKind{}, err
	}
	return ResolvedKind(k), nil
}
