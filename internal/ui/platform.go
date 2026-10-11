package ui

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// The Platform page (portal:D17): identity and status, the Installed
// counts that hand off to filtered views, the Providers and Catalogs tabs,
// and the recent events of the Platform and its readable registrations.
// No Platform health and no platform graph: the graph stays in the read
// API.

// Platform tabs.
const (
	tabProviders = "providers"
	tabCatalogs  = "catalogs"
)

// Registration verdicts and condition types the page reads.
const (
	verdictRefused        = "Refused"
	verdictPending        = "Pending"
	verdictRemovalBlocked = "RemovalBlocked"
	conditionReady        = "Ready"
	conditionContracts    = "ContractsFulfilled"
)

type platformView struct {
	Problem  *v1.Problem
	Platform v1.Platform
	Tab      string
	// Tabs open each tab, keeping every filter, with the counts the page
	// holds.
	Tabs []tabLink

	Status    platformStatus
	Installed installedCounts
	Providers providersTab
	Catalogs  catalogsTab
	Events    platformEvents
}

// --- Status ---

// platformStatus is the status card: Ready with its reason, and
// ContractsFulfilled beside it as information (portal:D3:R8).
type platformStatus struct {
	Ready      *v1.Condition
	Contracts  *v1.Condition
	Resolved   int
	Subscribed int
	Active     int
}

func statusOf(p *v1.Platform) platformStatus {
	st := platformStatus{Resolved: len(p.Catalogs), Subscribed: len(p.Subscriptions)}
	for i := range p.Conditions {
		switch c := &p.Conditions[i]; c.Type {
		case conditionReady:
			st.Ready = c
		case conditionContracts:
			st.Contracts = c
		}
	}
	for i := range p.Registrations {
		if p.Registrations[i].Active {
			st.Active++
		}
	}
	return st
}

// --- Installed counts ---

// countLink is one count that hands off to Installed filtered by it.
type countLink struct {
	Text  string
	Class string
	Href  string
	N     int
}

// installedCounts is the Installed card. A list the caller may not read
// makes its counts locked, never zero (portal:D7:R2).
type installedCounts struct {
	InstancesProblem *v1.Problem
	PackagesProblem  *v1.Problem
	InstancesLocked  bool
	PackagesLocked   bool
	Instances        int
	Packages         int
	Holders          int
	Health           []countLink
	Applied          []countLink
}

// Partial reports whether a list behind the counts could not be read.
func (c installedCounts) Partial() bool {
	return c.InstancesLocked || c.PackagesLocked || c.InstancesProblem != nil || c.PackagesProblem != nil
}

func (h *Handler) installedCounts(r *http.Request) installedCounts {
	var c installedCounts
	var instances v1.InstanceList
	var packages v1.PackageList
	c.InstancesProblem = h.fetch(r, "/instances", nil, &instances)
	c.PackagesProblem = h.fetch(r, "/packages", nil, &packages)
	c.InstancesLocked = c.InstancesProblem == nil && instances.Access != v1.AccessOK
	c.PackagesLocked = c.PackagesProblem == nil && packages.Access != v1.AccessOK
	health, applied := map[string]int{}, map[string]int{}
	count := func(rec v1.Reconcile, hl v1.Health, claims []v1.ProviderClaim) {
		health[hl.State]++
		applied[rec.State]++
		if len(claims) > 0 {
			c.Holders++
		}
	}
	for i := range instances.Items {
		it := &instances.Items[i]
		c.Instances++
		count(it.Reconcile, it.Health, it.ProviderOf)
	}
	for i := range packages.Items {
		it := &packages.Items[i]
		c.Packages++
		count(it.Reconcile, it.Health, it.ProviderOf)
	}
	for _, s := range healthValues {
		if n := health[s]; n > 0 {
			c.Health = append(c.Health, countLink{Text: healthText[s], Class: "health health-" + slug(s), N: n,
				Href: "/installed?" + url.Values{"health": {s}}.Encode()})
		}
	}
	for _, s := range appliedValues {
		if n := applied[s]; n > 0 {
			c.Applied = append(c.Applied, countLink{Text: appliedText[s], Class: "applied applied-" + slug(s), N: n,
				Href: "/installed?" + url.Values{"applied": {s}}.Encode()})
		}
	}
	return c
}

