package graph

import (
	"slices"
	"sort"
	"strconv"

	"github.com/open-platform-model/opm-portal/internal/health"
	"github.com/open-platform-model/opm-portal/internal/readmodel"
)

// builder collects one graph's nodes and edges. The first node added under
// an id wins, as does the first edge.
type builder struct {
	scope   Scope
	root    string
	opts    Options
	titles  []string
	nodes   []Node
	byID    map[string]int
	edges   []Edge
	edgeIDs map[string]int
}

func newBuilder(scope Scope, titles []string, opts Options) *builder {
	if opts.NodeCap <= 0 {
		opts.NodeCap = DefaultNodeCap
	}
	return &builder{scope: scope, titles: titles, opts: opts, byID: map[string]int{}, edgeIDs: map[string]int{}}
}

// add adds n in column col, unless a node with its id is already there.
func (b *builder) add(n Node, col int) {
	if _, ok := b.byID[n.ID]; ok {
		return
	}
	n.Column = col
	b.byID[n.ID] = len(b.nodes)
	b.nodes = append(b.nodes, n)
}

// edge adds an edge of kind from one node to another, naming the kind's
// source.
func (b *builder) edge(kind EdgeKind, from, to string) *Edge {
	eid := string(kind) + "|" + from + "|" + to
	if i, ok := b.edgeIDs[eid]; ok {
		return &b.edges[i]
	}
	b.edgeIDs[eid] = len(b.edges)
	b.edges = append(b.edges, Edge{ID: eid, Kind: kind, From: from, To: to, Source: Sources[kind]})
	return &b.edges[len(b.edges)-1]
}

func (b *builder) expanded(id string) bool {
	return slices.Contains(b.opts.Expand, id)
}

// finish applies the node cap, lays the graph out and sorts it.
func (b *builder) finish() Graph {
	b.capNodes()
	g := Graph{Scope: b.scope, Root: b.root, Nodes: b.nodes, Edges: b.edges}
	sort.Slice(g.Edges, func(i, j int) bool { return g.Edges[i].ID < g.Edges[j].ID })
	layout(&g, b.titles)
	if g.Nodes == nil {
		g.Nodes = []Node{}
	}
	if g.Edges == nil {
		g.Edges = []Edge{}
	}
	return g
}

// capNodes drops nodes from the last column back, within a column from the
// end of the id order, until one fewer than the cap remain, and puts one
// summary node in their place. Edges to dropped nodes go with them.
func (b *builder) capNodes() {
	if len(b.nodes) <= b.opts.NodeCap {
		return
	}
	order := make([]int, len(b.nodes))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		x, y := &b.nodes[order[i]], &b.nodes[order[j]]
		if x.Column != y.Column {
			return x.Column > y.Column
		}
		return x.ID > y.ID
	})
	drop := map[string]bool{}
	counts := map[string]int{}
	col := 0
	for _, i := range order[:len(b.nodes)-(b.opts.NodeCap-1)] {
		n := &b.nodes[i]
		drop[n.ID] = true
		counts[hiddenKind(n)]++
		col = n.Column
	}
	kept := make([]Node, 0, b.opts.NodeCap)
	for i := range b.nodes {
		if !drop[b.nodes[i].ID] {
			kept = append(kept, b.nodes[i])
		}
	}
	edges := b.edges[:0]
	for i := range b.edges {
		if e := &b.edges[i]; !drop[e.From] && !drop[e.To] {
			edges = append(edges, *e)
		}
	}
	hidden := make([]KindCount, 0, len(counts))
	total := 0
	for k, c := range counts {
		hidden = append(hidden, KindCount{Kind: k, Count: c})
		total += c
	}
	sort.Slice(hidden, func(i, j int) bool { return hidden[i].Kind < hidden[j].Kind })
	kept = append(kept, Node{
		ID:     groupID(GroupMore, b.root),
		Kind:   KindGroup,
		Label:  plural(total, "more node", "more nodes"),
		Group:  &Group{Kind: GroupMore, Hidden: hidden},
		Column: col,
	})
	b.nodes, b.edges = kept, edges
	b.byID = map[string]int{}
	for i := range b.nodes {
		b.byID[b.nodes[i].ID] = i
	}
}

// hiddenKind names a dropped node in the summary: its object kind when it
// stands for an object, its node kind otherwise.
func hiddenKind(n *Node) string {
	if n.Ref != nil && (n.Kind == KindObject || n.Kind == KindRuntime) {
		return n.Ref.Kind
	}
	return string(n.Kind)
}

func refOf(r readmodel.ObjectRef) *Ref {
	return &Ref{Group: r.Group, Version: r.Version, Kind: r.Kind, Namespace: r.Namespace, Name: r.Name}
}

func appliedOf(a health.Applied) *Applied {
	return &Applied{State: a.State, Reason: a.Reason, Message: a.Message, Retrying: a.Retrying}
}

func summaryHealth(s health.Summary) *Health {
	return &Health{State: s.State, Partial: s.Partial, NotLive: !s.Live, Counts: countsOf(s.Counts)}
}

func objectHealth(h health.ObjectHealth) *Health {
	return &Health{State: h.State, Reason: h.Reason, Message: h.Message}
}

func countsOf(c health.Counts) *Counts {
	return &Counts{
		Healthy: c.Healthy, Progressing: c.Progressing, Degraded: c.Degraded, Missing: c.Missing,
		Unknown: c.Unknown, Forbidden: c.Forbidden, NotReadable: c.NotReadable, Withheld: c.Withheld,
	}
}

func (c *Counts) add(o *Counts) {
	c.Healthy += o.Healthy
	c.Progressing += o.Progressing
	c.Degraded += o.Degraded
	c.Missing += o.Missing
	c.Unknown += o.Unknown
	c.Forbidden += o.Forbidden
	c.NotReadable += o.NotReadable
	c.Withheld += o.Withheld
}

func (c *Counts) addState(s health.State) {
	switch s {
	case health.Healthy:
		c.Healthy++
	case health.Progressing:
		c.Progressing++
	case health.Degraded:
		c.Degraded++
	case health.Missing:
		c.Missing++
	case health.Unknown:
		c.Unknown++
	default:
		c.Unknown++
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}
