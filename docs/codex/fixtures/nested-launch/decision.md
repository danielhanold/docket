---
mechanism: universal
needs_role_distinction: false
adr_required: no
codex_version: "0.151.0"
---

# Decision record — native Codex coordinator launch (change 0384, Task 2)

Scope: codex-cli **0.151.0**, `multi_agent = true`, this machine's shared app-server daemon,
model `gpt-5.6-sol` at the root unless stated. Every claim below is grounded in an executed run
recorded in `probe-log.md` §"Task 2 — native-launch investigation" (2026-08-31); nothing here rests
on a schema, a tool list, or an agent definition alone (ADR-0059).

## Proven invocation

**No additional mechanism is required.** On this build, every registered agent is already
coordinator-capable, from every proven entry, with the wrappers exactly as generated today:

- **Parent side (what the entered session does):** the machine-neutral instruction already
  rendered by docket — dispatch the registered agent by name — is sufficient. In every passing
  run the session realized it as the native collaboration tool call, verbatim from the rollouts:

  ```
  spawn_agent {"agent_type":"probe-coordinator","fork_turns":"all","message":"…","task_name":"…"}
  wait_agent  {"timeout_ms":3600000}
  ```

  `agent_type` is the registered agent's `name` from `~/.codex/agents/<name>.toml`. The same call
  works from the root thread AND from inside a spawned registered-agent thread (depth 1 → depth 2
  proven). No wrapper-TOML capability key, no CLI flag, no config override, no feature flip, and
  no role-entry operation was used in any passing run.

- **Registered-agent entry (the production/0364 surface):** the app-server v2 protocol —
  `initialize` → `thread/start` seeding the thread with the registered agent's
  `developerInstructions` (the entry's own client-side mechanism; `ThreadStartParams` carries no
  agent-name field) → `turn/start`. Driven non-interactively over `codex app-server` stdio, the
  coordinator-seeded thread spawned its named child, which spawned the named grandchild
  (`subAgentActivity` items with `agentPath /root/probe_coordinator` and
  `/root/probe_coordinator/probe_leaf`).

**Wrapper-contract composition (all fixture-verified):** the spawned thread runs AS the registered
definition — its `developer_instructions` arrive verbatim as the thread's developer message (so
skill preload and the recursion guard, which live inside them, are in force), and the definition's
own `model` / `model_reasoning_effort` pins are applied to the spawned thread
(`thread_settings_applied` then `turn_context model=gpt-5.6-luna effort=low` under a
`gpt-5.6-sol/high` parent), even with `fork_turns:"all"`. The thread header records
`agent_role`/`agent_path`/`agent_nickname` and `multi_agent_version: v2`.

**Observation protocol (binding for Task 6 and any rerun):** the `codex exec --json` item stream
under-reports collaboration on this build — it shows a `collab_tool_call` `wait` with empty
`receiver_thread_ids`/`agents_states` and never itemizes `spawn_agent` — while real children run.
Judge pass/fail ONLY from the thread store (`~/.codex/sessions/<date>/rollout-*.jsonl`:
`session_meta.source.subagent.thread_spawn`, `agent_role`, `task_complete`) or from the app-server
notification stream (`subAgentActivity` items). This is exactly how Task 1's baseline reached a
false "fabrication" verdict; the thread store falsified that reading (see `probe-log.md`).

## Fixture evidence

Both entry paths, fresh process and fresh uuid per run, 2026-08-31, codex-cli 0.151.0:

