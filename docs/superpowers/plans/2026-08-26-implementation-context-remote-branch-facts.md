<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0357 — Implementation context must load remote branch facts before judging stack base](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0357-implementation-context-loads-remote-branch-facts.md)**
<!-- docket:backlink:end -->
# Implementation Context Remote Branch Facts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `ContextImplementation` load real remote branch facts (instead of a hardcoded empty set) so a child change stacked on a live parent with a pushed recorded branch passes the pre-claim implementation-context gate.

**Architecture:** One new read in `internal/app/implementation_context.go`: after the operation pins context and builds the domain snapshot, call `deps.Reader.BranchFacts(ctx, pin, stackBranches(snap))` — the exact call shape already used by `ChangeClaim` (`internal/app/change_claim.go`, `resolveClaimTarget`), `WorkspacePrepare` (`internal/app/workspace_ops.go`), and `Status` (`internal/app/status.go`). The one returned fact set replaces the current `domain.NewBranchFacts(nil)` and threads unchanged into `selectContextChange`, `domain.EvaluateReadiness`, and `domain.ResolveEffectiveBase` (those three call sites already take a `facts` parameter — only its source changes). A facts-read error is classified through the existing `classifyStatusError` and returns no bundle. No domain rule, readiness precedence, or ADR changes.

**Tech Stack:** Go (`internal/app` orchestration layer), existing `fakeReader` test seam (`internal/app/status_test.go`), existing real-Git harness (`planRepoModes` / `planningDepsFor` / bare-origin oracles from `internal/app/planning_git_test.go` and `internal/app/status_git_test.go`).

**Spec:** `docs/superpowers/specs/2026-08-26-implementation-context-remote-branch-facts-design.md` (lives on the `docket` metadata branch, not this feature worktree; the change record is `docs/changes/active/0357-implementation-context-loads-remote-branch-facts.md` there).

## Global Constraints

- Production change is confined to `internal/app/implementation_context.go`. Tests go in `internal/app/implementation_context_test.go` and `internal/app/claim_workflow_git_test.go`. Nothing else changes except comments whose premise this change flips.
- Do NOT touch `domain.ResolveEffectiveBase`, readiness precedence, ADR-0092, `stack-base.sh`, or any other `NewBranchFacts(nil)` site (reclaim/finalize paths are out of scope per the spec).
- A failed `BranchFacts` read must return a typed operation failure (via `classifyStatusError`), never be swallowed into an empty fact set: an observation failure is not proven branch absence (learning `probe-error-is-not-clean-absence`).
- An empty fact set returned by the reader is a valid answer ("none of the requested branches exists") and must still refuse a stacked child as `stack-base-unresolved` — skipping the read is not equivalent to an empty result.
- Every test run and mutation probe uses `-count=1` — Go's test cache can hand back a pre-mutation verdict (learning `cached-runner-serves-a-mutated-tree`).
- Guards are code: the real-Git regression must be mutation-tested by reverting the new call to `domain.NewBranchFacts(nil)` and watching it redden at the pre-claim gate (repo AGENTS.md, spec "Removing or bypassing the new `BranchFacts` call must make this regression fail").
- Comments cross-reference symbol names or verbatim-quoted clauses, never line numbers (`tests/test_comment_anchor_style.sh`).
- The full suite gate is `finalize.test_command` → `scripts/run-tests.sh`; the build workflow (docket-build) runs it at its end gate — do not substitute a narrower run for that gate.

---

### Task 1: Load branch facts in ContextImplementation, with fake-reader coverage

**Files:**
- Modify: `internal/app/implementation_context.go` (the `facts := domain.NewBranchFacts(nil)` seam inside `ContextImplementation` and its stale comment)
- Test: `internal/app/implementation_context_test.go`

