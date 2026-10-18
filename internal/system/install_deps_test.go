package system

import (
	"strings"
	"testing"
)

func TestInstallHintGitDarwin(t *testing.T) {
	profile := PlatformProfile{OS: "darwin", PackageManager: "brew"}
	hint := installHintGit(profile)
	if hint != "brew install git" {
		t.Fatalf("installHintGit(darwin) = %q", hint)
	}
}

func TestInstallHintNodeDarwin(t *testing.T) {
	profile := PlatformProfile{OS: "darwin", PackageManager: "brew"}
	hint := installHintNode(profile)
	if hint != "brew install node" {
		t.Fatalf("installHintNode(darwin) = %q", hint)
	}
}

func TestInstallHintBrew(t *testing.T) {
	hint := installHintBrew()
	if !strings.Contains(hint, "Homebrew") {
		t.Fatalf("installHintBrew() = %q, want Homebrew URL", hint)
	}
}

func TestInstallHintGoDarwin(t *testing.T) {
	profile := PlatformProfile{OS: "darwin", PackageManager: "brew"}
	hint := installHintGo(profile)
	if hint != "brew install go" {
		t.Fatalf("installHintGo(darwin) = %q", hint)
	}
}

func TestInstallCommandsForDepGitDarwin(t *testing.T) {
	profile := PlatformProfile{OS: "darwin", PackageManager: "brew"}
	cmds := InstallCommandsForDep("git", profile)
	if len(cmds) != 1 {
		t.Fatalf("git darwin commands = %d, want 1", len(cmds))
	}
	if cmds[0][0] != "brew" || cmds[0][1] != "install" || cmds[0][2] != "git" {
		t.Fatalf("git darwin command = %v", cmds[0])
	}
}

func TestInstallCommandsForDepNpmReturnsNil(t *testing.T) {
	profile := PlatformProfile{OS: "darwin", PackageManager: "brew"}
	cmds := InstallCommandsForDep("npm", profile)
	if cmds != nil {
		t.Fatalf("npm commands = %v, want nil (comes with node)", cmds)
	}
}

func TestInstallCommandsForDepBrewOnLinuxReturnsNil(t *testing.T) {
	profile := PlatformProfile{OS: "linux"}
	cmds := InstallCommandsForDep("brew", profile)
	if cmds != nil {
		t.Fatalf("brew on linux = %v, want nil", cmds)
	}
}

func TestInstallCommandsForDepBrewOnDarwin(t *testing.T) {
	profile := PlatformProfile{OS: "darwin", PackageManager: "brew"}
	cmds := InstallCommandsForDep("brew", profile)
	if len(cmds) != 1 {
		t.Fatalf("brew darwin commands = %d, want 1", len(cmds))
	}
}

func TestInstallCommandsForDepUnknownReturnsNil(t *testing.T) {
	profile := PlatformProfile{OS: "darwin", PackageManager: "brew"}
	cmds := InstallCommandsForDep("unknown_tool", profile)
	if cmds != nil {
		t.Fatalf("unknown tool commands = %v, want nil", cmds)
	}
}

func TestFormatMissingDepsMessageAllPresent(t *testing.T) {
	report := DependencyReport{AllPresent: true}
	msg := FormatMissingDepsMessage(report)
	if !strings.Contains(msg, "All required dependencies are present") {
		t.Fatalf("unexpected message for all present: %q", msg)
	}
}

func TestFormatMissingDepsMessageWithMissing(t *testing.T) {
	report := DependencyReport{
		Dependencies: []Dependency{
			{Name: "node", Required: true, Installed: false, InstallHint: "brew install node"},
		},
		AllPresent:      false,
		MissingRequired: []string{"node"},
	}

	msg := FormatMissingDepsMessage(report)
	if !strings.Contains(msg, "node") || !strings.Contains(msg, "brew install node") {
		t.Fatalf("missing deps message = %q", msg)
	}
}

func TestInstallCommandsFullMatrix(t *testing.T) {
	profiles := []PlatformProfile{
		{OS: "darwin", PackageManager: "brew"},
	}

	deps := []string{"git", "curl", "node", "go"}

	for _, profile := range profiles {
		for _, dep := range deps {
			t.Run(profile.OS+"/"+profile.PackageManager+"/"+dep, func(t *testing.T) {
				cmds := InstallCommandsForDep(dep, profile)
				if cmds == nil {
					t.Fatalf("InstallCommandsForDep(%q, %s/%s) = nil", dep, profile.OS, profile.PackageManager)
				}
				if len(cmds) == 0 {
					t.Fatalf("InstallCommandsForDep(%q) returned empty slice", dep)
				}
				for _, cmd := range cmds {
					if len(cmd) == 0 {
						t.Fatalf("empty command in sequence for %q", dep)
					}
				}
			})
		}
	}
}
