<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0425 — Restore native Codex dispatch for Multi-Agent V2 Docket coordinators](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0425-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor.md)**
<!-- docket:backlink:end -->
# Native Codex dispatch — change 0425 implementation plan

> For the phase-2 coding agent: execute this plan directly at **gpt-5.6-sol / low**, sequentially in the prepared feature worktree. The user's launch-kit bootstrap overrides dispatch-based execution suggestions in planning skills. Do not invoke ImplementNext, build orchestration, worker agents, `agent.enter`, or another agent launcher to implement 425. Checkboxes describe work; commits and external receipts record completion.

**Goal:** Restore native registered Codex coordinator/child dispatch while preserving explicit feature ownership, complete inputs, private task authority, gate continuity, and verifiable artifacts.

**Architecture:** Codex's adapter supplies native routing and narrowly loaded role references. A small Go validation boundary checks immutable assignments/resources and the final private worker payload using the existing workspace and gate backends; it never launches an agent or test. Status resolves active plan/results objects against their owning feature revision, while existing attachment, gate, continuation, and lifecycle authorities remain in charge.

**Tech stack:** Existing Go module, Git adapter, Cobra capability/schema registry, TOML renderer, embedded asset generator, Go tests and maintained Go suite runner. No new Python/Node product dependency.

**Spec:** `docs/superpowers/specs/2026-09-14-restore-native-codex-dispatch-for-multi-agent-v2-docket-coor-design.md` on the authoritative metadata branch; during this bootstrap read `/Users/homer/dev/docket/.docket/` plus that path. The live spec and launch-kit `inputs/425-spec.md` were byte-identical during planning, SHA-256 `afa58d7f47c1bf78dbbf6ea705bdd105e572ed9726ea45c03887213748f7336c`.

## Authority, baseline, and phase boundaries

- Worktree: `/Users/homer/dev/docket/.worktrees/restore-native-codex-dispatch-for-multi-agent-v2-docket-coor`.
- Branch: `codex/restore-native-codex-dispatch-for-multi-agent-v2-docket-coor`.
- Source base: `06ebb52c058894b564ac2a8432922ddf4b2d56b3`; initial tree clean; `workspace.inspect` returned this registered path/ref/base/HEAD and `ready`.
- Launch kit: `/Users/homer/dev/docket-0425-launch-kit`; read `COMMON.md` and the selected phase prompt in each fresh task. Execute its checker with `python3 /Users/homer/dev/docket-0425-launch-kit/preflight.py` from the exact worktree. This is an external launch checker, not a new product dependency.
- Change 425 is already claimed/reconciled. Planning refreshed its claim through the live catalog. Do not create a second claim or workspace. `context.implementation --id 425` currently refuses `not-ready-not-proposed` because it selects proposed work; use synchronized `status`, `workspace.inspect`, and configuration diagnostics for this already-in-progress bootstrap.
- Bootstrap a fresh `capabilities --json` catalog, validate protocol/capability versions and unique operation IDs, and resolve argv plus operation schemas before mutation. Entity versions are record blob identities; metadata revisions are commit identities. Never substitute one for the other. Read `status.context.metadata_revision`, not an assumed local `docket` ref.
- All source/file operations explicitly target the feature root. Metadata operations use official writers and fresh entity versions. Stage only owned paths. No global install, model changes, PR publication, merge, native run, or 424 metadata change is authorized in phases 1–3.
- **Phase 2 owns code tasks 1–8**, focused tests, generated deterministic/native preparation assets, successor ADR, and implementation handoff. **Phase 3 owns verification track S**: full suite, inline repairs, source results checkpoint and final-head evidence. **Phase 4 owns independent source review and preparation for track N**; its fresh native launch is separately executed by the user. **Track D** is 424's later separate dogfood. Tasks below do not authorize executing those later tracks early.

The launch-kit baseline passed the configured source command `go run ./cmd/docket development test`: 46 suite files, zero failures, 399 reported assertions, 320.32 seconds. Nine parallel `BUDGET WATCH` findings occurred at concurrency 11; no serial-confirmed breach was reported. Preserve `evidence/baseline.json`, `baseline.log`, and `baseline-review.md`. This is direct source-runner evidence, not an attributed ImplementNext gate. It need not be repeated during planning.

## Source findings and generated-surface inventory

The following inventory comes from the current tree, not the POC's three-role list. Re-run whole-repo searches when implementing; classify executable instructions, generated copies, historical evidence, and dormant compatibility implementation before editing. A positive legacy string search alone is not a violation.

```sh
rg -n --hidden 'agent\.enter|CodexRootEntryClause|featureWorktreeStartupGuard|require equality|Feature worktree:' agents skills internal AGENTS.md cursor-rules docs/reference/harness docs/install
rg -n '^name:|^skills:|^launch:|^worktree-scope:' agents/docket-*.md
rg -n 'gate\.drive\.(start|prepare-scope|handoff|claim|takeover)|results-template|PLAN_PATH' agents skills
```

| Surface | Current authority and required treatment |
|---|---|
| `internal/harness/codex/codex.go` | `roleContract`, `codexTargetRouting`, `featureWorktreeStartupGuard`, `skillsPreambleFormat`, `Plan` generate all native definitions. Replace Codex route/startup prose and add role-specific reference loading before skill preloads. Preserve model/effort resolution and typed inventory. |
| `internal/harness/dispatch.go` | `CodexRootEntryClause`/`CodexDispatchInterior` add the contradictory parent route. Replace that Codex-only clause; retain harness-neutral `DispatchInterior` and shared gate payload. The exported historical symbol name may remain to avoid unrelated compatibility churn. |
| `internal/reposeed/plan.go`, `internal/app/repophase.go`, `internal/install/repophase.go` | Repository installation generates parent instructions only. Codex/OpenCode can share AGENTS.md; scope the new clause explicitly to Codex. Preserve non-Codex opt-in output and ownership/marker checks. |
| `AGENTS.md` managed dispatch block | Regenerate from `harness.CodexDispatchInterior` using `document.Parse`/`PatchSet.ReplaceBlock`; preserve all authored rules. Do not hand-edit an unvalidated marker range. |
| `agents/docket-*.md` | 17 current sources: 10 feature roles (plan writer, four build profiles, three review rungs, rebase resolver, integration repair), 7 metadata roles (ImplementNext, finalize, auto-groom, status, ADR, critic, consultant). Three metadata roles carry root-coordinator launch metadata. Keep shared role bodies/metadata unchanged unless a harness-neutral defect is separately justified; Codex overrides belong in generated preambles/references. |
| `skills/docket-implement-next`, `docket-build`, `docket-build-task`, `docket-review`, `docket-finalize-change`, `docket-convention` | Existing shared edges/roles remain the base contracts. Add Codex-only references under the existing packaged skills roots. Adapter-generated instructions select them; other harnesses do not load them. Read existing gate-caller-loop, gate-execution, task-routing, fix-loop, edge-paths, agent-layer and finalize gate-failure references before changing their callers. |
| `internal/harness/codex/testdata/golden/*.toml` | All 17 generated definitions change because every role gets native routing; regenerate the complete inventory-derived set. |
| `internal/assets/embedded/tree/skills/...`, `internal/assets/embedded/manifest.json` | New/changed skill references are copied by `go generate ./internal/assets/`. `DefaultAllowedRoots` already includes skills, agents, cursor-rules and `.docket.example.yml`; no new allowed root is needed. |
| `internal/assets/embedded/tree/agents/...`, `.../cursor-rules/...` | Must remain byte-identical if their authored sources do. Do not edit embedded copies independently. `cursor-rules/run-gate.md` stays harness-neutral, including optional epoch and cancellation semantics. |
| Other harness goldens, `internal/install/legacydata`, `internal/install/testdata/legacy` | Other harness outputs and frozen legacy ownership reproducers remain unchanged. They are not native-route violations. |
| Installed output | Codex `Plan` writes agent TOML under `UserRoots.Home/.codex/agents` and skill links under `.agents/skills`; it does not emit a new global AGENTS.md. Repository output does not install project agent definitions. Test generation/install with injected temporary roots, never the real global setup in this phase. |
| `internal/app/status.go`, `status_git.go`, `sweep_session.go` | `artifactChecks` currently sends both plan/results to `sourceIntegration`. Add one ownership-aware reader seam and forward it through bound readers. Metadata/spec and terminal maintenance rules retain their sources. |
| `internal/app/change_attach.go` | `ChangeAttachPlan`/`ChangeAttachResults` already inspect workspace/current HEAD, read Git blobs, and validate backlinks; plan also checks single-artifact commit/trailer. Preserve those checks. `changeAttachReceipt` currently persists id/kind/op/path, not feature commit/blob; workspace manifest also has no artifact commit field. |
| `internal/gatedrive`, `internal/app/gate_drive.go`, `internal/cli/gate.go` | Scope and drive authority already exists. `GateScopeResult` has top-level scope/capability fields; `GateDriveResult` nests `DriveDoc` in `drive`. Reuse backend checks, never fixture private-record paths. |
| `internal/repoguard/{root_entry_dispatch,feature_worktree_dispatch,gatedrive_scope_identity,gatedrive_json_capture}_test.go` | Replace obsolete route assertions, retain marker census and all JSON/identity invariants; add executable mutation coverage for the new contracts. |
| `docs/install/codex.md`, `docs/reference/harness/validation-runbook.md` | Update the live Codex route and link a new candidate-native runbook. Retain historical measurement labels; no broad retirement of old runner/CLI documentation. |

