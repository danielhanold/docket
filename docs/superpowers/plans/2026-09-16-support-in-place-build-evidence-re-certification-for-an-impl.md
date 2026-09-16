<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0415 — Support in-place build-evidence re-certification for an implemented change](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-16-0415-support-in-place-build-evidence-re-certification-for-an-impl.md)**
<!-- docket:backlink:end -->
# In-Place Build-Evidence Re-Certification (`evidence.recertify`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `docket evidence recertify --id <id> [--repo-dir <dir>]` — for an implemented change whose open PR received a published follow-up commit, rerun the build gate at the current head, record and verify canonical build evidence, and refresh only the existing PR's build-evidence block, leaving the change implemented throughout.

**Architecture:** A small app-layer composition (`internal/app/evidence_recertify.go`) of existing services: `loadWorkspaceContext`/`resolveWorkspaceTarget`/`Inspect` for identity, the existing production local-gate seam generalized to a BUILD owner (`NewBuildLocalGate`, reading only `build.test_command` through `NewBuildGateDriveService`), the landed `EvidenceRecord`/`evidence.Verify` path for evidence, and `FinalizePublish`'s PR-edit slice (`evidence.Upsert` + `finalizePublishEnsurer.EnsurePullRequest`) without its rebase receipt, `PublishRewrite`, or merge. One CLI leaf, one schema binding, no new subsystem.

**Tech Stack:** Go; existing packages `internal/app`, `internal/cli`, `internal/evidence`, `internal/gatedrive`, `internal/githubcli`, `internal/workspace`, `internal/config`. Tests are colocated `package app` tests reusing the real-git `rebaseFixture` and the `fakePublishGitHub`/`fakeGate` fakes.

**Spec:** `docs/superpowers/specs/2026-09-16-support-in-place-build-evidence-re-certification-for-an-impl-design.md` (change 0415; ADR-0102 governs build/finalize command independence).

## Global Constraints

- The gate runs ONLY the resolved `build.test_command` — never `finalize.test_command`, never an agent-supplied command; no finalize-command fallback (ADR-0102).
- `build.gate: off` records truthful skipped evidence with no run; an enabled gate with `build.test_command` unset refuses (`unconfigured-gate-command`).
- No new subsystem, agent, lifecycle status, config setting, evidence store, cache, recovery journal, or retry framework.
- No branch creation, commits, pushes, rebases, merges, results rewriting, or automatic fixes. The operation performs no Docket metadata mutation — the change record is never written; it stays `implemented`.
- WAITING gate slices are advanced in-process until terminal; WAITING never means success and never starts a second suite. One suite attempt per invocation; failure/halt is reported, never auto-repaired.
- A changed head, changed build configuration, dirty worktree, non-implemented status, or closed/mismatched PR can never publish successful evidence; a failed or uncertain PR edit is never reported as completion.
- Preserve evidence staleness rules, evidence rendering (`evidence.Upsert` stays green-only), all finalize behavior, and the deferred results-only-delta optimization (not implemented, not touched).
- Keep new code to the entry point plus composition glue; extract a shared helper only where reuse requires it.
- Cross-references in maintained source anchor on symbol names or quoted clauses, never line numbers (ADR-0054). Re-runs of Go tests during verification use `-count=1` (result cache).

## File Structure

- Create `internal/app/evidence_recertify.go` — operation key, request/result/reason types, `EvidenceRecertify` composition, the `recertifyProbe` precondition/identity helper (used for both the initial probe and the pre-publish recheck, so the predicate is one copy).
- Create `internal/app/evidence_recertify_test.go` — all operation tests, composing `setupRebaseFixture` (real git feature workspace, implemented change) + `fakePublishGitHub` + `fakeGate`/local sequence fakes.
- Modify `internal/app/finalize_rebase.go` — generalize `processFinalizeGate` with an `owner` field; add `NewBuildLocalGate`. No behavior change for the finalize owner.
- Modify `internal/app/schema_registry.go` — add the `evidence.recertify` binding in sorted position.
- Modify `internal/cli/evidence.go` — add the `recertify` leaf.
- Modify `internal/cli/finalize.go` — extract `newFinalizeDepsGated` (gate-constructor-parameterized core of `newFinalizeDepsOver`) and add `newRecertifyDeps`.
- Modify `docs/guide/proving-the-build.md` and `docs/guide/reviewing-before-the-human.md` — document the command. (These guides are NOT part of the embedded asset bundle — `internal/assets/embedded/tree` holds only `agents`/`cursor-rules`/`skills` — so no bundle regeneration.)

---

### Task 1: Build-owned production local gate (`NewBuildLocalGate`)

The production gate seam `processFinalizeGate` (in `internal/app/finalize_rebase.go`, section "production gate seam") currently hard-codes the finalize owner: `buildDriveService` guards on `pin.Config.Effective.Finalize.TestCommand.Value` and constructs `NewFinalizeGateDriveService`, and `RunLocalGate` starts drives with `Phase: finalizeLocalGatePhase`. Generalize it with an `owner` field so a second constructor yields a BUILD-owned gate reading only `build.test_command` via `NewBuildGateDriveService`. Per learning `duplicated-gate-copies-the-whole-predicate`, this parameterizes the ONE existing predicate rather than copying it into a second seam.

Note on budget semantics (document, do not change): a build-owned change-scoped `GateDriveService.Start` charges the change-lifetime suite-attempt budget keyed `(repo, change, "build")` against the snapshotted `build.max_attempts` (`reserveBuildSuiteAttempt`). A recertify therefore spends one build suite attempt; an exhausted budget refuses the Start, which `mapDriveOutcome` reports as a halt (`unavailable`) — fail closed, and the remedy is raising `build.max_attempts`. This preserves existing admission/ownership exactly, as the spec requires.

**Files:**
- Modify: `internal/app/finalize_rebase.go` (the `processFinalizeGate` type, `NewFinalizeGate`, `buildDriveService`, `RunLocalGate`; the `finalizeLocalGatePhase` const block)
- Test: `internal/app/evidence_recertify_test.go` (new file; this task adds the gate-owner tests only)

**Interfaces:**
- Consumes: `NewBuildGateDriveService(gitCommonDir, exePath string, eff config.Effective) (*GateDriveService, Result, string)`; `NewFinalizeGateDriveService` (same shape); `PlanningDeps`, `WorkspaceDeps`; the `FinalizeGate` interface `RunLocalGate(ctx, LocalGateRequest) (LocalGateResult, error)`.
- Produces: `func NewBuildLocalGate(planning PlanningDeps, wdeps WorkspaceDeps) FinalizeGate` — same signature shape as `NewFinalizeGate`, used by Task 5's CLI wiring and injectable-by-name nowhere else. Const `recertifyGatePhase = "evidence-recertify-gate"`.

- [ ] **Step 1: Write the failing tests**

Create `internal/app/evidence_recertify_test.go`:

```go
package app

import (
	"context"
	"testing"
)

// buildVsFinalizeYAML declares DIVERGENT build/finalize commands so a test can
// prove which owner's command a gate resolved (acceptance: differing commands
// prove only the BUILD command runs).
const buildVsFinalizeYAML = "build:\n  gate: local\n  test_command: go test ./build-only\nfinalize:\n  test_command: make finalize-only\n"

// TestBuildLocalGateResolvesBuildCommandOnly: the BUILD-owned production gate
// resolves build.test_command; the finalize twin resolves finalize.test_command
// from the same pin. Deleting the owner branch in buildDriveService reddens one
// of the two arms.
func TestBuildLocalGateResolvesBuildCommandOnly(t *testing.T) {
	deps, wdeps, repoDir := evidenceDepsWithConfig(t, readyWorkspace(), buildVsFinalizeYAML)
	ctx := context.Background()

	bg := NewBuildLocalGate(deps, wdeps).(*processFinalizeGate)
	bsvc, ok := bg.buildDriveService(ctx, repoDir)
	if !ok || bsvc.command != "go test ./build-only" {
		t.Fatalf("build gate resolved (ok=%v, command=%q); want build.test_command %q", ok, commandOf(bsvc), "go test ./build-only")
	}
	fg := NewFinalizeGate(deps, wdeps).(*processFinalizeGate)
	fsvc, ok := fg.buildDriveService(ctx, repoDir)
	if !ok || fsvc.command != "make finalize-only" {
		t.Fatalf("finalize gate resolved (ok=%v, command=%q); want finalize.test_command %q", ok, commandOf(fsvc), "make finalize-only")
	}
}

// commandOf tolerates a nil service in a failure message.
func commandOf(svc *GateDriveService) string {
	if svc == nil {
		return "<nil>"
	}
	return svc.command
}

// TestBuildLocalGateFailsClosedWithoutBuildCommand: a config with ONLY
// finalize.test_command set fails the build-owned gate closed (ok=false → the
// caller halts, never a fabricated red) while the finalize twin still resolves.
// This pins the guard's keying on the owner's OWN config key.
func TestBuildLocalGateFailsClosedWithoutBuildCommand(t *testing.T) {
	yaml := "finalize:\n  test_command: make finalize-only\n"
	deps, wdeps, repoDir := evidenceDepsWithConfig(t, readyWorkspace(), yaml)
	ctx := context.Background()

	bg := NewBuildLocalGate(deps, wdeps).(*processFinalizeGate)
	if _, ok := bg.buildDriveService(ctx, repoDir); ok {
		t.Fatalf("build-owned gate resolved a drive service with no build.test_command; must fail closed")
	}
	fg := NewFinalizeGate(deps, wdeps).(*processFinalizeGate)
	if _, ok := fg.buildDriveService(ctx, repoDir); !ok {
		t.Fatalf("finalize-owned gate must still resolve from finalize.test_command")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestBuildLocalGate' -count=1`
