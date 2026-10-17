---
id: 557
slug: 'retire-the-suite-runner-s-unused-strict-budget-mode'
title: 'Retire the suite runner''s unused strict budget mode'
status: 'proposed'
priority: 'low'
type: 'refactor'
created: '2026-10-16'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: [528, 273]
discovered_from: [528]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The suite runner's strict budget mode (`DOCKET_RUNTESTS_STRICT=1`) re-checks every slow test immediately and fails the run with exit 4 on a confirmed or doubtful breach. Nothing turns it on: no gate, no `.docket.yml` key, no skill, no CI. The release-candidate workflow gets the same effect its own way, by failing a release when the log carries an `OVER BUDGET:` or `SERIAL CONFIRMED OVER BUDGET:` line. So strict mode is code, tests, an exit code, and docs that every budget change still has to design around (#528 had to decide its behavior under contention) for a mode with no users.

Its user-facing text has also gone stale. When a single-job run goes over budget, the runner tells you to set `DOCKET_RUNTESTS_STRICT=1` "but see `scripts/run-tests.md` first", and that file no longer exists. The strict arm's message says "`--strict-budget` was given", a flag the Go runner does not have.

Surfaced while grooming #528.

## What changes

Remove strict budget mode from the suite runner: the `DOCKET_RUNTESTS_STRICT` environment seam, `Config.Strict`, the strict re-check path (`StrictConfirmCandidates`), exit code 4, the strict and advisory hint lines in the budget report (including the dead `scripts/run-tests.md` pointer and the `--strict-budget` wording), the strict tests, and the strict sentences in `tests/README.md`'s exit contract.

Also decide whether the runner's direct "over budget" reading still earns its place. A single-job run (`DOCKET_RUNTESTS_JOBS=1`) or a serial-mode budget row compares a test straight against `ceiling * 3/2` and prints `OVER BUDGET:`, which the release-candidate workflow fails on. Nothing runs single-job today, no serial-mode rows exist, and its only past users were in the deleted Bash runner. It shares the exit-4 arming with strict mode, so the same change should either retire it or say why it stays.

Finally, fix the stale comment in `.github/workflows/release-candidate.yml` that calls its two greps "the runner's complete budget vocabulary". #528 added the `SERIAL CONFIRMATION NOT COUNTED:` line, which is in neither grep, so the comment is no longer exactly true. The omission is deliberate (that line is meant to fail nothing), so reword the comment to say so rather than adding a third grep. This also makes the "unless a retired line prefix forces an edit there" carve-out below moot for the comment.

## Out of scope

- The scheduled solo re-check and its contention handling (#528).
- Host-relative budgets (#273).
- The release-candidate workflow's own budget check, unless a retired line prefix forces an edit there.
