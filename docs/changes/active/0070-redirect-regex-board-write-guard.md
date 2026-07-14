---
id: 70
slug: redirect-regex-board-write-guard
title: Harden the BOARD.md write guard — REDIRECT_RE misses real redirect forms
status: implemented
priority: medium
created: 2026-07-13
updated: 2026-07-14
depends_on: []
related: [59, 69]
adrs: [31]
spec: docs/superpowers/specs/2026-07-13-redirect-regex-board-write-guard-design.md
plan: docs/superpowers/plans/2026-07-13-redirect-regex-board-write-guard-plan.md
results: docs/results/2026-07-14-redirect-regex-board-write-guard-results.md
trivial: false
auto_groomable:
branch: feat/redirect-regex-board-write-guard
pr: https://github.com/danielhanold/docket/pull/80
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-13-redirect-regex-board-write-guard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-13-redirect-regex-board-write-guard-design.md) |
| Plan | [2026-07-13-redirect-regex-board-write-guard-plan.md](https://github.com/danielhanold/docket/blob/feat/redirect-regex-board-write-guard/docs/superpowers/plans/2026-07-13-redirect-regex-board-write-guard-plan.md) |
| Results | [2026-07-14-redirect-regex-board-write-guard-results.md](https://github.com/danielhanold/docket/blob/feat/redirect-regex-board-write-guard/docs/results/2026-07-14-redirect-regex-board-write-guard-results.md) |
| PR | [#80](https://github.com/danielhanold/docket/pull/80) |
| ADRs | [ADR-0031](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0031-complementary-board-write-guards-and-the-bound-of-source-scanning.md) |
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

### 2026-07-13 — verified against `origin/main` @ `d80dca8`; design unchanged

Re-read the change + spec against current code, #59/#69 (both `done`), and the ledger. Every premise
the design rests on is still true byte-for-byte:

- `REDIRECT_RE` (`tests/test_render_board.sh:270`) is unchanged, still whitespace-bounded, still
  scanning `skills/*/SKILL.md` **plus** `scripts/docket-status.sh` (0069's scan, lines ~290–302) —
  the scan Guard 1 replaces.
- The flag check (`tests/test_docket_status.sh:24–33`) is present and line-oriented
  (`grep -oE '[^;&|]*/render-board\.sh[^;&|]*'` over `grep -v '^[[:space:]]*#'`), confirming the
  continuation-line hole.
- The false-positive control is live: `docket-status.sh:172` is
  `out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest 2>&2)"`.
- A repo-wide sweep of `render-board.sh` across `scripts/*.sh` shows the only executable invocations
  are that one and `board-refresh.sh`'s `RENDER_BOARD` seam (the allowlisted writer); every other hit
  (`render-adr-index.sh`, `render-change-links.sh`, `render-board.sh` itself) is a **comment** —
  confirming comment-stripping is load-bearing for Guard 1's glob, not a nicety.

No scope change; no work done elsewhere. One clarification folded in, not a design change: Guard 1
prohibits *every* non-fd-dup redirect in a render-board invocation, so even a stderr-to-file form
(`2>/dev/null`) is rejected — deliberately conservative, since the invariant is about writes and the
correct way to route stderr here is the fd dup (`2>&2`) already in use. This is stated in the guard's
comment so a future author meets the rule instead of discovering it.

Change 0068 (`docket-command-facade`, in-progress) may add scripts under `scripts/`; Guard 1 derives
its call-site list from a glob, so a new script is covered automatically with no edit here.
