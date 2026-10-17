package suiterunner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// devFixture drops each name->body pair as a test_<name>.sh script into a fresh
// dir and returns it. Every fixture body is prefixed with a `# docket-suite: go`
// declaration so category-declared discovery admits it; the suite-under-test is
// entirely synthetic, so nothing here touches the real corpus.
func devFixture(t *testing.T, scripts map[string]string) string {
	t.Helper()
	dir := testsupport.TempDir(t)
	for name, body := range scripts {
		writeScript(t, dir, name, "# docket-suite: go\n"+body)
	}
	return dir
}

// runCfg builds a Config over a synthetic suite with two jobs and captured
// streams. Work/StatePath are left empty (a runner-owned tempdir and no budget
// history), which the callers override where a test needs them fixed.
func runCfg(t *testing.T, testsDir string, out, errBuf *bytes.Buffer) Config {
	t.Helper()
	return Config{
		TestsDir:    testsDir,
		BudgetsPath: "", // default ceiling for every file
		Bash:        bashPath(t),
		Jobs:        2,
		Stdout:      out,
		Stderr:      errBuf,
	}
}

func TestRunGreenSuite(t *testing.T) {
	dir := devFixture(t, map[string]string{
		"a": "echo 'ok - a'\n",
		"b": "echo 'ok - b'\n",
		"c": "echo 'ok - c'\n",
	})
	var out, errBuf bytes.Buffer
	code := Run(context.Background(), runCfg(t, dir, &out, &errBuf))
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(out.String(), "SUITE files=3 passed=3 failed=0") {
		t.Fatalf("missing green SUITE line:\n%s", out.String())
	}
}

func TestRunRedSuite(t *testing.T) {
	dir := devFixture(t, map[string]string{
		"a": "echo 'ok - a'\n",
		"b": "echo 'NOT OK - broke'\nexit 1\n",
	})
	var out, errBuf bytes.Buffer
	code := Run(context.Background(), runCfg(t, dir, &out, &errBuf))
	if code != 1 {
		t.Fatalf("exit = %d, want 1\nstdout:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "FAILED:") || !strings.Contains(out.String(), "test_b") {
		t.Fatalf("want a FAILED: line naming test_b:\n%s", out.String())
	}
}

// TestRunNoResultExitsThree makes durable publication fail for every target by
// making the stat dir unwritable: no file, so each target is NO RESULT — exit 3,
// never a test-failure exit 1.
func TestRunNoResultExitsThree(t *testing.T) {
	dir := devFixture(t, map[string]string{
		"a": "echo 'ok - a'\n",
		"b": "echo 'ok - b'\n",
	})
	work := testsupport.TempDir(t)
	statDir := filepath.Join(work, "stat")
	if err := os.MkdirAll(statDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(statDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(statDir, 0o755) })

	var out, errBuf bytes.Buffer
	cfg := runCfg(t, dir, &out, &errBuf)
	cfg.Work = work
	code := Run(context.Background(), cfg)
	if code != 3 {
		t.Fatalf("exit = %d, want 3\nstdout:\n%s\nstderr:\n%s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(out.String(), "NO RESULT:") {
		t.Fatalf("want a NO RESULT: line:\n%s", out.String())
	}
	if strings.Contains(out.String(), "FAILED:") {
		t.Fatalf("a missing result is not a test failure — no FAILED: line:\n%s", out.String())
	}
}

func TestRunUsageErrors(t *testing.T) {
	dir := devFixture(t, map[string]string{"a": "echo 'ok - a'\n"})

	t.Run("jobs below one", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		cfg := runCfg(t, dir, &out, &errBuf)
		cfg.Jobs = 0
		if code := Run(context.Background(), cfg); code != 2 {
			t.Fatalf("exit = %d, want 2\nstderr:\n%s", code, errBuf.String())
		}
	})

	t.Run("unresolvable bash", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		cfg := runCfg(t, dir, &out, &errBuf)
		cfg.Bash = "/nonexistent"
		if code := Run(context.Background(), cfg); code != 2 {
			t.Fatalf("exit = %d, want 2\nstderr:\n%s", code, errBuf.String())
		}
	})
}

