package workspace

// This file owns Prepare: the ownership-safe, idempotent creation of one feature
// workspace across its full disposition matrix. It follows the spec §"Prepare
// request and result" order:
//
//	1. validate repository identity and target;
//	2. acquire the per-workspace operation lock (first publication of the
//	   allocating manifest happens under the short registry lock);
//	3. fetch the resolved base branch from the remote and retain the exact commit
//	   — never a cached tracking ref after a failed freshness op (fresh arm only);
//	4. inventory target path, worktree registration, local feature ref, and
//	   manifest phase, each as a three-outcome probe;
//	5. manifest-proven READY workspace: verify Git registers that exact canonical
//	   path on the exact feature ref with HEAD == the ref tip, then return
//	   `existing` with NO checkout/reset/clean/stash/mutation — dirty/untracked is
//	   reported, never repaired;
//	6. manifest-proven interrupted ALLOCATION: resume only the missing suffix. A
//	   branch already created by this manifest must still contain the recorded
//	   base commit (else blocked, untouched); an attached worktree must match the
//	   manifest. Preserve any commits or dirty bytes that appeared after worktree
//	   creation, then advance to ready as `resumed`;
//	7. fresh allocation: require the local feature ref, remote feature ref, target
//	   path, and Git registration ALL absent (a present artifact with no matching
//	   manifest is `blocked`, byte-untouched — pre-Go work is never adopted), then
//	   add ONE branch-attached worktree at the exact fetched base via
//	   AddBranchWorktree (never -B, never reset, never force-remove);
//	8. reinspect registration/ref/HEAD/ancestry, then atomically advance the
//	   manifest to ready and return the verified facts.
//
// The allocating manifest carries the exact fetched base commit, so it is
// published only after the fetch resolves it — the manifest schema forbids an
// empty base commit, and a crash breadcrumb without the base is useless to a
// resume anyway. The resume arm therefore trusts the manifest's recorded base
// and never re-fetches. A manifest present but foreign or corrupt, or a probe
// that cannot see a resource, never licenses a create: foreign content is a
// blocked disposition; an unreadable probe is an external failure. Blocked and
// contended are dispositions returned as values (byte-untouched); `failed` is an
// error return.

