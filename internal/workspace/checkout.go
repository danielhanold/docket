package workspace

// This file owns Checkout: adopting an implemented change's pushed feature
// branch into its own docket-owned worktree at <primary>/.worktrees/<slug>, at
// the head the caller proved from the change's open PR and that the freshly
// fetched remote branch agrees with.
//
// Prepare refuses any branch it did not create, because a fresh build has
// nothing to prove that branch against. Checkout deliberately adopts one: the
// expected head is proven, and every action is checked against it. It adopts a
// local branch at or behind that head, and removes a foreign worktree holding
// the branch only when every removalProof holds, through git's own clean-only
// remove. A branch that is behind moves only after git's own in-use check has
// attached it to the target worktree, and then only by a compare-and-swap
// fast-forward inside that worktree: `git worktree list` shows a worktree
// mid-rebase or mid-bisect as detached, so only git's check sees that its
// operation still owns the branch, and a branch moved under it would break the
// operation's continue and abort. Every other local state is a blocked or
// contended value that leaves the repository as it was (only the per-workspace
// lock dir and fetched remote-tracking refs change).
//
// The order mirrors Prepare: validate, take the per-workspace operation lock,
// fetch, classify the manifest slot, then finish every probe and decision
// before the first mutation. A probe error is an error, never absence. An
// attach publishes an allocating manifest before it creates anything and
// advances it to ready only after reinspecting live Git, so an interrupted
// checkout is resumed by the next run.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

// checkoutOp is the Op recorded on every Checkout Failure.
const checkoutOp = "checkout"

// CheckoutDisposition is the closed set of value outcomes of a checkout. A
// failure is an error return, never a disposition.
type CheckoutDisposition string

const (
	CheckoutExisting  CheckoutDisposition = "existing"
	CheckoutAttached  CheckoutDisposition = "attached"
	CheckoutBlocked   CheckoutDisposition = "blocked"
	CheckoutContended CheckoutDisposition = "contended"
)

// Checkout refusal reasons, carried in CheckoutResult.Reason.
const (
	CheckoutReasonHeadMismatch          = "head-mismatch"
	CheckoutReasonRemoteFeatureAbsent   = "remote-feature-absent"
	CheckoutReasonForeignManifest       = "foreign-manifest"
	CheckoutReasonWorkspaceDirty        = "workspace-dirty"
	CheckoutReasonLocalHeadMismatch     = "local-head-mismatch"
	CheckoutReasonWorkspaceNotReady     = "workspace-not-ready"
	CheckoutReasonLocalBranchDiverged   = "local-branch-diverged"
	CheckoutReasonBranchInPrimary       = "branch-in-primary-checkout"
	CheckoutReasonForeignWorktreeUnsafe = "foreign-worktree-unsafe"
	CheckoutReasonTargetPathOccupied    = "target-path-occupied"
	CheckoutReasonLocalBranchMoved      = "local-branch-moved"
)

// Removal proofs a foreign worktree failed, carried in
// CheckoutResult.FailedProof alongside CheckoutReasonForeignWorktreeUnsafe.
const (
	ProofSeveralHolders   = "several-holders"
	ProofMetadataWorktree = "metadata-worktree"
	ProofPathMissing      = "path-missing"
	ProofLocked           = "locked"
	ProofManifestClaims   = "manifest-claims-path"
	ProofNotAttached      = "not-attached"
	ProofOperation        = "operation-in-progress"
	ProofHeadMismatch     = "head-mismatch"
	ProofDirty            = "dirty"
	ProofGitRefused       = "git-refused"
)

// CheckoutRequest names the repository, the remote holding the pushed feature
// branch, the validated target, the expected head the caller proved from the
// change's open PR, and the metadata worktree (when there is one), which is
// never removed.
type CheckoutRequest struct {
	Repository       gitcli.Repository
	Remote           gitcli.RemoteName
	Target           Target
	ExpectedHead     gitcli.ObjectID
	MetadataWorktree string
}

