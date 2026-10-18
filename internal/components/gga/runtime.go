package gga

import (
	"fmt"
	"path/filepath"

	"github.com/gentleman-programming/gentle-ai/v4/internal/assets"
	"github.com/gentleman-programming/gentle-ai/v4/internal/components/filemerge"
)

// RuntimeLibDir returns the runtime lib path used by gga.
func RuntimeLibDir(homeDir string) string {
	return filepath.Join(homeDir, ".local", "share", "gga", "lib")
}

// RuntimePRModePath returns the expected pr_mode.sh runtime path.
func RuntimePRModePath(homeDir string) string {
	return filepath.Join(RuntimeLibDir(homeDir), "pr_mode.sh")
}

// EnsureRuntimeAssets ensures critical gga runtime files are current.
//
// Behavior change from "only-if-missing" to "always-write":
// WriteFileAtomicMode performs a content-equality check: it never replaces the
// bytes when the embedded asset already matches the file on disk, and does an
// atomic replace when it differs. It always enforces the executable mode, so a
// script that lost its exec bit is repaired even when its content is current.
func EnsureRuntimeAssets(homeDir string) error {
	prModePath := RuntimePRModePath(homeDir)

	content, err := assets.Read("gga/pr_mode.sh")
	if err != nil {
		return fmt.Errorf("read embedded gga runtime asset pr_mode.sh: %w", err)
	}

	if _, err := filemerge.WriteFileAtomicMode(prModePath, []byte(content), 0o755); err != nil {
		return fmt.Errorf("write gga runtime file %q: %w", prModePath, err)
	}

	return nil
}
