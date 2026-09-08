package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/codex"
	"github.com/danielhanold/docket/internal/testsupport"
)

func TestResolveAgentEntryCWDRoleScopeMatrix(t *testing.T) {
	paths := newAgentEntryWorktrees(t)
	git := newAgentEntryGitClient(t)
	feature := codex.RoleContract{Name: "docket-rebase-resolver", LaunchPosture: harness.LaunchChild, WorktreeScope: harness.WorktreeScopeFeature}
	linkedB := filepath.Join(paths.root, "b-by-symlink")
	if err := os.Symlink(paths.b, linkedB); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		contract   codex.RoleContract
		caller     string
		worktree   string
		wantCWD    string
		wantReason string
	}{
		{
			name:     "root coordinator metadata preserves caller cwd bytes",
			contract: codex.RoleContract{Name: "docket-implement-next", LaunchPosture: harness.LaunchRootCoordinator, WorktreeScope: harness.WorktreeScopeMetadata},
			caller:   paths.a + string(filepath.Separator) + ".",
			wantCWD:  paths.a + string(filepath.Separator) + ".",
		},
		{
			name:       "root coordinator rejects worktree as contradictory",
			contract:   codex.RoleContract{Name: "docket-implement-next", LaunchPosture: harness.LaunchRootCoordinator, WorktreeScope: harness.WorktreeScopeMetadata},
			caller:     paths.a,
			worktree:   paths.b,
			wantReason: "worktree-contradicts-role",
		},
		{
			name:       "metadata child retains native dispatch refusal",
			contract:   codex.RoleContract{Name: "docket-review-standard", LaunchPosture: harness.LaunchChild, WorktreeScope: harness.WorktreeScopeMetadata},
			caller:     paths.a,
			worktree:   paths.b,
			wantReason: "ordinary-child-role",
		},
		{
			name:       "feature child requires a worktree",
			contract:   feature,
			caller:     paths.a,
			wantReason: "worktree-required",
		},
		{
			name:     "feature child enters exact target worktree",
			contract: feature,
			caller:   paths.a,
			worktree: paths.b,
			wantCWD:  paths.b,
		},
		{
			name:     "feature child canonicalizes a symlink spelling of target",
			contract: feature,
			caller:   paths.a,
			worktree: linkedB,
			wantCWD:  paths.b,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotCWD, gotReason, err := resolveAgentEntryCWD(context.Background(), git, tc.contract, tc.caller, tc.worktree)
			if err != nil {
				t.Fatalf("resolveAgentEntryCWD: %v", err)
			}
			if gotCWD != tc.wantCWD || gotReason != tc.wantReason {
				t.Fatalf("resolveAgentEntryCWD = (%q, %q), want (%q, %q)", gotCWD, gotReason, tc.wantCWD, tc.wantReason)
			}
		})
	}
}

func TestResolveAgentEntryCWDRejectsInvalidFeatureWorktrees(t *testing.T) {
	paths := newAgentEntryWorktrees(t)
	git := newAgentEntryGitClient(t)
	feature := codex.RoleContract{Name: "docket-rebase-resolver", LaunchPosture: harness.LaunchChild, WorktreeScope: harness.WorktreeScopeFeature}
	nested := filepath.Join(paths.b, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	notDirectory := filepath.Join(paths.root, "not-a-directory")
	if err := os.WriteFile(notDirectory, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, worktree, wantReason string
	}{
		{"nonexistent target", filepath.Join(paths.root, "missing"), "worktree-not-directory"},
		{"non-directory target", notDirectory, "worktree-not-directory"},
		{"primary target", paths.primary, "worktree-primary"},
		{"nested target", nested, "worktree-not-root"},
		{"unregistered target", paths.unregistered, "worktree-unregistered"},
		{"foreign repository target", paths.foreign, "worktree-foreign-repository"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotCWD, gotReason, err := resolveAgentEntryCWD(context.Background(), git, feature, paths.a, tc.worktree)
			if err != nil {
				t.Fatalf("resolveAgentEntryCWD: %v", err)
			}
			if gotCWD != "" || gotReason != tc.wantReason {
				t.Fatalf("resolveAgentEntryCWD = (%q, %q), want (%q, %q)", gotCWD, gotReason, "", tc.wantReason)
			}
		})
	}
}

