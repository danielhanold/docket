//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// This shard file carries `docket repository init`'s harness choice (prefix
// TestIntegrationRepoSetup, matched by tests/test_go_integration_app_reposetup.sh):
// init resolves agent_harnesses from --harnesses or the interactive chooser
// before any write, writes it to the repository's config target, and installs
// the dispatch surfaces that value authorizes in the same run.

// harnessFlag is a --harnesses value as the CLI passes it: Set, with tokens.
func harnessFlag(tokens ...string) HarnessesOptions {
	return HarnessesOptions{Set: true, Tokens: tokens}
}

// runInitInHome runs init with the user roots pinned to a fresh temp home in
// which each of homeDirs exists, so harness detection sees exactly those.
func (r *initRepo) runInitInHome(t *testing.T, o InitOptions, homeDirs ...string) RepositoryOpResult {
	t.Helper()
	pinInitUserRoots(t)
	home := os.Getenv("HOME")
	for _, dir := range homeDirs {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	client := newGitClient(t)
	return RunRepositoryInit(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation}, o)
}

// failingChooser is a chooser the test expects never to be asked.
func failingChooser(t *testing.T) HarnessChooser {
	return func(context.Context, HarnessChoiceRequest) ([]string, error) {
		t.Error("the harness chooser was called; want it never asked")
		return nil, ErrHarnessChoiceCancelled
	}
}

func TestIntegrationRepoSetupInitHarnessesFlagShared(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	res := r.runInitWith(t, InitOptions{Harnesses: harnessFlag("claude")})
	if res.Result != ResultApplied {
		t.Fatalf("Result = %q (%s), want applied", res.Result, res.HumanText())
	}
	if got := string(mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))); !strings.Contains(got, "\nagent_harnesses: [claude]\n") {
		t.Errorf(".docket.yml = %q, want it to carry agent_harnesses: [claude]", got)
	}
	if got := string(mustReadFile(t, filepath.Join(r.invocation, "CLAUDE.md"))); !strings.Contains(got, "docket:dispatch") {
		t.Errorf("CLAUDE.md = %q, want the docket:dispatch block written this run", got)
	}
	for _, want := range []string{".docket.yml", "CLAUDE.md", ".gitignore"} {
		if !contains(res.PendingPaths, want) {
			t.Errorf("PendingPaths = %v, want %s", res.PendingPaths, want)
		}
	}
	if res.AgentHarnesses == nil || !reflect.DeepEqual(*res.AgentHarnesses, []string{"claude"}) {
		t.Errorf("AgentHarnesses = %v, want [claude]", res.AgentHarnesses)
	}
	if staged := runGit(t, r.invocation, "diff", "--cached", "--name-only"); staged != "" {
		t.Errorf("init staged %q; want nothing staged", staged)
	}
}

func TestIntegrationRepoSetupPrivateInitHarnessesFlag(t *testing.T) {
	r, data := newPrivateInitRepo(t, nil)
	res := r.runInitWith(t, InitOptions{Private: true, Harnesses: harnessFlag("claude")})
	if res.Result != ResultApplied {
		t.Fatalf("Result = %q (%s), want applied", res.Result, res.HumanText())
	}
	// The trap first: the gather-time layout of a fresh repository is shared,
	// and writing through it would create .docket.yml in the working tree.
	if _, err := os.Stat(filepath.Join(r.invocation, ".docket.yml")); !os.IsNotExist(err) {
		t.Errorf(".docket.yml exists in the clone (err=%v); a private init must never create it", err)
	}
	if len(res.PendingPaths) != 0 {
		t.Errorf("PendingPaths = %v, want none for a private init", res.PendingPaths)
	}
	if status := runGit(t, r.invocation, "status", "--porcelain"); status != "" {
		t.Errorf("git status --porcelain = %q, want a clean working tree", status)
	}
	lay := privateLayoutOf(t, r.invocation, data)
	if got := string(mustReadFile(t, lay.ConfigPath)); !strings.Contains(got, "agent_harnesses: [claude]\n") {
		t.Errorf("%s = %q, want agent_harnesses: [claude]", lay.ConfigPath, got)
	}
	agents := filepath.Join(filepath.Dir(lay.ConfigPath), "AGENTS.md")
	if got, err := os.ReadFile(agents); err != nil || !strings.Contains(string(got), "docket:dispatch") {
		t.Errorf("%s = %q (%v), want the docket:dispatch block", agents, got, err)
	}
	if res.AgentHarnesses == nil || !reflect.DeepEqual(*res.AgentHarnesses, []string{"claude"}) {
		t.Errorf("AgentHarnesses = %v, want [claude]", res.AgentHarnesses)
	}
}

