#!/bin/sh
# Fast test lane for the fork: Claude Code packages plus the Claude and sync CLI
# tests. It does not replace `go test ./...`; run that after rebasing on upstream.
set -eu

cd "$(dirname "$0")/.."

# The module targets Go 1.25.10; newer toolchains change some error texts tests assert.
GOTOOLCHAIN="${GOTOOLCHAIN:-go1.25.10}"
export GOTOOLCHAIN

go test "$@" \
	./internal/agents/claude \
	./internal/assets \
	./internal/components/agentguidance \
	./internal/components/agenthooks \
	./internal/components/filemerge \
	./internal/components/persona \
	./internal/components/reviewassets \
	./internal/components/skills \
	./internal/components/telemetryruntime \
	./internal/model \
	./internal/state \
	./internal/telemetry \
	./internal/tui/...

go test "$@" -run 'Claude|Sync' ./internal/cli
