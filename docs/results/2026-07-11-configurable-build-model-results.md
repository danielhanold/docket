# Configurable SDD build models — results
Change: #44 · Branch: feat/configurable-build-model · PR: <set on open> · Plan: docs/superpowers/plans/2026-07-11-configurable-build-model.md · ADRs: 23 (relates 15/16/18)

## Verify (human)

- [ ] **Live per-harness honoring (spec Q4 — not hermetically testable).** The whole surface assumes the harness running the build honors the `model:` field on the build-phase dispatches the same way it honors it on docket's `agents:` wrappers. Set a repo `build:` block (e.g. `build.implementer` / `build.reviewer` to real IDs on your harness — the motivating case is Cursor's model roster) and run `docket-implement-next` on a change, confirming the implementer/fix subagents and reviewer/final-review dispatches actually run on the configured models. This is the same verification that gates the `agents:` block.
- [ ] **ADR-0023 rides this change's terminal publish.** It is on `origin/docket` and cited in `adrs:`; the mid-run main-publish is classifier-blocked, so it publishes onto `main` via terminal-publish at merge. Confirm the merge publishes it.

## Findings

- **Purely additive, verified:** with no `build:` set, `docket-config.sh --export` emits `BUILD_IMPLEMENTER=''`/`BUILD_REVIEWER=''` and `docket-implement-next` instructs nothing — SDD's own Model Selection is preserved byte-for-byte. Live smoke confirmed (20 emit lines, both BUILD vars empty here).
- **Mirrors the `skills:` block exactly:** `build_role` resolves local > repo-committed > global (same as `skill_role`), direct model-ID passthrough (no interpretation/validation), global-able and correctly ABSENT from the coordination-key fence.
- **Two roles → four SDD dispatch kinds:** `build.implementer` → SDD per-task implementer + fix subagents; `build.reviewer` → SDD per-task reviewer + the Step-6 final whole-branch review dispatch. A review note that the final reviewer runs via the separately-resolved `$SKILL_REVIEW` (not an SDD sub-dispatch) was addressed by tightening the wiring prose to describe it accurately rather than claim it fills "SDD's `model:` field".
- **ADR-0023** records the decision (relates 15/16/18): direct model-ID passthrough, additive/backward-compatible, a set role is a deliberate blunt override of SDD's per-complexity adaptivity.
- **Test note:** the config resolution (passthrough / unset / layering / unknown-role warn) is hermetically tested (R1–R5); the live per-harness `model:` honoring is a runtime check (above), matching the repo's metadata-branch-artifact testing convention.

## Follow-ups

- None required. A possible future refinement (explicitly out of scope here): per-task/per-complexity build-model buckets (mechanical/integration/architecture).
