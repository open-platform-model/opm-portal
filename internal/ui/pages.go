package ui

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// ownerKind is what an owner page is about: a ModuleInstance or a
// ModulePackage.
type ownerKind struct {
	Path   string // instances or packages
	Title  string // Instance or Package
	Plural string // Instances or Packages
	Topic  string // instance or package
}

var (
	instanceKind = ownerKind{Path: "instances", Title: "Instance", Plural: "Instances", Topic: "instance"}
	packageKind  = ownerKind{Path: "packages", Title: "Package", Plural: "Packages", Topic: "package"}
)

// eventsView is a recent-activity feed and where it is refreshed from.
type eventsView struct {
	Problem *v1.Problem
	Items   []v1.Event
	// About names the object the feed is about, when it is not the page's.
	About string
	// Follow is the events topic the feed follows, if any.
	Follow string
	// Partial is true when the feed left out what the caller could not read.
	Partial bool
	// Read is when the handler rendered the feed, and Reload the address that
	// renders it again: the note of a panel view that has gone stale.
	Read   *time.Time
	Reload string
}

// signIn answers a request the read API did not authenticate.
func (h *Handler) signIn(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusUnauthorized, "message", page{Title: "Sign in", Main: message{
		Heading: "You are not signed in",
		Text: "This browser holds no live session for this portal. Open the launch link opm-portal serve printed, " +
			"or restart it for a new one.",
		Code: v1.CodeUnauthenticated,
	}})
}

// unauthenticated reports whether a read was refused for want of a session.
func unauthenticated(p *v1.Problem) bool { return p != nil && p.Code == v1.CodeUnauthenticated }

// problemStatus is the page status a main document's problem gives. A
// forbidden document renders as a locked region of a page that was served
// as asked, so it is 200, as the read API answers a list the caller may
// not read; the others keep the API's status.
func problemStatus(p *v1.Problem) int {
	if p == nil || p.Code == v1.CodeForbidden {
		return http.StatusOK
	}
	if p.Status >= 400 && p.Status <= 599 {
		return p.Status
	}
	return http.StatusServiceUnavailable
}

// --- Installed ---

// ownerController is the owner the pages name for every package, the read
// API's owner value for the controller (portal:D17:R2).
const ownerController = "controller"

// verdictAccepted is a registration's accepted verdict on the wire.
const verdictAccepted = "Accepted"

// installedRow is one instance or package of the Installed list.
type installedRow struct {
	Kind      string // instance or package
	Href      string
	Namespace string
	Name      string
	// Module is the module path and version of an instance. Source is a
	// package's source as "<Kind> <namespace>/<name>", Path its path and
	// SourceArtifact the revision and digest the controller recorded.
	Module         v1.Module
	Source         string
	Path           string
	SourceArtifact *v1.SourceArtifact
	Reconcile      v1.Reconcile
	Health         v1.Health
	Objects        int64
	// Owner is controller or cli: an instance's is the read API's owner
	// value, and a package's is the controller.
	Owner      string
	ProviderOf []v1.ProviderClaim
	// Pills are the provider badges, one for each registration held.
	Pills []tipData
	// RenderContracts are an instance's; a package records none.
	RenderContracts []string
}

// ModuleValue is what the module filter matches: an instance's module path
// or a package's source string.
func (r *installedRow) ModuleValue() string {
	if r.Kind == packageKind.Topic {
		return r.Source
	}
	return r.Module.Path
}

// RowClass tints a row by its health, as the canvas tints a Degraded one.
func (r *installedRow) RowClass() string {
	if _, ok := healthText[r.Health.State]; ok {
		return "row-" + slug(r.Health.State)
	}
	return "row-" + unknownWord
}

// ShowPath says whether the package's path is worth a word: not when it is
// empty or the source's root.
func (r *installedRow) ShowPath() bool { return r.Path != "" && r.Path != "." }

// Revision is the second line of a package's source cell: the digest as
// "sha256:" and its first eight hex characters, with the recorded revision
// in the title, or nothing recorded.
func (r *installedRow) Revision() revisionLine {
	a := r.SourceArtifact
	switch {
	case a != nil && a.Digest != "":
		return revisionLine{Text: digestShort(a.Digest), Title: a.Revision}
	case a != nil && a.Revision != "":
		return revisionLine{Text: a.Revision}
	}
	return revisionLine{Text: "no revision recorded"}
}

