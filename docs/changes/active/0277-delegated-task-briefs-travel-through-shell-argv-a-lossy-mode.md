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
related: []
discovered_from: [271]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
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

Give `runner-dispatch` a non-argv path for the brief and make it the documented default:
a file path, or stdin. `--launch` already creates a durable per-dispatch directory (change 0271),
which is a natural place to carry the brief alongside the streams and the launch record.

Open questions for the brainstorm, not decisions:

- File path vs. stdin. A path composes with the existing detached launch; stdin does not survive
  detachment without being spooled first.
- Precedence when both a brief file and trailing argv are present — prefer one, refuse, or
  concatenate. Refusing is the only option with no silent-wrong-answer mode.
- Whether the adapters should switch `$*` to a `"$@"`-preserving construction regardless, so the
  argv path stops being lossy even when it is used.
- Lifecycle of the brief in the per-dispatch dir: it may contain the full plan text, so retention
  and cleanup are a real decision, not an afterthought.
- The shim template then has to teach a two-step (write the brief, then reference it), which is
  MORE steps for the model than appending a string. That is the main risk this change carries:
  it removes a lossiness failure mode and may widen the omission failure mode 0271 just closed.
  Any design here must keep 0271's emphatic, unbracketed payload treatment rather than replacing
  it with a bracketed `[--brief-file <path>]`, which would reintroduce the original defect in a
  new spelling.

## Notes

Validation constraint inherited from 0271: Claude Code loads agent definitions at PROCESS START,
so a shim change cannot be tested in the session that made it. Proven on 0271 — a wrapper edited
on disk to `--runner opencode2` still dispatched `--runner opencode` on the next call. Any test of
this change must restart the harness before concluding anything.
