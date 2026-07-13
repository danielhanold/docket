---
id: 70
slug: redirect-regex-board-write-guard
title: Harden the BOARD.md write guard — REDIRECT_RE misses real redirect forms
status: in-progress
priority: medium
created: 2026-07-13
updated: 2026-07-13
depends_on: []
related: [59, 69]
adrs: []
spec: docs/superpowers/specs/2026-07-13-redirect-regex-board-write-guard-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/redirect-regex-board-write-guard
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-13-redirect-regex-board-write-guard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-13-redirect-regex-board-write-guard-design.md) |
<!-- docket:artifacts:end -->

## Why

`render-board.sh` emits to STDOUT and writes no file; `board-refresh.sh` is the single gated primitive
allowed to turn that output into `BOARD.md`. Any other caller that redirects the renderer itself
silently produces a wrong or truncated board while every surface still reports success — the failure
docket has already been bitten by, and the reason a test guard exists.

`REDIRECT_RE` (`tests/test_render_board.sh`) is the negative sentinel enforcing that rule: nobody may
redirect `render-board.sh` into `BOARD.md`. It requires whitespace on **both** sides of `>`, a shape a
~15-line comment defends against real false-positive classes — bracket placeholders
(`<changes_dir>/BOARD.md`) and flattened markdown blockquotes. Those classes are real **in prose**.

Change 0069 aimed that prose-tuned regex at `scripts/docket-status.sh`, a bash script, where the hazard
profile inverts. The idiomatic shell redirect is `>"$dir/BOARD.md"` — no space — and the regex cannot
see it. Nor `>>`, nor `>|`. And `docket-status.sh` holds the board path in a variable
(`local rel="$CHANGES_DIR/BOARD.md"`), so a rogue `> "$mw/$rel"` writes the board with the literal
string `BOARD.md` nowhere near the redirect — **unreachable by any regex keyed on that string, however
widened.**

Not a regression: the regex is byte-identical to `origin/main`. What changed is its load-bearingness.

## What changes

State the invariant once — *`render-board.sh`'s stdout reaches a file through `board-refresh.sh` and
nothing else* — and derive the guards from it, instead of from proxies for it.

- **A repo-wide write sentinel** replaces 0069's scan: over every `scripts/*.sh` except the allowlisted
  `board-refresh.sh`, each `render-board.sh` invocation must contain no file-directed redirect (fd dups
  like `>&2` allowed). It never matches the target's name, so no-space, `>>`, `>|`, and variable-named
  targets all die identically. Continuation lines are joined *before* tokenizing — the current
  tokenizer is line-oriented, and a redirect on a continuation line evades it.
- **`REDIRECT_RE` is kept, unwidened, and re-scoped to `skills/*/SKILL.md` prose**, where its narrow
  shape is correct. Its design comment is re-derived rather than deleted, gaining the sentence that is
  currently missing: this defends prose; shell is guarded elsewhere, and can be guarded far more widely
  precisely because prose hazards cannot occur in a script.
- The `--format digest` flag check in `tests/test_docket_status.sh` stays (it guards a different
  property) and inherits the same continuation fix.

The substance is a **mutation battery**: each evasion above is injected into a fixture and must turn the
guard red, with `2>&2` — the current correct invocation — as the false-positive control. Per the ledger
(#64): a guard is code; mutation-test it or it is decoration.

Design: [`docs/superpowers/specs/2026-07-13-redirect-regex-board-write-guard-design.md`](../../superpowers/specs/2026-07-13-redirect-regex-board-write-guard-design.md).

## Out of scope

- Changing `render-board.sh`'s STDOUT contract. Emitting to STDOUT is the design (change 0059's surface
  gate depends on it); the guard exists precisely because that contract puts the burden on callers.
- Any production-code change. `render-board.sh`, `board-refresh.sh`, and `docket-status.sh` all behave
  correctly today — this is about the suite's ability to *notice* if a future one doesn't. Test-only.
- The filesystem-effect test (assert `BOARD.md`'s content after a fixture run). Syntax-independent but
  path-dependent — it misses a rogue redirect on a branch the fixture never takes. Deferred with a
  stated trigger, not rejected: it earns its cost when a write path exists that a source scan cannot
  reach.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
