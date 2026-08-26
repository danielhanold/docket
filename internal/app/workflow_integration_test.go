//go:build integration

package app

import (
	"context"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestAttachPlanBoardLinkAtomicity proves the attach-plan metadata transaction is
// atomic: the single applied commit rewrites the change record (its plan: field
// and its re-rendered artifact block) and the inline board together — never one
// without the other. It inspects the winning commit's exact changed-path set.
func TestIntegrationWorkflowAttachPlanBoardLinkAtomicity(t *testing.T) {
	requireRealGit(t)
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	planPath := "docs/superpowers/plans/2026-08-17-widget-plan.md"
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				recPath: lifecycleChange(id, slug, "in-progress"),
			})
			version := blobVersionAt(t, repo.origin, m.branch, recPath)
			node := planningDepsFor(t, repo.invocation)
			svc, err := workspace.NewService(node.deps.Client)
			if err != nil {
				t.Fatalf("workspace.NewService: %v", err)
			}
			wdeps := WorkspaceDeps{Service: svc}
			ctx := context.Background()

			prep := WorkspacePrepare(ctx, node.deps, wdeps, repo.invocation, WorkspaceIDRequest{ID: id, Version: version})
			if prep.Result != ResultApplied {
				t.Fatalf("prepare workspace = %q (reason %q msg %q)", prep.Result, prep.Reason, prep.Message)
			}
			head := commitPlanFile(t, prep.Path, planPath, attachHappyPlan(id, "A change", recPath), planPath)

			res := ChangeAttachPlan(ctx, node.deps, wdeps, repo.invocation,
				ChangeAttachRequest{ID: id, Version: version, Path: planPath, Commit: head})
			if res.Result != ResultApplied {
				t.Fatalf("attach = %q (reason %q msg %q findings %v)", res.Result, res.Reason, res.Message, res.Findings)
			}
			if res.Revision == "" {
				t.Fatalf("applied attach carried no committed revision")
			}

			// The one metadata commit changes exactly the change record and the board.
			want := []string{"docs/changes/BOARD.md", recPath}
			sort.Strings(want)
			got := originCommitPaths(t, repo.origin, res.Revision)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("attach commit changed %v, want exactly %v (record + board in one commit)", got, want)
			}
			final, ok := originFile(t, repo.origin, m.branch, recPath)
			if !ok || !strings.Contains(final, "plan: '"+planPath+"'") {
				t.Errorf("attach commit did not land the plan field on the record:\n%s", final)
			}
		})
	}
}

// TestChangeAttachPlanGitVerification is the guard-table mutation test: each row
// corrupts exactly one property of the happy fixture and asserts the operation
// refuses with that guard's stable reason.
func TestIntegrationWorkflowChangeAttachPlanGitVerification(t *testing.T) {
	f := attachSetup(t)
	otherPlan := "docs/superpowers/plans/2026-08-17-decoy.md"

	rows := []struct {
		name   string
		build  func(t *testing.T) ChangeAttachRequest // resets, mutates, returns the request
		reason string
	}{
		{
			name: "plan outside the planning root",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				head := f.commitPlan(t, map[string]string{f.planPath: attachHappyPlan(f.id, "A change", f.recPath)}, f.planPath)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: "docs/notes/outside.md", Commit: head}
			},
			reason: ReasonAttachPathOutsideRoot,
		},
		{
			name: "commit is not the feature head",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				_ = f.commitPlan(t, map[string]string{f.planPath: attachHappyPlan(f.id, "A change", f.recPath)}, f.planPath)
				// The base is a real commit that is not the head.
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: f.base}
			},
			reason: ReasonAttachCommitNotHead,
		},
		{
			name: "commit does not descend from the prepared base",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				// A root (orphan) commit carrying a valid plan: it is the head, but it
				// does not descend from the prepared base.
				runGit(t, f.wp, "checkout", "-q", "--orphan", "_orphan")
				writeRepoFile(t, f.wp, f.planPath, attachHappyPlan(f.id, "A change", f.recPath))
				runGit(t, f.wp, "add", "-A")
				runGit(t, f.wp, "commit", "-q", "-m", "orphan plan", "--trailer", "Docket-Plan-Path: "+f.planPath)
				orphan := runGit(t, f.wp, "rev-parse", "HEAD")
				runGit(t, f.wp, "branch", "-qf", "feat/"+f.slug, "_orphan")
				runGit(t, f.wp, "checkout", "-q", "feat/"+f.slug)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: orphan}
			},
			reason: ReasonAttachCommitNotDescendant,
		},
		{
			name: "artifact path is not tracked at the commit",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				// The commit adds a DIFFERENT file; the requested plan path is absent.
				head := f.commitPlan(t, map[string]string{otherPlan: attachHappyPlan(f.id, "A change", f.recPath)}, f.planPath)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachUntrackedFile,
		},
		{
			name: "artifact is a symlink at the commit",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				symlinkRepoFile(t, f.wp, f.planPath, "README.md")
				runGit(t, f.wp, "add", "-A")
				runGit(t, f.wp, "commit", "-q", "-m", "symlink plan", "--trailer", "Docket-Plan-Path: "+f.planPath)
				head := runGit(t, f.wp, "rev-parse", "HEAD")
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachSymlinkedPlan,
		},
		{
			name: "writer commit changes two artifacts",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				head := f.commitPlan(t, map[string]string{
					f.planPath: attachHappyPlan(f.id, "A change", f.recPath),
					"docs/superpowers/plans/2026-08-17-extra.md": "# extra\n",
				}, f.planPath)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachMultiArtifactDelta,
		},
		{
			name: "writer commit is a rename (two-sided with --no-renames)",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				// A prior commit places an old file; the writer commit renames it to
				// the plan path. With rename detection off this is a delete + an add.
				old := "docs/superpowers/plans/2026-08-17-old.md"
				f.commitPlan(t, map[string]string{old: attachHappyPlan(f.id, "A change", f.recPath)}, "")
				runGit(t, f.wp, "mv", old, f.planPath)
				runGit(t, f.wp, "commit", "-q", "-m", "rename plan", "--trailer", "Docket-Plan-Path: "+f.planPath)
				head := runGit(t, f.wp, "rev-parse", "HEAD")
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachMultiArtifactDelta,
		},
		{
			name: "writer commit lacks the plan-path trailer",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				head := f.commitPlan(t, map[string]string{f.planPath: attachHappyPlan(f.id, "A change", f.recPath)}, "")
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachMissingTrailer,
		},
		{
			name: "backlink markers are unbalanced",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				// A dangling start marker with no partner: the whole-population marker
				// validation fails the parse.
				bad := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> ↩ dangling\n\n# Plan\n\nSteps.\n"
				head := f.commitPlan(t, map[string]string{f.planPath: bad}, f.planPath)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachUnbalancedBacklink,
		},
		{
			name: "backlink targets another change",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				// A balanced backlink pointing at a different change.
				bad := attachBacklinkBlock(9, "Another change", "docs/changes/active/0009-other.md") + "\n# Plan\n\nSteps.\n"
				head := f.commitPlan(t, map[string]string{f.planPath: bad}, f.planPath)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachBacklinkMismatch,
		},
		{
			name: "plan carries an unresolved placeholder token",
			build: func(t *testing.T) ChangeAttachRequest {
				f.reset(t)
				withToken := attachBacklinkBlock(f.id, "A change", f.recPath) + "\n# Plan\n\nTODO: finish this section.\n"
				head := f.commitPlan(t, map[string]string{f.planPath: withToken}, f.planPath)
				return ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head}
			},
			reason: ReasonAttachPlaceholderToken,
		},
	}

	for _, row := range rows {
		row := row
		t.Run(row.name, func(t *testing.T) {
			req := row.build(t)
			res := ChangeAttachPlan(f.ctx, f.deps, f.wdeps, f.invocation, req)
			if res.Result == ResultApplied {
				t.Fatalf("%s: attach applied, want a refusal", row.name)
			}
			if res.Reason != row.reason {
				t.Fatalf("%s: reason = %q, want %q (msg %q)", row.name, res.Reason, row.reason, res.Message)
			}
			// A refusal opens no transaction: the remote record keeps no plan field.
			final, ok := originFile(t, f.repo.origin, "main", f.recPath)
			if ok && strings.Contains(final, "plan: '") {
				t.Errorf("%s: a refused attach wrote the plan field to the remote", row.name)
			}
		})
	}
}

