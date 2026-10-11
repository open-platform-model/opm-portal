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
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#ffffff"/>`+"\n", g.Layout.Width, g.Layout.Height)
	for _, c := range g.Layout.Columns {
		fmt.Fprintf(&b, `<text x="%d" y="%d" fill="#667085" font-size="11" font-weight="bold">%s</text>`+"\n", c.X, pad+12, esc(strings.ToUpper(c.Title)))
	}
	for _, r := range bandRects(g) {
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="8" fill="none" stroke="#8a8fa0" stroke-dasharray="6 4"><title>band %s</title></rect>`+"\n",
			r.x0, r.y0, r.x1-r.x0, r.y1-r.y0, esc(r.id))
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
	fmt.Fprintf(b, `<path d="M%d %d H%d V%d H%d" fill="none" stroke="#8a8fa0" stroke-width="2"%s><title>%s</title></path>`+"\n",
		r[0].X, r[0].Y, r[1].X, r[3].Y, r[3].X, dash, esc(title))
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
	fmt.Fprintf(b, `<text x="12" y="22" fill="#667085" font-size="10.5">%s</text>`, esc(truncate(subtitle(n), 36)))
	fmt.Fprintf(b, `<text x="12" y="44" fill="#101828" font-weight="bold">%s</text></g>`+"\n", esc(truncate(n.Label, 28)))
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
		// stands: a blocked removal reads Stalled like a refusal. The
		// column names the kind, so the two pills fit in full.
		parts = []string{string(r.Verdict), "acc " + yesNo(r.Accepted) + " act " + yesNo(r.Active)}
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
		return "✓"
	}
	return "✗"
}

// truncate shortens s to n runes, never splitting one. It cuts from the
// middle and keeps the longer part at the end, where names differ: a
// catalog's path and major, a Pod's hash suffix.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	tail := (n - 1) * 2 / 3
	return string(r[:n-1-tail]) + "…" + string(r[len(r)-tail:])
}

func esc(s string) string { return html.EscapeString(s) }
