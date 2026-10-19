# Claude model routing

Objective: make saved Claude model assignments reach ODD delegation, managed review agents, and the in-process reviewer.
Branch: `fix/claude-model-routing` (from `cmx`). Delivery: single PR into `cmx`, re-evaluated if the running count exceeds ~400 lines. Test runner: `go test` (`scripts/test-fast.sh`).

## Specs

- **S1 (upstream #5252, #5550; port of upstream PR #5269, Claude Code only):** "Custom Claude assignments for the ODD roles and "General delegation" are stored in state.json, but no agent file is generated for them in Claude Code. Delegations therefore ignore the configured model and inherit the parent session's model." Expected: "Each configured role is applied when the orchestrator delegates". Fix: render a model-only `Claude ODD worker assignments` table into the Claude routing guidance and instruct the orchestrator to pass it as the Agent tool `model` parameter. The Claude picker asks only for a model on ODD rows. Kiro and Pi changes from #5269 are out of scope.
- **S2 (Copilot finding on #5269):** "rerunning `gentle-ai install --agent claude-code` rewrites the routing table with the Sonnet fallback even though `mergeExplicitAgentInstallState` later preserves the user's saved Claude assignments, leaving the installed guidance inconsistent with state." Expected: an explicit-agent install renders the table from the persisted Claude assignments.
- **S3 (upstream #5358):** "the picked model and effort for every RDD row silently fall through to the `default` row (sonnet) in the generated `~/.claude/agents/review-*.md` files." Expected: "`review-risk.md` carries `model: opus` and `effort: max`" when `risk` is assigned `opus`/`max`; same for readability, reliability, resilience, refuter.
- **S4 (upstream #4275):** the in-process Claude reviewer must not run on the account default model when a review role has no explicit assignment: "the reviewer still falls back to the account default model (Opus 5 1M) and hits the same safeguards refusal." Expected: it resolves role, then `default`, then sonnet, exactly like the installed `review-*.md` files, and always passes `--model`.
- **S5 (follow-up on #4275):** the Claude picker paints an unassigned row as `sonnet` (display-only). Expected: an unassigned row shows the value it actually resolves to (the `default` row, then sonnet).

## Tasks

| ID | Specs | Route | Status | Commit |
|---|---|---|---|---|
| T1 | S1 | inline | done (focused + suite green; 3 pre-existing cmx failures) | see git log |
| T2 | S2 | inline | pending | |
| T3 | S3, S4, S5 | inline | pending | |

Route evidence: one understood change per task, no parallel units, no writer reason; independent verifier at feature end (Risk items 3 and 5).

## Log

- **L1 (2026-10-18, user):** "necesito que planeemos un bug que hay en repo original, en claude no se pasan los modelos oa los agentes, revisa las las issues y las prs que en el repo original estan intentando arreglarlo para poder portear directamente esto primero analiza"
- **L2 (2026-10-18, user):** asked to include #5358 and #4275: "Si incluyelo porfavor."
- **L3 (2026-10-18, user):** "si vamos con la implementacion"
- **L4:** #5269 diff applied with `git apply --3way`; only conflict was `RoutingOptions` (fork adds `ClaudeGlobalModules`), resolved keeping both. Kiro/Pi hunks dropped. Installer resolution (Balanced preset base, then legacy, then phase assignments; role, then default, then sonnet) is kept and moved to `model` so every consumer shares it.