// revisionLine is the second line of a package's source cell.
type revisionLine struct{ Text, Title string }

// digestShort is a digest as "algorithm:" and its first eight characters.
func digestShort(d string) string {
	alg, hex, ok := strings.Cut(d, ":")
	if !ok {
		return d[:min(8, len(d))]
	}
	return alg + ":" + hex[:min(8, len(hex))]
}

// installedView is the Installed list: instances and packages in one
// table, each kind locked on its own (portal:D17:R1, portal:D7:R2).
type installedView struct {
	Filters filters
	// Query is the page's query, whose other parameters the form keeps.
	Query url.Values
	// InstancesProblem and PackagesProblem are a list that failed;
	// InstancesLocked and PackagesLocked a list the caller may not read.
	InstancesProblem *v1.Problem
	PackagesProblem  *v1.Problem
	InstancesLocked  bool
	PackagesLocked   bool
	Rows             []installedRow
	// Total is the number of rows before the filters, over the rows the
	// caller may read, or within Namespace when a kind was read there.
	Total int
	// UsesSet: the uses filter leaves packages out, and the page says so.
	UsesSet bool
	// Scope says what the total counts when it is not every row: "in
	// default", "you may read".
	Scope string
	// Topic is the stream topic the list follows, which the count region
	// follows too.
	Topic string
	// input is what the page supplies to its filter form: the choices and
	// the counts of the rows the caller may read.
	input formInput
}

// listForbidden says the caller may not list a kind in the scope asked.
func listForbidden(p *v1.Problem, access string) bool {
	if p != nil {
		return p.Code == v1.CodeForbidden
	}
	return access == v1.AccessForbidden
}

// installedReads are the two lists as the page read them.
type installedReads struct {
	instances v1.InstanceList
	packages  v1.PackageList
	instProb  *v1.Problem
	pkgProb   *v1.Problem
	// wide: both lists were read for every namespace. scoped: at least one
	// kind was read in the filtered namespace instead.
	wide, scoped bool
}

// readInstalled reads both lists for every namespace. Where a kind is
// forbidden for every namespace and the filter names one, that kind is read
// in the namespace, which is the read the page made before (installed-23).
func (h *Handler) readInstalled(r *http.Request, ns string) installedReads {
	var rd installedReads
	rd.instProb = h.fetch(r, "/instances", nil, &rd.instances)
	rd.pkgProb = h.fetch(r, "/packages", nil, &rd.packages)
	rd.wide = rd.instProb == nil && rd.instances.Access == v1.AccessOK &&
		rd.pkgProb == nil && rd.packages.Access == v1.AccessOK
	if ns == "" || !dnsLabel(ns) {
		return rd
	}
	if listForbidden(rd.instProb, rd.instances.Access) {
		rd.instances, rd.scoped = v1.InstanceList{}, true
		rd.instProb = h.fetch(r, "/instances", nsQuery(ns), &rd.instances)
	}
	if listForbidden(rd.pkgProb, rd.packages.Access) {
		rd.packages, rd.scoped = v1.PackageList{}, true
		rd.pkgProb = h.fetch(r, "/packages", nsQuery(ns), &rd.packages)
	}
	return rd
}

// rows returns every row the caller may read, sorted. A kind read for one
// namespace beside a kind read for every namespace would count a mixed
// number, so both are kept to the namespace then.
func (rd *installedReads) rows(ns string) []installedRow {
	all := make([]installedRow, 0, len(rd.instances.Items)+len(rd.packages.Items))
	for i := range rd.instances.Items {
		all = append(all, instanceRow(&rd.instances.Items[i]))
	}
	for i := range rd.packages.Items {
		all = append(all, packageRow(&rd.packages.Items[i]))
	}
	if rd.scoped {
		all = slices.DeleteFunc(all, func(row installedRow) bool { return row.Namespace != ns })
	}
	slices.SortStableFunc(all, func(a, b installedRow) int {
		return strings.Compare(a.Namespace+"/"+a.Name+"/"+a.Kind, b.Namespace+"/"+b.Name+"/"+b.Kind)
	})
	return all
}

