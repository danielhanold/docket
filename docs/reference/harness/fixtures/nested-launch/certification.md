---
status: certified
codex_version: "0.151.0"
entry_path_a: PASS
entry_path_b: PASS
composition_edge: PASS
mutation_oracle: reddens-under-wrong-oracle
mutation_0365_guard: reddens-and-restores
date: 2026-08-31
---

# Live certification record — change 0384 (Task 6)

This is the completed certification section the change's results record must carry forward
(spec §6). Every entry below is either **COMPLETED** with thread-store evidence or explicitly
**NOT-RUN** with its reason; nothing here is an unchecked checklist presented as done. Full run
detail, verbatim invocations, and rollout ids live in `probe-log.md` §"Task 6 — live
fresh-process certification runs (2026-08-31)"; the routing authority is `decision.md`
(`mechanism: universal`).

## Scope

- **Codex version:** `codex --version` → `codex-cli 0.151.0` (banner `OpenAI Codex v0.151.0`).
- **Setting:** `multi_agent stable true` (`multi_agent_v2 stable false`); session banner
  `model: gpt-5.6-sol · approval: never · sandbox: danger-full-access · reasoning effort: high`.
- **Install:** branch installed via
  `docket development install --source /Users/homer/dev/docket/.worktrees/launch-compositional-docket-agents-in-coordinator-capable-ha`;
  the 17 regenerated `docket-*.toml` wrappers are **byte-identical** to the previously installed
  set (`diff -r` clean) — the branch changes no renderer output, per `decision.md`.
  `docket-plan-writer.toml` on disk carries the change-0365 `codexDispatchBoundary` paragraph.
- **Adjudication:** thread store only (`~/.codex/sessions/2026/08/31/rollout-*.jsonl` —
  `session_meta.source.subagent.thread_spawn`, `agent_role`, `agent_path`, `task_complete`),
  per the binding observation protocol in `decision.md`. The `codex exec --json` item stream is
  never used as a pass/fail oracle. Claims are scoped to this version and configuration.

## Selected launch shape

**Native `spawn_agent` by registered name, with no renderer change.** The parent-side invocation
is the machine-neutral dispatch instruction already rendered by docket; every passing run
realized it as the native collaboration call
`spawn_agent {"agent_type":"<registered name>", …}` + `wait_agent`, and the spawned thread runs
AS the registered definition (developer_instructions verbatim, own model/effort pins). No
wrapper key, CLI flag, config override, or role-entry operation was used.

## Certification matrix

| entry | status | evidence (thread store) |
|---|---|---|
| **B — direct registered-agent invocation** (fresh process, uuid `055FE80D-958D-4646-BD56-D75A7D7B8C80`) | **COMPLETED — PASS** | root `01a058d8-d0f5` called `spawn_agent {"agent_type":"probe-coordinator","fork_turns":"all",…}` → child `01a058d8-eca9` (`probe-coordinator`, depth 1) → grandchild `01a058d9-05e6` (`probe-leaf`, depth 2); grandchild `task_complete` **`LEAF_SENTINEL=055FE80D-958D-4646-BD56-D75A7D7B8C80`** (child sentinel); child `task_complete` **`COORDINATOR_CONSUMED=055FE80D-958D-4646-BD56-D75A7D7B8C80`** (coordinator sentinel); root final line identical |
| **A — repository managed-dispatch prose** (fresh process, uuid `4D5355CD-D547-4B0B-91F6-725570BC783D`) | **COMPLETED — PASS** | root `01a058d9-f9d5` → child `01a058da-2224` (`probe-coordinator`, depth 1) → grandchild `01a058da-3c22` (`probe-leaf`, depth 2); `LEAF_SENTINEL=4D5355CD-…` and `COORDINATOR_CONSUMED=4D5355CD-…` both `task_complete`; root relayed the coordinator line |
| **App-server registered-agent entry** (`thread/start` seeded with registered instructions) | **NOT-RUN in Task 6** — already proven in Task 2 (uuid `E5EF0B15-…`, chain root (id not captured) → child `01a058cf-c6cc` → grandchild `01a058cf-e01f`, leaf sentinel streamed); Task 2 noted the non-interactive driver exits before the root's final relay, an external limitation of the scratch driver, not of the launch. Task 6's required matrix is entry paths A and B, both COMPLETED above. | see `probe-log.md` §Task 2, run 4 |
| **Real Docket composition edge** (scratch docket-initialized repo; prose "refresh the docket board" → registered `docket-status`) | **COMPLETED — PASS** | root `01a058db-fa0d` spawned child `01a058dc-282f` (`agent_role docket-status`, `agent_path /root/docket_status`, depth 1); child ran the genuine workflow (`docket maintenance sweep --json`, `docket status`) in the scratch repo and its `task_complete` status report was consumed and relayed verbatim by the root — spec §5's bar met. The live `/Users/homer/dev/docket/.docket` backlog was never touched. |

## Failed-current / fixed-new comparison (with the wrong-oracle / right-oracle mutation)

The Task-1 "failed-current" baseline read the `codex exec --json` item stream and concluded no
child ever started ("fabricated" sentinels). Task 2 established — and Task 6 reproduced as its
**oracle mutation** — that this signal is an artifact of the observation surface, not of the
launch:

- **Wrong oracle, same run** (path B, uuid `055FE80D-…`, run under `--json`): the item stream
  contains **zero** `spawn_agent` items and only `collab_tool_call` `wait` records with
  `"receiver_thread_ids":[]` / `"agents_states":{}` — the exact false-negative that produced the
  baseline's "fabrication" verdict and change 0364's halt.
- **Right oracle, same run**: three real threads with registered `agent_role`s and both
  `task_complete` sentinels.

So the failed-current/fixed-new delta is **the observation protocol, not a launch-shape change**:
the launch was already passing under the unmodified invocations (Task 2 re-read all four
baseline runs from the thread store and each shows the full chain), and what the fix delivers is
the certified, thread-store-adjudicated protocol plus the retained 0365 boundary emission.

## Mutation check — change-0365 retention guard

There is no Task-3 launch-shape emission to revert (`mechanism: universal`), so the renderer-side
mutation is the retention guard: deleting the `codexDispatchBoundary` term from `renderAgent` in
`internal/harness/codex/codex.go` turned
`go test -count=1 ./internal/harness/codex/ -run TestCodexNestedDispatchBoundary` **red** with
per-agent missing-clause findings derived from the real inventory; restoring the file from a
backup copy turned it **green** again (`git diff` clean). **COMPLETED — reddens for the guarded
thing.**

## Change 0364

**Change 0364 is NOT resumed on this branch.** Per the spec (§6) and the plan, resuming it
through its `resume-halted` contract is the post-merge production confirmation, performed only
after the fix is merged and installed from `main`; it supplements this fixture certification and
does not replace it.

## Standing rule

Change 0384 must **not** be finalized `done` without this successful nested sentinel evidence —
a real child and grandchild thread in the thread store consuming the run's own freshly minted
uuid. The change's results file must carry this certification section forward. A review PR may
be opened with any external limitation clearly reported (spec §6); the only such limitation here
is the app-server scratch driver's early exit, recorded above as NOT-RUN-in-Task-6 with its
Task-2 evidence.

## Teardown

Probe TOMLs removed (`rm -f ~/.codex/agents/probe-*.toml`); scratch repos deleted; the 17
`docket-*.toml` wrappers verified present and byte-identical to the pre-session snapshot.
