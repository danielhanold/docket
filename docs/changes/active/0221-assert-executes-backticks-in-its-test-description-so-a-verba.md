---
id: 221
slug: assert-executes-backticks-in-its-test-description-so-a-verba
title: assert() executes backticks in its test description, so a verbatim-quoted anchor can run shell
status: proposed
priority: high
type: fix
created: 2026-08-05
updated: 2026-08-09
depends_on: []
related: []
discovered_from: [212]
adrs: []
spec: docs/superpowers/specs/2026-08-07-assert-executes-backticks-in-its-test-description-so-a-verba-design.md
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
| Spec | [2026-08-07-assert-executes-backticks-in-its-test-description-so-a-verba-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-assert-executes-backticks-in-its-test-description-so-a-verba-design.md) |
<!-- docket:artifacts:end -->

## Why

During change 0212's build, running `tests/test_inline_role_stop_scoping.sh` executed a backticked
`git checkout .` embedded in a verbatim-quoted guard anchor, silently reverting the worker's own
uncommitted edits while the test printed `ok`. The hazard is structural for this repo: docket's
guards deliberately anchor on verbatim clauses from skill bodies (AGENTS.md / ADR-0054), and those
clauses routinely contain backticked code spans, so the mandated guard style is exactly the style
that feeds backticks into test source. 0212's mitigation was a per-file comment forbidding backticks
in one SITES table — a convention, not an enforcement, covering none of the ~74 sibling files that
copy-paste the same `assert` idiom.

Grooming corrected the stub's original diagnosis (see the spec's "Corrected diagnosis", verified by
probe): the helper's `echo "ok - $1"` is provably NOT the executing vector — parameter expansion
does not re-trigger command substitution. The backtick executes at **parse time**: in double-quoted
source literals and multi-line double-quoted data assignments (0212's actual site), at `eval "$2"`'s
re-parse of literal backticks in a condition, and in unquoted-delimiter heredoc bodies. The fix must
therefore land at call sites and data blocks, with enforcement — not just in the helper.

## What changes

- Normalize every `assert(){` definition under `tests/` (grep-derived list, never hand-listed) to a
  canonical `printf 'ok - %s\n'` / `printf 'NOT OK - %s\n'` form — safety-neutral, but it aligns the
  ledger, preserves the runner's `^NOT OK` contract, and gives the guard a byte-exact anchor.
  Per-file edit; no shared sourced library (hermeticity is suite contract).
- Add `tests/test_assert_hygiene.sh` (+ its `runtime-budgets.tsv` row): (a) every assert definition
  must match the canonical allowlist; (b) a heredoc-aware quoting scanner — every backtick in
  executable context must be single-quoted or backslash-escaped, calibrated to zero false positives
  before landing, with a documented shrink rule if a class can't be soundly lexed.
- Write the quoting rule into `tests/README.md`.

Full design, decision audit trail (9 critic-gated assumptions), and acceptance criteria are in the
linked spec.

## Out of scope

- Changing what any individual guard asserts; introducing a test framework.
- Rewriting call sites away from `eval "$2"`.
- Editing the 0212 learning file or its in-file comment (recommended human follow-ups, noted in the
  spec's Assumptions 1 and 7).
