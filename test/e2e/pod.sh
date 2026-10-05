#!/usr/bin/env bash
# shellcheck source-path=SCRIPTDIR
# pod.sh: build the portal image, load it into the fixture cluster, apply deploy/ through the
# test/e2e/pod overlay, wait for the rollout, and run TestPod, which reaches the Pod through
# kubectl port-forward. Needs the cluster task e2e:up created (E2E_CLUSTER as for e2e:up).
# shellcheck source=lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

IMAGE=localhost/opm-portal:e2e

require_fixture_cluster
cd "$REPO_ROOT" || die "cannot enter $REPO_ROOT"

case "$E2E_PROVIDER" in
  podman | docker) ;;
  *) die "E2E_PROVIDER must be podman or docker, got '$E2E_PROVIDER'" ;;
esac

log "building $IMAGE with $E2E_PROVIDER"
"$E2E_PROVIDER" build -t "$IMAGE" -f Dockerfile .

archive=$(mktemp -d)/opm-portal.tar
trap 'rm -rf "$(dirname "$archive")"' EXIT
"$E2E_PROVIDER" save -o "$archive" "$IMAGE"
log "loading $IMAGE into $CLUSTER"
kind_cmd load image-archive "$archive" --name "$CLUSTER"

log "applying deploy/ through test/e2e/pod"
k apply -k test/e2e/pod
# A fresh Pod each run, so its log holds an unspent launch link and the image just loaded.
k -n opm-portal rollout restart deploy/opm-portal
k -n opm-portal rollout status deploy/opm-portal --timeout=180s
# The old Pod may still be terminating; the test reads the log of, and forwards to, the one left.
one_pod() { [ "$(k -n opm-portal get pods -l app.kubernetes.io/name=opm-portal -o name | wc -l)" -eq 1 ]; }
wait_until 120 "one portal Pod left" one_pod

log "running the Pod test against $CLUSTER"
OPM_PORTAL_E2E_KUBECONFIG=$KC OPM_PORTAL_E2E_CONTEXT=$CTX \
  go test -tags e2e -count=1 -timeout 10m -run '^TestPod$' -v ./cmd/opm-portal
