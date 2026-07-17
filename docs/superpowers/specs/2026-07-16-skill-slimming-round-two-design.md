# Design: second-round skill slimming — re-slim regrown skills + regrowth guard

**Date:** 2026-07-16 · **Change:** 0085 · **Depends on:** — (0053/0054/0055 all done)

## Problem

The 0053–0055 slimming round (merged 2026-07-11) hit its targets, but the ~30 changes since
(0056–0084: learnings ledger, brainstorm consultant, terminal-publish knob, worktree hooks,
finalize gate hardening, board-pass hardening) regrew the skill bodies:

| File | Post-0053/55 target | 2026-07-16 actual |
|---|---|---|
| docket-convention SKILL.md | ~190 L / ~2,400 w | **329 L / 5,453 w** |
| docket-finalize-change | ≤ ~140 L / ≤ ~2,200 w | 157 L / 2,821 w |
| docket-status | ≤ ~110 L / ≤ ~1,600 w | 114 L / 2,434 w |
| docket-implement-next | ~95–100 L | 108 L / 2,491 w |
| docket-adr / docket-groom-next | 78 / 65 L | 90 / 75 L |
| docket-new-change | ~55 L | 59 L / 1,549 w |
| references/agent-layer.md | — | 165 L / 1,833 w |
| references/terminal-close-out.md | — | 135 L / 1,155 w |

`docket-convention` is loaded as blocking Step 0 by every docket skill on every run; it is
nearly back to its pre-slim size, ~2.2× the agentskills.io < 5,000-token recommendation.
One duplication stands out: the **must-land Board pass litany** (~10 lines of report-line
semantics + bounded-retry instructions) is restated verbatim three times in
`docket-new-change` alone, plus in the convention, `docket-groom-next`, `docket-auto-groom`,
and `docket-finalize-change` — the same "identical — must not diverge" drift risk that
0053 eliminated for the close-out sequence.

## Strategy (carried forward from the 0053 research, which worked)

Core + one-level-deep reference splits behind loud blocking pointers; provenance narration cut
to bare `(ADR-NNNN)` pointers only where a why is load-bearing; single-sourcing of shared
sequences; byte-stable kept headings verified by an anchor grep-gate; explicit numbered
imperatives preserved for small-model-pinned skills (`docket-status`). New this round, by
explicit decision (brainstormed 2026-07-16, human-approved): **one mechanical shift is
allowed** — moving the board-pass retry loop into the script — and a **size-budget test**
guards against silent regrowth.

## Decisions (brainstormed 2026-07-16, human-approved)

1. **Scope: one change (0085) covers all skills** — the trio split of 0053–0055 existed
   because the reference files were being invented; this round re-applies proven moves.
2. **Not strictly behavior-neutral:** small mechanical shifts are allowed where they delete
   the most-duplicated prose at the root. Exactly one is in scope: the board-pass
   `--must-land` flag (§1 below). Everything else remains behavior-neutral restructure.
3. **Regrowth guard: a size-budget test** (not a status advisory, not nothing) — a future
   change that bloats a skill must slim elsewhere or consciously raise the budget in-diff.
4. **Approach A over minimal-core:** the aggressive router-style convention stays rejected
   (partial-read risk, small-model degradation — same grounds as 0053).

## Design

### 1. Board-pass `--must-land` (the one mechanical shift)

`scripts/docket-status.sh --board-only` gains a `--must-land` flag:

- The bounded retry — on `board inline changed push-failed` ONLY: re-sync the metadata tree
  and re-render, 3 attempts total — moves inside the script.
- Exit 0 ⇔ the run ended on a terminal success line (`board inline changed pushed`,
  `board inline clean`, `board off`, `board github ok`). Any other terminal line (or retry
  exhaustion) prints its report line and exits non-zero.
- The report-line vocabulary is unchanged; flagless behavior is byte-identical to today.
- `scripts/docket-status.md` documents the flag; new script tests cover the retry bound,
  exit-code mapping, and flagless neutrality.

Callers collapse to one line plus posture:

- **Must-land callers** (`docket-new-change` ×3 sites, `docket-groom-next`,
  `docket-auto-groom`, `docket-finalize-change`, `docket-implement-next` where must-land):
  invoke with `--must-land`; non-zero exit → STOP and surface (abort-and-report).
