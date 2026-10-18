package upgrade

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/system"
	"github.com/gentleman-programming/gentle-ai/v4/internal/update"
)

// captureStderr runs fn with os.Stderr redirected to a pipe and returns what fn wrote.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStderr := os.Stderr
	os.Stderr = writeEnd
	t.Cleanup(func() { os.Stderr = origStderr })

	fn()

	os.Stderr = origStderr
	if err := writeEnd.Close(); err != nil {
		t.Fatal(err)
	}
	captured, err := io.ReadAll(readEnd)
	if err != nil {
		t.Fatal(err)
	}
	if err := readEnd.Close(); err != nil {
		t.Fatal(err)
	}
	return string(captured)
}

// stubGoEnv routes `go env <KEY>` to the supplied values and every other command
// to a successful no-op, so a go-install upgrade can run without touching the host.
func stubGoEnv(t *testing.T, values map[string]string) {
	t.Helper()
	origExecCommand := execCommand
	t.Cleanup(func() { execCommand = origExecCommand })
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "go" && len(args) == 2 && args[0] == "env" {
			return mockCmd("echo", values[args[1]])
		}
		return mockCmd("true")
	}
}

// resolvedTempDir is t.TempDir() with symlinks and short names already
// resolved, so a fixture path compares equal to the same path after the
// production code normalizes it.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return dir
	}
	return resolved
}