// --- Providers ---

// holderLink is an instance or package holding a registration.
type holderLink struct {
	Kind string // instance or package
	Href string
	Text string
}

// holderOf links a holder to its Provider tab.
func holderOf(ref v1.ObjectRef) holderLink {
	k := instanceKind
	if ref.Kind == "ModulePackage" {
		k = packageKind
	}
	return holderLink{Kind: k.Topic, Href: k.base(ref.Namespace, ref.Name) + "?tab=" + tabProvider, Text: ref.Namespace + "/" + ref.Name}
}

// providerRow is one registration with the instances and packages that
// hold it.
type providerRow struct {
	v1.Registration
	Holders []holderLink
	// RefElsewhere: spec.providerRef names a ModuleInstance no holder is,
	// so both are shown (portal:D15, portal:D4:R2).
	RefElsewhere bool
}

type providersTab struct {
	Form   filterForm
	Access string
	Rows   []providerRow
	Total  int
}

func providerRows(p *v1.Platform, f map[string]string) (rows []providerRow, total int) {
	for i := range p.Registrations {
		r := providerRow{Registration: p.Registrations[i]}
		r.RefElsewhere = r.Provider != nil
		for _, ref := range r.HeldBy {
			r.Holders = append(r.Holders, holderOf(ref))
			if r.Provider != nil && ref.Kind == r.Provider.Kind && ref.Namespace == r.Provider.Namespace && ref.Name == r.Provider.Name {
				r.RefElsewhere = false
			}
		}
		total++
		if matchesProvider(&r, f) {
			rows = append(rows, r)
		}
	}
	return rows, total
}

func matchesProvider(r *providerRow, f map[string]string) bool {
	if q := strings.ToLower(f["pq"]); q != "" {
		hay := make([]string, 0, 2+len(r.Provides)+len(r.Holders))
		hay = append(hay, r.Name, r.Catalog)
		hay = append(hay, r.Provides...)
		for _, h := range r.Holders {
			hay = append(hay, h.Text)
		}
		if !strings.Contains(strings.ToLower(strings.Join(hay, " ")), q) {
			return false
		}
	}
	if want := f["provides"]; want != "" && !slices.Contains(r.Provides, want) {
		return false
	}
	switch f["pstatus"] {
	case "active":
		return r.Active
	case "accepted":
		return r.Accepted
	case "refused":
		return r.Verdict == verdictRefused
	case "blocked":
		return r.Verdict == verdictRemovalBlocked
	case "pending":
		return r.Verdict == verdictPending
	}
	return true
}

// --- Catalogs ---

// claimant is a registration claiming a catalog, with its holders.
type claimant struct {
	Name    string
	Holders []holderLink
}

// catalogRow is one catalog the Platform subscribes to, holds in its
// resolved registry, or a readable registration claims.
type catalogRow struct {
	Path    string
	Version string
	// Source is subscription, registration or claim, as the csource
	// filter names it; SourceText says it in words.
	Source        string
	SourceText    string
	ContributedBy string
	Resolved      bool
	// Enabled is nil when the Platform says nothing about it.
	Enabled   *bool
	Claimants []claimant
}

type catalogsTab struct {
	Form  filterForm
	Rows  []catalogRow
	Total int
	// Contracts is the Platform's ContractsFulfilled, shown as information.
	Contracts *v1.Condition
}

