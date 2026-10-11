package ui

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

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
	// Origin says where the catalog comes from, in words.
	Origin string
	// NotResolvedReason is why a catalog is not in the registry: the
	// Platform's Ready reason for a subscription, or the claiming
	// registration's refusal reason.
	NotResolvedReason string
	NotResolvedWhy    string
	Contracts         *v1.Condition
	Claims            []catalogClaim
	// ClaimsLocked: the caller may not list the registrations, so the
	// claims, and whether there are any, are locked (portal:D7:R2/R3).
	ClaimsLocked string
	// ContributorHolders are the holders of the registration that
	// contributed the catalog, linked to their Provider tab.
	ContributorHolders []holderLink
	Tab                string
	Tabs               []tabLink
	Events             platformEvents
}

// catalogClaim is one registration claiming the catalog.
type catalogClaim struct {
	Reg     v1.Registration
	Holders []holderLink
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
	v := catalogViewOf(&p, rows[i])
	if r.URL.Query().Get("tab") == tabEvents {
		v.Tab = tabEvents
	}
	base := catalogHref(path)
	for _, t := range []struct{ Name, Label string }{{tabClaims, "Claims"}, {tabEvents, "Events"}} {
		tab := tabLink{Name: t.Name, Label: t.Label, Href: base + "&tab=" + t.Name, Current: v.Tab == t.Name}
		if t.Name == tabClaims {
			tab.Follow = "platform"
		}
		v.Tabs = append(v.Tabs, tab)
	}
	// Claims are counted when the caller may read every registration; the
	// Events tab is read only when it opens, so it carries no count.
	if v.ClaimsLocked == "" {
		setCount(v.Tabs, tabClaims, len(v.Claims))
	}
	topics := []string{"platform"}
	if v.Tab == tabEvents {
		v.Events = h.catalogEvents(r, v.claimants())
		topics = append(topics, strings.Fields(v.Events.Follow)...)
	}
	h.render(w, r, http.StatusOK, "catalog", page{Title: "Catalog " + path, Nav: "platform", Topics: topics, Main: v,
		Canonical: "/catalog?" + url.Values{"path": {path}}.Encode()})
}

// catalogViewOf is one catalog row with what the Platform says about it:
// its origin, why it is not resolved, the Platform-wide contracts and its
// claims.
func catalogViewOf(p *v1.Platform, row catalogRow) catalogView {
	v := catalogView{Row: row, Tab: tabClaims, Origin: originOf(row)}
	if p.RegistrationsAccess != v1.AccessOK {
		v.ClaimsLocked = p.RegistrationsAccess
	}
	st := statusOf(p)
	ready := st.Ready
	v.Contracts = st.Contracts
	for j := range p.Registrations {
		reg := p.Registrations[j]
		if reg.Catalog != row.Path {
			continue
		}
		cl := catalogClaim{Reg: reg}
		for _, ref := range reg.HeldBy {
			cl.Holders = append(cl.Holders, holderOf(ref))
		}
		v.Claims = append(v.Claims, cl)
		if reg.Name == row.ContributedBy {
			v.ContributorHolders = cl.Holders
		}
		if !row.Resolved && v.NotResolvedReason == "" && !reg.Accepted && reg.Reason != "" {
			v.NotResolvedReason, v.NotResolvedWhy = reg.Reason, "From the claim "+reg.Name+"."
		}
	}
	if !row.Resolved && v.NotResolvedReason == "" && ready != nil && ready.Status != "True" {
		v.NotResolvedReason, v.NotResolvedWhy = ready.Reason, "From the Platform's Ready condition."
	}
	return v
}

func originOf(row catalogRow) string {
	switch row.Source {
	case "subscription":
		return "Subscribed"
	case "registration":
		return "From a provider"
	case "claim":
		return "Claimed only"
	}
	return row.SourceText
}

func (v *catalogView) claimants() []string {
	out := make([]string, 0, len(v.Claims))
	for i := range v.Claims {
		out = append(out, v.Claims[i].Reg.Name)
	}
	return out
}

// catalogEvents merges the Platform's feed and its claimants', newest
// first.
func (h *Handler) catalogEvents(r *http.Request, claimants []string) platformEvents {
	ev := platformEvents{Filters: parseFilters(&filterView{Path: "/catalog"}, nil)}
	follow := []string{"events:platform"}
	feed := h.events(r, "/platform/events", nil, "")
	ev.Feed.Problem = feed.Problem
	items := feed.Items
	for _, n := range claimants {
		follow = append(follow, "events:registration:"+n)
		f := h.events(r, "/platform/registrations/"+url.PathEscape(n)+"/events", nil, "")
		if f.Problem != nil {
			ev.Unread = append(ev.Unread, n)
			continue
		}
		items = append(items, f.Items...)
	}
	slices.SortStableFunc(items, func(a, b v1.Event) int { return newestFirst(a.LastSeen, b.LastSeen) })
	ev.Feed.Items, ev.Total = items, len(items)
	ev.Follow = strings.Join(follow, " ")
	return ev
}