import (
	"context"
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

// Prepare is safe to call repeatedly across the full disposition matrix. Its
// arm is selected by the owned manifest's phase (existing/resume), by a foreign
// or corrupt manifest (blocked), or by a clean absence (fresh allocation).
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

	// Steps 4-6: classify the manifest slot and dispatch the owned arms. A slot
	// occupied by a manifest we did not write — undecodable, invalid, or owned by
	// a different repository/target — is blocked and left byte-untouched; a slot
	// that cannot be read at all is an external failure, never clean absence.
	m, status, cerr := classifyManifest(dir)
	switch status {
	case manifestUnknown:
		return Workspace{}, &Failure{Op: prepareOp, Stage: "inventory", Kind: KindExternal, Detail: "workspace manifest is unreadable", Err: cerr}
	case manifestForeign:
		return blockedWorkspace(target, intendedPath), nil
	case manifestValid:
		if !ownsManifest(m, req.Repository, target, intendedPath) {
			return blockedWorkspace(target, intendedPath), nil
		}
		return s.prepareOwned(ctx, req.Repository, dir, m, intendedPath, target)
	}
	// status == manifestAbsent: this is a first attempt — fresh allocation.

	// Step 3: fetch the resolved base branch and retain the exact commit.
	rev, err := s.git.FetchBranch(ctx, req.Repository, req.Remote, target.BaseRef)
	if err != nil {
		return Workspace{}, mapGitFailure(prepareOp, "fetch", err)
	}
	baseCommit := rev.Commit

	// Steps 4 & 7: inventory the four fresh-allocation preconditions. A present
	// artifact with no matching manifest is blocked (byte-untouched, pre-Go work
	// never adopted); an unreadable probe is an external failure. Nothing is
	// published until this all-absent inventory passes, so a blocked or failed
	// return here leaves the repository untouched.
	blocked, err := s.inventoryForFresh(ctx, req.Repository, req.Remote, target, intendedPath)
	if err != nil {
		return Workspace{}, err
	}
	if blocked != nil {
		return *blocked, nil
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
	return s.reinspectAndFinalize(ctx, req.Repository, dir, man, intendedPath, target, baseCommit, PrepareCreated)
}

// prepareOwned dispatches a valid, owned manifest by its recorded phase: a ready
// manifest to the existing arm, an interrupted allocation to the resume arm. A
// cleaned tombstone is not re-prepared here — recovery and re-preparation after
// cleanup belong to the finalize change (0316), so it is a typed invalid-state
// failure rather than a silent re-allocation.
func (s *Service) prepareOwned(ctx context.Context, repo gitcli.Repository, dir string, m Manifest, intendedPath string, target Target) (Workspace, error) {
	switch m.Phase {
	case PhaseReady:
		return s.prepareExisting(ctx, repo, m, intendedPath, target)
	case PhaseAllocating:
		return s.prepareResume(ctx, repo, dir, m, intendedPath, target)
	case PhaseCleaned:
		return Workspace{}, &Failure{Op: prepareOp, Stage: "inventory", Kind: KindInvalidState, Detail: "workspace manifest is a cleaned tombstone; re-preparation is out of scope"}
	default:
		return blockedWorkspace(target, intendedPath), nil
	}
}

// prepareExisting is the manifest-proven READY arm (spec step 5). It verifies —
// read-only — that Git registers the exact canonical path on the exact feature
// ref and that the ref tip is the registered worktree's HEAD, then returns
// `existing`. It performs no checkout, reset, clean, stash, fetch, or manifest
// rewrite: dirty and untracked state is reported in Dirty, never repaired. A
// registration that disagrees with the ready manifest is blocked, byte-untouched
// (recovery is Inspect's classification plus a higher-level decision).
func (s *Service) prepareExisting(ctx context.Context, repo gitcli.Repository, m Manifest, intendedPath string, target Target) (Workspace, error) {
	infos, err := s.git.ListWorktrees(ctx, repo)
	if err != nil {
		return Workspace{}, mapGitFailure(prepareOp, "inventory", err)
	}
	reg, registered := worktreeAt(infos, intendedPath)
	if !registered || reg.Detached || reg.Branch != target.FeatureRef {
		return blockedWorkspace(target, intendedPath), nil
	}
	head, err := s.git.ResolveRef(ctx, repo, target.FeatureRef)
	if err != nil {
		return Workspace{}, mapGitFailure(prepareOp, "verify", err)
	}
	if head != reg.Head {
		return blockedWorkspace(target, intendedPath), nil
	}
	changes, err := s.git.ChangedPaths(ctx, intendedPath)
	if err != nil {
		return Workspace{}, mapGitFailure(prepareOp, "verify", err)
	}
	return Workspace{
		ID:          m.ID,
		Path:        intendedPath,
		FeatureRef:  target.FeatureRef,
		BaseRef:     target.BaseRef,
		BaseCommit:  m.BaseCommit,
		HeadCommit:  head,
		Dirty:       len(changes) > 0,
		Disposition: PrepareExisting,
	}, nil
}

// prepareResume is the manifest-proven interrupted-ALLOCATION arm (spec step 6).
// It trusts the manifest's recorded base commit (never re-fetches — the manifest
// was published after the fetch resolved it) and resumes only the missing suffix:
//   - branch + registered worktree already present → verify and advance to ready;
//   - branch present (still containing the recorded base) but no worktree →
//     attach the existing branch with AttachBranchWorktree, never moving its tip;
//   - neither present → create both at the exact recorded base with
//     AddBranchWorktree.
//
// A branch created by this manifest that no longer contains the recorded base
// commit was rewritten out of band: it is blocked and left byte-untouched, never
// reset. A path present without a matching registration is likewise a collision
// that is never force-removed.
func (s *Service) prepareResume(ctx context.Context, repo gitcli.Repository, dir string, m Manifest, intendedPath string, target Target) (Workspace, error) {
	base := m.BaseCommit

	// Probe the local feature branch: absent (ref-unavailable) is a resumable
	// suffix; any other probe error is external and never read as absence.
	branchTip, branchErr := s.git.ResolveRef(ctx, repo, target.FeatureRef)
	branchExists := false
	if branchErr == nil {
		branchExists = true
	} else if f, ok := gitcli.AsFailure(branchErr); !ok || f.Kind != gitcli.KindRefUnavailable {
		return Workspace{}, mapGitFailure(prepareOp, "inventory", branchErr)
	}
	if branchExists {
		reachable, err := s.git.IsAncestor(ctx, repo, base, branchTip)
		if err != nil {
			return Workspace{}, mapGitFailure(prepareOp, "inventory", err)
		}
		if !reachable {
			return blockedWorkspace(target, intendedPath), nil
		}
	}

	infos, err := s.git.ListWorktrees(ctx, repo)
	if err != nil {
		return Workspace{}, mapGitFailure(prepareOp, "inventory", err)
	}
	reg, registered := worktreeAt(infos, intendedPath)

	switch {
	case registered:
		// The worktree exists: it must be attached to the feature ref, or the
		// state disagrees and is blocked. Nothing else is created — the arm falls
		// through to reinspect-and-advance, preserving post-creation commits and
		// dirty bytes.
		if reg.Detached || reg.Branch != target.FeatureRef {
			return blockedWorkspace(target, intendedPath), nil
		}
	case branchExists:
		// Branch present at/after the recorded base, no worktree: attach it. The
		// attach never moves the branch tip, even if the remote advanced meanwhile.
		if err := s.git.AttachBranchWorktree(ctx, repo, intendedPath, target.FeatureRef); err != nil {
			return Workspace{}, mapGitFailure(prepareOp, "worktree", err)
		}
	default:
		// Neither branch nor worktree: create both at the exact recorded base. A
		// path already present without a registration is a collision we never
		// force-remove — blocked, byte-untouched.
		if present, perr := pathPresent(intendedPath); perr != nil {
			return Workspace{}, &Failure{Op: prepareOp, Stage: "inventory", Kind: KindExternal, Detail: "stat of target path failed", Err: perr}
		} else if present {
			return blockedWorkspace(target, intendedPath), nil
		}
		if err := s.git.AddBranchWorktree(ctx, repo, intendedPath, target.FeatureRef, base); err != nil {
			return Workspace{}, mapGitFailure(prepareOp, "worktree", err)
		}
	}

	return s.reinspectAndFinalize(ctx, repo, dir, m, intendedPath, target, base, PrepareResumed)
}

// blockedWorkspace is the byte-untouched blocked disposition: a value, not an
// error. It carries the intended path and target refs so a caller can name the
// collision; the workspace was neither created nor mutated.
func blockedWorkspace(target Target, intendedPath string) Workspace {
	return Workspace{
		Path:        intendedPath,
		FeatureRef:  target.FeatureRef,
		BaseRef:     target.BaseRef,
		Disposition: PrepareBlocked,
	}
}

// ownsManifest reports whether a valid manifest is this repository's and this
// target's: its ownership identity (CommonDir), its derived id and feature ref,
// its recorded canonical path, and its change/slug must all match. A valid
// manifest that fails any check occupies the slot but belongs to something else
// and is blocked, never adopted.
func ownsManifest(m Manifest, repo gitcli.Repository, target Target, intendedPath string) bool {
	return m.CommonDir == repo.CommonDir &&
		m.ID == workspaceID(target.FeatureRef) &&
		m.FeatureRef == target.FeatureRef &&
		m.BaseRef == target.BaseRef &&
		m.ChangeID == target.ChangeID &&
		m.Slug == target.Slug &&
		m.Path == intendedPath
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

// inventoryForFresh probes all four fresh-allocation preconditions: the local
// feature ref, the remote feature ref, the target path, and the Git worktree
// registration. Each probe is three-outcome. It returns:
//   - (nil, nil)   every precondition cleanly absent — proceed with allocation;
//   - (ws, nil)    a present artifact with no matching manifest — blocked,
//     byte-untouched (pre-Go in-flight work is never adopted from feat/<slug>);
//   - (nil, err)   a probe could not see a resource — external/invalid-output
//     failure, NEVER read as clean absence, so it never licenses a create.
func (s *Service) inventoryForFresh(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, target Target, intendedPath string) (*Workspace, error) {
	// Local feature ref: a ResolveRef that resolves means the branch exists; a
	// ref-unavailable failure means it is cleanly absent; any other error is real.
	if _, err := s.git.ResolveRef(ctx, repo, target.FeatureRef); err == nil {
		w := blockedWorkspace(target, intendedPath)
		return &w, nil
	} else if f, ok := gitcli.AsFailure(err); !ok || f.Kind != gitcli.KindRefUnavailable {
		return nil, mapGitFailure(prepareOp, "inventory", err)
	}

	// Remote feature ref: ProbeRemoteBranch distinguishes found/absent/error.
	rr, err := s.git.ProbeRemoteBranch(ctx, repo, remote, target.FeatureRef)
	if err != nil {
		return nil, mapGitFailure(prepareOp, "inventory", err)
	}
	if rr.State != gitcli.RemoteRefAbsent {
		w := blockedWorkspace(target, intendedPath)
		return &w, nil
	}

	// Target path: an existing path (of any kind) with no manifest is blocked.
	// os.Lstat does not follow a final symlink, so a symlink placed at the path
	// still counts as present.
	if present, err := pathPresent(intendedPath); err != nil {
		return nil, &Failure{Op: prepareOp, Stage: "inventory", Kind: KindExternal, Detail: "stat of target path failed", Err: err}
	} else if present {
		w := blockedWorkspace(target, intendedPath)
		return &w, nil
	}

	// Git worktree registration at the path.
	infos, err := s.git.ListWorktrees(ctx, repo)
	if err != nil {
		return nil, mapGitFailure(prepareOp, "inventory", err)
	}
	if registeredAt(infos, intendedPath) {
		w := blockedWorkspace(target, intendedPath)
		return &w, nil
	}
	return nil, nil
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

// reinspectAndFinalize re-reads the created or resumed worktree's registration,
// feature ref, HEAD, ancestry, and dirty state from live Git, then atomically
// advances the manifest from allocating to ready and returns the verified facts
// under disposition (created for a fresh allocation, resumed for a completed
// interrupted allocation). Any disagreement between the resulting state and what
// Git reports is an invalid-state failure: the postcondition was not established.
// The reinspected HEAD is read back from the ref, so post-creation commits on a
// resumed workspace surface as the reported head, and dirty bytes are reported,
// never repaired.
func (s *Service) reinspectAndFinalize(ctx context.Context, repo gitcli.Repository, dir string, man Manifest, intendedPath string, target Target, baseCommit gitcli.ObjectID, disposition PrepareDisposition) (Workspace, error) {
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
		Disposition: disposition,
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
