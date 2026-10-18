package system

import (
	"context"
	"os"
	"runtime"
)

type SystemInfo struct {
	OS        string
	Arch      string
	Shell     string
	Supported bool
	Profile   PlatformProfile
}

type PlatformProfile struct {
	OS             string
	PackageManager string
	GoAvailable    bool // true when `go` is found on PATH (used for auto-detect: brew → go-install → binary)
	Supported      bool
}

// detectedToolNames is the fixed probe list passed to DetectTools: the tools
// other detection reads, plus the package managers that exist on macOS.
func detectedToolNames() []string {
	return []string{"git", "curl", "node", "go", "brew", "nix"}
}

type DetectionResult struct {
	System       SystemInfo
	Tools        map[string]ToolStatus
	Configs      []ConfigState
	Dependencies DependencyReport
}

// IsSupportedOS reports whether goos is the only supported operating system:
// macOS (darwin).
func IsSupportedOS(goos string) bool {
	return goos == "darwin"
}

func Detect(ctx context.Context) (DetectionResult, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return DetectionResult{}, err
	}

	tools := DetectTools(ctx, detectedToolNames())
	configs := ScanConfigs(homeDir)

	result := detectFromInputs(runtime.GOOS, runtime.GOARCH, os.Getenv("SHELL"), tools, configs)
	result.Dependencies = DetectDependencies(ctx, result.System.Profile)

	return result, nil
}

func detectFromInputs(goos, arch, shell string, tools map[string]ToolStatus, configs []ConfigState) DetectionResult {
	if shell == "" {
		shell = "unknown"
	}

	profile := resolvePlatformProfile(goos, tools)

	return DetectionResult{
		System: SystemInfo{
			OS:        goos,
			Arch:      arch,
			Shell:     shell,
			Supported: profile.Supported,
			Profile:   profile,
		},
		Tools:   tools,
		Configs: configs,
	}
}

func resolvePlatformProfile(goos string, tools map[string]ToolStatus) PlatformProfile {
	profile := PlatformProfile{OS: goos}

	// Detect Go availability for the brew → go-install → binary auto-detect order.
	if go_, ok := tools["go"]; ok && go_.Installed {
		profile.GoAvailable = true
	}

	if goos == "darwin" {
		profile.PackageManager = "brew"
		profile.Supported = true
	}
	return profile
}