func catalogRows(p *v1.Platform, f map[string]string) (rows []catalogRow, total int) {
	claims := map[string][]claimant{}
	for i := range p.Registrations {
		r := &p.Registrations[i]
		c := claimant{Name: r.Name}
		for _, ref := range r.HeldBy {
			c.Holders = append(c.Holders, holderOf(ref))
		}
		claims[r.Catalog] = append(claims[r.Catalog], c)
	}
	seen := map[string]bool{}
	var all []catalogRow
	for i := range p.Catalogs {
		c := &p.Catalogs[i]
		enabled := c.Enabled
		row := catalogRow{Path: c.Catalog, Version: c.Version, Resolved: true, Enabled: &enabled,
			ContributedBy: c.ContributedBy, Claimants: claims[c.Catalog]}
		switch c.Source {
		case "Subscription":
			row.Source, row.SourceText = "subscription", "subscribed"
		case "Registration":
			row.Source, row.SourceText = "registration", "from a provider"
		default:
			row.Source, row.SourceText = strings.ToLower(c.Source), c.Source
		}
		seen[c.Catalog] = true
		all = append(all, row)
	}
	for _, s := range p.Subscriptions {
		if seen[s.Catalog] {
			continue
		}
		seen[s.Catalog] = true
		all = append(all, catalogRow{Path: s.Catalog, Version: s.Version, Source: "subscription", SourceText: "subscribed",
			Enabled: s.Enable, Claimants: claims[s.Catalog]})
	}
	for i := range p.Registrations {
		r := &p.Registrations[i]
		if seen[r.Catalog] {
			continue
		}
		seen[r.Catalog] = true
		all = append(all, catalogRow{Path: r.Catalog, Version: r.Version, Source: "claim", SourceText: "claimed only",
			Claimants: claims[r.Catalog]})
	}
	for i := range all {
		if matchesCatalog(&all[i], f) {
			rows = append(rows, all[i])
		}
	}
	return rows, len(all)
}

func matchesCatalog(c *catalogRow, f map[string]string) bool {
	if q := strings.ToLower(f["cq"]); q != "" {
		hay := make([]string, 0, 2+len(c.Claimants))
		hay = append(hay, c.Path, c.ContributedBy)
		for _, cl := range c.Claimants {
			hay = append(hay, cl.Name)
		}
		if !strings.Contains(strings.ToLower(strings.Join(hay, " ")), q) {
			return false
		}
	}
	if want := f["csource"]; want != "" && c.Source != want {
		return false
	}
	if want := f["claimed"]; want != "" && (len(c.Claimants) > 0) != (want == "yes") {
		return false
	}
	return true
}

// --- Events ---

type optionGroup struct {
	Label   string
	Options []selectOption
}

// platformEvents is the merged feed of the Platform and each readable
// registration, newest first, with its resource filter.
type platformEvents struct {
	Filters filters
	Groups  []optionGroup
	Hidden  []hiddenField
	Feed    eventsView
	Total   int
	// Unread names the registrations whose feed could not be read.
	Unread []string
	Follow string
}

func (h *Handler) platformEvents(r *http.Request, p *v1.Platform) platformEvents {
	resource := filterParam{Name: "eresource", Label: "Resource", Values: []string{"platform"},
		Text: map[string]string{"platform": "the Platform"}}
	var names []string
	if p != nil {
		for i := range p.Registrations {
			n := p.Registrations[i].Name
			names = append(names, n)
			resource.Values = append(resource.Values, "registration:"+n)
			resource.Text["registration:"+n] = n
		}
	}
	view := filterView{Path: "/", Params: []filterParam{resource}}
	ev := platformEvents{Filters: parseFilters(&view, r.URL.Query()), Hidden: keepHidden(r.URL.Query(), "eresource")}
	want := ev.Filters.Values["eresource"]

	follow := []string{"events:platform"}
	platform := h.events(r, "/platform/events", nil, "")
	ev.Feed.Problem = platform.Problem
	items := platform.Items
	ev.Total = len(platform.Items)
	if want != "" && want != "platform" {
		items = nil
	}
	groups := []optionGroup{{Label: "Platform", Options: []selectOption{{Value: "platform", Text: "the Platform", Selected: want == "platform"}}}}
	providers := optionGroup{Label: "Providers"}
	for _, n := range names {
		follow = append(follow, "events:registration:"+n)
		providers.Options = append(providers.Options, selectOption{Value: "registration:" + n, Text: n, Selected: want == "registration:"+n})
		feed := h.events(r, "/platform/registrations/"+url.PathEscape(n)+"/events", nil, "")
		if feed.Problem != nil {
			ev.Unread = append(ev.Unread, n)
			continue
		}
		ev.Total += len(feed.Items)
		if want == "" || want == "registration:"+n {
			items = append(items, feed.Items...)
		}
	}
	if len(providers.Options) > 0 {
		groups = append(groups, providers)
	}
	ev.Groups = groups
	slices.SortStableFunc(items, func(a, b v1.Event) int { return newestFirst(a.LastSeen, b.LastSeen) })
	ev.Feed.Items = items
	ev.Follow = strings.Join(follow, " ")
	return ev
}

