package ui

import (
	"fmt"
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
	// Intro is the sentence that opens the claim: the item is also a
	// provider, and what the controller's verdict adds to that.
	Intro string
	// ShowReason: the reason is drawn beside the standing only when the
	// claim is not accepted and active, so a success reason is not said
	// twice.
	ShowReason bool
	// ContractsText is the Contracts fact; there is one text for every
	// standing, so none falls through to the counts (OP-A4).
	ContractsText string
	// Provides: the standing is one under which the claim serves its
	// contracts, so the table draws their users. Otherwise the table lists
	// them as not provided, or says there is nothing when none is listed.
	Provides bool
	// NothingText is what "What it provides" says in place of a table.
	NothingText string
}

// usedBy is one provided contract and the instances whose render used it.
type usedBy struct {
	Contract string
	First    []holderLink
	More     int
	Total    int
	All      string
	// Button is the label of the pill that opens Installed filtered by the
	// contract, empty when there is none to offer; ButtonTitle is its title.
	Button      string
	ButtonTitle string
	// Empty: the list is complete and no readable instance uses the
	// contract.
	Empty bool
	// Note says why the claim serves no user of this contract, for a claim
	// that is not providing.
	Note string
}

func catalogHref(path string) string { return "/catalog?" + url.Values{"path": {path}}.Encode() }

// providerTab reads what the Provider tab shows for v's held claims.
func (h *Handler) providerTab(r *http.Request, v *ownerView) providerTabView {
	var t providerTabView
	p := v.plat.Doc
	t.PlatformLocked = v.plat.Locked
	t.PlatformProblem = v.plat.Problem
	items := h.usedByInstances(r, &t, v, &p)
	return claimsOf(t, v, &p, items)
}

// claimsOf fills t.Claims from the owner's held claims, the Platform
// document and the instances "Used by" reads. t carries what the reads
// found out: the Platform locked or failed, and the users' lists locked or
// incomplete.
func claimsOf(t providerTabView, v *ownerView, p *v1.Platform, items []v1.InstanceSummary) providerTabView {
	for _, c := range v.ProviderOf {
		cv := claimView{Claim: c}
		cv.Provides = servesContracts(claimStanding(c))
		cv.Intro = providerIntro(v.Kind.Topic, c)
		cv.ShowReason = c.Verdict != verdictAccepted || !c.Active
		if t.PlatformProblem == nil && !t.PlatformLocked {
			cv.Reg = findRegistration(p, c.Registration)
		}
		if cv.Reg != nil {
			cv.fillFromRegistration(&t, p, items)
		}
		t.Claims = append(t.Claims, cv)
	}
	return t
}

func findRegistration(p *v1.Platform, name string) *v1.Registration {
	for i := range p.Registrations {
		if p.Registrations[i].Name == name {
			return &p.Registrations[i]
		}
	}
	return nil
}

// servesContracts: under this standing the claim serves its contracts.
func servesContracts(standing string) bool {
	return standing == standingProviding || standing == standingBlocked
}

// fillFromRegistration adds what the Platform document says about the
// claim's registration: its catalog and registry state, its contracts and the
// Contracts fact.
func (cv *claimView) fillFromRegistration(t *providerTabView, p *v1.Platform, items []v1.InstanceSummary) {
	st := claimStanding(cv.Claim)
	incomplete := t.UsedByLocked || t.UsedByIncomplete || t.UsedByProblem != nil
	cv.CatalogHref = catalogHref(cv.Reg.Catalog)
	for i := range p.Catalogs {
		if cat := &p.Catalogs[i]; cat.Catalog == cv.Reg.Catalog {
			cv.InRegistry = true
			cv.ContributedBy = cat.ContributedBy
			cv.ContributedHere = cat.ContributedBy == cv.Claim.Registration
		}
	}
	for _, contract := range cv.Reg.Provides {
		switch {
		case cv.Provides:
			cv.Contracts = append(cv.Contracts, usedByOf(contract, items, t.UsedByLocked, incomplete))
		case st == standingInactive || st == standingPending:
			cv.Contracts = append(cv.Contracts, usedBy{Contract: contract, Note: notProvidedNote(st)})
		}
	}
	cv.ContractsText = contractsText(st, len(cv.Reg.Provides), inUse(cv.Contracts), incomplete)
	cv.NothingText = nothingText(st, len(cv.Contracts))
}

