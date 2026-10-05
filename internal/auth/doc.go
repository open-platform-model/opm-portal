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
// In-cluster (milestone 2) OIDC signs browsers in through an issuer with
// the authorization code flow and PKCE, holds their sessions in memory, and
// accepts the issuer's bearer tokens for the configured audience. Both map
// to an identity that fails closed: an empty username is refused before
// any Kubernetes call, system: names never come from the issuer, and every
// identity carries system:authenticated. Its Authenticate has the same
// contract as Local's.
//
// Tokens, codes, cookie values and the client secret are never logged;
// sessions and the launch token are kept only as digests.
package auth
