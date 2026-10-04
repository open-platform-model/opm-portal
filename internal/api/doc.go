// Package api serves the portal's read API, /api/v1alpha1, from the read
// model, the graph model and the change stream, as the wire types of
// api/v1alpha1.
//
// # Order of a request
//
// Every request runs the same steps: the mode's Authenticate resolves its
// principal (no principal is a 401 before any review); the route's cluster
// must be one the portal serves; path and query values are validated; every
// Kubernetes read the response will serve is authorized for the caller; only
// then is anything looked up (0030:D7). A caller who may not make a read gets
// one constant forbidden problem, whichever check failed and whether or not
// the object exists. Lists the caller may not read are empty and say
// forbidden, with no count.
//
// # What is never served
//
// The read model never holds spec.values, Secret data or the last-applied
// annotation, and no wire type has a field for them (0030:D8). Condition and
// history messages and event notes are served verbatim: whether the
// operator redacts secret paths in them is 0030:OQ8. In local mode the
// caller's kubeconfig reads the same text with kubectl; milestone 2 must
// decide before the in-cluster release.
//
// # The change stream
//
// Each stream topic carries one document, the one its GET returns, rendered
// for each subscriber by the GET's own code path. The producer here turns
// the read model's change feed into coalesced upserts and refreshes topics
// the feed cannot see (events, polled objects) on a timer.
package api
