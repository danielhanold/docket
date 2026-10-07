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

// Init's pending list is what the human commits: the Cursor rule the managed
// .gitignore block ignores is never on it, and `git add` takes the whole list.
func TestIntegrationRepoSetupInitPendingOmitsIgnoredCursorRule(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	res := r.runInitWith(t, InitOptions{Harnesses: harnessFlag("claude", "cursor")})
	if res.Result != ResultApplied {
		t.Fatalf("Result = %q (%s), want applied", res.Result, res.HumanText())
	}
	if _, err := os.Stat(filepath.Join(r.invocation, ".cursor", "rules", "docket-dispatch.mdc")); err != nil {
		t.Fatalf("the cursor rule was not installed: %v", err)
	}
	if contains(res.PendingPaths, ".cursor/rules/docket-dispatch.mdc") {
		t.Errorf("PendingPaths = %v, want the gitignored cursor rule left off", res.PendingPaths)
	}
	if strings.Contains(res.HumanText(), "docket-dispatch.mdc") {
		t.Errorf("human text %q names the gitignored cursor rule", res.HumanText())
	}
	if out, err := tryGit(r.invocation, append([]string{"add", "--"}, res.PendingPaths...)...); err != nil {
		t.Errorf("git add of the pending paths %v failed: %v (%s)", res.PendingPaths, err, out)
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
	lay := expectedPrivateLayout(t, r.invocation, data)
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

// runConfigureHarnesses runs configure-harnesses against the invocation clone
// with the user roots pinned and a fresh isolated client.
func (r *initRepo) runConfigureHarnesses(t *testing.T, o ConfigureHarnessesOptions) RepositoryOpResult {
	t.Helper()
	pinInitUserRoots(t)
	client := newGitClient(t)
	res := RunRepositoryConfigureHarnesses(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation}, o)
	got, ok := res.(RepositoryOpResult)
	if !ok {
		t.Fatalf("configure-harnesses result is %T, want RepositoryOpResult", res)
	}
	return got
}

// newHarnessRepo inits a shared repository with --harnesses tokens and commits
// and pushes every pending path, so it starts healthy with that selection.
func newHarnessRepo(t *testing.T, tokens ...string) *initRepo {
	t.Helper()
	r := newInitRepo(t, "integration_branch: main\n", nil)
	res := r.runInitWith(t, InitOptions{Harnesses: harnessFlag(tokens...)})
	if res.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
	}
	r.commitAndPushMain(t, "commit init's pending edits", res.PendingPaths...)
	return r
}

func TestIntegrationRepoSetupConfigureHarnessesDropsAHarness(t *testing.T) {
	r := newHarnessRepo(t, "claude", "cursor")
	mdc := filepath.Join(r.invocation, ".cursor", "rules", "docket-dispatch.mdc")
	if _, err := os.Stat(mdc); err != nil {
		t.Fatalf("fixture: the Cursor rule is missing: %v", err)
	}
	res := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: harnessFlag("claude")})
	if res.Result != ResultApplied || res.RepositoryState != string(reposetup.StateNeedsReview) {
		t.Fatalf("Result = %q state %q (%s), want applied needs-review", res.Result, res.RepositoryState, res.HumanText())
	}
	if _, err := os.Stat(mdc); !os.IsNotExist(err) {
		t.Errorf("the Cursor rule still exists (err=%v); a dropped harness loses its surface", err)
	}
	if got := string(mustReadFile(t, filepath.Join(r.invocation, "CLAUDE.md"))); !strings.Contains(got, "docket:dispatch") {
		t.Errorf("CLAUDE.md = %q, want the docket:dispatch block kept", got)
	}
	// The Cursor rule is ignored by the managed .gitignore block, so its removal
	// is no reviewable change; the config edit is.
	if !contains(res.PendingPaths, ".docket.yml") {
		t.Errorf("PendingPaths = %v, want .docket.yml", res.PendingPaths)
	}

	res = r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: harnessFlag("none")})
	if res.Result != ResultApplied {
		t.Fatalf("none: Result = %q (%s), want applied", res.Result, res.HumanText())
	}
	if got, err := os.ReadFile(filepath.Join(r.invocation, "CLAUDE.md")); err == nil && strings.Contains(string(got), "docket:dispatch") {
		t.Errorf("CLAUDE.md = %q, want no docket:dispatch block after none", got)
	}
	// CLAUDE.md is tracked, so retiring its block (or the whole file) is a
	// pending review path even when the file was deleted.
	if !contains(res.PendingPaths, "CLAUDE.md") {
		t.Errorf("none: PendingPaths = %v, want CLAUDE.md", res.PendingPaths)
	}
	yml := string(mustReadFile(t, filepath.Join(r.invocation, ".docket.yml")))
	if strings.Count(yml, "agent_harnesses:") != 1 || !strings.Contains(yml, "agent_harnesses: []\n") {
		t.Errorf(".docket.yml = %q, want exactly one agent_harnesses: [] line", yml)
	}
}

