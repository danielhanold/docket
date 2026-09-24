//go:build integration

package app

import (
	"bytes"
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/testsupport"
	"github.com/danielhanold/docket/internal/workspace"
)

// Change 0449 acceptance test 2: complete named workflows end to end. One
// isolated repository per flow carries, for the whole run, the unrelated
// damage the named change B must be able to route around:
//
//   - an unparseable change record A (unrelatedBrokenPath) — invisible to the
//     snapshot, surfaced on the board's repair notice;
//   - an unrelated stacked pair whose parent records an INVALID branch name
//     ("feat/a..parent"), so a whole-corpus live branch-fact probe fails where
//     the bounded, named probe (stackBranchesFor) never asks for it;
//   - mixed old runtime records in the repository's gate registry — a corrupt
//     drive record, an unsupported-schema drive, a HALTED drive bound to a
//     removed worktree, and a corrupt run epoch (change 0446's history class).
//
// Every named step drives the production operation through the production
// engine, loader, board renderer, and git status reader; only the GitHub and
// gate seams are fakes. After every metadata write the test proves the write
// applied, A's blob id on the metadata ref is unchanged, and the board carries
// B's row and the repair notice naming A — atomically, in the applied commit.
//
// Mutation checks (change 0449 Task 10 sweep): restoring the engine's
// unconditional before- or after-gate refusal, dropping the board's
// caller-supplied repair entries, dropping one named operation's Scope, or a
// whole-corpus stackBranches probe on a named caller (context, claim,
// workspace prepare, clear-block) reddens the flow tests; deleting the
// relevance check reddens the defective-dependency refusal.

// namedIsolationInvalidBranch is the unrelated stack parent's recorded branch:
// not a valid ref name, so a live probe of it fails as an external error.
const namedIsolationInvalidBranch = "feat/a..parent"

// namedIsolationUnrelatedStack renders the unrelated stacked pair (20 carrying
// the invalid branch, 21 stacked on it) that only a whole-corpus probe reaches.
func namedIsolationUnrelatedStack(t *testing.T) map[string]string {
	t.Helper()
	parent := strings.Replace(lifecycleChange(20, "a-parent", "in-progress"),
		"branch: feat/a-parent\n", "branch: '"+namedIsolationInvalidBranch+"'\n", 1)
	if !strings.Contains(parent, namedIsolationInvalidBranch) {
		t.Fatal("unrelated stack fixture did not record the invalid branch; the fixture shape changed")
	}
	return map[string]string{
		groomPath(20, "a-parent"): parent,
		groomPath(21, "a-child"):  stackedOn(lifecycleChange(21, "a-child", "proposed"), 20),
	}
}

// namedIsolationRuntime seeds mixed old runtime records into the gate registry
// under commonDir and returns a snapshot of their bytes, so the test can prove
// no named step consumed, repaired, or removed them.
func namedIsolationRuntime(t *testing.T, commonDir string) map[string][]byte {
	t.Helper()
	gone := filepath.Join(testsupport.TempDir(t), "gone")
	const pfx = "ee4490000000000000000000000000"
	writeCensusDriveBytes(t, commonDir, pfx+"01", []byte("{not json"))
	seedCensusDrive(t, commonDir, pfx+"02", map[string]any{"schema_version": 99, "worktree_path": filepath.Join(gone, "old")})
	seedCensusDrive(t, commonDir, pfx+"03", map[string]any{
		"worktree_path": filepath.Join(gone, "removed-worktree"), "raw_run_dir": filepath.Join(gone, "halted-run"),
		"last_outcome": string(gatedrive.HALTED), "last_cause": "stopped-not-initiated",
	})
	epochDir := filepath.Join(commonDir, "docket", "rungate", "old-damaged-unrelated-epoch")
	if err := os.MkdirAll(epochDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(epochDir, epochRecordFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, p := range []string{
		filepath.Join(commonDir, "docket", "gate-drives", "v1", pfx+"01", "record.json"),
		filepath.Join(commonDir, "docket", "gate-drives", "v1", pfx+"02", "record.json"),
		filepath.Join(commonDir, "docket", "gate-drives", "v1", pfx+"03", "record.json"),
		filepath.Join(epochDir, epochRecordFileName),
	} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read seeded runtime record: %v", err)
		}
		out[p] = b
	}
	return out
}

