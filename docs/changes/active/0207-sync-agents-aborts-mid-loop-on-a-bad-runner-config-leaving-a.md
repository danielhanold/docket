---
id: 207
slug: sync-agents-aborts-mid-loop-on-a-bad-runner-config-leaving-a
title: sync-agents aborts mid-loop on a bad runner config, leaving a zero-length wrapper and stale siblings
status: proposed
priority: medium
type: fix
created: 2026-08-05
updated: 2026-08-05
depends_on: []
related: []
discovered_from: [205]
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

`sync-agents.sh`'s `emit_wrapper` fails a bad `runner:` configuration with `log ERROR` followed by
`exit 1`, inline, in the middle of the generation loop. Two call-site facts make that abrupt:

- The call sites redirect the function's stdout into the target path
  (`emit_wrapper … > "$dir/docket-<name>.<ext>"`), so the shell **truncates the wrapper before the
  function body runs**. The offending agent is left with a zero-length file.
- `exit 1` terminates the whole script, so every agent later in glob order is never regenerated,
  and a failure during `user_level_pass` means `project_level_pass` never runs at all.

The mechanism is old — the unregistered-runner check has always worked this way — but change 0205's
runner-wide required-model rule changed its trip rate sharply: a model-less `runner:` was a
*documented supported configuration* until that change, and docket cannot reach or migrate the
config layers that may carry one (a machine-local `.docket.local.yml`, or the global
`~/.config/docket/config.yml`). An affected user now gets a partial regeneration with no statement
that generation stopped early or that the remaining wrappers are stale.

Change 0205 accepted this deliberately rather than expanding its own scope; the tests encode
`! -s` (absent or empty) rather than `! -e` precisely because the zero-length file is expected
today.

## What changes

Make generation-time config failures leave a coherent state. The shape suggested at review:
accumulate rather than exit inline — `emit_wrapper` logs the ERROR, emits nothing, sets a
script-level failure flag, and the script exits nonzero **after** the passes complete, so every
valid wrapper is still regenerated. Remove or avoid creating the zero-length file.

If accumulation is judged too invasive, the minimum is a diagnostic that states generation aborted
early and that the remaining wrappers were not regenerated.

## Out of scope

- The required-model rule itself and its runner-wide scope (ADR-0067, settled).
