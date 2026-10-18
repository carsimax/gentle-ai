# Supported Platforms

> [!NOTE]
> These docs track `main`, which may include unreleased changes. The install commands track the latest release. For the latest release docs, see the [v4.0.0 docs](https://github.com/Gentleman-Programming/gentle-ai/tree/v4.0.0/docs).

← [Back to README](../README.md)

---

| Platform | Package Manager | Status |
|----------|----------------|--------|
| macOS on Apple Silicon (arm64) | Homebrew | Supported |

This fork runs only on macOS on Apple Silicon; `gentle-ai` refuses to start on any other OS or architecture.

## OpenCode Managed Launcher

When OpenCode background subagents are enabled through `gentle-ai install` or `gentle-ai sync`, Gentle AI™ writes only its own launcher files under `~/.gentle-ai/bin/`. The launcher is `~/.gentle-ai/bin/opencode`. The launcher sets `OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS=true` only when the variable is not already defined, so an explicit `false` always selects foreground execution.

Gentle AI also persists `~/.gentle-ai/bin/` in the login profile your `SHELL` reads, inside a marked block (`# >>> gentle-ai managed OpenCode launcher >>>` … `<<<`). zsh uses `~/.zprofile`; bash uses the first existing of `~/.bash_profile`, `~/.bash_login`, or `~/.profile`, and creates `~/.profile` when none exists; `sh`, `dash`, and `ksh` use `~/.profile`. Gentle AI never modifies a profile that is a symlink, read-only, or carries an edited or duplicated block, and it does not manage other shells (such as fish) or a zsh `ZDOTDIR` outside your home. In those cases the activation report shows the `export PATH=...` line to add yourself, and `gentle-ai doctor` warns until the directory is persisted.

The activation report and `gentle-ai doctor` state whether activation is effective, not only whether your OpenCode version supports it. Gentle AI replays the `PATH` edits of your startup files (zsh: `~/.zshenv`, `~/.zprofile`, `~/.zshrc`, `~/.zlogin`; bash: the login profile; files they source are followed) and checks which `opencode` a new login shell runs:

| Status | Meaning |
|--------|---------|
| `ready` | New login shells run the managed launcher. |
| `pending` | Activation is not applied yet, or no startup file puts `~/.gentle-ai/bin/` on `PATH`. |
| `shadowed` | A startup file adds another OpenCode directory ahead of the launcher. The OpenCode installer does this when it appends `export PATH=~/.opencode/bin:$PATH` to `~/.zshrc` or `~/.bashrc`, which run after the login profile. The report names the file; remove that line or add the `export PATH=...` line after it. Gentle AI never edits rc files. |
| `unknown` | Gentle AI cannot verify which `opencode` new shells run. This is informational: either the OpenCode runtime could not be probed (execution stays in the foreground), the shell's startup cannot be modeled (fish, unset `SHELL`), or a startup file read after the managed block contains something that may change `PATH` but cannot be replayed statically — `eval` (for example `brew shellenv` or `mise activate`), command substitution in a `PATH` value, `PATH` changes inside a function, `case`, loop, or `else` body (pnpm, nvm), or a sourced file that cannot be resolved or read. The reason names the file; run `command -v opencode` in a new login shell to check it prints the managed launcher. |
| `unsupported` | The OpenCode version is too old; execution stays in the foreground. |
| `off` | Background subagents are turned off. |

Deactivation and uninstall remove managed launcher files and only the unedited managed profile block; every other profile line is preserved.

Restart OpenCode after enabling managed activation. Start a new login shell so the persisted launcher directory enters PATH. OpenCode `serve`, `attach`, Desktop, or any session started outside the managed launcher uses foreground fallback rather than receiving an unsafe partial activation.

---

## Gentle Shell installer

`gentle-ai shell install` has its own platform contract: macOS 14+ on Apple silicon. See [Gentle Shell on macOS](gentle-shell-macos-install.md).
