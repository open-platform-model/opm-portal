package ui

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// How many characters of a line fit a node: the name, the kind line beside
// the applied square, the status beside its dot, and a message on a card.
const (
	maxLabel   = 26
	maxKind    = 24
	maxStatus  = 30
	maxMessage = 120
)

// Where the three lines of a node sit below its top edge, and the gaps of
// a band's frame (design.md "Drawing nodes", "Group nodes, frames and the
// accordion").
const (
	kindLineY   = 20
	nameLineY   = 42
	statusLineY = 60
	frameLeft   = 12
	frameTop    = 48
	frameBottom = 24
	ghostWidth  = 30
	ghostLineH  = 16
	nodeGap     = 24
	graphMargin = 16
)

// svgGraph is a laid-out graph ready for the SVG template.
type svgGraph struct {
	ID      string
	RootID  string
	Width   int
	Height  int
	Columns []v1.GraphColumn
	Nodes   []svgNode
	Edges   []svgEdge
	// Frames surround the expanded configuration groups; Ghost is the box
	// that stands for what has not been applied yet.
	Frames []svgFrame
	Ghost  *svgGhost
	// Source: the graph draws a source node, so its legend names the third
	// origin.
	Source bool
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
	// Kind is the first line, Label the second and Status the third.
	Kind   string
	Label  string
	Status string
	// Mono: the label is a name from the cluster, set in the mono face.
	Mono bool
	// Full is the whole label, shown as the node's title.
	Full     string
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
	// TextX, KindY, NameY and StatusY place the lines; DotX and DotY the
	// status dot.
	TextX, KindY, NameY, StatusY int
	DotX, DotY, StatusX          int
}

type svgEdge struct {
	ID    string
	From  string
	To    string
	Class string
	D     string
	Title string
}

// svgFrame is the frame of an expanded configuration group: a rectangle
// from the components column to the last column, across the group's band.
type svgFrame struct {
	Group  string
	X, Y   int
	W, H   int
	Header string
	// HeaderX and HeaderY place the header text; Collapse is the link that
	// folds the group, drawn as a pill at PillX, PillY.
	HeaderX, HeaderY int
	Collapse         string
	PillX, PillY     int
	PillTextX        int
	PillTextY        int
	PillLabel        string
}

// svgGhost is the dashed box and edge of an owner that has applied
// nothing.
type svgGhost struct {
	X, Y, W, H int
	Lines      []svgLine
	D          string
}

// svgLine is one line of text, placed.
type svgLine struct {
	X, Y int
	Text string
}

// nodeCard is a node's hover card: only what the graph document carries.
type nodeCard struct {
	ID      string
	Kind    string
	Name    string
	Health  *badge
	Applied *badge
	// Standing is a registration's standing; Catalog its catalog line.
	Standing *badge
	Catalog  string
	Reason   string
	Message  string
	Origin   string
	Fact     string
	Access   string
	Hint     string
}

// originWords say where a node's object comes from, in the canvas's words.
var originWords = map[string]string{
	"object":    "Applied by the controller",
	"runtime":   "Made by the cluster below an inventory object",
	"instance":  "Reconciled by the controller",
	"package":   "Reconciled by the controller",
	"module":    "The module it was rendered from",
	"source":    "Read by the controller, owned by Flux",
	"component": "Part of the module",
	"group":     "Display group",
}

// kindText names node kinds for people.
var kindText = map[string]string{
	"instance":  "Instance",
	"package":   "Package",
	"module":    "Module",
	"source":    "Source",
	"component": "Component",
	"object":    "Object",
	"runtime":   "Runtime",
	"group":     "Group",
}

// familyText names a configuration group's family. A family this version
// does not know reads "Configuration".
var familyText = map[string]string{
	"crds":     "CRDs",
	"rbac":     "RBAC",
	"webhooks": "Webhook configuration",
	"other":    "Other configuration",
}

// The node kinds and group kinds the drawing branches on.
const (
	kindSource         = "source"
	kindRuntime        = "runtime"
	groupConfiguration = "configuration"
	configGroupPrefix  = "grp:configuration/"
)

// drawing is what the nodes of one graph share while they are drawn.
type drawing struct {
	g          *v1.Graph
	id         string
	page       string
	panel      string
	expanded   []string
	byID       map[string]*v1.GraphNode
	objects    map[string]int
	controller map[string]string
	dropped    map[string]bool
	// skip is how many leading columns the dropped module node leaves
	// empty; dx is their width.
	skip int
	dx   int
}

