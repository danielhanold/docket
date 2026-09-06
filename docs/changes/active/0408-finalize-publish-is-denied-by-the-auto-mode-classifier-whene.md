---
id: 408
slug: 'finalize-publish-is-denied-by-the-auto-mode-classifier-whene'
title: 'Finalize publish is denied by the auto-mode classifier whenever the gate rebases'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-06'
updated: '2026-09-06'
depends_on: []
stacked_on:
related: [404]
discovered_from: [404]
adrs: [40, 42, 43]
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
| ADRs | [ADR-0040](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0040-terminal-publish-default-opt-in.md), [ADR-0042](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0042-auto-approve-consent-model.md), [ADR-0043](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0043-retire-bot-auto-approval-zero-approvals-branch-protection.md) |
<!-- docket:artifacts:end -->

## Why

Closing change 404 (PR #280) on 2026-09-06, `docket-finalize-change` ran the rebase-retest gate cleanly (rebased onto `main`, suite green twice) and then halted at `finalize.publish`: Claude Code's auto-mode classifier soft-denied the operation, which force-pushes the rebased head to the PR and rewrites the build-evidence block. The convention-mandated exact-command retry was denied too, and so was the same command run from the parent interactive session after explicit human intent had been stated — the July recipe (retry after intent) no longer clears it. Only a human-typed `!`-prefixed command landed the push, after which merge, closeout, and cleanup ran without any denial.

This is not a one-off. The denial keys on the force-push shape, so it fires on every finalize whose gate actually rebases — i.e. whenever another change merged first — and it does not fire when the branch is already on top of the integration branch, which is why recent finalizes looked healthy. An autonomous finalize loop cannot type a `!` command, so today the unattended finalize path halts deterministically on exactly the case the rebase-retest gate exists for. That breaks docket's autonomous close-out end to end.

The merge-side classifier wall was solved once before by config (ADR-0043: require-PR / 0-approvals branch protection instead of a bot approver). The publish-side wall has been observed since 2026-07-19 but never captured, and the move of the push into the Go `docket finalize publish` operation appears to have made it strictly worse: the classifier now sees an opaque binary performing an external write, and a retry no longer helps.

## What changes

Make the publish step of `docket-finalize-change` land unattended in an auto-mode session. Investigate and decide between: (a) reshaping the publish so the classifier does not see a force-push-shaped external write from an opaque binary — e.g. a receipt-leased non-force push under `--force-with-lease` spelled as a recognizable Git command, or a two-step where the Go op verifies and a plain Git push executes; (b) a documented, validated `autoMode.allow` posture (user-level settings) that clears the specific soft-deny class, with its blast radius recorded; (c) an in-run recovery contract so the halt writes a `## Finalize blocked` marker naming the exact resume command and the next attended run resumes from publish rather than re-running the gate. Whichever lands, the halt report and README must state the failure and its recovery, and the finalize agent must never re-run a green gate just to retry the push.

## Out of scope

Merging without a rebase (already clean); the merge-without-review classifier (solved by ADR-0043 branch protection); the suite-gate yield-wedge where the finalize agent backgrounds the gate and waits for a notification (a separate contract violation, tracked elsewhere); any change to Claude Code itself.
