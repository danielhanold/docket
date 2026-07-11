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
- `docket-status`'s canonical inline Board-pass step is the raw redirect
  `render-board.sh --changes-dir … > BOARD.md` followed by an **unconditional** commit + push.
  The empty-surfaces guard lives only in prose immediately above it. The other skills
  (`docket-new-change`, `docket-groom-next`, `docket-auto-groom`, `docket-finalize-change`,
  `docket-implement-next`) delegate to it as "refresh `BOARD.md` via `docket-status`'s Board
  pass" — an executing agent can follow the refresh/commit/push instruction without carrying the
  cross-referenced surface gate into the delegated call, so it regenerates and pushes anyway.

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
| `docket-status` | **Post-0058 (merged): the Board pass moved out of skill prose into `scripts/docket-status.sh`.** Do NOT edit `docket-status/SKILL.md` (58 already deleted the raw redirect; the skill now just invokes the orchestrator and trusts its exit code). Instead compose the primitive into `scripts/docket-status.sh` `board_pass_inline` — its two `render-board.sh … > tmp` sites (initial render + the rebase-conflict regenerate loop) route through `board-refresh.sh` (already atomic/truncation-safe), so `render-board.sh` is only ever reached via `board-refresh.sh`. `board_pass` already gates on the `inline` token, so `board_pass_inline` passes `--surfaces inline` explicitly and keeps its own commit/push+rebase loop. |
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

**Post-0058 test wiring.** Leave `tests/test_render_board.sh`'s existing docket-status sentinel
(`grep -qF "/render-board.sh" "$SKILL"`) unchanged — the SKILL still *describes* `render-board.sh`
(58's reference section), and we are not editing that SKILL. Add to `test_render_board.sh` only the
**general negative scan**: no `skills/*/SKILL.md` body may redirect `render-board.sh` stdout into
`BOARD.md` (with a positive-control assertion so the guard regex can't silently weaken). The
meaningful docket-status assertion moves to `tests/test_docket_status.sh`: assert
`board_pass_inline` routes its render through `board-refresh.sh` (not `render-board.sh` directly).

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

## Reconcile notes (2026-07-11)

- Reconciled against `origin/main` at `3fad316` after change 0053's skill slimming landed. The
  current prose has one concrete raw redirect in `docket-status`; sibling skills delegate to that
  Board pass rather than repeating the command. The implementation still updates every listed
  caller so each delegated status-write path explicitly uses the gated helper and commits only a
  real `BOARD.md` diff.
- Related change 0058 remains proposed and introduces a future `docket-status.sh` orchestrator.
  It does not invalidate this change: `board-refresh.sh` is the deterministic inline primitive
  that orchestrator can call later, while this PR fixes the current executable skill path.
- Current tests include a `test_render_board.sh` sentinel requiring `docket-status` to name
  `render-board.sh`; it must move to `board-refresh.sh` or the full suite will reject the intended
  wiring. No recent ADR changes the approved ADR-0012 script/model boundary.

## Reconcile notes (2026-07-11 — after change 0058 merged, PR #65)

The prior note's premise ("0058 remains proposed … does not invalidate this change") inverted:
**0058 landed first** and, in `scripts/docket-status.sh`, independently built the same inline gate
this spec set out to add — `board_pass` short-circuits empty surfaces (`[ -n "$surfaces" ] ||
return 0`), keys on the `inline` token, and `board_pass_inline` renders to a temp + `cmp -s` diff
(no truncation trap) + commits/pushes only on change. 0058 also **deleted** the raw-redirect prose
section from `docket-status/SKILL.md` — the exact section this spec's original call-site table
targeted. Rescope, keeping the design core (a git-write-free `board-refresh.sh` gated on `inline`)
unchanged:

- **Drop** the `docket-status/SKILL.md` edit — obsolete + conflicting. The skill now just invokes
  the orchestrator; the gate lives in the script.
- **Keep** `board-refresh.sh` and the sibling-skill rewiring (`docket-new-change`,
  `docket-groom-next`, `docket-auto-groom`, `docket-finalize-change`, `docket-implement-next`, the
  two `docket-convention` references) — 0058 never touched those, so the residual bug (they still
  push `BOARD.md` under `board_surfaces: []`) is real and this change is what fixes it.
- **Add** composing `board-refresh.sh` into `scripts/docket-status.sh` `board_pass_inline` — a
  single-source-of-gate dedup (0058's path is already *correct*, just a second copy of the gate),
  yielding one inline-board primitive and the invariant "`render-board.sh` is only ever reached via
  `board-refresh.sh`."
- **Test wiring** updated as in the revised *Testing* section: keep `test_render_board.sh`'s
  `render-board.sh` SKILL sentinel, add its general negative-redirect scan, and move the
  docket-status assertion into `test_docket_status.sh`.
- Chose `board-refresh.sh` over routing siblings through 0058's `docket-status.sh --board-only`
  (a heavyweight self-syncing/self-committing pass that would fight each sibling's own commit
  discipline). PR #64 (branched pre-0058 at `3fad316`) is rebased onto `origin/main` and reworked
  accordingly.
