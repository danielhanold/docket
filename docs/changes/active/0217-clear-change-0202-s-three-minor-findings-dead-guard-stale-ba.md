---
id: 217
slug: clear-change-0202-s-three-minor-findings-dead-guard-stale-ba
title: Clear change 0202's three minor findings: dead guard, stale baseline comment, wrong plan pattern
status: proposed
priority: medium
type: chore
created: 2026-08-05
updated: 2026-08-05
depends_on: []
related: [200]
discovered_from: [202]
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

Three `minor` review findings from change 0202 were left unfixed at merge. Individually each is
cosmetic; together they are the same class 0202 exists to close — accurate-looking prose sitting
beside code that no longer matches it, which is exactly how 0113's findings aged into 0202.

## What changes

- `scripts/board-checks.sh` — `[ -n "$boa_p" ] || continue` in `branch_only_artifact` is
  unreachable under `-z` (`ls-tree -z` emits no empty records). It is now unguarded dead code
  sitting beside a carefully-argued comment block; remove it, or state why it is kept.
- `tests/test_board_checks.sh` — the mutation-baseline comment says the baseline "fires exactly the
  three expected findings"; it was already stale at four asserts, and 0202 widened the gap to six
  findings across five fixtures. Reword to drop the number rather than re-pin it, since re-pinning
  buys a maintenance burden with no guard behind it.
- `docs/superpowers/plans/2026-08-05-clear-the-unfixed-review-findings-from-change-0113.md` — Task 5
  Step 2's verification pattern (`grep -nE '4013|4050|147 for 143'`) matches the comment line that
  *explains* 4050 is stale, so following it literally halts on a false positive. The plan ships on
  the branch as an inaccurate instruction. Fix it in place, or decide that merged plans are
  historical records and are never edited — and record that decision somewhere a future build will
  read it.

## Out of scope

- The two `important` 0202 findings, which have their own stubs.

## Open questions

- Whether a merged plan file is editable at all, or is a frozen build record. This stub's third
  bullet is really that policy question wearing a specific instance.
