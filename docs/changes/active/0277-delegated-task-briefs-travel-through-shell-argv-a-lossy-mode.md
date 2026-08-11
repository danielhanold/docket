---
id: 277
slug: delegated-task-briefs-travel-through-shell-argv-a-lossy-mode
title: 'Delegated task briefs travel through shell argv, a lossy model-performed transformation'
status: implemented
priority: medium
type: refactor
created: 2026-08-09
updated: 2026-08-11
depends_on: []
related: [208, 270]
discovered_from: [271]
adrs: [82]
spec: docs/superpowers/specs/2026-08-09-delegated-brief-file-channel-design.md
plan: docs/superpowers/plans/2026-08-10-delegated-brief-file-channel.md
results: docs/results/2026-08-10-delegated-task-briefs-travel-through-shell-argv-a-lossy-mode-results.md
trivial: false
auto_groomable: true
branch: feat/delegated-task-briefs-travel-through-shell-argv-a-lossy-mode
pr: https://github.com/danielhanold/docket/pull/194
blocked_by:
claimed_at: 2026-08-11T01:33:01Z
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-delegated-brief-file-channel-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-delegated-brief-file-channel-design.md) |
| Plan | [2026-08-10-delegated-brief-file-channel.md](https://github.com/danielhanold/docket/blob/feat/delegated-task-briefs-travel-through-shell-argv-a-lossy-mode/docs/superpowers/plans/2026-08-10-delegated-brief-file-channel.md) |
| Results | [2026-08-10-delegated-task-briefs-travel-through-shell-argv-a-lossy-mode-results.md](https://github.com/danielhanold/docket/blob/feat/delegated-task-briefs-travel-through-shell-argv-a-lossy-mode/docs/results/2026-08-10-delegated-task-briefs-travel-through-shell-argv-a-lossy-mode-results.md) |
| PR | [#194](https://github.com/danielhanold/docket/pull/194) |
| ADRs | [ADR-0082](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0082-generated-shim-emits-brief-write-and-launch-as-one-harness-call.md) |
<!-- docket:artifacts:end -->

## Why

A delegated run's task brief reaches the child through shell argv, and the transformation from
"what my caller told me" to "a correctly quoted shell argument" is performed by a model, in one
shot, with no verification. That is a lossy step in the one channel that carries the child's
entire input — it inherits no conversation, no plan, and no task otherwise.

Two distinct defects live here. Change 0271 fixed the first (omission: the shim rendered the
payload as an optional trailing `[-- <caller args>]`, and models dropped it entirely on live
dispatches — the child then improvised from the worktree and the dispatch looked successful).
This stub is the second: even when the model DOES pass the brief, argv mangles it.

All three runner adapters interpolate the caller arguments as `$*`, not `"$@"`:

- `scripts/runners/opencode.sh`, `scripts/runners/codex.sh`, `scripts/runners/cursor.sh` each
  build the prompt with a literal `Additional caller arguments / task context:` heading followed
  by `$*`.

`$*` joins the positional parameters on the first character of `IFS`. A multi-line brief passed
as several arguments is therefore flattened to a single space-joined line: plan-task structure,
code blocks, and file lists all lose their boundaries, silently. 0271's shim now instructs the
model to pass ONE single-quoted argument, which avoids the join — but that is an instruction the
model must follow correctly every time, not a property of the mechanism.

## What

Groomed 2026-08-09 (auto-groom; design and full assumption audit in the linked spec). The settled
shape:

- `runner-dispatch` gains `--brief-file <path>` (both `--launch` and the legacy foreground verb).
  The caller writes the brief with a quoted-delimiter heredoc — no shell quoting of the content —
  and passes the path. `--launch` spools the brief atomically into the per-dispatch dir as
  `$DDIR/brief` (durable audit record, no caller-temp lifetime race) and hands the adapter the
  durable copy; the legacy verb passes the caller's file through. Stdin rejected: it does not
  survive detachment (`</dev/null` by design) without a hidden spool.
- Both channels present (brief file AND trailing argv) ⇒ **refuse**, at the facade and
  defensively in the adapters — the only shape with no silent-wrong-answer mode.
- A `build-*` dispatch with no payload at all dies at the same pre-verb validation point as the
  existing `build-*` `--worktree` gate (verb-neutral); non-build agents stay legal payload-free.
- All three adapters switch `$*` to a newline-preserving `"$@"` join regardless, so the surviving
  argv path stops being lossy; with `--brief-file` the prompt appends the file verbatim.
- The shim template teaches ONE path (heredoc write, then `--brief-file`), rendered unbracketed
  and emphatic per 0271's constraint; the single-quoting gymnastics paragraph is deleted with the
  argv teaching. Brief retention rides the dispatch dir's existing prune — no new lifecycle.

## Notes

Validation constraint inherited from 0271: Claude Code loads agent definitions at PROCESS START,
so a shim change cannot be tested in the session that made it. Proven on 0271 — a wrapper edited
on disk to `--runner opencode2` still dispatched `--runner opencode` on the next call. Any test of
this change must restart the harness before concluding anything.

## Reconcile log

### 2026-08-10 — implementer reconcile (change claimed for build)

Design re-validated against current `origin/main`; every premise in the spec still holds. All three
adapters still interpolate `$*`; `emit_shim` still bakes the single-quoted-argument paragraph; the
`build-*` `--worktree` gate still sits at pre-verb validation, so the empty-payload gate lands beside
it and stays verb-neutral. 0271 is `done`, so `depends_on` stays empty; 0270 has merged and does not
touch this seam; 0208 is queued next on the same two files, so this diff stays tight to its own spec.

One new requirement surfaced and was folded into the spec (`## Reconcile addendum — 2026-08-10`):
the synchronous verb's run gate re-dispatches once with the retry context appended as an EXTRA
trailing argument, which under `--brief-file` would present both channels and trip the adapters' own
defensive refusal. The facade therefore composes a combined brief (brief bytes + blank line + retry
context) into a templated temp file and re-dispatches through the single brief channel.

The `--observe` poll-loop prefix-strip defect class (0286's `gate-run` fix) is NOT this change's:
the spec's out-of-scope line already assigns it to 0284, and reconcile leaves it there.

Budget note carried into the plan: `tests/test_runner_dispatch.sh` has zero headroom against its 10s
row in `tests/runtime-budgets.tsv`, and this change adds cases to that file — re-measure and raise
the row with a measured number.

