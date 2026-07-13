# docket-status: a self-evidencing, board-independent report

**Change:** 0069
**Date:** 2026-07-13
**Status:** design
**Follow-up to:** change 0059 (board-refresh surface gate), change 0058 (docket-status orchestrator)

## Problem

In a repo with `board_surfaces: []`, a `docket-status` pass burns a large number of tokens
hunting for a `BOARD.md` that configuration says must not exist.

Observed live in `cet-terraform-modules` (`board_surfaces: []`, `terminal_publish: false`) under
Cursor/Grok, dispatched as `docket-implement-next`'s unconditional Step-0 merge sweep. The
subagent ran the orchestrator, got a near-empty report, decided "the report looks thin", and then
re-ran the whole pass, read `BOARD.md`, ran `git status` on the docket branch, read
`scripts/docket-status.md`, and finally `bash -x` traced `docket-status.sh` — at which point it
discovered `BOARD_SURFACES` was empty and the board pass was a no-op all along.

### Root cause: silence is not evidence

The board gate itself is **correct**. Change 0059 did its job: `board-refresh.sh` owns the
`BOARD.md` write decision and refuses to render when `inline` is absent from the resolved
surfaces, and `docket-status.sh`'s `board_pass()` returns before even calling it:

```bash
board_pass(){
  local surfaces="${BOARD_SURFACES:-}"
  [ -n "$surfaces" ] || return 0      # <-- correct, but emits zero bytes
```

The defect is in the **report**, not the logic. Two facts compose into the failure:

1. **The pass can emit an empty report.** With the board off, nothing merged, and no health
   findings, `docket-status.sh` exits `0` having printed *nothing at all* on stdout. To a model,
   "exit 0 + empty stdout" is indistinguishable from "the script silently did nothing." There is
   no positive evidence that the board was deliberately skipped.

