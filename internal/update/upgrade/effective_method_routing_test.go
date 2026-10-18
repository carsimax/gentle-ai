package upgrade

import (
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/system"
	"github.com/gentleman-programming/gentle-ai/v4/internal/update"
)

// TestEffectiveMethodPrecedenceIsUnchanged pins brew-ownership and
// declared-method precedence for gentle-ai self-upgrades.
func TestEffectiveMethodPrecedenceIsUnchanged(t *testing.T) {
	origHomebrewPackageInstalled := homebrewPackageInstalled
	t.Cleanup(func() { homebrewPackageInstalled = origHomebrewPackageInstalled })

	tests := []struct {
		name          string
		tool          update.ToolInfo
		profile       system.PlatformProfile
		brewInstalled bool
		want          update.InstallMethod
	}{
		{
			name:          "brew-owned package wins over go-install",
			tool:          update.ToolInfo{Name: "gentle-ai", InstallMethod: update.InstallBinary, GoImportPath: "github.com/gentleman-programming/gentle-ai/v3/cmd/gentle-ai"},
			profile:       system.PlatformProfile{OS: "darwin", PackageManager: "brew", GoAvailable: true},
			brewInstalled: true,
			want:          update.InstallBrew,
		},
		{
			name:    "no Go keeps the declared method",
			tool:    update.ToolInfo{Name: "gentle-ai", InstallMethod: update.InstallBinary, GoImportPath: "github.com/gentleman-programming/gentle-ai/v3/cmd/gentle-ai"},
			profile: system.PlatformProfile{OS: "darwin", GoAvailable: false},
			want:    update.InstallBinary,
		},
		{
			name:    "no import path keeps the declared method",
			tool:    update.ToolInfo{Name: "gentle-ai", InstallMethod: update.InstallBinary},
			profile: system.PlatformProfile{OS: "darwin", GoAvailable: true},
			want:    update.InstallBinary,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			homebrewPackageInstalled = func(string) bool { return tc.brewInstalled }
			if got := effectiveMethod(tc.tool, tc.profile); got != tc.want {
				t.Errorf("effectiveMethod(%q) = %q, want %q", tc.tool.Name, got, tc.want)
			}
		})
	}
}

// gentleAIImportPath is the module path gentle-ai publishes its command under.
// It is asserted against the registry below so the tests and the shipped
// declaration cannot drift apart. The /vN suffix is not stored here; it is
// derived at composition time via update.ModulePathForVersion.
const gentleAIImportPath = "github.com/gentleman-programming/gentle-ai/cmd/gentle-ai"

// registryGentleAI returns the shipped gentle-ai registry entry. Routing tests
// use the real declaration rather than a hand-built ToolInfo so a regression in
// registry.go cannot hide behind a synthetic fixture.
func registryGentleAI(t *testing.T) update.ToolInfo {
	t.Helper()
	for _, tool := range update.Tools {
		if tool.Name == "gentle-ai" {
			return tool
		}
	}
	t.Fatal("gentle-ai is missing from the tool registry")
	return update.ToolInfo{}
}

// TestRegistryDeclaresGentleAIGoImportPath pins the declaration that lets
// unanchored source builds self-upgrade. Without it, they have no import path
// to install from and fall back to the source-install refusal.
func TestRegistryDeclaresGentleAIGoImportPath(t *testing.T) {
	if got := registryGentleAI(t).GoImportPath; got != gentleAIImportPath {
		t.Fatalf("gentle-ai GoImportPath = %q, want %q", got, gentleAIImportPath)
	}
}

// TestGentleAIOnMacOSNeverRoutesToGoInstall: release builds must keep their
// minisign trust anchor even with Go on PATH.
func TestGentleAIOnMacOSNeverRoutesToGoInstall(t *testing.T) {
	useTestReleaseKey(t)
	origHomebrewPackageInstalled := homebrewPackageInstalled
	t.Cleanup(func() { homebrewPackageInstalled = origHomebrewPackageInstalled })
	homebrewPackageInstalled = func(string) bool { return false }

	for _, packageManager := range []string{"brew", ""} {
		profile := system.PlatformProfile{OS: "darwin", PackageManager: packageManager, GoAvailable: true}
		if got := effectiveMethod(registryGentleAI(t), profile); got != update.InstallBinary {
			t.Errorf("effectiveMethod on darwin/%s = %q, want %q (minisign-verified download)", packageManager, got, update.InstallBinary)
		}
	}
}
