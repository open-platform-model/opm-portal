#!/usr/bin/env bash
# shellcheck source-path=SCRIPTDIR
# down.sh: delete the e2e fixture cluster opm-portal-e2e and its kubeconfig. Touches no other
# cluster. Uses the provider up.sh recorded unless E2E_PROVIDER is set. Leaves the downloaded CLI and the CUE cache in .e2e/ for the next run.
# shellcheck source=lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

if cluster_exists; then
  log "deleting kind cluster $CLUSTER ($E2E_PROVIDER)"
  kind_cmd delete cluster --name "$CLUSTER" --kubeconfig "$KC"
else
  log "no kind cluster $CLUSTER ($E2E_PROVIDER)"
fi
rm -f "$KC" "$PROVIDER_FILE"