func writeFakeBinary(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func goInstallResult() update.UpdateResult {
	return update.UpdateResult{
		Tool: update.ToolInfo{
			Name:          "engram",
			InstallMethod: update.InstallGoInstall,
			GoImportPath:  "github.com/Gentleman-Programming/engram/cmd/engram",
		},
		LatestVersion: "0.4.0",
	}
}

func TestGoInstallUpgradeIsSilentWhenDestinationMatchesEffectiveBinary(t *testing.T) {
	gobin := t.TempDir()
	installed := writeFakeBinary(t, gobin, "engram")

	stubGoEnv(t, map[string]string{"GOBIN": gobin})
	origLookPath := lookPathFn
	t.Cleanup(func() { lookPathFn = origLookPath })
	lookPathFn = func(string) (string, error) { return installed, nil }

	var upgradeErr error
	stderr := captureStderr(t, func() {
		_, upgradeErr = runStrategy(context.Background(), goInstallResult(), system.PlatformProfile{OS: "darwin"})
	})

	if upgradeErr != nil {
		t.Fatalf("go-install upgrade returned error: %v", upgradeErr)
	}
	if strings.Contains(stderr, "WARNING") {
		t.Fatalf("matching destination must not warn; stderr = %q", stderr)
	}
}

func TestGoInstallUpgradeWarnsWhenDestinationDiffersFromEffectiveBinary(t *testing.T) {
	gobin := t.TempDir()
	stale := t.TempDir()
	installed := writeFakeBinary(t, gobin, "engram")
	shadowing := writeFakeBinary(t, stale, "engram")

	stubGoEnv(t, map[string]string{"GOBIN": gobin})
	origLookPath := lookPathFn
	t.Cleanup(func() { lookPathFn = origLookPath })
	lookPathFn = func(string) (string, error) { return shadowing, nil }

	var upgradeErr error
	stderr := captureStderr(t, func() {
		_, upgradeErr = runStrategy(context.Background(), goInstallResult(), system.PlatformProfile{OS: "darwin"})
	})

	if upgradeErr != nil {
		t.Fatalf("a destination mismatch must not fail the upgrade: %v", upgradeErr)
	}
	if !strings.Contains(stderr, "WARNING") {
		t.Fatalf("destination mismatch must surface a warning; stderr = %q", stderr)
	}
	for _, want := range []string{installed, shadowing} {
		if !strings.Contains(stderr, want) {
			t.Errorf("warning must name %q; stderr = %q", want, stderr)
		}
	}
	if !strings.Contains(stderr, "PATH") {
		t.Errorf("warning must tell the user what to do about PATH; stderr = %q", stderr)
	}
}

func TestGoInstallUpgradeTreatsSymlinkIntoGoBinAsMatch(t *testing.T) {
	gobin := t.TempDir()
	localBin := t.TempDir()
	installed := writeFakeBinary(t, gobin, "engram")
	link := filepath.Join(localBin, "engram")
	if err := os.Symlink(installed, link); err != nil {
		t.Fatal(err)
	}

	stubGoEnv(t, map[string]string{"GOBIN": gobin})
	origLookPath := lookPathFn
	t.Cleanup(func() { lookPathFn = origLookPath })
	lookPathFn = func(string) (string, error) { return link, nil }

	var upgradeErr error
	stderr := captureStderr(t, func() {
		_, upgradeErr = runStrategy(context.Background(), goInstallResult(), system.PlatformProfile{OS: "darwin"})
	})

	if upgradeErr != nil {
		t.Fatalf("go-install upgrade returned error: %v", upgradeErr)
	}
	if strings.Contains(stderr, "WARNING") {
		t.Fatalf("a symlink resolving into the go-install dir is a match, not a mismatch; stderr = %q", stderr)
	}
}

func TestGoInstallUpgradeFallsBackToGoPathBinWhenGoBinIsEmpty(t *testing.T) {
	gopath := t.TempDir()
	installed := writeFakeBinary(t, filepath.Join(gopath, "bin"), "engram")

	stubGoEnv(t, map[string]string{"GOBIN": "", "GOPATH": gopath})
	origLookPath := lookPathFn
	t.Cleanup(func() { lookPathFn = origLookPath })
	lookPathFn = func(string) (string, error) { return installed, nil }

	var upgradeErr error
	stderr := captureStderr(t, func() {
		_, upgradeErr = runStrategy(context.Background(), goInstallResult(), system.PlatformProfile{OS: "darwin"})
	})

	if upgradeErr != nil {
		t.Fatalf("go-install upgrade returned error: %v", upgradeErr)
	}
	if strings.Contains(stderr, "WARNING") {
		t.Fatalf("GOPATH/bin destination must be recognized as a match; stderr = %q", stderr)
	}
}

func TestGoInstallUpgradeReportsUnverifiedWhenGoEnvFails(t *testing.T) {
	origExecCommand := execCommand
	t.Cleanup(func() { execCommand = origExecCommand })
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "go" && len(args) == 2 && args[0] == "env" {
			return mockCmd("false")
		}
		return mockCmd("true")
	}
	origLookPath := lookPathFn
	t.Cleanup(func() { lookPathFn = origLookPath })
	lookPathFn = func(string) (string, error) { return "/usr/local/bin/engram", nil }

	var upgradeErr error
	stderr := captureStderr(t, func() {
		_, upgradeErr = runStrategy(context.Background(), goInstallResult(), system.PlatformProfile{OS: "darwin"})
	})

	if upgradeErr != nil {
		t.Fatalf("an unverifiable destination must not fail a successful install: %v", upgradeErr)
	}
	if !strings.Contains(stderr, "WARNING") || !strings.Contains(stderr, "could not be verified") {
		t.Fatalf("stderr must state plainly that the destination is unverified; stderr = %q", stderr)
	}
	if strings.Contains(stderr, "still resolves") {
		t.Fatalf("unverified must not be reported as a concrete mismatch; stderr = %q", stderr)
	}
}

func TestGoInstallUpgradeReportsUnverifiedWhenPathLookupFails(t *testing.T) {
	gobin := t.TempDir()
	stubGoEnv(t, map[string]string{"GOBIN": gobin})
	origLookPath := lookPathFn
	t.Cleanup(func() { lookPathFn = origLookPath })
	lookPathFn = func(string) (string, error) { return "", exec.ErrNotFound }

	var upgradeErr error
	stderr := captureStderr(t, func() {
		_, upgradeErr = runStrategy(context.Background(), goInstallResult(), system.PlatformProfile{OS: "darwin"})
	})

	if upgradeErr != nil {
		t.Fatalf("an unresolvable PATH entry must not fail a successful install: %v", upgradeErr)
	}
	if !strings.Contains(stderr, "WARNING") || !strings.Contains(stderr, "could not be verified") {
		t.Fatalf("stderr must state plainly that the destination is unverified; stderr = %q", stderr)
	}
}

