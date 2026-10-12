## MODIFIED Requirements

### Requirement: The role only reads, and never Secrets

The ClusterRole `opm-portal-reader` SHALL list every rule explicitly and SHALL grant only the
verbs `get`, `list` and `watch`: on the four OPM kinds of `opmodel.dev`; on `events.k8s.io`
`events` only the verb the portal uses, `list`; on `pods` and `get` on `pods/log`; on `apps`
`replicasets` and `batch` `jobs`; on `discovery.k8s.io` `endpointslices` only `list`; and on the non-Secret kinds the OPM catalog's transformers
render, a list maintained by hand against catalog 4.6.0's transformers. It SHALL NOT grant any
other verb, `secrets`, `impersonate`, a `*` in any group, resource or verb, any subresource but
`pods/log` (no `exec`, `attach`, `portforward` or `proxy`), a non-resource URL, or an aggregation
rule, and its one binding SHALL name only the ServiceAccount `opm-portal/opm-portal`, never a
group or a built-in role such as `view`. A Go test SHALL fail on any of these, and SHALL show it
does by refusing a table of denied roles and bindings. An inventory object of a kind the role
does not grant SHALL show as not readable, never omitted. An automated check that the role
covers every kind the pinned catalog renders stays with the in-cluster plan. Source:
portal:D13:R3.

#### Scenario: The shipped role

- **WHEN** the manifest test reads `deploy/clusterrole.yaml`
- **THEN** every verb is `get`, `list` or `watch`, and no rule names `secrets`, `impersonate` or
  `*`

#### Scenario: A denied role

- **WHEN** the manifest test checks a role granting `create` on `pods`, `get` on `secrets`,
  `impersonate` on `users`, or `*` as a resource
- **THEN** each check returns an error naming the offending rule

#### Scenario: A provider kind

- **WHEN** an instance's inventory names a cert-manager Certificate and the portal runs in the Pod
- **THEN** the object shows as not readable by the portal, and the rest of the instance renders

#### Scenario: The EndpointSlices rule

- **WHEN** the manifest test reads `deploy/clusterrole.yaml`
- **THEN** it finds one rule for `discovery.k8s.io` `endpointslices`, and the rule's verbs are
  exactly `list`

#### Scenario: A wider EndpointSlices rule

- **WHEN** the manifest test reads a role whose `endpointslices` rule also grants `get` or `watch`
- **THEN** the test fails and names the rule

#### Scenario: A write on EndpointSlices

- **WHEN** the manifest test checks a role granting `create` on `endpointslices`
- **THEN** the check returns an error naming the verb
