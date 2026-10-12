package graph

import (
	"maps"
	"slices"
	"strings"

	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
)

// Columns of the instance and package scopes. A runtime child sits in the
// column after its inventory object's plus its depth below it, less one.
const (
	colUpstream = iota // module (instance graphs only)
	colOwner
	colComponent
	colObject
	colRuntime
)

// Column titles up to the objects; a runtime column is titled from the kinds
// it holds (layout.go runtimeTitle). A package graph has nothing in its
// first column: its source and dependencies sit in the components column.
var (
	instanceTitles = []string{"Module", "Instance", "Components", "Objects"}
	packageTitles  = []string{"", "Package", "Source and components", "Objects"}
)

// podGroupThreshold is the most Pods under one parent shown one by one.
const podGroupThreshold = 5

const (
	kindPod        = "Pod"
	kindReplicaSet = "ReplicaSet"
)

// workloadKinds are the inventory kinds that make a component a workload
// component; every other component is configuration (portal:D4:R5).
var workloadKinds = map[string]bool{
	"apps/Deployment":  true,
	"apps/StatefulSet": true,
	"apps/DaemonSet":   true,
	"apps/ReplicaSet":  true,
	"batch/Job":        true,
	"batch/CronJob":    true,
	"/Pod":             true,
}

// Instance returns the graph of one ModuleInstance: its module, its
// components, their inventory objects and the runtime children below them.
func Instance(d readmodel.InstanceDetail, opts Options) Graph {
	b := newBuilder(ScopeInstance, instanceTitles, opts)
	root := instanceNode(d.InstanceItem)
	root.RenderContracts = d.RenderContracts
	b.claims = claimsByName(d.ProviderOf)
	b.root = root.ID
	b.add(root, colOwner)
	if d.Module.Path != "" {
		mod := Node{
			ID:      moduleID(d.Module.Path, d.Module.Version),
			Kind:    KindModule,
			Label:   d.Module.Path,
			Version: d.Module.Version,
		}
		b.add(mod, colUpstream)
		b.edge(EdgeInstantiates, root.ID, mod.ID)
	}
	b.inventory(root.ID, d.Components)
	return b.finish()
}

// Package returns the graph of one ModulePackage: its source, the packages
// it depends on, its components, their inventory objects and the runtime
// children below them. src is the source as the caller read it, cut for
// serving, or nil when it was not looked up: the source node then carries
// no access and no state. The source's health is the node's own; it is not
// counted in the package's (portal:D3).
func Package(d readmodel.PackageDetail, src *readmodel.SourceState, opts Options) Graph {
	titles := slices.Clone(packageTitles)
	if d.Source.Kind == "" && len(d.DependsOn) == 0 {
		titles[colComponent] = "Components"
	}
	b := newBuilder(ScopePackage, titles, opts)
	b.claims = claimsByName(d.ProviderOf)
	ns := d.Ref.Namespace
	root := Node{
		ID:      packageID(ns, d.Ref.Name),
		Kind:    KindPackage,
		Label:   ns + "/" + d.Ref.Name,
		Ref:     refOf(d.Ref),
		Access:  health.AccessOK,
		Health:  summaryHealth(d.Health),
		Applied: appliedOf(d.Applied),
		Path:    d.Path,
	}
	b.root = root.ID
	b.add(root, colOwner)
	if ref := d.Source; ref.Kind != "" && ref.Name != "" {
		group, version := splitAPIVersion(ref.APIVersion)
		if readmodel.SourceSupported(ref.Kind) {
			// The controller chooses the group from the kind and ignores
			// sourceRef.apiVersion, so the node names the group it reads.
			group, version = readmodel.FluxGroup, "v1"
		}
		// The operator reads the source from the package's namespace when
		// sourceRef names none (opm-operator modulepackage_controller.go).
		srcNS := ref.Namespace
		if srcNS == "" {
			srcNS = ns
		}
		n := Node{
			ID:    sourceID(group, ref.Kind, srcNS, ref.Name),
			Kind:  KindSource,
			Label: ref.Kind + " " + ref.Name,
			Ref:   &Ref{Group: group, Version: version, Kind: ref.Kind, Namespace: srcNS, Name: ref.Name},
		}
		sourceNode(&n, src)
		b.add(n, colComponent)
		b.edge(EdgeSourcedFrom, root.ID, n.ID)
	}
	for _, dep := range d.DependsOn {
		depNS := dep.Namespace
		if depNS == "" {
			depNS = ns
		}
		ref := dep
		ref.Namespace = depNS
		n := Node{ID: packageID(depNS, dep.Name), Kind: KindPackage, Label: depNS + "/" + dep.Name, Ref: refOf(ref)}
		b.add(n, colComponent)
		e := b.edge(EdgeDependsOn, root.ID, n.ID)
		// The operator refuses a dependency in another namespace
		// (opm-operator modulepackage_types.go dependsOn), so the edge is
		// what the spec asks for, not a dependency it honors.
		if depNS != ns {
			unverified := false
			e.Verified, e.Reason = &unverified, ReasonForeignNamespace
		}
	}
	b.inventory(root.ID, d.Components)
	return b.finish()
}