// Change 0465: two checkouts of one repository (a primary and a .worktrees/<slug>)
// sharing one budget-state store accumulate ONE streak for the same target. Before
// the fix each absolute path minted its own record and the second run read 1/5.
func TestRunBudgetStateConvergesAcrossCheckouts(t *testing.T) {
	state := filepath.Join(testsupport.TempDir(t), "state.tsv")
	durations := writeDurations(t, [][3]string{{"test_slow.sh", "1000", "1"}})
	run := func(root string) string {
		t.Helper()
		tests := filepath.Join(root, "tests")
		if err := os.MkdirAll(tests, 0o755); err != nil {
			t.Fatal(err)
		}
		writeScript(t, tests, "slow", "# docket-suite: go\necho 'ok - slow'\n")
		var out, errBuf bytes.Buffer
		cfg := runCfg(t, tests, &out, &errBuf)
		cfg.RepoRoot = root
		cfg.StatePath = state
		cfg.DurationsPath = durations
		if code := Run(context.Background(), cfg); code != 0 {
			t.Fatalf("run in %s exit = %d\nstdout:\n%s\nstderr:\n%s", root, code, out.String(), errBuf.String())
		}
		return out.String()
	}
	primary := filepath.Join(testsupport.TempDir(t), "docket")
	first := run(primary)
	second := run(filepath.Join(primary, ".worktrees", "fix-slow"))
	if !strings.Contains(first, "consecutive parallel-overrun streak 1/5") {
		t.Fatalf("first checkout must open the streak at 1/5:\n%s", first)
	}
	if !strings.Contains(second, "consecutive parallel-overrun streak 2/5") {
		t.Fatalf("second checkout must CONTINUE the same record (2/5), not start its own:\n%s", second)
	}
}

// signEntries lists the names in dir matching re (none for a missing dir).
func signEntries(t *testing.T, dir string, re *regexp.Regexp) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read sign folder: %v", err)
	}
	var out []string
	for _, e := range entries {
		if re.MatchString(e.Name()) {
			out = append(out, e.Name())
		}
	}
	return out
}

// TestRunSignLivesForTheRun pins the sign's lifetime: while a target is still
// running the run's own sign is visible, held, and names the worktree; once Run
// returns the sign is gone.
func TestRunSignLivesForTheRun(t *testing.T) {
	ctl := testsupport.TempDir(t)
	marker := filepath.Join(ctl, "started")
	release := filepath.Join(ctl, "release")
	dir := devFixture(t, map[string]string{
		"hold": "touch '" + marker + "'\n" +
			"i=0\n" +
			"while [ ! -e '" + release + "' ] && [ \"$i\" -lt 300 ]; do sleep 0.1; i=$((i+1)); done\n" +
			"echo 'ok - hold'\n",
	})
	var out, errBuf bytes.Buffer
	cfg := runCfg(t, dir, &out, &errBuf)
	cfg.StatePath = filepath.Join(testsupport.TempDir(t), "state", "state.tsv")
	cfg.RepoRoot = testsupport.TempDir(t)
	signDir := runSignDir(cfg.StatePath)

	done := make(chan int, 1)
	go func() { done <- Run(context.Background(), cfg) }()
	// Whatever happens below, let the fixture finish and wait for Run, so it never
	// outlives the test's temp dirs (cleanups run last-registered first).
	returned := false
	t.Cleanup(func() {
		_ = os.WriteFile(release, nil, 0o644)
		if !returned {
			select {
			case <-done:
			case <-time.After(30 * time.Second):
			}
		}
	})

	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("fixture never started")
		}
		time.Sleep(20 * time.Millisecond)
	}

	signs := signEntries(t, signDir, runSignName)
	if len(signs) != 1 {
		t.Fatalf("during the run: want exactly one sign in %s, got %v", signDir, signs)
	}
	signPath := filepath.Join(signDir, signs[0])
	if got := process.ProbeLock(signPath); got != process.LockProbeHeld {
		t.Fatalf("during the run: sign probe = %v, want held", got)
	}
	body, err := os.ReadFile(signPath)
	if err != nil {
		t.Fatalf("read sign: %v", err)
	}
	if !strings.Contains(string(body), "worktree="+cfg.RepoRoot+"\n") {
		t.Fatalf("sign must name the worktree %s:\n%s", cfg.RepoRoot, body)
	}
	if temps := signEntries(t, signDir, runSignTempName); len(temps) != 0 {
		t.Fatalf("no temporary sign may remain once published: %v", temps)
	}

	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		returned = true
		if code != 0 {
			t.Fatalf("exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, out.String(), errBuf.String())
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("Run did not return after release")
	}
	if left := signEntries(t, signDir, runSignName); len(left) != 0 {
		t.Fatalf("after the run: the sign must be taken down, found %v", left)
	}
}

