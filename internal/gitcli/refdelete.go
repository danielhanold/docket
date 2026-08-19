package gitcli

import "context"

// Operation labels for the checked ref-deletion surface.
const (
	deleteLocalBranchOp Operation = "delete-local-branch"
	deleteRemoteRefOp   Operation = "delete-remote-ref"
)

// DeleteLocalBranchChecked deletes a local branch only when both preconditions
// hold: the branch tip equals expectedTip exactly, and the branch is checked out
// in no worktree. It resolves the live tip and scans the worktree list itself
// (git's plumbing delete has no checked-out safety), refusing on either
// violation with the branch left intact, then deletes via
// `update-ref -d <branch> <expectedTip>` so the tip is re-verified atomically at
// the destructive boundary. branch must be a fully qualified refs/heads/<name>.
func (c *Client) DeleteLocalBranchChecked(ctx context.Context, repo Repository, branch RefName, expectedTip ObjectID) error {
	if err := validateRefName(branch); err != nil {
		return newFailure(deleteLocalBranchOp, KindInvalidRequest, "invalid branch ref", err)
	}
	if _, ok := branchShortName(branch); !ok {
		return newFailure(deleteLocalBranchOp, KindInvalidRequest, "branch must be fully qualified refs/heads/<name>", nil)
	}
	if err := validateObjectID(expectedTip); err != nil {
		return newFailure(deleteLocalBranchOp, KindInvalidRequest, "invalid expected tip id", err)
	}

	// Live tip must match exactly. An absent branch surfaces as ResolveRef's
	// ref-unavailable — never a silent success.
	tip, err := c.ResolveRef(ctx, repo, branch)
	if err != nil {
		return err
	}
	if tip != expectedTip {
		return newFailure(deleteLocalBranchOp, KindInvalidRepository, "branch tip does not match the expected tip", nil)
	}

	// The branch must be checked out nowhere: deleting a branch a worktree holds
	// would strand that worktree on a dangling HEAD.
	infos, err := c.ListWorktrees(ctx, repo)
	if err != nil {
		return err
	}
	for _, wi := range infos {
		if wi.Branch == branch {
			return newFailure(deleteLocalBranchOp, KindInvalidRepository, "branch is checked out in a worktree", nil)
		}
	}

	// Atomic checked delete: the old-value form re-verifies the tip at the delete
	// boundary, closing the resolve-then-delete race.
	res, f := c.run(ctx, runRequest{
		op:   deleteLocalBranchOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"update-ref", "-d", string(branch), string(expectedTip)},
	})
	if f != nil {
		return f
	}
	if res.exitCode != 0 {
		return newFailure(deleteLocalBranchOp, KindCommandFailed, "update-ref -d failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return nil
}

// DeleteRemoteRefLease deletes a remote branch ref under the exact old-value
// lease, via `push --porcelain --force-with-lease=<ref>:<expectedTip> <remote>
// :<ref>`. Classification never trusts the process exit alone: a structural per-ref
// delete flag ('-') is applied; anything else is decided by an authoritative
// ProbeRemoteBranch, giving three outcomes — the ref cleanly absent is applied
// (the promised state already holds, idempotent), the ref present at a commit
// other than expectedTip is lease-lost with the winner retained (never a
// destructive overwrite), and an unobservable remote (or one still at
// expectedTip after a rejection) is failed, so an unknown never authorizes
// treating the ref as gone. ref must be a fully qualified refs/heads/<name>.
func (c *Client) DeleteRemoteRefLease(ctx context.Context, repo Repository, remote RemoteName, ref RefName, expectedTip ObjectID) (PushOutcome, error) {
	if err := validateRemoteName(remote); err != nil {
		return PushOutcome{}, newFailure(deleteRemoteRefOp, KindInvalidRequest, "invalid remote name", err)
	}
	if err := validateRefName(ref); err != nil {
		return PushOutcome{}, newFailure(deleteRemoteRefOp, KindInvalidRequest, "invalid ref name", err)
	}
	if _, ok := branchShortName(ref); !ok {
		return PushOutcome{}, newFailure(deleteRemoteRefOp, KindInvalidRequest, "ref must be fully qualified refs/heads/<name>", nil)
	}
	if err := validateObjectID(expectedTip); err != nil {
		return PushOutcome{}, newFailure(deleteRemoteRefOp, KindInvalidRequest, "invalid expected tip id", err)
	}

	lease := "--force-with-lease=" + string(ref) + ":" + string(expectedTip)
	refspec := ":" + string(ref)
	res, f := c.run(ctx, runRequest{
		op:      deleteRemoteRefOp,
		dir:     repo.PrimaryWorktree,
		args:    []string{"push", "--porcelain", lease, string(remote), refspec},
		network: true,
	})
	if f != nil {
		return PushOutcome{}, f
	}

	// A structural delete flag is an unambiguous success.
	if flag, found := parsePushRefLine(res.stdout, ref); found && flag == "-" {
		return PushOutcome{Disposition: PushApplied}, nil
	}

	// Otherwise the authoritative probe decides. An errored probe is unknown and
	// never shares the "gone" branch with a clean absence.
	rr, err := c.ProbeRemoteBranch(ctx, repo, remote, ref)
	if err != nil {
		return PushOutcome{Disposition: PushFailed}, nil
	}
	if rr.State == RemoteRefAbsent {
		return PushOutcome{Disposition: PushApplied}, nil
	}
	if rr.Commit == expectedTip {
		// Rejected yet still exactly at the expected tip: nothing moved, but the
		// delete did not land — a plain failure the caller may retry.
		return PushOutcome{Disposition: PushFailed, Remote: rr.Commit}, nil
	}
	return PushOutcome{Disposition: PushLeaseLost, Remote: rr.Commit}, nil
}
