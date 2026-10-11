package ui

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// The Resources tab is one table (portal:D4, design.md decision 4): a row for
// each inventory object and each runtime child below it, a tbody of its own
// for each configuration group and for the old revisions of a workload. The
// group's header row holds a details element; CSS alone hides the member rows
// of a closed group, so the fold works without script.

// Row origins, in the words of the table.
const (
	originApplied = "applied"
	originCluster = "made by the cluster"
	originSource  = "source"
)

// resourcesTable is the Resources tab.
type resourcesTable struct {
	Bodies []resBody
	// Chip is the removable chip of the reason filter, nil without one.
	Chip *filterChip
	// Empty is the line under (or instead of) the table: nothing applied
	// yet, or no row with the asked reason.
	Empty string
}

// resBody is one tbody: plain rows, or a group with a header row.
type resBody struct {
	Head *resHead
	Rows []resRow
}

// resHead is a group's header row: the details element's summary.
type resHead struct {
	ID     string
	Label  string
	Note   string
	Count  int
	Health string
	Open   bool
}

// resRow is one row of the table.
type resRow struct {
	Kind      string
	Name      string // namespace/name, or the name
	Component string
	Origin    string
	// Health is the state, empty when none was read.
	Health string
	// Locked is true for an object the caller may not read; LockText says why.
	Locked   bool
	LockText string
	// Reason and Message are the health reason and message; Desired is a
	// ReplicaSet's desired count.
	Reason  string
	Message string
	Desired *int64
	// Note is a line of its own: a Secret's never-read line, the source's
	// revision, the children-unread line.
	Note string
	// NotLive and EvaluatedAt: the object is polled rather than watched.
	NotLive     bool
	EvaluatedAt *time.Time
	// Href is the link to the Graph tab focused on the row's node; empty for a
	// locked row.
	Href     string
	Child    bool
	Member   bool
	Degraded bool
	Current  bool
}

// resourcesOf builds the Resources tab. reason is the health reason the page
// filters by; the row of the node, else the focus, is marked current.
func resourcesOf(q url.Values, v *ownerView, g *v1.Graph) resourcesTable {
	reason := strings.TrimSpace(q.Get("reason"))
	sel := v.Node
	if sel == "" {
		sel = v.Focus
	}
	if sel == "" {
		sel = q.Get("focus")
	}
	byRef, groupOf := focusIDs(g)
	b := &tableBuilder{sel: sel}

	var t resourcesTable
	rv := filterView{Path: v.Base, Params: reasonFilter.Params}
	if f := parseFilters(&rv, q); len(f.Active) > 0 {
		t.Chip = &f.Active[0]
	}

	if v.Package != nil && reason == "" {
		b.sourceRow(v, g)
	}
	comps := annotateResources(v.Components, byRef, groupOf, v.Base, reason)
	for i := range comps {
		b.component(comps[i])
	}
	b.flush()
	for i := range v.Configs {
		cs := annotateResources(v.Configs[i].Components, byRef, groupOf, v.Base, reason)
		if len(cs) == 0 {
			continue
		}
		b.group(v.Configs[i], cs, reason != "")
	}
	t.Bodies = b.bodies

	switch {
	case reason != "" && b.rows == 0:
		t.Empty = "No object or child reports this reason."
	case reason == "" && !b.applied:
		t.Empty = nothingApplied(v)
	}
	return t
}

// nothingApplied is the line for an owner whose inventory is empty. A package
// with no source artifact has never fetched its source.
func nothingApplied(v *ownerView) string {
	if v.Package != nil && v.Package.SourceArtifact == nil {
		return "Nothing applied yet. The package has never fetched its source."
	}
	return "Nothing applied yet. The graph fills in once an apply succeeds."
}

// tableBuilder collects the tbody elements in page order.
type tableBuilder struct {
	sel    string
	bodies []resBody
	cur    resBody
	// rows counts every row drawn; applied is true once an inventory object
	// is drawn (the source row is not one).
	rows    int
	applied bool
}

// flush closes the open plain tbody.
func (b *tableBuilder) flush() {
	if len(b.cur.Rows) > 0 {
		b.bodies = append(b.bodies, b.cur)
	}
	b.cur = resBody{}
}

func (b *tableBuilder) add(r resRow) {
	b.cur.Rows = append(b.cur.Rows, r)
	b.rows++
}

// sourceNode is the graph's source node when it names an object, else nil.
func sourceNode(g *v1.Graph) *v1.GraphNode {
	for i := range g.Nodes {
		if g.Nodes[i].Kind == "source" {
			if g.Nodes[i].Ref == nil {
				return nil
			}
			return &g.Nodes[i]
		}
	}
	return nil
}

