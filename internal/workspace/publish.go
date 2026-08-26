package workspace

// This file owns PublishHead: the ownership-safe, idempotent publication of one
// owned ready workspace's exact feature HEAD onto the exact remote feature ref.
// It follows the spec §"Feature-branch publication" order:
//
//	1. validate the target and repository paths, then take the per-workspace
//	   operation lock (serializing against Prepare/Inspect-refresh/Cleanup);
//	2. reinspect the owned ready workspace: the manifest must be owned and ready,
//	   Git must register exactly the recorded canonical path on the exact feature
//	   ref with HEAD == the ref tip, the recorded base must still be reachable, and
//	   the tracked/untracked delta must be empty — a dirty or inconsistent
//	   workspace is refused (invalid-state), never repaired, published, or forced;
//	3. probe the authoritative remote feature ref structurally (ProbeRemoteBranch);
//	   an unobservable remote is `unknown` with no fabricated remote id;
//	4. the remote already equal to the local HEAD is `already-published` (keyed on
//	   the REMOTE state — never a clean tree, a local branch, an upstream
//	   configuration, or a process exit; learnings: idempotency-keying);
//	5. an absent remote ref is created under the absent-ref lease (PushCreateLease);
//	6. an existing remote ref whose exact commit is an ancestor of the local HEAD
//	   is fast-forwarded under the expected-old lease (PushLease); and
//	7. an existing remote ref that is neither equal nor an ancestor is refused as
//	   `contended` — NO force push, reset, merge, or rebase.
//
// After a push result that is not structurally conclusive (PushFailed), the
// operation re-derives the remote state from a fresh authoritative probe within
// the remaining caller budget: exact equality with the intended head is
// `published` (an adopted lost-response), an observed different commit is
// `contended`, an unobservable remote is `unknown`, and a ref that probes cleanly
// absent is a definite `failed` — the push did not land (learnings:
// cas-re-read-fresh-origin, probe-error-is-not-clean-absence). The promise is THE
// EXACT COMMIT REACHED THE EXACT REMOTE FEATURE REF.

import (
	"context"
	"path/filepath"

	"github.com/danielhanold/docket/internal/gitcli"
)

// publishOp is the Op recorded on every PublishHead Failure.
const publishOp = "publish-head"

// PublishRequest names the repository, the remote to publish to, and the fully
// validated target whose owned ready workspace HEAD is published.
type PublishRequest struct {
	Repository gitcli.Repository
	Remote     gitcli.RemoteName
	Target     Target
}

// PublishResult is the value outcome of a PublishHead. Head is the intended local
// feature HEAD (set whenever it was established by reinspection); Remote is the
// observed remote head when the remote state was established (the empty id when
// the remote was unobservable, so an `unknown` never carries a fabricated id).
type PublishResult struct {
	Disposition PublishDisposition
	Head        gitcli.ObjectID
	Remote      gitcli.ObjectID
}

