// Package readmodel holds what the portal knows about one cluster and builds
// every view from it: the cache, the watches, the on-demand reads and the
// joins of the portal's read path.
//
// # Tiers
//
//   - Tier 1: the four OPM kinds (ModuleInstance, ModulePackage, Platform,
//     TransformerRegistration), watched from Start to Stop, cluster-wide or
//     per configured namespace.
//   - Tier 2: the kinds inventories name, watched on the first view that
//     needs them, limited to objects carrying the module-instance uuid
//     label, and stopped once no view has used them for IdleTimeout. A kind
//     the reader may get but not list and watch is polled every
//     PollInterval instead, and its objects say they are not live.
//   - Tier 3: ReplicaSets, Pods and Jobs, watched in a namespace only while
//     someone holds interest (HoldChildren), otherwise listed on demand and
//     reused for ChildrenTTL.
//   - Tier 4: events about one object, listed when asked, folded so a repeat
//     shows once with a count.
//
// Kinds are resolved through discovery, cached for the process; an
// inventory kind the cache does not know refreshes it once. ResolveKind
// exposes the cache for kinds a request names and never refreshes it.
// TuneConfig raises the reading client's request rate.
//
// # Joins
//
// An instance's or package's provider claims (the TransformerRegistrations
// its inventory holds, with each one's own standing) and a registration's
// holders (the instances and packages whose inventory holds it) are joined
// from held state, kind-agnostic, for the caller.
//
// # The ready count of a Service
//
// InstanceWithEndpoints and PackageWithEndpoints add, to each Service of the
// inventory, the number of its ready endpoints, or the mark that the number
// could not be read. They are the only reads that make it: Instance,
// Package, the lists, the events checks and ReachPod ask no review of
// EndpointSlices and list none. For each Service the caller's list review
// of endpointslices in its namespace comes first, then the reader's, then one
// list selected by the Service's name, reused for ChildrenTTL. A refused or
// undecidable review and a failed, timed-out or partial list give the
// mark, never a count of 0; a Service the cluster records no slice for
// carries neither. Only a slice owned by the Service counts, and the
// result is a number: no slice, address, name or label is kept or served.
//
// # Changes
//
// OnChange tells a listener which OPM object's view may have changed: the
// object itself, the owners whose inventory names a changed object, the
// owners a changed runtime child names, and through the provider joins, a
// changed registration's holders and, for an owner that holds or held a
// registration, the Platform. A Change carries no content; the
// listener renders the view again through a grant like any other read.
//
// # Authorization
//
// Every exported read takes the caller's authz.Identity and an authz.Grant,
// and calls Grant.Covers for its own read before it looks anything up, so a
// caller without access gets the same refusal for an object that exists and
// one that does not. Reads inside a view (each inventory object, the
// registrations, the runtime children) are authorized for the caller one by
// one through the Authorizer; what the caller may not read is marked
// forbidden, never omitted and never a failed view. The informers, polls
// and on-demand lists read as Config.Reader, each after the reader's own
// grant. The discovery documents and the API server's version, read once,
// are the only reads without a review: they are open to every
// authenticated identity and name no object.
//
// # What is never held
//
// No Secret is read at any tier: an inventory entry naming one is withheld.
// Every object loses its managed fields and the last-applied annotation
// before it is stored, and ModuleInstances and ModulePackages lose
// spec.values. Views are portal-shaped values: none carries a raw custom
// resource, its status or an annotation map.
package readmodel
