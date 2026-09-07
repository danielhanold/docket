package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

const worktreeCheckoutStateOp Operation = "worktree-checkout-state"

// CheckoutState is one observation of a worktree's checkout: the symbolic
// branch (full ref) or detached, the HEAD commit, and whether an unfinished
// Git operation (merge, rebase, cherry-pick, revert, bisect) is in progress.
type CheckoutState struct {
	Branch              RefName // full ref, e.g. refs/heads/main; empty when Detached
	Detached            bool
	Head                ObjectID
	OperationInProgress bool
}

// WorktreeCheckoutState probes worktreeDir. Every failure is a typed error —
// a caller must never read a zero CheckoutState as an observed fact
// (learnings: probe-error-is-not-clean-absence).
func (c *Client) WorktreeCheckoutState(ctx context.Context, worktreeDir string) (CheckoutState, error) {
	var st CheckoutState
	if !filepath.IsAbs(worktreeDir) {
		return st, newFailure(worktreeCheckoutStateOp, KindInvalidRequest, "worktree path must be absolute", nil)
	}

	sym, f := c.run(ctx, runRequest{
		op:   worktreeCheckoutStateOp,
		dir:  worktreeDir,
		args: []string{"symbolic-ref", "--quiet", "HEAD"},
	})
	if f != nil {
		return st, f
	}
	switch sym.exitCode {
	case 0:
		st.Branch = RefName(strings.TrimSpace(string(sym.stdout)))
	case 1:
		st.Detached = true // --quiet: exit 1 IS the clean detached answer
	default:
		return CheckoutState{}, newFailure(worktreeCheckoutStateOp, KindCommandFailed,
			"symbolic-ref HEAD failed: "+stderrExcerpt(sym.stderr), nil).withExitCode(sym.exitCode)
	}

	head, hf := c.worktreeHead(ctx, worktreeCheckoutStateOp, worktreeDir)
	if hf != nil {
		return CheckoutState{}, hf
	}
	st.Head = head

	// Unfinished-operation markers, resolved through git one path at a time (as
	// rebaseInProgress does) so linked worktrees and split git-dirs are honored.
	// `rev-parse --git-path` binds only its immediately following token, so a
	// single invocation with several markers makes git parse the rest as
	// revisions and a non-existent marker aborts the whole probe — hence the
	// per-marker loop. Any marker's presence is in-progress.
	markers := []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_LOG", "rebase-merge", "rebase-apply"}
	for _, marker := range markers {
		res, pf := c.run(ctx, runRequest{
			op:   worktreeCheckoutStateOp,
			dir:  worktreeDir,
			args: []string{"rev-parse", "--git-path", marker},
		})
		if pf != nil {
			return CheckoutState{}, pf
		}
		if res.exitCode != 0 {
			return CheckoutState{}, newFailure(worktreeCheckoutStateOp, KindCommandFailed,
				"rev-parse --git-path failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
		}
		lines := stdoutLines(res.stdout)
		if len(lines) != 1 {
			return CheckoutState{}, newFailure(worktreeCheckoutStateOp, KindInvalidOutput, "unexpected rev-parse --git-path output", nil)
		}
		p := lines[0]
		if !filepath.IsAbs(p) {
			p = filepath.Join(worktreeDir, p)
		}
		if _, err := os.Stat(p); err == nil {
			st.OperationInProgress = true
			break
		}
	}
	return st, nil
}
