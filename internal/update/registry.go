package update

import (
	"path/filepath"
)

// Tools is the static registry of managed tools that can be checked for updates.
//
// InstallMethod controls which non-Homebrew upgrade strategy the executor uses:
//   - InstallBrew: explicitly managed via Homebrew
//   - InstallGoInstall: installed via `go install <GoImportPath>@version`,
//     where GoImportPath is the suffix-less module/import base and the /vN
//     suffix is derived from the target version via ModulePathForVersion.
//   - InstallBinary: downloaded binary from GitHub Releases (atomic replace)
//
// On platforms where Homebrew is available, the executor selects brew only
// when Homebrew confirms it owns the specific tool. Otherwise this field is
// the fallback strategy.
var Tools = []ToolInfo{
	{
		Name:          "gentle-ai",
		Owner:         "Gentleman-Programming",
		Repo:          "gentle-ai",
		DetectCmd:     nil, // version comes from build-time ldflags (app.Version)
		VersionPrefix: "v",
		// gentle-ai: Homebrew when the package is brew-owned, authenticated binary
		// release download otherwise.
		InstallMethod: InstallBinary,
		// GoImportPath is used only by unanchored source builds. It is
		// deliberately NOT a general opt-in to go-install: effectiveMethod routes
		// anchored release builds to InstallBinary regardless of this field, so
		// they keep the minisign-verified release download.
		//
		// The path stores the module/import base WITHOUT the /vN suffix; the
		// /vN suffix is derived at composition time from the target version via
		// ModulePathForVersion so a v2 binary composing "go install ...@v3.0.1"
		// resolves to github.com/.../v3/... and not the unresolvable /v2 path.
		GoImportPath: "github.com/gentleman-programming/gentle-ai/cmd/gentle-ai",
	},
	{
		Name:              "engram",
		Owner:             "Gentleman-Programming",
		Repo:              "engram",
		DetectCmd:         []string{"engram", "version"},
		VersionPrefix:     "v",
		ReleaseTagPattern: `^v[0-9]+\.[0-9]+\.[0-9]+$`,
		// engram: Homebrew when the package is brew-owned, binary download elsewhere.
		InstallMethod: InstallBinary,
		// FallbackPaths covers ~/.local/bin when not yet in PATH: the shell
		// profile is updated but the current process does not see the change
		// until a new shell session starts.
		FallbackPaths: func(homeDir string) []string {
			var paths []string
			// ~/.local/bin/engram (when /usr/local/bin is not writable,
			// the binary installer places it here, which may not be in PATH yet).
			if homeDir != "" {
				paths = append(paths, filepath.Join(homeDir, ".local", "bin", "engram"))
			}
			return paths
		},
	},
	{
		Name:          "gga",
		Owner:         "Gentleman-Programming",
		Repo:          "gentleman-guardian-angel",
		DetectCmd:     []string{"gga", "--version"},
		VersionPrefix: "v",
		// gga: Homebrew when the package is brew-owned, install.sh script elsewhere.
		// GGA does not publish pre-built release binary assets — only source archives.
		// Using InstallScript runs curl | bash via the project's install.sh.
		InstallMethod: InstallScript,
		// FallbackPaths covers the stale-PATH scenario: the gga install script
		// places the binary in ~/.local/bin/gga or ~/bin/gga, which may not be in
		// PATH immediately after install.
		FallbackPaths: func(homeDir string) []string {
			var paths []string
			if homeDir != "" {
				// ~/.local/bin/gga
				paths = append(paths, filepath.Join(homeDir, ".local", "bin", "gga"))
				// ~/bin/gga
				paths = append(paths, filepath.Join(homeDir, "bin", "gga"))
			}
			return paths
		},
	},
}