`RoleContractFromDefinition` and `internal/codexentry` still support the separately callable legacy entry implementation. Removing that implementation or its tests is 426. No automatic Codex workflow may select it after this change. Existing `agents/harness-defaults.yml`, `.docket.yml`, `.docket.local.yml`, and global model configuration are not implementation targets.

## Settled contracts

### Native routing and disclosure

Every Codex target, including root-marked coordinators and feature-marked children, uses the harness's actual top-level registered named-agent dispatch. Markers remain descriptive typed metadata; neither selects another root process. Keep the original outer gate key private to the caller, pass its dispatch context unchanged, and forward an epoch only when supplied and supported. Retain and observe the exact native child identity until terminal return. A notification or child success sentence does not replace the parent's keyed gate verdict.

Before generic skill loading, feature definitions say: read the supplied assignment and checker only, run the checker, then load role resources using explicit feature/control paths. Allowed initial reads are assignment/checker, exact pinned control resources, and necessary Git identity data. Worker/reviewer/planner do not run `repository.prepare` or write metadata. The planner's `artifact.backlink` writes its one feature artifact and is allowed. Reviewer stays read-only. Coordinator owns repository preparation and metadata. An inline configured build/review role still inherits its coordinator's feature binding; it does not invent a child scope merely because a skill was loaded.

Add these narrowly selected packaged references:

- `skills/docket-convention/references/codex-native-dispatch.md`: shared Codex dispatch/tool resolution, actual child observation, capability bootstrap and model prerequisite.
- `skills/docket-convention/references/codex-feature-binding.md`: role-aware entry/active checks, explicit cwd/path discipline, scoped read exceptions, resolver/repair entry distinctions.
- `skills/docket-implement-next/references/codex-planning-results.md`: planner resource closure, exact results template, plan receipt verification, results/checkpoint and source-review rules.
- `skills/docket-build/references/codex-task-handoff.md`: static assignment → scope → final private payload; receipt capture, same-scope sequence, continuation/escalation.
- `skills/docket-review/references/codex-review-binding.md`: pinned review HEAD/base/evidence, no worker capability, no tests or writes.

Use adapter helper `codexReferences(s harness.AgentSource) []string`, deriving feature binding from `WorktreeScope`, task/review/controller references from actual skill bindings, and the plan-writer special case from its unique registered role (it intentionally has no skill preload). All roles receive the small native route clause; only relevant roles load detailed references. Controller reference instructs active configured role skills to load the relevant Codex detail before the edge. No new model-capability inventory field.

### Static assignments and executable validation

Implement **one new input-check operation**, `agent.check-inputs`, as a local, read-only Go validator. This name is a planned addition, not a currently installed capability. Register it in the CLI catalog/schema before references invoke it. It must never launch an agent, prepare a scope, start a test, mutate metadata, change cwd globally, or copy/write resources.

Create `internal/codexcontract` for pure DTO/resource/receipt validation; app wiring composes Git/workspace/gatedrive. Exact public CLI shape:

```text
agent.check-inputs --assignment <absolute-file> --sha256 <digest>
  --stage <prepare|dispatch|entry|active>
  [--payload <absolute-private-file> --payload-sha256 <digest>]
  [--repo-dir <absolute-feature-root>] --json
```

Use a strict version-1 `Assignment` document, stable JSON tags and no unknown keys:

```go
type Resource struct {
    LogicalID string `json:"logical_id"`
    Path string `json:"path"`
    SHA256 string `json:"sha256"`
    Source string `json:"source"` // explicit package/root/revision provenance
}
type Assignment struct {
    SchemaVersion int `json:"schema_version"`
    ChangeID int `json:"change_id"`
    Role string `json:"role"`
    Phase string `json:"phase"`
    TaskID string `json:"task_id,omitempty"`
    Mode string `json:"mode"` // fresh, continuation, escalation, review, resolver, repair
    Primary string `json:"primary"`
    Feature string `json:"feature"`
    CommonDir string `json:"common_dir"`
    Branch string `json:"branch"` // short branch, exactly as gate scope expects
    EntryHEAD string `json:"entry_head"`
    MetadataRevision string `json:"metadata_revision"`
    ChangePath string `json:"change_path"` // relative to pinned metadata tree
    ArtifactPath string `json:"artifact_path,omitempty"` // safe feature-relative output
    DocketExecutable string `json:"docket_executable"`
    DocketCommit string `json:"docket_commit"`
    Resources []Resource `json:"resources"`
    ReadRoots []string `json:"read_roots"` // exact declared control/resource roots
    WritePaths []string `json:"write_paths"` // task-owned feature-relative boundaries
    InheritedPaths []string `json:"inherited_paths"`
    InheritedFingerprint *gatedrive.Fingerprint `json:"inherited_fingerprint,omitempty"`
    TestArgv []string `json:"test_argv,omitempty"`
    RunRoot string `json:"run_root,omitempty"`
}
```

Add explicit optional planner fields `plan_skill`, `build_skill`, `learnings_enabled`, `learnings_index`, and review fields `review_base`, `review_head`, `build_evidence` to this struct in Task 3/6; their meaning is fixed here. Skill fields name logical identities and point through `Resources`; they are not authority to rediscover a different installation. Use Go field names `PlanSkill`, `BuildSkill`, `LearningsEnabled`, `LearningsIndex`, `ReviewBase`, `ReviewHEAD`, `BuildEvidence`.

The assignment contains no gate context, scope IDs, capabilities, generations, reservation secrets or handoff tokens. The controller creates it with ordinary file tools in protected external control storage and pins the exact byte digest. Child reads are possible from primary startup because the exact control root is explicitly declared. A digest proves input consistency, not host-enforced isolation. Reject duplicate JSON keys, unknown schema/keys, noncanonical roots, relative/escaping output paths, Git redirect environment, symlink redirection, incorrect role/resource combinations, and a helper executable outside the declared boundary. Bound assignment/payload JSON to 1 MiB; resource contents are read individually and bounded to 8 MiB per file, with a clear resource-too-large refusal (no silent truncation).

`CheckInputsRequest` carries CLI fields; `CheckInputsResult` embeds `app.Envelope`, carries only safe `reason`, assignment/payload digests, root-identity witness and observed root/ref/HEAD facts. Never echo raw input on error. `prepare` validates fixed inputs before scope creation; `dispatch` validates the completed payload; `entry` repeats the entry identity/resource check in the child; `active` rechecks root/branch/containment and role/stage expectations after authorized work.

The app constructs `workspace.Target` from the change object at `MetadataRevision`, using existing effective-base resolution, then calls `workspace.Service.Inspect`. It does not parse workspace private files. Require same canonical common dir, a registered existing non-primary linked feature, matching change/ref/path and reachable base. Add `RootIdentity` with platform, device, inode and canonical Git-dir identity in `internal/codexcontract/root_identity_{darwin,linux,unsupported}.go`; use `os.Lstat`/platform stat fields on the two currently supported process platforms and a typed unsupported refusal elsewhere. The first read-only `prepare` returns this witness; the controller includes it in the final assignment's `root_identity`, then pins that final document and validates it again. Dispatch/entry require that witness; active checks compare it. This detects same-path replacement without adding a private state store. It is not a hard-isolation guarantee against an actor modifying and restoring the same directory.

