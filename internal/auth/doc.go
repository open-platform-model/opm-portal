// Package auth owns who may reach the portal.
//
// In local mode (milestone 1) the portal runs on the user's machine with
// their kubeconfig, so the kubeconfig's identity is the only one it serves.
// Local admits a browser through a one-time launch token exchanged for a
// session cookie, and refuses every request whose Host is not the portal's
// loopback address, every cross-origin non-safe request, and, through
// Authenticate, every request without the session. Every response carries
// a strict Content-Security-Policy and the other security headers.
//
// The token and the cookie value are never logged and are kept only as
// digests.
package auth
