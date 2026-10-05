## MODIFIED Requirements

### Requirement: Graphs are server-rendered, accessible SVG

Each graph SHALL render server-side as SVG from the read API's laid-out graph document, with no
edge from an instance to a contract. Nodes SHALL carry CSS classes for kind, health and access, an
accessible name, and a title with their full label, and SHALL be reachable with the keyboard in
document order. A node SHALL show three lines: its kind, its name, and a status line (its health
reason or state, a ReplicaSet's desired count, a component's object count, a group's object and
kind counts, a registration's standing or a source's state). A node's fill and outline SHALL carry
its health only, and a separate square mark on the node SHALL carry its applied state; nodes the
cluster made below an inventory object SHALL be drawn dashed. Edges SHALL be drawn as orthogonal
elbows through the document's route points, without arrowheads. A label too long for its node
SHALL keep the part that tells nodes apart. Activating a node SHALL show its detail panel and mark
the node as selected; activating a group node SHALL show it expanded. A graph SHALL open fitted to
its frame, SHALL pan with the pointer and SHALL zoom with a zoom slider, Fit and Ctrl/Cmd-wheel.
Graphs SHALL appear on instance and package pages; the Platform page SHALL not draw one. The
instance page SHALL not draw the module node; the module's path and version stay on the page's
identity card. Source: portal:D4:R1/R3/R5, portal:D3:R1, portal:D17.

#### Scenario: Keyboard focus

- **WHEN** the user tabs into podinfo's graph
- **THEN** focus moves through the nodes in column order and each announces its kind, name and
  health

#### Scenario: Two axes on a node

- **WHEN** an instance node is applied and degraded
- **THEN** its fill and outline are drawn as degraded and its applied mark as applied

#### Scenario: A ReplicaSet's status line

- **WHEN** the image-break podinfo graph is rendered
- **THEN** the new ReplicaSet's node shows its kind, its name and its desired replica count, and is
  drawn dashed

#### Scenario: Elbow edges

- **WHEN** any instance graph is rendered
- **THEN** every edge path is made of horizontal and vertical segments and carries no arrowhead

### Requirement: Graphs can be explored without leaving the page

A graph SHALL show a card beside a node on pointer hover and on keyboard focus (kind, namespace and
name, health and applied marks, health reason and message, its origin, for a child of the cluster
the object it was created from, access, and a hint to click for details or to expand), positioned
without any `style` attribute. Activating a node SHALL select it through a `node` parameter, which
shows its panel and dims nothing. A `focus` parameter SHALL spotlight its node: select it, scroll
it into the centre of the view and dim every node and edge outside its whole upstream and
downstream chain; pointer hover and keyboard focus SHALL dim the same chain while they last. A
"Clear selection" control SHALL remove the selection and the spotlight. The graph SHALL open in
full screen and back, refitting to the space it has. It SHALL zoom with a zoom slider from 30 % to
200 %, Fit (marked pressed while the view is fitted) and Ctrl/Cmd-wheel anchored at the pointer,
and pan with the pointer; a fit SHALL use both the frame's width and height, and the frame's
height SHALL follow the fitted graph. A group node SHALL expand in place through the graph's
`expand` parameter and fit the view to the group's frame at no less than 60 %, and an "Overview"
control and the Escape key SHALL collapse it and fit the whole graph again. Escape SHALL step back
one thing per press: an open card, a selection, an open group, full screen. Motion SHALL stop under
the reduced-motion preference. Source: portal:D4:R5.

#### Scenario: cert-manager's RBAC group

- **WHEN** the user expands cert-manager's RBAC group
- **THEN** the view fits the group's frame, and "Overview" returns to the collapsed, fitted graph

#### Scenario: Keyboard card

- **WHEN** the user tabs onto podinfo's Deployment node
- **THEN** its card shows kind, name, health and origin, and is announced with the node

#### Scenario: A click does not dim

- **WHEN** the user clicks podinfo's Service node
- **THEN** the details panel shows the Service, the address carries `node` and no `focus`, and no
  node is dimmed

#### Scenario: Hand-off spotlight

- **WHEN** the user follows the image-break Pod's row from the Resources tab
- **THEN** the Graph tab opens with the Pod centred in the view and its ReplicaSet, Deployment,
  component and instance lit, and podinfo's Service dimmed

#### Scenario: Escape steps back

- **WHEN** the user has opened the RBAC group in full screen and selected one of its objects, and
  presses Escape three times
- **THEN** the first press clears the selection, the second collapses the group, and the third
  leaves full screen

#### Scenario: Wheel zoom at the pointer

- **WHEN** the user zooms with Ctrl and the wheel over a node
- **THEN** that node stays under the pointer

## ADDED Requirements

### Requirement: Configuration groups open one at a time inside a frame

