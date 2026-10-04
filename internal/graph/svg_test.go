package graph

import (
	"bytes"
	"fmt"
	"html"
	"strings"

	"github.com/open-platform-model/opm-portal/internal/health"
)

// writeSVG draws a laid-out graph so a reviewer can look at a golden
// layout. It is a test aid, not the portal's renderer: the UI draws its own
// SVG from the graph's JSON. It uses presentation attributes only (no
// style element or attribute), so it stays within a strict
// Content-Security-Policy, and it is deterministic.
func writeSVG(g Graph) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="sans-serif" font-size="12" role="img" aria-label="%s graph of %s">`+"\n",
		g.Layout.Width, g.Layout.Height, g.Layout.Width, g.Layout.Height, g.Scope, esc(g.Root))
	b.WriteString(`<defs><marker id="arrow" viewBox="0 0 8 8" refX="8" refY="4" markerWidth="8" markerHeight="8" orient="auto"><path d="M0 0L8 4L0 8z" fill="#8a8fa0"/></marker></defs>` + "\n")
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#ffffff"/>`+"\n", g.Layout.Width, g.Layout.Height)
	for _, c := range g.Layout.Columns {
		fmt.Fprintf(&b, `<text x="%d" y="%d" fill="#667085" font-size="11" font-weight="bold">%s</text>`+"\n", c.X, pad+12, esc(strings.ToUpper(c.Title)))
	}
	for i := range g.Edges {
		writeEdge(&b, &g.Edges[i])
	}
	for i := range g.Nodes {
		writeNode(&b, &g.Nodes[i], g.Layout)
	}
	b.WriteString("</svg>\n")
	return b.Bytes()
}

func writeEdge(b *bytes.Buffer, e *Edge) {
	if len(e.Route) != 4 {
		return
	}
	r := e.Route
	dash := ""
	title := string(e.Kind) + ": " + e.Source
	if e.Verified != nil && !*e.Verified {
		dash = ` stroke-dasharray="5 4"`
		title += " (unverified: " + e.Reason + ")"
	}
	fmt.Fprintf(b, `<path d="M%d %d C%d %d %d %d %d %d" fill="none" stroke="#8a8fa0" stroke-width="1.3" marker-end="url(#arrow)"%s><title>%s</title></path>`+"\n",
		r[0].X, r[0].Y, r[1].X, r[1].Y, r[2].X, r[2].Y, r[3].X, r[3].Y, dash, esc(title))
}

func writeNode(b *bytes.Buffer, n *Node, l Layout) {
	stroke := stateColor(n)
	dash := ""
	if n.Access != "" && n.Access != health.AccessOK || n.Missing {
		dash = ` stroke-dasharray="4 3"`
	}
	fmt.Fprintf(b, `<g transform="translate(%d %d)"><title>%s</title>`, n.X, n.Y, esc(n.ID))
	fmt.Fprintf(b, `<rect width="%d" height="%d" rx="6" fill="#f8f9fb" stroke="%s" stroke-width="1.5"%s/>`, l.NodeWidth, l.NodeHeight, stroke, dash)
	fmt.Fprintf(b, `<rect width="5" height="%d" rx="2" fill="%s"/>`, l.NodeHeight, stroke)
	fmt.Fprintf(b, `<text x="12" y="17" fill="#667085" font-size="10.5">%s</text>`, esc(truncate(subtitle(n), 34)))
	fmt.Fprintf(b, `<text x="12" y="34" fill="#101828" font-weight="bold">%s</text></g>`+"\n", esc(truncate(n.Label, 26)))
}

// subtitle is the node's kind and its states, as words, so state is never
// shown by color alone.
func subtitle(n *Node) string {
	parts := []string{string(n.Kind)}
	if n.Ref != nil && (n.Kind == KindObject || n.Kind == KindRuntime) {
		parts[0] = n.Ref.Kind
	}
	if r := n.Registration; r != nil {
		// A registration's verdict, not its applied state, says how it
		// stands: a blocked removal reads Stalled like a refusal.
		parts = append(parts, string(r.Verdict), "accepted "+yesNo(r.Accepted), "active "+yesNo(r.Active))
	} else if n.Applied != nil {
		parts = append(parts, string(n.Applied.State))
	}
	switch {
	case n.Missing:
		parts = append(parts, "not found")
	case n.Access != "" && n.Access != health.AccessOK:
		parts = append(parts, string(n.Access))
	case n.Health != nil:
		h := string(n.Health.State)
		if n.Health.Partial {
			h += " (partial)"
		}
		parts = append(parts, h)
	}
	if n.HiddenScaledDown > 0 {
		parts = append(parts, fmt.Sprintf("%d scaled down", n.HiddenScaledDown))
	}
	return strings.Join(parts, " · ")
}

func stateColor(n *Node) string {
	if n.Health == nil {
		return "#8a8fa0"
	}
	switch n.Health.State {
	case health.Healthy:
		return "#2f8f4e"
	case health.Progressing:
		return "#c98a00"
	case health.Degraded, health.Missing:
		return "#c62f2f"
	case health.Unknown:
		return "#8a8fa0"
	default:
		return "#8a8fa0"
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// truncate shortens s to n runes, never splitting one.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func esc(s string) string { return html.EscapeString(s) }