// TestChangeAttachPlanGitVerificationHappyPath proves a correctly written plan
// commit passes every from-Git guard and lands the metadata transaction: the
// change record on the remote gains the plan: field, rendered by the engine.
func TestIntegrationWorkflowChangeAttachPlanGitVerificationHappyPath(t *testing.T) {
	f := attachSetup(t)
	head := f.commitPlan(t, map[string]string{
		f.planPath: attachHappyPlan(f.id, "A change", f.recPath),
	}, f.planPath)

	res := ChangeAttachPlan(f.ctx, f.deps, f.wdeps, f.invocation,
		ChangeAttachRequest{ID: f.id, Version: f.version, Path: f.planPath, Commit: head})
	if res.Result != ResultApplied {
		t.Fatalf("happy attach = %q (reason %q msg %q findings %v)", res.Result, res.Reason, res.Message, res.Findings)
	}
	if res.Revision == "" || res.Path != f.planPath || res.Kind != attachKindPlan {
		t.Fatalf("applied result malformed: %+v", res)
	}
	final, ok := originFile(t, f.repo.origin, "main", f.recPath)
	if !ok {
		t.Fatalf("change record missing on origin after attach")
	}
	if !strings.Contains(final, "plan: '"+f.planPath+"'") {
		t.Errorf("committed record missing the plan field:\n%s", final)
	}
}

// TestClaimRaceLosesCleanly proves a claimant working from a context version that
// the origin has since diverged past loses cleanly: its claim is refused as
// `contended` against fresh origin state, the metadata remote holds exactly one
// claim (the winner's), and the loser opened no transaction (the remote ref never
// moved for it) and created no feature branch/workspace.
//
// Claim carries an idempotency key derived from (id, version), so two claims at
// the exact SAME version are a safe replay, never a contention — the genuinely
// contended path is only reached when an independent writer diverges the record
// on the remote between a claimant's context read and its claim, leaving that
// claimant's version stale (learning green-suite-untested-branch: the contended
// path must actually diverge on the remote; learning cas-re-read-fresh-origin:
// the loser's stale local tree is never trusted).
func TestIntegrationWorkflowClaimRaceLosesCleanly(t *testing.T) {
	requireRealGit(t)
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				recPath: buildReadyChange(id, slug),
			})
			ctx := context.Background()

			// The loser reads authoritative context first, pinning the original
			// (soon-to-be-stale) record version.
			nodeA := planningDepsFor(t, cloneOrigin(t, repo.origin))
			loserCtx := ContextImplementation(ctx, nodeA.deps, nodeA.dir, ImplementationContextRequest{ID: id})
			if loserCtx.Result != ResultApplied || loserCtx.Context == nil {
				t.Fatalf("loser context read = %q (reason %q); want a bundle", loserCtx.Result, loserCtx.Reason)
			}
			if !loserCtx.Context.ClaimEligible {
				t.Fatalf("context read reports the build-ready change not claim-eligible: %q", loserCtx.Context.ClaimRefusal)
			}
			staleVersion := loserCtx.Context.Change.Version
			if staleVersion == "" {
				t.Fatalf("context bundle carried no change version")
			}

			// An independent writer, via a side clone, pushes a benign edit to the
			// SAME record first — keeping it proposed and build-ready but moving its
			// blob, so the loser's pinned version is now stale on the origin.
			diverged := strings.Replace(buildReadyChange(id, slug), "Original why.", "Edited by an independent writer.", 1)
			repo.writerAdvance(t, m.branch, map[string]string{recPath: diverged})
			if freshVersion := blobVersionAt(t, repo.origin, m.branch, recPath); freshVersion == staleVersion {
				t.Fatalf("independent writer did not diverge the record blob; the contended path is vacuous")
			}

			// The winner reads fresh context (the diverged version) and claims.
			nodeB := planningDepsFor(t, cloneOrigin(t, repo.origin))
			winnerCtx := ContextImplementation(ctx, nodeB.deps, nodeB.dir, ImplementationContextRequest{ID: id})
			if winnerCtx.Result != ResultApplied || winnerCtx.Context == nil || !winnerCtx.Context.ClaimEligible {
				t.Fatalf("winner context read = %q (reason %q)", winnerCtx.Result, winnerCtx.Reason)
			}
			beforeTip := originTip(t, repo.origin, m.branch)
			resB := ChangeClaim(ctx, nodeB.deps, nodeB.dir, ChangeClaimRequest{ID: id, Version: winnerCtx.Context.Change.Version})
			if resB.Result != ResultApplied || resB.Disposition != ClaimDispositionApplied {
				t.Fatalf("winning claim = (%q, %q), want applied/applied (findings %v)", resB.Result, resB.Disposition, resB.Findings)
			}
			wonTip := originTip(t, repo.origin, m.branch)
			if wonTip == beforeTip {
				t.Fatalf("winning claim did not move the metadata remote")
			}

			// A loses: it submits its now-stale context version. The engine's own
			// fresh-origin re-read discovers the record moved, and A's version keyed no
			// committed claim receipt, so this is a genuine contention — not a replay.
			resA := ChangeClaim(ctx, nodeA.deps, nodeA.dir, ChangeClaimRequest{ID: id, Version: staleVersion})
			if resA.Result != ResultContended || resA.Disposition != ClaimDispositionContended {
				t.Fatalf("losing claim = (%q, %q), want contended/contended (findings %v)", resA.Result, resA.Disposition, resA.Findings)
			}

			// The loser wrote nothing: the metadata remote is exactly where B left it.
			if got := originTip(t, repo.origin, m.branch); got != wonTip {
				t.Errorf("contended claim moved the metadata remote: %q -> %q", wonTip, got)
			}
			// Exactly one claim landed: the winner's, and only the winner's.
			final, ok := originFile(t, repo.origin, m.branch, recPath)
			if !ok {
				t.Fatalf("change record missing on origin after the race")
			}
			if !strings.Contains(final, "status: 'in-progress'") {
				t.Errorf("winning claim did not land in-progress:\n%s", final)
			}
			if !strings.Contains(final, "branch: 'feat/"+slug+"'") {
				t.Errorf("winning claim did not stamp the feature branch:\n%s", final)
			}
			// A claim creates no workspace and pushes no feature branch: neither
			// participant left one behind.
			if branches := originFeatureBranches(t, repo.origin); len(branches) != 0 {
				t.Errorf("a claim created feature-branch state: %v", branches)
			}
		})
	}
}

