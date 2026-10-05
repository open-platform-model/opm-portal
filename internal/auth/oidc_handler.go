package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// hsts is sent beside the shared security headers in-cluster, where the
// portal is reached over TLS.
const hsts = "max-age=31536000"

// Refusal and status bodies. They name no token, code, cookie or claim.
const (
	bodySignInExpired = "this sign-in was not started in this browser, or took too long; start again at " + LoginPath + "\n"
	bodySignInRefused = "the identity provider did not sign you in\n"
	bodySignInFailed  = "signing in failed: the identity provider's answer could not be verified\n"
	bodyUnmapped      = "your account maps to no portal user; ask the portal's administrator\n"
	bodySignedOut     = "you are signed out of the portal; your identity provider may still have you signed in\n"
)

// Handler returns next behind the in-cluster front door. Every request gets
// the security headers and HSTS and passes cross-origin protection. The
// sign-in, callback and sign-out paths are served here; a browser
// navigation with no session and no Authorization header is sent to sign
// in; everything else goes to next, which authenticates through
// Authenticate.
func (o *OIDC) Handler(next http.Handler) http.Handler {
	cop := http.NewCrossOriginProtection()
	inner := cop.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case LoginPath:
			o.serveLogin(w, r)
		case CallbackPath:
			o.serveCallback(w, r)
		case LogoutPath:
			o.serveLogout(w, r)
		default:
			if o.wantsSignIn(r) {
				w.Header().Set("Cache-Control", "no-store")
				http.Redirect(w, r, LoginPath+"?"+url.Values{"return": {r.URL.RequestURI()}}.Encode(), http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		}
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		for _, kv := range securityHeaders {
			h.Set(kv[0], kv[1])
		}
		h.Set("Strict-Transport-Security", hsts)
		inner.ServeHTTP(w, r)
	})
}

// wantsSignIn reports whether r is a browser navigating to a page with
// nothing that could authenticate it. API clients and htmx requests get
// the read API's 401 instead.
func (o *OIDC) wantsSignIn(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if len(r.Header.Values("Authorization")) > 0 || r.Header.Get("HX-Request") != "" {
		return false
	}
	switch r.Header.Get("Sec-Fetch-Mode") {
	case "navigate":
	case "":
		if !strings.Contains(r.Header.Get("Accept"), "text/html") {
			return false
		}
	default:
		return false
	}
	_, _, err := o.session(r)
	return err != nil
}

// serveLogin starts the code flow with PKCE: fresh state, nonce and
// verifier, sealed into the login cookie, and a redirect to the issuer.
func (o *OIDC) serveLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeText(w, o.log, http.StatusMethodNotAllowed, "method not allowed\n")
		return
	}
	ret := localPath(r.URL.Query().Get("return"))
	if _, _, err := o.session(r); err == nil {
		http.Redirect(w, r, ret, http.StatusSeeOther)
		return
	}
	state := loginState{
		State:    rand.Text(),
		Nonce:    rand.Text(),
		Verifier: oauth2.GenerateVerifier(),
		Return:   ret,
		Expires:  o.cfg.Now().Add(loginTTL).Unix(),
	}
	sealed, err := o.logins.seal(state)
	if err != nil {
		o.log.Error("starting a sign-in", "error", err)
		writeText(w, o.log, http.StatusInternalServerError, "could not start a sign-in\n")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     loginCookie,
		Value:    sealed,
		Path:     "/",
		MaxAge:   int(loginTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, o.oauth.AuthCodeURL(state.State, oidc.Nonce(state.Nonce), oauth2.S256ChallengeOption(state.Verifier)), http.StatusFound)
}

