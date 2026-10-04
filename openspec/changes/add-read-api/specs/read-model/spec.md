## ADDED Requirements

### Requirement: The read model reports which OPM object changed

The read model SHALL report, to a registered listener, the kind, namespace and name of every OPM
object (ModuleInstance, ModulePackage, Platform, TransformerRegistration) whose view may have
changed: when the object itself is added, updated or deleted, when an inventory object it owns
changes, and when a runtime child labelled with its name changes. A report SHALL carry no object
content, so a listener learns nothing it could serve without a grant. A deletion of the OPM
object itself SHALL be reported as a deletion.

#### Scenario: An inventory object changes

- **WHEN** Deployment `default/podinfo-podinfo`, labelled with instance `default/podinfo`'s uuid,
  is updated in a held informer
- **THEN** the listener is told that ModuleInstance `default/podinfo` changed

#### Scenario: The instance is deleted

- **WHEN** ModuleInstance `default/podinfo` is deleted
- **THEN** the listener is told it was deleted

### Requirement: Kinds resolve to resources for authorization

The read model SHALL resolve a group and kind to the resource it is served as, and whether it is
namespaced, through cached discovery, so a caller can authorize a read of that kind before it
looks anything up. An unknown kind SHALL be reported as unresolved, never guessed.

#### Scenario: A known kind

- **WHEN** the caller resolves group `apps`, kind `Deployment`
- **THEN** the result is resource `apps/v1 deployments`, namespaced

#### Scenario: An unknown kind

- **WHEN** the caller resolves group `example.com`, kind `Widget`, which discovery does not know
- **THEN** the result is an error
