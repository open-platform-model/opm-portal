package ui

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"

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

var ownerTabs = []struct{ Name, Label string }{
	{tabGraph, "Graph"}, {tabResources, "Resources"}, {tabEvents, "Events"}, {tabLogs, "Logs"}, {tabYAML, "YAML"},
}

// Event types the Events tab filters by.
const (
	eventNormal  = "Normal"
	eventWarning = "Warning"
)

// tabLink is one tab.
type tabLink struct {
	Label   string
	Href    string
	Current bool
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
	// Config is the configuration components the graph folds into one
	// group, listed folded the same way; nil when the graph does not fold.
	Config *configGroup

	Tab  string
	Tabs []tabLink

	Applied appliedCard
	Healthc healthCard

	Graph        svgGraph
	GraphProblem *v1.Problem
	// Focus is the node the graph spotlights.
	Focus string
	Panel *nodePanel

	// ReasonForm filters the Resources tab by a health reason.
	ReasonForm filterForm
	Events     eventsView
	EventsForm eventsForm
	Logs       []logPane
	YAML       yamlTab
	Provider   providerTabView

	firstContainer func(string) string
}

// --- Cards ---

// appliedCard is what the controller applied: the badge and its reason's
// meaning, the Reconciling mark, the Warning reasons of the current feed,
// and one dot per recorded attempt.
type appliedCard struct {
	Meaning     string
	Reconciling *v1.Condition
	Warnings    []countLink
	Dots        []attemptDot
}

// attemptDot is one status.history entry, oldest first, colored by the
// outcome the read API decided; there is no dot for an attempt the history
// does not record (portal:D9:R1).
type attemptDot struct {
	Class string
	Title string
}

// healthCard counts objects and runtime children by health reason, each
// linking to the Resources tab filtered by it. A reason an object only
// mirrors from a child below it is counted once, at the child.
type healthCard struct {
	Reasons []countLink
}

func appliedCardOf(v *ownerView) appliedCard {
	var c appliedCard
	for i := range v.Conditions {
		cond := &v.Conditions[i]
		if cond.Type == "Reconciling" && cond.Status == "True" {
			c.Reconciling = cond
		}
		if cond.Reason != "" && cond.Reason == v.Reconcile.Reason && c.Meaning == "" {
			c.Meaning = cond.Meaning
		}
	}
	counts := map[string]int{}
	var order []string
	for i := range v.Events.Items {
		e := &v.Events.Items[i]
		if e.Type != eventWarning {
			continue
		}
		if counts[e.Reason] == 0 {
			order = append(order, e.Reason)
		}
		counts[e.Reason] += int(e.Count)
	}
	for _, r := range order {
		c.Warnings = append(c.Warnings, countLink{Text: r, N: counts[r],
			Href: v.Base + "?" + url.Values{"tab": {tabEvents}, "type": {eventWarning}, "reason": {r}}.Encode()})
	}
	for i := len(v.History) - 1; i >= 0; i-- {
		h := &v.History[i]
		outcome := h.Outcome
		switch outcome {
		case "Succeeded", "Failed":
		default:
			outcome = "Unknown"
		}
		title := "#" + itoa(h.Sequence) + " " + h.Action + ": " + strings.ToLower(outcome)
		if h.FinishedAt != nil {
			title += ", " + h.FinishedAt.UTC().Format("2006-01-02 15:04:05 UTC")
		}
		if h.Message != "" {
			title += ": " + h.Message
		}
		c.Dots = append(c.Dots, attemptDot{Class: "dot-" + strings.ToLower(outcome), Title: title})
	}
	return c
}

func healthCardOf(v *ownerView) healthCard {
	counts := map[string]int{}
	add := func(h *v1.ObjectHealth) {
		if h != nil && h.Reason != "" && h.State != "Healthy" {
			counts[h.Reason]++
		}
	}
	walk := func(cs []componentView) {
		for i := range cs {
			for j := range cs[i].Objects {
				o := &cs[i].Objects[j]
				mirrored := false
				for k := range o.Children {
					add(&o.Children[k].Health)
					mirrored = mirrored || (o.Health != nil && o.Children[k].Health.Reason == o.Health.Reason)
				}
				// A workload degraded only because a child waits carries the
				// child's reason: the reason is counted once, at its source.
				if !mirrored {
					add(o.Health)
				}
			}
		}
	}
	walk(v.Components)
	if v.Config != nil {
		walk(v.Config.Components)
	}
	reasons := make([]string, 0, len(counts))
	for r := range counts {
		reasons = append(reasons, r)
	}
	slices.Sort(reasons)
	var c healthCard
	for _, r := range reasons {
		c.Reasons = append(c.Reasons, countLink{Text: r, N: counts[r],
			Href: v.Base + "?" + url.Values{"tab": {tabResources}, "reason": {r}}.Encode()})
	}
	return c
}

