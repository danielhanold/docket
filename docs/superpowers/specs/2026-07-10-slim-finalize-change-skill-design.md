# Design: slim docket-finalize-change — rewire close-out to the shared reference

**Date:** 2026-07-10 · **Change:** 0054 · **Depends on:** 0053 (feat/slim-convention-status-skills, in-progress at design time)

## Problem

`docket-finalize-change` is the largest operating skill — **234 lines / 3,529 words** (verified
on `origin/main` 2026-07-10). Change #0053 creates
`docket-convention/references/terminal-close-out.md` as the single source of the shared
close-out sequence (archive → re-render `## Artifacts` → terminal-publish → cleanup → board,
plus the per-caller failure-posture table); finalize still restates that sequence under
"identical — must not diverge" warnings, claims single-source ownership of terminal-publish,
and carries verbose gate/selection narration, the full Step-0 preamble, and provenance
archaeology. #0053's spec categorized it **high optimization potential / medium-high risk** —
the merge gate is docket's highest-blast-radius path.

## Decisions (brainstormed 2026-07-10, human-approved)

1. **The rebase-retest merge gate stays inline, compressed** — no reference file. The gate runs
   on every finalize (unless `gate: off`), so it is squarely ON the common path; extraction
   would add a mandatory read hop with no token savings (the LEARNINGS #20 rule: extract only
   sections that are heavy AND off the common path). Compression comes from cutting narration,
   never from moving or weakening the flow. This resolves the stub's open question.
