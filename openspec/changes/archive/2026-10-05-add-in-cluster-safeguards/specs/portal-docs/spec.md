## MODIFIED Requirements

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
