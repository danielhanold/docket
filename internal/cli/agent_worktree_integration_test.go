package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/codex"
	"github.com/danielhanold/docket/internal/testsupport"
)

// This catches the production boundary regressing from the selected feature
// worktree to the caller/coordinator worktree: a resolver would then see an
// empty unmerged index while its owned rebase remains conflicted elsewhere.
func TestIntegrationAgentEnterFeatureResolverObservesSelectedWorktreeConflict(t *testing.T) {
	paths := newAgentEntryWorktrees(t)
	cleanupResolverConflictWorktrees(t, paths)
	prepareResolverRebaseConflict(t, paths)

	if got := gitUnmergedEntries(t, paths.a); got != "" {
		t.Fatalf("coordinator worktree %s has unmerged entries before entry: %q", paths.a, got)
	}
	if got := gitUnmergedEntries(t, paths.b); got == "" {
		t.Fatalf("feature worktree %s has no owned rebase conflict", paths.b)
	}

	seedAgentInstallation(t)
	stubDir := testsupport.TempDir(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	stub := "#!/bin/sh\nexec '" + strings.ReplaceAll(exe, "'", "'\\''") + "' -test.run=^TestAgentEnterResolverConflictServerProcess$ -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(stubDir, "codex"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DOCKET_AGENT_RESOLVER_CONFLICT_SERVER", "1")
	t.Setenv("DOCKET_AGENT_TEST_SKILL", "docket-convention")
	resolver := installedAgentTestContract(t, "docket-rebase-resolver")
	t.Setenv("DOCKET_AGENT_TEST_DEVELOPER", resolver.DeveloperInstructions)
	t.Setenv("DOCKET_AGENT_TEST_MODEL", resolver.Model)
	t.Setenv("DOCKET_AGENT_TEST_EFFORT", resolver.Effort)

	request := "Resolve only the owned rebase conflict.\nFeature worktree: " + paths.b + "\nPreserve these request bytes: `unchanged`.\n"
	t.Setenv("DOCKET_AGENT_TEST_REQUEST", request)

	var out, stderr bytes.Buffer
	code := Run([]string{"agent", "enter", "--role", "docket-rebase-resolver", "--request", "-", "--cwd", paths.a, "--worktree", paths.b, "--approval-policy", "never", "--sandbox", "workspace-write", "--json"}, strings.NewReader(request), &out, &stderr, devInfo(), hostFacts())
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d out=%q stderr=%q", code, out.String(), stderr.String())
	}
	var result app.AgentEnterResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Result != app.ResultApplied || result.Role != "docket-rebase-resolver" || result.ThreadID != "root" || result.TurnID != "turn" {
		t.Fatalf("receipt: %+v", result)
	}
	const observationPrefix = "resolver-observation cwd="
	if !strings.HasPrefix(result.Output, observationPrefix) {
		t.Fatalf("resolver output = %q, want %q prefix", result.Output, observationPrefix)
	}
	parts := strings.SplitN(strings.TrimPrefix(result.Output, observationPrefix), "\nunmerged:\n", 2)
	if len(parts) != 2 {
		t.Fatalf("resolver output has no unmerged observation: %q", result.Output)
	}
	if parts[1] == "" {
		t.Fatalf("resolver observed no unmerged entries from %s; original cross-worktree symptom", parts[0])
	}
	if parts[0] != paths.b {
		t.Fatalf("resolver thread/start.cwd = %q, want canonical feature worktree %q", parts[0], paths.b)
	}
	if !strings.Contains(parts[1], "conflict.txt") {
		t.Fatalf("resolver unmerged observation = %q, want conflict.txt", parts[1])
	}
}

