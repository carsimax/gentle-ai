package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/model"
	"github.com/gentleman-programming/gentle-ai/v4/internal/state"
	"github.com/gentleman-programming/gentle-ai/v4/internal/system"
)

// Issue #5252: Claude ODD role models are only consumed through the routing
// table, so both install and sync must carry the saved assignments into it.
func TestClaudeODDRoutingCarriesSavedAssignments(t *testing.T) {
	selection := model.Selection{
		Agents: []model.AgentID{model.AgentClaudeCode},
		ClaudePhaseAssignments: map[string]model.ClaudePhaseAssignment{
			"odd-worker": {Model: model.ClaudeModelOpus},
			"odd-verify": {Model: model.ClaudeModelHaiku},
		},
	}
	want := []string{"| `odd-explorer` | `sonnet` |", "| `odd-worker` | `opus` |", "| `odd-verify` | `haiku` |"}

	t.Run("install", func(t *testing.T) {
		home := t.TempDir()
		runInstallInjectionSteps(t, newTestInstallRuntime(t, home, selection))
		body := readTextFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))
		for _, row := range want {
			if !strings.Contains(body, row) {
				t.Errorf("install routing missing %q", row)
			}
		}
	})

	t.Run("sync", func(t *testing.T) {
		home := t.TempDir()
		runtime, err := newSyncRuntimeWithScope(home, selection, ScopeGlobal)
		if err != nil {
			t.Fatal(err)
		}
		ran := false
		for _, step := range runtime.stagePlan().Apply {
			if routing, ok := step.(agentRoutingGuidanceStep); ok && routing.agent == model.AgentClaudeCode {
				if err := routing.Run(); err != nil {
					t.Fatal(err)
				}
				ran = true
			}
		}
		if !ran {
			t.Fatal("sync scheduled no Claude routing step")
		}
		body := readTextFile(t, filepath.Join(home, ".claude", "CLAUDE.md"))
		for _, row := range want {
			if !strings.Contains(body, row) {
				t.Errorf("sync routing missing %q", row)
			}
		}
	})
}

// An explicit-agent reinstall must render Claude guidance and managed agents
// from the saved assignments, not from the sonnet fallback, because the merge
// that runs afterwards keeps those saved assignments in state.
func TestExplicitClaudeInstallRendersSavedAssignments(t *testing.T) {
	home := installTestHome(t)
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	if err := state.Write(home, state.InstallState{
		InstalledAgents: []string{string(model.AgentClaudeCode)},
		ClaudePhaseAssignments: map[string]state.ClaudePhaseAssignmentState{
			"orchestrator": {Model: "opus"},
			"odd-worker":   {Model: "opus", Effort: "high"},
			"jd-judge-a":   {Model: "opus", Effort: "high"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RunInstall([]string{"--agent=claude-code", "--components=skills", "--skills=go-testing"}, system.DetectionResult{}); err != nil {
		t.Fatalf("RunInstall() error = %v", err)
	}
	if body := readTextFile(t, filepath.Join(home, ".claude", "CLAUDE.md")); !strings.Contains(body, "| `odd-worker` | `opus` |") {
		t.Error("install routing does not carry the saved odd-worker model")
	}
	judge := readTextFile(t, filepath.Join(home, ".claude", "agents", "jd-judge-a.md"))
	if !strings.Contains(judge, "model: opus") || !strings.Contains(judge, "effort: high") {
		t.Errorf("jd-judge-a.md does not carry the saved assignment:\n%s", judge)
	}
	saved, err := state.Read(home)
	if err != nil {
		t.Fatal(err)
	}
	if got := saved.ClaudePhaseAssignments["orchestrator"].Model; got != "opus" {
		t.Errorf("install rewrote saved orchestrator assignment: got %q, want opus", got)
	}
	if got := saved.ClaudePhaseAssignments["odd-worker"]; got.Model != "opus" || got.Effort != "high" {
		t.Errorf("install rewrote saved odd-worker assignment: %+v", got)
	}
}
