//go:build integration

package gitcli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file is the Task 8 proof matrix: it demonstrates the two invariants the
// whole adapter exists to protect. First, that the full Discover -> FetchBranch
// -> OpenObjectSource -> ListTree -> ReadBlobs chain leaves the invocation
// checkout byte-for-byte untouched — for every worktree of both metadata
// topologies, including a deliberately dirtied tracked file and an untracked
// file. Second, that an ObjectSource is pinned to its exact commit at open time,
// so later fetches advancing a tracking ref (or an unrelated branch) can never
// change what an already-open source returns. Every git command used to observe
// the checkout is a pure READ (rev-parse, symbolic-ref -q, ls-files -s -z, diff
// --cached --raw -z, status --porcelain=v2 -z) — never write-tree, which would
// write objects and contaminate the proof.

// checkoutSnapshot records every checkout property the spec names as preserved.
// All fields are captured with read-only plumbing so snapshotting cannot itself
// perturb what it measures.
type checkoutSnapshot struct {
	headSymbolic string // symbolic-ref -q HEAD (empty when detached)
	headCommit   string // rev-parse HEAD
	indexTree    string // diff --cached --raw -z + ls-files -s -z (pure reads; never write-tree)
	statusRaw    string // status --porcelain=v2 -z --untracked-files=all
	dirtyBytes   string // the deliberately dirtied tracked file's bytes
	untracked    string // the untracked file's bytes
}

// preserveWorktree is one checkout under test: its absolute directory and the
// repo-relative tracked file to dirty (a file known present on that worktree's
// branch).
type preserveWorktree struct {
	name      string
	dir       string
	dirtyFile string
}

// preserveTopology drives the preservation proof for one metadata topology: how
// to build it, whether it carries an orphan docket branch to also advance, the
// three blob paths to read from the pinned main source, and the set of
// checkouts to prove preserved.
type preserveTopology struct {
	name      string
	build     func(t *testing.T) *testRepos
	isDocket  bool
	readPaths []RepoPath
	worktrees func(r *testRepos) []preserveWorktree
}

// preserveTopologies enumerates the main-mode single-worktree repo and the
// docket-mode three-worktree repo (primary + .docket + feature).
func preserveTopologies() []preserveTopology {
	return []preserveTopology{
		{
			name:      "main",
			build:     newMainModeRepos,
			isDocket:  false,
			readPaths: []RepoPath{"README.md", ".docket.yml", "tool.sh"},
			worktrees: func(r *testRepos) []preserveWorktree {
				return []preserveWorktree{
					{name: "primary", dir: r.Invocation, dirtyFile: "README.md"},
				}
			},
		},
		{
			name:      "docket",
			build:     newDocketModeRepos,
			isDocket:  true,
			readPaths: []RepoPath{".docket.yml", "main.go", ".gitignore"},
			worktrees: func(r *testRepos) []preserveWorktree {
				return []preserveWorktree{
					{name: "primary", dir: r.Invocation, dirtyFile: "main.go"},
					{name: "docket", dir: filepath.Join(r.Invocation, ".docket"), dirtyFile: "docs/changes/active/0001-plan.md"},
					{name: "feature", dir: filepath.Join(r.Invocation, ".worktrees", "feat-x"), dirtyFile: "main.go"},
				}
			},
		},
	}
}

// dirtyTrackedFile appends a local edit to a tracked file so the worktree has an
// unstaged modification the adapter must not disturb.
func dirtyTrackedFile(t *testing.T, wtDir, rel string) {
	t.Helper()
	p := filepath.Join(wtDir, rel)
	orig, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read tracked file %q: %v", p, err)
	}
	if err := os.WriteFile(p, append(orig, []byte("\nLOCAL UNSTAGED EDIT\n")...), 0o644); err != nil {
		t.Fatalf("dirty tracked file %q: %v", p, err)
	}
}

// createUntracked writes a fresh untracked file the adapter must not disturb.
func createUntracked(t *testing.T, wtDir, rel string) {
	t.Helper()
	p := filepath.Join(wtDir, rel)
	if err := os.WriteFile(p, []byte("untracked local content\n"), 0o644); err != nil {
		t.Fatalf("create untracked file %q: %v", p, err)
	}
}

