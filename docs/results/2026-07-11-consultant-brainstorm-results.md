# Consultant-authored brainstorm — results
Change: #56 · Branch: feat/consultant-brainstorm · PR: <set on open> · Plan: docs/superpowers/plans/2026-07-11-consultant-brainstorm.md · ADRs: 22 (relates 8/9/18)

## Verify (human)

- [ ] **The build-time ADR rides this change's terminal publish.** ADR-0022 (consultant-authored brainstorm) is recorded on `origin/docket` and cited in change 0056's `adrs:` — it is deliberately NOT yet on `main` (the mid-run main-publish is classifier-blocked). It will be copied onto the integration branch by this change's terminal-publish at merge. The convention prose and the change both reference ADR-0022; confirm the merge publishes it so those references resolve on `main`.
- [ ] **Try the opt-in once (optional).** Run `docket-new-change`/`docket-groom-next` and ask for a consultant-written spec, or set `skills: brainstorm: docket-brainstorm`, and confirm the single pinned dispatch authors a spec (or returns critique concerns) and stops at the spec. On a harness without dispatch, confirm it degrades inline with a prominent warning.

## Findings

- **Off by default, verified:** the built-in brainstorm default stays `superpowers:brainstorming` (`SKILL_BRAINSTORM` unchanged in the read-only smoke); docket-new-change/docket-groom-next changes are a single behavior-neutral verbal-opt-in note each. The whole-branch review found no existing-repo behavior shift and no ADR-0006 boundary violation (the human dialogue never routes through the consultant).
- **New wrapper shape:** `agents/docket-brainstorm-consultant.md` is the first wrapper that injects **no skill and no convention** (fourth no-skill wrapper overall) — a deliberate deviation from the ADR-0009 critic, recorded in ADR-0022. Achieved with no sync-agents.sh code change (built-in wrappers are used verbatim except model/effort).
- **Cross-cutting count invariant handled:** adding the 9th wrapper made the convention's "eight wrappers / three no-skill" prose and `test_finalize_gate`'s "eight" assertion stale, plus seven "8 built-in wrappers" assertions in `test_sync_agents`. All updated to nine / four-no-skill, naming the consultant. The full suite (0 failures) was the gate that confirmed no remaining stale count.
- **Single-dispatch, harness-portable:** no `SendMessage`/agent-continuation anywhere; the consultant returns in-context (the finalize gate-agent contract). The author-or-critique gate keeps the pinned tier load-bearing even though option generation runs at the session model.

## Follow-ups

- None required. A possible LATER change (explicitly out of scope here) is flipping the built-in brainstorm default to `docket-brainstorm` once the pattern has mileage.
