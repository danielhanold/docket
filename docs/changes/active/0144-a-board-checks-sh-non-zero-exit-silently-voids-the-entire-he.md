---
id: 144
slug: a-board-checks-sh-non-zero-exit-silently-voids-the-entire-he
title: A board-checks.sh non-zero exit silently voids the entire health pass
status: proposed
priority: medium
type: chore
created: 2026-07-27
updated: 2026-07-28
depends_on: []
related: [145]
discovered_from: [117]
adrs: []
spec: docs/superpowers/specs/2026-07-28-board-checks-exit-swallowed-design.md
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
| Spec | [2026-07-28-board-checks-exit-swallowed-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-28-board-checks-exit-swallowed-design.md) |
<!-- docket:artifacts:end -->

## Why

`docket-status.sh`'s `health_checks()` pipes `board-checks.sh` into a `while read` loop and never
reads the producer's exit status. `board-checks.sh` accumulates findings and prints once at the
end, so every validation failure (`exit 2`) emits ZERO TSV lines: the loop body never runs,
`health_checks` returns 0, and the report is byte-indistinguishable from a clean tree.

Change 0117 hit a live instance (a missing `adrs_dir` made `board-checks.sh` exit 2 and silently
dropped every health check) and fixed that trigger. The swallowing itself remains, and 0117's
regression test structurally cannot see it: its mock `board-checks.sh` exits 0 regardless of
arguments, so the "still emits check lines" assert passes against both the fixed and the unfixed
code. Any future condition that makes the checker exit non-zero is an invisible loss of the entire
health pass.

## What changes

Keep the warn-only posture and make the failure loud: capture `board-checks.sh`'s output and exit
status, consume the captured text, and emit one new report line `health checks failed <exit>` when
the checker failed. The token deliberately sits outside the `board ` family that
`skills/docket-status/SKILL.md` teaches as meaning the board step failed. Retire the stale
"produces no extra output" claim from `scripts/docket-status.md` §7 and from the in-code comment on
`health_checks`, and add the contract-table row. Add tests that redden against the unfixed
function — including a mock that emits a finding and *then* fails, pinning the diagnostic as
additive rather than a replacement.

Design settled in the linked spec.

## Out of scope

- `board-checks.sh`'s own exit-2-on-bad-argument rule, which is correct for a hand-run caller.
- Change 0117's specific trigger, already fixed and merged.
- The check-id vocabulary and its pinned surfaces — no id is added.
- `skills/docket-status/SKILL.md`, which change 0145 owns and is currently building.
