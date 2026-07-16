---
slug: test-premise-deleted-not-regated
hook: "When a change invalidates a test's premise, ask what the block GUARDS, not what it asserts."
topics: [testing, guards, refactoring]
changes: [84]
created: 2026-07-16
updated: 2026-07-16
promotion_state: retained
promoted_to:
---

## Apply
When a change invalidates a test's premise, ask what the block GUARDS, not what it asserts —
a block guarding a mechanism (crash-safety, a fence) is inverted and kept; a block guarding only the
retired behavior is deleted, once you have confirmed its coverage lives elsewhere. Re-gating to
green is how a duplicate assert under a lying name survives. Mirror of the guards family's
never-delete-a-sentinel rule: NARROW a guard whose property still holds, DELETE one whose subject
is gone.

## War story
- 2026-07-16 (#84, PR #90) — A test whose PREMISE the change deletes must be deleted, not re-gated.
  `test_closeout.sh` carried a block named *"back-compat — omitting `--enabled` still publishes
  (default true)"*; the flip made an omitted flag a loud no-op, and re-gating the block with an
  explicit `--enabled true` made it pass — as a byte-for-byte duplicate of the adjacent explicit-true
  block, under a name contradicting its own body. Its sibling `test_docket_status.sh` Case B looked
  like the same call and was the opposite one: that block exists to guard a real `set -u`
  unbound-variable crash and the `:-` expansion IS the guard, so only its fallback VALUE flipped.
