package ui

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// The Events, Logs and YAML tabs of an owner page. Each tab is a GET form of
// selects (data-filters keys no view remembers, portal:D14), so a pick is a
// plain link that works without script.

// --- Events ---

// eventsForm is the Events tab's filter row: the resource choice (the merged
// feed's Regarding set, the owner always in), the type, the reason chip and the
// count of lines shown.
type eventsForm struct {
	Filters  filters
	Hidden   []hiddenField
	Resource []optionGroup
	Types    []selectOption
	Reason   string
	Shown    int
	Total    int
	// Partial is true when the feed left out what the caller could not read,
	// so the tab says so.
	Partial bool
}

// resourceValue names an object in the resource filter: group/Kind/ns/name.
func resourceValue(r v1.ObjectRef) string {
	return r.Group + "/" + r.Kind + "/" + r.Namespace + "/" + r.Name
}

func parseResource(s string) v1.ObjectRef {
	parts := strings.SplitN(s, "/", 4)
	if len(parts) != 4 {
		return v1.ObjectRef{}
	}
	return v1.ObjectRef{Group: parts[0], Kind: parts[1], Namespace: parts[2], Name: parts[3]}
}

// eventChoice is one object of the merged feed with the folded lines about it.
type eventChoice struct {
	ref v1.ObjectRef
	n   int
}

// eventChoices counts the folded lines of items by the object they regard, the
// owner first and always present, the rest by kind and name. Only objects found
// in rows the caller may read are listed (portal:D14:R7).
func eventChoices(owner v1.ObjectRef, items []v1.Event) []eventChoice {
	counts := map[string]*eventChoice{resourceValue(owner): {ref: owner}}
	var order []string
	for i := range items {
		ref := items[i].Regarding
		key := resourceValue(ref)
		c := counts[key]
		if c == nil {
			c = &eventChoice{ref: ref}
			counts[key] = c
			order = append(order, key)
		}
		c.n++
	}
	rest := make([]eventChoice, 0, len(order))
	for _, k := range order {
		if k != resourceValue(owner) {
			rest = append(rest, *counts[k])
		}
	}
	slices.SortFunc(rest, func(a, b eventChoice) int {
		return strings.Compare(a.ref.Kind+"/"+a.ref.Namespace+"/"+a.ref.Name, b.ref.Kind+"/"+b.ref.Namespace+"/"+b.ref.Name)
	})
	out := make([]eventChoice, 0, 1+len(rest))
	out = append(out, *counts[resourceValue(owner)])
	return append(out, rest...)
}

// ownerEventsTab builds the Events tab from the merged feed the page already
// read: the resource, type and reason narrow it in the page, with no second
// read, the owner itself included. v.Events holds the whole feed on entry
// and the filtered one on return.
//
// The stream's events topic carries the owner's own feed only (the producer
// re-renders events:<owner> from the owner-only list), so a refresh follows an
// event about the owner and not one about a child; the tab shows a child's event
// at its next render, when the page or the owner document next refreshes
// (design.md, "Scale and refresh").
func (h *Handler) ownerEventsTab(r *http.Request, v *ownerView) {
	q := r.URL.Query()
	items := v.Events.Items
	owner := v.ownerRef()
	choices := eventChoices(owner, items)

	resource := filterParam{Name: "resource", Label: "Resource", Values: []string{}, Text: map[string]string{}}
	for _, c := range choices {
		val := resourceValue(c.ref)
		resource.Values = append(resource.Values, val)
		resource.Text[val] = refText(c.ref)
	}
	typeText := map[string]string{eventNormal: eventNormal, eventWarning: eventWarning}
	view := filterView{Path: v.Base, Params: []filterParam{resource,
		{Name: "type", Label: "Type", Values: []string{eventNormal, eventWarning}, Text: typeText},
		{Name: "reason", Label: "Reason"}}}
	f := parseFilters(&view, q)
	form := eventsForm{Filters: f, Reason: f.Values["reason"], Partial: v.Events.Partial, Total: len(items),
		Hidden: keepHidden(q, "resource", "type", "reason", "tab")}

	form.Resource = resourceGroups(v, owner, choices, f.Values["resource"], len(items))
	form.Types = typeOptions(items, f.Values["type"])
	kept := filterEvents(items, f)
	v.Events.Items = kept
	form.Shown = len(kept)
	v.EventsForm = form

	empty := noEvents
	if len(f.Active) > 0 {
		empty = noEventsMatch
	}
	rows := make([]eventRow, len(kept))
	for i := range kept {
		// Every row regards the owner or an object the feed reached, so each
		// resource cell is a filter link; the other parameters stay.
		rows[i] = eventRowOf(kept[i], linkWith(v.Base, q, "resource", resourceValue(kept[i].Regarding)))
	}
	v.EventsTable = eventsTable{Problem: v.Events.Problem, Rows: rows, ShowResource: true, Empty: empty}
}

