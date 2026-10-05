// Package authz decides whether a caller may make one Kubernetes read, and
// hands back a Grant that proves the decision was made.
//
// # The seam
//
// Every read the portal makes starts here. A caller passes an Identity and
// the Attributes of the read (verb, group/version/resource, subresource,
// namespace, name) to Authorizer.Check. Check answers with either a Grant or
// a *DenialError; there is no third outcome, and every error is a denial.
// Read paths take a Grant as a parameter and, before every read, call
// Grant.Covers with the caller's identity and the read they are about to
// make. Covers, called by the read path, is the enforcement: it refuses the
// zero Grant, a grant issued to another identity, an expired grant, and any
// read the grant was not issued for. The Grant parameter makes a read path
// that never asked hard to write by accident; it does not prove that the
// path calls Covers, which review and the read path's tests must check.
//
// Only Checker.Check constructs a non-zero Grant. Grant has only unexported
// fields, so no other package can fill one in; the package's tests prove it
// by compiling a forging package that must fail, and by scanning the module's
// source for any other construction.
//
// # Authorize before lookup
//
// Check looks nothing up: it sees only the request's attributes and the
// caller's identity, and the local implementation holds a client for access
// reviews and nothing else. A caller without access therefore receives the
// same denial for an object that exists and one that does not, and the
// denial's message does not name the object (0030:D7).
//
// # Fail closed
//
// Check refuses before any Kubernetes call when:
//
//   - the identity's username is empty, blank or "system:anonymous", whatever
//     groups it carries;
//   - the verb is not get, list or watch (the portal is read-only);
//   - the subresource is not empty, status or log (a get on exec, attach,
//     portforward or proxy opens a stream into a workload or node);
//   - the resource is core "secrets" (the portal never reads Secret data);
//   - the attributes are malformed or use a wildcard.
//
// When the access review fails, times out, or reports an evaluation error
// without allowing, Check denies with CodeUnavailable. An error is never an
// allow and is never cached (0030:D6).
//
// # Local mode
//
// NewLocal builds a Checker that asks the cluster with
// SelfSubjectAccessReviews, so the answer is about whoever the kubeconfig
// authenticates as. That Checker serves exactly one Identity, the one it was
// built for; any other identity is refused before a review is sent, so a
// wiring mistake cannot lend the kubeconfig's access to another principal.
//
// # In-cluster mode
//
// NewInCluster builds a Checker that asks the cluster with
// SubjectAccessReviews naming the identity being checked: the signed-in
// person's username, UID, groups and extra values, with the read's exact
// attributes. It holds a SubjectAccessReview client and nothing else, so it
// never sends a self review, which in-cluster would answer for the portal's
// own ServiceAccount (0030:D6:R9).
//
// The Checker knows one more identity, the reader: the portal's
// ServiceAccount, as ServiceAccountIdentity builds it from a
// system:serviceaccount:<namespace>:<name> username (a flag, or the mounted
// token's subject through ServiceAccountFromToken). The read model's
// informers, polls and on-demand lists hold grants issued from reviews
// naming it, asked for when first needed and again once the cached decision
// expires. Every other identity is a person: a system username, or a system
// group other than system:authenticated, is refused before any review, so
// no person is ever answered with the ServiceAccount's access.
//
// # Access log
//
// With Options.AccessLog set, Check writes one line per decision about a
// person, allowed or denied, from the cache or not: the username, the read's
// attributes, the decision, the denial code and whether it was cached. It
// never writes groups, extra values or a denial's cause, and leaves out the
// in-cluster reader. In-cluster this is the only record of who read what,
// since the API server's audit log sees the portal's ServiceAccount.
//
// # Decision cache
//
// Allow and deny decisions are cached for a short time (30 seconds by
// default), keyed by the full identity and the full attributes. Group and
// extra-value order does not change the key. A Grant expires with the
// decision it was issued from, so a newly granted or revoked permission is
// seen within one TTL by new checks and by grants already held. A read path
// that holds a grant across many reads (a change stream, a log stream) gets
// ErrNoGrant from Covers once it expires and must call Check again.
package authz
