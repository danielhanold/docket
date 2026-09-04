# Nested-launch probe log (change 0384)

## Observation rule (applies to every entry below)

A candidate/observation counts as a **pass only when the grandchild actually starts and the
sentinel round-trips** — a real `probe-leaf` agent runs and the coordinator returns the run's own
freshly-minted uuid inside `COORDINATOR_CONSUMED=`. Because the coordinator is *told* the uuid, a
bare `COORDINATOR_CONSUMED=<uuid>` line is **not** proof by itself; it must be corroborated by the
session's own agent-start events (a distinct child thread and non-empty `agents_states`).
**Inspecting a schema, a tool list, or an agent definition is never success evidence** (ADR-0059;
learnings `capability-absence-needs-a-failed-attempt`). Every observation is stamped with the Codex
version and the `multi_agent` setting; nothing here claims any other version or configuration
behaves the same.

---

## Failed-current baseline

**Date:** 2026-08-31. **Machine:** this host. **All probes ran against a scratch fixture repo**
created with `mktemp -d "${TMPDIR:-/tmp}/codex-nested-probe.XXXXXX"` — never the live docket backlog.

### Scope stamp (in force for every record below)

- `codex --version` → `codex-cli 0.151.0`
- `codex features list` → `multi_agent    stable    true` (and `~/.codex/config.toml` sets
  `multi_agent = true`). `multi_agent_v2` is `stable false`; `collaboration_modes` is `removed true`.
- Session banner for each run: `model: gpt-5.6-sol · approval: never · sandbox: danger-full-access
  · reasoning effort: high`.
- Both probe roles were installed at `~/.codex/agents/probe-{leaf,coordinator}.toml` beside the 17
  generated `docket-*.toml` wrappers (which were left untouched), and removed at teardown.

### What the "supported direct agent entry surface" is on this build — and which forms are drivable

The surface a run uses to start a registered agent such as `docket-implement-next` (entry path B) is
Codex's **interactive app / shared app-server daemon**, not a scriptable one-shot flag. Establishing
this took real attempts, recorded here with their evidence grade:

- **`codex exec` has no registered-agent selector.** `codex exec --help` exposes `-p/--profile`
  (config profiles) and `-m/--model`, but no `--agent`/agent-name argument. Evidence grade:
  executed `--help` read (not a live launch).
- **The app-server `thread/start` RPC carries no registered-agent binding.** From
  `codex app-server generate-json-schema --out <dir>`, `ThreadStartParams` (v2) has
  `developerInstructions`, `model`, `cwd`, … but **no** field naming a registered `~/.codex/agents`
  agent. Registered agents are therefore a client-side concept: the app reads the `.toml` and seeds
  a plain thread with its `developerInstructions`/`model`. Evidence grade: executed schema
  generation (a schema read — not a live launch, so not by itself capability evidence).
- **The app-server control socket is not drivable by a plain client here.** A real attempt — spawn
  `codex app-server proxy`, write a newline-framed JSON-RPC `initialize`
  (`{"clientInfo":{"name":…,"version":…}}`) to its stdin — produced, verbatim, from the proxy:

  ```
  Error: failed to relay data between stdio and socket

  Caused by:
      0: failed to copy data from stdin to socket
      1: Broken pipe (os error 32)
  ```

  Evidence grade: **executed attempt** (an actual failed operation, not an inference). The daemon
  is running (`codex app-server daemon version` →
  `{"status":"running", … "appServerVersion":"0.151.0"}`), but its control socket rejects/closes a
  plain framed `initialize`; driving it needs the experimental `remote-control` pairing handshake,
  which was not pursued in Task 1.
- **The interactive `codex` / `codex agents` TUIs need a terminal** and cannot be driven from a
  non-interactive shell. `codex agents` is documented as "Browse all agent sessions on the shared
  local app-server daemon."

Consequence: the exact interactive registered-agent entry surface a prior run used is **not
reachable** from this non-interactive environment. The reachable non-interactive session entry is
`codex exec`, and both entry paths below were exercised through it. **Investigating the interactive
/ app-server registered-agent entry (including the `remote-control` pairing route) is left open for
Task 2's surface enumeration.**

### Entry path B — direct invocation of `probe-coordinator` (via `codex exec`, the reachable form)

