package engram

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/testenv"
)

func TestMain(m *testing.M) {
	testenv.RunBrewProbeFixture()
	os.Exit(m.Run())
}

// --- test helpers ---

// sha256Hex returns the SHA256 hex digest of data.
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// makeChecksumsTxt returns a BSD-style checksums.txt entry for the given filename and data.
func makeChecksumsTxt(filename string, data []byte) string {
	return fmt.Sprintf("%s  %s\n", sha256Hex(data), filename)
}

// makeServerWithFakeTarGz returns an httptest.Server that serves:
//   - GET /releases/latest       → GitHub API JSON with the given version
//   - GET /releases/download/…   → a real .tar.gz containing "engram" binary
//   - GET /…/checksums.txt       → a valid checksums.txt for the darwin/arm64 archive
func makeServerWithFakeTarGz(t *testing.T, version string) *httptest.Server {
	t.Helper()
	tarContent := buildFakeTarGz(t, "engram")
	checksums := makeChecksumsTxt(engramArchiveName(version), tarContent)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "releases/latest"):
			payload := map[string]string{"tag_name": "v" + version}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(payload)
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, checksums)
		default:
			// Binary asset
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			w.Write(tarContent)
		}
	}))
}

func buildFakeTarGz(t *testing.T, binaryName string) []byte {
	t.Helper()
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "release.tar.gz")

	f, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("create tar.gz: %v", err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	content := []byte("#!/bin/sh\necho engram fake binary")
	tw.WriteHeader(&tar.Header{Name: binaryName, Mode: 0o755, Size: int64(len(content))})
	tw.Write(content)
	tw.Close()
	gw.Close()
	f.Close()

	data, err := os.ReadFile(tarPath)
	if err != nil {
		t.Fatalf("read tar.gz: %v", err)
	}
	return data
}

// --- TestEngramAssetURL ---

func TestEngramAssetURL(t *testing.T) {
	url := engramAssetURL("https://github.com", "1.2.3")
	want := "https://github.com/Gentleman-Programming/engram/releases/download/v1.2.3/engram_1.2.3_darwin_arm64.tar.gz"
	if url != want {
		t.Errorf("engramAssetURL() = %q, want %q", url, want)
	}
}

// --- TestEngramInstallDir ---

func TestEngramInstallDir(t *testing.T) {
	dir := engramInstallDir()
	if !strings.HasSuffix(dir, "bin") {
		t.Errorf("engramInstallDir() = %q, want a bin directory", dir)
	}
}

// --- TestDownloadLatestBinary ---

func TestDownloadLatestBinary(t *testing.T) {
	server := makeServerWithFakeTarGz(t, "1.3.0")
	defer server.Close()

	// Override the HTTP client and the base URL for GitHub API.
	origClient := engramHTTPClient
	origBaseURL := engramGitHubBaseURL
	engramHTTPClient = server.Client()
	engramGitHubBaseURL = server.URL
	t.Cleanup(func() {
		engramHTTPClient = origClient
		engramGitHubBaseURL = origBaseURL
	})

	// Override install dir to a temp directory (avoids needing root).
	tmpDir := t.TempDir()
	origInstallDirFn := engramInstallDirFn
	engramInstallDirFn = func() string { return tmpDir }
	t.Cleanup(func() { engramInstallDirFn = origInstallDirFn })

	installedPath, err := DownloadLatestBinary(false)
	if err != nil {
		t.Fatalf("DownloadLatestBinary() error = %v", err)
	}

	// The installed path must be inside the temp dir.
	if installedPath != filepath.Join(tmpDir, "engram") {
		t.Errorf("installedPath = %q, want %q", installedPath, filepath.Join(tmpDir, "engram"))
	}

	// The binary must exist and be executable.
	info, err := os.Stat(installedPath)
	if err != nil {
		t.Fatalf("stat installed binary: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("installed binary is empty")
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("installed binary is not executable")
	}
}

// --- TestDownloadLatestBinaryAPIError ---

func TestDownloadLatestBinaryDownloadError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	origClient := engramHTTPClient
	origBaseURL := engramGitHubBaseURL
	engramHTTPClient = server.Client()
	engramGitHubBaseURL = server.URL
	t.Cleanup(func() {
		engramHTTPClient = origClient
		engramGitHubBaseURL = origBaseURL
	})

	_, err := DownloadLatestBinary(false)
	if err == nil {
		t.Fatal("expected error when GitHub API returns 500, got nil")
	}
}

