package ui

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// ownerKind is what an owner page is about: a ModuleInstance or a
// ModulePackage.
type ownerKind struct {
	Path   string // instances or packages
	Title  string // Instance or Package
	Plural string // Instances or Packages
	Topic  string // instance or package
}

var (
	instanceKind = ownerKind{Path: "instances", Title: "Instance", Plural: "Instances", Topic: "instance"}
	packageKind  = ownerKind{Path: "packages", Title: "Package", Plural: "Packages", Topic: "package"}
)

// eventsView is a recent-activity feed and where it is refreshed from.
type eventsView struct {
	Problem *v1.Problem
	Items   []v1.Event
	// About names the object the feed is about, when it is not the page's.
	About string
	// Follow is the events topic the feed follows, if any.
	Follow string
}

// signIn answers a request the read API did not authenticate.
func (h *Handler) signIn(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusUnauthorized, "message", page{Title: "Sign in", Main: message{
		Heading: "You are not signed in",
		Text: "This browser holds no live session for this portal. Open the launch link opm-portal serve printed, " +
			"or restart it for a new one.",
		Code: v1.CodeUnauthenticated,
	}})
}

// problemStatus is the page status a main document's problem gives. A
// forbidden document renders as a locked region of a page that was served
// as asked, so it is 200, as the read API answers a list the caller may
// not read; the others keep the API's status.
func problemStatus(p *v1.Problem) int {
	if p == nil || p.Code == v1.CodeForbidden {
		return http.StatusOK
	}
	if p.Status >= 400 && p.Status <= 599 {
		return p.Status
	}
	return http.StatusServiceUnavailable
}

// --- Platform ---

type platformView struct {
	Problem  *v1.Problem
	Platform v1.Platform
	// Conditions are the Platform's conditions less the ones the contracts
	// banner already shows.
	Conditions   []v1.Condition
	Accepted     int
	Refused      int
	Graph        svgGraph
	GraphProblem *v1.Problem
	Events       eventsView
	Panel        *nodePanel
}

func (h *Handler) platformPage(w http.ResponseWriter, r *http.Request) {
	var v platformView
	v.Problem = h.fetch(r, "/platform", nil, &v.Platform)
	if v.Problem != nil && v.Problem.Code == v1.CodeUnauthenticated {
		h.signIn(w, r)
		return
	}
	if v.Problem == nil {
		for i := range v.Platform.Registrations {
			switch v.Platform.Registrations[i].Verdict {
			case "Accepted", "RemovalBlocked":
				v.Accepted++
			case "Refused":
				v.Refused++
			}
		}
		var g v1.Graph
		if v.GraphProblem = h.fetch(r, "/platform/graph", graphQuery(r), &g); v.GraphProblem == nil {
			v.Graph = buildGraph(g, "platform-graph", "/", "/platform/node", r.URL.Query().Get("node"))
			if id := r.URL.Query().Get("node"); id != "" {
				v.Panel = h.panel(r, &g, id, panelContext{})
			}
		}
		v.Events = h.events(r, "/platform/events", nil, "events:platform")
		v.Conditions = withoutNotes(v.Platform.Conditions, v.Platform.Reconcile.Notes)
	}
	h.render(w, r, problemStatus(v.Problem), "platform", page{
		Title:  "Platform",
		Nav:    "platform",
		Topics: []string{"platform", "events:platform"},
		Main:   v,
	})
}

// withoutNotes drops the conditions the page already shows as notes
// beside the applied state.
func withoutNotes(cs, notes []v1.Condition) []v1.Condition {
	out := make([]v1.Condition, 0, len(cs))
	for i := range cs {
		if !slices.ContainsFunc(notes, func(n v1.Condition) bool { return n.Type == cs[i].Type }) {
			out = append(out, cs[i])
		}
	}
	return out
}

func (h *Handler) platformNode(w http.ResponseWriter, r *http.Request) {
	var g v1.Graph
	if p := h.fetch(r, "/platform/graph", graphQuery(r), &g); p != nil {
		h.renderProblemFragment(w, r, p)
		return
	}
	h.renderPanel(w, r, h.panel(r, &g, r.URL.Query().Get("id"), panelContext{}))
}

func (h *Handler) registrationEvents(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	ev := h.events(r, "/platform/registrations/"+url.PathEscape(name)+"/events", nil, "")
	ev.About = "TransformerRegistration " + name
	h.renderFragment(w, r, problemStatus(ev.Problem), "events", page{Title: "Events", Nav: "platform", Main: ev})
}

// --- Lists ---

