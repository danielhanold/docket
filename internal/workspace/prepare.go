package workspace

// This file owns Prepare: the ownership-safe, idempotent creation of one feature
// workspace. This task implements the FRESH-ALLOCATION arm — a target with no
// local feature ref, no remote feature ref, no target path, and no Git worktree
// registration. It follows the spec §"Prepare request and result" order:
//
//	1. validate repository identity and target;
//	2. acquire the per-workspace operation lock (first publication of the
//	   allocating manifest happens under the short registry lock);
//	3. fetch the resolved base branch from the remote and retain the exact commit
//	   — never a cached tracking ref after a failed freshness op;
//	4. inventory target path, worktree registration, local feature ref, and
//	   manifest phase, each as a three-outcome probe;
//	5/6. manifest-proven ready/interrupted arms — Task 6;
//	7. fresh allocation: require all four absent, then add ONE branch-attached
//	   worktree at the exact fetched base via AddBranchWorktree (never -B, never
//	   reset, never force-remove);
//	8. reinspect registration/ref/HEAD/ancestry, then atomically advance the
//	   manifest to ready and return the verified facts.
//
// The allocating manifest carries the exact fetched base commit, so it is
// published only after the fetch resolves it — the manifest schema forbids an
// empty base commit, and a crash breadcrumb without the base is useless to a
// resume anyway. Nothing is published until the all-absent inventory passes, so
// a not-yet-supported arm leaves the repository byte-untouched.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

// prepareOp is the Op recorded on every Prepare Failure.
const prepareOp = "prepare"

// PrepareRequest names the repository, the remote to fetch the base from, and
// the fully validated target. All three are copied into local values; the
// service exposes no mutable reference to any of them.
type PrepareRequest struct {
	Repository gitcli.Repository
	Remote     gitcli.RemoteName
	Target     Target
}

// Workspace is the verified result of a successful Prepare: the reinspected
// facts of the checkout, never an echo of the request. Dirty is reported, never
// repaired.
type Workspace struct {
	ID          string
	Path        string // canonical
	FeatureRef  gitcli.RefName
	BaseRef     gitcli.RefName
	BaseCommit  gitcli.ObjectID
	HeadCommit  gitcli.ObjectID
	Dirty       bool
	Disposition PrepareDisposition
}

