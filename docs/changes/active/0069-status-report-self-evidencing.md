---
id: 69
slug: status-report-self-evidencing
title: docket-status report is self-evidencing and board-independent — stop the board-off BOARD.md hunt
status: in-progress
priority: high
created: 2026-07-13
updated: 2026-07-13
depends_on: []
related: [58, 59]
adrs: []
spec: docs/superpowers/specs/2026-07-13-status-report-self-evidencing-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/status-report-self-evidencing
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-13-status-report-self-evidencing-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-13-status-report-self-evidencing-design.md) |
<!-- docket:artifacts:end -->

## Why

In a repo with `board_surfaces: []`, a `docket-status` pass burns a large number of tokens hunting
for a `BOARD.md` that configuration says must not exist.

Observed live in `cet-terraform-modules` under Cursor/Grok, dispatched as `docket-implement-next`'s
Step-0 merge sweep: the subagent ran the orchestrator, got a near-empty report, judged that "the
report looks thin", then re-ran the entire pass, read `BOARD.md`, ran `git status` on the docket
branch, read the orchestrator contract, and finally `bash -x` traced `docket-status.sh` — only then
discovering `BOARD_SURFACES` was empty and the board pass had been a deliberate no-op all along.

The board gate from change 0059 is **correct** and is not the bug. The bug is the **report**:

1. **Silence is not evidence.** With the board off, nothing merged, and no health findings,
   `docket-status.sh` exits `0` having printed *nothing at all*. "Exit 0 + empty stdout" is
   indistinguishable from "the script silently did nothing."
2. **Every prose surface promises a board** — the skill description, Overview, Final summary, and
   the `agents/docket-status.md` wrapper body all state that `BOARD.md` will be regenerated. That
   description is what `docket-implement-next`'s Step-0 dispatch prompt paraphrases, so the promise
   reaches the subagent verbatim.

Told to expect a board and handed silence, the hunt was the rational response. Underneath sits the
real hole: **with the board off the orchestrator provides no backlog-state channel at all**, yet the
skill is still told to summarize backlog state. Opening `BOARD.md` was its only way to comply — so
fixing only the missing line would leave the instruction intact and the hunt would continue.

This is cheap to fix and recurs on every autonomous Step-0 sweep in every board-off repo, which is
why it is `high` rather than `medium`.

## What changes

Make the report **self-evidencing** (it always states what it did) and **board-independent**
(backlog state stops flowing through the board). Design detail is in the linked spec.

- **`render-board.sh` gains `--format digest`** — a line-oriented projection of the
  dependency-resolution/readiness pass it already runs. Default output stays byte-identical.
- **`docket-status.sh`** emits a positive `board off` line instead of returning silently; gains an
  **ungated `backlog_pass()`** that emits the digest (`backlog <status> <count>` rollups + one
  `change` line per active change) in **both** modes, before the `--board-only` early exit; and
  always closes with `pass ok`, so stdout is **never empty** under any configuration. The digest is
  **report output, not a board surface** — it persists nothing and performs no git operations, which
  is what lets `board_surfaces: []` keep meaning "no board is rendered or committed."
- **Prose** — `skills/docket-status/SKILL.md` gains a board-off branch, an explicit "a thin report
  is the success case" rule, and a prohibition on ever probing `BOARD.md`; the SKILL `description`
  and `agents/docket-status.md` go **board-neutral** so the Step-0 dispatch prompt stops promising a
  board. Contracts `scripts/docket-status.md` and `scripts/render-board.md` document the new lines
  and flag.

## Out of scope

- Changing what `board_surfaces: []` **means** — it still disables the board entirely.
- The `github` board surface and `github-mirror.sh`.
- `board-refresh.sh` — its gate is correct; 0059 was right, only the report was wrong.
- The stray untracked `BOARD.md` in the affected repo's worktree: a downstream artifact of this
  confusion, needing a one-off cleanup, not a code change.

## Open questions

None — the design is settled in the linked spec.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-13 — reconciled against `origin/main` @ `97e8437`

**Verdict: the design holds unchanged. No scope drop, no scope growth beyond one adjacent doc-accuracy fix.**

Verified every premise of the spec against the current code rather than the snapshot it was drafted
from:

- `scripts/render-board.sh` still parses only `--changes-dir` / `--repo`; there is no `--format`
  flag and no digest projection. It already runs `resolve_deps` + `readiness` from
  `lib/docket-frontmatter.sh`, so the digest remains a **second projection of a pass that already
  exists**, exactly as designed.
- `readiness()` returns the coarse token `waiting` — the *flavor* (`not yet built` / `needs your
  merge`) and the blocking id live in `DEP_REASON[id]` / `DEP_ON[id]`. The digest's
  `waiting-on-<N>-unbuilt` / `waiting-on-<N>-needs-merge` tokens are therefore a **mapping** the
  digest formatter owns (same source as the board's readiness cell), not a change to `readiness()`.
  No caller of `readiness()` is touched.
- `scripts/docket-status.sh` `board_pass()` still returns silently on an empty `BOARD_SURFACES`
  (`[ -n "$surfaces" ] || return 0`); `main()` still has no completion line, no backlog channel, and
  still `exit 0`s for `--board-only` before the sweep. The empty-stdout failure mode is live.
- The prose promises are all still present verbatim: `skills/docket-status/SKILL.md`'s frontmatter
  `description` ("by regenerating the BOARD.md board"), its Overview ("three jobs: render
  `BOARD.md`…"), its Final summary ("Point the user at `BOARD.md`"), and `agents/docket-status.md`
  (description + body "Execute docket-status to refresh the board").
- **No sentinel pins the board-promising wording.** Grepped the whole suite: no test asserts the
  `description` or wrapper body mentions the board, so going board-neutral breaks nothing. The
  existing output-shape assertions in `tests/test_docket_status.sh` only assert `board inline …`
  lines are *present* — additive `board off` / `backlog` / `change` / `pass ok` lines cannot false-RED
  them.
- **No collision with work in flight.** The only other non-`proposed` change is 0044 (`implemented`,
  PR #69 open); its branch touches `docket-config.{sh,md}`, `README.md`, the convention +
  implement-next skills and two config tests — **zero overlap** with this change's files. Nothing in
  the recent archive (0063–0066) delivered any part of this scope.

**One fold-in (disclosed, not silent):** `scripts/docket-status.md`'s step-3 prose still describes
the inline board pass as rendering "via `render-board.sh` into a `BOARD.md.tmp` file" — that is
pre-existing drift from change 0059, which moved the write decision into `board-refresh.sh` (the
script's actual code calls `board-refresh.sh` and a suite sentinel already forbids it from calling
`render-board.sh` directly). Since this change rewrites that exact section to add the backlog pass,
the stale sentence is corrected in passing rather than left knowingly false beside new prose.
