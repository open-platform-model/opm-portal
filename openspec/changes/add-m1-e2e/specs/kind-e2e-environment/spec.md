## ADDED Requirements

### Requirement: Milestone 1 reads are checked end to end on the fixture cluster

`task e2e:m1` SHALL run the built binary in local mode against the fixture cluster and SHALL
fail unless, through the read API and its stream: the Platform shows `default.backup-provider`
accepted and active and contributing the backup catalog, and `default.refused-claim-fixture`
refused with its reason and message; the instance list shows each F1 instance's applied state
and health as two values (the operator's instances Applied and Healthy, the CLI-owned `web/web`
ManagedExternally and Healthy); the `web/web` page reads "Managed externally"; and a
ServiceAccount that may read the OPM kinds only in `default`, run with `--namespaces default`,
lists `default`'s instances only, gets forbidden for every other list, object and the Platform,
sees podinfo's inventory objects as forbidden with a partial health and as locked rows on the
page, and has the cluster-wide list topic closed as forbidden on the stream. It SHALL patch
podinfo's image tag to one that does not exist and SHALL fail unless the stream reports podinfo
Degraded within 10 seconds of the cluster first reporting a Pod that cannot pull its image, with
its applied state Applied, and for 5 seconds afterwards never reports it Failed or Stalled or
other than Degraded. It SHALL revert the
patch and wait for podinfo to settle before it ends. Source: 0030:D3:R2/R3/R5, 0030:D4:R4,
0030:D5:R5/R7.

#### Scenario: Image break

- **WHEN** `task e2e:m1` patches podinfo's image tag to a missing tag
- **THEN** the `instance:default/podinfo` topic carries an Instance with health Degraded and
  applied state Applied within 10 seconds of the Pod's `ErrImagePull`
- **AND** after the test, podinfo's Deployment has rolled back and its Pods are running

#### Scenario: Namespace-scoped reader

- **WHEN** the reader that may read the OPM kinds only in `default` asks for the cluster-wide
  instance list, the `web/web` instance or the Platform
- **THEN** the list answers with access `forbidden` and no items, and the reads answer 403 with
  the code `forbidden`

#### Scenario: Health stops following the stream

- **WHEN** a change makes the portal stop delivering podinfo's Pod changes on the stream
- **THEN** `task e2e:m1` fails, either because no Degraded document arrives or because it
  arrives more than 10 seconds after the cluster reported the broken Pod

## MODIFIED Requirements

### Requirement: The environment runs on a schedule, never on pull requests

The repository SHALL run `task e2e:up` and `task e2e:capture` on docker kind in a workflow
triggered by manual dispatch and a nightly schedule only, SHALL upload the capture as a workflow
artifact, and SHALL delete the cluster whether or not the earlier steps succeeded. It SHALL fail
when the capture's `meta.yaml`, apart from the capture time, the provider and the cluster name,
differs from the committed one. On the same cluster it SHALL run `task e2e:local` and
`task e2e:m1` whenever the cluster was created, even after a verdict difference, and in a
parallel job it SHALL run `task test:browser` with docker and the Playwright image pinned by
digest. When a test fails it SHALL upload the test output, a dump of the fixture cluster's OPM
objects, Pods and events that holds no Secret, and the browser screenshots. The workflow SHALL
hold only the `contents: read` permission and SHALL use no secrets. No pull request check SHALL
depend on this workflow.

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
- **THEN** the run still runs `task e2e:m1`, deletes the cluster, and uploads the test output
  and the cluster dump
