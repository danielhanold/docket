package transaction

// This file holds the two-way actual-delta guard: after a plan is materialized,
// the worktree's ACTUAL changed-path set (as Git sees it) must equal the plan's
// declared path set exactly, in both directions. Readback (verifyMaterialized)
// proves the declared paths hold the right bytes; this proves nothing else moved
// and that every declared path genuinely changed. Rename detection is off in the
// underlying status call so a move is seen as its two endpoints — a safety
// predicate must see both sides.

import (
	"context"

	"github.com/danielhanold/docket/internal/gitcli"
)

// verifyActualDelta asks Git for the worktree's actual changed-path set and
// requires set equality with the plan's declared paths in BOTH directions. An
// undeclared changed path (something moved that the plan did not describe) and a
// declared-but-unchanged path (the plan described a change that did not happen —
// the spec's "plan did not describe reality") are each a *Failure at stage
// "verify-delta" with kind invalid-state. repo is part of the engine-facing
// signature for symmetry with the other gitcli-backed steps; the changed-path
// query is keyed on the worktree directory alone.
func verifyActualDelta(ctx context.Context, client *gitcli.Client, repo gitcli.Repository,
	worktree string, plan MutationPlan) error {
	_ = repo
	changes, err := client.ChangedPaths(ctx, worktree)
	if err != nil {
		return &Failure{Stage: StageVerifyDelta, Kind: KindExternal, Detail: "reading actual changed paths", Err: err}
	}

	actual := make(map[gitcli.RepoPath]struct{}, len(changes))
	for _, ch := range changes {
		actual[ch.Path] = struct{}{}
	}
	declared := make(map[gitcli.RepoPath]struct{}, len(plan.Files))
	for _, f := range plan.Files {
		declared[f.Path] = struct{}{}
	}

	// Declared-but-unchanged: every declared path must be an actual change.
	for p := range declared {
		if _, ok := actual[p]; !ok {
			return &Failure{Stage: StageVerifyDelta, Kind: KindInvalidState, Detail: "a declared path is not an actual change"}
		}
	}
	// Undeclared: every actual change must have been declared.
	for p := range actual {
		if _, ok := declared[p]; !ok {
			return &Failure{Stage: StageVerifyDelta, Kind: KindInvalidState, Detail: "an undeclared path changed in the worktree"}
		}
	}
	return nil
}
