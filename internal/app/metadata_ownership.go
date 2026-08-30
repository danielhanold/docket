package app

import (
	"context"
	"errors"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
)

// seedProof names which ownership proof established RootParentless.
type seedProof int

const (
	proofNone             seedProof = iota // no proof (Shape says why)
	proofInitReceipt                       // OpInitRoot receipt + empty root tree
	proofMigrateReceipt                    // OpMigrateSeed receipt, source reachable, CopyDigest == root tree
	proofLegacyEmpty                       // receiptless empty-tree root (legacy bootstrap)
	proofLegacyEquivalent                  // receiptless root exactly equal to a historical source projection (Task 4)
)

// metadataOwnership is the shared verifier's result. It is never a boolean:
// Shape preserves the foreign-vs-unreadable distinction, and Err retains the
// probe error whenever Shape is RootUnknown.
type metadataOwnership struct {
	Tip            gitcli.ObjectID     // the pinned tip the proof was computed at
	Root           gitcli.ObjectID     // the sole parentless root ("" until proven sole)
	Shape          reposetup.RootShape // RootParentless / RootForeign / RootUnknown
	Proof          seedProof
	SourceRevision string // proving snapshot for proofMigrateReceipt / proofLegacyEquivalent
	Err            error  // retained diagnostics when Shape is RootUnknown
}

// verifyMetadataOwnership decides, at a single pinned metadata tip, whether the
// tip's sole parentless-root lineage is a verified docket seed root
// (RootParentless, with permitted descendants and merges), readable evidence
// with no ownership proof (RootForeign), or incomplete/unreadable evidence
// (RootUnknown). Every probe error maps to RootUnknown with the error retained —
// never collapsed into absence or into foreign — and a receipt that claims
// docket provenance but fails validation is foreign, never downgraded to the
// receiptless legacy path. Receipt trailers are read from the ROOT COMMIT
// ITSELF: a valid receipt on a descendant cannot authorize an unrecognized root.
//
// defaultBranch is threaded through for the legacy historical-config resolution
// (Task 4); it is unused by the topology and native-receipt proofs here.
func verifyMetadataOwnership(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, tip, integrationTip gitcli.ObjectID, defaultBranch string) metadataOwnership {
	own := metadataOwnership{Tip: tip, Shape: reposetup.RootUnknown}
	roots, err := git.RootCommits(ctx, repo, tip)
	if err != nil {
		own.Err = err
		return own // unreadable history: unknown, never foreign
	}
	if len(roots) != 1 {
		own.Shape = reposetup.RootForeign
		return own
	}
	own.Root = roots[0]
	if integrationTip == "" {
		own.Err = errors.New("integration tip unknown; cannot prove disjoint ancestry")
		return own
	}
	shared, err := git.HasSharedAncestry(ctx, repo, tip, integrationTip)
	if err != nil {
		own.Err = err
		return own
	}
	if shared {
		own.Shape = reposetup.RootForeign
		return own
	}
	// Receipt trailers are read from the ROOT COMMIT ITSELF; a valid receipt on
	// a descendant cannot authorize an unrecognized root. The root is parentless,
	// so a scan from the root sees exactly the root.
	scans, err := git.ScanCommitTrailers(ctx, repo, own.Root, []string{
		reposetup.TrailerOperation, reposetup.TrailerSourceRevision,
		reposetup.TrailerMetadataRev, reposetup.TrailerCopyDigest,
		reposetup.TrailerRepairDigest,
	})
	if err != nil {
		own.Err = err
		return own
	}
	var rootTrailers []reposetup.Trailer
	for _, s := range scans {
		if s.Commit == own.Root {
			rootTrailers = fromGitcliTrailers(s.Trailers)
		}
	}
	rec, verdict := reposetup.EvaluateSeedTrailers(rootTrailers)
	rootTree, err := git.TreeOID(ctx, repo, own.Root)
	if err != nil {
		own.Err = err
		return own
	}
	switch verdict {
	case reposetup.SeedInit:
		emptyTree, err := git.EmptyTreeOID(ctx, repo)
		if err != nil {
			own.Err = err
			return own
		}
		if rootTree != emptyTree {
			own.Shape = reposetup.RootForeign
			return own
		}
		own.Shape, own.Proof = reposetup.RootParentless, proofInitReceipt
		return own
	case reposetup.SeedMigrate:
		if !isFullObjectID(rec.SourceRevision) || rec.CopyDigest != string(rootTree) {
			own.Shape = reposetup.RootForeign
			return own
		}
		reachable, err := git.IsAncestor(ctx, repo, gitcli.ObjectID(rec.SourceRevision), integrationTip)
		if err != nil {
			own.Err = err
			return own
		}
		if !reachable {
			own.Shape = reposetup.RootForeign
			return own
		}
		// The recorded repair digest keeps its meaning: an authorized-repairs seed
		// is valid without equality to an unmodified source projection.
		own.Shape, own.Proof, own.SourceRevision = reposetup.RootParentless, proofMigrateReceipt, rec.SourceRevision
		return own
	case reposetup.SeedInvalid:
		own.Shape = reposetup.RootForeign // never downgraded to the legacy path
		return own
	}
	// SeedAbsent: receiptless legacy proofs.
	emptyTree, err := git.EmptyTreeOID(ctx, repo)
	if err != nil {
		own.Err = err
		return own
	}
	if rootTree == emptyTree {
		own.Shape, own.Proof = reposetup.RootParentless, proofLegacyEmpty
		return own
	}
	return verifyLegacyEquivalence(ctx, git, repo, own, rootTree, integrationTip, defaultBranch)
}

// verifyLegacyEquivalence proves (or refuses) a receiptless NONEMPTY legacy seed
// root by exact-tree equivalence against the historical integration snapshots.
//
// This is the Task 3 STUB: it returns RootUnknown with a diagnostic error. An
// unimplemented probe is Unknown, never a fabricated foreign — nothing consumes
// the verifier yet, and Task 4 replaces this with the real historical-snapshot
// search.
func verifyLegacyEquivalence(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, own metadataOwnership, rootTree, integrationTip gitcli.ObjectID, defaultBranch string) metadataOwnership {
	own.Shape = reposetup.RootUnknown
	own.Err = errors.New("legacy-equivalence search not implemented")
	return own
}

// isFullObjectID reports whether s is a full 40-character lowercase-hex Git
// object id. A receipt's recorded source revision is untrusted input, so it is
// validated here before it is ever handed to a gitcli reader.
func isFullObjectID(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
