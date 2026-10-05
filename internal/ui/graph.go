package ui

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// maxLabel is how many characters of a label fit a node's width;
// stampedLabel is how many fit beside the applied stamp, which takes the
// node's top-right corner.
const (
	maxLabel     = 24
	stampedLabel = maxLabel - 3
)

// svgGraph is a laid-out graph ready for the SVG template.
type svgGraph struct {
	ID      string
	Width   int
	Height  int
	Columns []v1.GraphColumn
	Nodes   []svgNode
	Edges   []svgEdge
	// Empty: the graph has no nodes.
	Empty bool
	// FitGroup is the group whose members the view fits to on load, after
	// it was expanded.
	FitGroup string
	// Focused: a node is focused; Clear is the graph without the focus and
	// Whole the graph with no group expanded, when one is.
	Focused bool
	Clear   string
	Whole   string
}

type svgNode struct {
	ID    string
	X, Y  int
	W, H  int
	Class string
	Label string
	// Full is the whole label, shown as the node's title.
	Full     string
	Kind     string
	Sub      string
	Aria     string
	Href     string
	Panel    string
	Locked   bool
	Selected bool
	// Dim: a node is focused and this one is not next to it.
	Dim bool
	// MemberOf is the group an expanded member belongs to; Group marks a
	// group node, which expands on activation.
	MemberOf string
	Group    bool
	// Card is what the node's hover and focus card says.
	Card nodeCard
	// Applied is set when the node has an applied state; StampX and
	// StampY place its square mark in the node's top-right corner.
	Applied        bool
	StampX, StampY int
	// TextX and TextY place the label; SubY the kind line.
	TextX, TextY, SubY int
}

type svgEdge struct {
	ID    string
	From  string
	To    string
	Class string
	D     string
	Title string
}

// nodeCard is a node's hover card: only what the graph document carries.
type nodeCard struct {
	ID      string
	Kind    string
	Name    string
	Health  *badge
	Applied *badge
	Reason  string
	Origin  string
	Access  string
}

// originText says where a node's object comes from.
var originText = map[string]string{
	"object":    "In the inventory: applied by the controller",
	"runtime":   "Made by the cluster below an inventory object",
	"instance":  "The ModuleInstance",
	"package":   "The ModulePackage",
	"module":    "The module it was rendered from",
	"source":    "The source the package reads",
	"component": "A component of the module",
	"group":     "A group of nodes folded into one",
}

// kindText names node kinds for people.
var kindText = map[string]string{
	"platform":     "Platform",
	"catalog":      "Catalog",
	"registration": "Registration",
	"instance":     "Instance",
	"package":      "Package",
	"module":       "Module",
	"source":       "Source",
	"component":    "Component",
	"object":       "Object",
	"runtime":      "Runtime",
	"group":        "Group",
}

// buildGraph turns a graph document into its SVG model. page is the page
// the graph is on: a node links to page with ?node=<id> (its panel without
// script) and names its panel fragment at panel?id=<id>. selected is the
// node whose panel the page shows, if any.
func buildGraph(g v1.Graph, id, page, panel, selected string, expanded ...string) svgGraph {
	out := svgGraph{
		ID:      id,
		Width:   g.Layout.Width,
		Height:  g.Layout.Height,
		Columns: g.Layout.Columns,
		Empty:   len(g.Nodes) == 0,
	}
	near := map[string]bool{selected: true}
	for i := range g.Edges {
		if e := &g.Edges[i]; e.From == selected || e.To == selected {
			near[e.From], near[e.To] = true, true
		}
	}
	spot := selected != "" && findNode(&g, selected) != nil
	locked := map[string]bool{}
	for i := range g.Nodes {
		gn := &g.Nodes[i]
		n := nodeOf(gn, g.Layout.NodeWidth, g.Layout.NodeHeight, page, panel)
		n.Selected = n.ID == selected
		n.Dim = spot && !near[n.ID]
		n.MemberOf = gn.MemberOf
		n.Card.ID = id + "-card-" + strconv.Itoa(i)
		if gn.Group != nil && !slices.Contains(expanded, gn.ID) {
			n.Group = true
			q := url.Values{"tab": {"graph"}}
			for _, e := range expanded {
				q.Add("expand", e)
			}
			q.Add("expand", gn.ID)
			q.Set("fit", gn.ID)
			n.Href = page + "?" + q.Encode()
		}
		locked[n.ID] = n.Locked
		out.Nodes = append(out.Nodes, n)
	}
	for i := range g.Edges {
		if e, ok := edgeOf(&g.Edges[i], locked); ok {
			if spot && e.From != selected && e.To != selected {
				e.Class += " dim"
			}
			out.Edges = append(out.Edges, e)
		}
	}
	return out
}

