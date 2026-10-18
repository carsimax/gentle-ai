package installcmd

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/model"
	"github.com/gentleman-programming/gentle-ai/v4/internal/system"
)

func TestValidateGoForModuleInstall(t *testing.T) {
	tests := []struct {
		name        string
		lookPath    func(string) (string, error)
		goVersion   func() ([]byte, error)
		env         map[string]string
		wantErr     bool
		errContains string
	}{
		{
			name: "go not in PATH returns error mentioning Go 1.24+",
			lookPath: func(file string) (string, error) {
				return "", fmt.Errorf("not found")
			},
			goVersion:   func() ([]byte, error) { return nil, nil },
			env:         map[string]string{},
			wantErr:     true,
			errContains: "Go 1.24+",
		},
		{
			name: "go version below 1.24 returns error",
			lookPath: func(file string) (string, error) {
				return "/usr/bin/go", nil
			},
			goVersion:   func() ([]byte, error) { return []byte("go version go1.21.0 darwin/arm64"), nil },
			env:         map[string]string{},
			wantErr:     true,
			errContains: "Go 1.24+",
		},
		{
			name: "go version 1.23 returns error",
			lookPath: func(file string) (string, error) {
				return "/usr/bin/go", nil
			},
			goVersion:   func() ([]byte, error) { return []byte("go version go1.23.5 darwin/arm64"), nil },
			env:         map[string]string{},
			wantErr:     true,
			errContains: "Go 1.24+",
		},
		{
			name: "GO111MODULE=off returns error with export fix",
			lookPath: func(file string) (string, error) {
				return "/usr/bin/go", nil
			},
			goVersion:   func() ([]byte, error) { return []byte("go version go1.24.0 darwin/arm64"), nil },
			env:         map[string]string{"GO111MODULE": "off"},
			wantErr:     true,
			errContains: "export GO111MODULE=on",
		},
		{
			name: "go 1.24 without GO111MODULE=off succeeds",
			lookPath: func(file string) (string, error) {
				return "/usr/bin/go", nil
			},
			goVersion: func() ([]byte, error) { return []byte("go version go1.24.0 darwin/arm64"), nil },
			env:       map[string]string{},
			wantErr:   false,
		},
		{
			name: "go 1.25 succeeds",
			lookPath: func(file string) (string, error) {
				return "/usr/bin/go", nil
			},
			goVersion: func() ([]byte, error) { return []byte("go version go1.25.0 darwin/arm64"), nil },
			env:       map[string]string{},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origLookPath := cmdLookPath
			origGoVersion := cmdGoVersion
			origGetenv := osGetenv
			cmdLookPath = tt.lookPath
			cmdGoVersion = tt.goVersion
			osGetenv = func(key string) string { return tt.env[key] }
			t.Cleanup(func() {
				cmdLookPath = origLookPath
				cmdGoVersion = origGoVersion
				osGetenv = origGetenv
			})

			err := validateGoForModuleInstall()
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateGoForModuleInstall() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestResolveEngramBrewBypassesGoValidation(t *testing.T) {
	// On macOS, brew manages Go — validation must be skipped entirely.
	origLookPath := cmdLookPath
	cmdLookPath = func(file string) (string, error) {
		return "", fmt.Errorf("go not found")
	}
	t.Cleanup(func() { cmdLookPath = origLookPath })

	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew"}
	cmds, err := resolveEngramInstall(profile)
	if err != nil {
		t.Fatalf("resolveEngramInstall() unexpected error = %v", err)
	}
	if len(cmds) == 0 {
		t.Fatal("resolveEngramInstall() returned empty CommandSequence")
	}
}

func TestResolveDependencyInstall(t *testing.T) {
	r := NewResolver()

	tests := []struct {
		name    string
		profile system.PlatformProfile
		dep     string
		want    CommandSequence
		wantErr bool
	}{
		{
			name:    "darwin resolves brew command",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			dep:     "somepkg",
			want:    CommandSequence{{"brew", "install", "somepkg"}},
		},
		{
			name:    "non-brew package manager returns error",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "nix"},
			dep:     "somepkg",
			wantErr: true,
		},
		{
			name:    "empty dependency returns error",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			dep:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, err := r.ResolveDependencyInstall(tt.profile, tt.dep)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveDependencyInstall() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if !reflect.DeepEqual(command, tt.want) {
				t.Fatalf("ResolveDependencyInstall() = %v, want %v", command, tt.want)
			}
		})
	}
}

func TestResolveAgentInstall(t *testing.T) {
	r := NewResolver()

	tests := []struct {
		name    string
		profile system.PlatformProfile
		agent   model.AgentID
		want    CommandSequence
		wantErr bool
	}{
		{
			name:    "claude-code on darwin uses npm without sudo",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			agent:   model.AgentClaudeCode,
			want:    CommandSequence{{"npm", "install", "-g", "--ignore-scripts", "@anthropic-ai/claude-code@latest"}},
		},
		{
			name:    "opencode on darwin uses V2 npm package",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			agent:   model.AgentOpenCode,
			want:    CommandSequence{{"npm", "install", "-g", "@opencode/cli@latest"}},
		},
		{
			// A Linux profile the probe rejected (no manager on PATH) must keep
			// erroring: the default arm is gated on a non-empty PackageManager.
			name:    "opencode without package manager returns error",
			profile: system.PlatformProfile{OS: "freebsd", PackageManager: ""},
			agent:   model.AgentOpenCode,
			wantErr: true,
		},
		{
			name:    "kimi on macOS uses uv to strictly enforce secure package installation",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true},
			agent:   model.AgentKimi,
			want:    CommandSequence{{"uv", "tool", "install", "--python", "3.13", "kimi-cli"}},
		},
		{
			name:    "kimi on unsupported profile returns error",
			profile: system.PlatformProfile{OS: "freebsd", Supported: false},
			agent:   model.AgentKimi,
			wantErr: true,
		},

		{
			name:    "unsupported agent returns error",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			agent:   "unsupported",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, err := r.ResolveAgentInstall(tt.profile, tt.agent)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveAgentInstall() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if !reflect.DeepEqual(command, tt.want) {
				t.Fatalf("ResolveAgentInstall() = %v, want %v", command, tt.want)
			}
		})
	}
}