// TestClaimRetryAfterLostResponse proves the idempotency key makes a lost-response
// retry safe: the identical claim request, re-run after its receipt was discarded,
// replays the original applied claim as `already-claimed` and commits nothing new,
// so exactly one claim commit sits on the metadata remote.
func TestIntegrationWorkflowClaimRetryAfterLostResponse(t *testing.T) {
	requireRealGit(t)
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				recPath: buildReadyChange(id, slug),
			})
			node := planningDepsFor(t, repo.invocation)
			ctx := context.Background()

			shared := ContextImplementation(ctx, node.deps, node.dir, ImplementationContextRequest{ID: id})
			if shared.Result != ResultApplied || shared.Context == nil {
				t.Fatalf("context read = %q (reason %q); want a bundle", shared.Result, shared.Reason)
			}
			version := shared.Context.Change.Version
			beforeTip := originTip(t, repo.origin, m.branch)

			first := ChangeClaim(ctx, node.deps, node.dir, ChangeClaimRequest{ID: id, Version: version})
			if first.Result != ResultApplied || first.Disposition != ClaimDispositionApplied {
				t.Fatalf("first claim = (%q, %q), want applied/applied (findings %v)", first.Result, first.Disposition, first.Findings)
			}
			tipAfterFirst := originTip(t, repo.origin, m.branch)
			if tipAfterFirst == beforeTip {
				t.Fatalf("first claim did not move the metadata remote")
			}

			// The response was lost: the SAME request re-runs. The version it carries
			// no longer matches the (now-claimed) record blob, so only the idempotency
			// key — not the exact-version CAS — can make this a replay rather than a
			// contention.
			replay := ChangeClaim(ctx, node.deps, node.dir, ChangeClaimRequest{ID: id, Version: version})
			if replay.Result != ResultApplied || replay.Disposition != ClaimDispositionAlreadyClaimed {
				t.Fatalf("replay = (%q, %q), want applied/already-claimed (findings %v)", replay.Result, replay.Disposition, replay.Findings)
			}
			if got := originTip(t, repo.origin, m.branch); got != tipAfterFirst {
				t.Errorf("replay produced a second commit: %q -> %q", tipAfterFirst, got)
			}
			// Exactly one claim commit landed between the fixture and now.
			count := runGit(t, repo.origin, "rev-list", "--count", beforeTip+".."+tipAfterFirst)
			if count != "1" {
				t.Errorf("metadata remote carries %s claim commits, want exactly 1", count)
			}
		})
	}
}

// TestClaimToImplementedWorkflow is the acceptance-1/6 end-to-end proof.
func TestIntegrationWorkflowClaimToImplementedWorkflow(t *testing.T) {
	requireRealGit(t)
	// The workflow must need no legacy Bash facade: clear the facade env var so a
	// stray export cannot silently satisfy anything, then assert it stayed clear.
	t.Setenv("DOCKET_SCRIPTS_DIR", "")

	ghBin := buildFakeGH(t)

	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			runClaimToImplemented(t, m, ghBin)
		})
	}

	// Acceptance 6, second clause: an actively-requested deferred capability fails
	// before any mutation. Kept in main mode — the property is about the mutation
	// preflight, not the metadata topology.
	t.Run("unsupported-capability-refused-before-mutation", func(t *testing.T) {
		assertDeferredCapabilityBlocksClaim(t)
	})

	if got := os.Getenv("DOCKET_SCRIPTS_DIR"); got != "" {
		t.Errorf("the workflow relied on a legacy Bash facade: DOCKET_SCRIPTS_DIR = %q", got)
	}
}

// TestEffectiveBaseConsumedFromDomain proves a stacked change's workspace prepare
// and attach ancestry checks run against the PARENT branch base that
// domain.ResolveEffectiveBase resolves — not the integration branch. The child's
// workspace is created at the parent feature branch tip, and the plan committed
// on it (which descends from that tip, not from main) passes attach's descendant
// check. Both metadata modes.
func TestIntegrationWorkflowEffectiveBaseConsumedFromDomain(t *testing.T) {
	requireRealGit(t)
	const (
		parentID   = 20
		parentSlug = "parent"
		childID    = 21
		childSlug  = "child"
	)
	parentRec := groomPath(parentID, parentSlug)
	childRec := groomPath(childID, childSlug)
	childPlan := "docs/superpowers/plans/2026-08-17-child-plan.md"

	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			// The child is stacked on the parent; both are in-progress with their
			// feature branches recorded.
			// stacked_on is an integer scalar (the parent id), not a zero-padded string.
			childBody := strings.Replace(lifecycleChange(childID, childSlug, "in-progress"),
				"stacked_on:\n", "stacked_on: 20\n", 1)
			repo := m.build(t, map[string]string{
				parentRec: lifecycleChange(parentID, parentSlug, "in-progress"),
				childRec:  childBody,
			})

			// The parent's feature branch exists on origin, advanced past main: it is
			// the base the child must resolve to (a live parent with a remote branch).
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

			version := blobVersionAt(t, repo.origin, m.branch, childRec)
			node := planningDepsFor(t, repo.invocation)
			svc, err := workspace.NewService(node.deps.Client)
			if err != nil {
				t.Fatalf("workspace.NewService: %v", err)
			}
			wdeps := WorkspaceDeps{Service: svc}
			ctx := context.Background()

			// The workspace is prepared at the DOMAIN-resolved parent base, not main.
			prep := WorkspacePrepare(ctx, node.deps, wdeps, repo.invocation, WorkspaceIDRequest{ID: childID, Version: version})
			if prep.Result != ResultApplied {
				t.Fatalf("prepare stacked workspace = %q (reason %q msg %q)", prep.Result, prep.Reason, prep.Message)
			}
			if prep.BaseCommit != parentTip {
				t.Fatalf("prepared base = %q, want the parent branch tip %q (effective base from the domain resolver)", prep.BaseCommit, parentTip)
			}
			if prep.BaseCommit == mainTip {
				t.Fatalf("prepared base is the integration branch tip; the stacked base was not consumed from the domain")
			}

			// A plan committed on the parent-based workspace descends from the parent
			// tip, so attach's ancestry check (run against the same resolved base)
			// passes.
			head := commitPlanFile(t, prep.Path, childPlan, attachHappyPlan(childID, "A change", childRec), childPlan)
			res := ChangeAttachPlan(ctx, node.deps, wdeps, repo.invocation,
				ChangeAttachRequest{ID: childID, Version: version, Path: childPlan, Commit: head})
			if res.Result != ResultApplied {
				t.Fatalf("attach on the stacked workspace = %q (reason %q msg %q findings %v)", res.Result, res.Reason, res.Message, res.Findings)
			}
		})
	}
}

// TestGitStatusReaderBranchFacts proves a pushed feature branch reads present
// and an absent one reads absent, with no error for the absent case.
func TestIntegrationWorkflowGitStatusReaderBranchFacts(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
	})
	repo.writerAdvance(t, "feat-present", map[string]string{"feature.txt": "x\n"})

	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(context.Background(), repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	facts, err := reader.BranchFacts(context.Background(), pin, []string{"feat-present", "feat-absent"})
	if err != nil {
		t.Fatalf("BranchFacts: %v", err)
	}
	if !facts.HasBranch("feat-present") {
		t.Errorf("feat-present should be present on the remote")
	}
	if facts.HasBranch("feat-absent") {
		t.Errorf("feat-absent should be absent on the remote")
	}
}

// TestGitStatusReaderConcurrentRemoteMovement proves a corpus read observes the
// exact pinned revision even when the remote advances after the pin: the source
// is fixed at open time and never re-fetches.
func TestIntegrationWorkflowGitStatusReaderConcurrentRemoteMovement(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
	})

	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(context.Background(), repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	pinnedRev := pin.DefaultRevision
	wantID := repo.blobID(t, repo.invocation, pinnedRev, "docs/changes/active/0001-alpha.md")

	// The remote advances the SAME record to different content after the pin.
	repo.writerAdvance(t, "main", map[string]string{"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha REWRITTEN")})

	blobs, err := reader.ReadCorpus(context.Background(), pin)
	if err != nil {
		t.Fatalf("ReadCorpus: %v", err)
	}
	byPath := map[string]StatusBlob{}
	for _, b := range blobs {
		byPath[b.Path] = b
	}
	got := byPath["docs/changes/active/0001-alpha.md"]
	if got.Version != wantID {
		t.Errorf("corpus read the advanced revision: version=%q want pinned %q", got.Version, wantID)
	}
	if strings.Contains(string(got.Data), "REWRITTEN") {
		t.Errorf("corpus content came from the advanced remote, not the pinned revision:\n%s", got.Data)
	}
}

// TestGitStatusReaderDiscoversFromNestedSubdir proves discovery canonicalizes a
// nested invocation directory to the same repository, so a pin succeeds from
// anywhere inside the worktree.
func TestIntegrationWorkflowGitStatusReaderDiscoversFromNestedSubdir(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
	})
	nested := filepath.Join(repo.invocation, "docs", "changes", "active")

	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(context.Background(), nested)
	if err != nil {
		t.Fatalf("PinContext from nested dir: %v", err)
	}
	if pin.DefaultBranch != "main" {
		t.Errorf("default branch = %q, want main", pin.DefaultBranch)
	}
	if pin.Mode != "main" {
		t.Errorf("mode = %q, want main", pin.Mode)
	}
}