// nodeOf draws one node. Its box outline and rail carry health; its applied
// state, when it has one, is a separate square stamp, so neither axis is
// drawn through the other (portal:D3:R1). Its label keeps the part that
// tells nodes apart; the whole label is its title.
func nodeOf(n *v1.GraphNode, w, hgt int, page, panel string) svgNode {
	state := ""
	if n.Health != nil {
		state = n.Health.State
	}
	locked := n.Access != "" && n.Access != v1.AccessOK
	classes := []string{"node", "kind-" + slugOrUnknown(n.Kind, kindText)}
	switch {
	case locked:
		classes = append(classes, "locked")
	case n.Missing:
		classes = append(classes, "health-missing")
	case state != "":
		classes = append(classes, strings.Fields(stateBadge(state).Class)[1])
	}
	if n.Reconcile != nil {
		classes = append(classes, "ap-"+strings.TrimPrefix(strings.Fields(appliedBadge(*n.Reconcile).Class)[1], "applied-"))
	}
	kind := kindText[n.Kind]
	if kind == "" {
		kind = "Unknown kind"
	}
	label, sub := displayLabel(n, kind, locked)
	card := cardOf(n, kind, locked, state)
	applied := n.Reconcile != nil && !locked
	fit := maxLabel
	if applied {
		fit = stampedLabel
	}
	return svgNode{
		ID:      n.ID,
		X:       n.X,
		Y:       n.Y,
		W:       w,
		H:       hgt,
		Class:   strings.Join(classes, " "),
		Label:   clipMiddle(label, fit),
		Full:    n.Label,
		Kind:    kind,
		Sub:     clipMiddle(sub, maxLabel+4),
		Aria:    nodeAria(n, kind, locked, state),
		Href:    page + "?" + url.Values{"tab": {"graph"}, "focus": {n.ID}}.Encode(),
		Card:    card,
		Panel:   panel + "?" + url.Values{"id": {n.ID}}.Encode(),
		Locked:  locked,
		Applied: applied,
		StampX:  n.X + w - 16,
		StampY:  n.Y + 6,
		TextX:   n.X + 12,
		TextY:   n.Y + 19,
		SubY:    n.Y + 34,
	}
}

// cardOf is what a node's hover card says: only what the graph document
// carries for it.
func cardOf(n *v1.GraphNode, kind string, locked bool, state string) nodeCard {
	card := nodeCard{Kind: kind, Name: n.Label, Origin: originText[n.Kind]}
	if n.Ref != nil && n.Kind != instanceKind.Topic && n.Kind != packageKind.Topic {
		card.Kind = n.Ref.Kind
	}
	switch {
	case locked:
		card.Access = accessText(n.Access)
		return card
	case n.Missing:
		b := badge{Class: "health health-missing", Text: "Missing"}
		card.Health = &b
	case state != "":
		b := stateBadge(state)
		card.Health = &b
	}
	if n.Health != nil {
		card.Reason = n.Health.Reason
	}
	if n.Reconcile != nil {
		b := appliedBadge(*n.Reconcile)
		card.Applied = &b
	}
	return card
}

// displayLabel is what a node's two lines say. A catalog or module shows
// its last path segment and version on the first line and its path's host
// on the second, because the host is what tells opmodel.dev from
// testing.opmodel.dev; a configuration group says what it stands for in
// short.
func displayLabel(n *v1.GraphNode, kind string, locked bool) (label, sub string) {
	label, sub = n.Label, subLine(n, kind, locked)
	switch {
	case n.Kind == "catalog" || n.Kind == "module":
		version := n.Version
		if n.Catalog != nil && version == "" {
			version = n.Catalog.Version
		}
		if head, last, ok := cutLast(n.Label); ok {
			label, sub = last, kind+" · "+hostOf(head, maxLabel+4-len(kind)-3)
		}
		if version != "" {
			label += " " + version
		}
	case n.Group != nil && n.Group.Kind == "configuration":
		label = fmt.Sprintf("%d config components", len(n.Group.Members))
	}
	return label, sub
}

