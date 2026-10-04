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
	colUpstream = iota // module, or source and dependencies
	colOwner
	colComponent
	colObject
	colRuntime
)

var (
	instanceTitles = []string{"Module", "Instance", "Components", "Objects", "Runtime", "Runtime"}
	packageTitles  = []string{"Source", "Package", "Components", "Objects", "Runtime", "Runtime"}
)

// podGroupThreshold is the most Pods under one parent shown one by one.
const podGroupThreshold = 5

// workloadKinds are the inventory kinds that make a component a workload
// component; every other component is configuration (0030:D4:R5).
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
// children below them.
func Package(d readmodel.PackageDetail, opts Options) Graph {
	b := newBuilder(ScopePackage, packageTitles, opts)
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
	if src := d.Source; src.Kind != "" && src.Name != "" {
		group, version := splitAPIVersion(src.APIVersion)
		// The operator reads the source from the package's namespace when
		// sourceRef names none (opm-operator modulepackage_controller.go).
		srcNS := src.Namespace
		if srcNS == "" {
			srcNS = ns
		}
		n := Node{
			ID:    sourceID(group, src.Kind, srcNS, src.Name),
			Kind:  KindSource,
			Label: src.Kind + " " + src.Name,
			Ref:   &Ref{Group: group, Version: version, Kind: src.Kind, Namespace: srcNS, Name: src.Name},
		}
		b.add(n, colUpstream)
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
		b.add(n, colUpstream)
		b.edge(EdgeDependsOn, root.ID, n.ID)
	}
	b.inventory(root.ID, d.Components)
	return b.finish()
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
// children below them. Configuration components are grouped into one node
// when there are two or more, unless the group is expanded.
func (b *builder) inventory(owner string, components []readmodel.Component) {
	group := groupID(GroupConfiguration, owner)
	config := 0
	for i := range components {
		if isConfiguration(&components[i]) {
			config++
		}
	}
	grouped := config >= 2 && !b.expanded(group)
	var groupNode *Node
	for i := range components {
		c := &components[i]
		isConfig := isConfiguration(c)
		if grouped && isConfig {
			groupNode = addToGroup(groupNode, group, c)
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
		if isConfig && config >= 2 {
			comp.MemberOf = group
		}
		b.add(comp, colComponent)
		b.edge(EdgeHasComponent, owner, comp.ID)
		for j := range c.Objects {
			b.object(comp.ID, &c.Objects[j])
		}
	}
	if groupNode != nil {
		groupNode.Label = plural(len(groupNode.Group.Members), "configuration component", "configuration components")
		b.add(*groupNode, colComponent)
		b.edge(EdgeHasComponent, owner, groupNode.ID)
	}
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

// addToGroup folds a component into the configuration group: worst state,
// partial and not live when any member is, counts summed.
func addToGroup(g *Node, id string, c *readmodel.Component) *Node {
	if g == nil {
		g = &Node{
			ID:     id,
			Kind:   KindGroup,
			Health: &Health{State: health.Healthy, Counts: &Counts{}},
			Group:  &Group{Kind: GroupConfiguration},
		}
	}
	h := summaryHealth(c.Health)
	g.Health.State = health.Worst(g.Health.State, h.State)
	g.Health.Partial = g.Health.Partial || h.Partial
	g.Health.NotLive = g.Health.NotLive || h.NotLive
	g.Health.Counts.add(h.Counts)
	g.Group.Members = append(g.Group.Members, c.Name)
	return g
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
	}
	n.HiddenScaledDown = b.children(o.Children)
	b.add(n, colObject)
	b.edge(EdgeOwns, comp, n.ID)
}

// children adds the runtime children below an inventory object, each below
// its controller, and returns how many scaled-down ReplicaSets it hid.
func (b *builder) children(children []readmodel.RuntimeChild) int {
	tree := newChildTree(children)
	shown, hiddenScaledDown := tree.visible(b.opts.ShowScaledDown)
	podsUnder := map[string][]*readmodel.RuntimeChild{}
	for _, c := range shown {
		if c.Ref.Kind == "Pod" {
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
		if owner := objectID(c.Owner); c.Ref.Kind == "Pod" && len(podsUnder[owner]) > podGroupThreshold {
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
	return c.Ref.Kind == "ReplicaSet" && c.Replicas != nil && *c.Replicas == 0
}

// splitAPIVersion splits group/version; a core apiVersion has no group.
func splitAPIVersion(apiVersion string) (group, version string) {
	if g, v, ok := strings.Cut(apiVersion, "/"); ok {
		return g, v
	}
	return "", apiVersion
}
