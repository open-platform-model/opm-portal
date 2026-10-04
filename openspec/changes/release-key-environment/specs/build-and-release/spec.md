## ADDED Requirements

### Requirement: The release key is read only on main

The release workflow SHALL read `RELEASE_APP_PRIVATE_KEY` in exactly one job, `release-please`,
and that job SHALL run in the `release` environment, whose deployment branch policy admits `main`
only. The `release-please` job acts only through the App token and SHALL declare no
`GITHUB_TOKEN` permissions. Every workflow SHALL declare its token permissions explicitly. No job
that builds or publishes a release artifact (`binaries`, `image-release`) SHALL restore an
Actions cache: `actions/setup-go` runs with `cache: false` and the image build uses no `type=gha`
cache.

#### Scenario: A push to main

- **WHEN** a commit is pushed to `main`
- **THEN** the `release-please` job runs in the `release` environment and mints the App token
  from the key

#### Scenario: A workflow on another branch asks for the key

- **WHEN** a workflow run on a branch other than `main` names the `release` environment in a job
- **THEN** GitHub refuses to run that job, and no job outside the environment reads the key

#### Scenario: A release builds without a cache

- **WHEN** the `binaries` and `image-release` jobs build a release
- **THEN** neither restores an Actions cache, so a cache written by another run cannot reach the
  released binaries or image
