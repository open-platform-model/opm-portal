package graph

import (
	"github.com/open-platform-model/opm-portal/internal/health"
)

// Scope is what a graph is about.
type Scope string

// Scopes.
const (
	ScopeInstance Scope = "instance"
	ScopePackage  Scope = "package"
	ScopePlatform Scope = "platform"
)

// NodeKind is what a node stands for.
type NodeKind string

// Node kinds.
const (
	KindPlatform     NodeKind = "platform"
	KindCatalog      NodeKind = "catalog"
	KindRegistration NodeKind = "registration"
	KindInstance     NodeKind = "instance"
	KindPackage      NodeKind = "package"
	KindModule       NodeKind = "module"
	KindSource       NodeKind = "source"
	KindComponent    NodeKind = "component"
	KindObject       NodeKind = "object"
	KindRuntime      NodeKind = "runtime"
	KindGroup        NodeKind = "group"
)

// GroupKind is what a group node stands in for.
type GroupKind string

// Group kinds.
const (
	// GroupConfiguration: the components of one owner that hold no
	// workload.
	GroupConfiguration GroupKind = "configuration"
	// GroupPods: the Pods under one parent, when there are more than
	// podGroupThreshold.
	GroupPods GroupKind = "pods"
	// GroupMore: the nodes the node cap dropped.
	GroupMore GroupKind = "more"
)

// EdgeKind is the relation an edge draws. Each kind has exactly one source
// of truth, which every edge of the kind names.
type EdgeKind string

// Edge kinds. No kind relates an instance to a contract: the contracts an
// instance's render used are not the provider contracts it demands, so they
// are text on the instance node (0030:D4).
const (
	EdgeResolves     EdgeKind = "resolves"
	EdgeContributes  EdgeKind = "contributes"
	EdgeProvidedBy   EdgeKind = "providedBy"
	EdgeInstantiates EdgeKind = "instantiates"
	EdgeSourcedFrom  EdgeKind = "sourcedFrom"
	EdgeDependsOn    EdgeKind = "dependsOn"
	EdgeHasComponent EdgeKind = "hasComponent"
	EdgeOwns         EdgeKind = "owns"
	EdgeControls     EdgeKind = "controls"
)

// Sources names the one field each edge kind is drawn from.
var Sources = map[EdgeKind]string{
	EdgeResolves:     "Platform status.registry",
	EdgeContributes:  "Platform status.registry entries with source Registration, joined on TransformerRegistration spec.catalog",
	EdgeProvidedBy:   "TransformerRegistration spec.providerRef, checked against the provider's status.inventory",
	EdgeInstantiates: "ModuleInstance spec.module",
	EdgeSourcedFrom:  "ModulePackage spec.sourceRef",
	EdgeDependsOn:    "ModulePackage spec.dependsOn",
	EdgeHasComponent: "status.inventory entries, by component",
	EdgeOwns:         "status.inventory entries",
	EdgeControls:     "metadata.ownerReferences, controller only",
}

// Reasons an edge from a registration to its provider is unverified.
const (
	ReasonProviderNotFound       = "ProviderNotFound"
	ReasonProviderUnreadable     = "ProviderUnreadable"
	ReasonNotInProviderInventory = "NotInProviderInventory"
)

// Graph is one graph with its layout.
type Graph struct {
	Scope Scope `json:"scope"`
	// Root is the id of the node the graph is about.
	Root   string `json:"root"`
	Layout Layout `json:"layout"`
	// Nodes are sorted by column, then row.
	Nodes []Node `json:"nodes"`
	// Edges are sorted by id.
	Edges []Edge `json:"edges"`
}

// Layout is the size of a laid-out graph and its columns.
type Layout struct {
	Width      int      `json:"width"`
	Height     int      `json:"height"`
	NodeWidth  int      `json:"nodeWidth"`
	NodeHeight int      `json:"nodeHeight"`
	Columns    []Column `json:"columns"`
}

// Column is one column of a layout. Columns that hold no node are left out.
type Column struct {
	Title string `json:"title"`
	X     int    `json:"x"`
}

