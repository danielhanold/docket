package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// leakSentinel is a recognizable byte sequence carried through both the child's
// argv AND its stdout/stderr. The headline security invariant (failure.go's
// Failure comment "never carries argv, environment values, or child output",
// and supervisor.go's writeFailure) is that free-form argv, environment values,
// and child output never reach protocol error text, a durable protocol record,
// or the HumanText derived downstream from them — only the run directory's log
// files may hold those bytes. The token is deliberately alnum-only (no spaces or
// control runes) so boundReason would preserve it verbatim if it ever leaked;
// its absence from a bounded reason is therefore real, not an artifact of
// flattening.
const leakSentinel = "ZZQXLEAKSENTINEL7F3A9C1E"

// assertNoSentinel fails if the sentinel appears anywhere in text.
func assertNoSentinel(t *testing.T, where, text string) {
	t.Helper()
	if strings.Contains(text, leakSentinel) {
		t.Fatalf("sentinel leaked into %s: %q", where, text)
	}
}

// soleRunDir returns the single 32-hex run directory under root. Launch can
// return only an error on the failure path, so the run dir is recovered from
// the filesystem rather than from a LaunchOutcome.
func soleRunDir(t *testing.T, root string) string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("reading root: %v", err)
	}
	var found string
	for _, e := range entries {
		if e.IsDir() && runIDPattern.MatchString(e.Name()) {
			if found != "" {
				t.Fatalf("more than one run dir under %s", root)
			}
			found = filepath.Join(root, e.Name())
		}
	}
	if found == "" {
		t.Fatalf("no run dir under %s", root)
	}
	return found
}

func readRunFile(t *testing.T, runDir, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(runDir, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(raw)
}

// TestNoLeakStartFailureArgv pins the no-leak invariant on the start-failure
// path, where the raw fork/exec error carries the child's argv[0]. The
// supervised program is a nonexistent absolute path embedding the sentinel;
// cmd.Start fails, writeFailure records a bounded static reason, and the
// launcher surfaces a *Failure. The sentinel must appear in NEITHER the
// returned error text NOR the durable failure.json record — but the run
// directory's supervisor.log MAY (and does) capture the raw error, which is
// exactly where the argv-bearing bytes are allowed to live.
//
// This reddens if the start-command reason interpolates the raw error (e.g.
// `%v`), the classic regression the invariant guards against.
func TestNoLeakStartFailureArgv(t *testing.T) {
	svc := newTestService(t)
	root := testsupport.TempDir(t)
	bogus := filepath.Join(testsupport.TempDir(t), leakSentinel+"-no-such-binary")

	_, err := svc.Launch(LaunchRequest{
		Root: root,
		Cwd:  testsupport.TempDir(t),
		Argv: []string{bogus, leakSentinel, "child-arg"},
	})
	if err == nil {
		t.Fatal("nonexistent program was expected to fail the launch")
	}
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("start failure did not classify as *Failure: %v", err)
	}
	assertNoSentinel(t, "Launch error text", err.Error())
	assertNoSentinel(t, "Failure.Reason", f.Reason)

	runDir := soleRunDir(t, root)
	reapSupervisor(runDir)
	testsupport.DrainOnCleanup(t, func() { quiesceRun(t, runDir) })

	// The durable protocol record must be sentinel-free in its entirety.
	assertNoSentinel(t, "failure.json", readRunFile(t, runDir, failureFile))

	// Meaningfulness: the sentinel truly reached the supervisor (the raw start
	// error names argv[0]), so its absence above is a real scrub, not a case
	// where the bytes never arrived. Logs are allowed to hold it.
	if log := readRunFile(t, runDir, supervisorLogFile); !strings.Contains(log, leakSentinel) {
		t.Fatalf("sentinel never reached the supervisor; the guard would pass vacuously")
	}
}

// TestNoLeakChildOutputAndArgvTerminal pins the invariant on the terminal
// failure path with a child that actually runs. Its argv carries the sentinel
// and it writes the sentinel to BOTH stdout and stderr before exiting nonzero.
// The run resolves to StateFailed. The sentinel must appear in the child's
// stdout.log/stderr.log (proving it was emitted — logs may hold it) but in
// NEITHER the Observation surface NOR the durable manifest.json / terminal.json
// protocol records.
//
// This reddens if manifest.json were to record the full argv rather than the
// bounded argv0/argc, or if Observe folded child output into its verdict.
func TestNoLeakChildOutputAndArgvTerminal(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, testsupport.TempDir(t), "emit-exit", leakSentinel, leakSentinel, "7")

	obs := observeUntilTerminal(t, svc, out.RunDir)
	if obs.State != StateFailed {
		t.Fatalf("emit-exit 7 observed %v, want failed", obs.State)
	}
	if obs.Terminal == nil || obs.Terminal.ExitCode != 7 {
		t.Fatalf("exact terminal lost: %+v", obs.Terminal)
	}

	// Meaningfulness: the child's bytes did land in the run-dir logs.
	if so := readRunFile(t, out.RunDir, stdoutLogFile); !strings.Contains(so, leakSentinel) {
		t.Fatalf("child stdout was not captured; the guard would pass vacuously")
	}
	if se := readRunFile(t, out.RunDir, stderrLogFile); !strings.Contains(se, leakSentinel) {
		t.Fatalf("child stderr was not captured; the guard would pass vacuously")
	}

	// Protocol/observation surface: no sentinel from argv or child output.
	assertNoSentinel(t, "Observation", fmt.Sprintf("%+v", obs))
	assertNoSentinel(t, "manifest.json", readRunFile(t, out.RunDir, manifestFile))
	assertNoSentinel(t, "terminal.json", readRunFile(t, out.RunDir, terminalFile))
}