type listView struct {
	Kind      ownerKind
	Namespace string
	Problem   *v1.Problem
	Forbidden bool
	Instances []v1.InstanceSummary
	Packages  []v1.PackageSummary
	// Namespaces are the namespaces the listed items live in, for the
	// filter's suggestions.
	Namespaces []string
}

func (h *Handler) instancesPage(w http.ResponseWriter, r *http.Request) {
	v := listView{Kind: instanceKind, Namespace: strings.TrimSpace(r.URL.Query().Get("namespace"))}
	var doc v1.InstanceList
	v.Problem = h.fetch(r, "/instances", nsQuery(v.Namespace), &doc)
	if v.Problem != nil && v.Problem.Code == v1.CodeUnauthenticated {
		h.signIn(w, r)
		return
	}
	v.Forbidden = v.Problem == nil && doc.Access != v1.AccessOK
	v.Instances = doc.Items
	for i := range doc.Items {
		v.Namespaces = appendUnique(v.Namespaces, doc.Items[i].Ref.Namespace)
	}
	topic := "instances"
	if v.Namespace != "" {
		topic += ":" + v.Namespace
	}
	h.render(w, r, problemStatus(v.Problem), "instances", page{Title: "Instances", Nav: "instances", Topics: []string{topic}, Main: v})
}

func (h *Handler) packagesPage(w http.ResponseWriter, r *http.Request) {
	v := listView{Kind: packageKind, Namespace: strings.TrimSpace(r.URL.Query().Get("namespace"))}
	var doc v1.PackageList
	v.Problem = h.fetch(r, "/packages", nsQuery(v.Namespace), &doc)
	if v.Problem != nil && v.Problem.Code == v1.CodeUnauthenticated {
		h.signIn(w, r)
		return
	}
	v.Forbidden = v.Problem == nil && doc.Access != v1.AccessOK
	v.Packages = doc.Items
	for i := range doc.Items {
		v.Namespaces = appendUnique(v.Namespaces, doc.Items[i].Ref.Namespace)
	}
	h.render(w, r, problemStatus(v.Problem), "packages", page{Title: "Packages", Nav: "packages", Main: v})
}

func nsQuery(ns string) url.Values {
	if ns == "" {
		return nil
	}
	return url.Values{"namespace": {ns}}
}

func appendUnique(list []string, s string) []string {
	if s == "" || slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

// --- Owner pages ---

// ownerView is an instance or package page.
type ownerView struct {
	Kind      ownerKind
	Namespace string
	Name      string
	Base      string // the page's path
	Problem   *v1.Problem

	Instance *v1.Instance
	Package  *v1.Package

	Reconcile   v1.Reconcile
	Health      v1.Health
	Conditions  []v1.Condition
	History     []v1.HistoryEntry
	LastApplied v1.Digests
	Components  []componentView
	// Config is the configuration components the graph folds into one
	// group, listed folded the same way; nil when the graph does not fold.
	Config *configGroup

	Graph        svgGraph
	GraphProblem *v1.Problem
	Events       eventsView
	Logs         []logPane
	Panel        *nodePanel
}

type componentView struct {
	Name    string
	Health  v1.Health
	Objects []objectView
}

// configGroup is the configuration components folded into one entry, with
// the health the graph's group node reports. It opens when that health is
// not Healthy.
type configGroup struct {
	Health     string
	Open       bool
	Components []componentView
}

type objectView struct {
	v1.InventoryObject
	Text      string
	YAML      string
	EventsURL string
	Children  []childView
	// Old are the ReplicaSets scaled to zero, folded apart from the live
	// children; OldID is the fold's id, so a refresh keeps it open.
	Old   []childView
	OldID string
}

type childView struct {
	v1.RuntimeChild
	Text      string
	YAML      string
	EventsURL string
	// LogID is the id of the Pod's first container's log pane.
	LogID string
}

// logPane is one container's log, followed on its log topic.
type logPane struct {
	ID        string
	Topic     string
	Pod       string
	Container string
	Previous  string
}

func (k ownerKind) base(ns, name string) string {
	return "/" + k.Path + "/" + url.PathEscape(ns) + "/" + url.PathEscape(name)
}

func (h *Handler) ownerPage(k ownerKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ns, name := r.PathValue("namespace"), r.PathValue("name")
		v := ownerView{Kind: k, Namespace: ns, Name: name, Base: k.base(ns, name)}
		api := v.Base
		if k == instanceKind {
			var doc v1.Instance
			if v.Problem = h.fetch(r, api, nil, &doc); v.Problem == nil {
				v.Instance = &doc
				v.Reconcile, v.Health, v.Conditions, v.History, v.LastApplied = doc.Reconcile, doc.Health, doc.Conditions, doc.History, doc.LastApplied
				v.Components = h.components(v.Base, doc.Components)
			}
		} else {
			var doc v1.Package
			if v.Problem = h.fetch(r, api, nil, &doc); v.Problem == nil {
				v.Package = &doc
				v.Reconcile, v.Health, v.Conditions, v.History, v.LastApplied = doc.Reconcile, doc.Health, doc.Conditions, doc.History, doc.LastApplied
				v.Components = h.components(v.Base, doc.Components)
			}
		}
		if v.Problem != nil && v.Problem.Code == v1.CodeUnauthenticated {
			h.signIn(w, r)
			return
		}
		topic := k.Topic + ":" + ns + "/" + name
		if v.Problem == nil {
			var g v1.Graph
			if v.GraphProblem = h.fetch(r, api+"/graph", graphQuery(r), &g); v.GraphProblem == nil {
				v.Graph = buildGraph(g, k.Topic+"-graph", v.Base, v.Base+"/node", r.URL.Query().Get("node"))
				v.Components, v.Config = foldConfig(v.Components, &g)
				if id := r.URL.Query().Get("node"); id != "" {
					v.Panel = h.panel(r, &g, id, panelContext{owner: k, base: v.Base})
				}
			}
			v.Events = h.events(r, api+"/events", nil, "events:"+topic)
			v.Logs = logPanes(v.Components)
		}
		h.render(w, r, problemStatus(v.Problem), "owner", page{
			Title:  k.Title + " " + ns + "/" + name,
			Nav:    k.Path,
			Topics: []string{topic, "events:" + topic},
			Main:   v,
		})
	}
}

