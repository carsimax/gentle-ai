package opencode

import (
	"os"
	"testing"
)

// mustSymlink creates a test symlink; any error is a test failure.
func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}
