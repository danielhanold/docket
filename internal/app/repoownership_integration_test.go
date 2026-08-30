//go:build integration

package app

import (
	"context"
	"errors"
	"fmt"
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

// --- Root-anchored trailer read: a descendant's receipt never authorizes -------
//
// Receipt trailers are read from the ROOT COMMIT ITSELF (verifyMetadataOwnership
// scans `own.Root`, the sole parentless root). These two fixtures place a VALID
// seed receipt on a DESCENDANT of a receiptless root and assert the verdict stays
// driven only by the root — never adopted through the descendant. They redden if
// a refactor ever re-anchored the trailer scan (or its match) to the tip: the
// descendant's receipt would then be read as the root's proof.

// A receiptless empty-tree legacy root with a descendant carrying a valid
// OpInitRoot receipt stays proofLegacyEmpty (the root's own proof), NEVER
// proofInitReceipt adopted from the descendant. Under a scan-anchored-at-tip
// mutation the descendant's OpInitRoot would flip Proof to proofInitReceipt.
func TestIntegrationRepoOwnershipDescendantInitReceiptCannotAuthorizeLegacyRoot(t *testing.T) {
	f := newOwnFixture(t)
	et := f.emptyTree()
	root := f.commitTree(et, nil, "legacy bootstrap seed") // receiptless empty root
	tip := f.commitTree(
		f.mktree(map[string]string{"docs/changes/active/0001-a.md": "a\n"}),
		[]string{root},
		seedMessage("docket init on a descendant", opTrailer(reposetup.OpInitRoot)))

	own := f.verify(t, gitcli.ObjectID(tip), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootParentless {
		t.Fatalf("Shape = %v, want RootParentless from the root's own legacy proof (err=%v)", own.Shape, own.Err)
	}
	if own.Proof != proofLegacyEmpty {
		t.Fatalf("Proof = %d, want proofLegacyEmpty (the root's own proof)", own.Proof)
	}
	if own.Proof == proofInitReceipt || own.Proof == proofMigrateReceipt {
		t.Fatal("a descendant's receipt was adopted as the root's proof: the trailer scan is not root-anchored")
	}
	if own.Root != gitcli.ObjectID(root) {
		t.Errorf("Root = %s, want the parentless root %s", own.Root, root)
	}
}

// A receiptless NONEMPTY root with no historical match is RootForeign; a valid
// OpMigrateSeed receipt on a descendant — even one whose CopyDigest equals the
// ROOT's tree and whose source is reachable — must NOT rescue it. Under a
// scan-anchored-at-tip mutation the descendant's receipt would be read against
// the root's tree and flip the verdict to RootParentless/proofMigrateReceipt.
func TestIntegrationRepoOwnershipDescendantMigrateReceiptCannotAuthorizeForeignRoot(t *testing.T) {
	f := newOwnFixture(t) // main carries only README: no live planning surface, so no historical match
	proj := f.mktree(map[string]string{"docs/changes/active/0001-a.md": "a\n"})
	root := f.commitTree(proj, nil, "receiptless foreign seed") // receiptless nonempty root; rootTree == proj
	tip := f.commitTree(
		f.mktree(map[string]string{"docs/changes/active/0002-b.md": "b\n"}),
		[]string{root},
		seedMessage("docket migrate seed on a descendant",
			opTrailer(reposetup.OpMigrateSeed),
			srcTrailer(f.mainTip()), // reachable from the integration tip
			copyTrailer(proj),       // == the ROOT tree, so a tip-anchored read would validate
			repairTrailer("deadbeef")))

	own := f.verify(t, gitcli.ObjectID(tip), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootForeign {
		t.Fatalf("Shape = %v, want RootForeign for a receiptless root with no historical match (err=%v)", own.Shape, own.Err)
	}
	if own.Proof == proofMigrateReceipt || own.Proof == proofInitReceipt {
		t.Fatal("a descendant's receipt was adopted as the root's proof: the trailer scan is not root-anchored")
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

// --- Check-level scenarios: augmentCheckFacts consumes the verifier -----------
//
// These drive the read-only `repository check` service end to end against the
// real bare-upstream + clone fixture (newHealthyRepo, from
// repocheck_integration_test.go). They live in this shard (prefix
// TestIntegrationRepoOwnership) rather than the RepoCheck shard because that
// shard's runtime-budget row has little headroom. They prove the defect fix at
// the check boundary: a multi-commit verified metadata branch classifies healthy
// with no metadata-root-foreign, while genuinely foreign lineages still do.

// pushMetadataCommits adds n content-changing metadata commits (new active-change
// records, no seed receipt on any of them) to the .docket worktree and pushes
// them to origin. The .docket worktree HEAD is the docket branch, so both the
// local docket ref and the remote docket tip advance in lockstep and stay
// synchronized; the seed's root tree contents are not retained at the new tip.
func (r *initRepo) pushMetadataCommits(t *testing.T, n int) {
	t.Helper()
	dotDocket := filepath.Join(r.invocation, ".docket")
	for i := 1; i <= n; i++ {
		slug := fmt.Sprintf("meta%d", i)
		rel := fmt.Sprintf("docs/changes/active/%04d-%s.md", i, slug)
		writeRepoFile(t, dotDocket, rel, changeRecord(i, slug, fmt.Sprintf("Meta %d", i)))
		runGit(t, dotDocket, "add", "-A")
		runGit(t, dotDocket, "commit", "-q", "-m", fmt.Sprintf("meta commit %d", i))
	}
	runGit(t, dotDocket, "push", "-q", "origin", "docket")
}

// findingWithCode reports whether the check result carries a finding of the given
// code (e.g. the metadata-root-foreign conflict finding).
func findingWithCode(res RepositoryCheckResult, code string) bool {
	for _, f := range res.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

// TestIntegrationRepoOwnershipCheckHealthyMultiCommit is the defect's headline
// check-level test: a healthy repository whose metadata branch has grown ordinary
// content-changing metadata commits (so its root no longer equals its tip) still
// classifies healthy — the old root-equals-tip predicate reported it foreign.
func TestIntegrationRepoOwnershipCheckHealthyMultiCommit(t *testing.T) {
	r := newHealthyRepo(t)
	r.pushMetadataCommits(t, 3)

	res := r.runCheck(t)
	if res.RepositoryState != string(reposetup.StateHealthy) {
		t.Errorf("RepositoryState = %q (%s), want healthy (a multi-commit verified metadata branch is not foreign)", res.RepositoryState, res.HumanText())
	}
	if findingWithCode(res, "metadata-root-foreign") {
		t.Errorf("check reported metadata-root-foreign on a verified multi-commit branch:\n%s", res.HumanText())
	}
	if code := res.CheckExitCode(); code != 0 {
		t.Errorf("exit = %d (%s), want 0", code, res.HumanText())
	}
}

// TestIntegrationRepoOwnershipCheckDescendantsPreserveFrontmatterFindings proves
// the ownership fix silences nothing else: a metadata branch with descendants AND
// a deliberately broken frontmatter record still surfaces that frontmatter
// finding (independent corpus validation runs regardless of the root proof).
func TestIntegrationRepoOwnershipCheckDescendantsPreserveFrontmatterFindings(t *testing.T) {
	r := newHealthyRepo(t)
	r.pushMetadataCommits(t, 2)

	dotDocket := filepath.Join(r.invocation, ".docket")
	broken := "docs/changes/active/0099-broken.md"
	// A malformed record: missing required frontmatter fields (only id present),
	// which the corpus validator reports as a finding referencing this path.
	writeRepoFile(t, dotDocket, broken, "---\nid: 99\n---\n\nbroken record\n")
	runGit(t, dotDocket, "add", "-A")
	runGit(t, dotDocket, "commit", "-q", "-m", "broken frontmatter record")
	runGit(t, dotDocket, "push", "-q", "origin", "docket")

	res := r.runCheck(t)
	if findingWithCode(res, "metadata-root-foreign") {
		t.Errorf("check reported metadata-root-foreign on a verified multi-commit branch:\n%s", res.HumanText())
	}
	found := false
	for _, f := range res.Findings {
		if strings.Contains(f.Ref, "0099-broken.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("no frontmatter finding referencing the broken record; findings = %+v", res.Findings)
	}
}

// TestIntegrationRepoOwnershipCheckForeignNonemptyRoot proves a single-root
// nonempty metadata branch that carries no seed receipt and matches no historical
// snapshot is still reported foreign at the check boundary.
func TestIntegrationRepoOwnershipCheckForeignNonemptyRoot(t *testing.T) {
	r := newHealthyRepo(t)
	// Replace origin/docket with a disjoint parentless NONEMPTY commit (built on an
	// orphan branch in the writer clone) carrying no receipt.
	runGit(t, r.writer, "checkout", "-q", "--orphan", "foreign-docket")
	runGit(t, r.writer, "rm", "-rf", "-q", ".")
	writeRepoFile(t, r.writer, "foreign.txt", "not a docket seed\n")
	runGit(t, r.writer, "add", "-A")
	runGit(t, r.writer, "commit", "-q", "-m", "foreign nonempty root")
	runGit(t, r.writer, "push", "-q", "-f", "origin", "foreign-docket:docket")

	res := r.runCheck(t)
	if !findingWithCode(res, "metadata-root-foreign") {
		t.Errorf("foreign nonempty root not reported foreign; state=%q findings=%+v", res.RepositoryState, res.Findings)
	}
}

// TestIntegrationRepoOwnershipCheckForeignSharedAncestry proves a metadata branch
// created FROM the integration branch (shared ancestry) — even carrying
// docket-looking files — is reported foreign at the check boundary.
func TestIntegrationRepoOwnershipCheckForeignSharedAncestry(t *testing.T) {
	r := newHealthyRepo(t)
	runGit(t, r.writer, "fetch", "-q", "origin", "main")
	runGit(t, r.writer, "checkout", "-q", "-B", "shared-docket", "origin/main")
	writeRepoFile(t, r.writer, "docs/changes/active/0001-x.md", changeRecord(1, "x", "X"))
	runGit(t, r.writer, "add", "-A")
	runGit(t, r.writer, "commit", "-q", "-m", "docket branched from integration")
	runGit(t, r.writer, "push", "-q", "-f", "origin", "shared-docket:docket")

	res := r.runCheck(t)
	if !findingWithCode(res, "metadata-root-foreign") {
		t.Errorf("shared-ancestry metadata branch not reported foreign; state=%q findings=%+v", res.RepositoryState, res.Findings)
	}
}

// TestIntegrationRepoOwnershipCheckFailedFetchIsUnknown proves a failed metadata
// fetch is unknown, never foreign: augmentCheckFacts leaves f.MetadataRoot at
// RootUnknown (even if an older object were available locally) and the classifier
// does not report metadata-root-foreign. augmentCheckFacts is driven directly
// with a setupContext whose remote has no docket branch to fetch.
func TestIntegrationRepoOwnershipCheckFailedFetchIsUnknown(t *testing.T) {
	r := newInitRepo(t, healthySetupYML, nil)
	mainTip := runGit(t, r.invocation, "rev-parse", "origin/main")

	f := reposetup.Facts{}
	f.RemoteMetadata.Presence = reposetup.PresencePresent
	f.RemoteMetadata.Tip = mainTip // an ls-remote-observed OID; the fetch below still fails
	sc := setupContext{
		repo:              gitcli.Repository{PrimaryWorktree: r.invocation},
		defaultBranch:     "main",
		integrationBranch: "main",
		sourceRevision:    mainTip,
		metadataTip:       mainTip,
	}
	augmentCheckFacts(context.Background(), newGitClient(t), &f, sc)

	if f.MetadataRoot != reposetup.RootUnknown {
		t.Errorf("MetadataRoot = %v, want RootUnknown after a failed metadata fetch", f.MetadataRoot)
	}
	if f.MetadataRoot == reposetup.RootForeign {
		t.Fatal("a failed fetch was collapsed into RootForeign")
	}
	cls := reposetup.Classify(f)
	if contains(cls.Reasons, "metadata-root-foreign") {
		t.Errorf("classifier reported metadata-root-foreign after a failed fetch; reasons=%v", cls.Reasons)
	}
}

// TestIntegrationRepoOwnershipCheckIsReadOnly proves check on a multi-commit
// metadata branch performs no observable write apart from the permitted
// refs/remotes/* advance and fetched objects.
func TestIntegrationRepoOwnershipCheckIsReadOnly(t *testing.T) {
	r := newHealthyRepo(t)
	r.pushMetadataCommits(t, 3)

	before := r.readOnlySnapshot(t)
	r.runCheck(t)
	after := r.readOnlySnapshot(t)
	if before != after {
		t.Errorf("check wrote observable state:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// --- Init-level scenarios: publishOrAdoptMetadataRoot consumes the verifier -----
//
// These drive RunRepositoryInit end to end against the bare-upstream + clone
// fixture (initRepo, from reposetup_integration_test.go), with the remote docket
// branch pre-published as a precisely-shaped lineage crafted through plumbing in
// the writer clone (reusing the ownFixture helpers bound to r.writer). They prove
// the race-loser arm: on a lost create-only lease, init adopts a verified
// INIT-EQUIVALENT lineage AT ITS REREAD TIP (descendants preserved, never reset
// to the seed, never re-pushed), refuses a migration-seeded or foreign or
// unreadable branch, and never overwrites the remote. The create-only
// TestIntegrationRepoSetupInitRefusesForeignMetadataBranch (single-commit foreign,
// shared ancestry) stays in the reposetup shard; this shard adds a multi-commit
// disjoint foreign lineage.

// publishCraftedDocket force-publishes commit (a tip built in the writer clone via
// plumbing) to the origin docket branch and returns the resulting remote tip OID.
func (r *initRepo) publishCraftedDocket(t *testing.T, commit string) string {
	t.Helper()
	runGit(t, r.writer, "update-ref", "refs/heads/docket", commit)
	runGit(t, r.writer, "push", "-q", "-f", "origin", "docket")
	return runGit(t, r.origin, "rev-parse", "refs/heads/docket")
}

// TestIntegrationRepoOwnershipInitAdoptsInitLineageWithDescendants is the race
// arm's headline: a genuine init seed (empty-tree OpInitRoot root) that has ALREADY
// grown descendants on the remote is adopted at its multi-commit tip, not reset to
// the seed. Init loses the create-only lease, verifies ownership at the reread tip,
// finds it init-equivalent, and adopts it — the remote is left byte-untouched.
func TestIntegrationRepoOwnershipInitAdoptsInitLineageWithDescendants(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	wf := &ownFixture{t: t, dir: r.writer}
	root := wf.commitTree(wf.emptyTree(), nil, seedMessage("docket init", opTrailer(reposetup.OpInitRoot)))
	c1 := wf.commitTree(wf.mktree(map[string]string{"docs/changes/active/0001-a.md": "a\n"}), []string{root}, "meta 1")
	tip := wf.commitTree(wf.mktree(map[string]string{"docs/changes/active/0002-b.md": "b\n"}), []string{c1}, "meta 2")
	before := r.publishCraftedDocket(t, tip)

	res := r.runInit(t)

	if res.Result == ResultInvalidState {
		t.Fatalf("init refused a verified init lineage: %q (%s)", res.Result, res.HumanText())
	}
	if res.RepositoryState != string(reposetup.StateNeedsReview) {
		t.Errorf("RepositoryState = %q, want needs-review", res.RepositoryState)
	}
	if res.MetadataTip != before {
		t.Errorf("adopted MetadataTip = %s, want the multi-commit remote tip %s", res.MetadataTip, before)
	}
	if res.MetadataTip == root {
		t.Error("adopted the seed root, not the descendant tip: descendants were reset to seed")
	}
	if after := runGit(t, r.origin, "rev-parse", "refs/heads/docket"); after != before {
		t.Errorf("remote docket tip moved from %s to %s; a race adoption must never re-push or reset to seed", before, after)
	}
}

// TestIntegrationRepoOwnershipInitAdoptsLegacyEmptyBootstrapWithDescendants proves
// the receiptless empty-root legacy bootstrap lineage is equally init-equivalent:
// an empty-tree root with NO receipt plus descendants is adopted at its tip.
func TestIntegrationRepoOwnershipInitAdoptsLegacyEmptyBootstrapWithDescendants(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	wf := &ownFixture{t: t, dir: r.writer}
	root := wf.commitTree(wf.emptyTree(), nil, "legacy bootstrap seed") // no receipt trailer
	tip := wf.commitTree(wf.mktree(map[string]string{"docs/changes/active/0001-a.md": "a\n"}), []string{root}, "first metadata commit")
	before := r.publishCraftedDocket(t, tip)

	res := r.runInit(t)

	if res.Result == ResultInvalidState {
		t.Fatalf("init refused a receiptless empty-bootstrap lineage: %q (%s)", res.Result, res.HumanText())
	}
	if res.MetadataTip != before {
		t.Errorf("adopted MetadataTip = %s, want the multi-commit remote tip %s", res.MetadataTip, before)
	}
	if res.MetadataTip == root {
		t.Error("adopted the seed root, not the descendant tip: descendants were reset to seed")
	}
	if after := runGit(t, r.origin, "rev-parse", "refs/heads/docket"); after != before {
		t.Errorf("remote docket tip moved from %s to %s; adoption must never re-push", before, after)
	}
}

// TestIntegrationRepoOwnershipInitRefusesMigrationSeededBranch proves a broadened
// ownership proof never becomes permission to initialize: a verified but
// MIGRATION-seeded lineage (OpMigrateSeed root + descendants) is recognized yet
// refused — init cannot adopt an established migrated branch. The remote and every
// local surface are left untouched.
func TestIntegrationRepoOwnershipInitRefusesMigrationSeededBranch(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	wf := &ownFixture{t: t, dir: r.writer}
	proj := wf.mktree(map[string]string{
		"docs/changes/active/0001-a.md": "a\n",
		"docs/adrs/0001-x.md":           "adr\n",
	})
	src := runGit(t, r.origin, "rev-parse", "refs/heads/main") // reachable integration tip
	root := wf.commitTree(proj, nil, seedMessage("docket migrate seed",
		opTrailer(reposetup.OpMigrateSeed),
		srcTrailer(src),
		copyTrailer(proj),
		repairTrailer("deadbeef")))
	tip := wf.commitTree(wf.mktree(map[string]string{"docs/changes/active/0002-b.md": "b\n"}), []string{root}, "meta after migrate")
	before := r.publishCraftedDocket(t, tip)
	snapBefore := r.readOnlySnapshot(t)

	res := r.runInit(t)

	if res.Result != ResultInvalidState {
		t.Fatalf("Result = %q (%s), want invalid-state for a migration-seeded branch", res.Result, res.HumanText())
	}
	if res.RepositoryState != string(reposetup.StateConflict) {
		t.Errorf("RepositoryState = %q, want conflict", res.RepositoryState)
	}
	if !strings.Contains(res.HumanText(), "established migrated metadata branch") {
		t.Errorf("remedy %q must name the established migrated metadata branch", res.HumanText())
	}
	if after := runGit(t, r.origin, "rev-parse", "refs/heads/docket"); after != before {
		t.Errorf("migration-seeded refusal moved the remote docket tip from %s to %s; it must be untouched", before, after)
	}
	if snapAfter := r.readOnlySnapshot(t); snapAfter != snapBefore {
		t.Errorf("a refusal wrote observable local state:\nbefore:\n%s\nafter:\n%s", snapBefore, snapAfter)
	}
}

// TestIntegrationRepoOwnershipInitRefusesMultiCommitForeignBranch adds the
// multi-commit disjoint foreign case the create-only single-commit test does not
// cover: a nonempty orphan root with no valid receipt and no historical match,
// plus a descendant, is refused as not a verified docket metadata branch. Remote
// and local state are preserved.
func TestIntegrationRepoOwnershipInitRefusesMultiCommitForeignBranch(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	wf := &ownFixture{t: t, dir: r.writer}
	root := wf.commitTree(wf.mktree(map[string]string{"foreign.txt": "not docket's\n"}), nil, "foreign root")
	tip := wf.commitTree(wf.mktree(map[string]string{"foreign2.txt": "more\n"}), []string{root}, "foreign descendant")
	before := r.publishCraftedDocket(t, tip)
	snapBefore := r.readOnlySnapshot(t)

	res := r.runInit(t)

	if res.Result != ResultInvalidState {
		t.Fatalf("Result = %q (%s), want invalid-state for a foreign multi-commit branch", res.Result, res.HumanText())
	}
	if res.RepositoryState != string(reposetup.StateConflict) {
		t.Errorf("RepositoryState = %q, want conflict", res.RepositoryState)
	}
	if !strings.Contains(res.HumanText(), "not a verified docket metadata branch") {
		t.Errorf("remedy %q must say the branch is not a verified docket metadata branch", res.HumanText())
	}
	if after := runGit(t, r.origin, "rev-parse", "refs/heads/docket"); after != before {
		t.Errorf("foreign refusal moved the remote docket tip from %s to %s; create-only protection must leave it untouched", before, after)
	}
	if snapAfter := r.readOnlySnapshot(t); snapAfter != snapBefore {
		t.Errorf("a refusal wrote observable local state:\nbefore:\n%s\nafter:\n%s", snapBefore, snapAfter)
	}
}

// --- Migrate-level scenarios: migrateRoute + reconcileResumeSeed consume the verifier ---
//
// These drive RunRepositoryMigrate end to end against the legacy migration fixture
// (newInitRepo with legacyDocketYML + cleanLegacyFiles; runMigrate /
// runMigrateWithHooks and the origin* oracles from repomigration_integration_test.go).
// They prove migrate recognizes an ESTABLISHED migrated branch however many commits
// it has grown (no-op / local attachment at the ACTUAL latest tip), and — the
// data-loss boundary this change guards — that a seed replacement is refused once
// descendants exist while the integration surface still needs pruning, so a
// broadened ownership proof can never discard a descendant commit. Every
// refusal/contention case asserts the remote OIDs and local state are preserved.

// cloneFreshInvocation clones the bare origin into a new sibling working clone with
// its own identity and NO local .docket, returning its path — a second invocation
// against the same remote for the local-attachment recovery scenario.
func (r *initRepo) cloneFreshInvocation(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(r.root, name)
	runGit(t, r.root, "clone", "-q", r.origin, dir)
	gitIdentity(t, dir)
	return dir
}

// advanceDocketDescendant fetches the remote docket branch into the writer clone,
// commits one file onto it as a DESCENDANT of the current tip, pushes it, and
// returns the new remote docket tip. The branch keeps its established seed root and
// grows a child — exactly the multi-commit history the fix must keep recognizing
// and the descendants guard must refuse replacing.
func (r *initRepo) advanceDocketDescendant(t *testing.T, rel, content string) string {
	t.Helper()
	runGit(t, r.writer, "fetch", "-q", "origin", "docket")
	runGit(t, r.writer, "checkout", "-q", "-B", "docket", "origin/docket")
	writeRepoFile(t, r.writer, rel, content)
	runGit(t, r.writer, "add", "--", rel)
	runGit(t, r.writer, "commit", "-q", "-m", "descendant on docket: "+rel)
	runGit(t, r.writer, "push", "-q", "origin", "docket")
	return r.originTip(t, "docket")
}

// TestIntegrationRepoOwnershipMigrateNoOpAtLatestTip is the migrate defect's
// headline: a fully migrated repository whose metadata branch has grown ordinary
// metadata commits (its root no longer equals its tip) re-runs as a no-op — the
// pre-fix root-equals-tip predicate refused this as foreign and would have tried to
// re-seed, discarding the descendants.
func TestIntegrationRepoOwnershipMigrateNoOpAtLatestTip(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	if first := r.runMigrate(t, MigrateOptions{Authorized: true}); first.Result != ResultApplied {
		t.Fatalf("first migrate = %q (%s), want applied", first.Result, first.HumanText())
	}
	r.pushMetadataCommits(t, 4)
	metaTip := r.originTip(t, "docket")
	intTip := r.originTip(t, "main")

	res := r.runMigrate(t, MigrateOptions{Authorized: true})
	if res.Result != ResultNoOp {
		t.Fatalf("re-run over an established multi-commit migrated branch = %q (%s), want no-op", res.Result, res.HumanText())
	}
	if got := r.originTip(t, "docket"); got != metaTip {
		t.Errorf("no-op moved the metadata branch from %s to %s (a descendant was lost or the branch was reset to seed)", metaTip, got)
	}
	if got := r.originTip(t, "main"); got != intTip {
		t.Errorf("no-op moved the integration branch from %s to %s", intTip, got)
	}
}

// TestIntegrationRepoOwnershipMigrateLocalAttachmentAtLatestTip proves a fresh
// clone with no local .docket, run against an established multi-commit migrated
// remote, finishes ONLY the local attachment at the ACTUAL latest tip (not the seed
// root), leaving both remote branches byte-untouched.
func TestIntegrationRepoOwnershipMigrateLocalAttachmentAtLatestTip(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	if res := r.runMigrate(t, MigrateOptions{Authorized: true}); res.Result != ResultApplied {
		t.Fatalf("first migrate = %q (%s), want applied", res.Result, res.HumanText())
	}
	r.pushMetadataCommits(t, 4)
	metaTip := r.originTip(t, "docket")
	intTip := r.originTip(t, "main")

	fresh := r.cloneFreshInvocation(t, "invocation2")
	res := RunRepositoryMigrate(context.Background(), SetupDeps{Git: newGitClient(t), RepoDir: fresh}, MigrateOptions{Authorized: true})
	if res.Result != ResultApplied {
		t.Fatalf("local-attachment resume = %q (%s), want applied", res.Result, res.HumanText())
	}
	dotDocket := filepath.Join(fresh, ".docket")
	if head := runGit(t, dotDocket, "rev-parse", "HEAD"); head != metaTip {
		t.Errorf(".docket HEAD = %s, want the multi-commit remote tip %s (must attach at the latest tip, not the seed)", head, metaTip)
	}
	if got := r.originTip(t, "docket"); got != metaTip {
		t.Errorf("local attachment moved the metadata branch from %s to %s", metaTip, got)
	}
	if got := r.originTip(t, "main"); got != intTip {
		t.Errorf("local attachment moved the integration branch from %s to %s", intTip, got)
	}
}

// TestIntegrationRepoOwnershipMigrateDescendantsBlockSeedReplacement is the
// data-loss guard: a migration interrupted after the seed (integration still live),
// whose metadata branch then grows a descendant AND whose source has moved so a
// resume would want to replace the seed, is refused — the descendant is never
// discarded and nothing is pruned; a human must reconcile the partial migration.
func TestIntegrationRepoOwnershipMigrateDescendantsBlockSeedReplacement(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
		afterSeedPush: func() error { return errors.New("crash after seed") },
	})
	if !r.remoteBranchExists(t, "docket") {
		t.Fatal("the seed push did not land")
	}
	descendantTip := r.advanceDocketDescendant(t, "docs/changes/active/0009-descendant.md", migChangeRecord(9, "descendant", "proposed", ""))
	// Advance the legacy source so a resume WOULD want to replace the seed.
	r.advanceIntegration(t, "docs/adrs/0002-second-decision.md",
		"---\nid: 2\nslug: second-decision\nstatus: Accepted\ntitle: Second decision\n---\nContext.\n")
	intTip := r.originTip(t, "main")
	snapBefore := r.readOnlySnapshot(t)

	res := r.runMigrate(t, MigrateOptions{Authorized: true})
	if res.Result != ResultInvalidState || res.RepositoryState != "conflict" {
		t.Fatalf("resume over a branch with descendants = %q (%s), want invalid-state conflict", res.Result, res.HumanText())
	}
	if !strings.Contains(res.HumanText(), "partial migration must be reconciled by a human") {
		t.Errorf("refusal %q must name the human-reconcile partial-migration remedy", res.HumanText())
	}
	if got := r.originTip(t, "docket"); got != descendantTip {
		t.Errorf("the descendant on the metadata branch was discarded (%s -> %s); a seed replacement must never lose a descendant", descendantTip, got)
	}
	if got := r.originTip(t, "main"); got != intTip {
		t.Errorf("the refusal pruned/moved integration from %s to %s; nothing must be written before a human reconciles", intTip, got)
	}
	if snapAfter := r.readOnlySnapshot(t); snapAfter != snapBefore {
		t.Errorf("a refusal wrote observable local state:\nbefore:\n%s\nafter:\n%s", snapBefore, snapAfter)
	}
}

// TestIntegrationRepoOwnershipMigrateDescendantsBlockWithUnchangedSource proves the
// descendants guard keys on descendants-while-prune-pending, NOT on a tree
// difference: even with the source unchanged (the published seed still exactly the
// current seed), a descendant present while integration is still live refuses.
func TestIntegrationRepoOwnershipMigrateDescendantsBlockWithUnchangedSource(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
		afterSeedPush: func() error { return errors.New("crash after seed") },
	})
	descendantTip := r.advanceDocketDescendant(t, "docs/changes/active/0009-descendant.md", migChangeRecord(9, "descendant", "proposed", ""))
	intTip := r.originTip(t, "main")
	snapBefore := r.readOnlySnapshot(t)

	res := r.runMigrate(t, MigrateOptions{Authorized: true})
	if res.Result != ResultInvalidState || res.RepositoryState != "conflict" {
		t.Fatalf("resume with unchanged source but descendants present = %q (%s), want invalid-state conflict", res.Result, res.HumanText())
	}
	if !strings.Contains(res.HumanText(), "partial migration must be reconciled by a human") {
		t.Errorf("refusal %q must name the human-reconcile remedy (the guard keys on descendants, not tree difference)", res.HumanText())
	}
	if got := r.originTip(t, "docket"); got != descendantTip {
		t.Errorf("the descendant was discarded (%s -> %s)", descendantTip, got)
	}
	if got := r.originTip(t, "main"); got != intTip {
		t.Errorf("integration moved from %s to %s on a refusal", intTip, got)
	}
	if snapAfter := r.readOnlySnapshot(t); snapAfter != snapBefore {
		t.Errorf("a refusal wrote observable local state:\nbefore:\n%s\nafter:\n%s", snapBefore, snapAfter)
	}
}

// TestIntegrationRepoOwnershipMigrateMovedSourceReplaceBoundToFreshTip is the
// ownership-shard copy of the moved-source seed replacement: with NO descendants,
// an advanced source recomposes and replaces the seed under the owned lease bound to
// the fresh re-read tip, then prunes. It pins that the descendants guard does not
// fire when the tip still equals the verified seed root and that the existing
// recovery replace survives the refactor.
func TestIntegrationRepoOwnershipMigrateMovedSourceReplaceBoundToFreshTip(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
		afterSeedPush: func() error { return errors.New("crash after seed") },
	})
	seedTip := r.originTip(t, "docket")
	r.advanceIntegration(t, "docs/adrs/0002-second-decision.md",
		"---\nid: 2\nslug: second-decision\nstatus: Accepted\ntitle: Second decision\n---\nContext.\n")

	res := r.runMigrate(t, MigrateOptions{Authorized: true})
	if res.Result != ResultApplied {
		t.Fatalf("moved-source resume = %q (%s), want applied", res.Result, res.HumanText())
	}
	if got := r.originTip(t, "docket"); got == seedTip {
		t.Error("the seed was not replaced; the new copy-set bytes would be missing")
	}
	if _, ok := r.originBlob(t, "docket", "docs/adrs/0002-second-decision.md"); !ok {
		t.Error("the replaced seed is missing the newly added ADR")
	}
	if contains(r.originTreePaths(t, "main"), "docs/changes/active/0001-first-change.md") {
		t.Error("the resume did not prune the active surface after replacing the seed")
	}
}

// TestIntegrationRepoOwnershipMigrateConcurrentAdvanceDuringRace proves the
// publication race is safe: a concurrent writer advancing remote docket to a
// descendant BETWEEN the fresh re-read and the owned-lease push makes the lease
// (bound to the fresh tip) lost, the migration contends, and the winner's
// descendant survives intact — the lease never forces over it.
func TestIntegrationRepoOwnershipMigrateConcurrentAdvanceDuringRace(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
		afterSeedPush: func() error { return errors.New("crash after seed") },
	})
	r.advanceIntegration(t, "docs/adrs/0002-second-decision.md",
		"---\nid: 2\nslug: second-decision\nstatus: Accepted\ntitle: Second decision\n---\nContext.\n")

	var racerTip string
	res := r.runMigrateWithHooks(t, MigrateOptions{Authorized: true}, setupHooks{
		beforeMetadataLeasePush: func() error {
			racerTip = r.advanceDocketDescendant(t, "docs/changes/active/0009-racer.md", migChangeRecord(9, "racer", "proposed", ""))
			return nil
		},
	})
	if res.Result != ResultContended {
		t.Fatalf("a concurrent advance during the lease push = %q (%s), want contended", res.Result, res.HumanText())
	}
	if got := r.originTip(t, "docket"); got != racerTip {
		t.Errorf("the concurrent winner's descendant was overwritten (%s -> %s); the lease must never force over it", racerTip, got)
	}
}