**Interfaces:**
- Consumes: `fakeReader` (from `internal/app/status_test.go` — fields `facts domain.BranchFacts`, `factsErr error`, recorders `branchAsks [][]string`, `seenPins []StatusPin`); `ErrStatusExternal`, `ReasonStatusExternal`, `classifyStatusError`, `stackBranches` (all `internal/app/status.go`); `domain.NewBranchFacts`, `domain.ReadyBuildReady`, `domain.ReadyStackBaseUnresolved`, `domain.BaseResolved`.
- Produces: `ContextImplementation` now calls `deps.Reader.BranchFacts(ctx, pin, stackBranches(snap))` exactly once on every success path; `liveParentBlob` test fixture helper reused by no other task (the real-Git task uses its own fixtures).

- [ ] **Step 1: Write the failing tests**

Add to `internal/app/implementation_context_test.go`. First a fixture helper for a live (claimed) parent record — `changeBlob` hardcodes `status: proposed`, so a claimed parent needs its own builder (place it next to `learningBlob` in the fixtures section):

```go
// liveParentBlob builds an in-progress (claimed) change record carrying a
// recorded feature branch — the live stack parent the facts-backed effective
// base resolver consults (domain rule 4: a live parent resolves to its
// recorded branch only when that branch is present in facts).
func liveParentBlob(id int, slug, branch string) StatusBlob {
	fm := fmt.Sprintf("---\nid: %d\nslug: %s\ntitle: Change %d\nstatus: 'in-progress'\npriority: medium\ntype: feat\ncreated: 2026-01-02\nbranch: '%s'\nclaimed_at: '2026-01-03T00:00:00Z'\nreconciled: true\n---\n\nBody of %d.\n",
		id, slug, id, branch, id)
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     fmt.Sprintf("docs/changes/active/%04d-%s.md", id, slug),
		Version:  fmt.Sprintf("blobchange%04d", id),
		Data:     []byte(fm),
	}
}
```

Then three tests (spec "Focused orchestration tests" items 1–4):

