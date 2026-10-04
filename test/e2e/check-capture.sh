#!/usr/bin/env bash
# check-capture.sh [dir]: refuse a capture that holds anything the portal may not serve: a
# Secret, managedFields, the last-applied annotation, or a ModuleInstance's or ModulePackage's
# spec.values; and a CRD schema, which captures leave out for size. Reads files only; never
# contacts a cluster.
set -euo pipefail

DIR=${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/testdata/clusters/f1}
command -v yq >/dev/null || { echo "check-capture: yq not found on PATH" >&2; exit 1; }

shopt -s nullglob
files=()
for f in "$DIR"/*.yaml; do
  [ "$(basename "$f")" = meta.yaml ] || files+=("$f")
done
[ "${#files[@]}" -gt 0 ] || { echo "check-capture: no capture files in $DIR" >&2; exit 1; }

failed=0
for f in "${files[@]}"; do
  hits=$(yq '
    .items[]
    | select(
        .kind == "Secret"
        or .metadata.managedFields != null
        or .metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"] != null
        or ((.kind == "ModuleInstance" or .kind == "ModulePackage") and .spec.values != null)
        or (.kind == "CustomResourceDefinition" and ([.spec.versions[] | select(.schema != null)] | length) > 0)
      )
    | .kind + " " + (.metadata.namespace // "-") + "/" + .metadata.name
  ' "$f")
  if [ -n "$hits" ]; then
    failed=1
    while IFS= read -r hit; do
      echo "check-capture: $(basename "$f"): $hit holds a Secret, managedFields, the last-applied annotation, spec.values or a CRD schema" >&2
    done <<<"$hits"
  fi
done
[ "$failed" -eq 0 ] || exit 1
echo "check-capture: ok (${#files[@]} files in $DIR)"
