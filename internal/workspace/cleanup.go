package workspace

// This file owns Cleanup: the proof-gated, non-forcing removal of exactly one
// feature workspace's checkout. Cleanup removes ONLY the checkout — never a local
// or remote branch, never an administrative directory by pathname, never a
// transaction or sibling worktree, and never via a global `git worktree prune` or
// an inventory sweep. It is the destructive counterpart of Prepare, and it earns
// the right to remove through the same proof Inspect classifies:
//
//	1. validate the target and repository paths, then take the per-workspace
//	   operation lock (serializing against Prepare/Inspect-refresh/Publish);
//	2. classify the manifest slot: an unreadable slot is a `failed` error (never a
//	   false clean); an absent, foreign, or unowned manifest is `blocked`,
//	   byte-untouched; a cleaned tombstone with no live registration is
//	   `already-clean`; anything else is proven below;
//	3. a ready manifest: prove the feature ref exists, that Git registers exactly
//	   the recorded canonical path on that ref (not detached, HEAD == ref tip),
//	   that the recorded base is still reachable from the head, and that the
//	   tracked/untracked delta is empty — any failure is `blocked`, byte-untouched;
//	4. remove via the NON-FORCING gitcli.RemoveWorktreeClean so Git itself rechecks
//	   cleanliness at the destructive boundary. A preflight status check followed by
//	   a forced removal would leave a race in which a worker writes between the two
//	   calls and loses data; the non-forcing primitive closes it. Git's refusal at
//	   the boundary is `blocked`, byte-untouched;
//	5. after Git confirms removal, advance the manifest atomically from ready to the
//	   cleaned tombstone (the monotonic chain allows ready->cleaned only) and return
//	   `cleaned`.
//
// A probe that cannot see a resource NEVER reads as clean absence (learnings:
// probe-error-is-not-clean-absence; the 0309 review's important finding): every
// registration or Git probe error is a `failed` error return, so an errored delta
// probe can never authorize a removal or a false already-clean.