func TestDownloadLatestBinarySkipsLatestReleaseWithoutBinaryAssets(t *testing.T) {
	const binaryVersion = "1.15.13"

	tarContent := buildFakeTarGz(t, "engram")
	checksums := makeChecksumsTxt(engramArchiveName(binaryVersion), tarContent)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "releases/latest"):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "pi-v0.1.7",
				"assets":   []any{},
			})
		case strings.Contains(r.URL.Path, "releases") && !strings.Contains(r.URL.Path, "releases/latest") &&
			!strings.Contains(r.URL.Path, "/releases/download") &&
			r.URL.Query().Get("per_page") == "20":
			w.Header().Set("Content-Type", "application/json")
			// Pages beyond 1 return empty to signal end-of-list.
			if r.URL.Query().Get("page") != "1" {
				json.NewEncoder(w).Encode([]map[string]any{})
				return
			}
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"tag_name":   "pi-v0.1.7",
					"draft":      false,
					"prerelease": false,
					"assets":     []any{},
				},
				{
					"tag_name":   "v" + binaryVersion,
					"draft":      false,
					"prerelease": false,
					"assets": []map[string]string{
						{"name": "checksums.txt"},
						{"name": "engram_" + binaryVersion + "_darwin_arm64.tar.gz"},
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, checksums)
		case strings.Contains(r.URL.Path, "/releases/download/v"+binaryVersion+"/engram_"+binaryVersion+"_darwin_"):
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			w.Write(tarContent)
		default:
			t.Fatalf("unexpected request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
	}))
	defer server.Close()

	origClient := engramHTTPClient
	origBaseURL := engramGitHubBaseURL
	engramHTTPClient = server.Client()
	engramGitHubBaseURL = server.URL
	t.Cleanup(func() {
		engramHTTPClient = origClient
		engramGitHubBaseURL = origBaseURL
	})

	tmpDir := t.TempDir()
	origInstallDirFn := engramInstallDirFn
	engramInstallDirFn = func() string { return tmpDir }
	t.Cleanup(func() { engramInstallDirFn = origInstallDirFn })

	installedPath, err := DownloadLatestBinary(false)
	if err != nil {
		t.Fatalf("DownloadLatestBinary() error = %v", err)
	}

	if _, err := os.Stat(installedPath); err != nil {
		t.Fatalf("stat installed binary: %v", err)
	}
}

func TestDownloadLatestBinaryReleaseListFallsBackToAnonymousWhenTokenGets403(t *testing.T) {
	const fakeToken = "ci-token"
	const binaryVersion = "1.15.13"

	tarContent := buildFakeTarGz(t, "engram")
	checksums := makeChecksumsTxt(engramArchiveName(binaryVersion), tarContent)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		switch {
		case strings.Contains(r.URL.Path, "releases/latest"):
			if auth != "" {
				// Simulate 403 when authenticated to trigger anonymous retry.
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"tag_name": "v" + binaryVersion})
		case strings.Contains(r.URL.Path, "releases") && !strings.Contains(r.URL.Path, "releases/latest") &&
			!strings.Contains(r.URL.Path, "/releases/download") &&
			r.URL.Query().Get("per_page") == "20":
			if auth != "" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			// Pages beyond 1 return empty to signal end-of-list.
			if r.URL.Query().Get("page") != "1" {
				json.NewEncoder(w).Encode([]map[string]any{})
				return
			}
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"tag_name":   "v" + binaryVersion,
					"draft":      false,
					"prerelease": false,
					"assets": []map[string]string{
						{"name": "engram_" + binaryVersion + "_darwin_arm64.tar.gz"},
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, checksums)
		case strings.Contains(r.URL.Path, "/releases/download/v"+binaryVersion+"/engram_"+binaryVersion+"_darwin_"):
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			w.Write(tarContent)
		default:
			t.Fatalf("unexpected request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
	}))
	defer server.Close()

	origClient := engramHTTPClient
	origBaseURL := engramGitHubBaseURL
	engramHTTPClient = server.Client()
	engramGitHubBaseURL = server.URL
	t.Cleanup(func() {
		engramHTTPClient = origClient
		engramGitHubBaseURL = origBaseURL
	})

	t.Setenv("GITHUB_TOKEN", fakeToken)
	t.Setenv("GH_TOKEN", "")

	tmpDir := t.TempDir()
	origInstallDirFn := engramInstallDirFn
	engramInstallDirFn = func() string { return tmpDir }
	t.Cleanup(func() { engramInstallDirFn = origInstallDirFn })

	installedPath, err := DownloadLatestBinary(false)
	if err != nil {
		t.Fatalf("DownloadLatestBinary() error = %v", err)
	}

	if _, err := os.Stat(installedPath); err != nil {
		t.Fatalf("stat installed binary: %v", err)
	}
}

