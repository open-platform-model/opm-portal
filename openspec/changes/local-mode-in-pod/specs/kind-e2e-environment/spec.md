## ADDED Requirements

### Requirement: The portal image is checked in a Pod on the fixture cluster

`task e2e:pod` SHALL build the portal image from the repository's `Dockerfile` with the e2e
provider's engine, SHALL load it into the fixture cluster without a registry, SHALL apply the
overlay under `test/e2e/pod/`, which uses `deploy/` unchanged apart from the image name and
`imagePullPolicy: Never`, and SHALL wait for the Deployment to roll out. It SHALL then run
`TestPod` (build tag `e2e`), which SHALL fail unless: the Pod log names the ServiceAccount
identity and holds a launch URL; with `kubectl port-forward` from local port 8090 to the Pod's
port 8090, a launch with the token from the log answers `200` with a session and the instance
list names the same instances the fixture cluster holds; and a forward from local port 8091 to
port 8090 is refused with `403`. It SHALL touch only the fixture cluster, through the kubeconfig
and context the other e2e tasks use. Source: portal:D13:R1/R2.

#### Scenario: The image runs in the fixture cluster

- **WHEN** a developer runs `task e2e:up` and then `task e2e:pod`
- **THEN** the `opm-portal` Deployment in namespace `opm-portal` is available, and `TestPod`
  passes

#### Scenario: A manifest that does not run

- **WHEN** a change makes the Pod fail to start or the role lose `list` on `moduleinstances`
- **THEN** `task e2e:pod` fails, at the rollout wait or in `TestPod`'s instance list

## MODIFIED Requirements

### Requirement: The environment runs on a schedule, never on pull requests

The repository SHALL run `task e2e:up` and `task e2e:capture` on docker kind in a workflow
triggered by manual dispatch and a nightly schedule only, SHALL upload the capture as a workflow
artifact, and SHALL delete the cluster whether or not the earlier steps succeeded. It SHALL fail
when the capture's `meta.yaml`, apart from the capture time, the provider and the cluster name,
differs from the committed one. On the same cluster it SHALL run `task e2e:local`, `task e2e:m1`
and then `task e2e:pod` whenever the cluster was created, even after a verdict difference or an
earlier test failure, and in a parallel job it SHALL run `task test:browser` with docker and the
Playwright image pinned by digest. When a test fails it SHALL upload the test output, a dump of
the fixture cluster's OPM objects, Pods and events that holds no Secret, and the browser
screenshots. The workflow SHALL hold only the `contents: read` permission and SHALL use no
secrets. No pull request check SHALL depend on this workflow.

#### Scenario: Nightly run

- **WHEN** the nightly schedule fires
- **THEN** the workflow creates the cluster, captures it, uploads the capture and deletes the
  cluster

#### Scenario: Verdict drift

- **WHEN** a nightly capture records a different `Ready` reason, claim verdict, operator or
  catalog version than the committed `meta.yaml`
- **THEN** the workflow run fails and its summary shows the difference

#### Scenario: Pull request

- **WHEN** a pull request is opened
- **THEN** the e2e workflow does not run

#### Scenario: A launch regression

- **WHEN** a change to `internal/auth` makes a browser land on the launch without its session
- **THEN** the next nightly run fails in the `Browser` job and uploads the browser screenshots

#### Scenario: A failing local-mode test

- **WHEN** `task e2e:local` fails in the nightly run
- **THEN** the run still runs `task e2e:m1` and `task e2e:pod`, deletes the cluster, and uploads
  the test output and the cluster dump
