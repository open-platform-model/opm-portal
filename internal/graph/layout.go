package graph

import (
	"sort"
)

// Layout geometry, in user units. The elbow of an edge turns in the middle
// of the gutter, 24 units from either column.
const (
	nodeWidth  = 216
	nodeHeight = 72
	colPitch   = 264 // node width plus the gutter edges run in
	rowPitch   = 96
	pad        = 16
	header     = 28 // room for the column titles

	// A band is drawn with a frame that overhangs the columns it spans by
	// frameOverhang (less than half the gutter, so no elbow runs through
	// the frame's sides), reaches bandHeader above its first row's nodes
	// and bandFooter below its last row's. Together they fit the half-row
	// margin the layout leaves above and below a band.
	frameOverhang = 12
	bandHeader    = 48
	bandFooter    = 24
)

// layout places every node in its column and gives every edge a route.
// The node kind decides the column, so no layering algorithm is needed;
// within a column, nodes are ordered by the barycenter of their neighbors:
// the first column by id, each later one by its neighbors' rows in the
// column before (ties by id, none last), then one upward sweep by the
// neighbors' rows in the column after (ties by the current row), which
// moves only the nodes with such neighbors, among the rows they hold. The
// source and the dependencies of a package come first in the components
// column. A graph with an expanded configuration group is placed on one
// row grid with the group as a band (placeBands); any other graph centers
// each column on the tallest. Every step is deterministic, so the same
// graph always lays out the same way.
func layout(g *Graph, titles []string) {
	columns := compactColumns(g, titles)
	orderColumns(columns, g.Edges, g.Root)
	if bands := findBands(columns); len(bands) > 0 {
		placeBands(g, columns, bands)
	} else {
		place(g, columns)
	}
	at := make(map[string]*Node, len(g.Nodes))
	for i := range g.Nodes {
		at[g.Nodes[i].ID] = &g.Nodes[i]
	}
	for i := range g.Edges {
		g.Edges[i].Route = route(at[g.Edges[i].From], at[g.Edges[i].To])
	}
}

// compactColumns renumbers the columns that hold a node densely, so an
// empty column takes no space, and returns each column's nodes. A runtime
// column is titled from the kinds it holds.
func compactColumns(g *Graph, titles []string) [][]*Node {
	var used []int
	seen := map[int]bool{}
	for i := range g.Nodes {
		if c := g.Nodes[i].Column; !seen[c] {
			seen[c] = true
			used = append(used, c)
		}
	}
	sort.Ints(used)
	dense := make(map[int]int, len(used))
	g.Layout = Layout{NodeWidth: nodeWidth, NodeHeight: nodeHeight, Columns: make([]Column, len(used))}
	for k, c := range used {
		dense[c] = k
		title := ""
		if c < len(titles) {
			title = titles[c]
		}
		g.Layout.Columns[k] = Column{Title: title, X: pad + k*colPitch}
	}
	columns := make([][]*Node, len(used))
	for i := range g.Nodes {
		n := &g.Nodes[i]
		orig := n.Column
		n.Column = dense[orig]
		columns[n.Column] = append(columns[n.Column], n)
	}
	for k, c := range used {
		if c >= colRuntime {
			g.Layout.Columns[k].Title = runtimeTitle(columns[k])
		}
	}
	return columns
}

// runtimeTitle names a runtime column by the one kind its nodes are
// ("ReplicaSets", "Pods", "Jobs"), or "Runtime" when it holds more. A
// summary node of the node cap says nothing about the column.
func runtimeTitle(col []*Node) string {
	kinds := map[string]bool{}
	for _, n := range col {
		switch {
		case n.Kind == KindRuntime && n.Ref != nil:
			kinds[n.Ref.Kind] = true
		case n.Kind == KindGroup && n.Group != nil && n.Group.Kind == GroupPods:
			kinds[kindPod] = true
		}
	}
	if len(kinds) == 1 {
		for k := range kinds {
			return k + "s"
		}
	}
	return "Runtime"
}

// headRank orders the nodes that come first in the components column of a
// package graph: its source, then the packages it depends on. Every other
// node ranks last.
func headRank(n *Node, root string) int {
	switch {
	case n.Kind == KindSource:
		return 0
	case n.Kind == KindPackage && n.ID != root:
		return 1
	}
	return 2
}