func (h *Handler) installedPage(w http.ResponseWriter, r *http.Request) {
	v := installedView{Filters: parseFilters(&installedFilters, r.URL.Query()), Query: r.URL.Query()}
	fv := v.Filters.Values
	ns := fv["namespace"]

	rd := h.readInstalled(r, ns)
	if unauthenticated(rd.instProb) || unauthenticated(rd.pkgProb) {
		h.signIn(w, r)
		return
	}
	v.InstancesProblem, v.PackagesProblem = rd.instProb, rd.pkgProb
	v.InstancesLocked = rd.instProb == nil && rd.instances.Access != v1.AccessOK
	v.PackagesLocked = rd.pkgProb == nil && rd.packages.Access != v1.AccessOK

	all := rd.rows(ns)
	v.Total = len(all)
	for i := range all {
		if matchesInstalled(&all[i], fv) {
			v.Rows = append(v.Rows, all[i])
		}
	}
	v.UsesSet = fv["uses"] != ""

	v.Scope = v.scope(rd.scoped, ns)
	v.input = installedInput(all, installedReadable{
		instances: v.InstancesProblem == nil && !v.InstancesLocked,
		packages:  v.PackagesProblem == nil && !v.PackagesLocked,
		instNote:  unreadableWord(v.InstancesProblem),
		pkgNote:   unreadableWord(v.PackagesProblem),
		wide:      rd.wide,
	})

	// The topic names the namespace only for a kind read in it, so a
	// malformed one never reaches the stream.
	topic := "instances"
	if rd.scoped {
		topic += ":" + ns
	}
	v.Topic = topic
	status := max(problemStatus(v.InstancesProblem), problemStatus(v.PackagesProblem))
	h.render(w, r, status, "installed", page{Title: "Installed", Nav: "installed", Topics: []string{topic}, Main: v})
}

// scope says what the total counts when it is not every row (installed-23):
// within the namespace a kind was read in, or only what the caller may read.
func (v *installedView) scope(scoped bool, ns string) string {
	partial := v.InstancesLocked || v.PackagesLocked || v.InstancesProblem != nil || v.PackagesProblem != nil
	switch {
	case scoped && partial:
		return "you may read in " + ns
	case scoped:
		return "in " + ns
	case partial:
		return "you may read"
	}
	return ""
}

// unreadableWord is the word an option takes where its count would stand
// for a list that is locked or failed.
func unreadableWord(p *v1.Problem) string {
	if p != nil && p.Code != v1.CodeForbidden {
		return "unavailable"
	}
	return wordLocked
}

// installedReadable says which kinds the caller may read in the scope the
// page asked, and whether both were read for every namespace.
type installedReadable struct {
	instances, packages bool
	instNote, pkgNote   string
	wide                bool
}

