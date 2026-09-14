---
id: 428
slug: 'recover-legacy-gate-history-without-blocking-unrelated-workt'
title: 'Recover legacy gate history without blocking unrelated worktree admission'
status: 'proposed'
priority: 'critical'
type: 'fix'
created: '2026-09-14'
updated: '2026-09-14'
depends_on: []
stacked_on:
related: [375, 427]
discovered_from: [375]
adrs: [87, 95, 118]
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
| ADRs | [ADR-0087](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0087-liveness-probe-non-zero-is-not-evidence-of-death.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md) |
<!-- docket:artifacts:end -->

## Why

Testing Docket after change 0375 on an existing consumer repository exposed a first-admission blocker: schema-2 gate-drive history under the repository's Git common directory is rejected by the newer reader, and the repository-wide legacy inventory reports unresolved-execution before unrelated work can start. The current source writes schema 4 and accepts schema 3; schema 2 remains unsupported. The reported baseline gate.drive.start returned invalid-input / unresolved-execution with the misleading message 'a prior execution in this worktree is unresolved; recover it through the parent or run.cancel, never a blind re-start' and no incumbent drive locator. No TDD cycle, edits, or commit occurred. Scope 08ae86c490d6d19fc55602e39ba0482d had drive_count:0 and closed:false. Empty workspace operation.lock (workspace for consumer change 16, c610db33…) and gate-admission/v1/c4725d99…/lock files were observed and left untouched; empty lock files alone do not establish a live or abandoned execution. Source inspection confirms inventoryLegacyDrives fails on unsupported records before filtering by worktree, while the application discards the internal inventory-legacy-drive-<id> operation locator in its public refusal.

## What changes

Provide a supported, idempotent recovery/cleanup command for legacy gate history so safely resolved history cannot permanently poison first admission of unrelated worktrees. Define explicit handling for known schema-2 records and retain fail-closed behavior for genuinely unknown, corrupt, or potentially live state; never equate an unsupported schema with a dead process. Preserve admission isolation, run cancellation authority, ownership fences, and recorded history. Carry the safe inventory-legacy-drive-<id> locator through machine-readable and human refusals and name a recovery action that works for the identified state. Scope cleanup eligibility, repeat and interrupted-run behavior, concurrency locking, and evidence retention in the design. Add regression coverage for upgrade-era repositories, unrelated first admissions, accurate locators, idempotence, ambiguous/live records, and cleanup racing admission. The user proposes a new cleanup command; exact CLI placement and default apply-versus-preview behavior remain design decisions.

## Out of scope

Implementing the fix during change capture; manually deleting consumer .git state or empty lock files; blanket purges, guessed schema upgrades, or treating unknown state as proof of teardown; changing gate budgets or weakening single-execution/cancellation guarantees; the separate verdict-path epoch binding fix tracked by 0427.