// readPlatform reads the Platform for the Provider card and the Provider tab.
func (h *Handler) readPlatform(r *http.Request) platformRead {
	var pr platformRead
	prob := h.fetch(r, "/platform", nil, &pr.Doc)
	switch {
	case prob == nil:
		pr.Read = true
	case prob.Code == v1.CodeForbidden:
		pr.Locked = true
	default:
		pr.Problem = prob
	}
	return pr
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

func usedByOf(contract string, items []v1.InstanceSummary, locked, incomplete bool) usedBy {
	u := usedBy{Contract: contract, All: "/installed?" + url.Values{"uses": {contract}}.Encode()}
	u.ButtonTitle = "Open Installed filtered to everything that uses " + contract
	if locked {
		u.Button = "All in Installed \u2192"
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
	switch {
	case u.Total == 0 && !incomplete:
		u.Empty = true
	case u.Total <= usedByShown:
		u.Button = "Show in Installed \u2192"
	default:
		u.Button = fmt.Sprintf("All %d in Installed \u2192", u.Total)
	}
	return u
}

// The standings of a claim, as far as the Provider tab tells them apart.
const (
	standingProviding = "providing"
	standingBlocked   = "removal blocked"
	standingInactive  = "accepted, not active"
	standingPending   = "pending"
	standingRefused   = "refused"
	standingUnknown   = "unknown"
)

// claimStanding sorts a claim by the verdict the controller wrote. A verdict
// the UI does not know is unknown, never a standing that provides.
func claimStanding(c v1.ProviderClaim) string {
	switch {
	case c.Verdict == verdictRemovalBlocked:
		return standingBlocked
	case c.Verdict == verdictAccepted && c.Active:
		return standingProviding
	case c.Verdict == verdictAccepted:
		return standingInactive
	case c.Verdict == verdictPending:
		return standingPending
	case c.Verdict == verdictRefused:
		return standingRefused
	}
	return standingUnknown
}

// providerIntro opens a claim: the item is also a provider, and what the
// controller's own verdict adds to that (portal:D15:R2/R4). kind is the
// owner's topic, instance or package.
func providerIntro(kind string, c v1.ProviderClaim) string {
	lead := "This " + kind + " is also a provider: it registers a catalog with the platform"
	if c.Access != v1.AccessOK {
		return lead + ". You may not read this registration, so its standing is not shown."
	}
	switch claimStanding(c) {
	case standingProviding, standingBlocked:
		return lead + ", and modules that use the contracts below render through it."
	case standingInactive:
		return lead + ". The controller accepted the claim, but it is not active, so it provides nothing yet."
	case standingPending:
		return lead + ". The controller has not decided on the claim yet, so it provides nothing yet."
	case standingRefused:
		return lead + ". The controller refused the claim, so it provides nothing."
	}
	return lead + ". The portal does not know this claim's standing."
}

// notProvidedNote is the Used by cell of a contract a claim lists but does
// not serve yet.
func notProvidedNote(standing string) string {
	if standing == standingPending {
		return "Not provided until the controller decides."
	}
	return "Not provided until the claim is active."
}

// inUse counts the contracts with at least one readable user.
func inUse(cs []usedBy) int {
	n := 0
	for i := range cs {
		if cs[i].Total > 0 {
			n++
		}
	}
	return n
}

// contractsText is the Contracts fact for every standing (OP-A4): n is how
// many contracts the registration lists, used how many of them a readable
// instance's render used. A claim that provides nothing never reads
// "N provided, M in use", and use is not counted when a list of users is
// incomplete or locked, since the count would be too low.
func contractsText(standing string, n, used int, incomplete bool) string {
	counts := func() string {
		if incomplete {
			return fmt.Sprintf("%d provided; use not fully readable", n)
		}
		return fmt.Sprintf("%d provided, %d in use", n, used)
	}
	switch standing {
	case standingProviding:
		return counts()
	case standingBlocked:
		return counts() + "; removal is blocked"
	case standingInactive:
		return fmt.Sprintf("%d listed; none provided until the claim is active", n)
	case standingPending:
		return fmt.Sprintf("%d listed; none provided until the controller decides", n)
	case standingRefused:
		return "none: the claim is refused"
	}
	return "not known: the portal does not know this claim's standing"
}

// nothingText is what "What it provides" says when it has no row to draw.
func nothingText(standing string, rows int) string {
	if rows > 0 {
		return ""
	}
	switch standing {
	case standingRefused:
		return "Nothing. The controller refused the claim, so it provides no contract."
	case standingInactive, standingPending:
		return "Nothing yet. The registration lists no contract."
	case standingProviding, standingBlocked:
		return "The registration provides no contract."
	}
	return "Not known: the portal does not know this claim's standing."
}
