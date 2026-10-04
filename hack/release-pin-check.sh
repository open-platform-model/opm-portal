#!/usr/bin/env bash
# Release-pin gate (G1, workspace RELEASING.md section "Gates"): fail when a
# release would ship an OPM pin no consumer can reproduce.
#
#   1. a Go replace directive in go.mod;
#   2. an OPM Go pin (github.com/open-platform-model/*) that is a pseudo-version
#      or is not an existing tag of its repository.
#
# Every failure is printed before the script exits non-zero. Run it from the repo
# root (task deps:release-check does). Needs go, jq, git and network access to
# github.com for the tag lookups.
set -euo pipefail

fail=0
bad() {
	printf 'release-pin-check: %s\n' "$*" >&2
	fail=1
}

# 1. No replace directives.
go mod edit -json | jq -e '.Replace == null' >/dev/null ||
	bad "go.mod has replace directives: $(go mod edit -json | jq -c '.Replace')"

# 2. OPM Go pins: no pseudo-versions, and the version is a tag of the module's repo.
#    Pins come from go.mod itself (no network, no module resolution), so an
#    unresolvable pin cannot hide behind a failed listing.
pseudo='([-.]0\.|-)[0-9]{14}-[0-9a-f]{12}$'
while read -r path ver; do
	[[ -n $path ]] || continue
	if [[ $ver =~ $pseudo ]]; then
		bad "go.mod: $path $ver is a pseudo-version"
		continue
	fi
	repo="https://$(cut -d/ -f1-3 <<<"$path")" # github.com/open-platform-model/<repo>
	git ls-remote --exit-code --tags "$repo" "refs/tags/$ver" >/dev/null ||
		bad "go.mod: $path $ver is not a tag of $repo"
done < <(go mod edit -json | jq -r '.Require[]?
	| select(.Path | startswith("github.com/open-platform-model/"))
	| "\(.Path) \(.Version)"')

if [[ $fail -eq 0 ]]; then
	echo 'release-pin-check: ok'
fi
exit "$fail"
