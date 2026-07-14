---
id: 31
slug: complementary-board-write-guards-and-the-bound-of-source-scanning
title: Two complementary board-write guards, and the bound of source-syntax scanning
status: Accepted
date: 2026-07-14
supersedes: []
reverses: []
relates_to: []
change: 70
---

## Context

`scripts/render-board.sh` prints the board to STDOUT and writes no file. `scripts/board-refresh.sh` is the ONE gated primitive allowed to turn that output into `BOARD.md` (change 0059). Any other caller that redirects the renderer silently produces a wrong or truncated board while every surface reports success — a failure this docket has already been bitten by.

Change 0069 pointed the existing prose-tuned regex sentinel `REDIRECT_RE` (whitespace-bounded ` > ` near a literal `/BOARD.md`) at `scripts/docket-status.sh`, a bash script rather than the skill prose it was written for. Change 0070's spec proposed replacing that scan with a repo-wide "write sentinel" that prohibits the WRITE rather than recognizing the write TARGET, and RETIRING the `REDIRECT_RE` scan over the script as subsumed by it.

## Decision

**1. Ship the write sentinel.** `render_board_write_free` in `tests/test_render_board.sh` tokenizes each `render-board.sh` invocation (through the pipeline it feeds), normalizes merged-output forms, erases only true fd dups, and fails on any surviving file-directed redirect — plus a one-hop TAINT stage that follows the renderer's stdout into a captured variable and flags a redirect of that variable. Because it never matches the target's *name*, no-space (`>"$f"`), `>>`, `>|`, `&>`, `>&file`, and variable targets all die identically.

**2. KEEP `REDIRECT_RE` and BOTH of its scans** (skills prose AND `scripts/docket-status.sh`). The spec's subsumption premise was DISPROVED by mutation testing: the write sentinel is token-scoped and is structurally blind to a write that crosses a statement boundary carrying the bytes in no variable — `{ render-board.sh ...; } > f`, and a wrapper function — which `REDIRECT_RE`'s whole-file flattened scan does catch. The two guards are **COMPLEMENTARY, not nested. NEITHER MAY BE DELETED as redundant.** A COMPLEMENTARITY block in `tests/test_render_board.sh` locks this by asserting, in both directions, shapes each guard uniquely catches.

**3. Source-syntax scanning has a HARD BOUND**, now established empirically rather than argued. Six successive review rounds each found a fresh evasion: `&>` / `>&file`; a comment ending in a backslash that laundered a live invocation; a pipeline redirect past `|`; capture-then-write; and finally `${out:-}` — the guarded file's OWN house idiom, which the taint stage missed because it enumerated *spellings* instead of describing *shape*. Known residual writes that stay green and are DISCLOSED rather than chased: `| tee f`, `exec 3>f` + `>&3`, an `eval`-conjured redirect operator, a tainted value passed to a function or copied to a second variable, `mapfile` / `read < <(...)` captures, and a metacharacter inside a quoted argument. The lesson: **a guard written as a list of spellings is always one spelling short.**

**4. The deferred filesystem-effect test's trigger has FIRED.** Change 0070's spec deferred asserting `BOARD.md`'s bytes after a fixture run (syntax-independent but path-dependent) with the condition that "it earns its cost when a write path exists that a source scan cannot reach." Point 3 is that path. The filesystem-effect test is the only real answer to the residual class and should be a follow-up change.

## Consequences

- **What it enables:** the realistic regression shapes in `scripts/docket-status.sh` — which holds the board path in a variable and captures the renderer's stdout into `out` — now redden the suite, proven by live-tree injection.
- **What it costs:** two guards to maintain instead of one, with a documented reason neither may be collapsed into the other; ~1000 lines added to `tests/test_render_board.sh` (a large mutation battery, whose per-row comments are load-bearing).
- **What is given up:** completeness. The guard pair's bound is published in the test file's KNOWN/ACCEPTED GAPS section rather than papered over.

Related changes: 0059 (board-refresh gated write), 0069 (docket-status self-evidencing report, which pointed the prose regex at the script). Related project ledger rule: *a guard is code — mutation-test it or it is decoration; one guard covers one hole — when a mutation slips past it, add an independent scan rather than widening the first; deleting a sentinel is how the guarded hole reopens.*