Expected: FAIL — `undefined: NewBuildLocalGate` (compile error).

- [ ] **Step 3: Implement the owner generalization**

In `internal/app/finalize_rebase.go`, section "production gate seam":

Add the owner field and constants (beside the `finalizeLocalGatePhase` const):

```go
type processFinalizeGate struct {
	planning PlanningDeps
	wdeps    WorkspaceDeps
	// owner selects which role's test command and drive-service constructor the
	// seam composes: gateOwnerFinalize (finalize.test_command) or gateOwnerBuild
	// (build.test_command — `evidence recertify`, change 0415). The command
	// choice is a domain boundary (ADR-0102): each owner reads ONLY its own key.
	owner string
}

// Gate seam owners. The zero value is treated as finalize for safety, but both
// constructors set the field explicitly.
const (
	gateOwnerFinalize = "finalize"
	gateOwnerBuild    = "build"
)

// recertifyGatePhase names the workflow phase a build-owned recertify drive
// certifies. It is recorded on the drive record only; the suite-attempt budget
// key uses the literal phase "build" regardless (reserveBuildSuiteAttempt), so
// a recertify charges the change's one build budget.
const recertifyGatePhase = "evidence-recertify-gate"
```

Update `NewFinalizeGate` and add `NewBuildLocalGate`:

```go
func NewFinalizeGate(planning PlanningDeps, wdeps WorkspaceDeps) FinalizeGate {
	return &processFinalizeGate{planning: planning, wdeps: wdeps, owner: gateOwnerFinalize}
}

// NewBuildLocalGate builds the production local-gate seam for the BUILD role
// (`evidence recertify`, change 0415). It reads ONLY build.test_command through
// NewBuildGateDriveService — never finalize's — and otherwise behaves exactly
// like the finalize seam: one slice per RunLocalGate call, WAITING carries the
// continuation, PASSED mints evidence through the landed evidence-record path,
// and every uncertainty is a halt, never a fabricated red.
func NewBuildLocalGate(planning PlanningDeps, wdeps WorkspaceDeps) FinalizeGate {
	return &processFinalizeGate{planning: planning, wdeps: wdeps, owner: gateOwnerBuild}
}
```

Rework `buildDriveService` to key command and constructor on the owner (the whole predicate moves together — guard and constructor may never disagree):

```go
func (g *processFinalizeGate) buildDriveService(ctx context.Context, repoDir string) (*GateDriveService, bool) {
	pin, err := g.planning.Reader.PinContext(ctx, repoDir)
	if err != nil {
		return nil, false
	}
	// Each owner reads ONLY its own test_command (ADR-0102); the guard below and
	// the constructor selection key on the same owner so they cannot diverge.
	command := pin.Config.Effective.Finalize.TestCommand.Value
	if g.owner == gateOwnerBuild {
		command = pin.Config.Effective.Build.TestCommand.Value
	}
	if command == "" {
		return nil, false
	}
	repo, err := g.planning.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repoDir})
	if err != nil {
		return nil, false
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, false
	}
	var svc *GateDriveService
	if g.owner == gateOwnerBuild {
		svc, _, _ = NewBuildGateDriveService(repo.CommonDir, exe, pin.Config.Effective)
	} else {
		svc, _, _ = NewFinalizeGateDriveService(repo.CommonDir, exe, pin.Config.Effective)
	}
	if svc == nil {
		return nil, false
	}
	return svc, true
}
```

(Preserve the existing comment "Finalize's gate is finalize-owned…" by replacing it with the ADR-0102 comment above. Keep the temp-dir prefix in `RunLocalGate` as is.)

In `RunLocalGate`, select the phase for a fresh Start:

```go
	phase := finalizeLocalGatePhase
	if g.owner == gateOwnerBuild {
		phase = recertifyGatePhase
	}
	out = svc.Start(GateDriveStartRequest{
		RepoDir:             req.WorkspaceDir,
		Worktree:            req.WorkspaceDir,
		ChangeID:            strconv.Itoa(req.ID),
		Phase:               phase,
		Cwd:                 req.WorkspaceDir,
		RunRoot:             runRoot,
		IdempotentSuiteGate: true,
	})
```

Everything else (`Advance`, `mapDriveOutcome`, evidence minting via `EvidenceRecord`, run-root cleanup, halt mapping) is owner-agnostic and stays untouched — `EvidenceRecord` is already build-owned (change 0374), so a build gate's PASSED terminal mints evidence recording `build.test_command`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestBuildLocalGate' -count=1`
Expected: PASS. Also run the finalize-gate neighbors to prove no regression: `go test ./internal/app/ -run 'TestFinalizeRebase|TestFinalizePublish' -count=1` — PASS.

- [ ] **Step 5: Mutation-check the guard (no commit of the mutation)**

Temporarily change `command := pin.Config.Effective.Finalize.TestCommand.Value` to read Build for both owners; run Step 4's first command; expect `TestBuildLocalGateFailsClosedWithoutBuildCommand` to FAIL (finalize arm). Revert the mutation (undo the edit by hand — never `git checkout --`, which would destroy the real work; learning `mutation-restore-needs-a-backup-copy`), and re-run to green with `-count=1`.

- [ ] **Step 6: Commit**

```bash
git add internal/app/finalize_rebase.go internal/app/evidence_recertify_test.go
git commit -m "feat(0415): build-owned production local gate (NewBuildLocalGate)"
```

---

### Task 2: Recertify types, probe, and preconditions

Add the operation's closed vocabulary and the `recertifyProbe` helper that resolves and validates all identity preconditions: implemented status, clean registered workspace, local/remote/PR head agreement, exactly one open PR. The same helper is re-run as the pre-publish recheck in Task 3, so the predicate exists once.

**Files:**
- Create: `internal/app/evidence_recertify.go`
- Test: `internal/app/evidence_recertify_test.go` (extend)

**Interfaces:**
- Consumes: `loadWorkspaceContext(ctx, deps PlanningDeps, repoDir string, id int, opKey string) (workspaceContext, *WorkspaceOpResult)`; `resolveWorkspaceTarget(opKey string, wc workspaceContext) (workspace.Target, *WorkspaceOpResult)`; `FinalizeDeps` (`.Planning`, `.GitHub`, `.Workspace`, `.Gate`); `deps.Workspace.Inspect`; `deps.Planning.Client.ProbeRemoteBranch`; `deps.GitHub.DiscoverRepository` / `FindOpenPullRequestsByHead`; `domain.StatusImplemented`; `workspace.StateReady` / `workspace.StateDirty`; `gitcli.RemoteRefFound`; `originRemote`, `branchRefPrefix` (package consts).
- Produces (for Tasks 3–5): `OperationEvidenceRecertify = "evidence.recertify"`; `EvidenceRecertifyRequest{ID int}`; `EvidenceRecertifyResult` (fields below); the `ReasonRecertify*` consts; `recertifyFacts{id int, head string, wsDir string, branch string, repo githubcli.Repository, pr githubcli.PullRequest, build config.Build}`; `func recertifyProbe(ctx context.Context, deps FinalizeDeps, repoDir string, id int) (recertifyFacts, *EvidenceRecertifyResult)`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/evidence_recertify_test.go` (helpers first — later tasks reuse them):

