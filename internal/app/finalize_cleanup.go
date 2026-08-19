package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file is `finalize cleanup` and `gate cleanup`: the two ownership-safe
// destructive suffixes of the terminal half. Both are RETRYABLE suffixes, never
// evidence that anything upstream succeeded, and both fail closed on every probe
// they cannot answer — present, cleanly absent, and unknown are three outcomes,
// and only cleanly-absent certifies an already-completed destructive leg
// (learning probe-error-is-not-clean-absence).
//
// `finalize cleanup` runs an ordered suffix over one terminal change: it reloads
// the archived/stacked state and the verified merge destination; repairs the
// terminal backlinks first when needed; removes the feature checkout through the
// landed manifest-fact-driven workspace.Cleanup (never a base recomputed from the
// now-terminal record); deletes the LOCAL feature ref only when the exact
// recorded tip is detached from every worktree AND contained in the verified
// merge chain; deletes the REMOTE feature ref only under an exact old-value lease
// AND only after a fresh probe proves no open child PR still targets it; and
// keeps the cleaned tombstone for replay and health attribution. A stacked-merged
// change retains its workspace and branches until its root reaches the
// integration branch; an out-of-band parent merge with unretargeted children
// archives truthfully but retains the parent branch and reports
// children-retarget-required. Any probe failure, moved ref, unproven ancestry,
// blocked workspace, malformed manifest, or exact-lease rejection returns cleanup
// pending and preserves the resource. It never calls a global worktree prune,
// force-removes a checkout, recursively deletes by pathname, or touches the
// primary, metadata, transaction, sibling, or foreign worktree.
//
// `gate cleanup` removes ONE exact private run directory's logs only after
// validating ownership (a manifest whose run id matches the slot), a durable
// terminal record, no live lock/group, and either durable exact-head green
// evidence (a passed run) or a persisted halt/finalize stop report (a stopped
// run). Failed, signalled, vanished, ambiguous, or unreported runs are retained
// so their diagnostics survive; a cleaned run leaves an owned tombstone receipt
// so a replay reads clean absence PLUS an owned receipt, never a foreign absence.

// Operation keys the two cleanup operations record in their result envelopes.
const (
	OperationFinalizeCleanup = "finalize.cleanup"
	OperationGateCleanup     = "gate.cleanup"
)

// gateCleanupReceiptFile is the owned tombstone `gate cleanup` writes into a run
// directory it cleaned, so a replay reads clean absence PLUS an owned receipt.
const gateCleanupReceiptFile = "cleanup-receipt.json"

// The closed set of dispositions a cleanup result may carry.
const (
	// CleanupDispCleaned: every applicable destructive leg completed.
	CleanupDispCleaned = "cleaned"
	// CleanupDispAlready: the promised clean state already holds (a replay keyed on
	// clean absence plus the owned tombstone/receipt); a verified no-op.
	CleanupDispAlready = "already-clean"
	// CleanupDispPending: a retained-refusal — a probe could not be answered, a ref
	// moved, ancestry is unproven, the workspace is blocked, or a lease was
	// rejected. Nothing was destroyed; the leg is independently retryable.
	CleanupDispPending = "pending"
	// CleanupDispRetained: the resource is deliberately retained (a stacked-merged
	// change kept until its root closes, or a gate run kept for its diagnostics).
	CleanupDispRetained = "retained"
	// CleanupDispChildrenRetargetRequired: the parent archived truthfully but an
	// open child PR still targets its branch; the remote branch is retained.
	CleanupDispChildrenRetargetRequired = "children-retarget-required"
	// CleanupDispRebaseScratchCleared: the one pre-terminal exception — an
	// explicitly-aborted owned rebase that restored its original head; the owned
	// scratch refs and receipt were cleared.
	CleanupDispRebaseScratchCleared = "rebase-scratch-cleared"
)

