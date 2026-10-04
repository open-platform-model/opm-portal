#!/usr/bin/env bash
# shellcheck source-path=SCRIPTDIR
# up.sh: create the throwaway kind cluster opm-portal-e2e, install the released opm-operator
# with the pinned, checksum-verified opm CLI, apply the fixture set F1 and wait for it to settle.
#
#   E2E_PROVIDER=podman (default) | docker; recorded in .e2e/provider for capture and down
#   OPM_OPERATOR_VERSION=<release tag>  install that operator instead of the versions.env pin
# shellcheck source=lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

for tool in kind kubectl curl tar; do
  command -v "$tool" >/dev/null || die "$tool not found on PATH"
done

kind_have=$(kind version | awk '{print $2}')
[ "$kind_have" = "$KIND_VERSION" ] ||
  die "kind $kind_have is on PATH, versions.env pins $KIND_VERSION: go install sigs.k8s.io/kind@$KIND_VERSION"

# fetch_cli downloads the pinned opm release archive once, keeps it in .e2e/bin, and checks its
# sha256 against versions.env before every extraction.
fetch_cli() {
  local os arch pin want archive cached got
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *) die "unsupported architecture $(uname -m)" ;;
  esac
  pin=OPM_CLI_SHA256_${os}_${arch}
  want=${!pin:-}
  [ -n "$want" ] || die "no $pin pin in versions.env"
  archive=opm-$os-$arch.tar.gz
  cached=$STATE_DIR/bin/opm-$OPM_CLI_VERSION-$os-$arch.tar.gz
  mkdir -p "$STATE_DIR/bin"
  fetch_tmp=$(mktemp -d "$STATE_DIR/dl.XXXXXX")
  trap 'rm -rf "$fetch_tmp"' EXIT
  if [ ! -f "$cached" ]; then
    log "downloading opm $OPM_CLI_VERSION ($archive)"
    curl -fsSL -o "$fetch_tmp/$archive" \
      "https://github.com/open-platform-model/cli/releases/download/$OPM_CLI_VERSION/$archive"
    mv "$fetch_tmp/$archive" "$cached"
  fi
  # The kept archive is hashed on every run and the binary re-extracted from it, so neither a
  # replaced download nor a binary swapped in .e2e/bin is ever run.
  got=$(sha256sum "$cached" | cut -d' ' -f1)
  if [ "$got" != "$want" ]; then
    rm -f "$cached"
    die "sha256 mismatch for $archive: got $got, pinned $want"
  fi
  tar -xzf "$cached" -C "$fetch_tmp" opm
  mv -f "$fetch_tmp/opm" "$OPM_BIN"
  log "opm $OPM_CLI_VERSION verified ($got)"
}

create_cluster() {
  cluster_exists && die "kind cluster $CLUSTER already exists: run task e2e:down first"
  log "creating kind cluster $CLUSTER ($E2E_PROVIDER, $KIND_NODE_IMAGE)"
  local args=(create cluster --name "$CLUSTER" --image "$KIND_NODE_IMAGE" --kubeconfig "$KC" --wait 180s)
  if [ "$E2E_PROVIDER" = podman ]; then
    # Rootless podman needs a delegated cgroup scope for the node container.
    KIND_EXPERIMENTAL_PROVIDER=podman systemd-run --scope --user kind "${args[@]}"
  else
    kind_cmd "${args[@]}"
  fi
  chmod 600 "$KC"
  echo "$E2E_PROVIDER" >"$PROVIDER_FILE"
}

# install_operator installs the operator without the CLI's Platform, which would subscribe to the
# newest catalog release, and applies one pinned to OPM_CATALOG_VERSION instead.
install_operator() {
  local args=(operator install --skip-platform --timeout 5m)
  [ -n "$OPM_OPERATOR_VERSION" ] && args+=(--version "$OPM_OPERATOR_VERSION")
  opm_k "${args[@]}"
  log "applying the cluster Platform pinned to $OPM_CATALOG $OPM_CATALOG_VERSION"
  k apply -f - <<EOF
apiVersion: opmodel.dev/v1alpha1
kind: Platform
metadata:
  name: cluster
spec:
  type: kubernetes
  registry:
    "$OPM_CATALOG":
      enable: true
      version: "$OPM_CATALOG_VERSION"
EOF
  wait_until 120 "Platform cluster Ready=True" cond_is platforms.opmodel.dev - cluster Ready status True
}