// TestDownloadLatestBinaryIgnoresGentleEngramAndPiTags asserts the CORRECTNESS
// CRUX: when the release list contains a mix of a core engram tag (vX.Y.Z), a
// gentle-engram tag, and a pi-v* tag, DownloadLatestBinary MUST pick the core
// engram version and MUST NOT pick the gentle-engram or pi-* tag.
func TestDownloadLatestBinaryIgnoresGentleEngramAndPiTags(t *testing.T) {
	const binaryVersion = "1.16.3"

	tarContent := buildFakeTarGz(t, "engram")
	checksums := makeChecksumsTxt(engramArchiveName(binaryVersion), tarContent)

	// The release list intentionally lists gentle-engram first (highest position)
	// and pi-v* second — both without binary assets — followed by the real core
	// engram release. The download MUST skip the first two and pick the third.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "releases/latest"):
			// /latest points at a gentle-engram tag — the non-core tag that must be skipped.
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "gentle-engram v0.1.8",
				"assets":   []any{},
			})
		case strings.Contains(r.URL.Path, "releases") && !strings.Contains(r.URL.Path, "releases/latest") &&
			!strings.Contains(r.URL.Path, "/releases/download") &&
			r.URL.Query().Get("per_page") == "20":
			w.Header().Set("Content-Type", "application/json")
			// Pages beyond 1 return empty to signal end-of-list.
			if r.URL.Query().Get("page") != "1" {
				json.NewEncoder(w).Encode([]map[string]any{})
				return
			}
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"tag_name":   "gentle-engram v0.1.8",
					"draft":      false,
					"prerelease": false,
					"assets":     []any{},
				},
				{
					"tag_name":   "pi-v0.1.7",
					"draft":      false,
					"prerelease": false,
					"assets":     []any{},
				},
				{
					"tag_name":   "v" + binaryVersion,
					"draft":      false,
					"prerelease": false,
					"assets": []map[string]string{
						{"name": "checksums.txt"},
						{"name": "engram_" + binaryVersion + "_darwin_arm64.tar.gz"},
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, checksums)
		case strings.Contains(r.URL.Path, "/releases/download/v"+binaryVersion+"/engram_"+binaryVersion+"_darwin_"):
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			w.Write(tarContent)
		default:
			t.Fatalf("unexpected request path (should be core engram v%s, not gentle-engram/pi): %s?%s",
				binaryVersion, r.URL.Path, r.URL.RawQuery)
		}
	}))
	defer server.Close()

	origClient := engramHTTPClient
	origBaseURL := engramGitHubBaseURL
	engramHTTPClient = server.Client()
	engramGitHubBaseURL = server.URL
	t.Cleanup(func() {
		engramHTTPClient = origClient
		engramGitHubBaseURL = origBaseURL
	})

	tmpDir := t.TempDir()
	origInstallDirFn := engramInstallDirFn
	engramInstallDirFn = func() string { return tmpDir }
	t.Cleanup(func() { engramInstallDirFn = origInstallDirFn })

	installedPath, err := DownloadLatestBinary(false)
	if err != nil {
		t.Fatalf("DownloadLatestBinary() error = %v, want core engram v%s to be selected", err, binaryVersion)
	}

	if !strings.HasPrefix(installedPath, tmpDir) {
		t.Errorf("installedPath = %q, want prefix %q", installedPath, tmpDir)
	}
	if _, err := os.Stat(installedPath); err != nil {
		t.Fatalf("stat installed binary: %v", err)
	}
}