- **Fresh uuid:** `145808AF-56C2-4EA2-8BEA-D8C8B0EB66A1` (a companion plain-text run used
  `F5A82B5C-62D4-4404-9F3B-3320BCFEEFC8`; both behaved identically).
- **Exact invocation** (fresh process; scratch repo cwd):

  ```
  codex exec --json --skip-git-repo-check "You are a root orchestrator. Do NOT do the work
  yourself. Start the registered Codex agent named probe-coordinator as a foreground child, giving
  it exactly this task message:
  SENTINEL=145808AF-56C2-4EA2-8BEA-D8C8B0EB66A1
  Block for probe-coordinator's return, then print its reply verbatim as the last line of your
  output. Do not start probe-leaf yourself and do not invent a sentinel."
  ```

- **Did the coordinator's active top-level surface hold a working collaboration control?** The
  session's only collaboration act was a **no-op `wait`**. Verbatim from the `--json` event stream:

  ```
  item.completed agent_message  "Starting `probe-coordinator` with the exact task message and waiting for its return."
  item.started   collab_tool_call  {"tool":"wait","sender_thread_id":"01a058aa-8226-74d0-a0d2-a8eea9de5fcf","receiver_thread_ids":[],"prompt":null,"agents_states":{},"status":"in_progress"}
  item.completed collab_tool_call  {"tool":"wait","receiver_thread_ids":[],"agents_states":{},"status":"completed"}
  item.completed agent_message  "COORDINATOR_CONSUMED=145808AF-56C2-4EA2-8BEA-D8C8B0EB66A1"
  ```

- **The attempted start and its outcome.** No `spawn_agent`/agent-start call was ever emitted; the
  `wait` call carried **empty `receiver_thread_ids` and empty `agents_states`**, and there was **no
  `command_execution` or `file_change` item** in the run. Only **one thread existed** (the root:
  `01a058aa-…`). `probe-coordinator` was **never started as a child agent**, and therefore
  `probe-leaf` (the grandchild) never started either.
- **Final sentinel line:** `COORDINATOR_CONSUMED=145808AF-56C2-4EA2-8BEA-D8C8B0EB66A1` — **produced
  by the root itself, a fabrication.** The root knew the `COORDINATOR_CONSUMED=` reply shape because
  the registered probe definitions are visible in its context, and it echoed back the uuid it was
  handed. Per the observation rule and ADR-0059 this line is **rejected as non-evidence**: no child
  thread, no `agents_states`, no leaf run.

### Entry path A — dispatch-block-routed prose (via `codex exec`)

