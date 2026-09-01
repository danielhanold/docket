<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0393 — Enter compositional Docket roles as coordinator-capable Codex root threads](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0393-enter-compositional-docket-roles-as-coordinator-capable-code.md)**
<!-- docket:backlink:end -->
# Coordinator-capable Codex root entry implementation plan

> **For Codex:** Execute this plan inline under the change-0393 one-run authorization. Use red/green TDD for every behavior change, keep each intermediate state buildable, and stop at an open PR.

**Goal:** Route compositional Docket roles into a foreground Codex root thread whose contract, model, effort, working directory, permissions, request, and nested-agent controls match the installed role, while ordinary roles retain native child launch.

**Architecture:** Add a closed launch-posture value to the shared agent inventory and mark dispatch-owning coordinator sources. The Codex renderer will derive both its ordinary TOML and a typed root-entry contract from the same `AgentSource` and pin values. A small Codex app-server client will drive newline-framed JSON-RPC over `codex app-server --stdio`; a new `docket agent enter` command will validate the requested role's posture, apply the caller-supplied execution context, wait for the root turn, and return the final coordinator message. Repository seeding will append a Codex-only routing clause only when Codex is opted in; Claude-, Cursor-, and OpenCode-only output remains byte-identical.

**Tech stack:** Go 1.26, Cobra, standard-library JSON/process primitives, Codex app-server v2 JSON-RPC.

---

## Task 1: Define and guard launch posture in the authoritative inventory

**Files:**

- Modify: `internal/harness/inventory.go`
- Modify: `internal/harness/inventory_test.go`
- Modify: `agents/docket-implement-next.md`
- Modify: `agents/docket-auto-groom.md`
- Modify: `agents/docket-finalize-change.md`
- Test: `internal/harness/inventory_test.go`
- Test: `internal/harness/cross_harness_test.go`

1. Add red table tests for accepted absent/`child`/`root-coordinator` posture, rejection of unknown values, and preservation on `AgentSource`.
2. Add a correspondence test that derives dispatch-owning agent sources from a stable contract shape and proves every such source is `root-coordinator`, while every marked coordinator owns a dispatch edge. Include a mutation row that removes the posture from `docket-implement-next` and must fail.
3. Run `go test -count=1 ./internal/harness`; confirm the new tests fail for missing posture support.
4. Implement a closed `LaunchPosture` type, parser/default, and frontmatter field; annotate the three compositional agent sources.
5. Re-run `go test -count=1 ./internal/harness`; confirm green.

## Task 2: Make ordinary registration and root entry consume one Codex role contract

**Files:**

- Modify: `internal/harness/codex/codex.go`
- Modify: `internal/harness/codex/codex_test.go`
- Modify: `internal/harness/cross_harness_test.go`

1. Add red tests for a typed `RoleContract` containing name, description, launch posture, developer instructions (including recursion guard and skill preamble), model, and effort.
2. Add a drift test proving `renderAgent` and root entry receive byte-identical contract values rather than separately reconstructing instructions or pins.
3. Add golden/byte assertions proving child roles render unchanged and Claude, Cursor, and OpenCode outputs remain unchanged.
4. Run `go test -count=1 ./internal/harness/codex ./internal/harness`; confirm red.
5. Refactor Codex rendering through one exported contract constructor and keep TOML serialization a pure projection of that contract.
6. Re-run the focused tests; confirm green.

## Task 3: Implement the foreground app-server protocol driver

**Files:**

- Create: `internal/codexentry/client.go`
- Create: `internal/codexentry/protocol.go`
- Create: `internal/codexentry/client_test.go`

1. Write a scripted transport test that expects, in order, `initialize`, `thread/start`, and `turn/start`, and asserts exact developer instructions, model, effort, unchanged request, absolute cwd, approval policy, and sandbox values.
2. Add red event tests for foreground waiting, final `agentMessage` extraction, unrelated child `turn/completed` events being ignored, root completion, JSON-RPC errors, terminal failed/interrupted turns, missing final output, malformed frames, and process exit before completion.
3. Run `go test -count=1 ./internal/codexentry`; confirm red because the package does not exist.
4. Implement newline-framed JSON-RPC with injected process/stream seams. Launch exactly `codex app-server --stdio`; never invoke a shell, `codex exec`, another harness, ordinary-agent fallback, or relay. Preserve stderr only as bounded diagnostics and terminate/reap the app-server after the root turn.
5. Re-run `go test -count=1 ./internal/codexentry`; confirm green.

