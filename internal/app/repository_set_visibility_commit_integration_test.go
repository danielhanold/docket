//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// newSwitchCommitRepo builds a one-commit repository on main holding a tracked
// .docket.yml, .gitignore, and AGENTS.md.
func newSwitchCommitRepo(t *testing.T) (*gitcli.Client, gitcli.Repository) {
	t.Helper()
	requireRealGit(t)
	git := newGitClient(t)
	dir, err := filepath.EvalSymlinks(testsupport.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init", "-q", "-b", "main")
	gitIdentity(t, dir)
	writeRepoFile(t, dir, ".docket.yml", "visibility: shared\n")
	writeRepoFile(t, dir, ".gitignore", "/.docket/\n")
	writeRepoFile(t, dir, "AGENTS.md", "# rules\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-q", "-m", "init")
	return git, gitcli.Repository{PrimaryWorktree: dir, CommonDir: filepath.Join(dir, ".git")}
}

func TestIntegrationRepoVisibilitySwitchCommitPathConflicts(t *testing.T) {
	ctx := context.Background()
	git, repo := newSwitchCommitRepo(t)
	dir, common := repo.PrimaryWorktree, repo.CommonDir

	conflicts := func() []string {
		t.Helper()
		got, err := commitPathConflicts(ctx, git, repo, common, visibilityCommitPaths)
		if err != nil {
			t.Fatalf("commitPathConflicts: %v", err)
		}
		return got
	}

	if got := conflicts(); len(got) != 0 {
		t.Fatalf("clean checkout: want no conflicts, got %q", got)
	}

	// A user's own edit to a commit path is a conflict.
	writeRepoFile(t, dir, "AGENTS.md", "# rules\nmine\n")
	got := conflicts()
	if len(got) != 1 || !strings.Contains(got[0], "AGENTS.md") {
		t.Fatalf("user edit: want one conflict naming AGENTS.md, got %q", got)
	}
	runGit(t, dir, "checkout", "--", "AGENTS.md")

	// The switch's own journaled write is not.
	if err := writeSwitchPath(common, dir, visibilityAddSubject, ".docket.yml", []byte("visibility: private\n")); err != nil {
		t.Fatal(err)
	}
	if err := writeSwitchPath(common, dir, visibilityAddSubject, ".gitignore", nil); err != nil {
		t.Fatal(err)
	}
	if got := conflicts(); len(got) != 0 {
		t.Fatalf("journaled writes: want no conflicts, got %q", got)
	}

	// A hand edit on top of the journaled write is.
	writeRepoFile(t, dir, ".docket.yml", "visibility: private\nextra: 1\n")
	got = conflicts()
	if len(got) != 1 || !strings.Contains(got[0], ".docket.yml") {
		t.Fatalf("hand edit after journaled write: want one conflict naming .docket.yml, got %q", got)
	}

	// A detached HEAD is a refusal of its own.
	if err := clearSwitchJournal(common); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "checkout", "-q", "--", ".")
	runGit(t, dir, "checkout", "-q", "--detach")
	got = conflicts()
	if len(got) != 1 || !strings.Contains(got[0], "detached HEAD") {
		t.Fatalf("detached HEAD: want one detached-HEAD refusal, got %q", got)
	}
}

func TestIntegrationRepoVisibilitySwitchJournalCommitsOnlyJournaledPaths(t *testing.T) {
	ctx := context.Background()
	git, repo := newSwitchCommitRepo(t)
	dir, common := repo.PrimaryWorktree, repo.CommonDir

	writeRepoFile(t, dir, "other.txt", "user work\n")
	runGit(t, dir, "add", "other.txt")
	if err := writeSwitchPath(common, dir, visibilityAddSubject, ".docket.yml", []byte("visibility: private\n")); err != nil {
		t.Fatal(err)
	}
	if err := writeSwitchPath(common, dir, visibilityAddSubject, ".gitignore", nil); err != nil {
		t.Fatal(err)
	}

	out, err := commitSwitchJournal(ctx, git, repo, common)
	if err != nil {
		t.Fatalf("commitSwitchJournal: %v", err)
	}
	if out.Commit == "" || !out.Complete {
		t.Fatalf("want a complete commit, got %+v", out)
	}
	if want := []string{".docket.yml", ".gitignore"}; strings.Join(out.Paths, ",") != strings.Join(want, ",") {
		t.Fatalf("outcome paths = %q, want %q", out.Paths, want)
	}
	if head := runGit(t, dir, "rev-parse", "HEAD"); head != out.Commit {
		t.Fatalf("HEAD %s, outcome commit %s", head, out.Commit)
	}
	if msg, _ := tryGit(dir, "log", "-1", "--format=%B"); msg != visibilityAddSubject+"\n\n" {
		t.Fatalf("commit message = %q, want exactly the subject", msg)
	}
	names := strings.Split(runGit(t, dir, "show", "--name-status", "--format=", "HEAD"), "\n")
	sort.Strings(names)
	if want := []string{"D\t.gitignore", "M\t.docket.yml"}; strings.Join(names, "|") != strings.Join(want, "|") {
		t.Fatalf("committed paths = %q, want %q", names, want)
	}
	if staged := runGit(t, dir, "diff", "--cached", "--name-only"); staged != "other.txt" {
		t.Fatalf("staged after commit = %q, want other.txt still staged", staged)
	}
	if _, ok, err := loadSwitchJournal(common); err != nil || ok {
		t.Fatalf("journal after commit: ok=%v err=%v, want cleared", ok, err)
	}
}