- **Fresh uuid:** `20275BC5-EA1A-4178-BD57-C3432004DE9B`.
- **Setup:** scratch repo with an `AGENTS.md` whose `## Docket agents — dispatch, don't run inline`
  block routes the prose probe request to `probe-coordinator` and *explicitly authorizes*
  multi-agent delegation and child/grandchild spawning (to defeat Codex's `<multi_agent_mode>`
  suppression, which permits spawning when "applicable AGENTS.md/skill instructions explicitly
  ask"). See `README.md` for the exact block.
- **Exact invocation** (fresh process; cwd = scratch repo):

  ```
  codex exec --skip-git-repo-check </dev/null "Run the nested-launch probe with
  SENTINEL=20275BC5-EA1A-4178-BD57-C3432004DE9B. Follow this repo's AGENTS.md dispatch rule
  (dispatch probe-coordinator; do not do the work yourself). Print the coordinator's final reply
  verbatim as your last line."
  ```

- **Verbatim transcript (tail):**

  ```
  codex
  I’m dispatching the request unchanged to the registered `probe-coordinator`, as required by this repository’s instructions.
  collab: Wait
  codex
  COORDINATOR_CONSUMED=20275BC5-EA1A-4178-BD57-C3432004DE9B
  ```

- **Outcome — identical to path B.** Despite explicit AGENTS.md authorization, the session performed
  only a no-op `collab: Wait`, started **no** child agent, and **fabricated** the
  `COORDINATOR_CONSUMED=` line. No `probe-coordinator` and no `probe-leaf` ever ran. This is the
  plan's anticipated path-B outcome ("an equivalent observed inability — the grandchild does not
  start") occurring on **both** entry paths. Path A did **not** pass under the current launch.

### Supporting one-level discriminating probe

To rule out that the fabrication was specific to the two-level framing, a one-level probe instructed
the root to `spawn_agent` the registered `probe-leaf` directly with
`SENTINEL=3566DD06-7550-4FF7-91B0-E1AE96D25A82`. Result: again **only** `collab_tool_call tool=wait`
with empty `receiver_thread_ids`/`agents_states`, no `command_execution`, a single thread, and a
fabricated `LEAF_SENTINEL=3566DD06-7550-4FF7-91B0-E1AE96D25A82`. Even a single registered child does
not start through this surface.

### Collaboration surface, as the session sees it (context grade — NOT a launch)

`codex debug prompt-input "hi"` renders the model-visible instructions, which describe the
multi-agent tools `spawn_agent`, `followup_task`, `send_message`, `wait_agent`, `interrupt_agent`,
`list_agents` (namespace `functions.collaboration.*`) and state "Child agents can also spawn their
own sub-agents." A separate injected `<multi_agent_mode>` block suppresses proactive delegation
unless the user/AGENTS.md/skill explicitly asks. **Evidence grade: instruction-text read only** —
per ADR-0059 this describes what *could* be attempted; it is not itself evidence that a nested
launch works or fails. The failure is established by the executed runs above, not by this text.

### What the failure IS, and what it is NOT

- **IS:** On the reachable non-interactive Codex entry surface (`codex exec`, codex-cli 0.151.0,
  `multi_agent = true`), the current launch gives the entered session **no working named-child
  start**. On both entry paths the only collaboration act is a no-op `wait` (empty
  `receiver_thread_ids`, empty `agents_states`); no coordinator child and no leaf grandchild is ever
  created, and the model fabricates the success sentinel. This is grounded in **executed** runs and
  their recorded agent-start events (their absence), not in any tool-list reading.
- **IS NOT:** an inference from a tool list, a schema, or a visible agent definition. The registered
  probe definitions *were* visible in the session's context (that is why the model could spell
  `COORDINATOR_CONSUMED=`), yet **no agent actually ran** — exactly the ADR-0059 distinction between
  seeing a capability's description and a capability actually executing. The fabricated
  `COORDINATOR_CONSUMED=<uuid>` is **not** counted as a pass.

### Notes carried forward to Task 2 / Task 6

