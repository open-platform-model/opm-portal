# The deploy/ manifest

Covers `deploy/`, the kustomization that runs local mode in a Pod as a test tool, and the tests that constrain it.

`deploy/` is a kustomization that runs local mode in a Pod, as a test tool (portal:D13). It is plain YAML that users copy. Therefore:

- No `portal:` citation in its comments. A user who copies the file cannot open the design record.
- `go test ./deploy` holds the ClusterRole read-only: `get`, `list` and `watch` only, no Secrets, no `impersonate`, no wildcard.
- The same test refuses a Service, an Ingress, a probe or a non-loopback bind.
- The same test holds the image tag to `internal/version`. release-please rewrites the marked image line, so do not edit it by hand.
- A rule added to the role widens what every token holder sees. Make it a reviewed decision, not a side effect.

The Pod acts as its ServiceAccount for whoever holds its launch token, and prints its single-use launch URL on standard output (both portal:D13). These are the two sole exceptions in the security rules of `CONSTITUTION.md`, Principle V.

## Pitfalls

- Citing a decision in a `deploy/` comment.
- Editing the marked image line by hand.
- Adding a Service, an Ingress or a probe "to make it reachable": the test refuses it. Use `kubectl port-forward`, as `task e2e:pod` does.

## Check before you finish

- [ ] `go test ./deploy` passes.
- [ ] `task e2e:pod` passes after a change to the manifest (it needs `task e2e:up`).
- [ ] A new role rule was reviewed as a widening of what every token holder sees.
