---
id: 334
slug: claude-md-global-and-project-copies-out-of-sync-on-finalize-a
title: Make Docket dispatch minimal, non-recursive, and mechanically gated
status: 'implemented'
priority: high
type: fix
created: 2026-08-21
updated: '2026-08-26'
depends_on: []
stacked_on:
related: [294]
discovered_from: [317]
adrs: []
spec: docs/superpowers/specs/2026-08-25-consolidate-dispatch-block-subagent-guard-design.md
plan: 'docs/superpowers/plans/2026-08-26-minimal-non-recursive-gated-dispatch.md'
results: 'docs/results/2026-08-26-claude-md-global-and-project-copies-out-of-sync-on-finalize-a-results.md'
trivial: false
auto_groomable:
branch: 'feat/claude-md-global-and-project-copies-out-of-sync-on-finalize-a'
pr: 'https://github.com/danielhanold/docket/pull/239'
blocked_by:
reconciled: true
claimed_at: '2026-08-26T02:36:27Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-25-consolidate-dispatch-block-subagent-guard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-25-consolidate-dispatch-block-subagent-guard-design.md) |
| Plan | [2026-08-26-minimal-non-recursive-gated-dispatch.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-26-minimal-non-recursive-gated-dispatch.md) |
| Results | [2026-08-26-claude-md-global-and-project-copies-out-of-sync-on-finalize-a-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-26-claude-md-global-and-project-copies-out-of-sync-on-finalize-a-results.md) |
<!-- docket:artifacts:end -->

## Why

The parent-facing dispatch rule can reach the named agent it just dispatched, causing that
agent to dispatch another instance of itself. Change 0317 observed a three-deep
`docket-status` tree before the actual status charter ran.

The same always-loaded block repeats every native agent description and asks the model to
execute the run gate's attribution and retry procedure by hand. A separate personal-global
Claude copy adds another independently drifting source. The result is recursive execution,
avoidable context weight, and safety policy spread across generated prose and scripts.

## What changes

- Inject an exact-name self-recursion guard through the shared wrapper-emission path
  while preserving every required dispatch to a *different* agent. No such guard exists
  today, and no wrapper body currently references "your preloaded skill".
- Replace the always-loaded dispatch block's per-agent roster with a compact rule that
  treats each harness's native agent registry as authoritative. The block today enumerates
  all 17 agents (one compact line each, derived from the agent inventory) — that roster is
  what shrinks.
- Move the foreground / detached / unattributed run-gate mechanics behind a durable
  `gate-before` / `gate-verdict` facade with opaque per-dispatch state and atomic one-retry
  accounting. Today those mechanics are hand-executed prose the model runs, shipped from an
  authored payload; the facade replaces the prose with two commands over durable state.
- Retire the hand-authored global Claude dispatch block at the human merge gate, leaving the
  generated per-repository surface authoritative for Claude, Codex, and OpenCode; Cursor keeps
  its distinct routing rule while consuming the same compact gate facade.
- Absorb change 0294's dispatch-roster and run-gate slimming scope into this change.

**Implementation reality (reconciled 2026-08-26).** Generation has moved into the Go binary
since this change was drafted: the per-harness wrapper and dispatch-block emitters now live under
`internal/harness/` (`dispatch.go`'s `DispatchInterior`/`RunGate`, the `claude`/`codex`/`cursor`/
`opencode` adapters, `inventory.go`), and `install.sh` drives the Go binary — not `sync-agents.sh`.
The shell `sync-agents.sh` (with `assemble_agents_md_dispatch`) still coexists as a `--check`-tested
mirror that must stay byte-identical, so every emitter change lands in BOTH the Go adapter and the
shell generator in lockstep, and both test surfaces must move together. The compact routing rule,
the recursion guard, and the compact gate trigger all originate from the authored payload sources
(`cursor-rules/run-gate.md` and its embedded copy under `internal/assets/embedded/`, plus the
per-agent `cursor-rules/dispatch/` fragments) consumed by both generators. The `gate-before` /
`gate-verdict` facade is implemented as new Go `docket` subcommands (spelling to follow the repo's
existing `docket <group> <verb>` conventions, as the spec permits) that reuse and generalize the
dispatch-record machinery already in `scripts/lib/docket-dispatch-dir.sh` (state root under the git
common dir, key mint/validation, atomic replacement, retention) and delegate the run predicate to
the existing `verify-run` / `internal/app/run_verify.go`. No behavior in `verify-run`'s predicates
or the ADR-0075/0084/0088 safety policy changes.