```go
// greenBlockFor renders a bare canonical green evidence block certifying head
// with the fixture repo's resolved build.test_command ("go test ./..." — see
// buildConfiguredRepo's .docket.yml).
func greenBlockFor(t *testing.T, head string) string {
	t.Helper()
	rec, err := evidence.NewRecord("go test ./...", head, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("evidence.NewRecord: %v", err)
	}
	return evidence.Render(rec)
}

// recertifyFixture assembles the standard recertify scenario over the real-git
// rebase fixture: an implemented change, a clean published feature workspace,
// and one open PR at the current head whose body carries STALE green evidence
// (it names the base tip, an older real commit). gate is injected per test.
func recertifyFixture(t *testing.T, gate FinalizeGate) (*rebaseFixture, *fakePublishGitHub, FinalizeDeps, WorkspaceDeps) {
	t.Helper()
	f := setupRebaseFixture(t, planRepoModes()[0])
	staleBody := greenEvidenceFor(t, f.baseTip) // "Authored prose.\n\n<block>\nMore prose.\n"
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.prForHead(f.head, staleBody)}
	deps := f.finalizeDeps(gh, gate)
	return f, gh, deps, WorkspaceDeps{Service: f.svc}
}

// TestEvidenceRecertifyRefusesNotImplemented: any non-implemented status is
// blocked before any probe of the gate or PR edit (acceptance 3).
func TestEvidenceRecertifyRefusesNotImplemented(t *testing.T) {
	f := setupRebaseFixtureStatus(t, planRepoModes()[0], "in-progress")
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.prForHead(f.head, greenEvidenceFor(t, f.baseTip))}
	res := EvidenceRecertify(context.Background(), f.finalizeDeps(gh, &fakeGate{}), WorkspaceDeps{Service: f.svc},
		f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != ReasonRecertifyNotImplemented {
		t.Fatalf("result = %s/%s; want blocked/%s", res.Result, res.Reason, ReasonRecertifyNotImplemented)
	}
	if gh.ensNext != 0 {
		t.Fatalf("a refused recertify edited the PR")
	}
}

// TestEvidenceRecertifyRefusesDirtyWorkspace: uncommitted work blocks (never
// gated over, never published) — acceptance 3.
func TestEvidenceRecertifyRefusesDirtyWorkspace(t *testing.T) {
	f, gh, deps, wdeps := recertifyFixture(t, &fakeGate{})
	writeRepoFile(t, f.wp, "dirty.txt", "uncommitted\n")
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != ReasonRecertifyWorkspaceDirty {
		t.Fatalf("result = %s/%s; want blocked/%s", res.Result, res.Reason, ReasonRecertifyWorkspaceDirty)
	}
	if gh.ensNext != 0 {
		t.Fatalf("a refused recertify edited the PR")
	}
}

// TestEvidenceRecertifyRefusesUnpublishedFollowUp: a local follow-up commit
// that was never pushed disagrees with the remote feature head; the operation
// refuses (publish first through the existing workflow) rather than certify a
// head the PR does not hold.
func TestEvidenceRecertifyRefusesUnpublishedFollowUp(t *testing.T) {
	f, gh, deps, wdeps := recertifyFixture(t, &fakeGate{})
	writeRepoFile(t, f.wp, "followup.txt", "review fix\n")
	runGit(t, f.wp, "add", "-A")
	runGit(t, f.wp, "commit", "-q", "-m", "review fix")
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result == ResultApplied || res.Reason != ReasonRecertifyHeadDisagreement {
		t.Fatalf("result = %s/%s; want a %s refusal", res.Result, res.Reason, ReasonRecertifyHeadDisagreement)
	}
	if gh.ensNext != 0 {
		t.Fatalf("a refused recertify edited the PR")
	}
}

// TestEvidenceRecertifyRefusesClosedOrMismatchedPR: no open PR for the feature
// head refuses (pr-not-open); an open PR naming a different head refuses
// (head-disagreement). Neither runs the gate — acceptance 3.
func TestEvidenceRecertifyRefusesClosedOrMismatchedPR(t *testing.T) {
	// Closed: the fake returns no open PR when State is not open.
	f, gh, deps, wdeps := recertifyFixture(t, &fakeGate{})
	gh.pr.State = githubcli.StateClosed
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != ReasonRecertifyPRNotOpen {
		t.Fatalf("closed PR: result = %s/%s; want blocked/%s", res.Result, res.Reason, ReasonRecertifyPRNotOpen)
	}

	// Mismatched head: the open PR names the base tip, not the feature head.
	f2 := setupRebaseFixture(t, planRepoModes()[0])
	gh2 := &fakePublishGitHub{repo: retargetRepo(), pr: f2.prForHead(f2.baseTip, greenEvidenceFor(t, f2.baseTip))}
	gate2 := &fakeGate{}
	res2 := EvidenceRecertify(context.Background(), f2.finalizeDeps(gh2, gate2), WorkspaceDeps{Service: f2.svc},
		f2.repo.invocation, EvidenceRecertifyRequest{ID: f2.id})
	if res2.Result == ResultApplied || res2.Reason != ReasonRecertifyHeadDisagreement {
		t.Fatalf("mismatched PR head: result = %s/%s; want a %s refusal", res2.Result, res2.Reason, ReasonRecertifyHeadDisagreement)
	}
	if gate2.calls != 0 {
		t.Fatalf("a refused recertify ran the gate")
	}
}

// TestEvidenceRecertifyShape: a non-positive id is an invalid-input shape
// refusal before any probe.
func TestEvidenceRecertifyShape(t *testing.T) {
	f, _, deps, wdeps := recertifyFixture(t, &fakeGate{})
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: 0})
	if res.Result != ResultInvalidInput {
		t.Fatalf("result = %s; want %s", res.Result, ResultInvalidInput)
	}
}
```

Imports this file will need by the end of the plan: `context`, `errors`, `strings`, `testing`, `time`, `github.com/danielhanold/docket/internal/evidence`, `github.com/danielhanold/docket/internal/githubcli`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestEvidenceRecertify' -count=1`
Expected: FAIL — `undefined: EvidenceRecertify` (compile error).

- [ ] **Step 3: Write the types and probe, plus a minimal `EvidenceRecertify`**

Create `internal/app/evidence_recertify.go`. This step lands the full types + probe and an `EvidenceRecertify` whose gate/publish legs land in Task 3; to keep this intermediate state buildable and truthful, the post-precondition tail fails closed with a gate-halted refusal (never a success shape):

```go
package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is the `evidence recertify` operation (change 0415): for an
// implemented change whose open PR received a PUBLISHED follow-up commit, it
// reruns the BUILD gate at the current feature head, records and verifies
// canonical build evidence, and refreshes ONLY the existing PR's build-evidence
// block. The change stays implemented throughout; the operation performs no
// Docket metadata mutation, no push, no rebase, no merge, and no automatic
// repair. It composes only landed services: the workspace context/target
// resolution, the build-owned production local gate (NewBuildLocalGate), the
// build-owned evidence record path, and finalize publish's loss-preserving PR
// evidence-block edit (evidence.Upsert + finalizePublishEnsurer) WITHOUT its
// rebase receipt, PublishRewrite, or merge path.

// OperationEvidenceRecertify is the operation key `evidence recertify` records
// in its result envelope.
const OperationEvidenceRecertify = "evidence.recertify"

// The closed recertify outcomes.
const (
	// RecertifyOutcomeGreen: the build gate passed and the PR's evidence block
	// was converged onto the exact current head.
	RecertifyOutcomeGreen = "green"
	// RecertifyOutcomeSkipped: build.gate is off; truthful skipped evidence was
	// minted and verified. The PR block is untouched — evidence.Upsert is
	// green-only by design, and this operation preserves evidence rendering.
	RecertifyOutcomeSkipped = "skipped"
)

// Stable machine reasons `evidence recertify` reports. Message text is
// explanatory and must not be parsed. Config refusals reuse the evidence
// vocabulary (ReasonEvidenceUnconfiguredGate).
const (
	ReasonRecertifyNotImplemented    = "not-implemented"
	ReasonRecertifyWorkspaceDirty    = "workspace-dirty"
	ReasonRecertifyWorkspaceNotReady = "workspace-not-ready"
	ReasonRecertifyRemoteProbe       = "remote-feature-probe-failed"
	ReasonRecertifyRemoteAbsent      = "remote-feature-absent"
	// ReasonRecertifyHeadDisagreement: local, remote, and PR feature heads must
	// all agree before (and still agree after) the gate; the message names the
	// disagreeing leg. An unpushed follow-up must be published first.
	ReasonRecertifyHeadDisagreement = "head-disagreement"
	ReasonRecertifyRepoUnresolved   = "repository-unresolved"
	ReasonRecertifyPRProbeFailed    = "pr-probe-failed"
	ReasonRecertifyPRNotOpen        = "pr-not-open"
	ReasonRecertifyGateFailed       = "gate-failed"
	ReasonRecertifyGateHalted       = "gate-halted"
	// ReasonRecertifyIdentityDrift: something the gate certified moved before
	// the publish — the PR identity or the resolved build command. A changed
	// head or command can never inherit the earlier pass.
	ReasonRecertifyIdentityDrift      = "identity-drift"
	ReasonRecertifyEvidenceUnverified = "evidence-unverified"
	ReasonRecertifyBodyAssembly       = "body-assembly-failed"
	ReasonRecertifyEditorUnavailable  = "pr-editor-unavailable"
	ReasonRecertifyEditContended      = "pr-edit-contended"
	ReasonRecertifyEditUnknown        = "pr-edit-unknown"
)

// EvidenceRecertifyRequest is the closed request for `evidence recertify`.
type EvidenceRecertifyRequest struct {
	ID int `json:"id" docket:"required"`
}

// EvidenceRecertifyResult is the protocol-v1 document the operation returns. It
// names identity, the exact certified head, the closed outcome, the PR
// reference, and — when a gate ran — its outcome facts. It holds no authored PR
// body bytes.
type EvidenceRecertifyResult struct {
	Envelope
	ID          int    `json:"id,omitempty"`
	Head        string `json:"head,omitempty"`
	Outcome     string `json:"outcome,omitempty"`
	Command     string `json:"command,omitempty"`
	Number      int    `json:"number,omitempty"`
	Reference   string `json:"reference,omitempty"`
	URL         string `json:"url,omitempty"`
	GateOutcome string `json:"gate_outcome,omitempty"`
	HaltCause   string `json:"halt_cause,omitempty"`
	RunDir      string `json:"run_dir,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Message     string `json:"message,omitempty"`
}

