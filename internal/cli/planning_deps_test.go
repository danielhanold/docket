package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/testsupport"
)

// followGitOut runs git -C dir args and returns trimmed stdout, failing the test
// on any error. It uses testsupport.GitEnv like statusGit.
func followGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), testsupport.GitEnv(t)...)
	var out, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("git -C %s %s: %v: %s", dir, strings.Join(args, " "), err, errBuf.String())
	}
	return strings.TrimSpace(out.String())
}

// newFollowFixture builds a shared-layout clone whose attached .docket worktree is
// one commit behind origin's docket branch, with that commit's objects fetched
// into the clone. It returns the clone and the newer docket commit.
func newFollowFixture(t *testing.T) (invocation, newTip string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	t.Setenv("XDG_CONFIG_HOME", testsupport.TempDir(t))
	root := testsupport.TempDir(t)
	origin := filepath.Join(root, "origin.git")
	writer := filepath.Join(root, "writer")
	invocation = filepath.Join(root, "invocation")

	statusGit(t, root, "init", "--bare", "-b", "main", origin)
	statusGit(t, root, "init", "-b", "main", writer)
	statusGit(t, writer, "config", "user.name", "t")
	statusGit(t, writer, "config", "user.email", "t@t")
	statusGit(t, writer, "config", "commit.gpgsign", "false")
	statusWriteFile(t, writer, "README.md", "readme\n")
	statusGit(t, writer, "add", "-A")
	statusGit(t, writer, "commit", "-q", "-m", "main content")
	statusGit(t, writer, "remote", "add", "origin", origin)
	statusGit(t, writer, "push", "-q", "-u", "origin", "main")
	statusGit(t, writer, "checkout", "-q", "--orphan", "docket")
	statusGit(t, writer, "rm", "-rf", "-q", ".")
	statusWriteFile(t, writer, "docs/changes/BOARD.md", "# Board\n")
	statusGit(t, writer, "add", "-A")
	statusGit(t, writer, "commit", "-q", "-m", "docket: initialize metadata branch")
	statusGit(t, writer, "push", "-q", "-u", "origin", "docket")

	statusGit(t, root, "clone", "-q", origin, invocation)
	statusGit(t, invocation, "worktree", "add", "-q", "-b", "docket", filepath.Join(invocation, ".docket"), "origin/docket")

	statusWriteFile(t, writer, "docs/changes/BOARD.md", "# Board\n\nupdated\n")
	statusGit(t, writer, "commit", "-q", "-am", "docket: board refresh")
	statusGit(t, writer, "push", "-q", "origin", "docket")
	statusGit(t, invocation, "fetch", "-q", "origin", "docket")
	return invocation, followGitOut(t, invocation, "rev-parse", "refs/remotes/origin/docket")
}

// TestPlanningDepsWireMetadataCheckoutFollower pins the wiring in
// newPlanningDepsOver: with and without a feature repoDir, the assembled engine's
// AppliedHook moves the clone's .docket checkout to the applied commit. Deleting
// the wiring (nil hook) or wiring a no-op both fail it.
func TestPlanningDepsWireMetadataCheckoutFollower(t *testing.T) {
	for _, tc := range []struct {
		name        string
		withRepoDir bool
	}{{"standalone", false}, {"feature", true}} {
		t.Run(tc.name, func(t *testing.T) {
			invocation, newTip := newFollowFixture(t)
			client, err := gitcli.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			var deps app.PlanningDeps
			if tc.withRepoDir {
				deps, err = newPlanningDepsOver(client, invocation)
			} else {
				deps, err = newPlanningDepsOver(client)
			}
			if err != nil {
				t.Fatal(err)
			}
			engine, ok := deps.Engine.(*transaction.Engine)
			if !ok {
				t.Fatalf("deps.Engine is %T, want *transaction.Engine", deps.Engine)
			}
			if engine.AppliedHook == nil {
				t.Fatal("newPlanningDepsOver left the engine's AppliedHook unset")
			}
			repo, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: invocation})
			if err != nil {
				t.Fatal(err)
			}
			engine.AppliedHook(context.Background(),
				transaction.Request{Repository: repo, Remote: "origin", TargetRef: "refs/heads/docket"},
				transaction.Result{Disposition: transaction.DispositionApplied, AppliedCommit: gitcli.ObjectID(newTip)})
			dot := filepath.Join(invocation, ".docket")
			if head := followGitOut(t, dot, "rev-parse", "HEAD"); head != newTip {
				t.Fatalf(".docket HEAD = %s, want the applied commit %s", head, newTip)
			}
			if b, err := os.ReadFile(filepath.Join(dot, "docs", "changes", "BOARD.md")); err != nil || string(b) != "# Board\n\nupdated\n" {
				t.Fatalf("BOARD.md on disk = %q, %v; want the applied board", b, err)
			}
		})
	}
}
