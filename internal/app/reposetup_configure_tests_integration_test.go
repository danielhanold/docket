//go:build integration

package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/reposetup"
)

// This shard file carries the `docket repository configure-tests` upgrade-path
// integration tests (change 0374, prefix TestIntegrationRepoSetupConfigureTests,
// matched by the existing reposetup runner tests/test_go_integration_app_reposetup.sh).
// configure-tests is the setup-time regenerator of the pending `.docket.yml`
// build/finalize test policy for an already-initialized (healthy) repository: it
// discovers the suite over the primary worktree and leaves the generated edit
// UNSTAGED for human review, exactly like init — it never commits or stages.

// runConfigureTests runs a discovery configure-tests (no --command).
func (r *initRepo) runConfigureTests(t *testing.T) RepositoryOpResult {
	t.Helper()
	return r.runConfigureTestsWith(t, ConfigureTestsOptions{})
}

// runConfigureTestsWith runs RunRepositoryConfigureTests against the invocation
// clone with a fresh isolated client and type-asserts the concrete result.
func (r *initRepo) runConfigureTestsWith(t *testing.T, o ConfigureTestsOptions) RepositoryOpResult {
	t.Helper()
	client := newGitClient(t)
	res := RunRepositoryConfigureTests(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation}, o)
	got, ok := res.(RepositoryOpResult)
	if !ok {
		t.Fatalf("configure-tests result is %T, want RepositoryOpResult", res)
	}
	return got
}

// TestIntegrationRepoSetupConfigureTestsWritesPendingForDetectedSuite proves the
// upgrade path: a healthy repository whose committed policy has no command, but
// whose primary tree now carries a detectable suite, gets a pending, UNSTAGED
// `.docket.yml` edit setting the detected command on both build and finalize.
// Re-running after the edit is committed is an idempotent no-op.
func TestIntegrationRepoSetupConfigureTestsWritesPendingForDetectedSuite(t *testing.T) {
	// A healthy repo with no detectable suite: init writes `gate: "off"` for both
	// and the pending edits are committed to reach healthy.
	r := newHealthyRepo(t)

	// A Go suite appears and is committed to the integration tip, keeping the
	// primary clean and at the remote tip (still healthy), but the committed
	// `.docket.yml` still declares no command.
	writeRepoFile(t, r.invocation, "go.mod", "module example.com/x\n\ngo 1.22\n")
	writeRepoFile(t, r.invocation, "x_test.go", "package x\n")
	r.commitAndPushMain(t, "add a Go test suite", "go.mod", "x_test.go")

	res := r.runConfigureTests(t)
	if res.Result != ResultApplied {
		t.Fatalf("configure-tests = %q (%s), want applied", res.Result, res.HumanText())
	}
	if res.RepositoryState != string(reposetup.StateNeedsReview) {
		t.Errorf("state = %q, want needs-review (a pending edit was written)", res.RepositoryState)
	}
	if !contains(res.PendingPaths, ".docket.yml") {
		t.Errorf("PendingPaths = %v, want the generated .docket.yml", res.PendingPaths)
	}

	got := string(mustReadFile(t, filepath.Join(r.invocation, ".docket.yml")))
	if strings.Count(got, "test_command: go test ./...") != 2 {
		t.Errorf(".docket.yml must set the detected command on build and finalize:\n%s", got)
	}

	// Never staged: the generated config is human-gated, shown as an unstaged edit.
	staged := runGit(t, r.invocation, "diff", "--cached", "--name-only")
	if strings.Contains(staged, ".docket.yml") {
		t.Errorf("configure-tests staged .docket.yml (must be human-gated); staged: %q", staged)
	}
	unstaged := runGit(t, r.invocation, "diff", "--name-only")
	if !strings.Contains(unstaged, ".docket.yml") {
		t.Errorf("configure-tests did not leave .docket.yml unstaged; unstaged: %q", unstaged)
	}

	// Commit the pending edit (back to healthy, now with explicit commands) and
	// re-run: an already-configured pair is an idempotent no-op with no rewrite.
	before := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
	r.commitAndPushMain(t, "commit generated test policy", ".docket.yml")
	second := r.runConfigureTests(t)
	if second.Result != ResultNoOp {
		t.Errorf("re-run over a configured pair = %q (%s), want no-op", second.Result, second.HumanText())
	}
	after := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
	if string(before) != string(after) {
		t.Errorf("configure-tests rewrote an already-configured .docket.yml; want byte-identical")
	}
}

