package ui

import (
	"net/http"
	"net/url"
	"slices"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// The Provider tab, on any instance or package whose inventory holds a
// TransformerRegistration (portal:D15): per held registration, its claim
// and the controller's standing, the catalog and whether the Platform's
// registry holds it, its conditions, and per provided contract the
// readable instances whose render used it (portal:D16:R3).

const tabProvider = "provider"

// usedByShown is how many instances "Used by" names before it counts the
// rest.
const usedByShown = 3

type providerTabView struct {
	// PlatformLocked: the caller may not read the Platform, so each claim's
	// conditions, catalog and registry state are locked.
	PlatformLocked  bool
	PlatformProblem *v1.Problem
	// UsedByIncomplete: the caller may not list every instance, so "Used
	// by" may lack some; UsedByLocked: it may list none; UsedByProblem: a
	// list failed for another reason.
	UsedByIncomplete bool
	UsedByLocked     bool
	UsedByProblem    *v1.Problem
	Claims           []claimView
}

// claimView is one held registration on the Provider tab.
type claimView struct {
	Claim v1.ProviderClaim
	// Reg is the registration as the Platform document carries it; nil when
	// the caller may not read it there.
	Reg         *v1.Registration
	CatalogHref string
	// InRegistry: the catalog is in the Platform's resolved registry;
	// ContributedHere: this registration contributed it.
	InRegistry      bool
	ContributedHere bool
	ContributedBy   string
	Contracts       []usedBy
}

// usedBy is one provided contract and the instances whose render used it.
type usedBy struct {
	Contract string
	First    []holderLink
	More     int
	Total    int
	All      string
}

func catalogHref(path string) string { return "/catalog?" + url.Values{"path": {path}}.Encode() }

// providerTab reads what the Provider tab shows for v's held claims.
func (h *Handler) providerTab(r *http.Request, v *ownerView) providerTabView {
	var t providerTabView
	var p v1.Platform
	if prob := h.fetch(r, "/platform", nil, &p); prob != nil {
		t.PlatformLocked = prob.Code == v1.CodeForbidden
		if !t.PlatformLocked {
			t.PlatformProblem = prob
		}
	}
	items := h.usedByInstances(r, &t, v, &p)
	for _, c := range v.ProviderOf {
		cv := claimView{Claim: c}
		if t.PlatformProblem == nil && !t.PlatformLocked {
			for i := range p.Registrations {
				if p.Registrations[i].Name == c.Registration {
					cv.Reg = &p.Registrations[i]
				}
			}
		}
		if cv.Reg != nil {
			cv.CatalogHref = catalogHref(cv.Reg.Catalog)
			for i := range p.Catalogs {
				if cat := &p.Catalogs[i]; cat.Catalog == cv.Reg.Catalog {
					cv.InRegistry = true
					cv.ContributedBy = cat.ContributedBy
					cv.ContributedHere = cat.ContributedBy == c.Registration
				}
			}
			for _, contract := range cv.Reg.Provides {
				cv.Contracts = append(cv.Contracts, usedByOf(contract, items, t.UsedByLocked))
			}
		}
		t.Claims = append(t.Claims, cv)
	}
	return t
}

// usedByInstances lists the instances "Used by" reads: every instance the
// caller may list cluster-wide, or, when it may not, those of each
// namespace it may list among the owner's and the holders' (portal:D7:R2,
// portal:D16:R3).
func (h *Handler) usedByInstances(r *http.Request, t *providerTabView, v *ownerView, p *v1.Platform) []v1.InstanceSummary {
	var all v1.InstanceList
	prob := h.fetch(r, "/instances", nil, &all)
	switch {
	case prob == nil && all.Access == v1.AccessOK:
		return all.Items
	case prob != nil && prob.Code != v1.CodeForbidden:
		t.UsedByProblem = prob
		return nil
	}
	namespaces := []string{v.Namespace}
	for i := range p.Registrations {
		for _, ref := range p.Registrations[i].HeldBy {
			namespaces = appendUnique(namespaces, ref.Namespace)
		}
	}
	var items []v1.InstanceSummary
	listed := false
	for _, ns := range namespaces {
		var list v1.InstanceList
		if h.fetch(r, "/instances", nsQuery(ns), &list) == nil && list.Access == v1.AccessOK {
			listed = true
			items = append(items, list.Items...)
		}
	}
	t.UsedByLocked = !listed
	t.UsedByIncomplete = listed
	return items
}

func usedByOf(contract string, items []v1.InstanceSummary, locked bool) usedBy {
	u := usedBy{Contract: contract, All: "/installed?" + url.Values{"uses": {contract}}.Encode()}
	if locked {
		return u
	}
	for i := range items {
		it := &items[i]
		if !slices.Contains(it.RenderContracts, contract) {
			continue
		}
		u.Total++
		if len(u.First) < usedByShown {
			u.First = append(u.First, holderLink{Kind: instanceKind.Topic, Href: instanceKind.base(it.Ref.Namespace, it.Ref.Name),
				Text: it.Ref.Namespace + "/" + it.Ref.Name})
		}
	}
	u.More = u.Total - len(u.First)
	return u
}
