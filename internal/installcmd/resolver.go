package installcmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gentleman-programming/gentle-ai/v4/internal/model"
	"github.com/gentleman-programming/gentle-ai/v4/internal/system"
)

// cmdLookPath, osStat, osGetenv, and cmdGoVersion are package-level vars for testability.
var cmdLookPath = exec.LookPath
var osStat = os.Stat
var osGetenv = os.Getenv

// cmdBrewHelpTrust probes capability without changing Homebrew's trust state.
var cmdBrewHelpTrust = func(brew string) error {
	return exec.Command(brew, "help", "trust").Run()
}

var cmdGoVersion = func() ([]byte, error) {
	return exec.Command("go", "version").Output()
}

// CommandSequence represents an ordered list of commands to run in sequence.
// Each inner slice is a single command with its arguments (e.g., ["brew", "install", "engram"]).
// Multi-step installs (e.g., tap + install) are expressed as multiple entries.
type CommandSequence = [][]string

type Resolver interface {
	ResolveAgentInstall(profile system.PlatformProfile, agent model.AgentID) (CommandSequence, error)
	ResolveComponentInstall(profile system.PlatformProfile, component model.ComponentID) (CommandSequence, error)
	ResolveDependencyInstall(profile system.PlatformProfile, dependency string) (CommandSequence, error)
}

type profileResolver struct{}

func NewResolver() Resolver {
	return profileResolver{}
}

func (profileResolver) ResolveAgentInstall(profile system.PlatformProfile, agent model.AgentID) (CommandSequence, error) {
	switch agent {
	case model.AgentClaudeCode:
		return resolveClaudeCodeInstall(profile), nil
	case model.AgentOpenCode:
		return resolveOpenCodeInstall(profile)
	case model.AgentKilocode:
		return resolveKilocodeInstall(profile), nil
	case model.AgentKimi:
		return resolveKimiInstall(profile)
	default:
		return nil, fmt.Errorf("install command is not supported for agent %q", agent)
	}
}

// resolveClaudeCodeInstall returns the npm install command sequence gentle-ai
// shows for Claude Code — display text only, never executed by gentle-ai
// (see agentInstallStep in internal/cli/run.go). On macOS, sudo is never
// needed.
//
// --ignore-scripts blocks postinstall hooks, the primary supply-chain attack
// vector for npm packages. The version advises "latest" rather than a pin:
// a pin only guarded against a tampered "latest" tag when gentle-ai itself
// ran the command unattended. Now a human reads and runs it, and a stale
// hardcoded version goes wrong the moment a newer release ships (the same
// drift this shape fixed for Codex's GPT-5.6 update advice).
func resolveClaudeCodeInstall(_ system.PlatformProfile) CommandSequence {
	const pkg = "@anthropic-ai/claude-code@latest"
	return CommandSequence{{"npm", "install", "-g", "--ignore-scripts", pkg}}
}

// resolveKilocodeInstall returns the npm install command sequence gentle-ai
// shows for Kilocode — display text only, never executed by gentle-ai. On
// macOS, sudo is never needed.
func resolveKilocodeInstall(_ system.PlatformProfile) CommandSequence {
	const pkg = "@kilocode/cli@latest"
	return CommandSequence{{"npm", "install", "-g", "--ignore-scripts", pkg}}
}

// resolveKimiInstall returns the official Kimi install command sequence.
// To avoid the security risks of pipe-to-shell patterns (curl | bash),
// we execute the underlying command that the scripts alias: `uv tool install`.
func resolveKimiInstall(profile system.PlatformProfile) (CommandSequence, error) {
	// Kimi CLI is a python-based tool. We use Astral's `uv` as our deterministic
	// prerequisite manager to ensure secure and isolated installs.
	if !profile.Supported {
		return nil, fmt.Errorf("Kimi is not supported on this platform (%s)", profile.OS)
	}

	// We explicitly request python 3.13 as strictly defined by Kimi upstream.
	return CommandSequence{{"uv", "tool", "install", "--python", "3.13", "kimi-cli"}}, nil
}

