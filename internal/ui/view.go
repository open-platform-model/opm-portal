package ui

import (
	"fmt"
	"html/template"
	"math"
	"net/url"
	"strings"
	"time"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// badge is one rendered status value: its CSS class, its words and, for a
// value the UI does not know, the raw value as a tooltip.
type badge struct {
	Class string
	Text  string
	Title string
}

// appliedText names the applied states the UI knows. Any other value is
// shown as unknown, never as an error (portal:D2:R3).
var appliedText = map[string]string{
	"Applied":           "Applied",
	"Reconciling":       "Reconciling",
	"Failed":            "Failed",
	"Stalled":           "Stalled",
	"Suspended":         "Suspended",
	"ManagedExternally": "Managed externally",
	"Unknown":           "Unknown",
}

// healthText names the health states the UI knows.
var healthText = map[string]string{
	"Healthy":     "Healthy",
	"Progressing": "Progressing",
	"Degraded":    "Degraded",
	"Missing":     "Missing",
	"Unknown":     "Unknown",
}

// verdictText names the registration verdicts the UI knows.
var verdictText = map[string]string{
	"Accepted":       "Accepted",
	"Refused":        "Refused",
	"Pending":        "Pending",
	"RemovalBlocked": "Removal blocked",
	"Unknown":        "Unknown",
}

// known returns the badge of value from table, or the unknown badge
// carrying the raw value.
func known(prefix string, table map[string]string, value string) badge {
	if text, ok := table[value]; ok {
		return badge{Class: prefix + " " + prefix + "-" + slug(value), Text: text}
	}
	b := badge{Class: prefix + " " + prefix + "-unknown", Text: "unknown"}
	if value != "" {
		b.Title = "The portal does not know the value " + value
	}
	return b
}

func slug(s string) string { return strings.ToLower(s) }

// appliedBadge is the operator's axis: a squared stamp. Ready=True reads
// Applied; ManagedExternally is neutral (portal:D3:R1/R6).
func appliedBadge(r v1.Reconcile) badge {
	b := known("applied", appliedText, r.State)
	if r.State == "Failed" && r.Retrying {
		b.Text = "Failed, retrying"
	}
	return b
}

// healthBadge is the portal's axis: a dot and a word, partial and not live
// said beside it (portal:D3:R4/R5).
func healthBadge(h v1.Health) badge {
	b := known("health", healthText, h.State)
	var notes []string
	if h.Partial {
		notes = append(notes, "partial")
	}
	if !h.Live && h.State != "" {
		notes = append(notes, "not live")
	}
	if len(notes) > 0 {
		b.Text += " (" + strings.Join(notes, ", ") + ")"
	}
	if h.Partial {
		b.Class += " health-partial"
	}
	return b
}

// stateBadge is a health badge for one object or graph node.
func stateBadge(state string) badge { return known("health", healthText, state) }

func verdictBadge(v string) badge { return known("verdict", verdictText, v) }

// providerBadge is the standing of one registration an instance or
// package holds, as the controller wrote it (portal:D15:R2/R4): never one
// the portal computed, and locked when the caller may not read it.
func providerBadge(c v1.ProviderClaim) badge {
	b := badge{Class: "prov", Text: "Provider", Title: c.Registration}
	if c.Access != v1.AccessOK {
		b.Class += " prov-locked"
		b.Text = "Provider, locked"
		b.Title = c.Registration + ": " + accessText(c.Access)
		return b
	}
	switch {
	case c.Verdict == verdictRemovalBlocked:
		b.Class += " prov-blocked"
		b.Text = "Provider, removal blocked"
	case c.Verdict == verdictAccepted && c.Active:
		b.Class += " prov-active"
	case c.Verdict == verdictAccepted:
		b.Class += " prov-inactive"
		b.Text = "Provider, not active"
	case c.Verdict == verdictRefused:
		b.Class += " prov-refused"
		b.Text = "Provider, refused"
	case c.Verdict == verdictPending:
		b.Class += " prov-pending"
		b.Text = "Provider, pending"
	default:
		b.Class += " prov-unknown"
		b.Text = "Provider, unknown"
	}
	if c.Reason != "" {
		b.Title += ": " + c.Reason
	}
	return b
}

// claimBadge is a held registration's standing on the Provider card, in
// the card's words; the classes are providerBadge's.
func claimBadge(c v1.ProviderClaim) badge {
	b := providerBadge(c)
	switch {
	case c.Access != v1.AccessOK:
		b.Text = "Locked"
	case c.Verdict == verdictRemovalBlocked:
		b.Text = "Removal blocked"
	case c.Verdict == verdictAccepted && c.Active:
		b.Text = "Active"
	case c.Verdict == verdictAccepted:
		b.Text = "Accepted, not active"
	case c.Verdict == verdictRefused:
		b.Text = "Refused"
	case c.Verdict == verdictPending:
		b.Text = "Pending"
	default:
		b.Text = "Unknown"
	}
	return b
}

// ownerText is an owner as the pages say it: the read API's operator is
// the controller (portal:D17:R2).
func ownerText(owner string) string {
	switch owner {
	case "operator", ownerController:
		return ownerController
	case "cli":
		return "cli"
	}
	return owner
}

// accessText says why something is locked.
func accessText(access string) string {
	switch access {
	case v1.AccessForbidden:
		return "Locked: you may not read this."
	case v1.AccessNotReadable:
		return "Locked: the portal cannot read this right now."
	case v1.AccessWithheld:
		return "Withheld: the portal never reads Secret data."
	}
	return "Locked: the portal does not know this access value."
}

// toneClass is a condition's stripe class from the tone the read API
// serves; a tone the UI does not know is shown as unknown.
func toneClass(tone string) string {
	switch tone {
	case "normal", "abnormal", "progressing", "informational":
		return "tone-" + tone
	}
	return "tone-unknown"
}

// stamp is a time as the pages show it: absolute in the datetime attribute
// and the tooltip, relative in the text.
type stamp struct {
	ISO      string
	Absolute string
	Relative string
}

func (h *Handler) stamp(t *time.Time) *stamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return &stamp{
		ISO:      t.UTC().Format(time.RFC3339),
		Absolute: t.UTC().Format("2006-01-02 15:04:05 UTC"),
		Relative: relative(h.cfg.Now().Sub(*t)),
	}
}

