//go:build integration

package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
)

// This is the real-Git ownership-verifier integration shard (prefix
// TestIntegrationRepoOwnership). Each test drives verifyMetadataOwnership
// directly against a precisely-shaped fixture repo built object-by-object with
// `git commit-tree` / `git write-tree`, so a root's tree, trailers, parents, and
// ancestry are under exact control. The verifier reads immutable objects only;
// no ref is required — commit-tree writes loose objects that persist for the run.
//
// Task 3 covers the topology and native-receipt proofs plus the unknown mapping.
// The receiptless-NONEMPTY legacy-equivalence path is stubbed to RootUnknown
// here (Task 4 implements it), so the one negative that depends on a receiptless
// nonempty root resolving to RootForeign lives in Task 4's tests, not this file.

// ownFixture is a single real repo carrying an integration branch (main) plus a
// metadata lineage assembled with plumbing so the verifier can be pointed at
// exact roots and tips.
type ownFixture struct {
	t   *testing.T
	dir string
}

// newOwnFixture builds a repo on `main` with one initial integration commit.
func newOwnFixture(t *testing.T) *ownFixture {
	t.Helper()
	requireRealGit(t)
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	gitIdentity(t, dir)
	writeRepoFile(t, dir, "README.md", "readme\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "initial integration")
	return &ownFixture{t: t, dir: dir}
}

// mainTip returns the integration branch tip OID (an independent oracle).
func (f *ownFixture) mainTip() string {
	f.t.Helper()
	return runGit(f.t, f.dir, "rev-parse", "main")
}

// emptyTree writes and returns the repository's empty-tree OID.
func (f *ownFixture) emptyTree() string {
	f.t.Helper()
	return runGit(f.t, f.dir, "hash-object", "-w", "-t", "tree", "/dev/null")
}

// gitStdin runs git -C dir with stdin piped in, failing on a nonzero exit.
func (f *ownFixture) gitStdin(stdin string, args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", f.dir}, args...)...)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		f.t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, errb.String())
	}
	return strings.TrimSpace(out.String())
}

// gitEnv runs git -C dir with extra environment entries appended.
func (f *ownFixture) gitEnv(env []string, args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", f.dir}, args...)...)
	cmd.Env = append(cmd.Environ(), env...)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		f.t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, errb.String())
	}
	return strings.TrimSpace(out.String())
}

// mktree composes a NONEMPTY tree from a path->content map via an isolated temp
// index, returning its tree OID without touching the working index.
func (f *ownFixture) mktree(files map[string]string) string {
	f.t.Helper()
	idx := filepath.Join(f.t.TempDir(), "index")
	env := []string{"GIT_INDEX_FILE=" + idx}
	for rel, content := range files {
		blob := f.gitStdin(content, "hash-object", "-w", "-t", "blob", "--stdin")
		f.gitEnv(env, "update-index", "--add", "--cacheinfo", "100644", blob, rel)
	}
	return f.gitEnv(env, "write-tree")
}

// commitTree writes a commit object over tree with the given parents and message
// (read from stdin), returning its OID. With no parents it is a parentless root.
func (f *ownFixture) commitTree(tree string, parents []string, message string) string {
	f.t.Helper()
	args := []string{"commit-tree", tree}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	return f.gitStdin(message, args...)
}

// verify runs the verifier against this fixture at the given tips.
func (f *ownFixture) verify(t *testing.T, tip, integration gitcli.ObjectID) metadataOwnership {
	t.Helper()
	client := newGitClient(t)
	return verifyMetadataOwnership(context.Background(), client, gitcli.Repository{PrimaryWorktree: f.dir}, tip, integration, "main")
}

// seedMessage builds a commit message whose final paragraph is the given trailer
// lines, so Git's own trailer interpretation (which ScanCommitTrailers reads)
// recognizes them.
func seedMessage(subject string, trailers ...string) string {
	return subject + "\n\n" + strings.Join(trailers, "\n") + "\n"
}

func opTrailer(op string) string    { return reposetup.TrailerOperation + ": " + op }
func srcTrailer(v string) string    { return reposetup.TrailerSourceRevision + ": " + v }
func copyTrailer(v string) string   { return reposetup.TrailerCopyDigest + ": " + v }
func repairTrailer(v string) string { return reposetup.TrailerRepairDigest + ": " + v }

