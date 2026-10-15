package app

import (
	"context"
	"path/filepath"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file keeps the clone's metadata checkout (shared .docket, private
// checkouts/<clone-id>) current after a docket write. A write commits in a
// throwaway transaction worktree and pushes straight to the metadata remote; this
// step then moves the local metadata branch and its checkout forward IN PLACE to
// the pushed commit, by the same compare-and-swap fast-forward `repository
// prepare` uses (gitcli's FastForwardCheckedOutBranch). It is best-effort and
// silent: it never attaches or creates a checkout, never moves one that is dirty,
// detached, ahead, diverged, or already past the pushed commit, and swallows every
// error — the next prepare sees the same state it sees today and reports it.

// MetadataCheckoutFollower returns the transaction engine's AppliedHook closure:
// after an applied or already-applied metadata write, follow the clone's metadata
// checkout to the applied commit. The engine invokes it only with a non-empty
// applied commit; it is safe for concurrent invocation (all state is per call).
func MetadataCheckoutFollower(client *gitcli.Client) func(context.Context, transaction.Request, transaction.Result) {
	return func(ctx context.Context, req transaction.Request, res transaction.Result) {
		_ = followMetadataCheckout(ctx, client, req.Repository, req.TargetRef, res.AppliedCommit)
	}
}

// followMetadataCheckout fast-forwards repo's metadata checkout to pushed when
// targetRef is the layout's metadata ref and the checkout is attached on that
// branch, clean, and strictly behind pushed. Anything else — a resolution or probe
// error included — leaves the checkout exactly as it was. The pushed commit was
// created in this repository, so no fetch is needed. It reports whether the
// checkout ends at pushed (already there, or moved there now); a caller that
// would otherwise tell the user to sync the checkout uses it to skip that note,
// and the engine's AppliedHook ignores it.
//
// The local tip is read BEFORE the clean check, and that order is what keeps two
// concurrent follows (two writes landing at once in one clone) from corrupting
// the checkout. A fast-forward moves the branch first and the index second, so
// mid-flight the checkout reads dirty. Reading the tip first means a follow either
// sees the other's move as dirty and stops, or proves the index matches the tip it
// read; and a tip read before the other's move makes its own compare-and-swap
// refuse. The reverse order could prove clean against the old tip, read the moved
// tip, and stack a second fast-forward onto the first one's stale index.
func followMetadataCheckout(ctx context.Context, client *gitcli.Client, repo gitcli.Repository, targetRef gitcli.RefName, pushed gitcli.ObjectID) bool {
	if client == nil || pushed == "" {
		return false
	}
	lay, err := resolveLayout(ctx, client, repo)
	if err != nil {
		return false
	}
	metaRef := metadataRef(lay)
	if targetRef != metaRef {
		return false
	}
	worktree := lay.MetadataWorktree
	if !metadataCheckoutAttached(ctx, client, repo, worktree, metaRef) {
		return false
	}
	localTip, err := client.ResolveRef(ctx, repo, metaRef)
	if err != nil {
		return false
	}
	// read-tree -u -m refuses only a change IN THE WAY and carries any other local
	// edit across, so only a clean checkout moves — prepare's own rule. A checkout
	// already at pushed must be clean too to count as synced: a half-finished
	// fast-forward leaves the branch there with the files behind.
	if clean, _, _ := worktreeCleanState(ctx, client, worktree); clean != reposetup.PresencePresent {
		return false
	}
	if localTip == pushed {
		return true
	}
	// Refuses (nothing changed) unless pushed descends from localTip and the branch
	// is still at localTip, so it never moves backwards or across a divergence.
	return client.FastForwardCheckedOutBranch(ctx, worktree, metaRef, localTip, pushed) == nil
}

// metadataCheckoutAttached reports whether a live worktree registration sits at
// worktree with metaRef checked out. Any probe error reads as not attached, which
// only ever leaves the checkout alone. Asking the registrations, rather than
// running Git inside the directory, keeps a plain directory at that path from
// resolving to the enclosing primary worktree.
func metadataCheckoutAttached(ctx context.Context, client *gitcli.Client, repo gitcli.Repository, worktree string, metaRef gitcli.RefName) bool {
	wts, err := client.ListWorktrees(ctx, repo)
	if err != nil {
		return false
	}
	for _, wt := range wts {
		if wt.Branch == metaRef && !wt.Prunable && filepath.Clean(wt.Path) == filepath.Clean(worktree) {
			return true
		}
	}
	return false
}
