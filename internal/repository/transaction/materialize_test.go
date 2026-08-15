package transaction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// Hostile fixture path names carrying bytes that line-oriented or quoting
// parsers mishandle: a space, a literal tab, and (for a create/delete target) an
// embedded newline. These exercise the NUL-delimited status parse end to end.
const (
	matHostileTab     = "spa ce/na\tme.md" // parent dir carries a space; leaf carries a tab
	matHostileNewline = "ho\nstile.md"     // embedded newline
	matHostileCreate  = "cr ea\tted.md"    // hostile create target (no parent)
)

// newMaterializeWorktree builds a real Git repository with a fixture base commit
// and returns a client, its canonical repository, and the absolute path of a
// fresh detached worktree checked out at that commit. The worktree is where the
// materializer writes; verifyActualDelta reads its Git status. Skipped when git
// is unavailable (newTxnRepo handles the skip).
func newMaterializeWorktree(t *testing.T) (*gitcli.Client, gitcli.Repository, string) {
	t.Helper()
	client, repo := newTxnRepo(t)
	dir := repo.PrimaryWorktree

	writeFixture(t, dir, "keep.md", "base keep\n", 0o644)
	writeFixture(t, dir, "keep2.md", "second keep\n", 0o644)
	writeFixture(t, dir, "replace-me.md", "base replace\n", 0o644)
	writeFixture(t, dir, "delete-me.md", "base delete\n", 0o644)
	writeFixture(t, dir, "exec.sh", "#!/bin/sh\necho base\n", 0o755)
	writeFixture(t, dir, "docs/sub/nested.md", "nested base\n", 0o644)
	writeFixture(t, dir, matHostileTab, "hostile tab base\n", 0o644)
	writeFixture(t, dir, matHostileNewline, "hostile newline base\n", 0o644)

	matGit(t, dir, "add", "-A")
	matGit(t, dir, "commit", "-q", "-m", "materialize fixtures")

	head := gitcli.ObjectID(matGit(t, dir, "rev-parse", "HEAD"))

	wt := filepath.Join(t.TempDir(), "wt")
	if err := client.AddDetachedWorktree(context.Background(), repo, wt, head); err != nil {
		t.Fatalf("AddDetachedWorktree: %v", err)
	}
	return client, repo, wt
}

// writeFixture writes content (creating parent directories) at a repo-relative
// path and forces mode with an explicit Chmod so the executable bit survives any
// ambient umask.
func writeFixture(t *testing.T, root, rel, content string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
}

// matGit runs real git with -C <dir>, returns trimmed stdout, and fails the test
// on a non-zero exit — an oracle independent of the adapter under test.
func matGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

// assertMaterializeFailure requires err to be a *Failure at the wanted stage.
func assertMaterializeFailure(t *testing.T, err error, wantStage Stage) *Failure {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a failure at stage %q, got nil", wantStage)
	}
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("error is not *Failure: %v", err)
	}
	if f.Stage != wantStage {
		t.Errorf("failure stage = %q, want %q (detail %q)", f.Stage, wantStage, f.Detail)
	}
	return f
}

// assertFailureKind requires err to be a *Failure of the wanted kind.
func assertFailureKind(t *testing.T, err error, want Kind) {
	t.Helper()
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("error is not *Failure: %v", err)
	}
	if f.Kind != want {
		t.Errorf("failure kind = %q, want %q", f.Kind, want)
	}
}

