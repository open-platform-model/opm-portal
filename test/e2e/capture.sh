#!/usr/bin/env bash
# shellcheck source-path=SCRIPTDIR
# capture.sh [out-dir]: snapshot the e2e fixture cluster into testdata/clusters/f1 (or out-dir)
# for golden suites.
#
# Writes the four OPM kinds, the objects their inventories name, the ReplicaSets and Pods
# labelled with an instance name, the events of the fixture namespaces, and meta.yaml. Never
# reads a Secret. Strips managedFields, the last-applied annotation (a client-side apply copies
# every value into it) and every ModuleInstance's and ModulePackage's spec.values, so the
# capture holds only what the portal itself may serve. check-capture.sh runs last.
# shellcheck source=lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

for tool in kubectl yq jq; do
  command -v "$tool" >/dev/null || die "$tool not found on PATH"
done

OUT=${1:-$REPO_ROOT/testdata/clusters/f1}
EVENT_NAMESPACES=(default cert-manager pkg web)

require_fixture_cluster

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

# STRIP removes what the portal never serves, drops any Secret that slipped in, drops Node
# events (kubelet and host noise that differs by provider, not OPM state), drops CRD schemas (about 90% of the capture's bytes, and nothing the portal reads), and sorts the list
# so a recapture diffs by content, not by API server order.
STRIP='
  .items |= map(
    del(.metadata.managedFields)
    | del(.metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"])
    | with(select((.metadata.annotations | length) == 0); del(.metadata.annotations))
    | with(select(.kind == "ModuleInstance" or .kind == "ModulePackage"); del(.spec.values))
    | with(select(.kind == "CustomResourceDefinition"); del(.spec.versions[].schema))
  )
  | .items |= map(select(.kind != "Secret"))
  | .items |= map(select(.kind != "Event" or .regarding.kind != "Node"))
  | .items |= sort_by(.kind, .metadata.namespace // "", .metadata.name)
  | del(.metadata)
'

# as_list <json-file...> merges single objects and lists into one v1 List.
as_list() {
  jq -s '{apiVersion: "v1", kind: "List", items: [.[] | if .kind == "List" then .items[] else . end]}' "$@"
}

# write <name> strips $TMP/<name>.raw.json into $TMP/out/<name>.yaml.
write() { yq -p json -o yaml -P "$STRIP" "$TMP/$1.raw.json" >"$TMP/out/$1.yaml"; }

mkdir -p "$TMP/out" "$TMP/objects"

log "capturing the OPM kinds"
for r in platforms moduleinstances modulepackages transformerregistrations; do
  k get "$r.opmodel.dev" -A -o json >"$TMP/$r.raw.json"
  write "$r"
done

log "capturing inventory objects"
# group|version|kind|namespace|name per entry; '|' because an empty group is a field too.
k get moduleinstances.opmodel.dev -A -o jsonpath='{range .items[*]}{range .status.inventory.entries[*]}{.group}|{.v}|{.kind}|{.namespace}|{.name}{"\n"}{end}{end}' |
  sort -u >"$TMP/entries.txt"
secrets_skipped=0
missing=()
n=0
while IFS='|' read -r group version kind namespace name; do
  [ -n "$kind" ] || continue
  if [ -z "$group" ] && [ "$kind" = Secret ]; then
    secrets_skipped=$((secrets_skipped + 1))
    continue
  fi
  if [ -n "$group" ]; then res=$kind.$version.$group; else res=$kind; fi
  ns=()
  [ -z "$namespace" ] || ns=(-n "$namespace")
  n=$((n + 1))
  if ! k get "$res" "${ns[@]}" "$name" -o json >"$TMP/objects/$(printf '%04d' "$n").json" 2>"$TMP/get.err"; then
    rm -f "$TMP/objects/$(printf '%04d' "$n").json"
    missing+=("$group/$kind/$namespace/$name")
  fi
done <"$TMP/entries.txt"
k get replicasets.apps,pods -A -l module-instance.opmodel.dev/name -o json >"$TMP/objects/workloads.json"
as_list "$TMP"/objects/*.json >"$TMP/objects.raw.json"
write objects

log "capturing events in ${EVENT_NAMESPACES[*]}"
for ns in "${EVENT_NAMESPACES[@]}"; do
  k get events.events.k8s.io -n "$ns" -o json >"$TMP/events-$ns.json"
done
as_list "$TMP"/events-*.json >"$TMP/events.raw.json"
write events

log "writing meta.yaml"
k version -o json >"$TMP/version.json"
k get deployment -n opm-operator-system opm-operator-controller-manager -o json >"$TMP/operator.json"
jq -n \
  --arg capturedAt "$(date -u +%FT%TZ)" \
  --arg cluster "$CLUSTER" \
  --arg provider "$E2E_PROVIDER" \
  --arg kindVersion "$KIND_VERSION" \
  --arg nodeImage "$KIND_NODE_IMAGE" \
  --arg cli "$OPM_CLI_VERSION" \
  --argjson secretsSkipped "$secrets_skipped" \
  --argjson objects "$(yq '.items | length' "$TMP/out/objects.yaml")" \
  --argjson missing "$(printf '%s\n' "${missing[@]}" | jq -R 'select(length > 0)' | jq -s .)" \
  --slurpfile version "$TMP/version.json" \
  --slurpfile operator "$TMP/operator.json" \
  --slurpfile platforms <(yq -o json "$TMP/out/platforms.yaml") \
  --slurpfile instances <(yq -o json "$TMP/out/moduleinstances.yaml") \
  --slurpfile packages <(yq -o json "$TMP/out/modulepackages.yaml") \
  --slurpfile registrations <(yq -o json "$TMP/out/transformerregistrations.yaml") '
  def ready: (.status.conditions // [] | map(select(.type == "Ready")) | first) // {};
  {
    capturedAt: $capturedAt,
    cluster: {
      name: $cluster, provider: $provider, kind: $kindVersion, nodeImage: $nodeImage,
      kubernetes: $version[0].serverVersion.gitVersion
    },
    opmCLI: $cli,
    operator: {
      image: $operator[0].spec.template.spec.containers[0].image,
      version: ($platforms[0].items[0].status.operatorVersion // null)
    },
    catalogs: [$platforms[0].items[0].status.registry[]? | {catalog, version, source}],
    instances: [$instances[0].items[] | {
      namespace: .metadata.namespace, name: .metadata.name, owner: (.spec.owner // "operator"),
      ready: (ready.status // "absent"), reason: (ready.reason // null)
    }],
    packages: [$packages[0].items[] | {
      namespace: .metadata.namespace, name: .metadata.name,
      ready: (ready.status // "absent"), reason: (ready.reason // null)
    }],
    registrations: [$registrations[0].items[] | {
      name: .metadata.name, ready: (ready.status // "absent"), reason: (ready.reason // null),
      accepted: (.status.accepted // false), active: (.status.active // false),
      deliberateRefusal: ((.metadata.labels // {})["e2e.opmodel.dev/fixture"] == "deliberate-refusal")
    } | if .deliberateRefusal then . + {
      note: "deliberate refusal fixture (test/e2e/fixtures/f1/60-refused-claim.yaml): refused on purpose"
    } elif (.accepted and .active) then del(.deliberateRefusal) else del(.deliberateRefusal) + {
      note: "not accepted and active on this operator; the claim is accepted only by an operator built on library v1.0.0-beta.2 or later"
    } end],
    inventory: {objects: $objects, secretsSkipped: $secretsSkipped, missing: $missing}
  }' | yq -P >"$TMP/out/meta.yaml"

"$E2E_DIR/check-capture.sh" "$TMP/out"

mkdir -p "$OUT"
for f in meta platforms moduleinstances modulepackages transformerregistrations objects events; do
  cp "$TMP/out/$f.yaml" "$OUT/$f.yaml"
done
log "capture written to $OUT"
