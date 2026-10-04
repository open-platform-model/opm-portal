package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
	"github.com/open-platform-model/opm-portal/internal/authz"
	"github.com/open-platform-model/opm-portal/internal/graph"
	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
	"github.com/open-platform-model/opm-portal/internal/stream"
)

const (
	opmGroup     = "opmodel.dev"
	opmVersion   = "v1alpha1"
	platformName = "cluster"

	kindModuleInstance = "ModuleInstance"
	kindModulePackage  = "ModulePackage"
	kindPlatform       = "Platform"
	kindRegistration   = "TransformerRegistration"

	verbGet  = "get"
	verbList = "list"
)

// The resources the API authorizes reads of.
var (
	instancesGVR     = stream.InstancesResource()
	packagesGVR      = schema.GroupVersionResource{Group: opmGroup, Version: opmVersion, Resource: "modulepackages"}
	platformsGVR     = schema.GroupVersionResource{Group: opmGroup, Version: opmVersion, Resource: "platforms"}
	registrationsGVR = schema.GroupVersionResource{Group: opmGroup, Version: opmVersion, Resource: "transformerregistrations"}
	eventsGVR        = schema.GroupVersionResource{Group: "events.k8s.io", Version: "v1", Resource: "events"}
)

// owner is what an instance or package request is about.
type owner struct {
	resource  schema.GroupVersionResource
	kind      string
	namespace string
	name      string
}

func (o owner) ref() readmodel.ObjectRef {
	return readmodel.ObjectRef{Group: opmGroup, Version: opmVersion, Kind: o.kind, Namespace: o.namespace, Name: o.name}
}

func (o owner) get() authz.Attributes {
	return authz.Attributes{Verb: verbGet, Resource: o.resource, Namespace: o.namespace, Name: o.name}
}

func instanceOwner(namespace, name string) owner {
	return owner{resource: instancesGVR, kind: kindModuleInstance, namespace: namespace, name: name}
}

func packageOwner(namespace, name string) owner {
	return owner{resource: packagesGVR, kind: kindModulePackage, namespace: namespace, name: name}
}

// pathOwner validates the namespace and name of a path. Nothing is
// authorized for a malformed one.
func pathOwner(r *http.Request, mk func(string, string) owner) (owner, error) {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	if err := checkNamespace(ns); err != nil {
		return owner{}, err
	}
	if err := checkName(name); err != nil {
		return owner{}, err
	}
	return mk(ns, name), nil
}

func checkNamespace(ns string) error {
	if len(validation.IsDNS1123Label(ns)) > 0 {
		return badRequest("namespace is not a DNS-1123 label")
	}
	return nil
}

func checkName(name string) error {
	if len(validation.IsDNS1123Subdomain(name)) > 0 {
		return badRequest("name is not a DNS-1123 subdomain")
	}
	return nil
}

// listScope reads the optional namespace query parameter of a list.
func listScope(r *http.Request) (string, error) {
	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		return "", nil
	}
	return ns, checkNamespace(ns)
}

// listGrant authorizes a list. A refusal the caller may see as an empty
// list returns ok false and no error; a review that failed is an error.
func (s *Server) listGrant(ctx context.Context, who authz.Identity, resource schema.GroupVersionResource, namespace string) (authz.Grant, bool, error) {
	g, err := s.cfg.Authorizer.Check(ctx, who, authz.Attributes{Verb: verbList, Resource: resource, Namespace: namespace})
	switch {
	case err == nil:
		return g, true, nil
	case classify(err).code == v1.CodeForbidden:
		return g, false, nil
	}
	return g, false, err
}

func (s *Server) listInstances(ctx context.Context, p Principal, r *http.Request) (any, error) {
	ns, err := listScope(r)
	if err != nil {
		return nil, err
	}
	return s.instanceListDoc(ctx, p.Identity, ns)
}

// instanceListDoc is the InstanceList the caller may list in namespace
// ("" for cluster-wide). A caller who may not list gets no items and
// access forbidden, with no count (0030:D7:R2).
func (s *Server) instanceListDoc(ctx context.Context, who authz.Identity, namespace string) (v1.InstanceList, error) {
	doc := v1.InstanceList{TypeMeta: meta(v1.KindInstanceList), Access: v1.AccessForbidden, Items: []v1.InstanceSummary{}}
	g, ok, err := s.listGrant(ctx, who, instancesGVR, namespace)
	if err != nil || !ok {
		return doc, err
	}
	items, err := s.cfg.Model.ListInstances(ctx, who, g, namespace)
	if err != nil {
		return v1.InstanceList{}, err
	}
	doc.Access = v1.AccessOK
	for i := range items {
		doc.Items = append(doc.Items, instanceSummary(items[i]))
	}
	return doc, nil
}

