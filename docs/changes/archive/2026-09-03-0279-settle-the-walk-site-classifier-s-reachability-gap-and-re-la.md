---
id: 279
slug: settle-the-walk-site-classifier-s-reachability-gap-and-re-la
title: 'Settle the walk-site classifier''s reachability gap and re-land 0258''s reverted fixes'
status: 'killed'
priority: medium
type: chore
created: 2026-08-09
updated: '2026-09-03'
depends_on: []
related: [251]
discovered_from: [258]
adrs: []
spec: docs/superpowers/specs/2026-08-09-settle-the-walk-site-classifier-s-reachability-gap-and-re-la-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-settle-the-walk-site-classifier-s-reachability-gap-and-re-la-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-settle-the-walk-site-classifier-s-reachability-gap-and-re-la-design.md) |
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced at #0258's close-out. That change's review fix loop authored four
mutation-proved fixes and had all four reverted, because the last one added
`git -C "$REPO" ls-files 'tests/test_docket_config*.sh'` to `tests/test_docket_config.sh` and
`tests/test_skip_allowlist_invisibility.sh` counted it as a third tree-walk site against a budget
of two (`found 3 (excluded 0, filtered 1, declared 1, hazard 1), budget 2`).

**Opportunity** — the walk-site classifier has no way to say "this site is shape-countable but
provably cannot reach `docs/results/`". A narrow, literal pathspec under `tests/` is not a
results-tree reader by any reading, yet it spends budget exactly like an unbounded traversal. The
guard needs either a reachability-aware classification for literal-pathspec walks, or an explicit
declaration channel that is cheaper and better-scoped than the current one, so that a legitimate
narrow walk does not have to be argued about at merge time.

**Independent value** — with #0258 reverted the guard still budgets every walk site in `tests/`
and `scripts/`, and the next change that adds a narrow `ls-files` hits the same wall. Settling the
classification unblocks #0258's four reverted fixes (`2fa1c162`, `9dad467d`, `7d6e914b`,
`0982b266`, cherry-pickable from PR #189's history) and every future one.

**Boundary** — scoped to `tests/test_skip_allowlist_invisibility.sh`'s walk-site classification and
its budget accounting, plus re-landing #0258's reverted fixes once the classification allows it.
It does not touch the skip-allowlist gate in `docket-finalize-change` itself, the
`FINALIZE_SKIP_RESULTS_ONLY_DELTA` semantics, or the runtime-budget regime for
`tests/test_docket_config.sh` (owned by #0251).

**Reason for deferral** — #0258's fix loop is bounded at two suite runs and its revert rule is
all-or-nothing, so the branch could not both keep the fix and settle the classifier. Deciding
whether a shape-keyed guard should learn reachability is a design question, not a fix.

## What changes

Groomed autonomously 2026-08-09 (critic-gated, two rounds); design detail in the linked spec.

- **Leg A** — `tests/test_skip_allowlist_invisibility.sh` limb 2's `site()` classifier learns
  literal-prefix reachability for glob pathspecs, anchored on `resolve()`'s output (never the raw
  token): a cleanly resolved glob whose last-`/`-truncated literal anchor is off the results chain
  classifies SCOPED; on-chain, root-anchored, and unresolvable (`r == ""`) globs stay reaching,
  fail-closed. No new class, no new declaration channel.
- **Leg B** — synthetic and mutation controls proving the new rule in both directions, including
  the repovar-prefixed on-chain glob that must stay reaching.
- **Leg C** — cherry-pick #0258's four reverted fixes (`2fa1c162`, `9dad467d`, `7d6e914b`,
  `0982b266`, reachable only via `refs/pull/189/head`), amending the last to spell its pathspec
  after `--` so the classifier sees it. `EXPECTED_WALK_REACHING` stays 2.

## Out of scope

- The finalize skip gate, `FINALIZE_SKIP_RESULTS_ONLY_DELTA` semantics, and limb 1.
- The runtime-budget regime for `tests/test_docket_config.sh` (#0251, `related:` — same test
  file; corpus-indifferent on both sides, whichever lands second rebases; no `depends_on`).
- `find`-site glob handling (no false positive exists there today).

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): superseded by the Go migration — tests/test_skip_allowlist_invisibility.sh and tests/test_docket_config.sh are deleted; the skip gate it guarded is deferred from Go v1.
