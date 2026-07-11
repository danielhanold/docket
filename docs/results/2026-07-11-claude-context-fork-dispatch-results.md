# Claude Code `context: fork` dispatch parity — results
Change: #61 · Branch: feat/claude-context-fork-dispatch · PR: <set on open> · Plan: docs/superpowers/plans/2026-07-11-claude-context-fork-dispatch.md · ADRs: 24

## Verify (human)

Automated tests fully cover the *static* invariant (frontmatter present on the 4, absent on the 3). The items below are **runtime** behaviors that no repo test can observe — they require a live Claude Code session and should be confirmed at the merge gate:

- [ ] **Fork actually fires.** Run `/docket-status` in a Claude Code session whose session model is NOT the wrapper's pin. Confirm it executes as a `docket-status` subagent at the wrapper's pinned model/effort (e.g. `haiku`), not inline at the session model. Repeat spot-check for one more, e.g. `/docket-auto-groom`.
- [ ] **Composition / no double-run.** Confirm the forked skill runs its body exactly once — the wrapper preloads the skill via `skills:` (startup injection) and `context: fork` fires on invocation, so there should be no re-fork loop.
- [ ] **Nested fork degrades gracefully (open question #2).** When `docket-implement-next` (itself forked) dispatches `docket-adr` / `docket-status` — which now also carry `context: fork` — confirm it either no-ops to inline within the already-pinned subagent or forks harmlessly, with no breakage. (This run itself exercised the `docket-adr` and `docket-status` dispatches via `Task`, which worked; the untested path is a *forked* implement-next reaching them.)
- [ ] **Inert cross-harness.** If you drive this repo from Cursor/Codex, confirm the new `SKILL.md` frontmatter is ignored there (Cursor keeps its own `docket-dispatch.mdc` rule).

## Findings

- **ADR-0024** records the decision: Claude Code uses `context: fork` + `agent:` frontmatter as its inline-skill dispatch mechanism (parallel to Cursor's generated dispatch rule, ADR-0017); fork only human-non-interactive skills. Accepted; published to `main`.
- **Open question #1 resolved — deferred.** The change asked whether `sync-agents.sh --check` should also enforce the fork invariant. Deferred: `--check` cannot derive "should be forked" from "is autonomous-wrapped" (`docket-finalize-change` is autonomous-wrapped yet deliberately unforked), so it would need an explicit fork-allowlist — more standing machinery than this minimal parity fix warrants, and a second place for the 4/3 split to drift. The dedicated `tests/test_skill_fork_dispatch.sh` is the guard. Rationale captured in ADR-0024 Consequences.
- **Doc-drift caught in whole-branch review (in-scope expansion).** The spec's "Files touched" named only `sync-agents.sh` and `README.md` for the "correct the wrong assumption" work, but the canonical config reference `skills/docket-convention/references/agent-layer.md` (a blocking read) and the convention contract `skills/docket-convention/SKILL.md` still framed the inline-defeat as Cursor-only. Both were updated to the two-mechanism story so the correction is complete and self-consistent (the change's own stated goal). Guided by the LEARNINGS lesson to grep the repo for a stale framing when correcting an assumption.

## Follow-ups

- **Change 0062** (`autonomous-finalize-merge-authorization`) already tracks the excluded case: making `docket-finalize-change` forkable/autonomous is blocked on Claude Code's auto-mode "Merge Without Review" classifier — a permissions-policy decision, not a model-pin fix.
- If a *new* headless-safe autonomous skill is added later, remember to (a) add its `context: fork` + `agent:` frontmatter and (b) extend `tests/test_skill_fork_dispatch.sh`'s `FORKED` list — the deferred `--check` leg does not catch this automatically.
