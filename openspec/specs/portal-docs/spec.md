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
the other. The page's problem code table SHALL name exactly the `Code` constants of
`api/v1alpha1`, and its topic table SHALL hold a row, in a form the stream parses, for every topic
kind the stream serves; a test SHALL fail on either drift.

#### Scenario: A route added without its reference entry

- **WHEN** a path is added to `openapi/v1alpha1.yaml` and not to the reference page
- **THEN** `task test` fails and names the missing path

#### Scenario: A reference entry with no route

- **WHEN** the reference page lists a path the OpenAPI document does not have
- **THEN** `task test` fails and names the extra path

#### Scenario: A problem code without its reference entry

- **WHEN** a `Code` constant is added to `api/v1alpha1` and not to the reference page's problem
  code table, or the table names a code no constant has
- **THEN** `task test` fails and names the code

#### Scenario: A stream topic the reference page cannot have

- **WHEN** the reference page's topic table names a topic `stream.ParseTopic` refuses, or a topic
  kind the read API serves on its stream has no row
- **THEN** `task test` fails and names the topic or the kind

### Requirement: The security page states what the portal reads and shows

The security page SHALL state that local mode listens on loopback only, admits a browser only
through a one-time launch token exchanged for a session cookie, refuses a request whose `Host` is
not the loopback address and port, and checks each read with a SelfSubjectAccessReview before
making it, per 0030:D5. It SHALL state that the portal never reads Secret data and serves no
instance's or package's `spec.values`, and that values a module renders into a non-Secret object
stay readable to anyone who may read that object, in the portal as in `kubectl`. Source:
0030:D8:R4. It SHALL state that local mode shows condition messages and event notes verbatim, as
the operator and the API server wrote them, and that the in-cluster mode, when built, will show
their reasons only.

#### Scenario: A reader asks whether a password set in values is safe

- **WHEN** a reader looks up whether the portal can show a password they passed in values
- **THEN** the security page says the instance's values are never shown, and that a value the
  module rendered into a ConfigMap or a container's environment is readable to anyone who may
  read that object

### Requirement: Behavior that is not built is marked as not built

A page that mentions behavior `main` does not have SHALL say so in an alert near its top, naming
what is missing, and SHALL drop the alert in the change that builds it. A page SHALL NOT describe
a command or a mode that `main` does not have outside a direction note, and SHALL NOT word an
alert about releases, because a release's docs bundle is built from its tag and keeps the alert
after the next release.

#### Scenario: The local-mode how-to while the web UI is not built

- **WHEN** the how-to for running the portal locally is published while `main` has
  `opm-portal serve` and no web UI
- **THEN** the page opens with an alert saying the web UI is not built and the browser shows the
  read API's JSON, and it says nothing about releases

### Requirement: The local-mode how-to installs from a release

The how-to for running the portal locally SHALL install `opm-portal` from the portal's GitHub
releases page: the archive for the reader's system and its entry in `checksums.txt`, checked
before the binary is used. Its commands SHALL name no version or branch, so they never break and
never install unreleased code. A note on building from source SHALL use `go install` with
`latest`, never a branch. The commands install the newest release, which can be newer than the
pages of an older release's bundle.

#### Scenario: A reader follows the how-to in an older release's bundle

- **WHEN** a reader follows the install step of the how-to in the bundle of any release
- **THEN** the step downloads a released archive and checks it against `checksums.txt`, and no
  command installs from `main`