func TestValidateAgentInstallPreflight(t *testing.T) {
	tests := []struct {
		name        string
		profile     system.PlatformProfile
		agent       model.AgentID
		lookPath    func(string) (string, error)
		wantErr     bool
		errContains string
	}{
		{
			name:    "kimi on unsupported platform returns unsupported error before uv lookup",
			profile: system.PlatformProfile{OS: "freebsd", PackageManager: "", Supported: false},
			agent:   model.AgentKimi,
			lookPath: func(file string) (string, error) {
				return "", fmt.Errorf("should not be called")
			},
			wantErr:     true,
			errContains: "not supported on this platform",
		},
		{
			name:    "kimi missing uv returns actionable remediation",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true},
			agent:   model.AgentKimi,
			lookPath: func(file string) (string, error) {
				if file == "uv" {
					return "", fmt.Errorf("not found")
				}
				return "/usr/bin/" + file, nil
			},
			wantErr:     true,
			errContains: "brew install uv",
		},
		{
			name:    "kimi with uv present passes preflight",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true},
			agent:   model.AgentKimi,
			lookPath: func(file string) (string, error) {
				if file == "uv" {
					return "/usr/bin/uv", nil
				}
				return "", fmt.Errorf("not found")
			},
			wantErr: false,
		},
		{
			name:    "pi missing binary returns actionable remediation",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true},
			agent:   model.AgentPi,
			lookPath: func(file string) (string, error) {
				if file == "pi" {
					return "", fmt.Errorf("not found")
				}
				return "/usr/bin/" + file, nil
			},
			wantErr:     true,
			errContains: "Pi requires the `pi` executable",
		},
		{
			// Pi requires both `pi` and npm: InstallCommand always runs engramInitCommand()
			// which executes `pnpm dlx` or `npm exec` (both need Node.js/npm).
			name:    "pi with binary and npm present passes preflight",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true},
			agent:   model.AgentPi,
			lookPath: func(file string) (string, error) {
				if file == "pi" || file == "npm" {
					return "/usr/bin/" + file, nil
				}
				return "", fmt.Errorf("not found")
			},
			wantErr: false,
		},
		{
			// Pi npm gate: pi present but npm absent must fail with Node.js remediation.
			name:    "pi missing npm returns actionable remediation",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true},
			agent:   model.AgentPi,
			lookPath: func(file string) (string, error) {
				if file == "pi" {
					return "/usr/local/bin/pi", nil
				}
				return "", fmt.Errorf("not found")
			},
			wantErr:     true,
			errContains: "Node.js",
		},
		{
			// ClaudeCode does not require uv (that is Kimi-specific), but it does
			// require npm. This case verifies that npm being present is sufficient
			// for the preflight to pass — uv absence is irrelevant.
			name:    "non kimi npm agent does not require uv but does require npm (npm present)",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true},
			agent:   model.AgentClaudeCode,
			lookPath: func(file string) (string, error) {
				if file == "npm" {
					return "/usr/local/bin/npm", nil
				}
				return "", fmt.Errorf("not found")
			},
			wantErr: false,
		},
		{
			// Bug A regression: ClaudeCode with npm absent must fail with a clear,
			// actionable error (not proceed into the pipeline to surface a cryptic
			// "exec: npm: executable file not found in PATH" during agent install).
			name:    "claude-code missing npm returns actionable remediation",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true},
			agent:   model.AgentClaudeCode,
			lookPath: func(file string) (string, error) {
				return "", fmt.Errorf("not found")
			},
			wantErr:     true,
			errContains: "brew install node",
		},
		{
			// OpenCode also uses npm.
			name:    "opencode missing npm returns actionable remediation",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true},
			agent:   model.AgentOpenCode,
			lookPath: func(file string) (string, error) {
				return "", fmt.Errorf("not found")
			},
			wantErr:     true,
			errContains: "npm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			lookPath := tt.lookPath
			if lookPath == nil {
				lookPath = func(string) (string, error) { return "", fmt.Errorf("not found") }
			}
			wrappedLookPath := func(file string) (string, error) {
				calls++
				return lookPath(file)
			}
			origLookPath := cmdLookPath
			cmdLookPath = wrappedLookPath
			t.Cleanup(func() { cmdLookPath = origLookPath })

			err := ValidateAgentInstallPreflight(tt.profile, tt.agent)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateAgentInstallPreflight() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("ValidateAgentInstallPreflight() error = %q, want to contain %q", err.Error(), tt.errContains)
			}
			if tt.name == "kimi on unsupported platform returns unsupported error before uv lookup" && strings.Contains(strings.ToLower(err.Error()), "install uv") {
				t.Fatalf("ValidateAgentInstallPreflight() unsupported-platform error leaked uv remediation: %q", err.Error())
			}

			if tt.name == "kimi on unsupported platform returns unsupported error before uv lookup" && calls != 0 {
				t.Fatalf("ValidateAgentInstallPreflight() called uv lookup %d times on unsupported platform, want 0", calls)
			}
		})
	}
}

