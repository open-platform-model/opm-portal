# build-and-release Specification

## Purpose
Defines what the opm-portal binary reports about its own version, which checks guard a pull
request, and what a release of the repository produces and in which order.

## Requirements

### Requirement: The binary reports its version

The `opm-portal` binary SHALL print `opm-portal v<version>` on standard output and exit 0 when it
is run with no arguments, with `version`, or with `--version`. `<version>` SHALL be the version of
the release the source was tagged with, optionally followed by `+g<short-revision>` (and
`.dirty` for a modified tree) when the build carries VCS information. Any other argument SHALL
print a usage line on standard error and exit 2. The binary SHALL NOT open a network listener or
contact a cluster.

#### Scenario: Version with no arguments

- **WHEN** a user runs `opm-portal`
- **THEN** standard output is one line starting with `opm-portal v`
- **AND** the exit code is 0

#### Scenario: Version subcommand and flag

- **WHEN** a user runs `opm-portal version` or `opm-portal --version`
- **THEN** standard output is the same line as with no arguments
- **AND** the exit code is 0

#### Scenario: Unknown argument

- **WHEN** a user runs `opm-portal serve`
- **THEN** standard error names the accepted arguments
- **AND** standard output is empty
- **AND** the exit code is 2

### Requirement: Pull requests pass the Lint and Test checks

Every pull request SHALL run a check named exactly `Lint` and a check named `Test`. `Lint` SHALL
run golangci-lint and validate every OpenSpec main spec and active change under `--strict`. On a
release-please pull request (`release-please--*` head branch) `Lint` SHALL also fail when
`go.mod` carries a `replace` directive, or an OPM Go pin (`github.com/open-platform-model/*`)
that is a pseudo-version or not an existing tag of its repository. On every pull request `Lint`
SHALL compare the read API's OpenAPI document with the base branch's copy, and fail when the
comparison reports a breaking change and the pull request title carries no `!`. The repository
SHALL have no other job named `Lint`. A pull request title SHALL be a Conventional Commit with a
lowercase subject.

#### Scenario: Release PR with an unreleased OPM pin

- **WHEN** a release-please pull request's `go.mod` requires an OPM Go module at a pseudo-version
- **THEN** the `Lint` check fails and names the pin

#### Scenario: Ordinary PR

- **WHEN** a pull request from any other branch is opened
- **THEN** `Lint` runs golangci-lint, the OpenSpec validation and the API breaking-change gate,
  and skips the release-pin gate

#### Scenario: Non-conventional title

- **WHEN** a pull request is titled `Bootstrap the repo`
- **THEN** the PR-title check fails

#### Scenario: Breaking API change with a breaking title

- **WHEN** a pull request titled `feat(api)!: rename the health block` removes a response
  property from the OpenAPI document
- **THEN** the API gate reports the breaking change and `Lint` passes

#### Scenario: Base branch without the document

- **WHEN** the base branch holds no OpenAPI document yet
- **THEN** the API gate reports nothing to compare and passes

### Requirement: Releases follow the 0.x line

Releases SHALL be cut only by release-please, tagged `vX.Y.Z` with no component. The first
release SHALL be `0.1.0`, and the line SHALL stay below `1.0.0`: a breaking change bumps the
minor version. Commits typed `feat`, `fix`, `perf`, `revert`, `deps` and `refactor` SHALL
release; `docs`, `test`, `ci`, `build` and `chore` SHALL NOT.

#### Scenario: Docs-only merge

- **WHEN** only `docs:` and `chore:` commits have merged since the last release
- **THEN** release-please opens no release pull request

#### Scenario: Breaking change on the 0.x line

- **WHEN** a `feat!:` commit merges after release `0.3.1`
- **THEN** the next release is `0.4.0`

### Requirement: A release is published only with all its assets

A release SHALL be created as a draft. Before it is published it SHALL carry the archives
`opm-portal-linux-amd64.tar.gz`, `opm-portal-linux-arm64.tar.gz`,
`opm-portal-darwin-amd64.tar.gz`, `opm-portal-darwin-arm64.tar.gz` and `checksums.txt`, and the
image `ghcr.io/open-platform-model/opm-portal:vX.Y.Z` SHALL exist for linux/amd64 and
linux/arm64, signed with cosign and carrying an SPDX SBOM attestation and build provenance. The
draft SHALL be published only after both have succeeded. A version image tag SHALL never be
overwritten with a different image. No image SHALL be pushed outside a release.

#### Scenario: Image job fails

- **WHEN** the image job of a release run fails
- **THEN** the release stays a draft and is not published

#### Scenario: Version image tag already holds another commit

- **WHEN** `ghcr.io/open-platform-model/opm-portal:vX.Y.Z` exists and was built from a different
  commit than the release commit
- **THEN** the image job fails without pushing

#### Scenario: Image runs as nonroot

- **WHEN** the release image is started
- **THEN** the process runs as user 65532 on a distroless static base with no shell