// npmBasedAgents is the set of agents whose auto-install runs npm commands.
// When any of these agents is selected, npm (and therefore Node.js) must be
// present before the pipeline reaches the agent install step.
//
// AgentPi is included because InstallCommand always runs engramInitCommand(),
// which executes either `pnpm dlx` or `npm exec` (both require Node.js). The
// npm-presence check is a sound proxy for Node.js availability.
var npmBasedAgents = map[model.AgentID]struct{}{
	model.AgentClaudeCode: {},
	model.AgentOpenCode:   {},
	model.AgentKilocode:   {},
	model.AgentGeminiCLI:  {},
	model.AgentCodex:      {},
	model.AgentQwenCode:   {},
	model.AgentPi:         {},
}

// ValidateAgentInstallPreflight validates agent-specific prerequisites that must
// exist before running installation commands.
func ValidateAgentInstallPreflight(profile system.PlatformProfile, agent model.AgentID) error {
	if _, ok := npmBasedAgents[agent]; ok {
		if err := validateNpmInstallPreflight(profile); err != nil {
			return err
		}
	}
	switch agent {
	case model.AgentKimi:
		return validateKimiInstallPreflight(profile)
	case model.AgentPi:
		return validatePiInstallPreflight()
	default:
		return nil
	}
}

func validatePiInstallPreflight() error {
	if _, err := cmdLookPath("pi"); err != nil {
		return fmt.Errorf("Pi requires the `pi` executable in PATH before installing Gentle AI Pi packages")
	}

	return nil
}

// validateNpmInstallPreflight ensures npm (and therefore Node.js) is available
// before attempting any npm-based agent install. Called for all agents in
// npmBasedAgents so the user gets a clear, actionable error instead of a
// cryptic "exec: npm: executable file not found in PATH" mid-pipeline.
func validateNpmInstallPreflight(profile system.PlatformProfile) error {
	if _, err := cmdLookPath("npm"); err != nil {
		hint := system.InstallHintForDep("node", profile)
		return fmt.Errorf(
			"Node.js / npm is required but `npm` was not found in PATH.\n"+
				"Install Node.js (npm is included) and retry:\n"+
				"  %s",
			hint,
		)
	}
	return nil
}

func validateKimiInstallPreflight(profile system.PlatformProfile) error {
	if !profile.Supported {
		return fmt.Errorf("Kimi is not supported on this platform (%s)", profile.OS)
	}

	if _, err := cmdLookPath("uv"); err != nil {
		return fmt.Errorf(
			"Kimi requires Astral uv, but `uv` was not found in PATH.\n"+
				"Install uv and retry:\n"+
				"  %s",
			uvInstallHint(profile),
		)
	}

	return nil
}

func uvInstallHint(profile system.PlatformProfile) string {
	switch profile.PackageManager {
	case "brew":
		return "brew install uv"
	default:
		return "https://docs.astral.sh/uv/getting-started/installation/"
	}
}

func (profileResolver) ResolveComponentInstall(profile system.PlatformProfile, component model.ComponentID) (CommandSequence, error) {
	switch component {
	case model.ComponentEngram:
		return resolveEngramInstall(profile)
	case model.ComponentGGA:
		return resolveGGAInstall(profile)
	default:
		return nil, fmt.Errorf("install command is not supported for component %q", component)
	}
}

func (profileResolver) ResolveDependencyInstall(profile system.PlatformProfile, dependency string) (CommandSequence, error) {
	if dependency == "" {
		return nil, fmt.Errorf("dependency name is required")
	}

	switch profile.PackageManager {
	case "brew":
		return CommandSequence{{"brew", "install", dependency}}, nil
	default:
		return nil, fmt.Errorf(
			"unsupported package manager %q for os=%q",
			profile.PackageManager,
			profile.OS,
		)
	}
}

// resolveOpenCodeInstall is display-only advice for new V2 installations.
// Released @opencode/cli needs its postinstall binary setup: never add
// --ignore-scripts. Existing V1 installations are not upgraded by this advice.
// Source: https://opencode.ai/v2/docs/migrate-v1/
func resolveOpenCodeInstall(profile system.PlatformProfile) (CommandSequence, error) {
	const pkg = "@opencode/cli@latest"
	switch profile.PackageManager {
	case "brew":
		return CommandSequence{
			{"npm", "install", "-g", pkg},
		}, nil
	default:
		return nil, fmt.Errorf(
			"unsupported platform for opencode: os=%q pm=%q",
			profile.OS, profile.PackageManager,
		)
	}
}

