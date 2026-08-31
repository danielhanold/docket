---
id: 249
slug: build-worker-contract-gate-execution-pointer-and-staging-dis
title: 'Build-worker contract: gate-execution pointer and staging discipline'
status: done
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-08
depends_on: [224]
related: [231, 253]
discovered_from: [232, 238]
adrs: []
spec: docs/superpowers/specs/2026-08-07-build-worker-contract-gate-execution-pointer-and-staging-dis-design.md
plan: docs/superpowers/plans/2026-08-08-build-worker-contract-gate-execution-pointer-and-staging-dis.md
results: docs/results/2026-08-08-build-worker-contract-gate-execution-pointer-and-staging-dis-results.md
trivial: false
auto_groomable: true
branch: feat/build-worker-contract-gate-execution-pointer-and-staging-dis
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/178
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-build-worker-contract-gate-execution-pointer-and-staging-dis-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-build-worker-contract-gate-execution-pointer-and-staging-dis-design.md) |
| Plan | [2026-08-08-build-worker-contract-gate-execution-pointer-and-staging-dis.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-08-build-worker-contract-gate-execution-pointer-and-staging-dis.md) |
| Results | [2026-08-08-build-worker-contract-gate-execution-pointer-and-staging-dis-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-08-build-worker-contract-gate-execution-pointer-and-staging-dis-results.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0232 and #0238 (2026-08-07 triage): two single-clause additions to the same file (`skills/docket-build-task/SKILL.md`) with guards in the same test file (`tests/test_docket_build.sh`).

Verified 2026-08-07:

- **Gate posture never reaches the workers (#0232).** 0223's gate execution posture lives in `skills/docket-build/SKILL.md` + `references/gate-execution.md`, but `docket-build-task/SKILL.md` contains no occurrence of `gate-execution`, "execution posture", "foreground ceiling", or "split" — and workers routinely run the full suite: four workers hit the foreground ceiling and three stalled on 0223's own build. Likely shape per the stub: one pointer + one line (reference-not-restatement, per the 0154 house policy).
- **Staging is unconstrained (#0238).** The worker contract's Scope section carries 0231's amend prohibition but says nothing about *staging*: nothing forbids `git add -A` / `commit -a`, so a worker can sweep another agent's or a human's dirty paths into its one commit — the actual mechanism of the 0223 incident. "The commit" section constrains how many commits, not what is in them. Zero occurrences of "stage" or "git add" in the file.

## What changes

Settled by the linked spec (2026-08-07, autonomous groom — its `## Assumptions` A1–A10 are the
audit trail). Four edits, one diff:

- **Gate-execution pointer** in `docket-build-task/SKILL.md` `## The cycle`: one paragraph pointing
  at the whole of `skills/docket-build/references/gate-execution.md` (the harness-neutral
  capability file — never at `docket-build`'s posture section, whose controller vocabulary a worker
  must not import), plus the worker-shaped consequence inline: never yield, observe by blocking,
  finite observation, unfinished-at-bound is not green (fail closed). Reaches the
  `docket-implement-next` fix-loop workers for free — they run this same contract.
- **Staging discipline** as a new `## Scope` bullet: stage by explicit path, only paths your task
  changed; never `git add -A` / `git add .` / `git commit -a`. "What your task changed" is defined
  by the task contract, not `git status` diffing; an unattributable dirty path is left in place and
  named in NOTES. Escalation carve-out bounded by the task boundary: inherited paths accounted for
  **within the task's scope** are the task's paths; out-of-task strays take leave-and-report.
- **Guards** for both clauses in `tests/test_docket_build.sh` under a change-0249 banner, reusing
  the 0231 Scope extractor; 0231's pins stay green.
- **Size-budget raise** for `docket-build-task/SKILL.md` (measured 122/1087 vs `130 1150` — the
  edits do not fit) per the row's documented rule, from the in-diff measured actual.

## Out of scope

- Mechanical enforcement of staging scope (hooks, wrappers) — contract prose + guard only, matching how 0231's amend rule landed.
- Restating any gate capability or per-harness verdict outside the reference file; edits to `docket-build`, `docket-implement-next`, or the references.

## Open questions

Resolved by the spec (2026-08-07): the pointer covers the whole reference file (A1); the escalation
carve-out was decidable without a human — its permission is merged 0231 contract text, and the
critic-revised wording bounds it by the task boundary (A5). `depends_on: [224]` was `implemented`
(PR #174 open, not merged) at groom time; build only after it merges (A9).

## Reconcile log

### 2026-08-08 — reconciled at claim (docket-implement-next)

Verified against `origin/main` and `origin/docket` at claim time; the design stands unchanged, and
every one of the spec's build-time re-measure instructions (A9) was discharged:

- **A9's gate is satisfied.** `depends_on: [224]` is now `done` and archived
  (`archive/2026-08-07-0224-the-build-gate-contract-never-says-green-red-is-the-exit-code.md`);
  its budget raise for `skills/docket-build/SKILL.md` (`335 3150`) is live on `main`. The
  collision surface A9 predicted (append-adjacency in the two test files, adjacent budget-table
  rows) is exactly what landed — no overlap with `skills/docket-build-task/SKILL.md`.
- **Re-measured the worker file:** `skills/docket-build-task/SKILL.md` is still **122 lines /
  1087 words** on `origin/main`, and its budget row is still `130 1150` — the figures Edit 4 was
  designed against are unchanged, so the 8-line / 63-word headroom finding holds and the raise is
  still required.
- **Re-verified the guard-file tail:** `tests/test_docket_build.sh` is 862 lines, ending in
  change 0224's banner block; the 0231 `worker_scope`/`worker_scope_flat` extractor and its
  non-vacuity companion are present and unmodified at lines 147–170, so Edit 3 appends a 0249
  banner after 0224's and reuses the extractor as designed (A7).
- **Both holes still exist.** `skills/docket-build-task/SKILL.md` has zero occurrences of
  `gate-execution`, "yield", "stage", or "git add" — neither clause has been closed by another
  change in the interim.
- **Couplings re-checked (A10).** 0231 is merged; its Scope prose and its five pinned phrases are
  live and must stay green. 0253 (prose-anchored guard house pattern) is still `proposed` and
  unbuilt, so Edit 3 writes against the current `flat()` + proximity-negation idiom; if 0253 later
  rewrites that house pattern it migrates these asserts along with the rest.
- **Pointer target re-confirmed.** `skills/docket-build/references/gate-execution.md` exists and is
  the harness-neutral capability file (0234 split the evidence out to
  `gate-execution-evidence.md`, which the pointer deliberately does not target). The correct
  relative link from the worker contract is `../docket-build/references/gate-execution.md`.

No scope change, no work dropped, no new constraint. Auto-capture: no discovery clearing the six
admission gates surfaced during this pass — the two remaining ideas in play (mechanical staging
enforcement, gate-posture audit of other worker contracts) are the spec's own recorded
out-of-scope items A8 and the Risks note, not new capability.
