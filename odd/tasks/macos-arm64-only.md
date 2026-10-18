# macOS arm64 only fork

Objective: make the `cmx` fork support only macOS on Apple Silicon and switch upstream intake from merge to per-PR cherry-pick.
Branch: `feat/macos-arm64-only` (from `cmx`). Delivery: `ask-on-risk`. Test runner: `go build ./... && go vet ./...`, `scripts/test-fast.sh`.

## Specs

- S1: "podemos eliminar completamente el soporte de otros sistemas operativos mi fork solo de va a encargar de MacOS con Apple Silicon"
- S2: Upstream intake uses cherry-pick ("ok vamos con esa forma del cherry-pick"): `main` stays a pure fast-forward mirror of `upstream/main`; `cmx` receives upstream PRs with `git cherry-pick -x -m 1` from `main`; a tag `upstream-synced/<sha>` marks the last reviewed `main` commit.
- S3: Remove every Go file that is never compiled for `darwin/arm64` (with or without the `bench_fixture`/`real_agent_e2e` tags); darwin build, vet and tests must keep passing.
- S4: Remove non-Go Windows/Linux assets: PowerShell scripts, Windows provisioning scripts and tests, Windows/Linux CI workflows.
- S5: Release and CI target only `darwin/arm64` (`.goreleaser.yaml`, `ci.yml`, `release.yml`); `install.sh` and `install-fork.sh` fail fast outside `Darwin arm64`.
- S6: Runtime guard: the binary refuses to run on any platform other than `darwin/arm64` with a clear message.
- S7: Fork tooling: `fork/removed-paths.txt` manifest and `scripts/port-upstream.sh` that lists pending upstream PRs from `main` since the last `upstream-synced/*` tag, skips PRs touching only removed paths, and resolves modify/delete conflicts on removed paths.
- S8: `FORK.md` documents the cherry-pick flow and the macOS-only scope, replacing the merge-based release intake section.
- S9 (open decision): fork version scheme and upstream update notice. Assumption pending user answer.

## Tasks

| ID | Specs | Route | Status | Commit |
| --- | --- | --- | --- | --- |
| T1 | S3 | inline | done: build/vet green; test-fast suite green except 3 baseline failures | f2512374 |
| T2 | S4, S5 | inline | done | f2512374 |
| T3 | S6 | inline | done: RED (undefined ensureForkPlatform) then GREEN; darwin/amd64 rejected under Rosetta | 94ee1a4b |
| T4 | S2, S7, S8 | inline | done: scripts/test-port-upstream.sh 11/11 PASS (found and fixed empty-commit bug) | 602b5072 |
| T5 | S9 | inline | blocked on decision | |

Route evidence: no Writer trigger (units depend on each other's deletions; no parallel units), so the parent works inline.

## Log

- L1 (2026-10-18): "podemos eliminar completamente el soporte de otros sistemas operativos mi fork solo de va a encargar de MacOS con Apple Silicon como quedaria el plan"
- L2 (2026-10-18): "ok vamos con esa forma del cherry-pick"
- L3: Mechanical deletion set computed with `GOOS=darwin GOARCH=arm64 go list -e -f '{{.IgnoredGoFiles}}'`, intersected across default and `bench_fixture,real_agent_e2e` tags: 93 files. Files like `rar_windows_owner_trust.go` and `detect_linux_probe_test.go` do compile on darwin and stay.
- L4: Shared files with `runtime.GOOS` branches (46 production files) are kept unchanged in this feature; editing them raises cherry-pick conflicts and gives no runtime gain.
- L5: Kept generic Docker e2e harness files (`e2e/Dockerfile.{ubuntu,arch,fedora,claude-network-none}`, `e2e/docker-test.sh`) because shipped skills reference `docker-test.sh`; their CI jobs were removed. Kept `internal/assets/gga/gga.ps1` because `internal/components/gga/runtime.go` embeds it (shared file, L4).
- L6: Linux build (`GOOS=linux go build ./...`) no longer compiles after T1, expected for a darwin-only fork.
- L7: Baseline failures also present on `cmx`: `internal/update` (TestReleaseSecurityScriptsAreSyntacticallyValidAndFailClosed, TestCanonicalReleasePublicKeysControlRealLinkerBuild) and `internal/components/reviewassets` (TestOwnershipReuseRejectsLedgerWithoutMutation).
- L8: Native review assessed high (item 5); user chose "Omitir esta vez"; decline recorded as declined_this_candidate.
- Next: S9 decision (fork version scheme and update notice).
