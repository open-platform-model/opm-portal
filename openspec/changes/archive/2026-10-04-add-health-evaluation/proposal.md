## Why

The portal's main promise is that a broken rollout looks broken. The operator's `Ready=True`
means every apply succeeded, not that the workload runs: on a live cluster an instance whose new
Pod could not pull its image stayed `Ready=True`, and its Deployment `Available=True`, for the
whole ten-minute progress deadline (enhancement 0030, experiment 01, observation 3). 0030:D3
keeps the two apart: an **Applied** axis read from the operator's conditions, and a **Health**
axis the portal computes from live objects. Every later change that shows status (the read
model, the graph, the read API, the UI) needs both computations, so they land first, as pure
functions tested against the objects the live capture recorded.

## What Changes

- New package `internal/health`, pure functions of objects the read model will hold:
  - per-object health from the standard Kubernetes status rules (kstatus, the Flux fork the
    operator already links), mapped to Healthy, Progressing, Degraded, Missing and Unknown;
  - the Pod rule: a Pod with a container waiting as `ErrImagePull`, `ImagePullBackOff`,
    `CrashLoopBackOff`, `CreateContainerConfigError` or `InvalidImageName` is Degraded, and so
    is the inventory workload that owns it through its controller chain (a ReplicaSet below a
    Deployment, for example);
  - a worst-of roll-up from objects to components to the instance, which excludes objects the
    reader could not read and then marks the result partial, never reads a Secret and never
    counts one against completeness, and carries when the result was evaluated and whether it
    is live;
  - the Applied axis for ModuleInstance, ModulePackage, Platform and TransformerRegistration:
    `Ready=True` is Applied (never healthy), a CLI-owned instance is managed externally and
    neutral, `ContractsFulfilled=False` and `Drifted=True` are informational notes, and failure
    counters are never read;
  - a registration's acceptance and activation read from `status.accepted` and
    `status.active` (absent means false), with a blocked removal kept apart from a refusal;
  - a table that explains every condition reason the operator writes, checked by a test
    against a copied list of the operator's reason constants.
- Test inputs are objects captured live (podinfo before, one minute after and ten minutes
  after the image break; cert-manager healthy with 42 inventory objects; a CLI-owned instance;
  every operator sample of experiment 01), trimmed of managed fields, annotations and values.
- New dependencies: `github.com/fluxcd/cli-utils` (kstatus) at the version opm-operator
  links, `k8s.io/apimachinery` for the unstructured object type kstatus takes, and, in tests
  only, `sigs.k8s.io/yaml` to turn the captured YAML into JSON so integers decode as `int64`
  the way client-go decodes them (already in the module graph through apimachinery).

Out of scope: reading anything from a cluster (the read model), any API field or page that
shows the axes, and liveness of the input (watches versus polling are the read model's;
this change only carries the per-object evaluation time and live flag through the roll-up).

## Capabilities

### New Capabilities

- `health-evaluation`: how the portal derives an object's, a component's and an instance's
  workload health from live objects, how it reads applied state and registration verdicts
  from the operator's status, and how it explains an operator reason.

### Modified Capabilities

None.

## Impact

- Packages: adds `internal/health` (derive layer, Principle II). No API resource or UI page
  changes yet; the read API change will expose these values.
- SemVer: MINOR after 1.0 (new capability, nothing removed). On the 0.x line it ships with a
  hidden `chore(health)` type because nothing a user runs changes until the read API exposes it.
- Principle V: no effect. The package reads nothing from a cluster; it evaluates objects it is
  handed and refuses to evaluate a Secret it is handed. Testdata holds no Secret, kubeconfig,
  token or instance values.
- Principle VII: kstatus is the dependency the design names (0030:D3) and the operator's own;
  re-implementing the per-kind status rules would make the portal disagree with the operator
  and with Flux.