func TestIntegrationRepoSetupInitInteractivePrecheckGlobal(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	var req HarnessChoiceRequest
	calls := 0
	res := r.runInitWithGlobal(t, "agent_harnesses: [codex]\n",
		InitOptions{Harnesses: HarnessesOptions{Chooser: recordingChooser(&req, &calls, []string{"cursor"}, nil)}})
	if res.Result != ResultApplied {
		t.Fatalf("Result = %q (%s), want applied", res.Result, res.HumanText())
	}
	if calls != 1 {
		t.Errorf("chooser calls = %d, want 1", calls)
	}
	if !reflect.DeepEqual(req.Preselected, []string{"codex"}) {
		t.Errorf("Preselected = %v, want the global [codex]", req.Preselected)
	}
	if req.ConfigPath != ".docket.yml" {
		t.Errorf("ConfigPath = %q, want .docket.yml", req.ConfigPath)
	}
	if got := string(mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))); !strings.Contains(got, "agent_harnesses: [cursor]\n") {
		t.Errorf(".docket.yml = %q, want agent_harnesses: [cursor]", got)
	}
	if _, err := os.Stat(filepath.Join(r.invocation, ".cursor", "rules", "docket-dispatch.mdc")); err != nil {
		t.Errorf("the Cursor dispatch rule was not written this run: %v", err)
	}
}

func TestIntegrationRepoSetupInitInteractivePrecheckDetected(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	var req HarnessChoiceRequest
	calls := 0
	res := r.runInitInHome(t, InitOptions{Harnesses: HarnessesOptions{Chooser: recordingChooser(&req, &calls, []string{}, nil)}}, ".cursor")
	if res.Result != ResultApplied {
		t.Fatalf("Result = %q (%s), want applied", res.Result, res.HumanText())
	}
	if !reflect.DeepEqual(req.Preselected, []string{"cursor"}) {
		t.Errorf("Preselected = %v, want the detected [cursor]", req.Preselected)
	}
	if got := string(mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))); !strings.Contains(got, "agent_harnesses: []\n") {
		t.Errorf(".docket.yml = %q, want agent_harnesses: []", got)
	}
	if res.AgentHarnesses == nil || *res.AgentHarnesses == nil || len(*res.AgentHarnesses) != 0 {
		t.Errorf("AgentHarnesses = %v, want a non-nil empty selection", res.AgentHarnesses)
	}
	if _, err := os.Stat(filepath.Join(r.invocation, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Errorf("CLAUDE.md exists (err=%v); an empty selection writes no surface", err)
	}
}

