package auth

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// securityHeaders are set on every response, refusals included. The policy
// allows nothing at all; the UI's pages replace it with their own, which
// widens it only to the portal's own origin.
var securityHeaders = [][2]string{
	{"Content-Security-Policy", "default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"},
	{"X-Content-Type-Options", "nosniff"},
	{"Referrer-Policy", "no-referrer"},
	{"X-Frame-Options", "DENY"},
	{"Cross-Origin-Opener-Policy", "same-origin"},
	{"Cross-Origin-Resource-Policy", "same-origin"},
}

// Refusal bodies. They name no host, token or cookie.
const (
	bodyHost   = "this portal answers only on its loopback address\n"
	bodyLaunch = "this launch link is not valid: it was already used, or is not this portal's; restart opm-portal serve for a new one\n"
)

// Handler returns next behind local mode's front door. In order, every
// request gets the security headers, is refused unless its Host is the
// portal's loopback address and port (against DNS rebinding), is refused
// when it is a cross-origin non-safe request, and GET LaunchPath exchanges
// the launch token for the session. Everything else goes to next, which
// authenticates through Authenticate.
func (l *Local) Handler(next http.Handler) http.Handler {
	cop := http.NewCrossOriginProtection()
	inner := cop.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == LaunchPath {
			l.serveLaunch(w, r, next)
			return
		}
		next.ServeHTTP(w, r)
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		for _, kv := range securityHeaders {
			h.Set(kv[0], kv[1])
		}
		if !l.hosts[strings.ToLower(r.Host)] {
			l.log.Warn("refused a request for another host", "method", r.Method, "path", r.URL.Path)
			l.refuse(w, http.StatusForbidden, bodyHost)
			return
		}
		inner.ServeHTTP(w, r)
	})
}

// serveLaunch exchanges the launch token for the session and answers with
// the landing page itself, served by next under the new session. A request
// that already carries the live session gets the landing page whatever its
// token, so opening the link again lands where the first one did.
//
// It does not redirect: serve --open starts the launch from a file:// page,
// and a browser treats a redirect as part of that cross-site navigation,
// withholding the new SameSite=Strict cookie from the landing request. The
// page's own requests start on the portal's origin, so they carry it; its
// script then replaces the address, so the spent token leaves the history.
// Referrer-Policy keeps the launch URL out of every later request.
func (l *Local) serveLaunch(w http.ResponseWriter, r *http.Request, next http.Handler) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		l.refuse(w, http.StatusMethodNotAllowed, "method not allowed\n")
		return
	}
	if _, ok := l.sessionOf(r); ok {
		next.ServeHTTP(w, l.landingRequest(r, nil))
		return
	}
	cookie, ok := l.launch(r.URL.Query().Get("token"))
	if !ok {
		l.log.Warn("refused a launch: the token is missing, wrong or already used")
		l.refuse(w, http.StatusForbidden, bodyLaunch)
		return
	}
	http.SetCookie(w, cookie)
	l.log.Info("browser session started")
	next.ServeHTTP(w, l.landingRequest(r, cookie))
}

// landingRequest is r as a request for the landing page, carrying the new
// session's cookie when one was just set, and no token.
func (l *Local) landingRequest(r *http.Request, cookie *http.Cookie) *http.Request {
	lr := r.Clone(r.Context())
	u := *r.URL
	u.Path, u.RawPath, u.RawQuery = l.landing, "", ""
	lr.URL, lr.RequestURI = &u, l.landing
	if cookie != nil {
		lr.Header.Del("Cookie")
		lr.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	}
	return lr
}

func (l *Local) refuse(w http.ResponseWriter, status int, body string) {
	writeText(w, l.log, status, body)
}

// writeText answers with a short plain-text body that is never cached.
func writeText(w http.ResponseWriter, log *slog.Logger, status int, body string) {
	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if _, err := io.WriteString(w, body); err != nil {
		log.Debug("writing a plain-text answer", "status", status, "error", err)
	}
}
