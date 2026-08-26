package gitcli

import (
	"context"
	"path/filepath"
)

// discoverWorktreeOp labels every Failure raised while resolving worktree
// identity.
const discoverWorktreeOp Operation = "discover-worktree"

// WorktreeIdentity is the canonical identity of the single working tree that
// CONTAINS an invocation path: its toplevel and its own git dir, both absolute
// and every-symlink-hop-resolved. Unlike Repository (from Discover, which always
// resolves to the primary worktree), a linked worktree resolves to ITSELF —
// per-worktree ownership isolation is the point for installation commands.
type WorktreeIdentity struct {
	Root   string // absolute, symlink-canonical toplevel of the containing worktree
	GitDir string // absolute, symlink-canonical git dir of THAT worktree
}

// DiscoverWorktree resolves the working tree containing the invocation path. It
// performs no writes. The invocation path is canonicalized (Abs + every symlink
// hop), then `git rev-parse --show-toplevel --absolute-git-dir` reads the
// containing worktree's toplevel and its own git dir in one process; both are
// canonicalized with filepath.EvalSymlinks (matching Discover's discipline). A
// bare repository (no working tree) and a non-repository both make rev-parse
// exit non-zero and are reported as KindInvalidRepository.
func (c *Client) DiscoverWorktree(ctx context.Context, opts DiscoverOptions) (WorktreeIdentity, error) {
	if opts.InvocationPath == "" {
		return WorktreeIdentity{}, newFailure(discoverWorktreeOp, KindInvalidRequest, "invocation path is empty", nil)
	}
	abs, err := filepath.Abs(opts.InvocationPath)
	if err != nil {
		return WorktreeIdentity{}, newFailure(discoverWorktreeOp, KindInvalidRepository, "invocation path not resolvable", err)
	}
	canonInvocation, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return WorktreeIdentity{}, newFailure(discoverWorktreeOp, KindInvalidRepository, "invocation path does not exist", err)
	}

	// Toplevel + git dir of the CONTAINING worktree in one process. A non-zero
	// exit means either "not a git repository" or a bare repository (whose
	// --show-toplevel fatally reports "must be run in a work tree").
	res, f := c.run(ctx, runRequest{
		op:   discoverWorktreeOp,
		dir:  canonInvocation,
		args: []string{"rev-parse", "--show-toplevel", "--absolute-git-dir"},
	})
	if f != nil {
		return WorktreeIdentity{}, f
	}
	if res.exitCode != 0 {
		return WorktreeIdentity{}, newFailure(discoverWorktreeOp, KindInvalidRepository, "not a git repository or has no working tree", nil).withExitCode(res.exitCode)
	}
	lines := stdoutLines(res.stdout)
	if len(lines) != 2 {
		return WorktreeIdentity{}, newFailure(discoverWorktreeOp, KindInvalidOutput, "unexpected rev-parse worktree output", nil)
	}
	canonRoot, err := filepath.EvalSymlinks(lines[0])
	if err != nil {
		return WorktreeIdentity{}, newFailure(discoverWorktreeOp, KindInvalidRepository, "worktree root not resolvable", err)
	}
	canonGitDir, err := filepath.EvalSymlinks(lines[1])
	if err != nil {
		return WorktreeIdentity{}, newFailure(discoverWorktreeOp, KindInvalidRepository, "worktree git dir not resolvable", err)
	}
	return WorktreeIdentity{Root: canonRoot, GitDir: canonGitDir}, nil
}
