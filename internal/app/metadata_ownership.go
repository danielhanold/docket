package app

import (
	"context"
	"errors"
	"path"

	"github.com/danielhanold/docket/internal/config"
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
// root by EXACT-tree equivalence against the reachable historical integration
// snapshots. It enumerates the COMPLETE reachable integration history (never just
// today's already-pruned tip), and for each live-surface-bearing snapshot composes
// the copy-set projection ({changes, adrs, specs}) resolved through that
// snapshot's OWN committed .docket.yml — never current user/machine config — and
// compares it to rootTree by git's content-addressed tree OID. Git tree identity
// gives complete path+mode+type+object-identity equality across the copied
// prefixes (unknown files within them preserved), and refuses any extra root path,
// missing copied path, changed bytes/mode, or unrelated tree.
//
//   - An exact match sets RootParentless / proofLegacyEquivalent with the matching
//     snapshot as SourceRevision (newest-first, so the first match wins — more than
//     one snapshot proving the same tree is content identity, not ambiguity).
//   - Any gitcli read/enumeration/composition error mid-search sets RootUnknown
//     with Err immediately: a truncated search is never reported as foreign.
//   - Readable history exhausted with no match sets RootForeign.
//
// It is content-read-only: BuildTree writes only loose objects through its own
// temp index; no checkout, ref, real-index, metadata, or receipt write occurs.
func verifyLegacyEquivalence(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, own metadataOwnership, rootTree, integrationTip gitcli.ObjectID, defaultBranch string) metadataOwnership {
	entries, err := git.ListHistoryTrees(ctx, repo, integrationTip)
	if err != nil {
		own.Err = err
		own.Shape = reposetup.RootUnknown
		return own
	}
	specsDir := gitcli.RepoPath(reposetup.SpecsDir)

	// Historical config is resolved once per distinct committed .docket.yml blob
	// OID (an absent file keys as ""). The projection-tuple set dedupes candidates
	// by (configBlobOID, changes/adrs/specs subtree OIDs) so an identical copy-set
	// projection is composed at most once — its eligibility and composition are a
	// pure function of that key.
	type resolvedDirs struct {
		changes, adrs string
		ok            bool
	}
	configCache := map[gitcli.ObjectID]resolvedDirs{}
	composedKeys := map[string]bool{}
	queries := make([]gitcli.ObjectPath, len(entries))
	for i, e := range entries {
		queries[i] = gitcli.ObjectPath{Tree: e.Tree, Path: ".docket.yml"}
	}
	configIDs, err := git.PathObjectIDs(ctx, repo, queries)
	if err != nil {
		own.Err = err
		return own
	}
	var uniqueConfigs []gitcli.ObjectID
	seenConfigs := map[gitcli.ObjectID]bool{}
	for _, id := range configIDs {
		if id != "" && !seenConfigs[id] {
			uniqueConfigs = append(uniqueConfigs, id)
			seenConfigs[id] = true
		}
	}
	configBlobs, err := git.ReadBlobObjects(ctx, repo, uniqueConfigs)
	if err != nil {
		own.Err = err
		return own
	}
	for _, blob := range configBlobs {
		changes, adrs, ok := historicalDirs(blob.Bytes, defaultBranch)
		configCache[blob.ObjectID] = resolvedDirs{changes: changes, adrs: adrs, ok: ok}
	}
	changes, adrs, ok := historicalDirs(nil, defaultBranch)
	configCache[""] = resolvedDirs{changes: changes, adrs: adrs, ok: ok}
	type candidate struct {
		dirs   resolvedDirs
		offset int
	}
	candidates := make([]candidate, len(entries))
	var subtreeQueries []gitcli.ObjectPath

	for i, e := range entries {
		configOID := configIDs[i]
		dirs := configCache[configOID]
		if !dirs.ok {
			// The snapshot's committed config does not resolve: readable but
			// ineligible — skip it, never error the whole search.
			continue
		}
		candidates[i] = candidate{dirs: dirs, offset: len(subtreeQueries)}
		for _, p := range []gitcli.RepoPath{gitcli.RepoPath(dirs.changes), gitcli.RepoPath(dirs.adrs), specsDir} {
			subtreeQueries = append(subtreeQueries, gitcli.ObjectPath{Tree: e.Tree, Path: p})
		}
	}
	subtreeIDs, err := git.PathObjectIDs(ctx, repo, subtreeQueries)
	if err != nil {
		own.Err = err
		return own
	}
	for i, e := range entries {
		commit := e.Commit
		dirs := candidates[i].dirs
		if !dirs.ok {
			continue
		}
		configOID := configIDs[i]
		changesDir := gitcli.RepoPath(dirs.changes)
		adrsDir := gitcli.RepoPath(dirs.adrs)
		subIDs := subtreeIDs[candidates[i].offset : candidates[i].offset+3]
		// Without a changes entry neither active/ nor BOARD.md can exist. Avoid
		// reopening absent planning surfaces as specs/config evolve.
		if subIDs[0] == "" {
			continue
		}
		key := string(configOID) + "\x00" +
			string(subIDs[0]) + "\x00" + string(subIDs[1]) + "\x00" + string(subIDs[2])
		if composedKeys[key] {
			continue
		}
		composedKeys[key] = true

		// Eligibility: the snapshot must carry the legacy live planning surface
		// (a blob under <changes>/active/, or a <changes>/BOARD.md).
		eligible, err := candidateHasLiveSurface(ctx, git, repo, commit, dirs.changes)
		if err != nil {
			own.Err = err
			own.Shape = reposetup.RootUnknown
			return own
		}
		if !eligible {
			continue
		}

		// Compose the copy-set projection exactly as migrateExecute composes a seed:
		// IncludePrefix only for the prefixes that exist in the candidate.
		var ops []gitcli.TreeOp
		for j, p := range []gitcli.RepoPath{changesDir, adrsDir, specsDir} {
			if subIDs[j] != "" {
				ops = append(ops, gitcli.TreeOp{IncludePrefix: &gitcli.IncludePrefixOp{From: commit, Prefix: p}})
			}
		}
		projection, err := git.BuildTree(ctx, repo, "", ops)
		if err != nil {
			own.Err = err
			own.Shape = reposetup.RootUnknown
			return own
		}
		if projection == rootTree {
			own.Shape = reposetup.RootParentless
			own.Proof = proofLegacyEquivalent
			own.SourceRevision = string(commit)
			return own
		}
	}

	// Readable history exhausted cleanly with no exact match: foreign.
	own.Shape = reposetup.RootForeign
	return own
}

// historicalDirs resolves the changes/adrs directories from a snapshot's OWN
// committed .docket.yml bytes with shipped defaults where absent. It builds a
// single repository-layer source (nil bytes → defaults only) and resolves it
// through config.Resolve — NEVER the global or repository-local machine layers,
// so current user/machine configuration cannot redefine historical evidence. The
// obsolete metadata_branch tombstone decodes as a warning, not an error, so a
// snapshot carrying it still resolves. A snapshot whose committed config fails to
// resolve is readable-but-ineligible (ok == false): the caller skips it.
func historicalDirs(docketYML []byte, defaultBranch string) (changes, adrs string, ok bool) {
	var sources []config.Source
	if docketYML != nil {
		sources = []config.Source{{Layer: config.LayerRepository, Name: ".docket.yml", Data: docketYML}}
	}
	snap, _, err := config.Resolve(sources, config.ResolveContext{DefaultBranch: defaultBranch})
	if err != nil {
		return "", "", false
	}
	return snap.Effective.ChangesDir.Value, snap.Effective.ADRsDir.Value, true
}

// candidateHasLiveSurface reports whether a historical snapshot carries the legacy
// live planning surface — a blob under <changes>/active/, or a <changes>/BOARD.md.
// It mirrors liveSurfacePresence's predicate, evaluated against the candidate
// commit's object source. A read error is returned to the caller (never a false
// absence), so an unreadable candidate maps to Unknown, not Foreign.
func candidateHasLiveSurface(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, commit gitcli.ObjectID, changesDir string) (bool, error) {
	src, err := git.OpenObjectSource(ctx, repo, gitcli.Revision{Commit: commit})
	if err != nil {
		return false, err
	}
	activePrefix := gitcli.RepoPath(path.Join(changesDir, "active"))
	entries, err := src.ListTree(ctx, []gitcli.RepoPath{activePrefix})
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.Type == "blob" {
			return true, nil
		}
	}
	boardPath := gitcli.RepoPath(path.Join(changesDir, "BOARD.md"))
	results, err := src.ReadBlobs(ctx, []gitcli.RepoPath{boardPath})
	if err != nil {
		return false, err
	}
	if len(results) == 1 && results[0].Found {
		return true, nil
	}
	return false, nil
}

// isFullObjectID reports whether s is a full lowercase-hex Git object id:
// exactly 40 characters (SHA-1) or exactly 64 (SHA-256), the same full widths
// the gitcli reader's validateObjectID accepts. A receipt's recorded source
// revision is untrusted input, so it is validated here before it is ever
// handed to a gitcli reader. This is a syntax check only: acceptance does not
// imply the object exists, is a commit, or is reachable — later ownership
// checks and Git itself still decide that.
func isFullObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
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