// assertRuntimeIntact proves every seeded runtime record is still present with
// its exact seeded bytes.
func assertRuntimeIntact(t *testing.T, seeded map[string][]byte) {
	t.Helper()
	for p, want := range seeded {
		got, err := os.ReadFile(p)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("old runtime record %s changed or vanished (err %v)", p, err)
		}
	}
}

// assertUnrelatedBytesIntact proves the unrelated broken record is
// byte-identical on the origin metadata ref and that the status pipeline's
// parse step still reports it as an error finding. (The whole-repository status
// read itself refuses here — its unbounded branch-fact probe reaches the
// unrelated invalid branch — which is why this does not go through Status.)
func assertUnrelatedBytesIntact(t *testing.T, repo *gitRepo, branch string) {
	t.Helper()
	got, ok := originFile(t, repo.origin, branch, unrelatedBrokenPath)
	if !ok || got != unrelatedBrokenBytes {
		t.Fatalf("unrelated broken record on origin = %q (present %v), want its exact seeded bytes", got, ok)
	}
	_, findings := parseCorpus([]StatusBlob{{
		Kind: repository.KindChange, Location: repository.LocationActive,
		Path: unrelatedBrokenPath, Version: "v", Data: []byte(got),
	}})
	for _, f := range findings {
		if f.Path == unrelatedBrokenPath && f.Severity == "error" {
			return
		}
	}
	t.Errorf("the unrelated record no longer parses to an error finding; findings %+v", findings)
}

// namedIsolationCheck is the per-step oracle: the unrelated broken record's
// blob id on the origin metadata ref is exactly its seeded id, and the board
// published on that ref carries B's row (recLink, with section's heading
// present) and the repair notice naming the broken record.
func namedIsolationCheck(t *testing.T, step string, repo *gitRepo, branch, brokenBlob, section, recLink string) {
	t.Helper()
	if got := runGit(t, repo.origin, "rev-parse", branch+":"+unrelatedBrokenPath); got != brokenBlob {
		t.Errorf("%s: unrelated broken record blob %s, want its seeded %s", step, got, brokenBlob)
	}
	board, ok := originFile(t, repo.origin, branch, "docs/changes/BOARD.md")
	if !ok {
		t.Fatalf("%s: no board on the metadata ref", step)
	}
	if !strings.Contains(board, section) || !strings.Contains(board, recLink) {
		t.Errorf("%s: board lacks B's row (%q under %q):\n%s", step, recLink, section, board)
	}
	if !strings.Contains(board, "| `"+unrelatedBrokenPath+"` | unclosed-frontmatter |") {
		t.Errorf("%s: board lacks the repair notice naming the unrelated record:\n%s", step, board)
	}
}