// mustReadFile reads a file or fails the test.
func mustReadFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %q: %v", p, err)
	}
	return string(b)
}

// snapshotCheckout captures every preserved property of the worktree at wtDir
// using read-only plumbing only. symbolic-ref -q returns non-zero on a detached
// HEAD, which is recorded as the empty string.
func snapshotCheckout(t *testing.T, wtDir, dirtyRel, untrackedRel string) checkoutSnapshot {
	t.Helper()
	sym := ""
	if out, err := gitTry(wtDir, "symbolic-ref", "-q", "HEAD"); err == nil {
		sym = strings.TrimSpace(out)
	}
	index := string(rawGitOut(t, wtDir, "diff", "--cached", "--raw", "-z")) +
		"\x1e" + string(rawGitOut(t, wtDir, "ls-files", "-s", "-z"))
	return checkoutSnapshot{
		headSymbolic: sym,
		headCommit:   gitOut(t, wtDir, "rev-parse", "HEAD"),
		indexTree:    index,
		statusRaw:    string(rawGitOut(t, wtDir, "status", "--porcelain=v2", "-z", "--untracked-files=all")),
		dirtyBytes:   mustReadFile(t, filepath.Join(wtDir, dirtyRel)),
		untracked:    mustReadFile(t, filepath.Join(wtDir, untrackedRel)),
	}
}

// requireSameSnapshot fails, field by field, on any difference between two
// checkout snapshots.
func requireSameSnapshot(t *testing.T, before, after checkoutSnapshot) {
	t.Helper()
	if before.headSymbolic != after.headSymbolic {
		t.Errorf("HEAD symbolic ref changed: %q -> %q", before.headSymbolic, after.headSymbolic)
	}
	if before.headCommit != after.headCommit {
		t.Errorf("HEAD commit changed: %q -> %q", before.headCommit, after.headCommit)
	}
	if before.indexTree != after.indexTree {
		t.Errorf("index / staged content changed:\nbefore %q\nafter  %q", before.indexTree, after.indexTree)
	}
	if before.statusRaw != after.statusRaw {
		t.Errorf("working-tree status changed:\nbefore %q\nafter  %q", before.statusRaw, after.statusRaw)
	}
	if before.dirtyBytes != after.dirtyBytes {
		t.Errorf("dirtied tracked file changed: %q -> %q", before.dirtyBytes, after.dirtyBytes)
	}
	if before.untracked != after.untracked {
		t.Errorf("untracked file changed: %q -> %q", before.untracked, after.untracked)
	}
}

// readOneBlob reads exactly one path through the source, requiring it be found,
// and returns its bytes.
func readOneBlob(t *testing.T, ctx context.Context, src ObjectSource, p RepoPath) []byte {
	t.Helper()
	res, err := src.ReadBlobs(ctx, []RepoPath{p})
	if err != nil {
		t.Fatalf("ReadBlobs(%q): %v", p, err)
	}
	if len(res) != 1 || !res[0].Found {
		t.Fatalf("ReadBlobs(%q) = %+v, want one found result", p, res)
	}
	return res[0].Blob.Bytes
}

