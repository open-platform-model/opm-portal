## MODIFIED Requirements

### Requirement: In-cluster pages show no controller text

Pages served over an `in-cluster` read API SHALL show no condition message (the conditions of a
registration included), reconcile message, history message, registration message or note of an
event the controller reported, because the read API does not serve them; they SHALL still show every
reason, state, history outcome, provider standing and the portal's meaning and next step, and
SHALL show the kubelet's and other controllers' event notes and the health messages of rendered
workloads as local mode does. Source: owner answer to portal:OQ8 (portal:D8:R5); its scope is the
supervisor ruling recorded under portal:D8.

#### Scenario: The broken podinfo page in-cluster

- **WHEN** the instance page of the image-break sample is rendered over an in-cluster read API
- **THEN** it shows the applied and health badges, each condition's reason, meaning and next
  step, and no message text the controller wrote

#### Scenario: A Pod's events in-cluster

- **WHEN** the events of a podinfo Pod are rendered over an in-cluster read API
- **THEN** the kubelet's and the scheduler's notes are shown as they wrote them

#### Scenario: Attempt dots and the Provider tab in-cluster

- **WHEN** backup-provider's page is rendered over an in-cluster read API
- **THEN** its attempt dots keep their outcomes, its registration conditions show type, status,
  reason and meaning, and no message text the controller wrote appears

### Requirement: The Catalog page shows one catalog as the cluster records it

`/catalog?path=<catalog>` SHALL show any catalog the Platform subscribes to, holds in its resolved
registry, or a readable registration claims: its path, version, origin (subscribed, from a
provider, or claimed only), source (the subscription, or the contributing registration and its
holder), enablement, whether it is resolved (with the Platform's `Ready` reason, or the claiming
registration's refusal reason), the Platform's `ContractsFulfilled` labelled as platform-wide, the
registrations that claim it with their holders and standing, and the recent events of the Platform
and its claimants. It SHALL show no definition, description, documentation link or transformer,
and SHALL say these are not recorded. When the caller may not read the Platform, the page SHALL
render locked with status `403`, saying nothing about whether the catalog exists. A path that a
readable Platform and the readable registrations do not name SHALL render not found with status
`404`. Source: portal:D17:R3, portal:D3:R8, portal:D7:R1.

#### Scenario: The contributed backup catalog

- **WHEN** a browser opens `/catalog?path=testing.opmodel.dev/catalogs/controller/backup@v0` on the
  F1 capture
- **THEN** it shows version `0.1.0`, origin from a provider, contributed by
  `default.backup-provider` held by `default/backup-provider`, and resolved

#### Scenario: A claimed catalog that does not resolve

- **WHEN** a browser opens the Catalog page of the catalog `default.refused-claim-fixture` claims
- **THEN** it shows the catalog as claimed only and not resolved, with the claim's reason
  `CatalogUnresolved`

#### Scenario: An unknown catalog

- **WHEN** a browser opens `/catalog?path=example.com/nothing@v0`
- **THEN** the response is `404` with the not-found page

#### Scenario: Platform forbidden

- **WHEN** a caller who may not get the Platform opens
  `/catalog?path=testing.opmodel.dev/catalogs/controller/backup@v0` and
  `/catalog?path=example.com/nothing@v0`
- **THEN** both render the same locked page with status `403`

### Requirement: Pages use the words a platform team uses

Page text SHALL name kinds in the words a platform team uses: "Providers" for
TransformerRegistrations and the instances and packages that hold them, "Installed" for
ModuleInstances and ModulePackages, and "controller" for opm-controller. The read API's
`controllerVersion` SHALL be labelled "Controller version", and its owner values `controller` and
`cli` SHALL read as they are. The YAML view shows objects as the cluster serves them and is exempt. Source:
portal:D17:R2.

#### Scenario: The owner column

- **WHEN** the Installed list shows an instance whose owner is `controller` in the read API
- **THEN** its owner cell reads "controller"

## RENAMED Requirements

- FROM: `### Requirement: In-cluster pages show no operator text`
- TO: `### Requirement: In-cluster pages show no controller text`
