package graph

import (
	"slices"
	"sort"
)

// A band is an expanded configuration group laid out as a block: a run of
// rows across the components column and every column to its right, holding
// the group's member components and the objects below them, and no other
// node. A frame drawn around the band therefore encloses its group and
// nothing else, and no edge between two nodes outside the band crosses it.
//
// Once a graph holds a band, every column is placed on one row grid, so the
// rows of the band line up across columns. The nodes outside the bands are
// split into segments, above the first band, between two bands and below
// the last; each segment is placed top-aligned in every column and is as
// tall as its tallest column. A band sits half a row below the segment
// above it and half a row above the segment below it, which leaves room for
// the frame's header and foot.
//
// A node right of the components column belongs to the segment of the
// component it hangs from (the component its owns and controls edges lead
// back to). Within a segment a column keeps its barycenter order. A band
// takes the place of its first member, in barycenter order, among the other
// components; its members are ordered by name and their objects follow in
// member order.

// band is one expanded configuration group.
type band struct {
	id      string
	members []*Node // by name, then id
	// anchor is the barycenter row of the band's first member in the
	// components column, where the band is put among the other components.
	anchor int
}

// findBands returns the bands of a laid-out graph's columns, in the order
// they take in the components column: a band for each group id that some
// component says it is a member of.
func findBands(columns [][]*Node) []*band {
	byID := map[string]*band{}
	for _, col := range columns {
		for r, n := range col {
			if n.Kind != KindComponent || n.MemberOf == "" {
				continue
			}
			b := byID[n.MemberOf]
			if b == nil {
				b = &band{id: n.MemberOf, anchor: r}
				byID[n.MemberOf] = b
			}
			b.anchor = min(b.anchor, r)
			b.members = append(b.members, n)
		}
	}
	bands := make([]*band, 0, len(byID))
	for _, b := range byID {
		slices.SortFunc(b.members, func(x, y *Node) int {
			if x.Label != y.Label {
				return compareStrings(x.Label, y.Label)
			}
			return compareStrings(x.ID, y.ID)
		})
		bands = append(bands, b)
	}
	sort.Slice(bands, func(i, j int) bool {
		if bands[i].anchor != bands[j].anchor {
			return bands[i].anchor < bands[j].anchor
		}
		return bands[i].id < bands[j].id
	})
	return bands
}

