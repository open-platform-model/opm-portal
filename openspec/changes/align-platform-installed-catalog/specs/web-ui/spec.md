## MODIFIED Requirements

### Requirement: The Platform page is the landing page

`/` SHALL show the Platform: an identity card (name, type, and the controller version labelled
"Controller"; the context, reader and Kubernetes version are in the header); a status block with
the applied state of `Ready` as a large state word in its tone, the time since that state
("since", from the deciding condition), the line of resolved catalogs, subscriptions and active
claims, and the reasons that need attention, each a count and a reason linking to its cause: per
refusal reason, the number of registrations refused with it, linking to the Providers tab filtered
to refused; per removal-blocked reason, the same, linking to it filtered to removal blocked; the
`Ready` reason (`BuildFailed`, `GenerateFailed`, `ContractCollisions`, `OverSubscribedContracts`,
`ComparablePredicates`) when `Ready` is not `True`, linking to its condition with the meaning,
next step and message; and `ContractsFulfilled`'s reason when it is `False`, as information
without a count, linking to the contracts note; an Installed card with one column of count rows
per health state and one per applied state, each row a badge and its count linking to Installed
filtered by it, and the numbers of instances, packages and of those that are providers; Providers
and Catalogs tabs with filters; and the recent events of the Platform and of each readable
registration, merged newest first, as a table with a resource filter whose options carry their
counts. A Providers row SHALL show the registration's name linking to its holder's Provider tab
when it has one, its catalog and version linking to the Catalog page, its provided contracts, its
holder ("Installed as") with kind and link, its acceptance and its activation as two separate
badges, and its refusal or removal-blocked reason and message. A removal-blocked registration
SHALL show as removal blocked while still accepted and active, never as refused. A Catalogs row
SHALL show the path linking to the Catalog page, version, source ("subscription", "contributed" or
"claimed only"), claimants linking to their holders' Provider tabs, and enablement as a badge. An
events row SHALL show type, reason, the object it is about as a link that filters the feed to that
object, the note with the repeat count and reporting controller, and the age. A Platform reporting
unfulfilled provider contracts SHALL show it as information, not as a failure. The page SHALL show
no Platform health and no platform graph. When the registrations the caller may read are not all
of them, or a list behind the Installed counts is forbidden, the page SHALL say so with the locked
style, whatever other reasons the status block shows, and SHALL never show a refusal count or an
active-claims count of zero for registrations it could not read. Source: portal:D4:R4/R7,
portal:D3:R8, portal:D17, portal:D9:R2/R4, portal:D19:R3/R5.

#### Scenario: The F1 platform

- **WHEN** a signed-in browser opens `/` on the F1 capture
- **THEN** its Catalogs tab shows the `opmodel.dev/catalogs/opm` subscription with its version
  and the backup catalog contributed by the accepted, active `backup-provider` claim, and its
  Providers tab shows the refused claim with its verdict Refused, its reason and its message

#### Scenario: Accepted and active apart

- **WHEN** a registration is accepted and not active
- **THEN** its row shows an "Accepted" badge and an "Inactive" badge, each on its own

#### Scenario: A count hands off

- **WHEN** the user follows the Degraded count on the Installed card
- **THEN** the browser opens `/installed?health=Degraded`

#### Scenario: Instances forbidden

- **WHEN** the caller may not list ModuleInstances
- **THEN** the Installed card shows the instance counts locked, not zero

#### Scenario: A refusal on the status block

- **WHEN** a signed-in browser opens `/` on the F1 capture
- **THEN** the status block reads "Applied" with a "since" time, and shows the count 1 with the
  reason `CatalogUnresolved` linking to `/?tab=providers&pstatus=refused#parts`

#### Scenario: Registrations forbidden on the status block

- **WHEN** the caller may get the Platform but may not list TransformerRegistrations
- **THEN** the status block says the refusals are locked and shows no refusal count, and its
  summary shows no "0 claims"

#### Scenario: Registrations forbidden while Ready is not True

- **WHEN** the caller may get the Platform but may not list TransformerRegistrations, and the
  Platform's `Ready` is `False`
- **THEN** the status block shows the `Ready` reason and, in the locked style, that the refusals
  are locked

#### Scenario: Removal blocked keeps acceptance and activation

- **WHEN** a registration's verdict is `RemovalBlocked`
- **THEN** its row shows "Accepted", "Active" and "Removal blocked" badges with its reason and
  message, and no "Refused" badge