// --- Logs ---

// logsTab is the Logs tab: one Pod and container choice, the picked pane
// and the hint about why it may have no log.
type logsTab struct {
	Hidden []hiddenField
	Groups []optionGroup
	// Pane is the picked container's pane, nil when the inventory holds no Pod.
	Pane *logPane
	Hint logHint
}

// logHint says why the picked container has no log yet, with the Events link.
type logHint struct {
	Text string
	Href string
}

// noLogReasons are the reasons under which no instance of a container has
// started (design.md, Logs): its log does not exist yet.
var noLogReasons = []string{"ContainerCreating", "ErrImagePull", "ImagePullBackOff", "InvalidImageName", "CreateContainerConfigError", "Pending"}

const (
	crashLoop = "CrashLoopBackOff"

	hintNotStarted = "The container has not started, so there are no logs yet."
	hintRestarting = "The container is restarting; its last run's logs are under Previous instance."
)

// ownerLogsTab picks the pane the log parameter names, else the first container
// of the first Pod, and lists every container, grouped by Pod, for the choice.
func ownerLogsTab(r *http.Request, v *ownerView) {
	q := r.URL.Query()
	panes := logPanes(v.allComponents())
	t := logsTab{Hidden: keepHidden(q, "log", "tab")}
	want := q.Get("log")
	for i := range panes {
		p := &panes[i]
		if p.ID == want {
			t.Pane = p
		}
		if len(t.Groups) == 0 || t.Groups[len(t.Groups)-1].Label != p.Pod {
			t.Groups = append(t.Groups, optionGroup{Label: p.Pod})
		}
		g := &t.Groups[len(t.Groups)-1]
		g.Options = append(g.Options, selectOption{Value: p.ID, Text: p.Container})
	}
	if t.Pane == nil && len(panes) > 0 {
		t.Pane = &panes[0]
	}
	if t.Pane != nil {
		for gi := range t.Groups {
			for oi := range t.Groups[gi].Options {
				o := &t.Groups[gi].Options[oi]
				o.Selected = o.Value == t.Pane.ID
			}
		}
		t.Hint = hintOf(t.Pane, v)
	}
	v.Logs = t
}

// hintOf is the line above a pane whose container has no log to read yet. The
// read model gives a Pod's health reason, not each container's waiting state,
// so the hint keys to the Pod's reason (design.md: "else the Pod's health
// reason").
func hintOf(p *logPane, v *ownerView) logHint {
	events := v.Base + "?" + url.Values{"tab": {tabEvents}, "resource": {p.PodValue}}.Encode()
	switch {
	case p.Reason == crashLoop:
		return logHint{Text: hintRestarting, Href: events}
	case slices.Contains(noLogReasons, p.Reason):
		return logHint{Text: hintNotStarted, Href: events}
	}
	return logHint{}
}

// --- YAML ---

// yamlTab is the YAML tab: a choice of the objects the inventory reaches,
// grouped by kind, and the YAML of the one picked.
type yamlTab struct {
	Hidden []hiddenField
	Groups []yamlGroup
	Picked objectYAML
	Has    bool
}

type yamlGroup struct {
	Kind  string
	Items []yamlItem
}

type yamlItem struct {
	Value   string
	Text    string
	Current bool
	// Secret: listed as never read, disabled.
	Secret bool
}

// yamlEntry is one object the YAML choice can list.
type yamlEntry struct {
	ref    v1.ObjectRef
	secret bool
}

// yamlEntries lists the objects of the inventory the choice can show, each
// object before its children and old revisions. The read API serves a Secret
// with access "withheld": it is listed by name and marked as never read, with
// no children and no content. An object that is forbidden or not readable is
// not listed.
func yamlEntries(v *ownerView) []yamlEntry {
	var entries []yamlEntry
	comps := v.allComponents()
	for i := range comps {
		for j := range comps[i].Objects {
			o := &comps[i].Objects[j]
			if o.Access == v1.AccessWithheld {
				entries = append(entries, yamlEntry{o.Ref, true})
				continue
			}
			if o.Access != v1.AccessOK {
				continue
			}
			entries = append(entries, yamlEntry{o.Ref, isSecret(o.Ref)})
			for k := range o.Children {
				entries = append(entries, yamlEntry{o.Children[k].Ref, false})
			}
			for k := range o.Old {
				entries = append(entries, yamlEntry{o.Old[k].Ref, false})
			}
		}
	}
	return entries
}

// yamlDefault is the object the tab opens on when no object is picked: the
// object of the node the page selects, else of the focus, when the choice lists
// it and it is not a Secret, else the first readable object that is not a
// Secret, so YAML shows on arrival.
func yamlDefault(v *ownerView, g *v1.Graph, entries []yamlEntry) string {
	sel := v.Node
	if sel == "" {
		sel = v.Focus
	}
	if n := findNode(g, sel); n != nil && n.Ref != nil && n.Access == v1.AccessOK && !isSecret(*n.Ref) {
		for _, e := range entries {
			if !e.secret && resourceValue(e.ref) == resourceValue(*n.Ref) {
				return resourceValue(e.ref)
			}
		}
	}
	for _, e := range entries {
		if !e.secret {
			return resourceValue(e.ref)
		}
	}
	return ""
}

