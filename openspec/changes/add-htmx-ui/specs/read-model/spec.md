## ADDED Requirements

### Requirement: One inventory object is read on demand, stripped

The read model SHALL read one object by kind, namespace and name on demand for a caller whose
grant covers `get` on it, after the reader identity's own `get` review, and SHALL return it
without `metadata.managedFields`, the `kubectl.kubernetes.io/last-applied-configuration`
annotation, or, on a ModuleInstance or ModulePackage, `spec.values`. It SHALL keep the fields the
held copies drop for memory, such as a Deployment's pod template. It SHALL refuse a core Secret
before any review or read. Source: 0030:D8:R1/R2/R3.

#### Scenario: A Deployment with its template

- **WHEN** a caller allowed `get` on Deployments in `default` reads `apps` `Deployment`
  `default/podinfo-podinfo`
- **THEN** the object carries `spec.template` and no managed fields or last-applied annotation

#### Scenario: A Secret

- **WHEN** a caller reads a core `Secret`
- **THEN** the read is refused without a review or a read

### Requirement: Pod children name their containers

A runtime child that is a Pod SHALL carry the names of its init containers, then its
containers, in spec order.

#### Scenario: podinfo's Pods

- **WHEN** the caller reads instance `default/podinfo` from the F1 capture
- **THEN** each of its two Pods carries container `podinfo`