func (s *Server) getInstance(ctx context.Context, p Principal, r *http.Request) (any, error) {
	o, err := pathOwner(r, instanceOwner)
	if err != nil {
		return nil, err
	}
	d, err := s.instanceDetail(ctx, p.Identity, o)
	if err != nil {
		return nil, err
	}
	return instanceDoc(d), nil
}

func (s *Server) instanceDetail(ctx context.Context, who authz.Identity, o owner) (readmodel.InstanceDetail, error) {
	g, err := s.authorize(ctx, who, o.get())
	if err != nil {
		return readmodel.InstanceDetail{}, err
	}
	return s.cfg.Model.Instance(ctx, who, g[0], o.namespace, o.name)
}

func (s *Server) instanceGraph(ctx context.Context, p Principal, r *http.Request) (any, error) {
	o, err := pathOwner(r, instanceOwner)
	if err != nil {
		return nil, err
	}
	opts, err := graphOptions(r)
	if err != nil {
		return nil, err
	}
	d, err := s.instanceDetail(ctx, p.Identity, o)
	if err != nil {
		return nil, err
	}
	return graphDoc(graph.Instance(d, opts)), nil
}

func (s *Server) instanceEvents(ctx context.Context, p Principal, r *http.Request) (any, error) {
	o, err := pathOwner(r, instanceOwner)
	if err != nil {
		return nil, err
	}
	return s.ownerEvents(ctx, p.Identity, o, r)
}

func (s *Server) listPackages(ctx context.Context, p Principal, r *http.Request) (any, error) {
	ns, err := listScope(r)
	if err != nil {
		return nil, err
	}
	doc := v1.PackageList{TypeMeta: meta(v1.KindPackageList), Access: v1.AccessForbidden, Items: []v1.PackageSummary{}}
	g, ok, err := s.listGrant(ctx, p.Identity, packagesGVR, ns)
	if err != nil || !ok {
		return doc, err
	}
	items, err := s.cfg.Model.ListPackages(ctx, p.Identity, g, ns)
	if err != nil {
		return nil, err
	}
	doc.Access = v1.AccessOK
	for i := range items {
		doc.Items = append(doc.Items, packageSummary(items[i]))
	}
	return doc, nil
}

func (s *Server) getPackage(ctx context.Context, p Principal, r *http.Request) (any, error) {
	o, err := pathOwner(r, packageOwner)
	if err != nil {
		return nil, err
	}
	d, err := s.packageDetail(ctx, p.Identity, o)
	if err != nil {
		return nil, err
	}
	return packageDoc(d), nil
}

func (s *Server) packageDetail(ctx context.Context, who authz.Identity, o owner) (readmodel.PackageDetail, error) {
	g, err := s.authorize(ctx, who, o.get())
	if err != nil {
		return readmodel.PackageDetail{}, err
	}
	return s.cfg.Model.Package(ctx, who, g[0], o.namespace, o.name)
}

func (s *Server) packageGraph(ctx context.Context, p Principal, r *http.Request) (any, error) {
	o, err := pathOwner(r, packageOwner)
	if err != nil {
		return nil, err
	}
	opts, err := graphOptions(r)
	if err != nil {
		return nil, err
	}
	d, err := s.packageDetail(ctx, p.Identity, o)
	if err != nil {
		return nil, err
	}
	return graphDoc(graph.Package(d, opts)), nil
}

func (s *Server) packageEvents(ctx context.Context, p Principal, r *http.Request) (any, error) {
	o, err := pathOwner(r, packageOwner)
	if err != nil {
		return nil, err
	}
	return s.ownerEvents(ctx, p.Identity, o, r)
}

func platformGet() authz.Attributes {
	return authz.Attributes{Verb: verbGet, Resource: platformsGVR, Name: platformName}
}

func (s *Server) getPlatform(ctx context.Context, p Principal, _ *http.Request) (any, error) {
	view, err := s.platformView(ctx, p.Identity)
	if err != nil {
		return nil, err
	}
	return platformDoc(view), nil
}

func (s *Server) platformView(ctx context.Context, who authz.Identity) (readmodel.PlatformView, error) {
	g, err := s.authorize(ctx, who, platformGet())
	if err != nil {
		return readmodel.PlatformView{}, err
	}
	return s.cfg.Model.Platform(ctx, who, g[0])
}

