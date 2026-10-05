## ADDED Requirements

### Requirement: Local mode runs unchanged in a Pod with in-cluster credentials

When `opm-portal serve` finds no kubeconfig and client-go loads the in-cluster configuration, the
portal SHALL run local mode unchanged: it SHALL bind only the loopback address `--addr` names,
SHALL learn its identity, the Pod's ServiceAccount, with one SelfSubjectReview and fail closed on
an empty or anonymous one, SHALL print the launch URL on standard output, which becomes the
container log, and SHALL apply the launch token, the session cookie, the `Host` allowlist, the
cross-origin refusal and the security headers exactly as on a user's machine. Every read SHALL be
made as that ServiceAccount after a SelfSubjectAccessReview for it. The startup log line naming
the identity SHALL carry `source=in-cluster` in place of an empty context name. A browser reaches
the portal only through `kubectl port-forward` to the bound port, and the `Host` allowlist SHALL
admit only the bound port, so a forward from any other local port is refused. Source:
portal:D13:R1/R2, portal:D5:R3/R4.

#### Scenario: The Pod starts in local mode

- **WHEN** the shipped Deployment starts `opm-portal serve --addr 127.0.0.1:8090` with no
  kubeconfig in the Pod
- **THEN** the container log names the user `system:serviceaccount:opm-portal:opm-portal` with
  `source=in-cluster`
- **AND** the container log holds one `Open this link once to sign in:
  http://127.0.0.1:8090/launch?token=...` line

#### Scenario: Port-forward to the bound port

- **WHEN** a user runs `kubectl -n opm-portal port-forward deploy/opm-portal 8090:8090` and opens
  the launch URL from the Pod log
- **THEN** the launch answers `200` with a session cookie
- **AND** `/api/v1alpha1/clusters/default/instances` lists the instances the ServiceAccount may
  read

#### Scenario: Port-forward from another local port

- **WHEN** a user forwards local port 8091 to the Pod's port 8090 and requests
  `http://127.0.0.1:8091/launch?token=...`
- **THEN** the response is `403`, the token is not spent, and no cookie is set

#### Scenario: The token is spent

- **WHEN** a second browser opens the launch URL after the first one did
- **THEN** the response is `403`
- **AND** after `kubectl -n opm-portal rollout restart deploy/opm-portal` the new Pod's log holds
  a new launch URL that admits a browser

#### Scenario: An identity the cluster does not name

- **WHEN** the in-cluster SelfSubjectReview fails or names no user
- **THEN** the container exits 1 before it listens, and the Pod restarts without ever printing a
  launch URL
