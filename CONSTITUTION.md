# Open Platform Model Portal Constitution

## Purpose

This document is the reader-friendly reference for the principles that shape the portal's
design, implementation, validation and change management. The normative source is
`openspec/config.yaml`; when the two disagree, the config wins and this file is fixed.

## Design Principles

| # | Principle | Summary |
| ---- | --------- | ------- |
| **I** | [Read Through the Published Surfaces](#i-read-through-the-published-surfaces) | The portal reads the Kubernetes API and published OPM Go modules, and re-implements no operator semantics |
| **II** | [Separation of Concerns](#ii-separation-of-concerns) | Authorization, read model, derived views, API and UI stay in separate packages |
| **III** | [The Read API Is the Contract](#iii-the-read-api-is-the-contract) | `/api/v1alpha1` is the durable interface; it changes only additively within a version |
| **IV** | [Status Is Read, Never Inferred](#iv-status-is-read-never-inferred) | The portal shows what the cluster says, and shows unknown as unknown |
| **V** | [Read-Only and Least Privilege](#v-read-only-and-least-privilege) | No writes (three non-persisted review APIs aside), no Secret data or values, act as the user, fail closed on an empty identity |
| **VI** | [Semantic Versioning and Commit Discipline](#vi-semantic-versioning-and-commit-discipline) | SemVer on the 0.x line; Conventional Commits decide what releases |
| **VII** | [Simplicity & YAGNI](#vii-simplicity--yagni) | Standard library first; complexity must be justified |
| **VIII** | [Mergeable Sections](#viii-mergeable-sections) | Every section ends green and commits; every merge leaves `main` releasable |

---

### I. Read Through the Published Surfaces

The portal MUST read OPM state only through the Kubernetes API (the operator's custom
resources, events, pod logs, and the objects an instance's inventory names) and through
published OPM Go modules.

- What the operator writes to `status` is what the portal shows
- The portal does not evaluate CUE, render modules or recompute what the operator decided
- A portal need the operator does not expose is an operator change, not a portal workaround

---

### II. Separation of Concerns

The codebase MUST keep these package boundaries:

- `cmd/opm-portal/` parses flags and environment, selects the mode and wires the parts
- `api/v1alpha1/` holds the public wire types of the read API and no logic
- `internal/authz/` makes the authorization decision; every read takes the grant it returns
- `internal/readmodel/` owns the cache, watches, on-demand reads and joins
- `internal/graph/` and `internal/health/` derive views as pure functions of read-model data
- `internal/stream/` and `internal/logs/` serve server-sent event topics and bounded log readers
- `internal/api/` serves `/api/v1alpha1`; `internal/ui/` renders HTMX pages from the same DTOs
- `internal/auth/` owns identity: the local launch token in milestone 1, OIDC in milestone 2 (a
  future plan)

A package is created by the first change that needs it, never ahead of it.

```text
auth -> authz -> readmodel -> graph/health -> api -> ui
  |        |          |             |           |      |
identity  grant     read         derive      serve  render
```

---

### III. The Read API Is the Contract

`/api/v1alpha1` is the durable interface. The HTMX UI is its first consumer; adapters for other
tools come later and read the same API.

- Within a version, the API changes only additively: new fields, new resources
- Removing or renaming a field, or changing its meaning, needs a new API version
- The UI never reads anything the API does not expose

---

### IV. Status Is Read, Never Inferred

The portal reports what the cluster says, with its source.

- `Ready` on a ModuleInstance means applied, not healthy; the portal shows the two apart
- An unknown condition, kind or API version renders as unknown, never as a guess
- A degraded read (missing permission, unreachable API) is shown as degraded, never as empty

---

### V. Read-Only and Least Privilege

- V1 MUST NOT create, update, patch or delete any Kubernetes object. The only allowed writes
  are `create` on review APIs that answer in the response and store nothing, and which ones
  depends on the mode:
  - Local mode (milestone 1) MUST create only `authorization.k8s.io`
    `selfsubjectaccessreviews`, to ask with the user's kubeconfig whether the user may make a
    read so the UI can show locked nodes up front, and `authentication.k8s.io`
    `selfsubjectreviews`, to learn which identity that kubeconfig authenticates as
    (portal:D5:R6)
  - In-cluster mode (milestone 2) MUST create only `authorization.k8s.io`
    `subjectaccessreviews`, to check each read for the signed-in user (portal:D6:R9); it swaps
    the backend behind the same seam
  - In-cluster mode MUST NOT create a `selfsubjectaccessreviews` or `selfsubjectreviews`: there
    it would check the portal's own ServiceAccount, not the user
- The portal MUST NOT read Secret data
- In V1 the portal MUST NOT serve any instance's or package's `spec.values`, and MUST strip the
  `kubectl.kubernetes.io/last-applied-configuration` annotation from every object it serves
  (portal:D8)
- The portal acts as the user: in milestone 1 with the user's kubeconfig, in milestone 2 through
  a SubjectAccessReview for the signed-in user
- An empty or unmapped identity MUST fail closed; the portal never falls back to its own
  ServiceAccount (a Pod running local mode reads as its configured ServiceAccount per
  portal:D13; the rule still binds in-cluster mode)
- Tokens, kubeconfig content and `Authorization` headers MUST NOT appear in logs or errors

`AGENTS.md`, "Security Rules", lists the working rules that follow from this principle.

---

### VI. Semantic Versioning and Commit Discipline

Releases MUST follow SemVer 2.0.0. Commits MUST follow Conventional Commits v1 in the form
`type(scope): description`.

0.x line: the portal releases on `0.x` (first release `0.1.0`) until its read API declares
stability. A breaking change is a `!` in the PR title (`feat!:`), the only text the squash commit
carries (`squash_merge_commit_message: BLANK`); on the 0.x line it bumps the minor version, never
the major. The migration note goes in the PR body, which the CHANGELOG entry links.

| Type | Releases |
| ---- | -------- |
| `feat`, `fix`, `perf`, `revert`, `deps`, `refactor` | yes |
| `docs`, `test`, `ci`, `build`, `chore` | no |

Recommended scopes: `api`, `ui`, `authz`, `auth`, `readmodel`, `graph`, `health`, `stream`,
`logs`, `version`, `release`.

---

### VII. Simplicity & YAGNI

Start with the simplest implementation that satisfies the current requirement. New complexity
MUST be justified by a concrete need.

- Standard library first: `net/http` routing, `html/template`, `log/slog`
- No JavaScript build step and no client-side component state in V1
- A dependency earns its place in the change that adds it

---

### VIII. Mergeable Sections

A change is delivered as the sections of its `tasks.md` (`## N. Title` headings with `N.M`
checkboxes). Two invariants hold at every section boundary; they replace any size limit on the
change itself.

- Every merge leaves `main` releasable: a section MUST end green under the validation gates and
  MUST close with a commit task naming its Conventional Commit
- Work survives a session boundary: the commit task is the pause point, leaving checked boxes and
  a clean tree for the next session to resume from
- A change SHOULD cut into at most about five sections; one PR per change with one commit per
  section is the default
- Section 1 is a spike whenever the design carries an unverified assumption

#### Execution Gate

Before beginning any implementation, the request MUST be evaluated against the
mergeable-sections principle.

If the request cannot be cut into sections that each end green and leave `main` releasable, or
needs more than about five, the required response is:

> "🛑 **Scope Warning**: This request does not cut into a handful of mergeable sections. I suggest we split it into the following changes: [list 2-3 changes, each a few sections that leave main releasable]. Should we start with the first?"

---

## Quality Gates

Before merge, these MUST pass (`task check` runs all six):

1. `task fmt`
2. `task vet`
3. `task lint`
4. `task test`
5. `task openspec:check`
6. `task e2e:capture:check`

## Further Reading

- `openspec/config.yaml`: normative constitutional source
- `AGENTS.md`: repository mechanics, commands, security rules and coding guidance