// sourceNode sets the access, state and health of a source node from the
// state the caller read (nil: not looked up, so the node says nothing).
func sourceNode(n *Node, src *readmodel.SourceState) {
	if src == nil {
		return
	}
	n.Access = src.Access
	n.SourceState = &SourceState{Access: src.Access, State: src.Readiness, Reason: src.Reason, Message: src.Message, Revision: src.Revision}
	switch src.Readiness {
	case readmodel.SourceReady:
		n.Health = &Health{State: health.Healthy}
	case readmodel.SourceNotReady:
		n.Health = &Health{State: health.Degraded, Reason: src.Reason, Message: src.Message}
	case readmodel.SourceNoArtifact, readmodel.SourceUnknown:
		n.Health = &Health{State: health.Unknown}
	case readmodel.SourceNotFound, readmodel.SourceKindNotServed:
		n.Missing = true
	case readmodel.SourceUnsupported:
		// Not read: no health, and not missing.
	}
}

// instanceNode is an instance with both axes, as the read model filled
// them.
func instanceNode(it readmodel.InstanceItem) Node {
	return Node{
		ID:      instanceID(it.Ref.Namespace, it.Ref.Name),
		Kind:    KindInstance,
		Label:   it.Ref.Namespace + "/" + it.Ref.Name,
		Ref:     refOf(it.Ref),
		Access:  health.AccessOK,
		Health:  summaryHealth(it.Health),
		Applied: appliedOf(it.Applied),
		Owner:   string(it.Owner),
	}
}

// inventory adds an owner's components, their objects and the runtime
// children below them. Configuration components are sorted into kind
// families; a family whose components hold two or more objects is one group
// node, unless the group is expanded. A component that holds a
// TransformerRegistration is never folded.
func (b *builder) inventory(owner string, components []readmodel.Component) {
	families := make([]Family, len(components))
	objects := map[Family]int{}
	for i := range components {
		c := &components[i]
		if isConfiguration(c) && !holdsClaim(c) {
			families[i] = familyOf(c)
			objects[families[i]] += len(c.Objects)
		}
	}
	groups := map[Family]*Node{}
	for i := range components {
		c := &components[i]
		fam := families[i]
		group := familyGroupID(owner, fam)
		formed := fam != "" && objects[fam] >= 2
		if formed && !b.expanded(group) {
			groups[fam] = addToGroup(groups[fam], group, fam, c)
			continue
		}
		label := c.Name
		if label == "" {
			label = "(no component)"
		}
		comp := Node{
			ID:     componentID(owner, c.Name),
			Kind:   KindComponent,
			Label:  label,
			Health: summaryHealth(c.Health),
		}
		if formed {
			comp.MemberOf = group
		}
		b.add(comp, colComponent)
		b.edge(EdgeHasComponent, owner, comp.ID)
		for j := range c.Objects {
			b.object(comp.ID, &c.Objects[j])
		}
	}
	for _, fam := range Families {
		g := groups[fam]
		if g == nil {
			continue
		}
		// Sorted, so reordering status.inventory does not change the group.
		slices.Sort(g.Group.Members)
		g.Group.ObjectKinds = sortedKindCounts(g.Group.ObjectKinds)
		g.Label = fam.Label() + " · " + plural(len(g.Group.Members), "component", "components")
		b.add(*g, colComponent)
		b.edge(EdgeHasComponent, owner, g.ID)
	}
}

// familyGroupID is the id of the configuration group of one family under an
// owner.
func familyGroupID(owner string, fam Family) string {
	if fam == "" {
		return ""
	}
	return groupID(GroupConfiguration, owner) + "/" + part(string(fam))
}

// API groups that decide a configuration component's family.
const (
	groupCRDs     = "apiextensions.k8s.io"
	groupRBAC     = "rbac.authorization.k8s.io"
	groupWebhooks = "admissionregistration.k8s.io"
)

// familyOf is the kind family of a configuration component: the family all
// its objects are in by API group, else Other.
func familyOf(c *readmodel.Component) Family {
	if len(c.Objects) == 0 {
		return FamilyOther
	}
	var fam Family
	for i := range c.Objects {
		var f Family
		switch c.Objects[i].Ref.Group {
		case groupCRDs:
			f = FamilyCRDs
		case groupRBAC:
			f = FamilyRBAC
		case groupWebhooks:
			f = FamilyWebhooks
		default:
			return FamilyOther
		}
		if fam != "" && f != fam {
			return FamilyOther
		}
		fam = f
	}
	return fam
}

