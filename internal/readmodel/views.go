package readmodel

import (
	"time"

	"github.com/open-platform-model/opm-portal/internal/health"
)

// The views below are what the read model hands to the read API and the UI:
// portal-shaped Go values, never a raw custom resource, its status,
// spec.values or an annotation map (portal:D2:R4, portal:D8:R2). The read API
// maps them onto its wire types.

// ObjectRef names one Kubernetes object.
type ObjectRef struct {
	Group     string
	Version   string
	Kind      string
	Namespace string
	Name      string
}

// Owner says who applies an instance.
type Owner string

// Owners.
const (
	OwnerOperator Owner = "operator"
	// OwnerCLI: the CLI applied the instance and the operator only records
	// it; its applied state is ManagedExternally (portal:D3:R6).
	OwnerCLI Owner = "cli"
)

// ModuleRef is the module an instance or package was rendered from.
type ModuleRef struct {
	Path    string
	Version string
}

// Digests are the digests of one apply.
type Digests struct {
	Source string
	Config string
	Render string
}

// Outcome is how a history entry's attempt ended, read from the shape the
// operator wrote it in.
type Outcome string

// Outcomes. The operator writes a success entry with phase complete and no
// message, and a failure entry with a message and no phase.
const (
	OutcomeSucceeded Outcome = "Succeeded"
	OutcomeFailed    Outcome = "Failed"
	// OutcomeUnknown: an entry of neither shape.
	OutcomeUnknown Outcome = "Unknown"
)

// HistoryEntry is one entry of an operator object's status.history, the
// durable record (portal:D9:R1).
type HistoryEntry struct {
	Action string
	Phase  string
	// Outcome is decided from Phase and Message as the operator wrote
	// them, so it survives any later omission of the message.
	Outcome         Outcome
	Sequence        int64
	StartedAt       time.Time
	FinishedAt      time.Time
	Message         string
	InventoryCount  int64
	InventoryDigest string
	Digests         Digests
}

// InstanceItem is one ModuleInstance in a list: the operator's axis and the
// portal's axis side by side, never merged (portal:D3:R1).
type InstanceItem struct {
	Ref            ObjectRef
	UID            string
	Module         ModuleRef
	Owner          Owner
	Applied        health.Applied
	Health         health.Summary
	InventoryCount int64
	LastAppliedAt  time.Time
	// RenderContracts are every contract the instance's render used, most
	// of them fulfilled by the catalog itself. They are not the provider
	// contracts the instance demands (portal:D4:R3).
	RenderContracts []string
	// ProviderOf are the TransformerRegistrations the instance's inventory
	// holds; nil when it holds none.
	ProviderOf []ProviderClaim
}

// InstanceDetail is one ModuleInstance with its inventory grouped by
// component.
type InstanceDetail struct {
	InstanceItem
	ServiceAccountName string
	Conditions         []health.Condition
	History            []HistoryEntry
	LastApplied        Digests
	Components         []Component
}

// ProviderClaim is one TransformerRegistration an owner's inventory holds,
// whatever the owner's kind (portal:D15).
type ProviderClaim struct {
	Registration string
	// Access is the caller's access to the registrations: ok, forbidden or
	// not readable. Standing and ProviderRefMatches mean something only
	// when it is ok.
	Access health.Access
	// Standing is the registration's own, as the controller wrote it.
	Standing health.Registration
	// ProviderRefMatches: the owner is a ModuleInstance whose namespace and
	// name the registration's spec.providerRef names. Always false for a
	// ModulePackage, since the reference names a ModuleInstance. Nil when
	// no registration was read: the caller may not read it, or the model
	// does not hold it.
	ProviderRefMatches *bool
}

// Component is one component of an inventory with its objects, in
// inventory order.
type Component struct {
	Name    string
	Health  health.Summary
	Objects []InventoryObject
}