// TestGitStatusReaderDocketModeDistinctRevisions proves docket mode pins the
// metadata branch separately from the integration branch and reads the corpus
// from the metadata revision, not the code branch.
func TestIntegrationWorkflowGitStatusReaderDocketModeDistinctRevisions(t *testing.T) {
	requireRealGit(t)
	repo := newDocketModeRepo(t,
		map[string]string{
			// A decoy record on main that must never appear in the corpus.
			"docs/changes/active/0009-decoy.md": changeRecord(9, "decoy", "Decoy"),
		},
		map[string]string{
			"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
		})

	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(context.Background(), repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	if pin.Mode != "docket" {
		t.Fatalf("mode = %q, want docket", pin.Mode)
	}
	if pin.MetadataBranch != "docket" {
		t.Errorf("metadata branch = %q, want docket", pin.MetadataBranch)
	}
	if pin.MetadataRevision == "" || pin.IntegrationRevision == "" {
		t.Fatalf("revisions unset: %+v", pin)
	}
	if pin.MetadataRevision == pin.IntegrationRevision {
		t.Errorf("metadata and integration revisions must differ (orphan branches): both %q", pin.MetadataRevision)
	}

	blobs, err := reader.ReadCorpus(context.Background(), pin)
	if err != nil {
		t.Fatalf("ReadCorpus: %v", err)
	}
	paths := pathsOf(blobs)
	if !contains(paths, "docs/changes/active/0001-alpha.md") {
		t.Errorf("corpus missing the metadata-branch record; got %v", paths)
	}
	if contains(paths, "docs/changes/active/0009-decoy.md") {
		t.Errorf("corpus leaked a record from the integration branch; got %v", paths)
	}
}

// TestGitStatusReaderMainModePinAndCorpus is the main-mode end-to-end read: the
// pin resolves the default branch and both revisions collapse to it, and the
// corpus carries the active change with its blob id from the pinned revision.
func TestIntegrationWorkflowGitStatusReaderMainModePinAndCorpus(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0001-alpha.md":             changeRecord(1, "alpha", "Alpha"),
		"docs/changes/active/0002-beta.md":              changeRecord(2, "beta", "Beta"),
		"docs/changes/archive/2026-01-01-0003-gamma.md": changeRecord(3, "gamma", "Gamma"),
		"docs/adrs/0001-first.md":                       "---\nid: 1\nslug: first\ntitle: First\nstatus: Accepted\ndate: 2026-01-02\n---\n\nContext.\n",
		"docs/changes/learnings/some-lesson.md":         "---\nslug: some-lesson\ntitle: Some lesson\n---\n\nA lesson.\n",
	})

	client := newGitClient(t)
	reader := NewGitStatusReader(client)
	pin, err := reader.PinContext(context.Background(), repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	if pin.Mode != "main" {
		t.Fatalf("mode = %q, want main", pin.Mode)
	}
	if pin.DefaultRevision == "" || pin.IntegrationRevision != pin.DefaultRevision {
		t.Errorf("main mode should collapse revisions: default=%q integration=%q", pin.DefaultRevision, pin.IntegrationRevision)
	}
	if pin.MetadataBranch != "" || pin.MetadataRevision != "" {
		t.Errorf("main mode carried a metadata branch: %+v", pin)
	}

	blobs, err := reader.ReadCorpus(context.Background(), pin)
	if err != nil {
		t.Fatalf("ReadCorpus: %v", err)
	}
	byPath := map[string]StatusBlob{}
	for _, b := range blobs {
		byPath[b.Path] = b
	}
	got, ok := byPath["docs/changes/active/0001-alpha.md"]
	if !ok {
		t.Fatalf("corpus missing the active change; got paths %v", pathsOf(blobs))
	}
	wantID := repo.blobID(t, repo.invocation, pin.DefaultRevision, "docs/changes/active/0001-alpha.md")
	if got.Version != wantID {
		t.Errorf("blob version = %q, want the pinned revision's blob id %q", got.Version, wantID)
	}
	if string(got.Data) != changeRecord(1, "alpha", "Alpha") {
		t.Errorf("blob bytes did not match the record content:\n%s", got.Data)
	}
	// Every configured record kind is present.
	for _, want := range []string{
		"docs/changes/active/0002-beta.md",
		"docs/changes/archive/2026-01-01-0003-gamma.md",
		"docs/adrs/0001-first.md",
		"docs/changes/learnings/some-lesson.md",
	} {
		if _, ok := byPath[want]; !ok {
			t.Errorf("corpus missing %q; got %v", want, pathsOf(blobs))
		}
	}
}

// TestGitStatusReaderMissingMetadataBranchIsExternal proves a metadata branch
// declared in configuration but absent from the remote fails as an external
// error, not a silent empty pin.
func TestIntegrationWorkflowGitStatusReaderMissingMetadataBranchIsExternal(t *testing.T) {
	requireRealGit(t)
	// A main-mode topology whose .docket.yml is overwritten to demand a docket
	// branch that was never pushed.
	repo := newMainModeRepo(t, nil)
	repo.writerAdvance(t, "main", map[string]string{
		".docket.yml": "metadata_branch: docket\nintegration_branch: main\n",
	})

	reader := NewGitStatusReader(newGitClient(t))
	_, err := reader.PinContext(context.Background(), repo.invocation)
	if err == nil {
		t.Fatal("PinContext succeeded despite a missing metadata branch")
	}
	if !isStatusExternal(err) {
		t.Errorf("error = %v, want an ErrStatusExternal classification", err)
	}
}

// TestGitStatusReaderReadOnly witnesses both halves of the read-only contract:
// the worktree, index, HEAD, and symbolic ref are byte-identical before and
// after a full read, while the one permitted mutation — a remote-tracking ref
// advancing to a newly-pushed commit — is positively observed.
func TestIntegrationWorkflowGitStatusReaderReadOnly(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
	})

	beforeFiles := worktreeChecksum(t, repo.invocation)
	beforeHead := runGit(t, repo.invocation, "rev-parse", "HEAD")
	beforeSymref := runGit(t, repo.invocation, "symbolic-ref", "HEAD")
	beforeStatus := runGit(t, repo.invocation, "status", "--porcelain")
	beforeTracking := runGit(t, repo.invocation, "rev-parse", "refs/remotes/origin/main")

	// Advance the remote so the permitted mutation has something to witness.
	newHead := repo.writerAdvance(t, "main", map[string]string{"docs/changes/active/0002-beta.md": changeRecord(2, "beta", "Beta")})

	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(context.Background(), repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	if _, err := reader.ReadCorpus(context.Background(), pin); err != nil {
		t.Fatalf("ReadCorpus: %v", err)
	}
	if _, err := reader.BranchFacts(context.Background(), pin, []string{"feat-absent"}); err != nil {
		t.Fatalf("BranchFacts: %v", err)
	}

	// Read-only over the worktree, index, HEAD, and checked-out branch.
	afterFiles := worktreeChecksum(t, repo.invocation)
	if !equalChecksums(beforeFiles, afterFiles) {
		t.Errorf("worktree files changed:\nbefore=%v\nafter=%v", beforeFiles, afterFiles)
	}
	if got := runGit(t, repo.invocation, "rev-parse", "HEAD"); got != beforeHead {
		t.Errorf("HEAD moved: %q -> %q", beforeHead, got)
	}
	if got := runGit(t, repo.invocation, "symbolic-ref", "HEAD"); got != beforeSymref {
		t.Errorf("symbolic ref moved: %q -> %q", beforeSymref, got)
	}
	if got := runGit(t, repo.invocation, "status", "--porcelain"); got != beforeStatus {
		t.Errorf("working tree status changed: %q -> %q", beforeStatus, got)
	}

	// The permitted mutation, positively witnessed: the tracking ref advanced to
	// the newly-pushed commit.
	afterTracking := runGit(t, repo.invocation, "rev-parse", "refs/remotes/origin/main")
	if afterTracking == beforeTracking {
		t.Errorf("remote-tracking ref did not move despite an advanced remote (still %q)", afterTracking)
	}
	if afterTracking != newHead {
		t.Errorf("remote-tracking ref = %q, want the newly-pushed commit %q", afterTracking, newHead)
	}
	if pin.DefaultRevision != newHead {
		t.Errorf("pinned default revision = %q, want the freshly fetched %q", pin.DefaultRevision, newHead)
	}
}

