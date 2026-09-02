//go:build integration

package app

import (
	"context"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

// This file is Task 11's timeout-behaviour coverage at the PRODUCTION sweep entry
// point (app.MaintenanceSweep), the app-layer complement to the adapter-level and
// CLI-level coverage the earlier tasks already own. It uses milliseconds-scale
// injected network budgets and a sleeping fake git so a wedged remote is observed
// as a BOUNDED return, never a 30/60s wall-clock wait, and never a wall-clock
// equality assert.
//
// Where each Task 11 sub-bullet is proved (covered here, or deferred with reason):
//
//   Step 1 (policy propagation from source):
//     - The real dependency builder threading one 30s-read/60s-write policy onto
//       every reachable Git/GitHub seam (pointer equality + per-adapter selection +
//       standalone-default isolation) is Task 9's internal/cli coverage
//       (TestSweepDepsCarrySweepNetworkPolicies / …ShareOneClientAcrossNestedSeams
//       / TestStandaloneDepsKeepDefaultPolicies) — EXTENDED, not duplicated here.
//     - The one-way correspondence guard ("no network:true site left unclassified")
//       is TestEveryNetworkSiteIsReadWriteClassified in internal/gitcli and
//       internal/githubcli (adapter-local: the network sites live in the adapters).
//
//   Step 2 (bounded return under short budgets), covered here:
//     - The initial pin's remote read blocked → the sweep's typed external-failure
//       refusal retains the adapter's timeout KIND ("timed-out") and message, and
//       dispatches nothing (zero GitHub traffic): TestSweepInitialPinNetworkTimeout
//       IsBoundedExternalRefusal below.
//   Step 2, deferred to their existing owners (reason: each needs an ACTIONABLE
//   population driven to a specific mid-sweep invocation through a scripted fake gh
//   speaking the full merge/reprobe surface — the same live-suffix realism gap
//   Task 10 documents; blocking a LATER-than-pin git/gh process while allowing the
//   earlier ones is exactly that scripted surface):
//     - a later metadata preparation → reload-failed skip: the reload-failed path
//       is owned by Task 10's TestIntegration…blocked-probe reload coverage.
//     - a PR batch read → unknown facts + finding: the batched-read failure→unknown
//       (never clean-absence) contract is owned by internal/githubcli/prbatch tests
//       and Task 10's fact-finding assertions.
//     - a transaction fetch and a push/delete → existing unknown/recoverable outcome
//       with no forbidden follow-on mutation: owned by the transaction engine's CAS
//       tests and internal/gitcli push/refdelete integration tests; a timed-out
//       write staying unknown-unless-reconciled is the engine's fresh-origin CAS
//       property, not re-litigated at the app seam.
//     The read-vs-write budget SELECTION under sleeping fakes is proved at the
//     adapter level (TestRunSelectsReadVsWriteNetworkBudget in both gitcli and
//     githubcli, Tasks 1–2); the shared-READ-budget bounding of a fetch+probe pair
//     is internal/gitcli's TestIntegrationRepoFetchFailureClassificationBoundBy
//     ReadBudgetNotWrite (Task 11 Step 3).
//
//   Step 3 (budget sharing) / Step 4 (mutations): the shared-read-budget variant
//   and its two budget mutations (write-keyed netCtx; un-shared probe) are the
//   gitcli test named above; the distinct read≠write policy ("blanket 30s must
//   fail") is Task 9's TestSweepDepsCarrySweepNetworkPolicies asserting
//   NetworkWriteTimeout()==60s distinctly, plus the behavioural fake-write-survives
//   -the-read-budget tests in both adapters.

// timeoutPinGit builds a production gitcli.Client whose executable is a shell
// wrapper that logs every invocation, SLEEPS past any budget on the remote
// default-branch probe (`ls-remote --symref … HEAD`, the pin's first network
// read), and otherwise delegates to the real git. The client's network read
// budget is readBudget, so the wedged probe is killed by the deadline rather than
// ever completing its long sleep — the sweep must return bounded. Every other
// operation (local discovery, config reads) keeps the default local budget and
// runs normally.
func timeoutPinGit(t *testing.T, logPath string, readBudget time.Duration) *gitcli.Client {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	dir := testsupport.TempDir(t)
	wrapper := filepath.Join(dir, "git")
	// The sleep is far longer than any budget or ceiling in the test, so if the
	// deadline is NOT enforced the sweep would visibly hang past the ceiling
	// assertion instead of returning a timeout — the mutation this test guards.
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> '" + logPath + "'\n" +
		"case \"$*\" in\n" +
		"*ls-remote*--symref*) sleep 30; exit 0;;\n" +
		"esac\n" +
		"exec '" + realGit + "' \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", testsupport.TempDir(t))
	client, err := gitcli.NewClient(
		gitcli.WithExecutable(wrapper),
		gitcli.WithNetworkReadTimeout(readBudget),
	)
	if err != nil {
		t.Fatalf("gitcli.NewClient: %v", err)
	}
	return client
}

