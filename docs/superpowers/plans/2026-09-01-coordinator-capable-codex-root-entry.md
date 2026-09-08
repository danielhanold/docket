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

---

## 2026-09-08 continuation — preserve the feature-worktree contract

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`
> (recommended) or `superpowers:executing-plans` to implement this continuation task by task.

**Goal:** Fix change 0393 so every feature-scoped Codex role runs in the explicitly selected feature
worktree, while coordinator roots keep their caller cwd and metadata-scoped children keep native
named-agent dispatch.

**Architecture:** Promote the already-required `worktree-scope: feature|metadata` frontmatter into
the typed inventory and `codex.RoleContract`. Extend `agent.enter` into a two-mode boundary: a
`root-coordinator` enters at `--cwd`, while an ordinary `feature` role requires `--worktree`, proves
that it is a registered non-primary worktree in the same repository as `--cwd`, and enters the
app-server thread there. Codex registration and repository dispatch prose expose a generated
feature marker, and feature-role developer instructions fail closed unless the actual cwd matches
the exact `Feature worktree: <absolute-path>` field supplied unchanged by the owning workflow.

**Tech Stack:** Go 1.26, Cobra, `internal/gitcli`, Codex app-server v2 JSON-RPC, generated Markdown
and TOML assets, Go integration tests.

**Spec:** The approved `2026-09-08 amendment — feature-scoped child entry is worktree-anchored` in
[`2026-09-01-enter-compositional-docket-roles-as-coordinator-capable-code-design.md`](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-01-enter-compositional-docket-roles-as-coordinator-capable-code-design.md).

### Global constraints

- This continuation supersedes the original plan's unqualified claims that ordinary roles always
  use native child dispatch and that ordinary-role Codex bytes remain unchanged. Only
  metadata-scoped ordinary roles retain that path; feature-scoped registrations gain the generated
  marker and worktree guard required by the amendment.
- `--cwd` remains the invoking coordinator/repository context. `--worktree` is the runtime authority
  for a feature child. Never infer it from cwd, role name, change id, branch name, or free-form prose.
- The child request remains byte-identical. Owning workflows must include the exact structured line
  `Feature worktree: <absolute-path>` in the payload they already author; `agent.enter` must not
  prepend or rewrite request bytes.
- Do not hand-list feature role names. Parse the frontmatter declaration, carry it through typed
  contracts, and derive all routing, marker, startup-guard, and coverage decisions from that value.
- Do not hand-edit `internal/assets/embedded/**` or generated Codex goldens. Update authored sources,
  regenerate with the repository generators, and prove a second generation is idempotent.
- Accepted ADRs, archived changes, prior results, and the original portion of this plan are
  point-in-time records. Record the changed boundary in a new ADR that supersedes ADR-0103; append
  new results rather than rewriting the earlier certification narrative.
- Use red/green TDD for every behavior change. A guard is complete only after its named mutation has
  been run and observed red.

### Task 7: Make worktree scope a required typed inventory fact

**Files:**

- Modify: `internal/harness/inventory.go`
- Modify: `internal/harness/inventory_test.go`
- Modify: `internal/harness/codex/codex.go`
- Modify: `internal/harness/codex/codex_test.go`
- Regenerate later: `internal/harness/codex/testdata/golden/*.toml`

**Build profile:** standard

1. Add failing inventory tests with synthetic catalogs for a missing declaration, the accepted
   `feature` and `metadata` values, and an unknown value. Update every pre-existing synthetic
   `agents/docket-*.md` fixture to declare a valid scope so the failures identify only the behavior
   under test.
2. Add an embedded-inventory correspondence test that parses the whole catalog and proves every
   source has one of the two typed values. Partition the parsed sources by their value and compare
   the partition cardinalities with the catalog size; the source names are diagnostic output, not
   an expected allowlist.
3. Run `go test -count=1 ./internal/harness`; expect the new missing/unknown tests to fail because
   `agentFrontmatter` currently drops `worktree-scope`.
4. Add the closed type and fields:

   ```go
   type WorktreeScope string

   const (
       WorktreeScopeFeature  WorktreeScope = "feature"
       WorktreeScopeMetadata WorktreeScope = "metadata"
   )

   type AgentSource struct {
       // existing fields
       WorktreeScope WorktreeScope
   }
   ```

   Decode `worktree-scope` in `agentFrontmatter`, reject empty and unknown values in
   `parseAgentSource`, and remove the stale comment that describes the key as intentionally
   ignored. There is no default: ADR-0083 already makes the fact required.
5. Add `WorktreeScope harness.WorktreeScope` to `codex.RoleContract`, populate it only through
   `roleContract`, and extend the existing registration/root-entry drift test so both consumers
   receive the same scope value.
6. Mutation proof: in a temporary catalog remove the entire `worktree-scope:` line from one real
   source, then change it to `workspace`; both mutations must make `ParseInventory` fail. Restore
   the source and run `go test -count=1 ./internal/harness ./internal/harness/codex`; expect green.
7. Commit the typed inventory slice with an explicit path list. Suggested message:
   `fix(agent): preserve declared worktree scope in role contracts`.

### Task 8: Extend `agent.enter` with a verified feature-worktree mode

**Files:**

- Modify: `internal/cli/agent.go`
- Modify: `internal/cli/agent_test.go`
- Create: `internal/cli/agent_worktree_integration_test.go`
- Modify: `internal/codexentry/client_test.go`

**Build profile:** premium

1. Add `--worktree` to `TestAgentEnterCommandRegistered` and change the capability-signature
   expectation to:

   ```text
   --approval-policy <policy> --cwd <dir> --request <file> --role <name> --sandbox <mode> --worktree <dir>
   ```

   The flag is conditionally required and therefore must not be added to Cobra's unconditional
   `MarkFlagRequired` loop.
2. Add a table test for the role/scope matrix:

   | launch posture | scope | expected entry |
   |---|---|---|
   | `root-coordinator` | `metadata` | preserve `--cwd` unchanged; reject a supplied `--worktree` as contradictory |
   | `child` | `metadata` | preserve `ordinary-child-role` refusal so the caller uses native dispatch |
   | `child` | `feature` | require and validate `--worktree`; use its canonical root as app-server cwd |

   Include stable semantic refusal reasons for missing, nonexistent/not-a-directory, nested rather
   than exact-root, primary, unregistered, and foreign-repository targets. Keep request/flag syntax
   errors as CLI errors and role/checkout state failures as `AgentEnterResult` refusals.
3. In a real temporary repository, create two linked worktrees A and B plus the primary worktree.
   Exercise the validator with `--cwd A` and prove it rejects: no `--worktree`, the primary path, a
   nested directory under B, a removed/unregistered checkout, and a linked worktree from another
   repository. Then prove exact B is accepted. This test must compare canonical paths so symlink
   spellings cannot bypass the checks.
4. Run `go test -count=1 ./internal/cli`; expect red before implementation.
5. Implement one helper used by the command, with this responsibility boundary:

   ```go
   func resolveAgentEntryCWD(
       ctx context.Context,
       git *gitcli.Client,
       contract codex.RoleContract,
       callerCWD string,
       requestedWorktree string,
   ) (effectiveCWD string, reason string, err error)
   ```

   For a feature child, require both supplied paths to be absolute existing directories; use
   `gitcli.Client.Discover` on caller and target; compare canonical `CommonDir` values; use
   `DiscoverWorktree` to require the request equals that checkout's canonical `Root`; reject a root
   equal to `Repository.PrimaryWorktree`; and use `ListWorktrees` to prove the exact canonical root
   is registered. Return B as `effectiveCWD`. Root coordinators return the caller's exact `--cwd` and
   metadata children retain their current refusal.
6. Pass only `effectiveCWD` to `codexentry.Request.CWD`. Do not add a change-id field, inject request
   prose, modify `codexentry`'s wire protocol, or create a second entry client.
7. Extend the scripted app-server CLI test with `docket-rebase-resolver`: parent cwd is A,
   `--worktree` is B, `thread/start.cwd` must be B, developer instructions must be the resolver's
   installed contract, and `turn/start` text must exactly equal the input file. Keep the existing
   coordinator row proving root entry still uses `--cwd` unchanged.
8. Mutation proof: temporarily replace `effectiveCWD` with `callerCWD` at the `codexentry.Request`
   construction. The feature-row assertion on `thread/start.cwd` must fail. Restore and run
   `go test -count=1 ./internal/cli ./internal/codexentry`; expect green.
9. Commit the entry-boundary slice. Suggested message:
   `fix(agent): anchor feature role entry to verified worktree`.

### Task 9: Generate scope-aware Codex contracts and dispatch policy

**Files:**

- Modify: `internal/harness/codex/codex.go`
- Modify: `internal/harness/codex/codex_test.go`
- Modify: `internal/harness/dispatch.go`
- Modify: `internal/reposeed/plan_test.go`
- Modify: `internal/repoguard/root_entry_dispatch_test.go`
- Modify only if the compact rule exceeds its existing ceiling: `internal/repoguard/budgets_test.go`
- Regenerate: `internal/harness/codex/testdata/golden/*.toml`

**Build profile:** standard

1. Add failing renderer tests that derive every feature-scoped role from `ParseInventory` and assert
   its registration description carries `[docket worktree: feature]` immediately after any launch
   marker; current feature-child registrations therefore begin with it. Metadata roles must not
   carry that marker. Preserve `[docket launch: root-coordinator]` as the first prefix for
   coordinator roles so the existing parent-facing launch selection stays stable.
2. Add a feature-only developer-instruction assertion for a compact startup guard with these exact
   semantics: before any read or write, locate the `Feature worktree: <absolute-path>` input,
   canonicalize it and the process cwd, require equality at the worktree root, and halt visibly on
   missing, relative, nonexistent, nested, or mismatched values. Metadata roles must not receive
   that guard.
3. Replace the unconditional `codexDispatchBoundary` contract with a scope-aware target-routing
   rule. It must tell a parent to inspect registered description markers and select:

   - `[docket launch: root-coordinator]` → foreground catalog-resolved `agent.enter` at caller cwd;
   - `[docket worktree: feature]` → foreground catalog-resolved `agent.enter` with the owning
     workflow's exact `--worktree`;
   - unmarked metadata child → direct native named-agent dispatch.

   The rule must still say that a nested tool inventory cannot prove dispatch unavailable and must
   forbid `codex exec`, a shell runner, another harness, generic-agent substitution, and relay.
4. Update `CodexRootEntryClause` to state the same precedence and to prescribe a request file,
   unchanged payload, caller `--cwd`, and explicit `--worktree` for feature children. Keep the
   implement-next run-gate paragraph and root-return verdict ownership intact.
5. Add generated-policy correspondence tests: parse the inventory, render each contract, and prove
   each feature role has both the marker and startup guard while each metadata child remains on the
   native route. The reverse assertion must reject a feature marker on any metadata role. Do not
   enumerate the plan writer, build profiles, review rungs, resolver, or repair role in test data.
6. Refresh the TOML goldens with
   `go test -count=1 ./internal/harness/codex -update`, then inspect the diff: only feature-role
   descriptions/instructions and the shared dispatch paragraph may change. Claude, Cursor, and
   OpenCode goldens must remain byte-identical.
7. Mutation proof: make the feature branch of the generated route say native dispatch. The
   inventory-derived route test must fail for every feature member; restore it and run
   `go test -count=1 ./internal/harness/... ./internal/reposeed ./internal/repoguard`.
8. Measure the managed dispatch block. If it exceeds the existing budget, first compact duplicated
   prose; only if the required contract still cannot fit, rebaseline to the exact measured count
   with a change-0393 explanation and no spare capacity.
9. Commit the generated-policy slice. Suggested message:
   `fix(codex): route feature roles through worktree entry`.

### Task 10: Make every owning workflow supply the same structured worktree input

**Files:**

- Modify: `skills/docket-convention/SKILL.md`
- Modify: `skills/docket-convention/references/agent-layer.md`
- Modify: `skills/docket-build/SKILL.md`
- Modify: `skills/docket-implement-next/SKILL.md`
- Modify: `skills/docket-finalize-change/SKILL.md`
- Modify: `skills/docket-finalize-change/references/gate-failure.md`
- Create: `internal/repoguard/feature_worktree_dispatch_test.go`
- Modify only if the measured contract cannot be compacted under its ceiling: `internal/repoguard/budgets_test.go`
- Regenerate: `internal/assets/embedded/**`

**Build profile:** standard

1. Whole-repo search the maintained executable/instruction surfaces for dispatches to
   feature-scoped roles. Derive targets by joining those hits to `ParseInventory`'s scope, then sort
   them into maintained source, generated mirror, and point-in-time history. Edit maintained source
   only; the grep result, not this file list, is authoritative.
2. In the convention's composition and agent-layer sections, replace the retired universal native
   child rule with the scope matrix. State that Codex feature entry is the same installed contract
   through `agent.enter`, metadata children remain native, and other harnesses retain their native
   worktree-preserving mechanisms.
3. At every owning payload site, require one exact line:

   ```text
   Feature worktree: <absolute canonical feature-worktree root>
   ```

   This includes plan writer and selected review rung in `docket-implement-next`, each worker
   profile in `docket-build`, and resolver and integration repair in `docket-finalize-change`.
   Remove obsolete statements that Codex children receive the path through the retired runner
   facade, but preserve the harness-neutral requirement that the payload names the path.
4. Add or extend a shape-based repoguard test that derives feature targets from agent frontmatter,
   finds their maintained dispatch sites, and requires the structured line contract without a
   hand-written target list. Mutation-test at least one site from each owner (`implement-next`,
   `build`, `finalize`) by deleting the line and observing red.
5. Measure changed skill files against `TestSkillSizeBudgets`. Compact first; if a required contract
   cannot fit, set the affected ceiling to its exact new line/word count and document this one-time
   change-0393 boundary extension in the adjacent comment.
6. Run `go generate ./internal/assets`, `go run ./cmd/genassets -check`, and
   `go test -count=1 ./internal/assets ./internal/repoguard`. Save the first generated diff to a
   temporary file created with an explicit `mktemp` template, run generation a second time, and
   require the new diff to byte-match the saved diff; remove the temporary file afterward.
7. Commit authored and generated files together. Suggested message:
   `docs(agent): require structured feature worktree dispatch input`.

### Task 11: Prove the original cross-worktree failure is fixed

**Files:**

- Modify: `internal/cli/agent_worktree_integration_test.go`
- Modify: `docs/codex/fixtures/root-entry/certification.md`
- Modify: `docs/reference/harness/validation-runbook.md`

**Build profile:** premium

1. Extend the real-Git fixture from Task 8 into the production-shaped regression. Create primary,
   coordinator worktree A, and feature worktree B. Start an owned rebase conflict in B so
   `git -C B ls-files -u -- conflict.txt` is non-empty while the same command in A is empty.
2. Seed the actual generated `docket-rebase-resolver` registration into an isolated home and enter
   it through the public CLI with `--cwd A --worktree B`. The scripted app-server transport may
   replace model reasoning only; it must consume the real selected role contract and, from the
   received `thread/start.cwd`, execute the read-only unmerged-index probe and return its observation
   through a normal terminal `agentMessage`.
3. Assert the returned observation names B's canonical root and a non-empty live unmerged set. Also
   assert request bytes include the exact `Feature worktree: B` line and are otherwise unchanged.
   Clean up the disposable rebase/worktrees through test helpers even on failure.
4. Mutation proof: route `thread/start.cwd` to A. The assertion must fail specifically because the
   observed unmerged set is empty—the same symptom reported while finalizing change 0349. Restore
   the production route and run the focused test at least three times:

   ```text
   go test -count=3 ./internal/cli -run 'TestIntegrationAgentEnterFeatureResolverObservesSelectedWorktreeConflict'
   ```

5. Update the validation runbook with the A/B fixture and append a dated section to the existing
   certification recording the failed-current symptom, fixed-new cwd/unmerged observation, Codex
   version, and exact test/head. Do not claim this test resolves or continues a production rebase.
6. Commit the regression and documentation. Suggested message:
   `test(agent): reproduce resolver conflict across coordinator worktrees`.

### Task 12: Record the revised architecture decision

**Files:**

- Create through the Docket ADR workflow: a new metadata ADR superseding ADR-0103
- Modify through `change.reconcile`: change 0393's `adrs:` relationship
- No direct edits: `docs/adrs/0103-enter-codex-coordinator-roles-through-app-server-root-thread.md`

**Build profile:** standard

1. Dispatch `docket-adr` with the approved amendment and implemented contract. The new ADR must
   preserve ADR-0103's root-coordinator decision while superseding only its claim that every ordinary
   role uses native child dispatch.
2. Record the decision matrix: metadata ordinary children use native dispatch; feature ordinary
   roles use foreground app-server entry at a verified explicit worktree; root coordinators use
   caller-cwd entry. Relate it to ADR-0083 and ADR-0103, and explain why change id, caller cwd,
   role-name pattern, request prose, `codex exec`, runner resurrection, and relay are not authority.
3. Follow the ADR workflow's fresh `repository.prepare`, explicit staging, commit, lease-push, and
   index update. Re-read the returned ADR from `origin/docket`, then attach its number to change 0393
   through the catalog-resolved `change.reconcile` operation with the fresh entity version.
4. Verify the old Accepted ADR is byte-unchanged and the new ADR's `supersedes` relation names 103.

### Task 13: Full verification, review, and updated build evidence

**Files:**

- Append: `docs/results/2026-09-07-enter-compositional-docket-roles-as-coordinator-capable-code-results.md`
- Modify as required by findings: only files already in the change-0393 implementation scope

**Build profile:** standard

1. Run format and focused checks:

   ```text
   go fmt ./internal/...
   git diff --check
   go test -count=1 ./internal/harness/... ./internal/codexentry ./internal/cli ./internal/reposeed ./internal/repoguard ./internal/assets
   go run ./cmd/genassets -check
   ```

   `go fmt` must leave no unexpected diff and `git diff --check` must print nothing. Investigate
   every failing package rather than weakening an assertion.
2. Run the complete configured gate from source exactly as required by repository policy:

   ```text
   go run ./cmd/docket development test
   ```

   Treat `SERIAL CONFIRMED OVER BUDGET:` as an authoritative breach. Record and investigate every
   `BUDGET WATCH:` or `PARALLEL-SENSITIVE:` line under `tests/README.md` even though the runner does
   not fail on those lines by default.
3. Run all named mutations from Tasks 7–11 against disposable copies or temporary source edits,
   restore each immediately, and rerun its green focused test. Append the exact red/green commands
   and outcomes to the existing results file; do not rewrite its earlier 2026-09-07 observations.
4. Review the entire branch against `origin/main`, including the pre-amendment implementation.
   Fix every in-scope blocker, important, and minor finding, then rerun focused checks and the full
   gate after the final code change.
5. Verify the final diff carries no changes to Claude/Cursor/OpenCode generated output, no hand-edits
   to embedded assets, no edits to Accepted ADRs or archived records, no request rewriting, and no
   feature-role name list. Confirm every currently feature-scoped source is covered by the
   inventory-derived assertions without encoding a population count as routing authority.
6. Commit the appended results with the final tested HEAD and suite evidence, push the exact feature
   head, and update the existing PR/build-evidence through change 0393's normal implemented-state
   transaction. Do not retry finalization of change 0349 as part of this change; its PR remains a
   separate human-authorized workflow.