// TestPinContextDerivesGitHubRepoWebURL is 0341's wiring regression guard:
// given a GitHub origin remote, the production reader's pin carries the derived
// web base and rendered link output carries blob URLs, not bare code spans.
// Hermetic: origin's CONFIGURED url is the GitHub spelling; the insteadOf
// rewrite routes all real network traffic to the local bare origin (RemoteURL
// reads raw config, so it still sees the GitHub spelling).
// Mutation probes (each must redden this test, run with -count=1):
//   - in PinContext, drop the RemoteURL call / the RepoWebURL assignment;
//   - in linkContextOf, drop the RepoWebURL field.
func TestIntegrationWorkflowPinContextDerivesGitHubRepoWebURL(t *testing.T) {
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0007-widget.md": changeRecord(7, "widget", "Widget"),
	})
	runGit(t, repo.invocation, "remote", "set-url", "origin", "git@github.com:owner/widgets.git")
	runGit(t, repo.invocation, "config", "url."+repo.origin+".insteadOf", "git@github.com:owner/widgets.git")

	ctx := context.Background()
	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(ctx, repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	if pin.RepoWebURL != "https://github.com/owner/widgets" {
		t.Fatalf("pin.RepoWebURL = %q, want https://github.com/owner/widgets", pin.RepoWebURL)
	}

	link := linkContextOf(pin)
	if got := link.BlobURL("docs/x.md"); got != "https://github.com/owner/widgets/blob/main/docs/x.md" {
		t.Fatalf("BlobURL = %q", got)
	}

	corpus, err := reader.ReadCorpus(ctx, pin)
	if err != nil {
		t.Fatalf("ReadCorpus: %v", err)
	}
	c := changeByPath(t, pin, corpus, "docs/changes/active/0007-widget.md")
	block, err := render.BacklinkContent(c, link)
	if err != nil {
		t.Fatalf("BacklinkContent: %v", err)
	}
	if !strings.Contains(block, "(https://github.com/owner/widgets/blob/main/docs/changes/active/0007-widget.md)") {
		t.Fatalf("backlink is not a GitHub link:\n%s", block)
	}
	if strings.Contains(block, "`docs/changes/active/0007-widget.md`") {
		t.Fatalf("backlink still renders the bare code span:\n%s", block)
	}
}

// TestPinContextNonGitHubOriginYieldsEmptyWebURL pins the fallback: a plain
// local-path origin derives "", and rendering stays in repo-relative mode.
func TestIntegrationWorkflowPinContextNonGitHubOriginYieldsEmptyWebURL(t *testing.T) {
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0007-widget.md": changeRecord(7, "widget", "Widget"),
	})
	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(context.Background(), repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	if pin.RepoWebURL != "" {
		t.Fatalf("pin.RepoWebURL = %q, want \"\"", pin.RepoWebURL)
	}
	if url := linkContextOf(pin).BlobURL("docs/x.md"); url != "" {
		t.Fatalf("BlobURL = %q, want \"\" (bare-path mode)", url)
	}
}

func TestIntegrationWorkflowPlanningIdempotentReplayEndToEnd(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
			})
			node := planningDepsFor(t, repo.invocation)
			req := validChangeCreateRequest()

			first := ChangeCreate(context.Background(), node.deps, node.dir, req)
			if first.Result != ResultApplied || first.Replayed {
				t.Fatalf("first create = (%q, replayed=%v), want applied/false", first.Result, first.Replayed)
			}
			if first.ID != 2 {
				t.Fatalf("first create allocated id %d, want 2", first.ID)
			}
			tipAfterFirst := originTip(t, repo.origin, m.branch)

			// Sever the response and re-run the identical request: the replay returns
			// the original receipt and commits nothing new.
			replay := ChangeCreate(context.Background(), node.deps, node.dir, req)
			if replay.Result != ResultApplied || !replay.Replayed {
				t.Fatalf("replay = (%q, replayed=%v), want applied/true (findings %v)", replay.Result, replay.Replayed, replay.Findings)
			}
			if replay.ID != first.ID {
				t.Errorf("replay allocated a different id: %d != %d", replay.ID, first.ID)
			}
			if tip := originTip(t, repo.origin, m.branch); tip != tipAfterFirst {
				t.Errorf("replay produced a second commit: %q -> %q", tipAfterFirst, tip)
			}

			// The same request id with a different digest is a conflicting reuse.
			conflict := req
			conflict.Title = "A completely different title"
			res := ChangeCreate(context.Background(), node.deps, node.dir, conflict)
			if res.Result != ResultInvalidInput {
				t.Errorf("request-id reuse with a changed digest mapped to %q, want invalid-input (findings %v)", res.Result, res.Findings)
			}
			if tip := originTip(t, repo.origin, m.branch); tip != tipAfterFirst {
				t.Errorf("a rejected digest-conflict moved the remote ref: %q -> %q", tipAfterFirst, tip)
			}
		})
	}
}

func TestIntegrationWorkflowPlanningKillEndToEnd(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			widgetPath := groomPath(3, "widget")
			specPath := "docs/superpowers/specs/2026-08-16-widget-design.md"
			repo := m.build(t, map[string]string{
				widgetPath: killableChangeWithSpec(3, "widget", specPath),
				specPath:   specWithBacklink(widgetPath),
			})
			ver := blobVersionAt(t, repo.origin, m.branch, widgetPath)
			node := planningDepsFor(t, repo.invocation)

			res := ChangeKill(context.Background(), node.deps, node.dir, ChangeKillRequest{
				ChangeID: 3, Path: widgetPath, Version: ver, WhyKilled: "Superseded by a better plan.\n",
			})
			if res.Result != ResultApplied {
				t.Fatalf("kill did not apply: %q (findings %v)", res.Result, res.Findings)
			}
			archivePath := "docs/changes/archive/2026-08-16-0003-widget.md"
			if res.ArchivePath != archivePath {
				t.Fatalf("archive path = %q, want %q", res.ArchivePath, archivePath)
			}

			// The active record is gone; the archived record carries the killed status
			// and the authored rationale.
			if _, ok := originFile(t, repo.origin, m.branch, widgetPath); ok {
				t.Errorf("active record still present after kill (presence-encoded state)")
			}
			archived, ok := originFile(t, repo.origin, m.branch, archivePath)
			if !ok {
				t.Fatalf("archived record absent at %q", archivePath)
			}
			if !strings.Contains(archived, "status: 'killed'") {
				t.Errorf("archived record not killed:\n%s", archived)
			}
			if !strings.Contains(archived, "## Why killed\n\nSuperseded by a better plan.\n") {
				t.Errorf("kill rationale not spliced:\n%s", archived)
			}

			// The linked spec's backlink is retargeted to the archive path.
			specFinal, ok := originFile(t, repo.origin, m.branch, specPath)
			if !ok {
				t.Fatalf("spec file vanished")
			}
			if !strings.Contains(specFinal, archivePath) {
				t.Errorf("spec backlink not retargeted to the archive path:\n%s", specFinal)
			}
			if strings.Contains(specFinal, "`"+widgetPath+"`") {
				t.Errorf("spec backlink still points at the vacated active path:\n%s", specFinal)
			}

			// The board is refreshed and current; no feature-branch state is touched.
			assertBoardMatchesCommitted(t, repo.origin, m.branch, repo.invocation)
			if branches := originFeatureBranches(t, repo.origin); len(branches) != 0 {
				t.Errorf("kill touched feature-branch state: %v", branches)
			}
		})
	}
}

