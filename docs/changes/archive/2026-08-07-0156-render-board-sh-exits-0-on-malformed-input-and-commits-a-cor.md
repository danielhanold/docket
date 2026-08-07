---
id: 156
slug: render-board-sh-exits-0-on-malformed-input-and-commits-a-cor
title: render-board.sh exits 0 on malformed input and commits a corrupt board
status: killed
priority: medium
type: fix
created: 2026-07-28
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [143]
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

`render-board.sh` exits 0 even when it has demonstrably produced a corrupt `BOARD.md`. Change 0143's
reproduction confirmed this on the current tip: a malformed manifest emits stderr diagnostics and
corrupt rows, the tally loop can abort mid-stream and silently drop every file sorted after it, and
the script still reports success — so the board commits clean and the corruption reaches
`origin/docket` unremarked.

The consequence is not cosmetic. The `--format digest` `ready` line that `docket-implement-next`
consumes can be emptied by this path, so a silent renderer failure can starve the autonomous build
loop while every caller believes the pass succeeded.

`board-checks.sh`'s `board-row-dropped` check reports the resulting state after the fact, but that is
a separate detection surface running later — the producer itself never signals failure to its caller.

Surfaced during 0143's design and recorded there as out of scope: 0143 fixes one specific corruption
trigger; making the script FAIL on malformed input is a contract change affecting every caller.

## Scope

Decide the failure contract for `render-board.sh` on malformed input — non-zero exit, a distinguishable
diagnostic, or both — and how its callers (`board-refresh.sh`, the board pass, `--must-land`) should
react. Pin it with a test that asserts the exit status, not just the output.

## Out of scope

- The specific triggers 0143 fixes.
- Re-litigating `board-checks.sh`'s after-the-fact detection, which stays valuable either way.

## Why killed

Consolidated into #0259 at the 2026-08-07 backlog triage: the failure-contract half of the same render-board hardening; 0143's demonstrated corruptions are fixed, the contract gap is what remains.