// Prepare is safe to call repeatedly. This task implements the fresh-allocation
// path; the existing/resume/blocked arms (a manifest already on disk, or a
// pre-existing artifact with no manifest) are Task 6 and are reported here as
// typed invalid-state failures because no fresh-path test reaches them.
func (s *Service) Prepare(ctx context.Context, req PrepareRequest) (Workspace, error) {
	// Step 1: validate identity and target before creating any directory or branch.
	if err := s.validateIdentity(ctx, req.Repository); err != nil {
		return Workspace{}, err
	}
	if err := validatePreparedTarget(req.Target); err != nil {
		return Workspace{}, err
	}
	if req.Remote == "" {
		return Workspace{}, &Failure{Op: prepareOp, Stage: "validate", Kind: KindInvalidInput, Detail: "empty remote name"}
	}

	target := req.Target
	commonDir := req.Repository.CommonDir
	root := workspacesRoot(commonDir)
	dir := workspaceDir(commonDir, target.FeatureRef)
	intendedPath := filepath.Join(req.Repository.PrimaryWorktree, ".worktrees", target.Slug)

	// Step 2: acquire the per-workspace operation lock for the whole operation.
	releaseOp, err := acquireOperationLock(dir)
	if err != nil {
		return Workspace{}, &Failure{Op: prepareOp, Stage: "lock", Kind: KindExternal, Detail: "acquiring workspace operation lock", Err: err}
	}
	defer releaseOp()

	// A manifest already on disk means this is not a first attempt: existing or
	// resume, both Task 6. loadManifest is three-outcome, so an unreadable or
	// malformed manifest is an error, never mistaken for clean absence.
	if m, present, err := loadManifest(dir); err != nil {
		return Workspace{}, &Failure{Op: prepareOp, Stage: "inventory", Kind: KindInvalidState, Detail: "existing manifest is unreadable or malformed", Err: err}
	} else if present {
		return Workspace{}, &Failure{Op: prepareOp, Stage: "inventory", Kind: KindInvalidState, Detail: fmt.Sprintf("workspace already has a %s manifest; existing/resume handling is a later task", m.Phase)}
	}

	// Step 3: fetch the resolved base branch and retain the exact commit.
	rev, err := s.git.FetchBranch(ctx, req.Repository, req.Remote, target.BaseRef)
	if err != nil {
		return Workspace{}, mapGitFailure(prepareOp, "fetch", err)
	}
	baseCommit := rev.Commit

	// Steps 4 & 7: inventory the four fresh-allocation preconditions. A present
	// artifact with no manifest is the blocked/adopt matrix (Task 6); with no
	// manifest published yet, returning here leaves the repository untouched.
	if err := s.requireFreshlyAbsent(ctx, req.Repository, req.Remote, target, intendedPath); err != nil {
		return Workspace{}, err
	}

	// Publish the allocating manifest under the short registry lock — the crash
	// breadcrumb recording the exact base commit — then create the worktree.
	now := time.Now().UTC().Format(time.RFC3339)
	man := Manifest{
		Schema:     manifestSchemaVersion,
		ID:         workspaceID(target.FeatureRef),
		CommonDir:  commonDir,
		ChangeID:   target.ChangeID,
		Slug:       target.Slug,
		FeatureRef: target.FeatureRef,
		BaseRef:    target.BaseRef,
		BaseCommit: baseCommit,
		Path:       intendedPath,
		Phase:      PhaseAllocating,
		CreatedUTC: now,
		UpdatedUTC: now,
	}
	if err := publishAllocating(root, dir, man); err != nil {
		return Workspace{}, err
	}

	// Step 7: add ONE branch-attached worktree at the exact fetched base. This
	// path never passes -B and never resets: a colliding local branch would be
	// git's own error, and the all-absent inventory above already excluded it.
	if err := s.git.AddBranchWorktree(ctx, req.Repository, intendedPath, target.FeatureRef, baseCommit); err != nil {
		return Workspace{}, mapGitFailure(prepareOp, "worktree", err)
	}

	// Step 8: reinspect and advance the manifest to ready.
	return s.reinspectAndFinalize(ctx, req.Repository, dir, man, intendedPath, target, baseCommit)
}

// validateIdentity confirms the supplied Repository is internally consistent by
// re-discovering it from its own primary worktree and requiring an exact match.
// A Repository whose CommonDir belongs to a different repository — or whose
// primary worktree is not the canonical one — is invalid-input, caught before
// any directory or branch is created.
func (s *Service) validateIdentity(ctx context.Context, repo gitcli.Repository) error {
	if repo.PrimaryWorktree == "" || !filepath.IsAbs(repo.PrimaryWorktree) {
		return &Failure{Op: prepareOp, Stage: "validate", Kind: KindInvalidInput, Detail: "primary worktree is not an absolute path"}
	}
	if repo.CommonDir == "" || !filepath.IsAbs(repo.CommonDir) {
		return &Failure{Op: prepareOp, Stage: "validate", Kind: KindInvalidInput, Detail: "common dir is not an absolute path"}
	}
	discovered, err := s.git.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repo.PrimaryWorktree})
	if err != nil {
		return mapGitFailure(prepareOp, "validate", err)
	}
	if discovered != repo {
		return &Failure{Op: prepareOp, Stage: "validate", Kind: KindInvalidInput, Detail: "repository identity does not match discovery from its primary worktree"}
	}
	return nil
}

