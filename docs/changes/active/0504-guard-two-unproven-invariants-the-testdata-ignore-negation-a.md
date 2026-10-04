---
id: 504
slug: 'guard-two-unproven-invariants-the-testdata-ignore-negation-a'
title: 'Guard two unproven invariants: the testdata ignore negation and the root-anchored receipt read'
status: 'implemented'
priority: 'medium'
type: 'chore'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [305, 378]
discovered_from: [320, 380]
adrs: []
spec: 'docs/superpowers/specs/2026-10-04-guard-two-unproven-invariants-the-testdata-ignore-negation-a-design.md'
plan: 'docs/superpowers/plans/2026-10-04-guard-two-unproven-invariants-the-testdata-ignore-negation-a.md'
results: 'docs/results/2026-10-04-guard-two-unproven-invariants-the-testdata-ignore-negation-a-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/guard-two-unproven-invariants-the-testdata-ignore-negation-a'
pr: 'https://github.com/danielhanold/docket/pull/377'
blocked_by:
reconciled: true
claimed_at: '2026-10-04T08:53:27Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-guard-two-unproven-invariants-the-testdata-ignore-negation-a-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-guard-two-unproven-invariants-the-testdata-ignore-negation-a-design.md) |
| Plan | [2026-10-04-guard-two-unproven-invariants-the-testdata-ignore-negation-a.md](https://github.com/danielhanold/docket/blob/chore/guard-two-unproven-invariants-the-testdata-ignore-negation-a/docs/superpowers/plans/2026-10-04-guard-two-unproven-invariants-the-testdata-ignore-negation-a.md) |
| Results | [2026-10-04-guard-two-unproven-invariants-the-testdata-ignore-negation-a-results.md](https://github.com/danielhanold/docket/blob/chore/guard-two-unproven-invariants-the-testdata-ignore-negation-a/docs/results/2026-10-04-guard-two-unproven-invariants-the-testdata-ignore-negation-a-results.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #320 and #380. Both are follow-ups that a review surfaced and nobody closed: a behaviour that is correct today but that no committed test proves, so a regression would pass the suite silently.

- **#320 (from #305)** — the repo-wide `.docket.local.yml` ignore swallowed the repository-local config fixtures under `testdata/repositories/`. The durable fix is a nested `testdata/repositories/.gitignore` negation (`!**/.docket.local.yml`), kept outside the managed docket gitignore block. Only two manual `git check-ignore` probes ever proved it. If the nested file is deleted, the tracked fixtures become tracked-but-ignored, a new sibling fixture silently never gets committed, and no test notices.
- **#380 (from #378)** — the metadata-ownership verifier reads its `OpInitRoot` / `OpMigrateSeed` receipt trailers only at the sole parentless seed root. No test pins that anchoring: a regression that accepted a receipt trailer from a descendant commit would go uncaught. The 0378 mutation audit and deep review both flagged it; the fixtures that closed it were authored, then reverted with 0378's other non-blocker fixes.

Both gaps have the same shape (an unprobed residual, not an undetectable one) and the same remedy (a mutation-proven guard), so they ship as one change.

## What changes

Add two regression guards, each mutation-proven (strip the guarded behaviour, watch the guard redden):

1. **No tracked file is hidden by the repository's own ignore rules.** A repo-wide guard in `internal/repoguard` asserts that git reports no tracked file as ignored by the committed `.gitignore` files (machine-local and user-global excludes deliberately not consulted). The testdata negation is one case of this rule: deleting it reddens the guard and names the three tracked fixtures. A committed control proves the probe actually detects an ignored tracked file.
2. **A descendant's receipt never authorizes the seed root.** Restore the two integration fixtures from build 0378 (commit `9c5ced015`, test hunk only). They place a valid `OpInitRoot` / `OpMigrateSeed` receipt on a descendant commit and assert the verifier does not adopt it as the root's ownership proof.

Detailed design, groom-time evidence, and acceptance are in the linked spec.

## Out of scope

- Changing the ownership verifier's behaviour, the ignore layout, the managed gitignore block, or the fixtures under `testdata/repositories/` — all correct today; this is test coverage only.
- The F4 `verifyLegacyEquivalence` refactor that shared commit `9c5ced015` with the fixtures, and the other 0378 follow-ups (SHA-256 width fix; internal/process flake).
- Probing untracked files or synthetic paths (the original #320 single-folder probe) — superseded by the repo-wide guard; a never-added new file is an undetectable residual.

## Reconcile log

### 2026-10-04

Reconciled against main at 2587e6dc7 (same base as groom). testdata/repositories/.gitignore negation still present; verifyMetadataOwnership still scans from own.Root and keeps only s.Commit == own.Root; internal/repoguard exists with default build tag; commit 9c5ced015 is reachable. No scope change.
