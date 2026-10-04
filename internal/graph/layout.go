package graph

import (
	"sort"
)

// Layout geometry, in user units.
const (
	nodeWidth  = 200
	nodeHeight = 44
	colPitch   = 264 // node width plus the gutter edges run in
	rowPitch   = 56
	pad        = 16
	header     = 28 // room for the column titles
)

// layout places every node in its column and gives every edge a route.
// The node kind decides the column, so no layering algorithm is needed;
// within a column, nodes are ordered by the barycenter of their neighbors:
// the first column by id, each later one by its neighbors' rows in the
// column before (ties by id, none last), then one upward sweep by the
// neighbors' rows in the column after (ties by the current row). Every
// step is deterministic, so the same graph always lays out the same way.
func layout(g *Graph, titles []string) {
	columns := compactColumns(g, titles)
	orderColumns(columns, g.Edges)
	place(g, columns)
	at := make(map[string]*Node, len(g.Nodes))
	for i := range g.Nodes {
		at[g.Nodes[i].ID] = &g.Nodes[i]
	}
	for i := range g.Edges {
		g.Edges[i].Route = route(at[g.Edges[i].From], at[g.Edges[i].To])
	}
}

// compactColumns renumbers the columns that hold a node densely, so an
// empty column takes no space, and returns each column's nodes.
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
		n.Column = dense[n.Column]
		columns[n.Column] = append(columns[n.Column], n)
	}
	return columns
}

// orderColumns sorts each column: the first by id, each later one by its
// neighbors' rows in the column before (ties by id, none last), then one
// upward sweep by the neighbors' rows in the column after (ties by the
// current row).
func orderColumns(columns [][]*Node, edges []Edge) {
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
			sort.Slice(col, func(i, j int) bool { return col[i].ID < col[j].ID })
			setRows(col)
			continue
		}
		keys := barycenters(col, columns[k-1], neighbors, row)
		sort.Slice(col, func(i, j int) bool {
			if c := keys[col[i].ID].compare(keys[col[j].ID]); c != 0 {
				return c < 0
			}
			return col[i].ID < col[j].ID
		})
		setRows(col)
	}
	for k := len(columns) - 2; k >= 0; k-- {
		col := columns[k]
		keys := barycenters(col, columns[k+1], neighbors, row)
		for _, n := range col {
			if keys[n.ID].count == 0 {
				keys[n.ID] = barycenter{sum: row[n.ID], count: 1}
			}
		}
		sort.Slice(col, func(i, j int) bool {
			if c := keys[col[i].ID].compare(keys[col[j].ID]); c != 0 {
				return c < 0
			}
			return row[col[i].ID] < row[col[j].ID]
		})
		setRows(col)
	}
}

// place gives every node its row and coordinates, centering each column on
// the tallest, and sorts the nodes by column and row.
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
	sort.Slice(g.Nodes, func(i, j int) bool {
		if g.Nodes[i].Column != g.Nodes[j].Column {
			return g.Nodes[i].Column < g.Nodes[j].Column
		}
		return g.Nodes[i].Row < g.Nodes[j].Row
	})
	g.Layout.Width = 2*pad + max(0, len(columns)*colPitch-(colPitch-nodeWidth))
	g.Layout.Height = 2*pad + header + max(0, maxRows*rowPitch-(rowPitch-nodeHeight))
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
