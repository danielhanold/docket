---
id: 389
slug: 'speed-up-implement-next-status-sweeps-and-retire-completed-s'
title: 'Speed up implement-next status sweeps and retire completed status children'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-08-31'
updated: '2026-08-31'
depends_on: []
stacked_on:
related: [58, 310, 360, 384]
discovered_from: [384]
adrs: [12, 24]
spec: 'docs/superpowers/specs/2026-08-31-speed-up-implement-next-status-sweeps-and-retire-completed-s-design.md'
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
| Spec | [2026-08-31-speed-up-implement-next-status-sweeps-and-retire-completed-s-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-31-speed-up-implement-next-status-sweeps-and-retire-completed-s-design.md) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md) |
<!-- docket:artifacts:end -->

## Why

The 2026-08-31 Claude implementation run for #0384 returned its Step-0 docket-status report after about six minutes while the maintenance command was still running. The parent proceeded toward claim; roughly thirty minutes after dispatch, the same child resumed to collect the real sweep result and notified the parent during the build phase. The parent's “duplicate stop; effects already landed” explanation did not establish prior sweep completion.

The matching Claude Code 2.1.251 logs and terminal JSON show 234 historical cleanup attempts, all cleanup entries: 33 applied and 201 blocked (192 workspace-blocked, 9 not-finalizable), with no merge-closeout entries. Current sweepWorklist schedules every done/stacked-merged record, causing repeated authoritative reloads and cleanup probes. The six-minute report was premature, not the actual end of the work. The linked spec preserves the timestamps, evidence locations, and limits of this diagnosis.

The user approved moving historical cleanup retries out of implementation startup and into explicit maintenance. Startup should retain current merge recovery without paying for the entire archive, and it must not advance on an unfinished child or misread a partially successful sweep as clean success.

## What changes

- Add an explicit implementation scope to maintenance sweep while preserving full maintenance as the default. Startup recovers current merged changes, their safe cleanup suffixes, and configured claim reclamation; independent historical cleanup retries remain in explicit full maintenance.
- Wire docket-implement-next's existing docket-status child to that scope. Preserve the registered-agent topology and fresh-origin/CAS safety.
- Require terminal command evidence, per-item outcome validation, a post-sweep status read, and verified child completion/retirement before selection. Handle failures, cancellation, and late notifications without abandoning work or confusing another worker.
- Verify zero per-historical-record cleanup work at startup, measured before/after performance, mutation-tested completion guards, and fresh-process Claude lifecycle behavior.
- The linked spec settles the design. #0360 and #0384 remain related work, not dependencies.

## Out of scope

- Full historical-cleanup optimization or removal; automatic scheduling; changing cleanup ownership, merge, or exact-ref safety.
- New agent topology, runner fallbacks, model/effort changes, generalized driver redesign, or skipping Step 0 for targeted runs.
- Automatically repairing the unrelated plan/config findings from the original report.
- Implementing code, running a live sweep, closing existing agents, or altering another implementation run during this grooming operation.