// --- Resources ---

// reasonFilter is the Resources tab's one filter: a health reason, matched
// as given.
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
	focus := func(r v1.ObjectRef, component string) string {
		if id := byRef[refKey(r)]; id != "" {
			return base + "?" + url.Values{"tab": {tabGraph}, "focus": {id}}.Encode()
		}
		// Folded into a group: open the group and focus the object's own
		// node in it.
		group := groupOf[r.Name]
		if group == "" {
			group = groupOf[component]
		}
		if group == "" {
			return base + "?tab=" + tabGraph
		}
		return base + "?" + url.Values{"tab": {tabGraph}, "expand": {group}, "focus": {objectNodeID(r)}}.Encode()
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
func annotateObject(o objectView, component string, focus func(v1.ObjectRef, string) string, reason string) (objectView, bool) {
	o.Focus = focus(o.Ref, component)
	kids := make([]childView, 0, len(o.Children))
	for k := range o.Children {
		ch := o.Children[k]
		ch.Focus = focus(ch.Ref, component)
		if reason == "" || ch.Health.Reason == reason {
			kids = append(kids, ch)
		}
	}
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

// --- Events ---

// eventsForm is the Events tab's filters: the resource, grouped by kind,
// the type and the reason.
type eventsForm struct {
	Filters filters
	Groups  []optionGroup
	Types   []selectOption
	Reason  string
	Shown   int
	Total   int
}

// resourceValue names an object in the resource filter: group/Kind/ns/name.
func resourceValue(r v1.ObjectRef) string {
	return r.Group + "/" + r.Kind + "/" + r.Namespace + "/" + r.Name
}

func parseResource(s string) v1.ObjectRef {
	parts := strings.SplitN(s, "/", 4)
	if len(parts) != 4 {
		return v1.ObjectRef{}
	}
	return v1.ObjectRef{Group: parts[0], Kind: parts[1], Namespace: parts[2], Name: parts[3]}
}

// reachedRefs lists every readable object and child an owner's inventory
// reaches, the Secrets aside.
func reachedRefs(cs []componentView) []v1.ObjectRef {
	var out []v1.ObjectRef
	for i := range cs {
		for j := range cs[i].Objects {
			o := &cs[i].Objects[j]
			if o.Access == v1.AccessOK && !isSecret(o.Ref) {
				out = append(out, o.Ref)
			}
			for k := range o.Children {
				out = append(out, o.Children[k].Ref)
			}
			for k := range o.Old {
				out = append(out, o.Old[k].Ref)
			}
		}
	}
	return out
}

func (h *Handler) ownerEventsTab(r *http.Request, v *ownerView, refs []v1.ObjectRef) {
	resource := filterParam{Name: "resource", Label: "Resource", Text: map[string]string{}}
	byKind := map[string][]v1.ObjectRef{}
	var kinds []string
	for _, ref := range refs {
		val := resourceValue(ref)
		resource.Values = append(resource.Values, val)
		resource.Text[val] = ref.Kind + " " + ref.Name
		if byKind[ref.Kind] == nil {
			kinds = append(kinds, ref.Kind)
		}
		byKind[ref.Kind] = append(byKind[ref.Kind], ref)
	}
	if resource.Values == nil {
		resource.Values = []string{}
	}
	view := filterView{Path: v.Base, Params: []filterParam{resource,
		{Name: "type", Label: "Type", Values: []string{eventNormal, eventWarning}},
		{Name: "reason", Label: "Reason"}}}
	f := parseFilters(&view, r.URL.Query())
	form := eventsForm{Filters: f, Reason: f.Values["reason"]}
	slices.Sort(kinds)
	for _, k := range kinds {
		g := optionGroup{Label: k}
		for _, ref := range byKind[k] {
			val := resourceValue(ref)
			g.Options = append(g.Options, selectOption{Value: val, Text: ref.Name, Selected: f.Values["resource"] == val})
		}
		form.Groups = append(form.Groups, g)
	}
	for _, t := range []string{eventNormal, eventWarning} {
		form.Types = append(form.Types, selectOption{Value: t, Text: t, Selected: f.Values["type"] == t})
	}
	if res := f.Values["resource"]; res != "" {
		ref := parseResource(res)
		// The object's own events have no topic; the region follows the
		// owner's, and the stream's periodic refresh re-reads it.
		ev := h.events(r, v.Base+"/events", refQuery(ref), "events:"+v.Topic)
		ev.About = refText(ref)
		v.Events = ev
	}
	form.Total = len(v.Events.Items)
	items := v.Events.Items[:0:0]
	for i := range v.Events.Items {
		e := &v.Events.Items[i]
		if (f.Values["type"] == "" || e.Type == f.Values["type"]) && (f.Values["reason"] == "" || e.Reason == f.Values["reason"]) {
			items = append(items, *e)
		}
	}
	v.Events.Items = items
	form.Shown = len(items)
	v.EventsForm = form
}

// --- YAML ---

// yamlTab is the YAML tab: a picker of the objects the inventory reaches,
// grouped by kind, and the YAML of the one picked.
type yamlTab struct {
	Groups []yamlGroup
	Picked objectYAML
	Has    bool
}

type yamlGroup struct {
	Kind  string
	Items []yamlItem
}

type yamlItem struct {
	Text    string
	Href    string
	Current bool
	// Secret: listed as never read, with no link.
	Secret bool
}

func (h *Handler) ownerYAMLTab(r *http.Request, v *ownerView) {
	picked := r.URL.Query().Get("object")
	byKind := map[string][]yamlItem{}
	var kinds []string
	add := func(ref v1.ObjectRef, secret bool) {
		val := resourceValue(ref)
		item := yamlItem{Text: ref.Name, Secret: secret, Current: val == picked}
		if ref.Namespace != "" {
			item.Text = ref.Namespace + "/" + ref.Name
		}
		if !secret {
			item.Href = v.Base + "?" + url.Values{"tab": {tabYAML}, "object": {val}}.Encode()
		}
		if byKind[ref.Kind] == nil {
			kinds = append(kinds, ref.Kind)
		}
		byKind[ref.Kind] = append(byKind[ref.Kind], item)
	}
	all := v.Components
	if v.Config != nil {
		all = append(slices.Clone(all), v.Config.Components...)
	}
	for i := range all {
		for j := range all[i].Objects {
			o := &all[i].Objects[j]
			if o.Access != v1.AccessOK {
				continue
			}
			add(o.Ref, isSecret(o.Ref))
			for k := range o.Children {
				add(o.Children[k].Ref, false)
			}
			for k := range o.Old {
				add(o.Old[k].Ref, false)
			}
		}
	}
	slices.Sort(kinds)
	for _, k := range kinds {
		v.YAML.Groups = append(v.YAML.Groups, yamlGroup{Kind: k, Items: byKind[k]})
	}
	if picked == "" {
		return
	}
	ref := parseResource(picked)
	var doc v1.Object
	v.YAML.Has = true
	if p := h.fetch(r, v.Base+"/object", refQuery(ref), &doc); p != nil {
		v.YAML.Picked.Problem = p
		return
	}
	out, err := yaml.Marshal(doc.Object)
	if err != nil {
		v.YAML.Picked.Problem = unavailable()
		return
	}
	v.YAML.Picked = objectYAML{Ref: doc.Ref, Text: refText(doc.Ref), YAML: string(out)}
}

// --- The page ---

func (h *Handler) ownerPage(k ownerKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ns, name := r.PathValue("namespace"), r.PathValue("name")
		q := r.URL.Query()
		v := ownerView{Kind: k, Namespace: ns, Name: name, Base: k.base(ns, name), Tab: tabGraph}
		v.Topic = k.Topic + ":" + ns + "/" + name
		api := v.Base
		if k == instanceKind {
			var doc v1.Instance
			if v.Problem = h.fetch(r, api, nil, &doc); v.Problem == nil {
				v.Instance = &doc
				v.Reconcile, v.Health, v.Conditions, v.History, v.LastApplied = doc.Reconcile, doc.Health, doc.Conditions, doc.History, doc.LastApplied
				v.ProviderOf = doc.ProviderOf
				v.Components = h.components(v.Base, doc.Components)
				v.firstContainer = containersOf(doc.Components)
			}
		} else {
			var doc v1.Package
			if v.Problem = h.fetch(r, api, nil, &doc); v.Problem == nil {
				v.Package = &doc
				v.Reconcile, v.Health, v.Conditions, v.History, v.LastApplied = doc.Reconcile, doc.Health, doc.Conditions, doc.History, doc.LastApplied
				v.ProviderOf = doc.ProviderOf
				v.Components = h.components(v.Base, doc.Components)
				v.firstContainer = containersOf(doc.Components)
			}
		}
		if unauthenticated(v.Problem) {
			h.signIn(w, r)
			return
		}
		tabs := ownerTabs
		if len(v.ProviderOf) > 0 {
			tabs = append(slices.Clone(tabs), struct{ Name, Label string }{tabProvider, "Provider"})
		}
		for _, t := range tabs {
			if q.Get("tab") == t.Name {
				v.Tab = t.Name
			}
		}
		for _, t := range tabs {
			v.Tabs = append(v.Tabs, tabLink{Label: t.Label, Href: v.Base + "?tab=" + t.Name, Current: t.Name == v.Tab})
		}
		if v.Problem == nil {
			h.ownerTabsOf(r, &v)
		}
		topics := []string{v.Topic, "events:" + v.Topic}
		if v.Tab == tabProvider {
			// The tab joins the Platform's registrations and the instances
			// that use what they provide.
			topics = append(topics, "platform", "instances")
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
	var g v1.Graph
	if v.GraphProblem = h.fetch(r, v.Base+"/graph", graphQuery(r), &g); v.GraphProblem == nil {
		v.Focus = q.Get("focus")
		v.Graph = buildGraph(g, v.Kind.Topic+"-graph", v.Base, v.Base+"/node", v.Focus, q["expand"]...)
		v.Graph.FitGroup = q.Get("fit")
		v.Components, v.Config = foldConfig(v.Components, &g)
		if v.Focus != "" {
			v.Panel = h.panel(r, &g, v.Focus, panelContext{owner: v.Kind, base: v.Base, firstContainer: v.firstContainer})
		}
		if len(q["expand"]) > 0 {
			v.Graph.Whole = v.Base + "?tab=" + tabGraph
		}
		v.Graph.Clear = linkWithout(v.Base, q, "focus")
		v.Graph.Focused = v.Panel != nil && !v.Panel.Missing
	}
	v.Logs = logPanes(v.Components)
	if v.Config != nil {
		v.Logs = append(v.Logs, logPanes(v.Config.Components)...)
	}
	v.Events = h.events(r, v.Base+"/events", nil, "events:"+v.Topic)
	v.Applied = appliedCardOf(v)
	v.Healthc = healthCardOf(v)
	switch v.Tab {
	case tabResources:
		byRef, groupOf := focusIDs(&g)
		reason := strings.TrimSpace(q.Get("reason"))
		rv := filterView{Path: v.Base, Params: reasonFilter.Params}
		reasons := make([]string, 0, len(v.Healthc.Reasons))
		for _, c := range v.Healthc.Reasons {
			reasons = append(reasons, c.Text)
		}
		v.ReasonForm = newFilterForm(parseFilters(&rv, q), "Filter resources", keepHidden(q, "reason", "focus"),
			map[string][]string{"reason": reasons}, map[string]string{"reason": "a reason, such as ImagePullBackOff"})
		v.Components = annotateResources(v.Components, byRef, groupOf, v.Base, reason)
		if v.Config != nil {
			v.Config.Components = annotateResources(v.Config.Components, byRef, groupOf, v.Base, reason)
			if len(v.Config.Components) == 0 {
				v.Config = nil
			} else if reason != "" {
				v.Config.Open = true
			}
		}
	case tabEvents:
		all := v.Components
		if v.Config != nil {
			all = append(slices.Clone(all), v.Config.Components...)
		}
		refs := append([]v1.ObjectRef{{Group: "opmodel.dev", Kind: v.Kind.objectKind(), Namespace: v.Namespace, Name: v.Name}}, reachedRefs(all)...)
		h.ownerEventsTab(r, v, refs)
	case tabYAML:
		h.ownerYAMLTab(r, v)
	case tabProvider:
		v.Provider = h.providerTab(r, v)
	}
}

// objectKind is the owner's Kubernetes kind.
func (k ownerKind) objectKind() string {
	if k == packageKind {
		return "ModulePackage"
	}
	return "ModuleInstance"
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
