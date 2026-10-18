#!/usr/bin/env bash
# Tests scripts/port-upstream.sh against a throwaway repository that mimics the
# fork layout: `main` mirrors upstream and receives merge commits, `cmx` holds
# the fork with a removed Windows file.
set -euo pipefail

script="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/port-upstream.sh"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

failures=0
check() {
  local name="$1"; shift
  if "$@"; then
    printf 'PASS %s\n' "$name"
  else
    printf 'FAIL %s\n' "$name" >&2
    failures=$((failures + 1))
  fi
}

# Isolate from the user's git config (identity, signing, hooks).
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
git_q() { git "$@"; }

cd "$work"
git_q init -q -b main
mkdir -p scripts fork
cp "$script" scripts/port-upstream.sh
printf 'shared\n' > app.go
printf 'win\n' > app_windows.go
git_q add -A && git_q commit -q -m "base"
base="$(git rev-parse HEAD)"

# Fork branch: drop the Windows file and record it in the manifest.
git_q checkout -q -b cmx
git_q rm -q app_windows.go
printf '*_windows.go\n' > fork/removed-paths.txt
git_q add -A && git_q commit -q -m "fork: macOS only"

# Upstream PR 1: shared change plus a Windows edit (merge commit on main).
git_q checkout -q main
git_q checkout -q -b pr1
printf 'shared v2\n' > app.go
printf 'win v2\n' > app_windows.go
git_q commit -q -am "feat: shared change"
git_q checkout -q main
git_q merge -q --no-ff -m "Merge pull request #1" pr1
pr1="$(git rev-parse HEAD)"

# Upstream PR 2: Windows-only change, plus a brand new Windows file.
git_q checkout -q -b pr2
printf 'win v3\n' > app_windows.go
printf 'new\n' > extra_windows.go
git_q add -A && git_q commit -q -m "fix: windows only"
git_q checkout -q main
git_q merge -q --no-ff -m "Merge pull request #2" pr2
pr2="$(git rev-parse HEAD)"

git_q checkout -q cmx
list="$(BASE="$base" bash scripts/port-upstream.sh list)"
check "list marks a shared PR as pending" grep -q "^pending  $(git rev-parse --short "$pr1") Merge pull request #1" <<<"$list"
check "list marks a removed-only PR as skip" grep -q "^skip     $(git rev-parse --short "$pr2") Merge pull request #2" <<<"$list"

BASE="$base" bash scripts/port-upstream.sh pick "$pr1" >/dev/null 2>&1 || true
check "pick applies the shared change" grep -qx 'shared v2' app.go
check "pick keeps the removed file deleted" test ! -e app_windows.go
check "pick leaves no cherry-pick in progress" test ! -e .git/CHERRY_PICK_HEAD
check "pick records the upstream commit with -x" \
  sh -c "git log -1 --format=%B | grep -q 'cherry picked from commit $pr1'"
check "list marks a picked PR as ported" \
  sh -c "BASE=$base bash scripts/port-upstream.sh list | grep -q '^ported   $(git rev-parse --short "$pr1")'"

BASE="$base" bash scripts/port-upstream.sh pick "$pr2" >/dev/null 2>&1 || true
check "pick of a removed-only PR adds no commit" \
  sh -c "git log -1 --format=%B | grep -q 'cherry picked from commit $pr1'"
check "pick drops new removed-path files" test ! -e extra_windows.go

bash scripts/port-upstream.sh mark "$pr2" >/dev/null
check "mark creates the sync tag" \
  git show-ref -q --verify -q --verify "refs/tags/upstream-synced/$(git rev-parse --short "$pr2")"
check "list starts after the sync tag" \
  sh -c "! bash scripts/port-upstream.sh list | grep -Eq '^(pending|skip|ported)'"

if [ "$failures" -ne 0 ]; then
  printf '%d check(s) failed\n' "$failures" >&2
  exit 1
fi
printf 'ok — port-upstream checks passed\n'