2. **Every prose surface promises a board.** The skill `description` ("…by regenerating the
   `BOARD.md` board…"), the Overview ("three jobs: render `BOARD.md`, sweep…, run health
   checks"), the Final summary step ("board state (counts/highlights)… Point the user at
   `BOARD.md`"), and `agents/docket-status.md`'s body ("Execute docket-status to refresh the
   board") all state a board will be produced. That `description` is also what the
   `docket-implement-next` Step-0 dispatch prompt paraphrases, so the promise propagates into the
   subagent's instructions verbatim.

Told to expect a board and handed silence, the agent's hunt was the rational response.

### The hole underneath

There is a deeper gap that made the hunt *necessary* rather than merely tempting: **with the board
off, the orchestrator gives the skill no backlog-state channel at all.** It emits only `board`,
`swept`, `check`, and `judgment` lines. Yet the skill is still instructed to summarize backlog
state, and its stated primary job is answering "what is done, what is next, or what is stuck."
Opening `BOARD.md` was the only way to comply. Fixing only the `board off` line would leave that
instruction in place and the hunt would continue.

## Design

Make the report **self-evidencing** (it always states what it did) and **board-independent**
(backlog state no longer flows through the board). Three seams.

### 1. `render-board.sh --format digest`

Add an output format to the existing pure renderer:

| Invocation | Output |
|---|---|
| `render-board.sh` | the markdown board (today's output, **byte-identical**) |
| `render-board.sh --format digest` | a line-oriented backlog digest |

`render-board.sh` already runs the dependency-resolution/readiness pass and already writes to
STDOUT, so this is a **second projection of a pass that already exists** — no new logic, and the
resolution pass keeps exactly one owner. `--format markdown` is the default; an unknown `--format`
value is an argument error (exit 2).

Digest shape, one record per line, space-separated:

```
backlog <status> <count>                        # one per non-zero status
change <id> <status> <readiness> <slug>         # one per active change
```

`<readiness>` reuses the board's existing readiness computation, as a machine-parseable token:
`build-ready`, `needs-brainstorm`, `auto-groom-blocked`, `waiting-on-<N>-unbuilt`,
`waiting-on-<N>-needs-merge`, or `-` when readiness does not apply (e.g. `in-progress`).

### 2. `docket-status.sh`

- **`board_pass()` emits `board off`** when `BOARD_SURFACES` is empty, instead of returning
  silently. Positive evidence that the skip was deliberate.
- **New `backlog_pass()`, ungated.** Calls `render-board.sh --format digest` and passes its lines
  through. This is the load-bearing boundary of the design: **the digest is report output, not a
  board surface.** It therefore runs regardless of `board_surfaces` — which is precisely what lets
  `[]` keep meaning "no board is rendered or committed" while backlog state still reaches the
  report. It performs **no git operations**: no write, no commit, no push, nothing to `BOARD.md`.
- **It runs in both modes**, placed **before** the `--board-only` early exit. `--board-only` is the
  skill's "user just wants to *see* the backlog" path; today, in a board-off repo, that path does
  literally nothing and returns nothing. After this change it reports the backlog in every config.
- **`main()` always closes with `pass ok`.** Stdout is now **never empty** under any
  configuration, so "thin" can never again read as "broken."

Delegating to `render-board.sh` preserves `docket-status.md`'s standing invariant — *"it does not
reimplement rendering, archiving, health checks, or publishing logic that already lives in
`render-board.sh`…"*. The orchestrator sequences and prefixes; it does not compute readiness.

`board-refresh.sh` is **untouched**: it still exclusively owns the gated `BOARD.md` *write*. The
split is clean — **board-refresh gates the surface, render-board serves the report.**

### 3. Prose — the part that actually stops the hunt

The script changes give the agent evidence; the prose changes stop instructing it to ignore that
evidence.

- **`skills/docket-status/SKILL.md`**
  - Overview and Final summary gain a **board-off branch**: when `board_surfaces` is empty there is
    no board, and the summary is written from the digest lines.
  - An explicit rule, stated once and plainly: **a thin report is the success case, not a symptom.**
    An empty sweep, no health findings, and `board off` together mean a healthy repo — the pass is
    complete and no investigation is warranted.
  - An explicit prohibition: **never probe `BOARD.md`.** With the board off it must not exist; with
    the board on, summarize from the digest rather than opening the file. The report is the channel
    in both cases.
- **`skills/docket-status/SKILL.md` frontmatter `description` and `agents/docket-status.md`
  (description + wrapper body)** go **board-neutral** — "refresh docket state," not "regenerate the
  `BOARD.md` board." This is what stops `docket-implement-next`'s Step-0 dispatch prompt from
  promising a board the repo has disabled.
- **`scripts/docket-status.md`** output-contract table gains `board off`, `backlog <status>
  <count>`, `change <id> <status> <readiness> <slug>`, and `pass ok`; the 7-step sequence gains the
  backlog pass and its ungated posture.
- **`scripts/render-board.md`** documents `--format`.

## Failure posture

The backlog pass is **best-effort**, matching the board pass: a `render-board.sh --format digest`
failure logs a diagnostic to stderr, emits no digest lines, and **never aborts the pass**. `pass ok`
is still printed — its meaning is "the orchestrator ran to completion," which remains true. A hard
error (config export failure, bad bootstrap verdict, unusable metadata worktree, bad argument) still
exits non-zero and prints no `pass ok`, so the line is a reliable completion signal.

## Testing

`tests/test_docket_status.sh`:

- Board-off run (`BOARD_SURFACES=`) emits `board off`, the digest lines, and `pass ok` — and its
  stdout is **never empty**.
- Board-off run performs **no** git write and leaves `BOARD.md` absent/untouched (the 0059 gate
  must not regress).
- Board-on run still renders, commits, and pushes `BOARD.md`, and *also* emits the digest + `pass ok`.
- `--board-only` reports the backlog in **both** configs.
- Digest readiness tokens are correct for each band: build-ready, needs-brainstorm, auto-groom
  blocked, and both waiting-on flavors.
- A failing `render-board.sh --format digest` degrades best-effort: no digest lines, still `pass ok`,
  still exit 0.

`tests/test_render_board.sh` (or the existing render-board coverage):

- **Regression guard:** default (no `--format`) markdown output is **byte-identical** to today's.
- `--format digest` emits the documented line shapes; an unknown `--format` exits 2.

## Out of scope

- Changing what `board_surfaces: []` **means**. It still disables the board entirely; the digest is
  a report, not a surface, and is never persisted or committed.
- The `github` board surface and `github-mirror.sh`.
- `board-refresh.sh`. Its gate is correct — 0059 was right; only the report was wrong.
- The stray untracked `BOARD.md` seen in the affected repo's worktree. That is a downstream
  artifact of this confusion (an earlier run hand-rendered one), not a cause; it needs a one-off
  cleanup, not a code change.
