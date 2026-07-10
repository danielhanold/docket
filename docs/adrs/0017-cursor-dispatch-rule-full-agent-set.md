---
id: 17
slug: cursor-dispatch-rule-full-agent-set
title: Per-repo agent generation goes always-full-set, opt-in, with a Cursor dispatch rule
status: Superseded by ADR-20
date: 2026-07-08
supersedes: []
reverses: []
relates_to: [15, 16]
change: 48
---

## Context

ADR-[[0008]] established the agent layer: `.docket.yml` `agents:` → `sync-agents.sh` →
generated, model/effort-pinned subagent wrappers per skill. ADR-[[0015]] made per-repo
generation fan out over an explicit `agent_harnesses:` list and fixed `model:` as a
harness-neutral, unvalidated passthrough — the running harness interprets the string, which
is exactly what lets docket drive a non-Claude harness like Cursor. ADR-[[0016]] made the
`agents:`/global config **harness-first**, resolving `model`/`effort` per `(harness, agent)`
pair. Neither ADR addressed *how many* agents each harness's per-repo pass writes, or how a
non-Claude harness is made to actually honor its pin.

Driving docket through Cursor surfaced two problems the prior ADRs didn't cover:

1. **Cursor doesn't dispatch on direct skill invocation.** When a skill is invoked directly
   in Cursor's agent chat, Cursor runs it inline at whichever model is currently selected —
   it does not route to the skill's bound subagent. The model/effort pins ADR-0008/0015/0016
   exist to enforce are silently defeated the moment Cursor is the harness. The proven
   workaround is an `alwaysApply: true` Cursor rule that forces a Task dispatch to the
   matching `subagent_type` instead of running inline.
2. **The per-repo generation pass was listed-only.** It generated committed wrapper files
   only for the agents named in `.docket.yml`'s `agents:` block. But docket's agents
   **compose** (ADR-[[0008]]'s Composition: `docket-implement-next` dispatches
   `docket-status`/`docket-adr`; `docket-finalize-change` dispatches
   `docket-rebase-resolver`/`docket-integration-repair`; `docket-auto-groom` dispatches its
   critic). A dispatch rule naming the full composed set would have unresolved targets for
   any agent the repo hadn't happened to list, and on a miss Cursor falls back to running that
   sub-invocation inline — the exact failure mode the rule exists to prevent. The `agents:`
   block was only ever meant to carry model/effort **overrides** (ADR-[[0016]]), not to decide
   which agents exist on disk.

## Decision

1. **Always-full-set per-repo generation.** `sync-agents.sh`'s per-repo pass now iterates the
   full built-in agent set (mirroring the user-level pass), resolving each agent's model/effort
   through the existing ADR-0016 harness-first fallback. The `agents:` block becomes
   **override-only**; a listed-but-nonexistent agent key is a typo warning, not a generation
   filter. Every targeted harness therefore gets every agent in every composition layer, so a
   dispatch rule's targets resolve **by construction** — no separate roster to keep in sync.
2. **A Cursor dispatch rule, assembled deterministically.** An authored `cursor-rules/` source
   (a static `dispatch.head.md` preamble plus one `dispatch/docket-<name>.md` fragment per
   agent) is assembled into `docket-dispatch.mdc` — head + one subsection per built-in agent in
   glob order (a fragment-less agent still gets a minimal auto-generated block plus a
   warning). It is written user-level (`~/.cursor/rules/`) whenever `~/.cursor/` is present,
   and per-repo (`<repo>/.cursor/rules/`) whenever `cursor` is in `agent_harnesses`, and its
   assembly is joined to the existing `sync-agents.sh --check` drift gate. Cursor is the only
   harness that gets a dispatch rule — Claude Code routes model/effort natively and needs none.
3. **Per-repo generation stays opt-in.** A repo opts into committed per-repo wrappers by
   declaring an `agents:` block OR a top-level `agent_harnesses:` key. A `.docket.yml` present
   for change-tracking only (neither key set) gets no per-repo wrappers, and its `--check`
   stays a no-op. This preserves backward compatibility for tracking-only repos (including
   docket's own repo) across the flip from listed-only to always-full-set: gating solely on
   `.docket.yml` presence would have newly littered untracked wrapper files into every such
   repo and flipped its `--check` from a no-op to failing.
4. **Prune orphaned docket-owned files.** After generation, `sync-agents.sh` deletes orphaned
   `docket-*` files: a built-in agent docket no longer ships, or a harness de-listed from
   `agent_harnesses` (including that harness's dispatch rule). Pruning is strictly scoped to
   `docket-*` names — it never touches a non-docket file — and only `rmdir`s a directory
   docket itself just emptied. `--check` reports orphans without deleting them.

## Consequences

- Cursor dispatch targets resolve by construction: the full composed agent set is generated
  wherever the dispatch rule is, so no agent invocation can miss its subagent and fall back to
  running inline at the wrong model.
- ADR-[[0015]]'s harness-neutral, unvalidated `model:` passthrough is unchanged — no
  validation is added here; this ADR refines *how many* files get generated and *how* Cursor
  is made to honor its pin, not the value semantics ADR-0015/0016 already settled.
- The `agents:` block's role narrows further to overrides-only — it can no longer be read as
  "the set of agents that exist" for a repo, only "the set of agents whose default model/effort
  is overridden."
- Opt-in (via `agents:` or `agent_harnesses:`) keeps the always-full-set flip backward-compatible
  for tracking-only adopters; a repo that wants nothing generated need not set either key.
- Determinism is preserved end to end: the rule assembles in glob order, and both generation
  and `--check` re-assemble it identically, so drift detection stays exact.
- Cost: a Cursor repo must explicitly list `cursor` in `agent_harnesses` to get both the
  committed per-repo agent set and the dispatch rule — nothing is auto-detected from a stray
  `.cursor/` directory (consistent with ADR-0015's explicit-over-auto-detect stance).
