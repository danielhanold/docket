package app

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/workspace"
)

// These are the real-git transaction integration tests for the claim → attach
// half of the agent workflow: they drive the landed ContextImplementation,
// ChangeClaim, ChangeReconcile, ChangeAttachPlan, and WorkspacePrepare operations
// through a real transaction.Engine over real bare-remote temporary repositories,
// in BOTH metadata modes (main and docket, via planRepoModes). The topology
// builders, the git oracle helpers (originTip/originFile/originCommitPaths/
// blobVersionAt/originFeatureBranches), and the invocation-clone node builder are
// reused from status_git_test.go / planning_git_test.go — this file invents no
// third harness. The attach fixtures (attachHappyPlan/attachBacklinkBlock) are
// reused from change_attach_git_test.go.
//
// The concurrency properties these tests pin cannot be faked: an independent
// writer must ACTUALLY diverge the contended path on the origin between the
// context read and the mutation, so the operation's own fresh-origin re-read is
// what discovers the divergence (learnings green-suite-untested-branch and
// cas-re-read-fresh-origin). Every "loser" assertion therefore reads the bare
// origin, never the invocation clone's stale local tree.

// buildReadyChange renders a proposed, unstacked, build-ready change record: the
// canonical groomable record with trivial flipped true, so EvaluateReadiness
// reports build-ready (design present) and ClaimEligibility passes without a
// separate spec artifact. It is the fixture the claim race/retry tests start
// from — a change a fresh context read reports claim-eligible.
func buildReadyChange(id int, slug string) string {
	return strings.Replace(groomableChange(id, slug), "trivial: false\n", "trivial: true\n", 1)
}

// commitPlanFile writes one plan artifact into a feature workspace and commits it
// with the ADR-0094 plan-path trailer, returning the new head. It is the writer
// half every attach test needs — the plan-writer's single-artifact commit.
func commitPlanFile(t *testing.T, wp, planPath, content, trailerPath string) string {
	t.Helper()
	writeRepoFile(t, wp, planPath, content)
	runGit(t, wp, "add", "-A")
	runGit(t, wp, "commit", "-q", "-m", "write plan", "--trailer", "Docket-Plan-Path: "+trailerPath)
	return runGit(t, wp, "rev-parse", "HEAD")
}

