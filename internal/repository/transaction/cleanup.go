package transaction

// This file owns per-candidate cleanup: tearing down exactly the candidate a run
// allocated, and nothing else. It never broadens to a global sweep and never runs
// `git worktree prune`. The recovery half — PruneAbandoned, with the full
// six-point ownership proof — lands in a later task and will share this file.
//
// Cleanup order matters. The registered detached worktree must be deregistered
// through Git (worktree remove) before its directory is deleted, or a bare
// RemoveAll would orphan the worktree registration in the common dir. The live
// lock is released before the candidate directory is removed. After a successful
// push the applied result is never relabelled failed: a cleanup that cannot
// finish only adds a "cleanup-pending: <id>" warning naming the state a later
// PruneAbandoned can reclaim.

import (
	"context"
	"os"
	"path/filepath"

	"github.com/danielhanold/docket/internal/gitcli"
)

// cleanupCandidate tears down exactly the candidate c: it deregisters c's detached
// worktree (only when Git still has it registered), releases the live lock, and
// removes the candidate directory through a root anchored at the transactions
// root. It returns any cleanup-pending warnings — never an error and never a
// relabel — so a caller keeps the disposition it already computed.
func (e *Engine) cleanupCandidate(ctx context.Context, repo gitcli.Repository, c *candidate) []string {
	var warnings []string
	canRemoveDir := true

	// Deregister the worktree through Git only if it is still registered. A worktree
	// that never got added (an allocate-then-fail path) must not be reported as a
	// failed removal, and its directory can be deleted directly.
	if e.worktreeRegistered(ctx, repo, c.worktree) {
		if err := e.client.RemoveWorktree(ctx, repo, c.worktree); err != nil {
			warnings = appendCleanupPending(warnings, c.id)
			// Leave the directory in place: deleting it now would orphan Git's
			// still-live worktree registration, which we may never prune globally.
			canRemoveDir = false
		}
	}

	if c.live != nil {
		_ = c.live.release()
	}

	if canRemoveDir {
		if err := e.removeCandidateRoot(repo, c); err != nil {
			warnings = appendCleanupPending(warnings, c.id)
		}
	}
	return warnings
}

// worktreeRegistered reports whether Git currently registers a worktree at
// worktreePath, comparing canonical (Abs + every-symlink-hop) paths so a
// /tmp -> /private/tmp indirection never hides a match. A list failure is treated
// as "not registered" — cleanup then falls back to a direct directory removal
// rather than attempting a remove that would fail anyway.
func (e *Engine) worktreeRegistered(ctx context.Context, repo gitcli.Repository, worktreePath string) bool {
	infos, err := e.client.ListWorktrees(ctx, repo)
	if err != nil {
		return false
	}
	target := canonicalPath(worktreePath)
	for _, info := range infos {
		if canonicalPath(info.Path) == target {
			return true
		}
	}
	return false
}

// removeCandidateRoot deletes the candidate's directory subtree through an os.Root
// anchored at the transactions root, so the removal can never escape that owned
// tree even if the candidate id were somehow adversarial.
func (e *Engine) removeCandidateRoot(repo gitcli.Repository, c *candidate) error {
	root, err := os.OpenRoot(transactionsRoot(repo))
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return root.RemoveAll(c.id)
}

// canonicalPath resolves p to an absolute, every-symlink-hop-canonicalized path
// for identity comparison. When the target no longer exists (already removed) it
// falls back to the absolute form, which is still a stable comparison key.
func canonicalPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

// appendCleanupPending appends a "cleanup-pending: <id>" warning unless one for id
// is already present, so a two-stage cleanup failure names the id once.
func appendCleanupPending(warnings []string, id string) []string {
	msg := "cleanup-pending: " + id
	for _, w := range warnings {
		if w == msg {
			return warnings
		}
	}
	return append(warnings, msg)
}