// The stable machine reasons a cleanup result reports. Message text is
// explanatory and must not be parsed.
const (
	ReasonCleanupNotTerminal      = "not-terminal"
	ReasonCleanupNotFinalizable   = "not-finalizable"
	ReasonCleanupRepoUnresolved   = "repository-unresolved"
	ReasonCleanupProbeUnknown     = "merge-probe-unknown"
	ReasonCleanupNotMerged        = "pr-not-merged"
	ReasonCleanupDestination      = "destination-mismatch"
	ReasonCleanupUnresolvedBase   = "unresolved-base"
	ReasonCleanupWorkspaceProbe   = "workspace-probe-failed"
	ReasonCleanupWorkspaceBlocked = "workspace-blocked"
	ReasonCleanupRefProbe         = "ref-probe-failed"
	ReasonCleanupTipMoved         = "recorded-tip-moved"
	ReasonCleanupAncestryProbe    = "ancestry-probe-failed"
	ReasonCleanupUnreachable      = "tip-not-in-merge-chain"
	ReasonCleanupListProbe        = "worktree-list-probe-failed"
	ReasonCleanupCheckedOut       = "branch-checked-out"
	ReasonCleanupLocalDelete      = "local-ref-delete-failed"
	ReasonCleanupChildProbe       = "child-pr-probe-failed"
	ReasonCleanupRemoteProbe      = "remote-ref-probe-failed"
	ReasonCleanupRemoteMoved      = "remote-ref-moved"
	ReasonCleanupLeaseRejected    = "remote-lease-rejected"
	ReasonCleanupBacklinkPending  = "terminal-backlink-pending"
	ReasonCleanupReceiptRead      = "receipt-read-failed"
	// gate cleanup reasons
	ReasonGateCleanupUnownable = "run-unownable"
	ReasonGateCleanupLive      = "run-live"
	ReasonGateCleanupRetained  = "run-retained"
	ReasonGateCleanupWrite     = "receipt-write-failed"
	ReasonGateCleanupInvalidID = "invalid-run-dir"
)

