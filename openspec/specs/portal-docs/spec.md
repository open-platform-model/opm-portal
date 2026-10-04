# portal-docs Specification

## Purpose
Defines the portal's documentation bundle: what it holds and where the site places it, when it is
checked and published, how its docs-kit pins stay in agreement, and what the read API reference
and the security page must state.

## Requirements

### Requirement: The portal publishes one docs bundle placed in the site's docs tree

The repository SHALL declare exactly one docs-kit bundle, `opm-portal`, with placement kind
`docs` at root `/docs/`, versioned from the repository's `v<semver>` release tags. Its only source
SHALL be the authored pages under `docs/site/`. The bundle SHALL own `operating/portal/` and
`reference/portal/`, and SHALL write no page outside them, so it never claims a section index
another bundle owns.

#### Scenario: Building the bundle

- **WHEN** `task docs:bundle` runs on a clean checkout
- **THEN** opm-docs writes `out/opm-portal/` with every page under `docs/site/` and exits 0

#### Scenario: A page outside the owned paths

- **WHEN** a page is added at `docs/site/start/portal.md`
- **THEN** it is not under `operating/portal/` or `reference/portal/`, and review refuses it
  because the bundle would claim a section the site gives to another repository

### Requirement: The bundle is checked on every pull request and published from main

The `Docs` workflow SHALL run docs-kit's `publish.yml` in `check` mode on every pull request,
which builds and lints the bundle and publishes nothing; in `edge` mode on every push to `main`;
and in `release` or `revision` mode only on a manual dispatch from `main`. The `Release` workflow
SHALL publish the bundle of a release, in `release` mode with the release tag, whenever
release-please creates a release. Each calling job SHALL grant only the permissions its mode
needs.

#### Scenario: A pull request with a page that breaks the dialect

- **WHEN** a pull request adds a page with an `![image](x.png)` or a relative link
- **THEN** the `Docs` check fails, naming the page and the rule, and nothing is pushed

#### Scenario: A release

- **WHEN** release-please creates the release `v0.2.0`
- **THEN** the `Release` workflow publishes the bundle `0.2.0.0` to
  `ghcr.io/open-platform-model/docs/opm-portal`, signed by docs-kit's workflow

#### Scenario: A push to main

- **WHEN** a commit lands on `main`
- **THEN** the `Docs` workflow publishes the `edge` bundle of that commit

### Requirement: The docs-kit pins agree

`.opm-docs-version` SHALL name one docs-kit release, and every docs-kit `publish.yml` ref under
`.github/workflows/` SHALL name that same release. The `Lint` job SHALL refuse a disagreement,
without installing anything.

#### Scenario: A bump that moves only one pin

- **WHEN** a pull request changes `.opm-docs-version` to `v0.8.0` and leaves a `publish.yml@v0.7.0`
  ref in `docs.yml`
- **THEN** the `Lint` job fails and names the ref that disagrees

### Requirement: The read API reference lists every resource of the OpenAPI document

The read API reference page SHALL list every path of `openapi/v1alpha1.yaml`, and no other, and
SHALL link that document as the full contract. A test SHALL fail when a path is in one and not
the other.

#### Scenario: A route added without its reference entry

- **WHEN** a path is added to `openapi/v1alpha1.yaml` and not to the reference page
- **THEN** `task test` fails and names the missing path

#### Scenario: A reference entry with no route

- **WHEN** the reference page lists a path the OpenAPI document does not have
- **THEN** `task test` fails and names the extra path

### Requirement: The security page states what the portal reads and shows

The security page SHALL state that local mode listens on loopback only, admits a browser only
through a one-time launch token exchanged for a session cookie, refuses a request whose `Host` is
not the loopback address and port, and checks each read with a SelfSubjectAccessReview before
making it, per 0030:D5. It SHALL state that the portal never reads Secret data and serves no
instance's or package's `spec.values`, and that values a module renders into a non-Secret object
stay readable to anyone who may read that object, in the portal as in `kubectl`. Source:
0030:D8:R4. It SHALL state that condition messages and event notes are shown verbatim, as the
operator and the API server wrote them.

#### Scenario: A reader asks whether a password set in values is safe

- **WHEN** a reader looks up whether the portal can show a password they passed in values
- **THEN** the security page says the instance's values are never shown, and that a value the
  module rendered into a ConfigMap or a container's environment is readable to anyone who may
  read that object

### Requirement: Unreleased behavior is marked as unreleased

A page that describes behavior no released `opm-portal` has SHALL say so in an alert near its
top, naming what is missing, and SHALL be updated in the change that releases it.

#### Scenario: The local-mode how-to before the first release with `serve`

- **WHEN** the how-to for running the portal locally is published while no release has the
  `serve` command
- **THEN** the page opens with an alert saying the command is not in a release yet