func TestIntegrationRepoVisibilitySwitchJournalIsIdempotentAfterCrash(t *testing.T) {
	ctx := context.Background()
	git, repo := newSwitchCommitRepo(t)
	dir, common := repo.PrimaryWorktree, repo.CommonDir

	if err := writeSwitchPath(common, dir, visibilityRemoveSubject, ".docket.yml", nil); err != nil {
		t.Fatal(err)
	}
	j, ok, err := loadSwitchJournal(common)
	if err != nil || !ok {
		t.Fatalf("load journal: ok=%v err=%v", ok, err)
	}
	first, err := commitSwitchJournal(ctx, git, repo, common)
	if err != nil || first.Commit == "" {
		t.Fatalf("first commit: %+v %v", first, err)
	}
	// Death between commit and cleanup: the journal is back.
	if err := saveSwitchJournal(common, j); err != nil {
		t.Fatal(err)
	}
	second, err := commitSwitchJournal(ctx, git, repo, common)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if second.Commit != "" || !second.Complete || len(second.Paths) != 0 {
		t.Fatalf("second call: want nothing committed and complete, got %+v", second)
	}
	if head := runGit(t, dir, "rev-parse", "HEAD"); head != first.Commit {
		t.Fatalf("HEAD moved to %s; want %s", head, first.Commit)
	}
	if _, ok, err := loadSwitchJournal(common); err != nil || ok {
		t.Fatalf("journal after idempotent call: ok=%v err=%v, want cleared", ok, err)
	}
}

func TestIntegrationRepoVisibilitySwitchJournalRetriesAfterHookFailure(t *testing.T) {
	ctx := context.Background()
	git, repo := newSwitchCommitRepo(t)
	dir, common := repo.PrimaryWorktree, repo.CommonDir

	marker := filepath.Join(testsupport.TempDir(t), "rejected-once")
	hook := "#!/bin/sh\nif [ -e '" + marker + "' ]; then exit 0; fi\n: > '" + marker + "'\nexit 1\n"
	if err := os.MkdirAll(filepath.Join(common, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(common, "hooks", "commit-msg"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	before := runGit(t, dir, "rev-parse", "HEAD")
	if err := writeSwitchPath(common, dir, visibilityAddSubject, ".docket.yml", []byte("visibility: shared\nx: 1\n")); err != nil {
		t.Fatal(err)
	}

	if _, err := commitSwitchJournal(ctx, git, repo, common); err == nil {
		t.Fatal("want the hook's rejection as an error")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("hook did not run: %v", err)
	}
	if head := runGit(t, dir, "rev-parse", "HEAD"); head != before {
		t.Fatalf("HEAD moved on a rejected commit")
	}
	if _, ok, err := loadSwitchJournal(common); err != nil || !ok {
		t.Fatalf("journal after failure: ok=%v err=%v, want kept", ok, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, ".docket.yml")); string(b) != "visibility: shared\nx: 1\n" {
		t.Fatalf("edit lost after failure: %q", b)
	}

	out, err := commitSwitchJournal(ctx, git, repo, common)
	if err != nil || out.Commit == "" || !out.Complete {
		t.Fatalf("retry: %+v %v", out, err)
	}
	if n := runGit(t, dir, "rev-list", "--count", before+"..HEAD"); n != "1" {
		t.Fatalf("retry made %s commits, want 1", n)
	}
}