// CheckoutResult is a checkout's value outcome. Path is the canonical workspace
// path. RemovedWorktree is the canonical path of a foreign worktree removed on
// the way to attaching; FastForwardedFrom is the local branch's tip before a
// fast-forward. FailedProof names the removal proof a foreign worktree failed.
type CheckoutResult struct {
	Disposition       CheckoutDisposition
	Reason            string
	Message           string
	FailedProof       string
	Path              string
	FeatureRef        gitcli.RefName
	BaseRef           gitcli.RefName
	BaseCommit        gitcli.ObjectID
	HeadCommit        gitcli.ObjectID
	RemovedWorktree   string
	FastForwardedFrom gitcli.ObjectID
}

// refuse returns r as a blocked or contended outcome with reason and message.
func (r CheckoutResult) refuse(d CheckoutDisposition, reason, msg string) CheckoutResult {
	r.Disposition = d
	r.Reason = reason
	r.Message = msg
	return r
}

// unsafeForeign returns r as a foreign-worktree-unsafe refusal naming proof.
func (r CheckoutResult) unsafeForeign(proof, msg string) CheckoutResult {
	r.FailedProof = proof
	return r.refuse(CheckoutBlocked, CheckoutReasonForeignWorktreeUnsafe, msg)
}

// checkoutPlan is the one action the attach arm decided on before any mutation.
type checkoutPlan int

const (
	planCreate      checkoutPlan = iota // no local branch: create it at the expected head
	planAttach                          // local branch at the expected head: attach it
	planFastForward                     // local branch behind: attach at its tip (unless already registered), then fast-forward in place
	planRegistered                      // a resumed checkout already attached the branch at the target, at the head
)

// unchanged is the suffix every refusal message carries.
const unchanged = "; nothing was changed"

// Checkout adopts the target's pushed feature branch at req.ExpectedHead into
// <primary>/.worktrees/<slug>. Its arm is selected by the manifest slot: an
// owned ready manifest is checked read-only (existing or blocked), an owned
// allocating manifest resumes the attach, and an absent one attaches fresh.
func (s *Service) Checkout(ctx context.Context, req CheckoutRequest) (CheckoutResult, error) {
	if err := s.validateIdentity(ctx, req.Repository); err != nil {
		return CheckoutResult{}, asCheckoutFailure(err)
	}
	if err := validatePreparedTarget(req.Target); err != nil {
		return CheckoutResult{}, asCheckoutFailure(err)
	}
	if req.Remote == "" {
		return CheckoutResult{}, &Failure{Op: checkoutOp, Stage: "validate", Kind: KindInvalidInput, Detail: "empty remote name"}
	}
	if !validObjectID(req.ExpectedHead) {
		return CheckoutResult{}, &Failure{Op: checkoutOp, Stage: "validate", Kind: KindInvalidInput, Detail: "expected head is not a full lowercase object id"}
	}

	repo := req.Repository
	target := req.Target
	dir := workspaceDir(repo.CommonDir, target.FeatureRef)
	intended := filepath.Join(repo.PrimaryWorktree, ".worktrees", target.Slug)
	out := CheckoutResult{Path: intended, FeatureRef: target.FeatureRef, BaseRef: target.BaseRef}

	release, err := acquireOperationLock(dir)
	if err != nil {
		return CheckoutResult{}, &Failure{Op: checkoutOp, Stage: "lock", Kind: KindExternal, Detail: "acquiring workspace operation lock", Err: err}
	}
	defer release()

	// The remote branch must exist and agree with the caller's proven head.
	rev, err := s.git.FetchBranch(ctx, repo, req.Remote, target.FeatureRef)
	if err != nil {
		if f, ok := gitcli.AsFailure(err); ok && f.Kind == gitcli.KindRefUnavailable {
			return out.refuse(CheckoutBlocked, CheckoutReasonRemoteFeatureAbsent,
				fmt.Sprintf("remote %s has no branch %s; push it, or correct the change's recorded branch%s", req.Remote, target.FeatureBranch(), unchanged)), nil
		}
		return CheckoutResult{}, mapGitFailure(checkoutOp, "fetch", err)
	}
	if rev.Commit != req.ExpectedHead {
		return out.refuse(CheckoutContended, CheckoutReasonHeadMismatch,
			fmt.Sprintf("%s/%s is at %s but the expected head is %s; re-read the PR head and retry%s", req.Remote, target.FeatureBranch(), rev.Commit, req.ExpectedHead, unchanged)), nil
	}

	m, status, cerr := classifyManifest(dir)
	switch status {
	case manifestUnknown:
		return CheckoutResult{}, &Failure{Op: checkoutOp, Stage: "inventory", Kind: KindExternal, Detail: "workspace manifest is unreadable", Err: cerr}
	case manifestForeign:
		return out.refuse(CheckoutBlocked, CheckoutReasonForeignManifest,
			fmt.Sprintf("the workspace record for %s in %s is not one docket can read (%s)%s%s", target.FeatureBranch(), dir, boundedDetail(cerr.Error()), foreignManifestRemedy(dir), unchanged)), nil
	case manifestValid:
		if !ownsManifest(m, repo, target, intended) {
			return out.refuse(CheckoutBlocked, CheckoutReasonForeignManifest,
				fmt.Sprintf("the workspace record for %s in %s belongs to another repository, change, or path%s%s", target.FeatureBranch(), dir, foreignManifestRemedy(dir), unchanged)), nil
		}
		switch m.Phase {
		case PhaseReady:
			return s.checkoutExisting(ctx, req, out)
		case PhaseAllocating:
			return s.checkoutAttach(ctx, req, out, dir, intended, &m)
		default:
			return out.refuse(CheckoutBlocked, CheckoutReasonWorkspaceNotReady,
				fmt.Sprintf("the workspace for %s at %s is recorded as %s; it was left as is", target.FeatureBranch(), intended, m.Phase)), nil
		}
	}
	return s.checkoutAttach(ctx, req, out, dir, intended, nil)
}

