#!/usr/bin/env bash
# Builds gentle-ai from this fork's working tree and installs it into $GOBIN
# (or $GOPATH/bin). The version is "<latest upstream tag>-cmx.<commit>" so
# plugins that refuse "dev" builds still accept it.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

base="$(git describe --tags --abbrev=0 | sed 's/^v//')"
commit="$(git rev-parse --short HEAD)"
dirty=""
if [ -n "$(git status --porcelain)" ]; then
  dirty=".dirty"
fi
version="${base}-cmx.${commit}${dirty}"

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