func TestIntegrationWorkflowPlanningRefusalPushesNothing(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
			})
			before := originTip(t, repo.origin, m.branch)

			node := planningDepsFor(t, repo.invocation)
			req := validChangeCreateRequest()
			// A depends_on reference that resolves against nothing: the whole-repository
			// validation inside the plan closure refuses, so the transaction commits
			// nothing.
			req.DependsOn = []int{999}
			res := ChangeCreate(context.Background(), node.deps, node.dir, req)

			if res.Result != ResultInvalidInput {
				t.Fatalf("dangling-reference create mapped to %q, want invalid-input (findings %v)", res.Result, res.Findings)
			}
			if after := originTip(t, repo.origin, m.branch); after != before {
				t.Errorf("remote ref moved on a refusal: %q -> %q", before, after)
			}
		})
	}
}

func TestIntegrationWorkflowPlanningSuccessIsOneCommitWithExplicitPathsCleanRoot(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
			})
			node := planningDepsFor(t, repo.invocation)

			res := ChangeCreate(context.Background(), node.deps, node.dir, validChangeCreateRequest())
			if res.Result != ResultApplied {
				t.Fatalf("create did not apply: %q (findings %v)", res.Result, res.Findings)
			}
			if res.Revision == "" {
				t.Fatalf("applied result carried no committed revision")
			}

			newRecord := "docs/changes/active/0002-add-a-widget.md"
			want := []string{"docs/changes/BOARD.md", newRecord}
			sort.Strings(want)
			got := originCommitPaths(t, repo.origin, res.Revision)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("winning commit changed %v, want exactly %v", got, want)
			}

			// The candidate never leaks: the transactions root is clean.
			if !transactionsRootEmpty(t, node.deps.Client, node.dir) {
				t.Errorf("transactions root left a candidate worktree behind")
			}

			// The committed board is a fresh render of the committed corpus.
			assertBoardMatchesCommitted(t, repo.origin, m.branch, repo.invocation)
		})
	}
}

// TestReconcileIndependentWriterWins proves a reconcile whose authored request
// was built against a now-stale record is refused as `contended` when an
// independent writer commits a conflicting edit to origin between the context
// read and the reconcile: the operation text-merges nothing, writes nothing, and
// the independent writer's bytes survive on origin byte-for-byte.
func TestIntegrationWorkflowReconcileIndependentWriterWins(t *testing.T) {
	requireRealGit(t)
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				recPath: lifecycleChange(id, slug, "in-progress"),
			})
			// The version the authored reconcile request pins — read BEFORE the
			// independent writer diverges the origin.
			version := blobVersionAt(t, repo.origin, m.branch, recPath)
			node := planningDepsFor(t, repo.invocation)
			ctx := context.Background()

			// An independent writer commits a conflicting edit to the SAME record on
			// origin, keeping it in-progress but changing its bytes.
			writerEdit := strings.Replace(lifecycleChange(id, slug, "in-progress"),
				"Original why.", "Rewritten by an independent writer.", 1)
			repo.writerAdvance(t, m.branch, map[string]string{recPath: writerEdit})

			res := ChangeReconcile(ctx, node.deps, node.dir, ChangeReconcileRequest{
				ID:                id,
				Version:           version,
				ReconcileLogEntry: "Reconciled against current reality.\n",
			})
			if res.Result != ResultContended || res.Disposition != ReconcileDispositionContended {
				t.Fatalf("reconcile = (%q, %q), want contended/contended (findings %v)", res.Result, res.Disposition, res.Findings)
			}

			// The independent writer's bytes survive untouched — no text-merge, no
			// overwrite.
			final, ok := originFile(t, repo.origin, m.branch, recPath)
			if !ok {
				t.Fatalf("record missing on origin after a contended reconcile")
			}
			if final != writerEdit {
				t.Errorf("contended reconcile did not preserve the independent writer's bytes:\n--want--\n%s\n--got--\n%s", writerEdit, final)
			}
		})
	}
}

// TestRefreshClaimAppliesWhenBoardReRenderIsUnchanged: a refresh re-stamps a
// claimed record's claimed_at and updated fields (a real change to the record)
// and re-renders the inline board. The board's in-progress row shows only
// id/title/priority/type/spec/branch — none of which a refresh touches — so
// the re-render is byte-identical to the board the claim already committed.
// Pre-0335 the plan nonetheless DECLARED the board, and the engine's two-way
// actual-delta guard rejected it as "a declared path is not an actual change"
// (a *Failure at stage verify-delta), silently failing every such lease
// refresh. Post-fix the plan declares only the record, so the refresh applies:
// one metadata commit touching exactly the record, the lease re-stamped, the
// committed board untouched and still matching a fresh render.
//
// The refresh runs a day after the claim (advanced clock) so the record
// genuinely changes; that isolates the board as the sole would-be
// declared-but-unchanged path.
func TestIntegrationWorkflowRefreshClaimAppliesWhenBoardReRenderIsUnchanged(t *testing.T) {
	requireRealGit(t)
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	advanced := fixedClock{t: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)}
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				recPath: buildReadyChange(id, slug),
			})
			ctx := context.Background()

			// Claim at the base clock: stamps the record in-progress and
			// commits the inline board reflecting that in-progress row.
			claimNode := planningDepsFor(t, cloneOrigin(t, repo.origin))
			claimCtx := ContextImplementation(ctx, claimNode.deps, claimNode.dir, ImplementationContextRequest{ID: id})
			if claimCtx.Result != ResultApplied || claimCtx.Context == nil || !claimCtx.Context.ClaimEligible {
				t.Fatalf("claim context read = %q (reason %q)", claimCtx.Result, claimCtx.Reason)
			}
			claim := ChangeClaim(ctx, claimNode.deps, claimNode.dir, ChangeClaimRequest{ID: id, Version: claimCtx.Context.Change.Version})
			if claim.Result != ResultApplied || claim.Disposition != ClaimDispositionApplied {
				t.Fatalf("claim = (%q, %q), want applied/applied (findings %v)", claim.Result, claim.Disposition, claim.Findings)
			}
			assertBoardMatchesCommitted(t, repo.origin, m.branch, claimNode.dir)

			claimedVersion := blobVersionAt(t, repo.origin, m.branch, recPath)

			// Refresh a day later: claimed_at/updated genuinely change on the
			// record while the board re-renders byte-identical.
			refreshNode := planningDepsForClock(t, cloneOrigin(t, repo.origin), advanced)
			res := ChangeRefreshClaim(ctx, refreshNode.deps, refreshNode.dir, ChangeClaimRequest{ID: id, Version: claimedVersion})
			if res.Result != ResultApplied || res.Disposition != ClaimDispositionApplied {
				t.Fatalf("refresh = (%q, %q), want applied/applied (failure %+v, findings %v)",
					res.Result, res.Disposition, res.Failure, res.Findings)
			}
			if res.Revision == "" {
				t.Fatal("applied refresh carried no committed revision")
			}

			// The refresh commit touches exactly the record — the
			// byte-identical board was not declared, so it is not in the
			// commit's changed-path set.
			got := originCommitPaths(t, repo.origin, res.Revision)
			if strings.Join(got, ",") != recPath {
				t.Errorf("refresh commit changed %v, want exactly [%s] (unchanged board must not be declared)", got, recPath)
			}

			// The lease actually moved to the advanced clock.
			final, ok := originFile(t, repo.origin, m.branch, recPath)
			if !ok {
				t.Fatalf("record %s missing from origin after refresh", recPath)
			}
			if !strings.Contains(final, "claimed_at: '2026-08-17T12:00:00Z'") {
				t.Errorf("refresh did not re-stamp claimed_at:\n%s", final)
			}
			if !strings.Contains(final, "updated: '2026-08-17'") {
				t.Errorf("refresh did not re-stamp updated:\n%s", final)
			}

			// The committed board still matches a fresh render of the corpus.
			assertBoardMatchesCommitted(t, repo.origin, m.branch, refreshNode.dir)
		})
	}
}

