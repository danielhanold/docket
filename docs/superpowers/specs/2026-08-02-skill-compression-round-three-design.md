<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0201 — Skill compression round three — targeted progressive disclosure on the Big 4 + regrowth-guard ratchet](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0201-skill-compression-round-three.md)**
<!-- docket:backlink:end -->

# Skill compression round three — targeted progressive disclosure — design

**Change:** 0201 · **Date:** 2026-08-02 · **Status:** approved (brainstorm with Daniel)

## Problem

The 0053/0055 (round one) and 0085 (round two) slims both hit their targets and both
regrew: the regrowth-guard budgets in `tests/test_skill_size_budgets.sh` have been
consciously raised five times since 0085 (0102, 0127, 0137, 0167 ×2), and the four
largest skill files now sit within 11–51 words of their caps:

| File | Words | Lines | Headroom | references/ today |
|---|---|---|---|---|
| docket-convention/SKILL.md | 6,349 | 363 | 51 w | 3 files |
| docket-finalize-change/SKILL.md | 4,302 | 191 | 48 w | none |
| docket-implement-next/SKILL.md | 3,939 | 143 | 11 w | none |
| docket-build/SKILL.md | 2,418 | 263 | 32 w | none |

`docket-convention` is loaded as blocking Step 0 on every docket run, so its excess is a
tax on every operation (guidance: < 5k words for a frequently-loaded SKILL.md — and it
always rides alongside an operating skill). The budget test has degraded into a speed
bump: each change asserts its new prose has "no other home" and raises the cap in-diff.

## Decision summary (settled in brainstorm)

1. **Targets: the Big 4** — convention, finalize-change, implement-next, build. The seven
   skills ≤ 1.4k words are out of scope.
2. **Prior inline-placement decisions are honored, not reversed.** 0137's
   dispatch-capability rule + A/B/C tier table stays in convention's SKILL.md; 0167's nine
   halting dispositions stay in docket-build's controller. Both are tightened in wording
   only — same content, same location, same citation anchors. Rationale: those rules fire
   at the moment an agent is about to reach a *wrong conclusion* (a misconception has no
   recognized trigger that would cause a reference read), and their consuming sites carry
   mutation-tested anchors that wholesale moves would break for ~400 words of savings.
3. **Approach: targeted hybrid** — extract only cold paths that have a *recognizable
   trigger moment*; tighten hot paths in place. Rejected: full progressive disclosure
   (extra reference reads on every happy-path run; much harder behavior-neutrality
   review) and tighten-only (0085 proved it insufficient — no room for future additions
   means round four).
4. **Regrowth guard: ratchet + raise-rule.** Budgets drop to post-slim actuals (+ the
   existing rounding margin), and the raise procedure is tightened (below).

## Extraction map (new reference files)

Each extraction moves a **cold path** behind a **loud blocking pointer placed at its
trigger moment** in the parent SKILL.md — the pattern already proven by
`docket-convention/references/{learnings,agent-layer,terminal-close-out}.md`.

### `skills/docket-finalize-change/references/gate-failure.md`

The merge-gate *failure* flows: rebase-conflict handling and the `docket-rebase-resolver`
dispatch, red-rebased-suite handling and the `docket-integration-repair` dispatch +
sign-off gating, and the `## Finalize blocked` marker lifecycle (write, board cell,
skip-on-auto-detect, named-id override, clearing rule). SKILL.md keeps: the gate sequence
itself, green-path merge, and one blocking pointer at the failure branch ("gate failed →
read `references/gate-failure.md` before acting").

### `skills/docket-implement-next/references/edge-paths.md`

The rare edges: reconcile-kill, the blocked transition, resume-with-explicit-id, and
PR-body assembly mechanics. SKILL.md keeps the spine — pick → claim → reconcile → plan →
build → review → PR — with each edge compressed to a one-line trigger + pointer. The four
dispatch-site Tier A/C clauses and their *Dispatch-capability resolution* back-pointers
stay inline verbatim-in-meaning (0137 anchors).

### `skills/docket-convention/references/auto-capture.md`

The full auto-capture shared definition: classify → admit → suppress, the materiality
bar, the deterministic mint-stub contract (flags, exit codes, `<n so far>` carry-forward,
best-effort posture). SKILL.md keeps a ~4-line definition (what auto-capture is, the
`enabled`/`types` knobs, mint sites vs. never-mint sites) + the read trigger: "discovered
follow-up work mid-run → read `references/auto-capture.md` before minting or suppressing."
This mirrors the learnings.md precedent exactly (summary + gate inline, mechanics behind
a read-before-acting pointer).

### Explicitly not extracted

Dispatch-capability resolution (0137), docket-build halting dispositions (0167), the
Step-0 preamble, the bootstrap 2×2 verdict table, build-readiness/selection, and the
lifecycle table — all hot-path or intervene-at-the-moment content.

## In-place tightening (all four files)

- Provenance narration → bare `(change NNNN)` / `(ADR-NNNN)` pointers.
- Duplicated litanies collapse to their single owner + citation (the 0053 recipe).
- Paragraph-form rules → tables where content is enumerable.
- Convention's bootstrap-guard probe prose compresses toward the verdict table.

## Targets

Post-slim word targets (budgets set from measured actuals via the existing rounding
rule — lines to next multiple of 5, words to next multiple of 50, within-25 pushes to the
next multiple):

| File | Now | Target |
|---|---|---|
| docket-convention/SKILL.md | 6,349 | ≤ ~4,700 |
| docket-finalize-change/SKILL.md | 4,302 | ≤ ~2,900 |
| docket-implement-next/SKILL.md | 3,939 | ≤ ~2,900 |
| docket-build/SKILL.md | 2,418 | ≤ ~2,100 |

New reference files: ≤ ~150 lines each, budget rows added (the test's completeness guard
auto-enforces coverage). Targets are direction, not gospel (learnings:
size-target-is-direction) — exact budget numbers are fixed at build time from post-slim
actuals.

## Regrowth-guard hardening

- All four SKILL.md budget rows ratchet **down** to post-slim actuals + margin; rows added
  for the three new reference files.
- The BUDGETS comment-block procedure gains one requirement: **a raise must name the
  reference file the new prose was considered for and state why it cannot live there.**
  "No other home" becomes a claim that gets argued in-diff, not asserted. Test mechanics
  are unchanged — still an in-diff, comment-documented raise.

## Verification (the proven 0085 recipe)

1. **Anchor grep-gate** — every cross-file citation and mutation-guard anchor (0137 site
   nouns + tiers, section headings cited by other skills, `DIRECTED to:` markers) present
   before and after.
2. **Behavior-neutrality diff review** — no rule added, dropped, or reweighted; only
   relocated or reworded. The sole structural changes are the three extractions + guard
   hardening.
3. **Loud-pointer check** — every extracted section reachable from exactly one blocking
   pointer sited at its trigger moment in the parent SKILL.md.
4. **Budget test green** at the new lower rows.
5. **`docket-status` smoke run** end-to-end.

## Out of scope

Skill semantics and workflow behavior; frontmatter `description:` lines; agent wrappers
and `sync-agents.sh`; scripts and script contracts; templates;
`github-board-mirror.md`; the existing three convention references (already within
budget); the seven skills ≤ 1.4k words.
