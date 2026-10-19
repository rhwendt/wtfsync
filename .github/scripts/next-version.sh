#!/usr/bin/env bash
# Prints the next release tag, worked out from the Conventional Commits since the last vX.Y.Z tag:
# a breaking change bumps major (minor while still 0.x), feat bumps minor, fix/perf bump patch.
# Prints nothing when no commit since the last tag warrants a release (docs, chore, ci, test, ...).
set -euo pipefail

last=$(git describe --tags --abbrev=0 --match 'v[0-9]*.[0-9]*.[0-9]*' 2>/dev/null || true)
if [ -z "$last" ]; then
	echo v0.1.0
	exit 0
fi

log=$(git log --no-merges --format='%s%n%b' "$last..HEAD")
IFS=. read -r major minor patch <<<"${last#v}"

if grep -qE '^[a-z]+(\([^)]*\))?!:|^BREAKING[ -]CHANGE:' <<<"$log"; then
	if [ "$major" -eq 0 ]; then
		minor=$((minor + 1)) patch=0
	else
		major=$((major + 1)) minor=0 patch=0
	fi
elif grep -qE '^feat(\([^)]*\))?:' <<<"$log"; then
	minor=$((minor + 1)) patch=0
elif grep -qE '^(fix|perf)(\([^)]*\))?:' <<<"$log"; then
	patch=$((patch + 1))
else
	exit 0
fi
echo "v$major.$minor.$patch"