func TestResolveComponentInstall(t *testing.T) {
	// Keep the pre-trust command sequences deterministic on every host.
	original := cmdBrewHelpTrust
	cmdBrewHelpTrust = func(string) error { return fmt.Errorf("unknown command: trust") }
	t.Cleanup(func() { cmdBrewHelpTrust = original })
	r := NewResolver()

	tests := []struct {
		name      string
		profile   system.PlatformProfile
		component model.ComponentID
		want      CommandSequence
		wantErr   bool
	}{
		{
			name:      "engram on darwin uses brew tap and install",
			profile:   system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			component: model.ComponentEngram,
			want:      CommandSequence{{"brew", "tap", "Gentleman-Programming/homebrew-tap"}, {"brew", "install", "engram"}},
		},
		{
			name:      "gga on darwin uses brew tap and reinstall",
			profile:   system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			component: model.ComponentGGA,
			want:      CommandSequence{{"brew", "tap", "Gentleman-Programming/homebrew-tap"}, {"brew", "reinstall", "gga"}},
		},
		{
			name:      "gga without package manager returns error",
			profile:   system.PlatformProfile{OS: "freebsd", PackageManager: ""},
			component: model.ComponentGGA,
			wantErr:   true,
		},
		{
			name:      "unsupported component returns error",
			profile:   system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			component: "unsupported",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, err := r.ResolveComponentInstall(tt.profile, tt.component)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveComponentInstall() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if !reflect.DeepEqual(command, tt.want) {
				t.Fatalf("ResolveComponentInstall() = %v, want %v", command, tt.want)
			}
		})
	}
}

func TestUVInstallHint(t *testing.T) {
	tests := []struct {
		pm   string
		want string
	}{
		{
			pm:   "brew",
			want: "brew install uv",
		},
		{
			pm:   "unknown-pm",
			want: "https://docs.astral.sh/uv/getting-started/installation/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.pm, func(t *testing.T) {
			profile := system.PlatformProfile{OS: "darwin", PackageManager: tt.pm}
			got := uvInstallHint(profile)
			if got != tt.want {
				t.Fatalf("uvInstallHint(%q) = %q, want %q", tt.pm, got, tt.want)
			}
		})
	}
}
