package gitcli

import (
	"context"
	"path/filepath"
	"strings"
)

const fastForwardCheckedOutOp Operation = "fast-forward-checked-out-branch"

// fastForwardReflogMessage names the in-place fast-forward in the branch reflog.
const fastForwardReflogMessage = "docket: fast-forward"

// FastForwardCheckedOutBranch advances branch — which must be the branch checked out
// at worktreeDir — from expectedTip to target IN PLACE. Nothing is removed: the
// worktree directory, its admin dir (registration, per-worktree config, lock), the
// branch, and its reflog all survive. The branch moves by compare-and-swap from
// expectedTip, so a commit made since the caller observed expectedTip makes it
// refuse with nothing changed. target must descend from expectedTip, judged with
// replace refs and grafts ignored. The tree moves with `read-tree -u -m`, which
// refuses rather than overwrite a local change or an untracked file in the way; on
// that refusal the branch is swapped back. An ignored file at a path target tracks
// is overwritten, Git's normal checkout rule. Every command forces docket's
// empty hooks dir, so no repository hook runs even when the worktree's own hooks-off
// setting is missing. An interruption after the ref swap leaves HEAD at target with
// the old index, which `git status` reports as staged changes (dirty), never clean.
func (c *Client) FastForwardCheckedOutBranch(ctx context.Context, worktreeDir string, branch RefName, expectedTip, target ObjectID) error {
	op := fastForwardCheckedOutOp
	if !filepath.IsAbs(worktreeDir) {
		return newFailure(op, KindInvalidRequest, "worktree path must be absolute", nil)
	}
	if err := validateRefName(branch); err != nil {
		return newFailure(op, KindInvalidRequest, "invalid branch ref", err)
	}
	if _, ok := branchShortName(branch); !ok {
		return newFailure(op, KindInvalidRequest, "branch must be fully qualified refs/heads/<name>", nil)
	}
	if err := validateObjectID(expectedTip); err != nil {
		return newFailure(op, KindInvalidRequest, "invalid expected tip id", err)
	}
	if err := validateObjectID(target); err != nil {
		return newFailure(op, KindInvalidRequest, "invalid target id", err)
	}

	sym, f := c.run(ctx, runRequest{op: op, dir: worktreeDir, args: []string{"symbolic-ref", "--quiet", "HEAD"}})
	if f != nil {
		return f
	}
	if sym.exitCode != 0 || RefName(strings.TrimSpace(string(sym.stdout))) != branch {
		return newFailure(op, KindInvalidRepository, "the worktree does not have the branch checked out", nil)
	}

	descends, err := c.isAncestorIn(ctx, worktreeDir, expectedTip, target, true)
	if err != nil {
		return err
	}
	if !descends {
		return newFailure(op, KindInvalidRepository, "the target does not descend from the expected tip", nil)
	}

	hooks, f := c.emptyHooksDir(ctx, op, worktreeDir)
	if f != nil {
		return f
	}
	noHooks := "core.hooksPath=" + hooks

	cas, f := c.run(ctx, runRequest{op: op, dir: worktreeDir,
		args: []string{"-c", noHooks, "update-ref", "-m", fastForwardReflogMessage, string(branch), string(target), string(expectedTip)}})
	if f != nil {
		return f
	}
	if cas.exitCode != 0 {
		return newFailure(op, KindInvalidRepository, "the branch is no longer at the expected tip; nothing changed: "+stderrExcerpt(cas.stderr), nil).withExitCode(cas.exitCode)
	}

	// Refresh stale stat info first: read-tree judges a tracked file whose stat
	// differs from the index as modified, and would refuse over an untouched file.
	// Exit 1 (some entries need updating) is accepted; read-tree still refuses a
	// real local change.
	refresh, f := c.run(ctx, runRequest{op: op, dir: worktreeDir, args: []string{"-c", noHooks, "update-index", "-q", "--refresh"}})
	if f == nil && refresh.exitCode > 1 {
		f = newFailure(op, KindCommandFailed, "update-index --refresh failed: "+stderrExcerpt(refresh.stderr), nil).withExitCode(refresh.exitCode)
	}
	if f == nil {
		rt, rf := c.run(ctx, runRequest{op: op, dir: worktreeDir,
			args: []string{"-c", noHooks, "read-tree", "-u", "-m", string(expectedTip), string(target)}})
		switch {
		case rf != nil:
			f = rf
		case rt.exitCode != 0:
			f = newFailure(op, KindInvalidRepository, "the working tree update was refused: "+stderrExcerpt(rt.stderr), nil).withExitCode(rt.exitCode)
		default:
			return nil
		}
	}

	// The tree update did not complete: swap the branch back so nothing changed. The
	// swap-back detaches from the caller's cancellation (it stays bounded by the
	// per-command timeout), so a cancelled or timed-out caller still gets the branch
	// restored rather than left ahead of its index.
	back, bf := c.run(context.WithoutCancel(ctx), runRequest{op: op, dir: worktreeDir,
		args: []string{"-c", noHooks, "update-ref", "-m", fastForwardReflogMessage + " (rolled back)", string(branch), string(expectedTip), string(target)}})
	if bf != nil {
		return newFailure(op, KindCommandFailed, f.Error()+"; rolling the branch back also failed: "+bf.Error(), nil)
	}
	if back.exitCode != 0 {
		return newFailure(op, KindCommandFailed, f.Error()+"; rolling the branch back also failed: "+stderrExcerpt(back.stderr), nil).withExitCode(back.exitCode)
	}
	return f
}

const interruptedFastForwardOp Operation = "interrupted-fast-forward"

// InterruptedFastForward reports whether worktreeDir, with branch checked out, holds
// the state FastForwardCheckedOutBranch leaves when killed between its ref swap and
// its tree update: the branch's newest reflog entry is the fast-forward's own (not
// its roll-back) and the index still equals the tree of the branch's previous tip.
// A completed fast-forward has an index at the new tip, so it reads false. Every
// probe failure is a typed error, never a false.
func (c *Client) InterruptedFastForward(ctx context.Context, worktreeDir string, branch RefName) (bool, error) {
	op := interruptedFastForwardOp
	if !filepath.IsAbs(worktreeDir) {
		return false, newFailure(op, KindInvalidRequest, "worktree path must be absolute", nil)
	}
	if err := validateRefName(branch); err != nil {
		return false, newFailure(op, KindInvalidRequest, "invalid branch ref", err)
	}
	if _, ok := branchShortName(branch); !ok {
		return false, newFailure(op, KindInvalidRequest, "branch must be fully qualified refs/heads/<name>", nil)
	}

	last, f := c.run(ctx, runRequest{op: op, dir: worktreeDir,
		args: []string{"log", "--walk-reflogs", "-1", "--format=%gs", string(branch), "--"}})
	if f != nil {
		return false, f
	}
	if last.exitCode != 0 {
		return false, newFailure(op, KindCommandFailed, "reading the branch reflog failed: "+stderrExcerpt(last.stderr), nil).withExitCode(last.exitCode)
	}
	if strings.TrimSpace(string(last.stdout)) != fastForwardReflogMessage {
		return false, nil
	}

	diff, f := c.run(ctx, runRequest{op: op, dir: worktreeDir,
		args: []string{"diff", "--cached", "--quiet", string(branch) + "@{1}", "--"}})
	if f != nil {
		return false, f
	}
	switch diff.exitCode {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, newFailure(op, KindCommandFailed, "comparing the index with the previous tip failed: "+stderrExcerpt(diff.stderr), nil).withExitCode(diff.exitCode)
	}
}