// resolveGGAInstall returns the correct install command sequence for GGA per platform.
// - Homebrew: tap + scoped formula trust (when supported) + reinstall
func resolveGGAInstall(profile system.PlatformProfile) (CommandSequence, error) {
	switch profile.PackageManager {
	case "brew":
		return resolveHomebrewComponentInstall("gga", "reinstall"), nil
	default:
		return nil, fmt.Errorf(
			"unsupported platform for gga: os=%q pm=%q",
			profile.OS, profile.PackageManager,
		)
	}
}

// resolveHomebrewExecutable mirrors the CLI runner fallback so the capability
// probe can find Homebrew even when the inherited PATH omits its standard prefix.
func resolveHomebrewExecutable() string {
	if path, err := cmdLookPath("brew"); err == nil {
		return path
	}
	for _, binDir := range []string{"/opt/homebrew/bin", "/usr/local/bin"} {
		path := filepath.Join(binDir, "brew")
		info, err := osStat(path)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return path
		}
	}
	return "brew"
}

// resolveHomebrewComponentInstall trusts only the selected formula, never the
// whole tap. Older Homebrew versions without `trust` keep their existing flow.
func resolveHomebrewComponentInstall(formula, action string) CommandSequence {
	commands := CommandSequence{{"brew", "tap", "Gentleman-Programming/homebrew-tap"}}
	if cmdBrewHelpTrust(resolveHomebrewExecutable()) == nil {
		commands = append(commands, []string{"brew", "trust", "--formula", "gentleman-programming/tap/" + formula})
	}
	return append(commands, []string{"brew", action, formula})
}

// validateGoForModuleInstall checks that Go ≥1.24 is installed and GO111MODULE is not
// disabled before attempting `go install`. Returns an actionable error if any check fails.
// MUST NOT be called for brew-based installs (brew manages Go transitively).
func validateGoForModuleInstall() error {
	if _, err := cmdLookPath("go"); err != nil {
		return fmt.Errorf(
			"Go 1.24+ is required to install Engram but was not found in PATH.\n" +
				"Please install Go from https://go.dev/dl/ and restart your terminal.")
	}

	out, err := cmdGoVersion()
	if err != nil {
		return fmt.Errorf(
			"Go 1.24+ is required but could not verify the installed version.\n" +
				"Please ensure Go is properly installed: https://go.dev/dl/")
	}

	// Parse "go version go1.XX.Y platform/arch"
	parts := strings.Fields(string(out))
	if len(parts) >= 3 {
		versionStr := strings.TrimPrefix(parts[2], "go")
		versionParts := strings.SplitN(versionStr, ".", 3)
		if len(versionParts) >= 2 {
			major, _ := strconv.Atoi(versionParts[0])
			minor, _ := strconv.Atoi(versionParts[1])
			if major < 1 || (major == 1 && minor < 24) {
				return fmt.Errorf(
					"Go 1.24+ is required to install Engram, but found go%s.\n"+
						"Please update Go: https://go.dev/dl/", versionStr)
			}
		}
	}

	if osGetenv("GO111MODULE") == "off" {
		return fmt.Errorf("Go modules are disabled (GO111MODULE=off).\nRun: export GO111MODULE=on  # then retry")
	}

	return nil
}

// resolveEngramInstall returns the correct install command sequence for Engram per platform.
// - Homebrew: tap + scoped formula trust (when supported) + install
// - otherwise: returns an error — callers must use engram.DownloadLatestBinary() instead.
//
// Pre-built binaries are available at:
// https://github.com/Gentleman-Programming/engram/releases
func resolveEngramInstall(profile system.PlatformProfile) (CommandSequence, error) {
	switch profile.PackageManager {
	case "brew":
		// macOS: brew manages Go transitively — no preflight needed.
		return resolveHomebrewComponentInstall("engram", "install"), nil
	default:
		return nil, fmt.Errorf(
			"engram on %q/%q uses direct binary download — use engram.DownloadLatestBinary() instead of CommandSequence",
			profile.OS, profile.PackageManager,
		)
	}
}