// TestIntegrationNamedImplementationFlowIsolation drives B by id through the
// implementation workflow — authoritative context, claim, reconcile, workspace
// prepare, plan attach, results attach, publish, mark-implemented — over a
// corpus that keeps the unrelated damage present throughout.
func TestIntegrationNamedImplementationFlowIsolation(t *testing.T) {
	requireRealGit(t)
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	planPath := "docs/superpowers/plans/2026-08-17-widget-plan.md"
	resultsPath := "docs/results/2026-08-17-widget-results.md"
	records := namedIsolationUnrelatedStack(t)
	records[recPath] = buildReadyChange(id, slug)
	records[unrelatedBrokenPath] = unrelatedBrokenBytes
	repo := newDocketModeRepo(t,
		map[string]string{".docket.yml": "integration_branch: main\nbuild:\n  test_command: 'go test ./...'\nfinalize:\n  test_command: 'go test ./...'\n"},
		records)
	runtime := namedIsolationRuntime(t, filepath.Join(repo.invocation, ".git"))
	brokenBlob := runGit(t, repo.origin, "rev-parse", "docket:"+unrelatedBrokenPath)
	baseTip := originTip(t, repo.origin, "docket")
	ctx := context.Background()

	node := planningDepsFor(t, repo.invocation)
	svc, err := workspace.NewService(node.deps.Client)
	if err != nil {
		t.Fatalf("workspace.NewService: %v", err)
	}
	wdeps := WorkspaceDeps{Service: svc}
	ver := func() string { return blobVersionAt(t, repo.origin, "docket", recPath) }
	recLink := "(active/" + path.Base(recPath) + ")"
	inProgress := "In progress ("

	// The whole-corpus probe genuinely fails on the unrelated invalid branch:
	// without that, the bounded-probe assertions below would be vacuous.
	pin, err := node.deps.Reader.PinContext(ctx, node.dir)
	if err != nil {
		t.Fatalf("pin: %v", err)
	}
	if _, err := node.deps.Reader.BranchFacts(ctx, pin, []string{namedIsolationInvalidBranch}); err == nil {
		t.Fatalf("probing the unrelated invalid branch %q succeeded; the fixture no longer poisons a whole-corpus probe", namedIsolationInvalidBranch)
	}

	// (1) Authoritative named context.
	ctxRes := ContextImplementation(ctx, node.deps, node.dir, ImplementationContextRequest{ID: id})
	if ctxRes.Result != ResultApplied || ctxRes.Context == nil {
		t.Fatalf("named context = %q (reason %q msg %q), want a bundle", ctxRes.Result, ctxRes.Reason, ctxRes.Message)
	}
	if !ctxRes.Context.ClaimEligible {
		t.Fatalf("named context reports B not claim-eligible: %q", ctxRes.Context.ClaimRefusal)
	}

	// (2) Claim.
	claim := ChangeClaim(ctx, node.deps, node.dir, ChangeClaimRequest{ID: id, Version: ctxRes.Context.Change.Version})
	if claim.Result != ResultApplied {
		t.Fatalf("claim = %q (disposition %q findings %v), want applied", claim.Result, claim.Disposition, claim.Findings)
	}
	namedIsolationCheck(t, "claim", repo, "docket", brokenBlob, inProgress, recLink)

	// (3) Reconcile.
	rec := ChangeReconcile(ctx, node.deps, node.dir, ChangeReconcileRequest{
		ID: id, Version: ver(), ReconcileLogEntry: "Reconciled against current reality.\n",
	})
	if rec.Result != ResultApplied {
		t.Fatalf("reconcile = %q (disposition %q findings %v), want applied", rec.Result, rec.Disposition, rec.Findings)
	}
	namedIsolationCheck(t, "reconcile", repo, "docket", brokenBlob, inProgress, recLink)

	// (4) Workspace prepare (a named branch-fact read).
	prep := WorkspacePrepare(ctx, node.deps, wdeps, node.dir, WorkspaceIDRequest{ID: id, Version: ver()})
	if prep.Result != ResultApplied {
		t.Fatalf("workspace prepare = %q (reason %q msg %q), want applied", prep.Result, prep.Reason, prep.Message)
	}
	wp := prep.Path

	// (5) Plan attach.
	writeRepoFile(t, wp, planPath, "# Implementation Plan\n\nConcrete steps here.\n")
	if bl := ArtifactBacklink(ctx, node.deps, wp, ArtifactBacklinkRequest{ArtifactPath: planPath, ChangePath: recPath}); bl.Result != ResultApplied {
		t.Fatalf("plan backlink = %q (reason %q msg %q)", bl.Result, bl.Reason, bl.Message)
	}
	planHead := commitPlanFile(t, wp, planPath, string(mustReadFile(t, filepath.Join(wp, planPath))), planPath)
	attach := ChangeAttachPlan(ctx, node.deps, wdeps, node.dir, ChangeAttachRequest{ID: id, Version: ver(), Path: planPath, Commit: planHead})
	if attach.Result != ResultApplied {
		t.Fatalf("attach plan = %q (reason %q msg %q findings %v), want applied", attach.Result, attach.Reason, attach.Message, attach.Findings)
	}
	namedIsolationCheck(t, "attach-plan", repo, "docket", brokenBlob, inProgress, recLink)

	// (6) Implementation + results attach.
	writeRepoFile(t, wp, "widget.go", "package widget\n")
	writeRepoFile(t, wp, resultsPath, "# Widget — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nDelivered the widget end to end; the gate certifies this head.\n")
	if bl := ArtifactBacklink(ctx, node.deps, wp, ArtifactBacklinkRequest{ArtifactPath: resultsPath, ChangePath: recPath}); bl.Result != ResultApplied {
		t.Fatalf("results backlink = %q (reason %q msg %q)", bl.Result, bl.Reason, bl.Message)
	}
	runGit(t, wp, "add", "-A")
	runGit(t, wp, "commit", "-q", "-m", "implement the widget")
	head := runGit(t, wp, "rev-parse", "HEAD")
	attachR := ChangeAttachResults(ctx, node.deps, wdeps, node.dir, ChangeAttachRequest{ID: id, Version: ver(), Path: resultsPath, Commit: head})
	if attachR.Result != ResultApplied {
		t.Fatalf("attach results = %q (reason %q msg %q findings %v), want applied", attachR.Result, attachR.Reason, attachR.Message, attachR.Findings)
	}
	namedIsolationCheck(t, "attach-results", repo, "docket", brokenBlob, inProgress, recLink)

	// (7) Publish the feature head, then mark implemented against a scripted
	// PR at that head carrying green evidence (the GitHub seam is the fake).
	if pub := WorkspacePublish(ctx, node.deps, wdeps, node.dir, WorkspacePublishRequest{ID: id, Head: head}); pub.Result != ResultApplied {
		t.Fatalf("workspace publish = %q (reason %q msg %q), want applied", pub.Result, pub.Reason, pub.Message)
	}
	gdeps := GitHubDeps{Service: &fakeGitHub{repo: prRepo(), probePRs: []githubcli.PullRequest{happyPR(head)}}}
	mi := ChangeMarkImplemented(ctx, node.deps, wdeps, gdeps, node.dir, MarkImplementedRequest{
		ID: id, Version: ver(), Head: head, PR: prRepo().Spec() + "#42", EvidenceRecord: prEvidenceBytes(t, head),
	})
	if mi.Result != ResultApplied || mi.Status != "implemented" {
		t.Fatalf("mark implemented = %q status %q (findings %v), want applied implemented", mi.Result, mi.Status, mi.Findings)
	}
	namedIsolationCheck(t, "mark-implemented", repo, "docket", brokenBlob, "Built (1)", recLink)

	// Every metadata commit was an engine transaction of exactly the named
	// steps; the unrelated findings stay visible; the runtime records survive.
	assertEngineOnlyMetadataCommits(t, repo.origin, "docket", baseTip,
		[]string{"change.attach-plan", "change.attach-results", "change.claim", "change.mark-implemented", "change.reconcile"})
	assertUnrelatedBytesIntact(t, repo, "docket")
	assertRuntimeIntact(t, runtime)
}