func TestIntegrationRepoSetupConfigureHarnessesNoPreconditions(t *testing.T) {
	t.Run("dirty", func(t *testing.T) {
		r := newHarnessRepo(t, "claude")
		writeRepoFile(t, r.invocation, "README.md", "edited\n")
		res := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: harnessFlag("codex")})
		if res.Result == ResultInvalidState {
			t.Errorf("a dirty checkout was refused: %s", res.HumanText())
		}
	})
	t.Run("behind", func(t *testing.T) {
		r := newHarnessRepo(t, "claude")
		runGit(t, r.writer, "pull", "-q", "--ff-only", "origin", "main")
		r.advanceIntegration(t, "later.txt", "x\n")
		res := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: harnessFlag("codex")})
		if res.Result == ResultInvalidState {
			t.Errorf("a checkout behind the tip was refused: %s", res.HumanText())
		}
	})
	t.Run("needs-review", func(t *testing.T) {
		r := newInitRepo(t, defaultSetupYML, nil)
		if init := r.runInitWith(t, InitOptions{}); init.Result != ResultApplied {
			t.Fatalf("init = %q (%s), want applied", init.Result, init.HumanText())
		}
		res := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: harnessFlag("codex")})
		if res.Result == ResultInvalidState {
			t.Errorf("a needs-review repository was refused: %s", res.HumanText())
		}
	})
}

func TestIntegrationRepoSetupConfigureHarnessesRefusesFreshAndLegacy(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		remedy string
	}{
		{"fresh", nil, "docket repository init"},
		{"legacy", map[string]string{"docs/changes/active/0001-x.md": "---\nid: 1\n---\n"}, "docket repository migrate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newInitRepo(t, defaultSetupYML, tc.files)
			before := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
			res := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: harnessFlag("claude")})
			if res.Result != ResultInvalidState || !strings.Contains(res.HumanText(), tc.remedy) {
				t.Errorf("Result = %q (%s), want invalid-state naming %q", res.Result, res.HumanText(), tc.remedy)
			}
			if after := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml")); string(after) != string(before) {
				t.Errorf(".docket.yml changed on a refusal:\n%s", after)
			}
		})
	}
}

func TestIntegrationRepoSetupConfigureHarnessesNoFlagNoTerminalRefuses(t *testing.T) {
	r := newHarnessRepo(t, "claude")
	before := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
	res := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{})
	if res.Result != ResultInvalidInput || !strings.Contains(res.HumanText(), "--harnesses") {
		t.Errorf("Result = %q (%s), want invalid-input naming --harnesses", res.Result, res.HumanText())
	}
	if after := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml")); string(after) != string(before) {
		t.Errorf(".docket.yml changed:\n%s", after)
	}
}

func TestIntegrationRepoSetupConfigureHarnessesRefusesBadTokens(t *testing.T) {
	r := newHarnessRepo(t, "claude")
	before := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
	for _, opts := range []HarnessesOptions{harnessFlag("claude", "claude"), harnessFlag("none", "claude"), harnessFlag("bogus")} {
		res := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: opts})
		if res.Result != ResultInvalidInput {
			t.Errorf("%v: Result = %q (%s), want invalid-input", opts.Tokens, res.Result, res.HumanText())
		}
		if after := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml")); string(after) != string(before) {
			t.Errorf("%v: .docket.yml changed:\n%s", opts.Tokens, after)
		}
	}
}

func TestIntegrationRepoSetupConfigureHarnessesPrechecksRepoValue(t *testing.T) {
	r := newHarnessRepo(t, "claude", "cursor")
	var req HarnessChoiceRequest
	calls := 0
	res := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: HarnessesOptions{Chooser: recordingChooser(&req, &calls, []string{"claude"}, nil)}})
	if res.Result != ResultApplied {
		t.Fatalf("Result = %q (%s), want applied", res.Result, res.HumanText())
	}
	if calls != 1 || !reflect.DeepEqual(req.Preselected, []string{"claude", "cursor"}) {
		t.Errorf("calls = %d, Preselected = %v; want 1 call pre-checking [claude cursor]", calls, req.Preselected)
	}
	if got := string(mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))); !strings.Contains(got, "agent_harnesses: [claude]\n") {
		t.Errorf(".docket.yml = %q, want agent_harnesses: [claude]", got)
	}
}