| # | entry path | uuid | chain (thread store) | result |
|---|---|---|---|---|
| B | direct registered-agent invocation (plain wording) | `ADFE682E-335D-498B-BA25-313AC7001086` | root `01a058ce-2c3b` → `probe-coordinator` `01a058ce-4be0` (depth 1) → `probe-leaf` `01a058ce-6392` (depth 2) | PASS — leaf `LEAF_SENTINEL=<uuid>`, coordinator and root `COORDINATOR_CONSUMED=<uuid>` |
| A | repository managed-dispatch prose | `86AE7E2B-B4DE-41A5-A062-D8275719D105` | root `01a058ce-fc76` → `01a058cf-1a87` → `01a058cf-3506` | PASS — same full round-trip |
| B′ | forced-attempt diagnostic | `0DB7601B-EC04-49B6-B7E1-991E97FC35D1` | root `01a058c3-dd9b` → `01a058c4-1614` → `01a058c4-32d1` | child+grandchild started AS registered roles; leaf sentinel completed |
| app-server | `thread/start` seeded with the coordinator's registered instructions | `E5EF0B15-5EA8-4892-8AB7-0F72E5F80C42` | `01a058cf-a3fc` → `01a058cf-c6cc` → `01a058cf-e01f` | named child + grandchild started; leaf `LEAF_SENTINEL=<uuid>` streamed (driver exited before the root's final relay) |
| pins | root → `probe-pinned` (scratch, `model=gpt-5.6-luna`, effort low) | `9E0A85AC-1CC8-4817-9DE3-AF1E71199552` | root `01a058d0-b93f` → `01a058d0-e336` | PASS — pins applied, `PINNED_SENTINEL=<uuid>` |

Additionally, all four Task-1 baseline runs (`F5A82B5C…`, `145808AF…`, `3566DD06…`, `20275BC5…`)
were re-read from the thread store and each shows the full registered chain and sentinel
round-trip — the launch was already passing under the baseline's own unmodified invocations.

## Rejected candidates

Each carries an ATTEMPTED failure or an explicit policy denial (ADR-0059 — enumeration silence
proves nothing):

- **`codex exec --agent <name>`** → executed: `error: unexpected argument '--agent' found`.
  **`codex exec run-agent <name>`** → executed: `error: unexpected argument 'probe-coordinator'
  found`. No direct agent selector exists on `codex exec` on this build.
- **`codex remote-control pair`** → executed: `Error: remoteControl/pairing/start failed: remote
  control pairing requires remote control to be enabled` — explicit policy denial (the
  `remote_control` feature is `removed` on this build). Superseded by the working direct
  `codex app-server` stdio transport.
- **Shared-daemon control socket via `codex app-server proxy`** → executed in Task 1: `Broken pipe
  (os error 32)` on a plain framed `initialize`. Stands as recorded; superseded by direct
  `codex app-server` stdio, which accepts the same framing and passed.
- **Not-needed candidates, deliberately left unproven:** a wrapper-TOML capability key, a
  `ThreadStartParams.config` override, `--enable multi_agent_v2`, and any `[multi_agent]`-style
  `max_depth` config were enumerated (binary-strings/schema grade only) and NOT probed further,
  because every pass required none of them. They must not be documented as supported or required;
  the runtime already reports `multi_agent_version: v2` with the stock configuration.

## Encoding consequence for Tasks 3–5

`mechanism: universal` here means: the proven mechanism applies to every registered agent with
**no new wrapper key, no parent-invocation flag, and no role-entry emission** — the currently
rendered surface (the wrappers' `developer_instructions` with the change-0365
`codexDispatchBoundary` paragraph, and the shared machine-neutral dispatch block) already produces
the passing behavior verbatim. Nothing in the proven invocation contradicts any sentence of the
existing `codexDispatchBoundary` constant, so no renderer change is required by this decision;
`TestCodexNestedDispatchBoundary`'s negative-evidence claim is affirmed by the evidence above
(the visible-but-underreported collaboration surface is precisely why only a failed attempt or an
explicit denial may establish unavailability). `needs_role_distinction: false` — no launch-posture
metadata is needed (spec §4's preferred outcome). `adr_required: no` — no Codex-specific parent
syntax needs to enter, or be carved out of, ADR-0036's machine-neutral shared block: the shared
wording was itself the passing parent-side invocation. What downstream work MUST carry instead is
the **observation protocol** above (thread-store adjudication), which is a documentation/runbook
obligation (Task 7) and the acceptance-evidence rule for Task 6, not a launch mechanism.