// --- Positive proofs ---------------------------------------------------------

// The defect under fix: an init seed followed by ordinary metadata commits must
// still verify. Written first per the plan.
func TestIntegrationRepoOwnershipInitSeedWithDescendants(t *testing.T) {
	f := newOwnFixture(t)
	et := f.emptyTree()
	root := f.commitTree(et, nil, seedMessage("docket init", opTrailer(reposetup.OpInitRoot)))
	c1 := f.commitTree(f.mktree(map[string]string{"docs/changes/active/0001-a.md": "a\n"}), []string{root}, "meta 1")
	c2 := f.commitTree(f.mktree(map[string]string{"docs/changes/active/0002-b.md": "b\n"}), []string{c1}, "meta 2")
	tip := f.commitTree(f.mktree(map[string]string{"docs/changes/active/0003-c.md": "c\n"}), []string{c2}, "meta 3")

	own := f.verify(t, gitcli.ObjectID(tip), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootParentless {
		t.Fatalf("Shape = %v, want RootParentless (err=%v)", own.Shape, own.Err)
	}
	if own.Proof != proofInitReceipt {
		t.Errorf("Proof = %d, want proofInitReceipt", own.Proof)
	}
	if own.Root != gitcli.ObjectID(root) {
		t.Errorf("Root = %s, want the seed root %s", own.Root, root)
	}
	if own.Tip != gitcli.ObjectID(tip) {
		t.Errorf("Tip = %s, want %s (the root need not equal the tip)", own.Tip, tip)
	}
	if own.Tip == own.Root {
		t.Error("tip equals root, but this fixture has descendants")
	}
}

func TestIntegrationRepoOwnershipInitSeedAlone(t *testing.T) {
	f := newOwnFixture(t)
	et := f.emptyTree()
	root := f.commitTree(et, nil, seedMessage("docket init", opTrailer(reposetup.OpInitRoot)))

	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootParentless || own.Proof != proofInitReceipt {
		t.Fatalf("got Shape=%v Proof=%d, want RootParentless/proofInitReceipt (err=%v)", own.Shape, own.Proof, own.Err)
	}
}

func TestIntegrationRepoOwnershipMigrateSeedWithDescendants(t *testing.T) {
	f := newOwnFixture(t)
	proj := f.mktree(map[string]string{
		"docs/changes/active/0001-a.md": "a\n",
		"docs/adrs/0001-x.md":           "adr\n",
	})
	src := f.mainTip() // reachable from the integration tip (it is the integration tip)
	root := f.commitTree(proj, nil, seedMessage("docket migrate seed",
		opTrailer(reposetup.OpMigrateSeed),
		srcTrailer(src),
		copyTrailer(proj),
		repairTrailer("deadbeef")))
	tip := f.commitTree(f.mktree(map[string]string{"docs/changes/active/0002-b.md": "b\n"}), []string{root}, "meta after migrate")

	own := f.verify(t, gitcli.ObjectID(tip), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootParentless {
		t.Fatalf("Shape = %v, want RootParentless (err=%v)", own.Shape, own.Err)
	}
	if own.Proof != proofMigrateReceipt {
		t.Errorf("Proof = %d, want proofMigrateReceipt", own.Proof)
	}
	if own.SourceRevision != src {
		t.Errorf("SourceRevision = %s, want %s (preserved from the receipt)", own.SourceRevision, src)
	}
}

func TestIntegrationRepoOwnershipMergeOfDescendantsSharingRoot(t *testing.T) {
	f := newOwnFixture(t)
	et := f.emptyTree()
	root := f.commitTree(et, nil, seedMessage("docket init", opTrailer(reposetup.OpInitRoot)))
	a := f.commitTree(f.mktree(map[string]string{"docs/changes/active/0001-a.md": "a\n"}), []string{root}, "branch a")
	b := f.commitTree(f.mktree(map[string]string{"docs/changes/active/0002-b.md": "b\n"}), []string{root}, "branch b")
	merge := f.commitTree(f.mktree(map[string]string{"docs/changes/active/0003-c.md": "c\n"}), []string{a, b}, "merge a and b")

	own := f.verify(t, gitcli.ObjectID(merge), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootParentless {
		t.Fatalf("Shape = %v, want RootParentless for a merge of descendants sharing the verified root (err=%v)", own.Shape, own.Err)
	}
	if own.Root != gitcli.ObjectID(root) {
		t.Errorf("Root = %s, want the shared seed root %s", own.Root, root)
	}
}

func TestIntegrationRepoOwnershipLegacyEmptyBootstrapWithDescendants(t *testing.T) {
	f := newOwnFixture(t)
	et := f.emptyTree()
	root := f.commitTree(et, nil, "legacy bootstrap seed") // no receipt trailer
	tip := f.commitTree(f.mktree(map[string]string{"docs/changes/active/0001-a.md": "a\n"}), []string{root}, "first metadata commit")

	own := f.verify(t, gitcli.ObjectID(tip), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootParentless {
		t.Fatalf("Shape = %v, want RootParentless (err=%v)", own.Shape, own.Err)
	}
	if own.Proof != proofLegacyEmpty {
		t.Errorf("Proof = %d, want proofLegacyEmpty", own.Proof)
	}
}

// --- Negative proofs (all resolve without the legacy-equivalence stub) --------

func TestIntegrationRepoOwnershipTwoParentlessRoots(t *testing.T) {
	f := newOwnFixture(t)
	et := f.emptyTree()
	r1 := f.commitTree(et, nil, "root one")
	r2 := f.commitTree(f.mktree(map[string]string{"a": "1\n"}), nil, "root two")
	merge := f.commitTree(et, []string{r1, r2}, "merge of two orphan roots")

	own := f.verify(t, gitcli.ObjectID(merge), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootForeign {
		t.Fatalf("Shape = %v, want RootForeign for two parentless roots (err=%v)", own.Shape, own.Err)
	}
}

// A branch created FROM the integration branch shares ancestry and is foreign
// even carrying docket-looking files and a valid-looking receipt: branch name,
// subject, author, timestamps, and a populated .docket dir prove nothing.
func TestIntegrationRepoOwnershipSharedAncestryWithIntegration(t *testing.T) {
	f := newOwnFixture(t)
	foreign := f.commitTree(
		f.mktree(map[string]string{
			"docs/changes/active/0001-a.md": "a\n",
			".docket/marker":                "present\n",
		}),
		[]string{f.mainTip()},
		seedMessage("docket metadata seed", opTrailer(reposetup.OpInitRoot)))

	own := f.verify(t, gitcli.ObjectID(foreign), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootForeign {
		t.Fatalf("Shape = %v, want RootForeign for a branch sharing ancestry with integration (err=%v)", own.Shape, own.Err)
	}
}

func TestIntegrationRepoOwnershipInitReceiptNonemptyTree(t *testing.T) {
	f := newOwnFixture(t)
	proj := f.mktree(map[string]string{"docs/changes/active/0001-a.md": "a\n"})
	root := f.commitTree(proj, nil, seedMessage("init but nonempty", opTrailer(reposetup.OpInitRoot)))

	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootForeign {
		t.Fatalf("Shape = %v, want RootForeign for an init receipt over a nonempty tree (err=%v)", own.Shape, own.Err)
	}
}

func TestIntegrationRepoOwnershipMigrateReceiptCopyDigestMismatch(t *testing.T) {
	f := newOwnFixture(t)
	proj := f.mktree(map[string]string{"docs/changes/active/0001-a.md": "a\n"})
	root := f.commitTree(proj, nil, seedMessage("migrate wrong digest",
		opTrailer(reposetup.OpMigrateSeed),
		srcTrailer(f.mainTip()),
		copyTrailer(f.emptyTree()), // != proj, the actual root tree
		repairTrailer("deadbeef")))

	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootForeign {
		t.Fatalf("Shape = %v, want RootForeign when CopyDigest != root tree (err=%v)", own.Shape, own.Err)
	}
}

func TestIntegrationRepoOwnershipMigrateReceiptSourceUnreachable(t *testing.T) {
	f := newOwnFixture(t)
	// A real object that exists but is NOT reachable from the integration tip.
	orphanSrc := f.commitTree(f.mktree(map[string]string{"unrelated": "z\n"}), nil, "unrelated orphan")
	proj := f.mktree(map[string]string{"docs/changes/active/0001-a.md": "a\n"})
	root := f.commitTree(proj, nil, seedMessage("migrate unreachable source",
		opTrailer(reposetup.OpMigrateSeed),
		srcTrailer(orphanSrc),
		copyTrailer(proj),
		repairTrailer("deadbeef")))

	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootForeign {
		t.Fatalf("Shape = %v, want RootForeign when the source revision is unreachable (err=%v)", own.Shape, own.Err)
	}
}

func TestIntegrationRepoOwnershipMigrateReceiptMalformedSource(t *testing.T) {
	f := newOwnFixture(t)
	proj := f.mktree(map[string]string{"docs/changes/active/0001-a.md": "a\n"})
	root := f.commitTree(proj, nil, seedMessage("migrate malformed source",
		opTrailer(reposetup.OpMigrateSeed),
		srcTrailer("not-a-valid-object-id"), // never handed to gitcli
		copyTrailer(proj),
		repairTrailer("deadbeef")))

	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootForeign {
		t.Fatalf("Shape = %v, want RootForeign for a malformed source revision (err=%v)", own.Shape, own.Err)
	}
}

// Duplicate, unknown-version, and prune receipts on the root are all foreign —
// docket-claiming but invalid, and NEVER downgraded to the legacy path (which,
// on this empty-tree root, would otherwise read as proofLegacyEmpty).
func TestIntegrationRepoOwnershipInvalidReceiptsAreForeign(t *testing.T) {
	cases := []struct {
		name     string
		trailers []string
	}{
		{"duplicate-operation", []string{opTrailer(reposetup.OpInitRoot), opTrailer(reposetup.OpInitRoot)}},
		{"unknown-version", []string{opTrailer("repository-init-root/v2")}},
		{"prune-receipt", []string{opTrailer(reposetup.OpMigratePrune)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newOwnFixture(t)
			et := f.emptyTree()
			root := f.commitTree(et, nil, seedMessage("docket-claiming but invalid", tc.trailers...))

			own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(f.mainTip()))
			if own.Shape != reposetup.RootForeign {
				t.Fatalf("Shape = %v, want RootForeign for %s over an empty tree (never legacy-downgraded; err=%v)", own.Shape, tc.name, own.Err)
			}
			if own.Proof == proofLegacyEmpty {
				t.Fatalf("Proof = proofLegacyEmpty: an invalid receipt must not fall through to the legacy path")
			}
		})
	}
}

// --- Unknown mapping (never foreign) -----------------------------------------

func TestIntegrationRepoOwnershipUnreadableTipIsUnknown(t *testing.T) {
	f := newOwnFixture(t)
	bogus := gitcli.ObjectID(strings.Repeat("a", 40)) // well-formed but absent

	own := f.verify(t, bogus, gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootUnknown {
		t.Fatalf("Shape = %v, want RootUnknown for an absent tip object", own.Shape)
	}
	if own.Shape == reposetup.RootForeign {
		t.Fatal("an unreadable tip was collapsed into RootForeign")
	}
	if own.Err == nil {
		t.Error("Err is nil, want the retained probe error")
	}
}

func TestIntegrationRepoOwnershipUnknownIntegrationTipIsUnknown(t *testing.T) {
	f := newOwnFixture(t)
	et := f.emptyTree()
	root := f.commitTree(et, nil, seedMessage("docket init", opTrailer(reposetup.OpInitRoot)))

	own := f.verify(t, gitcli.ObjectID(root), "")
	if own.Shape != reposetup.RootUnknown {
		t.Fatalf("Shape = %v, want RootUnknown when disjointness is unprovable", own.Shape)
	}
	if own.Shape == reposetup.RootForeign {
		t.Fatal("an unprovable-disjointness case was collapsed into RootForeign")
	}
	if own.Err == nil {
		t.Error("Err is nil, want the retained diagnostic")
	}
}

// --- Task 4: receiptless legacy exact-tree equivalence ------------------------
//
// A receiptless NONEMPTY orphan root is proven (or refused) by exact-tree
// equivalence against the historical integration snapshots: its whole tree must
// equal the copy-set projection ({changes, adrs, specs}) of some reachable,
// live-surface-bearing snapshot, resolved through that snapshot's OWN committed
// directory configuration. These fixtures build a real integration history on
// main and compose the metadata root's tree from a snapshot's projection so the
// content-identity comparison is exercised end to end.

// defaultCopyPrefixes are the copy-set prefixes for a default-config snapshot,
// in the order the verifier composes them. changes/adrs default per config;
// specs is the fixed convention path (reposetup.SpecsDir).
var defaultCopyPrefixes = []string{"docs/changes", "docs/adrs", reposetup.SpecsDir}

// commitOnMain writes files into the working tree and commits them onto the
// integration branch (main), returning the new tip OID.
func (f *ownFixture) commitOnMain(files map[string]string, message string) string {
	f.t.Helper()
	for rel, content := range files {
		writeRepoFile(f.t, f.dir, rel, content)
	}
	runGit(f.t, f.dir, "add", "-A")
	runGit(f.t, f.dir, "commit", "-q", "-m", message)
	return f.mainTip()
}

// treeExists reports whether commit^{tree}:path resolves (a present tree or blob).
func (f *ownFixture) treeExists(commit, path string) bool {
	f.t.Helper()
	_, err := tryGit(f.dir, "rev-parse", "--verify", "-q", commit+"^{tree}:"+path)
	return err == nil
}

// projectTree composes the copy-set projection of commit exactly as the verifier
// (and migrateExecute) do: for each existing prefix, read-tree --prefix mounts
// the source subtree onto a private index, then write-tree yields the tree OID.
// An absent prefix is skipped, never an error — matching BuildTree's caller.
func (f *ownFixture) projectTree(commit string, prefixes []string) string {
	f.t.Helper()
	idx := filepath.Join(f.t.TempDir(), "index")
	env := []string{"GIT_INDEX_FILE=" + idx}
	for _, p := range prefixes {
		if !f.treeExists(commit, p) {
			continue
		}
		f.gitEnv(env, "read-tree", "--prefix="+p+"/", commit+"^{tree}:"+p)
	}
	return f.gitEnv(env, "write-tree")
}

// treePlusFile returns baseTree with one extra blob added at rel (mode 100644).
func (f *ownFixture) treePlusFile(baseTree, rel, content string) string {
	f.t.Helper()
	idx := filepath.Join(f.t.TempDir(), "index")
	env := []string{"GIT_INDEX_FILE=" + idx}
	f.gitEnv(env, "read-tree", baseTree)
	blob := f.gitStdin(content, "hash-object", "-w", "-t", "blob", "--stdin")
	f.gitEnv(env, "update-index", "--add", "--cacheinfo", "100644", blob, rel)
	return f.gitEnv(env, "write-tree")
}

// treeChmod returns baseTree with the blob at rel re-staged at a different mode,
// its content (blob OID) unchanged.
func (f *ownFixture) treeChmod(baseTree, rel, mode string) string {
	f.t.Helper()
	idx := filepath.Join(f.t.TempDir(), "index")
	env := []string{"GIT_INDEX_FILE=" + idx}
	f.gitEnv(env, "read-tree", baseTree)
	blob := f.gitEnv(env, "rev-parse", baseTree+":"+rel)
	f.gitEnv(env, "update-index", "--add", "--cacheinfo", mode, blob, rel)
	return f.gitEnv(env, "write-tree")
}

// deleteLooseObject removes one loose object from the store, forcing a read of it
// to fail (used to simulate truncated/unreadable history mid-search).
func (f *ownFixture) deleteLooseObject(oid string) {
	f.t.Helper()
	p := filepath.Join(f.dir, ".git", "objects", oid[:2], oid[2:])
	if err := os.Remove(p); err != nil {
		f.t.Fatalf("removing loose object %s: %v", oid, err)
	}
}

// A receiptless nonempty root whose tree equals the CURRENT integration tip's
// copy-set projection: proven legacy-equivalent, source == that tip.
func TestIntegrationRepoOwnershipLegacyMatchesCurrentTip(t *testing.T) {
	f := newOwnFixture(t)
	tip := f.commitOnMain(map[string]string{
		"docs/changes/active/0001-a.md": "a\n",
		"docs/adrs/0001-x.md":           "adr\n",
		reposetup.SpecsDir + "/foo.md":  "spec\n",
	}, "populate the copy set")
	proj := f.projectTree(tip, defaultCopyPrefixes)
	root := f.commitTree(proj, nil, "legacy metadata seed") // receiptless, nonempty

	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(tip))
	if own.Shape != reposetup.RootParentless {
		t.Fatalf("Shape = %v, want RootParentless (err=%v)", own.Shape, own.Err)
	}
	if own.Proof != proofLegacyEquivalent {
		t.Errorf("Proof = %d, want proofLegacyEquivalent", own.Proof)
	}
	if own.SourceRevision != tip {
		t.Errorf("SourceRevision = %s, want the current tip %s", own.SourceRevision, tip)
	}
}

// The live case's shape: the metadata root matches an OLDER snapshot whose live
// surface was later pruned from the current tip. Descendants on the metadata
// branch do not change the verdict.
func TestIntegrationRepoOwnershipLegacyMatchesOlderSnapshotAfterPrune(t *testing.T) {
	f := newOwnFixture(t)
	snapOld := f.commitOnMain(map[string]string{
		"docs/changes/active/0001-a.md": "a\n",
		"docs/adrs/0001-x.md":           "adr\n",
		reposetup.SpecsDir + "/foo.md":  "spec\n",
	}, "legacy planning surface")
	proj := f.projectTree(snapOld, defaultCopyPrefixes)

	// Advance integration past a prune of the live planning surface.
	runGit(t, f.dir, "rm", "-q", "-r", "docs/changes/active")
	runGit(t, f.dir, "commit", "-q", "-m", "prune legacy planning surface")
	prunedTip := f.mainTip()
	if f.treeExists(prunedTip, "docs/changes/active") {
		t.Fatal("fixture error: active/ still present on the pruned tip")
	}

	root := f.commitTree(proj, nil, "legacy metadata seed")
	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(prunedTip))
	if own.Shape != reposetup.RootParentless || own.Proof != proofLegacyEquivalent {
		t.Fatalf("got Shape=%v Proof=%d, want RootParentless/proofLegacyEquivalent (err=%v)", own.Shape, own.Proof, own.Err)
	}
	if own.SourceRevision != snapOld {
		t.Errorf("SourceRevision = %s, want the older snapshot %s", own.SourceRevision, snapOld)
	}

	// Descendants on the metadata branch: still the same verified root.
	metaTip := f.commitTree(f.mktree(map[string]string{"docs/changes/active/0009-z.md": "z\n"}), []string{root}, "metadata commit")
	own2 := f.verify(t, gitcli.ObjectID(metaTip), gitcli.ObjectID(prunedTip))
	if own2.Shape != reposetup.RootParentless || own2.Proof != proofLegacyEquivalent {
		t.Fatalf("with descendants: Shape=%v Proof=%d, want RootParentless/proofLegacyEquivalent (err=%v)", own2.Shape, own2.Proof, own2.Err)
	}
	if own2.Root != gitcli.ObjectID(root) {
		t.Errorf("Root = %s, want the legacy seed root %s", own2.Root, root)
	}
}

// Historical NONDEFAULT directory config decides the copy set: the old snapshot
// keeps changes under planning/changes/, and the current committed .docket.yml
// (plus a repository-local .docket.local.yml) naming a different changes_dir must
// not redefine historical evidence.
func TestIntegrationRepoOwnershipLegacyHistoricalNondefaultDirs(t *testing.T) {
	f := newOwnFixture(t)
	snapOld := f.commitOnMain(map[string]string{
		".docket.yml":                       "changes_dir: planning/changes\n",
		"planning/changes/active/0001-a.md": "a\n",
		"docs/adrs/0001-x.md":               "adr\n",
		reposetup.SpecsDir + "/foo.md":      "spec\n",
	}, "legacy surface under a nondefault changes_dir")
	proj := f.projectTree(snapOld, []string{"planning/changes", "docs/adrs", reposetup.SpecsDir})

	// The current tip's config, and a repository-local override, say something
	// else. Neither may leak into the historical resolution.
	tip := f.commitOnMain(map[string]string{
		".docket.yml":       "changes_dir: docs/changes\n",
		".docket.local.yml": "changes_dir: elsewhere/changes\n",
	}, "current config names a different changes_dir")

	root := f.commitTree(proj, nil, "legacy metadata seed")
	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(tip))
	if own.Shape != reposetup.RootParentless || own.Proof != proofLegacyEquivalent {
		t.Fatalf("got Shape=%v Proof=%d, want RootParentless/proofLegacyEquivalent — historical config must decide (err=%v)", own.Shape, own.Proof, own.Err)
	}
	if own.SourceRevision != snapOld {
		t.Errorf("SourceRevision = %s, want the nondefault snapshot %s", own.SourceRevision, snapOld)
	}
}

// A snapshot carrying the obsolete metadata_branch tombstone key resolves (a
// warning, never an error) and stays eligible.
func TestIntegrationRepoOwnershipLegacyMetadataBranchKeyResolves(t *testing.T) {
	f := newOwnFixture(t)
	tip := f.commitOnMain(map[string]string{
		".docket.yml":                   "metadata_branch: docket\n",
		"docs/changes/active/0001-a.md": "a\n",
		"docs/adrs/0001-x.md":           "adr\n",
		reposetup.SpecsDir + "/foo.md":  "spec\n",
	}, "legacy surface with an obsolete metadata_branch key")
	proj := f.projectTree(tip, defaultCopyPrefixes)
	root := f.commitTree(proj, nil, "legacy metadata seed")

	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(tip))
	if own.Shape != reposetup.RootParentless || own.Proof != proofLegacyEquivalent {
		t.Fatalf("got Shape=%v Proof=%d, want RootParentless/proofLegacyEquivalent (err=%v)", own.Shape, own.Proof, own.Err)
	}
}

// --- Task 4 negatives: readable history, exhausted, no exact match → Foreign --

// The Task 3 deferred negative: a receiptless nonempty root with no historical
// match at all (no snapshot ever carried the live surface) is Foreign — a
// plausible commit subject rescues nothing.
func TestIntegrationRepoOwnershipLegacyNoHistoricalMatch(t *testing.T) {
	f := newOwnFixture(t) // main carries only README: no live planning surface
	root := f.commitTree(f.mktree(map[string]string{"docs/changes/active/0001-a.md": "a\n"}), nil, "docket metadata seed")

	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootForeign {
		t.Fatalf("Shape = %v, want RootForeign for a receiptless root with no historical match (err=%v)", own.Shape, own.Err)
	}
}

// One extra file outside the copy set: the root is the projection PLUS a top-level
// path, so a subset-tolerant equality would wrongly pass. Exact identity refuses.
func TestIntegrationRepoOwnershipLegacyExtraFile(t *testing.T) {
	f := newOwnFixture(t)
	tip := f.commitOnMain(map[string]string{
		"docs/changes/active/0001-a.md": "a\n",
		"docs/adrs/0001-x.md":           "adr\n",
		reposetup.SpecsDir + "/foo.md":  "spec\n",
	}, "populate the copy set")
	proj := f.projectTree(tip, defaultCopyPrefixes)
	root := f.commitTree(f.treePlusFile(proj, "EXTRA.md", "extra\n"), nil, "legacy seed + extra")

	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(tip))
	if own.Shape != reposetup.RootForeign {
		t.Fatalf("Shape = %v, want RootForeign for a root with an extra path outside the copy set (err=%v)", own.Shape, own.Err)
	}
}

// A missing copied path: the root omits one copy-set file the snapshot carries.
func TestIntegrationRepoOwnershipLegacyMissingFile(t *testing.T) {
	f := newOwnFixture(t)
	tip := f.commitOnMain(map[string]string{
		"docs/changes/active/0001-a.md": "a\n",
		"docs/adrs/0001-x.md":           "adr\n",
		reposetup.SpecsDir + "/foo.md":  "spec\n",
	}, "populate the copy set")
	// The root's tree omits docs/adrs entirely.
	root := f.commitTree(f.mktree(map[string]string{
		"docs/changes/active/0001-a.md": "a\n",
		reposetup.SpecsDir + "/foo.md":  "spec\n",
	}), nil, "legacy seed missing adrs")

	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(tip))
	if own.Shape != reposetup.RootForeign {
		t.Fatalf("Shape = %v, want RootForeign for a root missing a copied path (err=%v)", own.Shape, own.Err)
	}
}

