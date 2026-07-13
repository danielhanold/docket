# Harden the BOARD.md write guard — design

**Change:** #0070 · **Date:** 2026-07-13 · **Related:** #0059 (board-refresh surface gate), #0069 (status report self-evidencing)

## Context

`render-board.sh` prints the board to STDOUT and writes no file. `board-refresh.sh` is the one gated
primitive allowed to turn that output into `BOARD.md` (change 0059: render to temp → chmod → rename).
Every other caller must treat the renderer as read-only. A caller that forgets and redirects
STDOUT itself silently produces a wrong or truncated board while every surface still reports success —
the failure mode docket has already been bitten by, and the reason a test guard exists at all.

Two guards police that rule today:

- **`REDIRECT_RE`** (`tests/test_render_board.sh`) — a negative sentinel: no *skill body* may show the
  pre-0059 anti-pattern `render-board.sh … > …/BOARD.md`. Change 0069 additionally pointed it at
  `scripts/docket-status.sh`.
- **the flag check** (`tests/test_docket_status.sh`) — every `render-board.sh` invocation in the
  orchestrator must carry `--format digest`, the read-only projection 0069 introduced.

### The problem

`REDIRECT_RE` requires whitespace on **both** sides of `>`
(`render-board\.sh.{0,200}[[:space:]]>[[:space:]].{0,200}/BOARD\.md`). That shape was chosen
deliberately, and a ~15-line comment defends it: it must cross bracket placeholders
(`<changes_dir>/BOARD.md`) yet not fire on a markdown blockquote flattened to ` > `, nor on a
placeholder's closing bracket. Those are real false-positive classes — **in prose**.

0069 aimed that prose-tuned regex at a **bash script**, where the hazard profile is inverted. In a
shell script the idiomatic redirect is `>"$dir/BOARD.md"` — no space — and the regex is blind to it.
It is equally blind to `>>` and `>|`.

Worse, and unnamed in the stub: `docket-status.sh` holds the board path in a variable
(`local rel="$CHANGES_DIR/BOARD.md"`). A rogue `> "$mw/$rel"` writes the board while carrying the
literal string `BOARD.md` nowhere near the redirect. **No regex keyed on `BOARD\.md` can ever catch
that** — which forecloses "just widen `REDIRECT_RE`" as a complete answer, however wide you go.

And the flag check has its own hole, which 0069 knew about and documented: `render-board.sh
--format digest > "$1/BOARD.md"` satisfies it while writing the very file `board-refresh.sh` owns.

This is not a regression — the regex is byte-identical to `origin/main` and always had this shape.
What changed is its load-bearingness.

## Decision

State the invariant once, and derive every guard from it:

> **`render-board.sh`'s stdout reaches a file through `board-refresh.sh` and nothing else.**

The existing guards each encode a *proxy* for that sentence — "a `--format digest` flag is present",
"a ` > ` appears near the string `BOARD.md`" — and the evasions are all exploits of the gap between
proxy and invariant. The fix is to stop recognizing the write *target* and start prohibiting the
write.

### Guard 1 — repo-wide write sentinel (new; replaces 0069's scan)

Home: `tests/test_render_board.sh`, beside `REDIRECT_RE` and its positive control — that file already
owns the renderer's contract, and is where 0069 added the scan this replaces.

1. Iterate `scripts/*.sh`, skipping **`board-refresh.sh`** — the single allowlisted writer. No script
   is named; the call-site list is *derived from a glob*, not hand-maintained.
2. Join backslash-continuation lines into logical lines, **then** strip comments, **then** tokenize
   per invocation (`[^;&|]*/render-board\.sh[^;&|]*`).
3. Every surviving token must contain **no file-directed redirect** — any `>` that is not an fd dup
   (`>&2`, `2>&1`).

Because it never matches the target's name, `>"$1/BOARD.md"`, `>>`, `>|`, and `> "$mw/$rel"` all die
identically. It does not ask *where* you are writing; it asserts you are not writing at all.

**Ordering is load-bearing.** Continuation-joining must precede tokenization. The current tokenizer is
line-oriented (`grep -oE` reads one physical line at a time), so a redirect parked on a continuation
line hands it a first-line token carrying `--format digest` and no `>` — a clean pass. `REDIRECT_RE`
would have caught that only because it flattens the file with `tr '\n' ' '` first; that flattening is
not incidental. Guard 1 subsumes the flattened scan **only if** it joins logical lines, and dropping
the scan is safe only because it does.

