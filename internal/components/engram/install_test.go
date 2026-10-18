package engram

import (
	"reflect"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/system"
	"github.com/gentleman-programming/gentle-ai/v4/internal/testenv"
)

func TestInstallCommandByProfile(t *testing.T) {
	tests := []struct {
		name      string
		profile   system.PlatformProfile
		want      [][]string
		wantErr   bool
		brewTrust bool
	}{
		{
			name:      "darwin trusts formula before install",
			profile:   system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			want:      [][]string{{"brew", "tap", "Gentleman-Programming/homebrew-tap"}, {"brew", "trust", "--formula", "gentleman-programming/tap/engram"}, {"brew", "install", "engram"}},
			brewTrust: true,
		},
		{
			name:    "darwin uses brew tap and install",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			want:    [][]string{{"brew", "tap", "Gentleman-Programming/homebrew-tap"}, {"brew", "install", "engram"}},
		},
		// Without brew, InstallCommand returns an error so callers use
		// DownloadLatestBinary() instead.
		{
			name:    "darwin without brew returns error (uses DownloadLatestBinary)",
			profile: system.PlatformProfile{OS: "darwin"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.profile.PackageManager == "brew" {
				testenv.BrewProbe(t, tt.brewTrust)
			}
			command, err := InstallCommand(tt.profile)
			if (err != nil) != tt.wantErr {
				t.Fatalf("InstallCommand() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if !reflect.DeepEqual(command, tt.want) {
				t.Fatalf("InstallCommand() = %v, want %v", command, tt.want)
			}
		})
	}
}
