package ui

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// An instance or package page: summary cards (identity, Applied, Health,
// Provider) above tabs (Graph, Resources, Events, Logs, YAML), each tab a
// server-rendered link (portal:D3:R1, portal:D9, portal:D15).

// Owner page tabs.
const (
	tabGraph     = "graph"
	tabResources = "resources"
	tabEvents    = "events"
	tabLogs      = "logs"
	tabYAML      = "yaml"
)

// ownerTab is one tab of an owner page's strip.
type ownerTab struct{ Name, Label string }

// ownerTabs is the strip in order; a page that holds a registration puts the
// Provider tab after Graph (see ownerTabsFor).
var ownerTabs = []ownerTab{
	{tabGraph, "Graph"}, {tabResources, "Resources"}, {tabEvents, "Events"}, {tabLogs, "Logs"}, {tabYAML, "YAML"},
}

// ownerTabsFor is the tabs a page offers, in order: Graph, Provider when the
// item holds a registration, Resources, Events, Logs and YAML.
func ownerTabsFor(provider bool) []ownerTab {
	if !provider {
		return ownerTabs
	}
	out := make([]ownerTab, 0, len(ownerTabs)+1)
	out = append(out, ownerTabs[0], ownerTab{tabProvider, "Provider"})
	return append(out, ownerTabs[1:]...)
}

// selectionParams are the parameters that name the object the page is on: node
// is the selection, focus the spotlight a Resources row or a Health reason
// hands off. Every tab link keeps both.
var selectionParams = []string{"node", "focus"}

// tabHref is the link to tab name on the page at base, keeping the
// selection of q (portal:D9).
func tabHref(base string, q url.Values, name string) string {
	out := url.Values{"tab": {name}}
	for _, k := range selectionParams {
		if v := q.Get(k); v != "" {
			out.Set(k, v)
		}
	}
	return base + "?" + out.Encode()
}

// Event types the Events tab filters by.
const (
	eventNormal  = "Normal"
	eventWarning = "Warning"
)

// tabLink is one tab. Count is shown after the label when set; nil means
// the page holds no count it may show (portal:D19:R3, portal:D7).
type tabLink struct {
	Name    string // the tab= value, for the count's id
	Label   string
	Href    string
	Current bool
	Count   *int
	// Follow is the topics the tab's count follows, so a live refresh swaps
	// it. A tab that can carry a count has it whether or not the page holds
	// one now: the count span is then empty, and a refresh can fill it or
	// empty it (portal:D19:R3). A tab that never carries a count has none.
	Follow string
}

// platformTopic is the live topic of the Platform and its registrations.
const platformTopic = "platform"

// setCount sets the count of the tab called name. The strip keeps its links;
// only the count is a followed region. A tab with no count set keeps an empty
// span, so a refresh that finds the source unreadable removes the old number.
func setCount(tabs []tabLink, name string, n int) {
	for i := range tabs {
		if tabs[i].Name == name {
			tabs[i].Count = &n
		}
	}
}

// ownerView is an instance or package page.
type ownerView struct {
	Kind      ownerKind
	Namespace string
	Name      string
	Base      string // the page's path
	Problem   *v1.Problem
	Topic     string

	Instance *v1.Instance
	Package  *v1.Package

	Reconcile   v1.Reconcile
	Health      v1.Health
	Conditions  []v1.Condition
	History     []v1.HistoryEntry
	LastApplied v1.Digests
	ProviderOf  []v1.ProviderClaim
	Components  []componentView
	// Configs are the components the graph folds into group nodes, one entry
	// for each group, listed folded the same way; nil when the graph folds
	// none.
	Configs []configGroup

	Tab  string
	Tabs []tabLink

	// Identity, Cards and plat are the summary row: the identity card's
	// facts, the Applied, Health and Provider blocks, and the Platform the
	// Provider card reads on every tab.
	Identity ownerIdentity
	Cards    ownerCards
	plat     platformRead

	Graph        svgGraph
	GraphProblem *v1.Problem
	// Focus is the node the graph spotlights, Node the one the page
	// selects; the panel shows Node, else Focus, else the resting node.
	Focus string
	Node  string
	Panel *nodePanel
	// PanelRoot is the URL of the panel fragment of the node the panel rests
	// on, which Clear selection brings back; PanelAtRest is true when the
	// panel shows that node because neither node nor focus is set.
	PanelRoot   string
	PanelAtRest bool

	// Resources is the Resources tab's table.
	Resources  resourcesTable
	Events     eventsView
	EventsForm eventsForm
	// EventsTable is the Events tab's rows after the filters.
	EventsTable eventsTable
	Logs        logsTab
	YAML        yamlTab
	Provider    providerTabView

	firstContainer func(string) string
}