// InventoryObject is one inventory entry as the caller may see it. An entry
// the caller may not read carries only its reference and its access.
type InventoryObject struct {
	Ref    ObjectRef
	Access health.Access
	Health health.ObjectHealth
	// ChildrenUnread is set on a workload whose Pods could not be read, so
	// its health is the status rules alone.
	ChildrenUnread bool
	// EvaluatedAt is when the object was last read; Live is false when it
	// is polled rather than watched (portal:D3:R5).
	EvaluatedAt time.Time
	Live        bool
	// Children are the ReplicaSets, Pods and Jobs whose chain of controller
	// owner references reaches this object, sorted by kind and name. Empty
	// when the children could not be read (see ChildrenUnread).
	Children []RuntimeChild
}

// RuntimeChild is a ReplicaSet, Pod or Job below an inventory object.
type RuntimeChild struct {
	Ref ObjectRef
	// Owner is the child's controller: the inventory object or another
	// child.
	Owner  ObjectRef
	Health health.ObjectHealth
	// Replicas is a ReplicaSet's spec.replicas; nil for other kinds or when
	// unset.
	Replicas *int64
	// Containers are a Pod's init containers, then its containers, by name
	// in spec order; nil for other kinds.
	Containers []string
}

// SourceRef is the Flux source a ModulePackage reads.
type SourceRef struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

// SourceArtifact is what a ModulePackage's last reconcile fetched, from
// status.source. Its fetch URL is not kept.
type SourceArtifact struct {
	Revision string
	Digest   string
}

// PackageItem is one ModulePackage in a list.
type PackageItem struct {
	Ref    ObjectRef
	UID    string
	Source SourceRef
	// Interval is spec.interval as written; empty when the package sets
	// none.
	Interval string
	// SourceArtifact is nil when the operator recorded no source artifact.
	SourceArtifact *SourceArtifact
	// DependsOn are the ModulePackages spec.dependsOn names, as written: an
	// empty namespace means the package's own.
	DependsOn      []ObjectRef
	Path           string
	Applied        health.Applied
	Health         health.Summary
	InventoryCount int64
	LastAppliedAt  time.Time
	// ProviderOf are the TransformerRegistrations the package's inventory
	// holds; nil when it holds none.
	ProviderOf []ProviderClaim
}

// PackageDetail is one ModulePackage with its inventory, if it has one.
type PackageDetail struct {
	PackageItem
	// ServiceAccountName is the ServiceAccount the controller applies as;
	// empty means its own identity.
	ServiceAccountName string
	Conditions         []health.Condition
	History            []HistoryEntry
	LastApplied        Digests
	Components         []Component
}

// Subscription is one catalog the Platform's spec subscribes to. Enable is
// nil when the spec does not say.
type Subscription struct {
	Catalog string
	Version string
	Enable  *bool
}

// Catalog is one entry of the Platform's resolved registry, with the
// registrations that claim it.
type Catalog struct {
	Catalog       string
	Version       string
	Enabled       bool
	Source        string
	Registrations []string
}

// RegistrationView is one TransformerRegistration: acceptance, activation
// and verdict as separate values (portal:D4:R4/R7).
type RegistrationView struct {
	Name     string
	Catalog  string
	Version  string
	Provides []string
	Provider ObjectRef
	Standing health.Registration
	Applied  health.Applied
	// Conditions are the registration's status.conditions.
	Conditions []health.Condition
	// HeldBy are the ModuleInstances and ModulePackages the caller may list
	// whose inventory holds the registration, instances first. Set on the
	// Platform's registrations only.
	HeldBy []ObjectRef
	// HeldByPartial: the caller could not look at every instance and
	// package, so HeldBy may lack a holder.
	HeldByPartial bool
}

// PlatformView is the Platform with its subscriptions, resolved catalogs and
// registrations.
type PlatformView struct {
	Name            string
	UID             string
	Type            string
	OperatorVersion string
	Applied         health.Applied
	Conditions      []health.Condition
	Subscriptions   []Subscription
	Catalogs        []Catalog
	// Registrations holds what the caller may read; RegistrationsAccess
	// says whether that is all of them.
	Registrations       []RegistrationView
	RegistrationsAccess health.Access
}