Initial fresh planner/worker and review entry require expected HEAD and clean state. Continued/escalated workers accept exactly accounted inherited paths and a fingerprint matching the controller's observation; use the already exported `gatedrive.ComputeFingerprint` and `Fingerprint.Equal`. Serialize that struct in `inherited_fingerprint`, rather than inventing a filename-only hash. Active worker checks allow its assigned edits and descendant task commits; they do not re-demand entry HEAD or cleanliness. Active reviewer remains fixed to review HEAD. Resolver entry uses finalize's explicit owned conflict workspace/attempt and unmerged paths, allowing detached HEAD and conflict state only after the existing resolver reservation/attempt checks; do not force `workspace.Inspect`'s ordinary ready/branch rule onto a rebase scratch target. Extract a read-only `validateResolverEntry` helper in `internal/app/finalize_rebase.go` from `requireOwnedAttempt` and the reservation/stopped-commit checks in `finalizeRebaseContinueBudgeted`, without invoking continuation or spending its reservation. The private resolver payload carries attempt/reservation; assignment carries only public expected paths/HEAD. Repair uses its existing finalize context and expected repaired-branch boundary. Neither resolver nor repair receives fabricated build-worker credentials.

Checker invocation uses the absolute compiled Docket executable and the catalog-resolved `agent.check-inputs` argv. No script execution bit is assumed. A legacy/interpreter-bearing external test checker must be represented as the complete argv vector, e.g. `python3`, absolute checker path, arguments; never split off the interpreter. Production references require the Go checker. Independent shell calls reload fixed inputs from the pinned document; never rely on variables set in a previous tool call, `eval` JSON into shell, or use a login shell that relocates cwd.

### Resource closure and results template

Before planner dispatch, resolve the real configured planning skill and read its full main resource. Supply the main resource and every mandatory referenced resource with logical identity, usable canonical path, SHA-256, and package/revision provenance. Preserve relative layout or provide an explicit logical-path mapping; child uses supplied content, not a parent-only skill name. For local packages, inventory the selected package directory with hidden files included, pin regular resource files and reject escaping symlinks. Directory hashes are computed over sorted relative path + file digest pairs. For provider-backed resources, the controller materializes exact bounded text/resources into its declared control directory and records provider provenance; the child never guesses a local path for a provider URI.

The parent owns semantic selection of mandatory resources. It records an explicit dependency graph in the assignment's optional `resource_dependencies` (logical parent → logical children); the checker validates closure, every listed digest, and every local Markdown link that resolves to a file within the selected package. Missing imported skill/package links require explicit resolution or refusal, never silent fallback. Do not implement an NLP crawler or download arbitrary links. `auto` is explicit authoring mode with no external planning package; a named missing/incomplete skill in the Codex path halts before dispatch and does not silently degrade to `auto`. This Codex-only stricter preparation is disclosed ahead of the shared fallback text.

Results template is the actual packaged `skills/docket-implement-next/results-template.md`, resolved from the selected skill's canonical directory. Pin its bytes before costly work. If delivered as a tracked feature snapshot (including `.agents/skills/...`), verify it again after workspace creation against the pinned blob and canonical path. Plain `rg --files` is not a template resolver. Do not fall back to another global template. Template availability never counts as results creation, publication or attachment.

### Private worker payload and backend scope check

After `prepare` passes, derive every `prepare-scope` identity from the same assignment: canonical feature/common repository identity, exact change/task/phase/branch and unchanged optional context/epoch. In particular `1` and `task-1` are different opaque task IDs; never reconstruct one from the other.

The private `WorkerPayload` is a strict versioned document containing assignment path/digest, exact entry argv, task text, scope ID, actual child capability, unchanged optional context/epoch and applicable predecessor drive/generation. The preparing parent's capability and gate key stay in controller-private storage. Scope grant/first responses/private payloads use 0700 directories and 0600 files outside tracked artifacts. Parent fields in a child payload are rejected. Role selection and static assignment digest must match the target registered definition. The dispatch operation receives the exact validated message bytes (load the private file, compare its digest, then call native dispatch with those bytes). No draft validation payload can replace the final one.

Do not create circular hashes: assignment and private payload are independently hashed, with their outer digest/locator passed in the native message envelope. Entry argv pins the static assignment; the separate payload digest check uses that envelope. The private payload does not contain its own digest. Controller and child both check it before task work; the controller records the exact dispatched-payload digest privately.

Add `Driver.ValidateChildInputs(req StartRequest) error` in `internal/gatedrive`: a read-only seam reusing `precheckScopedStart`, `scopedIdentityMatch`, capability hash checks and epoch revocation checks. Refactor pure checks only; preserve atomic admission/reservation in `Start`. Also check that a supplied predecessor is the scope's actual current drive, not merely another reusable drive. A preparation check reserves/consumes nothing and cannot replace Start's revalidation. `app` calls it from `agent.check-inputs` for actual worker dispatch/entry. No CLI consumer reads `record.json`, a private scope envelope or a schema-version-specific path. Prepared-scope response remains its supported public source of actual capability values.

Planner/reviewer payloads contain their assignment/resources and necessary role authority only. Require no scope, child capability or predecessor for them. For resolver/repair, validate the existing role-specific finalize inputs instead of coercing them into WorkerPayload.

### First response and same-drive continuation

Add `agent.check-receipt` (second and final new CLI operation) as a **read-only data checker**, implemented by pure `codexcontract.ParseReceipt` plus app DTO mapping. Input: `--assignment <file> --sha256 <digest> --operation <gate-operation-id> --stdout <file> --stderr <file> --exit-code <integer> --json`. It reads the captured first response; it does not invoke any command or reopen gate private state. Register its schema/effects. Return a nested, validated protocol document to protected stdout for the consuming owner; human output and error messages are sanitized. Never print parent capability in human output.

Use current DTOs for actual shapes: scope replies have `scope_id`, `child_capability`, `parent_capability`; drive replies have `drive.drive_id`, `drive.generation`, `drive.outcome`. In a handoff reply `drive.generation` is the single-use handoff token; after claim/takeover it is the new owner generation. There is no separate `handoff_id` response field. `recordedDoc` supplies `drive.run_root` only for terminal start/advance/acknowledge replies; WAITING legitimately omits it. `transferDoc` for handoff/claim/takeover omits run_root even when its last outcome is terminal. Retain the assignment root through those operations, require equality whenever a root is present, and require it for terminal start/advance/acknowledge. For PASSED validate `raw_run_dir` containment beneath the assigned root. A recognized FAILED reply can arrive with nonzero process exit. Invalid JSON, missing required fields, wrong operation/protocol, half receipts, and command refusal preserve stdout/stderr/exit and halt. A diagnostic HALTED reply may lack ownership fields; classify it as a halt with its original cause, never a reusable receipt. Never call start again to recover output.

Codex reference gives one concrete capture pattern using Bash, explicit feature cwd, protected absolute files and an argv array resolved in the same call:

```bash
# gate_argv, first_stdout, first_stderr are loaded from the pinned private inputs
# during THIS shell invocation. The shell tool itself has login disabled.
if "${gate_argv[@]}" >"$first_stdout" 2>"$first_stderr"; then
  gate_rc=0
else
  gate_rc=$?
fi
# Retain gate_rc immediately; run the catalog-resolved receipt checker next.
# A shell-tool live session is observed to terminal before reading these files.
```

This sample is implemented and tested as literal reference content in Task 4; no product shell runner is introduced. No reserved `status`/`pipestatus`; no `&&` branch that drops error output or skips recovery reads. Capture preparation, handoff, claim, takeover and acknowledgement replies with the same discipline. The checker never deletes originals.

Baseline, assertion RED, GREEN and focused verification within one uninterrupted child dispatch use one real task scope with sequential predecessor receipts. Commit successful task edits before final acknowledgement. WAITING → handoff immediately → controller claims fresh generation → advances the same drive. **Current `Driver.Claim` and `Driver.Takeover` close the child scope.** Do not reopen it, reuse its child authority, or acknowledge it as though it were an ordinarily completed worker scope. Record the terminal recovered result and closed-by-transfer disposition separately.

Only after terminal consumption and quiescence may a fresh same-task continuation receive a new static assignment for current HEAD/inherited edits. If it needs further tests, prepare a new dispatch-boundary scope under the same change/task/phase/context/epoch; its first start omits predecessor flags because the previous scope is closed. Carry the recovered old drive/generation as **historical continuation evidence**, not as a predecessor in the new scope. Do not rerun the recovered passing stage. If only self-review/commit remains, use a continuation payload carrying the trusted recovered terminal receipt and no newly invented test/scope; return that recovery disposition explicitly instead of falsely claiming final acknowledgement. `agent.check-inputs` validates this continuation against the backend's recorded terminal drive and closed scope through a read-only `Driver.ValidateRecoveredInputs` seam; no direct private record access. If a continuation later needs a test not anticipated in its assignment, return control for the controller to prepare the proper new boundary rather than self-minting scope authority.