// --- Resources ---

// reasonFilter is the Resources tab's one filter: a health reason, matched
// as given, shown as a removable chip and set by the Health card's hand-off.
var reasonFilter = filterView{Params: []filterParam{{Name: "reason", Label: "Health reason"}}}

// focusIDs maps every object the inventory reaches to the graph node that
// stands for it: its own node, or the group it is folded into.
func focusIDs(g *v1.Graph) (byRef, groupOf map[string]string) {
	byRef, groupOf = map[string]string{}, map[string]string{}
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Ref != nil {
			byRef[refKey(*n.Ref)] = n.ID
		}
		if n.Group != nil {
			for _, m := range n.Group.Members {
				groupOf[m] = n.ID
			}
		}
	}
	return byRef, groupOf
}

// objectNodeID is the graph's id of an inventory object's or child's node,
// as internal/graph writes it: obj:<group>/<Kind>/<namespace>/<name>, each
// part path-escaped with ":" and "@" escaped too, an empty part written
// as "_" and a literal "_" as "%5F". TestFoldedObjectsFocusTheirOwnNode
// holds the two to the same ids.
func objectNodeID(r v1.ObjectRef) string {
	part := func(s string) string {
		switch s {
		case "":
			return "_"
		case "_":
			return "%5F"
		}
		return strings.NewReplacer(":", "%3A", "@", "%40").Replace(url.PathEscape(s))
	}
	return "obj:" + part(r.Group) + "/" + part(r.Kind) + "/" + part(r.Namespace) + "/" + part(r.Name)
}

func refKey(r v1.ObjectRef) string { return r.Group + "/" + r.Kind + "/" + r.Namespace + "/" + r.Name }

// annotateResources links each object and child to the graph node that
// shows it, and keeps only those with reason, when one is asked for.
func annotateResources(cs []componentView, byRef, groupOf map[string]string, base, reason string) []componentView {
	// focus is the link to the node that shows r, and the id of the node the
	// page marks for it: its own node, or its place in the group it is
	// folded into.
	focus := func(r v1.ObjectRef, component string) (href, id string) {
		if id := byRef[refKey(r)]; id != "" {
			return base + "?" + url.Values{"tab": {tabGraph}, "focus": {id}}.Encode(), id
		}
		// Folded into a group: open the group and focus the object's own
		// node in it.
		group := groupOf[r.Name]
		if group == "" {
			group = groupOf[component]
		}
		if group == "" {
			return base + "?tab=" + tabGraph, ""
		}
		id = objectNodeID(r)
		return base + "?" + url.Values{"tab": {tabGraph}, "expand": {group}, "focus": {id}}.Encode(), id
	}
	out := make([]componentView, 0, len(cs))
	for i := range cs {
		c := cs[i]
		objs := make([]objectView, 0, len(c.Objects))
		for j := range c.Objects {
			if o, ok := annotateObject(c.Objects[j], c.Name, focus, reason); ok {
				objs = append(objs, o)
			}
		}
		if len(objs) == 0 && reason != "" {
			continue
		}
		c.Objects = objs
		out = append(out, c)
	}
	return out
}

// annotateObject links one object and its children to the graph and keeps
// the children with reason; ok is false when neither it nor a child has it.
func annotateObject(o objectView, component string, focus func(v1.ObjectRef, string) (string, string), reason string) (objectView, bool) {
	o.Focus, o.NodeID = focus(o.Ref, component)
	kids := make([]childView, 0, len(o.Children))
	for k := range o.Children {
		ch := o.Children[k]
		ch.Focus, ch.NodeID = focus(ch.Ref, component)
		if reason == "" || ch.Health.Reason == reason {
			kids = append(kids, ch)
		}
	}
	// The old revisions are linked the same way; they show only without a
	// reason filter.
	old := make([]childView, len(o.Old))
	copy(old, o.Old)
	for k := range old {
		old[k].Focus, old[k].NodeID = focus(old[k].Ref, component)
	}
	o.Old = old
	own := reason == "" || (o.Health != nil && o.Health.Reason == reason)
	if !own && len(kids) == 0 {
		return o, false
	}
	o.Children = kids
	if reason != "" {
		o.Old = nil
	}
	return o, true
}