// holdsClaim reports whether a component holds a TransformerRegistration:
// the claim the provider views point at, so it is never folded.
func holdsClaim(c *readmodel.Component) bool {
	for i := range c.Objects {
		if r := c.Objects[i].Ref; r.Group == opmGroup && r.Kind == kindTransformer {
			return true
		}
	}
	return false
}

// isConfiguration reports whether a component holds no workload.
func isConfiguration(c *readmodel.Component) bool {
	for i := range c.Objects {
		r := c.Objects[i].Ref
		if workloadKinds[r.Group+"/"+r.Kind] {
			return false
		}
	}
	return true
}

// addToGroup folds a component into its family's group: worst state,
// partial and not live when any member is, counts summed, the objects it
// holds counted by kind.
func addToGroup(g *Node, id string, fam Family, c *readmodel.Component) *Node {
	if g == nil {
		g = &Node{
			ID:     id,
			Kind:   KindGroup,
			Health: &Health{State: health.Healthy, Counts: &Counts{}},
			Group:  &Group{Kind: GroupConfiguration, Family: fam},
		}
	}
	h := summaryHealth(c.Health)
	g.Health.State = health.Worst(g.Health.State, h.State)
	g.Health.Partial = g.Health.Partial || h.Partial
	g.Health.NotLive = g.Health.NotLive || h.NotLive
	g.Health.Counts.add(h.Counts)
	g.Group.Members = append(g.Group.Members, c.Name)
	for i := range c.Objects {
		g.Group.ObjectKinds = addKind(g.Group.ObjectKinds, c.Objects[i].Ref.Kind)
	}
	return g
}

func addKind(counts []KindCount, kind string) []KindCount {
	for i := range counts {
		if counts[i].Kind == kind {
			counts[i].Count++
			return counts
		}
	}
	return append(counts, KindCount{Kind: kind, Count: 1})
}

func sortedKindCounts(counts []KindCount) []KindCount {
	slices.SortFunc(counts, func(a, b KindCount) int { return compareStrings(a.Kind, b.Kind) })
	return counts
}

// object adds one inventory object below its component, and its runtime
// children below it.
func (b *builder) object(comp string, o *readmodel.InventoryObject) {
	n := Node{
		ID:             objectID(o.Ref),
		Kind:           KindObject,
		Label:          o.Ref.Name,
		Ref:            refOf(o.Ref),
		Access:         o.Access,
		ChildrenUnread: o.ChildrenUnread,
	}
	if o.Access == health.AccessOK {
		n.Health = objectHealth(o.Health)
		n.Missing = o.Health.State == health.Missing
		n.EndpointsReady = o.EndpointsReady
		n.EndpointsUnread = o.EndpointsUnread
	}
	if o.Ref.Group == opmGroup && o.Ref.Kind == kindTransformer {
		n.Registration = b.registration(o.Ref.Name)
	}
	n.HiddenScaledDown = b.children(o.Children)
	b.add(n, colObject)
	b.edge(EdgeOwns, comp, n.ID)
}

// claimsByName indexes an owner's provider claims by registration name.
func claimsByName(cs []readmodel.ProviderClaim) map[string]readmodel.ProviderClaim {
	if len(cs) == 0 {
		return nil
	}
	out := make(map[string]readmodel.ProviderClaim, len(cs))
	for i := range cs {
		out[cs[i].Registration] = cs[i]
	}
	return out
}

// registration is the standing the owner's claim for a registration the
// inventory holds, exactly as a registration node of the platform graph
// carries it. It is nil when the caller may not read the registration, or
// the cluster does not hold it: a claim not read says nothing.
func (b *builder) registration(name string) *Registration {
	c, ok := b.claims[name]
	if !ok || !c.Read() {
		return nil
	}
	return &Registration{
		Catalog:       c.Catalog,
		Version:       c.Version,
		Provides:      c.Provides,
		Accepted:      c.Standing.Accepted,
		Active:        c.Standing.Active,
		Verdict:       c.Standing.Verdict,
		Reason:        c.Standing.Reason,
		Message:       c.Standing.Message,
		ActiveReason:  c.Standing.ActiveReason,
		ActiveMessage: c.Standing.ActiveMessage,
	}
}

