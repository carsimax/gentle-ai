package opencode

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedLauncherSurvivesTargetUpgrade(t *testing.T) {
	if testing.Short() {
		t.Skip("executes a managed launcher")
	}
	home, root := t.TempDir(), t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "opencode"
	oldTarget, newTarget := filepath.Join(root, "2.0.22", name), filepath.Join(root, "2.0.23", name)
	for _, target := range []string{oldTarget, newTarget} {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		script := "#!/bin/sh\nprintf 'opencode v2.0.23\\n'\n"
		if err := os.WriteFile(target, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stable := filepath.Join(bin, name)
	mustSymlink(t, oldTarget, stable)
	target, err := ResolveTarget(home, bin)
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the generated public launcher, without changing the user's PATH or profiles.
	if err := os.MkdirAll(BinDir(home), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(POSIXLauncherPath(home), []byte(posixLauncher(target)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(stable); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, newTarget, stable)
	if err := os.Remove(oldTarget); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(POSIXLauncherPath(home), "--version")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("launcher after upgrade: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if strings.TrimSpace(string(output)) != "opencode v2.0.23" {
		t.Fatalf("stdout = %q", output)
	}
}

func TestDetectRuntimeMajorRecoversOwnedBrokenLauncher(t *testing.T) {
	for _, owned := range []bool{true, false} {
		label := "user launcher"
		if owned {
			label = "managed launcher"
		}
		t.Run(label, func(t *testing.T) {
			home, realDir := t.TempDir(), t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("PATH", BinDir(home)+string(os.PathListSeparator)+realDir)
			if err := os.MkdirAll(BinDir(home), 0o755); err != nil {
				t.Fatal(err)
			}
			name := "opencode"
			missing := filepath.Join(home, "removed-version", name)
			content := posixLauncher(missing)
			if !owned {
				content += "# user modification\n"
			}
			launcher := filepath.Join(BinDir(home), name)
			if err := os.WriteFile(launcher, []byte(content), 0o755); err != nil {
				t.Fatal(err)
			}
			real := filepath.Join(realDir, name)
			if err := os.WriteFile(real, []byte("fixture"), 0o755); err != nil {
				t.Fatal(err)
			}
			old := VersionRunnerOverride
			t.Cleanup(func() { VersionRunnerOverride = old })
			calls := 0
			VersionRunnerOverride = func(ctx context.Context, cmd Command) (CommandOutput, error) {
				calls++
				if len(cmd.Args) != 1 || cmd.Args[0] != "--version" || cmd.OutputLimit != 4096 {
					t.Fatalf("unsafe probe: %+v", cmd)
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("unbounded probe")
				}
				if cmd.Path == "opencode" {
					return CommandOutput{}, &exec.ExitError{}
				}
				if cmd.Path != normalizedCandidatePath(t, real) {
					t.Fatalf("fallback target = %q", cmd.Path)
				}
				return CommandOutput{Stdout: []byte("opencode v2.0.23\n")}, nil
			}
			got, err := DetectRuntimeMajor(context.Background())
			if owned {
				if err != nil || got != RuntimeV2 || calls != 2 {
					t.Fatalf("detect = %v, %v; probes = %d", got, err, calls)
				}
			} else if err == nil || got != RuntimeUnknown || calls != 1 {
				t.Fatalf("user launcher bypassed: %v, %v; probes = %d", got, err, calls)
			}
			after, err := os.ReadFile(launcher)
			if err != nil || string(after) != content {
				t.Fatalf("detection changed launcher: %v", err)
			}
			if owned {
				plan, err := PrepareActivation(home, ActivationOptions{
					Path: os.Getenv("PATH"), Shell: "/bin/sh",
					RunVersion: func(target string) (string, error) {
						if target != normalizedCandidatePath(t, real) {
							t.Fatalf("repair target = %q", target)
						}
						return "opencode v2.0.23\n", nil
					},
					AddToUserPath: func(string) error { return nil },
				})
				if err != nil {
					t.Fatal(err)
				}
				if err := plan.Apply(); err != nil {
					t.Fatal(err)
				}
				after, err := os.ReadFile(launcher)
				want := posixLauncher(normalizedCandidatePath(t, real))
				if err != nil || string(after) != want {
					t.Fatalf("launcher not repaired: %q; %v", after, err)
				}
			}
		})
	}
}

func TestResolveTargetRejectsAliasToManagedLauncher(t *testing.T) {
	home, aliasDir := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(BinDir(home), 0o755); err != nil {
		t.Fatal(err)
	}
	name := "opencode"
	managed := filepath.Join(BinDir(home), name)
	if err := os.WriteFile(managed, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, managed, filepath.Join(aliasDir, name))
	if target, err := ResolveTarget(home, aliasDir); err == nil {
		t.Fatalf("managed alias accepted: %q", target)
	}
}
