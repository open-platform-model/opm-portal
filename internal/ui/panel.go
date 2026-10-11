package ui

import (
	"net/http"
	"net/url"
	"strings"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// panelContext is the owner page a node panel opens on.
type panelContext struct {
	owner ownerKind
	base  string
	// firstContainer names each Pod's first container, for its Logs link.
	firstContainer func(pod string) string
	// self is the owner the page is about, for the identity facts of the
	// panel of the owner's own node. It is nil where the page did not need it.
	self *ownerView
}

// nodePanel is what a graph node's detail panel shows: only the fields
// the graph document carries for it, and links to what it may open.
type nodePanel struct {
	Missing   bool
	ID        string
	Kind      string
	Label     string
	Ref       *v1.ObjectRef
	RefText   string
	Locked    bool
	Access    string
	Absent    bool
	Health    *v1.GraphHealth
	Reconcile *v1.Reconcile
	// ReconcileMessage is the applied message, so the verdict's is not
	// shown twice when the operator wrote the same text to both.
	ReconcileMessage string
	Owner            string
	Contracts        []string
	Version          string
	Path             string
	Group            *v1.GraphGroup
	Replicas         *int64
	Unread           bool
	ScaledDown       int
	// Tone is the hue class of the message box, from the node's health.
	Tone string
	// Message is the node's health message, drawn in a box toned by its health.
	Message string
	// Root is true for the owner's own node: no Open link, an Events link,
	// and the owner's identity facts.
	Root bool
	// Component is true for a component node, which is not a Kubernetes
	// object; Renders names what it renders.
	Component bool
	Renders   []string
	// Facts are the per-kind facts of the object, or the identity facts of
	// the owner's node. FactsUnread says the object could not be read.
	Facts       []fact
	FactsUnread bool
	// Links are the buttons, in order: Logs first for a Pod.
	Links     []panelLink
	EdgesFrom []string
}

// panelLink is one button of the panel. Dark marks the primary one. HX links
// load into the panel when script runs and are plain links without it.
type panelLink struct {
	Text string
	Href string
	Dark bool
	HX   bool
}

// panel builds the panel of node id in g, or a missing panel.
func (h *Handler) panel(r *http.Request, g *v1.Graph, id string, ctx panelContext) *nodePanel {
	n := findNode(g, id)
	if n == nil {
		return &nodePanel{Missing: true, ID: id}
	}
	p := &nodePanel{
		ID:         n.ID,
		Kind:       kindText[n.Kind],
		Label:      n.Label,
		Ref:        n.Ref,
		Locked:     n.Access != "" && n.Access != v1.AccessOK,
		Access:     n.Access,
		Absent:     n.Missing,
		Health:     n.Health,
		Reconcile:  n.Reconcile,
		Owner:      n.Owner,
		Contracts:  n.RenderContracts,
		Version:    n.Version,
		Path:       n.Path,
		Group:      n.Group,
		Replicas:   n.Replicas,
		Unread:     n.ChildrenUnread,
		ScaledDown: n.HiddenScaledDown,
	}
	if p.Kind == "" {
		p.Kind = "Unknown kind"
	}
	if n.Reconcile != nil {
		p.ReconcileMessage = n.Reconcile.Message
	}
	if n.Ref != nil {
		p.RefText = refText(*n.Ref)
	}
	p.EdgesFrom = edgeLines(g, n.ID)
	if p.Locked {
		return p
	}
	if n.Health != nil {
		p.Message = n.Health.Message
		p.Tone = healthBlockHue(n.Health.State)
	}
	p.Component = n.Kind == "component"
	if p.Component {
		p.Renders = rendersOf(g, n.ID)
	}
	p.Root = isOwnerNode(n, ctx)
	if p.Root {
		p.Facts = ownerFacts(ctx.self)
	} else {
		p.Facts, p.FactsUnread = h.objectFacts(r, n, ctx)
	}
	p.Links = panelLinks(r, n, ctx)
	return p
}

// isOwnerNode reports whether n is the node of the owner the page is about.
func isOwnerNode(n *v1.GraphNode, ctx panelContext) bool {
	return n.Ref != nil && (n.Kind == instanceKind.Topic || n.Kind == packageKind.Topic) &&
		ctx.base != "" && ctx.owner.base(n.Ref.Namespace, n.Ref.Name) == ctx.base
}

// rendersOf lists the kinds and names a component node renders: the targets
// of its outgoing edges that stand for an object.
func rendersOf(g *v1.Graph, id string) []string {
	var out []string
	for i := range g.Edges {
		e := &g.Edges[i]
		if e.From != id {
			continue
		}
		if t := findNode(g, e.To); t != nil && t.Ref != nil {
			out = append(out, t.Ref.Kind+" "+t.Ref.Name)
		}
	}
	return out
}

// ownerFacts are the identity facts the panel of the owner's own node adds:
// the module and its version, or the source, path, interval and revision,
// then the owner and applier. They come from the owner document the page
// already read (design.md decision 5).
func ownerFacts(v *ownerView) []fact {
	if v == nil {
		return nil
	}
	id := identityOf(v)
	var out []fact
	switch {
	case v.Instance != nil:
		out = appendFact(out, "Module", id.Subtitle, true)
		out = appendFact(out, "Version", id.Version, false)
	case v.Package != nil:
		out = appendFact(out, "Source", strings.TrimPrefix(id.Subtitle, "from "), true)
		out = appendFact(out, "Path", v.Package.Path, true)
		out = appendFact(out, "Interval", id.Interval, false)
		rev := "not recorded"
		if v.Package.SourceArtifact != nil && v.Package.SourceArtifact.Revision != "" {
			rev = v.Package.SourceArtifact.Revision
		}
		out = append(out, fact{Label: "Revision", Value: rev, Mono: v.Package.SourceArtifact != nil})
	}
	out = appendFact(out, "Owner", id.Owner, false)
	sa := "the controller's own ServiceAccount"
	switch {
	case v.Instance != nil && v.Instance.ServiceAccountName != "":
		sa = "ServiceAccount " + v.Instance.ServiceAccountName
	case v.Package != nil && v.Package.ServiceAccountName != "":
		sa = "ServiceAccount " + v.Package.ServiceAccountName
	}
	return appendFact(out, "Applier", sa, false)
}

// panelLinks are the buttons a readable node may open, in order: Logs first
// and dark for a Pod, then Events and YAML, Open and Expand. The owner's own
// node gets no Open link (it is the open page) and an Events link to the tab.
func panelLinks(r *http.Request, n *v1.GraphNode, ctx panelContext) []panelLink {
	var out []panelLink
	if n.Ref != nil && isPod(*n.Ref) && ctx.base != "" && !n.Missing {
		href := ctx.base + "?tab=" + tabLogs
		if ctx.firstContainer != nil {
			if c := ctx.firstContainer(n.Ref.Name); c != "" {
				href += "#" + logID(n.Ref.Name, c)
			}
		}
		out = append(out, panelLink{Text: "Logs", Href: href, Dark: true})
	}
	open, yamlView, events := links(n, ctx)
	switch {
	case isOwnerNode(n, ctx):
		out = append(out, panelLink{Text: "Events", Href: ctx.base + "?tab=" + tabEvents})
	default:
		if events != "" {
			out = append(out, panelLink{Text: "Events", Href: events, HX: true})
		}
		if yamlView != "" {
			out = append(out, panelLink{Text: "YAML", Href: yamlView, HX: true})
		}
		if open != "" {
			out = append(out, panelLink{Text: "Open page", Href: open})
		}
	}
	if n.Group != nil {
		out = append(out, panelLink{Text: "Expand this group", Href: expandLink(r, n.ID, ctx)})
	}
	return out
}

// edgeLines describes every edge at node id with its source field.
func edgeLines(g *v1.Graph, id string) []string {
	var out []string
	for i := range g.Edges {
		e := &g.Edges[i]
		if e.From != id && e.To != id {
			continue
		}
		line := e.Kind + " (" + e.Source + ")"
		if e.Verified != nil && !*e.Verified {
			line += ", not confirmed: " + e.Reason
		}
		out = append(out, line)
	}
	return out
}

// links are what a readable node may open: its page, or its YAML and
// events on the owner page it is on.
func links(n *v1.GraphNode, ctx panelContext) (open, yamlView, events string) {
	switch {
	case n.Kind == instanceKind.Topic && n.Ref != nil:
		return instanceKind.base(n.Ref.Namespace, n.Ref.Name), "", ""
	case n.Kind == packageKind.Topic && n.Ref != nil:
		return packageKind.base(n.Ref.Namespace, n.Ref.Name), "", ""
	case (n.Kind == "object" || n.Kind == "runtime") && n.Ref != nil && ctx.base != "" && !n.Missing && !isSecret(*n.Ref):
		q := refQuery(*n.Ref).Encode()
		return "", ctx.base + "/object?" + q, ctx.base + "/events?" + q
	}
	return "", "", ""
}

// expandLink is the Graph tab with group id added to the expanded groups,
// fitted to its members.
func expandLink(r *http.Request, id string, ctx panelContext) string {
	q := url.Values{"tab": {tabGraph}}
	for _, e := range r.URL.Query()["expand"] {
		q.Add("expand", e)
	}
	q.Add("expand", id)
	q.Set("fit", id)
	return ctx.base + "?" + q.Encode()
}

// renderPanel writes a node panel fragment.
func (h *Handler) renderPanel(w http.ResponseWriter, r *http.Request, p *nodePanel) {
	status := http.StatusOK
	if p.Missing {
		status = http.StatusNotFound
	}
	h.renderFragment(w, r, status, "panel", page{Title: "Node", Main: p})
}