// platformGraph builds the platform graph. Each registration's provider
// instance is authorized and read here, for the caller: a refusal or a
// failed read becomes the lookup's access, never a failed graph.
func (s *Server) platformGraph(ctx context.Context, p Principal, r *http.Request) (any, error) {
	opts, err := graphOptions(r)
	if err != nil {
		return nil, err
	}
	view, err := s.platformView(ctx, p.Identity)
	if err != nil {
		return nil, err
	}
	in := graph.PlatformInput{Platform: view}
	seen := map[readmodel.ObjectRef]bool{}
	for i := range view.Registrations {
		ref := view.Registrations[i].Provider
		if ref.Name == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		in.Providers = append(in.Providers, s.lookupProvider(ctx, p.Identity, ref))
	}
	return graphDoc(graph.Platform(in, opts)), nil
}

func (s *Server) lookupProvider(ctx context.Context, who authz.Identity, ref readmodel.ObjectRef) graph.ProviderLookup {
	l := graph.ProviderLookup{Ref: ref}
	if checkNamespace(ref.Namespace) != nil || checkName(ref.Name) != nil {
		// A providerRef no instance could have is not looked up.
		l.Access = health.AccessForbidden
		return l
	}
	d, err := s.instanceDetail(ctx, who, instanceOwner(ref.Namespace, ref.Name))
	switch {
	case err == nil:
		l.Access, l.Instance = health.AccessOK, &d
	case errors.Is(err, readmodel.ErrNotFound):
		l.Access = health.AccessOK
	case classify(err).code == v1.CodeForbidden:
		l.Access = health.AccessForbidden
	default:
		l.Access = health.AccessNotReadable
	}
	return l
}

func (s *Server) platformEvents(ctx context.Context, p Principal, _ *http.Request) (any, error) {
	return s.platformEventsDoc(ctx, p.Identity)
}

func (s *Server) platformEventsDoc(ctx context.Context, who authz.Identity) (v1.EventList, error) {
	about := readmodel.ObjectRef{Group: opmGroup, Version: opmVersion, Kind: kindPlatform, Name: platformName}
	g, err := s.authorize(ctx, who, platformGet(), eventsList(about))
	if err != nil {
		return v1.EventList{}, err
	}
	if _, err := s.cfg.Model.Platform(ctx, who, g[0]); err != nil {
		return v1.EventList{}, err
	}
	return s.events(ctx, who, g[1], about)
}

func (s *Server) registrationEvents(ctx context.Context, p Principal, r *http.Request) (any, error) {
	name := r.PathValue("name")
	if err := checkName(name); err != nil {
		return nil, err
	}
	return s.registrationEventsDoc(ctx, p.Identity, name)
}

func (s *Server) registrationEventsDoc(ctx context.Context, who authz.Identity, name string) (v1.EventList, error) {
	about := readmodel.ObjectRef{Group: opmGroup, Version: opmVersion, Kind: kindRegistration, Name: name}
	g, err := s.authorize(ctx, who,
		authz.Attributes{Verb: verbGet, Resource: registrationsGVR, Name: name}, eventsList(about))
	if err != nil {
		return v1.EventList{}, err
	}
	if _, err := s.cfg.Model.Registration(ctx, who, g[0], name); err != nil {
		return v1.EventList{}, err
	}
	return s.events(ctx, who, g[1], about)
}

func eventsList(about readmodel.ObjectRef) authz.Attributes {
	return authz.Attributes{Verb: verbList, Resource: eventsGVR, Namespace: readmodel.EventNamespace(about)}
}

// regardingQuery reads the object an events request names, if any.
func regardingQuery(r *http.Request) (ref readmodel.ObjectRef, named bool, err error) {
	q := r.URL.Query()
	ref = readmodel.ObjectRef{Group: q.Get("group"), Kind: q.Get("kind"), Namespace: q.Get("namespace"), Name: q.Get("name")}
	if ref == (readmodel.ObjectRef{}) {
		return ref, false, nil
	}
	if ref.Kind == "" || ref.Name == "" {
		return ref, false, badRequest("an object is named by kind and name, with group and namespace where it has them")
	}
	if ref.Namespace != "" {
		if err := checkNamespace(ref.Namespace); err != nil {
			return ref, false, err
		}
	}
	if err := checkName(ref.Name); err != nil {
		return ref, false, err
	}
	return ref, true, nil
}

// ownerEvents serves the events about an instance or package, or about one
// object it reaches. Every read is authorized before any lookup: get on the
// owner, get on the named object, list of events where they live. An object
// the owner does not reach, or whose kind the cluster does not serve, is
// refused with the forbidden problem, the same a forbidden caller gets
// (0030:D7:R4).
func (s *Server) ownerEvents(ctx context.Context, who authz.Identity, o owner, r *http.Request) (v1.EventList, error) {
	about, named, err := regardingQuery(r)
	if err != nil {
		return v1.EventList{}, err
	}
	return s.ownerEventsDoc(ctx, who, o, about, named)
}

