package ui

import (
	"fmt"
	"net/url"
	"strings"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// maxLabel is how many characters of a label fit a node's width.
const maxLabel = 26

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
}

type svgNode struct {
	ID     string
	X, Y   int
	W, H   int
	Class  string
	Label  string
	Kind   string
	Sub    string
	Aria   string
	Href   string
	Panel  string
	Locked bool
	// TextX and TextY place the label; SubY the kind line.
	TextX, TextY, SubY int
}

type svgEdge struct {
	ID    string
	Class string
	D     string
	Title string
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
// script) and names its panel fragment at panel?id=<id>.
func buildGraph(g v1.Graph, id, page, panel string) svgGraph {
	out := svgGraph{
		ID:      id,
		Width:   g.Layout.Width,
		Height:  g.Layout.Height,
		Columns: g.Layout.Columns,
		Empty:   len(g.Nodes) == 0,
	}
	for i := range g.Nodes {
		out.Nodes = append(out.Nodes, nodeOf(&g.Nodes[i], g.Layout.NodeWidth, g.Layout.NodeHeight, page, panel))
	}
	for i := range g.Edges {
		if e, ok := edgeOf(&g.Edges[i]); ok {
			out.Edges = append(out.Edges, e)
		}
	}
	return out
}

// nodeOf draws one node: its classes say kind, access, health and applied
// state; its label and kind line are clipped to the box.
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
		classes = append(classes, strings.Fields(appliedBadge(*n.Reconcile).Class)[1])
	}
	kind := kindText[n.Kind]
	if kind == "" {
		kind = "Unknown kind"
	}
	return svgNode{
		ID:     n.ID,
		X:      n.X,
		Y:      n.Y,
		W:      w,
		H:      hgt,
		Class:  strings.Join(classes, " "),
		Label:  clip(n.Label, maxLabel),
		Kind:   kind,
		Sub:    clip(subLine(n, kind, locked), maxLabel+4),
		Aria:   nodeAria(n, kind, locked, state),
		Href:   page + "?" + url.Values{"node": {n.ID}}.Encode(),
		Panel:  panel + "?" + url.Values{"id": {n.ID}}.Encode(),
		Locked: locked,
		TextX:  n.X + 12,
		TextY:  n.Y + 19,
		SubY:   n.Y + 34,
	}
}

// subLine is the small line under a node's label: its object kind, or
// what a group stands for, marked when locked.
func subLine(n *v1.GraphNode, kind string, locked bool) string {
	sub := kind
	if n.Ref != nil && n.Kind != "instance" && n.Kind != "package" {
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

// edgeOf draws one edge from its four-point cubic route.
func edgeOf(e *v1.GraphEdge) (svgEdge, bool) {
	if len(e.Route) != 4 {
		return svgEdge{}, false
	}
	p := e.Route
	class := "edge edge-" + e.Kind
	title := e.Kind + ", from " + e.Source
	if e.Verified != nil && !*e.Verified {
		class += " unverified"
		title += "; not confirmed: " + e.Reason
	}
	return svgEdge{
		ID:    e.ID,
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

// clip shortens s to n characters with an ellipsis.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
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