// buildGraph turns a graph document into its SVG model. page is the page
// the graph is on: a node links to page with ?node=<id> (its panel without
// script) and names its panel fragment at panel?id=<id>. selected is the
// node whose panel the page shows, if any.
func buildGraph(g v1.Graph, id, page, panel, selected string, expanded ...string) svgGraph {
	d := newDrawing(&g, id, page, panel, expanded)
	out := svgGraph{
		ID:      id,
		RootID:  g.Root,
		Width:   g.Layout.Width - d.dx,
		Height:  g.Layout.Height,
		Columns: d.columns(),
		Empty:   len(g.Nodes) == 0,
	}
	// A node is next to the selected one when an edge joins them.
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
		if d.dropped[gn.ID] {
			continue
		}
		n := d.nodeOf(gn)
		n.Selected = n.ID == selected
		n.Dim = spot && !near[n.ID]
		n.MemberOf = gn.MemberOf
		n.Card.ID = id + "-card-" + strconv.Itoa(len(out.Nodes))
		d.groupLink(&n, gn)
		out.Source = out.Source || gn.Kind == kindSource
		locked[n.ID] = n.Locked
		out.Nodes = append(out.Nodes, n)
	}
	out.Edges = d.edgesOf(locked, selected, spot)
	out.Frames = d.frames(out.Columns)
	return out
}

// edgesOf draws the edges that remain; with a node spotlit, the edges that
// do not touch it fade.
func (d *drawing) edgesOf(locked map[string]bool, selected string, spot bool) []svgEdge {
	var out []svgEdge
	for i := range d.g.Edges {
		if d.dropped[d.g.Edges[i].From] || d.dropped[d.g.Edges[i].To] {
			continue
		}
		if e, ok := edgeOf(&d.g.Edges[i], locked, d.dx); ok {
			if spot && e.From != selected && e.To != selected {
				e.Class += " dim"
			}
			out = append(out, e)
		}
	}
	return out
}

// groupLink makes a closed group's node open the group: its link adds the
// group to the expanded ones and asks the view to fit it.
func (d *drawing) groupLink(n *svgNode, gn *v1.GraphNode) {
	if gn.Group == nil || slices.Contains(d.expanded, gn.ID) {
		return
	}
	n.Group = true
	q := url.Values{"tab": {"graph"}}
	for _, e := range d.expanded {
		q.Add("expand", e)
	}
	q.Add("expand", gn.ID)
	q.Set("fit", gn.ID)
	n.Href = d.page + "?" + q.Encode()
}

// newDrawing indexes a graph document for drawing. The instance page draws
// no module node: the nodes of kind module and their edges are dropped,
// and when the first column then holds nothing every x moves left by one
// column pitch (instance-17).
func newDrawing(g *v1.Graph, id, page, panel string, expanded []string) *drawing {
	d := &drawing{
		g: g, id: id, page: page, panel: panel, expanded: expanded,
		byID:       make(map[string]*v1.GraphNode, len(g.Nodes)),
		objects:    map[string]int{},
		controller: map[string]string{},
		dropped:    map[string]bool{},
	}
	first := false
	for i := range g.Nodes {
		n := &g.Nodes[i]
		d.byID[n.ID] = n
		if n.Kind == "module" {
			d.dropped[n.ID] = true
		} else if n.Column == 0 {
			first = true
		}
	}
	for i := range g.Edges {
		switch e := &g.Edges[i]; e.Kind {
		case "owns":
			d.objects[e.From]++
		case "controls":
			d.controller[e.To] = e.From
		}
	}
	if len(d.dropped) > 0 && !first && len(g.Layout.Columns) > 1 {
		d.dx = g.Layout.Columns[1].X - g.Layout.Columns[0].X
		d.skip = 1
	}
	return d
}

// columns are the column titles, moved with the nodes.
func (d *drawing) columns() []v1.GraphColumn {
	cols := d.g.Layout.Columns[min(d.skip, len(d.g.Layout.Columns)):]
	out := make([]v1.GraphColumn, len(cols))
	for i, c := range cols {
		out[i] = v1.GraphColumn{Title: c.Title, X: c.X - d.dx}
	}
	return out
}