// foreignManifestRemedy is the way out of a foreign-manifest refusal: with the
// record directory moved aside the slot is cleanly absent, and the same checkout
// attaches. A docket command still holding that directory must finish first.
func foreignManifestRemedy(dir string) string {
	return fmt.Sprintf("; inspect %s and, if no docket command is using it, move %s aside and retry", filepath.Join(dir, manifestFileName), dir)
}

// checkoutExisting is the owned-ready arm: a read-only Inspect decides between
// the existing no-op and a refusal. It never repairs an owned workspace.
func (s *Service) checkoutExisting(ctx context.Context, req CheckoutRequest, out CheckoutResult) (CheckoutResult, error) {
	insp, err := s.Inspect(ctx, InspectRequest{Repository: req.Repository, Target: req.Target})
	if err != nil {
		return CheckoutResult{}, asCheckoutFailure(err)
	}
	out.BaseCommit = insp.BaseCommit
	out.HeadCommit = insp.HeadCommit
	switch {
	case insp.Kind == StateReady && insp.HeadCommit == req.ExpectedHead:
		out.Disposition = CheckoutExisting
		out.Message = fmt.Sprintf("%s is already checked out at %s in %s; it was left as is", req.Target.FeatureBranch(), insp.HeadCommit, out.Path)
		return out, nil
	case insp.Kind == StateReady:
		return out.refuse(CheckoutBlocked, CheckoutReasonLocalHeadMismatch,
			fmt.Sprintf("the workspace at %s is at %s, not the expected head %s; it was left as is", out.Path, insp.HeadCommit, req.ExpectedHead)), nil
	case insp.Kind == StateDirty:
		return out.refuse(CheckoutBlocked, CheckoutReasonWorkspaceDirty,
			fmt.Sprintf("the workspace at %s has %d uncommitted or untracked path(s); it was left as is", out.Path, len(insp.DirtyPaths))), nil
	default:
		detail := string(insp.Kind)
		if insp.Detail != "" {
			detail += ": " + insp.Detail
		}
		return out.refuse(CheckoutBlocked, CheckoutReasonWorkspaceNotReady,
			fmt.Sprintf("the workspace at %s is not ready (%s); it was left as is", out.Path, detail)), nil
	}
}

