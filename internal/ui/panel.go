package ui

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// panelContext is the owner page a node panel opens on.
type panelContext struct {
	owner ownerKind
	base  string
	// firstContainer names each Pod's first container, for its Logs link.
	firstContainer func(pod string) string
	// resting is the id of the node the page rests on; its panel carries
	// no Clear selection control.
	resting string
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
	// Resting: this is the panel the page rests on, which is not a
	// selection. Clear is the page without a selection, for any other panel.
	Resting bool
	Clear   string
	// The variants: a display group, a registration claim and a package's
	// source (design.md "Panel variants").
	Family    string
	GroupNote string
	Kinds     []v1.GraphKindCount
	Collapse  string
	Reg       *regPanel
	Src       *srcPanel
}

// regPanel is a registration node's panel: its standing, the controller's
// message (served in local mode only), the claim and who provides it.
type regPanel struct {
	Standing    badge
	Reason      string
	Message     string
	ActiveBy    string
	CatalogText string
	CatalogHref string
	Provider    string
	ProviderRef string
	Role        string
}

// srcPanel is a package source node's panel: the readiness the controller
// reads, and a sentence for the states that are not a reading.
type srcPanel struct {
	Status   string
	Reason   string
	Message  string
	Revision string
	Sentence string
}

// The words of the panel variants.
const (
	groupSentence = "A display group, not a Kubernetes object. It folds %d configuration components so the workloads stay readable."
	regRole       = "Claims a catalog for this provider; the controller decides whether it is accepted and active."
	srcNotServed  = "The cluster does not serve this kind. Install Flux source-controller, or point the package at a source kind the cluster has."
	srcUnsupport  = "The controller reads only OCIRepository, GitRepository and Bucket."
)

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
	p.Resting = ctx.resting != "" && n.ID == ctx.resting
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
		p.variant(r, g, n, ctx)
	}
	if !p.Resting {
		p.Clear = ctx.base + "?tab=" + tabGraph
	}
	return p
}

// variant fills the part of a readable node's panel that only a group, a
// registration or a source has.
func (p *nodePanel) variant(r *http.Request, g *v1.Graph, n *v1.GraphNode, ctx panelContext) {
	switch {
	case n.Group != nil:
		if n.Group.Kind == groupConfiguration {
			p.Family = firstOf(familyText[n.Group.Family], "Configuration")
			p.Kinds = n.Group.ObjectKinds
			p.GroupNote = fmt.Sprintf(groupSentence, len(n.Group.Members))
		}
		if slices.Contains(r.URL.Query()["expand"], n.ID) {
			p.Collapse = collapseLink(r, n.ID, ctx)
		} else {
			p.Expand = expandLink(r, n.ID, ctx)
		}
	case n.Registration != nil:
		p.Reg = registrationPanel(g, n)
	case n.Kind == kindSource && n.SourceState != nil:
		p.Src = sourcePanel(n.SourceState)
	}
}

// registrationPanel words a claim as the controller wrote it: its
// standing, reason and messages are the controller's, never computed here.
// The read API serves the messages in local mode only, so an in-cluster
// panel holds none.
func registrationPanel(g *v1.Graph, n *v1.GraphNode) *regPanel {
	r := n.Registration
	slug, text := standingOf(r)
	out := &regPanel{
		Standing:    badge{Class: "prov prov-" + slug, Text: text},
		Reason:      r.Reason,
		Message:     r.Message,
		CatalogText: strings.TrimSpace(r.Catalog + " " + r.Version),
		CatalogHref: catalogHref(r.Catalog),
		Role:        regRole,
	}
	if slug == "inactive" {
		out.ActiveBy = firstOf(r.ActiveMessage, r.ActiveReason)
	}
	if root := findNode(g, g.Root); root != nil {
		out.Provider = qualified(root)
		switch {
		case r.Verdict == verdictAccepted:
			out.ProviderRef = "spec.providerRef names it: the controller accepted the claim."
		case r.Verdict == verdictRefused && r.Reason == "ProviderMismatch":
			out.ProviderRef = "spec.providerRef does not name it: the controller refused the claim."
		default:
			out.ProviderRef = "The controller has not judged spec.providerRef yet."
		}
	}
	return out
}

// sourcePanel is a package source's panel: the Ready condition's reason and
// message and the artifact revision, cut when the read API served them.
func sourcePanel(s *v1.SourceState) *srcPanel {
	out := &srcPanel{Status: sourceStatus(s), Reason: s.Reason, Message: s.Message, Revision: s.Revision}
	switch s.State {
	case "kindNotServed":
		out.Sentence = srcNotServed
	case "unsupportedKind":
		out.Sentence = srcUnsupport
	case "noArtifact":
		out.Sentence = "The source is ready but has published no artifact yet."
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

// collapseLink is the Graph tab with group id removed from the expanded
// groups and selected.
func collapseLink(r *http.Request, id string, ctx panelContext) string {
	q := url.Values{"tab": {tabGraph}, "node": {id}}
	for _, e := range r.URL.Query()["expand"] {
		if e != id {
			q.Add("expand", e)
		}
	}
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
