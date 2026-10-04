---
id: 504
slug: 'guard-two-unproven-invariants-the-testdata-ignore-negation-a'
title: 'Guard two unproven invariants: the testdata ignore negation and the root-anchored receipt read'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [305, 378]
discovered_from: [320, 380]
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

Consolidates #320 and #380. Both are follow-ups that a review surfaced and nobody closed: a behaviour that is correct today but that no committed test proves, so a regression would pass the suite silently.

- **#320 (from #305)** — the repo-wide `.docket.local.yml` ignore swallowed the repository-local config fixtures under `testdata/repositories/`. The durable fix is a nested `testdata/repositories/.gitignore` negation (`!**/.docket.local.yml`), kept outside the managed docket gitignore block. Only two manual `git check-ignore` probes ever proved it. If the nested file is deleted, or the managed block is rewritten so it overrides the negation, a new fixture silently never gets committed and no test notices.
- **#380 (from #378)** — the metadata-ownership verifier reads its `OpInitRoot` / `OpMigrateSeed` receipt trailers only at the sole parentless seed root. No test pins that anchoring: a regression that accepted a receipt trailer from a descendant commit would go uncaught. The 0378 mutation audit and deep review both flagged it; the fixture that closed it was authored, then reverted with 0378's other non-blocker fixes.

Both gaps have the same shape (an unprobed residual, not an undetectable one) and the same remedy (one mutation-proven guard each), so they ship as one change.

## What changes

Add two regression guards, each mutation-proven (strip the guarded behaviour, watch the guard redden):

1. A guard proving a new `.docket.local.yml` under `testdata/repositories/` is NOT ignored by git, and that the deciding rule is the nested negation rather than an incidental user-global excludesfile.
2. A negative ownership fixture that places a valid receipt trailer on a descendant commit (not the seed root) and asserts the verifier does not accept it as an ownership proof. The reverted 0378 fixture is the starting point if it can still be recovered.

## Out of scope

- Changing the ownership verifier's behaviour or the ignore layout — both are correct today; this is test coverage only.
- Restructuring the managed gitignore block, or a general ignore linter for the rest of the repo.
- Touching the fixtures under `testdata/repositories/` themselves.
- The other 0378 follow-ups (SHA-256 width fix; internal/process flake).