// TestIntegrationNamedClaimRefusesDefectiveDependency is the implementation
// flow's paired refusal on a record B actually requires: B depends on a done
// change D whose own record carries a validation error. D is structural (a
// depends_on target), so the error is relevant and the claim refuses before any
// effect — even though the claim's plan never writes D, so only relevance (not
// the after-gate's changed-blob rule) can catch it.
func TestIntegrationNamedClaimRefusesDefectiveDependency(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	depPath := "docs/changes/archive/2026-08-01-0007-dep.md"
	b := strings.Replace(buildReadyChange(id, "widget"), "depends_on: []\n", "depends_on: [7]\n", 1)
	dep := strings.Replace(fixtureArchivedDone(7, "dep"), "type: feat\n", "type: 'Not A Token'\n", 1)
	if !strings.Contains(b, "depends_on: [7]") || !strings.Contains(dep, "Not A Token") {
		t.Fatal("dependency refusal fixtures did not rewrite their records; the fixture shape changed")
	}
	records := namedIsolationUnrelatedStack(t)
	records[recPath] = b
	records[depPath] = dep
	records[unrelatedBrokenPath] = unrelatedBrokenBytes
	repo := newWorkingRepo(t, records)
	node := planningDepsFor(t, repo.invocation)
	tip := originTip(t, repo.origin, "docket")

	res := ChangeClaim(context.Background(), node.deps, node.dir,
		ChangeClaimRequest{ID: id, Version: blobVersionAt(t, repo.origin, "docket", recPath)})
	if res.Result == ResultApplied {
		t.Fatalf("claim applied although B's dependency %s carries an error; want a refusal", depPath)
	}
	named := false
	for _, f := range res.Findings {
		if f.Path == depPath {
			named = true
		}
	}
	if !named {
		t.Errorf("refusal does not name the defective dependency %s: %q %q findings %+v", depPath, res.Result, res.Disposition, res.Findings)
	}
	if got := originTip(t, repo.origin, "docket"); got != tip {
		t.Errorf("a refused claim moved the metadata branch %s -> %s", tip, got)
	}
}