// --- The page ---

func (h *Handler) platformPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	v := platformView{Tab: tabProviders}
	if q.Get("tab") == tabCatalogs {
		v.Tab = tabCatalogs
	}
	v.Tabs = []tabLink{
		{Name: tabProviders, Label: "Providers", Href: linkWith("/", q, "tab", tabProviders), Current: v.Tab == tabProviders},
		{Name: tabCatalogs, Label: "Catalogs", Href: linkWith("/", q, "tab", tabCatalogs), Current: v.Tab == tabCatalogs},
	}
	v.Problem = h.fetch(r, "/platform", nil, &v.Platform)
	if unauthenticated(v.Problem) {
		h.signIn(w, r)
		return
	}
	v.Installed = h.installedCounts(r)
	pf := parseFilters(&providerFilters, q)
	cf := parseFilters(&catalogFilters, q)
	var plat *v1.Platform
	if v.Problem == nil {
		plat = &v.Platform
		v.Status = statusOf(plat)
		v.Providers.Access = v.Platform.RegistrationsAccess
		v.Providers.Rows, v.Providers.Total = providerRows(plat, pf.Values)
		if v.Platform.RegistrationsAccess != v1.AccessOK {
			// Claims come from the registrations, which the caller may not
			// read: whether a catalog is claimed is locked, not "no"
			// (portal:D7:R2/R3).
			cf = withoutFilter(cf, "claimed")
		}
		v.Catalogs.Rows, v.Catalogs.Total = catalogRows(plat, cf.Values)
		v.Catalogs.Contracts = v.Status.Contracts
		if v.Platform.RegistrationsAccess == v1.AccessOK {
			// Both lists come from the registrations; without them a
			// count would be short (portal:D7).
			setCount(v.Tabs, tabProviders, v.Providers.Total, "platform")
			setCount(v.Tabs, tabCatalogs, v.Catalogs.Total, "platform")
		}
	}
	var provides []string
	for i := range v.Platform.Registrations {
		for _, c := range v.Platform.Registrations[i].Provides {
			provides = appendUnique(provides, c)
		}
	}
	slices.Sort(provides)
	v.Providers.Form = newFilterForm(pf, "Filter providers", keepHidden(q, providerFilters.paramNames()...),
		map[string][]string{"provides": provides},
		map[string]string{"pq": "Name, catalog, contract or holder", "provides": "a contract"})
	v.Catalogs.Form = newFilterForm(cf, "Filter catalogs", keepHidden(q, catalogFilters.paramNames()...),
		nil, map[string]string{"cq": "Catalog path or claimant"})
	v.Events = h.platformEvents(r, plat)

	follow := strings.Fields(v.Events.Follow)
	topics := make([]string, 0, 2+len(follow))
	topics = append(topics, "platform", "instances")
	topics = append(topics, follow...)
	h.render(w, r, problemStatus(v.Problem), "platform", page{
		Title:  "Platform",
		Nav:    "platform",
		Topics: topics,
		Main:   v,
	})
}

// withoutFilter moves an applied filter to the ignored ones.
func withoutFilter(f filters, name string) filters {
	if _, ok := f.Values[name]; !ok {
		return f
	}
	delete(f.Values, name)
	active := f.Active[:0:0]
	for _, c := range f.Active {
		if c.Name == name {
			f.Ignored = append(f.Ignored, c)
			continue
		}
		active = append(active, c)
	}
	f.Active = active
	return f
}

// contractShort names a contract by its last two path segments and its
// version, as the pages show it; the full name goes in a title.
func contractShort(c string) string {
	parts := strings.Split(c, "/")
	if len(parts) <= 2 {
		return c
	}
	return strings.Join(parts[len(parts)-2:], "/")
}