// One changed byte in a copied blob: content identity refuses.
func TestIntegrationRepoOwnershipLegacyChangedByte(t *testing.T) {
	f := newOwnFixture(t)
	tip := f.commitOnMain(map[string]string{
		"docs/changes/active/0001-a.md": "a\n",
		"docs/adrs/0001-x.md":           "adr\n",
		reposetup.SpecsDir + "/foo.md":  "spec\n",
	}, "populate the copy set")
	root := f.commitTree(f.mktree(map[string]string{
		"docs/changes/active/0001-a.md": "a\n",
		"docs/adrs/0001-x.md":           "adr CHANGED\n", // one blob differs
		reposetup.SpecsDir + "/foo.md":  "spec\n",
	}), nil, "legacy seed changed byte")

	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(tip))
	if own.Shape != reposetup.RootForeign {
		t.Fatalf("Shape = %v, want RootForeign for a root with a changed byte (err=%v)", own.Shape, own.Err)
	}
}

// One changed mode (100755 vs 100644) with identical content: mode identity refuses.
func TestIntegrationRepoOwnershipLegacyChangedMode(t *testing.T) {
	f := newOwnFixture(t)
	tip := f.commitOnMain(map[string]string{
		"docs/changes/active/0001-a.md": "a\n",
		"docs/adrs/0001-x.md":           "adr\n",
		reposetup.SpecsDir + "/foo.md":  "spec\n",
	}, "populate the copy set")
	proj := f.projectTree(tip, defaultCopyPrefixes)
	root := f.commitTree(f.treeChmod(proj, "docs/adrs/0001-x.md", "100755"), nil, "legacy seed changed mode")

	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(tip))
	if own.Shape != reposetup.RootForeign {
		t.Fatalf("Shape = %v, want RootForeign for a root with a changed mode (err=%v)", own.Shape, own.Err)
	}
}