// CleanupOpResult is the protocol-v1 document both cleanup operations return. It
// names identity, the closed disposition, the refs it removed on a success, and
// — on a refusal or a partial — a stable reason and message plus retryable
// findings. It leaks no authored bytes, no logs, and no credentialed data.
type CleanupOpResult struct {
	Envelope
	ID          int             `json:"id,omitempty"`
	RunDir      string          `json:"run_dir,omitempty"`
	Disposition string          `json:"disposition,omitempty"`
	RemovedRefs []string        `json:"removed_refs,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	Message     string          `json:"message,omitempty"`
	Findings    []StatusFinding `json:"findings"`
}

// HumanText renders a one-line summary naming identity and disposition only.
func (r CleanupOpResult) HumanText() string {
	if r.Result == ResultApplied || r.Result == ResultNoOp {
		if r.RunDir != "" {
			return r.Operation + ": " + r.Disposition + " " + r.RunDir
		}
		return r.Operation + ": change " + itoa(r.ID) + " " + r.Disposition
	}
	if r.Reason != "" {
		return r.Operation + ": " + string(r.Result) + " (" + r.Reason + ")"
	}
	return r.Operation + ": " + string(r.Result)
}

// newCleanupResult stamps the envelope for opKey and normalizes Findings so the
// array marshals as [] on every path.
func newCleanupResult(opKey string, result Result, out CleanupOpResult) CleanupOpResult {
	out.Envelope = NewEnvelope(opKey, result)
	if out.Findings == nil {
		out.Findings = []StatusFinding{}
	}
	return out
}

// cleanupRefusal builds a finalize-cleanup refusal carrying a stable reason,
// message, and disposition.
func cleanupRefusal(result Result, disposition, reason, message string, id int) CleanupOpResult {
	return newCleanupResult(OperationFinalizeCleanup, result, CleanupOpResult{
		ID: id, Disposition: disposition, Reason: reason, Message: message,
	})
}

// cleanupWarning is one retryable pending finding.
func cleanupWarning(code, msg string) StatusFinding {
	return StatusFinding{Code: code, Severity: string(domain.SeverityWarning), Message: msg}
}

// cleanupGit returns the branch-deletion Git seam: the injected CleanupGit when
// set, else the concrete Planning.Client (which satisfies the seam). Production
// leaves CleanupGit nil; a cleanup test injects a faulting wrapper.
func cleanupGit(deps FinalizeDeps) FinalizeCleanupGit {
	if deps.CleanupGit != nil {
		return deps.CleanupGit
	}
	return deps.Planning.Client
}

// FinalizeCleanup runs the ordered ownership-safe destructive suffix over one
// terminal change (or clears the owned scratch of an explicitly-aborted owned
// rebase). Every leg fails closed: a probe error, a moved ref, unproven
// ancestry, a blocked workspace, an open child, or a lease rejection returns
// cleanup pending and preserves the resource.
func FinalizeCleanup(ctx context.Context, deps FinalizeDeps, repoDir string, id int) CleanupOpResult {
	if id <= 0 {
		return newCleanupResult(OperationFinalizeCleanup, ResultInvalidInput, CleanupOpResult{
			ID: id, Findings: []StatusFinding{lifecycleFinding("invalid-id", "id must be a positive change id")},
		})
	}

	cc, refusal := loadCloseoutContext(ctx, deps, repoDir, id)
	if refusal != nil {
		return cleanupFromCloseoutRefusal(*refusal, id)
	}

	switch cc.change.Status() {
	case domain.StatusDone:
		return finalizeCleanupDone(ctx, deps, cc)
	case domain.StatusStackedMerged:
		// Retained until the stack root reaches the integration branch: the
		// workspace and both branches carry the stacked code the root still needs.
		return newCleanupResult(OperationFinalizeCleanup, ResultNoOp, CleanupOpResult{
			ID: id, Disposition: CleanupDispRetained, Reason: ReasonCleanupNotTerminal,
			Message: "change is stacked-merged; its workspace and branches are retained until its root closes",
		})
	default:
		// The one pre-terminal exception: an explicitly-aborted owned rebase that
		// restored its own original head, whose owned scratch may still linger.
		if res, handled := finalizeCleanupAbortedRebase(ctx, deps, cc); handled {
			return res
		}
		return cleanupRefusal(ResultInvalidState, CleanupDispPending, ReasonCleanupNotTerminal,
			"change is not terminal and carries no aborted-rebase scratch to clear; nothing to clean", id)
	}
}

// cleanupFromCloseoutRefusal folds a loadCloseoutContext refusal (a
// CloseoutResult) into a CleanupOpResult with the same result/reason/message.
func cleanupFromCloseoutRefusal(cr CloseoutResult, id int) CleanupOpResult {
	return newCleanupResult(OperationFinalizeCleanup, cr.Result, CleanupOpResult{
		ID: id, Disposition: CleanupDispPending, Reason: cr.Reason, Message: cr.Message,
	})
}

// finalizeCleanupDone runs the destructive suffix over a done, archived change:
// reprobe the verified merge, repair backlinks, remove the workspace, then delete
// the local and remote feature refs under proof.
func finalizeCleanupDone(ctx context.Context, deps FinalizeDeps, cc *closeoutContext) CleanupOpResult {
	id := int(cc.change.ID())

	// Reprobe the verified merge authoritatively (idempotency keyed on the promised
	// state, not a local proxy). A probe error is unknown; a not-merged PR is
	// pending; a merge into a non-integration destination is a mismatch.
	number, ok := parsePRNumber(cc.change.PR().Value)
	if !finalizeHasPRRef(cc.change) || !ok {
		return cleanupRefusal(ResultBlocked, CleanupDispPending, ReasonCleanupNotFinalizable,
			"change carries no canonical pull-request reference to verify before cleanup", id)
	}
	ghRepo, err := deps.GitHub.DiscoverRepository(ctx, cc.repo.PrimaryWorktree)
	if err != nil {
		return cleanupRefusal(ResultExternalFailed, CleanupDispPending, ReasonCleanupRepoUnresolved, err.Error(), id)
	}
	outcome, facts, err := deps.GitHub.ProbeMerged(ctx, ghRepo, number)
	if err != nil {
		return cleanupRefusal(ResultExternalFailed, CleanupDispPending, ReasonCleanupProbeUnknown, err.Error(), id)
	}
	if outcome != githubcli.MergeMerged && outcome != githubcli.MergeAlreadyMerged {
		return cleanupRefusal(ResultBlocked, CleanupDispPending, ReasonCleanupNotMerged,
			"the pull request is not merged; cleanup preserves the resources", id)
	}
	if facts.BaseRef != cc.integrationBranch {
		return cleanupRefusal(ResultBlocked, CleanupDispPending, ReasonCleanupDestination,
			"the verified merge destination is not the integration branch; cleanup preserves the resources", id)
	}

	featureBranch := domain.BranchForSlug(cc.change.Slug())
	featureRef := gitcli.RefName(branchRefPrefix + featureBranch)
	git := cleanupGit(deps)

	var findings []StatusFinding
	var removed []string

	// Leg 1: repair the terminal backlinks first when needed (docket mode). A
	// failed/contended leg is a pending finding; it never blocks the independent
	// ref-deletion legs.
	if f := finalizeCleanupBacklinkRepair(ctx, deps, cc, facts); f != nil {
		findings = append(findings, *f)
	}

	// Leg 2: remove the feature checkout through the landed manifest-fact-driven
	// Cleanup. A blocked workspace or an unanswerable inspection retains the
	// workspace and — because the branch may still be checked out — skips the ref
	// legs entirely.
	workspaceClean, wsFinding := finalizeCleanupWorkspace(ctx, deps, cc)
	if wsFinding != nil {
		findings = append(findings, *wsFinding)
	}
	if !workspaceClean {
		return finalizeCleanupResult(id, CleanupDispPending, removed, findings,
			"the feature workspace could not be cleanly removed; the branches are retained")
	}

	// Leg 3: delete the local feature ref only under exact tip + worktree-detached
	// + merge-chain containment.
	localDone, localRef, localFinding := finalizeCleanupLocalRef(ctx, deps, git, cc, featureRef, featureBranch, facts)
	if localFinding != nil {
		findings = append(findings, *localFinding)
	}
	if localRef != "" {
		removed = append(removed, localRef)
	}

	// Leg 4: delete the remote feature ref only under an exact lease and only after
	// a fresh probe proves no open child PR still targets it.
	remoteDone, remoteRef, childRetarget, remoteFinding := finalizeCleanupRemoteRef(ctx, deps, git, ghRepo, cc, featureRef, featureBranch, facts)
	if remoteFinding != nil {
		findings = append(findings, *remoteFinding)
	}
	if remoteRef != "" {
		removed = append(removed, remoteRef)
	}

	if childRetarget {
		return newCleanupResult(OperationFinalizeCleanup, ResultApplied, CleanupOpResult{
			ID: id, Disposition: CleanupDispChildrenRetargetRequired, RemovedRefs: removed, Findings: findings,
			Reason:  ReasonCleanupChildProbe,
			Message: "an open child pull request still targets this branch; the remote branch is retained until the children are retargeted",
		})
	}
	if localDone && remoteDone && len(findings) == 0 {
		return newCleanupResult(OperationFinalizeCleanup, ResultApplied, CleanupOpResult{
			ID: id, Disposition: CleanupDispCleaned, RemovedRefs: removed, Findings: findings,
			Message: "the terminal change was cleaned: workspace removed and feature refs deleted",
		})
	}
	return finalizeCleanupResult(id, CleanupDispPending, removed, findings,
		"cleanup is partially complete; the retained legs are independently retryable")
}

// finalizeCleanupResult builds a pending/partial cleanup result.
func finalizeCleanupResult(id int, disp string, removed []string, findings []StatusFinding, msg string) CleanupOpResult {
	reason := ""
	if len(findings) > 0 {
		reason = findings[0].Code
	}
	result := ResultBlocked
	if disp == CleanupDispCleaned {
		result = ResultApplied
	}
	return newCleanupResult(OperationFinalizeCleanup, result, CleanupOpResult{
		ID: id, Disposition: disp, RemovedRefs: removed, Findings: findings, Reason: reason, Message: msg,
	})
}

// finalizeCleanupBacklinkRepair re-runs the docket-mode integration-ref backlink
// retarget idempotently. In the normal flow (closeout already retargeted the
// blocks) it is a clean no-op; when closeout left the leg pending it lands the
// exact generated-only patch. A failed/contended leg is a retryable
// terminal-backlink-pending finding — the change stays truthfully done. Main mode
// carries the backlinks in the metadata transaction, so there is no second leg.
func finalizeCleanupBacklinkRepair(ctx context.Context, deps FinalizeDeps, cc *closeoutContext, facts githubcli.MergedFacts) *StatusFinding {
	if cc.pin.Mode != "docket" {
		return nil
	}
	archiveDate, ok := archiveDateFromMerge(facts.MergedAtUTC)
	if !ok {
		return nil
	}
	targets := []closeoutTarget{{
		id: int(cc.change.ID()), activePath: cc.change.Path(), slug: cc.change.Slug(), archivePath: cc.change.Path(),
	}}
	backlinkTargets, err := closeoutBacklinkTargets(cc, targets)
	if err != nil {
		f := cleanupWarning(ReasonCleanupBacklinkPending, "the terminal backlink interior could not be rendered; retry cleanup")
		return &f
	}
	if len(backlinkTargets) == 0 {
		return nil
	}
	op := cleanupBacklinkOp{rootID: int(cc.change.ID()), archiveDate: archiveDate, targets: backlinkTargets}
	res, execErr := deps.Planning.Engine.Execute(ctx, transaction.Request{
		Repository: cc.repo,
		Remote:     originRemote,
		TargetRef:  gitcli.RefName(branchRefPrefix + cc.integrationBranch),
		Loader:     newPlanningLoader(cc.eff),
		Operation:  op,
	})
	result, _ := mapOutcome(res, execErr, ResultInvalidState)
	if result == ResultApplied || result == ResultNoOp {
		return nil
	}
	f := cleanupWarning(ReasonCleanupBacklinkPending, "the integration-ref backlink leg did not land ("+string(result)+"); the sweep will retry it")
	return &f
}

// cleanupBacklinkOp patches the docket:backlink block of each merged
// plan/results artifact on the integration ref to point at the archive path. It
// mirrors closeoutBacklinkOp but returns a VALID no-op plan (a commit subject on
// an empty file set) so an already-retargeted replay reaches the engine's
// empty-plan no-op path rather than an empty-commit-subject refusal — the
// idempotent case a standalone cleanup replay exercises that closeout never does.
type cleanupBacklinkOp struct {
	rootID      int
	archiveDate string
	targets     []closeoutBacklinkTarget
}

func (o cleanupBacklinkOp) Key() transaction.OperationKey {
	return transaction.OperationKey(OperationFinalizeCloseoutBacklink)
}

func (o cleanupBacklinkOp) Plan(ctx context.Context, st transaction.AttemptState) (transaction.MutationPlan, transaction.OperationResult, error) {
	var files []transaction.FileMutation
	for _, tg := range o.targets {
		for _, p := range tg.artifactPaths {
			original, present, err := readTreeBlob(ctx, st.Tree, p)
			if err != nil {
				return transaction.MutationPlan{}, transaction.OperationResult{}, err
			}
			if !present {
				continue
			}
			doc, err := document.Parse(original)
			if err != nil {
				return transaction.MutationPlan{}, transaction.OperationResult{}, err
			}
			if _, ok := doc.Block(backlinkBlockName); !ok {
				continue
			}
			var ps document.PatchSet
			ps.ReplaceBlock(backlinkBlockName, tg.interior)
			updated, err := doc.Apply(ps)
			if err != nil {
				return transaction.MutationPlan{}, transaction.OperationResult{}, err
			}
			if string(updated) == string(original) {
				continue
			}
			files = append(files, transaction.FileMutation{Path: gitcli.RepoPath(p), Kind: transaction.MutationReplace, Bytes: updated})
		}
	}
	subject := "change " + itoa(o.rootID) + " terminal backlinks verified (cleanup)"
	receipt, err := json.Marshal(closeoutBacklinkReceipt{ArchiveDate: o.archiveDate, Op: OperationFinalizeCloseoutBacklink, Root: o.rootID})
	if err != nil {
		return transaction.MutationPlan{}, transaction.OperationResult{}, err
	}
	if len(files) == 0 {
		// A valid no-op plan carries a commit subject AND a receipt (validatePlan
		// requires both before the engine's len(Files)==0 no-op branch is reached),
		// so an already-retargeted replay is a clean no-op rather than an
		// empty-subject/empty-receipt refusal.
		return transaction.MutationPlan{CommitSubject: subject, Receipt: receipt}, transaction.OperationResult{}, nil
	}
	return transaction.MutationPlan{Files: files, CommitSubject: subject, Receipt: receipt}, transaction.OperationResult{}, nil
}

// finalizeCleanupWorkspace removes the feature checkout through workspace.Cleanup.
// It returns (true, nil) when the checkout is cleanly gone (cleaned or an
// already-clean tombstone), and (false, finding) when the base is unresolved, the
// target is malformed, the inspection could not be answered, or the workspace is
// blocked — each retaining the workspace byte-untouched.
func finalizeCleanupWorkspace(ctx context.Context, deps FinalizeDeps, cc *closeoutContext) (bool, *StatusFinding) {
	base := domain.ResolveEffectiveBase(cc.snap, cc.change, domain.NewBranchFacts(nil))
	if base.Kind != domain.BaseResolved {
		f := cleanupWarning(ReasonCleanupUnresolvedBase, "the change's effective base did not resolve; the workspace is retained")
		return false, &f
	}
	target, err := workspace.NewTarget(cc.change.ID(), cc.change.Slug(), base)
	if err != nil {
		f := cleanupWarning(ReasonCleanupUnresolvedBase, "the change's workspace target is malformed; the workspace is retained")
		return false, &f
	}
	res, err := deps.Workspace.Cleanup(ctx, workspace.CleanupRequest{Repository: cc.repo, Target: target})
	if err != nil {
		f := cleanupWarning(ReasonCleanupWorkspaceProbe, "the feature workspace could not be inspected; it is retained")
		return false, &f
	}
	switch res.Disposition {
	case workspace.CleanupCleaned, workspace.CleanupAlreadyClean:
		return true, nil
	default:
		f := cleanupWarning(ReasonCleanupWorkspaceBlocked, "the feature workspace is not a clean, ready checkout; it is retained")
		return false, &f
	}
}

// finalizeCleanupLocalRef deletes the local feature ref only when the live tip
// equals the exact merged head, the ref is checked out in no worktree, and the
// merged head is contained in the verified merge chain (an ancestor of the
// freshly-fetched integration tip). A cleanly-absent ref is an already-done leg;
// every unprovable probe or unmet proof is a pending finding with the ref intact.
func finalizeCleanupLocalRef(ctx context.Context, deps FinalizeDeps, git FinalizeCleanupGit, cc *closeoutContext, featureRef gitcli.RefName, featureBranch string, facts githubcli.MergedFacts) (done bool, removedRef string, finding *StatusFinding) {
	expectedTip := gitcli.ObjectID(facts.HeadOID)
	if !validFullObjectID(string(expectedTip)) {
		f := cleanupWarning(ReasonCleanupRefProbe, "the verified merge carries no usable head; the local ref is retained")
		return false, "", &f
	}

	tip, err := git.ResolveRef(ctx, cc.repo, featureRef)
	if err != nil {
		if f, ok := gitcli.AsFailure(err); ok && f.Kind == gitcli.KindRefUnavailable {
			return true, "", nil // cleanly absent: already deleted
		}
		w := cleanupWarning(ReasonCleanupRefProbe, "the local feature ref could not be probed; it is retained")
		return false, "", &w
	}
	if tip != expectedTip {
		w := cleanupWarning(ReasonCleanupTipMoved, "the local feature ref moved off the merged head; it is retained")
		return false, "", &w
	}

	// Merge-chain containment: fetch the integration tip and prove the merged head
	// is an ancestor of it. A fetch or ancestry probe error is unknown (retain).
	rev, err := git.FetchBranch(ctx, cc.repo, originRemote, gitcli.RefName(branchRefPrefix+cc.integrationBranch))
	if err != nil {
		w := cleanupWarning(ReasonCleanupAncestryProbe, "the integration tip could not be fetched; the local ref is retained")
		return false, "", &w
	}
	reachable, err := git.IsAncestor(ctx, cc.repo, expectedTip, rev.Commit)
	if err != nil {
		w := cleanupWarning(ReasonCleanupAncestryProbe, "merge-chain containment could not be proven; the local ref is retained")
		return false, "", &w
	}
	if !reachable {
		w := cleanupWarning(ReasonCleanupUnreachable, "the recorded tip is not contained in the verified merge chain; the local ref is retained")
		return false, "", &w
	}

	// Worktree-detached: the merged branch must be checked out nowhere.
	infos, err := git.ListWorktrees(ctx, cc.repo)
	if err != nil {
		w := cleanupWarning(ReasonCleanupListProbe, "the worktree list could not be read; the local ref is retained")
		return false, "", &w
	}
	for _, wi := range infos {
		if wi.Branch == featureRef {
			w := cleanupWarning(ReasonCleanupCheckedOut, "the feature ref is still checked out in a worktree; it is retained")
			return false, "", &w
		}
	}

	if err := git.DeleteLocalBranchChecked(ctx, cc.repo, featureRef, expectedTip); err != nil {
		w := cleanupWarning(ReasonCleanupLocalDelete, "the checked local ref delete did not land; the ref is retained")
		return false, "", &w
	}
	return true, "local:" + featureBranch, nil
}

// finalizeCleanupRemoteRef deletes the remote feature ref only under an exact
// old-value lease and only after a fresh probe proves no open child PR still
// targets the branch. An open child retains the remote and signals
// children-retarget-required; a cleanly-absent remote is an already-done leg;
// every unprovable probe, moved remote, or rejected lease is a pending finding
// with the remote intact.
func finalizeCleanupRemoteRef(ctx context.Context, deps FinalizeDeps, git FinalizeCleanupGit, ghRepo githubcli.Repository, cc *closeoutContext, featureRef gitcli.RefName, featureBranch string, facts githubcli.MergedFacts) (done bool, removedRef string, childRetarget bool, finding *StatusFinding) {
	// Fresh no-open-child probe: any direct stack child whose live PR is open and
	// targets this branch retains the remote. A probe error is unknown (retain).
	for _, childID := range domain.StackChildren(cc.snap, cc.change.ID()) {
		child, ok := cc.snap.Change(childID)
		if ok != domain.LookupFound {
			continue
		}
		childHead := domain.BranchForSlug(child.Slug())
		prs, err := deps.GitHub.FindOpenPullRequestsByHead(ctx, ghRepo, childHead)
		if err != nil {
			w := cleanupWarning(ReasonCleanupChildProbe, "an open-child probe could not be answered; the remote ref is retained")
			return false, "", false, &w
		}
		for _, pr := range prs {
			if pr.State == githubcli.StateOpen && pr.BaseBranch == featureBranch {
				return false, "", true, nil
			}
		}
	}

	expectedTip := gitcli.ObjectID(facts.HeadOID)
	rr, err := git.ProbeRemoteBranch(ctx, cc.repo, originRemote, featureRef)
	if err != nil {
		w := cleanupWarning(ReasonCleanupRemoteProbe, "the remote feature ref could not be probed; it is retained")
		return false, "", false, &w
	}
	if rr.State == gitcli.RemoteRefAbsent {
		return true, "", false, nil // cleanly absent: already deleted
	}
	if rr.Commit != expectedTip {
		w := cleanupWarning(ReasonCleanupRemoteMoved, "the remote feature ref moved off the merged head; it is retained")
		return false, "", false, &w
	}

	out, err := git.DeleteRemoteRefLease(ctx, cc.repo, originRemote, featureRef, expectedTip)
	if err != nil {
		w := cleanupWarning(ReasonCleanupRemoteProbe, "the remote ref delete could not be issued; it is retained")
		return false, "", false, &w
	}
	if out.Disposition != gitcli.PushApplied {
		w := cleanupWarning(ReasonCleanupLeaseRejected, "the exact-lease remote delete did not land; the remote ref is retained")
		return false, "", false, &w
	}
	return true, "remote:" + featureBranch, false, nil
}

// finalizeCleanupAbortedRebase clears the owned scratch (the two anchor refs and
// the receipt) of an explicitly-aborted owned rebase whose head was restored to
// the receipt's recorded original head. It returns (result, true) when this
// pre-terminal exception applies, and (_, false) when there is no such
// aborted-restored scratch (the caller then refuses the non-terminal change).
func finalizeCleanupAbortedRebase(ctx context.Context, deps FinalizeDeps, cc *closeoutContext) (CleanupOpResult, bool) {
	id := int(cc.change.ID())
	if deps.Workspace == nil {
		return CleanupOpResult{}, false
	}
	featureRef := gitcli.RefName(branchRefPrefix + domain.BranchForSlug(cc.change.Slug()))
	metaDir := workspace.MetaDir(cc.repo.CommonDir, featureRef)

	rec, present, err := deps.Workspace.ReadRebaseReceipt(ctx, metaDir)
	if err != nil {
		return cleanupRefusal(ResultExternalFailed, CleanupDispPending, ReasonCleanupReceiptRead,
			"the owned rebase receipt could not be read; nothing is cleared", id), true
	}
	if !present {
		return CleanupOpResult{}, false
	}

	// The rewrite must be undone: the live feature tip equals the receipt's
	// original head. A probe error retains everything; a tip that still differs is
	// not a restored abort, so the non-terminal change is refused by the caller.
	tip, err := deps.Planning.Client.ResolveRef(ctx, cc.repo, featureRef)
	if err != nil {
		return cleanupRefusal(ResultExternalFailed, CleanupDispPending, ReasonCleanupRefProbe,
			"the feature ref could not be probed; the owned scratch is retained", id), true
	}
	if string(tip) != rec.OrigHead {
		return CleanupOpResult{}, false
	}

	// No rebase may be in progress in the feature workspace. An in-progress or
	// conflicted rebase is live work — retain, never adopt.
	if wsPath := finalizeCleanupWorkspacePath(ctx, deps, cc); wsPath != "" {
		if st, serr := deps.Planning.Client.RebaseState(ctx, wsPath); serr == nil && st.Disposition != gitcli.RebaseUnchanged {
			return cleanupRefusal(ResultBlocked, CleanupDispPending, ReasonCleanupNotTerminal,
				"a rebase is in progress in the feature workspace; the owned scratch is retained", id), true
		}
	}

	prefix := ownedRefPrefixFor(id)
	_ = deps.Planning.Client.DeleteOwnedRef(ctx, cc.repo, gitcli.RefName(prefix+"/orig"))
	_ = deps.Planning.Client.DeleteOwnedRef(ctx, cc.repo, gitcli.RefName(prefix+"/base"))
	_ = deps.Workspace.ClearRebaseReceipt(ctx, metaDir)

	return newCleanupResult(OperationFinalizeCleanup, ResultApplied, CleanupOpResult{
		ID: id, Disposition: CleanupDispRebaseScratchCleared,
		Message: "the aborted owned rebase left its head restored; its owned scratch refs and receipt were cleared",
	}), true
}

// finalizeCleanupWorkspacePath resolves the feature workspace's checkout path
// through a read-only inspect, returning "" when the base is unresolved or the
// inspection cannot name a path (the caller then skips the rebase-state check).
func finalizeCleanupWorkspacePath(ctx context.Context, deps FinalizeDeps, cc *closeoutContext) string {
	base := domain.ResolveEffectiveBase(cc.snap, cc.change, domain.NewBranchFacts(nil))
	if base.Kind != domain.BaseResolved {
		return ""
	}
	target, err := workspace.NewTarget(cc.change.ID(), cc.change.Slug(), base)
	if err != nil {
		return ""
	}
	insp, err := deps.Workspace.Inspect(ctx, workspace.InspectRequest{Repository: cc.repo, Target: target})
	if err != nil {
		return ""
	}
	if !insp.Registered {
		return ""
	}
	return insp.Path
}

// --- gate cleanup ---------------------------------------------------------

// gateCleanupReceipt is the owned tombstone `gate cleanup` writes into a cleaned
// run directory. Field order is alphabetical for a canonical compact form.
type gateCleanupReceipt struct {
	CleanedAt string `json:"cleaned_at"`
	Op        string `json:"op"`
	RunID     string `json:"run_id"`
}

// GateCleanup removes one exact private run directory's logs only after
// validating ownership, a durable terminal record, no live lock/group, and
// either durable exact-head green evidence (a passed run) or a persisted
// halt/finalize stop report (a stopped run). Failed, signalled, vanished,
// ambiguous, or unreported runs are retained; a cleaned run leaves an owned
// tombstone receipt so a replay reads clean absence PLUS an owned receipt.
func GateCleanup(ctx context.Context, _ FinalizeDeps, runDir string) CleanupOpResult {
	if runDir == "" || !filepath.IsAbs(runDir) {
		return newCleanupResult(OperationGateCleanup, ResultInvalidInput, CleanupOpResult{
			RunDir: runDir, Disposition: CleanupDispPending, Reason: ReasonGateCleanupInvalidID,
			Message: "run directory must be an absolute path",
		})
	}

	// Receipt-first idempotency: an owned tombstone proves WE cleaned this exact
	// run (clean absence plus an owned receipt), so a replay is a no-op. A foreign
	// absence carries no owned receipt and is never adopted.
	if rec, ok := readGateCleanupReceipt(runDir); ok {
		return newCleanupResult(OperationGateCleanup, ResultNoOp, CleanupOpResult{
			RunDir: runDir, Disposition: CleanupDispAlready,
			Message: "run " + rec.RunID + " was already cleaned (owned tombstone receipt present)",
		})
	}

	obs, res, reason := observeGateRun(runDir)
	if obs == nil {
		// An unownable or malformed run slot: retain, never treat as clean.
		return newCleanupResult(OperationGateCleanup, res, CleanupOpResult{
			RunDir: runDir, Disposition: CleanupDispRetained, Reason: ReasonGateCleanupUnownable, Message: reason,
		})
	}

	if !gateRunRemovable(obs) {
		return newCleanupResult(OperationGateCleanup, ResultNoOp, CleanupOpResult{
			RunDir: runDir, Disposition: CleanupDispRetained, Reason: ReasonGateCleanupRetained,
			Message: "run is " + string(obs.State) + " without durable exact-head evidence or a persisted stop report; retained",
		})
	}

	// Write the owned tombstone FIRST, so a crash before the log removal still
	// replays as an owned no-op, then remove only the named log files (never a
	// recursive pathname delete).
	if err := writeGateCleanupReceipt(runDir, obs.RunID); err != nil {
		return newCleanupResult(OperationGateCleanup, ResultExternalFailed, CleanupOpResult{
			RunDir: runDir, Disposition: CleanupDispPending, Reason: ReasonGateCleanupWrite,
			Message: "the cleanup receipt could not be written; the run is retained",
		})
	}
	for _, name := range []string{"stdout.log", "stderr.log", "supervisor.log"} {
		if err := os.Remove(filepath.Join(runDir, name)); err != nil && !os.IsNotExist(err) {
			// The receipt is already durable; a stubborn log is a retryable pending
			// finding, never a false clean.
			return newCleanupResult(OperationGateCleanup, ResultExternalFailed, CleanupOpResult{
				RunDir: runDir, Disposition: CleanupDispPending, Reason: ReasonGateCleanupWrite,
				Message: "a run log could not be removed; retry cleanup",
			})
		}
	}
	return newCleanupResult(OperationGateCleanup, ResultApplied, CleanupOpResult{
		RunDir: runDir, Disposition: CleanupDispCleaned,
		Message: "run " + obs.RunID + " logs removed; owned tombstone receipt retained for replay and health attribution",
	})
}

// readGateCleanupReceipt reads and validates the owned tombstone in runDir. It
// returns ok only for a well-formed receipt whose run id matches the slot name,
// so a foreign or malformed file is never adopted as our own clean proof.
func readGateCleanupReceipt(runDir string) (gateCleanupReceipt, bool) {
	raw, err := os.ReadFile(filepath.Join(runDir, gateCleanupReceiptFile))
	if err != nil {
		return gateCleanupReceipt{}, false
	}
	var rec gateCleanupReceipt
	if err := json.Unmarshal(raw, &rec); err != nil {
		return gateCleanupReceipt{}, false
	}
	if rec.Op != OperationGateCleanup || rec.RunID == "" || rec.RunID != filepath.Base(runDir) {
		return gateCleanupReceipt{}, false
	}
	return rec, true
}

// writeGateCleanupReceipt writes the owned tombstone atomically (a temp file
// beside the destination, then a rename) so a partial write never reads as a
// valid receipt.
func writeGateCleanupReceipt(runDir, runID string) error {
	rec := gateCleanupReceipt{CleanedAt: "", Op: OperationGateCleanup, RunID: runID}
	body, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(runDir, gateCleanupReceiptFile+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, filepath.Join(runDir, gateCleanupReceiptFile)); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// ensure the concrete StatusFinding severity import is exercised even when a
// build trims the warning path.
