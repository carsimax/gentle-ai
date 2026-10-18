#!/usr/bin/env bash
# Ports upstream changes into this fork with cherry-pick instead of merge.
#
# `main` is a pure fast-forward mirror of upstream/main. Upstream pull requests
# land on `main` as merge commits; this script lists the ones not yet ported to
# the current branch (normally `cmx`) and cherry-picks them with `-x`, dropping
# any path listed in fork/removed-paths.txt.
#
# Usage:
#   scripts/port-upstream.sh list            # pending main commits since the last sync tag
#   scripts/port-upstream.sh pick <sha>...   # cherry-pick -x (with -m 1 for merges), in order
#   scripts/port-upstream.sh mark <sha>      # tag a reviewed main commit as upstream-synced/<short-sha>
#
# Environment:
#   MIRROR=ref   mirror branch to read from (default: main)
#   BASE=ref     review start when no upstream-synced/* tag exists (default: 4acaf424)
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

mirror="${MIRROR:-main}"
manifest="fork/removed-paths.txt"
tag_prefix="upstream-synced/"

die() { printf 'port-upstream: %s\n' "$*" >&2; exit 1; }

# Prints the manifest entries, without comments or blank lines.
manifest_entries() {
  grep -v -e '^#' -e '^[[:space:]]*$' "$manifest"
}

# Succeeds when the path matches an exact entry or a glob in the manifest.
is_removed() {
  local path="$1" entry
  while IFS= read -r entry; do
    case "$entry" in
      *'*'*) [[ "$path" == $entry ]] && return 0 ;;
      *) [ "$path" = "$entry" ] && return 0 ;;
    esac
  done < <(manifest_entries)
  return 1
}

# Latest upstream-synced tag reachable from the mirror, or BASE.
review_base() {
  local tag
  tag="$(git tag --list "${tag_prefix}*" --merged "$mirror" --sort=-creatordate | head -n 1)"
  if [ -n "$tag" ]; then
    printf '%s\n' "$tag"
  else
    printf '%s\n' "${BASE:-4acaf424}"
  fi
}

is_merge() {
  [ "$(git rev-list --parents -n 1 "$1" | wc -w)" -gt 2 ]
}

# Files the commit changes relative to its first parent.
changed_files() {
  git diff --name-only "$1^1" "$1"
}

already_ported() {
  local full
  full="$(git rev-parse "$1")"
  git log --format=%B "${mirror}..HEAD" | grep -q "cherry picked from commit ${full}"
}

cmd_list() {
  local base sha subject status file all_removed
  base="$(review_base)"
  printf 'Reviewing %s..%s\n' "$base" "$mirror"
  git rev-list --first-parent --reverse "${base}..${mirror}" | while IFS= read -r sha; do
    subject="$(git log -1 --format=%s "$sha")"
    if already_ported "$sha"; then
      status="ported"
    else
      all_removed=1
      while IFS= read -r file; do
        if ! is_removed "$file"; then
          all_removed=0
          break
        fi
      done < <(changed_files "$sha")
      if [ "$all_removed" -eq 1 ]; then
        status="skip"
      else
        status="pending"
      fi
    fi
    printf '%-8s %s %s\n' "$status" "$(git rev-parse --short "$sha")" "$subject"
  done
}

# Removes manifest paths from the index: conflicted ones and, with a
# revision argument, the ones added since that revision.
drop_removed_paths() {
  local file
  while IFS= read -r file; do
    if is_removed "$file"; then
      git rm -q -f --ignore-unmatch -- "$file"
      printf '  dropped removed path: %s\n' "$file"
    fi
  done < <(git diff --name-only --diff-filter=U; git diff --cached --name-only --diff-filter=A "${1:-HEAD}")
}

cmd_pick() {
  [ "$#" -gt 0 ] || die "pick needs at least one commit"
  local sha
  local -a args
  for sha in "$@"; do
    git rev-parse --verify -q "${sha}^{commit}" >/dev/null || die "unknown commit: $sha"
    args=(-x)
    if is_merge "$sha"; then
      args+=(-m 1)
    fi
    printf 'Picking %s %s\n' "$(git rev-parse --short "$sha")" "$(git log -1 --format=%s "$sha")"
    if git cherry-pick "${args[@]}" "$sha"; then
      # Committed cleanly: drop removed paths the pick added back.
      drop_removed_paths HEAD~1
      if git diff --cached --quiet HEAD~1; then
        git reset -q --soft HEAD~1
        printf '  skipped: only removed paths changed\n'
      elif ! git diff --cached --quiet; then
        git commit -q --amend --no-edit
      fi
      continue
    fi
    drop_removed_paths
    if [ -n "$(git diff --name-only --diff-filter=U)" ]; then
      die "conflicts remain; resolve them, then run: git cherry-pick --continue"
    fi
    if git diff --cached --quiet; then
      git cherry-pick --skip
      printf '  skipped: only removed paths changed\n'
      continue
    fi
    GIT_EDITOR=true git cherry-pick --continue
  done
}

cmd_mark() {
  [ "$#" -eq 1 ] || die "mark needs exactly one commit"
  git merge-base --is-ancestor "$1" "$mirror" || die "$1 is not on $mirror"
  local tag="${tag_prefix}$(git rev-parse --short "$1")"
  git tag "$tag" "$1"
  printf 'Tagged %s\n' "$tag"
}

[ -f "$manifest" ] || die "missing $manifest"
case "${1:-}" in
  list) shift; cmd_list "$@" ;;
  pick) shift; cmd_pick "$@" ;;
  mark) shift; cmd_mark "$@" ;;
  *) sed -n '2,17p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 2 ;;
esac
