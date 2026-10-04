## MODIFIED Requirements

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
