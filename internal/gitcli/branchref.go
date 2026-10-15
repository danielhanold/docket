package gitcli

import "context"

const fastForwardBranchRefOp Operation = "fast-forward-branch-ref"

// FastForwardBranchRef moves a local branch no worktree has checked out from
// expectedTip to target by compare-and-swap, so a branch that moved since the
// caller read it is refused unchanged; FastForwardCheckedOutBranch moves a
// checked-out branch. branch must be a fully qualified refs/heads/... name and
// both ids well-formed (invalid-request). A target that does not descend from
// expectedTip, or a branch some worktree has checked out, is
// invalid-repository with nothing moved. The move itself is
// `update-ref <branch> <target> <expectedTip>`, run from the primary worktree:
// git refuses it when the branch no longer sits at expectedTip, surfaced as
// command-failed.
func (c *Client) FastForwardBranchRef(ctx context.Context, repo Repository, branch RefName, expectedTip, target ObjectID) error {
	if err := validateRefName(branch); err != nil {
		return newFailure(fastForwardBranchRefOp, KindInvalidRequest, "invalid branch ref", err)
	}
	if _, ok := branchShortName(branch); !ok {
		return newFailure(fastForwardBranchRefOp, KindInvalidRequest, "branch must be fully qualified refs/heads/<name>", nil)
	}
	if err := validateObjectID(expectedTip); err != nil {
		return newFailure(fastForwardBranchRefOp, KindInvalidRequest, "invalid expected tip id", err)
	}
	if err := validateObjectID(target); err != nil {
		return newFailure(fastForwardBranchRefOp, KindInvalidRequest, "invalid target id", err)
	}
	descends, err := c.IsAncestorIgnoringReplacements(ctx, repo, expectedTip, target)
	if err != nil {
		return err
	}
	if !descends {
		return newFailure(fastForwardBranchRefOp, KindInvalidRepository, "target does not descend from the expected tip", nil)
	}
	infos, err := c.ListWorktrees(ctx, repo)
	if err != nil {
		return err
	}
	for _, wi := range infos {
		if !wi.Detached && wi.Branch == branch {
			return newFailure(fastForwardBranchRefOp, KindInvalidRepository, "branch is checked out in a worktree", nil)
		}
	}
	res, f := c.run(ctx, runRequest{
		op:   fastForwardBranchRefOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"update-ref", "-m", "docket: fast-forward", string(branch), string(target), string(expectedTip)},
	})
	if f != nil {
		return f
	}
	if res.exitCode != 0 {
		return newFailure(fastForwardBranchRefOp, KindCommandFailed, "update-ref failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return nil
}