// TestIntegrationWorkflowStackedContextClaimWorkspaceFromParentBranch is the
// change-0357 regression: a proposed, designed child stacked on a LIVE parent
// whose recorded branch is pushed to the origin must pass the pre-claim
// implementation-context gate (both automatic selection and explicit id),
// resolve its effective base to the exact recorded parent branch, be claimed
// under claim's independent in-transaction re-proof, and prepare its workspace
// at the parent branch tip — not the integration-branch tip. Pre-0357
// ContextImplementation judged the stack base against an empty fact set, so this
// exact topology was refused stack-base-unresolved before claim could run.
// Reverting ContextImplementation's BranchFacts read to domain.NewBranchFacts(nil)
// must make this test fail at the pre-claim gate. It joins
// TestIntegrationWorkflowEffectiveBaseConsumedFromDomain (which starts PAST the
// gate, from an already-claimed child) rather than replacing it. Both metadata
// modes; the git reader is the production NewGitStatusReader.
func TestIntegrationWorkflowStackedContextClaimWorkspaceFromParentBranch(t *testing.T) {
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

// TestWorkspaceOpsGitLifecycle walks one change through prepare (fresh + resume),
// inspect, and publish (create, fast-forward, and a contended divergence) against
// a real bare remote, in both metadata modes.
func TestIntegrationWorkflowWorkspaceOpsGitLifecycle(t *testing.T) {
	requireRealGit(t)
	const (
		id   = 3
		slug = "widget"
	)
	recPath := groomPath(id, slug)
	featureRef := "refs/heads/feat/" + slug

	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			repo := m.build(t, map[string]string{
				recPath: lifecycleChange(id, slug, "in-progress"),
			})
			version := blobVersionAt(t, repo.origin, m.branch, recPath)
			mainTip := originTip(t, repo.origin, "main")

			node := planningDepsFor(t, repo.invocation)
			deps := node.deps
			svc, err := workspace.NewService(deps.Client)
			if err != nil {
				t.Fatalf("workspace.NewService: %v", err)
			}
			wdeps := WorkspaceDeps{Service: svc}
			ctx := context.Background()

			// --- prepare a fresh workspace at the resolved base ----------------
			prep := WorkspacePrepare(ctx, deps, wdeps, repo.invocation, WorkspaceIDRequest{ID: id, Version: version})
			if prep.Result != ResultApplied || prep.Disposition != string(workspace.PrepareCreated) {
				t.Fatalf("fresh prepare = (%q, %q), want applied/created (reason %q msg %q)", prep.Result, prep.Disposition, prep.Reason, prep.Message)
			}
			if prep.BaseCommit != mainTip {
				t.Errorf("prepared base = %q, want origin main tip %q (a real fetch of the resolved base)", prep.BaseCommit, mainTip)
			}
			if prep.Path == "" || prep.Head == "" {
				t.Fatalf("prepared workspace missing path/head: %+v", prep)
			}
			workspacePath := prep.Path
			createdHead := prep.Head

			// --- inspect reports the ready state ------------------------------
			insp := WorkspaceInspect(ctx, deps, wdeps, repo.invocation, WorkspaceIDRequest{ID: id})
			if insp.Result != ResultApplied || insp.State != string(workspace.StateReady) {
				t.Fatalf("inspect = (%q, state %q), want applied/ready (reason %q)", insp.Result, insp.State, insp.Reason)
			}
			if insp.Head != createdHead {
				t.Errorf("inspect head = %q, want the created head %q", insp.Head, createdHead)
			}

			// --- resuming yields the existing disposition ---------------------
			resume := WorkspacePrepare(ctx, deps, wdeps, repo.invocation, WorkspaceIDRequest{ID: id, Version: version})
			if resume.Disposition != string(workspace.PrepareExisting) {
				t.Fatalf("second prepare disposition = %q, want existing", resume.Disposition)
			}

			// --- publish the current head: creates the absent remote ref ------
			pub := WorkspacePublish(ctx, deps, wdeps, repo.invocation, WorkspacePublishRequest{ID: id, Head: createdHead})
			if pub.Result != ResultApplied || pub.Disposition != string(workspace.PublishPublished) {
				t.Fatalf("publish = (%q, %q), want applied/published (reason %q msg %q)", pub.Result, pub.Disposition, pub.Reason, pub.Message)
			}
			if got := originTip(t, repo.origin, featureRef); got != createdHead {
				t.Errorf("remote feature ref = %q, want the exact published head %q", got, createdHead)
			}

			// --- advance the head and publish the fast-forward ----------------
			writeRepoFile(t, workspacePath, "feature.txt", "feature work\n")
			runGit(t, workspacePath, "add", "-A")
			runGit(t, workspacePath, "commit", "-q", "-m", "feature work")
			advancedHead := runGit(t, workspacePath, "rev-parse", "HEAD")
			pub2 := WorkspacePublish(ctx, deps, wdeps, repo.invocation, WorkspacePublishRequest{ID: id, Head: advancedHead})
			if pub2.Result != ResultApplied || pub2.Disposition != string(workspace.PublishPublished) {
				t.Fatalf("fast-forward publish = (%q, %q), want applied/published", pub2.Result, pub2.Disposition)
			}
			if got := originTip(t, repo.origin, featureRef); got != advancedHead {
				t.Errorf("remote feature ref = %q after ff, want %q", got, advancedHead)
			}

			// --- diverge the remote, then publish → contended, remote untouched
			// An independent writer force-pushes a divergent commit onto the same
			// feature ref; the local head then advances along its own line, so the
			// remote commit is neither equal to nor an ancestor of it.
			runGit(t, repo.writer, "checkout", "-q", "-B", "feat/"+slug, "origin/main")
			writeRepoFile(t, repo.writer, "writer-side.txt", "independent writer\n")
			runGit(t, repo.writer, "add", "-A")
			runGit(t, repo.writer, "commit", "-q", "-m", "writer divergence")
			runGit(t, repo.writer, "push", "-q", "-f", "origin", "feat/"+slug)
			divergent := originTip(t, repo.origin, featureRef)

			writeRepoFile(t, workspacePath, "local2.txt", "more local work\n")
			runGit(t, workspacePath, "add", "-A")
			runGit(t, workspacePath, "commit", "-q", "-m", "more local work")
			localHead2 := runGit(t, workspacePath, "rev-parse", "HEAD")

			contended := WorkspacePublish(ctx, deps, wdeps, repo.invocation, WorkspacePublishRequest{ID: id, Head: localHead2})
			if contended.Result != ResultContended || contended.Disposition != string(workspace.PublishContended) {
				t.Fatalf("divergent publish = (%q, %q), want contended/contended", contended.Result, contended.Disposition)
			}
			if got := originTip(t, repo.origin, featureRef); got != divergent {
				t.Errorf("remote feature ref moved on a contended publish: %q -> %q (must be untouched, no force)", divergent, got)
			}
		})
	}
}

// TestWorkspacePrepareHonorsRecordedBranch: the Target handed to the service
// carries the record's recorded branch verbatim (feature/renamed-head), never a
// slug derivation. Mutation: derive FeatureRef from the slug and this reddens
// (it would be refs/heads/feat/widget).
func TestIntegrationWorkflowWorkspacePrepareHonorsRecordedBranch(t *testing.T) {
	const ver = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	reader := &fakeReader{
		pin:    mainPin(t),
		corpus: []StatusBlob{renamedBranchBlob(30, "widget", ver, "feature/renamed-head")},
	}
	svc := &fakeWorkspaceService{prepareWS: workspace.Workspace{Disposition: workspace.PrepareCreated}}
	repoDir := newMainModeRepo(t, nil).invocation

	res := WorkspacePrepare(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
		repoDir, WorkspaceIDRequest{ID: 30, Version: ver})

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (reason %q msg %q)", res.Result, res.Reason, res.Message)
	}
	if len(svc.prepareCalls) != 1 {
		t.Fatalf("Prepare called %d times, want 1", len(svc.prepareCalls))
	}
	tgt := svc.prepareCalls[0].Target
	if want := gitcli.RefName("refs/heads/feature/renamed-head"); tgt.FeatureRef != want {
		t.Errorf("Target.FeatureRef = %q, want %q (the recorded branch, not a slug derivation)", tgt.FeatureRef, want)
	}
	if got := tgt.FeatureBranch(); got != "feature/renamed-head" {
		t.Errorf("Target.FeatureBranch() = %q, want feature/renamed-head", got)
	}
}