// A candidate read that errors mid-search maps to Unknown, never Foreign: the
// sole eligible snapshot's tree object is removed, so reading it fails.
func TestIntegrationRepoOwnershipLegacyUnreadableHistoryIsUnknown(t *testing.T) {
	f := newOwnFixture(t)
	tip := f.commitOnMain(map[string]string{
		"docs/changes/active/0001-a.md": "a\n",
		"docs/adrs/0001-x.md":           "adr\n",
		reposetup.SpecsDir + "/foo.md":  "spec\n",
	}, "populate the copy set")
	// A receiptless nonempty root that does NOT match the tip's projection, so
	// the search must actually read the (soon-unreadable) candidate.
	root := f.commitTree(f.mktree(map[string]string{"docs/changes/active/zzz.md": "different\n"}), nil, "legacy seed")

	// Remove the tip snapshot's tree object. Its COMMIT object stays, so the
	// commit graph (RootCommits, merge-base, rev-list) is still walkable, but any
	// read of that snapshot's tree content fails mid-search.
	tipTree := runGit(t, f.dir, "rev-parse", tip+"^{tree}")
	f.deleteLooseObject(tipTree)

	own := f.verify(t, gitcli.ObjectID(root), gitcli.ObjectID(tip))
	if own.Shape != reposetup.RootUnknown {
		t.Fatalf("Shape = %v, want RootUnknown when a candidate read errors mid-search (err=%v)", own.Shape, own.Err)
	}
	if own.Shape == reposetup.RootForeign {
		t.Fatal("a truncated/unreadable history was collapsed into RootForeign")
	}
	if own.Err == nil {
		t.Error("Err is nil, want the retained probe error")
	}
}
