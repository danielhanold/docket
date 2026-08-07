---
id: 249
slug: build-worker-contract-gate-execution-pointer-and-staging-dis
title: 'Build-worker contract: gate-execution pointer and staging discipline'
status: proposed
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [232]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Consolidates #0232 and #0238 (2026-08-07 triage): two single-clause additions to the same file (`skills/docket-build-task/SKILL.md`) with guards in the same test file (`tests/test_docket_build.sh`).

Verified 2026-08-07:

- **Gate posture never reaches the workers (#0232).** 0223's gate execution posture lives in `skills/docket-build/SKILL.md` + `references/gate-execution.md`, but `docket-build-task/SKILL.md` contains no occurrence of `gate-execution`, "execution posture", "foreground ceiling", or "split" — and workers routinely run the full suite: four workers hit the foreground ceiling and three stalled on 0223's own build. Likely shape per the stub: one pointer + one line (reference-not-restatement, per the 0154 house policy).
- **Staging is unconstrained (#0238).** The worker contract's Scope section carries 0231's amend prohibition but says nothing about *staging*: nothing forbids `git add -A` / `commit -a`, so a worker can sweep another agent's or a human's dirty paths into its one commit — the actual mechanism of the 0223 incident. "The commit" section constrains how many commits, not what is in them. Zero occurrences of "stage" or "git add" in the file.

## What changes

- Add the gate-execution-posture pointer to `docket-build-task/SKILL.md` (pointer, not restatement; decide whether the whole posture or just split-never-yield is referenced — default: the pointer covers the whole reference file).
- Add the staging-discipline clause: a worker stages only paths its task touched — with an explicit carve-out for the escalation flow (a stronger worker dispatched into a worktree already holding the weaker worker's uncommitted changes must be able to revise/replace them). Address observability honestly: where a task regenerates a derived file, "what your task changed" is defined by the task contract, not by `git status` diffing.
- Guards for both clauses in `tests/test_docket_build.sh`.
- Raise `docket-build-task/SKILL.md`'s size-budget row if the additions require it (0224 is already raising `docket-build/SKILL.md`'s).

## Out of scope

- Mechanical enforcement of staging scope (hooks, wrappers) — contract prose + guard only, matching how 0231's amend rule landed.

## Open questions

- The exact escalation carve-out wording — the one genuinely normative decision here; the critic may push this to a human.
