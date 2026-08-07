---
id: 228
slug: finalize-s-auto-detect-suite-loop-has-no-failure-accumulator
title: finalize's auto-detect suite loop has no failure accumulator, so a mid-suite red merges
status: in-progress
priority: high
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: [190, 223, 224]
discovered_from: [227]
adrs: []
spec: docs/superpowers/specs/2026-08-07-finalize-s-auto-detect-suite-loop-has-no-failure-accumulator-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/finalize-s-auto-detect-suite-loop-has-no-failure-accumulator
claimed_at: 2026-08-07T12:02:21Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-finalize-s-auto-detect-suite-loop-has-no-failure-accumulator-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-finalize-s-auto-detect-suite-loop-has-no-failure-accumulator-design.md) |
<!-- docket:artifacts:end -->

## Why

`docket-finalize-change`'s merge gate publishes its suite command in the
`configured-bash-finalize` marker block of `skills/docket-finalize-change/SKILL.md`. Its
auto-detect branch is:

```bash
for test in tests/test_*.sh; do
  "$DOCKET_BASH_PATH" "$test"
done
```

The loop has no failure accumulator, so the block's exit status is **the last test's**. If
`tests/test_adr_checks.sh` (3rd of 86) fails and `tests/test_worktree_hooks_wiring.sh` (86th)
passes, the gate reads green and the merge proceeds. The gate exists precisely to stop a red
suite reaching the integration branch, and in its auto-detect shape it only reliably catches a
failure in the alphabetically last file.

Found while running change 0227's own build gate: that run used the auto-detect branch (the
resolver reads the committed `.docket.yml` from `origin/HEAD`, so 0227's newly-added
`finalize.test_command` was not yet visible to its own gate), and the accumulator had to be added
by hand to get a trustworthy result.

## What changes

- Replace the `configured-bash-finalize` marker block's auto-detect branch with a keep-going
  accumulator, reporting non-zero if **any** test failed:
  `suite_status=0; for … do "$DOCKET_BASH_PATH" "$test" || suite_status=1; done; [ "$suite_status" -eq 0 ]`.
  The `FINALIZE_TEST_COMMAND` branch stays byte-identical. One edit site repairs both consumers.
- Keep the literal-glob (no test files) failure exactly as-is — no `nullglob` — and add a guard
  for it, since a suiteless repo exiting 0 with zero tests run would be a worse bug than this one.
- Add the regression case to `tests/test_configured_bash_finalize.sh`: a fixture whose **non-final**
  test fails, asserting the extracted fragment reports non-zero and that every test still ran.
  Mutation-check it (restore the accumulator-free loop; it must redden).

## Out of scope

- The `FINALIZE_TEST_COMMAND` branch — user-authored shell text is executed unchanged by
  contract, so its exit semantics are the user's to get right.
- Stating the green/red-is-the-exit-code rule normatively — that is change **0224**'s scope. 0228
  fixes the fragment; 0224 states the rule.
- Rebinding auto-detect onto `scripts/run-tests.sh` (a separate design question with its own
  consumers).

## Open questions

- ~~Does `docket-build`'s own gate (which reuses this same detection) have the same hole?~~ —
  RESOLVED 2026-08-07 by inspection: **yes, but there is only one site to fix.**
  `skills/docket-build/SKILL.md` deliberately does *not* copy the fragment — it names finalize's
  `configured-bash-finalize` marker block as the single source. Repairing the block repairs both
  consumers at once.
- ~~Keep going after the first failure, or stop at the first?~~ — RESOLVED at grooming: **keep
  going**; see the spec's `## Assumptions`.
- ~~How should the status leave the block?~~ — RESOLVED at grooming: a trailing
  `[ "$suite_status" -eq 0 ]`, not `exit`. Reversed under critic review; rationale in the spec.

Note: the stub's original claim that the existing guard's assertions "pin the command's *text*" is
wrong — the guard executes the extracted fragment and never inspects its exit status. That single
uninspected status is the gap.