Return-without-handoff takeover is authorized by the observed child return, never quiet logs or time. Escalation preserves existing one-escalation policy and inherited edits. A returned worker that has not created a drive has no process to recover; otherwise consume/acknowledge or take over its actual result before any replacement boundary. A new dispatch scope is permitted only after the old work is accounted for; it never duplicates a live drive or resets the outer run's budget/deadline. Busy/unresolved/stale-owner/broken-predecessor states halt under existing policy. Retain cancellation and outer gate retry/continuation meanings unchanged.

### Ownership-aware artifact diagnostics

Add `StatusReader.ReadChangeArtifact(ctx context.Context, pin StatusPin, target ChangeArtifactTarget) (ChangeArtifactObservation, error)`; implement and forward it in `gitStatusReader`, `boundStatusReader`, counting/fake readers. `ChangeArtifactTarget` is a typed app DTO containing change ID, slug, status/location, branch, resolved effective base, artifact kind/path, all derived from the **same pinned snapshot**. `ChangeArtifactObservation` contains found/regular/backlink-valid flags, source kind, full revision, blob identity and a bounded reason; raw artifact bytes remain internal as in `StatusArtifact`.

For active `in-progress`/`implemented` feature artifacts, resolve/inspect the registered owning workspace, pin its verified HEAD once, read the regular Git blob at that immutable commit, validate its backlink to the metadata change, and keep source revision/blob in the diagnostic observation. Dirty working-tree bytes cannot satisfy a missing committed artifact. Wrong branch, missing/foreign workspace, unresolved revision/base, symlink blob or wrong backlink remain findings. Re-read root/ref identity at the end; a changed live identity yields an uncertain/read failure, never success. Use the existing domain effective-base policy for stacked work.

After integration/terminal ownership is established, retain the existing integration/stacked-terminal resolution rules. Do not turn an active missing feature into success because an unrelated integration file happens to exist at the same path. Do not change `maintenance_assess.go`'s terminal source rules as a side effect. Metadata specs continue using `pin.MetadataRevision`.

Use attachment provenance that actually exists: `changeAttachReceipt` id/kind/op/path and the validated transaction's metadata revision support attachment identity. They do not supply an original feature commit/blob. This change need not add a second persistent artifact ledger or change attachment schemas: status proves the linked committed artifact at a pinned owning HEAD; final acceptance separately proves the actual attach request/receipt and Git object. When a caller supplies persisted feature commit/blob evidence, verify it against reachable objects rather than ignoring mismatches. Do not infer a commit from workspace manifest `BaseCommit`. Diagnostics must state what was verified and never claim original-attachment blob comparison where no such value was retained.

## Ordered phase-2 implementation tasks

Each task has one focused baseline/RED/GREEN cycle, mutation receipts where guards change, self-review and a commit. This manual bootstrap runs tests directly; it does not invent task scope credentials. Generated-output refreshes belong to the owning task. Record task commands, exact assertion failures, mutations, changed paths and full commits in launch-kit `evidence/implementation-log.md`. The test command must actually run assertions; an undefined symbol/compiler error is scaffolding feedback, not sufficient behavioral RED evidence.

### Task 1: Add role-aware feature input validation

**Own:** new `internal/codexcontract/assignment.go`, `assignment_test.go`; new `internal/app/agent_inputs.go`, `agent_inputs_test.go`, `agent_inputs_integration_test.go`; `internal/cli/agent.go` command registration plus new `agent_inputs.go`/`agent_inputs_test.go`; `internal/app/schema_registry.go`; capability/schema tests affected by the added operation. Existing `workspace.Inspect` and `gitcli` are dependencies; do not duplicate their manifest parser.

**Before → after:** feature-role protection is an instruction requiring cwd equality; a real read-only check now accepts primary startup while proving the assigned feature. No generated route changes until this operation works.

- [ ] Add strict Assignment/CheckInputs DTOs and compile a refusing stub so tests reach assertions. Implement JSON duplicate-key/unknown-field rejection and digest checks using standard Go libraries. Add `ValidateAssignment(Assignment) error` in `codexcontract`; app `CheckAgentInputs(context.Context, AgentInputDeps, CheckInputsRequest) CheckInputsResult` owns backend calls. `AgentInputDeps` holds Git/workspace/catalog readers, not a launcher. Include the root-identity platform files and the read-only resolver validation extraction described above in this task's ownership.
- [ ] Add table tests using a real temporary primary/registered feature: primary-start and feature-start both succeed; wrong repository/change/ref, primary-as-feature, missing/unregistered root, root symlink/replacement, relative/nested path, stale HEAD, dirty fresh planner, Git redirect environment, escaping artifact, modified assignment fail before task reads. Include legitimate worker edits then commit → active check passes. Include escalation inherited diff and resolver detached conflict state through its existing finalize validation seam.
- [ ] Implement role/mode checks described above. Reuse `gatedrive.ComputeFingerprint` for inherited edits; no new hash of only `git status` filenames. Keep the resolver exception bound to actual reservation/attempt authority, not `mode: resolver` alone.
- [ ] Validate exact checker argv and executable identity. Mutation: strip an interpreter from a 0644 test checker and observe failure; normal production Go executable succeeds. Child remains allowed to bootstrap its own catalog after binding.
- [ ] Run `go test -count=1 ./internal/codexcontract ./internal/cli ./internal/app -run 'Test.*(AgentInput|Assignment|Capability|Schema)'` and `go test -tags integration -count=1 ./internal/app -run '^TestIntegrationWorkflowAgentInputs'`. Name tagged integration tests with that prefix so the existing workflow shard owns them. Inspect actual RED/green output; commit explicit owned files.

**Completion evidence:** path/role mutation table, real primary-start successful binding, active-edit/commit acceptance, operation visible in source-built catalog/schema. No source preparation or gate launch side effects. **Generated:** none yet beyond any catalog/schema test fixtures actually derived from code.

### Task 2: Validate final worker authority against the real scope

**Own:** new `internal/gatedrive/validate_inputs.go`, `validate_inputs_test.go`; narrowly refactor shared checks in `driver.go`/`scope.go` if needed; extend Task 1 app validator and `codexcontract/worker_payload.go`, `worker_payload_test.go`. Extend `internal/gatedrive/integration_sequence_test.go` and appropriate app/CLI tests. Do not change scope storage format or admission/retry policy.

**Interfaces:** implement `Driver.ValidateChildInputs(StartRequest) error`; extend `AgentInputDeps` with a read-only validator interface of that signature. `WorkerPayload` uses exact identity fields defined above, with optional predecessor pair and optional epoch; child messages never serialize a ScopeGrant wholesale.

- [ ] Baseline current scope sequence/capability tests. Add a real prepared scope with `task_id: "1"` against assignment `"task-1"`; expect pre-dispatch refusal and zero drives. Add omission/substitution cases for capability, context, epoch, task/change/ref/worktree, assignment digest and predecessor. Add a parent-capability contamination case. Planner/review payload with no worker fields must succeed.
- [ ] Refactor `precheckScopedStart`'s read checks for the new seam, retaining atomic validation at actual Start. Explicitly compare epoch presence/value and cancellation fencing; precheck must not mint a scope, advance generation, reserve a slot or acknowledge a predecessor. A race after validation must still be refused by Start.
- [ ] Require `dispatch`/worker `entry` to validate the actual completed private payload; `prepare` intentionally runs without a scope. Tests keep a draft without child capability and show it cannot pass dispatch. Compare exact payload-byte digest immediately before native call; native calls themselves remain agent actions.
- [ ] Test fresh, terminal predecessor, WAITING/pending, closed scope, stale generation, and current-drive mismatch cases. Keep original scope and retained first response untouched on refusal. Mutation: bypass identity comparison and ensure the 1/task-1 test fails.
- [ ] Run `go test -count=1 ./internal/gatedrive ./internal/codexcontract ./internal/app ./internal/cli -run 'Test.*(Scope|Scoped|ChildInputs|WorkerPayload|AgentInput)'`; self-review and commit explicit paths.

**Completion evidence:** original mismatch fails before dispatch, real capability accepted, planner/reviewer unscoped cases accepted, no private-record reader outside gatedrive. **Generated:** none; refresh schema fixtures if DTO fields expand.

### Task 3: Make planner resources and hidden template availability explicit

**Own:** new `internal/codexcontract/resources.go`, `resources_test.go`; extend assignment/app checks; new `skills/docket-implement-next/references/codex-planning-results.md` (resource/planning/template sections first); source-asset tests in `internal/assets/embedded_test.go` as needed. Preserve `agents/docket-plan-writer.md`'s shared single-artifact receipt contract.

**Interfaces:** `ValidateResources(Assignment) error` validates declared resource closure/layout/digests and exact results-template identity; add the optional planner/dependency fields specified above. No command launches a skill or automatically fetches its links.