func (s *Server) ownerEventsDoc(ctx context.Context, who authz.Identity, o owner, about readmodel.ObjectRef, named bool) (v1.EventList, error) {
	if !named || (about.Group == opmGroup && about.Kind == o.kind && about.Namespace == o.namespace && about.Name == o.name) {
		about, named = o.ref(), false
	}
	// The owner get is reviewed before anything the request names is
	// looked up, the kind included.
	og, err := s.authorize(ctx, who, o.get())
	if err != nil {
		return v1.EventList{}, err
	}
	var reads []authz.Attributes
	if named {
		get, version, err := s.objectGet(about)
		if err != nil {
			return v1.EventList{}, err
		}
		about.Version = version
		reads = append(reads, get)
	}
	reads = append(reads, eventsList(about))
	g, err := s.authorize(ctx, who, reads...)
	if err != nil {
		return v1.EventList{}, err
	}
	if err := s.ownerReaches(ctx, who, og[0], o, about, named); err != nil {
		return v1.EventList{}, err
	}
	return s.events(ctx, who, g[len(g)-1], about)
}

// ownerReaches checks the owner exists and, when the request names an
// object, that the owner's inventory reaches it. Only a named object needs
// the inventory walked; the owner's own events need just its existence, so
// a missing owner is still not found without evaluating its health.
func (s *Server) ownerReaches(ctx context.Context, who authz.Identity, g authz.Grant, o owner, about readmodel.ObjectRef, named bool) error {
	if !named {
		if o.kind == kindModulePackage {
			return s.cfg.Model.PackageExists(ctx, who, g, o.namespace, o.name)
		}
		return s.cfg.Model.InstanceExists(ctx, who, g, o.namespace, o.name)
	}
	components, err := s.ownerComponents(ctx, who, g, o)
	if err != nil {
		return err
	}
	if !reaches(components, about) {
		return forbidden()
	}
	return nil
}

// objectGet is the get a caller needs on the object an events request
// names, and the version its kind is served at. A kind the cluster does not
// serve is refused with the forbidden problem.
func (s *Server) objectGet(about readmodel.ObjectRef) (authz.Attributes, string, error) {
	kind, err := s.cfg.Model.ResolveKind(about.Group, about.Kind)
	if err != nil {
		return authz.Attributes{}, "", forbidden()
	}
	ns := about.Namespace
	if !kind.Namespaced {
		ns = ""
	}
	return authz.Attributes{Verb: verbGet, Resource: kind.Resource, Namespace: ns, Name: about.Name}, kind.Resource.Version, nil
}

// ownerComponents reads the owner's inventory under g.
func (s *Server) ownerComponents(ctx context.Context, who authz.Identity, g authz.Grant, o owner) ([]readmodel.Component, error) {
	if o.kind == kindModulePackage {
		d, err := s.cfg.Model.Package(ctx, who, g, o.namespace, o.name)
		return d.Components, err
	}
	d, err := s.cfg.Model.Instance(ctx, who, g, o.namespace, o.name)
	return d.Components, err
}

// reaches reports whether about is an inventory object of components or a
// runtime child below one.
func reaches(components []readmodel.Component, about readmodel.ObjectRef) bool {
	same := func(r readmodel.ObjectRef) bool {
		return r.Group == about.Group && r.Kind == about.Kind && r.Namespace == about.Namespace && r.Name == about.Name
	}
	for i := range components {
		for j := range components[i].Objects {
			obj := &components[i].Objects[j]
			if same(obj.Ref) {
				return true
			}
			for k := range obj.Children {
				if same(obj.Children[k].Ref) {
					return true
				}
			}
		}
	}
	return false
}

func (s *Server) events(ctx context.Context, who authz.Identity, g authz.Grant, about readmodel.ObjectRef) (v1.EventList, error) {
	evs, err := s.cfg.Model.Events(ctx, who, g, about)
	if err != nil {
		return v1.EventList{}, err
	}
	return eventListDoc(about, evs), nil
}

// graphOptions reads expand (repeatable) and showScaledDown. The node cap
// keeps its default.
func graphOptions(r *http.Request) (graph.Options, error) {
	q := r.URL.Query()
	opts := graph.Options{Expand: q["expand"]}
	if v := q.Get("showScaledDown"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return graph.Options{}, badRequest("showScaledDown is not a boolean")
		}
		opts.ShowScaledDown = b
	}
	return opts, nil
}