// installedInput builds the choices and the counts of the Installed filter
// form from the rows the caller may read, before any filter. A value whose
// rows come only from a list the caller may not read carries a word where
// its count would stand, never a zero (portal:D14:R7, portal:D7:R2).
func installedInput(rows []installedRow, rd installedReadable) formInput {
	counts := map[string]map[string]int{}
	for _, p := range installedFilters.Params {
		counts[p.Name] = map[string]int{}
	}
	bump := func(param, value string) {
		if value != "" {
			counts[param][value]++
		}
	}
	for i := range rows {
		row := &rows[i]
		bump("kind", row.Kind)
		bump("provider", map[bool]string{true: "yes", false: "no"}[len(row.ProviderOf) > 0])
		for _, c := range row.RenderContracts {
			bump("uses", c)
		}
		bump("namespace", row.Namespace)
		bump("health", row.Health.State)
		bump("applied", row.Reconcile.State)
		bump("owner", row.Owner)
		bump("module", row.ModuleValue())
	}
	notes := map[string]map[string]string{"kind": {}, "owner": {}}
	if !rd.instances {
		notes["kind"][instanceKind.Topic] = rd.instNote
		// Only instances are owned by the cli.
		notes["owner"]["cli"] = rd.instNote
	}
	if !rd.packages {
		notes["kind"][packageKind.Topic] = rd.pkgNote
	}
	// A fixed value with no rows among the rows the caller may read counts
	// zero, unless its rows could only have come from an unread list.
	for _, p := range installedFilters.Params {
		for _, val := range p.Values {
			if _, ok := counts[p.Name][val]; !ok && notes[p.Name][val] == "" {
				counts[p.Name][val] = 0
			}
		}
	}
	// With neither list read, every count stands for rows nobody may read:
	// "Healthy (0)" would say nothing is unhealthy, when the portal does not
	// know (portal:D14:R7).
	var noCount map[string]bool
	if !rd.instances && !rd.packages {
		noCount = map[string]bool{"health": true, "applied": true, "provider": true, "owner": true}
	}
	keys := func(param string) []string {
		out := make([]string, 0, len(counts[param]))
		for k := range counts[param] {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	in := formInput{
		Choices: map[string][]string{"uses": keys("uses"), "module": keys("module")},
		Counts:  counts,
		NoCount: noCount,
		Notes:   notes,
		Suggest: map[string][]string{},
		Placeholders: map[string]string{
			"q": "Name, namespace, module or source", "namespace": "all namespaces",
		},
		ShownID: "shown", Variant: filtersCard,
	}
	if rd.wide {
		in.Choices["namespace"] = keys("namespace")
	} else {
		// A kind that is locked for every namespace leaves the namespace a
		// field to type into: a reader who may list some namespaces can
		// still name one (portal:D14:R7). The datalist offers what is read.
		in.Suggest["namespace"] = keys("namespace")
		delete(counts, "namespace")
	}
	return in
}

func instanceRow(it *v1.InstanceSummary) installedRow {
	return installedRow{
		Kind: instanceKind.Topic, Href: instanceKind.base(it.Ref.Namespace, it.Ref.Name),
		Namespace: it.Ref.Namespace, Name: it.Ref.Name, Module: it.Module,
		Reconcile: it.Reconcile, Health: it.Health, Objects: it.InventoryCount, Owner: it.Owner,
		ProviderOf: it.ProviderOf, RenderContracts: it.RenderContracts,
		Pills: providerPills(instanceKind.Topic, it.Ref, it.ProviderOf),
	}
}

func packageRow(it *v1.PackageSummary) installedRow {
	return installedRow{
		Kind: packageKind.Topic, Href: packageKind.base(it.Ref.Namespace, it.Ref.Name),
		Namespace: it.Ref.Namespace, Name: it.Ref.Name, Source: packageSource(it), Path: it.Path,
		SourceArtifact: it.SourceArtifact,
		Reconcile:      it.Reconcile, Health: it.Health, Objects: it.InventoryCount, Owner: ownerController,
		ProviderOf: it.ProviderOf,
		Pills:      providerPills(packageKind.Topic, it.Ref, it.ProviderOf),
	}
}

// packageSource is a package's source as "<Kind> <namespace>/<name>", the
// namespace defaulting to the package's own (installed-26).
func packageSource(it *v1.PackageSummary) string {
	ns := it.Source.Namespace
	if ns == "" {
		ns = it.Ref.Namespace
	}
	return it.Source.Kind + " " + ns + "/" + it.Source.Name
}

// providerPills builds one provider badge for each registration an item
// holds. The tooltip ids are unique on the page.
func providerPills(kind string, ref v1.ObjectRef, claims []v1.ProviderClaim) []tipData {
	out := make([]tipData, 0, len(claims))
	for i, c := range claims {
		out = append(out, providerPill(c, fmt.Sprintf("tip-prov-%s-%s-%s-%d", kind, ref.Namespace, ref.Name, i)))
	}
	return out
}

// matchesInstalled applies the Installed filters to one row. Free text
// and names are matched as given. uses names what an instance's render
// used, which a package does not record (portal:D16:R2), so a package
// matches no uses value. module is an instance's module path or a
// package's source string.
func matchesInstalled(row *installedRow, f map[string]string) bool {
	if q := strings.ToLower(f["q"]); q != "" {
		hay := strings.ToLower(strings.Join([]string{row.Kind, row.Namespace, row.Name, row.Module.Path, row.Source, row.Path}, " "))
		if !strings.Contains(hay, q) {
			return false
		}
	}
	checks := []struct {
		param string
		ok    func(string) bool
	}{
		{"kind", func(want string) bool { return row.Kind == want }},
		{"provider", func(want string) bool { return (len(row.ProviderOf) > 0) == (want == yesWord) }},
		{"uses", func(want string) bool { return slices.Contains(row.RenderContracts, want) }},
		{"namespace", func(want string) bool { return row.Namespace == want }},
		{"health", func(want string) bool { return row.Health.State == want }},
		{"applied", func(want string) bool { return row.Reconcile.State == want }},
		{"owner", func(want string) bool { return row.Owner == want }},
		{"module", func(want string) bool { return row.ModuleValue() == want }},
	}
	for _, c := range checks {
		if want := f[c.param]; want != "" && !c.ok(want) {
			return false
		}
	}
	return true
}

// listRedirect answers a former list page with 308 to Installed filtered
// by its kind, keeping its namespace (portal:D17:R1).
func listRedirect(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := url.Values{"kind": {kind}}
		if ns := r.URL.Query().Get("namespace"); ns != "" {
			q.Set("namespace", ns)
		}
		http.Redirect(w, r, "/installed?"+q.Encode(), http.StatusPermanentRedirect)
	}
}

func nsQuery(ns string) url.Values {
	if ns == "" {
		return nil
	}
	return url.Values{"namespace": {ns}}
}

// dnsLabel reports whether s could be a namespace: a DNS-1123 label.
func dnsLabel(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for i, c := range s {
		alnum := c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
		if !alnum && (c != '-' || i == 0 || i == len(s)-1) {
			return false
		}
	}
	return true
}

func appendUnique(list []string, s string) []string {
	if s == "" || slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

// --- Owner pages ---

type componentView struct {
	Name    string
	Health  v1.Health
	Objects []objectView
}

// configGroup is the components the graph folds into one group node, listed
// folded the same way. It opens when its health is not Healthy.
type configGroup struct {
	// ID is the id of the group's <details>, so a refresh keeps it open.
	ID string
	// Label is the group node's label, such as "17 configuration components".
	Label string
	// Family is the group's kind family as the graph wrote it: crds, rbac,
	// webhooks or other. It words the fold's note.
	Family     string
	Health     string
	Open       bool
	Components []componentView
}

type objectView struct {
	v1.InventoryObject
	// Focus opens the Graph tab focused on the node that shows the object,
	// and NodeID is that node's id.
	Focus     string
	NodeID    string
	Text      string
	YAML      string
	EventsURL string
	Children  []childView
	// Old are the ReplicaSets scaled to zero, folded apart from the live
	// children; OldID is the fold's id, so a refresh keeps it open.
	Old   []childView
	OldID string
}

type childView struct {
	v1.RuntimeChild
	Focus     string
	NodeID    string
	Text      string
	YAML      string
	EventsURL string
	// LogID is the id of the Pod's first container's log pane, and LogHref
	// the Logs tab opened at it.
	LogID   string
	LogHref string
}

// logPane is one container's log, followed on its log topic.
type logPane struct {
	ID        string
	Topic     string
	Pod       string
	Container string
	Previous  string
	// PodValue names the Pod in the Events resource filter; Reason is the
	// Pod's health reason, which says whether the container has started.
	PodValue string
	Reason   string
}

func (k ownerKind) base(ns, name string) string {
	return "/" + k.Path + "/" + url.PathEscape(ns) + "/" + url.PathEscape(name)
}

// foldConfig moves the components the graph folds into a group node out of
// cs, in their order, one configGroup for each group node of the graph, so the
// list folds what the graph folds (portal:D4:R5). A group none of whose
// members is among cs is left out.
func foldConfig(cs []componentView, g *v1.Graph) ([]componentView, []configGroup) {
	var groups []configGroup
	rest := cs
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Kind != "group" || n.Group == nil || n.Group.Kind != "configuration" {
			continue
		}
		cg := configGroup{ID: "config-components", Label: n.Label, Family: n.Group.Family, Health: "Unknown"}
		if len(groups) > 0 {
			cg.ID += "-" + strconv.Itoa(len(groups)+1)
		}
		if n.Health != nil {
			cg.Health = n.Health.State
		}
		cg.Open = cg.Health != healthHealthy
		next := make([]componentView, 0, len(rest))
		for j := range rest {
			if slices.Contains(n.Group.Members, rest[j].Name) {
				cg.Components = append(cg.Components, rest[j])
			} else {
				next = append(next, rest[j])
			}
		}
		if len(cg.Components) == 0 {
			continue
		}
		rest = next
		groups = append(groups, cg)
	}
	return rest, groups
}

// components adds the links each object offers: its YAML and its events,
// for an object the caller may read and that is not a Secret.
func (h *Handler) components(base string, cs []v1.Component) []componentView {
	out := make([]componentView, 0, len(cs))
	for i := range cs {
		c := componentView{Name: cs[i].Name, Health: cs[i].Health}
		for _, obj := range objectsOf(&cs[i]) {
			c.Objects = append(c.Objects, objectViewOf(base, obj))
		}
		out = append(out, c)
	}
	return out
}

// objectsOf returns pointers to a component's objects.
func objectsOf(c *v1.Component) []*v1.InventoryObject {
	out := make([]*v1.InventoryObject, len(c.Objects))
	for j := range out {
		out[j] = &c.Objects[j]
	}
	return out
}

func objectViewOf(base string, obj *v1.InventoryObject) objectView {
	o := objectView{InventoryObject: *obj, Text: refText(obj.Ref), OldID: domID("old", obj.Ref.Group, obj.Ref.Kind, obj.Ref.Namespace, obj.Ref.Name)}
	if o.Access == v1.AccessOK && !isSecret(o.Ref) {
		q := refQuery(o.Ref).Encode()
		o.YAML, o.EventsURL = base+"/object?"+q, base+"/events?"+q
	}
	for k := range obj.Children {
		ch := obj.Children[k]
		q := refQuery(ch.Ref).Encode()
		cv := childView{RuntimeChild: ch, Text: refText(ch.Ref), YAML: base + "/object?" + q, EventsURL: base + "/events?" + q}
		if isPod(ch.Ref) && len(ch.Containers) > 0 {
			cv.LogID = logID(ch.Ref.Name, ch.Containers[0])
			cv.LogHref = logLink(base, cv.LogID)
		}
		if ch.Ref.Kind == kindReplicaSet && ch.Replicas != nil && *ch.Replicas == 0 {
			o.Old = append(o.Old, cv)
			continue
		}
		o.Children = append(o.Children, cv)
	}
	return o
}

// logPanes lists one pane per container of every Pod below the inventory.
func logPanes(cs []componentView) []logPane {
	var out []logPane
	for i := range cs {
		for j := range cs[i].Objects {
			for k := range cs[i].Objects[j].Children {
				ch := &cs[i].Objects[j].Children[k]
				if !isPod(ch.Ref) {
					continue
				}
				for _, c := range ch.Containers {
					topic := "log:" + ch.Ref.Namespace + "/" + ch.Ref.Name + "/" + c
					out = append(out, logPane{ID: logID(ch.Ref.Name, c), Topic: topic, Previous: topic + "/previous", Pod: ch.Ref.Name, Container: c,
						PodValue: resourceValue(ch.Ref), Reason: ch.Health.Reason})
				}
			}
		}
	}
	return out
}

func isPod(r v1.ObjectRef) bool { return r.Kind == "Pod" && r.Group == "" }

// logLink is the Logs tab opened on the pane id: ?tab=logs&log=<id>.
func logLink(base, id string) string {
	return base + "?tab=" + tabLogs + "&log=" + url.QueryEscape(id)
}

// logID is the id of a container's log pane.
func logID(pod, container string) string { return domID("log", pod, container) }

// domID joins parts into an element id. The ids it makes reach a page
// only for a Pod's containers (logID: Pod names and container names, both
// DNS labels) and for a workload's old-revisions fold (objectViewOf: an
// object that has old ReplicaSets, so its group, kind, namespace and name
// follow DNS and identifier rules). None of those parts hold an underscore
// or whitespace, so joining them with an underscore keeps ids of different
// parts apart: Pod a-b's container c and Pod a's container b-c differ. A
// name validated only as a path segment, such as an RBAC role's, may hold
// other characters, so an id rendered for such an object needs another
// separator.
func domID(prefix string, parts ...string) string {
	return prefix + "_" + strings.Join(parts, "_")
}

func (h *Handler) ownerNode(k ownerKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := k.base(r.PathValue("namespace"), r.PathValue("name"))
		var g v1.Graph
		if p := h.fetchGraph(r, base+"/graph", &g); p != nil {
			h.renderProblemFragment(w, r, p)
			return
		}
		ctx := panelContext{owner: k, base: base}
		if n := restingNode(&g); n != nil {
			ctx.resting = n.ID
		}
		// A Pod's Logs link needs the owner's containers, and the owner's own
		// node its identity facts, both from the owner document.
		if n := findNode(&g, r.URL.Query().Get("id")); n != nil && n.Ref != nil && (isPod(*n.Ref) || isOwnerNode(n, ctx)) {
			self := h.loadOwner(r, k, r.PathValue("namespace"), r.PathValue("name"))
			ctx.firstContainer = self.firstContainer
			if self.Problem == nil {
				ctx.self = &self
			}
		}
		h.renderPanel(w, r, h.panel(r, &g, r.URL.Query().Get("id"), ctx))
	}
}

