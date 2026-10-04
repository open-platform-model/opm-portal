#!/usr/bin/env bash
# API breaking-change gate: compare the read API's OpenAPI document with the
# base branch's copy and fail on a breaking change, unless the pull request's
# title marks one with "!" (feat!: or feat(api)!:). Within v1alpha1 the API
# changes only additively; a breaking change is allowed on the 0.x line only
# as a marked one, which bumps the minor version.
#
# Usage: hack/api-breaking.sh <base-ref>
#   PR_TITLE  the pull request title (optional; unset means unmarked)
#   OASDIFF   the oasdiff binary (default: oasdiff on PATH)
#
# hack/oasdiff-levels.txt raises the removal of an optional response property
# to an error: clients may rely on a field that is sometimes absent.
#
# oasdiff ignores x-extensible-enum, the keyword every enumerated string in
# the document uses, so the script also fails on a value the base lists at
# some location and the head no longer does (yq and jq on PATH). Adding a
# value passes.
#
# A base ref without the document has nothing to compare and passes. A
# document oasdiff cannot read fails, whatever the title says. Run it from the
# repo root (task api:breaking does).
set -euo pipefail

base_ref="${1:?usage: hack/api-breaking.sh <base-ref>}"
doc=openapi/v1alpha1.yaml
levels=hack/oasdiff-levels.txt
oasdiff="${OASDIFF:-oasdiff}"

if ! git cat-file -e "${base_ref}:${doc}" 2>/dev/null; then
	echo "api-breaking: ${base_ref} holds no ${doc}; nothing to compare"
	exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
git show "${base_ref}:${doc}" >"$tmp/base.yaml"

# enum_values prints "<location> <value>" for every x-extensible-enum value.
enum_values() {
	yq -o=json . "$1" | jq -r '
		paths(type == "object" and has("x-extensible-enum")) as $p
		| getpath($p)["x-extensible-enum"][] as $v
		| ($p | map(tostring) | join("/")) + " " + $v' | LC_ALL=C sort -u
}
for side in base head; do
	src="$tmp/base.yaml"
	[[ "$side" == head ]] && src="$doc"
	if ! enum_values "$src" >"$tmp/${side}.enums"; then
		echo "api-breaking: could not read the enumerated values of ${src}" >&2
		exit 2
	fi
done
removed_enums="$(LC_ALL=C comm -23 "$tmp/base.enums" "$tmp/head.enums")"

status=0
"$oasdiff" breaking "$tmp/base.yaml" "$doc" --fail-on ERR --severity-levels "$levels" || status=$?
case "$status" in
0 | 1) ;;
*)
	echo "api-breaking: oasdiff could not compare the documents (exit ${status})" >&2
	exit "$status"
	;;
esac

if [[ -n "$removed_enums" ]]; then
	echo "api-breaking: enumerated values removed (location, value):"
	printf '%s\n' "$removed_enums"
	status=1
fi
if [[ "$status" -eq 0 ]]; then
	echo "api-breaking: no breaking change against ${base_ref}"
	exit 0
fi

if [[ "${PR_TITLE:-}" =~ ^[a-z]+(\([^\)]*\))?!: ]]; then
	echo "api-breaking: breaking change against ${base_ref}, marked by the title"
	exit 0
fi
echo "api-breaking: breaking change against ${base_ref}; mark it with ! in the PR title, or make it additive" >&2
exit 1