// --- The page ---

// loadOwner reads the owner document of the page at ns/name and the lists
// the page derives from it. A document that failed is in Problem.
func (h *Handler) loadOwner(r *http.Request, k ownerKind, ns, name string) ownerView {
	v := ownerView{Kind: k, Namespace: ns, Name: name, Base: k.base(ns, name), Tab: tabGraph, Topic: k.Topic + ":" + ns + "/" + name}
	var comps []v1.Component
	if k == instanceKind {
		var doc v1.Instance
		if v.Problem = h.fetch(r, v.Base, nil, &doc); v.Problem == nil {
			v.Instance = &doc
			v.Reconcile, v.Health, v.Conditions, v.History, v.LastApplied = doc.Reconcile, doc.Health, doc.Conditions, doc.History, doc.LastApplied
			v.ProviderOf, comps = doc.ProviderOf, doc.Components
		}
	} else {
		var doc v1.Package
		if v.Problem = h.fetch(r, v.Base, nil, &doc); v.Problem == nil {
			v.Package = &doc
			v.Reconcile, v.Health, v.Conditions, v.History, v.LastApplied = doc.Reconcile, doc.Health, doc.Conditions, doc.History, doc.LastApplied
			v.ProviderOf, comps = doc.ProviderOf, doc.Components
		}
	}
	if v.Problem == nil {
		v.Components = h.components(v.Base, comps)
		v.firstContainer = containersOf(comps)
	}
	return v
}

func (h *Handler) ownerPage(k ownerKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ns, name := r.PathValue("namespace"), r.PathValue("name")
		q := r.URL.Query()
		v := h.loadOwner(r, k, ns, name)
		if unauthenticated(v.Problem) {
			h.signIn(w, r)
			return
		}
		tabs := ownerTabsFor(len(v.ProviderOf) > 0)
		for _, t := range tabs {
			if q.Get("tab") == t.Name {
				v.Tab = t.Name
			}
		}
		for _, t := range tabs {
			tab := tabLink{Name: t.Name, Label: t.Label, Href: tabHref(v.Base, q, t.Name), Current: t.Name == v.Tab}
			if t.Name == tabResources || t.Name == tabEvents {
				tab.Follow = v.countFollow()
			}
			v.Tabs = append(v.Tabs, tab)
		}
		if v.Problem == nil {
			h.ownerTabsOf(r, &v)
		}
		topics := []string{v.Topic, "events:" + v.Topic}
		if len(v.ProviderOf) > 0 {
			// The Provider card reads the Platform's registration on every tab.
			topics = append(topics, platformTopic)
		}
		if v.Tab == tabProvider {
			// The tab joins the instances that use what a registration provides.
			topics = append(topics, "instances")
		}
		h.render(w, r, problemStatus(v.Problem), "owner", page{
			Title:  k.Title + " " + ns + "/" + name,
			Nav:    "installed",
			Topics: topics,
			Main:   v,
		})
	}
}

