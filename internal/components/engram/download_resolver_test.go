package engram

import (
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/system"
)

// TestInstallCommandNonBrewReturnsError verifies resolver.go only handles
// brew; the non-brew path is handled by DownloadLatestBinary.
func TestInstallCommandNonBrewReturnsError(t *testing.T) {
	profile := system.PlatformProfile{OS: "darwin"}
	if _, err := InstallCommand(profile); err == nil {
		t.Errorf("InstallCommand(%q) should return error for non-brew: expected error, got nil", profile.PackageManager)
	}
}

// TestInstallCommandBrewStillWorks verifies brew path is unchanged.
func TestInstallCommandBrewStillWorks(t *testing.T) {
	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew"}
	cmds, err := InstallCommand(profile)
	if err != nil {
		t.Fatalf("InstallCommand(brew) unexpected error = %v", err)
	}
	if len(cmds) == 0 {
		t.Fatal("InstallCommand(brew) returned empty CommandSequence")
	}
	// Must still use brew tap + brew install
	found := false
	for _, cmd := range cmds {
		for _, arg := range cmd {
			if strings.Contains(arg, "engram") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("InstallCommand(brew) commands don't reference engram: %v", cmds)
	}
}
