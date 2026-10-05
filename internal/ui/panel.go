package ui

import (
	"net/http"
	"net/url"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// panelContext is the owner page a node panel opens on.
type panelContext struct {
	owner ownerKind
	base  string
	// firstContainer names each Pod's first container, for its Logs link.
	firstContainer func(pod string) string
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
	// Links.
	Open   string
	YAML   string
	Events string
	Expand string
	// Logs opens the Logs tab, for a Pod.
	Logs      string
	EdgesFrom []string
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
	if !p.Locked {
		p.Open, p.YAML, p.Events = links(n, ctx)
		if n.Ref != nil && isPod(*n.Ref) && ctx.base != "" && !n.Missing {
			p.Logs = ctx.base + "?tab=" + tabLogs
			if ctx.firstContainer != nil {
				if c := ctx.firstContainer(n.Ref.Name); c != "" {
					p.Logs += "#" + logID(n.Ref.Name, c)
				}
			}
		}
		if n.Group != nil {
			p.Expand = expandLink(r, n.ID, ctx)
		}
	}
	return p
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