// --- TestEngramChecksumVerification ---
//
// Table-driven tests covering all checksum verification paths:
//
//   - success: valid checksums.txt with correct digest → install succeeds
//   - missing checksums.txt: server returns 404 → fail closed
//   - digest mismatch: checksums.txt lists wrong digest → fail closed
//   - malformed checksums.txt: content has no parseable entries → fail closed
func TestEngramChecksumVerification(t *testing.T) {
	version := "1.3.0"

	// tarContent is a real .tar.gz archive used across sub-tests.
	tarContent := buildFakeTarGz(t, "engram")
	correctDigest := sha256Hex(tarContent)
	archiveName := engramArchiveName(version)

	tests := []struct {
		name          string
		checksumBody  string // content served at /…/checksums.txt; empty = serve 404
		checksumCode  int    // HTTP status for checksums.txt (0 → use 200 when body set)
		wantErrSubstr string // expected substring in error; empty = success
	}{
		{
			name:         "success: valid checksum passes",
			checksumBody: fmt.Sprintf("%s  %s\n", correctDigest, archiveName),
		},
		{
			name:          "missing checksums.txt: 404 fails closed",
			checksumCode:  http.StatusNotFound,
			wantErrSubstr: "checksums.txt unavailable",
		},
		{
			name:          "digest mismatch: wrong hash fails closed",
			checksumBody:  fmt.Sprintf("%s  %s\n", strings.Repeat("a", 64), archiveName),
			wantErrSubstr: "checksum mismatch",
		},
		{
			name:          "malformed checksums.txt: no matching entry fails closed",
			checksumBody:  "thisisnotavalidchecksumline\n",
			wantErrSubstr: "not listed in checksums.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.Contains(r.URL.Path, "releases/latest"):
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]string{"tag_name": "v" + version})

				case strings.HasSuffix(r.URL.Path, "checksums.txt"):
					code := tt.checksumCode
					if code == 0 {
						code = http.StatusOK
					}
					w.WriteHeader(code)
					if tt.checksumBody != "" {
						fmt.Fprint(w, tt.checksumBody)
					}

				default:
					w.Header().Set("Content-Type", "application/octet-stream")
					w.WriteHeader(http.StatusOK)
					w.Write(tarContent)
				}
			}))
			defer server.Close()

			origClient := engramHTTPClient
			origBaseURL := engramGitHubBaseURL
			engramHTTPClient = server.Client()
			engramGitHubBaseURL = server.URL
			t.Cleanup(func() {
				engramHTTPClient = origClient
				engramGitHubBaseURL = origBaseURL
			})

			tmpDir := t.TempDir()
			origInstallDirFn := engramInstallDirFn
			engramInstallDirFn = func() string { return tmpDir }
			t.Cleanup(func() { engramInstallDirFn = origInstallDirFn })

			_, err := DownloadLatestBinary(false)

			if tt.wantErrSubstr == "" {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErrSubstr)
			}
			if !strings.Contains(err.Error(), tt.wantErrSubstr) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErrSubstr)
			}
		})
	}
}

