<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0207 — sync-agents aborts mid-loop on a bad runner config, leaving a zero-length wrapper and stale siblings](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-05-0207-sync-agents-aborts-mid-loop-on-a-bad-runner-config-leaving-a.md)**
<!-- docket:backlink:end -->

# sync-agents aborts mid-loop on a bad runner config — results

Change: #0207 · Branch: feat/sync-agents-aborts-mid-loop-on-a-bad-runner-config-leaving-a · PR: <url> · Plan: docs/superpowers/plans/2026-08-05-atomic-wrapper-generation.md · ADRs: none

## Verify (human)

- [ ] Confirm the stricter posture is what you want in daily use: **one bad `runner:` entry now refreshes zero wrappers.** On a fresh machine that means no wrappers at all until the config is fixed. The spec accepted this deliberately (today's alternative is an undetectable mixture of fresh, stale, and zero-length wrappers), and an escape-hatch flag was considered and rejected — but you are the one who lives with it.
- [ ] `sync-agents.sh --check` can now emit `unknown agent_harnesses token '…' in <global cfg>` where it previously stayed silent. This falls out of the gate calling `resolve_global_agent_harnesses` on the `--check` path. The suite is green with it and it is arguably more correct for a validation mode, but it is a visible output change to a command you run often.
- [ ] Run `bash sync-agents.sh` once on this machine against your real config and confirm the wrapper set regenerates unchanged.

## Findings

**The gate cost was measured, not assumed** (the plan required this rather than hand-waving it). Three-run averages on the plan's fixture: **0.82s without the gate, 1.36s with** — roughly +0.54s, from doubling `resolve_agent_layers` across both legs (16 agents × 2 passes). The enumeration was deliberately **not** narrowed: narrowing would put the rule's scope in a second place, which is the exact drift the shared predicate exists to prevent, and `claude` was the only harness in the fixture so it would not have recovered the cost anyway.

**The `set -u` hazard was worse than the plan predicted.** The plan anticipated that `$USER_TARGETS` is unset on the `--check` path and proposed calling `compute_user_targets`. The build found that `compute_user_targets` itself reads `$USER_HARNESSES_SET`, which is *also* unset there (`resolve_global_agent_harnesses` never runs on `--check`), so the plan's remedy alone would still have died under `set -u`. Resolved as `[ -n "${USER_HARNESSES_SET:-}" ] || resolve_global_agent_harnesses` followed by `compute_user_targets`. `${USER_TARGETS:-}` was explicitly rejected — an empty list silently skips the whole user-level leg, which is precisely the under-enumeration the gate exists to prevent.

**Review outcome: 7 findings, 0 blocker, 3 important, 4 minor.** No blockers, so nothing was auto-fixed; all 7 are recorded in the PR body for merge-time judgment and carried into a follow-up change. The three important ones are summarized in *Follow-ups* below. No decision in this build was non-obvious enough to warrant a new ADR — the atomicity posture is an application of a mechanism `sync-agents.sh` already used twice (`validate_harness_defaults`, `validate_user_agent_values`), and the required-model rule it restructures is already settled by ADR-0067.

**TDD exception, Task 1 only.** The predicate extraction was behavior-preserving, so a pre-implementation RED would have misrepresented intended behavior. Substituted an exact green-baseline match (597/597 asserts before and after) plus the plan's required mutation test on the ORDERING FENCE — swapping the two `if` blocks reddened `0205: an unregistered AND model-less runner still reports the REGISTRATION failure first`, and the swap was reverted. Task 2 was strict RED-first: 9 asserts failed for the intended reasons before the gate landed.

## Follow-ups

Captured as a new change (see the change's `## Artifacts` / the board): **clear the unfixed review findings from change 0207.** The three important findings:

1. **The gate can under-enumerate on `--check`.** `validate_runner_config`'s project-level leg is guarded by `per_repo_opted_in`; `check_project_level`'s leg (c) — the third `emit_wrapper` call site — is guarded by the strictly weaker `gitignore_block_wanted`. A global config whose `agent_harnesses:` omits `claude` lets a bad `runner:` escape the gate and die inside leg (c) with the raw assertion. Diagnostic quality only (the write path enumerates correctly), but the in-source comment claiming no gap exists is false.
2. **The gate's user-level leg has no test.** Every `runner:` fixture in every suite lives in `.docket.yml`, so the entire `$USER_TARGETS` block — including the `set -u` resolution above — is mutation-survivable. That is the leg protecting `~/.claude/agents`, the widest blast radius of the original bug.
3. **`user_flag_model`'s "spelled once" claim is false.** `emit_wrapper` keeps its own copy of the provenance filter computed from positional `$2` rather than `$RES_MODEL`. They agree only because all three call sites happen to pass `$RES_MODEL` — undocumented and unenforced.

Four minor findings (a vacuous ordering assert, overbroad "changes nothing on disk" wording given `migrate_legacy_global`, duplicate diagnostics when both legs see the same global offender, and a `--check` assert that was already true pre-change) are carried in the same follow-up.
