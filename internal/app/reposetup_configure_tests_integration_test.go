//go:build integration

package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// This shard file carries the `docket repository configure-tests` upgrade-path
// integration tests (change 0374, prefix TestIntegrationRepoSetupConfigureTests,
// matched by the existing reposetup runner tests/test_go_integration_app_reposetup.sh).
// configure-tests is the setup-time regenerator of the pending `.docket.yml`
// build/finalize test policy for an already-initialized (healthy) repository: it
// discovers the suite over the primary worktree and leaves the generated edit
// UNSTAGED for human review, exactly like init — it never commits or stages.

// runConfigureTests runs RunRepositoryConfigureTests against the invocation clone
// with a fresh isolated client and type-asserts the concrete repository result.
func (r *initRepo) runConfigureTests(t *testing.T) RepositoryOpResult {
	t.Helper()
	client := newGitClient(t)
	res := RunRepositoryConfigureTests(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation})
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
	if !strings.Contains(human, "docket repository configure-tests") {
		t.Errorf("ambiguous note %q must name the configure-tests remedy", human)
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
