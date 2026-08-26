---
id: 334
slug: claude-md-global-and-project-copies-out-of-sync-on-finalize-a
title: Consolidate the docket dispatch block — subagent guard + single per-repo source
status: proposed
priority: high
type: fix
created: 2026-08-21
updated: 2026-08-25
depends_on: []
stacked_on:
related: []
discovered_from: [317]
adrs: []
spec: docs/superpowers/specs/2026-08-25-consolidate-dispatch-block-subagent-guard-design.md
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
| Spec | [2026-08-25-consolidate-dispatch-block-subagent-guard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-25-consolidate-dispatch-block-subagent-guard-design.md) |
<!-- docket:artifacts:end -->

## Why

The "Docket agents — dispatch, don't run inline" block tells a session to dispatch the
matching model/effort-pinned agent instead of running a docket skill inline. Correct for a
**top-level human session** — it is what preserves each agent's model pin, dispatch
contract, and skill preload. But the block has **no subagent guard**, so a docket-* agent
that is *itself* dispatched reads the same rule and re-dispatches itself instead of running
its preloaded skill — recursing until some depth happens to run it. Observed live during
change 0317's four-harness acceptance (Claude harness): `docket-status → docket-status →
docket-status`, with two of the session's `subagents/agent-*.jsonl` transcripts each
carrying their own `subagent_type:"docket-status"` dispatch before one child finally ran.

This is the same defect this change originally reported as *drift*: the global
`~/.claude/CLAUDE.md` and the in-repo project `CLAUDE.md` carried divergent finalize-agent
descriptions (`docket-integration-repair`, `docket-rebase-resolver`) because there are
**two hand-maintained copies**. There are two independent trigger sites for one bug: (1) the
shared block template written per-repo by `sync-agents.sh` lacks the guard, so every synced
repo recurses too; (2) a hand-authored global copy that both recurses and drifts. Eliminate
the second copy and add the guard to the single generated source, and both the recursion and
the drift are fixed at once. (Design, evidence, and the `sync-agents.sh` machinery:
see the linked spec.)

## What changes

- Add an **identity-based subagent guard** to the single shared dispatch-block template in
  `sync-agents.sh` (mirrors superpowers' `<SUBAGENT-STOP>`): a docket-* agent that finds this
  rule inside its own dispatched context runs its skill **inline** and does not re-dispatch;
  the rule applies to the top-level session only. Content-only; no new per-wrapper wiring.
- Make the **per-repo, sync-agents-generated block the single source** and **retire reliance
  on the global `~/.claude/CLAUDE.md` copy**. Delivery to Claude already exists
  (`claude_surface_target` + the 0242 `CLAUDE.md → AGENTS.md` symlink); verify a Claude-only
  repo receives the guarded block and close any gap. With one generated source the
  global/project drift cannot recur — there is no global copy.
- Do **not** delete the rule — removal would run docket skills inline at the session model,
  discarding the model pin and dispatch contract.

## Out of scope

- Rewriting the finalize agents themselves or their behavior, or the merge-gate architecture
  their descriptions describe.
- Automating edits to the user's personal global `~/.claude/CLAUDE.md` — docket cannot and
  must not touch it; the maintainer deletes the retired global block by hand.

## Open questions

- Does a Claude-**only** repo actually receive the block today via `claude_surface_target`,
  or is there a gap to close? Verify against a fresh synced Claude-only fixture.
- Should a `test_sync_agents_*` test assert the guard clause is present in the emitted block
  (shape assertion, not behavior) to prevent silent removal?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
