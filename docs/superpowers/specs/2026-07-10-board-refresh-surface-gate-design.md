# Design: board-refresh honors board_surfaces — gate BOARD.md regeneration on the resolved surface set

**Date:** 2026-07-10 · **Change:** 0059 · **Related:** 0058 (docket-status orchestrator — overlaps at docket-status's Board pass), 0011 (github board mirror)

## Problem

`board_surfaces: []` is documented to disable the board entirely — "no `BOARD.md`, no
mirror" (convention SKILL.md), and `docket-status`'s Board pass restates "`board_surfaces: []`
makes the whole pass a no-op." In practice the opt-out is **ignored**: skills keep regenerating
`BOARD.md` and pushing it to `origin/docket` (visible on GitHub) even when the resolved surface
set is empty.

Root cause — the no-op is documented centrally but **not enforced at each board-refresh call
site**:

- `docket-config.sh --export` resolves `board_surfaces: []` correctly to an empty
  `BOARD_SURFACES` string (verified: `docket-config.md` line 77, `docket-config.sh` line 178).
  The config layer is **not** the bug.
- `render-board.sh` is a **pure renderer** with no surface gate — it always emits a full board to
  stdout.
- Every skill's board step is the raw redirect `render-board.sh --changes-dir … > BOARD.md`
  followed by an **unconditional** commit + push. The empty-surfaces guard lives only in prose,
  inside `docket-status`'s Board-pass section. The other skills
  (`docket-new-change`, `docket-groom-next`, `docket-auto-groom`, `docket-finalize-change`,
  `docket-implement-next`) delegate to it as "refresh `BOARD.md` via `docket-status`'s Board
  pass" — an executing agent reads "refresh, commit, push" and never re-loads the cross-referenced
  gate, so it regenerates and pushes anyway.

Compounding trap: the redirect `render-board.sh > BOARD.md` **truncates `BOARD.md` before the
script runs**. So a naive fix that makes `render-board.sh` emit nothing when disabled would blank
the committed board — a regression. The gate must own the *write* decision, so "disabled" means
*do not touch the file*, not *overwrite it with empty*.

## Decisions (brainstormed 2026-07-10, human-approved)

1. **Enforcement moves into a deterministic helper script, not prose.** A new
   `scripts/board-refresh.sh` becomes the single gated entry point for the inline board surface,
   replacing the raw `render-board.sh > BOARD.md` redirect at every call site. Prose enforcement
   is exactly the class of thing that just failed; the fix belongs in code. This applies
   ADR-0012's script-vs-model boundary: the deterministic "should the board be written at all"
   decision is script-owned; the skill still owns commit/push discipline (the "when" that varies
   per skill — must-land vs best-effort — stays model-owned).
2. **General fix — honor `BOARD_SURFACES` exactly, not just `[]`.** The gate keys on the presence
   of the `inline` token, so it is correct for every combination: `[]` → no board;
   `[inline]` → board; `[github]` → **no** `BOARD.md` render/push (github-only); `[inline,
   github]` → board. The reported empty-list case is one instance.
3. **The helper writes `BOARD.md` itself** (file I/O), rather than staying stdout-only, so a
   disabled run never truncates the file. It does **no git writes** — skills keep ownership of
   `git add`/commit/push, preserving the render-family convention (`render-board.sh` "performs no
   git writes").
4. **Stale `BOARD.md` cleanup is out of scope.** If a repo switches from `[inline]` to `[]`, the
   existing committed `BOARD.md` is **left untouched** — never deleted. The bug is unwanted
   *regeneration and pushing*; leaving a stale artifact in place is the minimal, non-destructive
   no-op and keeps the helper git-write-free. Deletion would be a separate, opt-in concern.

## Design

### New script: `scripts/board-refresh.sh` (+ `scripts/board-refresh.md` contract)

A thin deterministic wrapper — the gated inline-board entry point.

```
board-refresh.sh --changes-dir DIR --surfaces "TOKENS" [--repo OWNER/REPO]
```

| Flag | Required | Meaning |
|---|---|---|
| `--changes-dir DIR` | yes | Metadata-working-tree changes dir (`active/`, `archive/`, `BOARD.md` are children). Exit 2 if missing / not a directory (matches `render-board.sh`). |
| `--surfaces "TOKENS"` | yes (value may be empty) | The caller's already-resolved `$BOARD_SURFACES`, verbatim — space-separated tokens, possibly the empty string. Passed explicitly (not read from config) so the script stays pure and testable. Flag absent ⇒ exit 2 (surfaces a wiring bug loudly rather than silently rendering or silently no-oping). |
| `--repo OWNER/REPO` | no | Forwarded to `render-board.sh` for `pr:` hyperlinks. |

**Behavior.** Tokenize `--surfaces`:

- **`inline` ∈ tokens** → run `render-board.sh --changes-dir DIR [--repo …]` and write its stdout
  to `DIR/BOARD.md` (the script owns the write, so a later no-op never truncates). Print
  `board-refresh: inline rendered <path>`. Exit 0.
- **`inline` ∉ tokens** (empty string, or `github`-only) → **touch nothing** — no create, no
  write, no delete of `BOARD.md`. Print `board-refresh: inline disabled — no-op`. Exit 0.
- Tokens other than `inline`/`github` are warned-and-ignored (convention parity — a typo never
  aborts). The decision keys only on `inline`.

**Boundaries.** `render-board.sh` keeps its pure-stdout contract and existing tests untouched —
`board-refresh.sh` composes on top. The **github surface stays out of this helper**: it is a
separate best-effort script (`github-mirror.sh`) already invoked conditionally in
`docket-status`'s Board pass, and it needs docket-status-specific plumbing (`--repo`, project
args, `issue:` write-back). This helper owns the **inline** surface only. The scope-2 matrix is
still fully satisfied: `[github]`-only correctly yields "no `BOARD.md` render/push" here, and the
existing conditional handles "run the mirror."

### Call-site rewiring (skill prose)

Every board-refresh site swaps `render-board.sh … > BOARD.md` for
`board-refresh.sh --changes-dir … --surfaces "$BOARD_SURFACES" [--repo …]`, and makes commit +
push **conditional on the board actually changing**:

> after `board-refresh.sh`, stage `BOARD.md`; commit + push (per this skill's existing
> discipline) **only if** `git status --porcelain -- <changes_dir>/BOARD.md` is non-empty.

Deterministic and agent-proof: when inline is disabled the file is untouched ⇒ no staged diff ⇒
the whole board step is a genuine no-op (no commit, no push). Each skill keeps its existing
semantics — the gate just wraps them:

- **must-land** passes (`docket-new-change`, `docket-groom-next`, `docket-auto-groom`,
  `docket-finalize-change`): "must land" now means "*if* there is a board change, it must land";
  a disabled/no-change board trivially satisfies it.
- **best-effort** pass (`docket-implement-next`): unchanged — attempt-then-log-and-continue,
  gated the same way.

Concrete sites to edit:

| Skill | Site(s) |
|---|---|
| `docket-status` | Board pass inline-surface render **and** the rebase-conflict "re-run `render-board.sh`" regenerate loop → re-run `board-refresh.sh` |
| `docket-new-change` | Step 5 (Board, commit & push), scan mode's board refresh, proposed-kill board refresh |
| `docket-groom-next` | The must-land Board-pass step |
| `docket-auto-groom` | The must-land Board-pass step |
| `docket-finalize-change` | Step 5 (Board) |
| `docket-implement-next` | The *Best-effort board refresh* subsection (claim / reconcile-kill / implemented) |
| `docket-convention` | "Board refresh on status writes" and "Derived-view script family" — name `board-refresh.sh` as the inline entry point (with `render-board.sh` as its internal renderer) |

No board-refresh site may name `render-board.sh > BOARD.md` directly after this change —
`render-board.sh` becomes an internal implementation detail of `board-refresh.sh`, so no un-gated
path survives.

### Testing

New `tests/test_board_refresh.sh` (peer of `tests/test_render_board.sh`, same fixture-and-assert
pattern):

- `--surfaces "inline"` → `BOARD.md` written; content **byte-identical** to
  `render-board.sh --changes-dir …` output.
- `--surfaces ""` (empty) → `BOARD.md` **not created**.
- `--surfaces "github"` → `BOARD.md` **not written** (inline absent).
- `--surfaces "inline github"` → `BOARD.md` written.
- **Truncation-trap regression:** a pre-existing `BOARD.md` with content + `--surfaces ""` → file
  is **byte-identical** afterward (untouched).
- Missing `--surfaces` flag → exit 2; missing/invalid `--changes-dir` → exit 2.
- Exit 0 in all rendered/no-op cases.

The new `scripts/board-refresh.md` contract satisfies the existing
`tests/test_script_contracts_coverage.sh` (globs `scripts/*.sh` → requires a co-located `.md`).

## Verification

- Unit: the new test passes; `test_render_board.sh` still green (renderer contract unchanged).
- Integration: with `board_surfaces: []` set, run a status-writing skill and confirm **no**
  `BOARD.md` commit lands on `origin/docket`; with `[inline]`, confirm the board still refreshes
  and pushes as before (behavior-neutral for the default).
- Grep gate: no skill body contains `render-board.sh` followed by a `> …BOARD.md` redirect after
  the rewire (all inline renders go through `board-refresh.sh`).

## Out of scope

- Deleting/cleaning up a stale `BOARD.md` when a repo switches to `[]` (decision 4).
- Any change to the `github` mirror surface or its existing conditional invocation.
- Any change to `render-board.sh`'s rendering logic or its stdout contract.
