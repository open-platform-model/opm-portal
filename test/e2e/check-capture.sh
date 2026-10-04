#!/usr/bin/env bash
# check-capture.sh [dir]: refuse a capture that holds anything the portal may not serve: a
# Secret, managedFields, the last-applied annotation, or a ModuleInstance's or ModulePackage's
# spec.values; and a CRD schema, which captures leave out for size. Reads files only; never
# contacts a cluster.
#
# Checks every file under dir (default testdata/clusters), recursively and whatever its
# extension, except Markdown. Every document in a file is walked to any depth, so an object is
# found wherever it sits: a bare object, a List's items, a List inside a List, a top-level
# sequence, or a map that also carries an items key. An object is any map with both kind and
# metadata, which leaves out Event.regarding, ownerReferences and inventory entries; a map of
# kind Secret that carries data or stringData counts as a Secret even without metadata. A file
# that does not parse as YAML (JSON included) fails the check.
set -euo pipefail

DIR=${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/testdata/clusters}
command -v yq >/dev/null || { echo "check-capture: yq not found on PATH" >&2; exit 1; }
[ -d "$DIR" ] || { echo "check-capture: no directory $DIR" >&2; exit 1; }

files=()
while IFS= read -r -d '' f; do
  files+=("$f")
done < <(find "$DIR" -type f ! -name '*.md' -print0 | sort -z)
[ "${#files[@]}" -gt 0 ] || { echo "check-capture: no capture files in $DIR" >&2; exit 1; }

err=$(mktemp)
trap 'rm -f "$err"' EXIT

failed=0
for f in "${files[@]}"; do
  rel=${f#"$DIR"/}
  if ! hits=$(yq -p yaml -o yaml '
    ..
    | select(tag == "!!map")
    | select((has("kind") and has("metadata")) or (.kind == "Secret" and (has("data") or has("stringData"))))
    | select(
        .kind == "Secret"
        or .metadata.managedFields != null
        or .metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"] != null
        or ((.kind == "ModuleInstance" or .kind == "ModulePackage") and .spec.values != null)
        or (.kind == "CustomResourceDefinition" and ([.spec.versions[]? | select(.schema != null)] | length) > 0)
      )
    | (.kind // "-") + " " + (.metadata.namespace // "-") + "/" + (.metadata.name // "-")
  ' "$f" 2>"$err"); then
    failed=1
    echo "check-capture: $rel: does not parse: $(cat "$err")" >&2
    continue
  fi
  if [ -n "$hits" ]; then
    failed=1
    while IFS= read -r hit; do
      echo "check-capture: $rel: $hit holds a Secret, managedFields, the last-applied annotation, spec.values or a CRD schema" >&2
    done <<<"$hits"
  fi
done
[ "$failed" -eq 0 ] || exit 1
echo "check-capture: ok (${#files[@]} files in $DIR)"
