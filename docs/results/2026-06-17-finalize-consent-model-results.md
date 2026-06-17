# Finalize consent model — results
Change: #21 · Branch: feat/finalize-consent-model · PR: <set at PR open> · Plan: docs/superpowers/plans/2026-06-17-finalize-consent-model.md · ADRs: 10, 11

## Verify (human)

Automated structural sentinels cover the behavior (40/40 in `tests/test_finalize_gate.sh`, full repo suite green). No interactive checks are strictly required, but one documentation decision is worth a glance at the merge gate:

- [ ] **Convention-parity call (decide & optionally follow up).** Per spec §7 this PR deliberately did **not** edit `skills/docket-convention/`. The convention's `.docket.yml` example (its `finalize:` block) lists `gate` + `test_command` but now omits the new sibling `require_pr_approval`. Confirm you're OK leaving the canonical convention example without the new knob (finalize owns its config doc — the test asserts the doc lives in the finalize SKILL), or ask for a one-line commented `require_pr_approval:` parity addition to the convention example. See Findings #2.

## Findings

1. **ADR-0011 recorded** — *Finalize consent model — ambiguity-only prompt + `require_pr_approval` policy gate* (`relates_to: [10]`, `change: 21`, Accepted). Captures the two-part consent model and the principle "`require_pr_approval` ensures a human authorized the merge — on the auto-detect path that proof is a GitHub approval; an explicit id is that proof by another means; correctness is checked regardless."
2. **Spec §3 rationale was factually imprecise** (caught in whole-branch review). §3 justified not documenting `require_pr_approval` in the convention by saying "the convention's `.docket.yml` example does not enumerate `finalize:`" — but it *does* (it lists `gate`/`test_command`). The scope call to keep `require_pr_approval` in finalize's own SKILL.md is still defensible (it follows the `gate`/`test_command` doc-ownership precedent and the test asserts it), but the stated reason doesn't hold. Recorded here rather than silently "fixing" an explicit spec scope boundary autonomously.
3. **Sentinel-sharpening during review.** The initial `require_pr_approval.*default.*false` doc sentinel was non-vacuous but double-guarded (the YAML config comment *and* the prose paragraph both satisfied it — dropping the substantive prose would leave it green). Split into two independently-anchored asserts: the config-block YAML knob line, and the prose's unique "validates *human sign-off*" sentence. Each mutation-tested to flip in isolation (LEARNINGS #15).

## Follow-ups

- **(optional) Convention example parity** — if the human prefers parity over the spec's scope boundary, add a commented `require_pr_approval: false` line to the `finalize:` block of the `.docket.yml` example in `skills/docket-convention/SKILL.md`. One-line doc-only change; breaks no test (the convention's gate/wrapper count sentinels are untouched). Left out of this PR to honor spec §7's "no convention edit."