// TestRunSignFailureWarnsAndPasses: a sign folder that cannot be created costs
// one stderr warning and never the exit code.
func TestRunSignFailureWarnsAndPasses(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	defer setLockTiming(2, time.Millisecond)()
	ro := filepath.Join(testsupport.TempDir(t), "ro")
	if err := os.Mkdir(ro, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })

	dir := devFixture(t, map[string]string{"a": "echo 'ok - a'\n"})
	var out, errBuf bytes.Buffer
	cfg := runCfg(t, dir, &out, &errBuf)
	cfg.StatePath = filepath.Join(ro, "state.tsv")
	cfg.RepoRoot = testsupport.TempDir(t)
	if code := Run(context.Background(), cfg); code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, out.String(), errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "run sign not put up") {
		t.Fatalf("want a run-sign warning on stderr:\n%s", errBuf.String())
	}
}

// TestRunOwnSignIsNotContention drives a real over-line scheduled solo re-check
// through Run: the run's own sign is in the sign folder the whole time, and must
// never make that re-check NOT COUNTED (Run hands it to the budget state as
// selfSign).
func TestRunOwnSignIsNotContention(t *testing.T) {
	setup := func(t *testing.T) (Config, *bytes.Buffer, *bytes.Buffer) {
		t.Helper()
		tests := devFixture(t, map[string]string{"slow": "echo 'ok - slow'\n"})
		var out, errBuf bytes.Buffer
		cfg := runCfg(t, tests, &out, &errBuf)
		cfg.RepoRoot = filepath.Dir(tests)
		cfg.StatePath = filepath.Join(testsupport.TempDir(t), "state.tsv")
		cfg.DurationsPath = writeDurations(t, [][3]string{{"test_slow.sh", "1000", "1000"}})
		return cfg, &out, &errBuf
	}
	check := func(t *testing.T, out string) {
		t.Helper()
		if strings.Contains(out, "SERIAL CONFIRMATION NOT COUNTED") {
			t.Fatalf("the run's own sign was counted as another suite:\n%s", out)
		}
		if !strings.Contains(out, "SERIAL CONFIRMED OVER BUDGET: ") {
			t.Fatalf("want an uncontended over-line re-check to confirm:\n%s", out)
		}
	}

	t.Run("scheduled", func(t *testing.T) {
		cfg, out, errBuf := setup(t)
		// The fifth consecutive overrun makes the record due, and that same run
		// takes the scheduled confirmation.
		for i := 1; i <= 5; i++ {
			out.Reset()
			errBuf.Reset()
			if code := Run(context.Background(), cfg); code != 0 {
				t.Fatalf("run %d exit = %d\nstdout:\n%s\nstderr:\n%s", i, code, out.String(), errBuf.String())
			}
		}
		check(t, out.String())
	})
}

// TestRunWithoutStatePathPutsUpNoSign: no budget-state store means no sign and
// no sign warning — exactly the behavior before signs existed.
func TestRunWithoutStatePathPutsUpNoSign(t *testing.T) {
	dir := devFixture(t, map[string]string{"a": "echo 'ok - a'\n"})
	var out, errBuf bytes.Buffer
	cfg := runCfg(t, dir, &out, &errBuf)
	if code := Run(context.Background(), cfg); code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, out.String(), errBuf.String())
	}
	if strings.Contains(errBuf.String(), "run sign") {
		t.Fatalf("no StatePath must mean no run sign at all:\n%s", errBuf.String())
	}
}

// TestRunSingleJobDirectReading pins the direct budget reading a single-job run
// gets: no screening and no solo re-check run on this path, so the measured time
// is compared straight against ceiling*3/2. A crossing is loud in the report and
// still exits 0 — no budget finding changes the exit code.
func TestRunSingleJobDirectReading(t *testing.T) {
	dir := devFixture(t, map[string]string{"slow": "echo 'ok - slow'\n"})
	var out, errBuf bytes.Buffer
	cfg := runCfg(t, dir, &out, &errBuf)
	cfg.Jobs = 1
	// No budget row: default ceiling 60, direct line 90. 100 crosses it.
	cfg.DurationsPath = writeDurations(t, [][3]string{{"test_slow.sh", "100", "100"}})

	code := Run(context.Background(), cfg)
	report := out.String()
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, report, errBuf.String())
	}
	if !strings.Contains(report, "OVER BUDGET (ceiling 60s)") {
		t.Fatalf("want the OVER BUDGET row tag:\n%s", report)
	}
	if !regexp.MustCompile(`(?m)^OVER BUDGET: test_slow$`).MatchString(report) {
		t.Fatalf("want an OVER BUDGET: line naming test_slow:\n%s", report)
	}
	if strings.Contains(report, "SERIAL CONFIRMATION") {
		t.Fatalf("a single-job run takes no solo re-check:\n%s", report)
	}
}