// HumanText renders a one-line summary naming identity, outcome, head, and PR —
// never a body.
func (r EvidenceRecertifyResult) HumanText() string {
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		s := fmt.Sprintf("%s: change %04d %s (head %s)", r.Operation, r.ID, r.Outcome, shortCommit(r.Head))
		if r.Reference != "" {
			s += " " + r.Reference
		}
		return s
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

// recertifyRefusal stamps a typed refusal for the recertify operation.
func recertifyRefusal(result Result, reason, message string, id int) EvidenceRecertifyResult {
	return EvidenceRecertifyResult{Envelope: NewEnvelope(OperationEvidenceRecertify, result), ID: id, Reason: reason, Message: message}
}

// recertifyFacts is the identity bundle one probe pass resolves: the exact
// agreed feature head, the workspace checkout, the discovered GitHub repo, and
// the single open PR, plus the pinned build gate policy.
type recertifyFacts struct {
	id     int
	head   string
	wsDir  string
	branch string
	repo   githubcli.Repository
	pr     githubcli.PullRequest
	build  config.Build
}

// recertifyProbe resolves and validates every identity precondition, in one
// reusable predicate (learning duplicated-gate-copies-the-whole-predicate): the
// change is implemented; the manifest-owned feature workspace is clean,
// registered, and inspectable; the local head, the remote feature head, and the
// single open PR's head all agree. It is run BEFORE the gate and AGAIN before
// the publish — the recheck is the same whole predicate, not a copied subset.
func recertifyProbe(ctx context.Context, deps FinalizeDeps, repoDir string, id int) (recertifyFacts, *EvidenceRecertifyResult) {
	pin, err := deps.Planning.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		r := recertifyRefusal(result, reason, err.Error(), id)
		return recertifyFacts{}, &r
	}
	wc, wref := loadWorkspaceContext(ctx, deps.Planning, repoDir, id, OperationEvidenceRecertify)
	if wref != nil {
		r := recertifyRefusal(wref.Result, wref.Reason, wref.Message, wref.ID)
		return recertifyFacts{}, &r
	}
	cid := int(wc.change.ID())
	if wc.change.Status() != domain.StatusImplemented {
		r := recertifyRefusal(ResultBlocked, ReasonRecertifyNotImplemented,
			fmt.Sprintf("change %04d is %q, not implemented; recertify only refreshes an implemented change's evidence", cid, wc.change.RawStatus()), cid)
		return recertifyFacts{}, &r
	}
	target, tref := resolveWorkspaceTarget(OperationEvidenceRecertify, wc)
	if tref != nil {
		r := recertifyRefusal(tref.Result, tref.Reason, tref.Message, tref.ID)
		return recertifyFacts{}, &r
	}
	insp, err := deps.Workspace.Inspect(ctx, workspace.InspectRequest{Repository: wc.repo, Target: target})
	if err != nil {
		r := recertifyRefusal(ResultExternalFailed, ReasonRecertifyWorkspaceNotReady, err.Error(), cid)
		return recertifyFacts{}, &r
	}
	switch insp.Kind {
	case workspace.StateReady:
		// clean and registered; fall through
	case workspace.StateDirty:
		r := recertifyRefusal(ResultBlocked, ReasonRecertifyWorkspaceDirty,
			"the feature workspace has uncommitted changes; a recertify requires a clean tree", cid)
		return recertifyFacts{}, &r
	default:
		r := recertifyRefusal(ResultBlocked, ReasonRecertifyWorkspaceNotReady,
			fmt.Sprintf("the feature workspace is %q, not the clean registered feature state", insp.Kind), cid)
		return recertifyFacts{}, &r
	}
	head := strings.ToLower(string(insp.HeadCommit))

	// The remote feature head must equal the local head: an unpushed follow-up
	// is published through the existing workflow first, never certified here.
	rref, err := deps.Planning.Client.ProbeRemoteBranch(ctx, wc.repo, originRemote, target.FeatureRef)
	if err != nil {
		r := recertifyRefusal(ResultExternalFailed, ReasonRecertifyRemoteProbe, err.Error(), cid)
		return recertifyFacts{}, &r
	}
	if rref.State != gitcli.RemoteRefFound {
		r := recertifyRefusal(ResultBlocked, ReasonRecertifyRemoteAbsent,
			"the remote feature ref is absent; publish the feature head before recertifying", cid)
		return recertifyFacts{}, &r
	}
	if strings.ToLower(string(rref.Commit)) != head {
		r := recertifyRefusal(ResultBlocked, ReasonRecertifyHeadDisagreement,
			"the remote feature head is not the local head; publish the follow-up through the existing workflow first", cid)
		return recertifyFacts{}, &r
	}

	// Exactly one open PR, naming exactly this head.
	repo, err := deps.GitHub.DiscoverRepository(ctx, repoDir)
	if err != nil {
		r := recertifyRefusal(ResultExternalFailed, ReasonRecertifyRepoUnresolved, err.Error(), cid)
		return recertifyFacts{}, &r
	}
	branch := strings.TrimPrefix(string(target.FeatureRef), branchRefPrefix)
	prs, err := deps.GitHub.FindOpenPullRequestsByHead(ctx, repo, branch)
	if err != nil {
		r := recertifyRefusal(ResultExternalFailed, ReasonRecertifyPRProbeFailed, err.Error(), cid)
		return recertifyFacts{}, &r
	}
	if len(prs) != 1 {
		r := recertifyRefusal(ResultBlocked, ReasonRecertifyPRNotOpen,
			fmt.Sprintf("%d open pull requests for the feature head; a recertify requires exactly one", len(prs)), cid)
		return recertifyFacts{}, &r
	}
	pr := prs[0]
	if strings.ToLower(pr.HeadCommit) != head {
		r := recertifyRefusal(ResultBlocked, ReasonRecertifyHeadDisagreement,
			"the open PR names a head other than the current feature head; re-read the change state", cid)
		return recertifyFacts{}, &r
	}
	return recertifyFacts{
		id: cid, head: head, wsDir: insp.Path, branch: branch,
		repo: repo, pr: pr, build: pin.Config.Effective.Build,
	}, nil
}

// EvidenceRecertify re-certifies an implemented change's build evidence in
// place. Task 3 composes the gate and publish legs; until then the tail fails
// closed rather than shaping a success.
func EvidenceRecertify(ctx context.Context, deps FinalizeDeps, wdeps WorkspaceDeps, repoDir string, req EvidenceRecertifyRequest) EvidenceRecertifyResult {
	if req.ID <= 0 {
		return recertifyRefusal(ResultInvalidInput, "", "id must be a positive change id", req.ID)
	}
	// Capability preflight before any external effect (mirrors FinalizePublish).
	pin, err := deps.Planning.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return recertifyRefusal(result, reason, err.Error(), req.ID)
	}
	if decision := config.PreflightMutation(&pin.Config); !decision.Allowed {
		return recertifyRefusal(ResultUnsupportedConfig, ReasonDeferredCapRequested,
			"configuration actively requests a deferred capability docket does not ship in this version ("+
				strings.Join(blockerPaths(decision.Blockers), ", ")+"); withdraw it before any mutation", req.ID)
	}
	facts, refusal := recertifyProbe(ctx, deps, repoDir, req.ID)
	if refusal != nil {
		return *refusal
	}
	// Task 3 replaces this fail-closed tail with the gate + publish composition.
	return recertifyRefusal(ResultInternalError, ReasonRecertifyGateHalted,
		"recertify gate composition not yet wired", facts.id)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestEvidenceRecertify|TestBuildLocalGate' -count=1`
Expected: PASS (the Task 2 tests exercise only refusal paths, which are complete).

- [ ] **Step 5: Commit**

```bash
git add internal/app/evidence_recertify.go internal/app/evidence_recertify_test.go
git commit -m "feat(0415): evidence.recertify types and identity probe"
```

---

### Task 3: Gate composition and the publish leg

Complete `EvidenceRecertify`: the config-policy branch (gate-off skipped / unconfigured refusal), the in-process WAITING-to-terminal gate loop, the whole-predicate recheck, and the loss-preserving PR evidence-block edit.

**Files:**
- Modify: `internal/app/evidence_recertify.go`
- Test: `internal/app/evidence_recertify_test.go` (extend)

**Interfaces:**
- Consumes: `deps.Gate.RunLocalGate(ctx, LocalGateRequest{RepoDir, ID, WorkspaceDir, Head, Continuation}) (LocalGateResult, error)` with outcomes `FinalizeGatePassed|Failed|Halted|Waiting` and `GateContinuation{DriveID, Generation}`; `EvidenceRecord(ctx, deps.Planning, wdeps, repoDir, EvidenceRecordRequest{ID, Head})` for the gate-off skipped record; `evidence.Verify(body []byte, head string) Verdict`; `evidence.Extract(body []byte) (evidence.Record, error)`; `evidence.Upsert(body []byte, r evidence.Record) ([]byte, error)`; `finalizePublishEnsurer` (the local interface in `finalize_publish.go`) and `githubcli.EnsurePullRequestRequest{Repository, HeadBranch, ExpectedHead, BaseBranch, Title, Body, ExpectedVersion}`; `maxAuthoredMarkdownBytes`; `githubcli.AsFailure`.
- Produces: the complete `EvidenceRecertify`; helper `runRecertifyGate(ctx, deps, facts) (block, runDir string, refusal *EvidenceRecertifyResult)`; helper `publishRecertifiedEvidence(ctx, deps, repoDir, first recertifyFacts, block string) EvidenceRecertifyResult`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/evidence_recertify_test.go`:

```go
// seqGate scripts a per-call sequence of gate results and records the
// continuation each call received, so a test can prove ONE drive is advanced
// across WAITING slices (never a second suite).
type seqGate struct {
	results []LocalGateResult
	calls   int
	conts   []GateContinuation
}

func (g *seqGate) RunLocalGate(_ context.Context, req LocalGateRequest) (LocalGateResult, error) {
	g.conts = append(g.conts, req.Continuation)
	r := g.results[g.calls]
	if g.calls < len(g.results)-1 {
		g.calls++
	}
	return r, nil
}

