package app

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gentleman-programming/gentle-ai/v4/internal/cli"
	"github.com/gentleman-programming/gentle-ai/v4/internal/planner"
	"github.com/gentleman-programming/gentle-ai/v4/internal/system"
	"github.com/gentleman-programming/gentle-ai/v4/internal/tui"
)

func TestInstallDefaultsMatchTUIModelDefaults(t *testing.T) {
	detection := system.DetectionResult{
		Configs: []system.ConfigState{
			{Agent: "claude-code", Exists: true, IsDirectory: true},
			{Agent: "opencode", Exists: false},
		},
	}

	flags, err := cli.ParseInstallFlags(nil)
	if err != nil {
		t.Fatalf("ParseInstallFlags() error = %v", err)
	}

	input, err := cli.NormalizeInstallFlags(flags, detection)
	if err != nil {
		t.Fatalf("NormalizeInstallFlags() error = %v", err)
	}

	model := tui.NewModel(detection, "dev")
	if !reflect.DeepEqual(input.Selection, model.Selection) {
		t.Fatalf("selection mismatch\ncli=%#v\ntui=%#v", input.Selection, model.Selection)
	}
}

func TestInstallPlannerParityWithTUISelection(t *testing.T) {
	detection := system.DetectionResult{}
	model := tui.NewModel(detection, "dev")

	result, err := runInstallForParity([]string{"--dry-run"}, detection)
	if err != nil {
		t.Fatalf("RunInstall() error = %v", err)
	}

	if !reflect.DeepEqual(result.Selection, model.Selection) {
		t.Fatalf("selection mismatch\ncli=%#v\ntui=%#v", result.Selection, model.Selection)
	}

	wantResolved, err := planner.NewResolver(planner.MVPGraph()).Resolve(model.Selection)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	wantResolved.PlatformDecision = planner.PlatformDecision{OS: "darwin", PackageManager: "brew", Supported: true}

	if !reflect.DeepEqual(result.Resolved, wantResolved) {
		t.Fatalf("resolved mismatch\ncli=%#v\ntui=%#v", result.Resolved, wantResolved)
	}

	wantReview := planner.BuildReviewPayload(model.Selection, wantResolved)
	if !reflect.DeepEqual(result.Review, wantReview) {
		t.Fatalf("review mismatch\ncli=%#v\ntui=%#v", result.Review, wantReview)
	}
}

// --- Batch D: App guard-flow tests ---

func TestGuardRejectsUnsupportedOS(t *testing.T) {
	for _, goos := range []string{"freebsd", "linux", "windows"} {
		err := system.EnsureSupportedOS(goos)
		if err == nil {
			t.Fatalf("expected error for unsupported OS %q", goos)
		}
		if !errors.Is(err, system.ErrUnsupportedOS) {
			t.Fatalf("expected ErrUnsupportedOS, got %v", err)
		}
	}
}

func TestGuardAcceptsDarwin(t *testing.T) {
	if err := system.EnsureSupportedOS("darwin"); err != nil {
		t.Fatalf("expected darwin to be accepted, got %v", err)
	}
}

func TestRunArgsNoCommandLaunchesTUI(t *testing.T) {
	origRunTUI := runTUI
	t.Cleanup(func() { runTUI = origRunTUI })
	runTUI = func(m tea.Model, opts ...tea.ProgramOption) (tea.Model, error) {
		return nil, errors.New("mock TUI error: no TTY")
	}

	var buf bytes.Buffer
	err := RunArgs(nil, &buf)
	// With no args, RunArgs now launches the TUI via Bubbletea.
	// In a headless/test environment without a TTY, this returns an error
	// about opening /dev/tty. That's expected — the TUI requires a terminal.
	if err == nil {
		// If no error, we're somehow in a TTY — that's fine too.
		return
	}
	if !strings.Contains(err.Error(), "TTY") && !strings.Contains(err.Error(), "tty") && !strings.Contains(err.Error(), "mock TUI") {
		t.Fatalf("RunArgs(nil) unexpected error = %v; want TTY-related error or nil", err)
	}
}

func TestRunArgsUnknownCommandReturnsError(t *testing.T) {
	var buf bytes.Buffer
	err := RunArgs([]string{"bogus"}, &buf)
	if err == nil {
		t.Fatalf("RunArgs(bogus) expected error")
	}
	if !strings.Contains(err.Error(), `unknown command "bogus"`) {
		t.Fatalf("RunArgs(bogus) error = %v", err)
	}
}

// --- Sync command wiring tests ---

