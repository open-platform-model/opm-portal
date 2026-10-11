package ui

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// The Catalog page: one catalog as the Platform and the registrations
// record it (portal:D17:R3). It reads no resource of its own: the Platform
// document carries the registry, the subscriptions, the registrations
// claiming it and their holders. Definitions, descriptions and
// transformers are not recorded, and the page says so.

const (
	tabClaims = "claims"
)

type catalogView struct {
	Row catalogRow
	// Short is the headline: the catalog's last path segment without its
	// major version, or a longer form when another catalog shares it
	// (catalog-01, catalog-24).
	Short string
	// Origin says where the catalog comes from, in words; OriginClass is the
	// pill's class (catalog-02).
	Origin      string
	OriginClass string
	// Source is the Source row: "Platform subscription", or the registrations
	// the catalog comes from, each linked to its holder (catalog-05,
	// catalog-25).
	SourceSubscription bool
	SourceLinks        []claimLink
	// EnabledText is the Enabled row's words; EnabledMuted says it is a muted
	// "not recorded" (catalog-07).
	EnabledText  string
	EnabledMuted bool
	// Block is the Resolved state block (catalog-08, catalog-26).
	Block     stateBlock
	Contracts *v1.Condition
	Claims    []catalogClaim
	// ClaimsLocked: the caller may not list the registrations, so the
	// claims, and whether there are any, are locked (portal:D7:R2/R3).
	ClaimsLocked string
	Tab          string
	Tabs         []tabLink
	Events       catalogEvents
}

// claimLink is one registration named on the Source row: its name, and its
// holders when the caller may read them. One holder makes the name a link to
// that holder's Provider tab; several follow the name as a muted "held by".
type claimLink struct {
	Name    string
	Href    string
	Holders []holderLink
}

// catalogClaim is one registration claiming the catalog.
type catalogClaim struct {
	v1.Registration
	Holders []holderLink
	Badges  []sqBadge
	Why     string
}

func (h *Handler) catalogPage(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	var p v1.Platform
	prob := h.fetch(r, "/platform", nil, &p)
	if unauthenticated(prob) {
		h.signIn(w, r)
		return
	}
	switch {
	case prob != nil && prob.Code == v1.CodeForbidden:
		// The same locked page for every path: it says nothing about
		// whether the catalog exists.
		h.render(w, r, http.StatusForbidden, "message", page{Title: "Catalog", Nav: "platform", Main: message{
			Heading: "Catalog locked",
			Text:    "You may not read the Platform, which records the catalogs, so the portal shows none.",
			Code:    v1.CodeForbidden,
		}})
		return
	case prob != nil:
		h.render(w, r, problemStatus(prob), "message", page{Title: "Catalog", Nav: "platform", Main: message{
			Heading: "Catalog unavailable", Text: "The Platform could not be read right now.", Code: prob.Code,
		}})
		return
	}
	rows, _ := catalogRows(&p, nil)
	i := slices.IndexFunc(rows, func(c catalogRow) bool { return c.Path == path })
	if i < 0 {
		h.render(w, r, http.StatusNotFound, "message", page{Title: "Not found", Nav: "platform", Main: message{
			Heading: "No such catalog",
			Text:    "The Platform subscribes to no catalog of this path, holds none in its registry, and no registration you may read claims one.",
			Code:    v1.CodeNotFound,
		}})
		return
	}
	v := catalogViewOf(&p, rows[i], rows)
	if r.URL.Query().Get("tab") == tabEvents {
		v.Tab = tabEvents
	}
	base := catalogHref(path)
	for _, t := range []struct{ Name, Label string }{{tabClaims, "Claims"}, {tabEvents, "Events"}} {
		tab := tabLink{Name: t.Name, Label: t.Label, Href: base + "&tab=" + t.Name, Current: v.Tab == t.Name}
		if t.Name == tabClaims {
			tab.Follow = platformTopic
		}
		v.Tabs = append(v.Tabs, tab)
	}
	// Claims are counted when the caller may read every registration; the
	// Events tab is read only when it opens, so it carries no count.
	if v.ClaimsLocked == "" {
		setCount(v.Tabs, tabClaims, len(v.Claims))
	}
	topics := []string{platformTopic}
	if v.Tab == tabEvents {
		v.Events = h.eventsTab(r, v.claimants())
		topics = append(topics, strings.Fields(v.Events.Follow)...)
	}
	h.render(w, r, http.StatusOK, "catalog", page{Title: "Catalog " + path, Nav: "platform", Topics: topics, Main: v,
		Canonical: "/catalog?" + url.Values{"path": {path}}.Encode()})
}