// nodeOf draws one node as three lines: its kind, its name and its status.
// Its fill and outline carry health (or a registration's standing); its
// applied state, when it has one, is a separate square stamp, so neither
// axis is drawn through the other (portal:D3:R1).
func (d *drawing) nodeOf(n *v1.GraphNode) svgNode {
	state := ""
	if n.Health != nil {
		state = n.Health.State
	}
	locked := n.Access != "" && n.Access != v1.AccessOK
	classes := []string{"node", "kind-" + slugOrUnknown(n.Kind, kindText), toneOf(n, locked, state)}
	if n.Reconcile != nil {
		classes = append(classes, "ap-"+strings.TrimPrefix(strings.Fields(appliedBadge(*n.Reconcile).Class)[1], "applied-"))
	}
	word := kindText[n.Kind]
	if word == "" {
		word = "Unknown kind"
	}
	status := d.status(n, locked)
	x, w := n.X-d.dx, d.g.Layout.NodeWidth
	return svgNode{
		ID:      n.ID,
		X:       x,
		Y:       n.Y,
		W:       w,
		H:       d.g.Layout.NodeHeight,
		Class:   strings.Join(slices.DeleteFunc(classes, func(c string) bool { return c == "" }), " "),
		Kind:    clipMiddle(kindLine(n, word), maxKind),
		Label:   clipMiddle(nameLine(n), maxLabel),
		Status:  clipEnd(status, maxStatus),
		Mono:    n.Group == nil && (n.Kind == "object" || n.Kind == kindRuntime || n.Kind == kindSource),
		Full:    n.Label,
		Aria:    nodeAria(n, word, status),
		Href:    d.page + "?" + url.Values{"tab": {"graph"}, "focus": {n.ID}}.Encode(),
		Card:    d.cardOf(n, word, locked, state),
		Panel:   d.panel + "?" + url.Values{"id": {n.ID}}.Encode(),
		Locked:  locked,
		Applied: n.Reconcile != nil && !locked,
		StampX:  x + w - 16,
		StampY:  n.Y + 6,
		TextX:   x + 12,
		KindY:   n.Y + kindLineY,
		NameY:   n.Y + nameLineY,
		StatusY: n.Y + statusLineY,
		DotX:    x + 16,
		DotY:    n.Y + statusLineY - 3,
		StatusX: x + 25,
	}
}

// toneOf is the class that tones a node's fill and outline: locked, a
// registration's standing, missing, or its health. One exception to
// health, recorded under portal:D3: a registration node carries the
// claim's standing, because the standing is what the node is for.
func toneOf(n *v1.GraphNode, locked bool, state string) string {
	switch {
	case locked:
		return "locked"
	case n.Registration != nil:
		slug, _ := standingOf(n.Registration)
		return "standing-" + slug
	case n.Missing:
		return "health-missing"
	case state != "":
		return strings.Fields(stateBadge(state).Class)[1]
	}
	return ""
}

// standingOf is a registration's standing as a class suffix and a status
// line: what the controller wrote, never one the portal computed.
func standingOf(r *v1.GraphRegistration) (slug, text string) {
	switch r.Verdict {
	case verdictRemovalBlocked:
		return "blocked", "Removal blocked"
	case verdictAccepted:
		if r.Active {
			return "active", "Accepted · active"
		}
		return "inactive", "Accepted · not active"
	case verdictRefused:
		return "refused", firstOf(r.Reason, "Refused")
	case verdictPending:
		return "pending", firstOf(r.Reason, "Pending")
	}
	return unknownWord, stateUnknown
}

