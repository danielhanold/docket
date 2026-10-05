package app

import (
	"context"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
)

// syncRelationship is the ONE ancestry computation behind `repository check`,
// `repository configure-tests`, and `repository prepare`: how a local tip relates to
// the remote tip it tracks. An empty tip or a probe error is the safe SyncUnknown —
// an unproven relationship is never read as current or behind, so it never
// fast-forwards and never reads healthy. The probe ignores replace refs and grafts
// (IsAncestorIgnoringReplacements), so neither can fake "behind" and strand
// local-only commits.
func syncRelationship(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, localTip, remoteTip string) reposetup.SyncRelation {
	if localTip == "" || remoteTip == "" {
		return reposetup.SyncUnknown
	}
	if localTip == remoteTip {
		return reposetup.SyncCurrent
	}
	local, remote := gitcli.ObjectID(localTip), gitcli.ObjectID(remoteTip)
	localBehind, err := git.IsAncestorIgnoringReplacements(ctx, repo, local, remote)
	if err != nil {
		return reposetup.SyncUnknown
	}
	remoteBehind, err := git.IsAncestorIgnoringReplacements(ctx, repo, remote, local)
	if err != nil {
		return reposetup.SyncUnknown
	}
	switch {
	case localBehind:
		return reposetup.SyncBehind
	case remoteBehind:
		return reposetup.SyncAhead
	default:
		return reposetup.SyncDiverged
	}
}

// applyLocalMetadataSync records how the local docket branch relates to the remote
// docket tip, and derives the .docket worktree's synchronized fact from it. Both the
// check augmentation (augmentCheckFacts) and the prepare augmentation (prepareAugment)
// call it, so their facts never disagree. It reads LocalMetadata and RemoteMetadata,
// which the caller has already resolved.
func applyLocalMetadataSync(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, f *reposetup.Facts) {
	if f.LocalMetadata.Presence == reposetup.PresencePresent && f.RemoteMetadata.Presence == reposetup.PresencePresent {
		f.LocalMetadataSync = syncRelationship(ctx, git, repo, f.LocalMetadata.Tip, f.RemoteMetadata.Tip)
	}
	if f.DocketWorktree.Presence == reposetup.PresencePresent {
		f.DocketWorktree.Synchronized = f.LocalMetadataSync.Synchronized()
	}
}
