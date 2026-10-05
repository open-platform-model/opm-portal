package ui

import (
	"encoding/json"
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
	// Text names a value for the chip; a value it does not name is shown
	// as it is.
	Text map[string]string
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
	{Name: "kind", Label: "Kind", Values: []string{"instance", "package"},
		Text: map[string]string{"instance": "Instance", "package": "Package"}},
	{Name: "provider", Label: "Provider", Values: []string{"yes", "no"},
		Text: map[string]string{"yes": "Provider", "no": "Not a provider"}},
	{Name: "uses", Label: "Uses"},
	{Name: "namespace", Label: "Namespace"},
	{Name: "health", Label: "Health", Values: healthValues, Text: healthText},
	{Name: "applied", Label: "Applied", Values: appliedValues, Text: appliedText},
	{Name: "owner", Label: "Owner", Values: []string{"controller", "cli"},
		Text: map[string]string{"controller": "controller", "cli": "cli"}},
	{Name: "module", Label: "Module"},
}}

// providerFilters and catalogFilters are the Platform page's two tabs.
var (
	providerFilters = filterView{Key: "providers", Path: "/", Params: []filterParam{
		{Name: "pq", Label: "Search"},
		{Name: "pstatus", Label: "Status", Values: []string{"active", "accepted", "refused", "blocked", "pending"},
			Text: map[string]string{"active": "active", "accepted": "accepted", "refused": "refused", "blocked": "removal blocked", "pending": "pending"}},
		{Name: "provides", Label: "Provides"},
	}}
	catalogFilters = filterView{Key: "catalogs", Path: "/", Params: []filterParam{
		{Name: "cq", Label: "Search"},
		{Name: "csource", Label: "Source", Values: []string{"subscription", "registration", "claim"},
			Text: map[string]string{"subscription": "subscribed", "registration": "from a provider", "claim": "claimed only"}},
		{Name: "claimed", Label: "Claimed", Values: []string{"yes", "no"}},
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
	Name   string
	Label  string
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

// selectField and textField are the filter form's controls.
type selectField struct {
	ID, Name, Label string
	Options         []selectOption
}

type selectOption struct {
	Value, Text string
	Selected    bool
}

type textField struct {
	ID, Name, Label, Value, Placeholder string
	// Search: the view's free-text search, drawn first and wide.
	Search bool
	// List is the id of the field's suggestions, if it has any.
	List        string
	Suggestions []string
}

// form returns the filter form's controls for f: a select for each
// enumerated parameter, a text field for the others, with suggestions by
// parameter name.
func (f *filters) form(suggest map[string][]string, placeholders map[string]string) (selects []selectField, texts []textField) {
	for _, p := range f.View.Params {
		id := "f-" + p.Name
		cur := f.Values[p.Name]
		if p.Values == nil {
			t := textField{ID: id, Name: p.Name, Label: p.Label, Value: cur, Placeholder: placeholders[p.Name], Search: p.Label == "Search"}
			if s := suggest[p.Name]; len(s) > 0 {
				t.List, t.Suggestions = id+"-list", s
			}
			texts = append(texts, t)
			continue
		}
		s := selectField{ID: id, Name: p.Name, Label: p.Label, Options: []selectOption{{Value: "", Text: "Any", Selected: cur == ""}}}
		for _, v := range p.Values {
			text := v
			if t, ok := p.Text[v]; ok {
				text = t
			}
			s.Options = append(s.Options, selectOption{Value: v, Text: text, Selected: cur == v})
		}
		selects = append(selects, s)
	}
	return selects, texts
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
	Fields  formFields
	Filters filters
	Hidden  []hiddenField
}

// formFields are a filter form's controls.
type formFields struct {
	Selects []selectField
	Texts   []textField
}

func newFilterForm(f filters, aria string, hidden []hiddenField, suggest map[string][]string, placeholders map[string]string) filterForm {
	sel, txt := f.form(suggest, placeholders)
	return filterForm{Action: f.View.Path, Aria: aria, Fields: formFields{Selects: sel, Texts: txt}, Filters: f, Hidden: hidden}
}