func TestIntegrationRepoSetupConfigureHarnessesTwiceInNeedsReview(t *testing.T) {
	r := newHarnessRepo(t, "cursor")
	first := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: harnessFlag("claude")})
	if first.Result != ResultApplied || first.RepositoryState != string(reposetup.StateNeedsReview) {
		t.Fatalf("first = %q state %q (%s), want applied needs-review", first.Result, first.RepositoryState, first.HumanText())
	}
	second := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: harnessFlag("claude")})
	if second.Result != ResultNoOp {
		t.Errorf("second = %q (%s), want no-op", second.Result, second.HumanText())
	}
	if got := string(mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))); strings.Count(got, "agent_harnesses") != 1 {
		t.Errorf(".docket.yml = %q, want exactly one agent_harnesses line", got)
	}
}

// An uncommitted .gitignore edit unrelated to docket is not a pending review
// path once the managed block is committed: configure-harnesses decides the
// .gitignore from the committed-ignore fact, as repository check does, so a
// healthy repository stays healthy.
func TestIntegrationRepoSetupConfigureHarnessesIgnoresUnrelatedGitignoreEdit(t *testing.T) {
	r := newHarnessRepo(t, "claude")
	gi := filepath.Join(r.invocation, ".gitignore")
	writeRepoFile(t, r.invocation, ".gitignore", string(mustReadFile(t, gi))+"unrelated-build-output/\n")
	res := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: harnessFlag("claude")})
	if res.Result != ResultNoOp {
		t.Fatalf("Result = %q (%s), want no-op", res.Result, res.HumanText())
	}
	if contains(res.PendingPaths, ".gitignore") || res.RepositoryState != string(reposetup.StateHealthy) {
		t.Errorf("PendingPaths = %v state %q, want no .gitignore and healthy", res.PendingPaths, res.RepositoryState)
	}
}

func TestIntegrationRepoSetupConfigureHarnessesLocalOverrideWarns(t *testing.T) {
	r := newHarnessRepo(t, "claude")
	writeRepoFile(t, r.invocation, ".docket.local.yml", "agent_harnesses: [cursor]\n")
	res := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: harnessFlag("codex")})
	if res.Result != ResultApplied {
		t.Fatalf("Result = %q (%s), want applied", res.Result, res.HumanText())
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], ".docket.local.yml") || !strings.Contains(res.Warnings[0], "`[cursor]`") {
		t.Errorf("Warnings = %v, want one naming .docket.local.yml and `[cursor]`", res.Warnings)
	}
	if got := string(mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))); !strings.Contains(got, "agent_harnesses: [codex]\n") {
		t.Errorf(".docket.yml = %q, want agent_harnesses: [codex]", got)
	}
	if _, err := os.Stat(filepath.Join(r.invocation, ".cursor", "rules", "docket-dispatch.mdc")); err != nil {
		t.Errorf("the applied value is cursor, but its rule is missing: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(r.invocation, "AGENTS.md")); err == nil && strings.Contains(string(got), "docket:dispatch") {
		t.Errorf("AGENTS.md = %q, want no docket:dispatch block while the override applies cursor", got)
	}
}

func TestIntegrationRepoSetupPrivateConfigureHarnesses(t *testing.T) {
	r, data := initPrivateHealthy(t)
	res := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: harnessFlag("claude")})
	if res.Result != ResultApplied || res.RepositoryState != string(reposetup.StateHealthy) {
		t.Fatalf("Result = %q state %q (%s), want applied healthy", res.Result, res.RepositoryState, res.HumanText())
	}
	if len(res.PendingPaths) != 0 {
		t.Errorf("PendingPaths = %v, want none for a private repository", res.PendingPaths)
	}
	if !strings.Contains(res.HumanText(), ".git/dckt/config.yml") {
		t.Errorf("human text %q does not name .git/dckt/config.yml", res.HumanText())
	}
	lay := expectedPrivateLayout(t, r.invocation, data)
	if got := string(mustReadFile(t, lay.ConfigPath)); !strings.Contains(got, "agent_harnesses: [claude]\n") {
		t.Errorf("%s = %q, want agent_harnesses: [claude]", lay.ConfigPath, got)
	}
	agents := filepath.Join(filepath.Dir(lay.ConfigPath), "AGENTS.md")
	if got, err := os.ReadFile(agents); err != nil || !strings.Contains(string(got), "docket:dispatch") {
		t.Errorf("%s = %q (%v), want the docket:dispatch block", agents, got, err)
	}
}