import (
	"context"
	"path/filepath"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

// cleanupOp is the Op recorded on every Cleanup Failure.
const cleanupOp = "cleanup"

// CleanupRequest names the repository and the fully validated target to clean.
// Cleanup takes no remote: it never fetches and never touches any remote ref.
type CleanupRequest struct {
	Repository gitcli.Repository
	Target     Target
}

// CleanupResult is the value outcome of a Cleanup. BlockedBy carries a bounded,
// redacted set of reasons/paths when the disposition is blocked (empty
// otherwise); it is diagnostic data, never an oracle.
type CleanupResult struct {
	Disposition CleanupDisposition
	Path        string
	BlockedBy   []string
}

// Cleanup removes only the checkout of one owned, ready, clean workspace and
// advances its manifest to a cleaned tombstone. Blocked and already-clean are
// value dispositions returned with a nil error and no mutation; a probe or
// removal-process failure is a `failed` error return with the workspace intact.
func (s *Service) Cleanup(ctx context.Context, req CleanupRequest) (CleanupResult, error) {
	if err := validateCleanupTarget(req.Target); err != nil {
		return CleanupResult{Disposition: CleanupFailed}, err
	}
	repo := req.Repository
	if repo.CommonDir == "" || !filepath.IsAbs(repo.CommonDir) || repo.PrimaryWorktree == "" || !filepath.IsAbs(repo.PrimaryWorktree) {
		return CleanupResult{Disposition: CleanupFailed}, &Failure{Op: cleanupOp, Stage: "validate", Kind: KindInvalidInput, Detail: "repository paths are not absolute"}
	}
	target := req.Target
	dir := workspaceDir(repo.CommonDir, target.FeatureRef)
	intendedPath := filepath.Join(repo.PrimaryWorktree, ".worktrees", target.Slug)

	// Serialize the whole operation against concurrent Prepare/Inspect/Publish on
	// this one workspace.
	releaseOp, err := acquireOperationLock(dir)
	if err != nil {
		return CleanupResult{Disposition: CleanupFailed}, &Failure{Op: cleanupOp, Stage: "lock", Kind: KindExternal, Detail: "acquiring workspace operation lock", Err: err}
	}
	defer releaseOp()

	// Classify the manifest slot. An unreadable slot is a failure (never a false
	// clean); an absent/foreign/unowned manifest is blocked, byte-untouched.
	m, status, cerr := classifyManifest(dir)
	switch status {
	case manifestUnknown:
		return CleanupResult{Disposition: CleanupFailed, Path: intendedPath}, &Failure{Op: cleanupOp, Stage: "inventory", Kind: KindExternal, Detail: "workspace manifest is unreadable", Err: cerr}
	case manifestAbsent:
		return blockedCleanup(intendedPath, "no workspace manifest"), nil
	case manifestForeign:
		return blockedCleanup(intendedPath, "manifest is foreign or malformed"), nil
	}
	if !ownsManifest(m, repo, target, intendedPath) {
		return blockedCleanup(m.Path, "manifest identity does not match this repository or target"), nil
	}

	switch m.Phase {
	case PhaseCleaned:
		return s.cleanupTombstone(ctx, repo, m)
	case PhaseReady:
		return s.cleanupReady(ctx, repo, dir, m, target)
	default:
		// An allocating partial (or any non-terminal phase) is unproven: the
		// monotonic phase chain forbids advancing it to cleaned, so it is blocked
		// and left byte-untouched.
		return blockedCleanup(m.Path, "workspace is not in a ready phase"), nil
	}
}

// cleanupTombstone handles a cleaned manifest: a retry after a prior successful
// removal. It confirms — via a live registration probe that never reads an error
// as absence — that no worktree is registered at the recorded path, and returns
// already-clean. A tombstone that is still registered disagrees with its recorded
// phase and is blocked.
func (s *Service) cleanupTombstone(ctx context.Context, repo gitcli.Repository, m Manifest) (CleanupResult, error) {
	infos, err := s.git.ListWorktrees(ctx, repo)
	if err != nil {
		return CleanupResult{Disposition: CleanupFailed, Path: m.Path}, mapGitFailure(cleanupOp, "inventory", err)
	}
	if registeredAt(infos, m.Path) {
		return blockedCleanup(m.Path, "cleaned tombstone is still registered"), nil
	}
	return CleanupResult{Disposition: CleanupAlreadyClean, Path: m.Path}, nil
}

// cleanupReady proves a ready workspace is exactly the recorded owned checkout,
// clean and on the recorded feature ref with the recorded base still reachable,
// then removes it non-forcingly and advances the manifest to the cleaned
// tombstone. Every unproven condition is blocked (byte-untouched); every probe
// error is a failed error return with the workspace intact.
func (s *Service) cleanupReady(ctx context.Context, repo gitcli.Repository, dir string, m Manifest, target Target) (CleanupResult, error) {
	// The feature ref must still exist; a cleanly-absent ref is a mismatch
	// (blocked), any other probe error is a failure.
	branchHead, branchErr := s.git.ResolveRef(ctx, repo, target.FeatureRef)
	if branchErr != nil {
		if f, ok := gitcli.AsFailure(branchErr); ok && f.Kind == gitcli.KindRefUnavailable {
			return blockedCleanup(m.Path, "feature ref is missing"), nil
		}
		return CleanupResult{Disposition: CleanupFailed, Path: m.Path}, mapGitFailure(cleanupOp, "inventory", branchErr)
	}

	// Live registration proof: exactly the recorded path, attached to the feature
	// ref (not detached), with HEAD == the ref tip.
	infos, err := s.git.ListWorktrees(ctx, repo)
	if err != nil {
		return CleanupResult{Disposition: CleanupFailed, Path: m.Path}, mapGitFailure(cleanupOp, "inventory", err)
	}
	reg, registered := worktreeAt(infos, m.Path)
	if !registered {
		return blockedCleanup(m.Path, "recorded path is not registered"), nil
	}
	if reg.Detached || reg.Branch != target.FeatureRef {
		return blockedCleanup(m.Path, "registration is detached or on a different branch"), nil
	}
	if reg.Head != branchHead {
		return blockedCleanup(m.Path, "registered HEAD is not the feature ref tip"), nil
	}

	// The recorded base must still be reachable from the head, or the branch was
	// moved out of band: blocked, never reset.
	reachable, err := s.git.IsAncestor(ctx, repo, m.BaseCommit, reg.Head)
	if err != nil {
		return CleanupResult{Disposition: CleanupFailed, Path: m.Path}, mapGitFailure(cleanupOp, "inventory", err)
	}
	if !reachable {
		return blockedCleanup(m.Path, "recorded base is not reachable from the head"), nil
	}

	// Exact tracked/untracked delta: any dirty, staged, untracked, or conflicted
	// path blocks removal. A probe error is a failure, never a false clean.
	dirty, err := s.dirtyPaths(ctx, m.Path)
	if err != nil {
		return CleanupResult{Disposition: CleanupFailed, Path: m.Path}, remapInspectStage(err)
	}
	if len(dirty) > 0 {
		return CleanupResult{Disposition: CleanupBlocked, Path: m.Path, BlockedBy: boundedReasons(dirty)}, nil
	}

	// Remove non-forcingly: Git rechecks cleanliness at the destructive boundary,
	// closing the check-then-remove race a forced removal would leave. Git's
	// refusal there (a command failure) means the workspace turned dirty and is
	// left byte-untouched — blocked, not removed. Any other removal error (the
	// process could not run, was cancelled, or timed out) is a failure.
	if err := s.git.RemoveWorktreeClean(ctx, repo, m.Path); err != nil {
		if f, ok := gitcli.AsFailure(err); ok && f.Kind == gitcli.KindCommandFailed {
			return blockedCleanup(m.Path, "git refused the non-forcing removal (workspace not clean)"), nil
		}
		return CleanupResult{Disposition: CleanupFailed, Path: m.Path}, mapGitFailure(cleanupOp, "remove", err)
	}

	// Advance the manifest atomically to the cleaned tombstone. The monotonic phase
	// chain permits ready->cleaned only; refuse defensively if it does not hold.
	if !m.Phase.canAdvanceTo(PhaseCleaned) {
		return CleanupResult{Disposition: CleanupFailed, Path: m.Path}, &Failure{Op: cleanupOp, Stage: "manifest", Kind: KindInvalidState, Detail: "manifest phase may not advance to cleaned"}
	}
	m.Phase = PhaseCleaned
	m.UpdatedUTC = time.Now().UTC().Format(time.RFC3339)
	if err := writeManifest(dir, m); err != nil {
		return CleanupResult{Disposition: CleanupFailed, Path: m.Path}, &Failure{Op: cleanupOp, Stage: "manifest", Kind: KindExternal, Detail: "advancing manifest to cleaned", Err: err}
	}
	return CleanupResult{Disposition: CleanupCleaned, Path: m.Path}, nil
}

// blockedCleanup is the byte-untouched blocked disposition: a value, not an error.
// It carries the recorded/intended path and a single bounded reason.
func blockedCleanup(path, reason string) CleanupResult {
	return CleanupResult{Disposition: CleanupBlocked, Path: path, BlockedBy: []string{reason}}
}

// boundedReasons caps the reason/path list so a blocked result stays bounded
// (never an unbounded dirty-path dump): at most the first eight entries, each
// length-capped, with an overflow marker appended when truncated.
func boundedReasons(paths []string) []string {
	const maxEntries = 8
	out := make([]string, 0, maxEntries+1)
	for i, p := range paths {
		if i >= maxEntries {
			out = append(out, "…")
			break
		}
		out = append(out, boundedDetail(p))
	}
	return out
}

// remapInspectStage re-tags a Failure produced by the shared dirtyPaths helper
// (which records Op "inspect") onto the cleanup operation, so a delta-probe error
// surfaced during cleanup carries the cleanup Op. A non-Failure error is wrapped
// as an external cleanup failure.
func remapInspectStage(err error) *Failure {
	if f, ok := AsFailure(err); ok {
		return &Failure{Op: cleanupOp, Stage: "inventory", Kind: f.Kind, Detail: f.Detail, Err: f.Err}
	}
	return &Failure{Op: cleanupOp, Stage: "inventory", Kind: KindExternal, Detail: "reading workspace delta", Err: err}
}

// validateCleanupTarget rejects a target that is not self-consistent with its
// slug and base, reusing NewTarget's rules and re-tagging any rejection to the
// cleanup operation.
func validateCleanupTarget(t Target) error {
	derived, err := NewTarget(t.ChangeID, t.Slug, t.Base)
	if err != nil {
		return &Failure{Op: cleanupOp, Stage: "validate", Kind: KindInvalidInput, Detail: "target is not self-consistent with its slug and base"}
	}
	if derived != t {
		return &Failure{Op: cleanupOp, Stage: "validate", Kind: KindInvalidInput, Detail: "target fields are not self-consistent with its slug and base"}
	}
	return nil
}
