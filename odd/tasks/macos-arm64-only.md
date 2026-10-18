# macOS arm64 only fork

Objective: make the `cmx` fork support only macOS on Apple Silicon and switch upstream intake from merge to per-PR cherry-pick.
Branch: `feat/macos-arm64-only` (from `cmx`). Delivery: `ask-on-risk`. Test runner: `go build ./... && go vet ./...`, `scripts/test-fast.sh`.

## Specs

- S1: "podemos eliminar completamente el soporte de otros sistemas operativos mi fork solo de va a encargar de MacOS con Apple Silicon"
- S2: Upstream intake uses cherry-pick ("ok vamos con esa forma del cherry-pick"): `main` stays a pure fast-forward mirror of `upstream/main`; `cmx` receives upstream PRs with `git cherry-pick -x -m 1` from `main`; a tag `upstream-synced/<sha>` marks the last reviewed `main` commit.
- S3: Absolute removal ("Si este cambio tiene que ser absoluto, SOLO Mac Apple Silicon"). Remove every Go file that is never compiled for `darwin/arm64` (with or without the `bench_fixture`/`real_agent_e2e` tags); darwin build, vet and tests must keep passing.
- S4: Remove non-Go Windows/Linux assets: PowerShell scripts, Windows provisioning scripts and tests, Windows/Linux CI workflows, Docker/Linux e2e harness and Linux-only e2e tests, and their mentions in docs and skills.
- S10: Remove Windows/Linux/Intel code paths from shared production code (`runtime.GOOS` branches, Linux distro detection, non-macOS package managers, PowerShell helpers, embedded Windows assets) and the tests that only exercise them; darwin behavior must stay identical.
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
| T6 | S4 | inline | done: assets tests green | 10e8131c |
| T7a | S10 | delegated (parallel) | done: engram | 3acb687a |
| T7b | S10 | delegated (parallel) | done: reviewtransaction + reviewerprovider + cli/review_* | d9bf394b |
| T7c | S10 | delegated (parallel) | done: update + update/upgrade | fadf8fe4 |
| T7d | S10 | delegated (parallel) | done: opencode + uninstall (exported signatures kept) | b236be15 |
| T7e | S10 | delegated (parallel) | done: leaf packages + agents | 145c6812 |
| T7f | S10 | delegated (after wave A) | done: system, installcmd, planner, gga, cli hub, exported-signature cleanup | d4c0b9b0 |
| T7g | S4, S5, S10 | inline | done: release scripts darwin-only, Linux bootstrap case removed, 75 OS build tags stripped, full suite green except 3 baseline failures | 228dd82b + this commit |

Route evidence: no Writer trigger (units depend on each other's deletions; no parallel units), so the parent works inline.

## Log

- L1 (2026-10-18): "podemos eliminar completamente el soporte de otros sistemas operativos mi fork solo de va a encargar de MacOS con Apple Silicon como quedaria el plan"
- L2 (2026-10-18): "ok vamos con esa forma del cherry-pick"
- L3: Mechanical deletion set computed with `GOOS=darwin GOARCH=arm64 go list -e -f '{{.IgnoredGoFiles}}'`, intersected across default and `bench_fixture,real_agent_e2e` tags: 93 files. Files like `rar_windows_owner_trust.go` and `detect_linux_probe_test.go` do compile on darwin and stay.
- L4: (superseded by L9) Shared files with `runtime.GOOS` branches were initially kept to reduce cherry-pick conflicts.
- L5: Kept generic Docker e2e harness files (`e2e/Dockerfile.{ubuntu,arch,fedora,claude-network-none}`, `e2e/docker-test.sh`) because shipped skills reference `docker-test.sh`; their CI jobs were removed. Kept `internal/assets/gga/gga.ps1` because `internal/components/gga/runtime.go` embeds it (shared file, L4).
- L6: Linux build (`GOOS=linux go build ./...`) no longer compiles after T1, expected for a darwin-only fork.
- L7: Baseline failures also present on `cmx`: `internal/update` (TestReleaseSecurityScriptsAreSyntacticallyValidAndFailClosed, TestCanonicalReleasePublicKeysControlRealLinkerBuild) and `internal/components/reviewassets` (TestOwnershipReuseRejectsLedgerWithoutMutation).
- L8: Native review assessed high (item 5); user chose "Omitir esta vez"; decline recorded as declined_this_candidate.
- L9 (2026-10-18): "contemplaste los e2e tambien?" then "Si este cambio tiene que ser absoluto, SOLO Mac Apple Silicon". Removal becomes absolute: Docker/Linux e2e, Linux-only mjs tests, gga.ps1, OS branches in shared code, and doc/skill mentions. Accepts higher cherry-pick conflict cost.
- L10: Explorer map: ~2.5k removable production lines across ~15 packages plus ~100 coupled tests; `bench/` is a separate Go module and its `lock_other.go` was missed by L3 (fixed in 8ab02a57). Route evidence for T7a-e: Writer trigger (a), five independent units with disjoint edit surfaces, each several files; wave A keeps exported APIs used outside the unit so the tree keeps compiling; T7f owns the cli hub and shared PlatformProfile.
- L11: Decisions for visible darwin output: strings naming Windows/Linux support now say macOS on Apple Silicon only; detection keeps macOS-relevant tools (brew, nix) and drops Linux/Windows package managers; keep `/usr/local/bin` brew fallback; keep OpenCode XDG_CONFIG_HOME handling; keep Pi `windowsDrivePath` input validation (repository/user input, not host OS); release provenance lists only darwin/arm64. Assumption: "absolute" overrides the Claude-Code-only memory for OS branches in other agents' adapters.
- L12: Another session (gentle-ai-10) removed the bench module and gentle-ai-bench skill on this branch (24ee38cd); user decided it commits its own work. Its TestEmbeddedAssetCount regression (16 vs 15 skill dirs) is that session's to fix.
- L13: Wave A committed: engram 3acb687a, review d9bf394b, update fadf8fe4, opencode b236be15, leaves 145c6812; integration build/vet green; package tests green except baseline update failures.
- L14: T7f kept the non-Homebrew engram download branch (macOS without brew, also used by upgrade) and rebaselined the Linux-only orchestrator module tests on darwin (only install paths changed, content bytes identical). Ratchet baseline lost 2 STALE entries.
- L15: Full suite (`COUNT=1 scripts/test-fast.sh`) green except baseline failures L7. Remaining "Windows"/"Linux" text in production Go is rationale comments, recoverytrace history rows, and input validation kept on purpose (pi windowsDrivePath, backup `..\..` traversal, risk `.ps1` classification).
- L16: Differential old (cmx) vs new binary with isolated HOME: only the version string differs for --help, version, install/sync --dry-run, doctor --help, review mode status. Native review assessed high for 54056 lines; user chose "Omitir esta vez" (declined_this_candidate). Open: S9 version scheme.