// catalogShort is a catalog's short name: its last path segment without the
// major version. When another catalog among all shares that segment, the
// longer form contractShort gives is used instead.
func catalogShort(path string, all []string) string {
	last := func(p string) string {
		p = p[strings.LastIndex(p, "/")+1:]
		if i := strings.LastIndex(p, "@"); i > 0 {
			p = p[:i]
		}
		return p
	}
	name := last(path)
	for _, o := range all {
		if o != path && last(o) == name {
			return contractShort(path)
		}
	}
	return name
}

// catalogViewOf is one catalog row with what the Platform says about it:
// its origin, its source, why it is not resolved, the Platform-wide contracts
// and its claims.
func catalogViewOf(p *v1.Platform, row catalogRow, all []catalogRow) catalogView {
	paths := make([]string, 0, len(all))
	for i := range all {
		paths = append(paths, all[i].Path)
	}
	v := catalogView{Row: row, Tab: tabClaims, Short: catalogShort(row.Path, paths)}
	v.Origin, v.OriginClass = originOf(row)
	if p.RegistrationsAccess != v1.AccessOK {
		v.ClaimsLocked = p.RegistrationsAccess
	}
	st := statusOf(p)
	v.Contracts = st.Contracts
	for j := range p.Registrations {
		reg := p.Registrations[j]
		if reg.Catalog != row.Path {
			continue
		}
		cl := catalogClaim{Registration: reg, Badges: registrationBadges(&p.Registrations[j]), Why: whyHue(reg.Verdict)}
		for _, ref := range reg.HeldBy {
			cl.Holders = append(cl.Holders, holderOf(ref))
		}
		v.Claims = append(v.Claims, cl)
	}
	switch row.Source {
	case sourceSubscription:
		v.SourceSubscription = true
	case sourceRegistration:
		v.SourceLinks = v.claimLinks(row.ContributedBy)
	default:
		v.SourceLinks = v.claimLinks("")
	}
	switch {
	case row.Enabled != nil && *row.Enabled:
		v.EnabledText = yesWord
	case row.Enabled != nil:
		v.EnabledText = "no"
	case !row.Resolved:
		v.EnabledText = "no: not in the registry"
	default:
		v.EnabledText, v.EnabledMuted = notRecorded, true
	}
	v.Block = catalogBlock(p, &v, st.Ready)
	return v
}

// claimLinks names the registrations the Source row shows: the one called
// only, or every claim when only is empty. A registration the caller may not
// read is named plain, with no holders.
func (v *catalogView) claimLinks(only string) []claimLink {
	var out []claimLink
	if only != "" {
		l := claimLink{Name: only}
		for i := range v.Claims {
			if v.Claims[i].Name == only {
				l.Holders = v.Claims[i].Holders
			}
		}
		return []claimLink{withHref(l)}
	}
	for i := range v.Claims {
		out = append(out, withHref(claimLink{Name: v.Claims[i].Name, Holders: v.Claims[i].Holders}))
	}
	return out
}

// withHref links a name to its holder's Provider tab when it has exactly one;
// with several, the template names them after the name; with none, the name
// stays plain.
func withHref(l claimLink) claimLink {
	if len(l.Holders) == 1 {
		l.Href = l.Holders[0].Href
	}
	return l
}

// Words the Catalog page and the Platform page share.
const (
	sourceRegistration = "registration"
	sourceClaim        = "claim"
	hueAppliedWord     = "applied"
	sinceWord          = "since"
	refusedSinceWord   = "refused since"
	yesWord            = "yes"
)