// TestFetchLatestEngramVersionWithAssetsPaginates asserts that
// fetchLatestEngramVersionWithAssets paginates beyond the first page when
// page 1 contains only non-core tags (pi-v* / gentle-engram) and the valid
// core vX.Y.Z release with a binary asset is on page 2.
//
// Regression test for the issue where per_page=20 capped discovery and returned
// "no engram release with downloadable binary assets found" even though a valid
// core release existed just beyond the window.
func TestFetchLatestEngramVersionWithAssetsPaginates(t *testing.T) {
	const binaryVersion = "1.17.0"

	tarContent := buildFakeTarGz(t, "engram")
	checksums := makeChecksumsTxt(engramArchiveName(binaryVersion), tarContent)

	// Track which pages were requested so we can assert pagination occurred.
	var pagesRequested []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "releases/latest"):
			// /latest points at a pi-v* tag so the single-release path falls
			// through to fetchLatestEngramVersionWithAssets.
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "pi-v0.2.0",
				"assets":   []any{},
			})

		case strings.Contains(r.URL.Path, "/repos/") && strings.Contains(r.URL.Path, "/releases") &&
			!strings.Contains(r.URL.Path, "/releases/download") &&
			!strings.Contains(r.URL.Path, "releases/latest"):
			// Release list endpoint — track the page parameter.
			page := r.URL.Query().Get("page")
			if page == "" {
				page = "1"
			}
			pagesRequested = append(pagesRequested, page)

			w.Header().Set("Content-Type", "application/json")
			switch page {
			case "1":
				// Page 1: only non-core tags — must NOT pick any of these.
				json.NewEncoder(w).Encode([]map[string]any{
					{
						"tag_name":   "pi-v0.2.0",
						"draft":      false,
						"prerelease": false,
						"assets":     []any{},
					},
					{
						"tag_name":   "gentle-engram v0.2.0",
						"draft":      false,
						"prerelease": false,
						"assets":     []any{},
					},
					{
						"tag_name":   "pi-v0.1.9",
						"draft":      false,
						"prerelease": false,
						"assets":     []any{},
					},
				})
			case "2":
				// Page 2: the real core engram release.
				json.NewEncoder(w).Encode([]map[string]any{
					{
						"tag_name":   "v" + binaryVersion,
						"draft":      false,
						"prerelease": false,
						"assets": []map[string]string{
							{"name": "checksums.txt"},
							{"name": "engram_" + binaryVersion + "_darwin_arm64.tar.gz"},
						},
					},
				})
			default:
				// Any further page is empty — signals end of releases.
				json.NewEncoder(w).Encode([]map[string]any{})
			}

		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, checksums)

		case strings.Contains(r.URL.Path, "/releases/download/v"+binaryVersion+"/engram_"+binaryVersion+"_darwin_"):
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			w.Write(tarContent)

		default:
			t.Fatalf("unexpected request: %s?%s", r.URL.Path, r.URL.RawQuery)
		}
	}))
	defer server.Close()

	origClient := engramHTTPClient
	origBaseURL := engramGitHubBaseURL
	engramHTTPClient = server.Client()
	engramGitHubBaseURL = server.URL
	t.Cleanup(func() {
		engramHTTPClient = origClient
		engramGitHubBaseURL = origBaseURL
	})

	tmpDir := t.TempDir()
	origInstallDirFn := engramInstallDirFn
	engramInstallDirFn = func() string { return tmpDir }
	t.Cleanup(func() { engramInstallDirFn = origInstallDirFn })

	installedPath, err := DownloadLatestBinary(false)
	if err != nil {
		t.Fatalf("DownloadLatestBinary() error = %v, want core engram v%s selected from page 2", err, binaryVersion)
	}

	if _, err := os.Stat(installedPath); err != nil {
		t.Fatalf("stat installed binary: %v", err)
	}

	// Verify that pagination actually happened — both page 1 and page 2 must have been requested.
	gotPages := strings.Join(pagesRequested, ",")
	if !strings.Contains(gotPages, "1") || !strings.Contains(gotPages, "2") {
		t.Errorf("expected pagination across pages 1 and 2, got pages requested: [%s]", gotPages)
	}
}

// --- TestEngramExpectedChecksumFor ---
//
// Table-driven unit tests for the BSD-style checksums.txt parser.
func TestEngramExpectedChecksumFor(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		filename      string
		wantDigest    string
		wantErrSubstr string
	}{
		{
			name:       "exact match returns digest",
			content:    "abc123  engram_1.0.0_darwin_arm64.tar.gz\n",
			filename:   "engram_1.0.0_darwin_arm64.tar.gz",
			wantDigest: "abc123",
		},
		{
			name: "finds correct entry among multiple",
			content: "aaa111  engram_0.9.0_darwin_arm64.tar.gz\n" +
				"bbb222  engram_1.0.0_darwin_arm64.tar.gz\n" +
				"ccc333  engram_1.1.0_darwin_arm64.tar.gz\n",
			filename:   "engram_1.0.0_darwin_arm64.tar.gz",
			wantDigest: "bbb222",
		},
		{
			name:          "missing filename returns error",
			content:       "abc123  engram_0.9.0_darwin_arm64.tar.gz\n",
			filename:      "engram_1.0.0_darwin_arm64.tar.gz",
			wantErrSubstr: "not listed in checksums.txt",
		},
		{
			name:          "empty content returns error",
			content:       "",
			filename:      "engram_1.0.0_darwin_arm64.tar.gz",
			wantErrSubstr: "not listed in checksums.txt",
		},
		{
			name:          "malformed lines (single field) are skipped",
			content:       "justonefield\n",
			filename:      "justonefield",
			wantErrSubstr: "not listed in checksums.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := engramExpectedChecksumFor(tt.content, tt.filename)

			if tt.wantErrSubstr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrSubstr)
				}
				if !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErrSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantDigest {
				t.Errorf("got digest %q, want %q", got, tt.wantDigest)
			}
		})
	}
}

