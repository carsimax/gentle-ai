# carsimax/gentle-ai fork

Personal fork of [Gentleman-Programming/gentle-ai](https://github.com/Gentleman-Programming/gentle-ai).
It tracks upstream `main` and carries a few upstream pull requests that are not merged yet.
See [Fork changelog](#fork-changelog) for exactly what differs.

## Build and install locally

### Prerequisites

- Go 1.25.10 or newer (`go version`)
- Git, Node.js 18+ and npm (checked by `gentle-ai install`)
- Claude Code, if you want the Claude Code agent configured

### Build

```bash
git clone https://github.com/carsimax/gentle-ai.git
cd gentle-ai
go build -o gentle-ai ./cmd/gentle-ai
./gentle-ai version   # prints "dev"
```

A source build reports version `dev`. `gentle-ai upgrade` skips upgrading itself on a
`dev` build, so it never replaces this binary with an upstream release.

To have it on `PATH`, copy it somewhere already on `PATH`, for example:

```bash
install -m 0755 gentle-ai ~/.local/bin/gentle-ai
```

Do not also install the Homebrew `gentle-ai` formula: whichever comes first on `PATH`
wins, and the upstream release does not contain the fork changes.

### Install into Claude Code

```bash
gentle-ai install --dry-run --agent claude-code   # preview the plan
gentle-ai                                         # interactive TUI install
```

Then pick the subagent models:

1. Run `gentle-ai`, open **Configure Claude models**, choose **Custom**.
2. Set **ODD Explorer**, **ODD Worker** and **ODD Verify**. Leave a row on `inherit` to
   run that agent on the session model.
3. Optionally set **Built-in Explore** and **Built-in general-purpose** (see the caveat
   below).
4. Run `gentle-ai sync`, then start a new Claude Code session. Agents load at session
   start.

Check the result:

```bash
grep -H '^model:\|^effort:' ~/.claude/agents/gentle-ai-*.md
```

### Backups and rollback

Every `install`, `sync` and `upgrade` snapshots the files it touches first.

```bash
gentle-ai restore --list     # list backups
gentle-ai restore latest     # restore the most recent one
gentle-ai doctor             # health diagnostics
```

### Run the tests

Day to day, run the fast lane: the Claude Code packages plus the Claude and sync CLI
tests, about 75 seconds uncached on Apple silicon. Extra arguments go to `go test`.

```bash
./scripts/test-claude.sh            # cached results are reused
./scripts/test-claude.sh -count=1   # force a fresh run
```

After rebasing on upstream and before pushing `main`, run the full suite, about 17
minutes, almost all of it `internal/cli`:

```bash
GOTOOLCHAIN=go1.25.10 go test -timeout 40m ./...
```

On macOS the full suite also fails `internal/update` release-script tests, because
those scripts need Bash 4 and macOS ships Bash 3.2. They only matter for publishing
releases.

The module targets Go 1.25.10. Under Go 1.27,
`TestOwnershipReuseRejectsLedgerWithoutMutation/hash_number` in `reviewassets` fails
because `encoding/json` adds the map key to its error path; it fails identically on
upstream `main`. Run the suite with `GOTOOLCHAIN=go1.25.10 go test ...` to match CI.

## Keeping up with upstream

```bash
git remote add upstream https://github.com/Gentleman-Programming/gentle-ai.git  # once
git fetch upstream
git rebase upstream/main      # drops fork commits whose patch upstream already merged
git push --force-with-lease origin main
```

When upstream merges all the pull requests below, `main` can be reset to
`upstream/main` and this file removed.

## Fork changelog

### 2026-10-24: Claude Code subagents honor the configured models

**Problem.** Upstream issues [#5610](https://github.com/Gentleman-Programming/gentle-ai/issues/5610)
and [#5611](https://github.com/Gentleman-Programming/gentle-ai/issues/5611): the ODD
Explorer, Worker and Verify model picks were saved but never applied. Claude Code
delegated to its built-in agents, which ran on the parent session's model.

**Fix.** Five upstream pull requests by viniciosrab, cherry-picked onto upstream
`f05ce6ee` and rebased onto `32701269a` on 2026-10-25:

| Fork commit | Upstream PR | Change |
|---|---|---|
| `e0d8bcbcd` | [#5612](https://github.com/Gentleman-Programming/gentle-ai/pull/5612) | `sync` installs `gentle-ai-explore`, `gentle-ai-worker` and `gentle-ai-verify` in `~/.claude/agents/` with `model:`/`effort:` from the picks; `CLAUDE.md` routes ODD delegation to them |
| `4295a2692` | [#5660](https://github.com/Gentleman-Programming/gentle-ai/pull/5660) | Presets no longer seed the ODD rows; an unpicked row is `inherit` |
| `b19248794` | [#5659](https://github.com/Gentleman-Programming/gentle-ai/pull/5659) | Opt-in `Explore.md` / `general-purpose.md` overrides, written only while picked and removed when cleared |
| `43182289b` | [#5661](https://github.com/Gentleman-Programming/gentle-ai/pull/5661) | Built-in Explore and general-purpose rows in the Claude model picker |
| `7a824f304` | [#5658](https://github.com/Gentleman-Programming/gentle-ai/pull/5658) | Telemetry classifies the managed ODD agents |

`#5659` comes from the copy stacked in `#5661` (`c6685e68`); the standalone PR head
conflicts without `#5612`.

**Verified.**

- `go build ./...` and the affected package tests pass; `internal/cli` passes with
  `-timeout 40m`.
- `install` plus `sync` in an isolated `HOME`: without picks, all three agents are
  `model: inherit` and no override is written. With picks they render the chosen
  model and effort. Clearing the picks removes the overrides on the next `sync`.
- No live Claude Code call was made from this fork. Upstream reported a live check on
  Linux with Claude Code 2.1.306 where each subagent ran on its picked model.

**Known limitations.**

- An explicit pick of Sonnet with no effort on all three ODD rows, on top of an
  otherwise unchanged preset, is treated as a legacy seed and becomes `inherit`. Set an
  effort to avoid it.
- A `model` argument passed in an `Agent` call overrides the agent's frontmatter.
- The `general-purpose.md` override keeps every tool, including Bash, Edit and Write.
- Telemetry reads `~/.claude/agents/`, even when a project defines an agent with the
  same name.
- Other plugins that inject their own delegation contract (for example `construct`)
  compete with the Gentle-AI `CLAUDE.md` routing.
