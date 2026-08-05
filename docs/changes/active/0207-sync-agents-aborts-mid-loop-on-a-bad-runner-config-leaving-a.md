---
id: 207
slug: sync-agents-aborts-mid-loop-on-a-bad-runner-config-leaving-a
title: sync-agents aborts mid-loop on a bad runner config, leaving a zero-length wrapper and stale siblings
status: proposed
priority: medium
type: fix
created: 2026-08-05
updated: 2026-08-05
depends_on: [205]
related: [206]
discovered_from: [205]
adrs: []
spec: docs/superpowers/specs/2026-08-05-atomic-wrapper-generation-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-05-atomic-wrapper-generation-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-05-atomic-wrapper-generation-design.md) |
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

Make wrapper generation **atomic**: a run either regenerates every wrapper or changes nothing on
disk. `nginx -t` semantics — the whole configuration is validated as a set, and a failed validation
leaves the previously generated wrappers in place, on the assumption that what was already there
was working.

This is not a new mechanism. `sync-agents.sh` already gates exactly this way twice
(`validate_harness_defaults`, `validate_user_agent_values`) and documents the posture in its own
comments; the runner checks are a third leg of that gate that was never migrated into it. The
stub's original suggestion — accumulate a failure flag and exit after the passes complete — was
rejected during grooming: it would leave the file holding two contradictory postures and still ship
a mixed-state agent directory.

- Extract both runner rules into one shared predicate — the single source of truth for the rules,
  their scope, their diagnostics, and their ordering (registration before required-model).
- Add a pre-flight gate that walks every candidate (pass, agent, harness) triple, reports **all**
  offenders in one run, and fails before the first wrapper write. It sits above `user_level_pass`
  but below the legacy-config migration, so its resolution is accurate; `--check` gets the same leg.
- Reduce `emit_wrapper`'s inline checks to a can't-happen assertion covering future call sites.

Deliberately stricter than today: one bad runner entry now refreshes zero wrappers. That is the
trade — today's alternative is not "more wrappers" but an undetectable mixture of fresh, stale, and
zero-length ones. An escape-hatch flag was considered and rejected.

Design: `docs/superpowers/specs/2026-08-05-atomic-wrapper-generation-design.md`.

## Out of scope

- The required-model rule itself and its runner-wide scope (ADR-0067, settled).
- Gate 2's pre-migration blind spot (`validate_user_agent_values` runs before
  `migrate_legacy_global`). Real and pre-existing; only fixable by hoisting a write above every
  gate, which would break `--check`'s read-only property. Noted in the spec, not fixed here.
