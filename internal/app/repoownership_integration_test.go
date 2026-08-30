//go:build integration

package app

import (
	"context"
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
