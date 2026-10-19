## REMOVED Requirements

### Requirement: The binary reports its version
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The binary prints its version and exits 0", "The binary runs local mode for serve and refuses any other argument".

### Requirement: Pull requests pass the Lint and Test checks
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "Every pull request runs a Lint check and a Test check", "Lint fails a release-please pull request with an unreleased OPM pin", "Lint fails a breaking API change unless the pull request title carries a bang", "A pull request title is a Conventional Commit with a lowercase subject".

### Requirement: A release is published only with all its assets
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "A release is a draft until its archives and its signed image exist", "A version image tag is never overwritten and no image is pushed outside a release".

### Requirement: The release key is read only on main
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The release key is read in one job that runs in the release environment", "Every workflow declares its token permissions and a release job does not persist a write token", "A job that builds or publishes a release artifact restores no Actions cache".

## ADDED Requirements

### Requirement: The binary prints its version and exits 0
The `opm-portal` binary SHALL print `opm-portal v<version>` on standard output and exit 0 when it is
run with no arguments, with `version`, or with `--version`. `<version>` SHALL be the version of the
release the source was tagged with, optionally followed by `+g<short-revision>` (and `.dirty` for a
modified tree) when the build carries VCS information. Reporting the version SHALL NOT open a
network listener or contact a cluster.

#### Scenario: Version with no arguments

- **WHEN** a user runs `opm-portal`
- **THEN** standard output is one line starting with `opm-portal v`
- **AND** the exit code is 0

#### Scenario: Version subcommand and flag

- **WHEN** a user runs `opm-portal version` or `opm-portal --version`
- **THEN** standard output is the same line as with no arguments
- **AND** the exit code is 0

### Requirement: The binary runs local mode for serve and refuses any other argument
For the `opm-portal` binary, `serve` SHALL run local mode (the `local-mode` capability). Any other
argument SHALL print a usage line on standard error and exit 2.

#### Scenario: Unknown argument

- **WHEN** a user runs `opm-portal start`
- **THEN** standard error names the accepted arguments
- **AND** standard output is empty
- **AND** the exit code is 2

### Requirement: Every pull request runs a Lint check and a Test check
Every pull request SHALL run a check named exactly `Lint` and a check named `Test`. `Lint` SHALL run
golangci-lint and validate every OpenSpec main spec and active change under `--strict`. The
repository SHALL have no other job named `Lint`.

#### Scenario: Ordinary PR

- **WHEN** a pull request from any other branch is opened
- **THEN** `Lint` runs golangci-lint, the OpenSpec validation and the API breaking-change gate,
  and skips the release-pin gate

### Requirement: Lint fails a release-please pull request with an unreleased OPM pin
On a release-please pull request (`release-please--*` head branch) `Lint` SHALL also fail when
`go.mod` carries a `replace` directive, or an OPM Go pin (`github.com/open-platform-model/*`) that
is a pseudo-version or not an existing tag of its repository.

#### Scenario: Release PR with an unreleased OPM pin

- **WHEN** a release-please pull request's `go.mod` requires an OPM Go module at a pseudo-version
- **THEN** the `Lint` check fails and names the pin

### Requirement: Lint fails a breaking API change unless the pull request title carries a bang
On every pull request `Lint` SHALL compare the read API's OpenAPI document with the base branch's
copy, and fail when the comparison reports a breaking change and the pull request title carries no
`!`.

#### Scenario: Breaking API change with a breaking title

- **WHEN** a pull request titled `feat(api)!: rename the health block` removes a response
  property from the OpenAPI document
- **THEN** the API gate reports the breaking change and `Lint` passes

#### Scenario: Base branch without the document

- **WHEN** the base branch holds no OpenAPI document yet
- **THEN** the API gate reports nothing to compare and passes

### Requirement: A pull request title is a Conventional Commit with a lowercase subject
A pull request title SHALL be a Conventional Commit with a lowercase subject.

#### Scenario: Non-conventional title

- **WHEN** a pull request is titled `Bootstrap the repo`
- **THEN** the PR-title check fails

### Requirement: A release is a draft until its archives and its signed image exist
A release SHALL be created as a draft. Before it is published it SHALL carry the archives
`opm-portal-linux-amd64.tar.gz`, `opm-portal-linux-arm64.tar.gz`, `opm-portal-darwin-amd64.tar.gz`,
`opm-portal-darwin-arm64.tar.gz` and `checksums.txt`, and the image
`ghcr.io/open-platform-model/opm-portal:vX.Y.Z` SHALL exist for linux/amd64 and linux/arm64, signed
with cosign and carrying an SPDX SBOM attestation and build provenance. The draft SHALL be published
only after both have succeeded.

#### Scenario: Image job fails

- **WHEN** the image job of a release run fails
- **THEN** the release stays a draft and is not published

#### Scenario: Image runs as nonroot

- **WHEN** the release image is started
- **THEN** the process runs as user 65532 on a distroless static base with no shell

### Requirement: A version image tag is never overwritten and no image is pushed outside a release
A version image tag SHALL never be overwritten with a different image. No image SHALL be pushed
outside a release.

#### Scenario: Version image tag already holds another commit

- **WHEN** `ghcr.io/open-platform-model/opm-portal:vX.Y.Z` exists and was built from a different
  commit than the release commit
- **THEN** the image job fails without pushing

### Requirement: The release key is read in one job that runs in the release environment
The release workflow SHALL read `RELEASE_APP_PRIVATE_KEY` in exactly one job, `release-please`, and
that job SHALL run in the `release` environment, whose deployment branch policy admits `main` only.
The `release-please` job acts only through the App token and SHALL declare no `GITHUB_TOKEN`
permissions; it SHALL mint that token with only `contents`, `pull-requests` and `issues` write.

#### Scenario: A push to main

- **WHEN** a commit is pushed to `main`
- **THEN** the `release-please` job runs in the `release` environment and mints the App token
  from the key

#### Scenario: A workflow on another branch asks for the key

- **WHEN** a workflow run on a branch other than `main` names the `release` environment in a job
- **THEN** GitHub refuses to run that job, and no job outside the environment reads the key

### Requirement: Every workflow declares its token permissions and a release job does not persist a write token
Every workflow SHALL declare its token permissions explicitly. A release job that holds a write
token SHALL check out without persisting it.

#### Scenario: A release job that holds a write token

- **WHEN** a release job holds a write token
- **THEN** it SHALL check out without persisting it

### Requirement: A job that builds or publishes a release artifact restores no Actions cache
No job that builds or publishes a release artifact (`binaries`, `image-release`) SHALL restore an
Actions cache: `actions/setup-go` runs with `cache: false` and the image build uses no `type=gha`
cache.

#### Scenario: A release builds without a cache

- **WHEN** the `binaries` and `image-release` jobs build a release
- **THEN** neither restores an Actions cache, so a cache written by another run cannot reach the
  released binaries or image