- [ ] Baseline real resource/template fixture. Add parent-only path, missing mandatory nested reference, hash drift, imported reference without mapped resource, hidden `.agents` template, changed template, symlink escape, provider URI masquerading as local path and oversized file cases. Match `auto` semantics separately.
- [ ] Implement canonical file reads with explicit read roots, sorted package manifests and dependency closure. The controller resolves selected resources before dispatch; the checker rejects a complete-looking main file whose linked local dependency is absent. Use a package fixture with at least two levels of links so one-hop checking cannot pass.
- [ ] Write precise Codex reference: planner reads the supplied skill, authors only its assigned plan, calls official backlink, commits only that path with one `Docket-Plan-Path` trailer and returns existing `PLAN_PATH=...`. Parent obtains the full commit from verified Git state, optionally accepts an informational commit line, and attaches with fresh version. No second compulsory success grammar or replacement plan authoring.
- [ ] Preserve baseline single-artifact/trailer/backlink tests in `internal/app/workflow_integration_test.go`; add resource-closure mutations to executable tests. Run `go test -count=1 ./internal/codexcontract ./internal/app -run 'Test.*(Resource|Template|AgentInput)'` and `go test -tags integration -count=1 ./internal/app -run '^TestIntegrationWorkflowChangeAttachPlan'`.
- [ ] Regenerate embedded assets, check drift, self-review and commit owned source/reference/embedded paths.

**Completion evidence:** complete resource closure usable from inherited primary startup; exact hidden template succeeds, genuine absence stops before planner/worker work. **Generated:** matching embedded new reference and manifest only.

### Task 4: Preserve and parse the first gate response

**Own:** new `internal/codexcontract/receipt.go`, `receipt_test.go`; new `internal/app/agent_receipt.go`, `agent_receipt_test.go`; new `internal/cli/agent_receipt.go`, `agent_receipt_test.go`; `agent.go`/`schema_registry.go`; new `skills/docket-build/references/codex-task-handoff.md`; extend `internal/repoguard/gatedrive_json_capture_test.go` with executable reference capture tests (a separate topical Go test file is acceptable).

**Interfaces:** `ParseReceipt(operation string, stdout, stderr []byte, exitCode int, assignment Assignment) (Receipt, error)` preserves originals and validates operation-specific nested DTOs. `Receipt` carries the parsed existing gate/scope document and classification, not a newly invented owner token. `agent.check-receipt` wraps this pure function; payload output is private operational data. Avoid an import cycle: `codexcontract` can import `gatedrive.DriveDoc`, but must not import `app`; its wire envelope has only the existing protocol/operation/result fields. Cross-package tests in `app` serialize real `GateDriveResult`/`GateScopeResult` into this parser to enforce wire correspondence.

- [ ] Baseline existing CLI gate envelope tests. Serialize actual `app.GateDriveResult` values into new parser fixtures; add PASSED, FAILED with nonzero exit, WAITING without run_root, terminal wrong/relative root, invalid/missing JSON, command refusal, handoff and claim generation cases. Scope replies use their different top-level shape.
- [ ] Implement parser without accepting the old top-level `drive_id/owner_generation` shape. Validate required fields before consuming any; preserve stdout/stderr/rc even when malformed. Do not demand terminal-only run_root on WAITING. Match raw_run_dir containment only where present/allowed.
- [ ] Put the concrete capture block and complete per-operation field selectors in the Codex handoff reference. Test the literal block with temporary executables that emit valid FAILED+exit1 and invalid JSON+exit2; assert one invocation, original stdout/stderr available, and rc retained. Run in both Bash and zsh when available; a missing optional shell is reported in local focused evidence, while the required supported-shell CI matrix must exercise it.
- [ ] Add an independent-shell test: a second shell can reconstruct arguments from pinned files without variables from the first. Mutations restore old parser, drop error stdout, use `status`, lose interpreter, or rerun start; each test must turn red. Never create a producer→early-exit-consumer SIGPIPE guard.
- [ ] Run `go test -count=1 ./internal/codexcontract ./internal/app ./internal/cli ./internal/repoguard -run 'Test.*(Receipt|Capture|GateDrive|ScopeIdentity)'`; regenerate assets, self-review and commit explicit paths.

**Completion evidence:** actual serialized nested documents accepted, malformed originals retained, exactly one start in error tests, shell reload coverage. **Generated:** new handoff reference embedded, manifest and affected schema fixtures.

### Task 5: Switch generated Codex routes and inject role-bound references

**Own:** `internal/harness/codex/codex.go`, `codex_test.go`; `internal/harness/dispatch.go`, `dispatch_test.go`, `native_dispatch_test.go`; `internal/reposeed/plan_test.go`; `internal/repoguard/root_entry_dispatch_test.go`, `feature_worktree_dispatch_test.go`; new Codex native-dispatch/feature-binding references; `AGENTS.md` managed block; Codex goldens. Add a bounded `cmd/gendispatch/main.go` developer generator: it calls existing `document`/`harness` renderers with explicit `-repo` and `-check`; it must not install anything. This generator's purpose and interface are fixed, not an invitation to add a general installer command.

**Before → after:** every generated Codex role and opted-in parent use native named dispatch; feature entry checks target ownership regardless of startup cwd. Other harness rendered bytes retain their behavior.

- [ ] Establish baseline adapter/inventory tests. Replace obsolete assertions with semantic tests over **all** `ParseInventory` roles, including a synthetic added feature role. Require native route, no executable automatic `agent.enter`/root-relocation/startup-equality instruction, correct scope reference, and permission preamble before generic skill loading. Retain recursion guards.
- [ ] Implement `codexReferences` and the preamble order; keep source launch/scope fields and resolved model/effort unchanged. Missing registration/tool or explicit denial/uncertain launch halts with configuration remedies; no automatic generic/inline/runner substitute. Do not probe by duplicating real work.
- [ ] Rework parent clause to retain native child identity, first-output observation, gate context/key/epoch and retry/continuation contracts. Keep common `DispatchInterior` byte-identical; on co-owned AGENTS.md the Codex clause is explicitly conditional.
- [ ] Mutation-test generated content: restore automatic root entry, remove feature check, put generic convention loading before role exception, remove context forwarding, or leave an uncovered newly added inventory role. Separate executable shape from negative prose and historical legacy code. Retain feature-dispatch marker order/balance and direct-site census.
- [ ] Regenerate assets **before** Codex goldens; run `go test ./internal/harness/codex -run '^TestCodexGoldenAgents$' -update`, then without `-update`. Regenerate only AGENTS.md's validated block through the renderer; no source-wide hand replacement. Run `go test -count=1 ./internal/harness/... ./internal/reposeed ./internal/repoguard`.
- [ ] Prove other harness goldens and frozen legacy assets unchanged with an explicit diff; test config precedence and manual pin preservation through adapter/install fixtures. Commit owned paths.

**Completion evidence:** generated parent plus all 17 candidate definitions select native dispatch, non-vacuous mutations fail, other harness comparisons pass. **Generated:** all Codex TOML goldens; managed AGENTS.md block; embedded new references/manifest. No global artifacts are touched.

### Task 6: Wire continuation, review and results sequencing into Codex contracts

**Own:** Codex handoff/planning-results references from Tasks 3–4; new `skills/docket-review/references/codex-review-binding.md`; Codex adapter reference selection; tests in `internal/gatedrive/{integration_sequence,integration_takeover,handoff,acknowledge}_test.go`, `internal/app/agent_inputs*_test.go`, `internal/repoguard` as appropriate. Keep shared workflow skills unchanged; Codex precedence handles the stricter input/re-review requirements without changing other harness behavior.

**Before → after:** native children receive current continuation state and role authority; their completion cannot skip results/final-head or certify a moved review HEAD.

