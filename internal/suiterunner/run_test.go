package suiterunner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// devFixture drops each name->body pair as a test_<name>.sh script into a fresh
// dir and returns it. The suite-under-test is entirely synthetic, so nothing here
// touches the real corpus.
func devFixture(t *testing.T, scripts map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range scripts {
		writeScript(t, dir, name, body)
	}
	return dir
}

// fakeHygiene writes a contract-faithful stand-in for
// scripts/check-test-source-hygiene.sh: it exits 1 when any scanned target holds
// a backtick (the executing vector the real checker refuses) and 0 otherwise.
// Run's preflight branches on that exit status, which is what these tests pin;
// the real checker's detection logic is proved by tests/test_assert_hygiene.sh
// and exercised end to end by tests/test_devtest_runner.sh.
func fakeHygiene(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "check.sh")
	content := "#!/usr/bin/env bash\n" +
		"set -uo pipefail\n" +
		"rc=0\n" +
		"for f in \"$@\"; do\n" +
		"  [ \"$f\" = \"--\" ] && continue\n" +
		"  if grep -qF '\x60' \"$f\"; then echo \"backtick in $f\"; rc=1; fi\n" +
		"done\n" +
		"exit $rc\n"
	if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake hygiene checker: %v", err)
	}
	return p
}

// runCfg builds a Config over a synthetic suite with the fake hygiene checker,
// two jobs, and captured streams. Work/StatePath are left empty (a runner-owned
// tempdir and no budget history), which the callers override where a test needs
// them fixed.
func runCfg(t *testing.T, testsDir string, out, errBuf *bytes.Buffer) Config {
	t.Helper()
	return Config{
		TestsDir:    testsDir,
		BudgetsPath: "", // default ceiling for every file
		HygienePath: fakeHygiene(t),
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

// TestRunHygieneViolationExecutesNothing proves the preflight aborts with ZERO
// test files executed: a fixture with a backtick reddens the checker (exit 1),
// Run returns 5, and no target's stat/log dir was ever created.
func TestRunHygieneViolationExecutesNothing(t *testing.T) {
	dir := devFixture(t, map[string]string{
		"good": "echo 'ok - good'\n",
		"bad":  "echo \"\x60whoami\x60\"\n",
	})
	work := t.TempDir()
	var out, errBuf bytes.Buffer
	cfg := runCfg(t, dir, &out, &errBuf)
	cfg.Work = work
	code := Run(context.Background(), cfg)
	if code != 5 {
		t.Fatalf("exit = %d, want 5\nstderr:\n%s", code, errBuf.String())
	}
	for _, sub := range []string{"stat", "logs"} {
		if _, err := os.Stat(filepath.Join(work, sub)); !os.IsNotExist(err) {
			t.Fatalf("a hygiene violation must execute nothing, but %s/%s exists (err=%v)", work, sub, err)
		}
	}
}

func TestRunHygieneCheckerMissingFailsClosed(t *testing.T) {
	dir := devFixture(t, map[string]string{"a": "echo 'ok - a'\n"})
	var out, errBuf bytes.Buffer
	cfg := runCfg(t, dir, &out, &errBuf)
	cfg.HygienePath = filepath.Join(t.TempDir(), "nonexistent-checker.sh")
	code := Run(context.Background(), cfg)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (an unusable checker fails closed)\nstderr:\n%s", code, errBuf.String())
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
	work := t.TempDir()
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
