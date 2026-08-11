---
id: 221
slug: assert-executes-backticks-in-its-test-description-so-a-verba
title: assert() executes backticks in its test description, so a verbatim-quoted anchor can run shell
status: proposed
priority: high
type: fix
created: 2026-08-05
updated: 2026-08-11
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
does not re-trigger command substitution. The backtick executes at **source evaluation**: in
double-quoted source literals and multi-line double-quoted data assignments (0212's actual site),
at `eval "$2"`'s re-parse of literal backticks in a condition, and in unquoted-delimiter heredoc
bodies. The fix must therefore land at call sites and data blocks, with enforcement — not just in
the helper.

## What changes

- Normalize every assert-family definition under `tests/` (shape-tolerant grep-derived census,
  freshly re-run at build time, never hand-listed) to a canonical `printf 'ok - %s\n'` /
  `printf 'NOT OK - %s\n'` form — safety-neutral, but it aligns the ledger, preserves the runner's
  `^NOT OK` contract, and gives the guard a byte-exact anchor. Per-file edit; no shared sourced
  library (hermeticity is suite contract).
- Add `scripts/check-test-source-hygiene.sh`, a standalone checker run by `scripts/run-tests.sh`
  as a synchronous preflight over every target before the first launch — a violation aborts the run
  with zero test files executed. Rules: (a) every assert definition matches the canonical
  allowlist; (b) a heredoc-aware quoting scanner — no backtick in a double-quoted region (bare or
  escaped), none bare in normal state or unquoted heredoc bodies, and none unescaped in a
  single-quoted assert condition (the eval re-parse vector). Calibrated to zero false positives,
  with a documented shrink rule that may never reopen a demonstrated execution path.
- Add `tests/test_assert_hygiene.sh` (+ its `runtime-budgets.tsv` row) as the checker's regression
  test, exercising committed red/green mutation fixtures including a side-effect sentinel proving
  detection-without-execution.
- Write the quoting rule, the enforcement point, and the standalone-run limitation into
  `tests/README.md`.
- Correct the mechanism claim in the candidate learning
  `test-helper-interpolates-its-own-description` (docket-branch markdown edit; the auto-groom
  constraint that deferred it does not bind human review).

Full design, decision audit trail (11 gated assumptions), and acceptance criteria are in the
linked spec.

## Out of scope

- Changing what any individual guard asserts; introducing a test framework.
- Rewriting call sites away from `eval "$2"`.
- A per-file preamble to protect direct `bash tests/test_x.sh` runs (documented limitation).
- Editing 0212's in-file comment (human follow-up, spec Assumption 7).