func relative(d time.Duration) string {
	switch {
	case d < 0:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(math.Floor(d.Hours()/24)))
}

// short cuts a digest for display; the full value stays in the tooltip.
func short(digest string) string {
	if _, rest, ok := strings.Cut(digest, ":"); ok {
		digest = rest
	}
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}

// refText names an object the way kubectl does: kind[.group] ns/name.
func refText(r v1.ObjectRef) string {
	kind := r.Kind
	if r.Group != "" {
		kind += "." + r.Group
	}
	if r.Namespace != "" {
		return kind + " " + r.Namespace + "/" + r.Name
	}
	return kind + " " + r.Name
}

// refQuery is the query naming an object for the object and events
// resources.
func refQuery(r v1.ObjectRef) url.Values {
	q := url.Values{"kind": {r.Kind}, "name": {r.Name}}
	if r.Group != "" {
		q.Set("group", r.Group)
	}
	if r.Namespace != "" {
		q.Set("namespace", r.Namespace)
	}
	return q
}

// segments cuts s after every dot and slash, so a template can let a long
// reference wrap only there.
func segments(s string) []string {
	var out []string
	start := 0
	for i, r := range s {
		if r == '.' || r == '/' {
			out = append(out, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// plural picks the word for n.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// isSecret reports whether r is a core Secret, which the portal never
// reads.
func isSecret(r v1.ObjectRef) bool { return r.Group == "" && r.Kind == "Secret" }

var funcs = template.FuncMap{
	"appliedBadge":  appliedBadge,
	"healthBadge":   healthBadge,
	"stateBadge":    stateBadge,
	"verdictBadge":  verdictBadge,
	"providerBadge": providerBadge,
	"claimBadge":    claimBadge,
	"ownerText":     ownerText,
	"accessText":    accessText,
	"toneClass":     toneClass,
	"short":         short,
	"contractShort": contractShort,
	"refText":       refText,
	"isSecret":      isSecret,
	"join":          strings.Join,
	"segments":      segments,
	"plural":        plural,
	"add":           func(a, b int) int { return a + b },
	"locked": func(access string) bool {
		return access != "" && access != v1.AccessOK
	},
}

// newestFirst orders two event times newest first, an event with no time
// last.
func newestFirst(x, y *time.Time) int {
	switch {
	case x == nil && y == nil:
		return 0
	case x == nil:
		return 1
	case y == nil:
		return -1
	}
	return y.Compare(*x)
}