// TestCheckoutPreservationAcrossWorktreesAndTopologies proves the full adapter
// chain never disturbs the invocation checkout. For every worktree of both
// topologies it dirties a tracked file, drops an untracked file, snapshots,
// advances the remote, runs Discover -> FetchBranch -> OpenObjectSource ->
// ListTree -> ReadBlobs, then requires the checkout snapshot byte-identical AND
// that the remote-tracking ref genuinely moved (so the fetch was real, not a
// no-op that would trivially "preserve" the checkout).
func TestIntegrationRepoCheckoutPreservationAcrossWorktreesAndTopologies(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	const untrackedRel = "preserve-untracked.md"

	for _, topo := range preserveTopologies() {
		t.Run(topo.name, func(t *testing.T) {
			c := newRealClient(t)
			r := topo.build(t)
			for _, wt := range topo.worktrees(r) {
				t.Run(wt.name, func(t *testing.T) {
					// 1. Dirty a tracked file + create an untracked file.
					dirtyTrackedFile(t, wt.dir, wt.dirtyFile)
					createUntracked(t, wt.dir, untrackedRel)

					// 2. Snapshot the checkout.
					before := snapshotCheckout(t, wt.dir, wt.dirtyFile, untrackedRel)

					// 3. Writer advances origin/main (and origin/docket in docket mode).
					newMain := r.writerCommit(t, "main", map[string]string{
						"advance.md": "advance for " + topo.name + "/" + wt.name + "\n",
					})
					if topo.isDocket {
						r.writerCommit(t, "docket", map[string]string{
							"docs/changes/active/0002-adv.md": "docket advance for " + wt.name + "\n",
						})
					}

					// 4. Run the whole adapter chain from this worktree.
					repo := mustDiscover(t, c, wt.dir)
					rev, err := c.FetchBranch(ctx, repo, "origin", "refs/heads/main")
					if err != nil {
						t.Fatalf("FetchBranch: %v", err)
					}
					src, err := c.OpenObjectSource(ctx, repo, rev)
					if err != nil {
						t.Fatalf("OpenObjectSource: %v", err)
					}
					if _, err := src.ListTree(ctx, nil); err != nil {
						t.Fatalf("ListTree: %v", err)
					}
					if _, err := src.ReadBlobs(ctx, topo.readPaths); err != nil {
						t.Fatalf("ReadBlobs: %v", err)
					}

					// 5. Re-snapshot; must be byte-identical.
					after := snapshotCheckout(t, wt.dir, wt.dirtyFile, untrackedRel)
					requireSameSnapshot(t, before, after)

					// 6. The tracking ref DID move — the fetch was real work.
					if got := gitOut(t, wt.dir, "rev-parse", "refs/remotes/origin/main"); got != string(newMain) {
						t.Fatalf("refs/remotes/origin/main = %s, want advanced %s (fetch was skipped)", got, newMain)
					}
				})
			}
		})
	}
}

// TestRevisionConsistencyABAndUnrelatedBranch proves an ObjectSource is pinned
// to its open-time commit. A source opened at A keeps returning A's tree and
// bytes after the remote advances to B and B is fetched locally; a source opened
// at B returns B; and advancing an entirely unrelated branch leaves both
// sources' revisions and blob reads unchanged.
func TestIntegrationRepoRevisionConsistencyABAndUnrelatedBranch(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	revA, err := c.FetchBranch(ctx, repo, "origin", "refs/heads/main")
	if err != nil {
		t.Fatalf("FetchBranch A: %v", err)
	}
	srcA, err := c.OpenObjectSource(ctx, repo, revA)
	if err != nil {
		t.Fatalf("OpenObjectSource A: %v", err)
	}
	bytesA := readOneBlob(t, ctx, srcA, "README.md")
	treeA, err := srcA.ListTree(ctx, nil)
	if err != nil {
		t.Fatalf("ListTree A: %v", err)
	}

	// Writer advances main to B: change README.md and add a new file.
	commitB := r.writerCommit(t, "main", map[string]string{
		"README.md":   "CHANGED README at B\n",
		"new-file.md": "added at B\n",
	})

	// Source A still sees A after the remote advanced (before any local fetch).
	if got := readOneBlob(t, ctx, srcA, "README.md"); !bytes.Equal(got, bytesA) {
		t.Fatalf("srcA README.md changed after remote advance: %q != %q", got, bytesA)
	}
	treeA2, err := srcA.ListTree(ctx, nil)
	if err != nil {
		t.Fatalf("ListTree A (post-advance): %v", err)
	}
	sameEntries(t, treeA, treeA2)
	if hasPath(treeA2, "new-file.md") {
		t.Fatal("srcA leaked B's new-file.md into A's tree")
	}

	// Fetch B locally; source A is still pinned to A.
	revB, err := c.FetchBranch(ctx, repo, "origin", "refs/heads/main")
	if err != nil {
		t.Fatalf("FetchBranch B: %v", err)
	}
	if revB.Commit == revA.Commit {
		t.Fatal("origin/main did not advance between the two fetches")
	}
	if revB.Commit != commitB {
		t.Fatalf("revB.Commit = %s, want writer commit %s", revB.Commit, commitB)
	}
	if got := readOneBlob(t, ctx, srcA, "README.md"); !bytes.Equal(got, bytesA) {
		t.Fatalf("srcA README.md changed after fetching B: %q != %q", got, bytesA)
	}

	// A source opened at B returns B's content.
	srcB, err := c.OpenObjectSource(ctx, repo, revB)
	if err != nil {
		t.Fatalf("OpenObjectSource B: %v", err)
	}
	bytesB := readOneBlob(t, ctx, srcB, "README.md")
	if string(bytesB) != "CHANGED README at B\n" {
		t.Fatalf("srcB README.md = %q, want B's changed content", bytesB)
	}
	treeB, err := srcB.ListTree(ctx, nil)
	if err != nil {
		t.Fatalf("ListTree B: %v", err)
	}
	if !hasPath(treeB, "new-file.md") {
		t.Fatal("srcB missing B's new-file.md")
	}

	// Advancing an unrelated branch changes nothing either source sees.
	r.writerCommit(t, "unrelated", map[string]string{"unrelated.md": "unrelated content\n"})

	if srcA.Revision().Commit != revA.Commit {
		t.Fatalf("srcA revision moved to %s, want pinned %s", srcA.Revision().Commit, revA.Commit)
	}
	if srcB.Revision().Commit != revB.Commit {
		t.Fatalf("srcB revision moved to %s, want pinned %s", srcB.Revision().Commit, revB.Commit)
	}
	if got := readOneBlob(t, ctx, srcA, "README.md"); !bytes.Equal(got, bytesA) {
		t.Fatalf("srcA README.md changed after unrelated advance: %q != %q", got, bytesA)
	}
	if got := readOneBlob(t, ctx, srcB, "README.md"); !bytes.Equal(got, bytesB) {
		t.Fatalf("srcB README.md changed after unrelated advance: %q != %q", got, bytesB)
	}
}