func prepareResolverRebaseConflict(t *testing.T, paths agentEntryWorktrees) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(paths.a, "conflict.txt"), []byte("coordinator side\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, paths.a, "add", "--", "conflict.txt")
	gitCommand(t, paths.a, "commit", "-q", "-m", "coordinator conflict side")
	if err := os.WriteFile(filepath.Join(paths.b, "conflict.txt"), []byte("feature side\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, paths.b, "add", "--", "conflict.txt")
	gitCommand(t, paths.b, "commit", "-q", "-m", "feature conflict side")
	cmd := exec.Command("git", "-C", paths.b, "rebase", "feature-a")
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "CONFLICT") {
		t.Fatalf("git rebase feature-a: err=%v output=%q, want owned conflict", err, out)
	}
}

func cleanupResolverConflictWorktrees(t *testing.T, paths agentEntryWorktrees) {
	t.Helper()
	t.Cleanup(func() {
		for _, cleanup := range []struct {
			dir  string
			args []string
		}{{paths.b, []string{"rebase", "--abort"}}, {paths.primary, []string{"worktree", "remove", "--force", "--", paths.b}}, {paths.primary, []string{"worktree", "remove", "--force", "--", paths.a}}} {
			cmd := exec.Command("git", append([]string{"-C", cleanup.dir}, cleanup.args...)...)
			if out, err := cmd.CombinedOutput(); err != nil && !(cleanup.args[0] == "rebase" && strings.Contains(strings.ToLower(string(out)), "no rebase in progress")) {
				t.Errorf("cleanup git %s: %v\n%s", strings.Join(cleanup.args, " "), err, out)
			}
		}
	})
}

func gitUnmergedEntries(t *testing.T, worktree string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", worktree, "ls-files", "-u", "--", "conflict.txt")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git ls-files -u in %s: %v\n%s", worktree, err, out)
	}
	return string(out)
}

// The process is a narrow app-server replacement: it validates the generated
// resolver contract and executes only the read-only unmerged-index observation
// that the actual resolver needs from its received thread/start cwd.
func TestAgentEnterResolverConflictServerProcess(t *testing.T) {
	if os.Getenv("DOCKET_AGENT_RESOLVER_CONFLICT_SERVER") != "1" {
		return
	}
	if got := os.Args[len(os.Args)-2:]; got[0] != "app-server" || got[1] != "--stdio" {
		os.Exit(2)
	}
	var threadCWD string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var msg struct {
			Method string                     `json:"method"`
			Params map[string]json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			os.Exit(3)
		}
		switch msg.Method {
		case "initialize":
			fmt.Println(`{"id":1,"result":{}}`)
		case "thread/start":
			_ = json.Unmarshal(msg.Params["cwd"], &threadCWD)
			processCWD, err := os.Getwd()
			if err != nil || threadCWD == "" || processCWD != threadCWD {
				os.Exit(4)
			}
			for key, want := range map[string]string{"approvalPolicy": "never", "sandbox": "workspace-write", "threadSource": "vscode"} {
				var got string
				_ = json.Unmarshal(msg.Params[key], &got)
				if got != want {
					os.Exit(4)
				}
			}
			var developer string
			_ = json.Unmarshal(msg.Params["developerInstructions"], &developer)
			if developer != os.Getenv("DOCKET_AGENT_TEST_DEVELOPER") {
				os.Exit(5)
			}
			var model string
			_ = json.Unmarshal(msg.Params["model"], &model)
			if model != os.Getenv("DOCKET_AGENT_TEST_MODEL") {
				os.Exit(5)
			}
			fmt.Println(`{"id":2,"result":{"thread":{"id":"root"}}}`)
		case "turn/start":
			var effort string
			_ = json.Unmarshal(msg.Params["effort"], &effort)
			if effort != os.Getenv("DOCKET_AGENT_TEST_EFFORT") {
				os.Exit(5)
			}
			var inputs []struct{ Type, Text, Name, Path string }
			_ = json.Unmarshal(msg.Params["input"], &inputs)
			var text string
			skill := false
			for _, input := range inputs {
				if input.Type == "text" {
					text += input.Text
				}
				if input.Type == "skill" && input.Name == os.Getenv("DOCKET_AGENT_TEST_SKILL") && filepath.IsAbs(input.Path) {
					skill = true
				}
			}
			if text != os.Getenv("DOCKET_AGENT_TEST_REQUEST") || !skill {
				os.Exit(6)
			}
			unmerged, err := exec.Command("git", "-C", threadCWD, "ls-files", "-u", "--", "conflict.txt").CombinedOutput()
			if err != nil {
				os.Exit(7)
			}
			observation := "resolver-observation cwd=" + threadCWD + "\nunmerged:\n" + string(unmerged)
			fmt.Println(`{"id":3,"result":{"turn":{"id":"turn"}}}`)
			encoded, err := json.Marshal(observation)
			if err != nil {
				os.Exit(8)
			}
			fmt.Printf(`{"method":"turn/completed","params":{"threadId":"root","turn":{"id":"turn","status":"completed","items":[{"type":"agentMessage","text":%s}]}}}`+"\n", encoded)
		}
	}
	os.Exit(0)
}

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
