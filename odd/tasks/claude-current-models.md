# Claude current models and effort levels

Objective: align the Claude Code model table with the current Anthropic models (Opus 5.5, Sonnet 5.5, Haiku 5.5, Fable 5.1) and their effort levels. Branch: feat/claude-current-models. Baseline: fc308d49f. Delivery strategy: ask-on-risk. Test runner: ./scripts/test-claude.sh.

## Specs

- **S1 — Haiku effort levels.** "actualiza la tabla de modelos a los modelos actuales y niveles de opensamiento de anthropic"
  - Given a Custom Claude assignment on `haiku` When the user picks an effort Then the picker offers `default`, `low`, `medium`, `high`, `xhigh`, `max` (Claude Haiku 5.5 supports all five; default `medium`), the assignment validates, and sync and the native reviewer pass the effort through. Upstream issue #5653.
- **S2 — Telemetry alias table.** "actualiza la tabla de modelos a los modelos actuales"
  - Given an agent definition whose `model:` is an alias When telemetry resolves the selected model Then `opus` maps to `claude-opus-5-5`, `sonnet` to `claude-sonnet-5-5`, `haiku` to `claude-haiku-5-5`, and `fable` to `claude-fable-5-1` (today `fable` falls to `custom`).
  - Assumption: aliases themselves stay unchanged; Claude Code resolves them to the current model at run time.

## Tasks

| ID | Specs | Task | Route | Risk | Status | Commit |
|---|---|---|---|---|---|---|
| T1 | S1 | Haiku offers all effort levels in `ClaudeEffortsForModel`; tests in model, picker, reviewerprovider; `docs/usage.md` | worker | 3 | done (verified) | — |
| T2 | S2 | Telemetry alias table maps to 5.5 / Fable 5.1 IDs, adds `fable`; tests; `docs/telemetry.md` | worker | 3 | done (verified) | — |
| T3 | S1 | Converge gap: assert a haiku assignment with explicit effort validates (`ClaudePhaseAssignment.Valid`) and reaches the native reviewer (`internal/cli/review_provider_runtime_model_test.go`, which encoded haiku/low as invalid) | inline | 3 | done (verified) | — |

## Log

- **L1 (2026-10-26):** "actualiza la tabla de modelos a los modelos actuales y niveles de opensamiento de anthropic"
- **L2 (2026-10-26):** "y este cambio lo podriamos tambien mandar a gentle ia si dices que no hay prs?" — keep T1 and T2 as separate commits so each can be replayed onto upstream/main for its own issue.

- **L3 (2026-10-26):** T1/T2 green; fast lane found `TestClaudeReviewAdapterMissingAndInvalidAssignmentsUseNativeDefault` asserting the old haiku rule, fixed under T3 with RED shown against the old effort table.

Next step: full `internal/cli` run, then one commit per task (T1+T3, T2) so each can be replayed onto upstream/main.
