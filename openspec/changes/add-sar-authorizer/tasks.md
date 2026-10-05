## 1. In-cluster SubjectAccessReview backend (internal/authz)

- [x] 1.1 Add `ServiceAccountIdentity` and `ServiceAccountFromToken`; verify with table tests for a valid username, a malformed one (missing parts, extra parts, not a ServiceAccount), and tokens with a valid `sub`, a missing claim, bad base64 and a non-JWT
- [x] 1.2 Add `NewInCluster` and the `subjectReviewer` backend (reader route by full key, person route refusing `system:` usernames and groups other than `system:authenticated`, exact user, UID, groups, extra and attributes, `verdict`); verify with fake-clientset tests that record every action: exact review contents for a person and for the reader, verdict mapping, errors and timeouts unavailable and uncached, the reader reviewed once per TTL, system identities refused with no action
- [x] 1.3 Add the regression tests: an identity with empty claims (no username, with groups, UID and extra) records zero actions and never a review naming the ServiceAccount; across people, the reader, empty and system identities, every recorded action is a create of `subjectaccessreviews` and none is a self review
- [x] 1.4 Update `doc.go` with the in-cluster mode; verify `go doc ./internal/authz` prints it
- [x] 1.5 `task check` green, then commit `feat(authz): authorize as the user in-cluster`

## 2. Per-user access log (internal/authz)

- [x] 2.1 Add `Options.AccessLog` and log one line per decision in `Checker.Check` (allow, deny, cached, every denial code), skipping the in-cluster reader; verify with a JSON slog handler that lines carry the user, the attributes, the decision, the code and `cached`, and never groups, extra values or a review error's text
- [x] 2.2 `task check` green, then commit `feat(authz): log each in-cluster access decision per user`
