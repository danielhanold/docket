package gitcli

import "context"

const advanceBranchOp Operation = "advance-branch"

// AdvanceBranchChecked moves a branch that no worktree has checked out from
// expectedTip to target by compare-and-swap (`update-ref <branch> <target>
// <expectedTip>`), with repository hooks forced off. target must descend from
// expectedTip, judged with replace refs and grafts ignored. A branch some worktree
// holds is refused (moving it would strand that worktree's index), and a branch no
// longer at expectedTip is refused with nothing changed. expectedTip == target is a
// CAS-verified no-op. branch must be a fully qualified refs/heads/<name>.
func (c *Client) AdvanceBranchChecked(ctx context.Context, repo Repository, branch RefName, expectedTip, target ObjectID) error {
	op := advanceBranchOp
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
	descends, err := c.isAncestorIn(ctx, repo.PrimaryWorktree, expectedTip, target, true)
	if err != nil {
		return err
	}
	if !descends {
		return newFailure(op, KindInvalidRepository, "the target does not descend from the expected tip", nil)
	}
	infos, err := c.ListWorktrees(ctx, repo)
	if err != nil {
		return err
	}
	for _, wi := range infos {
		if wi.Branch == branch {
			return newFailure(op, KindInvalidRepository, "the branch is checked out in a worktree", nil)
		}
	}
	hooks, f := c.emptyHooksDir(ctx, op, repo.PrimaryWorktree)
	if f != nil {
		return f
	}
	res, f := c.run(ctx, runRequest{op: op, dir: repo.PrimaryWorktree,
		args: []string{"-c", "core.hooksPath=" + hooks, "update-ref", "-m", fastForwardReflogMessage, string(branch), string(target), string(expectedTip)}})
	if f != nil {
		return f
	}
	if res.exitCode != 0 {
		return newFailure(op, KindInvalidRepository, "the branch is no longer at the expected tip; nothing changed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return nil
}
