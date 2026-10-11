package ui

import (
	"fmt"
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
	condTrue              = "True"
	sourceSubscription    = "subscription"
	stateFailed           = "Failed"
	eventPlatform         = "platform" // the eresource value of the Platform's own events
)

type platformView struct {
	Problem  *v1.Problem
	Platform v1.Platform
	Tab      string
	// Tabs open each tab, keeping every filter, with the counts the page
	// holds.
	Tabs []tabLink

	Status    platformStatus
	Block     stateBlock
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

// reconcileHue is the hue the Platform's reconcile state takes on the status
// block: the icon and the tone follow it.
func reconcileHue(state string) string {
	switch state {
	case "Applied":
		return hueAppliedWord
	case "Reconciling":
		return "progressing"
	case stateFailed, "Stalled":
		return "degraded"
	case "Suspended", "ManagedExternally":
		return "neutral"
	}
	return unknownWord
}

// platformStatusBlock builds the status block of the Platform page
// (platform-04, platform-05): the applied word with its "since" time, the
// line of catalogs and claims, and the reasons that need attention, each a
// count and a reason linking to its cause. With registrations the caller may
// not list, the claims part reads "claims locked", no refusal count shows
// (never a zero) and a locked note line says so, whatever other reasons the
// block holds (portal:D7:R2).
func platformStatusBlock(p *v1.Platform) stateBlock {
	st := statusOf(p)
	locked := p.RegistrationsAccess != v1.AccessOK
	b := stateBlock{
		ID: "platform-status", Label: "Platform status", Eyebrow: "Platform · the controller",
		Hue: reconcileHue(p.Reconcile.State), State: appliedBadge(p.Reconcile).Text,
		None: "Nothing refused or unfulfilled", Follow: platformTopic,
	}
	if p.Reconcile.Since != nil {
		b.When, b.WhenText = p.Reconcile.Since, sinceWord
	}
	claims := fmt.Sprintf("%d %s active", st.Active, plural(st.Active, "claim", "claims"))
	if locked {
		claims = "claims locked"
	}
	b.Summary = fmt.Sprintf("%d %s resolved from %d %s · %s", st.Resolved, plural(st.Resolved, "catalog", "catalogs"),
		st.Subscribed, plural(st.Subscribed, "subscription", "subscriptions"), claims)
	if locked {
		b.Notes = append(b.Notes, sbNote{Text: "Refusals not shown: you may not list TransformerRegistrations", Class: hueLocked})
	} else {
		b.Reasons = append(b.Reasons, verdictReasons(p, verdictRefused, "refused", "hue-degraded")...)
		b.Reasons = append(b.Reasons, verdictReasons(p, verdictRemovalBlocked, "blocked", "hue-unknown")...)
	}
	if c := st.Ready; c != nil && c.Status != condTrue {
		b.Reasons = append(b.Reasons, countLink{Text: reasonOf(c), Class: "hue-degraded", Href: "#cond-Ready"})
	}
	if c := st.Contracts; c != nil && c.Status == "False" {
		b.Reasons = append(b.Reasons, countLink{Text: reasonOf(c), Class: "hue-unknown", Href: "/?tab=catalogs#contracts"})
	}
	return newStateBlock(b)
}

func reasonOf(c *v1.Condition) string {
	if c.Reason != "" {
		return c.Reason
	}
	return c.Status
}

// verdictReasons counts the registrations of one verdict per reason, each a
// link to the Providers tab filtered to that verdict, most counted first.
func verdictReasons(p *v1.Platform, verdict, filter, class string) []countLink {
	n := map[string]int{}
	for i := range p.Registrations {
		if r := &p.Registrations[i]; r.Verdict == verdict {
			n[r.Reason]++
		}
	}
	out := make([]countLink, 0, len(n))
	for reason, c := range n {
		text := reason
		if text == "" {
			text = verdictText[verdict]
		}
		out = append(out, countLink{Text: text, Class: class, N: c, Href: "/?tab=providers&pstatus=" + filter + "#parts"})
	}
	slices.SortFunc(out, func(a, b countLink) int {
		if a.N != b.N {
			return b.N - a.N
		}
		return strings.Compare(a.Text, b.Text)
	})
	return out
}

// --- Installed counts ---

// countLink is one count that hands off to Installed filtered by it.
type countLink struct {
	Text  string
	Class string
	Href  string
	N     int
	// Title is the link's title, "Show the N installed with health X".
	Title string
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
				Href:  "/installed?" + url.Values{"health": {s}}.Encode(),
				Title: fmt.Sprintf("Show the %d installed with health %s", n, healthText[s])})
		}
	}
	for _, s := range appliedValues {
		if n := applied[s]; n > 0 {
			c.Applied = append(c.Applied, countLink{Text: appliedText[s], Class: "applied applied-" + slug(s), N: n,
				Href:  "/installed?" + url.Values{"applied": {s}}.Encode(),
				Title: fmt.Sprintf("Show the %d installed with applied state %s", n, appliedText[s])})
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
	// NameHref is the first holder's Provider tab, "" when nothing holds it.
	NameHref string
	// Badges are acceptance, activation and, for a removal-blocked
	// registration, the third; Why is the reason line's hue class.
	Badges []sqBadge
	Why    string
}

// sqBadge is one square badge: its hue class and its words.
type sqBadge struct{ Hue, Text string }

// registrationBadges are a registration's acceptance, then its activation,
// then "Removal blocked" beside them only for that verdict (portal:D4:R4/R7).
// A removal-blocked registration stays accepted and active, never refused.
func registrationBadges(r *v1.Registration) []sqBadge {
	acc := sqBadge{"healthy", "Accepted"}
	if !r.Accepted {
		switch r.Verdict {
		case verdictRefused:
			acc = sqBadge{"degraded", "Refused"}
		case verdictPending:
			acc = sqBadge{"progressing", "Pending"}
		default:
			acc = sqBadge{"neutral", "Not accepted"}
		}
	}
	act := sqBadge{"neutral", "Inactive"}
	if r.Active {
		act = sqBadge{"healthy", "Active"}
	}
	out := []sqBadge{acc, act}
	if r.Verdict == verdictRemovalBlocked {
		out = append(out, sqBadge{unknownWord, "Removal blocked"})
	}
	return out
}

// whyHue is the hue of a registration's reason line.
func whyHue(verdict string) string {
	switch verdict {
	case verdictRefused:
		return "hue-degraded"
	case verdictRemovalBlocked:
		return "hue-unknown"
	case verdictPending:
		return "hue-progressing"
	}
	return "hue-neutral"
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
		if len(r.Holders) > 0 {
			r.NameHref = r.Holders[0].Href
		}
		r.Badges = registrationBadges(&p.Registrations[i])
		r.Why = whyHue(r.Verdict)
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
	case "inactive":
		return r.Accepted && !r.Active
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
			row.Source, row.SourceText = sourceSubscription, "subscription"
		case "Registration":
			row.Source, row.SourceText = "registration", "contributed"
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
		all = append(all, catalogRow{Path: s.Catalog, Version: s.Version, Source: sourceSubscription, SourceText: "subscription",
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

// eventsTable is what the "events-table" partial draws: the rows of a feed
// as a table, with a Resource column where the feed mixes objects.
type eventsTable struct {
	Problem      *v1.Problem
	Rows         []eventRow
	ShowResource bool
	// Empty is the words of an empty feed.
	Empty string
}

// eventRow is one event with the object it is about, and the link that
// filters the feed to that object ("" when the object is not a filter).
type eventRow struct {
	v1.Event
	Kind, Name   string
	ResourceHref string
}

// platformEvents is the merged feed of the Platform and each readable
// registration, newest first, with its resource filter.
type platformEvents struct {
	Filters filters
	Groups  []optionGroup
	Hidden  []hiddenField
	Feed    eventsView
	Table   eventsTable
	Total   int
	// Unread names the registrations whose feed could not be read.
	Unread []string
	Follow string
}

// eventResource is the eresource value of the object an event is about:
// the Platform, or a registration; "" for any other object.
func eventResource(ref v1.ObjectRef) string {
	switch ref.Kind {
	case "Platform":
		return eventPlatform
	case "TransformerRegistration":
		return "registration:" + ref.Name
	}
	return ""
}

func (h *Handler) platformEvents(r *http.Request, p *v1.Platform) platformEvents {
	platformName := "the Platform"
	if p != nil && p.Name != "" {
		platformName = p.Name
	}
	resource := filterParam{Name: "eresource", Label: "Resource", Values: []string{eventPlatform},
		Text: map[string]string{eventPlatform: "the Platform"}}
	var names []string
	if p != nil {
		for i := range p.Registrations {
			n := p.Registrations[i].Name
			names = append(names, n)
			resource.Values = append(resource.Values, "registration:"+n)
			resource.Text["registration:"+n] = n
		}
	}
	q := r.URL.Query()
	view := filterView{Path: "/", Params: []filterParam{resource}}
	ev := platformEvents{Filters: parseFilters(&view, q), Hidden: keepHidden(q, "eresource")}
	want := ev.Filters.Values["eresource"]

	follow := []string{"events:platform"}
	platform := h.events(r, "/platform/events", nil, "")
	ev.Feed.Problem = platform.Problem
	items := platform.Items
	ev.Total = len(platform.Items)
	if want != "" && want != eventPlatform {
		items = nil
	}
	platformOpt := selectOption{Value: eventPlatform, Text: platformName, Selected: want == eventPlatform}
	if platform.Problem == nil {
		platformOpt.Text += fmt.Sprintf(" (%d)", len(platform.Items))
	}
	providers := optionGroup{Label: "Providers (TransformerRegistration)"}
	for _, n := range names {
		follow = append(follow, "events:registration:"+n)
		opt := selectOption{Value: "registration:" + n, Text: n, Selected: want == "registration:"+n}
		feed := h.events(r, "/platform/registrations/"+url.PathEscape(n)+"/events", nil, "")
		if feed.Problem != nil {
			// A registration whose feed was not read is offered with no count.
			ev.Unread = append(ev.Unread, n)
			providers.Options = append(providers.Options, opt)
			continue
		}
		opt.Text += fmt.Sprintf(" (%d)", len(feed.Items))
		providers.Options = append(providers.Options, opt)
		ev.Total += len(feed.Items)
		if want == "" || want == "registration:"+n {
			items = append(items, feed.Items...)
		}
	}
	all := selectOption{Value: "", Text: "All resources", Selected: want == ""}
	if len(ev.Unread) == 0 && platform.Problem == nil {
		all.Text += fmt.Sprintf(" (%d)", ev.Total)
	}
	groups := []optionGroup{{Label: "Everything", Options: []selectOption{all}}, {Label: "Platform", Options: []selectOption{platformOpt}}}
	if len(providers.Options) > 0 {
		groups = append(groups, providers)
	}
	ev.Groups = groups
	slices.SortStableFunc(items, func(a, b v1.Event) int { return newestFirst(a.LastSeen, b.LastSeen) })
	ev.Feed.Items = items
	ev.Table = eventsTableOf(items, ev.Feed.Problem, q)
	ev.Follow = strings.Join(follow, " ")
	return ev
}

// eventsTableOf builds the table of a feed: each row links to the filter that
// shows only its object's events, keeping the other query parameters.
func eventsTableOf(items []v1.Event, problem *v1.Problem, q url.Values) eventsTable {
	t := eventsTable{Problem: problem, ShowResource: true, Empty: "No events in the last hour or so.", Rows: make([]eventRow, 0, len(items))}
	for i := range items {
		row := eventRow{Event: items[i], Kind: items[i].Regarding.Kind, Name: items[i].Regarding.Name}
		if v := eventResource(items[i].Regarding); v != "" {
			row.ResourceHref = linkWith("/", q, "eresource", v)
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// --- The page ---

func (h *Handler) platformPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	v := platformView{Tab: tabProviders}
	if q.Get("tab") == tabCatalogs {
		v.Tab = tabCatalogs
	}
	v.Tabs = []tabLink{
		{Name: tabProviders, Label: "Providers", Href: linkWith("/", q, "tab", tabProviders), Current: v.Tab == tabProviders, Follow: platformTopic},
		{Name: tabCatalogs, Label: "Catalogs", Href: linkWith("/", q, "tab", tabCatalogs), Current: v.Tab == tabCatalogs, Follow: platformTopic},
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
		v.Block = platformStatusBlock(plat)
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
			setCount(v.Tabs, tabProviders, v.Providers.Total)
			setCount(v.Tabs, tabCatalogs, v.Catalogs.Total)
		}
	}
	provides, provCounts := providerCounts(&v.Platform)
	allCatalogs, _ := catalogRows(&v.Platform, nil)
	catCounts := catalogCounts(&v.Platform, allCatalogs)
	regsLocked := v.Platform.RegistrationsAccess != v1.AccessOK
	// A count row stands for rows the page shows: none when the source is a
	// problem, and none for providers the caller may not list (portal:D7:R2).
	provShown, provTotal := len(v.Providers.Rows), v.Providers.Total
	catShown, catTotal := len(v.Catalogs.Rows), v.Catalogs.Total
	if v.Problem != nil {
		provTotal, catTotal = -1, -1
	} else if v.Providers.Access != v1.AccessOK {
		provTotal = -1
	}
	v.Providers.Form = newChoiceForm(pf, "Filter providers", keepHidden(q, providerFilters.paramNames()...), formInput{
		Choices: map[string][]string{"provides": provides}, Counts: provCounts,
		NoCount:      map[string]bool{"pstatus": regsLocked, "provides": regsLocked},
		Placeholders: map[string]string{"pq": "Name, catalog, contract or holder"},
		Shown:        provShown, Total: provTotal, ShownID: "shown", Follow: platformTopic, Variant: filtersStrip,
	})
	v.Catalogs.Form = newChoiceForm(cf, "Filter catalogs", keepHidden(q, catalogFilters.paramNames()...), formInput{
		Counts: catCounts, NoCount: map[string]bool{"claimed": regsLocked, "csource": v.Problem != nil}, Omit: map[string]bool{"claimed": regsLocked},
		Placeholders: map[string]string{"cq": "Catalog path or claimant"},
		Shown:        catShown, Total: catTotal, ShownID: "shown", Follow: platformTopic, Variant: filtersStrip,
	})
	v.Events = h.platformEvents(r, plat)

	follow := strings.Fields(v.Events.Follow)
	topics := make([]string, 0, 2+len(follow))
	topics = append(topics, platformTopic, "instances")
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

// providerCounts are the contracts the registrations provide, sorted, and the
// counts of each Status and Provides option over every registration the
// Platform document holds. A count is only meaningful when the registrations
// are all readable; the caller of this function marks the options NoCount
// otherwise (portal:D7).
func providerCounts(p *v1.Platform) (provides []string, counts map[string]map[string]int) {
	counts = map[string]map[string]int{"pstatus": {}, "provides": {}}
	for _, v := range providerFilters.Params[1].Values {
		counts["pstatus"][v] = 0 // a fixed value with no rows reads (0) when the list is readable
	}
	for i := range p.Registrations {
		r := &p.Registrations[i]
		if r.Accepted {
			counts["pstatus"]["accepted"]++
		}
		if r.Active {
			counts["pstatus"]["active"]++
		}
		if r.Accepted && !r.Active {
			counts["pstatus"]["inactive"]++
		}
		switch r.Verdict {
		case verdictRefused:
			counts["pstatus"]["refused"]++
		case verdictRemovalBlocked:
			counts["pstatus"]["blocked"]++
		case verdictPending:
			counts["pstatus"]["pending"]++
		}
		for _, c := range r.Provides {
			provides = appendUnique(provides, c)
			counts["provides"][c]++
		}
	}
	slices.Sort(provides)
	return provides, counts
}

// catalogCounts are the counts of the Source and Claimed options over every
// catalog row. With registrations the caller may not read, claimed-only rows
// and claims are unknown: those options carry no count.
func catalogCounts(p *v1.Platform, rows []catalogRow) map[string]map[string]int {
	counts := map[string]map[string]int{"csource": {}, "claimed": {}}
	for _, v := range catalogFilters.Params[1].Values {
		counts["csource"][v] = 0
	}
	counts["claimed"]["yes"], counts["claimed"]["no"] = 0, 0
	locked := p.RegistrationsAccess != v1.AccessOK
	for i := range rows {
		c := &rows[i]
		if !locked || c.Source != "claim" {
			counts["csource"][c.Source]++
		}
		if !locked {
			if len(c.Claimants) > 0 {
				counts["claimed"]["yes"]++
			} else {
				counts["claimed"]["no"]++
			}
		}
	}
	if locked {
		delete(counts["csource"], "claim")
	}
	return counts
}