// serveCallback finishes the code flow and starts a session. Every check
// fails closed, and the login cookie is cleared whatever happens.
func (o *OIDC) serveCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeText(w, o.log, http.StatusMethodNotAllowed, "method not allowed\n")
		return
	}
	http.SetCookie(w, expiredCookie(loginCookie))
	state, ok := o.loginOf(r)
	if !ok {
		o.log.Warn("refused a sign-in callback: no matching sign-in was started in this browser")
		writeText(w, o.log, http.StatusBadRequest, bodySignInExpired)
		return
	}
	q := r.URL.Query()
	if q.Get("error") != "" {
		o.log.Warn("the identity provider refused a sign-in")
		writeText(w, o.log, http.StatusForbidden, bodySignInRefused)
		return
	}
	ctx, cancel := context.WithTimeout(oidc.ClientContext(r.Context(), o.cfg.HTTPClient), defaultIssuerTimeout)
	defer cancel()
	token, err := o.oauth.Exchange(ctx, q.Get("code"), oauth2.VerifierOption(state.Verifier))
	if err != nil {
		o.log.Warn("refused a sign-in: the code exchange failed")
		writeText(w, o.log, http.StatusForbidden, bodySignInFailed)
		return
	}
	rawID, ok := token.Extra("id_token").(string)
	if !ok || rawID == "" {
		o.log.Warn("refused a sign-in: the token response carries no ID token")
		writeText(w, o.log, http.StatusForbidden, bodySignInFailed)
		return
	}
	idToken, err := o.idTokens.Verify(ctx, rawID)
	if err != nil || subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(state.Nonce)) != 1 {
		o.log.Warn("refused a sign-in: the ID token does not verify or carries another nonce")
		writeText(w, o.log, http.StatusForbidden, bodySignInFailed)
		return
	}
	id, err := o.identityOf(idToken)
	if err != nil {
		o.log.Warn("refused a sign-in", "reason", err.Error())
		writeText(w, o.log, http.StatusForbidden, bodyUnmapped)
		return
	}
	http.SetCookie(w, sessionCookieFor(o.sessions.create(id, o.cfg.SessionTTL), o.cfg.SessionTTL))
	o.log.Info("browser signed in", "user", id.Username)
	http.Redirect(w, r, state.Return, http.StatusSeeOther)
}

// loginOf returns the sign-in r's login cookie carries, when it is
// unexpired and its state is the callback's.
func (o *OIDC) loginOf(r *http.Request) (loginState, bool) {
	c, err := r.Cookie(loginCookie)
	if err != nil || c.Value == "" {
		return loginState{}, false
	}
	state, err := o.logins.open(c.Value)
	if err != nil || o.cfg.Now().Unix() >= state.Expires || state.State == "" {
		return loginState{}, false
	}
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(state.State)) != 1 {
		return loginState{}, false
	}
	return state, true
}

// serveLogout ends the browser's session and clears its cookie, then sends
// the browser to the issuer's end-session endpoint when it has one. It is
// POST only, so cross-origin protection guards it.
func (o *OIDC) serveLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeText(w, o.log, http.StatusMethodNotAllowed, "method not allowed\n")
		return
	}
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		o.sessions.delete(c.Value)
	}
	http.SetCookie(w, expiredCookie(sessionCookie))
	o.log.Info("browser signed out")
	if o.endSession == "" {
		writeText(w, o.log, http.StatusOK, bodySignedOut)
		return
	}
	u, err := url.Parse(o.endSession)
	if err != nil {
		writeText(w, o.log, http.StatusOK, bodySignedOut)
		return
	}
	q := u.Query()
	q.Set("client_id", o.cfg.ClientID)
	if o.cfg.PostLogoutRedirectURL != "" {
		q.Set("post_logout_redirect_uri", o.cfg.PostLogoutRedirectURL)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

// localPath returns p when it is a path on this portal, and "/" for
// anything that could leave it: a scheme, a host, "//" or "/\".
func localPath(p string) string {
	if p == "" || p[0] != '/' || strings.HasPrefix(p, "//") || strings.HasPrefix(p, "/\\") || strings.ContainsAny(p, "\r\n\t") {
		return "/"
	}
	u, err := url.Parse(p)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return "/"
	}
	return p
}