// TestIntegrationRepoSetupConfigureTestsAmbiguousLeavesFileUntouched proves an
// ambiguous discovery writes nothing (no guess), leaves the committed file
// byte-untouched, and names the candidate families with the configure-tests
// remedy so a human chooses.
func TestIntegrationRepoSetupConfigureTestsAmbiguousLeavesFileUntouched(t *testing.T) {
	r := newHealthyRepo(t)
	// Two independent suite families → ambiguous.
	writeRepoFile(t, r.invocation, "go.mod", "module example.com/x\n\ngo 1.22\n")
	writeRepoFile(t, r.invocation, "x_test.go", "package x\n")
	writeRepoFile(t, r.invocation, "Cargo.toml", "[package]\nname = \"x\"\n")
	r.commitAndPushMain(t, "add two suite families", "go.mod", "x_test.go", "Cargo.toml")

	before := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
	res := r.runConfigureTests(t)
	if res.Result != ResultNoOp {
		t.Fatalf("ambiguous configure-tests = %q (%s), want no-op (nothing written)", res.Result, res.HumanText())
	}
	after := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
	if string(before) != string(after) {
		t.Errorf("ambiguous discovery must leave .docket.yml byte-untouched")
	}
	human := res.HumanText()
	if !strings.Contains(human, "go") || !strings.Contains(human, "rust") {
		t.Errorf("ambiguous note %q must name the candidate families (go, rust)", human)
	}
	if !strings.Contains(human, reposetup.ConfigureTestsCommandRemedy) {
		t.Errorf("ambiguous note %q must name the configure-tests --command remedy %q", human, reposetup.ConfigureTestsCommandRemedy)
	}
}

// TestIntegrationRepoSetupConfigureTestsRefusesFresh proves configure-tests is an
// upgrade path, not a bootstrap: a fresh (uninitialized) repository is refused
// with the remedy naming init, and nothing is written.
func TestIntegrationRepoSetupConfigureTestsRefusesFresh(t *testing.T) {
	r := newInitRepo(t, healthySetupYML, nil)
	res := r.runConfigureTests(t)
	if res.Result != ResultInvalidState {
		t.Fatalf("fresh configure-tests = %q (%s), want invalid-state", res.Result, res.HumanText())
	}
	if res.RepositoryState != string(reposetup.StateFresh) {
		t.Errorf("state = %q, want fresh", res.RepositoryState)
	}
	if !strings.Contains(res.HumanText(), "docket repository init") {
		t.Errorf("fresh remedy %q must name `docket repository init`", res.HumanText())
	}
}

// resolveDocketYML resolves the invocation clone's .docket.yml through the real
// config resolver, so asserts read what docket will actually run.
func resolveDocketYML(t *testing.T, r *initRepo) config.Effective {
	t.Helper()
	data := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
	snap, _, err := config.Resolve([]config.Source{{
		Layer: config.LayerRepository, Name: ".docket.yml", Data: data,
	}}, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("resolving .docket.yml: %v\n%s", err, data)
	}
	return snap.Effective
}

