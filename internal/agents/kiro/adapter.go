package kiro

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gentleman-programming/gentle-ai/v4/internal/agents/capabilitymanifest"
	"github.com/gentleman-programming/gentle-ai/v4/internal/model"
	"github.com/gentleman-programming/gentle-ai/v4/internal/system"
)

type Adapter struct {
	lookPath func(string) (string, error)
	statPath func(string) (os.FileInfo, error)
}

func NewAdapter() *Adapter {
	return &Adapter{
		lookPath: exec.LookPath,
		statPath: os.Stat,
	}
}

// --- Identity ---

func (a *Adapter) Agent() model.AgentID {
	return model.AgentKiroIDE
}

func (a *Adapter) Tier() model.SupportTier {
	return model.TierFull
}

// --- Detection ---

func (a *Adapter) Detect(_ context.Context, homeDir string) (bool, string, string, bool, error) {
	// Kiro IDE is a VS Code fork available as a desktop application.
	// Official website: https://kiro.dev/
	// Detection uses two signals:
	//   1. "kiro" binary on PATH — primary indicator that Kiro is installed.
	//   2. ~/.kiro config dir — returned as configPath so callers/UI can
	//      show the managed directory and configFound reflects filesystem reality.
	//
	// Note: configPath is ~/.kiro (the home-based root where all managed
	// artifacts live), NOT the OS app-config dir used only for settings.json.
	configPath := filepath.Join(homeDir, ".kiro")
	info, statErr := a.statPath(configPath)
	configFound := statErr == nil && info.IsDir()

	binaryPath, err := a.lookPath("kiro")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return false, "", configPath, configFound, nil
		}
		// Unexpected error (permission / IO) — surface it so callers can distinguish.
		return false, "", configPath, false, err
	}

	return true, binaryPath, configPath, configFound, nil
}

// --- Installation ---

func (a *Adapter) CapabilityManifest() capabilitymanifest.AgentCapabilityManifest {
	return capabilitymanifest.MustForAgent(model.AgentKiroIDE)
}

func (a *Adapter) InstallCommand(_ system.PlatformProfile) ([][]string, error) {
	return nil, AgentNotInstallableError{Agent: model.AgentKiroIDE}
}

// --- Config paths ---
// Kiro IDE (VS Code fork) uses a split-root layout:
//   - Steering/skills/agents/MCP: ~/.kiro/ (home-based, all platforms)
//   - Settings:  ~/Library/Application Support/Kiro/User/
// Steering content is written to ~/.kiro/steering/gentle-ai.md via StrategySteeringFile.

func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".kiro")
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return filepath.Join(homeDir, ".kiro", "steering")
}

func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(a.SystemPromptDir(homeDir), "gentle-ai.md")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	// Skills are always stored in ~/.kiro/skills/. Kiro uses a split-root layout
	// where settings live in the OS app-config dir but the IDE reads skills,
	// steering, agents, and MCP from the home-based ~/.kiro/ root.
	return filepath.Join(homeDir, ".kiro", "skills")
}

func (a *Adapter) SettingsPath(homeDir string) string {
	// Kiro's OS app settings remain a secondary Gentle AI path; CodeGraph MCP
	// ownership is rooted independently under ~/.kiro/settings.
	return filepath.Join(a.kiroConfigDir(homeDir), "settings.json")
}

// --- Sub-agent support (Kiro native agents in ~/.kiro/agents/) ---

func (a *Adapter) SupportsSubAgents() bool {
	return a.CapabilityManifest().Features.FileSubAgents
}

func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(homeDir, ".kiro", "agents")
}

func (a *Adapter) EmbeddedSubAgentsDir() string {
	return "kiro/agents"
}

// KiroModelID resolves a KiroModelAlias to a Kiro-native model identifier.
// Used by the SDD injector to stamp the `model:` field in agent frontmatter.
func (a *Adapter) KiroModelID(alias model.KiroModelAlias) string {
	return model.KiroModelID(alias)
}

// --- Config strategies ---

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategySteeringFile
}

func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return model.StrategyMCPConfigFile
}

// --- MCP ---

// MCPConfigPath returns the user-level MCP config file.
// Kiro reads MCP configuration from ~/.kiro/settings/mcp.json (user level)
// or .kiro/settings/mcp.json (workspace level). This is separate from the
// app config dir used for settings.
func (a *Adapter) MCPConfigPath(homeDir string, _ string) string {
	return filepath.Join(homeDir, ".kiro", "settings", "mcp.json")
}

func (a *Adapter) kiroConfigDir(homeDir string) string {
	return filepath.Join(homeDir, "Library", "Application Support", "Kiro", "User")
}

// --- Optional capabilities ---

func (a *Adapter) SupportsOutputStyles() bool {
	return a.CapabilityManifest().Features.OutputStyles
}

func (a *Adapter) OutputStyleDir(_ string) string {
	return ""
}

func (a *Adapter) SupportsSlashCommands() bool {
	return a.CapabilityManifest().Features.SlashCommands
}

func (a *Adapter) CommandsDir(_ string) string {
	return ""
}

func (a *Adapter) SupportsSkills() bool {
	return a.CapabilityManifest().Features.Skills
}

func (a *Adapter) SupportsSystemPrompt() bool {
	return a.CapabilityManifest().Features.SystemPrompt
}

func (a *Adapter) SupportsMCP() bool {
	return a.CapabilityManifest().Features.MCP
}
