#!/usr/bin/env bash
# Fast local test runner for this fork. Runs packages concurrently and splits
# the slowest packages into parallel shards (separate `go test -run` processes),
# so no single test binary approaches the 10m default timeout. Each shard keeps
# its own `go test` cache entry, so unchanged shards are skipped on re-runs.
#
# Usage:
#   scripts/test-fast.sh                  # every package
#   scripts/test-fast.sh --changed        # packages changed since the merge-base
#                                         # with cmx (incl. uncommitted), plus importers
#   scripts/test-fast.sh ./internal/cli   # explicit package patterns
#
# Environment:
#   JOBS=N     concurrent go test processes (default: logical CPU count)
#   COUNT=1    bypass the test cache (passes -count=1)
#   BASE=ref   base ref for --changed (default: cmx)
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

# Shard counts come from measured sequential runtimes (M-series, 10 cores):
# internal/cli ~1000s, reviewtransaction ~290s, legacyassets ~180s (one 138s
# test), organicruntime ~80s, app ~70s. Everything else finishes under ~25s.
# internal/cli uses more shards than cores so a package-only run packs well.
shards_for() {
  case "$1" in
    */internal/cli) echo 20 ;;
    */internal/reviewtransaction) echo 4 ;;
    */internal/app | */e2e/organicruntime | */internal/components/legacyassets) echo 2 ;;
    *) echo 1 ;;
  esac
}

changed_packages() {
  local base merge_base module
  base="${BASE:-cmx}"
  merge_base="$(git merge-base HEAD "$base")"
  module="$(go list -m)"

  # Map each changed file to the nearest directory that holds Go files, so
  # edits to embedded assets still select the package that embeds them.
  local dirs
  dirs="$(
    { git diff --name-only "$merge_base"; git ls-files --others --exclude-standard; } |
      sort -u |
      while IFS= read -r file; do
        local dir
        dir="$(dirname "$file")"
        while [ "$dir" != "." ] && ! ls "$dir"/*.go >/dev/null 2>&1; do
          dir="$(dirname "$dir")"
        done
        [ "$dir" != "." ] && echo "$module/$dir"
      done | sort -u
  )"
  [ -z "$dirs" ] && return 0

  # Select changed packages plus every package that imports them, directly,
  # transitively, or from its tests.
  go list -f '{{.ImportPath}}{{range .Deps}} {{.}}{{end}}{{range .TestImports}} {{.}}{{end}}{{range .XTestImports}} {{.}}{{end}}' ./... |
    CHANGED="$dirs" awk '
      BEGIN { n = split(ENVIRON["CHANGED"], list, "\n"); for (i = 1; i <= n; i++) want[list[i]] = 1 }
      { for (i = 1; i <= NF; i++) if ($i in want) { print $1; next } }
    '
}

patterns=()
changed=0
for arg in "$@"; do
  case "$arg" in
    --changed) changed=1 ;;
    -h | --help) sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) patterns+=("$arg") ;;
  esac
done

if [ "$changed" -eq 1 ] && [ "${#patterns[@]}" -gt 0 ]; then
  echo "--changed cannot be combined with package patterns: ${patterns[*]}" >&2
  exit 2
fi

if [ "$changed" -eq 1 ]; then
  packages="$(changed_packages)"
  if [ -z "$packages" ]; then
    echo "No changed Go packages since merge-base with ${BASE:-cmx}."
    exit 0
  fi
else
  [ "${#patterns[@]}" -eq 0 ] && patterns=(./...)
  packages="$(go list "${patterns[@]}")"
fi

jobs="${JOBS:-$(sysctl -n hw.ncpu 2>/dev/null || nproc)}"
work="$(mktemp -d "${TMPDIR:-/tmp}/gentle-ai-test-fast.XXXXXX")"

# Build the job list: sharded packages first (longest work starts earliest).
job=0
add_job() {
  printf '%s\n%s\n' "$1" "$2" >"$work/job.$job"
  job=$((job + 1))
}
for pkg in $packages; do
  n="$(shards_for "$pkg")"
  [ "$n" -le 1 ] && continue
  # A package whose tests do not build cannot be listed; run it unsharded so
  # go test reports the build failure instead of silently dropping it.
  if ! names="$(go test -list . "$pkg" 2>/dev/null | grep '^Test')"; then
    add_job "$pkg" ""
    continue
  fi
  for i in $(seq 0 $((n - 1))); do
    re="$(echo "$names" | awk -v n="$n" -v i="$i" 'NR % n == i' | paste -sd'|' -)"
    [ -n "$re" ] && add_job "$pkg" "^($re)\$"
  done
done
for pkg in $packages; do
  [ "$(shards_for "$pkg")" -le 1 ] && add_job "$pkg" ""
done

run_job() {
  local file="$1" pkg re log
  pkg="$(sed -n 1p "$file")"
  re="$(sed -n 2p "$file")"
  log="$file.log"
  local args=(-timeout 30m)
  [ "${COUNT:-}" = "1" ] && args+=(-count=1)
  [ -n "$re" ] && args+=(-run "$re")
  if go test "${args[@]}" "$pkg" >"$log" 2>&1; then
    echo "ok   $pkg"
  else
    echo "FAIL $pkg"
    touch "$file.failed"
  fi
}
export -f run_job

echo "Running $job jobs ($(echo "$packages" | wc -l | tr -d ' ') packages) with JOBS=$jobs..."
start="$(date +%s)"
ls "$work"/job.* | sort -t. -k2 -n | xargs -P "$jobs" -I{} bash -c 'run_job "$1"' _ {} |
  grep -v '^ok ' || true
elapsed=$(($(date +%s) - start))

cached="$(cat "$work"/*.log | grep -c '^ok .*(cached)$' || true)"
echo "Slowest jobs ($cached/$job served from the test cache):"
cat "$work"/*.log | awk '$1 == "ok" && $NF != "(cached)" { print $NF, $2 }' |
  sort -rn | head -5 | sed 's|github.com/[^ ]*/v4/||; s/^/  /'

failed="$(ls "$work"/*.failed 2>/dev/null || true)"
if [ -z "$failed" ]; then
  echo "All $job jobs passed in ${elapsed}s."
  rm -rf "$work"
  exit 0
fi

echo
echo "Failures (${elapsed}s):"
for marker in $failed; do
  log="${marker%.failed}.log"
  echo "== $(sed -n 1p "${marker%.failed}")"
  grep -E '^(--- FAIL|panic:|FAIL|.*\.go:[0-9]+:)' "$log" | head -20
done
echo
echo "Full logs: $work"
exit 1
