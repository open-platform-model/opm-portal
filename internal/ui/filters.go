package ui

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// The list views filter on the server from their URL query, so every
// filtered view is a plain link that works without script (portal:D14).
// Only the views here are remembered by the browser; their parameter sets
// and values are rendered into every page head for the page scripts, so
// the server and the scripts check a stored query against one definition.

// filterParam is one filter parameter of a view.
type filterParam struct {
	Name  string
	Label string
	// Values are the parameter's values, nil for free text or a name,
	// which is matched as given.
	Values []string
	// Text names a value for its option and its chip; a value it does not
	// name is shown as it is.
	Text map[string]string
	// Any is the first option's text, the empty choice in words; "" reads
	// "Any".
	Any string
	// Choices: a free-text parameter that the view draws as a select of the
	// values it supplies (namespace, uses, module, provides). It is still
	// validated as free text, so prefs.js and parseFilters treat it as
	// before. Without supplied values it stays a text input.
	Choices bool
	// Option formats the option text of a choice (a contract by its last two
	// path segments); nil shows the value itself. The value stays the full
	// string.
	Option func(string) string
	// Chip formats the chip text of a value; nil uses Text or the value. The
	// chip's title always holds the full value.
	Chip func(string) string
}

// filterView is a view whose filters live in its URL query.
type filterView struct {
	// Key names the view in browser storage: opm-portal.filters.<Key>:<context>.
	Key string
	// Path is the page the view is on.
	Path   string
	Params []filterParam
}

// installedFilters are the Installed list's filters.
var installedFilters = filterView{Key: "installed", Path: "/installed", Params: []filterParam{
	{Name: "q", Label: "Search"},
	{Name: "kind", Label: "Kind", Any: "Any kind", Values: []string{"instance", "package"},
		Text: map[string]string{"instance": "Instance", "package": "Package"}},
	{Name: "provider", Label: "Provider", Any: "Anything", Values: []string{"yes", "no"},
		Text: map[string]string{"yes": "Provider", "no": "Not a provider"}},
	{Name: "uses", Label: "Uses", Any: "Anything", Choices: true, Option: contractShort, Chip: contractShort},
	{Name: "namespace", Label: "Namespace", Any: "All namespaces", Choices: true},
	{Name: "health", Label: "Health", Any: "Any health", Values: healthValues, Text: healthText},
	{Name: "applied", Label: "Applied", Any: "Any state", Values: appliedValues, Text: appliedText},
	{Name: "owner", Label: "Owner", Any: "Anyone", Values: []string{"controller", "cli"},
		Text: map[string]string{"controller": "controller", "cli": "cli"}},
	{Name: "module", Label: "Module", Any: "Any module", Choices: true, Chip: moduleChip},
}}

// providerFilters and catalogFilters are the Platform page's two tabs.
var (
	providerFilters = filterView{Key: "providers", Path: "/", Params: []filterParam{
		{Name: "pq", Label: "Search"},
		{Name: "pstatus", Label: "Status", Any: "Any status", Values: []string{"accepted", "active", "refused", "inactive", "blocked", "pending"},
			Text: map[string]string{"accepted": "Accepted", "active": "Active", "refused": "Refused", "inactive": "Inactive", "blocked": "Removal blocked", "pending": "Pending"}},
		{Name: "provides", Label: "Provides", Any: "Anything", Choices: true, Option: contractShort, Chip: contractShort},
	}}
	catalogFilters = filterView{Key: "catalogs", Path: "/", Params: []filterParam{
		{Name: "cq", Label: "Search"},
		{Name: "csource", Label: "Source", Any: "Any source", Values: []string{"subscription", "registration", "claim"},
			Text: map[string]string{"subscription": "Subscription", "registration": "Contributed", "claim": "Claimed only"}},
		{Name: "claimed", Label: "Claimed", Any: "Either", Values: []string{"yes", "no"},
			Text: map[string]string{"yes": "Claimed", "no": "Unclaimed"}},
	}}
)

// rememberedViews are the views the browser remembers filters for.
var rememberedViews = []*filterView{&installedFilters, &providerFilters, &catalogFilters}

