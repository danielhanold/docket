package workspace

import "github.com/danielhanold/docket/internal/gitcli"

// This file exposes the one path derivation a finalize caller needs to target
// the rebase receipt: the per-workspace metadata directory. The receipt
// lifecycle (WriteRebaseReceipt / ReadRebaseReceipt / ClearRebaseReceipt) takes
// an already-resolved `dir`, so its caller must name the exact same directory
// the workspace service itself uses. That directory is derived from the common
// dir and the feature ref through the package-private workspaceDir; recomputing
// the sha256-of-ref derivation in another package would be a second copy that
// silently drifts from this one. MetaDir is the single public seam onto that
// derivation, so the finalize layer decides-and-acts on the same location the
// manifest and receipt already live in.

// MetaDir returns the per-workspace metadata directory a target's manifest and
// rebase receipt live in — <commonDir>/docket/workspaces/<sha256(featureRef)>.
// commonDir is a repository's canonical common directory (gitcli.Repository's
// CommonDir); featureRef is the fully qualified refs/heads/feat/<slug>. The
// returned path is exactly the directory WriteRebaseReceipt and its siblings
// operate on.
func MetaDir(commonDir string, featureRef gitcli.RefName) string {
	return workspaceDir(commonDir, featureRef)
}
