package v1alpha1

import "time"

// APIVersion is the apiVersion every document carries.
const APIVersion = "portal.opmodel.dev/v1alpha1"

// Document kinds.
const (
	KindInstanceList = "InstanceList"
	KindInstance     = "Instance"
	KindPackageList  = "PackageList"
	KindPackage      = "Package"
	KindPlatform     = "Platform"
	KindEventList    = "EventList"
	KindGraph        = "Graph"
	KindRemoved      = "Removed"
	KindObject       = "Object"
)

// Access values: how reading an object or a list went for the caller.
const (
	AccessOK          = "ok"
	AccessForbidden   = "forbidden"
	AccessNotReadable = "notReadable"
	AccessWithheld    = "withheld"
)

// TypeMeta names a document's version and kind.
type TypeMeta struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

// ObjectRef names one Kubernetes object.
type ObjectRef struct {
	Group     string `json:"group,omitempty"`
	Version   string `json:"version,omitempty"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

// Module is the module an instance was rendered from.
type Module struct {
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
}

// Condition is one status condition as the operator wrote it. Its message is
// the operator's text, unchanged.
type Condition struct {
	Type               string     `json:"type"`
	Status             string     `json:"status"`
	Reason             string     `json:"reason,omitempty"`
	Message            string     `json:"message,omitempty"`
	LastTransitionTime *time.Time `json:"lastTransitionTime,omitempty"`
	ObservedGeneration int64      `json:"observedGeneration,omitempty"`
	// Tone is how the condition reads for its type: normal, abnormal,
	// progressing, informational or unknown. Status alone does not say it:
	// Stalled=True is a fault and ContractsFulfilled=False is information.
	Tone string `json:"tone"`
	// Meaning and NextStep are the portal's explanation of Reason, absent
	// for a reason the portal does not know.
	Meaning  string `json:"meaning,omitempty"`
	NextStep string `json:"nextStep,omitempty"`
}

// Reconcile is what the operator says it applied: Applied, Reconciling,
// Failed, Stalled, Suspended, ManagedExternally or Unknown. Applied means
// every apply succeeded, never that the workload is healthy; Health is the
// other axis.
type Reconcile struct {
	State   string     `json:"state"`
	Reason  string     `json:"reason,omitempty"`
	Message string     `json:"message,omitempty"`
	Since   *time.Time `json:"since,omitempty"`
	// Retrying is set on a Failed state the operator is still retrying.
	Retrying bool `json:"retrying,omitempty"`
	// Notes are informational conditions shown beside the state; they never
	// change it.
	Notes []Condition `json:"notes,omitempty"`
}

// HealthCounts tallies the objects a health summary counted, by state and by
// access.
type HealthCounts struct {
	Healthy     int `json:"healthy"`
	Progressing int `json:"progressing"`
	Degraded    int `json:"degraded"`
	Missing     int `json:"missing"`
	Unknown     int `json:"unknown"`
	Forbidden   int `json:"forbidden"`
	NotReadable int `json:"notReadable"`
	Withheld    int `json:"withheld"`
}

// Health is the portal's workload health of an instance, package or
// component: Healthy, Progressing, Degraded, Missing or Unknown.
type Health struct {
	State  string       `json:"state"`
	Counts HealthCounts `json:"counts"`
	// Partial: an object could not be read and was left out.
	Partial bool `json:"partial"`
	// EvaluatedAt is the oldest evaluation among the counted objects.
	EvaluatedAt *time.Time `json:"evaluatedAt,omitempty"`
	// Live is false when a counted object is refreshed by polling.
	Live bool `json:"live"`
}

// ObjectHealth is one object's health and why.
type ObjectHealth struct {
	State   string `json:"state"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// Digests are the digests of one apply.
type Digests struct {
	Source string `json:"source,omitempty"`
	Config string `json:"config,omitempty"`
	Render string `json:"render,omitempty"`
}

// HistoryEntry is one entry of the operator's status history, newest
// first.
type HistoryEntry struct {
	Action          string     `json:"action"`
	Phase           string     `json:"phase,omitempty"`
	Sequence        int64      `json:"sequence"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	FinishedAt      *time.Time `json:"finishedAt,omitempty"`
	Message         string     `json:"message,omitempty"`
	InventoryCount  int64      `json:"inventoryCount"`
	InventoryDigest string     `json:"inventoryDigest,omitempty"`
	Digests         Digests    `json:"digests"`
}

// Component is one component of an inventory with its objects.
type Component struct {
	Name    string            `json:"name"`
	Health  Health            `json:"health"`
	Objects []InventoryObject `json:"objects"`
}

// InventoryObject is one inventory entry as the caller may see it. One the
// caller may not read carries its reference and access only.
type InventoryObject struct {
	Ref    ObjectRef     `json:"ref"`
	Access string        `json:"access"`
	Health *ObjectHealth `json:"health,omitempty"`
	// ChildrenUnread: the Pods below this workload could not be read, so its
	// health is the status rules alone.
	ChildrenUnread bool       `json:"childrenUnread,omitempty"`
	EvaluatedAt    *time.Time `json:"evaluatedAt,omitempty"`
	// Live is false when the object is polled rather than watched.
	Live bool `json:"live"`
	// Children are the ReplicaSets, Pods and Jobs below the object.
	Children []RuntimeChild `json:"children,omitempty"`
}

// RuntimeChild is a ReplicaSet, Pod or Job below an inventory object.
type RuntimeChild struct {
	Ref ObjectRef `json:"ref"`
	// Owner is the child's controller.
	Owner  ObjectRef    `json:"owner"`
	Health ObjectHealth `json:"health"`
	// Replicas is a ReplicaSet's desired replica count.
	Replicas *int64 `json:"replicas,omitempty"`
	// Containers are a Pod's init containers, then its containers, by name:
	// the names its log topics take.
	Containers []string `json:"containers,omitempty"`
}

// InstanceSummary is one ModuleInstance in a list.
type InstanceSummary struct {
	Ref    ObjectRef `json:"ref"`
	UID    string    `json:"uid,omitempty"`
	Module Module    `json:"module"`
	// Owner is operator or cli; a cli instance's reconcile state is
	// ManagedExternally.
	Owner          string     `json:"owner"`
	Reconcile      Reconcile  `json:"reconcile"`
	Health         Health     `json:"health"`
	InventoryCount int64      `json:"inventoryCount"`
	LastAppliedAt  *time.Time `json:"lastAppliedAt,omitempty"`
}

// InstanceList is the instances the caller may list. Access is forbidden,
// with no items, when the caller may not list them in the scope asked.
type InstanceList struct {
	TypeMeta
	Access string            `json:"access"`
	Items  []InstanceSummary `json:"items"`
}

// Instance is one ModuleInstance with its inventory grouped by component.
type Instance struct {
	TypeMeta
	InstanceSummary
	ServiceAccountName string         `json:"serviceAccountName,omitempty"`
	Conditions         []Condition    `json:"conditions"`
	History            []HistoryEntry `json:"history"`
	LastApplied        Digests        `json:"lastApplied"`
	// RenderContracts are every contract the instance's render used, most
	// fulfilled by the catalog itself; they are not the provider contracts
	// it demands.
	RenderContracts []string    `json:"renderContracts"`
	Components      []Component `json:"components"`
}

// SourceRef is the source a ModulePackage reads.
type SourceRef struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
}

// PackageSummary is one ModulePackage in a list.
type PackageSummary struct {
	Ref    ObjectRef `json:"ref"`
	UID    string    `json:"uid,omitempty"`
	Source SourceRef `json:"source"`
	// DependsOn are the packages spec.dependsOn names; an absent namespace
	// is the package's own.
	DependsOn      []ObjectRef `json:"dependsOn,omitempty"`
	Path           string      `json:"path,omitempty"`
	Reconcile      Reconcile   `json:"reconcile"`
	Health         Health      `json:"health"`
	InventoryCount int64       `json:"inventoryCount"`
	LastAppliedAt  *time.Time  `json:"lastAppliedAt,omitempty"`
}

// PackageList is the packages the caller may list.
type PackageList struct {
	TypeMeta
	Access string           `json:"access"`
	Items  []PackageSummary `json:"items"`
}

// Package is one ModulePackage with its inventory.
type Package struct {
	TypeMeta
	PackageSummary
	Conditions  []Condition    `json:"conditions"`
	History     []HistoryEntry `json:"history"`
	LastApplied Digests        `json:"lastApplied"`
	Components  []Component    `json:"components"`
}

// Subscription is one catalog the Platform's spec subscribes to.
type Subscription struct {
	Catalog string `json:"catalog"`
	Version string `json:"version,omitempty"`
	Enable  *bool  `json:"enable,omitempty"`
}

// Catalog is one entry of the Platform's resolved registry.
type Catalog struct {
	Catalog string `json:"catalog"`
	Version string `json:"version,omitempty"`
	Enabled bool   `json:"enabled"`
	// Source is Subscription or Registration.
	Source string `json:"source,omitempty"`
	// Claimants are the readable registrations that claim the catalog,
	// whatever their verdict.
	Claimants []string `json:"claimants"`
	// ContributedBy is the accepted, active registration the registry's
	// contribution is joined to, when the source is Registration.
	ContributedBy string `json:"contributedBy,omitempty"`
}

// Registration is one TransformerRegistration: whether it was accepted and
// whether it is active are two values; Verdict is Accepted, Refused,
// Pending, RemovalBlocked or Unknown.
type Registration struct {
	Name          string     `json:"name"`
	Catalog       string     `json:"catalog"`
	Version       string     `json:"version,omitempty"`
	Provides      []string   `json:"provides,omitempty"`
	Provider      *ObjectRef `json:"provider,omitempty"`
	Accepted      bool       `json:"accepted"`
	Active        bool       `json:"active"`
	Verdict       string     `json:"verdict"`
	Reason        string     `json:"reason,omitempty"`
	Message       string     `json:"message,omitempty"`
	ActiveReason  string     `json:"activeReason,omitempty"`
	ActiveMessage string     `json:"activeMessage,omitempty"`
	Reconcile     Reconcile  `json:"reconcile"`
}

// Platform is the Platform with its subscriptions, resolved catalogs and
// the registrations the caller may read.
type Platform struct {
	TypeMeta
	Name            string         `json:"name"`
	UID             string         `json:"uid,omitempty"`
	Type            string         `json:"type,omitempty"`
	OperatorVersion string         `json:"operatorVersion,omitempty"`
	Reconcile       Reconcile      `json:"reconcile"`
	Conditions      []Condition    `json:"conditions"`
	Subscriptions   []Subscription `json:"subscriptions"`
	Catalogs        []Catalog      `json:"catalogs"`
	Registrations   []Registration `json:"registrations"`
	// RegistrationsAccess says whether Registrations is all of them.
	RegistrationsAccess string `json:"registrationsAccess"`
}

// Event is one line of a recent-activity feed: repeats with the same type,
// reason and note folded into one, with a count and the latest time. Events
// expire after about an hour; no state is read from them.
type Event struct {
	Type                string     `json:"type"`
	Reason              string     `json:"reason"`
	Note                string     `json:"note,omitempty"`
	ReportingController string     `json:"reportingController,omitempty"`
	Regarding           ObjectRef  `json:"regarding"`
	FieldPath           string     `json:"fieldPath,omitempty"`
	Count               int64      `json:"count"`
	LastSeen            *time.Time `json:"lastSeen,omitempty"`
}

// EventList is the recent activity about one object, newest first.
type EventList struct {
	TypeMeta
	Regarding ObjectRef `json:"regarding"`
	Items     []Event   `json:"items"`
}

// Graph is a laid-out relationship graph: of an instance, a package or the
// Platform (Scope).
type Graph struct {
	TypeMeta
	Scope  string      `json:"scope"`
	Root   string      `json:"root"`
	Layout GraphLayout `json:"layout"`
	Nodes  []GraphNode `json:"nodes"`
	Edges  []GraphEdge `json:"edges"`
}

// GraphLayout is the size of a laid-out graph and its columns.
type GraphLayout struct {
	Width      int           `json:"width"`
	Height     int           `json:"height"`
	NodeWidth  int           `json:"nodeWidth"`
	NodeHeight int           `json:"nodeHeight"`
	Columns    []GraphColumn `json:"columns"`
}

// GraphColumn is one column of a layout.
type GraphColumn struct {
	Title string `json:"title"`
	X     int    `json:"x"`
}

// GraphHealth is a node's workload health.
type GraphHealth struct {
	State   string        `json:"state"`
	Reason  string        `json:"reason,omitempty"`
	Message string        `json:"message,omitempty"`
	Partial bool          `json:"partial,omitempty"`
	NotLive bool          `json:"notLive,omitempty"`
	Counts  *HealthCounts `json:"counts,omitempty"`
}

// GraphRegistration is a registration node's claim and standing.
type GraphRegistration struct {
	Catalog       string   `json:"catalog"`
	Version       string   `json:"version,omitempty"`
	Provides      []string `json:"provides,omitempty"`
	Accepted      bool     `json:"accepted"`
	Active        bool     `json:"active"`
	Verdict       string   `json:"verdict"`
	Reason        string   `json:"reason,omitempty"`
	Message       string   `json:"message,omitempty"`
	ActiveReason  string   `json:"activeReason,omitempty"`
	ActiveMessage string   `json:"activeMessage,omitempty"`
}

// GraphCatalog is a catalog node's registry entry.
type GraphCatalog struct {
	Version string `json:"version,omitempty"`
	Enabled bool   `json:"enabled"`
	Source  string `json:"source,omitempty"`
}

// GraphPlatform is what the platform node says beyond its reconcile state.
type GraphPlatform struct {
	Type                string `json:"type,omitempty"`
	OperatorVersion     string `json:"operatorVersion,omitempty"`
	RegistrationsAccess string `json:"registrationsAccess"`
}

// GraphKindCount is a number of nodes of one kind.
type GraphKindCount struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// GraphGroup is what a group node stands in for: configuration, pods or
// more.
type GraphGroup struct {
	Kind    string           `json:"kind"`
	Members []string         `json:"members,omitempty"`
	Hidden  []GraphKindCount `json:"hidden,omitempty"`
}

// GraphNode is one node. Ids are stable across restarts and never hold a
// UID. Fields that do not apply to the node's kind are absent.
type GraphNode struct {
	ID    string     `json:"id"`
	Kind  string     `json:"kind"`
	Label string     `json:"label"`
	Ref   *ObjectRef `json:"ref,omitempty"`
	// Access: ok, forbidden or notReadable; absent on nodes that stand for
	// no read.
	Access  string       `json:"access,omitempty"`
	Missing bool         `json:"missing,omitempty"`
	Health  *GraphHealth `json:"health,omitempty"`
	// Reconcile carries state, reason, message and retrying only.
	Reconcile        *Reconcile         `json:"reconcile,omitempty"`
	Owner            string             `json:"owner,omitempty"`
	RenderContracts  []string           `json:"renderContracts,omitempty"`
	Version          string             `json:"version,omitempty"`
	Path             string             `json:"path,omitempty"`
	Registration     *GraphRegistration `json:"registration,omitempty"`
	Catalog          *GraphCatalog      `json:"catalog,omitempty"`
	Platform         *GraphPlatform     `json:"platform,omitempty"`
	Group            *GraphGroup        `json:"group,omitempty"`
	MemberOf         string             `json:"memberOf,omitempty"`
	Replicas         *int64             `json:"replicas,omitempty"`
	ChildrenUnread   bool               `json:"childrenUnread,omitempty"`
	HiddenScaledDown int                `json:"hiddenScaledDown,omitempty"`
	Column           int                `json:"column"`
	Row              int                `json:"row"`
	X                int                `json:"x"`
	Y                int                `json:"y"`
}

// GraphPoint is a position in a layout.
type GraphPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// GraphEdge is one edge, with the field it was drawn from. Verified is set
// on an edge a second source can confirm, false with Reason when it does
// not.
type GraphEdge struct {
	ID       string       `json:"id"`
	Kind     string       `json:"kind"`
	From     string       `json:"from"`
	To       string       `json:"to"`
	Source   string       `json:"source"`
	Verified *bool        `json:"verified,omitempty"`
	Reason   string       `json:"reason,omitempty"`
	Route    []GraphPoint `json:"route"`
}

// Object is one object an inventory reaches, as the cluster serves it,
// without managed fields, the last-applied annotation or, on a
// ModuleInstance or ModulePackage, spec.values. It is the only document
// that carries a raw object; a YAML view renders it.
type Object struct {
	TypeMeta
	Ref    ObjectRef      `json:"ref"`
	Object map[string]any `json:"object"`
}

// TopicChange adds topics to an open stream and removes topics from it.
type TopicChange struct {
	Add    []string `json:"add,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

// Removed says a followed object does not exist: it was deleted, or was
// not there when the stream's snapshot was taken.
type Removed struct {
	TypeMeta
	Ref ObjectRef `json:"ref"`
}

// Problem codes. The set is open: a client treats an unknown code by its
// HTTP status.
const (
	CodeUnauthenticated     = "unauthenticated"
	CodeForbidden           = "forbidden"
	CodeNotFound            = "not_found"
	CodeBadRequest          = "bad_request"
	CodeMethodNotAllowed    = "method_not_allowed"
	CodeTooManyStreams      = "too_many_streams"
	CodeNotReadableByPortal = "not_readable_by_portal"
	CodeUpstreamUnavailable = "upstream_unavailable"
	ProblemContentType      = "application/problem+json"
	ProblemTypeBlank        = "about:blank"
)

// Problem is an RFC 9457 problem document. Code is the machine-readable
// part.
type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code"`
}