show_conditions() {
  k get "$@" -o jsonpath='{range .status.conditions[*]}{.type}={.status} {.reason}: {.message}{"\n"}{end}' >&2 || true
}

# require_ready <namespace> <name> <seconds>: the ModuleInstance must reach Ready=True.
require_ready() {
  wait_until "$3" "ModuleInstance $1/$2 Ready=True" \
    cond_is moduleinstances.opmodel.dev "$1" "$2" Ready status True || {
    show_conditions moduleinstances.opmodel.dev -n "$1" "$2"
    die "ModuleInstance $1/$2 did not become ready"
  }
}

apply_fixtures() {
  log "applying cert-manager, podinfo and the ModulePackage"
  k apply -f "$FIXTURES/10-cert-manager.yaml" -f "$FIXTURES/20-podinfo.yaml" \
    -f "$FIXTURES/30-modulepackage-noflux.yaml"

  require_ready cert-manager cert-manager 600
  wait_until 300 "cert-manager Deployments Available" \
    k wait --for=condition=Available deployment --all -n cert-manager --timeout=10s ||
    die "cert-manager Deployments did not become available"
  require_ready default podinfo 300
  wait_until 300 "podinfo Deployment Available" \
    k wait --for=condition=Available deployment -l module-instance.opmodel.dev/name=podinfo \
    -n default --timeout=10s || die "podinfo Deployment did not become available"
  wait_until 180 "ModulePackage pkg/podinfo Ready reason SourceNotReady" \
    cond_is modulepackages.opmodel.dev pkg podinfo Ready reason SourceNotReady ||
    log "warning: ModulePackage pkg/podinfo did not reach SourceNotReady; recorded as is"

  log "applying the backup provider"
  k apply -f "$FIXTURES/40-backup-provider.yaml"
  require_ready default backup-provider 300
  # The claim's verdict depends on the operator's library; record it, do not judge it.
  wait_until 180 "claim default.backup-provider has a verdict" \
    ready_at_generation transformerregistrations.opmodel.dev - default.backup-provider ||
    log "warning: claim default.backup-provider has no Ready condition; recorded as is"

  log "applying the backup consumer"
  k apply -f "$FIXTURES/50-backup-consumer.yaml"
  wait_until 300 "ModuleInstance default/backup-consumer has a verdict" \
    ready_at_generation moduleinstances.opmodel.dev default backup-consumer ||
    log "warning: ModuleInstance default/backup-consumer has no Ready condition; recorded as is"

  log "applying the CLI-owned web_app instance"
  (cd "$FIXTURES/web" && opm_k instance apply instance.cue --create-namespace --wait --timeout 5m)
}

summary() {
  log "verdicts"
  k get moduleinstances.opmodel.dev -A -o jsonpath='{range .items[*]}ModuleInstance {.metadata.namespace}/{.metadata.name}: Ready={.status.conditions[?(@.type=="Ready")].status} {.status.conditions[?(@.type=="Ready")].reason}{"\n"}{end}' >&2
  k get modulepackages.opmodel.dev -A -o jsonpath='{range .items[*]}ModulePackage {.metadata.namespace}/{.metadata.name}: Ready={.status.conditions[?(@.type=="Ready")].status} {.status.conditions[?(@.type=="Ready")].reason}{"\n"}{end}' >&2
  k get transformerregistrations.opmodel.dev -o jsonpath='{range .items[*]}TransformerRegistration {.metadata.name}: Ready={.status.conditions[?(@.type=="Ready")].status} {.status.conditions[?(@.type=="Ready")].reason} accepted={.status.accepted} active={.status.active}{"\n"}{end}' >&2
  local accepted active
  accepted=$(k get transformerregistrations.opmodel.dev default.backup-provider -o jsonpath='{.status.accepted}' 2>/dev/null || true)
  active=$(k get transformerregistrations.opmodel.dev default.backup-provider -o jsonpath='{.status.active}' 2>/dev/null || true)
  if [ "$accepted" != true ] || [ "$active" != true ]; then
    log "note: the backup claim is not accepted and active; an operator built on library v1.0.0-beta.2 or later is needed for that"
  fi
}

mkdir -p "$STATE_DIR"
fetch_cli
"$OPM_BIN" version >&2
create_cluster
require_fixture_cluster
install_operator
apply_fixtures
summary
log "ready: kubectl --kubeconfig $KC --context $CTX get moduleinstances -A"