```go
// TestContextImplementationStackedLiveParentUsesRemoteFacts: a proposed,
// designed child stacked on a live parent whose recorded branch IS in the
// reader's facts gets an applied bundle — build-ready, claim-eligible, and an
// effective base resolved to the parent's recorded branch — through BOTH
// automatic selection and explicit-id inspection. The reader is asked for
// exactly the deterministic stackBranches set, once, with the original pin.
func TestContextImplementationStackedLiveParentUsesRemoteFacts(t *testing.T) {
	pin := docketPin(t)
	specPath := "docs/changes/specs/spec-child.md"
	corpus := []StatusBlob{
		liveParentBlob(20, "parent", "feat/parent"),
		changeBlob(21, "child", "feat", "high", "spec: "+specPath+"\nstacked_on: 20\n"),
	}
	for name, req := range map[string]ImplementationContextRequest{
		"automatic-selection": {},
		"explicit-id":         {ID: 21},
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeReader{
				pin:    pin,
				corpus: corpus,
				facts:  domain.NewBranchFacts(map[string]bool{"feat/parent": true}),
				artifactData: map[string]StatusArtifact{
					sourceMetadata + "|" + specPath: {Found: true, Version: "sc", Data: []byte("spec child\n")},
				},
			}
			got := ContextImplementation(context.Background(), contextDeps(fake), "", req)
			if got.Result != ResultApplied || got.Context == nil {
				t.Fatalf("result=%q reason=%q message=%q", got.Result, got.Reason, got.Message)
			}
			b := got.Context
			if b.Change.Summary == nil || b.Change.Summary.ID != 21 {
				t.Fatalf("selected change = %+v, want the stacked child 21", b.Change.Summary)
			}
			if b.Readiness != string(domain.ReadyBuildReady) {
				t.Errorf("readiness = %q, want build-ready", b.Readiness)
			}
			if !b.ClaimEligible || b.ClaimRefusal != "" {
				t.Errorf("claim eligibility = %v / %q, want eligible", b.ClaimEligible, b.ClaimRefusal)
			}
			if b.EffectiveBase.Kind != string(domain.BaseResolved) || b.EffectiveBase.Branch != "feat/parent" {
				t.Errorf("effective base = %+v, want resolved/feat/parent (the parent's recorded branch, not the integration branch)", b.EffectiveBase)
			}
			// The reader was asked exactly once, for the deterministic
			// stackBranches(snapshot) set, with the original pin threaded.
			if len(fake.branchAsks) != 1 {
				t.Fatalf("BranchFacts called %d times, want exactly 1 (asks: %v)", len(fake.branchAsks), fake.branchAsks)
			}
			if want := []string{"feat/parent"}; !reflect.DeepEqual(fake.branchAsks[0], want) {
				t.Errorf("BranchFacts asked for %v, want %v", fake.branchAsks[0], want)
			}
			for i, seen := range fake.seenPins {
				if !reflect.DeepEqual(seen, pin) {
					t.Errorf("post-pin call %d threaded a different pin", i)
				}
			}
		})
	}
}

// TestContextImplementationStackedParentBranchAbsent: the SAME stacked child
// with the parent branch absent from the returned facts remains refused —
// an empty fact set is a valid "branch does not exist" answer, and docket
// must not claim a child from an invented fallback base. Explicit id refuses
// as not-ready-stack-base-unresolved; automatic selection skips it entirely.
func TestContextImplementationStackedParentBranchAbsent(t *testing.T) {
	pin := docketPin(t)
	specPath := "docs/changes/specs/spec-child.md"
	corpus := []StatusBlob{
		liveParentBlob(20, "parent", "feat/parent"),
		changeBlob(21, "child", "feat", "high", "spec: "+specPath+"\nstacked_on: 20\n"),
	}
	art := map[string]StatusArtifact{
		sourceMetadata + "|" + specPath: {Found: true, Version: "sc", Data: []byte("spec child\n")},
	}

	t.Run("explicit-id", func(t *testing.T) {
		fake := &fakeReader{pin: pin, corpus: corpus, facts: domain.NewBranchFacts(nil), artifactData: art}
		got := ContextImplementation(context.Background(), contextDeps(fake), "", ImplementationContextRequest{ID: 21})
		if got.Result != ResultInvalidState || got.Reason != "not-ready-"+string(domain.ReadyStackBaseUnresolved) {
			t.Errorf("result=%q reason=%q, want invalid-state/not-ready-stack-base-unresolved", got.Result, got.Reason)
		}
		if got.Context != nil {
			t.Errorf("refusal fabricated a bundle: %+v", got.Context)
		}
	})

	t.Run("automatic-selection", func(t *testing.T) {
		fake := &fakeReader{pin: pin, corpus: corpus, facts: domain.NewBranchFacts(nil), artifactData: art}
		got := ContextImplementation(context.Background(), contextDeps(fake), "", ImplementationContextRequest{})
		if got.Result != ResultNoOp || got.Reason != ReasonContextNoCandidate {
			t.Errorf("result=%q reason=%q, want no-op/no-candidate (the unresolved child must not be selected)", got.Result, got.Reason)
		}
	})
}

// TestContextImplementationBranchFactsFailure: a failed facts lookup is a
// typed operation failure through classifyStatusError, never an empty fact
// set — an observation failure must not be misreported as proven branch
// absence (learning probe-error-is-not-clean-absence). No partial bundle.
func TestContextImplementationBranchFactsFailure(t *testing.T) {
	pin := docketPin(t)
	specPath := "docs/changes/specs/spec-child.md"
	fake := &fakeReader{
		pin: pin,
		corpus: []StatusBlob{
			changeBlob(11, "alpha", "feat", "high", "spec: "+specPath+"\n"),
		},
		factsErr: fmt.Errorf("git ls-remote: connection reset: %w", ErrStatusExternal),
		artifactData: map[string]StatusArtifact{
			sourceMetadata + "|" + specPath: {Found: true, Version: "sa", Data: []byte("spec a\n")},
		},
	}
	got := ContextImplementation(context.Background(), contextDeps(fake), "", ImplementationContextRequest{})
	if got.Result != ResultExternalFailed || got.Reason != ReasonStatusExternal {
		t.Errorf("result=%q reason=%q, want external-failed/external-failed", got.Result, got.Reason)
	}
	if got.Context != nil {
		t.Errorf("failed facts read fabricated a bundle: %+v", got.Context)
	}
}
```