// containersOf maps each Pod below cs to its first container.
func containersOf(cs []v1.Component) func(string) string {
	first := map[string]string{}
	for i := range cs {
		for j := range cs[i].Objects {
			for k := range cs[i].Objects[j].Children {
				ch := &cs[i].Objects[j].Children[k]
				if isPod(ch.Ref) && len(ch.Containers) > 0 {
					first[ch.Ref.Name] = ch.Containers[0]
				}
			}
		}
	}
	return func(pod string) string { return first[pod] }
}

// objectYAML is the YAML view of one object.
type objectYAML struct {
	Problem *v1.Problem
	Ref     v1.ObjectRef
	Text    string
	YAML    string
	// Read is when the handler rendered the view, and Reload the address that
	// renders it again: the note of a panel view that has gone stale.
	Read   *time.Time
	Reload string
}

// reloadAddress is the request's own address, so the note's link renders the
// same view again. It is built from the path the mux matched and the query the
// request carried; nothing else reaches it.
func reloadAddress(r *http.Request) string {
	return (&url.URL{Path: r.URL.Path, RawQuery: r.URL.RawQuery}).String()
}

// readAt is the handler's clock, as the pointer the time partial takes.
func (h *Handler) readAt() *time.Time {
	t := h.cfg.Now().UTC()
	return &t
}