// validatePreparedTarget re-derives the target from its own inputs and requires
// an exact match, so a hand-built Target whose feature ref, base ref, or fields
// were not produced by NewTarget's rules is rejected as invalid-input. This
// reuses every rule NewTarget enforces rather than restating them.
func validatePreparedTarget(t Target) error {
	derived, err := NewTarget(t.ChangeID, t.Slug, t.Base)
	if err != nil {
		return err // already a *Failure{Op:"prepare", Stage:"validate", invalid-input}
	}
	if derived != t {
		return &Failure{Op: prepareOp, Stage: "validate", Kind: KindInvalidInput, Detail: "target fields are not self-consistent with the slug and base"}
	}
	return nil
}

// requireFreshlyAbsent proves all four fresh-allocation preconditions: the local
// feature ref, the remote feature ref, the target path, and the Git worktree
// registration must every one be cleanly absent. Each probe is three-outcome —
// an errored probe is an external/invalid-output failure, NEVER read as clean
// absence, so a probe that cannot see the resource never licenses a create. A
// present artifact is the blocked/adopt matrix (Task 6) and is reported as a
// typed invalid-state failure here; no manifest has been published, so the
// repository is left byte-untouched.
func (s *Service) requireFreshlyAbsent(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, target Target, intendedPath string) error {
	// Local feature ref: a ResolveRef that resolves means the branch exists; a
	// ref-unavailable failure means it is cleanly absent; any other error is real.
	if _, err := s.git.ResolveRef(ctx, repo, target.FeatureRef); err == nil {
		return &Failure{Op: prepareOp, Stage: "inventory", Kind: KindInvalidState, Detail: "local feature branch already exists; adoption is a later task"}
	} else if f, ok := gitcli.AsFailure(err); !ok || f.Kind != gitcli.KindRefUnavailable {
		return mapGitFailure(prepareOp, "inventory", err)
	}

	// Remote feature ref: ProbeRemoteBranch distinguishes found/absent/error.
	rr, err := s.git.ProbeRemoteBranch(ctx, repo, remote, target.FeatureRef)
	if err != nil {
		return mapGitFailure(prepareOp, "inventory", err)
	}
	if rr.State != gitcli.RemoteRefAbsent {
		return &Failure{Op: prepareOp, Stage: "inventory", Kind: KindInvalidState, Detail: "remote feature branch already exists; adoption is a later task"}
	}

	// Target path: an existing path (of any kind) with no manifest is blocked.
	// os.Lstat does not follow a final symlink, so a symlink placed at the path
	// still counts as present.
	if present, err := pathPresent(intendedPath); err != nil {
		return &Failure{Op: prepareOp, Stage: "inventory", Kind: KindExternal, Detail: "stat of target path failed", Err: err}
	} else if present {
		return &Failure{Op: prepareOp, Stage: "inventory", Kind: KindInvalidState, Detail: "target path already exists; blocked handling is a later task"}
	}

	// Git worktree registration at the path.
	infos, err := s.git.ListWorktrees(ctx, repo)
	if err != nil {
		return mapGitFailure(prepareOp, "inventory", err)
	}
	if registeredAt(infos, intendedPath) {
		return &Failure{Op: prepareOp, Stage: "inventory", Kind: KindInvalidState, Detail: "a worktree is already registered at the target path; blocked handling is a later task"}
	}
	return nil
}

// publishAllocating writes the allocating manifest under the short registry
// lock, which serializes first publication across workspaces sharing the root.
func publishAllocating(root, dir string, m Manifest) error {
	release, err := acquireRegistryLock(root)
	if err != nil {
		return &Failure{Op: prepareOp, Stage: "lock", Kind: KindExternal, Detail: "acquiring registry lock", Err: err}
	}
	defer release()
	if err := writeManifest(dir, m); err != nil {
		return &Failure{Op: prepareOp, Stage: "manifest", Kind: KindExternal, Detail: "publishing allocating manifest", Err: err}
	}
	return nil
}