func compareStrings(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// bandLayout is the nodes of a graph with bands sorted into the bands and
// the segments between them, per column.
type bandLayout struct {
	comp     int
	segs     [][][]*Node // segment -> column -> nodes
	inBand   [][][]*Node // band -> column -> nodes
	segRows  []int
	bandRows []int
}

// sortIntoBands assigns every node of the components column and the columns
// after it to a band or a segment (see the comment at the top of the file).
func sortIntoBands(g *Graph, columns [][]*Node, bands []*band, comp int) *bandLayout {
	member := map[string]int{}      // component id -> index of its band
	memberIndex := map[string]int{} // component id -> index among its band's members
	for i, b := range bands {
		for j, m := range b.members {
			member[m.ID], memberIndex[m.ID] = i, j
		}
	}
	rowOf := map[string]int{}
	for r, n := range columns[comp] {
		rowOf[n.ID] = r
	}
	segmentOf := func(componentRow int) int {
		seg := 0
		for _, b := range bands {
			if b.anchor < componentRow {
				seg++
			}
		}
		return seg
	}
	anchors := anchorsOf(g, columns, comp)
	l := newBandLayout(comp, len(bands), len(columns))
	order := map[string]int{}
	for k := comp; k < len(columns); k++ {
		for r, n := range columns[k] {
			a, anchored := anchors[n.ID]
			bi, inside := member[a]
			switch {
			case anchored && inside:
				order[n.ID] = memberIndex[a]*len(columns[k]) + r
				l.inBand[bi][k] = append(l.inBand[bi][k], n)
			case anchored:
				l.segs[segmentOf(rowOf[a])][k] = append(l.segs[segmentOf(rowOf[a])][k], n)
			default:
				l.segs[len(bands)][k] = append(l.segs[len(bands)][k], n)
			}
		}
	}
	l.measure(order)
	return l
}

func newBandLayout(comp, bands, columns int) *bandLayout {
	l := &bandLayout{
		comp:     comp,
		segs:     make([][][]*Node, bands+1),
		inBand:   make([][][]*Node, bands),
		segRows:  make([]int, bands+1),
		bandRows: make([]int, bands),
	}
	for s := range l.segs {
		l.segs[s] = make([][]*Node, columns)
	}
	for b := range l.inBand {
		l.inBand[b] = make([][]*Node, columns)
	}
	return l
}

// measure orders the nodes of each band by order and counts the rows each
// band and segment needs: the tallest of its columns.
func (l *bandLayout) measure(order map[string]int) {
	for b := range l.inBand {
		for k := l.comp; k < len(l.inBand[b]); k++ {
			col := l.inBand[b][k]
			sort.SliceStable(col, func(i, j int) bool { return order[col[i].ID] < order[col[j].ID] })
			l.bandRows[b] = max(l.bandRows[b], len(col))
		}
	}
	for s := range l.segs {
		for k := l.comp; k < len(l.segs[s]); k++ {
			l.segRows[s] = max(l.segRows[s], len(l.segs[s][k]))
		}
	}
}

// placeBands lays a graph with bands out on one row grid.
func placeBands(g *Graph, columns [][]*Node, bands []*band) {
	comp := bands[0].members[0].Column
	if comp >= len(columns) {
		place(g, columns)
		return
	}
	l := sortIntoBands(g, columns, bands, comp)
	top := pad + header
	bottom := 0
	put := func(n *Node, k, at int) {
		n.X = g.Layout.Columns[k].X
		n.Y = at
		n.Row = (at - top) / rowPitch
		bottom = max(bottom, at+nodeHeight)
	}
	y := top
	for s := 0; s <= len(bands); s++ {
		putRows(l.segs[s], comp, y, put)
		y += l.segRows[s] * rowPitch
		if s == len(bands) {
			break
		}
		y += rowPitch / 2
		putRows(l.inBand[s], comp, y, put)
		bottom = max(bottom, y+(l.bandRows[s]-1)*rowPitch+nodeHeight+bandFooter)
		y += l.bandRows[s]*rowPitch + rowPitch/2
	}
	placeLeft(g, columns, comp, top, put)
	sortNodes(g)
	g.Layout.Width = 2*pad + max(0, len(columns)*colPitch-(colPitch-nodeWidth))
	g.Layout.Height = bottom + pad
}

// putRows places the nodes of each column from comp on, top-aligned at y.
func putRows(cols [][]*Node, comp, y int, put func(n *Node, k, at int)) {
	for k := comp; k < len(cols); k++ {
		for r, n := range cols[k] {
			put(n, k, y+r*rowPitch)
		}
	}
}

// placeLeft puts the columns left of the components at the mean row of
// their neighbors in the column after, rounded down, so their elbows run in
// the gutter and the one grid stays whole.
func placeLeft(g *Graph, columns [][]*Node, comp, top int, put func(n *Node, k, at int)) {
	neighbors := map[string][]string{}
	for i := range g.Edges {
		e := &g.Edges[i]
		neighbors[e.From] = append(neighbors[e.From], e.To)
		neighbors[e.To] = append(neighbors[e.To], e.From)
	}
	at := map[string]*Node{}
	for _, col := range columns {
		for _, n := range col {
			at[n.ID] = n
		}
	}
	for k := comp - 1; k >= 0; k-- {
		for _, n := range columns[k] {
			sum, count := 0, 0
			for _, id := range neighbors[n.ID] {
				if m := at[id]; m != nil && m.Column == k+1 {
					sum += m.Y
					count++
				}
			}
			y := top
			if count > 0 {
				y = sum / count
			}
			put(n, k, y)
		}
	}
}

// anchorsOf maps the id of every node right of the components column (and
// the components themselves) to the id of the component it hangs from,
// walking owns and controls edges back. A node no component leads to has no
// entry.
func anchorsOf(g *Graph, columns [][]*Node, comp int) map[string]string {
	parent := map[string]string{}
	for i := range g.Edges { // sorted by id, so the first parent wins
		e := &g.Edges[i]
		if e.Kind != EdgeOwns && e.Kind != EdgeControls {
			continue
		}
		if _, ok := parent[e.To]; !ok {
			parent[e.To] = e.From
		}
	}
	isComp := map[string]bool{}
	for _, n := range columns[comp] {
		isComp[n.ID] = true
	}
	anchors := map[string]string{}
	for k := comp; k < len(columns); k++ {
		for _, n := range columns[k] {
			id := n.ID
			for hops := 0; !isComp[id] && hops <= len(g.Nodes); hops++ {
				p, ok := parent[id]
				if !ok {
					id = ""
					break
				}
				id = p
			}
			if id != "" && isComp[id] {
				anchors[n.ID] = id
			}
		}
	}
	return anchors
}
