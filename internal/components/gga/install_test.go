package gga

import (
	"os"
	"reflect"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/system"
	"github.com/gentleman-programming/gentle-ai/v4/internal/testenv"
)

func TestMain(m *testing.M) {
	testenv.RunBrewProbeFixture()
	os.Exit(m.Run())
}

func TestInstallCommandByProfile(t *testing.T) {
	tests := []struct {
		name      string
		profile   system.PlatformProfile
		want      [][]string
		wantErr   bool
		brewTrust bool
	}{
		{
			name:    "non-brew profile returns error",
			profile: system.PlatformProfile{OS: "freebsd"},
			wantErr: true,
		},
		{
			name:      "darwin trusts formula before reinstall",
			profile:   system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			want:      [][]string{{"brew", "tap", "Gentleman-Programming/homebrew-tap"}, {"brew", "trust", "--formula", "gentleman-programming/tap/gga"}, {"brew", "reinstall", "gga"}},
			brewTrust: true,
		},
		{
			name:    "darwin uses brew tap and reinstall",
			profile: system.PlatformProfile{OS: "darwin", PackageManager: "brew"},
			want:    [][]string{{"brew", "tap", "Gentleman-Programming/homebrew-tap"}, {"brew", "reinstall", "gga"}},
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

func TestShouldInstall(t *testing.T) {
	if !ShouldInstall(true) {
		t.Fatalf("ShouldInstall(true) = false")
	}

	if ShouldInstall(false) {
		t.Fatalf("ShouldInstall(false) = true")
	}
}