// TestEvidenceRecertifyHappyPath: stale evidence at an older head becomes
// verified evidence for the exact current head on the SAME open PR; authored
// body bytes survive; the result is applied/green (acceptance 1).
func TestEvidenceRecertifyHappyPath(t *testing.T) {
	gate := &fakeGate{}
	f, gh, deps, wdeps := recertifyFixture(t, gate)
	gate.result = LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenBlockFor(t, f.head), RunDir: "/run/x"}

	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultApplied || res.Outcome != RecertifyOutcomeGreen {
		t.Fatalf("result = %s/%s outcome %q (reason %q: %s); want applied green", res.Result, res.Reason, res.Outcome, res.Reason, res.Message)
	}
	if res.Head != f.head || res.Number != 1 {
		t.Fatalf("result names head %q PR %d; want %q PR 1", res.Head, res.Number, f.head)
	}
	if gh.ensNext != 1 {
		t.Fatalf("EnsurePullRequest calls = %d; want exactly 1", gh.ensNext)
	}
	body := gh.lastEnsuredBody()
	if evidence.Verify([]byte(body), f.head) != evidence.VerdictVerified {
		t.Fatalf("converged PR body does not verify green for the current head:\n%s", body)
	}
	if !strings.Contains(body, "Authored prose.") || !strings.Contains(body, "More prose.") {
		t.Fatalf("authored PR body bytes were not preserved:\n%s", body)
	}
	if gh.ensLast.ExpectedHead != f.head || gh.ensLast.ExpectedVersion == "" {
		t.Fatalf("PR edit was not pinned to head+version: %+v", gh.ensLast)
	}
}

// TestEvidenceRecertifyAdvancesOneDriveAcrossWaiting: WAITING is nonterminal —
// the operation re-enters the SAME drive (continuation threaded) until a
// terminal, and only then publishes. Deleting the loop's continuation
// threading reddens the continuation asserts.
func TestEvidenceRecertifyAdvancesOneDriveAcrossWaiting(t *testing.T) {
	f0 := setupRebaseFixture(t, planRepoModes()[0])
	gate := &seqGate{results: []LocalGateResult{
		{Outcome: FinalizeGateWaiting, Continuation: GateContinuation{DriveID: "d1", Generation: "g1"}},
		{Outcome: FinalizeGateWaiting, Continuation: GateContinuation{DriveID: "d1", Generation: "g1"}},
		{Outcome: FinalizeGatePassed, Evidence: greenBlockFor(t, f0.head), RunDir: "/run/x"},
	}}
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f0.prForHead(f0.head, greenEvidenceFor(t, f0.baseTip))}
	res := EvidenceRecertify(context.Background(), f0.finalizeDeps(gh, gate), WorkspaceDeps{Service: f0.svc},
		f0.repo.invocation, EvidenceRecertifyRequest{ID: f0.id})
	if res.Result != ResultApplied {
		t.Fatalf("result = %s/%s: %s; want applied", res.Result, res.Reason, res.Message)
	}
	if len(gate.conts) != 3 || gate.conts[0].DriveID != "" || gate.conts[1].DriveID != "d1" || gate.conts[2].DriveID != "d1" {
		t.Fatalf("continuation threading = %+v; want empty, then d1, then d1", gate.conts)
	}
}

// TestEvidenceRecertifyGateFailureAndHalt: a red suite is gate-failed (repair
// work) and a halt is blocked — neither touches the PR (acceptance 3), and a
// WAITING with no continuation fails closed instead of spinning.
func TestEvidenceRecertifyGateFailureAndHalt(t *testing.T) {
	cases := []struct {
		name   string
		result LocalGateResult
		err    error
		want   Result
		reason string
	}{
		{"failed", LocalGateResult{Outcome: FinalizeGateFailed, RunDir: "/run/red"}, nil, ResultGateFailed, ReasonRecertifyGateFailed},
		{"halted", LocalGateResult{Outcome: FinalizeGateHalted, HaltCause: GateHaltRunningAtBudget}, nil, ResultBlocked, ReasonRecertifyGateHalted},
		{"seam error", LocalGateResult{}, errors.New("seam down"), ResultBlocked, ReasonRecertifyGateHalted},
		{"waiting without continuation", LocalGateResult{Outcome: FinalizeGateWaiting}, nil, ResultBlocked, ReasonRecertifyGateHalted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gate := &fakeGate{result: tc.result, err: tc.err}
			f, gh, deps, wdeps := recertifyFixture(t, gate)
			res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
			if res.Result != tc.want || res.Reason != tc.reason {
				t.Fatalf("result = %s/%s; want %s/%s", res.Result, res.Reason, tc.want, tc.reason)
			}
			if gh.ensNext != 0 {
				t.Fatalf("a non-passed gate edited the PR")
			}
		})
	}
}

// TestEvidenceRecertifyGateOffRecordsSkipped: build.gate off mints truthful
// skipped evidence and completes as skipped WITHOUT editing the PR block —
// evidence.Upsert is green-only by design and this change preserves evidence
// rendering (acceptance 2).
func TestEvidenceRecertifyGateOffRecordsSkipped(t *testing.T) {
	gate := &fakeGate{}
	f, gh, deps, wdeps := recertifyFixture(t, gate)
	writeRepoFile(t, f.repo.invocation, ".docket.yml",
		"integration_branch: main\nbuild:\n  gate: 'off'\nfinalize:\n  test_command: 'go test ./...'\n")
	runGit(t, f.repo.invocation, "add", ".docket.yml")
	runGit(t, f.repo.invocation, "commit", "-q", "-m", "gate off")

	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultApplied || res.Outcome != RecertifyOutcomeSkipped {
		t.Fatalf("result = %s/%s outcome %q: %s; want applied skipped", res.Result, res.Reason, res.Outcome, res.Message)
	}
	if gate.calls != 0 {
		t.Fatalf("gate ran under build.gate: off")
	}
	if gh.ensNext != 0 {
		t.Fatalf("a skipped recertify edited the PR block")
	}
}

// TestEvidenceRecertifyRefusesUnconfiguredGate: a local build gate with no
// build.test_command refuses; no suite, no PR edit (acceptance 2).
func TestEvidenceRecertifyRefusesUnconfiguredGate(t *testing.T) {
	gate := &fakeGate{}
	f, gh, deps, wdeps := recertifyFixture(t, gate)
	writeRepoFile(t, f.repo.invocation, ".docket.yml",
		"integration_branch: main\nfinalize:\n  test_command: 'go test ./...'\n")
	runGit(t, f.repo.invocation, "add", ".docket.yml")
	runGit(t, f.repo.invocation, "commit", "-q", "-m", "drop build command")

	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultUnsupportedConfig || res.Reason != ReasonEvidenceUnconfiguredGate {
		t.Fatalf("result = %s/%s; want unsupported-config/%s", res.Result, res.Reason, ReasonEvidenceUnconfiguredGate)
	}
	if gate.calls != 0 || gh.ensNext != 0 {
		t.Fatalf("an unconfigured gate ran the suite or edited the PR")
	}
}
```

Note for the two config-overlay tests: the fixture reads authoritative config from the primary checkout's `.docket.yml` (`buildConfiguredRepo` wrote it), so overwriting and committing it in `f.repo.invocation` changes the pinned policy for the next operation call. If `writeRepoFile`'s signature differs for the primary repo dir, use the same file-write helper the e2e tests use against `repo.invocation`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestEvidenceRecertify' -count=1`
Expected: FAIL — happy path and the new cases hit the Task 2 fail-closed tail (`recertify gate composition not yet wired`); Task 2's refusal tests still PASS.

- [ ] **Step 3: Implement the gate loop, config branch, recheck, and publish leg**

In `internal/app/evidence_recertify.go`, replace the fail-closed tail of `EvidenceRecertify` with:

```go
	// Config policy branch (the pinned build gate decides what "recertify" means).
	if facts.build.Gate.Value == "off" {
		// Truthful skipped evidence at the verified current head; no run, and no
		// PR edit — evidence.Upsert is green-only by design (see
		// TestPRPublishAcceptsSkippedEvidenceAtExactHead's note), and this
		// operation preserves evidence rendering.
		evd := EvidenceRecord(ctx, deps.Planning, wdeps, repoDir, EvidenceRecordRequest{ID: facts.id, Head: facts.head})
		if evd.Result != ResultApplied || evd.Block == "" {
			return recertifyRefusal(evd.Result, evd.Reason, evd.Message, facts.id)
		}
		if v := evidence.Verify([]byte(evd.Block), facts.head); v != evidence.VerdictSkipped {
			return recertifyRefusal(ResultInvalidState, ReasonRecertifyEvidenceUnverified,
				"the skipped evidence record did not verify against the current head ("+string(v)+")", facts.id)
		}
		return EvidenceRecertifyResult{
			Envelope: NewEnvelope(OperationEvidenceRecertify, ResultApplied),
			ID:       facts.id, Head: facts.head, Outcome: RecertifyOutcomeSkipped,
			Number: facts.pr.Number, Reference: fmt.Sprintf("%s#%d", facts.repo.Spec(), facts.pr.Number), URL: facts.pr.URL,
			Message: "build.gate is off; truthful skipped evidence was recorded and verified (the PR evidence block is untouched — skipped evidence is never woven into a PR body)",
		}
	}
	if facts.build.TestCommand.Value == "" {
		return recertifyRefusal(ResultUnsupportedConfig, ReasonEvidenceUnconfiguredGate,
			"build.gate is local but build.test_command is unconfigured; run `docket repository configure-tests` and review the pending edit", facts.id)
	}

	block, runDir, gref := runRecertifyGate(ctx, deps, repoDir, facts)
	if gref != nil {
		return *gref
	}
	res := publishRecertifiedEvidence(ctx, deps, repoDir, facts, block)
	res.RunDir = runDir
	return res
```

Add the two helpers:

