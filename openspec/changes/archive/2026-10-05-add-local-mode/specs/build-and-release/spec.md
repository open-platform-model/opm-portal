## MODIFIED Requirements

### Requirement: The binary reports its version

The `opm-portal` binary SHALL print `opm-portal v<version>` on standard output and exit 0 when it
is run with no arguments, with `version`, or with `--version`. `<version>` SHALL be the version of
the release the source was tagged with, optionally followed by `+g<short-revision>` (and
`.dirty` for a modified tree) when the build carries VCS information. `serve` SHALL run local
mode (the `local-mode` capability). Any other argument SHALL print a usage line on standard error
and exit 2. Reporting the version SHALL NOT open a network listener or contact a cluster.

#### Scenario: Version with no arguments

- **WHEN** a user runs `opm-portal`
- **THEN** standard output is one line starting with `opm-portal v`
- **AND** the exit code is 0

#### Scenario: Version subcommand and flag

- **WHEN** a user runs `opm-portal version` or `opm-portal --version`
- **THEN** standard output is the same line as with no arguments
- **AND** the exit code is 0

#### Scenario: Unknown argument

- **WHEN** a user runs `opm-portal start`
- **THEN** standard error names the accepted arguments
- **AND** standard output is empty
- **AND** the exit code is 2
