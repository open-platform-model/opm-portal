// Package v1alpha1 holds the wire types of the portal's read API,
// /api/v1alpha1: the documents its resources and its change stream return,
// and the problem document every error is. It holds no logic.
//
// The OpenAPI document openapi/v1alpha1.yaml describes the same types, and
// a test in internal/api holds the two together. Within v1alpha1 the types
// change only additively: a field or an enumerated value may be added, never
// removed or renamed. Clients ignore fields they do not know and treat every
// enumerated string as open, so an unknown value is shown as unknown.
//
// Times are RFC 3339 and absent when unknown. Lists that the resource always
// returns are present and empty rather than absent.
package v1alpha1