// TestDocketModeTwoSourceReads proves the docket-mode composition primitive: two
// sources opened at two branches of the same repository each see only their own
// branch's tree. It fetches refs/heads/main and refs/heads/docket, opens a
// source at each, reads .docket.yml from main and a planning file from docket,
// and asserts the cross reads miss — the planning file is absent from the main
// source and .docket.yml is absent from the orphan docket source.
func TestIntegrationRepoDocketModeTwoSourceReads(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newDocketModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	revMain, err := c.FetchBranch(ctx, repo, "origin", "refs/heads/main")
	if err != nil {
		t.Fatalf("FetchBranch main: %v", err)
	}
	revDocket, err := c.FetchBranch(ctx, repo, "origin", "refs/heads/docket")
	if err != nil {
		t.Fatalf("FetchBranch docket: %v", err)
	}
	if revMain.Commit == revDocket.Commit {
		t.Fatal("main and docket resolved to the same commit (orphan branches must differ)")
	}

	srcMain, err := c.OpenObjectSource(ctx, repo, revMain)
	if err != nil {
		t.Fatalf("OpenObjectSource main: %v", err)
	}
	srcDocket, err := c.OpenObjectSource(ctx, repo, revDocket)
	if err != nil {
		t.Fatalf("OpenObjectSource docket: %v", err)
	}

	if got := readOneBlob(t, ctx, srcMain, ".docket.yml"); string(got) != "version: 1\n" {
		t.Fatalf("main source .docket.yml = %q, want \"version: 1\\n\"", got)
	}
	planPath := RepoPath("docs/changes/active/0001-plan.md")
	if got := readOneBlob(t, ctx, srcDocket, planPath); string(got) != "plan\n" {
		t.Fatalf("docket source plan file = %q, want \"plan\\n\"", got)
	}

	// The planning file is NOT in the main source.
	resMain, err := srcMain.ReadBlobs(ctx, []RepoPath{planPath})
	if err != nil {
		t.Fatalf("srcMain ReadBlobs(plan): %v", err)
	}
	if len(resMain) != 1 || resMain[0].Found {
		t.Fatalf("main source unexpectedly holds the docket planning file: %+v", resMain)
	}

	// .docket.yml is NOT in the orphan docket source.
	resDocket, err := srcDocket.ReadBlobs(ctx, []RepoPath{".docket.yml"})
	if err != nil {
		t.Fatalf("srcDocket ReadBlobs(.docket.yml): %v", err)
	}
	if len(resDocket) != 1 || resDocket[0].Found {
		t.Fatalf("docket source unexpectedly holds .docket.yml: %+v", resDocket)
	}
}