// PublishHead publishes one owned ready workspace's exact feature HEAD onto the
// exact remote feature ref, idempotently. published, already-published,
// contended, and unknown are value dispositions returned with a nil error (no
// force/reset/merge/rebase is ever performed); a refusal of a dirty or
// inconsistent workspace and a definite push failure are `failed` error returns
// with the remote untouched by this operation.
func (s *Service) PublishHead(ctx context.Context, req PublishRequest) (PublishResult, error) {
	if err := validatePublishTarget(req.Target); err != nil {
		return PublishResult{Disposition: PublishFailed}, err
	}
	repo := req.Repository
	if repo.CommonDir == "" || !filepath.IsAbs(repo.CommonDir) || repo.PrimaryWorktree == "" || !filepath.IsAbs(repo.PrimaryWorktree) {
		return PublishResult{Disposition: PublishFailed}, &Failure{Op: publishOp, Stage: "validate", Kind: KindInvalidInput, Detail: "repository paths are not absolute"}
	}
	if req.Remote == "" {
		return PublishResult{Disposition: PublishFailed}, &Failure{Op: publishOp, Stage: "validate", Kind: KindInvalidInput, Detail: "empty remote name"}
	}
	target := req.Target
	dir := workspaceDir(repo.CommonDir, target.FeatureRef)
	intendedPath := filepath.Join(repo.PrimaryWorktree, ".worktrees", target.Slug)

	// Serialize the whole operation against concurrent Prepare/Inspect/Cleanup on
	// this one workspace.
	releaseOp, err := acquireOperationLock(dir)
	if err != nil {
		return PublishResult{Disposition: PublishFailed}, &Failure{Op: publishOp, Stage: "lock", Kind: KindExternal, Detail: "acquiring workspace operation lock", Err: err}
	}
	defer releaseOp()

	// Reinspect the owned ready workspace; a dirty or inconsistent workspace is a
	// refusal, and the intended local head comes from the reinspected registration.
	localHead, ferr := s.reinspectForPublish(ctx, repo, dir, target, intendedPath)
	if ferr != nil {
		return PublishResult{Disposition: PublishFailed}, ferr
	}

	ref := target.FeatureRef

	// Probe the authoritative remote feature ref. An unobservable remote is
	// unknown — never a clean absence, and never a fabricated remote id.
	rr, err := s.git.ProbeRemoteBranch(ctx, repo, req.Remote, ref)
	if err != nil {
		return PublishResult{Disposition: PublishUnknown, Head: localHead}, nil
	}

	switch rr.State {
	case gitcli.RemoteRefAbsent:
		out, perr := s.git.PushCreateLease(ctx, repo, req.Remote, ref, localHead)
		if perr != nil {
			return PublishResult{Disposition: PublishFailed, Head: localHead}, mapGitFailure(publishOp, "push", perr)
		}
		return s.resolvePushOutcome(ctx, repo, req.Remote, ref, localHead, out)

	case gitcli.RemoteRefFound:
		if rr.Commit == localHead {
			// Keyed on the remote state: the exact commit already reached the exact ref.
			return PublishResult{Disposition: PublishAlreadyPublished, Head: localHead, Remote: rr.Commit}, nil
		}
		// A fast-forward is safe ONLY when the observed remote commit is an ancestor
		// of the local HEAD: PushLease carries `--force-with-lease`, which forces a
		// non-fast-forward update whenever the lease matches, so calling it on a
		// divergent ref would OVERWRITE the interloper. The ancestry check gates that.
		//
		// A genuine fast-forward's observed commit is always present in the local
		// object store (it lies on the local HEAD's own history), so IsAncestor
		// resolves it and returns true. IsAncestor therefore only errors when the
		// observed commit is NOT present locally — which means it cannot be an
		// ancestor of the local HEAD, i.e. the ref has diverged. Both a false answer
		// and an unresolvable-observed error are `contended`: PublishHead refuses and
		// pushes nothing (fail-closed — no force, reset, merge, or rebase; the origin
		// keeps the interloper). A real fast-forward can never be misclassified this
		// way, and reinspection has just proven Git itself is working.
		ancestor, aerr := s.git.IsAncestor(ctx, repo, rr.Commit, localHead)
		if aerr != nil || !ancestor {
			return PublishResult{Disposition: PublishContended, Head: localHead, Remote: rr.Commit}, nil
		}
		out, perr := s.git.PushLease(ctx, repo, req.Remote, ref, localHead, rr.Commit)
		if perr != nil {
			return PublishResult{Disposition: PublishFailed, Head: localHead}, mapGitFailure(publishOp, "push", perr)
		}
		return s.resolvePushOutcome(ctx, repo, req.Remote, ref, localHead, out)

	default:
		return PublishResult{Disposition: PublishFailed, Head: localHead}, &Failure{Op: publishOp, Stage: "probe", Kind: KindInvalidOutput, Detail: "remote probe returned an unknown state"}
	}
}

// resolvePushOutcome maps a structural push outcome to a publish disposition. An
// applied push is conclusive (Git confirmed the ref update): published. A lease
// loss — the primitive's own re-probe proved the remote moved to a commit the
// pushed commit does not contain — is contended. A plain failure is not
// structurally conclusive, so the remote state is re-derived from a fresh probe.
func (s *Service) resolvePushOutcome(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, ref gitcli.RefName, localHead gitcli.ObjectID, out gitcli.PushOutcome) (PublishResult, error) {
	switch out.Disposition {
	case gitcli.PushApplied:
		return PublishResult{Disposition: PublishPublished, Head: localHead, Remote: out.Remote}, nil
	case gitcli.PushLeaseLost:
		return PublishResult{Disposition: PublishContended, Head: localHead, Remote: out.Remote}, nil
	default:
		return s.reprobeAfterPush(ctx, repo, remote, ref, localHead)
	}
}

// reprobeAfterPush re-derives the remote state after a non-conclusive push,
// within the remaining caller budget. An unobservable remote is unknown (the
// effect may have landed but cannot be established — never a clean absence); the
// exact intended head is published (an adopted lost-response); a different commit
// is contended; and a cleanly-absent ref is a definite failed — the push did not
// land.
func (s *Service) reprobeAfterPush(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, ref gitcli.RefName, localHead gitcli.ObjectID) (PublishResult, error) {
	rr, err := s.git.ProbeRemoteBranch(ctx, repo, remote, ref)
	if err != nil {
		return PublishResult{Disposition: PublishUnknown, Head: localHead}, nil
	}
	if rr.State != gitcli.RemoteRefFound {
		return PublishResult{Disposition: PublishFailed, Head: localHead}, &Failure{Op: publishOp, Stage: "push", Kind: KindExternal, Detail: "push failed and the remote ref is absent"}
	}
	if rr.Commit == localHead {
		return PublishResult{Disposition: PublishPublished, Head: localHead, Remote: rr.Commit}, nil
	}
	return PublishResult{Disposition: PublishContended, Head: localHead, Remote: rr.Commit}, nil
}