// yamlPick is the object to show: the one asked for when the choice lists it
// and it is not a Secret, else the default. A Secret is never read, so a
// URL that names one gets the default.
func yamlPick(asked string, v *ownerView, g *v1.Graph, entries []yamlEntry) string {
	for _, e := range entries {
		if !e.secret && resourceValue(e.ref) == asked {
			return asked
		}
	}
	return yamlDefault(v, g, entries)
}

func (h *Handler) ownerYAMLTab(r *http.Request, v *ownerView, g *v1.Graph) {
	entries := yamlEntries(v)
	picked := yamlPick(r.URL.Query().Get("object"), v, g, entries)
	v.YAML.Hidden = keepHidden(r.URL.Query(), "object", "tab")
	byKind := map[string][]yamlItem{}
	var kinds []string
	for _, e := range entries {
		val := resourceValue(e.ref)
		item := yamlItem{Value: val, Text: e.ref.Name, Secret: e.secret, Current: val == picked}
		if e.ref.Namespace != "" {
			item.Text = e.ref.Namespace + "/" + e.ref.Name
		}
		if byKind[e.ref.Kind] == nil {
			kinds = append(kinds, e.ref.Kind)
		}
		byKind[e.ref.Kind] = append(byKind[e.ref.Kind], item)
	}
	slices.Sort(kinds)
	for _, k := range kinds {
		v.YAML.Groups = append(v.YAML.Groups, yamlGroup{Kind: k, Items: byKind[k]})
	}
	if picked == "" {
		return
	}
	ref := parseResource(picked)
	var doc v1.Object
	v.YAML.Has = true
	if p := h.fetch(r, v.Base+"/object", refQuery(ref), &doc); p != nil {
		v.YAML.Picked.Problem = p
		return
	}
	out, err := yaml.Marshal(doc.Object)
	if err != nil {
		v.YAML.Picked.Problem = unavailable()
		return
	}
	v.YAML.Picked = objectYAML{Ref: doc.Ref, Text: refText(doc.Ref), YAML: string(out)}
}

// resourceGroups is the Resource choice: the Everything group (all resources and
// the owner), then one group per kind of the objects the feed regards.
func resourceGroups(v *ownerView, owner v1.ObjectRef, choices []eventChoice, picked string, total int) []optionGroup {
	all := "All resources"
	if v.Events.Partial {
		all = "All readable resources"
	}
	groups := []optionGroup{{Label: "Everything", Options: []selectOption{
		{Value: "", Text: fmt.Sprintf("%s (%d)", all, total), Selected: picked == ""},
		{Value: resourceValue(owner), Text: fmt.Sprintf("This %s (%d)", v.Kind.Topic, choices[0].n), Selected: picked == resourceValue(owner)},
	}}}
	for _, c := range choices[1:] {
		if last := &groups[len(groups)-1]; len(groups) == 1 || last.Label != c.ref.Kind {
			groups = append(groups, optionGroup{Label: c.ref.Kind})
		}
		g := &groups[len(groups)-1]
		val := resourceValue(c.ref)
		name := c.ref.Name
		if c.ref.Namespace != "" && c.ref.Namespace != v.Namespace {
			name = c.ref.Namespace + "/" + name
		}
		g.Options = append(g.Options, selectOption{Value: val, Text: fmt.Sprintf("%s (%d)", name, c.n), Selected: picked == val})
	}
	return groups
}

// typeOptions are the Warning and Normal choices with their counts over the
// whole merged feed.
func typeOptions(items []v1.Event, picked string) []selectOption {
	var warn int
	for i := range items {
		if items[i].Type == eventWarning {
			warn++
		}
	}
	return []selectOption{
		{Value: eventWarning, Text: fmt.Sprintf("%s (%d)", eventWarning, warn), Selected: picked == eventWarning},
		{Value: eventNormal, Text: fmt.Sprintf("%s (%d)", eventNormal, len(items)-warn), Selected: picked == eventNormal},
	}
}

// filterEvents keeps the lines the resource, type and reason filters match.
func filterEvents(items []v1.Event, f filters) []v1.Event {
	kept := items[:0:0]
	for i := range items {
		e := &items[i]
		if res := f.Values["resource"]; res != "" && resourceValue(e.Regarding) != res {
			continue
		}
		if t := f.Values["type"]; t != "" && e.Type != t {
			continue
		}
		if rs := f.Values["reason"]; rs != "" && e.Reason != rs {
			continue
		}
		kept = append(kept, *e)
	}
	return kept
}