// TestIntegrationRepoSetupConfigureTestsCommandTurnsOnRootTestSh is the
// acceptance-fixture shape: a repository whose only test is a root test.sh
// (no registered detector family) is initialized with both gates off, plain
// configure-tests says so truthfully and names --command, and
// `configure-tests --command "sh ./test.sh"` turns both gates on. configure-tests
// never runs the command, so the file's mode is irrelevant here.
func TestIntegrationRepoSetupConfigureTestsCommandTurnsOnRootTestSh(t *testing.T) {
	r := newInitRepo(t, healthySetupYML, map[string]string{"test.sh": "#!/bin/sh\nexit 0\n"})

	initRes := r.runInit(t)
	if initRes.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", initRes.Result, initRes.HumanText())
	}
	if !strings.Contains(initRes.HumanText(), reposetup.ConfigureTestsCommandRemedy) {
		t.Errorf("init's none note %q must name %q", initRes.HumanText(), reposetup.ConfigureTestsCommandRemedy)
	}
	if eff := resolveDocketYML(t, r); eff.Build.Gate.Value != "off" || eff.Finalize.Gate.Value != "off" {
		t.Fatalf("init on a no-suite repo must write both gates off, got build=%q finalize=%q", eff.Build.Gate.Value, eff.Finalize.Gate.Value)
	}
	r.commitAndPushMain(t, "commit init's pending edits", ".gitignore", ".docket.yml")

	plain := r.runConfigureTests(t)
	if plain.Result != ResultNoOp {
		t.Fatalf("plain configure-tests = %q (%s), want no-op", plain.Result, plain.HumanText())
	}
	if strings.Contains(plain.HumanText(), "already configured") || !strings.Contains(plain.HumanText(), reposetup.ConfigureTestsCommandRemedy) {
		t.Errorf("plain configure-tests on none %q must name --command, not claim already configured", plain.HumanText())
	}

	cmd := "sh ./test.sh"
	res := r.runConfigureTestsWith(t, ConfigureTestsOptions{Command: &cmd})
	if res.Result != ResultApplied || res.RepositoryState != string(reposetup.StateNeedsReview) {
		t.Fatalf("configure-tests --command = %q/%q (%s), want applied/needs-review", res.Result, res.RepositoryState, res.HumanText())
	}
	if !contains(res.PendingPaths, ".docket.yml") {
		t.Errorf("PendingPaths = %v, want .docket.yml", res.PendingPaths)
	}
	if staged := runGit(t, r.invocation, "diff", "--cached", "--name-only"); strings.Contains(staged, ".docket.yml") {
		t.Errorf("configure-tests --command staged .docket.yml; staged: %q", staged)
	}
	eff := resolveDocketYML(t, r)
	if eff.Build.Gate.Value != "local" || eff.Finalize.Gate.Value != "local" ||
		eff.Build.TestCommand.Value != cmd || eff.Finalize.TestCommand.Value != cmd {
		t.Fatalf("resolved policy build=%q/%q finalize=%q/%q, want local/%q for both",
			eff.Build.Gate.Value, eff.Build.TestCommand.Value, eff.Finalize.Gate.Value, eff.Finalize.TestCommand.Value, cmd)
	}

	r.commitAndPushMain(t, "commit the explicit test policy", ".docket.yml")
	check := r.runCheck(t)
	if check.RepositoryState != string(reposetup.StateHealthy) || check.CheckExitCode() != 0 {
		t.Fatalf("check after commit = %q exit %d (%s), want healthy/0", check.RepositoryState, check.CheckExitCode(), check.HumanText())
	}
	for _, f := range check.Findings {
		if f.Code == reposetup.TestConfigMissingCode {
			t.Errorf("check still reports %s: %+v", reposetup.TestConfigMissingCode, f)
		}
	}

	before := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
	again := r.runConfigureTestsWith(t, ConfigureTestsOptions{Command: &cmd})
	if again.Result != ResultNoOp {
		t.Errorf("repeat --command = %q (%s), want no-op", again.Result, again.HumanText())
	}
	if after := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml")); string(before) != string(after) {
		t.Errorf("repeat --command rewrote .docket.yml; want byte-identical")
	}
}

// TestIntegrationRepoSetupConfigureTestsCommandRefusesInvalidInput proves an
// empty, whitespace-only, or `auto` --command is invalid-input and leaves the
// healthy repository's .docket.yml byte-identical with no working-tree change.
func TestIntegrationRepoSetupConfigureTestsCommandRefusesInvalidInput(t *testing.T) {
	r := newHealthyRepo(t)
	before := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))
	for _, raw := range []string{"", "   ", "auto"} {
		v := raw
		res := r.runConfigureTestsWith(t, ConfigureTestsOptions{Command: &v})
		if res.Result != ResultInvalidInput {
			t.Errorf("--command %q = %q (%s), want invalid-input", raw, res.Result, res.HumanText())
		}
	}
	if after := mustReadFile(t, filepath.Join(r.invocation, ".docket.yml")); string(before) != string(after) {
		t.Errorf("a refused --command changed .docket.yml")
	}
	if dirty := runGit(t, r.invocation, "status", "--porcelain"); strings.TrimSpace(dirty) != "" {
		t.Errorf("a refused --command left working-tree changes: %q", dirty)
	}
}