// cutLast splits a module or catalog path at its last slash.
func cutLast(path string) (head, last string, ok bool) {
	i := strings.LastIndex(path, "/")
	if i <= 0 || i == len(path)-1 {
		return "", path, false
	}
	return path[:i], path[i+1:], true
}

// subLine is the small line under a node's label: its object kind, or
// what a group stands for, marked when locked.
func subLine(n *v1.GraphNode, kind string, locked bool) string {
	sub := kind
	if n.Ref != nil && n.Kind != instanceKind.Topic && n.Kind != packageKind.Topic {
		sub = n.Ref.Kind
	}
	if n.Group != nil {
		sub = fmt.Sprintf("%s, %d", n.Group.Kind, len(n.Group.Members))
	}
	if locked {
		sub = "Locked · " + sub
	}
	return sub
}

// edgeOf draws one edge from its four-point cubic route. An edge a second
// source does not confirm is drawn as broken only when the portal could
// read both ends; one whose end is locked, or whose provider the portal
// could not read, is drawn as unconfirmed in the locked style, because
// "cannot see" is not "broken" (portal:D5:R7).
func edgeOf(e *v1.GraphEdge, locked map[string]bool) (svgEdge, bool) {
	if len(e.Route) != 4 {
		return svgEdge{}, false
	}
	p := e.Route
	class := "edge edge-" + e.Kind
	title := e.Kind + ", from " + e.Source
	if e.Verified != nil && !*e.Verified {
		if locked[e.From] || locked[e.To] || e.Reason == "ProviderUnreadable" || e.Reason == "ProviderNotLookedUp" {
			class += " unconfirmed"
			title += "; cannot be confirmed: " + e.Reason
		} else {
			class += " unverified"
			title += "; not confirmed: " + e.Reason
		}
	}
	return svgEdge{
		ID:    e.ID,
		From:  e.From,
		To:    e.To,
		Class: class,
		D:     fmt.Sprintf("M%d %d C%d %d, %d %d, %d %d", p[0].X, p[0].Y, p[1].X, p[1].Y, p[2].X, p[2].Y, p[3].X, p[3].Y),
		Title: title,
	}, true
}

func slugOrUnknown(v string, table map[string]string) string {
	if _, ok := table[v]; ok {
		return slug(v)
	}
	return "unknown"
}

// nodeAria is a node's accessible name: kind, name and state.
func nodeAria(n *v1.GraphNode, kind string, locked bool, state string) string {
	parts := []string{kind, n.Label}
	switch {
	case locked:
		parts = append(parts, "locked")
	case n.Missing:
		parts = append(parts, "missing")
	case state != "":
		parts = append(parts, "health "+stateBadge(state).Text)
	}
	if n.Reconcile != nil {
		parts = append(parts, appliedBadge(*n.Reconcile).Text)
	}
	return strings.Join(parts, ", ")
}

// clipMiddle shortens s to n characters, keeping its head and its longer
// tail around an ellipsis: names that share a prefix differ at the end.
func clipMiddle(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	head := (n - 1) * 2 / 5
	tail := n - 1 - head
	return string(r[:head]) + "…" + string(r[len(r)-tail:])
}

// hostOf is the first segment of a path head, marked "/…" when more
// follows, shortened to n characters at its end.
func hostOf(head string, n int) string {
	host, rest, more := strings.Cut(head, "/")
	if more && rest != "" {
		host += "/…"
	}
	r := []rune(host)
	if len(r) <= n || n < 2 {
		return host
	}
	return string(r[:n-1]) + "…"
}

// findNode returns the node with id, or nil.
func findNode(g *v1.Graph, id string) *v1.GraphNode {
	for i := range g.Nodes {
		if g.Nodes[i].ID == id {
			return &g.Nodes[i]
		}
	}
	return nil
}
