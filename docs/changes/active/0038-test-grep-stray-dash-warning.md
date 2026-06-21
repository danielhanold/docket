---
id: 38
slug: test-grep-stray-dash-warning
title: Test suite — drop over-escaped dashes in test_docket_metadata_branch.sh grep (silences "stray \ before -")
status: in-progress
priority: low
created: 2026-06-21
updated: 2026-06-21
depends_on: []
related: [34]
adrs: []
spec:
plan:
results:
trivial: true
auto_groomable:
branch: feat/test-grep-stray-dash-warning
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`tests/test_docket_metadata_branch.sh:106` asserts migrate-to-docket.sh has a `--yes`/`-y`
bypass with:

```sh
'grep -qE "\-\-yes\b|\b-y\b" migrate-to-docket.sh'
```

The `\-\-yes` over-escapes the leading dashes. In an ERE a `-` is a literal outside a bracket
expression, so the backslashes are needless and grep emits `grep: warning: stray \ before -`
to stderr on every run. The test still passes, but the warning is real stderr noise — it
violates the project's "a green run leaves stderr pristine" discipline (LEARNINGS #19/#22) and
was surfaced during change #34's regression sweep (the change did not touch this file).

## What changes

One-line fix in `tests/test_docket_metadata_branch.sh:106`: drop the two backslashes before the
leading dashes so the ERE reads `--yes\b|\b-y\b`. The pattern's meaning is unchanged (it still
matches `--yes` or `-y`); only the spurious warning goes away.

After the fix, a full-suite run should leave `test_docket_metadata_branch.sh` with 0-byte
stderr (it currently emits the one warning line).

## Out of scope

- Any other test-file stderr noise (audit separately if found).
- Changing what the assertion verifies, or the migrate `--yes`/`-y` behavior it guards.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
