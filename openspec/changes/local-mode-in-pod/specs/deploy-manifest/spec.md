## ADDED Requirements

### Requirement: The manifest runs local mode and opens no network path

The repository SHALL ship `deploy/`, a kustomization of plain YAML holding exactly a Namespace
`opm-portal`, a ServiceAccount `opm-portal`, a ClusterRole `opm-portal-reader`, a
ClusterRoleBinding of that role to that ServiceAccount, and a Deployment of one replica running
`opm-portal serve --addr 127.0.0.1:8090` as that ServiceAccount. It SHALL hold no Service, no
Ingress, no Gateway route and no probe, so `kubectl port-forward` is the only way to reach the
portal. The Pod SHALL run as non-root with the `RuntimeDefault` seccomp profile, and the container
SHALL have a read-only root filesystem, no privilege escalation, every capability dropped, and
memory requests and limits. A Go test SHALL parse every file under `deploy/` and SHALL fail on a
Service or Ingress object, a probe, a bind address other than `127.0.0.1`, or a missing or
loosened security setting. Comments in the files SHALL carry no design citation. Source:
portal:D13:R2.

#### Scenario: Apply the manifest

- **WHEN** a cluster administrator runs `kubectl apply -k deploy/`
- **THEN** the five objects exist and the Deployment rolls out one Pod
- **AND** `kubectl -n opm-portal get services,ingresses` lists nothing

#### Scenario: A Service is added

- **WHEN** a change adds a Service to `deploy/` or binds the container to `0.0.0.0:8090`
- **THEN** the manifest test fails naming the object or the address

#### Scenario: The security context is loosened

- **WHEN** a change removes `readOnlyRootFilesystem` or sets `allowPrivilegeEscalation: true`
- **THEN** the manifest test fails naming the setting

### Requirement: The role only reads, and never Secrets

The ClusterRole `opm-portal-reader` SHALL list every rule explicitly and SHALL grant only the
verbs `get`, `list` and `watch`: on the four OPM kinds of `opmodel.dev`; on `events.k8s.io`
`events` only the verb the portal uses, `list`; on `pods` and `get` on `pods/log`; on `apps`
`replicasets` and `batch` `jobs`; and on every non-Secret kind the pinned OPM catalog's
transformers render. It SHALL NOT grant any other verb, `secrets`, `impersonate`, a `*` in any
group, resource or verb, a non-resource URL, or an aggregation rule, and no manifest SHALL bind
the ServiceAccount to `view` or another built-in role. A Go test SHALL fail on any of these, and
SHALL show it does by refusing a table of denied roles. An inventory object of a kind the role
does not grant SHALL show as not readable, never omitted. Source: portal:D13:R3,
portal:D11:R1/R3.

#### Scenario: The shipped role

- **WHEN** the manifest test reads `deploy/clusterrole.yaml`
- **THEN** every verb is `get`, `list` or `watch`, and no rule names `secrets`, `impersonate` or
  `*`

#### Scenario: A denied role

- **WHEN** the manifest test checks a role granting `create` on `pods`, `get` on `secrets`,
  `impersonate` on `users`, or `*` as a resource
- **THEN** each check returns an error naming the offending rule

#### Scenario: A provider kind

- **WHEN** an instance's inventory names a cert-manager Certificate and the portal runs in the Pod
- **THEN** the object shows as not readable by the portal, and the rest of the instance renders

### Requirement: The manifest names the image of its own release

The Deployment's `image:` line SHALL name `ghcr.io/open-platform-model/opm-portal:v<version>` and
SHALL carry the `x-release-please-version` marker, and `release-please-config.json` SHALL list
`deploy/deployment.yaml` as a `generic` extra file, so each release PR moves the tag with
`internal/version`. A Go test SHALL fail when the tag is not `"v" + version.Version` or the marker
is not on the image line.

#### Scenario: A release PR

- **WHEN** release-please opens the release PR for version 0.2.0
- **THEN** it rewrites the image line to `ghcr.io/open-platform-model/opm-portal:v0.2.0` with the
  marker kept, beside `internal/version/version.go`

#### Scenario: A hand edit drifts

- **WHEN** the image tag differs from `"v" + version.Version`
- **THEN** the manifest test fails naming both values
