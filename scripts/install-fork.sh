#!/usr/bin/env bash
# Builds gentle-ai from this fork's working tree and installs it into $GOBIN
# (or $GOPATH/bin). The version is "<upstream base>-cmx.<fork version>", for
# example 4.2.0-cmx.1.0.0: the fork version lives in fork/VERSION, and the
# upstream base keeps it valid semver >= 2.0.0, which the OpenCode review
# plugin requires. Builds with uncommitted changes end in ".dirty".
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if [ "$(uname -s)" != "Darwin" ] || [ "$(uname -m)" != "arm64" ]; then
  echo "install-fork.sh: only macOS on Apple Silicon (Darwin arm64) is supported" >&2
  exit 1
fi

base="$(git describe --tags --abbrev=0 --match 'v[0-9]*' | sed 's/^v//')"
fork_version="$(tr -d '[:space:]' < fork/VERSION)"
if ! [[ "$fork_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "install-fork.sh: fork/VERSION must be MAJOR.MINOR.PATCH, got '${fork_version}'" >&2
  exit 1
fi
dirty=""
if [ -n "$(git status --porcelain)" ]; then
  dirty=".dirty"
fi
version="${base}-cmx.${fork_version}${dirty}"

bin_dir="$(go env GOBIN)"
if [ -z "$bin_dir" ]; then
  bin_dir="$(go env GOPATH)/bin"
fi
mkdir -p "$bin_dir"

echo "Building gentle-ai ${version} -> ${bin_dir}/gentle-ai"
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=${version}" \
  -o "${bin_dir}/gentle-ai" ./cmd/gentle-ai

"${bin_dir}/gentle-ai" version 2>/dev/null || "${bin_dir}/gentle-ai" --version