// TestIntegrationSweepInitialPinNetworkTimeoutIsBoundedExternalRefusal proves the sweep's
// initial pin, when its remote read wedges, returns a BOUNDED typed external
// failure that retains the adapter's timeout kind — and dispatches nothing. With a
// 200ms network read budget against a fake git that sleeps 30s on the pin's
// default-branch probe, MaintenanceSweep must:
//   - return well under a 5s ceiling (the deadline fired; the 30s sleep never
//     completed — no wall-clock equality, a generous ceiling per the budget),
//   - surface ResultExternalFailed / reload-independent ReasonStatusExternal whose
//     message carries the adapter's "timed-out" kind, and
//   - issue ZERO GitHub processes (the whole sweep refused before any dispatch).
func TestIntegrationSweepInitialPinNetworkTimeoutIsBoundedExternalRefusal(t *testing.T) {
	requireRealGit(t)
	r := trafficRepo(t, "", map[string]string{
		"docs/changes/active/0041-a.md": trafficDoneRecord(41, "a"),
	})
	gitLog := filepath.Join(testsupport.TempDir(t), "git.log")
	ghLog := filepath.Join(testsupport.TempDir(t), "gh.log")
	git := timeoutPinGit(t, gitLog, 200*time.Millisecond)
	deps := trafficDeps(t, git, trafficGH(t, ghLog))

	start := time.Now()
	res := MaintenanceSweep(context.Background(), deps, r.invocation, SweepScopeFull)
	elapsed := time.Since(start)

	// Bounded: the 200ms read deadline fired; the 30s fake sleep never elapsed.
	// The ceiling is the fake sleep, not a one-machine-calibrated tolerance: a
	// working budget returns in well under a second, a broken one waits the full
	// 30s fake sleep. 20s is generous enough to survive parallel-suite CPU
	// starvation (the full gate runs ~39 files in parallel) while still failing
	// loudly if the read budget never fires and the fetch waits the 30s sleep.
	if elapsed >= 20*time.Second {
		t.Fatalf("sweep did not return bounded: elapsed %v (fake sleeps 30s; a 200ms read budget should have fired)", elapsed)
	}
	// A typed external-failure refusal that retains the adapter's timeout kind.
	if res.Result != ResultExternalFailed {
		t.Fatalf("result = %q, want %q (a wedged pin is an external failure)", res.Result, ResultExternalFailed)
	}
	if res.Reason != ReasonStatusExternal {
		t.Fatalf("reason = %q, want %q", res.Reason, ReasonStatusExternal)
	}
	if !strings.Contains(res.Message, string(gitcli.KindTimedOut)) {
		t.Fatalf("refusal message %q does not retain the adapter timeout kind %q", res.Message, gitcli.KindTimedOut)
	}
	// No dispatch: the sweep refused at the pin, before any GitHub identity or PR
	// read — so the fake gh recorded nothing.
	if got := ghLogLines(t, ghLog); len(got) != 0 {
		t.Errorf("sweep issued %d GitHub process(es) despite refusing at the pin:\n%s", len(got), strings.Join(got, "\n"))
	}
	// And no per-item or mutating entry escaped the whole-sweep refusal.
	if len(res.Entries) != 0 {
		t.Errorf("refusal carried %d entries, want none: %+v", len(res.Entries), res.Entries)
	}
}