// sourceRow is the package's source, the first row, from the graph's source
// node. Without a health on the node it reads "not read".
func (b *tableBuilder) sourceRow(v *ownerView, g *v1.Graph) {
	n := sourceNode(g)
	if n == nil {
		return
	}
	row := resRow{Kind: n.Ref.Kind, Name: nsName(*n.Ref), Origin: originSource, Current: b.sel == n.ID}
	if n.Access != "" && n.Access != v1.AccessOK {
		row.Locked, row.LockText = true, accessText(n.Access)
		b.add(row)
		return
	}
	row.Href = v.Base + "?" + url.Values{"tab": {tabGraph}, "focus": {n.ID}}.Encode()
	if n.Health != nil {
		row.Health, row.Reason, row.Message = n.Health.State, n.Health.Reason, n.Health.Message
		row.Degraded = degradedState(n.Health.State)
	}
	if a := v.Package.SourceArtifact; a != nil && a.Revision != "" {
		row.Note = "revision " + a.Revision
	}
	b.add(row)
}

// component draws the rows of each object of c in the plain tbody; the old
// revisions of a workload close it and take a tbody of their own.
func (b *tableBuilder) component(c componentView) {
	for i := range c.Objects {
		rows, old := b.objectRows(&c.Objects[i], c.Name, false)
		for j := range rows {
			b.add(rows[j])
		}
		if len(old) == 0 {
			continue
		}
		b.flush()
		b.bodies = append(b.bodies, b.oldBody(&c.Objects[i], old))
		b.rows += len(old)
	}
	b.applied = b.applied || len(c.Objects) > 0
}

// group draws one configuration group: a header row and a member row for
// every object and child of its components. It is open when the group's
// health is not Healthy, a member is not healthy, or a reason is asked for.
func (b *tableBuilder) group(cg configGroup, cs []componentView, filtered bool) {
	body := resBody{Head: &resHead{ID: cg.ID, Label: cg.Label, Health: cg.Health, Open: cg.Open || filtered,
		Note: "RBAC, CRDs, webhooks and other objects with no workload"}}
	for i := range cs {
		for j := range cs[i].Objects {
			rows, old := b.objectRows(&cs[i].Objects[j], cs[i].Name, true)
			body.Rows = append(body.Rows, rows...)
			for k := range old {
				old[k].Member = true
				body.Rows = append(body.Rows, old[k])
			}
		}
	}
	for i := range body.Rows {
		if body.Rows[i].Health != "" && body.Rows[i].Health != healthHealthy {
			body.Head.Open = true
		}
	}
	body.Head.Count = len(body.Rows)
	b.rows += len(body.Rows)
	b.applied = b.applied || len(body.Rows) > 0
	b.bodies = append(b.bodies, body)
}

// oldBody is the tbody of a workload's old revisions: a header row with the
// fold, and a member row for each. It is open when a revision is not healthy.
func (b *tableBuilder) oldBody(o *objectView, old []resRow) resBody {
	head := &resHead{ID: o.OldID, Count: len(old), Note: "scaled to zero"}
	head.Label = strconv.Itoa(len(old)) + " old " + plural(len(old), "revision", "revisions")
	for i := range old {
		old[i].Member = true
		if old[i].Health != healthHealthy {
			head.Open = true
		}
	}
	return resBody{Head: head, Rows: old}
}

// objectRows draws an object's row and the rows of the children below it,
// and returns the old revisions apart, as their rows. member marks the rows
// that belong to a group's tbody.
func (b *tableBuilder) objectRows(o *objectView, component string, member bool) (rows, old []resRow) {
	row := resRow{Kind: o.Ref.Kind, Name: nsName(o.Ref), Component: component, Origin: originApplied, Member: member}
	row.Current = o.NodeID != "" && o.NodeID == b.sel
	switch {
	case o.Access != v1.AccessOK:
		row.Locked, row.LockText = true, accessText(o.Access)
	default:
		row.Href = o.Focus
		if isSecret(o.Ref) {
			row.Note = "Secret data is never read"
		}
		if o.Health != nil {
			row.Health, row.Reason, row.Message = o.Health.State, o.Health.Reason, o.Health.Message
			row.Degraded = degradedState(o.Health.State)
		}
		if !o.Live {
			row.NotLive, row.EvaluatedAt = true, o.EvaluatedAt
		}
		if o.ChildrenUnread {
			row.Note = "Its Pods could not be read, so its health is its own status alone."
		}
	}
	rows = append(rows, row)
	for i := range o.Children {
		rows = append(rows, childRow(&o.Children[i], component, member, b.sel))
	}
	for i := range o.Old {
		old = append(old, childRow(&o.Old[i], component, true, b.sel))
	}
	return rows, old
}

func childRow(c *childView, component string, member bool, sel string) resRow {
	return resRow{
		Kind: c.Ref.Kind, Name: nsName(c.Ref), Component: component, Origin: originCluster,
		Health: c.Health.State, Reason: c.Health.Reason, Message: c.Health.Message, Desired: c.Replicas,
		Href: c.Focus, Child: true, Member: member, Degraded: degradedState(c.Health.State),
		Current: c.NodeID != "" && c.NodeID == sel,
	}
}

// degradedState is a state a row is marked apart for.
func degradedState(state string) bool { return state == healthDegraded || state == healthMissing }

// nsName is namespace/name, or the name of a cluster-scoped object.
func nsName(r v1.ObjectRef) string {
	if r.Namespace == "" {
		return r.Name
	}
	return r.Namespace + "/" + r.Name
}
