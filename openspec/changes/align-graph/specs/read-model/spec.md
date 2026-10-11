## ADDED Requirements

### Requirement: A package's source is read on demand for the caller

The read model SHALL read the source a ModulePackage's `spec.sourceRef` names only when its kind is
OCIRepository, GitRepository or Bucket, always in `source.toolkit.fluxcd.io` whatever the
sourceRef's `apiVersion` says, in the sourceRef's namespace or else the package's, with one `get` for a caller whose grant covers that `get`, after the reader
identity's own `get` review. It SHALL NOT list or watch source objects, follow a source's
`secretRef`, or read a core Secret, which it SHALL refuse before any review or read. It SHALL return
the source's Ready condition status, reason and message and `status.artifact.revision`, and SHALL
NOT return `status.artifact.url`. A Ready source with no `status.artifact` SHALL be reported as
having no artifact, not as ready. A kind outside the three SHALL be reported as unsupported without
a review or a read; an object that does not exist SHALL be reported as not found; a reader denied
the `get` SHALL make the source not readable. Discovery of the source kinds SHALL be re-checked by
the server on a timer, never on a request, so that a kind installed after the portal starts is
found. Source: portal:D1, portal:D7:R3, portal:D8:R1, portal:D20.

#### Scenario: A ready OCIRepository

- **WHEN** a caller allowed to get OCIRepositories in `pkg` reads `pkg/podinfo-release`, which has
  `Ready=True` and artifact revision `v6.7.0@sha256:4f1c9a2e`
- **THEN** the result is ready with that revision and carries no artifact URL

#### Scenario: A kind the controller does not read

- **WHEN** a package's `spec.sourceRef` names kind `ConfigMap`
- **THEN** the result is unsupported, and the cluster receives no access review and no read

#### Scenario: A Secret named as a source

- **WHEN** a package's `spec.sourceRef` names a core `Secret`
- **THEN** the read is refused without a review or a read

#### Scenario: The reader may not read the source

- **WHEN** the caller may get the OCIRepository but the reader identity's own `get` review is
  denied
- **THEN** the source is not readable and nothing is read

#### Scenario: A source that does not exist

- **WHEN** a caller allowed to get OCIRepositories in `pkg` reads `pkg/absent`, which does not
  exist
- **THEN** the result is not found

#### Scenario: Ready without an artifact

- **WHEN** the OCIRepository has `Ready=True` and no `status.artifact`
- **THEN** the result says the source has no artifact, not that it is ready

#### Scenario: Flux installed after the portal starts

- **WHEN** the cluster starts serving OCIRepositories after the portal started
- **THEN** within one discovery timer period, a lookup finds the kind served
