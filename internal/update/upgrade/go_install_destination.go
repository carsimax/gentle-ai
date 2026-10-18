package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// `go install` writes the built binary to GOBIN, or to the first GOPATH entry's
// bin/ directory when GOBIN is unset. That location is not necessarily the one
// the user's shell resolves for the tool: a stale copy earlier on PATH keeps
// running after a successful install, which looks exactly like an upgrade that
// silently did nothing.
//
// The helpers below resolve both sides and describe the difference. They never
// turn a successful install into a failure — the new binary really was written,
// so the worst honest outcome is a successful upgrade carrying a warning, and
// the second-worst is a successful upgrade the tool could not verify.

// goEnvValue reads a single `go env` variable through the execCommand seam.
func goEnvValue(key string) (string, error) {
	cmd := execCommand("go", "env", key)
	cmd.Stdin = nil
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go env %s: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// goInstallDestinationDir resolves the directory `go install` writes into:
// GOBIN when set, otherwise the first GOPATH entry plus "bin".
func goInstallDestinationDir() (string, error) {
	gobin, err := goEnvValue("GOBIN")
	if err != nil {
		return "", err
	}
	if gobin != "" {
		return gobin, nil
	}

	gopath, err := goEnvValue("GOPATH")
	if err != nil {
		return "", err
	}
	entries := filepath.SplitList(gopath)
	if len(entries) == 0 || strings.TrimSpace(entries[0]) == "" {
		return "", fmt.Errorf("go env reported neither GOBIN nor GOPATH")
	}
	return filepath.Join(entries[0], "bin"), nil
}

// sameBinaryPath reports whether two paths designate the same executable.
// Symlinks are resolved when they can be, so a ~/.local/bin shim pointing into
// the go-install directory counts as a match.
func sameBinaryPath(a, b string) bool {
	a = normalizeBinaryPath(a)
	b = normalizeBinaryPath(b)
	if a == "" || b == "" {
		return false
	}
	return a == b
}

func normalizeBinaryPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = absoluteBinaryPath(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Clean(path)
}

func absoluteBinaryPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// goInstallDestinationNotice compares where `go install` wrote against the
// binary the shell resolves for toolName, and returns the message the user
// needs. An empty string means the destination was verified as a match.
//
// destDir/destErr come from goInstallDestinationDir, resolved before the
// install runs: GOBIN and GOPATH are static Go configuration that `go install`
// does not change, while the PATH lookup below is deliberately performed after
// the install so a first-time install is resolved correctly.
func goInstallDestinationNotice(toolName, destDir string, destErr error) string {
	if destErr != nil {
		return unverifiedDestinationNotice(toolName, "", destErr.Error())
	}

	installed := absoluteBinaryPath(filepath.Join(destDir, toolName))
	if _, err := os.Stat(installed); err != nil {
		// Claiming a mismatch means claiming `go install` wrote a binary here.
		// Without an observable file that claim is unfounded, so report the
		// honest weaker result instead of fabricating either outcome.
		return unverifiedDestinationNotice(toolName, "", fmt.Sprintf("no binary was found at the expected go-install destination %s: %v", installed, err))
	}

	effective, err := lookPathFn(toolName)
	if err != nil {
		return unverifiedDestinationNotice(toolName, installed, fmt.Sprintf("%s could not be resolved on PATH: %v", toolName, err))
	}
	effective = absoluteBinaryPath(effective)

	if sameBinaryPath(installed, effective) {
		return ""
	}

	return fmt.Sprintf(
		"`go install` wrote the new %s to %s, but your shell still resolves %s to %s — that stale binary will keep running. Fix it by putting %s ahead of %s on PATH, or by replacing/relinking %s so it points at %s.",
		toolName, installed,
		toolName, effective,
		filepath.Dir(installed), filepath.Dir(effective),
		effective, installed,
	)
}

func unverifiedDestinationNotice(toolName, installed, reason string) string {
	notice := fmt.Sprintf("%s was upgraded with `go install`, but its destination could not be verified: %s.", toolName, reason)
	if installed != "" {
		notice += fmt.Sprintf(" `go install` writes to %s.", installed)
	}
	return notice + fmt.Sprintf(" The install itself succeeded; confirm that the directory reported by `go env GOBIN` (or `go env GOPATH` plus /bin) is the one your shell resolves for %s.", toolName)
}

// warnGoInstallDestination emits the destination notice, if any, as a non-fatal
// warning. It never returns an error: the install already succeeded.
func warnGoInstallDestination(toolName, destDir string, destErr error) {
	if notice := goInstallDestinationNotice(toolName, destDir, destErr); notice != "" {
		fmt.Fprintf(os.Stderr, "WARNING: %s\n", notice)
	}
}
