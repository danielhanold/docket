package gitcli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
)

// discoverOp labels every Failure raised while resolving repository identity.
const discoverOp Operation = "discover"

// DiscoverOptions selects the checkout to resolve identity from. InvocationPath
// may be the primary worktree, any nested directory beneath it, or any linked
// worktree — discovery canonicalizes all of them to the same repository.
type DiscoverOptions struct{ InvocationPath string }

// Repository is the canonical identity of a Git repository: the absolute,
// every-symlink-hop-resolved primary worktree and its shared common directory.
type Repository struct {
	PrimaryWorktree string // absolute, symlink-canonical
	CommonDir       string // absolute, symlink-canonical
}

// Discover resolves the canonical repository identity from any invocation path
// inside it (ADR-0034: a linked worktree's toplevel is never treated as the
// root). It performs no writes of any kind. The invocation path is canonicalized
// (Abs + every symlink hop), the repository is confirmed non-bare, the primary
// worktree is read from `git worktree list` (main-first), and the invocation's
// common dir is cross-checked against the primary worktree's own common dir so a
// broken or unregistered worktree is rejected rather than silently accepted.
func (c *Client) Discover(ctx context.Context, opts DiscoverOptions) (Repository, error) {
	if opts.InvocationPath == "" {
		return Repository{}, newFailure(discoverOp, KindInvalidRequest, "invocation path is empty", nil)
	}
	abs, err := filepath.Abs(opts.InvocationPath)
	if err != nil {
		return Repository{}, newFailure(discoverOp, KindInvalidRepository, "invocation path not resolvable", err)
	}
	canonInvocation, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return Repository{}, newFailure(discoverOp, KindInvalidRepository, "invocation path does not exist", err)
	}

	// Identity + bare check in one process. A non-zero exit means "not a git
	// repository"; a bare repository has no worktree to preserve.
	res, f := c.run(ctx, runRequest{
		op:   discoverOp,
		dir:  canonInvocation,
		args: []string{"rev-parse", "--is-bare-repository", "--git-common-dir"},
	})
	if f != nil {
		return Repository{}, f
	}
	if res.exitCode != 0 {
		return Repository{}, newFailure(discoverOp, KindInvalidRepository, "not a git repository", nil)
	}
	lines := stdoutLines(res.stdout)
	if len(lines) != 2 {
		return Repository{}, newFailure(discoverOp, KindInvalidOutput, "unexpected rev-parse identity output", nil)
	}
	switch lines[0] {
	case "true":
		return Repository{}, newFailure(discoverOp, KindInvalidRepository, "bare repository has no worktree", nil)
	case "false":
		// non-bare: proceed
	default:
		return Repository{}, newFailure(discoverOp, KindInvalidOutput, "unexpected is-bare-repository value", nil)
	}
	canonCommonDir, err := resolveGitPath(lines[1], canonInvocation)
	if err != nil {
		return Repository{}, newFailure(discoverOp, KindInvalidRepository, "common dir not resolvable", err)
	}

	// Primary worktree: the FIRST stanza of `worktree list` is documented as the
	// main worktree. Run with -C at the invocation path so it reflects this repo.
	wl, f := c.run(ctx, runRequest{
		op:   discoverOp,
		dir:  canonInvocation,
		args: []string{"worktree", "list", "--porcelain", "-z"},
	})
	if f != nil {
		return Repository{}, f
	}
	if wl.exitCode != 0 {
		return Repository{}, newFailure(discoverOp, KindInvalidRepository, "worktree list failed", nil)
	}
	mainRaw, ok := firstWorktreePath(wl.stdout)
	if !ok {
		return Repository{}, newFailure(discoverOp, KindInvalidOutput, "missing primary worktree entry", nil)
	}
	canonMain, err := filepath.EvalSymlinks(mainRaw)
	if err != nil {
		return Repository{}, newFailure(discoverOp, KindInvalidRepository, "primary worktree not resolvable", err)
	}

	// Consistency: the primary worktree must share the invocation's common dir.
	// A mismatch means the invocation is an unregistered or broken worktree.
	mres, f := c.run(ctx, runRequest{
		op:   discoverOp,
		dir:  canonMain,
		args: []string{"rev-parse", "--git-common-dir"},
	})
	if f != nil {
		return Repository{}, f
	}
	if mres.exitCode != 0 {
		return Repository{}, newFailure(discoverOp, KindInvalidRepository, "primary worktree rev-parse failed", nil)
	}
	mainLines := stdoutLines(mres.stdout)
	if len(mainLines) != 1 {
		return Repository{}, newFailure(discoverOp, KindInvalidOutput, "unexpected primary common-dir output", nil)
	}
	canonMainCommon, err := resolveGitPath(mainLines[0], canonMain)
	if err != nil {
		return Repository{}, newFailure(discoverOp, KindInvalidRepository, "primary common dir not resolvable", err)
	}
	if canonMainCommon != canonCommonDir {
		return Repository{}, newFailure(discoverOp, KindInvalidRepository, "invocation common dir does not match primary worktree", nil)
	}

	return Repository{PrimaryWorktree: canonMain, CommonDir: canonCommonDir}, nil
}

// stdoutLines splits git stdout into its non-terminating lines, dropping only
// the single trailing newline git appends. An empty line anywhere else survives
// so callers can reject it as malformed.
func stdoutLines(stdout []byte) []string {
	s := strings.TrimSuffix(string(stdout), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// resolveGitPath canonicalizes a git-reported path, resolving it relative to
// base when git returned it relative (the common dir is ".git" when read from
// the primary worktree, absolute when read from a linked one).
func resolveGitPath(p, base string) (string, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	return filepath.EvalSymlinks(p)
}

// firstWorktreePath returns the path from the first "worktree <path>" record of
// `git worktree list --porcelain -z` output (fields NUL-terminated). It reports
// false when the first non-empty field is absent, is not a worktree line, or
// carries an empty path.
func firstWorktreePath(out []byte) (string, bool) {
	for _, field := range bytes.Split(out, []byte{0}) {
		if len(field) == 0 {
			continue
		}
		prefix := []byte("worktree ")
		if !bytes.HasPrefix(field, prefix) {
			return "", false
		}
		p := string(field[len(prefix):])
		if p == "" {
			return "", false
		}
		return p, true
	}
	return "", false
}