// catalogBlock is the Resolved state block (catalog-08, catalog-26): the
// state word, what the registry records, the time since the refusal and the
// reasons, each linking to the Events tab filtered by it. It never invents a
// time: a catalog that resolved has none.
func catalogBlock(p *v1.Platform, v *catalogView, ready *v1.Condition) stateBlock {
	row := v.Row
	b := stateBlock{ID: "catalog-resolved", Label: "Resolution status", Eyebrow: "Resolved · the controller",
		Hue: "degraded", State: "Unresolved", None: "No resolution errors", Follow: platformTopic}
	if row.Resolved {
		b.Hue, b.State = hueAppliedWord, "Resolved"
		b.Summary = "In the resolved registry" + atVersion(row.Version) + ", from the Platform's subscription"
		if row.Source != sourceSubscription {
			b.Summary = "In the resolved registry" + atVersion(row.Version) + ", contributed by its provider's registration"
		}
		return newStateBlock(b)
	}
	b.Summary = "Not in the resolved registry"
	if row.Source == sourceClaim && len(v.Claims) > 0 {
		names := make([]string, 0, len(v.Claims))
		for i := range v.Claims {
			names = append(names, v.Claims[i].Name)
		}
		b.Summary += ". Claimed by " + strings.Join(names, ", ")
	}
	b.None = noReasonRecorded
	reasons, earliest := claimReasons(row.Path, v.Claims)
	platformReady := ready != nil && ready.Status != condTrue
	refused := len(reasons) > 0
	if !refused && platformReady {
		// The reason counts nothing recorded: it is the Platform's, not a
		// claim's.
		reasons = append(reasons, countLink{Text: reasonOf(ready), Class: "hue-degraded", Href: catalogEventsHref(row.Path, reasonOf(ready))})
	}
	b.Reasons = reasons
	b.When, b.WhenText = refusalTime(row, p, ready, refused, earliest)
	return newStateBlock(b)
}

// refusalTime is the recorded time a block shows, with its words: for a
// claim, when the earliest claiming registration turned Ready False (the
// Platform's own "since" when none carries that condition); for a
// subscription, when the Platform's Ready changed. Nil when nothing records one.
func refusalTime(row catalogRow, p *v1.Platform, ready *v1.Condition, refused bool, earliest *time.Time) (when *time.Time, words string) {
	switch {
	case refused && earliest != nil:
		return earliest, refusedSinceWord
	case refused && p.Reconcile.Since != nil:
		return p.Reconcile.Since, refusedSinceWord
	case row.Source == sourceSubscription && ready != nil && ready.Status != condTrue && ready.LastTransitionTime != nil:
		return ready.LastTransitionTime, sinceWord
	}
	return nil, ""
}

// catalogEventsHref opens a catalog's Events tab filtered by one reason.
func catalogEventsHref(path, reason string) string {
	return "/catalog?" + url.Values{"path": {path}, "tab": {tabEvents}, "reason": {reason}}.Encode()
}

// claimReasons counts the refusal reasons of a catalog's claiming
// registrations, most counted first, and returns the earliest time one of
// them turned Ready False.
func claimReasons(path string, claims []catalogClaim) (reasons []countLink, earliest *time.Time) {
	counts := map[string]int{}
	for i := range claims {
		reg := &claims[i].Registration
		if !reg.Accepted && reg.Reason != "" {
			counts[reg.Reason]++
		}
		for j := range reg.Conditions {
			c := &reg.Conditions[j]
			if c.Type == conditionReady && c.Status == "False" && c.LastTransitionTime != nil &&
				(earliest == nil || c.LastTransitionTime.Before(*earliest)) {
				earliest = c.LastTransitionTime
			}
		}
	}
	for reason, n := range counts {
		reasons = append(reasons, countLink{Text: reason, Class: "hue-degraded", N: n, Href: catalogEventsHref(path, reason)})
	}
	slices.SortFunc(reasons, func(x, y countLink) int {
		if x.N != y.N {
			return y.N - x.N
		}
		return strings.Compare(x.Text, y.Text)
	})
	return reasons, earliest
}