- [ ] Add scenario tests: baseline→RED→GREEN→commit→ack; WAITING→handoff→claim→advance→same-task continuation; returned child without handoff→authorized takeover; stale owner and duplicate scope/start refused. Assert actual scope closure on claim/takeover, refusal of a closed-scope successor, a quiescent new boundary with no cross-scope predecessor, and commit-only continuation using the recovered pass without another test. Add `RecoveredInputs` (scope ID, drive ID, current generation, expected task identity/context/epoch) and `Driver.ValidateRecoveredInputs(RecoveredInputs) error` in `internal/gatedrive/validate_inputs.go`, checking closed/transferred scope, bound current drive, terminal outcome, current owner and complete identity; return no authority and perform no mutation. Extend the private payload tagged union accordingly. Use existing injected clocks/supervisor seams, not long sleeps. A new static assignment may inherit accounted dirty changes and updated HEAD; it must not reset gate deadline or budget.
- [ ] Complete reference instructions for final payload dispatch, child-owned catalog, explicit scope-derived identity, controller-private parent cap, final acknowledgement and sanitized task report. Handle escalation at most once, with no commit on failed attempt and preserved inherited paths. A pending handoff is claimed, not taken over.
- [ ] Implement review fields/checks: immutable review base/head/evidence, root/ref checks before reads, no test runner, no code edits, no child scope. Mutation: move branch after preparing review input; entry refuses. Source code fixes after independent review require a fresh review in this bootstrap and Codex acceptance; never label the earlier review as covering them. Do not expand other harness review-loop policy.
- [ ] Complete results instructions: pin template early; author actual results; backlink→explicit-path commit→publication→fresh-version attachment→gate at final results HEAD. Preserve implementation-stage evidence. No results write under a live drive. Allow external final-checkpoint receipts so results do not need their own final hash embedded.
- [ ] Test acceptance contract refuses template-only results evidence, unpublished/misattached results, missing final checkpoint, final HEAD mismatch, and changed/omitted context or supplied epoch. Context-free existing ungated operation remains supported; actual gated native acceptance never drops its context.
- [ ] Run focused `go test -count=1 ./internal/gatedrive ./internal/codexcontract ./internal/app ./internal/repoguard -run 'Test.*(Sequence|Handoff|Takeover|Acknowledge|Continuation|Review|Checkpoint|AgentInput)'`; regenerate references/manifest and affected Codex goldens, self-review and commit.

**Completion evidence:** each continuation path independently tested; no inference that POC passed WAITING/review. Separate implementation/final-head and review identity assertions. **Generated:** changed reference mirrors/manifest and affected Codex goldens only.

### Task 7: Resolve plan and results diagnostics from the owning feature snapshot

**Own:** `internal/app/status.go`, `status_git.go`, new `status_artifact.go`/`status_artifact_test.go`; `sweep_session.go`; fake/counting StatusReader implementations found by whole-repo search; `status_test.go`, `status_git_test.go`/new tagged `status_artifact_integration_test.go`; `workflow_integration_test.go`, maintenance tests where the reader interface changes. Preserve `change_attach.go` checks and existing DTO compatibility.

**Interfaces:** implement `ChangeArtifactTarget`, `ChangeArtifactObservation`, and `StatusReader.ReadChangeArtifact` exactly as specified above. Build the target using the same snapshot/branch facts that status already owns. Reuse `workspace.Service.Inspect` and pinned `gitcli.ObjectSource`; do not call the CLI recursively.

- [ ] Baseline status/attach tests. Create real registered temporary feature workspace, attach a plan and results that exist only on feature, keep integration without those paths, and pin a newer metadata revision while the local docket ref is stale. Expect both links healthy in `in-progress` and `implemented`.
- [ ] Implement ownership resolution once and call it for both fields. Add observations to findings' messages/related data without changing existing finding codes unnecessarily. For absent/mismatched owned artifacts emit a finding; for a failed Git/I/O probe retain typed external failure. Do not globally suppress active errors.
- [ ] Add cases: missing file; untracked-only file; symlink blob; wrong backlink; removed/foreign/mismatched workspace; moved ref during read; unavailable commit; stack uses parent's actual effective base; integrated/terminal artifact resolves under existing ownership. Verify dirty contents cannot override committed blob; inspect manifest never assumes a commit field.
- [ ] If attachment receipt provenance is read, use the transaction layer's validated receipt/trailer parser against pinned metadata history; do not parse private transaction files. Compare only fields actually persisted. Add tests explicitly proving current receipt lacks original commit/blob and status does not claim that check. Preserve normal attach idempotency and single-artifact/trailer negative cases.
- [ ] Mutation: restore unconditional integration lookup → feature-only positive cases fail; suppress all missing active links → negative cases fail; use local docket ref → stale-ref case fails. Run `go test -count=1 ./internal/app -run 'Test.*(Status|Artifact|Attach|Maintenance)'` and `go test -tags integration -count=1 ./internal/app -run '^TestIntegrationWorkflow.*(Status|Artifact|Attach)'`.
- [ ] Use `TestIntegrationWorkflow...` for new tagged real-Git app tests so the existing workflow shard discovers them; inspect its budget before growing expensive cases. Self-review and commit explicit paths.

**Completion evidence:** feature-only plan **and** results are healthy at pinned revisions; all negative ownership probes remain visible; stale metadata case covered. **Generated:** none unless public schema test fixtures legitimately change.

### Task 8: Package deterministic/native acceptance preparation and decision documentation

**Own:** new `internal/harness/codex/native_fixture_test.go`, new `internal/codexcontract/acceptance_test.go`; new developer-only `cmd/nativefixture/main.go` plus tests; new `docs/reference/harness/native-codex-acceptance.md`; current Codex sections in `docs/install/codex.md` and `docs/reference/harness/validation-runbook.md`; successor ADR authored through metadata operations. Generated fixture contents live only in an explicit fresh external destination. Do not change frozen 423 files.

**Fixture interface:** `go run ./cmd/nativefixture -source <clean-425-checkout> -binary <absolute-candidate-docket> -destination <new-absolute-directory> -pins <operator-authored-config>`. This is a developer fixture generator, not an installed Docket capability or agent launcher. It refuses an existing destination, dirty/unpinned source, unverified binary identity, missing skills/template or unsafe targets. It creates a disposable primary/local bare origin, a tiny build-ready fixture change and baseline tests, using supported candidate metadata operations. It creates no feature workspace, scope, agent or gate. Production routing/checker bytes come from candidate sources and the actual renderer; no POC role override is copied.

