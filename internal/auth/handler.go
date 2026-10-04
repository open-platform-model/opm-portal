package auth

import (
	"io"
	"net/http"
	"strings"
)

// securityHeaders are set on every response, refusals included. The
// portal serves no HTML yet, so the policy allows nothing at all.
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
	launch := http.HandlerFunc(l.serveLaunch)
	inner := cop.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == LaunchPath {
			launch(w, r)
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

// serveLaunch exchanges the launch token for the session. A request that
// already carries the live session is sent on whatever its token, so
// opening the link again after a launch lands where the first one did.
func (l *Local) serveLaunch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		l.refuse(w, http.StatusMethodNotAllowed, "method not allowed\n")
		return
	}
	if _, ok := l.sessionOf(r); ok {
		http.Redirect(w, r, l.landing, http.StatusSeeOther)
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
	http.Redirect(w, r, l.landing, http.StatusSeeOther)
}

func (l *Local) refuse(w http.ResponseWriter, status int, body string) {
	h := w.Header()
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if _, err := io.WriteString(w, body); err != nil {
		l.log.Debug("writing a refusal", "status", status, "error", err)
	}
}
