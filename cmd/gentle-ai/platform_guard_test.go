package main

import (
	"strings"
	"testing"
)

func TestEnsureForkPlatformAcceptsDarwinArm64(t *testing.T) {
	if err := ensureForkPlatform("darwin", "arm64"); err != nil {
		t.Fatalf("ensureForkPlatform(darwin, arm64) error = %v, want nil", err)
	}
}

func TestEnsureForkPlatformRejectsOtherPlatforms(t *testing.T) {
	for _, tc := range []struct{ goos, goarch string }{
		{"darwin", "amd64"},
		{"linux", "arm64"},
		{"linux", "amd64"},
		{"windows", "amd64"},
	} {
		err := ensureForkPlatform(tc.goos, tc.goarch)
		if err == nil {
			t.Fatalf("ensureForkPlatform(%s, %s) error = nil, want unsupported platform", tc.goos, tc.goarch)
		}
		want := "unsupported platform " + tc.goos + "/" + tc.goarch + ": this build supports only macOS on Apple Silicon (darwin/arm64)"
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ensureForkPlatform(%s, %s) error = %q, want it to contain %q", tc.goos, tc.goarch, err, want)
		}
	}
}