// ownerTabsOf reads what the cards and the open tab show.
func (h *Handler) ownerTabsOf(r *http.Request, v *ownerView) {
	q := r.URL.Query()
	v.Node = q.Get("node")
	var g v1.Graph
	if v.GraphProblem = h.fetch(r, v.Base+"/graph", graphQuery(r), &g); v.GraphProblem == nil {
		v.Focus = q.Get("focus")
		v.Graph = buildGraph(g, v.Kind.Topic+"-graph", v.Base, v.Base+"/node", v.Focus, q["expand"]...)
		v.Graph.FitGroup = q.Get("fit")
		v.Components, v.Configs = foldConfig(v.Components, &g)
		if len(q["expand"]) > 0 {
			v.Graph.Whole = v.Base + "?tab=" + tabGraph
		}
		v.Graph.Clear = linkWithout(v.Base, q, selectionParams...)
		if v.Tab == tabGraph || v.Tab == tabResources {
			h.ownerPanel(r, v, &g)
		}
	}
	// The Resources count comes before the tab switch: annotateResources
	// drops the rows a reason filter does not match, and the count is the
	// tab's whole list (portal:D19:R3).
	setCount(v.Tabs, tabResources, resourceCount(v.Components, v.Configs))
	// The merged feed is read on every tab (design.md, "Scale and refresh"),
	// so the Events count is its folded count on every tab.
	v.Events = h.events(r, v.Base+"/events", url.Values{"scope": {"all"}}, "events:"+v.Topic)
	// The Events count comes before ownerEventsTab, which filters the feed
	// by type and reason or replaces it with another object's.
	v.countEvents()
	h.ownerCards(r, v)
	switch v.Tab {
	case tabResources:
		v.Resources = resourcesOf(q, v, &g)
	case tabEvents:
		h.ownerEventsTab(r, v)
	case tabLogs:
		ownerLogsTab(r, v)
	case tabYAML:
		h.ownerYAMLTab(r, v, &g)
	case tabProvider:
		v.Provider = h.providerTab(r, v)
	}
}

// restingNode is the node the details panel shows when neither node nor
// focus is set: the owner's own. align-graph extends it with its resting rule
// (a held registration whose claim needs attention) and replaces this body.
func restingNode(g *v1.Graph) string { return g.Root }

// ownerPanel builds the details panel of the Graph and Resources tabs: the
// node named by node, else focus, else the node the graph rests on, which
// shows with no spotlight, so the panel is never an empty placeholder.
func (h *Handler) ownerPanel(r *http.Request, v *ownerView, g *v1.Graph) {
	id := v.Node
	if id == "" {
		id = v.Focus
	}
	rest := restingNode(g)
	v.PanelAtRest = id == ""
	if v.PanelAtRest {
		id = rest
	}
	if rest != "" {
		v.PanelRoot = v.Base + "/node?" + url.Values{"id": {rest}}.Encode()
	}
	if id == "" {
		return
	}
	v.Panel = h.panel(r, g, id, panelContext{owner: v.Kind, base: v.Base, firstContainer: v.firstContainer, self: v})
	v.Graph.Focused = !v.PanelAtRest && !v.Panel.Missing
}

// allComponents lists every component of the page: the unfolded ones and
// those folded into a group.
func (v *ownerView) allComponents() []componentView {
	if len(v.Configs) == 0 {
		return v.Components
	}
	all := slices.Clone(v.Components)
	for i := range v.Configs {
		all = append(all, v.Configs[i].Components...)
	}
	return all
}

// ownerCards builds the identity facts and the cards under it. The Provider
// card reads the Platform on every tab, so the page reads it once here.
func (h *Handler) ownerCards(r *http.Request, v *ownerView) {
	v.Identity = identityOf(v)
	if len(v.ProviderOf) > 0 {
		v.plat = h.readPlatform(r)
	}
	v.Cards = h.cardsOf(v)
}

// countFollow is the topics a tab count follows: the owner's own and its events.
func (v *ownerView) countFollow() string { return v.Topic + " events:" + v.Topic }

// countEvents counts the Events tab: the folded lines of the feed it opens
// with, and no count when that feed is a problem or left objects out, because
// the caller could not read the whole list (portal:D19:R3).
func (v *ownerView) countEvents() {
	if v.Events.Problem == nil && !v.Events.Partial {
		setCount(v.Tabs, tabEvents, len(v.Events.Items))
	}
}

// resourceCount counts the rows the Resources tab lists: each inventory
// object and every runtime child below it, the scaled-down ReplicaSets and
// the configuration group included.
func resourceCount(cs []componentView, configs []configGroup) int {
	n := 0
	count := func(cs []componentView) {
		for i := range cs {
			for j := range cs[i].Objects {
				o := &cs[i].Objects[j]
				n += 1 + len(o.Children) + len(o.Old)
			}
		}
	}
	count(cs)
	for i := range configs {
		count(configs[i].Components)
	}
	return n
}

// kindModulePackage is the Kubernetes kind of a package owner.
const kindModulePackage = "ModulePackage"

// objectKind is the owner's Kubernetes kind.
func (k ownerKind) objectKind() string {
	if k == packageKind {
		return kindModulePackage
	}
	return "ModuleInstance"
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