// reinspectForPublish proves the workspace is the owned, ready, clean checkout
// exactly attached to the feature ref, and returns the intended local head (the
// feature ref tip == the registered HEAD). Every inconsistency — an absent,
// foreign, unowned, or non-ready manifest; a missing feature ref; a detached,
// relocated, or mismatched registration; an unreachable recorded base; or a
// non-empty dirty delta — is an invalid-state refusal. Every probe error is an
// external failure: an unreadable manifest or Git probe never reads as a clean,
// publishable state (learnings: probe-error-is-not-clean-absence).
func (s *Service) reinspectForPublish(ctx context.Context, repo gitcli.Repository, dir string, target Target, intendedPath string) (gitcli.ObjectID, *Failure) {
	m, status, cerr := classifyManifest(dir)
	switch status {
	case manifestUnknown:
		return "", &Failure{Op: publishOp, Stage: "inventory", Kind: KindExternal, Detail: "workspace manifest is unreadable", Err: cerr}
	case manifestAbsent:
		return "", &Failure{Op: publishOp, Stage: "verify", Kind: KindInvalidState, Detail: "no workspace manifest"}
	case manifestForeign:
		return "", &Failure{Op: publishOp, Stage: "verify", Kind: KindInvalidState, Detail: "manifest is foreign or malformed"}
	}
	if !ownsManifest(m, repo, target, intendedPath) {
		return "", &Failure{Op: publishOp, Stage: "verify", Kind: KindInvalidState, Detail: "manifest identity does not match this repository or target"}
	}
	if m.Phase != PhaseReady {
		return "", &Failure{Op: publishOp, Stage: "verify", Kind: KindInvalidState, Detail: "workspace is not in a ready phase"}
	}

	// The feature ref must exist; a cleanly-absent ref is an inconsistency
	// (invalid-state), any other probe error is an external failure.
	branchHead, branchErr := s.git.ResolveRef(ctx, repo, target.FeatureRef)
	if branchErr != nil {
		if f, ok := gitcli.AsFailure(branchErr); ok && f.Kind == gitcli.KindRefUnavailable {
			return "", &Failure{Op: publishOp, Stage: "verify", Kind: KindInvalidState, Detail: "feature ref is missing"}
		}
		return "", mapGitFailure(publishOp, "inventory", branchErr)
	}

	// Live registration proof: exactly the recorded path, attached to the feature
	// ref (not detached), with HEAD == the ref tip.
	infos, err := s.git.ListWorktrees(ctx, repo)
	if err != nil {
		return "", mapGitFailure(publishOp, "inventory", err)
	}
	reg, registered := worktreeAt(infos, m.Path)
	if !registered {
		return "", &Failure{Op: publishOp, Stage: "verify", Kind: KindInvalidState, Detail: "recorded path is not registered"}
	}
	if reg.Detached || reg.Branch != target.FeatureRef {
		return "", &Failure{Op: publishOp, Stage: "verify", Kind: KindInvalidState, Detail: "registration is detached or on a different branch"}
	}
	if reg.Head != branchHead {
		return "", &Failure{Op: publishOp, Stage: "verify", Kind: KindInvalidState, Detail: "registered HEAD is not the feature ref tip"}
	}

	// The recorded base must still be reachable from the head, or the branch was
	// moved out of band: refuse, never reset.
	reachable, err := s.git.IsAncestor(ctx, repo, m.BaseCommit, reg.Head)
	if err != nil {
		return "", mapGitFailure(publishOp, "inventory", err)
	}
	if !reachable {
		return "", &Failure{Op: publishOp, Stage: "verify", Kind: KindInvalidState, Detail: "recorded base is not reachable from the head"}
	}

	// Exact tracked/untracked delta: any dirty, staged, untracked, or conflicted
	// path refuses publication. A probe error is an external failure, never a false
	// clean.
	dirty, err := s.dirtyPaths(ctx, m.Path)
	if err != nil {
		return "", remapPublishInventory(err)
	}
	if len(dirty) > 0 {
		return "", &Failure{Op: publishOp, Stage: "verify", Kind: KindInvalidState, Detail: "workspace is dirty"}
	}
	return reg.Head, nil
}

// remapPublishInventory re-tags a Failure produced by the shared dirtyPaths
// helper (which records Op "inspect") onto the publish operation. A non-Failure
// error is wrapped as an external publish failure.
func remapPublishInventory(err error) *Failure {
	if f, ok := AsFailure(err); ok {
		return &Failure{Op: publishOp, Stage: "inventory", Kind: f.Kind, Detail: f.Detail, Err: f.Err}
	}
	return &Failure{Op: publishOp, Stage: "inventory", Kind: KindExternal, Detail: "reading workspace delta", Err: err}
}

// validatePublishTarget rejects a target that is not self-consistent with its
// slug and base, reusing NewTarget's rules and re-tagging any rejection to the
// publish operation.
func validatePublishTarget(t Target) error {
	derived, err := NewTarget(t.ChangeID, t.Slug, t.Base, t.FeatureBranch())
	if err != nil {
		return &Failure{Op: publishOp, Stage: "validate", Kind: KindInvalidInput, Detail: "target is not self-consistent with its slug and base"}
	}
	if derived != t {
		return &Failure{Op: publishOp, Stage: "validate", Kind: KindInvalidInput, Detail: "target fields are not self-consistent with its slug and base"}
	}
	return nil
}
