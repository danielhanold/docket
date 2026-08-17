<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0315 — Claim-to-implemented agent workflow](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0315-claim-to-implemented-workflow.md)**
<!-- docket:backlink:end -->

# Claim-to-Implemented Agent Workflow — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Compose the landed Go packages into the claim→implemented half of the agent lifecycle: an authoritative implementation-context read, typed claim/lease/reconcile/attach/workspace/evidence/PR/mark-implemented/run-verify operations at the app+CLI layers, revised Go-v1 skill assets that sequence them, regenerated embedded assets, and hermetic tests proving the end-to-end path.

**Architecture:** `internal/cli` gains thin Cobra commands (no policy) delegating to new `internal/app` operation files; every metadata mutation goes through the landed `repository/transaction.Engine` with exact entity versions; workspace/gate/evidence/GitHub mechanics stay in their landed packages (`internal/workspace`, `internal/process`, `internal/evidence`, `internal/githubcli`) and are only *wired*, never forked. Claude-facing skills (`skills/docket-implement-next`, `skills/docket-build` gate portion, `agents/docket-plan-writer.md`) are revised to invoke these operations; `go generate ./internal/assets/` regenerates the embedded copies in the same task that edits the sources.

**Tech Stack:** Go (Cobra CLI, protocol-v1 JSON one-shot results), real-git integration tests with disposable repos + bare remotes, protocol-faithful fake `gh`, bash suite runner `scripts/run-tests.sh`.

**Spec:** `docs/superpowers/specs/2026-08-17-claim-to-implemented-workflow-design.md` (synchronized copy in the metadata worktree; the spec is the authority — this plan argues from it).

## Global Constraints

Copied from the spec; every task's requirements implicitly include these.

- Operation spellings are fixed (spec "Package and command boundaries"): `context implementation`, `change claim|refresh-claim|reconcile|attach-plan|attach-results|mark-implemented`, `artifact backlink`, `workspace prepare|inspect|publish`, `evidence record|verify`, `pr publish`, `run verify`. Flag names as listed there; request files/stdin for authored Markdown, never shell-escaped flags.
- `internal/cli` contains **no** lifecycle, Git, document, gate, evidence, or GitHub policy — parse, delegate, present exactly one protocol-v1 result. `internal/app` assembles inputs, verifies cross-package preconditions, calls one coarse operation, maps closed outcomes; it does not dispatch agents or retain workflow state.
- Every metadata mutation: capability preflight before the transaction, exact entity version from the last authoritative operation, atomic rendering of all affected v1-owned derived views (artifact block, backlink, inline board), whole-repository validation, lease push. Semantic retry reloads fresh origin; never blindly reuses a stale patch; incompatible divergence is `contended`, never a text-merge.
- No force pushes, no duplicate PR creation, no compensating deletes on `unknown`. An unverified external effect is `unknown` and must be reprobed.
- Redaction: logs/protocol results never carry credentials, env values, credentialed remote URLs, unbounded stderr, authored document bodies, or PR body bytes. Any byte-bounded truncation backs off to a rune boundary (learning `byte-limited-truncation-splits-runes`).
- Managed blocks are rewritten only after full marker order+balance validation; malformed blocks fail without mutation (AGENTS.md rule).
- Paths crossing the CLI are canonical repository-relative; symlink escapes, absolute artifact paths, and out-of-root paths are rejected. Authored Markdown inputs are size-bounded.
- Child-agent returns are hints; every plan/build/review/PR claim is verified from Git/documents/gate state/evidence/metadata/GitHub before the next durable transition.
- Nothing from 0316–0318 (no reclaim, merge, archive, cleanup, halted record, release, cutover) and no re-implementation of 0305–0314 mechanics.
- Mutation-test every new guard (AGENTS.md "Guards and tests") and defeat Go's test cache when doing so: `go test -count=1` (learning `cached-runner-serves-a-mutated-tree`).
- The build gate runs the whole suite via `scripts/run-tests.sh` (resolved `finalize.test_command`); Go tests wire in through `tests/test_go_toolchain.sh` — no new suite entry point is needed for Go-only tests, but any **new** `tests/test_*.sh` file needs a `tests/runtime-budgets.tsv` row.
- Skill prose ships into consuming repos: no sentence keyed to this repo's shape (learning `distributed-body-has-no-local-repo`); every prohibition added to a closed-vocabulary contract names the return value it maps to (learning `prohibition-needs-a-return-value`).

## File Structure

New app files (one operation family per file, following `internal/app/change_lifecycle.go` conventions):

- `internal/app/implementation_context.go` (+`_test.go`) — `ContextImplementation` bundle assembly.
- `internal/app/change_claim.go` (+`_test.go`) — claim + refresh-claim transactions.
- `internal/app/change_reconcile.go` (+`_test.go`) — reconciliation transaction.
- `internal/app/artifact_backlink.go` (+`_test.go`) — deterministic backlink render into a feature-tree artifact.
- `internal/app/change_attach.go` (+`_test.go`, `change_attach_git_test.go`) — attach-plan / attach-results with Git verification.
- `internal/app/workspace_ops.go` (+`_test.go`, `workspace_ops_git_test.go`) — prepare/inspect/publish wiring.
- `internal/app/evidence_ops.go` (+`_test.go`) — evidence record/verify wiring.
- `internal/app/pr_publish.go` (+`_test.go`) — body assembly + `EnsurePullRequest` wiring.
- `internal/app/change_implemented.go` (+`_test.go`) — mark-implemented reprobe + transaction.
- `internal/app/run_verify.go` (+`_test.go`) — read-only postcondition report.
- `internal/app/workflow_e2e_test.go` — hermetic end-to-end workflow test.

New CLI files: `internal/cli/context.go`, `internal/cli/artifact.go`, `internal/cli/workspace.go`, `internal/cli/evidence.go`, `internal/cli/pr.go`, `internal/cli/run.go` (+ `_test.go` each); `internal/cli/change.go` gains the new `change` subcommands; `internal/cli/root.go` registers the new top-level commands.

