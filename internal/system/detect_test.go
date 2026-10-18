package system

import (
	"slices"
	"testing"
)

func TestIsSupportedOS(t *testing.T) {
	tests := []struct {
		name string
		goos string
		want bool
	}{
		{name: "darwin is supported", goos: "darwin", want: true},
		{name: "linux is unsupported", goos: "linux", want: false},
		{name: "windows is unsupported", goos: "windows", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsSupportedOS(tc.goos)
			if got != tc.want {
				t.Fatalf("IsSupportedOS(%q) = %v, want %v", tc.goos, got, tc.want)
			}
		})
	}
}

func TestDetectFromInputsMarksSupportedMacOS(t *testing.T) {
	result := detectFromInputs("darwin", "arm64", "/bin/zsh", nil, nil)

	if !result.System.Supported {
		t.Fatalf("expected supported system for darwin")
	}

	if result.System.OS != "darwin" {
		t.Fatalf("expected OS darwin, got %q", result.System.OS)
	}

	if result.System.Profile.PackageManager != "brew" {
		t.Fatalf("expected brew package manager for macOS, got %q", result.System.Profile.PackageManager)
	}
	// System.Supported should mirror profile
	if result.System.Supported != result.System.Profile.Supported {
		t.Fatalf("System.Supported (%v) != Profile.Supported (%v)", result.System.Supported, result.System.Profile.Supported)
	}
}

func TestDetectFromInputsMarksOtherOSUnsupported(t *testing.T) {
	result := detectFromInputs("linux", "arm64", "/bin/bash", map[string]ToolStatus{"brew": {Name: "brew", Installed: true}}, nil)

	if result.System.Supported || result.System.Profile.Supported {
		t.Fatalf("Supported = %v/%v, want false", result.System.Supported, result.System.Profile.Supported)
	}
	if result.System.Profile.PackageManager != "" {
		t.Fatalf("PackageManager = %q, want empty", result.System.Profile.PackageManager)
	}
}

func TestDetectedToolNamesProbesOnlyMacOSPackageManagers(t *testing.T) {
	got := detectedToolNames()
	want := []string{"git", "curl", "node", "go", "brew", "nix"}
	if !slices.Equal(got, want) {
		t.Fatalf("detectedToolNames() = %v, want %v", got, want)
	}
}

// TestGoAvailableInPlatformProfile asserts that GoAvailable in PlatformProfile
// reflects whether `go` is present in the tools map. This is the detection signal
// used by effectiveMethod to implement the brew → go-install → binary auto-detect order.
func TestGoAvailableInPlatformProfile(t *testing.T) {
	tests := []struct {
		name        string
		tools       map[string]ToolStatus
		wantGoAvail bool
	}{
		{
			name:        "go in tools and installed → GoAvailable true",
			tools:       map[string]ToolStatus{"go": {Name: "go", Installed: true}},
			wantGoAvail: true,
		},
		{
			name:        "go in tools but not installed → GoAvailable false",
			tools:       map[string]ToolStatus{"go": {Name: "go", Installed: false}},
			wantGoAvail: false,
		},
		{
			name:        "go not in tools map → GoAvailable false",
			tools:       map[string]ToolStatus{"brew": {Name: "brew", Installed: true}},
			wantGoAvail: false,
		},
		{
			name:        "nil tools map → GoAvailable false",
			tools:       nil,
			wantGoAvail: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			profile := resolvePlatformProfile("darwin", tc.tools)
			if profile.GoAvailable != tc.wantGoAvail {
				t.Fatalf("GoAvailable = %v, want %v", profile.GoAvailable, tc.wantGoAvail)
			}
		})
	}
}

func TestDetectFromInputsShellDefaultsToUnknown(t *testing.T) {
	result := detectFromInputs("darwin", "arm64", "", nil, nil)
	if result.System.Shell != "unknown" {
		t.Fatalf("Shell = %q, want %q", result.System.Shell, "unknown")
	}
}