## Task 4: Expose the typed root-entry command and Codex-only parent route

**Files:**

- Create: `internal/app/agent_enter.go`
- Create: `internal/app/agent_enter_test.go`
- Create: `internal/cli/agent.go`
- Create: `internal/cli/agent_test.go`
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/assets.go` or the command asset-dependence registry source
- Modify: `internal/harness/dispatch.go`
- Modify: `internal/reposeed/plan.go`
- Modify: `internal/reposeed/plan_test.go`
- Modify: `internal/reposeed/record_test.go`

1. Add red application/CLI tests for `docket agent enter --role <name> --request <file|-> --cwd <absolute> --approval-policy <value> --sandbox <value>`, including unchanged multiline request bytes and JSON/human result presentation.
2. Add refusal tests for unknown role, ordinary-child posture, relative/nonexistent cwd, invalid execution context, unavailable/mismatched installed contract, and app-server failures. Each error class must remain distinguishable.
3. Add red repository-seeding tests: Codex opt-in carries the root-coordinator routing clause; OpenCode-, Claude-, and Cursor-only surfaces remain exactly `harness.DispatchInterior`; shared Codex+OpenCode `AGENTS.md` contains one clause; no generated route names `codex exec`, a relay, or another harness.
4. Run `go test -count=1 ./internal/app ./internal/cli ./internal/reposeed`; confirm red.
5. Wire the command through the compatible installed asset catalog, select only a `root-coordinator` contract from the inventory, and call the foreground app-server driver. Add the Codex-only routing extension at the repository planner boundary.
6. Re-run the focused tests; confirm green.

## Task 5: Documentation, decision record, and mutation proof

**Files:**

- Modify: `docs/codex/setup.md`
- Modify: `docs/codex/validation-runbook.md`
- Modify: `docs/codex/fixtures/nested-launch/README.md`
- Modify: `skills/docket-convention/references/agent-layer.md`
- Create: `docs/codex/fixtures/root-entry/README.md`
- Create: `docs/codex/fixtures/root-entry/probe-log.md`
- Create or update through the Docket ADR workflow: the ADR refining ADR-0036's parent-routing boundary

1. Document registration vs ordinary child vs root-coordinator entry, the exact command/context contract, restart requirement for installed artifacts, and unsupported-host diagnostics.
2. Record the narrow architecture decision: compositional Codex roles use native app-server root entry; leaves remain registered children; no relay or `codex exec` fallback exists.
3. Mutation-test the inventory guard by removing `launch: root-coordinator` from a disposable copy of a known coordinator and record the red/green commands.
4. Mutation-test the production route by substituting ordinary child posture in the fixture and prove the coordinator-to-plan-writer sentinel fails.

## Task 6: Live regression, full gate, and review

**Files:**

- Modify: `docs/codex/fixtures/root-entry/probe-log.md`
- Create if warranted: `docs/results/2026-09-01-coordinator-capable-codex-root-entry-results.md`

1. Build a candidate `docket` binary from the feature worktree and install its Codex assets into an isolated temporary home.
2. Run a fresh-process app-server fixture with fresh sentinels. Assert root source/context, developer marker, model, effort, skill marker, role identity, nested registered-leaf identity, and final sentinel round-trip from the root coordinator.
3. Run the actual installed `docket-implement-next` definition in a disposable repository with a bounded request that requires a real `docket-plan-writer` launch but cannot mutate the live backlog; record the consumed `PLAN_PATH=` receipt.
4. Run `go run ./cmd/docket development test`. Investigate every failure and handle any budget warning under `tests/README.md`.
5. Perform an inline whole-branch review against `origin/main`, fix every blocker/important/minor finding in scope, and rerun the full gate after fixes.
6. Publish the exact green head, open the PR with build evidence, and transition change 0393 to `implemented`. Because the prior halt marker could not be cleared through the defective pre-workspace resume path, remove only that marker through the explicitly authorized manual close-out after proving the final state, then verify `docket run verify --id 393` reports `run-complete`.