func (h *Handler) ownerObject(k ownerKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := k.base(r.PathValue("namespace"), r.PathValue("name"))
		var doc v1.Object
		v := objectYAML{Read: h.readAt(), Reload: reloadAddress(r)}
		if v.Problem = h.fetch(r, base+"/object", r.URL.Query(), &doc); v.Problem == nil {
			v.Ref, v.Text = doc.Ref, refText(doc.Ref)
			out, err := yaml.Marshal(doc.Object)
			if err != nil {
				v.Problem = unavailable()
			}
			v.YAML = string(out)
		}
		if unauthenticated(v.Problem) {
			h.signIn(w, r)
			return
		}
		h.renderFragment(w, r, problemStatus(v.Problem), "object", page{Title: "Object", Nav: "installed", Main: v})
	}
}

func (h *Handler) ownerEvents(k ownerKind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := k.base(r.PathValue("namespace"), r.PathValue("name"))
		q := r.URL.Query()
		ev := h.events(r, base+"/events", q, "")
		ev.Read, ev.Reload = h.readAt(), reloadAddress(r)
		if q.Get("kind") != "" {
			ev.About = refText(v1.ObjectRef{Group: q.Get("group"), Kind: q.Get("kind"), Namespace: q.Get("namespace"), Name: q.Get("name")})
		}
		if unauthenticated(ev.Problem) {
			h.signIn(w, r)
			return
		}
		h.renderFragment(w, r, problemStatus(ev.Problem), "events", page{Title: "Events", Nav: "installed", Main: ev})
	}
}

