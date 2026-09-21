#!/usr/bin/env bash
# Cut a release: roll [Unreleased] into VERSION, commit, tag, push.
set -euo pipefail

die() {
	echo "$1" >&2
	exit 1
}

cd "$(git rev-parse --show-toplevel)"

version="${VERSION:-}"
[ -n "$version" ] || die "VERSION is required: make tag VERSION=v1.2.3"
[[ "$version" =~ ^v(0|[1-9][0-9]*)(\.(0|[1-9][0-9]*)){2}$ ]] || die "VERSION must look like v1.2.3, got: $version"

branch="$(git rev-parse --abbrev-ref HEAD)"
[ "$branch" = "master" ] || die "releases are cut from master, not $branch"
[ -z "$(git status --porcelain)" ] || die "working tree is dirty; commit or stash first"
if git rev-parse -q --verify "refs/tags/$version" >/dev/null; then
	die "tag $version already exists locally"
fi
git fetch --quiet origin master
remote_tag="$(git ls-remote --tags origin "refs/tags/$version")"
[ -z "$remote_tag" ] || die "tag $version already exists on origin"
head="$(git rev-parse HEAD)"
origin_head="$(git rev-parse FETCH_HEAD)"
if [ "$head" != "$origin_head" ]; then
	if git merge-base --is-ancestor "$head" "$origin_head"; then
		die "local master is behind origin/master; pull first"
	elif git merge-base --is-ancestor "$origin_head" "$head"; then
		die "local master is ahead of origin/master; push it first"
	fi
	die "local master and origin/master have diverged; reconcile them first"
fi

awk '/^## \[Unreleased\]/ { seen = 1; next }
	seen && /^## / { exit }
	seen && NF { found = 1; exit }
	END { exit !(seen && found) }' CHANGELOG.md ||
	die "CHANGELOG.md has nothing under [Unreleased] to release as $version"

number="${version#v}"
today="$(date +%Y-%m-%d)"
link="$(grep '^\[Unreleased\]: ' CHANGELOG.md)" ||
	die "CHANGELOG.md has no [Unreleased] compare link"
prev="$(sed -n 's#^\[Unreleased\]: .*/compare/\(v[0-9.]*\)\.\.\.HEAD$#\1#p' <<<"$link")"
[ -n "$prev" ] || die "cannot read the previous tag from: $link"
unreleased_link="${link/$prev...HEAD/$version...HEAD}"
version_link="[$number]: ${link#\[Unreleased\]: }"
version_link="${version_link/$prev...HEAD/$prev...$version}"
awk -v heading="## [$number] - $today" -v link="$link" \
	-v unreleased_link="$unreleased_link" -v version_link="$version_link" '
	!rolled && $0 == "## [Unreleased]" { print; print ""; print heading; rolled = 1; next }
	$0 == link { print unreleased_link; print version_link; next }
	{ print }' CHANGELOG.md >CHANGELOG.md.tmp
mv CHANGELOG.md.tmp CHANGELOG.md

git add CHANGELOG.md
git commit -m "Release $version"
git tag "$version"
git push origin master "$version"
