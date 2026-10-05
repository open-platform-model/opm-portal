## REMOVED Requirements

### Requirement: The OIDC authenticator refuses an unsafe configuration

**Reason**: Owner decision 2026-10-05: OIDC is removed and kept as a future plan (portal:D6). No
command constructed the authenticator.
**Migration**: None; nothing used it. The design is kept in `docs/DESIGN.md` (portal:D6:R6) and
the code in the archived change `2026-10-05-add-oidc-sessions` and commit f5a8eaa.

### Requirement: Claims map to an identity that fails closed

**Reason**: Owner decision 2026-10-05: OIDC is removed and kept as a future plan (portal:D6).
**Migration**: None. The read API still refuses an identity that names no one before any review
or read (portal:D6:R2), covered by `TestEmptyIdentityNeverReachesTheClusterInCluster`.

### Requirement: Bearer tokens are accepted only from the configured issuer and audience

**Reason**: Owner decision 2026-10-05: OIDC is removed and kept as a future plan (portal:D6:R8).
**Migration**: None; nothing used it.

### Requirement: Signing keys are cached and refetched with a lower bound

**Reason**: Owner decision 2026-10-05: OIDC is removed and kept as a future plan (portal:D6).
**Migration**: None; nothing used it.

### Requirement: Browsers sign in with the authorization code flow and PKCE

**Reason**: Owner decision 2026-10-05: OIDC is removed and kept as a future plan (portal:D6).
**Migration**: None; nothing used it.

### Requirement: Sessions are server-side and revocable

**Reason**: Owner decision 2026-10-05: OIDC is removed and kept as a future plan (portal:D6).
Session lifetime and caps stay open on the roadmap (issue 33).
**Migration**: None; nothing used it.

### Requirement: The in-cluster front door protects every response

**Reason**: Owner decision 2026-10-05: OIDC is removed and kept as a future plan (portal:D6).
**Migration**: None; local mode's front door (`local-mode` spec) is unchanged.