// reinspectAndFinalize re-reads the created worktree's registration, feature
// ref, HEAD, ancestry, and dirty state from live Git, then atomically advances
// the manifest from allocating to ready and returns the verified facts. Any
// disagreement between the just-created state and what Git reports is an
// invalid-state failure: the postcondition was not established.
func (s *Service) reinspectAndFinalize(ctx context.Context, repo gitcli.Repository, dir string, man Manifest, intendedPath string, target Target, baseCommit gitcli.ObjectID) (Workspace, error) {
	infos, err := s.git.ListWorktrees(ctx, repo)
	if err != nil {
		return Workspace{}, mapGitFailure(prepareOp, "verify", err)
	}
	reg, ok := worktreeAt(infos, intendedPath)
	if !ok {
		return Workspace{}, &Failure{Op: prepareOp, Stage: "verify", Kind: KindInvalidState, Detail: "worktree is not registered at the target path after creation"}
	}
	if reg.Branch != target.FeatureRef {
		return Workspace{}, &Failure{Op: prepareOp, Stage: "verify", Kind: KindInvalidState, Detail: "created worktree is not attached to the feature branch"}
	}

	head, err := s.git.ResolveRef(ctx, repo, target.FeatureRef)
	if err != nil {
		return Workspace{}, mapGitFailure(prepareOp, "verify", err)
	}
	reachable, err := s.git.IsAncestor(ctx, repo, baseCommit, head)
	if err != nil {
		return Workspace{}, mapGitFailure(prepareOp, "verify", err)
	}
	if !reachable {
		return Workspace{}, &Failure{Op: prepareOp, Stage: "verify", Kind: KindInvalidState, Detail: "fetched base is not reachable from the created head"}
	}

	changes, err := s.git.ChangedPaths(ctx, intendedPath)
	if err != nil {
		return Workspace{}, mapGitFailure(prepareOp, "verify", err)
	}
	dirty := len(changes) > 0

	man.Phase = PhaseReady
	man.UpdatedUTC = time.Now().UTC().Format(time.RFC3339)
	if err := writeManifest(dir, man); err != nil {
		return Workspace{}, &Failure{Op: prepareOp, Stage: "manifest", Kind: KindExternal, Detail: "advancing manifest to ready", Err: err}
	}

	return Workspace{
		ID:          man.ID,
		Path:        intendedPath,
		FeatureRef:  target.FeatureRef,
		BaseRef:     target.BaseRef,
		BaseCommit:  baseCommit,
		HeadCommit:  head,
		Dirty:       dirty,
		Disposition: PrepareCreated,
	}, nil
}

// pathPresent reports whether anything (file, directory, or symlink) exists at
// path. It uses Lstat so a symlink at the leaf counts as present rather than
// being followed. os.IsNotExist is clean absence; any other error is returned.
func pathPresent(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// registeredAt reports whether any registered worktree's canonical path equals
// want. want is a canonical path (its parent is symlink-canonical and its tail
// does not yet exist), and each registered path is canonicalized through every
// symlink hop before comparison.
func registeredAt(infos []gitcli.WorktreeInfo, want string) bool {
	_, ok := worktreeAt(infos, want)
	return ok
}

// worktreeAt returns the registered worktree whose canonical path equals want.
// A registered path that cannot be canonicalized is skipped rather than matched.
func worktreeAt(infos []gitcli.WorktreeInfo, want string) (gitcli.WorktreeInfo, bool) {
	for _, info := range infos {
		cp, err := canonicalizePath(info.Path)
		if err != nil {
			continue
		}
		if cp == want {
			return info, true
		}
	}
	return gitcli.WorktreeInfo{}, false
}

// canonicalizePath resolves an existing path to its absolute, every-symlink-hop
// form, matching the canonicalization Discover applies to the primary worktree.
func canonicalizePath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}