## Out of scope

- Changing `verify-run` predicates, run-gate safety policy, model/effort selections, or
  cross-agent composition.
- Unifying Cursor's routing surface with `AGENTS.md`/`CLAUDE.md`.
- Automating edits to the user's personal global `~/.claude/CLAUDE.md`; its removal is a
  human acceptance prerequisite.
- Pruning unrelated always-loaded instructions.

## Open questions

None remaining for the design. Reconcile confirmed the two design dependencies the spec
flagged are settled in current reality: change 0294 is already killed and archived (its scope is
absorbed here), and generation has already moved to the Go binary with a shell mirror, so the
"reuse the existing dispatch-record machinery" and "single shared emitter" assumptions have concrete
homes (`scripts/lib/docket-dispatch-dir.sh` and `internal/harness/` + `sync-agents.sh`). The
four-harness external behavioral acceptance and the removal of the personal global
`~/.claude/CLAUDE.md` block remain human merge-gate preconditions, not implementation work.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-26

### 2026-08-26 — reconcile before planning

Re-read the change + spec against current code, the `related`/`discovered_from` changes, and the
cited ADRs (0016/0024/0059/0075/0078/0084/0088). Findings:

- **294 already killed.** Change 0294 (the absorbed roster/run-gate slimming scope) is already
  archived as killed (2026-08-26); the absorb-and-retire PM op the spec calls for is done. `related:
  [294]` retained as historical linkage; no relations change needed.
- **Generation moved to Go.** The spec assumes shell-based generation (`sync-agents.sh`). In current
  reality the production generator is the Go binary (`internal/harness/` adapters + `dispatch.go`
  `DispatchInterior`/`RunGate`; `install.sh` runs the binary). `sync-agents.sh` still exists as a
  `--check`-tested byte-identical mirror. Both must change in lockstep; the spec's acceptance already
  names `test_sync_agents_claude_surface.sh` as remaining green, consistent with the shell path
  staying. Recorded this in `## What changes` so the plan targets both generators + their tests.
- **No recursion guard exists yet**, and no wrapper body says "your preloaded skill" — the guard is
  net-new, injected once through the shared emitter for all four harnesses.
- **The run-gate algorithm is authored prose**, shipped from `cursor-rules/run-gate.md` (+ embedded
  copy), appended verbatim by both generators; the three-filter attribution is a hand-executed
  procedure, not code. The facade will implement it as Go subcommands reusing
  `scripts/lib/docket-dispatch-dir.sh` and delegating to `verify-run`.
- **No `gate-before`/`gate-verdict` command exists** today; `docket gate` currently exposes
  launch/observe/stop/recover/cleanup and the `gate drive` group only.
- **Design is NOT invalidated** — all five decision parts remain implementable against current
  reality. Scope unchanged; auto-capture disabled this run (no stubs minted). Global-copy removal and
  four-harness fresh-process acceptance stay human merge-gate preconditions.

## Finalize blocked

### 2026-08-26 — attempt 20260826T110316Z-d736ecdcc93c

<!-- attempt:20260826T110316Z-d736ecdcc93c -->

- Reason: rebase-conflict-unresolved
- Head: 7d84970d91f6c16fed36f29fc9b321d1498fadf2
- PR: #239
- Comment: https://github.com/danielhanold/docket/pull/239#issuecomment-5424467072

Remedy: A human should reconcile this branch onto main manually (the conflicts are the runtime-budget EXPECTED_TOTAL pin and the embedded-asset manifest digest, both mechanically resolvable — re-run the asset generator and re-sum the budget table after rebasing), push the rebased head, then re-run docket-finalize-change by naming change id 334 to clear this block and merge. Alternatively, if the installed docket-rebase-resolver agent is intended to drive the whole rebase to completion in a single dispatch (its registered contract says so), that mismatch with the skill body's two-dispatch controller loop should be reconciled before retrying.
