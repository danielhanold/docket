---
id: 215
slug: escape-newlines-in-board-checks-sanitize-now-that-z-can-deli
title: Escape newlines in board-checks sanitize now that -z can deliver a raw LF
status: proposed
priority: medium
type: fix
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

Change 0202 rewrote `branch_only_artifact` in `scripts/board-checks.sh` to read
`git ls-tree -r -z` instead of `--name-only`, which fixed a false positive on C-quoted paths. That
rewrite silently invalidated a premise `sanitize` still relies on.

`sanitize` escapes only TAB and CR, justified by a comment asserting that every emitted value
arrives via `field`/`fm_field`, both of which truncate at the first newline. `$ar_hit` is the one
emitted value that is a **git path**, not a frontmatter field. Under the old `--name-only` read,
C-quoting guaranteed an embedded newline arrived as the two characters `\n`; under `-z` the raw LF
reaches `emit`, splitting one finding across two TSV records and breaking the `sort` determinism
downstream.

Raised as an `important` (non-blocking) finding at 0202's deep-rung review and left for merge-time
judgment; the trigger is a pathological path (git permits embedded newlines, nothing in this repo
has one) and the check is warn-only, which is why it graded important rather than blocker.

## What changes

- `scripts/board-checks.sh` — add `v="${v//$'\n'/\\n}"` to `sanitize`, and update the comment that
  currently justifies the TAB/CR-only scope so it no longer asserts a premise the `-z` read broke.
- `tests/test_board_checks.sh` — a fixture whose branch carries a path with an embedded newline,
  asserting the finding stays one TSV record.

## Out of scope

- Any further change to the `-z` read itself (0202 settled that shape).

## Open questions

- Whether the same premise is relied on by any other `emit` caller passing a non-frontmatter value.