Add `"reflect"` to the test file's imports (it is not imported there yet; `fmt`, `strings`, `context`, `domain`, `repository` already are).

Note on the existing tests: the unstacked tests (`TestContextImplementationSelectsByPolicy` etc.) leave `fake.facts` as the zero `domain.BranchFacts` — with the new read they receive that zero value from `BranchFacts` instead of constructing it locally, and an unstacked change resolves to the integration branch regardless of facts, so they must keep passing unchanged. The existing `"unresolved effective base"` case in `TestContextImplementationTypedAbsence` (proposed parent, no recorded branch) also still refuses — do not delete or re-gate it; it guards a different rule (a branchless parent, not an absent remote ref) (learning `test-premise-deleted-not-regated`).

- [ ] **Step 2: Run the new tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestContextImplementationStackedLiveParentUsesRemoteFacts|TestContextImplementationStackedParentBranchAbsent|TestContextImplementationBranchFactsFailure' -count=1 -v`

Expected: `TestContextImplementationStackedLiveParentUsesRemoteFacts` FAILS — the operation still constructs empty facts, so the stacked child is refused (`not-ready-stack-base-unresolved` / `no-candidate`) instead of applied, and `fake.branchAsks` is empty (BranchFacts never called). `TestContextImplementationBranchFactsFailure` also FAILS (the error is never surfaced; the bundle applies). `TestContextImplementationStackedParentBranchAbsent` may already pass (the pre-fix behavior coincides) — that is expected; its red evidence comes from the Task 2 mutation probe plus the `branchAsks` assert in the first test.

- [ ] **Step 3: Implement the fix**

In `internal/app/implementation_context.go`, inside `ContextImplementation`, replace this block (the empty-facts construction and its stale comment, currently between the `blobByPath` loop and the `selectContextChange` call):

```go
	// The context bundle resolves the effective base from the pinned snapshot
	// alone; remote-branch existence is not re-read here (the authoritative,
	// facts-backed resolution happens inside the claim transaction). A stacked
	// change whose base needs a live remote branch therefore reports an
	// unresolved-base readiness rather than a fabricated resolution.
	facts := domain.NewBranchFacts(nil)
```

with:

```go
	// One facts read from the same pin drives the whole decision: automatic
	// selection, explicit-id eligibility, readiness, and the reported
	// effective base all consume this single fact set, so the base whose
	// resolution licensed the bundle is the one the bundle reports. Claim
	// still re-reads the corpus and branch facts inside its transaction and
	// re-proves eligibility there; this read only supplies the pre-claim gate
	// with real remote evidence instead of a fabricated empty set. A failed
	// lookup is a typed failure, never an empty fact set — an observation
	// failure must not be misreported as proven branch absence.
	facts, err := deps.Reader.BranchFacts(ctx, pin, stackBranches(snap))
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newContextResult(result, reason, err.Error(), nil)
	}
```

Everything downstream (`selectContextChange(snap, facts, req)`, `domain.EvaluateReadiness(snap, selected, facts)`, `domain.ResolveEffectiveBase(snap, selected, facts)`) is unchanged — only the source of `facts` changes. If the `domain` import becomes otherwise unused it will not (it is used throughout the file); do not touch the import block.

- [ ] **Step 4: Sweep for other comments whose premise this flips**

Run: `grep -rn -i "empty fact\|not re-read\|NewBranchFacts(nil)" internal/ --include='*.go'`