// TestIntegrationNamedFinalizeFlowIsolation drives B's named finalize path —
// block, clear-block, merge (landed out of band: merge-already-landed
// recovery), archive closeout, and cleanup — with the unrelated stack and old
// runtime records present throughout and A's corruption introduced BETWEEN the
// merge and closeout. The paired refusal corrupts B's own record at the same
// point: closeout refuses before any archive effect.
func TestIntegrationNamedFinalizeFlowIsolation(t *testing.T) {
	requireRealGit(t)

	// setup builds the implemented fixture with the unrelated stack and runtime
	// records, then blocks and clears the block by id.
	setup := func(t *testing.T) (*closeoutFixture, map[string][]byte) {
		t.Helper()
		f := setupCloseoutFixture(t, planRepoModeDocket())
		f.repo.writerAdvance(t, f.branch, namedIsolationUnrelatedStack(t))
		runtime := namedIsolationRuntime(t, f.gitrepo.CommonDir)
		ctx := context.Background()

		blockGH := &fakeBlockGitHub{repo: retargetRepo(), commentOutcome: githubcli.CommentCreated, commentURL: "https://example.test/c/9"}
		block := FinalizeBlock(ctx, FinalizeDeps{Planning: f.deps, GitHub: blockGH, Workspace: f.svc}, f.repo.invocation, BlockRequest{
			ID: f.id, Version: f.version, PRNumber: closeoutPR, Attempt: "att1", Reason: "gate-repair-required",
			Head: f.head, Report: "The gate failed.\n", Remedy: "Fix and retry.\n",
		})
		if block.Result != ResultApplied || block.Disposition != BlockDispRecorded {
			t.Fatalf("finalize block = %q disp %q reason %q msg %q (findings %v), want applied recorded",
				block.Result, block.Disposition, block.Reason, block.Message, block.Findings)
		}

		pr := f.prForHead(f.head, greenEvidenceFor(t, f.head))
		pr.Number = closeoutPR
		clearGH := &fakeBlockGitHub{repo: retargetRepo(), openByHead: map[string][]githubcli.PullRequest{"feat/" + f.slug: {pr}}}
		clear := FinalizeClearBlock(ctx, FinalizeDeps{Planning: f.deps, GitHub: clearGH, Workspace: f.svc}, f.repo.invocation, ClearBlockRequest{
			ID: f.id, Version: blobVersionAt(t, f.repo.origin, f.branch, groomPath(f.id, f.slug)), Head: f.head, PRNumber: closeoutPR,
		})
		if clear.Result != ResultApplied || clear.Disposition != BlockDispCleared {
			t.Fatalf("finalize clear-block = %q disp %q reason %q msg %q (findings %v), want applied cleared",
				clear.Result, clear.Disposition, clear.Reason, clear.Message, clear.Findings)
		}
		return f, runtime
	}

	t.Run("closeout-and-cleanup-apply", func(t *testing.T) {
		f, runtime := setup(t)
		mergeCommit := f.mergeIntoBase(t)
		advanceDocketOrigin(t, f.repo, map[string]string{unrelatedBrokenPath: unrelatedBrokenBytes})
		brokenBlob := runGit(t, f.repo.origin, "rev-parse", f.branch+":"+unrelatedBrokenPath)

		res := FinalizeCloseout(context.Background(), f.closeoutDeps(f.baselineMergedFake(f.head, mergeCommit)), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
			t.Fatalf("closeout = %q disp %q (reason %q msg %q findings %v), want applied done-archived",
				res.Result, res.Disposition, res.Reason, res.Message, res.Findings)
		}
		if _, ok := originFile(t, f.repo.origin, f.branch, res.ArchivePath); !ok {
			t.Fatalf("archived record absent at %q", res.ArchivePath)
		}
		namedIsolationCheck(t, "closeout", f.repo, f.branch, brokenBlob, "Archive — ", "](archive/"+path.Base(res.ArchivePath)+")")

		cleanup := FinalizeCleanup(context.Background(), f.cleanupDeps(f.mergedCleanupFake(f.head, mergeCommit), f.deps.Client, f.svc), f.repo.invocation, f.id)
		if cleanup.Disposition != CleanupDispCleaned {
			t.Fatalf("cleanup = %q disp %q (reason %q msg %q findings %v), want cleaned",
				cleanup.Result, cleanup.Disposition, cleanup.Reason, cleanup.Message, cleanup.Findings)
		}
		if _, err := os.Stat(f.wp); !os.IsNotExist(err) {
			t.Errorf("cleanup left the feature workspace scratch at %s (stat err %v)", f.wp, err)
		}
		if f.localBranchPresent(t) {
			t.Errorf("cleanup left the local feature branch")
		}
		assertUnrelatedBytesIntact(t, f.repo, f.branch)
		assertRuntimeIntact(t, runtime)
	})

	t.Run("defective-B-refuses-before-archive", func(t *testing.T) {
		f, runtime := setup(t)
		mergeCommit := f.mergeIntoBase(t)
		recPath := groomPath(f.id, f.slug)
		cur, _ := originFile(t, f.repo.origin, f.branch, recPath)
		bad := strings.Replace(cur, "type: feat\n", "type: 'Not A Token'\n", 1)
		if bad == cur {
			t.Fatal("B defect fixture did not rewrite the record; the fixture shape changed")
		}
		advanceDocketOrigin(t, f.repo, map[string]string{unrelatedBrokenPath: unrelatedBrokenBytes, recPath: bad})
		tip := originTip(t, f.repo.origin, f.branch)

		res := FinalizeCloseout(context.Background(), f.closeoutDeps(f.baselineMergedFake(f.head, mergeCommit)), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result == ResultApplied || res.Result == ResultNoOp {
			t.Fatalf("closeout applied despite a defect on B itself: %q disp %q", res.Result, res.Disposition)
		}
		assertRefusalBeyondUnrelated(t, res.Reason, res.Findings)
		if after := originTip(t, f.repo.origin, f.branch); after != tip {
			t.Errorf("a refused closeout moved the metadata branch %s -> %s", tip, after)
		}
		if _, ok := originFile(t, f.repo.origin, f.branch, recPath); !ok {
			t.Errorf("refused closeout relocated B away from its active path")
		}
		assertRuntimeIntact(t, runtime)
	})
}