2. **ADR-0002 gets a dated `## Update`** — its recorded "terminal-publish single-sourced in
   finalize" clause goes stale when this change strips finalize's ownership claim. The update
   notes the doc home moved to `docket-convention/references/terminal-close-out.md`
   (via #0053/#0054) and that the single-source *principle* is unchanged. The change lists
   `adrs: [2]` so terminal-publish re-copies the updated ADR onto the integration branch at
   merge (the #0017 lesson: never a standalone push).
3. **The *Harvest learnings* step stays finalize-owned and substantially intact.** Harvest is
   NOT part of #0053's reference (which covers archive → re-render → publish → cleanup →
   board); the convention's *Learnings ledger* section and `docket-status`'s sweep both cite
   "the *Harvest learnings* step in `docket-finalize-change`, its single source" by name. The
   step's heading stays byte-stable.
4. **Behavior-neutral** (inherited from #0053 decision 3): the gate flow, the two-agent split
   at rebase-completion, the sign-off rule, the full abort-and-report set, the Selection
   matrix + explicit-id-overrides-`require_pr_approval` rule, and close-out ordering all
   survive **in meaning**. No contract semantics change.

## Design — section by section

Target: **234 → ≤ ~140 lines, 3,529 → ≤ ~2,200 words** (asserted in the PR description).

| Section | Now (approx) | Plan |
|---|---|---|
| Overview | ~15 L | ~6 L — drop bookend narration; **delete the "this skill is its single source" ownership claims** (both the Overview's and the Terminal-publish section's — ownership transfers to the reference) |
| When to use | 7 L | Keep as-is (already lean) |
| Convention / Step-0 | ~9 L | ~3 L — cite the convention's new *Step-0 preamble* section (#0053 §3): "Run the convention's Step-0 preamble; this skill's writes land on `metadata_branch` + the integration branch (merge)" |
| Selection | ~35 L | ~20 L — the classification matrix table, the "eligible" definition, and the explicit-id-overrides-`require_pr_approval` rule survive in meaning; the prompt-rationale narration compresses |
| Per-change steps | ~70 L | Numbered structure (1, 2, 2.5, 3, 4, 5, 6) stays. Step 2.5 (harvest) intact per decision 3. Step 3's archive → re-render → publish prose collapses to a **loud blocking pointer** at `references/terminal-close-out.md` plus the finalize-only facts: UTC merge date via `gh` `mergedAt`, the `--results` flag, and finalize's failure posture (**abort-and-report** — finalize's row in the reference's posture table). Step 4 (cleanup) → ~2 L invoking the script + trusting the exit code. The trailing "identical to docket-status's sweep — must not diverge" note is **deleted**: the reference is the single source; the warning's duplication was the drift risk |
| The rebase-retest merge gate | ~95 L | ~65 L inline. **Survives in meaning:** the `finalize:` config block (including `require_pr_approval` — documented here, not in the convention, per the #21 doc-ownership precedent), the 6-step flow, the two-agents boundary (① resolves conflicts during the rebase, never runs tests; ② owns the red suite after the rebase lands), the sign-off rule (interactive prompt vs autonomous force-push-and-stop), the full abort-and-report set, and the PR-comment durable-reason rule. **Cut:** the why-rebase explanation paragraph, restated wrapper/model-resolution prose (the convention owns it), and narration that carries no rule |
| Where finishing-a-development-branch fits | ~8 L | ~5 L |
| Terminal publish (docket-mode) | ~45 L | ~8–10 L — a compact pointer section: finalize drives the `done` transition per `references/terminal-close-out.md` (`T = <id>`); ADR-only publish (`T = adr-<NN>`) points at `docket-adr` + `scripts/terminal-publish.md`. Safe to collapse: the kill origins and `docket-adr` cite the script contracts and "the same sequence finalize uses" in prose — no heading-anchor dependency (verified by grep 2026-07-10) |

Cross-cutting: provenance narration (change 0015/0029/0035 archaeology) is cut per #0053
decision 2 — bare `(ADR-NNNN)` pointers remain where a why is load-bearing.

## Verification (build-time acceptance)

1. **Sentinel re-anchoring:** seven test files grep finalize's SKILL.md
   (`test_finalize_gate.sh`, `test_closeout.sh`, `test_docket_metadata_branch.sh`,
   `test_learnings_ledger.sh`, `test_board_refresh_on_transition.sh`,
   `test_results_artifact.sh`, `test_docket_config.sh`). Re-anchor each failing sentinel
   deliberately, preserving the assertion's intent — positive anchors on unique phrases the
   target clause owns; never relocate a must-preserve substring just to pass (the #36/#37
   twin-sentinel lessons). Treat every failing sentinel as a review prompt: does the
   compressed prose still carry the invariant?
2. **Anchor grep-gate:** no skill, agent wrapper, or script contract cites a finalize heading
   that no longer exists; the *Harvest learnings* heading stays byte-stable.
3. **Behavior-neutrality diff review:** every deleted sentence is (a) narration, (b) restated
   inline, (c) covered by `references/terminal-close-out.md`, or (d) covered by a script
   contract. No invariant simply vanishes.
4. **No literal model/effort tier** appears in the two agent-dispatch clauses (existing #17
   guard — compression must not reintroduce one).
5. **Live smoke:** this change's own close-out — finalizing #0054's merged PR — exercises the
   slimmed skill end-to-end (gate + harvest + archive + re-render + publish + cleanup + board).
6. **Size targets asserted** in the PR description: ≤ ~140 lines / ≤ ~2,200 words.

## Design-ahead note (reconcile obligations)

This spec is written while #0053 is in-progress. The implementer's reconcile pass must
re-validate against the landed #0053 before build:

- the reference file's actual path/name and section structure (this spec assumes
  `skills/docket-convention/references/terminal-close-out.md` with a per-caller
  failure-posture table containing a finalize row);
- the convention's final *Step-0 preamble* section name (the ~3-line citation must use the
  landed heading verbatim);
- whether #0053's build already touched any finalize cross-reference this spec assumes intact.

## Out of scope

- Any semantic change to gate behavior, selection matrix behavior, sign-off, or close-out
  ordering.
- The other skills — #0053 (convention + status) and #0055 (implement-next + small skills).
- New scripts, script-contract changes, or frontmatter `description:` rewrites.