Sort hits into (a) the site just edited, (b) out-of-scope `NewBranchFacts(nil)` sites this change deliberately leaves alone (reclaim/finalize paths — leave them and their comments untouched), and (c) any comment in `internal/app/implementation_context.go`, `internal/app/implementation_context_test.go`, or `internal/cli/` that still claims implementation context deliberately uses empty facts — correct only (c). At planning time no (c) hits existed outside the block replaced in Step 3, but the grep is the authority, not this sentence (repo AGENTS.md: derive sites from a whole-repo grep, never a hand-list).

- [ ] **Step 5: Run the focused package tests**

Run: `go test ./internal/app/ -run TestContextImplementation -count=1 -v`

Expected: ALL PASS — the three new tests and every pre-existing `TestContextImplementation*` test (unstacked selection, explicit id, mint prefix, revision consistency, typed absences, learnings capability, redaction, halt).

- [ ] **Step 6: Commit**

```bash
git add internal/app/implementation_context.go internal/app/implementation_context_test.go
git commit -m "fix(context): load remote branch facts before judging stack base

ContextImplementation evaluated every candidate with an empty BranchFacts
set, so a child stacked on a live parent with a pushed recorded branch was
refused as stack-base-unresolved before claim could run. Load facts once
via deps.Reader.BranchFacts from the same pin and thread the one fact set
through selection, readiness, and effective-base reporting; classify a
failed lookup as a typed failure instead of a false absence."
```

(Include any file corrected in Step 4 in the `git add` list. Stage the named paths only — never `git add -A`; other agents may share the tree.)

---

### Task 2: Real-Git workflow regression — stacked child passes the gate, claims, and builds from the parent branch