// foldConfig moves the components the graph folds into its configuration
// group out of cs, in their order, so the list folds what the graph folds
// (0030:D4:R5).
func foldConfig(cs []componentView, g *v1.Graph) ([]componentView, *configGroup) {
	var group *v1.GraphNode
	for i := range g.Nodes {
		if n := &g.Nodes[i]; n.Kind == "group" && n.Group != nil && n.Group.Kind == "configuration" {
			group = n
		}
	}
	if group == nil {
		return cs, nil
	}
	cg := &configGroup{Health: "Unknown"}
	if group.Health != nil {
		cg.Health = group.Health.State
	}
	cg.Open = cg.Health != "Healthy"
	rest := make([]componentView, 0, len(cs))
	for i := range cs {
		if slices.Contains(group.Group.Members, cs[i].Name) {
			cg.Components = append(cg.Components, cs[i])
		} else {
			rest = append(rest, cs[i])
		}
	}
	if len(cg.Components) == 0 {
		return cs, nil
	}
	return rest, cg
}

// components adds the links each object offers: its YAML and its events,
// for an object the caller may read and that is not a Secret.
func (h *Handler) components(base string, cs []v1.Component) []componentView {
	out := make([]componentView, 0, len(cs))
	for i := range cs {
		c := componentView{Name: cs[i].Name, Health: cs[i].Health}
		for _, obj := range objectsOf(&cs[i]) {
			c.Objects = append(c.Objects, objectViewOf(base, obj))
		}
		out = append(out, c)
	}
	return out
}

// objectsOf returns pointers to a component's objects.
func objectsOf(c *v1.Component) []*v1.InventoryObject {
	out := make([]*v1.InventoryObject, len(c.Objects))
	for j := range out {
		out[j] = &c.Objects[j]
	}
	return out
}

func objectViewOf(base string, obj *v1.InventoryObject) objectView {
	o := objectView{InventoryObject: *obj, Text: refText(obj.Ref), OldID: domID("old", obj.Ref.Group, obj.Ref.Kind, obj.Ref.Namespace, obj.Ref.Name)}
	if o.Access == v1.AccessOK && !isSecret(o.Ref) {
		q := refQuery(o.Ref).Encode()
		o.YAML, o.EventsURL = base+"/object?"+q, base+"/events?"+q
	}
	for k := range obj.Children {
		ch := obj.Children[k]
		q := refQuery(ch.Ref).Encode()
		cv := childView{RuntimeChild: ch, Text: refText(ch.Ref), YAML: base + "/object?" + q, EventsURL: base + "/events?" + q}
		if isPod(ch.Ref) && len(ch.Containers) > 0 {
			cv.LogID = logID(ch.Ref.Name, ch.Containers[0])
		}
		if ch.Ref.Kind == "ReplicaSet" && ch.Replicas != nil && *ch.Replicas == 0 {
			o.Old = append(o.Old, cv)
			continue
		}
		o.Children = append(o.Children, cv)
	}
	return o
}

