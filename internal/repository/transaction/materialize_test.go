package transaction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/danielhanold/docket/internal/testsupport"
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

	wt := filepath.Join(testsupport.TempDir(t), "wt")
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