#### Scenario: An event's resource filters the feed

- **WHEN** the user follows the resource link of an event about the TransformerRegistration
  `default.refused-claim-fixture`
- **THEN** the browser opens `/` with `eresource=registration:default.refused-claim-fixture` and
  the table holds only that registration's events

### Requirement: Installed lists instances and packages together

`/installed` SHALL list the ModuleInstances and ModulePackages the caller may read in one table:
name and namespace, a kind chip with its icon (instance and package drawn apart), a provider badge
reading "Provider" when the item holds a TransformerRegistration, coloured by its standing and
opening, on hover and on keyboard focus, a tooltip that names the registration and its standing in
words, the module path with its version on a line below, or the package's source as
`<Kind> <namespace>/<name>` with its source revision's short digest on a line below or "no
revision recorded" when the controller recorded none, and its path only when it is neither empty
nor `.`, the applied badge, the health badge, the object count and the owner (`controller` or
`cli` for an instance, `controller` for a package). Each row SHALL be a link to the item's page,
and a row whose health is Degraded SHALL be tinted apart from the others. When filters match
nothing, the list SHALL say so and offer a button that clears them. A kind the caller may not list
SHALL render locked for that kind only, with no count, while the other kind's rows show.
`/instances` and `/packages` SHALL redirect with `308` to `/installed` with `kind=instance` or
`kind=package`, keeping a `namespace` parameter. Source: portal:D17:R1, portal:D7:R2,
portal:D15:R2/R4.

#### Scenario: The F1 Installed list

- **WHEN** a signed-in browser opens `/installed` on the F1 capture
- **THEN** it lists the instances `cert-manager/cert-manager`, `default/podinfo`,
  `default/backup-provider`, `default/backup-consumer` and `web/web` and the package
  `pkg/podinfo`, with `web/web` owned by `cli` and `default/backup-provider` carrying an accepted,
  active provider badge

#### Scenario: Packages forbidden

- **WHEN** the caller may list ModuleInstances but not ModulePackages
- **THEN** the instances are listed and the packages show as a locked group with no count

#### Scenario: An old link

- **WHEN** a browser requests `/instances?namespace=team-a`
- **THEN** the response is `308` to `/installed?kind=instance&namespace=team-a`

#### Scenario: The provider tooltip by keyboard

- **WHEN** a keyboard user tabs to the provider badge of `default/backup-provider`
- **THEN** a tooltip shows that registration `default.backup-provider` is accepted and active,
  and the standing is also in the badge's text for assistive technology

#### Scenario: A package with no recorded revision

- **WHEN** the Installed list shows the F1 package `pkg/podinfo`
- **THEN** its source cell reads its source kind with `pkg/` and the source name on one line and
  "no revision recorded" on the next, and shows no "path ."

#### Scenario: A whole row opens the item

- **WHEN** the user clicks the Owner cell of the `default/podinfo` row
- **THEN** the browser opens `/instances/default/podinfo`

### Requirement: The Catalog page shows one catalog as the cluster records it