// filterSpec is the remembered views' parameters as the page scripts read
// them from the page head: {key: {path, params: {name: [values] | null}}}.
func filterSpec() string {
	type view struct {
		Path   string              `json:"path"`
		Params map[string][]string `json:"params"`
	}
	out := map[string]view{}
	for _, v := range rememberedViews {
		vw := view{Path: v.Path, Params: map[string][]string{}}
		for _, p := range v.Params {
			vw.Params[p.Name] = p.Values
		}
		out[v.Key] = vw
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// filterChip is one filter the view applies, or one it ignored, with the
// link that drops it.
type filterChip struct {
	Name  string
	Label string
	// Value is the full value, which the chip's title holds; Text is the
	// words the chip shows.
	Value  string
	Text   string
	Remove string
}

// filters is a view's filters as its URL carries them.
type filters struct {
	View *filterView
	// Values are the filters applied, by parameter.
	Values map[string]string
	// Active are the filters applied, in the view's order, and Ignored
	// the enumerated values the view does not know.
	Active  []filterChip
	Ignored []filterChip
	// Clear is the view without its filters, keeping every other
	// parameter (a tab, say).
	Clear string
}

// parseFilters reads v's filters from q. An empty value is no filter. An
// enumerated value v does not know is ignored and named, never applied,
// and every link the view offers drops it.
func parseFilters(v *filterView, q url.Values) filters {
	f := filters{View: v, Values: map[string]string{}}
	var ignored []string
	for _, p := range v.Params {
		if val := strings.TrimSpace(q.Get(p.Name)); val != "" && p.Values != nil && !slices.Contains(p.Values, val) {
			ignored = append(ignored, p.Name)
		}
	}
	f.Clear = linkWithout(v.Path, q, v.paramNames()...)
	for _, p := range v.Params {
		val := strings.TrimSpace(q.Get(p.Name))
		if val == "" {
			continue
		}
		chip := filterChip{Name: p.Name, Label: p.Label, Value: val, Text: val}
		if slices.Contains(ignored, p.Name) {
			chip.Remove = linkWithout(v.Path, q, p.Name)
			f.Ignored = append(f.Ignored, chip)
			continue
		}
		chip.Remove = linkWithout(v.Path, q, append([]string{p.Name}, ignored...)...)
		if t, ok := p.Text[val]; ok {
			chip.Text = t
		} else if p.Chip != nil {
			chip.Text = p.Chip(val)
		}
		f.Values[p.Name] = val
		f.Active = append(f.Active, chip)
	}
	return f
}

func (v *filterView) paramNames() []string {
	out := make([]string, 0, len(v.Params))
	for _, p := range v.Params {
		out = append(out, p.Name)
	}
	return out
}

// linkWithout is path with q less the named parameters.
func linkWithout(path string, q url.Values, drop ...string) string {
	rest := url.Values{}
	for k, vs := range q {
		if slices.Contains(drop, k) {
			continue
		}
		for _, val := range vs {
			if val != "" {
				rest.Add(k, val)
			}
		}
	}
	if len(rest) == 0 {
		return path
	}
	return path + "?" + rest.Encode()
}

// selectOption is one option of a filter select.
type selectOption struct {
	Value, Text string
	Selected    bool
}

// field is one control of a filter form, in the view's parameter order.
type field struct {
	ID, Name, Label string
	// Search marks the view's free-text search, drawn wide with its
	// magnifier.
	Search bool
	// IsSelect says the control is a select; Select holds its options.
	IsSelect bool
	Select   []selectOption
	// Value, Placeholder, List and Suggestions describe a text input; List
	// is the id of its suggestions.
	Value, Placeholder string
	List               string
	Suggestions        []string
}

// formInput is what a view supplies to its filter form beyond the
// filters themselves.
type formInput struct {
	// Choices are the values a Choices parameter offers, drawn as a select
	// of the values present in the rows the caller may read. A parameter
	// without an entry stays a text input.
	Choices map[string][]string
	// Counts are the rows each option stands for, counted over the rows
	// before filtering: parameter, then value. An option without an entry
	// carries no count, never a zero; a parameter in NoCount carries none.
	Counts  map[string]map[string]int
	NoCount map[string]bool
	// Suggest are the suggestions of a text input (a datalist).
	Suggest      map[string][]string
	Placeholders map[string]string
	// Shown, Total and Scope draw the count row, "Showing n of M scope";
	// Total below zero hides it. ShownID is the id of the followed region
	// that holds it, Follow its data-follow topics.
	Shown, Total int
	Scope        string
	ShownID      string
	Follow       string
	// Variant is filtersCard (its own card) or filtersStrip (a flat strip
	// inside a panel), or empty for the plain form.
	Variant string
	// ClearText is the words of the link that drops every filter.
	ClearText string
}

// The two forms a filter form takes (design.md, "Filter forms").
const (
	filtersCard  = "card"
	filtersStrip = "strip"
)

// fields returns the form's controls in f's parameter order: a select for
// each enumerated parameter and each Choices parameter that has supplied
// values, a text input for the others.
func (f *filters) fields(in formInput) []field {
	out := make([]field, 0, len(f.View.Params))
	for _, p := range f.View.Params {
		id := "f-" + p.Name
		cur := f.Values[p.Name]
		fld := field{ID: id, Name: p.Name, Label: p.Label, Search: p.Label == "Search"}
		choices, haveChoices := in.Choices[p.Name]
		switch {
		case p.Values != nil:
			fld.IsSelect = true
			fld.Select = f.options(p, in, p.Values, cur)
		case p.Choices && haveChoices:
			fld.IsSelect = true
			fld.Select = f.options(p, in, choices, cur)
		default:
			fld.Value, fld.Placeholder = cur, in.Placeholders[p.Name]
			if s := in.Suggest[p.Name]; len(s) > 0 {
				fld.List, fld.Suggestions = id+"-list", s
			}
		}
		out = append(out, fld)
	}
	return out
}

// options returns the options of p's select: the empty choice in words,
// then each value as "Text (n)" when a count is known, and a value in the
// URL that is not among them as an extra selected option, so the form never
// silently drops a filter.
func (f *filters) options(p filterParam, in formInput, values []string, cur string) []selectOption {
	empty := p.Any
	if empty == "" {
		empty = "Any"
	}
	opts := []selectOption{{Value: "", Text: empty, Selected: cur == ""}}
	text := func(v string) string {
		switch {
		case p.Text[v] != "":
			return p.Text[v]
		case p.Option != nil:
			return p.Option(v)
		}
		return v
	}
	counts := in.Counts[p.Name]
	if in.NoCount[p.Name] {
		counts = nil
	}
	for _, v := range values {
		t := text(v)
		if n, ok := counts[v]; ok {
			t = fmt.Sprintf("%s (%d)", t, n)
		}
		opts = append(opts, selectOption{Value: v, Text: t, Selected: cur == v})
	}
	if cur != "" && !slices.Contains(values, cur) {
		opts = append(opts, selectOption{Value: cur, Text: text(cur), Selected: true})
	}
	return opts
}

// moduleChip shows a module path by its last path segment; a package's
// source string, which holds a space, stays whole.
func moduleChip(v string) string {
	if strings.Contains(v, " ") {
		return v
	}
	return v[strings.LastIndex(v, "/")+1:]
}

// linkWith is path with q and one parameter set to value.
func linkWith(path string, q url.Values, name, value string) string {
	next := url.Values{}
	for k, vs := range q {
		for _, val := range vs {
			if val != "" {
				next.Add(k, val)
			}
		}
	}
	next.Set(name, value)
	return path + "?" + next.Encode()
}

// hiddenField is a parameter a filter form carries along unchanged, so a
// form on a page with several views keeps the others' filters and tab.
type hiddenField struct {
	Name, Value string
}

// keepHidden is every non-empty parameter of q but the named ones.
func keepHidden(q url.Values, drop ...string) []hiddenField {
	keys := make([]string, 0, len(q))
	for k := range q {
		if !slices.Contains(drop, k) {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	var out []hiddenField
	for _, k := range keys {
		for _, v := range q[k] {
			if v != "" {
				out = append(out, hiddenField{Name: k, Value: v})
			}
		}
	}
	return out
}

// filterForm is one view's filter form, chips and notes, as the
// filter-form partial renders it.
type filterForm struct {
	Action  string
	Aria    string
	Fields  []field
	Filters filters
	Hidden  []hiddenField
	// Variant is filtersCard or filtersStrip; empty draws the plain form of
	// the owner page's Resources reason form.
	Variant string
	// Shown, Total and Scope draw the always-present count row; Total
	// below zero hides it. ShownID and Follow make it a followed region.
	Shown, Total int
	Scope        string
	ShownID      string
	Follow       string
	// ClearText is the words of the link that drops every filter.
	ClearText string
}

// newChoiceForm builds a view's filter form from its filters and what the
// view supplies: choices, counts, the count row.
func newChoiceForm(f filters, aria string, hidden []hiddenField, in formInput) filterForm {
	if in.ClearText == "" {
		in.ClearText = "Clear all"
	}
	return filterForm{
		Action: f.View.Path, Aria: aria, Fields: f.fields(in), Filters: f, Hidden: hidden,
		Variant: in.Variant, Shown: in.Shown, Total: in.Total, Scope: in.Scope,
		ShownID: in.ShownID, Follow: in.Follow, ClearText: in.ClearText,
	}
}

// newFilterForm builds a filter form that offers suggestions to its text
// inputs and draws no count row, "Clear filters" as its clear link: the form
// of a view that has not moved to newChoiceForm (the owner page's Resources
// reason form).
func newFilterForm(f filters, aria string, hidden []hiddenField, suggest map[string][]string, placeholders map[string]string) filterForm {
	return newChoiceForm(f, aria, hidden, formInput{Suggest: suggest, Placeholders: placeholders, Total: -1, ClearText: "Clear filters"})
}