// A mismatch warning asserts that `go install` wrote a binary to a specific
// path. When nothing is there, that claim is unfounded and the honest report is
// "unverified", not a fabricated mismatch.
func TestGoInstallUpgradeReportsUnverifiedWhenNothingLandedInTheDestination(t *testing.T) {
	gobin := t.TempDir()
	stale := t.TempDir()
	shadowing := writeFakeBinary(t, stale, "engram")

	stubGoEnv(t, map[string]string{"GOBIN": gobin})
	origLookPath := lookPathFn
	t.Cleanup(func() { lookPathFn = origLookPath })
	lookPathFn = func(string) (string, error) { return shadowing, nil }

	var upgradeErr error
	stderr := captureStderr(t, func() {
		_, upgradeErr = runStrategy(context.Background(), goInstallResult(), system.PlatformProfile{OS: "darwin"})
	})

	if upgradeErr != nil {
		t.Fatalf("an unverifiable destination must not fail a successful install: %v", upgradeErr)
	}
	if !strings.Contains(stderr, "could not be verified") {
		t.Fatalf("an empty destination must be reported as unverified; stderr = %q", stderr)
	}
	if strings.Contains(stderr, "still resolves") {
		t.Fatalf("must not claim a mismatch against a binary that was never observed; stderr = %q", stderr)
	}
}

// The beta self-upgrade runs the same `go install` mechanism and carries the
// same silent-no-op risk, so it must be verified identically.
func TestBetaGoInstallMainUpgradeWarnsWhenDestinationDiffers(t *testing.T) {
	gobin := t.TempDir()
	stale := t.TempDir()
	installed := writeFakeBinary(t, gobin, "gentle-ai")
	shadowing := writeFakeBinary(t, stale, "gentle-ai")

	stubGoEnv(t, map[string]string{"GOBIN": gobin})
	origLookPath := lookPathFn
	t.Cleanup(func() { lookPathFn = origLookPath })
	lookPathFn = func(string) (string, error) { return shadowing, nil }

	r := update.UpdateResult{
		Tool: update.ToolInfo{
			Name:          "gentle-ai",
			Owner:         "Gentleman-Programming",
			Repo:          "gentle-ai",
			InstallMethod: update.InstallBinary,
		},
		LatestVersion:  "main@abc1234abcde",
		BetaCommit:     "abc1234abcde0123456789abcdef0123456789ab",
		BetaModulePath: "github.com/gentleman-programming/gentle-ai/v4",
		Status:         update.UpdateAvailable,
	}

	var upgradeErr error
	stderr := captureStderr(t, func() {
		_, upgradeErr = runStrategy(context.Background(), r, system.PlatformProfile{OS: "darwin", Supported: true})
	})

	if upgradeErr != nil {
		t.Fatalf("beta go-install upgrade returned error: %v", upgradeErr)
	}
	if !strings.Contains(stderr, "WARNING") {
		t.Fatalf("beta destination mismatch must surface a warning; stderr = %q", stderr)
	}
	for _, want := range []string{installed, shadowing} {
		if !strings.Contains(stderr, want) {
			t.Errorf("beta warning must name %q; stderr = %q", want, stderr)
		}
	}
}

func TestSameBinaryPathRequiresSameLocation(t *testing.T) {
	base := t.TempDir()
	installed := filepath.Join(base, "gentle-ai")

	if !sameBinaryPath(installed, installed) {
		t.Errorf("identical paths must compare equal: %q", installed)
	}
	if sameBinaryPath(filepath.Join(base, "a", "gentle-ai"), filepath.Join(base, "b", "gentle-ai")) {
		t.Error("different directories must not compare equal")
	}
	if sameBinaryPath("", installed) || sameBinaryPath(installed, "") {
		t.Error("an empty path must never compare equal")
	}
}