A collapsed configuration group SHALL be drawn as a stacked, hatched node naming its family, its
component count and its object and kind counts. Opening one configuration group SHALL close any
other open configuration group, while open Pod groups stay open. An open configuration group SHALL
be drawn inside a dotted frame whose header names the family and counts its components and objects
and which carries a Collapse control; collapsing SHALL leave the group selected, with its panel
showing the object kinds it folds and their counts, its health and a control to expand it again.
The graph toolbar SHALL offer Expand all, Collapse all and a Group configuration toggle that is
pressed while no configuration group is open, and SHALL say how many boxes are drawn of how many
there are. Source: portal:D4:R5.

#### Scenario: Opening CRDs closes RBAC

- **WHEN** cert-manager's RBAC group is open and the user activates the CRDs group
- **THEN** the CRDs group is open inside its frame and the RBAC group is folded again

#### Scenario: Collapse from the frame

- **WHEN** the user activates the RBAC frame's Collapse control
- **THEN** the group is folded and selected, and the panel counts its ClusterRoles,
  ClusterRoleBindings, Roles and RoleBindings

#### Scenario: Expand all

- **WHEN** the user activates Expand all on cert-manager's graph
- **THEN** all three groups are open, each inside its own frame, and the Group configuration toggle
  is not pressed

### Requirement: The graph explains its marks and an empty inventory

Below every graph a legend SHALL say what the marks mean: applied by the controller, made by the
cluster, a configuration group and how it opens, that fill and outline carry health, and how to
zoom and step back. When an instance or package has applied nothing (no inventory and no last
applied time), the graph SHALL show a dashed placeholder box beside its root saying nothing was
applied yet and that the graph fills in once an apply succeeds, and, for a package with no
recorded source artifact, that it has never fetched its source. The placeholder SHALL not be a
node: it has no panel and no link.

#### Scenario: The F1 package

- **WHEN** a browser opens the Graph tab of `/packages/pkg/podinfo` on the F1 capture
- **THEN** a dashed box beside the package says nothing was applied yet and that the package has
  never fetched its source

#### Scenario: The legend

- **WHEN** a browser opens podinfo's Graph tab
- **THEN** the legend names the applied mark, the dashed cluster-made boxes, the group box, and the
  Ctrl + scroll and Esc keys

### Requirement: Registration and source nodes show their own state

A TransformerRegistration node on an instance or package graph SHALL be toned and labelled by the
registration's own standing (active, accepted but not active, refused with its reason, removal
blocked, pending), its card SHALL show that standing with the claimed catalog and the provider, and
its panel SHALL show the standing, the controller's message in local mode, the catalog with its
version linking to the Catalog page, and the provider and whether `spec.providerRef` names it. A
claim the caller may not read SHALL render locked. A package's source node SHALL be toned and
labelled by the source's state (ready with its revision, not ready with its reason, unknown, not
found, kind not installed, kind not read), and its panel SHALL show the Ready reason, message and
revision, and for a kind the cluster does not serve, that Flux source-controller is not installed
or the package names a kind the cluster lacks. Source: portal:D15:R2/R3/R4, portal:D8:R5, portal:D20.

#### Scenario: backup-provider's registration

- **WHEN** a browser opens backup-provider's Graph tab on the F1 capture
- **THEN** the `default.backup-provider` node reads "Accepted · active" in the healthy tone, and its
  panel names the catalog `testing.opmodel.dev/catalogs/operator/backup@v0` and version `0.1.0`

#### Scenario: Kind not installed

- **WHEN** a browser opens the Graph tab of `/packages/pkg/podinfo` on the F1 capture
- **THEN** the source node reads "Kind not installed", and its panel says the cluster does not serve
  the OCIRepository kind

#### Scenario: Locked claim

- **WHEN** a caller who may not list TransformerRegistrations opens backup-provider's Graph tab
- **THEN** the registration node shows no standing and its panel says the standing is locked

#### Scenario: In-cluster registration panel

- **WHEN** backup-provider's registration panel is rendered over an in-cluster read API
- **THEN** it shows the standing and reason and no message text the operator wrote

### Requirement: The details panel rests on the node that needs attention

On the Graph and Resources tabs, with neither `node` nor `focus` set, the details panel SHALL show
the resting node without a spotlight: the node of the first held TransformerRegistration the caller
may read whose claim is refused, pending, accepted but not active, or removal blocked, and
otherwise the instance's or package's own node. Clearing a selection SHALL return the panel to the
resting node. A panel other than the resting node's SHALL carry a Clear selection control.

#### Scenario: cert-manager at rest

- **WHEN** a browser opens cert-manager's Graph tab with no parameters
- **THEN** the details panel shows the ModuleInstance `cert-manager/cert-manager`

#### Scenario: A refused package claim

- **WHEN** a browser opens the Graph tab of a package whose held registration was refused with
  `ProviderMismatch`
- **THEN** the details panel shows that registration with its refusal reason, and no node is dimmed

#### Scenario: Clearing returns to rest

- **WHEN** the user selects podinfo's Service and then activates Clear selection
- **THEN** the panel shows the ModuleInstance `default/podinfo` again and the address carries no
  `node`
