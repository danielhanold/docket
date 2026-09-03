---
id: 289
slug: bind-budget-ledger-entries-to-the-numbers-they-narrate
title: 'Bind budget-ledger entries to the numbers they narrate'
status: 'killed'
priority: medium
type: chore
created: 2026-08-11
updated: '2026-09-03'
depends_on: []
related: []
discovered_from: [281]
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

**Trigger** — surfaced during change 0281's review, which returned two separate findings (3 and 4)
with the same root cause: a hand-maintained ledger comment drifted from the code it narrates.
`tests/test_runtime_budgets.sh`'s `EXPECTED_TOTAL` was raised 1670 -> 1680 with no matching
`NNNN -> NNNN (change XXXX)` entry, and `tests/test_skill_size_budgets.sh`'s change-0281 raise entry
stated compression figures (23 words, 12 lines) that no longer matched the final diff (-15 words,
+4 lines). Both blocks exist precisely so a quiet ceiling raise stays auditable, and both were
caught only because a human-equivalent reviewer read them line by line.

**Opportunity** — no mechanism binds a pinned constant or a budget row to its own ledger entry.
Today the rule is stated in prose inside each comment block and enforced by nothing: the asserts
read the numbers, never the narration. A deterministic guard could assert the correspondence —
every `EXPECTED_TOTAL` value has a head-of-block entry naming the change that set it, and every
`BUDGETS` row raised relative to the merge base carries an argued entry naming the change and the
`references/` home it rejected.

**Independent value** — it stands with change 0281 reverted. These two ledgers govern the repo's
only durable anti-regrowth pressure (suite wall-clock and skill size), and their whole design rests
on a raise being visible and argued. An unenforced audit trail decays silently, and the decay is
invisible exactly when the ledger matters most — reading back a raise months later. The same
mechanism would cover any future pinned-constant-plus-rationale pair.

**Boundary** — one guard over the two existing ledger blocks in `tests/test_runtime_budgets.sh` and
`tests/test_skill_size_budgets.sh`: correspondence between a changed number and a ledger entry
naming a change id, plus the ordering convention each block already follows (newest-first). It stops
at structural correspondence and does not attempt to judge whether an argument is *good* — that
stays a human review call. It does not touch the numbers, the rows, or the raise rules themselves,
and it introduces no new ledger format.

**Reason for deferral** — change 0281 is a fix to the auto-groom critic return channel; it touched
these two files only because its own prose grew past their ceilings. Building a general ledger-audit
guard there would expand a prose-contract fix into test-infrastructure work, and the guard needs its
own design pass (what counts as "raised relative to the merge base" in a guard that cannot assume
network access is the real question).

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): superseded by the Go migration — both ledger test files are deleted; skill size budgets are a Go table in internal/repoguard/budgets_test.go with a ratchet rule.
