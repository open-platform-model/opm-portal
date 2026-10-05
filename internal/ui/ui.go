package ui

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// PagePolicy is the Content-Security-Policy of every page: the front door's
// default-src 'none', widened only to the portal's own origin for what a
// page loads. No inline script or style and no eval are allowed.
const PagePolicy = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; " +
	"connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

// defaultAPIBase is the read API's cluster path the pages read from.
const defaultAPIBase = "/api/v1alpha1/clusters/default"

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// Config wires a Handler.
type Config struct {
	// API is the read API's handler. Every document a page shows is fetched
	// through it in-process, with the incoming request's cookies, so it
	// authenticates and authorizes the caller itself.
	API http.Handler
	// APIBase is the path of the cluster the pages read. Default
	// /api/v1alpha1/clusters/default.
	APIBase string
	// Now is the clock relative times are shown against. Default time.Now.
	Now func() time.Time
	// Logger receives operational logs. Default: discarded.
	Logger *slog.Logger
}

// Handler serves the pages and their static assets.
type Handler struct {
	cfg    Config
	log    *slog.Logger
	mux    *http.ServeMux
	pages  map[string]*template.Template
	static http.Handler
}

// New returns a Handler over cfg. API is required.
func New(cfg Config) (*Handler, error) {
	if cfg.API == nil {
		return nil, errors.New("ui: no read API")
	}
	if cfg.APIBase == "" {
		cfg.APIBase = defaultAPIBase
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, fmt.Errorf("ui: static assets: %w", err)
	}
	h := &Handler{cfg: cfg, log: cfg.Logger, mux: http.NewServeMux(), static: http.StripPrefix("/static/", http.FileServerFS(sub))}
	if h.pages, err = parsePages(h.templateFuncs()); err != nil {
		return nil, err
	}
	h.routes()
	return h, nil
}

// ServeHTTP serves GET and HEAD only.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) routes() {
	h.mux.HandleFunc("/{$}", h.platformPage)
	h.mux.HandleFunc("/platform/registrations/{name}/events", h.registrationEvents)
	h.mux.HandleFunc("/installed", h.installedPage)
	h.mux.HandleFunc("/instances", listRedirect(instanceKind.Topic))
	h.mux.HandleFunc("/packages", listRedirect("package"))
	for _, k := range []ownerKind{instanceKind, packageKind} {
		h.mux.HandleFunc("/"+k.Path+"/{namespace}/{name}", h.ownerPage(k))
		h.mux.HandleFunc("/"+k.Path+"/{namespace}/{name}/node", h.ownerNode(k))
		h.mux.HandleFunc("/"+k.Path+"/{namespace}/{name}/object", h.ownerObject(k))
		h.mux.HandleFunc("/"+k.Path+"/{namespace}/{name}/events", h.ownerEvents(k))
	}
	h.mux.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		h.static.ServeHTTP(w, r)
	})
	h.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		h.render(w, r, http.StatusNotFound, "message", page{Title: "Not found", Main: message{
			Heading: "Nothing here",
			Text:    "The portal has no page at this address.",
		}})
	})
}

// render writes a page, or only its main region when htmx asked for a
// fragment of it, with the page policy.
func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, name string, p page) {
	t, ok := h.pages[name]
	if !ok {
		h.log.Error("no such page template", "page", name)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	p.Path = r.URL.RequestURI()
	if p.Canonical == "" {
		p.Canonical = r.URL.Path
	}
	p.StreamURL = h.cfg.APIBase + "/stream"
	p.Header = h.header(r)
	p.FilterSpec = filterSpec()
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", p); err != nil {
		h.log.Error("rendering a page", "page", name, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeHTML(w, status, buf.Bytes(), h.log)
}

// renderFragment writes one fragment template: alone for htmx, inside the
// layout for a plain request, so every fragment link also works without
// script.
func (h *Handler) renderFragment(w http.ResponseWriter, r *http.Request, status int, name string, p page) {
	if r.Header.Get("HX-Request") != "true" || r.Header.Get("HX-Boosted") == "true" {
		h.render(w, r, status, name, p)
		return
	}
	t, ok := h.pages[name]
	if !ok {
		h.log.Error("no such fragment template", "fragment", name)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "content", p); err != nil {
		h.log.Error("rendering a fragment", "fragment", name, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeHTML(w, status, buf.Bytes(), h.log)
}

func writeHTML(w http.ResponseWriter, status int, body []byte, log *slog.Logger) {
	hd := w.Header()
	hd.Set("Content-Type", "text/html; charset=utf-8")
	hd.Set("Cache-Control", "no-store")
	hd.Set("Content-Security-Policy", PagePolicy)
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Add("Vary", "HX-Request")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		log.Debug("writing a page", "error", err)
	}
}

// page is what the layout renders.
type page struct {
	Title string
	// Nav names the active navigation entry: platform or installed.
	Nav string
	// Header is what the page header says about the connection.
	Header header
	// FilterSpec is the remembered views' filter parameters, for the page
	// scripts.
	FilterSpec string
	// Topics are the stream topics the page follows.
	Topics []string
	// Canonical is the page's own address; after a launch the page script
	// puts it in the address bar.
	Canonical string
	Path      string
	StreamURL string
	Main      any
}

// header is the connection the page header names: the cluster, the reader
// and the version, from the read API's Cluster document, the one place
// they show (portal:D18). OK is false when that document could not be
// read; the header then says unknown in the degraded style and the page
// still renders.
type header struct {
	OK bool
	// Cluster is the kubeconfig context, or "in cluster".
	Cluster string
	// Context keys what the browser remembers: the context name, or
	// in-cluster.
	Context  string
	Username string
	Version  string
}

// header reads the Cluster document for the caller of r.
func (h *Handler) header(r *http.Request) header {
	var c v1.Cluster
	if p := h.fetch(r, "", nil, &c); p != nil {
		return header{}
	}
	hd := header{OK: true, Cluster: c.Context, Context: c.Context, Username: c.ReadingAs.Username, Version: c.KubernetesVersion}
	if c.Source == v1.SourceInCluster || c.Context == "" {
		hd.Cluster, hd.Context = "in cluster", v1.SourceInCluster
	}
	return hd
}

// message is a page that only says something: not found, sign in.
type message struct {
	Heading string
	Text    string
	Code    string
}
