package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/components/gga"
)

func TestInstallTestHomeIsolatesGGA(t *testing.T) {
	t.Run("isolated home", func(t *testing.T) {
		home := installTestHome(t)
		for _, path := range []string{gga.ConfigPath(home), gga.AgentsTemplatePath(home)} {
			rel, err := filepath.Rel(home, path)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				t.Errorf("GGA path %q escapes isolated home %q", path, home)
			}
		}
	})
}