- [ ] Test fixture preparation with injectable filesystem/process seams and temporary roots. Generate `codex.New().Plan` with candidate assets and resolved explicit pins using an injected staging `UserRoots`; materialize exact rendered TOML into the disposable project's `.codex/agents` and canonical skill resources into `.agents/skills`, preserving relative resource layout. Record source/destination mapping and hashes. This is test preparation only; do not change machine/repository install ownership policy to support it.
- [ ] Keep parent AGENTS.md production routing bytes from `CodexDispatchInterior`; add a separate fixture launch document with the absolute candidate executable and bounded stopping instructions. Parent and children resolve their own catalog from that executable. Catalog argv[0] is bound to this verified absolute binary; subcommand/flag/schema content is taken unchanged from that candidate's catalog. No reliance on terminal PATH inheritance.
- [ ] Define acceptance receipt fields and cross-checks in tests: source/full binary commit and hashes, generated definitions/resources, configured and independently observed roles, canonical runtime roots, native lineage, plan/task/results commits, scope sequence/ack, actual publication/attachment metadata revision, implementation/final checkpoint gates, read-only review head, original keyed terminal verdict, primary audit limitations. Public fixtures/receipts contain no live capabilities or parent keys.
- [ ] Port failure **cases**, not POC transports: parent-only skill, missing capability, 0644 direct execution, lost variables, 1/task-1, wrong response shape, lost first error, hidden template, stale local metadata, absent final results checkpoint. Add the transient-write-restored test: final snapshots compare equal after write+restore; acceptance must retain `evidence_audit_complete: false` and not claim hard isolation.
- [ ] Add track N/D launch instructions below, including candidate loading prerequisite. Tests verify fixture preparation cannot dispatch, claim a production change, precreate its feature or touch global homes. Keep 424 policy and metadata outside fixture generation.
- [ ] Record the successor ADR only once the implemented choices are concrete, using fresh catalog/schema and exact ADR/change versions. The draft content below is settled proposal text; do not mutate ADR-0114 directly. Use official `adr.supersede` (or its live schema's successor-record flow), index/backlink rendering, and link the real successor to 425 through supported metadata mutation. No guessed ADR number.
- [ ] Run `go test -count=1 ./cmd/nativefixture ./internal/harness/codex ./internal/codexcontract ./internal/assets ./internal/repoguard`; run `go run ./cmd/genassets -check`; inspect complete source diff and generated outputs. Commit explicit source/doc/test paths, retain metadata ADR receipt externally.

**Completion evidence:** fixture generator tests prove candidate production assets are used and no launch occurs; risk/acceptance docs and successor ADR receipt exist. **Generated:** embedded/goldens only if references changed; generated disposable trees never staged. **Phase-2 stop:** write `implementation.json`/report with all task commits, focused test/mutation receipts, and explicit full-suite/native/review work still outstanding. No full native result may be inferred from these tests.

## Verification track S — actual 425 source, phase 3 and subsequent review

1. Verify planning/implementation receipts, exact source ancestry, clean current feature HEAD and spec hash. Resolve `diagnostic.config` and the relevant prepared context/schema; read **both** `build.test_command` and `finalize.test_command` from current effective config. The checked-in baseline has both `go run ./cmd/docket development test`; `finalize.skip_results_only_delta` is **false**. Do not replace either with a remembered command or a subset.
2. Run the configured full source suite from this checkout through `internal/suiterunner`'s maintained CLI entry. Use a real supported build-owned driver when collecting publishable gate evidence; its catalog/schema chooses exact arguments and budget. An unscoped manual run must not invent an outer key/epoch or worker cap. Preserve the first complete response and observe the same live session/drive until terminal. If only a direct source-runner invocation was used, label it honestly and do not present it as driver evidence.
3. Inspect all suite/budget output. Compare new warnings to the nine recorded baseline screenings. Act on serial-confirmed breaches and in-scope regressions; preserve failure logs and run narrowly to diagnose. Never raise ceilings, weaken guards or skip a red shard. A new shell suite requires category declaration and `tests/runtime-budgets.tsv` row; prefer current Go packages/shards. Existing `tests/test_go_toolchain.sh`, race runner, integration shard census and asset guards must exercise new code.
4. Repair inline under phase-3 authority, commit, then run affected checks and the configured full suite at the new clean source HEAD. After pass, avoid extra repeats without new changes/concerns. Record raw command, HEAD, initial cleanliness, logs, exit, suite summary and budget result externally.
5. Author the real source results artifact from this checkout's exact `skills/docket-implement-next/results-template.md`; record completed source validation and pending native acceptance/review honestly. Stamp official backlink, commit only results path, attach through fresh-version `change.attach-results`. Phase 3 forbids pushing: report publication as pending, not durable remote evidence. Attachment and local commit can be complete while publication remains pending.
6. The results commit advances HEAD. Obtain the configured final-checkpoint gate at that clean commit, retaining implementation-stage evidence separately. Do not rewrite results simply to paste their own final SHA; store final receipts externally in `verification.json` and adjacent evidence. A substantive later results edit requires a new checkpoint because results-only skipping is disabled here.
7. Phase 4 independently reviews the **whole** 425 source branch at an exact HEAD. Any later code fix returns to repair/full verification and fresh independent review. A results-only delta is disclosed explicitly with the exact changed paths, prior reviewed source commit and applicable review policy; it still requires final-head gate evidence under current config.
8. A real source PR may be opened only in the later authorized phase using `workspace.publish`/`pr.publish` and truthful current-head evidence. If `evidence.record`, `pr.publish` or `change.mark-implemented` rejects manual-bootstrap evidence, preserve the exact refusal and unsatisfied conjunct. Do not forge ImplementNext provenance, claim native `run-complete`, or bypass lifecycle state. A separately authorized metadata exception is a later decision. The phase-1 plan attachment is not such an exception.

## Validation track N — minimal generated-production native acceptance

This is subsequent native acceptance, not a phase-2 or phase-3 action. Phase 4 prepares it; the user starts it in a fresh Codex app Local task.

**Candidate loading decision:** use a clean pinned candidate checkout and an explicitly addressed candidate binary, plus project-scoped definitions generated from the production adapter and project-scoped skill resources in a disposable fixture. No change to stable global Docket/Codex definitions. Current `development.install --bin-dir` redirects only the binary; it still plans global role/skill targets. Current `reposeed.Plan` installs no repository agent definitions. Therefore neither flag is a safe candidate-loading recipe by itself. Do not run global development installation against the user's home to prepare this test.

The fixture generator uses actual rendering APIs with injected temporary roots and materializes their generated output as test assets. Candidate binary is built from a clean checkout with the same `internal/buildinfo` version/commit/date stamping used by `internal/install/devmode.go`; verify full clean commit through its own `version` operation and SHA-256. No modified checkout or unstamped `go build` may masquerade as the pinned candidate. Exact executable path is carried to parent and children, which each bootstrap their own live catalog and verify its binary commit.

**Named launch prerequisite: `codex-candidate-loading-unverified`.** Planning observed installed CLI `0.154.0`; `codex app --help` exposes no documented per-session agent/skill-root flag. The 423 POC corroborates project definitions, but does not prove that this app session will load a newly generated complete candidate set or preserve selection in native children. Before substantive native work, the fresh parent must verify the registered candidate definitions/paths and its candidate executable; each actual child must verify its loaded role/resource provenance and executable before work. A parent-only file check or PATH export is insufficient. If project definitions cannot be selected safely or provenance cannot be established, stop launch with this named prerequisite and concrete host evidence. Do not invent a loader flag, restart another harness, call `agent.enter`, or overwrite stable global setup. This blocker does not prevent completing phase-2 source work.

Operator sets exact existing configuration values deliberately: minimal parent/ImplementNext `gpt-5.6-terra / low`, planner `gpt-5.6-sol / medium`, standard worker `gpt-5.6-terra / medium`, as observed in 423. Any other active dispatch-owning role uses an operator-selected independently verified exact assignment. A leaf reviewer needs an appropriate configured review model but no automatic V2 family rule. Foreground model selection does not alter child pins. Record actual config provenance, regenerated definition bytes and runtime values where observable; no automatic edits to model settings and no 424 registry gate.

Continuous acceptance sequence:

1. Fresh disposable primary/local bare origin, build-ready fixture change, frozen primary source snapshot; no feature or scope exists yet. Parent verifies candidate/loading/model/resource prerequisites and arms the real outer gate with explicit fixture change selection.
2. Native ImplementNext claims/prepares a **new** registered feature. It resolves complete planner resources/template and native-dispatches a fresh planner. Planner starts at inherited primary or feature, binds explicitly, writes/backlinks/commits one plan. Coordinator verifies and attaches it at fresh metadata revision.
3. Prepare real task scope from immutable assignment; validate final private worker payload. Native worker executes focused baseline→real assertion RED→GREEN under one scope, commits and acknowledges. Controller runs the fixture's configured full suite at implementation HEAD. Preserve canonical absolute run roots and first responses.
4. Exercise the actual native registered read-only review at pinned feature HEAD, within the chain or as a separately attributed bounded native review. This must load generated production review assets; source PR review alone does not certify native review dispatch.
5. Author actual results from the verified template; commit/publish to the fixture's local bare origin/attach; run the independent full checkpoint gate at the final results commit. Verify actual remote head, results Git blob, attachment metadata revision and task acknowledgement. Template receipt alone cannot pass.
6. Stop at the declared boundary with official typed halt; collect terminal native return and the **original** parent's keyed verdict. `gate-stop / run-halted` is correct bounded acceptance, not a production completed PR. Preserve key/epoch privately for cancellation if needed; no duplicate start after liveness uncertainty.
7. Independent evidence review checks native lineage, actual loaded production bytes, pinned role settings, all commits/receipts and primary audit. Separate unauthorized source writes from authorized metadata/Git/private gate state. Report telemetry gaps; final snapshots cannot exclude transient restored writes. Native WAITING paths not exercised live remain deterministic-only coverage. Do not claim parallel or hard-isolation certification.

Any source fix invalidates candidate evidence for the older source: repair 425, update its source verification/review, regenerate from the new clean commit, and prepare a new fixture. Preserve earlier failed runs and partial work; stop live work through existing typed cancellation/continuation, never by deleting its tree.

## Validation track D — 424 as a separate stacked dogfood change

424 currently remains `depends_on: [423, 425]`, with no stack, and `status` reports waiting on 425. Planning must not alter that. Later, after 425 source verification, independent review, reviewed track-N acceptance, pushed branch and real PR, groom 424 into its own complete spec using normal metadata operations. Its policy/registry/diagnostic work is not included in tasks 1–8.

At the separately authorized launch boundary, apply the supported exact-version conversion to `depends_on: [423]` plus `stacked_on: 425`. Resolve candidate catalog/schema first. Verify effective base is the exact pushed 425 feature branch; absence/unresolved base stops launch. Do not precreate 424's worktree: the native coordinator must create it from that stack base, exercising inherited-primary binding.

Use a fresh native parent at primary with verified candidate loading and operator model assignments. Arm the normal outer gate for **explicit 424**, preserve context/key/epoch, and use actual native planner/workers/review/results/PR flow. 424's PR targets 425's branch and lives in a separate worktree. A 425 runtime defect is fixed on 425 and invalidates older dogfood attribution; preserve 424 partial work and use existing halt/resume/stack refresh. Never merge 424 into 425 to make a run pass.

Retain both exact heads, candidate source/binary/assets, lineage, gates, 424 plan/task/results/PR, and primary/feature audits. Prefer merge 425 first; keep its branch while open children need it; retarget/rebase/retest 424 under normal stack-aware finalize rules. No merge authority follows from launch. After a real 425 merge, follow AGENTS.md's sync-integration → clean-main/full merge ancestry → development.install → exact installed version proof. Failure is `binary rebuild incomplete` while the merged change remains done.

## Proposed successor ADR content

**Title:** Native Codex dispatch with explicit, role-aware feature binding.

**Relation:** Supersede ADR-0114's Codex route/startup-equality decision; retain its ownership invariant and links to ADR-0103/0083. Record using the next authoritative ADR ID only after implementation establishes the decision. Do not rewrite the accepted historical body.

**Context:** ADR-0114 addressed a real cross-worktree resolver failure by selecting app-server root entry and startup cwd equality. Accepted change-423 evidence now proves a continuous native coordinator/planner/worker flow where children inherit primary startup and bind to an explicitly validated fresh feature. The earlier failures involved incomplete inputs, scope identity, lost/misparsed replies and hidden templates, not proof that native dispatch was unavailable. Production must validate those boundaries and test generated assets.

**Decision:** Codex uses registered native named-agent dispatch for every role. Typed launch/scope metadata remains shared, but Codex does not translate it into automatic `agent.enter`. Validate a pinned role assignment against registered repository/workspace identity before feature reads/writes, permit primary startup, target all later feature operations explicitly, and use stage-aware rules for task edits, continuations, review and resolver work. Keep planner resources/template explicit; separate immutable assignments from live worker authority. A small read-only Go check boundary reuses existing backend scope validation and public gate response shapes; no private-record transport or extra agent runner. Existing gate ownership/continuation/cancellation remains authoritative. Other harness behavior and model policy are unchanged.

**Consequences:** Correct feature work no longer depends on an unsupported native cwd option. Integrity checks and explicit path discipline are not a sandbox; telemetry limitations remain visible. Operator-selected exact model/effort pins remain a launch prerequisite until 424. New candidate production assets and native review require fresh acceptance; 423 is design evidence only. Legacy runner retirement remains 426. Missing resources, tools, identity or ownership halt rather than selecting an alternate runtime.

**Alternatives:** Keep root entry (conflicts with restored native coordination); assume startup equals target (fails the observed host behavior); trust instructions without checked assignments (repeats cross-tree/input errors); ship the POC private-record/Node/Python glue (duplicates backend authority/dependencies); add a model registry now (424 scope); build a sandbox/tracer (not required by the observation contract).

## Risk register

| Risk | Mitigation and decisive evidence |
|---|---|
| Stale app registrations load stable definitions | Named candidate-loading prerequisite; fresh session; parent and actual child loaded-path/hash/runtime checks before work. No global overwrite. |
| Validator becomes a launcher or second gate implementation | Only two read-only data operations; no process-control effects; existing workspace/gatedrive seams; side-effect tests and source review. |
| Entry checks prohibit valid edits or resolver conflicts | Role/mode table; active descendant/owned-diff checks; existing finalize reservation; primary-start and conflict fixture tests. |
| Successful validation followed by identity drift | Exact dispatch digest, repeated child binding, pinned Git reads, Start's atomic revalidation; no hard-isolation claim. |
| Skill snapshot looks complete but omits required resources | Explicit graph, complete selected resource inventory, relative-link closure/digest mutations; unresolved mandatory provider resource stops preparation. |
| Parent authority leaks | Strict separate DTOs, 0700/0600 private storage, bounded redacted errors, payload/public-receipt secret-mutation tests. |
| Parser treats exit1 as no document or WAITING as malformed | Actual DTO serialization, first-stream capture, nested shape and terminal-only root tests, one-start counter. |
| Artifact fix hides real missing content | Both positive feature-only and negative owner/blob/backlink cases; missing versus probe-error distinction; metadata pinned by returned revision. |
| Scope/continuation changes spend retries or strand ownership | Existing driver authority; scenario tests for handoff/claim/takeover/ack, cancellation/epoch checks; native live paths reported individually. |
| Cross-harness behavior changes through shared bodies | Adapter-only selection, unchanged shared core/other harness goldens, co-owned AGENTS.md conditional clause tests. |
| Growing real-Git test shard breaches runtime budget | Extend current topical shards, inspect report and serial confirmation; no budget raises or redundant full-suite runs. |
| Manual evidence cannot pass ordinary publication lifecycle | Record truthful direct/driver distinction and exact refusal; later explicit exception only if needed. No forged run key or completed native run. |
| Results/review evidence becomes stale after commits | Separate implementation/results/final checkpoints; re-review code fixes; external final receipts avoid self-reference. |

## Acceptance traceability

| Spec / observed failure | Implementation | Required proof |
|---|---|---|
| Native parent/child route; all registered roles | Tasks 5, 8 | Inventory-derived generated route mutations; track N actual native lineage |
| Explicit feature binding, no startup equality | Tasks 1, 5 | Primary/feature entry; foreign/root/ref/head/path/replacement refusals; active edit/commit succeeds |
| Parent-only planner payload and missing references | Task 3 | Child-readable complete resource graph, nested missing-resource/hash mutations |
| Non-executable checker invoked directly | Tasks 1, 4 | Exact interpreter argv negative case; shipped compiled checker positive case |
| Lost shell variables | Task 4 | Fresh-shell reconstruction from pinned files |
| Omitted child capability / wrong scope task identity | Task 2 | Actual grant validation before dispatch; `1` versus `task-1`; zero drives on refusal |
| Parent secret separation / each agent's catalog | Tasks 2, 5, 8 | Strict DTO/leak tests; actual child binary/catalog receipts |
| Top-level versus nested drive response | Task 4 | Actual GateDriveResult fixtures and mutation restoring old shape |
| First response lost on error | Task 4 | FAILED+nonzero and invalid JSON preserve stdout/stderr/rc; exactly one call |
| Correct run-root/epoch/predecessor | Tasks 2, 4, 6 | Terminal canonical root, WAITING omission, stale/omitted epoch/owner/predecessor tests |
| WAITING, takeover, escalation and final acknowledgement | Task 6 | Independent scenario tests and explicit live-coverage limits |
| Hidden results-template discovery | Tasks 3, 6 | Exact packaged hidden path accepted; absent/modified/redirected template fails before work |
| Feature-only plan/results diagnostics | Task 7 | Both fields, both active states, stack and integration tests; no global suppression |
| Stale local metadata ref | Task 7 | New authoritative status revision with intentionally stale local ref |
| Missing final results checkpoint | Tasks 6, 8; tracks S/N | Actual committed/published/attached artifact plus separate gate at clean final HEAD |
| Native read-only review / stale inputs | Task 6; track N | Pinned review mutation and actual production review-role dispatch |
| Clean snapshots overclaim isolation | Task 8 | Write+restore escapes comparison; audit remains explicitly incomplete |
| Full configured suite/budgets | Track S | Source Go runner complete pass and budget assessment at final relevant HEAD |
| Generated production assets, not frozen POC | Task 8; track N | Clean source/binary/asset hashes, actual loaded definitions; independent review |
| 425 before separate groomed 424 / later 426 | Track D; scope limits | No 424 mutation during planning/code; later supported stack conversion and separate PR |
| ADR successor and truthful bootstrap PR | Task 8; track S | Official successor receipt; real source PR/current evidence; no simulated native completion |

## Self-review and plan handoff

The implementation scope is tasks 1–8; the two new CLI operations validate data only. No model registry, auto pin repair, startup relocation, new launch transport, custom gate state machine, global candidate install or 424 dependency edit is included. Resource semantic selection remains the controller's existing judgment, with explicit completeness data and refusal rules. Candidate definition loading is an external launch prerequisite, not architecture deferred to the coding agent. The validator's resolver exception is tied to existing finalize authority; ordinary workers/planners/reviewers cannot select it to bypass their entry checks.

Before committing this plan: run launch-kit integrity preflight, compare live spec hash, inspect all task/interface names and acceptance rows, verify no unresolved planning markers, stamp the backlink with catalog-resolved `artifact.backlink`, and stage only this file. Commit with trailer `Docket-Plan-Path: docs/superpowers/plans/2026-09-14-native-codex-dispatch-0425.md`. Verify the single-file commit, actual blob, full HEAD and clean feature tree, then attach that commit/path to 425 at its fresh entity version. Known feature-only lookup findings are recorded; no plan copy into primary and no fabricated attachment.

Write external `evidence/planning.json` with schema 1, phase planning, status planned only after actual attachment, exact worktree/branch/base/full plan commit/path, plan/spec SHA-256, attachment metadata revision and remaining launch prerequisites. Add `planning-report.md`. Stop there. The next user task starts phase 2 at **Sol/low**.
