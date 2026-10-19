## REMOVED Requirements

### Requirement: The security page states what the portal reads and shows
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The security page states how local mode is closed to other readers", "The security page states that Secret data and spec.values are never read", "The security page states that local mode shows condition messages and event notes verbatim".

### Requirement: The local-mode how-to installs from a release
**Reason**: Split into requirements that each state one behavior.
**Migration**: See "The how-to installs opm-portal from the GitHub releases page and names no version or branch", "The how-to builds from source with go install at latest".

## ADDED Requirements

### Requirement: The security page states how local mode is closed to other readers
The security page SHALL state that local mode listens on loopback only, admits a browser only
through a one-time launch token exchanged for a session cookie, refuses a request whose `Host` is
not the loopback address and port, and checks each read with a SelfSubjectAccessReview before making
it, per portal:D5. Source: portal:D8:R4.

#### Scenario: Local mode is closed to other readers

- **WHEN** a reader looks up how local mode admits a browser
- **THEN** the security page SHALL state that local mode listens on loopback only, admits a browser only through a one-time launch token exchanged for a session cookie, refuses a request whose `Host` is not the loopback address and port, and checks each read with a SelfSubjectAccessReview before making it

### Requirement: The security page states that Secret data and spec.values are never read
The security page SHALL state that the portal never reads Secret data and serves no instance's or
package's `spec.values`, and that values a module renders into a non-Secret object stay readable to
anyone who may read that object, in the portal as in `kubectl`. Source: portal:D8:R4.

#### Scenario: A reader asks whether a password set in values is safe

- **WHEN** a reader looks up whether the portal can show a password they passed in values
- **THEN** the security page says the instance's values are never shown, and that a value the
  module rendered into a ConfigMap or a container's environment is readable to anyone who may
  read that object

### Requirement: The security page states that local mode shows condition messages and event notes verbatim
The security page SHALL state that local mode shows condition messages and event notes verbatim, as
the operator and the API server wrote them, and that the in-cluster mode, when built, will show only
the reasons of the text the operator writes, and the kubelet's and other controllers' text as
written. Source: portal:D8:R4.

#### Scenario: A reader looks up condition messages

- **WHEN** a reader looks up condition messages and event notes
- **THEN** the security page SHALL state that local mode shows them verbatim, as the operator and the API server wrote them

### Requirement: The how-to installs opm-portal from the GitHub releases page and names no version or branch
The how-to for running the portal locally SHALL install `opm-portal` from the portal's GitHub
releases page: the archive for the reader's system and its entry in `checksums.txt`, checked before
the binary is used. Its commands SHALL name no version or branch, so they never break and never
install unreleased code.

#### Scenario: A reader follows the how-to in an older release's bundle

- **WHEN** a reader follows the install step of the how-to in the bundle of any release
- **THEN** the step downloads a released archive and checks it against `checksums.txt`, and no
  command installs from `main`

### Requirement: The how-to builds from source with go install at latest
A note on building from source in the how-to for running the portal locally SHALL use `go install`
with `latest`, never a branch. The commands of the how-to for running the portal locally SHALL
install the newest release, which can be newer than the pages of an older release's bundle.

#### Scenario: A note on building from source

- **WHEN** the how-to carries a note on building from source
- **THEN** it SHALL use `go install` with `latest`, never a branch