// --- TestDownloadLatestBinary_ChannelRouting (Slice 3) ---

// TestDownloadLatestBinary_StableChannelUsesRelease verifies that calling
// DownloadLatestBinary with isBeta=false fetches from the release download
// path (the existing GitHub Releases flow).
func TestDownloadLatestBinary_StableChannelUsesRelease(t *testing.T) {
	server := makeServerWithFakeTarGz(t, "1.3.0")
	defer server.Close()

	origClient := engramHTTPClient
	origBaseURL := engramGitHubBaseURL
	engramHTTPClient = server.Client()
	engramGitHubBaseURL = server.URL
	t.Cleanup(func() {
		engramHTTPClient = origClient
		engramGitHubBaseURL = origBaseURL
	})

	tmpDir := t.TempDir()
	origInstallDirFn := engramInstallDirFn
	engramInstallDirFn = func() string { return tmpDir }
	t.Cleanup(func() { engramInstallDirFn = origInstallDirFn })

	// isBeta=false → should use the release download path.
	installedPath, err := DownloadLatestBinary(false)
	if err != nil {
		t.Fatalf("DownloadLatestBinary(stable): unexpected error: %v", err)
	}
	if !strings.HasPrefix(installedPath, tmpDir) {
		t.Errorf("installedPath = %q, want prefix %q", installedPath, tmpDir)
	}
}

// TestDownloadLatestBinary_BetaChannelUsesGoInstallMain verifies that calling
// DownloadLatestBinary with isBeta=true performs go install @main instead of
// fetching a release archive.
func TestDownloadLatestBinary_BetaChannelUsesGoInstallMain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires go install command")
	}

	origGoInstallFn := engramGoInstallFn
	t.Cleanup(func() { engramGoInstallFn = origGoInstallFn })

	var gotPkg string
	engramGoInstallFn = func(pkg string) (string, error) {
		gotPkg = pkg
		return "/tmp/engram-beta", nil
	}

	// isBeta=true → should use go install @main, not release download.
	installedPath, err := DownloadLatestBinary(true)
	if err != nil {
		t.Fatalf("DownloadLatestBinary(beta): unexpected error: %v", err)
	}
	if !strings.Contains(gotPkg, "@main") {
		t.Errorf("go install pkg = %q, want @main suffix", gotPkg)
	}
	if installedPath == "" {
		t.Error("expected non-empty installedPath for beta channel")
	}
}

func TestCanonicalEngramGoInstallPackagePreservesDeclaredModuleCasing(t *testing.T) {
	tests := []struct {
		name string
		pkg  string
		want string
	}{
		{
			name: "lowercase owner is canonicalized",
			pkg:  "github.com/gentleman-programming/engram/cmd/engram@main",
			want: "github.com/Gentleman-Programming/engram/cmd/engram@main",
		},
		{
			name: "canonical owner remains unchanged",
			pkg:  "github.com/Gentleman-Programming/engram/cmd/engram@v1.2.3",
			want: "github.com/Gentleman-Programming/engram/cmd/engram@v1.2.3",
		},
		{
			name: "unrelated package remains unchanged",
			pkg:  "github.com/gentleman-programming/gentle-ai/v3/cmd/gentle-ai@latest",
			want: "github.com/gentleman-programming/gentle-ai/v3/cmd/gentle-ai@latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canonicalEngramGoInstallPackage(tt.pkg); got != tt.want {
				t.Fatalf("canonicalEngramGoInstallPackage(%q) = %q, want %q", tt.pkg, got, tt.want)
			}
		})
	}
}

