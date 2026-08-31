---
id: 207
slug: sync-agents-aborts-mid-loop-on-a-bad-runner-config-leaving-a
title: sync-agents aborts mid-loop on a bad runner config, leaving a zero-length wrapper and stale siblings
status: done
priority: medium
type: fix
created: 2026-08-05
updated: 2026-08-05
depends_on: [205]
related: [206]
discovered_from: [205]
adrs: []
spec: docs/superpowers/specs/2026-08-05-atomic-wrapper-generation-design.md
plan: docs/superpowers/plans/2026-08-05-atomic-wrapper-generation.md
results: docs/results/2026-08-05-sync-agents-aborts-mid-loop-on-a-bad-runner-config-leaving-a-results.md
trivial: false
auto_groomable:
branch: feat/sync-agents-aborts-mid-loop-on-a-bad-runner-config-leaving-a
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/159
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-05-atomic-wrapper-generation-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-05-atomic-wrapper-generation-design.md) |
| Plan | [2026-08-05-atomic-wrapper-generation.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-05-atomic-wrapper-generation.md) |
| Results | [2026-08-05-sync-agents-aborts-mid-loop-on-a-bad-runner-config-leaving-a-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-05-sync-agents-aborts-mid-loop-on-a-bad-runner-config-leaving-a-results.md) |
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

## Reconcile log

### 2026-08-05

Reconciled against `origin/main` at `2d1a3e9e`. The design holds unchanged — no scope adjustment,
no folding-in, no drop.

- **Dependency 0205 is now `done`** (merged; archived as
  `2026-08-05-0205-opencode-runner-adapter.md`). The spec's *Dependencies* section still describes
  it as `implemented` on `feat/opencode-runner-adapter` / PR #156. That is now stale prose only:
  the required-model rule this change restructures is present on `main`, so the feature branch cuts
  from `origin/main` normally and no cross-branch build is needed. Sibling 0206 is also merged,
  and its work was confined to `scripts/runner-dispatch.sh` — the predicted no-collision held.
- **Every structural claim in the spec re-verified against `main`.** `sync-agents.sh` lives at the
  **repo root**, not under `scripts/`. `emit_wrapper` is at line 832 and still carries both inline
  `log ERROR` + `exit 1` blocks — the registration check (line 839) above the required-model check
  (line 863), the order the spec requires the shared predicate to preserve. The redirected call
  shape, the `RES_MODEL_FROM_USER` provenance filter, the non-claude reserved-runner warn path, and
  `is_registered_runner` (line 88) are all exactly as described.
- **The gate-placement plan is confirmed by the real `main()` body.** The `--check` branch returns
  after `validate_harness_defaults` + `validate_user_agent_values`, and the real-run path runs those
  two gates, then `migrate_legacy_global`, then `resolve_global_agent_harnesses`, then
  `user_level_pass`. The new gate 3's slot — below `resolve_global_agent_harnesses`, above
  `user_level_pass` — is available exactly as specified, and the comment it must mirror is present
  verbatim above gate 2.
- **The migrated test is where the spec says**, though line numbers have drifted slightly from the
  spec's citations: the `! -s` assertion and the comment explaining why `-s` rather than `-e` was
  chosen sit in the change-0205 block of `tests/test_sync_agents.sh`, and the
  registration-before-required-model ordering test follows it. The plan should locate both by
  content, not by the spec's line numbers.

No follow-up work minted. Gate 2's pre-migration blind spot is a documented design boundary already
recorded in *Out of scope* — a deliberate decision, not a newly discovered defect.

#### Resume re-reconcile (same day, post-build)

The run stopped at the build/review boundary and was resumed. `origin/main` had advanced
`2d1a3e9e` → `18195d91` in the interim, so the resume rule's second trigger fired and the pass was
re-run. **Substantively a no-op:** the new commits are all change 0202, which touched
`scripts/board-checks.sh`, `tests/test_board_checks.sh`, and `tests/test_docket_status.sh` — zero
overlap with this change's files (`sync-agents.sh`, `tests/test_sync_agents.sh`, `README.md`). No
scope change, no new constraint, nothing to fold in. Recorded rather than skipped so the audit
signal stays honest about which base the build was validated against.