func TestIntegrationRepoSetupInitInteractiveCancelWritesNothing(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	before := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
	res := r.runInitWith(t, InitOptions{Harnesses: HarnessesOptions{Chooser: recordingChooser(nil, nil, nil, ErrHarnessChoiceCancelled)}})
	if res.Result != ResultInterrupted {
		t.Fatalf("Result = %q (%s), want interrupted", res.Result, res.HumanText())
	}
	if r.remoteBranchExists(t, "docket") {
		t.Error("a cancelled init published the remote docket branch")
	}
	if _, err := os.Stat(filepath.Join(r.invocation, ".docket")); !os.IsNotExist(err) {
		t.Errorf(".docket exists after a cancelled init (err=%v)", err)
	}
	gi, err := os.ReadFile(filepath.Join(r.invocation, ".gitignore"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if reposetup.ValidGitignoreBlock(gi) {
		t.Error("a cancelled init wrote the managed .gitignore block")
	}
	if after := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml")); string(after) != string(before) {
		t.Errorf(".docket.yml changed on a cancelled init:\n%s", after)
	}
}

func TestIntegrationRepoSetupInitNoInputWarns(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	res := r.runInitWith(t, InitOptions{})
	if res.Result != ResultApplied {
		t.Fatalf("Result = %q (%s), want applied", res.Result, res.HumanText())
	}
	text := res.HumanText()
	for _, want := range []string{"`agent_harnesses: [claude]`", ".docket.yml", reposetup.ConfigureHarnessesCommand} {
		if !strings.Contains(text, want) {
			t.Errorf("human text lacks %q:\n%s", want, text)
		}
	}
	if len(res.Warnings) != 1 {
		t.Errorf("Warnings = %v, want exactly one", res.Warnings)
	}
	if got := string(mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))); strings.Contains(got, "agent_harnesses") {
		t.Errorf(".docket.yml = %q, want no agent_harnesses written", got)
	}
	if _, err := os.Stat(filepath.Join(r.invocation, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Errorf("CLAUDE.md exists (err=%v); want none without a choice", err)
	}
}

func TestIntegrationRepoSetupInitKeepsDeclaredValue(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML+"agent_harnesses: [codex]\n", nil)
	res := r.runInitWith(t, InitOptions{Harnesses: HarnessesOptions{Chooser: failingChooser(t)}})
	if res.Result != ResultApplied {
		t.Fatalf("Result = %q (%s), want applied", res.Result, res.HumanText())
	}
	if got := string(mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))); strings.Count(got, "agent_harnesses: [codex]") != 1 {
		t.Errorf(".docket.yml = %q, want agent_harnesses: [codex] exactly once", got)
	}
	if got := string(mustReadFile(t, filepath.Join(r.invocation, "AGENTS.md"))); !strings.Contains(got, "docket:dispatch") {
		t.Errorf("AGENTS.md = %q, want the docket:dispatch block", got)
	}
	if res.AgentHarnesses != nil {
		t.Errorf("AgentHarnesses = %v, want nil for a kept value", *res.AgentHarnesses)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", res.Warnings)
	}
}

func TestIntegrationRepoSetupInitRerunDoesNotReask(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	first := r.runInitWith(t, InitOptions{Harnesses: harnessFlag("claude")})
	if first.Result != ResultApplied {
		t.Fatalf("first init = %q (%s), want applied", first.Result, first.HumanText())
	}
	second := r.runInitWith(t, InitOptions{Harnesses: HarnessesOptions{Chooser: failingChooser(t)}})
	if second.Result != ResultNoOp && second.Result != ResultApplied {
		t.Fatalf("second init = %q (%s), want no-op or applied", second.Result, second.HumanText())
	}
	if got := string(mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))); strings.Count(got, "agent_harnesses") != 1 {
		t.Errorf(".docket.yml = %q, want exactly one agent_harnesses line", got)
	}
}

func TestIntegrationRepoSetupInitRefusesBadHarnessesBeforeAnyWrite(t *testing.T) {
	cases := []HarnessesOptions{
		harnessFlag("bogus", "claude"),
		harnessFlag("claude", "claude"),
		harnessFlag("none", "claude"),
		harnessFlag(""),
	}
	for i, opts := range cases {
		r := newInitRepo(t, defaultSetupYML, nil)
		before := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
		res := r.runInitWith(t, InitOptions{Harnesses: opts})
		if res.Result != ResultInvalidInput {
			t.Errorf("%v: Result = %q (%s), want invalid-input", opts.Tokens, res.Result, res.HumanText())
		}
		if i == 0 && !strings.Contains(res.HumanText(), "bogus") {
			t.Errorf("%v: human text %q does not name bogus", opts.Tokens, res.HumanText())
		}
		if r.remoteBranchExists(t, "docket") {
			t.Errorf("%v: a refused init published the remote docket branch", opts.Tokens)
		}
		if after := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml")); string(after) != string(before) {
			t.Errorf("%v: .docket.yml changed on a refused init:\n%s", opts.Tokens, after)
		}
	}
}
