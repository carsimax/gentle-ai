package cli

import (
	"os"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/model"
	"github.com/gentleman-programming/gentle-ai/v4/internal/reviewerprovider"
	"github.com/gentleman-programming/gentle-ai/v4/internal/state"
)

func TestClaudeReviewAdapterUsesSavedModelForEachNativeRole(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	roles := []struct{ role, lens, model, effort string }{
		{reviewProviderRoleLens, "review-risk", "opus", "low"},
		{reviewProviderRoleLens, "review-readability", "haiku", ""},
		{reviewProviderRoleLens, "review-reliability", "sonnet", "high"},
		{reviewProviderRoleLens, "review-resilience", "fable", "xhigh"},
		{reviewProviderRoleRefuter, "", "opus", "medium"},
		{reviewProviderRoleTargetedValidator, "", "opus", "medium"},
	}
	keys := []string{"risk", "readability", "reliability", "resilience", "refuter", "validator"}
	persisted := state.InstallState{ClaudePhaseAssignments: make(map[string]state.ClaudePhaseAssignmentState)}
	for i, key := range keys {
		persisted.ClaudePhaseAssignments[key] = state.ClaudePhaseAssignmentState{Model: roles[i].model, Effort: roles[i].effort}
	}
	if err := state.Write(home, persisted); err != nil {
		t.Fatal(err)
	}
	loaded, err := state.Read(home)
	if err != nil {
		t.Fatal(err)
	}
	for i, key := range keys {
		if loaded.ClaudePhaseAssignments[key].Model != roles[i].model {
			t.Fatalf("persisted role %s was lost", key)
		}
		adapter, err := reviewProviderAdapter(roles[i].role, model.AgentClaudeCode, roles[i].lens)
		if err != nil {
			t.Fatal(err)
		}
		claude := adapter.(*reviewerprovider.ClaudeAdapter)
		if claude.Model != model.ClaudeModelAlias(roles[i].model) || claude.Effort != model.ClaudeEffort(roles[i].effort) {
			t.Errorf("%s selected model=%q effort=%q, want model=%q effort=%q", key, claude.Model, claude.Effort, roles[i].model, roles[i].effort)
		}
	}
}

// Issue #4275: a review role without a valid assignment resolves like the
// installed review-*.md files (the default row, then sonnet) instead of
// launching the reviewer without --model on the account default model.
func TestClaudeReviewAdapterMissingAndInvalidAssignmentsFallBackToDefaultRow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sonnet := model.ClaudePhaseAssignment{Model: model.ClaudeModelSonnet}
	opusHigh := model.ClaudePhaseAssignment{Model: model.ClaudeModelOpus, Effort: model.ClaudeEffortHigh}
	for _, tc := range []struct {
		persisted state.InstallState
		want      model.ClaudePhaseAssignment
	}{
		{state.InstallState{}, sonnet},
		{state.InstallState{ClaudePhaseAssignments: map[string]state.ClaudePhaseAssignmentState{"risk": {Model: "invalid"}}}, sonnet},
		{state.InstallState{ClaudePhaseAssignments: map[string]state.ClaudePhaseAssignmentState{"risk": {Model: "opus", Effort: "invalid"}}}, sonnet},
		{state.InstallState{ClaudePhaseAssignments: map[string]state.ClaudePhaseAssignmentState{"risk": {Model: "haiku", Effort: "low"}}}, sonnet},
		{state.InstallState{ClaudeModelAssignments: map[string]string{"risk": "invalid"}}, sonnet},
		{state.InstallState{ClaudePhaseAssignments: map[string]state.ClaudePhaseAssignmentState{"default": {Model: "opus", Effort: "high"}}}, opusHigh},
	} {
		if err := state.Write(home, tc.persisted); err != nil {
			t.Fatal(err)
		}
		adapter, err := reviewProviderAdapter(reviewProviderRoleLens, model.AgentClaudeCode, "review-risk")
		if err != nil {
			t.Fatal(err)
		}
		claude := adapter.(*reviewerprovider.ClaudeAdapter)
		if got := (model.ClaudePhaseAssignment{Model: claude.Model, Effort: claude.Effort}); got != tc.want {
			t.Errorf("state %+v selected %+v, want %+v", tc.persisted, got, tc.want)
		}
	}
	if err := state.Write(home, state.InstallState{ClaudeModelAssignments: map[string]string{"refuter": "sonnet"}}); err != nil {
		t.Fatal(err)
	}
	adapter, err := reviewProviderAdapter(reviewProviderRoleRefuter, model.AgentClaudeCode)
	if err != nil || adapter.(*reviewerprovider.ClaudeAdapter).Model != model.ClaudeModelSonnet || adapter.(*reviewerprovider.ClaudeAdapter).Effort != model.ClaudeEffortDefault {
		t.Fatalf("legacy assignment = %v, %v", adapter, err)
	}
	if err := state.Write(home, state.InstallState{
		ClaudePhaseAssignments: map[string]state.ClaudePhaseAssignmentState{"refuter": {Model: "invalid"}},
		ClaudeModelAssignments: map[string]string{"refuter": "sonnet"},
	}); err != nil {
		t.Fatal(err)
	}
	if got := savedClaudeReviewAssignment("refuter"); got != (model.ClaudePhaseAssignment{Model: model.ClaudeModelSonnet}) {
		t.Fatalf("invalid phase assignment should preserve valid legacy fallback with default effort, got %+v", got)
	}
	if err := state.Write(home, state.InstallState{
		ClaudePhaseAssignments: map[string]state.ClaudePhaseAssignmentState{"refuter": {Model: "opus", Effort: "invalid"}},
		ClaudeModelAssignments: map[string]string{"refuter": "sonnet"},
	}); err != nil {
		t.Fatal(err)
	}
	if got := savedClaudeReviewAssignment("refuter"); got != (model.ClaudePhaseAssignment{Model: model.ClaudeModelSonnet}) {
		t.Fatalf("invalid effort should preserve valid legacy fallback with default effort, got %+v", got)
	}
	if err := state.Write(home, state.InstallState{
		ClaudePhaseAssignments: map[string]state.ClaudePhaseAssignmentState{"refuter": {Model: "opus", Effort: "high"}},
		ClaudeModelAssignments: map[string]string{"refuter": "sonnet"},
	}); err != nil {
		t.Fatal(err)
	}
	adapter, err = reviewProviderAdapter(reviewProviderRoleRefuter, model.AgentClaudeCode)
	if err != nil {
		t.Fatal(err)
	}
	if got := adapter.(*reviewerprovider.ClaudeAdapter).Model; got != model.ClaudeModelOpus {
		t.Fatalf("valid model+effort selected %q, want opus", got)
	}
	if err := os.WriteFile(state.Path(home), []byte("invalid JSON"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := savedClaudeReviewAssignment("refuter"); got != sonnet {
		t.Fatalf("invalid persisted state selected %+v, want sonnet", got)
	}
}
