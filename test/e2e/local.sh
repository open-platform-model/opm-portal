#!/usr/bin/env bash
# shellcheck source-path=SCRIPTDIR
# local.sh: run the local-mode e2e test. The built opm-portal binary serves the fixture cluster
# as its kubeconfig's user, and the test reads through it with and without the session. Needs
# the cluster task e2e:up created (E2E_CLUSTER as for e2e:up).
# shellcheck source=lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

require_fixture_cluster
cd "$REPO_ROOT" || die "cannot enter $REPO_ROOT"
log "running the local-mode test against $CLUSTER"
OPM_PORTAL_E2E_KUBECONFIG=$KC OPM_PORTAL_E2E_CONTEXT=$CTX \
  go test -tags e2e -count=1 -run '^TestLocalMode$' -v ./cmd/opm-portal
