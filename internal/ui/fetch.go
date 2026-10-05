package ui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"

	v1 "github.com/open-platform-model/opm-portal/api/v1alpha1"
)

// recorder is the in-memory response the read API writes a document into.
type recorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}

// fetch reads one read API document for the caller of r: it serves GET
// path (below APIBase) through the API handler with r's cookies and Host,
// and decodes the 200 body into out. Any other answer is returned as its
// problem document; a body that is not one becomes upstream_unavailable.
// The read API authenticates and authorizes the request itself.
func (h *Handler) fetch(r *http.Request, path string, query url.Values, out any) *v1.Problem {
	u := h.cfg.APIBase + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	// The request is served by the API handler in this process; it is never
	// dialed, so its path cannot reach another host.
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, http.NoBody) //nolint:gosec // in-process, never dialed
	if err != nil {
		return unavailable()
	}
	req.Host = r.Host
	req.RemoteAddr = r.RemoteAddr
	if c := r.Header.Values("Cookie"); len(c) > 0 {
		req.Header["Cookie"] = c
	}
	rec := &recorder{header: http.Header{}}
	h.cfg.API.ServeHTTP(rec, req)
	if rec.status == http.StatusOK {
		if err := json.Unmarshal(rec.body.Bytes(), out); err != nil {
			h.log.Warn("decoding a read API document", "path", path, "error", err)
			return unavailable()
		}
		return nil
	}
	var p v1.Problem
	if err := json.Unmarshal(rec.body.Bytes(), &p); err != nil || p.Code == "" {
		return &v1.Problem{Status: rec.status, Code: v1.CodeUpstreamUnavailable}
	}
	return &p
}

func unavailable() *v1.Problem {
	return &v1.Problem{Status: http.StatusServiceUnavailable, Code: v1.CodeUpstreamUnavailable}
}