func atVersion(v string) string {
	if v == "" {
		return ""
	}
	return " at " + v
}

// originOf names where a catalog comes from and the class of its pill.
func originOf(row catalogRow) (text, class string) {
	switch row.Source {
	case sourceSubscription:
		return "Subscribed", "origin-subscription"
	case sourceRegistration:
		return "From a provider", "origin-registration"
	case sourceClaim:
		return "Claimed only", "origin-claim"
	}
	return row.SourceText, "origin-claim"
}

func (v *catalogView) claimants() []string {
	out := make([]string, 0, len(v.Claims))
	for i := range v.Claims {
		out = append(out, v.Claims[i].Name)
	}
	return out
}

// catalogEventFilters is the Events tab's filters: the type and the reason.
// The reason arrives on a deep link from the Resolved block and has no field
// of its own, so the form carries it as a hidden field. The view is not
// remembered (portal:D14), so it is not in rememberedViews.
var catalogEventFilters = filterView{Key: "catalog-events", Path: "/catalog", Params: []filterParam{
	{Name: "type", Label: "Type", Any: "Any type", Values: []string{eventNormal, eventWarning}},
	{Name: "reason", Label: "Reason"},
}}

// catalogEvents is the Events tab: the Platform's feed and its claimants',
// newest first, filtered by type and reason.
type catalogEvents struct {
	Form  filterForm
	Table eventsTable
	// Unread names the registrations whose feed could not be read.
	Unread []string
	Follow string
}

// eventsTab reads the merged feed of the Platform and the claiming
// registrations and applies the type and reason filters (catalog-18,
// catalog-19). Total is the unfiltered length; the type options count over it.
func (h *Handler) eventsTab(r *http.Request, claimants []string) catalogEvents {
	q := r.URL.Query()
	f := parseFilters(&catalogEventFilters, q)
	ev := catalogEvents{}
	follow := []string{"events:platform"}
	feed := h.events(r, "/platform/events", nil, "")
	items := feed.Items
	for _, n := range claimants {
		follow = append(follow, "events:registration:"+n)
		reg := h.events(r, "/platform/registrations/"+url.PathEscape(n)+"/events", nil, "")
		if reg.Problem != nil {
			ev.Unread = append(ev.Unread, n)
			continue
		}
		items = append(items, reg.Items...)
	}
	slices.SortStableFunc(items, func(a, b v1.Event) int { return newestFirst(a.LastSeen, b.LastSeen) })
	ev.Follow = strings.Join(follow, " ")
	total := len(items)
	counts := map[string]int{eventNormal: 0, eventWarning: 0}
	shown := make([]v1.Event, 0, len(items))
	for i := range items {
		counts[items[i].Type]++
		if (f.Values["type"] == "" || items[i].Type == f.Values["type"]) && (f.Values["reason"] == "" || items[i].Reason == f.Values["reason"]) {
			shown = append(shown, items[i])
		}
	}
	empty := "No events in the last hour or so."
	if len(f.Active) > 0 && total > 0 {
		empty = "No events match this filter."
	}
	ev.Table = eventsTable{Problem: feed.Problem, Rows: make([]eventRow, 0, len(shown)), Empty: empty}
	for i := range shown {
		ev.Table.Rows = append(ev.Table.Rows, eventRow{Event: shown[i], Kind: shown[i].Regarding.Kind, Name: shown[i].Regarding.Name})
	}
	incomplete := feed.Problem != nil || len(ev.Unread) > 0
	count := total
	if feed.Problem != nil {
		count = -1
	}
	ev.Form = newChoiceForm(f, "Filter events", keepHidden(q, "type"), formInput{
		Counts: map[string]map[string]int{"type": counts}, NoCount: map[string]bool{"type": incomplete},
		Omit:  map[string]bool{"reason": true},
		Shown: len(shown), Total: count, ShownID: "events-shown", Follow: ev.Follow, Variant: filtersStrip,
	})
	return ev
}
