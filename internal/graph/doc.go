// Package graph derives the portal's relationship graphs from read-model
// views: the graph of one instance or package, and the platform graph. It
// reads nothing from a cluster; every value comes from a view the caller
// was authorized for.
//
// # What a graph may say
//
// Every node and edge comes from a field a Kubernetes object carries, and
// each edge kind is drawn from exactly one source, which the edge names
// (Sources). Where a second source can confirm an edge (a registration's
// provider reference against the provider's inventory) the edge says
// whether it does, rather than the graph picking one. No edge relates an
// instance to a contract: the contracts an instance's render used are text
// on its node (0030:D4).
//
// # Ids
//
// Node ids are "<prefix>:<part>/<part>...", each part path-escaped, built
// from kinds, groups, namespaces and names, never from a UID: an id
// survives a restart and a delete and recreate of its object.
//
// # Collapse rules
//
//   - Two or more configuration components of one owner (no workload
//     among their objects) are one group node; Options.Expand shows them.
//   - ReplicaSets scaled to zero are hidden behind a count on their parent;
//     Options.ShowScaledDown shows them.
//   - More than five Pods under one parent are one group node.
//   - Past Options.NodeCap, nodes are dropped from the last column back and
//     one summary node counts them by kind.
//
// # Layout
//
// Each scope has a fixed column per node kind. Within a column nodes are
// ordered by the barycenter of their neighbors with a stable tie-break,
// and placed on an integer grid; each edge gets a cubic Bézier route. The
// same views always give the same graph, byte for byte.
package graph
