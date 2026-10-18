package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/system"
)

func TestIsForkBuild(t *testing.T) {
	for version, want := range map[string]bool{
		"4.2.0-cmx.1.0.0":       true,
		"4.2.0-cmx.1.0.0.dirty": true,
		"4.2.0":                 false,
		"4.2.0-rc.1":            false,
		"dev":                   false,
		"":                      false,
	} {
		if got := IsForkBuild(version); got != want {
			t.Errorf("IsForkBuild(%q) = %t, want %t", version, got, want)
		}
	}
}

func TestCheckFiltered_ForkBuildNeverOffersUpstreamRelease(t *testing.T) {
	mockNoHomebrew(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(githubRelease{TagName: "v9.9.9"})
	}))
	defer server.Close()

	origClient := httpClient
	origLookPath := lookPath
	origExecCommand := execCommand
	origTools := Tools
	t.Cleanup(func() {
		httpClient = origClient
		lookPath = origLookPath
		execCommand = origExecCommand
		Tools = origTools
	})

	httpClient = server.Client()
	httpClient.Transport = &testTransport{server: server}
	lookPath = func(string) (string, error) { return "", fmt.Errorf("not found") }
	execCommand = func(name string, args ...string) *exec.Cmd { return mockCmd("false") }
	Tools = []ToolInfo{Tools[0]} // gentle-ai only

	profile := system.PlatformProfile{OS: "darwin", PackageManager: "brew", Supported: true}

	results := CheckFiltered(context.Background(), "4.2.0-cmx.1.0.0", profile, nil)
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
	if results[0].Status != DevBuild {
		t.Fatalf("fork build status = %q, want DevBuild so the upstream release is never offered", results[0].Status)
	}

	results = CheckFiltered(context.Background(), "4.2.0", profile, nil)
	if results[0].Status != UpdateAvailable {
		t.Fatalf("upstream build status = %q, want UpdateAvailable (unchanged behavior)", results[0].Status)
	}
}
