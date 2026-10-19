package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/gentleman-programming/gentle-ai/v4/internal/model"
	"github.com/gentleman-programming/gentle-ai/v4/internal/reviewerprovider"
	"github.com/gentleman-programming/gentle-ai/v4/internal/state"
)

// reviewProviderAdapter resolves a compiled runtime only after the role's
// canonical contract has declared the shared transport capability. Execution
// can therefore never bypass the role registry that owns the schema, budget,
// prompt, and immutable storage binding.
func reviewProviderAdapter(role string, agent model.AgentID, lens ...string) (reviewerprovider.Adapter, error) {
	contract, err := reviewProviderRoleContractFor(role)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(contract.RequiredCapabilities, reviewProviderTransportCapability) {
		return nil, fmt.Errorf("reviewer provider role %q does not permit the compiled transport", contract.Role) // refusal:by-design world-action: a role must explicitly opt in to the compiled provider transport
	}
	adapter, err := reviewProviderAdapterFor(contract, agent)
	if err != nil {
		return nil, err
	}
	if agent != model.AgentClaudeCode && agent != model.AgentCodex {
		return adapter, nil
	}
	key := ""
	switch role {
	case reviewProviderRoleLens:
		if len(lens) == 1 {
			key = strings.TrimPrefix(lens[0], "review-")
			switch key {
			case "risk", "readability", "reliability", "resilience":
			default:
				key = ""
			}
		}
	case reviewProviderRoleRefuter:
		key = "refuter"
	case reviewProviderRoleTargetedValidator:
		key = "validator"
	}
	if claude, ok := adapter.(*reviewerprovider.ClaudeAdapter); ok {
		assignment := savedClaudeReviewAssignment(key)
		claude.Model = assignment.Model
		claude.Effort = assignment.Effort
	}
	if codex, ok := adapter.(*reviewerprovider.CodexAdapter); ok {
		codex.Model = savedCodexReviewModel(key)
	}
	return adapter, nil
}

// savedCodexReviewModel uses only explicit, syntactically valid role assignments.
// Absent or malformed state leaves the isolated CLI's native model default intact.
func savedCodexReviewModel(role string) string {
	if role == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	installed, err := state.Read(home)
	if err != nil {
		return ""
	}
	id := installed.CodexPhaseModelAssignments["rdd-"+role]
	if model.ValidCodexReviewModel(id) {
		return id
	}
	return ""
}

// savedClaudeReviewAssignment resolves a review role exactly like the
// installed review-*.md files: the role, then the default row, then sonnet.
// The reviewer therefore always runs on an explicit model instead of the
// account default (#4275); unreadable state resolves like an empty one.
func savedClaudeReviewAssignment(role string) model.ClaudePhaseAssignment {
	var installed state.InstallState
	if home, err := os.UserHomeDir(); err == nil {
		if read, err := state.Read(home); err == nil {
			installed = read
		}
	}
	legacy := make(map[string]model.ClaudeModelAlias, len(installed.ClaudeModelAssignments))
	for key, value := range installed.ClaudeModelAssignments {
		legacy[key] = model.ClaudeModelAlias(value)
	}
	phases := make(map[string]model.ClaudePhaseAssignment, len(installed.ClaudePhaseAssignments))
	for key, value := range installed.ClaudePhaseAssignments {
		phases[key] = model.ClaudePhaseAssignment{Model: model.ClaudeModelAlias(value.Model), Effort: model.ClaudeEffort(value.Effort)}
	}
	return model.ResolveClaudePhaseAssignment(legacy, phases, role)
}

var reviewProviderAdapterFor = func(contract reviewerprovider.Contract, agent model.AgentID) (reviewerprovider.Adapter, error) {
	if !slices.Contains(contract.RequiredCapabilities, reviewerprovider.TransportCapability) {
		return nil, fmt.Errorf("reviewer provider role %q does not permit the compiled transport", contract.Role) // refusal:by-design world-action: a role must explicitly opt in to the compiled provider transport
	}
	switch agent {
	case model.AgentClaudeCode:
		return reviewerprovider.NewClaudeAdapter(), nil
	case model.AgentCodex:
		return reviewerprovider.NewCodexAdapter(), nil
	case model.AgentOpenCode:
		return nil, fmt.Errorf("reviewer provider runtime %q is host-mediated; launch the provider-issued OpenCode reviewer task", agent) // refusal:by-design world-action: OpenCode must relay through its ordinary managed host
	case model.AgentPi:
		return nil, fmt.Errorf("reviewer provider runtime %q is host-mediated; launch the provider-issued Pi reviewer task", agent) // refusal:by-design world-action: Pi's launcher lives in gentle-pi and relays the Go-issued opaque task
	default:
		return nil, fmt.Errorf("reviewer provider runtime %q has no registered adapter", agent) // refusal:by-design world-action: immutable reviewer execution requires a compiled adapter binding
	}
}

// reviewProviderCaptureRuntime reads the one canonical answer rather than
// restating it. The installed orchestrator contract renders from the same
// predicate, so the transport this dispatcher will execute and the transport
// the contract tells a parent to use can never describe different runtimes
// (issue #3825).
func reviewProviderCaptureRuntime(agent model.AgentID) bool {
	return reviewerprovider.CapturesInProcess(agent)
}

// reviewProviderHostRelayMaterializeRuntime reports whether the runtime's
// compiled immutable transport is the Pi host relay: the one transport whose
// host first prints the exact Go-materialized opaque provider task
// (`review capture-result ... --agent=pi --materialize=true`), runs its own
// fresh locked-down reviewer subprocess on those bytes, and then submits the
// raw result through the existing --input path with the same binding. The
// answer stays false without the exact relay handshake, so materialization is
// never offered to a Pi installation whose launcher cannot collect it.
func reviewProviderHostRelayMaterializeRuntime(agent model.AgentID) bool {
	return reviewImmutableRuntimeCapability(agent).Transport == reviewImmutableTransportPiHostRelay
}