// checkoutAttach is the attach arm, fresh (resume == nil) or resuming an
// interrupted checkout's allocating manifest. Every probe and decision precedes
// the first mutation.
func (s *Service) checkoutAttach(ctx context.Context, req CheckoutRequest, out CheckoutResult, dir, intended string, resume *Manifest) (CheckoutResult, error) {
	repo := req.Repository
	target := req.Target
	head := req.ExpectedHead
	branch := target.FeatureBranch()

	// 1. Who holds the branch, and what is registered at the target path.
	infos, err := s.git.ListWorktrees(ctx, repo)
	if err != nil {
		return CheckoutResult{}, mapGitFailure(checkoutOp, "inventory", err)
	}
	var holders []gitcli.WorktreeInfo
	var atTarget *gitcli.WorktreeInfo
	for i := range infos {
		if !infos[i].Detached && infos[i].Branch == target.FeatureRef {
			holders = append(holders, infos[i])
		}
		if atTarget == nil && canonicalOrLexical(infos[i].Path) == intended {
			atTarget = &infos[i]
		}
	}

	// 2. The holder decides between the primary refusal, our own resumed
	// registration, and a foreign removal candidate.
	var candidate *gitcli.WorktreeInfo
	candidateHolds, registered, holderAtTarget := false, false, false
	if len(holders) > 1 {
		paths := make([]string, 0, len(holders))
		for _, h := range holders {
			paths = append(paths, canonicalOrLexical(h.Path))
		}
		return out.unsafeForeign(ProofSeveralHolders,
			fmt.Sprintf("%s is checked out in several worktrees (%s); keep at most one by hand and retry%s", branch, strings.Join(paths, ", "), unchanged)), nil
	}
	if len(holders) == 1 {
		hp := canonicalOrLexical(holders[0].Path)
		holderAtTarget = hp == intended
		switch {
		case hp == repo.PrimaryWorktree:
			return out.refuse(CheckoutBlocked, CheckoutReasonBranchInPrimary,
				fmt.Sprintf("%s is checked out in the main checkout %s; switch it to another branch (git -C %s switch %s) and retry%s", branch, hp, hp, target.Base.Branch, unchanged)), nil
		case holderAtTarget && resume != nil:
			registered = true
		default:
			candidate = &holders[0]
			candidateHolds = true
		}
	}

	// 3. The target path: a registration that does not hold the branch is a
	// candidate only when it is detached and nothing holds the branch; an
	// unregistered path is never touched.
	if atTarget != nil && !holderAtTarget {
		if atTarget.Detached && len(holders) == 0 {
			candidate = atTarget
		} else {
			return out.refuse(CheckoutBlocked, CheckoutReasonTargetPathOccupied,
				fmt.Sprintf("%s is a registered worktree on %s; remove it (git worktree remove %s) or, if its folder is gone, run git worktree prune, then retry%s", intended, describeRegistration(*atTarget), intended, unchanged)), nil
		}
	}
	if atTarget == nil {
		present, perr := pathPresent(intended)
		if perr != nil {
			return CheckoutResult{}, &Failure{Op: checkoutOp, Stage: "inventory", Kind: KindExternal, Detail: "stat of target path failed", Err: perr}
		}
		if present {
			return out.refuse(CheckoutBlocked, CheckoutReasonTargetPathOccupied,
				fmt.Sprintf("%s exists but is not a worktree; move it aside and retry%s", intended, unchanged)), nil
		}
	}

	// 4. The local branch. A holding candidate's head is judged by its removal
	// proof instead.
	tip, terr := s.git.ResolveRef(ctx, repo, target.FeatureRef)
	branchPresent := terr == nil
	if terr != nil {
		if f, ok := gitcli.AsFailure(terr); !ok || f.Kind != gitcli.KindRefUnavailable {
			return CheckoutResult{}, mapGitFailure(checkoutOp, "inventory", terr)
		}
	}
	var plan checkoutPlan
	switch {
	case candidateHolds:
		plan = planAttach
	case registered && tip == head:
		// A fast-forward killed between its ref swap and its tree update leaves
		// the branch at the head with the old files staged against it.
		interrupted, err := s.git.InterruptedFastForward(ctx, intended, target.FeatureRef)
		if err != nil {
			return CheckoutResult{}, mapGitFailure(checkoutOp, "inventory", err)
		}
		if interrupted {
			return out.refuse(CheckoutBlocked, CheckoutReasonWorkspaceNotReady,
				fmt.Sprintf("the interrupted checkout at %s stopped mid fast-forward: %s moved to %s but its files did not; finish it with git -C %s reset --merge HEAD (it refuses rather than overwrite a local edit), never commit the staged changes, and retry", intended, branch, head, intended)), nil
		}
		plan = planRegistered
	case registered:
		behind, err := s.git.IsAncestorIgnoringReplacements(ctx, repo, tip, head)
		if err != nil {
			return CheckoutResult{}, mapGitFailure(checkoutOp, "inventory", err)
		}
		if !behind {
			return out.refuse(CheckoutBlocked, CheckoutReasonLocalHeadMismatch,
				fmt.Sprintf("the interrupted workspace at %s holds %s at %s, not the expected head %s; it was left as is", intended, branch, tip, head)), nil
		}
		plan = planFastForward
	case !branchPresent:
		plan = planCreate
	case tip == head:
		plan = planAttach
	default:
		behind, err := s.git.IsAncestorIgnoringReplacements(ctx, repo, tip, head)
		if err != nil {
			return CheckoutResult{}, mapGitFailure(checkoutOp, "inventory", err)
		}
		if !behind {
			return out.refuse(CheckoutBlocked, CheckoutReasonLocalBranchDiverged,
				fmt.Sprintf("local branch %s is at %s, which is ahead of or diverged from the expected head %s; keep it under another name (git branch %s-local %s), delete it (git branch -D %s), and retry%s", branch, tip, head, branch, tip, branch, unchanged)), nil
		}
		plan = planFastForward
	}

	// 5. The base commit: a resumed checkout keeps its recorded base, which the
	// head must still contain; a fresh one records the merge-base of the head and
	// the freshly fetched base tip.
	var base gitcli.ObjectID
	if resume != nil {
		contained, err := s.git.IsAncestor(ctx, repo, resume.BaseCommit, head)
		if err != nil {
			return CheckoutResult{}, mapGitFailure(checkoutOp, "inventory", err)
		}
		if !contained {
			return out.refuse(CheckoutBlocked, CheckoutReasonWorkspaceNotReady,
				fmt.Sprintf("the interrupted checkout of %s at %s recorded base %s, which the expected head %s does not contain; it was left as is", branch, intended, resume.BaseCommit, head)), nil
		}
		base = resume.BaseCommit
	} else {
		baseRev, err := s.git.FetchBranch(ctx, repo, req.Remote, target.BaseRef)
		if err != nil {
			return CheckoutResult{}, mapGitFailure(checkoutOp, "fetch", err)
		}
		mb, found, err := s.git.MergeBase(ctx, repo, head, baseRev.Commit)
		if err != nil {
			return CheckoutResult{}, mapGitFailure(checkoutOp, "inventory", err)
		}
		if !found {
			return CheckoutResult{}, &Failure{Op: checkoutOp, Stage: "inventory", Kind: KindInvalidState, Detail: "the expected head shares no history with the base branch"}
		}
		base = mb
	}
	out.BaseCommit = base

	// 6. A foreign candidate goes only when every proof holds, read last, and
	// through git's clean-only remove, which rechecks at the destructive boundary.
	if candidate != nil {
		cpath := canonicalOrLexical(candidate.Path)
		proof, remedy, err := s.removalProof(ctx, req, *candidate)
		if err != nil {
			return CheckoutResult{}, err
		}
		if proof != "" {
			return out.unsafeForeign(proof, remedy), nil
		}
		if err := s.git.RemoveWorktreeClean(ctx, repo, candidate.Path); err != nil {
			if f, ok := gitcli.AsFailure(err); ok && f.Kind == gitcli.KindCommandFailed {
				return out.unsafeForeign(ProofGitRefused,
					fmt.Sprintf("git refused to remove %s (%s); it was left as is", cpath, f.Detail)), nil
			}
			return CheckoutResult{}, mapGitFailure(checkoutOp, "remove", err)
		}
		out.RemovedWorktree = cpath
	}

	// 7. Publish the allocating breadcrumb (a resume reuses its own), then carry
	// out the plan.
	var man Manifest
	if resume != nil {
		man = *resume
	} else {
		now := time.Now().UTC().Format(time.RFC3339)
		man = Manifest{
			Schema:     manifestSchemaVersion,
			ID:         workspaceID(target.FeatureRef),
			CommonDir:  repo.CommonDir,
			ChangeID:   target.ChangeID,
			Slug:       target.Slug,
			FeatureRef: target.FeatureRef,
			BaseRef:    target.BaseRef,
			BaseCommit: base,
			Path:       intended,
			Phase:      PhaseAllocating,
			CreatedUTC: now,
			UpdatedUTC: now,
		}
		if err := publishAllocating(checkoutOp, workspacesRoot(repo.CommonDir), dir, man); err != nil {
			return CheckoutResult{}, err
		}
	}
	switch plan {
	case planCreate:
		if err := s.git.AddBranchWorktree(ctx, repo, intended, target.FeatureRef, head); err != nil {
			return CheckoutResult{}, mapGitFailure(checkoutOp, "worktree", err)
		}
	case planFastForward:
		// Attach first, so git's own in-use check refuses a branch that a
		// rebase or bisect in another worktree still owns before anything
		// moves it; the branch then moves only inside the worktree it is
		// checked out in, by compare-and-swap from the inspected tip.
		if !registered {
			if err := s.git.AttachBranchWorktree(ctx, repo, intended, target.FeatureRef); err != nil {
				return CheckoutResult{}, mapGitFailure(checkoutOp, "worktree", err)
			}
		}
		if err := s.git.FastForwardCheckedOutBranch(ctx, intended, target.FeatureRef, tip, head); err != nil {
			now, rerr := s.git.ResolveRef(ctx, repo, target.FeatureRef)
			gone := false
			if rerr != nil {
				if f, ok := gitcli.AsFailure(rerr); !ok || f.Kind != gitcli.KindRefUnavailable {
					return CheckoutResult{}, mapGitFailure(checkoutOp, "allocate", err)
				}
				gone = true
			}
			if gone || now != tip {
				return out.refuse(CheckoutContended, CheckoutReasonLocalBranchMoved,
					fmt.Sprintf("local branch %s moved from %s while it was being checked out; the interrupted checkout was recorded, so a rerun resumes it", branch, tip)), nil
			}
			return CheckoutResult{}, mapGitFailure(checkoutOp, "allocate", err)
		}
		out.FastForwardedFrom = tip
	case planAttach:
		if err := s.git.AttachBranchWorktree(ctx, repo, intended, target.FeatureRef); err != nil {
			return CheckoutResult{}, mapGitFailure(checkoutOp, "worktree", err)
		}
	case planRegistered:
		// A resumed checkout already attached the branch here; only the
		// reinspection and the manifest advance remain.
	}

	// 8. Reinspect live Git and advance the manifest to ready.
	ws, err := s.reinspectAndFinalize(ctx, checkoutOp, repo, dir, man, intended, target, base, PrepareCreated)
	if err != nil {
		return CheckoutResult{}, err
	}
	if ws.HeadCommit != head {
		return CheckoutResult{}, &Failure{Op: checkoutOp, Stage: "verify", Kind: KindInvalidState, Detail: "the attached workspace is not at the expected head"}
	}
	out.Disposition = CheckoutAttached
	out.BaseCommit = ws.BaseCommit
	out.HeadCommit = ws.HeadCommit
	out.Message = fmt.Sprintf("checked out %s at %s in %s", branch, head, intended)
	return out, nil
}

