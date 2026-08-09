---
id: 277
slug: delegated-task-briefs-travel-through-shell-argv-a-lossy-mode
title: 'Delegated task briefs travel through shell argv, a lossy model-performed transformation'
status: proposed
priority: medium
type: refactor
created: 2026-08-09
updated: 2026-08-09
depends_on: []
related: [208, 270]
discovered_from: [271]
adrs: []
spec: docs/superpowers/specs/2026-08-09-delegated-brief-file-channel-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-delegated-brief-file-channel-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-delegated-brief-file-channel-design.md) |
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
