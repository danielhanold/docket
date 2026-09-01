package gitcli

import (
	"context"
	"path/filepath"
)

const fastForwardWorktreeOp Operation = "worktree-fast-forward"

// FastForwardWorktree advances the branch checked out at worktree to target
// only when the checkout is clean and Git can perform a fast-forward. It returns
// false, nil when HEAD already equals target.
func (c *Client) FastForwardWorktree(ctx context.Context, worktree string, target ObjectID) (bool, error) {
	if !filepath.IsAbs(worktree) {
		return false, newFailure(fastForwardWorktreeOp, KindInvalidRequest, "worktree path must be absolute", nil)
	}
	if err := validateObjectID(target); err != nil {
		return false, newFailure(fastForwardWorktreeOp, KindInvalidRequest, "invalid target id", err)
	}

	status, f := c.run(ctx, runRequest{
		op:   fastForwardWorktreeOp,
		dir:  worktree,
		args: []string{"status", "--porcelain=v1", "-z", "--untracked-files=all"},
	})
	if f != nil {
		return false, f
	}
	if status.exitCode != 0 {
		return false, newFailure(fastForwardWorktreeOp, KindCommandFailed,
			"worktree status failed: "+stderrExcerpt(status.stderr), nil).withExitCode(status.exitCode)
	}
	if len(status.stdout) != 0 {
		return false, newFailure(fastForwardWorktreeOp, KindInvalidRepository, "worktree has uncommitted changes", nil)
	}
	head, f := c.worktreeHead(ctx, fastForwardWorktreeOp, worktree)
	if f != nil {
		return false, f
	}
	if head == target {
		return false, nil
	}

	merged, f := c.run(ctx, runRequest{
		op:   fastForwardWorktreeOp,
		dir:  worktree,
		args: []string{"merge", "--ff-only", string(target)},
	})
	if f != nil {
		return false, f
	}
	if merged.exitCode != 0 {
		return false, newFailure(fastForwardWorktreeOp, KindCommandFailed,
			"merge --ff-only failed: "+stderrExcerpt(merged.stderr), nil).withExitCode(merged.exitCode)
	}
	return true, nil
}
