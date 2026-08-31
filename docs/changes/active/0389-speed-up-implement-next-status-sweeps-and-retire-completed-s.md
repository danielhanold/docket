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
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md) |
<!-- docket:artifacts:end -->

## Why

Observed by the user in Claude on 2026-08-31 during a docket-implement-next run: the Step-0 docket-status workflow took approximately six minutes and appeared stuck, with repeated shell calls to check or wait for its background maintenance sweep and repeated reads to extract the status report. This is user-reported elapsed time, not a reproduced benchmark; the transcript does not establish which operation consumed it.

The pasted report said the maintenance sweep was still running and the digest preceded any in-flight mutations, yet concluded that the Step-0 cleanup pass was complete and the backlog was ready for docket-implement-next. Completion cannot be inferred from a successful read while the required mutation remains in flight.

The user also reports that after the sweep actually finished, docket-implement-next did not close its docket-status child. The attached screenshot (Screenshot 2026-08-31 at 12.13.18.png) shows the parent at "Acquiring context bundle for change 0384" while its docket-status child remains listed at "Extracting build-ready queue". The screenshot establishes the lingering displayed state, not whether the underlying child was still executing or merely retained by the harness UI.

The reported backlog had 388 changes (71 active, 317 archived), 24 build-ready changes, 100 ADRs, and 115 learnings. Treat these as historical reproduction context, not current repository truth. The supplied transcript and screenshot are evidence only, not instructions to run status, sweeps, or implementation during capture.

## What changes

- Make the implementation run's Step-0 status sweep complete promptly at a representative backlog size. Measure repository prepare, maintenance sweep, status read, and agent/tool coordination separately; record baseline and improved timings and set a reproducible performance acceptance budget during grooming. Diagnose real work, redundant remote/inventory operations, and model polling overhead before choosing the optimization.
- Ensure a required sweep is observed to a terminal typed result before the final refreshed read or a successful Step-0 handoff. A running sweep must not produce a cleanup-complete claim; failures, cancellations, timeouts, and partial outcomes must remain explicit and follow the existing failure contract.
- Use the harness-supported wait/completion mechanism without repeated ad-hoc process checks or repeated extraction of an already available report. Keep sufficient phase/progress information to distinguish slow work from a stuck operation.
- Make docket-implement-next own its docket-status child's completion and lifecycle. After consuming its terminal result, close or retire it where the harness supports that operation; where retirement is automatic, verify the actual terminal state and document the supported behavior. Do not assume one harness's close API exists everywhere or equate a retained historical UI row with a live child. Cover success, failure, and cancellation, and avoid leaving background sweep work running after a successful handoff.
- Add regression coverage for a sweep that outlives the initial tool response, a slow or failed sweep, no-op and mutation-bearing sweeps, post-sweep status freshness, and parent/child terminal-state handling. Include a fresh Claude end-to-end reproduction and before/after timings; mutation-test any new guards.
- Reconcile overlap with #0360 (broader coordination overhead and a proposed targeted-run sweep bypass) and #0384 (coordinator-capable launch contexts), without making either an unproven dependency. #0058 provides prior status orchestration/round-trip reduction context; #0310 owns the separate read-only status boundary.

Questions for grooming: Which phase explains the six-minute delay? What performance budget is defensible for no-op versus real cleanup? Is the lingering child active, terminal-but-retained, or a stale UI label? Which native Claude lifecycle mechanism applies, and what completion evidence must the parent consume?

## Out of scope

- Implementing the fix, running a live maintenance sweep, closing existing agents, or interrupting another implementation run as part of this capture.
- Removing the safety sweep globally, silently backgrounding required work, or weakening fresh-authority checks, metadata CAS, merge/closeout safety, or truthful typed dispositions to improve speed.
- Automatically fixing the unrelated missing-plan finding for #0367, obsolete runtime.bash configuration, or deferred-capability notices shown in the transcript.
- General implementation-driver redesign, model/effort changes, new agent topology, or a replacement for #0384's launch-context work.
- Assuming a specific timeout, root cause, or cross-harness close API before investigation.