// --- claim race: two claimants, same context version, one loses cleanly -----

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
func TestClaimRaceLosesCleanly(t *testing.T) {
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

// --- claim retry after a lost response: replay, never a second claim --------

// TestClaimRetryAfterLostResponse proves the idempotency key makes a lost-response
// retry safe: the identical claim request, re-run after its receipt was discarded,
// replays the original applied claim as `already-claimed` and commits nothing new,
// so exactly one claim commit sits on the metadata remote.
func TestClaimRetryAfterLostResponse(t *testing.T) {
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

// --- reconcile against an independent writer's divergence --------------------

// TestReconcileIndependentWriterWins proves a reconcile whose authored request
// was built against a now-stale record is refused as `contended` when an
// independent writer commits a conflicting edit to origin between the context
// read and the reconcile: the operation text-merges nothing, writes nothing, and
// the independent writer's bytes survive on origin byte-for-byte.
func TestReconcileIndependentWriterWins(t *testing.T) {
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

// --- attach-plan lands change record + board in ONE metadata commit ----------

// TestAttachPlanBoardLinkAtomicity proves the attach-plan metadata transaction is
// atomic: the single applied commit rewrites the change record (its plan: field
// and its re-rendered artifact block) and the inline board together — never one
// without the other. It inspects the winning commit's exact changed-path set.
func TestAttachPlanBoardLinkAtomicity(t *testing.T) {
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

// --- 0329 spike: refresh-claim fails when the re-rendered board is unchanged --

// planningDepsForClock is planningDepsFor with an explicit clock, so a refresh
// run can happen at a later wall time than the claim that preceded it against the
// same origin — the record's claimed_at/updated then genuinely change while the
// inline board (which shows none of those fields) re-renders byte-identical.
func planningDepsForClock(t *testing.T, dir string, clock transaction.Clock) realNode {
	t.Helper()
	client := newGitClient(t)
	engine, err := transaction.NewEngine(client, clock)
	if err != nil {
		t.Fatalf("transaction.NewEngine: %v", err)
	}
	return realNode{
		dir: dir,
		deps: PlanningDeps{
			Client: client,
			Engine: engine,
			Reader: NewGitStatusReader(client),
			Clock:  clock,
		},
	}
}

// rawRefreshClaimOutcome drives the exact transaction ChangeRefreshClaim drives
// but returns the engine's raw Result and typed error, so the spike can assert
// the failure's Stage and Kind with positive evidence. The app-level
// ChangeRefreshClaim discards that typed Failure today — that discard is exactly
// the 0329 bug — so the raw engine boundary is the only place the mechanism is
// observable before the fix lands.
func rawRefreshClaimOutcome(t *testing.T, node realNode, req ChangeClaimRequest) (transaction.Result, error) {
	t.Helper()
	ctx := context.Background()
	pin, eff, inline, repo, pre := claimPreflight(ctx, node.deps, node.dir, OperationChangeRefreshClaim)
	if pre != nil {
		t.Fatalf("refresh preflight refused before the transaction: %+v", *pre)
	}
	recPath, _, terr := resolveClaimTarget(ctx, node.deps, pin, eff, req.ID, OperationChangeRefreshClaim)
	if terr != nil {
		t.Fatalf("resolving the refresh target refused before the transaction: %+v", *terr)
	}
	op := changeClaimOp{
		opKey:      OperationChangeRefreshClaim,
		changeID:   req.ID,
		refresh:    true,
		eff:        eff,
		clock:      node.deps.Clock,
		ttlHours:   eff.Reclaim.LeaseTTL.Value,
		inline:     inline,
		link:       render.LinkContext{MetadataBranch: metadataBranchOf(pin)},
		changesDir: eff.ChangesDir.Value,
	}
	return node.deps.Engine.Execute(ctx, transaction.Request{
		Repository: repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + metadataBranchOf(pin)),
		Expected: []transaction.EntityExpectation{{
			Path:    gitcli.RepoPath(recPath),
			Version: transaction.ExpectedVersion{Kind: transaction.VersionBlob, ObjectID: gitcli.ObjectID(req.Version)},
		}},
		Loader:    newPlanningLoader(eff),
		Operation: op,
	})
}

// TestRefreshClaimFailsWhenBoardReRenderIsUnchanged reproduces the 0316
// refresh-claim failure. A refresh re-stamps a claimed record's claimed_at and
// updated fields (a real change to the record) and re-renders the inline board.
// The board's in-progress row shows only id/title/priority/type/spec/branch —
// none of which a refresh touches — so the re-rendered board is byte-identical to
// the one the claim already committed. The refresh plan nonetheless DECLARES both
// the record and the board, so the engine's two-way actual-delta guard rejects
// the board as "a declared path is not an actual change": a *Failure at stage
// verify-delta, kind invalid-state, which the DispositionFailed path maps to
// result: invalid-state, disposition: invalid-state, findings: [] — the reported
// symptom, with the typed cause dropped.
//
// The refresh runs a day after the claim (advancedClock) so the record genuinely
// changes; that isolates the board as the sole declared-but-unchanged path and
// distinguishes this from a fixed-clock artifact where the record too would be
// unchanged.
func TestRefreshClaimFailsWhenBoardReRenderIsUnchanged(t *testing.T) {
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

			// Claim the change at the base clock: this stamps the record in-progress
			// and commits the inline board reflecting that in-progress row.
			claimNode := planningDepsFor(t, cloneOrigin(t, repo.origin))
			claimCtx := ContextImplementation(ctx, claimNode.deps, claimNode.dir, ImplementationContextRequest{ID: id})
			if claimCtx.Result != ResultApplied || claimCtx.Context == nil || !claimCtx.Context.ClaimEligible {
				t.Fatalf("claim context read = %q (reason %q)", claimCtx.Result, claimCtx.Reason)
			}
			claim := ChangeClaim(ctx, claimNode.deps, claimNode.dir, ChangeClaimRequest{ID: id, Version: claimCtx.Context.Change.Version})
			if claim.Result != ResultApplied || claim.Disposition != ClaimDispositionApplied {
				t.Fatalf("claim = (%q, %q), want applied/applied (findings %v)", claim.Result, claim.Disposition, claim.Findings)
			}
			// The claim committed a board that is a fresh render of the corpus.
			assertBoardMatchesCommitted(t, repo.origin, m.branch, claimNode.dir)

			// The record's still-valid version after the claim.
			claimedVersion := blobVersionAt(t, repo.origin, m.branch, recPath)
			tipAfterClaim := originTip(t, repo.origin, m.branch)

			// Refresh a day later, so claimed_at/updated genuinely change on the
			// record while the board would re-render identically.
			refreshNode := planningDepsForClock(t, cloneOrigin(t, repo.origin), advanced)

			// App-level symptom: the exact reported shape.
			res := ChangeRefreshClaim(ctx, refreshNode.deps, refreshNode.dir, ChangeClaimRequest{ID: id, Version: claimedVersion})
			if res.Result != ResultInvalidState {
				t.Fatalf("refresh result = %q, want %q (the reported invalid-state)", res.Result, ResultInvalidState)
			}
			if res.Disposition != string(ResultInvalidState) {
				t.Errorf("refresh disposition = %q, want the tautological %q the bug produces", res.Disposition, string(ResultInvalidState))
			}
			if len(res.Findings) != 0 {
				t.Errorf("refresh findings = %v, want empty — the failure carried no diagnostic", res.Findings)
			}

			// The failed refresh committed nothing.
			if tip := originTip(t, repo.origin, m.branch); tip != tipAfterClaim {
				t.Errorf("failed refresh moved the metadata remote: %q -> %q", tipAfterClaim, tip)
			}

			// Mechanism, with positive evidence: the raw engine outcome is a
			// verify-delta invalid-state Failure, not merely "it failed" — this
			// distinguishes hypothesis 1 (verify-delta) from load-after or
			// idempotency-scan invalid-state.
			rawRes, execErr := rawRefreshClaimOutcome(t, refreshNode, ChangeClaimRequest{ID: id, Version: claimedVersion})
			if rawRes.Disposition != transaction.DispositionFailed {
				t.Fatalf("raw refresh disposition = %q, want %q", rawRes.Disposition, transaction.DispositionFailed)
			}
			f, ok := transaction.AsFailure(execErr)
			if !ok {
				t.Fatalf("failed refresh carried no typed transaction.Failure: %v", execErr)
			}
			if f.Stage != transaction.StageVerifyDelta {
				t.Errorf("failure stage = %q, want %q", f.Stage, transaction.StageVerifyDelta)
			}
			if f.Kind != transaction.KindInvalidState {
				t.Errorf("failure kind = %q, want %q", f.Kind, transaction.KindInvalidState)
			}
			if !strings.Contains(f.Detail, "a declared path is not an actual change") {
				t.Errorf("failure detail = %q, want the declared-but-unchanged-path message", f.Detail)
			}
		})
	}
}

// --- effective base consumed from the domain resolver, not hard-coded --------

// TestEffectiveBaseConsumedFromDomain proves a stacked change's workspace prepare
// and attach ancestry checks run against the PARENT branch base that
// domain.ResolveEffectiveBase resolves — not the integration branch. The child's
// workspace is created at the parent feature branch tip, and the plan committed
// on it (which descends from that tip, not from main) passes attach's descendant
// check. Both metadata modes.
func TestEffectiveBaseConsumedFromDomain(t *testing.T) {
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