func firstOf(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// kindLine is a node's first line: its object kind, or what a group stands
// for.
func kindLine(n *v1.GraphNode, word string) string {
	switch {
	case n.Group != nil && n.Group.Kind == groupConfiguration:
		return fmt.Sprintf("Group · %d %s", len(n.Group.Members), plural(len(n.Group.Members), "component", "components"))
	case n.Group != nil && n.Group.Kind == "pods":
		return "Group · Pods"
	case n.Ref != nil && n.Kind != instanceKind.Topic && n.Kind != packageKind.Topic:
		return n.Ref.Kind
	}
	return word
}

// nameLine is a node's second line: its name, or a configuration group's
// family.
func nameLine(n *v1.GraphNode) string {
	if n.Group != nil && n.Group.Kind == groupConfiguration {
		return firstOf(familyText[n.Group.Family], "Configuration")
	}
	if n.Kind == kindSource && n.Ref != nil && n.Ref.Name != "" {
		return n.Ref.Name
	}
	return n.Label
}

// status is a node's third line, first match wins (design.md "Drawing
// nodes"). A source's own states and a registration's standing come before
// "Missing": a source whose kind is not installed is missing, and says why,
// and a registration the portal read has a standing, which is what its node
// is for.
func (d *drawing) status(n *v1.GraphNode, locked bool) string {
	switch {
	case locked:
		return "Locked"
	case n.Kind == kindSource && n.SourceState != nil:
		return sourceStatus(n.SourceState)
	case n.Registration != nil:
		_, text := standingOf(n.Registration)
		return text
	case n.Missing:
		return "Missing"
	case n.Group != nil:
		return groupStatus(n)
	case n.Kind == kindRuntime && n.Ref != nil && n.Ref.Kind == "ReplicaSet" && n.Replicas != nil:
		return replicaStatus(n)
	case n.Kind == "component":
		c := d.objects[n.ID]
		return fmt.Sprintf("%d %s", c, plural(c, "object", "objects"))
	}
	return healthStatus(n)
}

func sourceStatus(s *v1.SourceState) string {
	switch s.State {
	case "ready":
		if s.Revision != "" {
			return "Artifact ready · " + shortRevision(s.Revision)
		}
		return "Artifact ready"
	case "notReady":
		return firstOf(s.Reason, "Not ready")
	case "noArtifact":
		return "No artifact yet"
	case "kindNotServed":
		return "Kind not installed"
	case "notFound":
		return "Not found"
	case "unsupportedKind":
		return "Not read by the portal"
	}
	return stateUnknown
}

// shortRevision keeps the start of a digest ("sha256:0123456789ab" reads
// "0123456") and any other revision as it is, clipped.
func shortRevision(rev string) string {
	if i := strings.LastIndex(rev, ":"); i >= 0 {
		return string([]rune(rev[i+1:])[:min(7, len([]rune(rev[i+1:])))])
	}
	return clipMiddle(rev, 12)
}

func groupStatus(n *v1.GraphNode) string {
	switch n.Group.Kind {
	case groupConfiguration:
		objects := 0
		for _, k := range n.Group.ObjectKinds {
			objects += k.Count
		}
		return fmt.Sprintf("%d %s: %d %s", objects, plural(objects, "object", "objects"),
			len(n.Group.ObjectKinds), plural(len(n.Group.ObjectKinds), "kind", "kinds"))
	case "pods":
		pods := len(n.Group.Members)
		s := fmt.Sprintf("%d %s", pods, plural(pods, "Pod", "Pods"))
		if n.Health != nil {
			s += " · " + stateBadge(n.Health.State).Text
		}
		return s
	}
	return healthStatus(n)
}

func replicaStatus(n *v1.GraphNode) string {
	s := fmt.Sprintf("%d desired", *n.Replicas)
	if n.Health != nil && n.Health.State != "Healthy" && n.Health.Reason != "" {
		s += " · " + n.Health.Reason
	}
	return s
}

// healthStatus is the health reason, or else the health word.
func healthStatus(n *v1.GraphNode) string {
	switch {
	case n.Health != nil && n.Health.Reason != "":
		return n.Health.Reason
	case n.Health != nil:
		return stateBadge(n.Health.State).Text
	case n.Reconcile != nil:
		return appliedBadge(*n.Reconcile).Text
	}
	return "Not read"
}

// cardOf is what a node's hover card says: only what the graph document
// carries for it.
func (d *drawing) cardOf(n *v1.GraphNode, word string, locked bool, state string) nodeCard {
	card := nodeCard{Kind: word, Name: qualified(n), Origin: d.originOf(n), Hint: "Click for details"}
	if n.Group != nil && !slices.Contains(d.expanded, n.ID) {
		card.Hint = "Click to expand"
	}
	if n.Ref != nil && n.Kind != instanceKind.Topic && n.Kind != packageKind.Topic {
		card.Kind = n.Ref.Kind
	}
	if locked {
		card.Access = accessText(n.Access)
		return card
	}
	switch {
	case n.Kind == kindSource && n.SourceState != nil:
		card.Reason = sourceStatus(n.SourceState)
	case n.Missing:
		b := badge{Class: "health health-missing", Text: "Missing"}
		card.Health = &b
	case state != "":
		b := stateBadge(state)
		card.Health = &b
	}
	if n.Health != nil {
		card.Reason = firstOf(n.Health.Reason, card.Reason)
		card.Message = clipEnd(n.Health.Message, maxMessage)
	}
	if n.Reconcile != nil {
		b := appliedBadge(*n.Reconcile)
		card.Applied = &b
	}
	if n.Replicas != nil {
		card.Fact = "Replicas: " + strconv.FormatInt(*n.Replicas, 10)
	}
	d.registrationCard(&card, n.Registration)
	return card
}

// registrationCard adds a registration's standing, and the catalog it
// claims and the owner that provides it.
func (d *drawing) registrationCard(card *nodeCard, r *v1.GraphRegistration) {
	if r == nil {
		return
	}
	slug, text := standingOf(r)
	card.Standing = &badge{Class: "prov prov-" + slug, Text: text}
	card.Catalog = "Catalog: " + strings.TrimSpace(r.Catalog+" "+r.Version)
	if root := d.byID[d.g.Root]; root != nil {
		card.Catalog += " · Provider: " + root.Label
	}
}

// qualified is a node's name as namespace/name.
func qualified(n *v1.GraphNode) string {
	if n.Ref != nil && n.Ref.Name != "" && n.Group == nil {
		if n.Ref.Namespace != "" {
			return n.Ref.Namespace + "/" + n.Ref.Name
		}
		return n.Ref.Name
	}
	return n.Label
}

// originOf says where a node comes from; a runtime child names the object
// that controls it.
func (d *drawing) originOf(n *v1.GraphNode) string {
	if n.Kind == kindRuntime {
		if p := d.byID[d.controller[n.ID]]; p != nil && p.Ref != nil {
			return "Created by the cluster from " + p.Ref.Kind + " " + p.Label
		}
	}
	return originWords[n.Kind]
}

// frames draws a frame around each expanded configuration group: from the
// left edge of the components column to the right edge of the last column,
// across the band's rows, 48 px above its first node for the header and
// 24 px below its last. The group node is gone while it is open, so the
// family, the member count and the object count come from its members.
func (d *drawing) frames(cols []v1.GraphColumn) []svgFrame {
	var out []svgFrame
	seen := map[string]bool{}
	for i := range d.g.Nodes {
		id := d.g.Nodes[i].MemberOf
		if id == "" || seen[id] || !strings.HasPrefix(id, configGroupPrefix) {
			continue
		}
		seen[id] = true
		out = append(out, d.frameOf(id, cols))
	}
	return out
}

func (d *drawing) frameOf(id string, cols []v1.GraphColumn) svgFrame {
	left, right, top, bottom, members, objects := -1, 0, -1, 0, 0, 0
	extend := func(n *v1.GraphNode) {
		x := n.X - d.dx
		if left < 0 || x < left {
			left = x
		}
		if top < 0 || n.Y < top {
			top = n.Y
		}
		bottom = max(bottom, n.Y+d.g.Layout.NodeHeight)
		right = max(right, x+d.g.Layout.NodeWidth)
	}
	for i := range d.g.Nodes {
		if n := &d.g.Nodes[i]; n.MemberOf == id {
			members++
			extend(n)
		}
	}
	for i := range d.g.Edges {
		e := &d.g.Edges[i]
		if e.Kind != "owns" || d.byID[e.From] == nil || d.byID[e.From].MemberOf != id {
			continue
		}
		if o := d.byID[e.To]; o != nil {
			objects++
			extend(o)
		}
	}
	if len(cols) > 0 {
		right = max(right, cols[len(cols)-1].X+d.g.Layout.NodeWidth)
	}
	family := id[strings.LastIndex(id, "/")+1:]
	label := firstOf(familyText[family], "Configuration")
	f := svgFrame{
		Group:     id,
		X:         left - frameLeft,
		Y:         top - frameTop,
		W:         right - left + 2*frameLeft,
		H:         bottom - top + frameTop + frameBottom,
		Header:    fmt.Sprintf("%s · %d %s · %d %s", label, members, plural(members, "component", "components"), objects, plural(objects, "object", "objects")),
		PillLabel: "Collapse " + label,
	}
	f.HeaderX, f.HeaderY = f.X+16, f.Y+30
	f.PillX, f.PillY = f.X+f.W-12-collapseWidth, f.Y+12
	f.PillTextX, f.PillTextY = f.PillX+34, f.PillY+16
	q := url.Values{"tab": {"graph"}, "node": {id}}
	for _, e := range d.expanded {
		if e != id {
			q.Add("expand", e)
		}
	}
	f.Collapse = d.page + "?" + q.Encode()
	return f
}

// collapseWidth is the width of the Collapse pill.
const collapseWidth = 104

// addGhost adds the box of an owner that has applied nothing: one column
// right of the root, joined to it by a dashed edge. It says what happens
// next, and for a package that never fetched its source, why nothing can.
func (o *svgGraph) addGhost(neverFetched bool) {
	var root *svgNode
	for i := range o.Nodes {
		if o.Nodes[i].ID == o.RootID {
			root = &o.Nodes[i]
		}
	}
	if root == nil {
		return
	}
	pitch := root.W + 48
	if len(o.Columns) > 1 {
		pitch = o.Columns[1].X - o.Columns[0].X
	}
	gh := &svgGhost{X: root.X + pitch, Y: root.Y, W: root.W}
	// A package draws its source and dependencies in that column: the box
	// goes below them.
	for i := range o.Nodes {
		if n := &o.Nodes[i]; n.X == gh.X {
			gh.Y = max(gh.Y, n.Y+n.H+nodeGap)
		}
	}
	text := wrapText("Nothing applied yet. The graph fills in once an apply succeeds.", ghostWidth)
	if neverFetched {
		text = append(text, wrapText("The package has never fetched its source.", ghostWidth)...)
	}
	gh.H = len(text)*ghostLineH + 24
	for i, line := range text {
		gh.Lines = append(gh.Lines, svgLine{X: gh.X + 12, Y: gh.Y + 24 + i*ghostLineH, Text: line})
	}
	x0, y0 := root.X+root.W, root.Y+root.H/2
	gh.D = fmt.Sprintf("M%d %d H%d V%d H%d", x0, y0, (x0+gh.X)/2, gh.Y+gh.H/2, gh.X)
	o.Ghost = gh
	o.Width = max(o.Width, gh.X+gh.W+graphMargin)
	o.Height = max(o.Height, gh.Y+gh.H+graphMargin)
}

// wrapText breaks s into lines of at most width characters at word
// boundaries: SVG text does not wrap, so the server does it.
func wrapText(s string, width int) []string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(s) {
		switch {
		case line == "":
			line = w
		case len([]rune(line))+1+len([]rune(w)) <= width:
			line += " " + w
		default:
			lines = append(lines, line)
			line = w
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

// edgeOf draws one edge as an elbow through the route's four points,
// which are the elbow's corners: M p0 H p1.x V p3.y H p3.x, stroke by
// class, no marker. An edge a second source does not confirm is drawn as
// broken only when the portal could read both ends; one whose end is
// locked, or whose provider the portal could not read, is drawn as
// unconfirmed in the locked style, because "cannot see" is not "broken"
// (portal:D5:R7).
func edgeOf(e *v1.GraphEdge, locked map[string]bool, dx int) (svgEdge, bool) {
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
		D:     fmt.Sprintf("M%d %d H%d V%d H%d", p[0].X-dx, p[0].Y, p[1].X-dx, p[3].Y, p[3].X-dx),
		Title: title,
	}, true
}

// nodeAria is a node's accessible name: kind, name and status.
func nodeAria(n *v1.GraphNode, kind, status string) string {
	parts := []string{kind, n.Label, status}
	if n.Reconcile != nil && (n.Access == "" || n.Access == v1.AccessOK) {
		parts = append(parts, appliedBadge(*n.Reconcile).Text)
	}
	return strings.Join(parts, ", ")
}

// clipEnd shortens s to n characters, keeping its head.
func clipEnd(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func slugOrUnknown(v string, table map[string]string) string {
	if _, ok := table[v]; ok {
		return slug(v)
	}
	return unknownWord
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

// findNode returns the node with id, or nil.
func findNode(g *v1.Graph, id string) *v1.GraphNode {
	for i := range g.Nodes {
		if g.Nodes[i].ID == id {
			return &g.Nodes[i]
		}
	}
	return nil
}
