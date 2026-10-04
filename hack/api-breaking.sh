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

status=0
"$oasdiff" breaking "$tmp/base.yaml" "$doc" --fail-on ERR --severity-levels "$levels" || status=$?
case "$status" in
0)
	echo "api-breaking: no breaking change against ${base_ref}"
	exit 0
	;;
1) ;;
*)
	echo "api-breaking: oasdiff could not compare the documents (exit ${status})" >&2
	exit "$status"
	;;
esac

if [[ "${PR_TITLE:-}" =~ ^[a-z]+(\([^\)]*\))?!: ]]; then
	echo "api-breaking: breaking change against ${base_ref}, marked by the title"
	exit 0
fi
echo "api-breaking: breaking change against ${base_ref}; mark it with ! in the PR title, or make it additive" >&2
exit 1
