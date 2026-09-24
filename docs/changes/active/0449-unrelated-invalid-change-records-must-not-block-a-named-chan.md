---
id: 449
slug: 'unrelated-invalid-change-records-must-not-block-a-named-chan'
title: 'Unrelated invalid change records must not block a named change''s metadata writes or board'
status: 'in-progress'
priority: 'critical'
type: 'fix'
created: '2026-09-23'
updated: '2026-09-24'
depends_on: [448]
stacked_on:
related: [309, 310, 312, 337, 367, 446, 448]
discovered_from: [446]
adrs: [93]
spec: 'docs/superpowers/specs/2026-09-23-unrelated-invalid-change-records-must-not-block-a-named-chan-design.md'
plan: 'docs/superpowers/plans/2026-09-24-unrelated-invalid-change-records-must-not-block-a-named-chan.md'
results: 'docs/results/2026-09-24-unrelated-invalid-change-records-must-not-block-a-named-chan-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/unrelated-invalid-change-records-must-not-block-a-named-chan'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-24T15:43:07Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-23-unrelated-invalid-change-records-must-not-block-a-named-chan-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-23-unrelated-invalid-change-records-must-not-block-a-named-chan-design.md) |
| Plan | [2026-09-24-unrelated-invalid-change-records-must-not-block-a-named-chan.md](https://github.com/danielhanold/docket/blob/fix/unrelated-invalid-change-records-must-not-block-a-named-chan/docs/superpowers/plans/2026-09-24-unrelated-invalid-change-records-must-not-block-a-named-chan.md) |
| Results | [2026-09-24-unrelated-invalid-change-records-must-not-block-a-named-chan-results.md](https://github.com/danielhanold/docket/blob/fix/unrelated-invalid-change-records-must-not-block-a-named-chan/docs/results/2026-09-24-unrelated-invalid-change-records-must-not-block-a-named-chan-results.md) |
| ADRs | [ADR-0093](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0093-repository-reference-severity-graded-by-structural-role.md) |
<!-- docket:artifacts:end -->

## Why

Every metadata transaction requires the whole corpus to be error-free: the engine refuses a base with errors before the operation plans anything. So one malformed or invalid unrelated change blocks another change's claim, lifecycle writes, and final archive. The canonical board renderer likewise aborts on one bad record. Named reads fetch live branch facts for every stack branch in the repository, not just the named change's. Split out of change 446 (its former design section 8).

## What changes

- Add a bounded in-memory validation subject set to existing metadata operations. Errors in the named change and in records it actually requires still refuse. Unchanged pre-existing errors in unrelated records are reported but do not refuse. Any new or changed error anywhere still refuses.
- Keep the complete corpus read, exact-version checks, evolution checks, declared-path enforcement, and lease/replay machinery. Operations without a subject contract stay strictly whole-corpus.
- Thread this through the named implementation and finalize mutation population, including final archive/closeout.
- The canonical board renders usable records and shows a repair notice for unrenderable ones instead of aborting.
- Bound named live branch-fact probes to the named change's base/stack.
- Record the departure from change 309's error-free-corpus contract.

Built last, after changes 446 and 448.

## Out of scope

User-supplied ignore lists, error-code allowlists, persisted baselines, a second transaction engine, an alternate renderer, new lifecycle statuses, or permissive parsers. Read-only health checks keep reporting the whole repository. Gate admission and run bookkeeping (change 446). Named-start maintenance preflight (change 448).

## Reconcile log

### 2026-09-24

2026-09-24 — Reconciled at claim against main 9d4cb1fe. Dependency 0448 and the split-origin change 0446 are both done. The code anchors the spec grounds on are still present unchanged in shape: `Engine.runCandidate` still refuses on `before.Report.HasErrors()` before planning (internal/repository/transaction/engine.go) and `planningLoader.Load` still lives in internal/app/planning.go. No work landed elsewhere that covers this scope; scope, relations, and design stand as written.