// assertFile requires the file at wt/rel to hold exactly want.
func assertFile(t *testing.T, wt, rel, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(wt, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if string(got) != want {
		t.Errorf("%s = %q, want %q", rel, got, want)
	}
}

// hashTreeExcept hashes every regular file under root (skipping the worktree's
// .git pointer and every excluded repo-relative path) into one order-independent
// digest of (path, mode, content). Two digests being equal proves the whole tree
// minus the excluded set is byte-identical.
func hashTreeExcept(t *testing.T, root string, exclude map[string]bool) string {
	t.Helper()
	type ent struct{ rel, mode, sum string }
	var ents []ent
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || exclude[rel] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		var content []byte
		if info.Mode().IsRegular() {
			if content, err = os.ReadFile(p); err != nil {
				return err
			}
		}
		sum := sha256.Sum256(content)
		ents = append(ents, ent{rel: rel, mode: info.Mode().String(), sum: hex.EncodeToString(sum[:])})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Slice(ents, func(i, j int) bool { return ents[i].rel < ents[j].rel })
	h := sha256.New()
	for _, e := range ents {
		fmt.Fprintf(h, "%s\x00%s\x00%s\n", e.rel, e.mode, e.sum)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// planWith wraps a file set into a valid MutationPlan (subject + receipt fixed).
func planWith(files ...FileMutation) MutationPlan {
	return MutationPlan{Files: files, CommitSubject: "materialize test", Receipt: []byte(`{}`)}
}

// TestMaterializeCreateReplaceDeleteByteExact proves create/replace/delete land
// byte-exact (including a create under a brand-new nested directory), that
// verifyMaterialized accepts them, and that every unrelated file is untouched.
func TestMaterializeCreateReplaceDeleteByteExact(t *testing.T) {
	_, _, wt := newMaterializeWorktree(t)
	declared := map[string]bool{
		"created.md":        true,
		"fresh/deep/new.md": true,
		"replace-me.md":     true,
		"delete-me.md":      true,
	}
	before := hashTreeExcept(t, wt, declared)

	plan := planWith(
		FileMutation{Path: "created.md", Kind: MutationCreate, Bytes: []byte("created bytes\n")},
		FileMutation{Path: "fresh/deep/new.md", Kind: MutationCreate, Bytes: []byte("nested new\n")},
		FileMutation{Path: "replace-me.md", Kind: MutationReplace, Bytes: []byte("replaced bytes\n")},
		FileMutation{Path: "delete-me.md", Kind: MutationDelete},
	)
	if err := materializePlan(wt, plan); err != nil {
		t.Fatalf("materializePlan: %v", err)
	}
	if err := verifyMaterialized(wt, plan); err != nil {
		t.Fatalf("verifyMaterialized: %v", err)
	}

	assertFile(t, wt, "created.md", "created bytes\n")
	assertFile(t, wt, "fresh/deep/new.md", "nested new\n")
	assertFile(t, wt, "replace-me.md", "replaced bytes\n")
	if _, err := os.Lstat(filepath.Join(wt, "delete-me.md")); !os.IsNotExist(err) {
		t.Errorf("delete-me.md still present: err=%v", err)
	}

	if after := hashTreeExcept(t, wt, declared); before != after {
		t.Errorf("unrelated files changed: %s != %s", before, after)
	}
}

// TestMaterializeCreateEmptyFile proves an intentionally empty create lands as a
// zero-byte regular file that verifyMaterialized accepts.
func TestMaterializeCreateEmptyFile(t *testing.T) {
	_, _, wt := newMaterializeWorktree(t)
	plan := planWith(FileMutation{Path: "empty.md", Kind: MutationCreate, Bytes: nil})
	if err := materializePlan(wt, plan); err != nil {
		t.Fatalf("materializePlan: %v", err)
	}
	if err := verifyMaterialized(wt, plan); err != nil {
		t.Fatalf("verifyMaterialized: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(wt, "empty.md"))
	if err != nil || len(got) != 0 {
		t.Errorf("empty.md not a zero-byte file: content=%q err=%v", got, err)
	}
}

// TestMaterializeReplacePreservesExecutableMode proves the sibling-temp+rename
// path copies the base file's mode onto the replacement: an executable file
// keeps its 100755 mode through a replace. The assertion compares against the
// file's own pre-replace mode, so it is umask-independent.
func TestMaterializeReplacePreservesExecutableMode(t *testing.T) {
	_, _, wt := newMaterializeWorktree(t)
	execPath := filepath.Join(wt, "exec.sh")
	orig, err := os.Lstat(execPath)
	if err != nil {
		t.Fatal(err)
	}
	origMode := orig.Mode().Perm()
	if origMode&0o100 == 0 {
		t.Fatalf("fixture exec.sh is not executable: %04o", origMode)
	}

	plan := planWith(FileMutation{Path: "exec.sh", Kind: MutationReplace, Bytes: []byte("#!/bin/sh\necho replaced\n")})
	if err := materializePlan(wt, plan); err != nil {
		t.Fatalf("materializePlan: %v", err)
	}

	after, err := os.Lstat(execPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.Mode().Perm(); got != origMode {
		t.Errorf("exec.sh mode = %04o, want %04o (mode not preserved through replace)", got, origMode)
	}
	assertFile(t, wt, "exec.sh", "#!/bin/sh\necho replaced\n")
}

// TestMaterializeRefusesReplaceTargetSymlink proves a replace whose target is a
// symlink is refused (never writing through the link), and the link and its
// referent are left untouched.
func TestMaterializeRefusesReplaceTargetSymlink(t *testing.T) {
	_, _, wt := newMaterializeWorktree(t)
	linkPath := filepath.Join(wt, "linktarget.md")
	if err := os.Symlink("keep.md", linkPath); err != nil {
		t.Fatal(err)
	}

	plan := planWith(FileMutation{Path: "linktarget.md", Kind: MutationReplace, Bytes: []byte("through the link\n")})
	err := materializePlan(wt, plan)
	assertMaterializeFailure(t, err, StageMaterialize)

	fi, lerr := os.Lstat(linkPath)
	if lerr != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("linktarget.md is no longer a symlink: mode=%v err=%v", fi.Mode(), lerr)
	}
	assertFile(t, wt, "keep.md", "base keep\n")
}

// TestMaterializeRefusesSymlinkParentComponent proves a create whose parent
// component is a symlink is refused. Two cases matter for different reasons:
//
//   - "outside": the symlink points to a directory OUTSIDE the worktree. os.Root
//     alone would refuse the escape, but this also asserts the stronger property
//     that nothing at all is written into that outside directory.
//   - "inside": the symlink points to a real directory INSIDE the worktree. This
//     is the case os.Root does NOT catch — it would happily FOLLOW the link and
//     write the file through it into the linked directory, silently escaping the
//     declared path. Only the explicit parent-component Lstat walk refuses it.
//     This is the load-bearing containment guard.
func TestMaterializeRefusesSymlinkParentComponent(t *testing.T) {
	t.Run("outside", func(t *testing.T) {
		_, _, wt := newMaterializeWorktree(t)

		outside := t.TempDir()
		secret := filepath.Join(outside, "secret.md")
		if err := os.WriteFile(secret, []byte("do not touch\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		beforeInfo, err := os.Stat(secret)
		if err != nil {
			t.Fatal(err)
		}
		beforeList, err := os.ReadDir(outside)
		if err != nil {
			t.Fatal(err)
		}

		if err := os.Symlink(outside, filepath.Join(wt, "evil")); err != nil {
			t.Fatal(err)
		}

		plan := planWith(FileMutation{Path: "evil/planted.md", Kind: MutationCreate, Bytes: []byte("planted\n")})
		assertMaterializeFailure(t, materializePlan(wt, plan), StageMaterialize)

		if _, err := os.Stat(filepath.Join(outside, "planted.md")); !os.IsNotExist(err) {
			t.Errorf("a file escaped the root into the outside dir: err=%v", err)
		}
		afterList, err := os.ReadDir(outside)
		if err != nil {
			t.Fatal(err)
		}
		if len(afterList) != len(beforeList) {
			t.Errorf("outside dir entry count changed: %d -> %d", len(beforeList), len(afterList))
		}
		afterInfo, err := os.Stat(secret)
		if err != nil {
			t.Fatal(err)
		}
		if !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
			t.Errorf("outside secret mtime changed: %v -> %v", beforeInfo.ModTime(), afterInfo.ModTime())
		}
		if got, _ := os.ReadFile(secret); string(got) != "do not touch\n" {
			t.Errorf("outside secret content changed: %q", got)
		}
	})

	t.Run("inside", func(t *testing.T) {
		_, _, wt := newMaterializeWorktree(t)

		// A symlink to a real directory INSIDE the worktree. os.Root would follow
		// it; the explicit guard must refuse it. "docs/sub" exists in the fixture.
		if err := os.Symlink("docs/sub", filepath.Join(wt, "inside-link")); err != nil {
			t.Fatal(err)
		}

		plan := planWith(FileMutation{Path: "inside-link/planted.md", Kind: MutationCreate, Bytes: []byte("planted\n")})
		assertMaterializeFailure(t, materializePlan(wt, plan), StageMaterialize)

		// The write must NOT have gone through the symlink into docs/sub.
		if _, err := os.Lstat(filepath.Join(wt, "docs", "sub", "planted.md")); !os.IsNotExist(err) {
			t.Errorf("a file was written THROUGH the inside symlink into docs/sub: err=%v", err)
		}
		// And the link's own name resolved to nothing on disk beneath it.
		if _, err := os.Lstat(filepath.Join(wt, "inside-link", "planted.md")); err == nil {
			t.Errorf("planted.md exists under the symlinked parent")
		}
	})
}

// TestVerifyMaterializedDetectsCorruption proves the readback guard reddens when
// the on-disk bytes no longer match the plan (post-materialization tampering).
func TestVerifyMaterializedDetectsCorruption(t *testing.T) {
	_, _, wt := newMaterializeWorktree(t)
	plan := planWith(FileMutation{Path: "created.md", Kind: MutationCreate, Bytes: []byte("good bytes\n")})
	if err := materializePlan(wt, plan); err != nil {
		t.Fatalf("materializePlan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt, "created.md"), []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := verifyMaterialized(wt, plan)
	assertMaterializeFailure(t, err, StageMaterialize)
}

// TestVerifyActualDeltaExactMatch proves the two-way delta guard accepts a
// worktree whose actual changed-path set equals the plan's declared set exactly.
func TestVerifyActualDeltaExactMatch(t *testing.T) {
	client, repo, wt := newMaterializeWorktree(t)
	plan := planWith(
		FileMutation{Path: "created.md", Kind: MutationCreate, Bytes: []byte("created\n")},
		FileMutation{Path: "replace-me.md", Kind: MutationReplace, Bytes: []byte("changed\n")},
		FileMutation{Path: "delete-me.md", Kind: MutationDelete},
	)
	if err := materializePlan(wt, plan); err != nil {
		t.Fatalf("materializePlan: %v", err)
	}
	if err := verifyActualDelta(context.Background(), client, repo, wt, plan); err != nil {
		t.Fatalf("verifyActualDelta: %v", err)
	}
}

// TestVerifyActualDeltaRejectsUndeclaredChange proves an undeclared changed path
// in the worktree fails the guard (undeclared direction).
func TestVerifyActualDeltaRejectsUndeclaredChange(t *testing.T) {
	client, repo, wt := newMaterializeWorktree(t)
	plan := planWith(FileMutation{Path: "replace-me.md", Kind: MutationReplace, Bytes: []byte("changed\n")})
	if err := materializePlan(wt, plan); err != nil {
		t.Fatalf("materializePlan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt, "stray.md"), []byte("stray\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := verifyActualDelta(context.Background(), client, repo, wt, plan)
	assertMaterializeFailure(t, err, StageVerifyDelta)
	assertFailureKind(t, err, KindInvalidState)
}

// TestVerifyActualDeltaRejectsDeclaredUnchanged proves a declared path whose
// bytes equal base — so it produces no Git delta — fails the guard
// (declared-but-unchanged direction), even though verifyMaterialized passes.
func TestVerifyActualDeltaRejectsDeclaredUnchanged(t *testing.T) {
	client, repo, wt := newMaterializeWorktree(t)
	// Replace with bytes identical to the base content: no actual change.
	plan := planWith(FileMutation{Path: "replace-me.md", Kind: MutationReplace, Bytes: []byte("base replace\n")})
	if err := materializePlan(wt, plan); err != nil {
		t.Fatalf("materializePlan: %v", err)
	}
	// The readback guard is content-vs-plan and must PASS here — the delta guard
	// is the one that catches "the plan did not describe reality".
	if err := verifyMaterialized(wt, plan); err != nil {
		t.Fatalf("verifyMaterialized should pass when bytes match the plan: %v", err)
	}
	err := verifyActualDelta(context.Background(), client, repo, wt, plan)
	assertMaterializeFailure(t, err, StageVerifyDelta)
	assertFailureKind(t, err, KindInvalidState)
}

// TestMaterializeHostilePathsByteExact drives create/replace/delete on
// space/tab/newline-laden paths through the whole pipeline: materialize,
// readback, and the Git-derived delta guard all accept them byte-exact.
func TestMaterializeHostilePathsByteExact(t *testing.T) {
	client, repo, wt := newMaterializeWorktree(t)
	plan := planWith(
		FileMutation{Path: gitcli.RepoPath(matHostileTab), Kind: MutationReplace, Bytes: []byte("hostile tab replaced\n")},
		FileMutation{Path: gitcli.RepoPath(matHostileNewline), Kind: MutationDelete},
		FileMutation{Path: gitcli.RepoPath(matHostileCreate), Kind: MutationCreate, Bytes: []byte("hostile create\n")},
	)
	if err := materializePlan(wt, plan); err != nil {
		t.Fatalf("materializePlan: %v", err)
	}
	if err := verifyMaterialized(wt, plan); err != nil {
		t.Fatalf("verifyMaterialized: %v", err)
	}
	if err := verifyActualDelta(context.Background(), client, repo, wt, plan); err != nil {
		t.Fatalf("verifyActualDelta: %v", err)
	}

	assertFile(t, wt, matHostileTab, "hostile tab replaced\n")
	assertFile(t, wt, matHostileCreate, "hostile create\n")
	if _, err := os.Lstat(filepath.Join(wt, matHostileNewline)); !os.IsNotExist(err) {
		t.Errorf("hostile newline path still present: err=%v", err)
	}
}