// ListWorktrees permits a relative Path spelling. The registration comparison
// must convert that spelling to the same absolute canonical root used by
// DiscoverWorktree before deciding whether a target remains registered.
func TestCanonicalAgentEntryWorktreePathMakesRelativeRegistrationAbsolute(t *testing.T) {
	paths := newAgentEntryWorktrees(t)
	t.Chdir(paths.root)
	registration := gitcli.WorktreeInfo{Path: "b"}
	got, err := canonicalAgentEntryWorktreePath(registration.Path)
	if err != nil {
		t.Fatalf("canonicalAgentEntryWorktreePath(%q): %v", registration.Path, err)
	}
	if got != paths.b {
		t.Fatalf("canonical registration path = %q, want %q", got, paths.b)
	}
}

type agentEntryWorktrees struct {
	root, primary, a, b, unregistered, foreign string
}

func newAgentEntryWorktrees(t *testing.T) agentEntryWorktrees {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	root := testsupport.TempDir(t)
	primary := filepath.Join(root, "primary")
	gitCommand(t, root, "init", "-q", "--initial-branch=main", primary)
	gitCommand(t, primary, "config", "user.email", "agent-enter@example.test")
	gitCommand(t, primary, "config", "user.name", "Agent Enter")
	if err := os.WriteFile(filepath.Join(primary, "README"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, primary, "add", "--", "README")
	gitCommand(t, primary, "commit", "-q", "-m", "seed")

	paths := agentEntryWorktrees{root: root, primary: primary, a: filepath.Join(root, "a"), b: filepath.Join(root, "b"), unregistered: filepath.Join(root, "unregistered")}
	gitCommand(t, primary, "worktree", "add", "-q", "-b", "feature-a", "--", paths.a, "HEAD")
	gitCommand(t, primary, "worktree", "add", "-q", "-b", "feature-b", "--", paths.b, "HEAD")
	gitCommand(t, primary, "worktree", "add", "-q", "-b", "unregistered", "--", paths.unregistered, "HEAD")

	gitFile, err := os.ReadFile(filepath.Join(paths.unregistered, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	gitDir := strings.TrimSpace(strings.TrimPrefix(string(gitFile), "gitdir: "))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(paths.unregistered, gitDir)
	}
	if err := os.RemoveAll(gitDir); err != nil {
		t.Fatal(err)
	}

	foreignPrimary := filepath.Join(root, "foreign-primary")
	paths.foreign = filepath.Join(root, "foreign")
	gitCommand(t, root, "init", "-q", "--initial-branch=main", foreignPrimary)
	gitCommand(t, foreignPrimary, "config", "user.email", "agent-enter@example.test")
	gitCommand(t, foreignPrimary, "config", "user.name", "Agent Enter")
	if err := os.WriteFile(filepath.Join(foreignPrimary, "README"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, foreignPrimary, "add", "--", "README")
	gitCommand(t, foreignPrimary, "commit", "-q", "-m", "seed")
	gitCommand(t, foreignPrimary, "worktree", "add", "-q", "-b", "foreign-feature", "--", paths.foreign, "HEAD")

	for _, p := range []*string{&paths.primary, &paths.a, &paths.b, &paths.unregistered, &paths.foreign} {
		canonical, err := filepath.EvalSymlinks(*p)
		if err != nil {
			t.Fatal(err)
		}
		*p = canonical
	}
	return paths
}

func newAgentEntryGitClient(t *testing.T) *gitcli.Client {
	t.Helper()
	client, err := gitcli.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func gitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
