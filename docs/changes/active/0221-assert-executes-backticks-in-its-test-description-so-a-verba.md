---
id: 221
slug: assert-executes-backticks-in-its-test-description-so-a-verba
title: assert() executes backticks in its test description, so a verbatim-quoted anchor can run shell
status: proposed
priority: medium
type: fix
created: 2026-08-05
updated: 2026-08-05
depends_on: []
related: []
discovered_from: [212]
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

`tests/test_inline_role_stop_scoping.sh` (and the sibling test files sharing the same idiom) define
their assertion helper as roughly:

```bash
assert(){ if ( eval "$2" ); then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
```

The description `$1` is interpolated into a **double-quoted** string, so a backtick inside a test
description is command-substituted by the shell and **executed**.

This is not theoretical. During change 0212's build a worker anchored a guard site on the verbatim
clause ``never `git checkout .` over them``. Running the test executed `git checkout .` in the
feature worktree, reverting the worker's own uncommitted edits. The tree was otherwise clean so
nothing was lost, but a dirtier tree would have destroyed unrelated work — and the failure is
completely silent: the test still prints `ok`.

The hazard is structural for this repo specifically, because docket's guards deliberately anchor on
**verbatim-quoted clauses** from markdown skill bodies (AGENTS.md / ADR-0054), and those clauses
routinely contain backticked code spans. So the guard style the repo mandates is exactly the style
that feeds backticks into `assert`.

0212 mitigated it locally by forbidding backticks in that one file's SITES anchors, recorded in a
comment. That is a convention, not an enforcement, and it does not cover the sibling files.

## What changes

- Harden the `assert` helper at the source so a description is never evaluated — e.g. `printf '%s'`
  the description rather than interpolating it, keeping `eval "$2"` (the condition) as-is.
- Derive the real site list from a whole-repo grep of the `assert(){` definition rather than
  hand-listing files; the helper is copy-pasted across many `tests/test_*.sh`.
- Decide whether the fix is a per-file edit or a shared sourced helper, given the suite has no
  common library today.
- Consider a guard that fails a test file whose `assert` interpolates its description unquoted.

## Out of scope

- Changing what any individual guard asserts.
- Introducing a test framework.