// removalProof reads, last and fresh, whether a foreign worktree may be removed.
// It returns the first failed proof with a remedy valid in exactly that state
// (both empty when every proof holds); any probe error is a Failure at stage
// probe, never a pass.
func (s *Service) removalProof(ctx context.Context, req CheckoutRequest, info gitcli.WorktreeInfo) (proof, remedy string, err error) {
	path := canonicalOrLexical(info.Path)
	branch := req.Target.FeatureBranch()

	if req.MetadataWorktree != "" && path == canonicalOrLexical(req.MetadataWorktree) {
		return ProofMetadataWorktree, fmt.Sprintf("%s is the docket metadata worktree but has %s checked out; move it back to the metadata branch by hand%s", path, branch, unchanged), nil
	}
	present, perr := pathPresent(path)
	if perr != nil {
		return "", "", &Failure{Op: checkoutOp, Stage: "probe", Kind: KindExternal, Detail: "stat of foreign worktree failed", Err: perr}
	}
	if info.Prunable || !present {
		prune := "git worktree prune"
		if info.Locked {
			prune = "git worktree unlock " + path + " and git worktree prune"
		}
		return ProofPathMissing, fmt.Sprintf("the worktree registered at %s no longer exists; run %s and retry%s", path, prune, unchanged), nil
	}
	if info.Locked {
		return ProofLocked, fmt.Sprintf("%s is locked; if nothing needs it, run git worktree unlock %s and retry%s", path, path, unchanged), nil
	}
	claimed, err := manifestClaimsPath(req.Repository.CommonDir, workspaceID(req.Target.FeatureRef), path)
	if err != nil {
		return "", "", err
	}
	if claimed {
		return ProofManifestClaims, fmt.Sprintf("another change's docket workspace record names %s, so it is not removed%s", path, unchanged), nil
	}
	st, err := s.git.WorktreeCheckoutState(ctx, path)
	if err != nil {
		return "", "", mapGitFailure(checkoutOp, "probe", err)
	}
	if st.Detached || st.Branch != req.Target.FeatureRef {
		return ProofNotAttached, fmt.Sprintf("%s is not attached to %s; if it holds nothing you need, run git worktree remove %s and retry%s", path, branch, path, unchanged), nil
	}
	if st.OperationInProgress {
		return ProofOperation, fmt.Sprintf("%s has a rebase, merge, cherry-pick, revert, or bisect in progress; finish or abort it there and retry%s", path, unchanged), nil
	}
	if st.Head != req.ExpectedHead {
		return ProofHeadMismatch, fmt.Sprintf("%s is at %s, not the expected head %s; push or move its commits, then run git worktree remove %s and retry%s", path, st.Head, req.ExpectedHead, path, unchanged), nil
	}
	changes, err := s.git.ChangedPaths(ctx, path)
	if err != nil {
		return "", "", mapGitFailure(checkoutOp, "probe", err)
	}
	if len(changes) > 0 {
		return ProofDirty, fmt.Sprintf("%s has %d uncommitted or untracked path(s); commit, stash, or discard them there and retry%s", path, len(changes), unchanged), nil
	}
	return "", "", nil
}