// Ref names the Kubernetes object a node stands for.
type Ref struct {
	Group     string `json:"group,omitempty"`
	Version   string `json:"version,omitempty"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

// Health is a node's workload health as internal/health computed it.
type Health struct {
	State   health.State `json:"state"`
	Reason  string       `json:"reason,omitempty"`
	Message string       `json:"message,omitempty"`
	// Partial: something below was left out because it could not be read.
	Partial bool `json:"partial,omitempty"`
	// NotLive: something below is refreshed by polling.
	NotLive bool    `json:"notLive,omitempty"`
	Counts  *Counts `json:"counts,omitempty"`
}

// Counts tallies what a summary counted, by health state and by access.
type Counts struct {
	Healthy     int `json:"healthy,omitempty"`
	Progressing int `json:"progressing,omitempty"`
	Degraded    int `json:"degraded,omitempty"`
	Missing     int `json:"missing,omitempty"`
	Unknown     int `json:"unknown,omitempty"`
	Forbidden   int `json:"forbidden,omitempty"`
	NotReadable int `json:"notReadable,omitempty"`
	Withheld    int `json:"withheld,omitempty"`
}

// Applied is an operator object's applied state, never merged with health
// (0030:D3).
type Applied struct {
	State    health.AppliedState `json:"state"`
	Reason   string              `json:"reason,omitempty"`
	Message  string              `json:"message,omitempty"`
	Retrying bool                `json:"retrying,omitempty"`
}

// Registration is a TransformerRegistration's claim and standing:
// acceptance and activation as two values, read from the status fields.
type Registration struct {
	Catalog       string         `json:"catalog"`
	Version       string         `json:"version,omitempty"`
	Provides      []string       `json:"provides,omitempty"`
	Accepted      bool           `json:"accepted"`
	Active        bool           `json:"active"`
	Verdict       health.Verdict `json:"verdict"`
	Reason        string         `json:"reason,omitempty"`
	Message       string         `json:"message,omitempty"`
	ActiveReason  string         `json:"activeReason,omitempty"`
	ActiveMessage string         `json:"activeMessage,omitempty"`
}

// Catalog is one entry of the Platform's resolved registry.
type Catalog struct {
	Version string `json:"version,omitempty"`
	Enabled bool   `json:"enabled"`
	Source  string `json:"source,omitempty"`
}

// PlatformFacts is what the platform node says beyond its applied state.
type PlatformFacts struct {
	Type            string `json:"type,omitempty"`
	OperatorVersion string `json:"operatorVersion,omitempty"`
	// RegistrationsAccess says whether the registrations shown are all of
	// them.
	RegistrationsAccess health.Access `json:"registrationsAccess"`
}

// Group is what a group node stands in for.
type Group struct {
	Kind GroupKind `json:"kind"`
	// Members are the labels of what the group holds, in order.
	Members []string `json:"members,omitempty"`
	// Hidden counts, for the node-cap summary, the dropped nodes by kind.
	Hidden []KindCount `json:"hidden,omitempty"`
}

// KindCount is a number of nodes of one kind.
type KindCount struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// Node is one node of a graph. Fields that do not apply to its kind are
// empty.
type Node struct {
	ID    string   `json:"id"`
	Kind  NodeKind `json:"kind"`
	Label string   `json:"label"`
	Ref   *Ref     `json:"ref,omitempty"`
	// Access says how reading the object went; empty for nodes that stand
	// for no read (a catalog, a module, a group).
	Access health.Access `json:"access,omitempty"`
	// Missing: the object was read and does not exist.
	Missing bool     `json:"missing,omitempty"`
	Health  *Health  `json:"health,omitempty"`
	Applied *Applied `json:"applied,omitempty"`

	// Owner is an instance's owner: operator or cli.
	Owner string `json:"owner,omitempty"`
	// RenderContracts are every contract an instance's render used, most
	// fulfilled by the catalog itself: not the provider contracts it
	// demands, so no edge is drawn to them (0030:D4:R3).
	RenderContracts []string `json:"renderContracts,omitempty"`
	// Version is a module's version.
	Version string `json:"version,omitempty"`
	// Path is a package's path inside its source.
	Path         string         `json:"path,omitempty"`
	Registration *Registration  `json:"registration,omitempty"`
	Catalog      *Catalog       `json:"catalog,omitempty"`
	Platform     *PlatformFacts `json:"platform,omitempty"`
	Group        *Group         `json:"group,omitempty"`
	// MemberOf is the id of the group an expanded member belongs to, so a
	// consumer can collapse it again.
	MemberOf string `json:"memberOf,omitempty"`

	// Replicas is a ReplicaSet's desired replica count.
	Replicas *int64 `json:"replicas,omitempty"`
	// ChildrenUnread: the runtime children below this workload could not be
	// read, so its health is the status rules alone.
	ChildrenUnread bool `json:"childrenUnread,omitempty"`
	// HiddenScaledDown counts the ReplicaSets scaled to zero hidden below
	// this node.
	HiddenScaledDown int `json:"hiddenScaledDown,omitempty"`

	Column int `json:"column"`
	Row    int `json:"row"`
	X      int `json:"x"`
	Y      int `json:"y"`
}

// Edge is one edge of a graph.
type Edge struct {
	ID   string   `json:"id"`
	Kind EdgeKind `json:"kind"`
	From string   `json:"from"`
	To   string   `json:"to"`
	// Source is the field the edge was drawn from (Sources).
	Source string `json:"source"`
	// Verified is set on edges a second source can confirm: true when it
	// does, false with Reason when it does not (0030:D4:R2).
	Verified *bool  `json:"verified,omitempty"`
	Reason   string `json:"reason,omitempty"`
	// Route is a cubic Bézier: start, two control points, end.
	Route []Point `json:"route"`
}

// Point is a position in the layout.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Options adjust what a graph shows.
type Options struct {
	// Expand holds the ids of group nodes to show expanded.
	Expand []string
	// ShowScaledDown shows ReplicaSets scaled to zero instead of hiding
	// them behind a count.
	ShowScaledDown bool
	// NodeCap is the most nodes a graph holds; 0 means DefaultNodeCap.
	NodeCap int
}

// DefaultNodeCap is the node cap when Options leaves it at zero.
const DefaultNodeCap = 150