// events reads one feed.
func (h *Handler) events(r *http.Request, path string, q url.Values, follow string) eventsView {
	var doc v1.EventList
	ev := eventsView{Follow: follow}
	ev.Problem = h.fetch(r, path, q, &doc)
	ev.Items = doc.Items
	ev.Partial = doc.Partial
	return ev
}

// graphQuery passes the graph options of a page request to the API.
func graphQuery(r *http.Request) url.Values {
	in := r.URL.Query()
	out := url.Values{}
	for _, id := range in["expand"] {
		out.Add("expand", id)
	}
	if v := in.Get("showScaledDown"); v != "" {
		out.Set("showScaledDown", v)
	}
	return out
}

// renderProblemFragment answers a fragment request whose document failed.
func (h *Handler) renderProblemFragment(w http.ResponseWriter, r *http.Request, p *v1.Problem) {
	if p.Code == v1.CodeUnauthenticated {
		h.signIn(w, r)
		return
	}
	h.renderFragment(w, r, problemStatus(p), "problem", page{Title: "Unavailable", Main: p})
}

// FilterForm returns the Installed filter form, with its count row.
func (v installedView) FilterForm() filterForm {
	in := v.input
	in.Shown, in.Total, in.Scope, in.Follow = len(v.Rows), v.Total, v.Scope, v.Topic
	return newChoiceForm(v.Filters, "Filter what is installed", keepHidden(v.Query, installedFilters.paramNames()...), in)
}