// manifestClaimsPath reports whether any workspace manifest other than ownID's
// records path. An unreadable root or manifest is a probe Failure, never a
// "no"; an undecodable manifest names no path and is skipped.
func manifestClaimsPath(commonDir, ownID, path string) (bool, error) {
	root := workspacesRoot(commonDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, &Failure{Op: checkoutOp, Stage: "probe", Kind: KindExternal, Detail: "reading workspace records failed", Err: err}
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == ownID {
			continue
		}
		m, status, cerr := classifyManifest(filepath.Join(root, e.Name()))
		switch status {
		case manifestUnknown:
			return false, &Failure{Op: checkoutOp, Stage: "probe", Kind: KindExternal, Detail: "a workspace record is unreadable", Err: cerr}
		case manifestValid:
			if canonicalOrLexical(m.Path) == path {
				return true, nil
			}
		}
	}
	return false, nil
}

// canonicalOrLexical is canonicalizePath when p resolves; otherwise its parent
// is resolved and the base name rejoined, so a stale registration whose folder
// is gone still equals a canonical target path; the last resort is the cleaned
// absolute path.
func canonicalOrLexical(p string) string {
	if cp, err := canonicalizePath(p); err == nil {
		return cp
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	if parent, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		return filepath.Join(parent, filepath.Base(abs))
	}
	return filepath.Clean(abs)
}

// describeRegistration names what a registered worktree has checked out.
func describeRegistration(info gitcli.WorktreeInfo) string {
	if info.Detached || info.Branch == "" {
		return "a detached HEAD"
	}
	return strings.TrimPrefix(string(info.Branch), "refs/heads/")
}

// asCheckoutFailure re-tags a Failure from a shared step (Prepare's validation,
// Inspect) to the checkout operation. Each such Failure is freshly built, so the
// re-tag mutates nothing shared.
func asCheckoutFailure(err error) error {
	if f, ok := AsFailure(err); ok {
		f.Op = checkoutOp
	}
	return err
}
