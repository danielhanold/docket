---
id: 38
slug: test-grep-stray-dash-warning
title: Test suite — drop over-escaped dashes in test_docket_metadata_branch.sh grep (silences "stray \ before -")
status: done
priority: low
created: 2026-06-21
updated: 2026-06-21
depends_on: []
related: [34]
adrs: []
spec:
plan: docs/superpowers/plans/2026-06-21-test-grep-stray-dash-warning.md
results:
trivial: true
auto_groomable:
branch: feat/test-grep-stray-dash-warning
pr: https://github.com/danielhanold/docket/pull/46
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-06-21-test-grep-stray-dash-warning.md](https://github.com/danielhanold/docket/blob/feat/test-grep-stray-dash-warning/docs/superpowers/plans/2026-06-21-test-grep-stray-dash-warning.md) |
| PR | [#46](https://github.com/danielhanold/docket/pull/46) |
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

One-line fix in `tests/test_docket_metadata_branch.sh:106`. The `\-\-yes` over-escaping must go
(it triggers GNU grep's `stray \ before -` warning), but **simply dropping the backslashes is not
enough**: an unescaped ERE that *begins* with `--` (`--yes\b|\b-y\b`) is parsed by grep as a
command-line **option**, not a pattern, so `grep -qE "--yes\b|\b-y\b"` fails with
`unrecognized option '--yes…'` (exit 2) on GNU grep. The over-escaping was incidentally doubling
as option-guarding. The correct fix uses grep's explicit pattern flag `-e`:

```sh
'grep -qE -e "--yes\b|\b-y\b" migrate-to-docket.sh'
```

`-e <PATTERN>` is POSIX and tells grep the next argument is a pattern, never an option — verified
to give exit 0 with **empty stderr on both GNU grep and BSD grep**. The assertion's meaning is
unchanged (it still matches `--yes` or `-y`); the spurious warning goes away and the leading-`--`
option-parse trap is closed.

After the fix, a full-suite run should leave `test_docket_metadata_branch.sh` with 0-byte
stderr (it currently emits the one warning line under GNU grep).

## Out of scope

- Any other test-file stderr noise (audit separately if found).
- Changing what the assertion verifies, or the migrate `--yes`/`-y` behavior it guards.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-06-21 — reconcile before build

- **Verified the premise against current `origin/main`.** Line 106 is unchanged and still the
  sole over-escaped grep in `tests/`: `'grep -qE "\-\-yes\b|\b-y\b" migrate-to-docket.sh'`.
  Reproduced `grep: warning: stray \ before -` with GNU grep (BSD grep stays silent — the warning
  is GNU/Linux-CI specific, as the change implied). `migrate-to-docket.sh` still carries the
  `-y|--yes` bypass (line 55), so the assertion remains meaningful.
- **Corrected the proposed fix — the change body's original fix was wrong.** The body said to
  "drop the two backslashes" leaving `grep -qE "--yes\b|\b-y\b"`. That FAILS on GNU grep: a pattern
  beginning with `--` is parsed as a command-line option (`unrecognized option '--yes…'`, exit 2).
  The `\-` over-escaping was incidentally guarding against option-parsing. Adopted the explicit
  pattern flag instead: `grep -qE -e "--yes\b|\b-y\b"`. Verified exit 0 + empty stderr on **both**
  GNU grep and BSD grep. Updated `## What changes` to document this.
- **Scope unchanged:** single-line edit, one file. `related: [34]` (the regression sweep that
  surfaced this) is now `done` (PR #45 merged 2026-06-21) — no longer in flight, nothing to fold
  in from it.
