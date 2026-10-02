---
id: 494
slug: 'a-publish-killed-mid-flight-wedges-its-run-s-cancel-and-clos'
title: 'A publish killed mid-flight wedges its run''s cancel and closeout'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-10-02'
updated: '2026-10-02'
depends_on: []
stacked_on:
related: [444, 491]
discovered_from: [491]
adrs: [124]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0124](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0124-successful-run-ownership-closeout-extends-the-run-epoch-life.md) |
<!-- docket:artifacts:end -->

## Why

Found while grooming 0491 (2026-10-02). It is confirmed only by reading the code on `main` (756fea9fe); nothing has reproduced it yet.

Before `pr.publish` or `workspace.publish` does its remote work for a tracked run, `admitWorkflowMutation` (`internal/app/runtracker_fence.go`) appends an `admitted` entry to the owning run's mutation journal. The same process moves the entry to `completed` or `uncertain` when the remote work returns (`mutationJournalOutcome`). Metadata transactions are not affected: `MutationAdmissionHook` marks their entry `completed` at admission.

If the command is killed in between (Ctrl-C, a coordinator interrupt, or a crash during the GitHub call or the push), the entry stays `admitted` for good:

- **Nothing settles it.** `settleUncertainPublications` and `publicationRetryMatch` (`internal/app/runtracker_publication.go`, change 0444) settle only `uncertain` entries. Even a later identical publish that completes and is verified leaves the `admitted` entry in place.
- **The success closeout never finishes.** The keyed closeout counts the entry as outstanding (`accountCompletionMutations` → `mutation-pending:<op>`). `run.verdict` prints `run-stop <key> run-tracker-unavailable completion-unaccounted` on every call, and the run stays `completing`.
- **Cancel never finishes.** `run.cancel` counts the entry the same way (the journal step of `reconcileRunTeardown`, and `verifyTerminalRunQuiescence` on a repeat cancel). Cancel stays `cancellation-pending` for good, and `run.start --resume` refuses the change.

No docket operation clears the entry. The only remedy is hand-editing the run record under `.git/docket/run-tracker/<key>/`.

A likely real-world path: the coordinator is interrupted during implement-next's final branch push or PR creation. The human re-runs it, and the retry publishes successfully. The run can still neither complete nor be cancelled.

## What changes

These are hypotheses for grooming to evaluate, not decisions:

- **Let a later verified identical retry settle a stale `admitted` publication**, the way it already settles an `uncertain` one (`publicationRetryMatch`). This reuses 0444's machinery. The open point is proving that the process which wrote the `admitted` entry is gone, not still mid-publish, before settling it.
- **Or verify the postcondition directly** for a stale `admitted` entry: the PR exists at the recorded head, or the remote branch is at the recorded head. This adds GitHub and git reads to the closeout or cancel path.
- **Or have the publish record its entry as `uncertain` at a safe earlier point**, so a crash leaves an entry the existing retry match can settle.
- Whatever settles it, decide what `run.cancel` does over an entry whose outcome can never be known: stay pending (today), or report it as a finding and finish.

Any new check states its failure posture up front, and prefers making the problem visible over halting a run.

## Out of scope

- The never-started test run that wedges `run.verdict` (0490 review finding F3), and the missing test-run folder. 0491 fixes both.
- Metadata transactions, whose journal entry is completed at admission.
- The run id and its retirement (0491).
