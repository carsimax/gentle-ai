package system

import (
	"errors"
	"strings"
	"testing"
)

func TestEnsureSupportedOSAllowsMacOS(t *testing.T) {
	if err := EnsureSupportedOS("darwin"); err != nil {
		t.Fatalf("expected no error for macOS, got %v", err)
	}
}

func TestEnsureSupportedOSRejectsUnsupported(t *testing.T) {
	for _, goos := range []string{"freebsd", "linux", "windows"} {
		err := EnsureSupportedOS(goos)
		if err == nil {
			t.Fatalf("expected error for unsupported OS %q", goos)
		}

		if !errors.Is(err, ErrUnsupportedOS) {
			t.Fatalf("expected ErrUnsupportedOS, got %v", err)
		}

		if !strings.Contains(err.Error(), "only macOS on Apple Silicon is supported") {
			t.Fatalf("expected explicit OS support message, got %q", err.Error())
		}
	}
}

func TestEnsureSupportedPlatformFollowsOS(t *testing.T) {
	if err := EnsureSupportedPlatform(PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true}); err != nil {
		t.Fatalf("expected darwin profile to be supported, got %v", err)
	}
	if err := EnsureSupportedPlatform(PlatformProfile{OS: "linux"}); !errors.Is(err, ErrUnsupportedOS) {
		t.Fatalf("expected ErrUnsupportedOS for linux profile, got %v", err)
	}
}