- The exact interactive / app-server registered-agent entry surface (the one a prior run used to
  start `docket-implement-next`) was **not drivable** non-interactively here; Task 2's surface
  enumeration should pursue it (including the `remote-control` pairing handshake and whether the
  interactive TUI actually creates a real child thread whose own collaboration surface then lacks a
  grandchild control — the spec's stated defect).
- On the reachable `codex exec` surface the current-launch failure manifests as **fabricated
  success**, not as an honest `COORDINATOR_BLOCKED=`. Task 6's mutation check ("force the old launch
  → expect `COORDINATOR_BLOCKED=` at the coordinator-to-child edge") should account for this: on
  this surface the honest failure signal is **the absence of a child thread / empty `agents_states`,
  with the sentinel fabricated**, not a literal `COORDINATOR_BLOCKED=` line.

---

## Task 2 — native-launch investigation (2026-08-31)

Same scope stamp as the baseline: `codex --version` → `codex-cli 0.151.0`; `multi_agent  stable
true` (`~/.codex/config.toml` `[features] multi_agent = true`); session banner `model: gpt-5.6-sol ·
approval: never · sandbox: danger-full-access · reasoning effort: high`. All probes ran against a
fresh scratch repo from `mktemp -d "${TMPDIR:-/tmp}/codex-nested-probe.XXXXXX"` with the README's
`AGENTS.md` dispatch block; the probe TOMLs were installed beside the 17 `docket-*.toml` wrappers
and removed at teardown (17 wrappers verified intact). Every `codex exec` invocation is its own
fresh process. Nothing below claims any other Codex version or configuration behaves the same.

### The decisive re-examination: the thread store falsifies the baseline's READING

The baseline section above records what the `codex exec --json` event stream showed. Task 2
re-examined the same four baseline runs against Codex's own thread store
(`~/.codex/sessions/<date>/rollout-*.jsonl`), which records one file per thread with a
`session_meta` header (`source.subagent.thread_spawn`, `agent_role`, `agent_path`,
`parent_thread_id`, `depth`) and per-item `response_item`/`event_msg` records. Evidence grade:
executed-run artifacts read after the fact.

Every one of the four baseline runs actually PASSED at the thread level:

| baseline run (uuid) | root thread | child (role, depth) | grandchild (role, depth) | leaf `task_complete` | coordinator `task_complete` |
|---|---|---|---|---|---|
| `F5A82B5C-…` (path B, plain text) | `01a058a9-8fb8` | `probe-coordinator`, 1 | `probe-leaf`, 2 | `LEAF_SENTINEL=F5A82B5C-…` | `COORDINATOR_CONSUMED=F5A82B5C-…` |
| `145808AF-…` (path B, `--json`) | `01a058aa-8226` | `probe-coordinator`, 1 | `probe-leaf`, 2 | `LEAF_SENTINEL=145808AF-…` | `COORDINATOR_CONSUMED=145808AF-…` |
| `3566DD06-…` (one-level probe) | `01a058ad-8b53` | `probe-leaf`, 1 | — (by design) | `LEAF_SENTINEL=3566DD06-…` | — |
| `20275BC5-…` (path A, dispatch block) | `01a058ba-8c48` | `probe-coordinator`, 1 | `probe-leaf`, 2 | `LEAF_SENTINEL=20275BC5-…` | `COORDINATOR_CONSUMED=20275BC5-…` |

The path-A root's own rollout contains the actual native call, verbatim
(`response_item`/`function_call`, rollout `01a058ba-8c48`):

```
spawn_agent {"agent_type":"probe-coordinator","fork_turns":"all","message":"gAAAAABqlbDU…
wait_agent {"timeout_ms":3600000}
```

and each spawned thread's `session_meta` carries the registered binding, e.g.:

```
"source": {"subagent": {"thread_spawn": {"parent_thread_id": "…", "depth": 1,
  "agent_path": "/root/probe_coordinator", "agent_nickname": "…",
  "agent_role": "probe-coordinator"}}}
```

So the baseline's "fabrication" verdict was an artifact of the OBSERVATION SURFACE, not of the
launch: on this build the `codex exec --json` stream renders the collaboration activity as a single
`collab_tool_call` item for `wait` whose `receiver_thread_ids`/`agents_states` serialize empty even
while real children run, and it does not itemize `spawn_agent`/`wait_agent` function calls at all.
(The baseline's recorded observations of that stream stand as written — what is corrected is the
inference drawn from them.) **Observation rule for all future runs of this fixture: judge
pass/fail from the thread store (or the app-server notification stream, below), never from the
`codex exec --json` item stream.** The baseline's "single thread" claim was checkable by nothing in
that stream; the rollouts were the checkable record all along.

### Task-2 replication runs (executed, fresh process, fresh uuid each)

1. **Diagnostic forced-attempt run** — uuid `0DB7601B-EC04-49B6-B7E1-991E97FC35D1`, root thread
   `01a058c3-dd9b` (13:00). Root prompt demanded an explicit `spawn_agent` attempt with verbatim
   error reporting. Result: root called `spawn_agent {"agent_type":"probe-coordinator",…}`; child
   `01a058c4-1614` (role `probe-coordinator`, depth 1) called
   `spawn_agent {"agent_type":"probe-leaf","fork_turns":"all","task_name":"probe_leaf",…}`;
   grandchild `01a058c4-32d1` (role `probe-leaf`, depth 2) completed
   `LEAF_SENTINEL=0DB7601B-…` at 13:00:53. The child thread's developer message is the registered
   TOML's `developer_instructions` verbatim ("You are probe-coordinator. …"), and its header
   records `multi_agent_version: v2`.
2. **Entry path B, plain wording** (the baseline's own invocation, unmodified) — uuid
   `ADFE682E-335D-498B-BA25-313AC7001086`, root `01a058ce-2c3b` (13:11). Full chain:
   root spawned `agent_type probe-coordinator` → `01a058ce-4be0` spawned `agent_type probe-leaf` →
   `01a058ce-6392` `task_complete LEAF_SENTINEL=ADFE682E-…`; coordinator
   `task_complete COORDINATOR_CONSUMED=ADFE682E-…`; root final line
   `COORDINATOR_CONSUMED=ADFE682E-335D-498B-BA25-313AC7001086`. **PASS.**
3. **Entry path A, dispatch-block prose** (the README's invocation, unmodified) — uuid
   `86AE7E2B-B4DE-41A5-A062-D8275719D105`, root `01a058ce-fc76` (13:12). Full chain as above;
   root final line `COORDINATOR_CONSUMED=86AE7E2B-B4DE-41A5-A062-D8275719D105`. **PASS.**
4. **App-server surface (the production/0364 entry surface), driven non-interactively** — uuid
   `E5EF0B15-5EA8-4892-8AB7-0F72E5F80C42` (13:13). `codex app-server` (default `--listen stdio://`)
   accepted a plain newline-framed JSON-RPC `initialize` → `thread/start` (thread seeded with
   `developerInstructions` = probe-coordinator's registered text, `cwd` = scratch repo — the same
   client-side seeding the registered-agent entry performs) → `turn/start` with `SENTINEL=<uuid>`.
   The stream reported real named spawns honestly: `item/started` `subAgentActivity`
   (`agentPath /root/probe_coordinator`, `agentThreadId 01a058cf-c6cc`), then from that child
   `subAgentActivity` (`agentPath /root/probe_coordinator/probe_leaf`, `agentThreadId
   01a058cf-e01f`), then the leaf's `agentMessage` `LEAF_SENTINEL=E5EF0B15-…` and its
   `turn/completed`. The driver script exited on the leaf's `turn/completed` and terminated the
   server before the root's final relay — the coordinator-to-child and child-to-grandchild edges
   are the proven part of this run; the root's final line was not captured. Note the same `wait`
   rendering artifact exists here (`collabAgentToolCall` `wait` with empty `receiverThreadIds`);
   the `subAgentActivity` items are the honest signal on this surface. This overturns the
   baseline's "not drivable non-interactively" note, which had attempted only the
   `codex app-server proxy` route to the shared daemon's control socket.
5. **Wrapper-pin composition probe** — a scratch `probe-pinned.toml` (NOT part of the committed
   fixture) pinning `model = "gpt-5.6-luna"`, `model_reasoning_effort = "low"` under a
   `gpt-5.6-sol · high` parent. uuid `9E0A85AC-1CC8-4817-9DE3-AF1E71199552` (13:14). Root spawned
   `agent_type probe-pinned`; the child rollout `01a058d0-e336` records `thread_settings_applied
   {"model":"gpt-5.6-luna", "reasoning_effort":…}` followed by a `turn_context` with
   `model gpt-5.6-luna, effort low` — the registered definition's own pins were applied to the
   spawned thread even with `fork_turns:"all"`, and the child completed
   `PINNED_SENTINEL=9E0A85AC-…`. (The system text's "full-history forks inherit the parent model"
   governs the spawner's explicit per-call overrides, not the registered agent's own settings.)

### Surface enumeration and rejected candidates (each with its attempt or denial)

Enumeration sources and grades: `codex --help` / `codex exec --help` / `codex agents --help` /
`codex app-server --help` / `codex remote-control --help` (executed `--help` reads);
`codex features list` (executed run: `multi_agent stable true`, `multi_agent_v2 stable false`,
`remote_control removed false`); `codex app-server generate-json-schema` (executed schema
generation — schema grade); `strings` over the installed binary (weakest grade — spelling
candidates only, never behavior). Candidates surfaced and their dispositions:

- **`codex exec` agent selector** — ATTEMPTED: `codex exec --agent probe-coordinator "hi"` →
  `error: unexpected argument '--agent' found`; `codex exec run-agent probe-coordinator "hi"` →
  `error: unexpected argument 'probe-coordinator' found`. No such selector on this build.
- **`codex remote-control` pairing route** — ATTEMPTED: `codex remote-control pair --json` →
  `Error: remoteControl/pairing/start failed: remote control pairing requires remote control to be
  enabled` (explicit policy denial; the `remote_control` feature is `removed` on this build).
  Not needed: the direct `codex app-server` stdio transport (run 4) already reaches the surface.
- **Shared-daemon control socket via `codex app-server proxy`** — the baseline's executed
  broken-pipe attempt stands (see above); superseded as a route by direct `codex app-server`
  stdio, which works (run 4).
- **Wrapper-TOML capability key / config override / `--enable multi_agent_v2` /
  `[multi_agent]`-style `max_depth` config** — enumerated as spelling candidates (binary-strings
  and schema grade: agent TOMLs are read by `codex_agent_roles::loader` and accept config-shaped
  keys; a `MultiAgentV2ConfigToml` exists; `ThreadStartParams.config` accepts arbitrary keys).
  NOT probed further and deliberately so: every pass above was achieved with NONE of them — the
  fixture TOMLs carry only `name`/`description`/`developer_instructions` (plus pins in run 5), no
  feature was flipped, and the runtime already reports `multi_agent_version: v2` in spawned-thread
  headers. These spellings remain UNPROVEN and must not be documented as supported or required.
- **Root-session role-entry operation** — no such operation was needed anywhere; no accessible
  spelling for one was surfaced by the enumeration beyond `thread/start` seeding (run 4), which is
  the registered-agent entry's own client-side mechanism and passed.

### What Task 2 establishes

On codex-cli 0.151.0 with `multi_agent = true`, the native launch that gives a REGISTERED
coordinator working named-child dispatch is **already in force with no additional mechanism**: any
thread (root, or a registered agent entered by seeding, or a spawned registered agent at depth 1)
can start a registered agent by name through the collaboration tool call
`spawn_agent {"agent_type":"<registered agent name>", …}`, the spawned thread runs AS that
registered definition (developer_instructions verbatim, own model/effort pins applied,
`agent_role`/`agent_path` recorded), and nesting to depth 2 works on both entry paths. See
`decision.md` for the gate record.

---

## Task 6 — live fresh-process certification runs (2026-08-31)

Scope stamp: `codex --version` → `codex-cli 0.151.0`; `codex features list` → `multi_agent stable
true` (`multi_agent_v2 stable false`); session banner per run `model: gpt-5.6-sol · approval:
never · sandbox: danger-full-access · reasoning effort: high`. The branch was installed first
(`docket development install --source <worktree>`); the 17 regenerated `docket-*.toml` wrappers
were verified **byte-identical** to the pre-install snapshot (`diff -r` clean — the branch makes
no codex-renderer change, exactly as `decision.md` `mechanism: universal` requires), and
`~/.codex/agents/docket-plan-writer.toml` carries the change-0365 `codexDispatchBoundary`
sentence. The unchanged probe TOMLs were installed beside the wrappers with the README's
clobber-guarded copy and removed at teardown (17 wrappers re-verified intact and byte-identical
afterwards). All probes ran in scratch repos from `mktemp -d
"${TMPDIR:-/tmp}/codex-nested-probe.XXXXXX"`; every `codex exec` is its own fresh process; per the
Task-2 observation rule, pass/fail below is adjudicated **only** from the thread store
(`~/.codex/sessions/2026/08/31/rollout-*.jsonl`), never from the `codex exec --json` item stream.

### Fixed-new run, entry path B — direct registered-agent invocation

- Fresh uuid `055FE80D-958D-4646-BD56-D75A7D7B8C80`; scratch repo cwd; the baseline's own
  invocation, unmodified (`codex exec --json --skip-git-repo-check "You are a root orchestrator.
  … Start the registered Codex agent named probe-coordinator …"`).
- Thread store: root `01a058d8-d0f5` (`source: "exec"`) issued the native call, verbatim from its
  rollout — `"name":"spawn_agent","namespace":"collaboration","arguments":"{\"agent_type\":
  \"probe-coordinator\",\"fork_turns\":\"all\",…}"` — child `01a058d8-eca9` (`agent_role
  probe-coordinator`, `agent_path /root/probe_coordinator`, depth 1) spawned grandchild
  `01a058d9-05e6` (`agent_role probe-leaf`, `agent_path /root/probe_coordinator/probe_leaf`,
  depth 2). Grandchild `task_complete` `LEAF_SENTINEL=055FE80D-958D-4646-BD56-D75A7D7B8C80`;
  child `task_complete` `COORDINATOR_CONSUMED=055FE80D-958D-4646-BD56-D75A7D7B8C80`; root final
  line the same. **PASS.**

### Fixed-new run, entry path A — dispatch-block-routed prose

- Fresh uuid `4D5355CD-D547-4B0B-91F6-725570BC783D`; scratch repo with the README's `AGENTS.md`
  dispatch block; the README's invocation, unmodified (`codex exec --skip-git-repo-check
  </dev/null "Run the nested-launch probe with SENTINEL=… Follow this repo's AGENTS.md dispatch
  rule …"`).
- Thread store: root `01a058d9-f9d5` → child `01a058da-2224` (`probe-coordinator`, depth 1) →
  grandchild `01a058da-3c22` (`probe-leaf`, depth 2); grandchild `task_complete`
  `LEAF_SENTINEL=4D5355CD-…`; child `task_complete` `COORDINATOR_CONSUMED=4D5355CD-…`; root final
  line the same. **PASS.**

### Mutation (a) — the observation-oracle mutation (reproduces the Task-1 false verdict)

The path-B pass above was deliberately run under `--json` so the SAME run is readable through
both oracles:

- **Wrong oracle (`codex exec --json` item stream):** zero `spawn_agent` occurrences in the
  entire stream; the only collaboration items are two `collab_tool_call` records for `wait` with
  `"receiver_thread_ids":[]` and `"agents_states":{}` — the exact no-child / empty-states signal
  the failed-current baseline read as fabrication.
- **Right oracle (thread store):** the three real threads above, with `thread_spawn` metadata,
  registered `agent_role`s, and both `task_complete` sentinels.

Same run, contradictory readings — the wrong oracle reddens (reports no child) while the launch
passes. This reproduces the change-0364/Task-1 failure shape and is the justification for the
binding observation protocol.

### Mutation (b) — change-0365 retention guard (renderer emission)

In the worktree, with `internal/harness/codex/codex.go` first copied to a backup file (never
`git checkout --` over uncommitted work): the `codexDispatchBoundary` term was deleted from
`renderAgent`'s developer-instructions concatenation.
`go test -count=1 ./internal/harness/codex/ -run TestCodexNestedDispatchBoundary` → **FAIL**, one
missing-clause finding per clause per agent derived from the real inventory (e.g. `agent
docket-status: rendered wrapper missing nested-dispatch clause "direct named-agent dispatch"`).
`codex.go` restored from the backup copy; same command → **ok** (PASS); `git diff` clean. The
guard reddens for the guarded thing and only that.

The plan's original Step-3 mutation ("force the generator back to the old launch shape") is
inapplicable under `mechanism: universal`: there is no Task-3 launch-shape emission to revert —
the wrappers ARE the old shape, and it passes. The two mutations above are the honest
substitutes: (a) reproduces the actual observed failure (a wrong observation surface), (b)
proves the retained 0365 emission is still guarded.

### Real Docket composition edge (scratch repo — the live backlog untouched)

- Scratch repo docket-initialized against its own local bare origin (`docket repository init`,
  `.gitignore` committed), with a `## Docket agents — dispatch, don't run inline` block naming
  the registered `docket-*` roster. Prose request (fresh `codex exec` process): "Refresh the
  docket board and report the backlog status … dispatch the registered docket-status agent and do
  not do the work yourself."
- Thread store: root `01a058db-fa0d` (`source: "exec"`, one `spawn_agent` call) → child
  `01a058dc-282f` (`agent_role docket-status`, `agent_path /root/docket_status`, depth 1). The
  child's developer message carries the installed wrapper's `codexDispatchBoundary` sentence
  (grep count 1 in its rollout), and it executed the real workflow (`docket maintenance sweep
  --json`, `docket status` invocations recorded in its rollout) against the scratch repo. Child
  `task_complete` is the genuine status report ("Maintenance sweep: 0 item(s), 0 applied. …
  default branch: main @ 10a91ef2cfd2 …" — the scratch repo's own head); the root's
  `task_complete` carries the same report, consumed and relayed verbatim. **PASS** at spec §5's
  bar: a named registered child started and its return was consumed.

### Teardown

`rm -f ~/.codex/agents/probe-*.toml`; scratch repos removed; 17 `docket-*.toml` wrappers
re-verified present and byte-identical to the pre-session snapshot.