// TestWorkspacePrepareRefusesMissingBranch: an in-progress record whose branch:
// field was stripped fails closed — invalid-state, no mutation, the service is
// never called. Mutation: reconstruct the branch from the slug and this reddens
// (the service would be called and the result applied).
func TestIntegrationWorkflowWorkspacePrepareRefusesMissingBranch(t *testing.T) {
	const ver = "ffffffffffffffffffffffffffffffffffffffff"
	src := lifecycleChange(31, "widget", "in-progress")
	src = strings.Replace(src, "branch: feat/widget\n", "", 1)
	reader := &fakeReader{
		pin: mainPin(t),
		corpus: []StatusBlob{{
			Kind: repository.KindChange, Location: repository.LocationActive,
			Path: groomPath(31, "widget"), Version: ver, Data: []byte(src),
		}},
	}
	svc := &fakeWorkspaceService{}
	repoDir := newMainModeRepo(t, nil).invocation

	res := WorkspacePrepare(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
		repoDir, WorkspaceIDRequest{ID: 31, Version: ver})

	if res.Result != ResultInvalidState || res.Reason != errBranchMissing.Error() {
		t.Fatalf("result=%q reason=%q, want invalid-state/branch-missing (msg %q)", res.Result, res.Reason, res.Message)
	}
	if len(svc.prepareCalls) != 0 {
		t.Errorf("service called on a missing-branch refusal (%d calls); no mutation must occur", len(svc.prepareCalls))
	}
}

// TestWorkspacePrepareRequiresClaimedVersion: a proposed (unclaimed) change and
// a stale version each refuse before any Git work — the service is never called.
func TestIntegrationWorkflowWorkspacePrepareRequiresClaimedVersion(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation

	t.Run("not in-progress", func(t *testing.T) {
		reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{proposedChangeBlob(7, "widget", "v7")}}
		svc := &fakeWorkspaceService{}
		res := WorkspacePrepare(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
			repoDir, WorkspaceIDRequest{ID: 7, Version: "v7"})
		if res.Result != ResultInvalidState || res.Reason != ReasonWorkspaceNotInProgress {
			t.Fatalf("result=%q reason=%q, want invalid-state/not-in-progress", res.Result, res.Reason)
		}
		if len(svc.prepareCalls) != 0 {
			t.Errorf("service called on a not-in-progress refusal (%d calls)", len(svc.prepareCalls))
		}
	})

	t.Run("stale version", func(t *testing.T) {
		reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "current-v", "")}}
		svc := &fakeWorkspaceService{}
		res := WorkspacePrepare(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
			repoDir, WorkspaceIDRequest{ID: 7, Version: "stale-v"})
		if res.Result != ResultContended || res.Reason != ReasonWorkspaceVersionMismatch {
			t.Fatalf("result=%q reason=%q, want contended/version-mismatch", res.Result, res.Reason)
		}
		if len(svc.prepareCalls) != 0 {
			t.Errorf("service called on a stale-version refusal (%d calls)", len(svc.prepareCalls))
		}
	})
}

// TestWorkspacePrepareResolvesBaseFromDomain: the Target.Base handed to the
// service equals domain.ResolveEffectiveBase's answer for a STACKED change —
// the parent's feature branch, not the integration branch. Mutation: hard-code
// the base to "main" and this reddens (the resolved base is feat/parent).
func TestIntegrationWorkflowWorkspacePrepareResolvesBaseFromDomain(t *testing.T) {
	const childVer = "cccccccccccccccccccccccccccccccccccccccc"
	reader := &fakeReader{
		pin: mainPin(t),
		corpus: []StatusBlob{
			inProgressChangeBlob(20, "parent", "pppppppppppppppppppppppppppppppppppppppp", ""),
			inProgressChangeBlob(21, "child", childVer, "20"),
		},
		facts: domain.NewBranchFacts(map[string]bool{"feat/parent": true}),
	}
	svc := &fakeWorkspaceService{prepareWS: workspace.Workspace{Disposition: workspace.PrepareCreated}}
	deps := workspaceDepsFor(t, reader)
	repoDir := newMainModeRepo(t, nil).invocation

	res := WorkspacePrepare(context.Background(), deps, WorkspaceDeps{Service: svc},
		repoDir, WorkspaceIDRequest{ID: 21, Version: childVer})

	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied (reason %q msg %q)", res.Result, res.Reason, res.Message)
	}
	if len(svc.prepareCalls) != 1 {
		t.Fatalf("Prepare called %d times, want 1", len(svc.prepareCalls))
	}
	base := svc.prepareCalls[0].Target.Base
	if base.Kind != domain.BaseResolved || base.Branch != "feat/parent" {
		t.Errorf("Target.Base = %+v, want resolved/feat/parent (the domain's answer, not a hard-coded main)", base)
	}
	if svc.prepareCalls[0].Target.BaseRef != gitcli.RefName("refs/heads/feat/parent") {
		t.Errorf("Target.BaseRef = %q, want refs/heads/feat/parent", svc.prepareCalls[0].Target.BaseRef)
	}
}

// TestWorkspacePublishHeadMismatch: when the reinspected workspace head differs
// from the expected head, the operation refuses and never calls PublishHead.
func TestIntegrationWorkflowWorkspacePublishHeadMismatch(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")}}
	svc := &fakeWorkspaceService{
		inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID("actualhead")},
	}
	res := WorkspacePublish(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
		repoDir, WorkspacePublishRequest{ID: 7, Head: "expectedhead"})

	if res.Result != ResultInvalidState || res.Reason != ReasonWorkspaceHeadMismatch {
		t.Fatalf("result=%q reason=%q, want invalid-state/head-mismatch", res.Result, res.Reason)
	}
	if len(svc.inspectCalls) != 1 {
		t.Errorf("Inspect called %d times, want 1", len(svc.inspectCalls))
	}
	if len(svc.publishCalls) != 0 {
		t.Errorf("PublishHead called on a head mismatch (%d calls)", len(svc.publishCalls))
	}
}

// TestWorkspacePublishPassesThroughDispositions: each service publish
// disposition maps to a fixed protocol result, with the service's disposition
// carried through verbatim (no force, no retry).
func TestIntegrationWorkflowWorkspacePublishPassesThroughDispositions(t *testing.T) {
	repoDir := newMainModeRepo(t, nil).invocation
	const head = "abcdef0000000000000000000000000000000000"

	cases := []struct {
		name       string
		disp       workspace.PublishDisposition
		wantResult Result
	}{
		{"published", workspace.PublishPublished, ResultApplied},
		{"already-published", workspace.PublishAlreadyPublished, ResultNoOp},
		{"contended", workspace.PublishContended, ResultContended},
		{"unknown", workspace.PublishUnknown, ResultExternalFailed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")}}
			svc := &fakeWorkspaceService{
				inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)},
				publishRes: workspace.PublishResult{Disposition: c.disp, Head: gitcli.ObjectID(head)},
			}
			res := WorkspacePublish(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
				repoDir, WorkspacePublishRequest{ID: 7, Head: head})

			if res.Result != c.wantResult {
				t.Errorf("result = %q, want %q", res.Result, c.wantResult)
			}
			if res.Disposition != string(c.disp) {
				t.Errorf("disposition = %q, want %q (carried verbatim)", res.Disposition, c.disp)
			}
			if len(svc.publishCalls) != 1 {
				t.Errorf("PublishHead called %d times, want 1", len(svc.publishCalls))
			}
		})
	}
}