```go
// runRecertifyGate drives the BUILD-owned local gate to a terminal within this
// operation: it advances WAITING slices of the SAME drive (each RunLocalGate
// call blocks for one bounded driver slice; the drive's own observation budget
// turns an overlong run into a running-at-budget halt, so the loop terminates).
// WAITING never means success and never starts a second suite. A PASSED
// terminal returns the canonical evidence block the seam minted through the
// landed build-owned evidence-record path (which re-verifies the head at mint
// time); FAILED is repair work; everything else is a halt — never a fabricated
// red, and never a PR edit.
func runRecertifyGate(ctx context.Context, deps FinalizeDeps, repoDir string, facts recertifyFacts) (block, runDir string, refusal *EvidenceRecertifyResult) {
	if deps.Gate == nil {
		r := recertifyRefusal(ResultInternalError, ReasonRecertifyGateHalted, "no local-gate seam is wired; cannot run the suite", facts.id)
		return "", "", &r
	}
	cont := GateContinuation{}
	for {
		gres, gerr := deps.Gate.RunLocalGate(ctx, LocalGateRequest{
			RepoDir: repoDir, ID: facts.id, WorkspaceDir: facts.wsDir, Head: facts.head, Continuation: cont,
		})
		if gerr != nil {
			r := recertifyRefusal(ResultBlocked, ReasonRecertifyGateHalted,
				"the local gate could not be established; retained, no red fabricated: "+gerr.Error(), facts.id)
			return "", "", &r
		}
		switch gres.Outcome {
		case FinalizeGateWaiting:
			if gres.Continuation.DriveID == "" {
				// A WAITING with no drive handle can never be advanced; fail closed
				// rather than loop forever.
				r := recertifyRefusal(ResultBlocked, ReasonRecertifyGateHalted,
					"the gate reported waiting without a continuation; retained", facts.id)
				return "", "", &r
			}
			cont = gres.Continuation
			continue
		case FinalizeGatePassed:
			return gres.Evidence, gres.RunDir, nil
		case FinalizeGateFailed:
			r := recertifyRefusal(ResultGateFailed, ReasonRecertifyGateFailed,
				"the build suite failed at the current head; this is repair work — no evidence was published", facts.id)
			r.GateOutcome, r.RunDir = string(gres.Outcome), gres.RunDir
			return "", "", &r
		default: // FinalizeGateHalted
			r := recertifyRefusal(ResultBlocked, ReasonRecertifyGateHalted,
				"the build gate did not reach a decidable pass/fail; retained, no red fabricated", facts.id)
			r.GateOutcome, r.HaltCause = string(gres.Outcome), gres.HaltCause
			return "", "", &r
		}
	}
}
```

NOTE: `facts.wsDirRepo(ctx)` above is a placeholder mistake — do NOT add such a method. `RunLocalGate`'s `RepoDir` is the invocation repo dir, exactly as `composeLocalGate` passes it. Thread `repoDir` into `runRecertifyGate` as a parameter instead: signature `runRecertifyGate(ctx context.Context, deps FinalizeDeps, repoDir string, facts recertifyFacts)`, and the request field is `RepoDir: repoDir`. Update the call site accordingly.

```go
// publishRecertifiedEvidence re-proves the whole identity predicate AFTER the
// gate (the same recertifyProbe — a changed head, status, worktree state, or PR
// cannot inherit the pass), re-pins the build command against the evidence, and
// then loss-preservingly replaces ONLY the PR's build-evidence block, exactly
// as FinalizePublish does — without its rebase receipt, PublishRewrite, or
// merge path. A failed or uncertain edit is never reported as completion.
func publishRecertifiedEvidence(ctx context.Context, deps FinalizeDeps, repoDir string, first recertifyFacts, block string) EvidenceRecertifyResult {
	if len(block) > maxAuthoredMarkdownBytes {
		return recertifyRefusal(ResultInvalidInput, ReasonRecertifyEvidenceUnverified,
			fmt.Sprintf("the evidence record is %d bytes, over the %d-byte authored-input bound", len(block), maxAuthoredMarkdownBytes), first.id)
	}
	if v := evidence.Verify([]byte(block), first.head); v != evidence.VerdictVerified {
		return recertifyRefusal(ResultInvalidState, ReasonRecertifyEvidenceUnverified,
			"the minted evidence does not verify green against the tested head ("+string(v)+")", first.id)
	}
	rec, err := evidence.Extract([]byte(block))
	if err != nil {
		// Unreachable after a verified verdict, but fail closed rather than trust it.
		return recertifyRefusal(ResultInvalidState, ReasonRecertifyEvidenceUnverified, err.Error(), first.id)
	}

	// Recheck: the SAME whole predicate that admitted the gate (implemented
	// status, clean workspace, local/remote/PR head agreement, one open PR).
	second, refusal := recertifyProbe(ctx, deps, repoDir, first.id)
	if refusal != nil {
		return *refusal
	}
	if second.head != first.head {
		return recertifyRefusal(ResultContended, ReasonRecertifyIdentityDrift,
			"the feature head moved after the gate; the run no longer certifies the current commit — rerun recertify", first.id)
	}
	if second.pr.Number != first.pr.Number {
		return recertifyRefusal(ResultContended, ReasonRecertifyIdentityDrift,
			"the open pull request changed after the gate; re-read the change state and rerun recertify", first.id)
	}
	// A changed build configuration cannot inherit the pass: the recorded
	// command must be byte-equal to the currently resolved build.test_command.
	if second.build.Gate.Value == "off" || rec.Command == "" || rec.Command != second.build.TestCommand.Value {
		return recertifyRefusal(ResultBlocked, ReasonRecertifyIdentityDrift,
			"the resolved build gate configuration changed after the run (or the evidence names a different command); rerun recertify under the current configuration", first.id)
	}

	newBody, err := evidence.Upsert([]byte(second.pr.Body), rec)
	if err != nil {
		return recertifyRefusal(ResultInvalidState, ReasonRecertifyBodyAssembly, err.Error(), first.id)
	}
	ensurer, ok := deps.GitHub.(finalizePublishEnsurer)
	if !ok {
		return recertifyRefusal(ResultInternalError, ReasonRecertifyEditorUnavailable,
			"the wired GitHub seam does not provide the pull-request edit face", first.id)
	}
	eres, eerr := ensurer.EnsurePullRequest(ctx, githubcli.EnsurePullRequestRequest{
		Repository:      second.repo,
		HeadBranch:      second.branch,
		ExpectedHead:    second.head,
		BaseBranch:      second.pr.BaseBranch,
		Title:           second.pr.Title,
		Body:            string(newBody),
		ExpectedVersion: second.pr.Version,
	})
	if eerr != nil {
		return mapRecertifyEnsureFailure(first.id, second.head, eerr)
	}
	base := EvidenceRecertifyResult{
		ID: first.id, Head: second.head, Outcome: RecertifyOutcomeGreen, Command: rec.Command,
		Number: eres.PR.Number, URL: eres.PR.URL,
	}
	if eres.PR.Number != 0 {
		base.Reference = fmt.Sprintf("%s#%d", second.repo.Spec(), eres.PR.Number)
	}
	switch eres.Disposition {
	case githubcli.EnsureCreated, githubcli.EnsureUpdated:
		base.Envelope = NewEnvelope(OperationEvidenceRecertify, ResultApplied)
		return base
	case githubcli.EnsureAdopted, githubcli.EnsureUnchanged:
		// The PR already carried this exact evidence — an idempotent replay.
		base.Envelope = NewEnvelope(OperationEvidenceRecertify, ResultNoOp)
		base.Message = "the pull request already carries verified evidence for this head"
		return base
	case githubcli.EnsureContended:
		return recertifyRefusal(ResultContended, ReasonRecertifyEditContended,
			"the pull request diverged under the update; rerun recertify", first.id)
	case githubcli.EnsureUnknown:
		return recertifyRefusal(ResultExternalFailed, ReasonRecertifyEditUnknown,
			"the pull-request update could not be verified; retained, no second mutation — rerun recertify to converge", first.id)
	default:
		return recertifyRefusal(ResultInternalError, ReasonStatusInternalError,
			fmt.Sprintf("unexpected pull-request edit disposition %q", eres.Disposition), first.id)
	}
}

// mapRecertifyEnsureFailure folds a githubcli EnsureFailed error onto the
// protocol taxonomy (mirrors mapPublishEnsureFailure's kind mapping; the
// failure's kind is the stable reason and its detail is bounded/redacted).
func mapRecertifyEnsureFailure(id int, head string, err error) EvidenceRecertifyResult {
	result := ResultInternalError
	reason := ReasonStatusInternalError
	message := err.Error()
	if f, ok := githubcli.AsFailure(err); ok {
		reason = string(f.Kind)
		message = f.Error()
		switch f.Kind {
		case githubcli.KindInvalidInput:
			result = ResultInvalidInput
		case githubcli.KindInvalidState:
			result = ResultInvalidState
		case githubcli.KindExternal, githubcli.KindInvalidOutput, githubcli.KindTimedOut:
			result = ResultExternalFailed
		case githubcli.KindCancelled:
			result = ResultInterrupted
		}
	}
	r := recertifyRefusal(result, reason, message, id)
	r.Head = head
	return r
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestEvidenceRecertify' -count=1`
Expected: PASS, all cases.

- [ ] **Step 5: Commit**

```bash
git add internal/app/evidence_recertify.go internal/app/evidence_recertify_test.go
git commit -m "feat(0415): compose the recertify gate loop and PR evidence publish"
```

---

### Task 4: Drift, verification, and retry hardening tests

Prove the recheck and edit-failure contracts with adversarial fixtures: a head that moves under the gate, evidence recording a foreign command (the changed-configuration face), evidence for the wrong head, and an uncertain PR edit followed by a clean retry on the same PR.

**Files:**
- Test: `internal/app/evidence_recertify_test.go` (extend)
- Modify: `internal/app/evidence_recertify.go` only if a test exposes a defect.

