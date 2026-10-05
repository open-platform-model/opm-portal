#!/usr/bin/env bash
# shellcheck source-path=SCRIPTDIR
# dump.sh: write what a failed e2e run needs to be read afterwards into a directory (first
# argument, default .e2e/dump/<cluster>): the OPM objects, the Pods, the events and the
# operator's log of the fixture cluster, and the log of the portal Pod e2e:pod deploys (with its
# launch token blanked). Never reads Secrets; strips spec.values, managed
# fields and the last-applied annotation as the capture does. Best effort: a read that fails is
# noted in errors.txt and the dump goes on.
# shellcheck source=lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

require_fixture_cluster
out=${1:-$STATE_DIR/dump/$CLUSTER}
mkdir -p "$out"
STRIP='
  .items |= map(
    del(.metadata.managedFields)
    | del(.metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"])
    | with(select(.kind == "ModuleInstance" or .kind == "ModulePackage"); del(.spec.values))
  )
  | .items |= map(select(.kind != "Secret"))
'

# save <file> <command...> runs the command into the file, noting a failure.
save() {
  local file=$1
  shift
  "$@" >"$out/$file" 2>>"$out/errors.txt" || echo "failed: $*" >>"$out/errors.txt"
}

# portal_log is the opm-portal Pod's log with the launch token blanked, so an uploaded dump holds
# no token; it fails, and is noted, when e2e:pod never deployed the portal.
portal_log() { k logs -n opm-portal deployment/opm-portal --tail=2000 | sed -E 's/token=[^[:space:]]+/token=REDACTED/g'; }

opm_yaml() { k get platforms,moduleinstances,modulepackages,transformerregistrations -A -o yaml | yq "$STRIP"; }

log "dumping $CLUSTER into $out"
save opm.yaml opm_yaml
save pods.txt k get pods -A -o wide
save events.txt k get events -A --sort-by=.lastTimestamp
save operator.log k logs -n opm-operator-system deployment/opm-operator-controller-manager --tail=2000
save portal.log portal_log
log "dumped into $out"