// logPanes lists one pane per container of every Pod below the inventory.
func logPanes(cs []componentView) []logPane {
	var out []logPane
	for i := range cs {
		for j := range cs[i].Objects {
			for k := range cs[i].Objects[j].Children {
				ch := &cs[i].Objects[j].Children[k]
				if !isPod(ch.Ref) {
					continue
				}
				for _, c := range ch.Containers {
					topic := "log:" + ch.Ref.Namespace + "/" + ch.Ref.Name + "/" + c
					out = append(out, logPane{ID: logID(ch.Ref.Name, c), Topic: topic, Previous: topic + "/previous", Pod: ch.Ref.Name, Container: c})
				}
			}
		}
	}
	return out
}

func isPod(r v1.ObjectRef) bool { return r.Kind == "Pod" && r.Group == "" }

// logID is the id of a container's log pane.
func logID(pod, container string) string { return domID("log", pod, container) }

// domID joins parts into an element id. The ids it makes reach a page
// only for a Pod's containers (logID: Pod names and container names, both
// DNS labels) and for a workload's old-revisions fold (objectViewOf: an
// object that has old ReplicaSets, so its group, kind, namespace and name
// follow DNS and identifier rules). None of those parts hold an underscore
// or whitespace, so joining them with an underscore keeps ids of different
// parts apart: Pod a-b's container c and Pod a's container b-c differ. A
// name validated only as a path segment, such as an RBAC role's, may hold
// other characters, so an id rendered for such an object needs another
// separator.
func domID(prefix string, parts ...string) string {
	return prefix + "_" + strings.Join(parts, "_")
}

func (h *Handler) ownerNode(k ownerKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := k.base(r.PathValue("namespace"), r.PathValue("name"))
		var g v1.Graph
		if p := h.fetch(r, base+"/graph", graphQuery(r), &g); p != nil {
			h.renderProblemFragment(w, r, p)
			return
		}
		h.renderPanel(w, r, h.panel(r, &g, r.URL.Query().Get("id"), panelContext{owner: k, base: base}))
	}
}

// objectYAML is the YAML view of one object.
type objectYAML struct {
	Problem *v1.Problem
	Ref     v1.ObjectRef
	Text    string
	YAML    string
}

func (h *Handler) ownerObject(k ownerKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := k.base(r.PathValue("namespace"), r.PathValue("name"))
		var doc v1.Object
		v := objectYAML{}
		if v.Problem = h.fetch(r, base+"/object", r.URL.Query(), &doc); v.Problem == nil {
			v.Ref, v.Text = doc.Ref, refText(doc.Ref)
			out, err := yaml.Marshal(doc.Object)
			if err != nil {
				v.Problem = unavailable()
			}
			v.YAML = string(out)
		}
		if v.Problem != nil && v.Problem.Code == v1.CodeUnauthenticated {
			h.signIn(w, r)
			return
		}
		h.renderFragment(w, r, problemStatus(v.Problem), "object", page{Title: "Object", Nav: k.Path, Main: v})
	}
}

func (h *Handler) ownerEvents(k ownerKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := k.base(r.PathValue("namespace"), r.PathValue("name"))
		q := r.URL.Query()
		ev := h.events(r, base+"/events", q, "")
		if q.Get("kind") != "" {
			ev.About = refText(v1.ObjectRef{Group: q.Get("group"), Kind: q.Get("kind"), Namespace: q.Get("namespace"), Name: q.Get("name")})
		}
		if ev.Problem != nil && ev.Problem.Code == v1.CodeUnauthenticated {
			h.signIn(w, r)
			return
		}
		h.renderFragment(w, r, problemStatus(ev.Problem), "events", page{Title: "Events", Nav: k.Path, Main: ev})
	}
}

// events reads one feed.
func (h *Handler) events(r *http.Request, path string, q url.Values, follow string) eventsView {
	var doc v1.EventList
	ev := eventsView{Follow: follow}
	ev.Problem = h.fetch(r, path, q, &doc)
	ev.Items = doc.Items
	return ev
}

// graphQuery passes the graph options of a page request to the API.
func graphQuery(r *http.Request) url.Values {
	in := r.URL.Query()
	out := url.Values{}
	for _, id := range in["expand"] {
		out.Add("expand", id)
	}
	if v := in.Get("showScaledDown"); v != "" {
		out.Set("showScaledDown", v)
	}
	return out
}

// renderProblemFragment answers a fragment request whose document failed.
func (h *Handler) renderProblemFragment(w http.ResponseWriter, r *http.Request, p *v1.Problem) {
	if p.Code == v1.CodeUnauthenticated {
		h.signIn(w, r)
		return
	}
	h.renderFragment(w, r, problemStatus(p), "problem", page{Title: "Unavailable", Main: p})
}