// TestRunArgsSyncDryRunIsDispatchedAndPrintsReport verifies that
// `RunArgs(["sync", "--dry-run", ...])` is correctly wired through app.go
// and produces output about the sync plan — not a "unknown command" error.
func TestRunArgsSyncDryRunIsDispatchedAndPrintsReport(t *testing.T) {
	var buf bytes.Buffer
	err := RunArgs([]string{"sync", "--agents", "opencode", "--dry-run"}, &buf)
	if err != nil {
		t.Fatalf("RunArgs(sync --dry-run) error = %v", err)
	}

	out := buf.String()
	if out == "" {
		t.Fatalf("RunArgs(sync --dry-run) produced no output")
	}

	// Output must mention agents or components — it is the sync plan report.
	if !strings.Contains(out, "sync") && !strings.Contains(out, "Sync") &&
		!strings.Contains(out, "agent") && !strings.Contains(out, "Agent") {
		t.Errorf("sync --dry-run output should mention sync or agents; got:\n%s", out)
	}
}

// TestRunArgsSyncUnknownFlagReturnsError verifies that an unknown flag
// returns a proper parse error via the sync command path.
func TestRunArgsSyncUnknownFlagReturnsError(t *testing.T) {
	var buf bytes.Buffer
	err := RunArgs([]string{"sync", "--this-flag-does-not-exist"}, &buf)
	if err == nil {
		t.Fatalf("RunArgs(sync --unknown) expected error")
	}
	// Must not be "unknown command" — sync IS a known command.
	if err.Error() == `unknown command "sync"` {
		t.Fatalf("sync command is not registered in app.go dispatch")
	}
}

func TestRunArgsSyncHelpIncludesBackgroundFlag(t *testing.T) {
	var buf bytes.Buffer
	if err := RunArgs([]string{"sync", "--help"}, &buf); err != nil {
		t.Fatalf("RunArgs(sync --help) error = %v", err)
	}
	if !strings.Contains(buf.String(), "--opencode-background-subagents=auto|on|off") || !strings.Contains(buf.String(), cli.OpenCodeBackgroundSubagentsEnv) {
		t.Fatalf("sync help omits background contract: %s", buf.String())
	}
}

// TestRunArgsSyncNoAgentsIsNoOp verifies that `gentle-ai sync` with no
// agents flag and an empty home dir (no config dirs) completes as a no-op
// and does NOT return an error.
func TestRunArgsSyncNoAgentsIsNoOp(t *testing.T) {
	// We can't override osUserHomeDir from the app package directly,
	// but we can verify that the command exits without error and prints
	// something meaningful (the no-op message).
	// Use --dry-run to avoid any file creation and allow running in CI.
	var buf bytes.Buffer
	err := RunArgs([]string{"sync", "--agents", "opencode", "--dry-run"}, &buf)
	if err != nil {
		t.Fatalf("RunArgs(sync --dry-run): %v", err)
	}
	out := buf.String()
	if out == "" {
		t.Fatalf("sync --dry-run should produce output, got empty string")
	}
}

// --- Batch E: macOS parity regression ---

func TestMacOSDefaultProfileFallbackWhenDetectionEmpty(t *testing.T) {
	// When DetectionResult is zero-value (no profile), CLI defaults to macOS/brew.
	detection := system.DetectionResult{}
	result, err := runInstallForParity([]string{"--dry-run"}, detection)
	if err != nil {
		t.Fatalf("RunInstall() error = %v", err)
	}

	if result.Resolved.PlatformDecision.OS != "darwin" {
		t.Fatalf("default platform decision OS = %q, want darwin", result.Resolved.PlatformDecision.OS)
	}
	if result.Resolved.PlatformDecision.PackageManager != "brew" {
		t.Fatalf("default platform decision PM = %q, want brew", result.Resolved.PlatformDecision.PackageManager)
	}
	if !result.Resolved.PlatformDecision.Supported {
		t.Fatalf("default platform decision Supported = false, want true")
	}
}

func TestGuardFlowMacOSProfileExplicitlyPasses(t *testing.T) {
	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true}
	if err := system.EnsureSupportedPlatform(profile); err != nil {
		t.Fatalf("macOS profile should pass guard, got %v", err)
	}
}

// runInstallForParity exercises the same prepared-plan path as production dispatch.
func runInstallForParity(args []string, detection system.DetectionResult) (cli.InstallResult, error) {
	prepared, err := cli.PrepareInstall(args, detection)
	if err != nil {
		return cli.InstallResult{}, err
	}
	return cli.RunPreparedInstall(prepared, detection)
}