// children adds the runtime children below an inventory object, each below
// its controller, and returns how many scaled-down ReplicaSets it hid.
func (b *builder) children(children []readmodel.RuntimeChild) int {
	tree := newChildTree(children)
	shown, hiddenScaledDown := tree.visible(b.opts.ShowScaledDown)
	podsUnder := map[string][]*readmodel.RuntimeChild{}
	for _, c := range shown {
		if c.Ref.Kind == kindPod {
			owner := objectID(c.Owner)
			podsUnder[owner] = append(podsUnder[owner], c)
		}
	}
	grouped := b.podGroups(tree, podsUnder)
	for _, c := range shown {
		cid := objectID(c.Ref)
		if grouped[cid] {
			continue
		}
		n := Node{
			ID:       cid,
			Kind:     KindRuntime,
			Label:    c.Ref.Name,
			Ref:      refOf(c.Ref),
			Access:   health.AccessOK,
			Health:   objectHealth(c.Health),
			Replicas: c.Replicas,
		}
		if owner := objectID(c.Owner); c.Ref.Kind == kindPod && len(podsUnder[owner]) > podGroupThreshold {
			n.MemberOf = groupID(GroupPods, owner)
		}
		b.add(n, colObject+tree.depth(c))
		b.edge(EdgeControls, objectID(c.Owner), cid)
	}
	return hiddenScaledDown
}

// podGroups replaces the Pods of each parent holding more than
// podGroupThreshold with one group node, unless it is expanded, and
// returns the ids of the Pods it grouped.
func (b *builder) podGroups(tree childTree, podsUnder map[string][]*readmodel.RuntimeChild) map[string]bool {
	grouped := map[string]bool{}
	for _, owner := range slices.Sorted(maps.Keys(podsUnder)) {
		pods := podsUnder[owner]
		gid := groupID(GroupPods, owner)
		if len(pods) <= podGroupThreshold || b.expanded(gid) {
			continue
		}
		g := Node{
			ID:     gid,
			Kind:   KindGroup,
			Label:  plural(len(pods), "Pod", "Pods"),
			Health: &Health{State: health.Healthy, Counts: &Counts{}},
			Group:  &Group{Kind: GroupPods},
		}
		for _, p := range pods {
			g.Health.State = health.Worst(g.Health.State, p.Health.State)
			g.Health.Counts.addState(p.Health.State)
			g.Group.Members = append(g.Group.Members, p.Ref.Name)
			grouped[objectID(p.Ref)] = true
		}
		b.add(g, colObject+tree.depth(pods[0]))
		b.edge(EdgeControls, owner, gid)
	}
	return grouped
}

// childTree indexes the runtime children below one inventory object by id,
// so each child's controller can be found among them.
type childTree map[string]*readmodel.RuntimeChild

func newChildTree(children []readmodel.RuntimeChild) childTree {
	t := make(childTree, len(children))
	for i := range children {
		t[objectID(children[i].Ref)] = &children[i]
	}
	return t
}

// depth is how many controllers a child is below the inventory object: 1
// for a ReplicaSet below a Deployment, 2 for its Pods.
func (t childTree) depth(c *readmodel.RuntimeChild) int {
	d := 1
	for owner := t[objectID(c.Owner)]; owner != nil && d <= len(t); owner = t[objectID(owner.Owner)] {
		d++
	}
	return d
}

// visible returns the children to show, in their order, and how many
// ReplicaSets scaled to zero it hid. Anything below a hidden ReplicaSet is
// hidden with it.
func (t childTree) visible(showScaledDown bool) (shown []*readmodel.RuntimeChild, hiddenScaledDown int) {
	ids := slices.Sorted(maps.Keys(t))
	slices.SortStableFunc(ids, func(a, b string) int { return t.depth(t[a]) - t.depth(t[b]) })
	for _, id := range ids {
		c := t[id]
		if scaledDown(c) && !showScaledDown {
			hiddenScaledDown++
			continue
		}
		if !t.belowScaledDown(c) || showScaledDown {
			shown = append(shown, c)
		}
	}
	return shown, hiddenScaledDown
}

func (t childTree) belowScaledDown(c *readmodel.RuntimeChild) bool {
	owner := t[objectID(c.Owner)]
	for hops := 0; owner != nil && hops <= len(t); hops++ {
		if scaledDown(owner) {
			return true
		}
		owner = t[objectID(owner.Owner)]
	}
	return false
}

func scaledDown(c *readmodel.RuntimeChild) bool {
	return c.Ref.Kind == kindReplicaSet && c.Replicas != nil && *c.Replicas == 0
}

// splitAPIVersion splits group/version; a core apiVersion has no group.
func splitAPIVersion(apiVersion string) (group, version string) {
	if g, v, ok := strings.Cut(apiVersion, "/"); ok {
		return g, v
	}
	return "", apiVersion
}
