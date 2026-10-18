package system

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestAddToUserPathAlreadyPresent verifies that if the directory is already in PATH,
// AddToUserPath returns nil and does not duplicate it.
func TestAddToUserPathAlreadyPresent(t *testing.T) {
	// Set up a PATH that already contains the target dir.
	targetDir := filepath.Join(t.TempDir(), "already-present")
	original := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", original) })

	os.Setenv("PATH", targetDir+string(os.PathListSeparator)+original)

	err := AddToUserPath(targetDir)
	if err != nil {
		t.Fatalf("AddToUserPath returned unexpected error: %v", err)
	}

	// PATH should not have duplicates.
	currentPath := os.Getenv("PATH")
	count := 0
	for _, p := range filepath.SplitList(currentPath) {
		if strings.EqualFold(filepath.Clean(p), filepath.Clean(targetDir)) {
			count++
		}
	}
	if count > 1 {
		t.Fatalf("expected dir to appear at most once in PATH, got %d occurrences", count)
	}
}

// TestAddToProcessPathAddsToProcessEnv verifies the process-local PATH update
// helper.
func TestAddToProcessPathAddsToProcessEnv(t *testing.T) {
	targetDir := filepath.Join(t.TempDir(), "new-bin-dir")
	original := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", original) })

	// Ensure target is NOT currently in PATH.
	os.Setenv("PATH", strings.ReplaceAll(original, targetDir, ""))

	err := addToProcessPath(targetDir)
	if err != nil {
		t.Fatalf("addToProcessPath returned unexpected error: %v", err)
	}

	// The directory must now be in the process PATH.
	found := false
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.EqualFold(filepath.Clean(p), filepath.Clean(targetDir)) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %q to be present in process PATH after AddToUserPath, got: %s", targetDir, os.Getenv("PATH"))
	}
}

// TestAddToUserPathDoesNotError verifies that adding a new directory updates
// only the process PATH and never fails.
func TestAddToUserPathDoesNotError(t *testing.T) {
	targetDir := filepath.Join(t.TempDir(), "bin")
	original := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", original) })

	// Remove targetDir from PATH to force the add path.
	os.Setenv("PATH", strings.ReplaceAll(original, targetDir, ""))

	err := AddToUserPath(targetDir)
	if err != nil {
		t.Fatalf("AddToUserPath returned error: %v", err)
	}
}

func TestAddToUserPathUsesProcessPathInGoTests(t *testing.T) {
	targetDir := filepath.Join(t.TempDir(), "test-bin")
	original := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", original) })

	os.Setenv("PATH", strings.ReplaceAll(original, targetDir, ""))

	if err := AddToUserPath(targetDir); err != nil {
		t.Fatalf("AddToUserPath() error = %v", err)
	}

	entries := filepath.SplitList(os.Getenv("PATH"))
	if len(entries) == 0 || !strings.EqualFold(filepath.Clean(entries[0]), filepath.Clean(targetDir)) {
		t.Fatalf("PATH first entry = %q, want %q; full PATH=%q", entries, targetDir, os.Getenv("PATH"))
	}
}

func TestPrioritizeUserPathUsesProcessPathInGoTests(t *testing.T) {
	firstDir := filepath.Join(t.TempDir(), "first")
	targetDir := filepath.Join(t.TempDir(), "target")
	original := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", original) })

	os.Setenv("PATH", strings.Join([]string{firstDir, targetDir}, string(os.PathListSeparator)))

	if err := PrioritizeUserPath(targetDir); err != nil {
		t.Fatalf("PrioritizeUserPath() error = %v", err)
	}

	entries := filepath.SplitList(os.Getenv("PATH"))
	if len(entries) == 0 || !strings.EqualFold(filepath.Clean(entries[0]), filepath.Clean(targetDir)) {
		t.Fatalf("PATH first entry = %q, want %q; full PATH=%q", entries, targetDir, os.Getenv("PATH"))
	}
}

func TestPrioritizeProcessPathMovesExistingEntryToFront(t *testing.T) {
	firstDir := filepath.Join(t.TempDir(), "first")
	targetDir := filepath.Join(t.TempDir(), "target")
	lastDir := filepath.Join(t.TempDir(), "last")
	original := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", original) })

	os.Setenv("PATH", strings.Join([]string{firstDir, targetDir, lastDir}, string(os.PathListSeparator)))

	if err := prioritizeProcessPath(targetDir); err != nil {
		t.Fatalf("prioritizeProcessPath() error = %v", err)
	}

	entries := filepath.SplitList(os.Getenv("PATH"))
	if len(entries) != 3 {
		t.Fatalf("PATH entries = %v, want three preserved entries", entries)
	}
	if entries[0] != targetDir {
		t.Fatalf("PATH first entry = %q, want %q", entries[0], targetDir)
	}
	if entries[1] != firstDir || entries[2] != lastDir {
		t.Fatalf("PATH should preserve non-target order after target move, got %v", entries)
	}
}

func TestRemoveFromUserPathRemovesOneProcessEntry(t *testing.T) {
	firstDir := filepath.Join(t.TempDir(), "first")
	targetDir := filepath.Join(t.TempDir(), "target")
	lastDir := filepath.Join(t.TempDir(), "last")
	original := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", original) })
	os.Setenv("PATH", strings.Join([]string{firstDir, targetDir, targetDir, lastDir}, string(os.PathListSeparator)))

	if err := RemoveFromUserPath(targetDir); err != nil {
		t.Fatalf("RemoveFromUserPath() error = %v", err)
	}
	entries := filepath.SplitList(os.Getenv("PATH"))
	if got, want := strings.Join(entries, ","), strings.Join([]string{firstDir, targetDir, lastDir}, ","); got != want {
		t.Fatalf("PATH = %q, want %q", got, want)
	}
}

func TestAddToUserPathWithResultRollbackRemovesOnlyOwnedEntry(t *testing.T) {
	targetDir := filepath.Join(t.TempDir(), "bin")
	otherDir := filepath.Join(t.TempDir(), "tools")

	t.Setenv("PATH", otherDir)
	addition, err := AddToUserPathWithResult(targetDir)
	if err != nil || !addition.ProcessAdded {
		t.Fatalf("AddToUserPathWithResult() = %+v, %v; want ProcessAdded", addition, err)
	}
	if err := RollbackUserPathAddition(targetDir, addition); err != nil {
		t.Fatalf("RollbackUserPathAddition() error = %v", err)
	}
	if got, want := filepath.SplitList(os.Getenv("PATH")), []string{otherDir}; !slices.Equal(got, want) {
		t.Fatalf("PATH entries = %v, want %v", got, want)
	}

	present := strings.Join([]string{targetDir, otherDir}, string(os.PathListSeparator))
	t.Setenv("PATH", present)
	addition, err = AddToUserPathWithResult(targetDir)
	if err != nil || addition.ProcessAdded {
		t.Fatalf("AddToUserPathWithResult(present) = %+v, %v; want no owned mutation", addition, err)
	}
	if err := RollbackUserPathAddition(targetDir, addition); err != nil {
		t.Fatalf("RollbackUserPathAddition() error = %v", err)
	}
	if got := os.Getenv("PATH"); got != present {
		t.Fatalf("PATH = %q, want pre-existing entry preserved as %q", got, present)
	}
}