**Interfaces:**
- Consumes: everything Task 3 produced; `fakePublishGitHub` (embedded), `runGit`, `writeRepoFile`.

- [ ] **Step 1: Write the failing (or, if Task 3 is correct, immediately green) tests — then mutation-check each one**

```go
// movingGate commits to the feature worktree DURING the gate and then reports
// PASSED for the pre-move head — the moved-HEAD publication hazard.
type movingGate struct {
	f  *rebaseFixture
	ev string
}

func (g *movingGate) RunLocalGate(context.Context, LocalGateRequest) (LocalGateResult, error) {
	writeRepoFile(g.f.t, g.f.wp, "late-edit.txt", "moved under the gate\n")
	runGit(g.f.t, g.f.wp, "add", "-A")
	runGit(g.f.t, g.f.wp, "commit", "-q", "-m", "late edit")
	return LocalGateResult{Outcome: FinalizeGatePassed, Evidence: g.ev, RunDir: "/run/x"}, nil
}

// TestEvidenceRecertifyRefusesHeadMovedUnderGate: a HEAD that moved between
// the gate and the publish can never publish (acceptance 3). The recheck's
// local-vs-remote leg catches it (the late commit is unpublished).
func TestEvidenceRecertifyRefusesHeadMovedUnderGate(t *testing.T) {
	f := setupRebaseFixture(t, planRepoModes()[0])
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.prForHead(f.head, greenEvidenceFor(t, f.baseTip))}
	gate := &movingGate{f: f, ev: greenBlockFor(t, f.head)}
	res := EvidenceRecertify(context.Background(), f.finalizeDeps(gh, gate), WorkspaceDeps{Service: f.svc},
		f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result == ResultApplied || res.Result == ResultNoOp {
		t.Fatalf("a moved head published evidence: %s/%s", res.Result, res.Reason)
	}
	if res.Reason != ReasonRecertifyHeadDisagreement && res.Reason != ReasonRecertifyIdentityDrift {
		t.Fatalf("reason = %q; want a head-disagreement/identity-drift refusal", res.Reason)
	}
	if gh.ensNext != 0 {
		t.Fatalf("a moved head reached the PR edit")
	}
}

// TestEvidenceRecertifyRefusesForeignCommandEvidence: evidence recording a
// command other than the currently resolved build.test_command cannot publish —
// the changed-configuration face of acceptance 3.
func TestEvidenceRecertifyRefusesForeignCommandEvidence(t *testing.T) {
	gate := &fakeGate{}
	f, gh, deps, wdeps := recertifyFixture(t, gate)
	foreign, err := evidence.NewRecord("make other-suite", f.head, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("evidence.NewRecord: %v", err)
	}
	gate.result = LocalGateResult{Outcome: FinalizeGatePassed, Evidence: evidence.Render(foreign), RunDir: "/run/x"}
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != ReasonRecertifyIdentityDrift {
		t.Fatalf("result = %s/%s; want blocked/%s", res.Result, res.Reason, ReasonRecertifyIdentityDrift)
	}
	if gh.ensNext != 0 {
		t.Fatalf("foreign-command evidence reached the PR edit")
	}
}

// TestEvidenceRecertifyRefusesWrongHeadEvidence: gate evidence naming another
// head is stale at verification and never published.
func TestEvidenceRecertifyRefusesWrongHeadEvidence(t *testing.T) {
	gate := &fakeGate{}
	f, gh, deps, wdeps := recertifyFixture(t, gate)
	gate.result = LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenBlockFor(t, f.baseTip), RunDir: "/run/x"}
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultInvalidState || res.Reason != ReasonRecertifyEvidenceUnverified {
		t.Fatalf("result = %s/%s; want invalid-state/%s", res.Result, res.Reason, ReasonRecertifyEvidenceUnverified)
	}
	if gh.ensNext != 0 {
		t.Fatalf("unverified evidence reached the PR edit")
	}
}

// flakyEnsureGitHub reports the first N EnsurePullRequest calls as UNKNOWN
// (an unverifiable external effect) and then delegates to the real fake.
type flakyEnsureGitHub struct {
	*fakePublishGitHub
	unknowns int
}

func (f *flakyEnsureGitHub) EnsurePullRequest(ctx context.Context, req githubcli.EnsurePullRequestRequest) (githubcli.EnsureResult, error) {
	if f.unknowns > 0 {
		f.unknowns--
		return githubcli.EnsureResult{Disposition: githubcli.EnsureUnknown}, nil
	}
	return f.fakePublishGitHub.EnsurePullRequest(ctx, req)
}

// TestEvidenceRecertifyEditFailureThenRetry: an uncertain PR edit is NOT
// completion; a later invocation converges the SAME PR and preserves authored
// content (acceptance 4).
func TestEvidenceRecertifyEditFailureThenRetry(t *testing.T) {
	f := setupRebaseFixture(t, planRepoModes()[0])
	inner := &fakePublishGitHub{repo: retargetRepo(), pr: f.prForHead(f.head, greenEvidenceFor(t, f.baseTip))}
	gh := &flakyEnsureGitHub{fakePublishGitHub: inner, unknowns: 1}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenBlockFor(t, f.head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)
	wdeps := WorkspaceDeps{Service: f.svc}

	first := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if first.Result == ResultApplied || first.Result == ResultNoOp {
		t.Fatalf("an unverified PR edit was reported as completion: %s/%s", first.Result, first.Reason)
	}
	if first.Reason != ReasonRecertifyEditUnknown {
		t.Fatalf("first reason = %q; want %s", first.Reason, ReasonRecertifyEditUnknown)
	}

	second := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if second.Result != ResultApplied || second.Number != 1 {
		t.Fatalf("retry = %s/%s PR %d: %s; want applied on the same PR 1", second.Result, second.Reason, second.Number, second.Message)
	}
	body := inner.pr.Body
	if evidence.Verify([]byte(body), f.head) != evidence.VerdictVerified {
		t.Fatalf("retried PR body does not verify for the current head:\n%s", body)
	}
	if !strings.Contains(body, "Authored prose.") || !strings.Contains(body, "More prose.") {
		t.Fatalf("authored PR content was not preserved across the retry:\n%s", body)
	}
}
```

- [ ] **Step 2: Run the tests; fix any exposed defects**

Run: `go test ./internal/app/ -run 'TestEvidenceRecertify' -count=1`
Expected: PASS (Task 3 already implemented these guards). Any failure here is a genuine Task 3 defect — fix `evidence_recertify.go`, not the test, unless the test contradicts the spec.

- [ ] **Step 3: Mutation-test the guards (guards are code)**

For each, apply the mutation, run `go test ./internal/app/ -run 'TestEvidenceRecertify' -count=1`, confirm the named test REDDENS, then revert the mutation by hand:
1. Delete the `second.head != first.head` recheck AND the remote-head-agreement refusal inside `recertifyProbe` → `TestEvidenceRecertifyRefusesHeadMovedUnderGate` must fail (this proves the belt-and-suspenders pair is not both dead).
2. Delete the `rec.Command != second.build.TestCommand.Value` clause → `TestEvidenceRecertifyRefusesForeignCommandEvidence` must fail.
3. Change `evidence.Verify` acceptance to also accept `VerdictStale` → `TestEvidenceRecertifyRefusesWrongHeadEvidence` must fail.
4. Map `EnsureUnknown` to `ResultApplied` → `TestEvidenceRecertifyEditFailureThenRetry` must fail.
Record in the eventual results notes any mutation that did NOT redden, as a defect investigated (learning `residual-is-for-undetectable-not-unprobed`).

- [ ] **Step 4: Commit**

```bash
git add internal/app/evidence_recertify_test.go internal/app/evidence_recertify.go
git commit -m "test(0415): drift, verification, and retry hardening for recertify"
```

---

### Task 5: CLI leaf, deps wiring, and schema binding

Expose the operation as `docket evidence recertify --id <id> [--repo-dir <dir>]` with an external-write capability annotation, wire production deps with the BUILD-owned gate, and register the operation schema. The existing correspondence guards (`TestSchemaCatalogCorrespondence`, `collectCapabilities` fail-closed, `TestOperationBindingsSortedUniqueAndDescribable`) enforce the joins — plumbing auto-discovers; no hand lists.

**Files:**
- Modify: `internal/cli/evidence.go` (add the `recertify` leaf; register it in `evidenceCmd.AddCommand`)
- Modify: `internal/cli/finalize.go` (extract `newFinalizeDepsGated`; add `newRecertifyDeps`)
- Modify: `internal/app/schema_registry.go` (add the binding)

**Interfaces:**
- Consumes: `app.EvidenceRecertify`, `app.EvidenceRecertifyRequest`, `app.NewBuildLocalGate`, `app.NewFinalizeGate`; `capability(id string, effects ...Effect)` and `EffectExternalWrite` from `internal/cli/capability.go`; `resolveRepoDir`; `newPlanningDepsOver`; `workspace.NewService`; `gitcli.NewClient`; `githubcli.NewClient`.
- Produces: `newRecertifyDeps(repoDir string) (app.FinalizeDeps, app.WorkspaceDeps, error)`; the CLI leaf `evidence recertify`; schema binding `evidence.recertify`.

- [ ] **Step 1: Add the schema binding (its guard is the failing test)**

In `internal/app/schema_registry.go`, `operationBindings`, insert in sorted position — `"evidence.recertify"` sorts BEFORE `"evidence.record"` (`rece` < `reco`):

