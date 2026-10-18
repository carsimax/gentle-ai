package system

import (
	"os"
	"path/filepath"
	"strings"
)

// UserPathAddition records the PATH entries this invocation created.
type UserPathAddition struct {
	ProcessAdded bool
}

// AddToUserPath prepends dir to the current process PATH so subsequent
// commands in this run can find a newly installed binary immediately.
func AddToUserPath(dir string) error {
	_, err := AddToUserPathWithResult(dir)
	return err
}

// AddToUserPathWithResult adds dir and reports exactly which PATH scopes changed.
// Callers that must compensate a failed larger transaction can safely restore only
// entries created by this invocation.
func AddToUserPathWithResult(dir string) (UserPathAddition, error) {
	addition := UserPathAddition{ProcessAdded: !processPathContains(dir)}
	return addition, addToProcessPath(dir)
}

// RemoveFromUserPath removes one matching directory from the current process
// PATH. Matching trims whitespace and quotes and is case-insensitive, while
// all unrelated entries retain their original order.
func RemoveFromUserPath(dir string) error {
	return removeFromProcessPath(dir)
}

// RollbackUserPathAddition restores only PATH entries created by
// AddToUserPathWithResult.
func RollbackUserPathAddition(dir string, addition UserPathAddition) error {
	if addition.ProcessAdded {
		return removeFromProcessPath(dir)
	}
	return nil
}

// PrioritizeUserPath moves dir to the front of PATH for the current process.
// Existing entries are preserved; only exact matches for dir are removed
// before the refreshed dir is prepended.
func PrioritizeUserPath(dir string) error {
	dir = strings.Trim(strings.TrimSpace(dir), `"`)
	if dir == "" {
		return nil
	}
	return prioritizeProcessPath(dir)
}

// UserPathEntries returns the current process PATH entries.
func UserPathEntries() ([]string, error) {
	return filepath.SplitList(os.Getenv("PATH")), nil
}

// NewShellPath returns the PATH a newly started shell inherits: the current
// process PATH.
func NewShellPath() (string, error) {
	return os.Getenv("PATH"), nil
}

// addToProcessPath prepends dir to the current process PATH if it is not already
// present. This is a low-level helper called by AddToUserPath.
func addToProcessPath(dir string) error {
	currentPath := os.Getenv("PATH")

	// Already present in process PATH? Skip.
	for _, p := range filepath.SplitList(currentPath) {
		if strings.EqualFold(filepath.Clean(p), filepath.Clean(dir)) {
			return nil
		}
	}

	if currentPath == "" {
		return os.Setenv("PATH", dir)
	}
	return os.Setenv("PATH", dir+string(os.PathListSeparator)+currentPath)
}

func processPathContains(dir string) bool {
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.EqualFold(filepath.Clean(entry), filepath.Clean(dir)) {
			return true
		}
	}
	return false
}

func removeFromProcessPath(dir string) error {
	currentPath := os.Getenv("PATH")
	entries := filepath.SplitList(currentPath)
	for index, entry := range entries {
		if strings.EqualFold(filepath.Clean(strings.Trim(strings.TrimSpace(entry), `"`)), filepath.Clean(strings.Trim(strings.TrimSpace(dir), `"`))) {
			return os.Setenv("PATH", strings.Join(append(entries[:index], entries[index+1:]...), string(os.PathListSeparator)))
		}
	}
	return nil
}

func prioritizeProcessPath(dir string) error {
	currentPath := os.Getenv("PATH")
	if currentPath == "" {
		return os.Setenv("PATH", dir)
	}

	entries := []string{dir}
	for _, entry := range filepath.SplitList(currentPath) {
		entry = strings.TrimSpace(entry)
		if entry == "" || strings.EqualFold(filepath.Clean(entry), filepath.Clean(dir)) {
			continue
		}
		entries = append(entries, entry)
	}
	return os.Setenv("PATH", strings.Join(entries, string(os.PathListSeparator)))
}
