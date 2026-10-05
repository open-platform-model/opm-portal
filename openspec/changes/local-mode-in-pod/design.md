## Context

Local mode (portal:D5) is a binary that binds loopback, prints a one-time launch link, trades it
for a session cookie, refuses any foreign `Host`, and reads as whatever identity its client
configuration authenticates as, after a SelfSubjectReview and before each read a
SelfSubjectAccessReview. Nothing in that path needs a kubeconfig file: client-go's
`NewNonInteractiveDeferredLoadingClientConfig` falls back to the in-cluster config when no
kubeconfig is found and the Pod environment is present.

Evidence, manual run on kind, 2026-10-05: the stock image (`Dockerfile`, distroless `nonroot`)
ran unchanged with `args: [serve, --addr, "127.0.0.1:8090"]`, `readOnlyRootFilesystem` and all
capabilities dropped. The SelfSubjectReview returned
`system:serviceaccount:opm-portal:opm-portal`; the launch URL was in `kubectl logs`;
`kubectl port-forward 8090:8090` reached the loopback-bound server (it dials `localhost` inside
the Pod's network namespace); `port-forward 8091:8090` got `403` from the `Host` check; with
`Host: 127.0.0.1:8090` the launch, the pages, `/api/v1alpha1/clusters/default/instances` and the
cert-manager graph worked. The startup line read `context=""`.

Supervisor rulings (2026-10-05, not reopened here): this is not in-cluster mode; the port
mismatch is not fixed in code; a new decision portal:D13 records it; the manifest lives under
`deploy/` with an explicit role; the image line carries the release-please marker; a Go test
holds the role read-only; the startup log is the only Go behaviour change; an e2e task deploys
the image into the fixture cluster.

## Goals / Non-Goals

**Goals:**

- `kubectl apply -k deploy/` gives a cluster administrator a portal they reach with
  `kubectl port-forward` and the launch link from `kubectl logs`, reading as a read-only
  ServiceAccount.
- The role cannot drift into a write verb, Secrets, impersonate or a wildcard without a test
  failing, and the manifest cannot grow a Service, an Ingress or a non-loopback bind.
- The trust model is written down where the decision lives and where users read.

**Non-Goals:**

- In-cluster mode (portal:D6): OIDC, SubjectAccessReview for a signed-in user, per-user access.
- Fixing the port mismatch: the `Host` allowlist names the bound port, and stays that way.
- Probes, a Service, an Ingress, TLS, more than one replica, a Helm chart or an OPM module.
- An automated check that the role covers every kind the pinned catalog renders (portal:D11:R2);
  that stays with the in-cluster plan.

## Decisions

### Local mode runs unchanged in the Pod (portal:D13)

The Deployment runs `opm-portal serve --addr 127.0.0.1:8090` with no kubeconfig. client-go loads
the in-cluster config, `selfIdentity` names the ServiceAccount, and `authz.NewLocal` sends
SelfSubjectAccessReviews as that ServiceAccount. Every local-mode safeguard stays: loopback bind,
launch token, `Host` allowlist, cross-origin protection, response headers, 12-hour sessions.

```text
browser ──http://127.0.0.1:8090──▶ kubectl port-forward 8090:8090 ──▶ Pod netns localhost:8090
                                                                         opm-portal serve
                                                                         (reads as SA opm-portal)
```

### The startup log names the in-cluster source

```go
// configSource returns the log attribute naming where the client
// configuration came from: the kubeconfig context, or the in-cluster
// config client-go falls back to when no kubeconfig holds a context.
func configSource(raw clientcmdapi.Config, contextName string) slog.Attr
```

`loadKubeconfig` already reads the raw config. When it holds no context at all and
`ClientConfig()` still succeeded, client-go used the in-cluster config (its
`DeferredLoadingClientConfig.ClientConfig` falls back only when the merged config is empty or the
default), so the log attribute is `source=in-cluster`; otherwise it stays `context=<name>`. The
message stays "reading as the kubeconfig's user" so existing log greps hold.

### Manifest shape

`deploy/kustomization.yaml` lists `namespace.yaml`, `serviceaccount.yaml`, `clusterrole.yaml`,
`clusterrolebinding.yaml`, `deployment.yaml`. The Deployment: 1 replica, `serviceAccountName:
opm-portal`, `automountServiceAccountToken` left at its default (the in-cluster config needs the
token), Pod `runAsNonRoot: true`, `seccompProfile: RuntimeDefault`; container
`readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`,
requests `cpu: 50m, memory: 64Mi`, limits `memory: 512Mi`, and no probes: a kubelet probe dials
the Pod IP, which a loopback bind does not answer, and the distroless image has no shell for an
exec probe. Comments in these files carry no `portal:` citation: a user copies them.

### Verbs, resources and grants

The ClusterRole `opm-portal-reader` lists every rule explicitly; there is no binding to `view`
and no aggregation label (portal:D11:R6 in spirit). Verbs come from the code:

| Group | Resources | Verbs | Read by (code) |
| --- | --- | --- | --- |
| `opmodel.dev` | `moduleinstances`, `modulepackages`, `platforms`, `transformerregistrations` | get, list, watch | `internal/readmodel/tier1.go` watches, `object.go` get |
| `events.k8s.io` | `events` | list | `internal/readmodel/events.go:73` (list with field selector; no get, no watch) |
| `""` | `pods` | get, list, watch | `children.go` watch and list, `internal/logs/source.go:30` get |
| `""` | `pods/log` | get | `internal/logs/source.go:38` (`GetLogs().Stream`) |
| `apps` | `replicasets` | get, list, watch | `children.go` (runtime children) |
| `batch` | `jobs` | get, list, watch | `children.go` (runtime children) |
| `""` | `configmaps`, `services`, `serviceaccounts`, `persistentvolumeclaims`, `namespaces` | get, list, watch | inventory kinds (`tier2.go` watches, `object.go` get, `poll.go` get) |
| `apps` | `deployments`, `statefulsets`, `daemonsets` | get, list, watch | inventory kinds |
| `batch` | `cronjobs` | get, list, watch | inventory kinds |
| `autoscaling` | `horizontalpodautoscalers` | get, list, watch | inventory kinds |
| `policy` | `poddisruptionbudgets` | get, list, watch | inventory kinds |
| `networking.k8s.io` | `networkpolicies` | get, list, watch | inventory kinds |
| `gateway.networking.k8s.io` | `httproutes`, `grpcroutes`, `tcproutes`, `tlsroutes` | get, list, watch | inventory kinds |
| `rbac.authorization.k8s.io` | `roles`, `rolebindings`, `clusterroles`, `clusterrolebindings` | get, list, watch | inventory kinds |
| `admissionregistration.k8s.io` | `mutatingwebhookconfigurations`, `validatingwebhookconfigurations`, `validatingadmissionpolicies`, `validatingadmissionpolicybindings` | get, list, watch | inventory kinds |
| `apiextensions.k8s.io` | `customresourcedefinitions` | get, list, watch | inventory kinds |

Not granted, and why:

- `secrets`: the read model refuses Secrets in code and the role never grants them (portal:D8).
- `selfsubjectreviews`, `selfsubjectaccessreviews` (`create`): `system:basic-user` allows both to
  every authenticated identity, so the role needs no `create` at all.
- Discovery: `system:discovery` allows it to every authenticated identity.
- `impersonate`, any write verb, any `*`.

The inventory kinds are the pinned catalog's transformer outputs: `opmodel.dev/catalogs/opm`
4.6.0 (`test/e2e/versions.env` `OPM_CATALOG_VERSION`), read from
`catalog_opm/src/transformers/*_transformer.cue` (`.release-please-manifest.json` `src: 4.6.0`),
minus `secret_transformer.cue`'s Secret. Every inventory kind in the F1 capture
(`testdata/clusters/f1/objects.yaml`: ConfigMap, Service, ServiceAccount, Namespace, Deployment,
Role, RoleBinding, ClusterRole, ClusterRoleBinding, CustomResourceDefinition,
MutatingWebhookConfiguration, ValidatingWebhookConfiguration, TransformerRegistration) is in the
list. `objects_transformer.cue` renders arbitrary objects as written, so it has no fixed kind, and
provider-defined kinds (cert-manager's Certificate, Issuer, ClusterIssuer) are not covered: an
inventory object the role does not cover shows as not readable, the existing behaviour
(portal:D11:R3). An automated catalog-coverage check (portal:D11:R2) stays with the in-cluster
plan.

### The manifest test

`deploy/manifest_test.go` (package `deploy`, tests only) reads every `deploy/*.yaml`, splits it
into documents and decodes each with `sigs.k8s.io/yaml` into `rbacv1.ClusterRole`,
`appsv1.Deployment` or a bare `metav1.TypeMeta`. It asserts:

```go
func checkRole(r rbacv1.ClusterRole) error      // verbs ⊆ {get, list, watch}; no "secrets"; no "impersonate"; no "*" in groups, resources, verbs; no nonResourceURLs; no aggregationRule
func checkObjects(kinds []string) error         // no Service, Ingress, Route, LoadBalancer-type object
func checkDeployment(d appsv1.Deployment) error // args bind 127.0.0.1; securityContext as above; no probes; 1 replica
```

Each `check*` runs over the shipped files and over a table of denied inputs (a role with
`create`, with `secrets`, with `impersonate`, with `*`, a Service, an `0.0.0.0` bind, a container
without `readOnlyRootFilesystem`), each of which must return an error. A second test reads the
`image:` line carrying `x-release-please-version` and requires its tag to be `"v" +
version.Version`.

### The image pin and release-please

```yaml
image: ghcr.io/open-platform-model/opm-portal:v0.0.0 # x-release-please-version
```

`release-please-config.json` `extra-files` gains `{"type": "generic", "path":
"deploy/deployment.yaml"}`.

## Research & Decisions

### How release-please rewrites the image line

**Context**: The manifest must name the released image, and release-please must move it with
every release.
**Explored**: `googleapis/release-please` `src/updaters/generic.ts` and `src/strategies/base.ts`
(`extraFileUpdates`), main branch, read 2026-10-05.
**Options considered**:
1. Plain string entry `"deploy/deployment.yaml"` - for a `.yaml` path release-please composes
   `GenericYaml('$.version')` with `Generic`. The YAML updater finds no top-level `version` and
   leaves the file alone, but if it ever matched it would re-serialize the file through
   `js-yaml` and drop every comment, the marker included.
2. Object entry `{"type": "generic", "path": "deploy/deployment.yaml"}` - only the `Generic`
   updater runs. On a line matching `x-release-please-version` it replaces the first match of
   `\d+\.\d+\.\d+(-[\w.]+)?(\+[-\w.]+)?` with the new version, so in `opm-portal:v0.0.0` it
   rewrites `0.0.0` and keeps the `v`.
**Decision**: Option 2.
**Rationale**: It runs the one updater that understands the marker and can never strip comments.

### Which tag the manifest names on main

**Context**: The supervisor asked for `v0.1.0` and for a test that the tag matches
`internal/version`. On `main`, `version.Version` is `0.0.0` until release PR 18 (0.1.0) merges.
**Options considered**:
1. `v0.1.0` now - names the image the next release publishes, but the equality test fails until
   the release PR merges.
2. `v0.0.0` now - equals `version.Version`; release-please rewrites both to `0.1.0` in the
   release PR, which it regenerates after this change merges, so every tagged commit names its
   own image.
**Decision**: Option 2.
**Rationale**: The tag on any commit then names the release that commit belongs to, and the
test holds from the first commit. `main` between releases names the last release, which is what
a user applying `main` can pull. Before 0.1.0 no image exists under any tag, so the README tells
users to apply `deploy/` from a release tag.

### The startup source signal

**Context**: In-cluster the startup line read `context=""`, which looks like a misconfiguration.
**Options considered**:
1. Call `rest.InClusterConfig()` ourselves when no kubeconfig exists - duplicates client-go's
   fallback and changes how the config is chosen.
2. Keep client-go's choice and label it: no context in the raw config and a successful
   `ClientConfig()` means the in-cluster fallback ran.
**Decision**: Option 2.
**Rationale**: Labels what happened without changing what happens; a pure function is unit
testable without a Pod.

### Image into kind for the e2e

**Context**: The e2e cluster runs on podman locally and docker in CI.
**Options considered**:
1. `kind load docker-image` - with the podman provider it depends on podman's image naming.
2. `$E2E_PROVIDER build -t localhost/opm-portal:e2e`, `$E2E_PROVIDER save -o <archive>`,
   `kind load image-archive` - the same path for both providers.
**Decision**: Option 2, image `localhost/opm-portal:e2e`, `imagePullPolicy: Never` in the overlay.
**Rationale**: One code path, no registry, and the tag is a fully qualified name containerd and
both engines agree on.

### The e2e overlay

`test/e2e/pod/kustomization.yaml` takes `../../../deploy` as its one resource, sets the image to
`localhost/opm-portal:e2e` with kustomize's `images:` field and patches `imagePullPolicy: Never`.
The shipped role is used unchanged, so the e2e reads through exactly what users apply.

## Risks / Trade-offs

- [The launch token sits in the Pod log] → Anyone with `pods/log` in `opm-portal` can read it,
  and anyone with `pods/portforward` there can reach the portal; together they read what the
  ServiceAccount reads. D13 states this, and the README says it in two sentences. Recovery from a
  spent or leaked token is `kubectl -n opm-portal rollout restart deploy/opm-portal`.
- [A shared identity] → Everyone who launches reads as the ServiceAccount, not as themselves.
  That is why D13 calls it a single-user test tool and not portal:D6's in-cluster mode.
- [A kind missing from the role] → Shows as not readable (portal:D11:R3), never as empty; the
  catalog-coverage check stays with the in-cluster plan.
- [No image before 0.1.0] → The README names applying from a release tag; the e2e builds its own.
- [Port-forward from another local port gets 403] → Documented and pinned by a scenario; the
  `Host` check stays, as DNS-rebinding defence.
