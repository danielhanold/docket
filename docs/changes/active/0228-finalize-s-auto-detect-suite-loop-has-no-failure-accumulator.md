---
id: 228
slug: finalize-s-auto-detect-suite-loop-has-no-failure-accumulator
title: finalize's auto-detect suite loop has no failure accumulator, so a mid-suite red merges
status: proposed
priority: high
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [227]
adrs: []
spec:
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

- Add a failure accumulator to the marker block's auto-detect loop — the shape every results file
  in this repo already uses ad hoc (`suite_status=0; ... || suite_status=1; ... exit "$suite_status"`).
- Decide whether the block should also keep going after the first failure (report all reds) or
  stop at the first; the ad-hoc shape in `docs/results/` keeps going, which is more useful.
- Check `tests/test_configured_bash_finalize.sh` for a regression fixture that would have caught
  this — a suite where a non-final test fails and the block must report non-zero. Its current
  assertions pin the command's *text* and `DOCKET_BASH_PATH` propagation, not its exit status
  under a mid-suite failure.

## Out of scope

- The `FINALIZE_TEST_COMMAND` branch — user-authored shell text is executed unchanged by
  contract, so its exit semantics are the user's to get right.

## Open questions

- ~~Does `docket-build`'s own gate (which reuses this same detection) have the same hole?~~ —
  RESOLVED 2026-08-07 by inspection: **yes, but there is only one site to fix.**
  `skills/docket-build/SKILL.md` (lines 189–192) deliberately does *not* copy the fragment — it
  names finalize's `configured-bash-finalize` marker block as the single source and explicitly
  instructs "Do not copy that fragment into this file," keeping the awkward `finalize` namespace
  rather than introducing a second, driftable test command. So `docket-build`'s gate inherits the
  missing accumulator, and repairing the marker block repairs both consumers at once. No second
  edit site; the fixture should still cover both callers reading the same block.