Asset revisions: `skills/docket-implement-next/SKILL.md`, `skills/docket-build/SKILL.md` (gate portion), `agents/docket-plan-writer.md`, plus regenerated `internal/assets/embedded/` in the same commits.

---

### Task 1: Implementation context bundle (`docket context implementation`)

**Files:**
- Create: `internal/app/implementation_context.go`, `internal/app/implementation_context_test.go`
- Create: `internal/cli/context.go`, `internal/cli/context_test.go`
- Modify: `internal/cli/root.go` (register `contextCmd` in the `root.AddCommand(...)` call; the file's registration block currently ends `root.AddCommand(versionCmd, statusCmd, changeCmd, learningCmd, adrCmd, gateCmd, diagnosticCmd, installCmd, developmentCmd)`)

**Interfaces:**
- Consumes: `app.StatusReader.PinContext/ReadCorpus` (`internal/app/status.go`), `domain.SelectQueue`, `domain.EvaluateReadiness`, `domain.ClaimEligibility`, `domain.ResolveEffectiveBase`, `domain.NewBranchFacts`.
- Produces (later tasks and the skills rely on these exact names):

```go
// ImplementationContextRequest: ID==0 means "apply the landed selection policy".
type ImplementationContextRequest struct{ ID int }

type ImplementationContext struct {
    MetadataRef    string
    MetadataCommit string
    Change         ContextEntity   // path, source bytes (base64 in JSON), parsed summary, entity version
    Spec           ContextEntity
    Related        []ContextChangeSummary // deps, stack, related
    Readiness      string           // closed outcome token
    ClaimEligible  bool
    ClaimRefusal   string           // stable reason when not eligible
    EffectiveBase  ContextBase      // kind, base branch, source change id
    ADRs           []ContextADREntry
    Learnings      []ContextLearningEntry // or CapabilityWarnings
    Workflow       ContextWorkflow  // repo mode, integration branch, test command, remote, feature branch
    Warnings       []string
}

func ContextImplementation(ctx context.Context, deps PlanningDeps, repoDir string, req ImplementationContextRequest) ImplementationContextResult
```

`ImplementationContextResult` follows the `ChangeLifecycleResult` pattern (embeds `Result`, has `HumanText()`, one protocol-v1 JSON document in JSON mode).

**Requirements (spec "Authoritative implementation context"):** read-only; all facts from ONE `StatusReader` pin/corpus read — never re-read between selection and report; exact loss-preserving source bytes for change and spec (no summarizing); typed outcomes for no-candidate, absent id, malformed, duplicate id (ambiguous — never choose), not-ready, unsupported-config, unresolved effective base; nil collections normalized; learnings entries only when the capability is enabled, else an explicit warning.

- [ ] **Step 1: Write failing unit tests** in `internal/app/implementation_context_test.go` against a fake `StatusReader` (reuse the existing fake-reader pattern from `internal/app/status_test.go`). Name and pin at minimum:
  - `TestContextImplementationSelectsByPolicy` — no `--id`: the bundle's change equals `domain.SelectQueue`'s first build-ready candidate; `MetadataCommit` equals the pin's commit; change+spec source bytes are byte-identical to the corpus fixtures (compare `[]byte` exactly, including trailing newline).
  - `TestContextImplementationExplicitID` — `ID` set: that change is returned even when not first in queue (attributed-retry support).
  - `TestContextImplementationRevisionConsistency` — every fact (versions, metadata commit, readiness) derives from one snapshot: fake reader counts `PinContext` calls; assert exactly 1.
  - `TestContextImplementationTypedAbsence` — table over: unknown id, duplicate id (two records, same id — assert an `ambiguous` reason, not a pick), missing spec file (typed missing-artifact, not silent omission), not-ready change, unresolved effective base, no candidate at all. Each row asserts the closed outcome token and that no bundle is fabricated.
  - `TestContextImplementationLearningsCapability` — learnings disabled ⇒ `Warnings` names the capability, `Learnings` empty; enabled ⇒ index entries present.
  - `TestContextImplementationDoesNotEchoAuthoredBody` — the protocol JSON carries the source bytes field but `HumanText()` never includes authored document bodies (redaction constraint).
- [ ] **Step 2: Run to verify red:** `go test ./internal/app/ -run TestContextImplementation -count=1` — expect compile failure (`ContextImplementation` undefined).
- [ ] **Step 3: Implement** `internal/app/implementation_context.go`. Pin once, read corpus once, build `domain.Snapshot` (reuse the snapshot-building helper the status path uses — grep `internal/app/status.go` for where `ReadCorpus` output becomes `domain.Snapshot`), compute `BranchFacts` from the pin's remote-branch facts, then: selection (`SelectQueue` with the build-ready filter) or explicit-id lookup; `EvaluateReadiness`, `ClaimEligibility`, `ResolveEffectiveBase`; assemble the bundle. Map failures through the existing `planningError` taxonomy.
- [ ] **Step 4: Green:** `go test ./internal/app/ -run TestContextImplementation -count=1`.
- [ ] **Step 5: CLI.** Write failing `internal/cli/context_test.go` (follow `internal/cli/change_test.go` conventions): `docket context implementation --json` emits exactly one protocol-v1 document; `--id 7` routes the id; human mode prints `HumanText()`. Implement `internal/cli/context.go` with `newContextCommand(setResult ...)` mirroring `newChangeCommand`; register in `root.go`. Green.
- [ ] **Step 6: Run the package suites:** `go test ./internal/app/ ./internal/cli/ -count=1`; `gofmt -l internal/ | (! grep .)`; `go vet ./...`.
- [ ] **Step 7: Commit:** `git add internal/app/implementation_context.go internal/app/implementation_context_test.go internal/cli/context.go internal/cli/context_test.go internal/cli/root.go && git commit -m "feat(0315): authoritative implementation context bundle"`

### Task 2: Claim and refresh-claim transactions

**Files:**
- Create: `internal/app/change_claim.go`, `internal/app/change_claim_test.go`
- Modify: `internal/cli/change.go` (add `claim` and `refresh-claim` subcommands via the existing `changeSubcommand` helper), `internal/cli/change_test.go`

**Interfaces:**
- Consumes: `transaction.Engine.Execute`, `domain.Claim(c, now)`, `domain.RefreshClaim(c, now)`, `domain.ClaimEligibility`, `domain.ResolveEffectiveBase`, `transaction.Clock` (never `time.Now` — `PlanningDeps.Clock` is the sole time source).
- Produces:

```go
type ChangeClaimRequest struct{ ID int; Version string }
func ChangeClaim(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeClaimRequest) ChangeClaimResult
func ChangeRefreshClaim(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeClaimRequest) ChangeClaimResult
```

`ChangeClaimResult` carries: committed metadata revision, new entity version, feature branch, claim timestamp, lease classification (`domain.EvaluateLease`), closed disposition (`applied` / `already-claimed` / `contended` / policy refusal reason).

**Requirements (spec "Claim and lease contract"):** the transaction reloads fresh origin state and re-proves proposed + unique + build-ready + resolved-base before applying `domain.Claim`; change record, artifact block, and inline board update together; refusal/contention writes nothing and creates no workspace; response-loss retry re-reads authority — an already-held matching claim returns the applied receipt or typed `already-claimed`, a foreign claim returns `contended`; refresh requires `in-progress` + exact version + existing claim identity and re-stamps only `claimed_at` + derived views. Never clears or steals an expired claim (0316).

- [ ] **Step 1: Failing tests.** Model on `internal/app/change_lifecycle_test.go` (fake engine + fake reader). Pin:
  - `TestChangeClaimApplies` — proposed build-ready change: the submitted `transaction.Request` carries the exact expected version, patches `status: in-progress` + `claimed_at` + `branch`, and names the change file, board, and artifact surfaces; result echoes new version + lease class.
  - `TestChangeClaimRefusals` — table: wrong version ⇒ `contended`; not proposed; not build-ready; duplicate id; unresolved base — each a typed refusal, engine sees either no request or a request whose in-transaction re-proof fails (per the engine's validator seam), and NO other field is touched.
  - `TestChangeClaimRetryConvergence` — retry with same request against a snapshot already claimed with matching identity ⇒ `already-claimed` (not `contended`, not a second write); claimed by another identity ⇒ `contended`.
  - `TestChangeRefreshClaimStampsOnly` — the patch touches `claimed_at` (+ `updated`) and nothing else; requires `in-progress`; version mismatch ⇒ `contended` and the caller-visible instruction is stop-don't-overwrite.
  - Mutation check (run manually, note in commit): strip the in-transaction re-proof, `go test -count=1` must redden `TestChangeClaimRefusals`.
- [ ] **Step 2: Red:** `go test ./internal/app/ -run 'TestChangeClaim|TestChangeRefreshClaim' -count=1`.
- [ ] **Step 3: Implement** `change_claim.go` as a `transaction` operation (follow the `changeLifecycleOp` pattern: `Key()`, validation closure, patch closure, receipt decode). Preflight capability + board fence via the shared plumbing in `internal/app/planning.go`.
- [ ] **Step 4: Green**, then CLI: failing tests then implementation for `docket change claim --id N --version V` and `docket change refresh-claim --id N --version V` via `changeSubcommand`. Green.
- [ ] **Step 5: Commit:** `git add internal/app/change_claim.go internal/app/change_claim_test.go internal/cli/change.go internal/cli/change_test.go && git commit -m "feat(0315): exact-version claim and refresh-claim transactions"`

### Task 3: Reconciliation transaction (`change reconcile`)

**Files:**
- Create: `internal/app/change_reconcile.go`, `internal/app/change_reconcile_test.go`
- Modify: `internal/cli/change.go`, `internal/cli/change_test.go` (subcommand `reconcile` with `--input <request-file>` decoded via the existing `decodeRequestFlag`)

**Interfaces:**
- Produces:

```go
type ChangeReconcileRequest struct {
    ID int; Version string
    Sections     map[string]string // owned proposal sections by canonical heading, full replacement text
    SpecSections map[string]string // still-mutable linked-spec content
    Relations    *DesiredRelations // complete desired values: DependsOn, StackedOn, Related, ADRs, DiscoveredFrom
    ReconcileLogEntry string       // authored Markdown, dated entry body (required)
}
func ChangeReconcile(ctx context.Context, deps PlanningDeps, repoDir string, req ChangeReconcileRequest) ChangeReconcileResult
```

**Requirements (spec "Reconciliation transaction"):** structured edits, never whole-record replacement; sets `reconciled: true` and refreshes the claim in the same transaction; unknown frontmatter, unlisted authored sections, line endings, unrelated files stay **byte-identical**; validates edits stay within operation-owned fields/sections; rerenders backlinks/artifact block/board; whole-repo validation; incompatible fresh state ⇒ `contended`, never a text-merge. Authored Markdown is size-bounded.

- [ ] **Step 1: Failing tests.** Pin:
  - `TestChangeReconcileAppliesPatch` — sections replaced, one dated log entry appended under `## Reconcile log`, `reconciled: true`, `claimed_at` restamped, relations written as flow collections (unquoted — AGENTS.md YAML rule), everything else byte-identical (assert on full output bytes of an untouched sibling section and an unknown frontmatter key).
  - `TestChangeReconcileOwnedFieldFence` — a request naming a non-owned field/section (e.g. `status`, `id`, an `## Artifacts` managed block) is a typed validation refusal with no write.
  - `TestChangeReconcileContention` — stale version / status no longer `in-progress` ⇒ `contended`, nothing written.
  - `TestChangeReconcileGuardsRedden` — **mutation-style table** (spec Testing bullet 4): for each guarded field, owned section, and marker shape, corrupt the fixture (dangling `docket:artifacts` marker; out-of-order markers; missing `## Reconcile log` terminator) and assert the operation refuses without mutation. Use a named terminator when slicing sections, and assert the terminator exists (learning `section-slice-needs-a-named-terminator`).
  - `TestChangeReconcileBoundsInput` — oversized `ReconcileLogEntry` (pick the codebase's existing authored-input bound; grep `internal/app` for the size-bound constant used by 0312 mutations and reuse it) ⇒ typed refusal.
- [ ] **Step 2: Red.** `go test ./internal/app/ -run TestChangeReconcile -count=1`
- [ ] **Step 3: Implement**, reusing `internal/document`'s loss-preserving parse/serialize and the 0312 mutation layer's section-patch helpers (grep `internal/app/change_groom.go` — grooming already patches sections + relations; follow it, do not fork it).
- [ ] **Step 4: Green; CLI subcommand with `--input`; green; commit:** `git add internal/app/change_reconcile.go internal/app/change_reconcile_test.go internal/cli/change.go internal/cli/change_test.go && git commit -m "feat(0315): authored reconciliation transaction"`

### Task 4: `artifact backlink` command

**Files:**
- Create: `internal/app/artifact_backlink.go`, `internal/app/artifact_backlink_test.go`, `internal/cli/artifact.go`, `internal/cli/artifact_test.go`
- Modify: `internal/cli/root.go` (register `artifactCmd`)

**Interfaces:**
- Consumes: `render.BacklinkContent(c domain.Change, link render.LinkContext)` (`internal/render/artifacts.go`), `internal/document` managed-block rewrite with marker order+balance validation.
- Produces: `func ArtifactBacklink(ctx context.Context, deps PlanningDeps, repoDir string, req ArtifactBacklinkRequest) ArtifactBacklinkResult` with `ArtifactBacklinkRequest{ArtifactPath, ChangePath string}` — both canonical repo-relative; writes the rendered backlink block into the artifact file **in the feature worktree** (this is the one operation that writes a working-tree file, not a metadata transaction — the plan-writer commits it).

**Requirements:** rejects absolute paths, `..`/symlink escapes, paths outside their allowed roots; refuses on dangling/out-of-order/nested backlink markers, leaving the file untouched; idempotent (re-run yields byte-identical file); inserts markers when absent at the canonical position (top of file, matching the existing rendered shape — see the `docket:backlink` block at the head of the spec file itself for the target shape).

- [ ] **Step 1: Failing tests:** `TestArtifactBacklinkRendersBlock` (fresh file gains the block; content matches `render.BacklinkContent` golden), `TestArtifactBacklinkIdempotent` (second run byte-identical), `TestArtifactBacklinkRefusesMalformedMarkers` (dangling start marker ⇒ typed refusal, file bytes unchanged — assert bytes), `TestArtifactBacklinkPathContainment` (absolute path, `../` escape, symlink pointing outside the repo ⇒ refusal; canonicalise every hop — learning `canonicalise-every-symlink-hop`).
- [ ] **Step 2: Red; implement; green** (`go test ./internal/app/ -run TestArtifactBacklink -count=1`).
- [ ] **Step 3: CLI:** `docket artifact backlink --artifact <path> --change <path>`; test JSON single-document + human line; register in root. Green.
- [ ] **Step 4: Commit:** `git add internal/app/artifact_backlink.go internal/app/artifact_backlink_test.go internal/cli/artifact.go internal/cli/artifact_test.go internal/cli/root.go && git commit -m "feat(0315): deterministic artifact backlink renderer command"`

### Task 5: Workspace prepare / inspect / publish wiring

**Files:**
- Create: `internal/app/workspace_ops.go`, `internal/app/workspace_ops_test.go`, `internal/app/workspace_ops_git_test.go`, `internal/cli/workspace.go`, `internal/cli/workspace_test.go`
- Modify: `internal/cli/root.go`

**Interfaces:**
- Consumes: `workspace.NewService(git)`, `Service.Prepare(ctx, PrepareRequest{Repository, Remote, Target})`, `Service.Inspect(ctx, InspectRequest{Repository, Target})`, `Service.PublishHead(ctx, PublishRequest{Repository, Remote, Target})`, `workspace.NewTarget(changeID, slug, effectiveBase)`, `domain.ResolveEffectiveBase`.
- Produces:

```go
func WorkspacePrepare(ctx context.Context, deps PlanningDeps, repoDir string, req WorkspaceIDRequest) WorkspaceOpResult   // req: ID, Version
func WorkspaceInspect(ctx context.Context, deps PlanningDeps, repoDir string, req WorkspaceIDRequest) WorkspaceOpResult   // req: ID
func WorkspacePublish(ctx context.Context, deps PlanningDeps, repoDir string, req WorkspacePublishRequest) WorkspaceOpResult // req: ID, Head
```

`WorkspaceOpResult` reports the canonical workspace path, exact base commit, feature ref, head, and the closed disposition/state kind straight from the service — the app layer adds NO base selection or stack policy: it reloads the authoritative snapshot, resolves the effective base through `domain.ResolveEffectiveBase`, builds the `workspace.Target`, and delegates.

**Requirements:** `workspace prepare` requires the change to be `in-progress` at the exact `--version` (claim happens first — a losing claimant creates no workspace); `workspace publish` takes the expected head and refuses when the reinspected head differs; `contended`/`unknown` dispositions pass through verbatim (no force, no retry-with-force).

- [ ] **Step 1: Failing unit tests** (fake reader; a workspace-service seam interface in `PlanningDeps`-adjacent deps so unit tests inject a fake — add `type WorkspaceService interface { Prepare(...); Inspect(...); PublishHead(...) }` in `workspace_ops.go` and accept it via a `WorkspaceDeps` struct): `TestWorkspacePrepareResolvesBaseFromDomain` (the `Target.Base` handed to the service equals `ResolveEffectiveBase`'s answer — mutation: hard-code `main`, test reddens for a stacked fixture), `TestWorkspacePrepareRequiresClaimedVersion` (proposed change or stale version ⇒ typed refusal, service never called), `TestWorkspacePublishHeadMismatch` (expected head ≠ reinspected head ⇒ typed refusal, no publish call), `TestWorkspacePublishPassesThroughDispositions` (table: published/already-published/contended/unknown map 1:1 to protocol outcomes).
- [ ] **Step 2: Red; implement; green.**
- [ ] **Step 3: Real-git integration test** `workspace_ops_git_test.go` (copy the harness setup from `internal/workspace/harness_test.go` / `internal/app/planning_git_test.go`: disposable repo + bare remote): prepare a fresh workspace at a resolved base and assert `StateReady`; resume the same workspace (second Prepare ⇒ existing disposition); publish an advanced head to the bare remote and assert the remote ref equals the exact head; diverge the remote first and assert `contended` with the remote untouched.
- [ ] **Step 4: CLI commands** (`docket workspace prepare|inspect|publish`); tests; register; green: `go test ./internal/app/ ./internal/cli/ -count=1`.
- [ ] **Step 5: Commit:** `git add internal/app/workspace_ops.go internal/app/workspace_ops_test.go internal/app/workspace_ops_git_test.go internal/cli/workspace.go internal/cli/workspace_test.go internal/cli/root.go && git commit -m "feat(0315): workspace prepare/inspect/publish operations"`

### Task 6: Evidence record and verify

**Files:**
- Create: `internal/app/evidence_ops.go`, `internal/app/evidence_ops_test.go`, `internal/cli/evidence.go`, `internal/cli/evidence_test.go`
- Modify: `internal/cli/root.go`

**Interfaces:**
- Consumes: `process` service `Observe(runDir)` (terminal state + exact command), workspace inspection for the current feature head, `evidence.NewRecord(command, head, ranAt)`, `evidence.Render(r)`, `evidence.Verify(body, head)`.
- Produces:

```go
func EvidenceRecord(ctx context.Context, deps ..., req EvidenceRecordRequest) EvidenceOpResult // req: ID, RunDir (absolute), Head
func EvidenceVerify(ctx context.Context, deps ..., req EvidenceVerifyRequest) EvidenceOpResult // req: RecordFile (request file bytes), Head
```

`EvidenceRecord` returns the immutable typed record and canonical rendered block (exact command, commit, timestamp, outcome). No second evidence store — the block travels as bytes; the PR body becomes the durable record after `pr publish`.

**Requirements (spec "Workspace, build, gate, and evidence sequence"):** evidence is created **only** from a `passed` terminal observation whose head equals the current feature head; failed/running/stopped/vanished/malformed/mismatched runs produce no evidence — and a probe error is a distinct typed failure, never folded into "no run there" (learning `probe-error-is-not-clean-absence`); the recorded command is the observed gate command, never an agent-supplied string; there is no agent-supplied `passed` boolean anywhere in the request shapes.

- [ ] **Step 1: Failing tests.** Fixtures: real run dirs written by the landed process package's record writer (reuse its test helpers — grep `internal/process/records_test.go` for the terminal-record fixture constructor). Pin: `TestEvidenceRecordFromPassedRun` (green terminal + matching head ⇒ record with exact command/head, `Render` block round-trips through `evidence.Extract`), `TestEvidenceRecordRefusals` (table: failed run, still-running lock, vanished dir, malformed terminal.json, head mismatch — each a distinct stable reason; unreadable run dir (chmod 000) is its own error, not "no evidence"), `TestEvidenceVerifyHeadPin` (`Verify` green for the exact head, red after head changes — this is the invalidate-on-fix property).
- [ ] **Step 2: Red; implement; green** (`-count=1`).
- [ ] **Step 3: CLI** (`docket evidence record --id N --run <abs> --head <oid>`, `docket evidence verify --record <file> --head <oid>`); tests; register; green.
- [ ] **Step 4: Commit:** `git add internal/app/evidence_ops.go internal/app/evidence_ops_test.go internal/cli/evidence.go internal/cli/evidence_test.go internal/cli/root.go && git commit -m "feat(0315): gate-derived evidence record and verify operations"`

### Task 7: Plan and results attachment (`change attach-plan`, `change attach-results`)

**Files:**
- Create: `internal/app/change_attach.go`, `internal/app/change_attach_test.go`, `internal/app/change_attach_git_test.go`
- Modify: `internal/cli/change.go`, `internal/cli/change_test.go`

**Interfaces:**
- Consumes: `gitcli.Client` (feature-workspace reads: head, ancestry, commit delta, trailer, tracked-file check), transaction engine, `workspace` inspection for the owned checkout.
- Produces:

```go
type ChangeAttachRequest struct{ ID int; Version string; Path string; Commit string }
func ChangeAttachPlan(ctx context.Context, deps ..., req ChangeAttachRequest) ChangeAttachResult
func ChangeAttachResults(ctx context.Context, deps ..., req ChangeAttachRequest) ChangeAttachResult
```

**Requirements (spec "Plan and results attachment") — pre-transaction Git verification, from Git, never from the child return:** commit is current feature head AND descends from the prepared base; path canonical repo-relative inside the allowed planning directory (plans: `docs/superpowers/plans/`; results: the configured results root — read both from the resolved workflow config, not hard-coded), regular tracked file, no symlink; the writer commit has the ADR-0094 single-artifact delta (exactly the plan file, `--no-renames` when diffing — learning `diff-derived-allowlist-needs-no-renames`) and the `Docket-Plan-Path:` trailer; managed backlink balanced and targeting this change; no unresolved placeholder token (the planning contract's placeholder set — grep the docket-plan-writer contract for the token shape). Retry verifies the same plan identity (path + blob at the verified commit) and returns the prior applied outcome — never attaches whatever now occupies the path (learning `idempotency-keying`: key on the promised state).

- [ ] **Step 1: Failing unit tests** (transaction/patch side, fake engine): `TestChangeAttachPlanPatch` (stores `plan:` path, renders artifact block + board, exact version), `TestChangeAttachResultsPatch` (same for `results:`), `TestChangeAttachContention` (stale version ⇒ `contended`).
- [ ] **Step 2: Real-git verification tests** `change_attach_git_test.go` (disposable feature repo): table `TestChangeAttachPlanGitVerification` — happy path passes; then one row per guard, each constructed by mutating the happy fixture: commit not head; commit not descending from base; untracked file; symlinked plan; plan outside `docs/superpowers/plans/`; two-file delta; rename delta (with `--no-renames` this must still redden); missing trailer; unbalanced backlink; backlink naming another change; placeholder token present. Assert each row's **stable reason string**, not just failure (learning `assert-pins-outcome-not-mechanism`).
- [ ] **Step 3: Red; implement; green** (`go test ./internal/app/ -run TestChangeAttach -count=1`).
- [ ] **Step 4: CLI subcommands** `attach-plan` / `attach-results` (`--id --version --path --commit`); tests; green.
- [ ] **Step 5: Commit:** `git add internal/app/change_attach.go internal/app/change_attach_test.go internal/app/change_attach_git_test.go internal/cli/change.go internal/cli/change_test.go && git commit -m "feat(0315): verified plan and results attachment"`

### Task 8: PR publication (`pr publish`)

**Files:**
- Create: `internal/app/pr_publish.go`, `internal/app/pr_publish_test.go`, `internal/cli/pr.go`, `internal/cli/pr_test.go`
- Modify: `internal/cli/root.go`

**Interfaces:**
- Consumes: `githubcli.NewClient` + `Client.EnsurePullRequest(ctx, EnsurePullRequestRequest{Repository, HeadBranch, ExpectedHead, BaseBranch, Title, Body, ExpectedVersion})` and the package's fake-`gh` test harness (`internal/githubcli/fakegh_test.go` — reuse its protocol-faithful fake, do not write a second one); `evidence.Extract`/`Verify`; the evidence **upsert** helper (`internal/evidence/upsert.go`) for deterministic body-block replacement; workspace inspect for local head; render backlink block for the PR body.
- Produces: `func PRPublish(ctx context.Context, deps ..., req PRPublishRequest) PRPublishResult` — `req: ID, Head, BodyFile (authored title+body request), EvidenceFile (canonical evidence request bytes)`; result: canonical PR reference, URL, number, head, base, disposition.

**Requirements (spec "Feature publication, PR publication, and implemented transition"):** reparse the evidence bytes (never trust a prior command result) and verify local HEAD == published remote head == evidence head == requested head, feature branch and effective-base branch and repository identity agree — all BEFORE calling `EnsurePullRequest`; authored prose preserved byte-for-byte while ONLY the Docket-owned backlink and evidence blocks are inserted/replaced (via the upsert helper); adoption rules stay in the adapter — no second lookup policy; `unknown` stays `unknown`; result redacts the body bytes.

- [ ] **Step 1: Failing tests:** `TestPRPublishAgreementChecks` (table: each identity conjunct broken in turn ⇒ typed refusal with stable reason, `gh` never invoked — count fake invocations), `TestPRPublishBodyAssembly` (authored prose with pre-existing evidence block: prose preserved, block replaced deterministically, backlink inserted once; assert full body bytes), `TestPRPublishThroughFakeGH` (create + adopt + unknown dispositions surface verbatim; PR snapshot fields round-trip), `TestPRPublishRedaction` (result JSON and human text contain no body bytes).
- [ ] **Step 2: Red; implement; green** (`-count=1`).
- [ ] **Step 3: CLI** `docket pr publish --id N --head <oid> --body <file> --evidence <file>`; tests; register; green.
- [ ] **Step 4: Commit:** `git add internal/app/pr_publish.go internal/app/pr_publish_test.go internal/cli/pr.go internal/cli/pr_test.go internal/cli/root.go && git commit -m "feat(0315): probe/act/verify PR publication"`

### Task 9: `change mark-implemented`

**Files:**
- Create: `internal/app/change_implemented.go`, `internal/app/change_implemented_test.go`
- Modify: `internal/cli/change.go`, `internal/cli/change_test.go`

**Interfaces:**
- Consumes: `domain.MarkImplemented(c, ImplementedFacts)`, transaction engine, workspace inspect (local head), gitcli remote-ref read, `githubcli` PR probe, `evidence.Verify`.
- Produces: `func ChangeMarkImplemented(ctx context.Context, deps ..., req MarkImplementedRequest) ChangeLifecycleResult`-shaped result — `req: ID, Version, Head, PR (canonical reference), EvidenceFile`.

**Requirements (spec, five reprobe conjuncts):** before the transaction, reprobe and prove (1) change still exact `in-progress` version, `reconciled: true`, linked to the verified plan; (2) local and remote feature heads equal the supplied head; (3) valid evidence names that head and a passed gate; (4) exactly one verified PR for the feature branch, targeting the resolved effective-base branch, naming that head; (5) any attached results path still satisfies its recorded artifact identity. Then apply `domain.MarkImplemented` atomically with PR reference, `status: implemented`, updated date, artifact block, board, audit receipt. Does NOT clear the claim, delete branch/workspace, merge, archive, or close descendants.

- [ ] **Step 1: Failing tests:** `TestMarkImplementedApplies` (all conjuncts hold ⇒ transaction request patches status/pr/updated + surfaces; claim fields untouched — assert `claimed_at`/`branch` unchanged), `TestMarkImplementedConjuncts` (table: break each of the five conjuncts independently ⇒ typed refusal with that conjunct's stable reason, engine never called — this table is the mutation test for the reprobe; each row must be proven to redden by the row's own fixture delta), `TestMarkImplementedRetry` (already `implemented` with matching PR/head ⇒ prior applied outcome, no duplicate transition).
- [ ] **Step 2: Red; implement; green** (`-count=1`).
- [ ] **Step 3: CLI subcommand** `mark-implemented --id --version --head --pr --evidence <file>`; tests; green.
- [ ] **Step 4: Commit:** `git add internal/app/change_implemented.go internal/app/change_implemented_test.go internal/cli/change.go internal/cli/change_test.go && git commit -m "feat(0315): verified implemented transition"`

### Task 10: `run verify --id`

**Files:**
- Create: `internal/app/run_verify.go`, `internal/app/run_verify_test.go`, `internal/cli/run.go`, `internal/cli/run_test.go`
- Modify: `internal/cli/root.go`

**Interfaces:**
- Produces: `func RunVerify(ctx context.Context, deps ..., req RunVerifyRequest) RunVerifyResult` — read-only; closed verdicts `run-complete` / `run-unclaimed` / `run-incomplete`; result enumerates every unmet conjunct as `{Reason string; Observed string}` pairs; human mode prints ONE report line (`run-complete`, etc.) plus detail; JSON carries typed fields. Automation keys on the verdict field, never the exit code — the command exits 0 for all three verdicts (learning `exit-code-encodes-a-non-failure`: `run-incomplete` is a report, not a process failure; only operational errors exit non-zero).

- [ ] **Step 1: Failing tests:** `TestRunVerifyComplete` (an `implemented` fixture satisfying all postconditions ⇒ `run-complete`), `TestRunVerifyUnclaimed` (proposed, no claim ⇒ `run-unclaimed`), `TestRunVerifyIncompleteEnumeratesConjuncts` — the spec's own testing rule: mutate or remove each promised postcondition (missing plan link, plan file gone at recorded path, feature head ≠ remote, stale/absent evidence, PR absent/mismatched, results identity broken, lease contended) and expect `run-incomplete` carrying that conjunct's stable reason; assert the FULL reason list, not just non-emptiness. `TestRunVerifyExitCodeContract` (all three verdicts exit 0 in CLI test).
- [ ] **Step 2: Red; implement; green; CLI (`docket run verify --id N`); register; green.**
- [ ] **Step 3: Commit:** `git add internal/app/run_verify.go internal/app/run_verify_test.go internal/cli/run.go internal/cli/run_test.go internal/cli/root.go && git commit -m "feat(0315): read-only run verification report"`

### Task 11: Real-git transaction integration tests (both metadata modes)

**Files:**
- Create: `internal/app/claim_workflow_git_test.go`

**Interfaces:** consumes only Tasks 1–3, 7, 9 public functions; produces the race/retry coverage the spec's "Real-Git integration tests" section demands.

Harness: disposable repo + real bare remotes in **both** `main` and `docket` metadata modes — copy the dual-mode fixture constructor from `internal/app/planning_git_test.go` / `internal/app/status_git_test.go` (do not invent a third harness). Independent-writer races must actually **diverge the contended path** on the remote between context and mutation (learning `green-suite-untested-branch`), and post-rejection re-reads must come from fresh origin, never the local tree (learning `cas-re-read-fresh-origin`).

- [ ] **Step 1: Write failing tests:**
  - `TestClaimRaceLosesCleanly` — two claimants from the same context version; the second pushes first via a side clone; the first's claim returns `contended`, the remote holds exactly one claim, and the loser created no workspace.
  - `TestClaimRetryAfterLostResponse` — claim applies, response discarded, same request re-run ⇒ `already-claimed`/applied receipt, exactly one claim commit on the metadata remote (`git rev-list --count`).
  - `TestReconcileIndependentWriterWins` — an independent writer commits a conflicting change-file edit to origin between context and reconcile ⇒ `contended`, origin bytes preserved.
  - `TestAttachPlanBoardLinkAtomicity` — attach-plan's metadata commit updates change file + board + artifact block in ONE commit (inspect the commit's path set).
  - `TestEffectiveBaseConsumedFromDomain` — a stacked change's workspace prepare + attach ancestry checks run against the parent branch base, in both modes.
  - Run each in both metadata modes via the harness's mode table.
- [ ] **Step 2: Red where the behavior is missing, green after wiring fixes; run:** `go test ./internal/app/ -run 'Git' -count=1` (these are the slow tests — keep them in the existing `-short`-guard style if the harness uses one; match `planning_git_test.go`).
- [ ] **Step 3: Commit:** `git add internal/app/claim_workflow_git_test.go && git commit -m "test(0315): real-git claim/reconcile/attach race and retry coverage"`

### Task 12: Hermetic end-to-end workflow test

**Files:**
- Create: `internal/app/workflow_e2e_test.go`

**Interfaces:** consumes every operation from Tasks 1–10 in spec order; produces acceptance-criterion 1 and 6 coverage.

- [ ] **Step 1: Write the test** `TestClaimToImplementedWorkflow` (table over both metadata modes): disposable supported repo, one build-ready change, real metadata + feature bare remotes, fake `gh` on PATH (reuse the `githubcli` fake harness), a real trivially-passing gate command (e.g. a fixture script exiting 0) launched through the landed gate supervisor. Sequence exactly: `ContextImplementation` → `ChangeClaim` → `WorkspacePrepare` → deterministic fixture writes the plan + backlink (via `ArtifactBacklink`) + commits with the ADR-0094 trailer → `ChangeAttachPlan` → fixture implementation commit → gate launch/observe to `passed` → `EvidenceRecord` → `EvidenceVerify` → `WorkspacePublish` → `PRPublish` (authored fixture prose) → `ChangeMarkImplemented` → `RunVerify` ⇒ `run-complete`. Then the negative halves: assert **no direct metadata write** occurred outside engine commits (every metadata-remote commit message/receipt comes from the transaction engine — inspect `git log` of the metadata remote) and no legacy Bash facade was invoked (the test environment simply has none on PATH; assert the workflow needed no `$DOCKET_SCRIPTS_DIR`). Also: an actively-requested deferred capability in the fixture config ⇒ `unsupported-config` **before** any mutation (acceptance 6, second clause).
- [ ] **Step 2: Run:** `go test ./internal/app/ -run TestClaimToImplementedWorkflow -count=1 -v` — expect PASS; fix wiring gaps it exposes (this test is the integration chokepoint; fixes belong in the task that owns the broken operation).
- [ ] **Step 3: Commit:** `git add internal/app/workflow_e2e_test.go && git commit -m "test(0315): hermetic claim-to-implemented end-to-end workflow"`

### Task 13: Skill and agent asset revisions + embedded regeneration

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md`, `skills/docket-build/SKILL.md` (build-gate portion only), `agents/docket-plan-writer.md`
- Regenerate: `internal/assets/embedded/` via `go generate ./internal/assets/` (same commit — `tests/test_asset_bundle_drift.sh` reddens otherwise)

**Interfaces:** consumes the exact CLI spellings from Tasks 1–10; produces the Go-v1 sequencer prose.

**Requirements (spec "Skill and asset revisions"):**
- `docket-implement-next` becomes the sequencer for: `docket context implementation` → judgment on the bundle → `docket change claim` → `docket change reconcile` (model-authored) → `docket workspace prepare` → plan-writer dispatch → `docket change attach-plan` → refresh-claim around long phases (`docket change refresh-claim` immediately before and after plan/build dispatches; a refresh contention stops NEW dispatch but never cancels a child whose outcome must still be inspected) → build → gate → `docket evidence record`/`verify` → review/fix loop (any fix invalidates evidence ⇒ full gate + evidence again) → optional results + `attach-results` → `docket workspace publish` → authored PR prose + `docket pr publish` → `docket change mark-implemented` → `docket run verify --id`. It retains candidate judgment, reconciliation, dispatch, bounded repair, and stop/report decisions; it never edits frontmatter/managed blocks/boards, never calls git/gh for owned effects, never interprets gate state from PIDs/logs, never invokes legacy facade mutations.
- `docket-plan-writer` (`agents/docket-plan-writer.md`): replace the Bash `render-artifact-backlink` facade call with `docket artifact backlink --artifact <path> --change <path>`; preserve the pinned role, single-artifact commit + trailer, and `PLAN_PATH=` return contract unchanged.
- `docket-build` gate portion: launch/observe the native gate (`docket gate launch` / `gate observe` — landed spellings) and record exact evidence; leave TDD/dispatch/review judgment text untouched.
- Every "never do X" added to a contract with a closed return vocabulary names the return it maps to, in the clause itself. No sentence keyed to this repo's shape. Where existing tests grep the old prose (facade invocations, sequencing sentences), repoint the assert at the new canonical content — relocation, not restoration (learning `restatement-accumulates-its-own-guards`): before editing, `grep -rn` the `tests/` tree for each phrase you remove and fix the dependents in this task.

- [ ] **Step 1: Grep for dependents:** `grep -rn "render-artifact-backlink\|DOCKET_SCRIPTS_DIR" skills/ agents/ tests/ | grep -v embedded` — list every test asserting current skill prose; note which must be repointed.
- [ ] **Step 2: Edit the three assets** per the requirements above. Keep `docket-implement-next`'s existing structure (Step numbering, halt semantics) — this is a re-sequencing onto the new commands, not a rewrite of its judgment/halt policy.
- [ ] **Step 3: Regenerate:** `go generate ./internal/assets/ && git status --short` — confirm only `internal/assets/embedded/**` (+ manifest) changed alongside your edits.
- [ ] **Step 4: Run the guards:** `go test ./internal/assets/ -count=1` and `bash tests/test_asset_bundle_drift.sh`; repoint any reddened prose-sentinel tests found in Step 1; re-run those test files.
- [ ] **Step 5: Commit (sources + embedded together):** `git add skills/docket-implement-next/SKILL.md skills/docket-build/SKILL.md agents/docket-plan-writer.md internal/assets/embedded tests/ && git commit -m "feat(0315): revise implement-next/plan-writer/build-gate assets to the Go-v1 operations"`

Note (learning `generated-artifact-loaded-at-process-start`): the revised agent/skill assets cannot be behaviorally validated by this session's harness — the running process holds the old copies. Do not claim live validation; the e2e test (Task 12) plus asset-drift guards are this change's evidence, and live acceptance is 0317.

### Task 14: Whole-suite gate and budget check

**Files:**
- Modify (only if measurement demands): `tests/runtime-budgets.tsv`

- [ ] **Step 1: Full suite:** run `scripts/run-tests.sh` (background to a log with a blocking monitor — the suite runs near the 600s foreground ceiling). Key on the exit code and the summary, and treat any trailing `OVER BUDGET:` line as a finding to act on (AGENTS.md).
- [ ] **Step 2: Budget headroom.** The new Go tests (real-git integration + e2e) run inside `tests/test_go_toolchain.sh`'s `go test ./...` check, whose budget row was already near its ceiling before this change grew it substantially (learning `budget-headroom-is-spent-before-it-is-breached`). Read that file's current row in `tests/runtime-budgets.tsv`, compare against the measured wall clock from Step 1, and if headroom is thin or breached, raise the row in this change with a comment recording the measurement — do not leave the breach for the next change to diagnose.
- [ ] **Step 3: Fix anything red; re-run the affected files; then re-run the full suite once green.**
- [ ] **Step 4: Commit (if the budget row or fixes changed anything):** `git add -u tests/ && git commit -m "test(0315): suite gate — budget row for the grown Go check"`

---

## Self-Review

- **Spec coverage:** context bundle → T1; claim/lease → T2; reconcile → T3; backlink → T4; workspace → T5; evidence → T6; attach plan/results → T7; workspace publish + pr publish → T5/T8; mark-implemented → T9; run verify + resumption-by-inspection → T10 (resumption needs no new code: it is the retry/idempotency behavior pinned in T2/T7/T8/T9 and the e2e verdict in T12); unit/contract tests → per task; real-git → T5/T7/T11; gate/evidence/GitHub → T6/T8 (fake-gh matrix lives in the landed `githubcli` suite; T8 covers the app-layer dispositions); e2e both modes + unsupported-config → T12; skill/asset revisions + regeneration → T13; failure/concurrency/security rules → Global Constraints enforced in each task's refusal tables.
- **Deliberate exclusions:** no `run-halted` verdict (spec: deliberately absent), no reclaim/finalize/release work, no new evidence store, no second PR-lookup policy, no changes to finalize/auto-groom/capture assets.
- **Type consistency:** request/result names are declared in each task's Interfaces block and consumed by name in later tasks (`ImplementationContext` → T2 versions; `ChangeAttachRequest` → T7 CLI; `EnsurePullRequestRequest` fields match `internal/githubcli/ensure.go` verbatim; workspace shapes match `internal/workspace` verbatim).
- **Placeholder scan:** intentional executor judgment points are phrased as "grep X and reuse it" against named existing code, never "TBD"; no bare "add validation" steps remain — each guard is enumerated in its task's refusal table.