### Guard 2 — `REDIRECT_RE`, re-scoped and re-justified (kept, not widened)

Unchanged byte-for-byte, but scanning **only `skills/*/SKILL.md`**. Its narrow shape is *correct* for
prose: bracket placeholders and flattened blockquotes are live false-positive classes there, and
nobody writes `>"$f"` in documentation.

The ~15-line design comment is **re-derived, not deleted** — narrowed to prose, and gaining the
sentence currently missing from the file:

> This regex defends **prose**. Shell scripts are guarded by the repo-wide write sentinel, which can
> be far wider precisely because prose hazards (bracket placeholders, blockquotes) cannot occur in a
> script.

That asymmetry is the insight the change exists to record. A future reader who finds a regex defending
against markdown blockquotes, aimed at a bash script that can contain none, either cargo-cults it or
deletes it — and neither outcome is acceptable for a guard this load-bearing.

### Guard 3 — the `--format digest` flag check (kept, tokenizer aligned)

Stays in `tests/test_docket_status.sh`. It guards a *different* property — docket-status's calls are
the digest projection, not the markdown board — and Guard 1 does not subsume it. It inherits the same
continuation-joining fix. Its version of the tokenizer bug produces a **false positive** (flags on a
legitimate invocation whose flag sits on a continuation line) rather than a false negative, so it is
loud rather than silent; it is fixed anyway, because leaving a known-broken tokenizer beside a fixed
one is how the next author cargo-cults the broken one.

## Rejected alternatives

- **Widen `REDIRECT_RE`** (the stub's option (a)). Cannot be made complete: the variable-target
  evasion (`> "$mw/$rel"`) is unreachable by any regex keyed on `BOARD\.md`.
- **Replace source-text matching with a filesystem-effect test** (the stub's option (b)) — run the
  orchestrator against a fixture and assert `BOARD.md`'s content equals the markdown render. This is
  *syntax*-independent (it catches any write form, including ones no regex author anticipated) but
  *path*-dependent: a rogue redirect on a branch the fixture never executes — the rebase-conflict path,
  for instance — sails through. **Deferred, not rejected on merit.** It earns its cost when there is a
  write path a source scan cannot reach; today there is not.
- **Keep both guards without the continuation fix.** Their union still leaves a live hole:
  continuation **and** no-space (`\` newline, then `>"$f"`) evades the tokenizer *and* `REDIRECT_RE`.

## Verification — mutation, not inspection

Guard 1 ships with a positive-control battery. Each mutation is injected into a throwaway fixture
script and **must** turn the guard red; the last row must keep it **green**.

| mutation | evades the guards today? |
|---|---|
| `> "$dir/BOARD.md"` | no — already caught |
| `>"$dir/BOARD.md"` | **yes** |
| `>> "$dir/BOARD.md"` | **yes** |
| `>\| "$dir/BOARD.md"` | **yes** |
| `> "$mw/$rel"` (variable target) | **yes** |
| `\` + newline, then `> "$f"` | **yes** |
| `>&2`, `2>&1` (fd dup) | must stay GREEN — false-positive control |

The final row carries as much weight as the rest: `docket-status.sh`'s current, correct invocation is
`out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest 2>&2)"`. A guard that
fires on that is a guard someone disables.

This battery is the change's substance. Per the ledger (#64, 2026-07-13): *a guard is code —
mutation-test it before trusting it, or it is decoration. A grep sentinel must tokenize at the unit it
claims to guard.* The unit here is the **invocation**, and in shell an invocation spans physical lines.

## Scope

Test-only. `tests/test_render_board.sh`, `tests/test_docket_status.sh`, and their fixtures.

**No production code changes.** `render-board.sh`'s STDOUT contract, `board-refresh.sh`'s gated write,
and `docket-status.sh`'s read-only digest call are all correct today. This change is about the suite's
ability to *notice* if a future one is not.

## Open questions resolved

1. **Widen or replace?** Neither as posed — reframe. Prohibit the write instead of recognizing the
   target; the effect-test is deferred with a stated trigger.
2. **Which false-positive classes still bind?** All of them, for **prose**. None, for shell. One regex
   could not serve both targets — hence the split.
3. **Is `>>` worth guarding, or a bug?** A bug, rejected without special-casing. The board is a full
   regeneration, never an accumulation; Guard 1 rejects append for the same reason it rejects every
   other write from a non-allowlisted script — not because append is uniquely wrong, but because no
   write is permitted at all.
