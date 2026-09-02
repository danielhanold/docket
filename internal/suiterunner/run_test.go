package suiterunner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