func TestEngramGoInstallFromMainCanonicalizesModuleCasing(t *testing.T) {
	const fakeInstallDir = "/custom/gobin"

	origGoInstallCmdFn := engramGoInstallCmdFn
	origGoEnvFn := engramGoEnvFn
	t.Cleanup(func() {
		engramGoInstallCmdFn = origGoInstallCmdFn
		engramGoEnvFn = origGoEnvFn
	})

	var gotPkg string
	engramGoInstallCmdFn = func(pkg string) error {
		gotPkg = pkg
		return nil
	}
	engramGoEnvFn = func(keys ...string) (map[string]string, error) {
		return map[string]string{"GOBIN": fakeInstallDir, "GOPATH": ""}, nil
	}

	_, err := engramGoInstallFromMain("github.com/gentleman-programming/engram/cmd/engram@main")
	if err != nil {
		t.Fatalf("engramGoInstallFromMain: unexpected error: %v", err)
	}

	wantPkg := "github.com/Gentleman-Programming/engram/cmd/engram@main"
	if gotPkg != wantPkg {
		t.Fatalf("go install package = %q, want %q", gotPkg, wantPkg)
	}
}

// TestEngramGoInstallFromMain_UsesGoEnvForBinDir verifies that
// engramGoInstallFromMain resolves the install directory via `go env GOBIN GOPATH`
// (the effective Go environment) rather than reading raw shell env vars.
// This matters when GOBIN is set via `go env -w GOBIN=...` (stored in Go's
// env file) but NOT exported into the shell environment.
func TestEngramGoInstallFromMain_UsesGoEnvForBinDir(t *testing.T) {
	fakeInstallDir := filepath.Join(t.TempDir(), "custom", "gobin", "via", "go-env")

	origGoEnvFn := engramGoEnvFn
	t.Cleanup(func() { engramGoEnvFn = origGoEnvFn })

	// Simulate GOBIN set only via `go env -w` (not in shell env).
	// The raw os.Getenv("GOBIN") would return "", but go env GOBIN returns
	// the persisted value from the Go env file.
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")

	engramGoEnvFn = func(keys ...string) (map[string]string, error) {
		result := make(map[string]string, len(keys))
		for _, k := range keys {
			if k == "GOBIN" {
				result[k] = fakeInstallDir
			} else {
				result[k] = ""
			}
		}
		return result, nil
	}

	// Inject a fake go install that does nothing (no real network/build).
	origGoInstallCmdFn := engramGoInstallCmdFn
	t.Cleanup(func() { engramGoInstallCmdFn = origGoInstallCmdFn })
	engramGoInstallCmdFn = func(pkg string) error { return nil }

	binaryPath, err := engramGoInstallFromMain("github.com/Gentleman-Programming/engram/cmd/engram@main")
	if err != nil {
		t.Fatalf("engramGoInstallFromMain: unexpected error: %v", err)
	}

	wantDir := fakeInstallDir
	gotDir := filepath.Dir(binaryPath)
	if gotDir != wantDir {
		t.Errorf("binary dir = %q, want %q (from go env GOBIN)", gotDir, wantDir)
	}
}

func TestEngramGoInstallFromMain_BypassesPublicGoProxy(t *testing.T) {
	binDir := t.TempDir()
	goPath := filepath.Join(binDir, "go")
	recordPath := filepath.Join(t.TempDir(), "go-env.txt")
	fakeGo := filepath.Join(binDir, "go")
	script := "#!/usr/bin/env bash\n" +
		"printf 'GONOSUMDB=%s\\nGOPRIVATE=%s\\nGONOPROXY=%s\\n' \"${GONOSUMDB:-}\" \"${GOPRIVATE:-}\" \"${GONOPROXY:-}\" > \"$GO_ENV_RECORD\"\n"
	if err := os.WriteFile(fakeGo, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GO_ENV_RECORD", recordPath)

	origGoEnvFn := engramGoEnvFn
	t.Cleanup(func() { engramGoEnvFn = origGoEnvFn })
	engramGoEnvFn = func(keys ...string) (map[string]string, error) {
		return map[string]string{"GOBIN": goPath, "GOPATH": filepath.Join(t.TempDir(), "gopath")}, nil
	}

	if _, err := engramGoInstallFromMain("github.com/Gentleman-Programming/engram/cmd/engram@main"); err != nil {
		t.Fatalf("engramGoInstallFromMain() error = %v", err)
	}

	recorded, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", recordPath, err)
	}
	for _, want := range []string{
		"GONOSUMDB=github.com/Gentleman-Programming/engram",
		"GOPRIVATE=github.com/Gentleman-Programming/engram",
		"GONOPROXY=github.com/Gentleman-Programming/engram",
	} {
		if !strings.Contains(string(recorded), want) {
			t.Fatalf("go install env missing %q\nrecorded:\n%s", want, recorded)
		}
	}
}