// orderColumns sorts each column: the first by id, each later one by its
// neighbors' rows in the column before (ties by id, none last), then one
// upward sweep by the neighbors' rows in the column after (ties by the
// current row), among the rows of the nodes that have such neighbors. A
// source and the packages a package depends on go first in their column,
// whatever their neighbors.
func orderColumns(columns [][]*Node, edges []Edge, root string) {
	neighbors := map[string][]string{}
	for i := range edges {
		e := &edges[i]
		neighbors[e.From] = append(neighbors[e.From], e.To)
		neighbors[e.To] = append(neighbors[e.To], e.From)
	}
	row := map[string]int{}
	setRows := func(col []*Node) {
		for r, n := range col {
			row[n.ID] = r
		}
	}
	for k, col := range columns {
		if k == 0 {
			sort.Slice(col, func(i, j int) bool {
				if a, b := headRank(col[i], root), headRank(col[j], root); a != b {
					return a < b
				}
				return col[i].ID < col[j].ID
			})
			setRows(col)
			continue
		}
		keys := barycenters(col, columns[k-1], neighbors, row)
		sort.Slice(col, func(i, j int) bool {
			if a, b := headRank(col[i], root), headRank(col[j], root); a != b {
				return a < b
			}
			if c := keys[col[i].ID].compare(keys[col[j].ID]); c != 0 {
				return c < 0
			}
			return col[i].ID < col[j].ID
		})
		setRows(col)
	}
	for k := len(columns) - 2; k >= 0; k-- {
		sweepUp(columns[k], barycenters(columns[k], columns[k+1], neighbors, row), row)
		setRows(columns[k])
	}
}

// sweepUp reorders the nodes of col that have neighbors in the column after
// by their barycenter there, ties by the current row, within the rows they
// already hold. A node without such a neighbor keeps its row: its own row
// and a barycenter of the next column's rows are not on one scale.
func sweepUp(col []*Node, keys map[string]barycenter, row map[string]int) {
	var slots []int
	var movable []*Node
	for i, n := range col {
		if keys[n.ID].count > 0 {
			slots = append(slots, i)
			movable = append(movable, n)
		}
	}
	sort.SliceStable(movable, func(i, j int) bool {
		if c := keys[movable[i].ID].compare(keys[movable[j].ID]); c != 0 {
			return c < 0
		}
		return row[movable[i].ID] < row[movable[j].ID]
	})
	for i, slot := range slots {
		col[slot] = movable[i]
	}
}

// place gives every node its row and coordinates, centering each column on
// the tallest, and sorts the nodes by column and row. It lays out a graph
// with no band.
func place(g *Graph, columns [][]*Node) {
	maxRows := 0
	for _, col := range columns {
		maxRows = max(maxRows, len(col))
	}
	for k, col := range columns {
		offset := (maxRows - len(col)) * rowPitch / 2
		for r, n := range col {
			n.Row = r
			n.X = g.Layout.Columns[k].X
			n.Y = pad + header + offset + r*rowPitch
		}
	}
	sortNodes(g)
	g.Layout.Width = 2*pad + max(0, len(columns)*colPitch-(colPitch-nodeWidth))
	g.Layout.Height = 2*pad + header + max(0, maxRows*rowPitch-(rowPitch-nodeHeight))
}

func sortNodes(g *Graph) {
	sort.Slice(g.Nodes, func(i, j int) bool {
		a, b := &g.Nodes[i], &g.Nodes[j]
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.Row != b.Row {
			return a.Row < b.Row
		}
		return a.Y < b.Y
	})
}

// barycenter is the mean row of a node's neighbors in one column, kept as
// a sum and a count so comparisons are exact.
type barycenter struct {
	sum, count int
}

// compare orders barycenters; one with no neighbors sorts last.
func (a barycenter) compare(b barycenter) int {
	switch {
	case a.count == 0 && b.count == 0:
		return 0
	case a.count == 0:
		return 1
	case b.count == 0:
		return -1
	}
	l, r := a.sum*b.count, b.sum*a.count
	switch {
	case l < r:
		return -1
	case l > r:
		return 1
	}
	return 0
}

func barycenters(col, other []*Node, neighbors map[string][]string, row map[string]int) map[string]barycenter {
	in := make(map[string]bool, len(other))
	for _, n := range other {
		in[n.ID] = true
	}
	keys := make(map[string]barycenter, len(col))
	for _, n := range col {
		var b barycenter
		for _, m := range neighbors[n.ID] {
			if in[m] {
				b.sum += row[m]
				b.count++
			}
		}
		keys[n.ID] = b
	}
	return keys
}

// route is a cubic Bézier from the facing side of one node to the facing
// side of the other, with both control points at the horizontal midpoint.
func route(from, to *Node) []Point {
	if from == nil || to == nil {
		return []Point{}
	}
	sx, ex := from.X+nodeWidth, to.X
	if from.X > to.X {
		sx, ex = from.X, to.X+nodeWidth
	}
	sy, ey := from.Y+nodeHeight/2, to.Y+nodeHeight/2
	mid := (sx + ex) / 2
	return []Point{{sx, sy}, {mid, sy}, {mid, ey}, {ex, ey}}
}