```go
	{ID: "evidence.recertify", Request: EvidenceRecertifyRequest{}, Result: EvidenceRecertifyResult{}},              // EvidenceRecertify
	{ID: "evidence.record", Request: EvidenceRecordRequest{}, Result: EvidenceOpResult{}},                          // EvidenceRecord
```

Run: `go test ./internal/app/ -run 'TestOperationBindings' -count=1`
Expected: PASS (if it fails on sort order or describability, fix the insertion point/types, not the test).

- [ ] **Step 2: Extract the gated deps builder and add `newRecertifyDeps`**

In `internal/cli/finalize.go`, replace the body of `newFinalizeDepsOver` with a delegation and add the two functions. The extraction changes NOTHING for finalize callers — the only caller variance is the gate constructor, which becomes the parameter (learning `consolidation-flattens-caller-variance`: the variance moves into an argument, never flattened):

```go
func newFinalizeDepsOver(gitClient *gitcli.Client, ghClient *githubcli.Client, repoDir ...string) (app.FinalizeDeps, error) {
	deps, _, err := newFinalizeDepsGated(gitClient, ghClient, app.NewFinalizeGate, repoDir...)
	return deps, err
}

// newFinalizeDepsGated is the shared core: identical seam wiring over the two
// supplied clients, with the local-gate constructor as the ONLY caller-varying
// piece — finalize wires app.NewFinalizeGate (finalize.test_command), evidence
// recertify wires app.NewBuildLocalGate (build.test_command, change 0415).
func newFinalizeDepsGated(gitClient *gitcli.Client, ghClient *githubcli.Client,
	gate func(app.PlanningDeps, app.WorkspaceDeps) app.FinalizeGate, repoDir ...string) (app.FinalizeDeps, app.WorkspaceDeps, error) {
	planning, err := newPlanningDepsOver(gitClient, repoDir...)
	if err != nil {
		return app.FinalizeDeps{}, app.WorkspaceDeps{}, err
	}
	ws, err := workspace.NewService(gitClient)
	if err != nil {
		return app.FinalizeDeps{}, app.WorkspaceDeps{}, err
	}
	wdeps := app.WorkspaceDeps{Service: ws}
	return app.FinalizeDeps{
		Planning:   planning,
		GitHub:     ghClient,
		Workspace:  ws,
		PRProber:   app.NewGitHubFinalizeProber(ghClient),
		PRBatch:    app.NewSweepPRBatchReader(ghClient),
		Gate:       gate(planning, wdeps),
		CleanupGit: gitClient,
	}, wdeps, nil
}

// newRecertifyDeps assembles the seams `evidence recertify` composes: the same
// finalize wiring with the BUILD-owned local gate, plus the WorkspaceDeps the
// evidence operations need.
func newRecertifyDeps(repoDir string) (app.FinalizeDeps, app.WorkspaceDeps, error) {
	gitClient, err := gitcli.NewClient()
	if err != nil {
		return app.FinalizeDeps{}, app.WorkspaceDeps{}, err
	}
	ghClient, err := githubcli.NewClient()
	if err != nil {
		return app.FinalizeDeps{}, app.WorkspaceDeps{}, err
	}
	return newFinalizeDepsGated(gitClient, ghClient, app.NewBuildLocalGate, repoDir)
}
```

(Keep the existing doc comment on `newFinalizeDepsOver` about threading one policy-carrying client instance — move it onto `newFinalizeDepsGated`, where the wiring now lives. If `newSweepFinalizeDeps` calls `newFinalizeDepsOver`, it is untouched by construction.)

- [ ] **Step 3: Add the CLI leaf**

In `internal/cli/evidence.go`, after the `verify` command:

```go
	recertify := &cobra.Command{
		Use:   "recertify",
		Short: "Rerun the build gate for an implemented change and refresh its open PR's evidence block in place",
		Args:  cobra.NoArgs,
		// external-write: the operation edits the existing pull request's
		// build-evidence block (and only that block) on GitHub.
		Annotations: capability("evidence.recertify", EffectExternalWrite),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			id, _ := c.Flags().GetInt("id")
			deps, wdeps, err := newRecertifyDeps(repoDir)
			if err != nil {
				return err
			}
			setResult(app.EvidenceRecertify(c.Context(), deps, wdeps, repoDir,
				app.EvidenceRecertifyRequest{ID: id}))
			return nil
		},
	}
	recertify.Flags().Int("id", 0, "implemented change `id` whose evidence to re-certify (required)")
	recertify.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	_ = recertify.MarkFlagRequired("id")
```

And change the registration line to:

```go
	evidenceCmd.AddCommand(record, verify, recertify)
```

- [ ] **Step 4: Run the correspondence and CLI guards**

Run: `go test ./internal/cli/ -count=1 && go test ./internal/app/ -run 'TestOperationBindings|TestEvidenceRecertify' -count=1`
Expected: PASS — `collectCapabilities` fails closed on an unannotated leaf, `TestSchemaCatalogCorrespondence` joins catalog↔schema in both directions, and the capability-catalog production tests pick the new leaf up by discovery. If any capability test asserts an expected count/fixture list, follow that test's own remedy message.

- [ ] **Step 5: Commit**

```bash
git add internal/app/schema_registry.go internal/cli/evidence.go internal/cli/finalize.go
git commit -m "feat(0415): expose evidence.recertify (CLI leaf, build-gated deps, schema)"
```

---

### Task 6: Documentation in the evidence and review-follow-up guides

Document the command where the stale-evidence problem is already described. Both files are plain repo docs (not embedded assets, no sentinel tests grep them today — verify with `grep -rn "recertify" internal tests` before assuming otherwise).

**Files:**
- Modify: `docs/guide/proving-the-build.md`
- Modify: `docs/guide/reviewing-before-the-human.md`

- [ ] **Step 1: Extend `docs/guide/proving-the-build.md`**

At the end of the `## Build evidence` section (immediately before the next `##` heading), append:

```markdown
### Re-certifying after a follow-up commit

A follow-up commit pushed to an implemented change's open PR (say, addressing review feedback
before merge) makes the recorded evidence stale — it still names the old head. The supported
in-place recovery is:

    docket evidence recertify --id <id>

It reruns the configured `build.test_command` (only the build command — never finalize's) in the
change's feature worktree at the current published head, records and verifies fresh evidence, and
replaces only the build-evidence block on the existing PR. The change stays `implemented`; nothing
is committed, pushed, rebased, or merged. Preconditions: a clean feature worktree whose local head,
remote feature head, and single open PR head all agree — an unpushed follow-up must be published
through the normal workflow first. `build.gate: off` records truthful skipped evidence (the PR
block is untouched); a failed or halted gate reports repair work and never touches the PR. The run
charges one attempt against the change's `build.max_attempts` suite budget.
```

- [ ] **Step 2: Extend `docs/guide/reviewing-before-the-human.md`**

In the paragraph that describes build evidence being "missing, malformed, or stale" (the `unverified-build-state` blocker discussion), append a sentence to that paragraph:

```markdown
When a review-feedback follow-up commit has already been pushed to the open PR and only the
evidence went stale, `docket evidence recertify --id <id>` re-runs the build gate at the new head
and refreshes the PR's evidence block in place — no re-entry into implement-next and no merge-time
re-gate needed.
```

- [ ] **Step 3: Verify and commit**

Run: `go build ./... && go test ./internal/app/ ./internal/cli/ -count=1` (docs cannot break Go, but this is the pre-commit sanity gate).

```bash
git add docs/guide/proving-the-build.md docs/guide/reviewing-before-the-human.md
git commit -m "docs(0415): document evidence recertify in the build-evidence and review guides"
```

---

### Task 7: Full-suite build gate

- [ ] **Step 1: Run the whole suite through the build gate**

The BUILD gate's command is whatever `build.test_command` resolves to in this repository's config — read it from config, never a second copy; the docket-build role drives it via the gate driver (`docket gate drive` slices), entered from source. Do not substitute a hand-picked test subset.

Expected: green. Read the budget report even on green — any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line is a screening finding and a `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative breach; the new `internal/app` tests use real git fixtures, so watch that package's row.

- [ ] **Step 2: Fix anything red, re-run to green, and commit any fixes**

```bash
git add <specific files changed by fixes>
git commit -m "fix(0415): <what the suite surfaced>"
```

---

## Spec acceptance → task map (self-review record)

1. Stale → verified evidence at the exact head, same PR, change stays implemented → Task 3 `TestEvidenceRecertifyHappyPath` (+ the operation takes no metadata writer at all — it cannot change status by construction).
2. Only the build command runs; gate-off skipped; missing build config refuses → Task 1 both tests; Task 3 `TestEvidenceRecertifyGateOffRecordsSkipped`, `TestEvidenceRecertifyRefusesUnconfiguredGate`.
3. Gate failure/halt, dirty or moved HEAD, changed configuration, non-implemented status, closed/mismatched PR cannot publish → Task 2 refusal tests; Task 3 `TestEvidenceRecertifyGateFailureAndHalt`; Task 4 moved-head / foreign-command / wrong-head tests.
4. PR edit failure is not completion; retry updates the same PR, authored content preserved → Task 4 `TestEvidenceRecertifyEditFailureThenRetry`.
5. Document in existing evidence/review-follow-up guidance → Task 6.

Known deliberate deviations to carry into the results notes: (a) gate-off completes WITHOUT a PR edit because `evidence.Upsert` is green-only by design and the spec forbids changing evidence rendering — the skipped outcome is explicit in the result message; (b) a recertify charges one attempt against the change-lifetime `build.max_attempts` suite budget (existing admission preserved; an exhausted budget surfaces as a halt whose remedy is raising `build.max_attempts`).
