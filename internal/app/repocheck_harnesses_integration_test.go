//go:build integration

package app

import (
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// This shard file carries `docket repository check`'s harnesses-unset warning
// (prefix TestIntegrationRepoCheck, matched by
// tests/test_go_integration_app_repocheck.sh): while no repository-level layer
// declares agent_harnesses the check warns, never changing the classified
// state, and a recorded choice (`[]` included) silences it.

// assertHarnessesUnset checks the classified state, whether the
// harnesses-unset finding is present, and the exit code.
func assertHarnessesUnset(t *testing.T, label string, chk RepositoryCheckResult, wantFinding bool, wantExit int) {
	t.Helper()
	if chk.RepositoryState != string(reposetup.StateHealthy) {
		t.Fatalf("%s: state = %q, want healthy:\n%s", label, chk.RepositoryState, chk.HumanText())
	}
	if got := len(findingsWithCode(chk.Findings, reposetup.HarnessesUnsetCode)) > 0; got != wantFinding {
		t.Errorf("%s: harnesses-unset present = %v, want %v:\n%s", label, got, wantFinding, chk.HumanText())
	}
	if code := chk.CheckExitCode(); code != wantExit {
		t.Errorf("%s: exit = %d, want %d:\n%s", label, code, wantExit, chk.HumanText())
	}
}

func TestIntegrationRepoCheckHarnessesUnsetShared(t *testing.T) {
	r := newInitRepo(t, "integration_branch: main\n", nil)
	if res := r.runInitWith(t, InitOptions{}); res.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
	}
	r.commitAndPushMain(t, "commit managed .gitignore and generated test policy", ".gitignore", ".docket.yml")
	assertHarnessesUnset(t, "unset", r.runCheck(t), true, 1)

	none := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: harnessFlag(reposetup.HarnessesNone)})
	if none.Result != ResultApplied {
		t.Fatalf("configure-harnesses none = %q (%s), want applied", none.Result, none.HumanText())
	}
	r.commitAndPushMain(t, "record no agents", ".docket.yml")
	assertHarnessesUnset(t, "none", r.runCheck(t), false, 0)

	claude := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: harnessFlag("claude")})
	if claude.Result != ResultApplied {
		t.Fatalf("configure-harnesses claude = %q (%s), want applied", claude.Result, claude.HumanText())
	}
	r.commitAndPushMain(t, "choose claude", claude.PendingPaths...)
	assertHarnessesUnset(t, "claude", r.runCheck(t), false, 0)
}

func TestIntegrationRepoCheckHarnessesUnsetPrivate(t *testing.T) {
	r, _ := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
	}
	cmd := "true"
	if ct := r.runConfigureTestsWith(t, ConfigureTestsOptions{Command: &cmd}); ct.Result != ResultApplied {
		t.Fatalf("configure-tests = %q (%s), want applied", ct.Result, ct.HumanText())
	}
	assertHarnessesUnset(t, "unset", checkIn(t, r.invocation), true, 1)

	none := r.runConfigureHarnesses(t, ConfigureHarnessesOptions{Harnesses: harnessFlag(reposetup.HarnessesNone)})
	if none.Result != ResultApplied {
		t.Fatalf("configure-harnesses none = %q (%s), want applied", none.Result, none.HumanText())
	}
	assertHarnessesUnset(t, "none", checkIn(t, r.invocation), false, 0)
}
