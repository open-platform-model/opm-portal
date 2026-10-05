package ui

import (
	"fmt"
	"html/template"
	"strings"
)

// baseTemplates are parsed into every page: the layout and the shared
// partials. Every other file defines one page's "content".
var baseTemplates = []string{"layout.html", "partials.html", "panel-body.html"}

// pageTemplates name each page and fragment by its file.
var pageTemplates = []string{"platform", "installed", "owner", "message", "object", "events", "panel", "problem"}

// parsePages parses one template set per page, each the base templates and
// the page's own file.
func parsePages(fm template.FuncMap) (map[string]*template.Template, error) {
	paths := make([]string, 0, len(baseTemplates))
	for _, b := range baseTemplates {
		paths = append(paths, "templates/"+b)
	}
	base, err := template.New("base").Funcs(fm).ParseFS(templateFS, paths...)
	if err != nil {
		return nil, fmt.Errorf("ui: parsing the layout: %w", err)
	}
	pages := make(map[string]*template.Template, len(pageTemplates))
	for _, name := range pageTemplates {
		t, err := base.Clone()
		if err != nil {
			return nil, fmt.Errorf("ui: %s: %w", name, err)
		}
		if t, err = t.ParseFS(templateFS, "templates/"+name+".html"); err != nil {
			return nil, fmt.Errorf("ui: parsing %s: %w", name, err)
		}
		pages[name] = t
	}
	return pages, nil
}

// templateFuncs are the functions templates call; stamp needs the
// handler's clock.
func (h *Handler) templateFuncs() template.FuncMap {
	fm := template.FuncMap{"stamp": h.stamp, "lower": strings.ToLower}
	for k, v := range funcs {
		fm[k] = v
	}
	return fm
}
