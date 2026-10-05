#!/usr/bin/env bash
# shellcheck source-path=SCRIPTDIR
# m1.sh: run the milestone 1 e2e test. The built opm-portal binary serves the fixture cluster in
# local mode, and the test checks the Platform's claims, the instance list's two axes, a
# CLI-owned instance, a namespace-scoped reader and a scripted image break through the read API
# and its stream. The image break patches podinfo's image tag and reverts it before the test
# ends. Needs the cluster task e2e:up created (E2E_CLUSTER as for e2e:up).
# shellcheck source=lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

require_fixture_cluster
cd "$REPO_ROOT" || die "cannot enter $REPO_ROOT"
log "running the milestone 1 test against $CLUSTER"
OPM_PORTAL_E2E_KUBECONFIG=$KC OPM_PORTAL_E2E_CONTEXT=$CTX \
  go test -tags e2e -count=1 -timeout 20m -run '^TestM1' -v ./cmd/opm-portal