- **Best-effort callers** keep the flagless call and log-and-continue.
- The convention's ~25-line "Board refresh on status writes" paragraph compresses to ~8
  lines: the facade call, the no-surfaces-forwarded rule, derived-view-never-trails, and the
  two posture sentences. The closed-report-channel and per-line semantics live in the script
  contract.

### 2. docket-convention re-slim — 329 L / 5,453 w → ≤ ~200 L / ≤ ~2,600 w

- **Learnings ledger → `references/learnings.md`** (the largest regrowth, ~60 lines).
  Inline keeps ~8 lines: what the ledger is, the two-step read contract (index always, finding
  files on relevance), the `learnings.enabled` read/write gate, who reads (implement-next at
  plan+review, groom-next, auto-groom) and who writes (the finalize harvest only), plus a
  loud blocking pointer for structure, frontmatter, harvest, promotion, and cap mechanics.
- **Bootstrap guard:** keep the 2×2 table and verdict actions; cut surrounding prose to
  pointers at `scripts/docket-config.md`.
- **Agent layer:** the composition paragraph's load-bearing rules (foreground = actively
  block, never background-and-yield; a bare `completed` is not proof; never adopt a child's
  uncommitted files) survive in meaning but tightened; expansion detail joins
  `references/agent-layer.md`.
- **Branch model / terminal-publish / hooks:** narration cuts; change-number archaeology
  deleted or reduced to `(ADR-NNNN)` pointers.
- **Skill layer:** the roles table stays; bullets tightened.

### 3. Operating-skill re-slim (all nine)

| Skill | Now | Target |
|---|---|---|
| docket-finalize-change | 157 L / 2,821 w | ≤ ~140 L / ≤ ~2,200 w |
| docket-status | 114 L / 2,434 w | ~100 L / ≤ ~1,700 w |
| docket-implement-next | 108 L / 2,491 w | ~100 L / ≤ ~2,100 w |
| docket-adr | 90 L / 1,402 w | ~80 L |
| docket-groom-next | 75 L / 1,527 w | ~65 L |
| docket-new-change | 59 L / 1,549 w | ~55 L / ≤ ~1,100 w |
| docket-auto-groom | 64 L / 1,256 w | light narration pass |
| docket-brainstorm | 78 L / 653 w | light narration pass |

Levers: the board-litany collapse (§1), Step-0 sections re-compressed to the ~3-line
convention citation, provenance cuts. The small-model constraint holds: `docket-status` keeps
every step an explicit numbered imperative; cuts remove duplication and narration, never step
explicitness. Gate flow, sign-off rule, and abort-and-report sets in finalize survive
verbatim in meaning.

### 4. References trim

`references/agent-layer.md` (165 L) and `references/terminal-close-out.md` (135 L) get the
same narration pass; each stays one level deep, ≤ ~150 L, with a leading TOC if > 100 L.
`github-board-mirror.md` is untouched (already right-sized).

### 5. Size-budget test (the regrowth guard)

New `tests/test_skill_size_budgets.sh`: a budget table of per-file **max lines and max
words** covering every `skills/**/*.md` (templates included). Budgets set ~10% above
post-slim actuals (exact numbers fixed at plan/build time once the slim lands). Exceeding a
budget fails the suite; the raising-the-budget path is a conscious in-diff edit of the table.

### 6. Verification (build-time acceptance)

1. **Anchor grep-gate:** no file in `skills/`, `agents/`, `scripts/`, or `tests/` references
   a section heading that no longer exists; kept headings stay byte-stable.
2. **Behavior-neutrality diff review:** every deleted sentence is (a) narration, (b) restated
   inline elsewhere, (c) moved to a reference, or (d) covered by a script contract — the sole
   exception is the named `--must-land` shift, covered by its own script tests.
3. **Sentinel re-anchoring:** test files that grep skill prose re-point to moved text,
   preserving each assertion's intent (0053/0055 precedent).
4. **Smoke run:** post-refactor `docket-status` end-to-end on this repo; board-pass callers
   exercised via the existing suite.
5. **Size targets asserted** by the new budget test itself.

## Out of scope

- Any semantics change beyond the `--must-land` flag.
- Frontmatter `description:` lines, templates' content, agent wrappers, `sync-agents.sh`.
- `github-board-mirror.md`; script behavior other than `docket-status.sh`.

## Open questions

- Exact per-file budget numbers — fixed at build time from post-slim actuals + ~10%.
