# shellcheck shell=bash source-path=SCRIPTDIR
# Shared environment for the e2e fixture cluster scripts. Source it; do not run it.
#
# Every cluster call goes through k() or opm_k(), which name the kubeconfig and the context
# explicitly, and KUBECONFIG is unset, so a developer's own clusters are never read or written.

set -euo pipefail

E2E_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "$E2E_DIR/../.." && pwd)

_operator_override=${OPM_OPERATOR_VERSION-}
# shellcheck source=versions.env
. "$E2E_DIR/versions.env"
if [ -n "$_operator_override" ]; then
  OPM_OPERATOR_VERSION=$_operator_override
fi

# E2E_CLUSTER names a second fixture cluster beside the default one; it must start with
# opm-portal-e2e, so no script ever creates, reads or deletes a cluster outside that family.
CLUSTER=${E2E_CLUSTER:-opm-portal-e2e}
case "$CLUSTER" in
  opm-portal-e2e | opm-portal-e2e-[a-z0-9]*) ;;
  *) printf 'e2e: error: E2E_CLUSTER must be opm-portal-e2e or start with opm-portal-e2e-, got %s\n' "$CLUSTER" >&2; exit 1 ;;
esac
[[ $CLUSTER =~ ^[a-z0-9-]+$ ]] || { printf 'e2e: error: E2E_CLUSTER %s holds characters other than a-z, 0-9 and -\n' "$CLUSTER" >&2; exit 1; }
CTX=kind-$CLUSTER
STATE_DIR=$REPO_ROOT/.e2e
# The default cluster keeps its kubeconfig and provider in .e2e/; any other keeps them in
# .e2e/clusters/<name>/, so deleting one never removes the other's kubeconfig. The CLI download
# and the CUE cache stay shared.
CLUSTER_STATE=$STATE_DIR
[ "$CLUSTER" = opm-portal-e2e ] || CLUSTER_STATE=$STATE_DIR/clusters/$CLUSTER
KC=$CLUSTER_STATE/kubeconfig
OPM_BIN=$STATE_DIR/bin/opm-$OPM_CLI_VERSION
# shellcheck disable=SC2034 # read by the scripts that source this file
FIXTURES=$E2E_DIR/fixtures/f1
# up.sh records the provider it created the cluster with, so capture and down find that cluster
# without being told again. An explicit E2E_PROVIDER wins; podman is the default.
PROVIDER_FILE=$CLUSTER_STATE/provider
if [ -z "${E2E_PROVIDER:-}" ] && [ -f "$PROVIDER_FILE" ]; then
  E2E_PROVIDER=$(cat "$PROVIDER_FILE")
fi
E2E_PROVIDER=${E2E_PROVIDER:-podman}

unset KUBECONFIG
# The opm CLI resolves modules from GHCR only, with a cache of its own, so a developer's
# ~/.opm/config.cue registry mapping and CUE cache take no part.
export OPM_REGISTRY=testing.opmodel.dev=ghcr.io/open-platform-model,opmodel.dev=ghcr.io/open-platform-model,registry.cue.works
export CUE_CACHE_DIR=$STATE_DIR/cue-cache

log() { printf '%s e2e: %s\n' "$(date -u +%H:%M:%S)" "$*" >&2; }
die() { log "error: $*"; exit 1; }

k() { kubectl --kubeconfig "$KC" --context "$CTX" "$@"; }
opm_k() { "$OPM_BIN" "$@" --kubeconfig "$KC" --context "$CTX"; }

kind_cmd() {
  case "$E2E_PROVIDER" in
    podman) KIND_EXPERIMENTAL_PROVIDER=podman kind "$@" ;;
    docker) kind "$@" ;;
    *) die "E2E_PROVIDER must be podman or docker, got '$E2E_PROVIDER'" ;;
  esac
}

cluster_exists() {
  kind_cmd get clusters 2>/dev/null | grep -qx "$CLUSTER"
}

# require_fixture_cluster refuses to go on unless the kubeconfig exists, its current context is
# the fixture cluster's, and that context's API server is on a loopback address.
require_fixture_cluster() {
  [ -f "$KC" ] || die "no kubeconfig at $KC: run task e2e:up first"
  local current server
  current=$(kubectl --kubeconfig "$KC" config current-context)
  [ "$current" = "$CTX" ] || die "kubeconfig current context is '$current', expected '$CTX'"
  server=$(k config view --minify -o jsonpath='{.clusters[0].cluster.server}')
  case "$server" in
    https://127.0.0.1:* | https://localhost:* | "https://[::1]:"*) ;;
    *) die "context $CTX points at '$server', not a loopback address" ;;
  esac
}

# wait_until <seconds> <description> <command...> polls the command every 3 s until it succeeds.
wait_until() {
  local timeout=$1 what=$2 deadline
  shift 2
  deadline=$((SECONDS + timeout))
  until "$@" >/dev/null 2>&1; do
    if [ "$SECONDS" -ge "$deadline" ]; then
      log "timed out after ${timeout}s waiting for $what"
      return 1
    fi
    sleep 3
  done
  log "$what"
}

# cond <resource> <namespace|-> <name> <type> <field> prints one condition field.
cond() {
  local ns=()
  [ "$2" = "-" ] || ns=(-n "$2")
  k get "$1" "${ns[@]}" "$3" -o jsonpath="{.status.conditions[?(@.type==\"$4\")].$5}"
}

cond_is() { [ "$(cond "$1" "$2" "$3" "$4" "$5")" = "$6" ]; }

# ready_at_generation <resource> <namespace|-> <name>: a Ready condition exists for the
# object's current generation, whatever its status.
ready_at_generation() {
  local gen observed ns=()
  [ "$2" = "-" ] || ns=(-n "$2")
  gen=$(k get "$1" "${ns[@]}" "$3" -o jsonpath='{.metadata.generation}')
  observed=$(cond "$1" "$2" "$3" Ready observedGeneration)
  [ -n "$observed" ] && [ "$observed" = "$gen" ]
}
