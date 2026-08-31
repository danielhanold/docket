---
id: 221
slug: assert-executes-backticks-in-its-test-description-so-a-verba
title: assert() executes backticks in its test description, so a verbatim-quoted anchor can run shell
status: done
priority: high
type: fix
created: 2026-08-05
updated: 2026-08-12
depends_on: []
related: []
discovered_from: [212]
adrs: [91]
spec: docs/superpowers/specs/2026-08-07-assert-executes-backticks-in-its-test-description-so-a-verba-design.md
plan: docs/superpowers/plans/2026-08-11-assert-backtick-source-hygiene.md
results: docs/results/2026-08-11-assert-executes-backticks-in-its-test-description-so-a-verba-results.md
trivial: false
auto_groomable: true
branch: feat/assert-executes-backticks-in-its-test-description-so-a-verba
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/202
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-assert-executes-backticks-in-its-test-description-so-a-verba-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-assert-executes-backticks-in-its-test-description-so-a-verba-design.md) |
| Plan | [2026-08-11-assert-backtick-source-hygiene.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-11-assert-backtick-source-hygiene.md) |
| Results | [2026-08-11-assert-executes-backticks-in-its-test-description-so-a-verba-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-11-assert-executes-backticks-in-its-test-description-so-a-verba-results.md) |
| ADRs | [ADR-0091](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0091-every-backtick-in-a-double-quoted-region-is-a-violation.md) |
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

## Reconcile log

### 2026-08-11 — reconciled against origin/main @ ddd5ffc7

Spec (2026-08-07, human-revised 2026-08-11) survives intact: the corrected diagnosis, the four
violation classes, and the preflight-gate design all still describe current reality. No scope drop.
Seven concrete adjustments, all additive:

1. **Census re-derived (spec D1/Assumption 11 mandate it, and it drifted again).** Shape-tolerant
   grep over `tests/**/*.sh` on origin/main finds **88 assert definitions** — 84 canonical
   `assert(){ if eval "$2"; …}`, 3 subshell `( eval "$2" )`, 1 `fails`-counter using the divergent
   `FAIL - ` marker — plus **22 wrapper definitions** across six spellings (`ok(){ printf 'ok   -
   %s\n' …}` ×7, `no(){ …}` ×7, `ok(){ echo …}` ×2, `ok(){   printf …}` ×2, `nok(){  printf …}` ×2,
   and one each of `ok(){  printf …}` / `nok(){ printf …}`). The spec's review-round count was 85;
   it is 88 three days later. The build re-runs this census itself and never reuses these numbers.
   Today's tree is still uniform in *declaration* shape (no `assert () {`, no `function assert`,
   no multiline) — the alternate spellings live only in the mutation fixtures, as the spec intends.

2. **`tests/lib/` is a scanning gap the spec does not name.** Three sourced common libs —
   `tests/lib/gate_run_common.sh`, `tests/lib/runner_dispatch_detach_common.sh`,
   `tests/lib/sync_agents_common.sh` — each define `assert`. They are not `tests/test_*.sh`, so the
   target list `run-tests.sh` hands the preflight excludes them, and rule (a) would leave three
   definitions permanently outside the allowlist while rule (b) never scanned their bodies. The
   checker's own discovery must therefore cover `tests/lib/*.sh` regardless of what the caller
   passes, rather than trusting the caller's target list to be the whole census surface.

3. **A new obligation the spec omits: the script needs a contract file.**
   `tests/test_script_contracts_coverage.sh` asserts every top-level `scripts/<name>.sh` has a
   co-located `scripts/<name>.md`. Adding `scripts/check-test-source-hygiene.sh` without
   `scripts/check-test-source-hygiene.md` reddens the suite. The contract is now build scope
   (Purpose / Usage / Behavior / Exit codes / Invariants, per the convention).

4. **Exit code for the preflight abort.** `scripts/run-tests.sh` already spends 0, 1, 2, 3 and 4
   (4 is `--strict-budget` breach). The hygiene abort takes the next free code, **5**, and the
   runner contract in `scripts/run-tests.md` gains the row.

5. **Scale of the calibration gate is now measured, and it is not small.** 2857 backtick-bearing
   lines across 91 of 107 test files. A deliberately crude line-local prototype (no heredoc
   awareness, no cross-line quote state) flags 41 bare-in-double-quote, 76 escaped-in-double-quote
   and 108 normal-state hits. Those numbers are an upper bound, not a work estimate: the majority
   sit in heredoc bodies and multi-line strings that a *correct* scanner reclassifies. The load-
   bearing consequence is a design constraint the spec left implicit — the scanner must carry
   quote state **across** lines, because 0212's actual vector was a multi-line double-quoted
   `SITES="…"` assignment, and a per-line scanner cannot see it. Real hits like `tok="\`$d\`"` and
   `probe_flat="$(flat "\`docket-auto-groom\` …")"` confirm the escaped-in-double-quote class is
   populated by live code, not hypotheticals.

6. **Precedent to follow.** `tests/test_pipe_shapes.sh` (change 0276, merged since the spec was
   written) is now the house pattern for a repo-wide shape guard with its own budget row; the new
   guard should read like its sibling rather than invent a shape.

7. **Budget table.** 150 rows on origin/main; `test_assert_hygiene.sh` adds one, measured, not
   guessed. Two rows are explicitly off-limits: `test_docket_status.sh` (at the table's hard 60s
   ceiling; sharding tracked as #0296) and `test_sync_agents_runners.sh` (pre-existing breach,
   tracked as #0280).

Unchanged and re-confirmed: `discovered_from: [212]` is merged context only, `depends_on` is
empty, and nothing in the eleven merged changes since 2026-08-07 touches the assert idiom itself.
The D4 learning file `test-helper-interpolates-its-own-description.md` still carries the disproven
mechanism claim and is still in scope.