**Files:**
- Test: `internal/app/claim_workflow_git_test.go` (the real-Git planning/claim workflow area; reuses that file's harness — `requireRealGit`, `planRepoModes`, `planningDepsFor`, `groomPath`, `buildReadyChange`, `lifecycleChange`, `originTip`, `blobVersionAt`, `runGit`, `writeRepoFile` — and invents no new harness)

**Interfaces:**
- Consumes: production `ContextImplementation`, `ChangeClaim`, `WorkspacePrepare` through `NewGitStatusReader` (wired by `planningDepsFor`); `workspace.NewService` (already imported in this file); Task 1's landed fix.
- Produces: `TestStackedContextClaimWorkspaceFromParentBranch` — the end-to-end proof the spec's "Real-Git workflow regression" section demands, in both metadata modes.

- [ ] **Step 1: Write the regression test**

Append to `internal/app/claim_workflow_git_test.go`:

```go
// --- 0357: stacked child passes the pre-claim gate on real remote facts ------

// TestStackedContextClaimWorkspaceFromParentBranch is the change-0357
// regression: a proposed, designed child stacked on a LIVE parent whose
// recorded branch is pushed to the origin must pass the pre-claim
// implementation-context gate (both automatic selection and explicit id),
// resolve its effective base to the exact recorded parent branch, be claimed
// under claim's independent in-transaction re-proof, and prepare its
// workspace at the parent branch tip — not the integration-branch tip.
// Pre-0357 ContextImplementation judged the stack base against an empty
// fact set, so this exact topology was refused stack-base-unresolved before
// claim could run. Reverting ContextImplementation's BranchFacts read to
// domain.NewBranchFacts(nil) must make this test fail at the pre-claim gate.
// It joins TestEffectiveBaseConsumedFromDomain (which starts PAST the gate,
// from an already-claimed child) rather than replacing it. Both metadata
// modes; the git reader is the production NewGitStatusReader.
func TestStackedContextClaimWorkspaceFromParentBranch(t *testing.T) {
	requireRealGit(t)
	const (
		parentID   = 20
		parentSlug = "parent"
		childID    = 21
		childSlug  = "child"
	)
	parentRec := groomPath(parentID, parentSlug)
	childRec := groomPath(childID, childSlug)
	// The child is proposed + designed (trivial: true stands in for a spec,
	// as in buildReadyChange) and stacked on the live parent.
	childBody := strings.Replace(buildReadyChange(childID, childSlug),
		"stacked_on:\n", "stacked_on: 20\n", 1)

	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				parentRec: lifecycleChange(parentID, parentSlug, "in-progress"),
				childRec:  childBody,
			})
			ctx := context.Background()

			// The parent's recorded branch exists on the origin, advanced past
			// the integration branch — the live-parent topology of domain rule 4.
			runGit(t, repo.writer, "checkout", "-q", "-B", "feat/"+parentSlug, "origin/main")
			writeRepoFile(t, repo.writer, "parent-work.txt", "parent feature work\n")
			runGit(t, repo.writer, "add", "-A")
			runGit(t, repo.writer, "commit", "-q", "-m", "parent feature work")
			runGit(t, repo.writer, "push", "-q", "origin", "feat/"+parentSlug)
			parentTip := originTip(t, repo.origin, "refs/heads/feat/"+parentSlug)
			mainTip := originTip(t, repo.origin, "main")
			if parentTip == mainTip {
				t.Fatalf("parent branch did not advance past main; the base contrast is vacuous")
			}

			node := planningDepsFor(t, repo.invocation)

			// The pre-claim gate applies for BOTH request shapes — this is the
			// gate that pre-0357 refused stack-base-unresolved.
			for name, req := range map[string]ImplementationContextRequest{
				"automatic-selection": {},
				"explicit-id":         {ID: childID},
			} {
				got := ContextImplementation(ctx, node.deps, node.dir, req)
				if got.Result != ResultApplied || got.Context == nil {
					t.Fatalf("%s context = %q (reason %q msg %q); want an applied bundle", name, got.Result, got.Reason, got.Message)
				}
				if got.Context.Change.Summary == nil || got.Context.Change.Summary.ID != childID {
					t.Fatalf("%s selected %+v, want the stacked child %d", name, got.Context.Change.Summary, childID)
				}
				if !got.Context.ClaimEligible {
					t.Fatalf("%s bundle not claim-eligible: %q", name, got.Context.ClaimRefusal)
				}
				if got.Context.EffectiveBase.Branch != "feat/"+parentSlug {
					t.Fatalf("%s effective base = %+v, want the exact recorded parent branch feat/%s", name, got.Context.EffectiveBase, parentSlug)
				}
			}

			// Claim succeeds under its own in-transaction, facts-backed re-proof.
			bundle := ContextImplementation(ctx, node.deps, node.dir, ImplementationContextRequest{ID: childID})
			if bundle.Result != ResultApplied || bundle.Context == nil {
				t.Fatalf("claim context read = %q (reason %q)", bundle.Result, bundle.Reason)
			}
			claim := ChangeClaim(ctx, node.deps, node.dir, ChangeClaimRequest{ID: childID, Version: bundle.Context.Change.Version})
			if claim.Result != ResultApplied || claim.Disposition != ClaimDispositionApplied {
				t.Fatalf("claim = (%q, %q), want applied/applied (findings %v)", claim.Result, claim.Disposition, claim.Findings)
			}

			// Workspace preparation uses the parent branch tip, not the
			// integration-branch tip — the child builds on the parent's
			// unmerged work.
			version := blobVersionAt(t, repo.origin, m.branch, childRec)
			svc, err := workspace.NewService(node.deps.Client)
			if err != nil {
				t.Fatalf("workspace.NewService: %v", err)
			}
			wdeps := WorkspaceDeps{Service: svc}
			prep := WorkspacePrepare(ctx, node.deps, wdeps, repo.invocation, WorkspaceIDRequest{ID: childID, Version: version})
			if prep.Result != ResultApplied {
				t.Fatalf("prepare workspace = %q (reason %q msg %q)", prep.Result, prep.Reason, prep.Message)
			}
			if prep.BaseCommit != parentTip {
				t.Fatalf("prepared base = %q, want the parent branch tip %q", prep.BaseCommit, parentTip)
			}
			if prep.BaseCommit == mainTip {
				t.Fatalf("prepared base is the integration-branch tip; the parent's unmerged work was lost")
			}
		})
	}
}
```

No new imports are needed — `context`, `strings`, `testing`, and `workspace` are already imported in this file.

- [ ] **Step 2: Run the regression to verify it passes on the fixed code**

Run: `go test ./internal/app/ -run TestStackedContextClaimWorkspaceFromParentBranch -count=1 -v`

Expected: PASS in both mode subtests (`main` and `docket`). This test is written after Task 1's fix landed, so it cannot go red naturally here — Step 3 supplies the required red evidence.

- [ ] **Step 3: Mutation-test the regression against the original defect**

The spec requires: "Removing or bypassing the new `BranchFacts` call must make this regression fail at the original pre-claim gate." Task 1's commit is already on the branch, so `git checkout --` restores exactly the fixed state (safe here — never use `checkout --` to restore uncommitted work, learning `mutation-restore-needs-a-backup-copy`):

1. In `internal/app/implementation_context.go`, replace the `facts, err := deps.Reader.BranchFacts(ctx, pin, stackBranches(snap))` call and its error branch with `facts := domain.NewBranchFacts(nil)` (the pre-0357 defect).
2. Run: `go test ./internal/app/ -run 'TestStackedContextClaimWorkspaceFromParentBranch|TestContextImplementationStackedLiveParentUsesRemoteFacts' -count=1`
3. Expected: FAIL — the git regression at the pre-claim gate (context refused, reason `not-ready-stack-base-unresolved` on explicit-id / `no-candidate` on automatic selection, before any claim), and the fake-reader test on the missing applied bundle and empty `branchAsks`. If either stays green, the guard is decoration: stop and fix the test.
4. Restore: `git checkout -- internal/app/implementation_context.go`
5. Re-run the same command with `-count=1`; expected: PASS (confirms the restore, defeating the test cache).

- [ ] **Step 4: Run the focused app package**

Run: `go test ./internal/app/ -count=1`

Expected: PASS. (The full repository suite — `finalize.test_command` → `scripts/run-tests.sh` — runs at the build workflow's end gate as always; this step is the per-task package check, not a substitute for that gate.)

- [ ] **Step 5: Commit**

```bash
git add internal/app/claim_workflow_git_test.go
git commit -m "test(context): real-git regression for stacked pre-claim gate on remote facts

A proposed child stacked on a live parent with a pushed recorded branch
passes implementation context (both request shapes), resolves the exact
parent branch, claims under the in-transaction re-proof, and prepares its
workspace at the parent branch tip. Mutation-verified: reverting the
BranchFacts read to an empty fact set fails this test at the pre-claim
gate."
```

---

## Self-Review

- **Spec coverage:** Load-once from the same pin via `stackBranches(snap)` → Task 1 Step 3; one fact set threading selection/readiness/base → Task 1 Steps 1+3 (the `branchAsks`/pin asserts pin single-read and threading); empty-facts construction and stale comment removed → Task 1 Steps 3–4; facts error through `classifyStatusError`, no bundle → Task 1 (failure test + implementation); preserved stack semantics (absent branch still unresolved; unstacked unchanged) → Task 1 (absent-branch test, existing tests kept); focused tests items 1–4 → Task 1 Step 1; real-Git regression incl. claim + workspace + mutation requirement → Task 2; acceptance criteria all map to the above; no domain/ADR change anywhere.
- **Placeholder scan:** all test and production code is written verbatim; no TBDs.
- **Type consistency:** `liveParentBlob(id int, slug, branch string) StatusBlob` used only in Task 1; Task 2 uses only pre-existing helpers with their existing signatures; `facts, err :=` is valid Go at that scope (`facts` is new; `err` redeclared in the multi-assign).