`/catalog?path=<catalog>` SHALL show any catalog the Platform subscribes to, holds in its resolved
registry, or a readable registration claims: as its headline, the catalog's short name (its last
path segment without the major version, or a longer form when two catalogs share it), with the
full path and version on a line below; its origin (subscribed, from a provider, or claimed only)
as a pill; its source ("Platform subscription", or the contributing registration, or for a
claimed-only catalog the claiming registrations, each linking to its holder's Provider tab when it
has one holder); its enablement, reading "no: not in the registry" for a catalog the registry does
not hold; whether it is resolved, as a state block with the state word, a summary of what the
registry records, and for an unresolved catalog the time since it was refused (the earliest time
a claiming registration's `Ready` condition turned `False`, or the Platform's `Ready` for a
subscription) and each reason (the Platform's `Ready` reason without a count, or the claiming
registrations' refusal reasons with their count)
linking to the Events tab filtered by that reason; the Platform's `ContractsFulfilled` labelled as
platform-wide; the registrations that claim it with their holders and standing; and the recent
events of the Platform and its claimants as a table, filterable by type and reason, with how many
are shown of how many. The breadcrumb SHALL end with the short name. It SHALL show no definition,
description, documentation link or transformer, and SHALL say these are not recorded. When the
caller may not read the Platform, the page SHALL render locked with status `403`, saying nothing
about whether the catalog exists. A path that a readable Platform and the readable registrations
do not name SHALL render not found with status `404`. Source: portal:D17:R3, portal:D3:R8,
portal:D7:R1, portal:D9:R2, portal:D19:R5.

#### Scenario: The contributed backup catalog

- **WHEN** a browser opens `/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0` on the
  F1 capture
- **THEN** it shows the headline `backup`, version `0.1.0`, origin from a provider, the source
  `default.backup-provider` linking to the Provider tab of `default/backup-provider`, and resolved

#### Scenario: A claimed catalog that does not resolve

- **WHEN** a browser opens the Catalog page of the catalog `default.refused-claim-fixture` claims
- **THEN** it shows the catalog as claimed only and not resolved, with the claim's reason
  `CatalogUnresolved`

#### Scenario: An unknown catalog

- **WHEN** a browser opens `/catalog?path=example.com/nothing@v0`
- **THEN** the response is `404` with the not-found page

#### Scenario: Platform forbidden

- **WHEN** a caller who may not get the Platform opens
  `/catalog?path=testing.opmodel.dev/catalogs/operator/backup@v0` and
  `/catalog?path=example.com/nothing@v0`
- **THEN** both render the same locked page with status `403`

#### Scenario: A refusal reason opens the events

- **WHEN** the user follows the `CatalogUnresolved` reason on the Resolved block of the catalog
  `default.refused-claim-fixture` claims
- **THEN** the Events tab opens filtered to reason `CatalogUnresolved`, holding the claim's
  warning, with its source reading the claiming registration's name, its enablement "no: not in
  the registry", and a "refused since" time on the block

#### Scenario: Events filtered by type

- **WHEN** a browser opens the Events tab of a catalog with `type=Warning`
- **THEN** the table holds only Warning events and says how many it shows of all the events

#### Scenario: A Type choice keeps the reason

- **WHEN** a browser opens a catalog's Events tab with `reason=CatalogUnresolved` and then picks
  type Warning
- **THEN** the address carries both `reason=CatalogUnresolved` and `type=Warning`

#### Scenario: Claimants forbidden

- **WHEN** the caller may get the Platform but may not list TransformerRegistrations and opens a
  subscribed catalog
- **THEN** the claims render locked and the Resolved block shows the Platform's own resolution,
  with no refusal count

### Requirement: Pages use the words a platform team uses

Page text SHALL name kinds in the words a platform team uses: "Providers" for
TransformerRegistrations and the instances and packages that hold them, "Installed" for
ModuleInstances and ModulePackages, and "controller" for the opm-operator. The read API's
`operatorVersion` SHALL be labelled "Controller", and its owner value `operator` SHALL read
"controller". The Installed card SHALL count the items that hold a registration as providers. The
YAML view shows objects as the cluster serves them and is exempt. Source: portal:D17:R2.

#### Scenario: The owner column

- **WHEN** the Installed list shows an instance whose owner is `operator` in the read API
- **THEN** its owner cell reads "controller"

#### Scenario: The providers line

- **WHEN** the Platform page's Installed card counts the F1 capture
- **THEN** its closing line says how many of the installed items are providers

## ADDED Requirements

### Requirement: Filter forms offer the values present and apply as the user types

This requirement covers the filter forms of the Installed list, the Platform's Providers and
Catalogs tabs, the Platform's recent events and the Catalog page's Events tab. Each SHALL draw its
fields in its view's parameter order. A filter whose values are
facts of the listed rows (Installed's namespace, uses and module; the Providers tab's provides)
SHALL be a select of the values present in the rows the caller may read. Every select option
SHALL carry its count over the rows before filtering, and the first option SHALL name the empty
choice in words ("Any kind", "All namespaces", "Anyone", "Any status", "Either"). An option whose
rows come from a list the caller may not read, or that failed, SHALL carry no count, never a zero.
A contract SHALL
show by its last two path segments with its version, with the full contract as its value. A
value in the URL that is not among the choices SHALL still apply and show as selected. When a
list behind a form is locked or failed, the counts SHALL cover only what the caller may read and
say so, and the namespace filter SHALL stay a field the user can type into, so a reader who may
list only some namespaces can name one. Each active filter's chip SHALL read "Label: value", and
the form SHALL always show how many rows it shows of how many, and "Clear all" when a filter is
active. Without script, the button SHALL show and every filter SHALL work as a plain `GET`. The URL
query SHALL stay the filters' source of truth. On every page, with script, a search or text field
of a `data-filters` form SHALL apply after a short pause in typing, with the user's focus, caret
and any text typed meanwhile kept, and that form's Apply button SHALL be hidden until it takes
keyboard focus. Source: portal:D14:R1/R2/R3/R7, portal:D7:R2, portal:D19.

#### Scenario: Namespace choices with counts

- **WHEN** a signed-in browser opens `/installed` on the F1 capture
- **THEN** the Namespace filter offers `cert-manager (1)`, `default (3)`, `pkg (1)` and
  `web (1)` after "All namespaces"

#### Scenario: A value not among the choices

- **WHEN** a browser opens `/installed?namespace=team-z` on the F1 capture
- **THEN** the Namespace select shows `team-z` selected, the list is empty, and its chip reads
  "Namespace: team-z"

#### Scenario: Typing applies the search

- **WHEN** a browser with script types `backup` into Installed's search field and stops typing
- **THEN** the page shows `/installed?q=backup` listing `default/backup-provider` and
  `default/backup-consumer`, with focus and caret still in the search field, and no Apply button
  is visible

#### Scenario: Without script

- **WHEN** a browser with script disabled submits Installed's filter form with search `backup`
- **THEN** the Apply button is visible and the page shows `/installed?q=backup`

#### Scenario: A namespace-only reader

- **WHEN** the caller may list ModuleInstances and ModulePackages only in namespace `default` and
  opens `/installed`
- **THEN** both lists render locked, the Namespace filter is a field the user can type into, and
  no option count stands for the locked lists

#### Scenario: Packages forbidden in the Kind filter

- **WHEN** the caller may list ModuleInstances but not ModulePackages and opens `/installed`
- **THEN** the Kind filter's Package option carries no count

#### Scenario: Registrations forbidden in the Platform filters

- **WHEN** the caller may get the Platform but may not list TransformerRegistrations and opens
  `/?tab=providers`, then `/?tab=catalogs`
- **THEN** the Status and Provides options carry no counts, and the Catalogs tab offers no Claimed
  filter

#### Scenario: Provider status choices

- **WHEN** a signed-in browser opens `/?tab=providers` on the F1 capture
- **THEN** the Status filter offers "Any status", then each status with its count, among them
  `Accepted (1)` and `Refused (1)`

### Requirement: Installed counts every namespace the caller may list

When the caller may list ModuleInstances and ModulePackages in every namespace, Installed SHALL
apply its namespace filter to the full lists, so its total, its filter counts and its namespace
choices cover every namespace. When a kind's full list is forbidden and a namespace filter is set,
Installed SHALL list that kind in the filtered namespace only, SHALL apply the namespace filter to
the other kind's rows before counting, so that the total, the filter counts and the choices all
cover that namespace only, and SHALL label its total as within that namespace. Source:
portal:D7:R2, portal:D14:R2/R7.

#### Scenario: The total under a namespace filter

- **WHEN** a signed-in browser opens `/installed?namespace=default` on the F1 capture
- **THEN** the list holds the three instances in `default` and says it shows 3 of 6

#### Scenario: A namespace-scoped reader with a namespace filter

- **WHEN** the caller may list ModuleInstances only in namespace `default` and opens
  `/installed?namespace=default`
- **THEN** the instances in `default` are listed and the total is labelled as within `default`

#### Scenario: One kind scoped

- **WHEN** the caller may list ModuleInstances in every namespace but ModulePackages only in
  `default`, and opens `/installed?namespace=default`
- **THEN** the page says "of N in default", where N counts only the instances and packages in
  `default`

### Requirement: The module filter matches a package by its source

Installed's module filter SHALL offer, beside the instances' module paths, each package's source
as `<Kind> <namespace>/<name>`, and SHALL match a package whose source is that value. Search SHALL
also match an item's kind word (`instance`, `package`) and a package's source. Source: portal:D17.

#### Scenario: A package by its source

- **WHEN** a browser chooses the F1 package's source in Installed's Module filter
- **THEN** the list holds `pkg/podinfo` only, and no note says packages are excluded

#### Scenario: Searching by kind

- **WHEN** a browser opens `/installed?q=package` on the F1 capture
- **THEN** the list holds `pkg/podinfo`
