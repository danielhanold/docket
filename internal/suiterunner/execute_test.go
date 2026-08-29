package suiterunner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// writeScript drops a test_<name>.sh fixture into dir and returns its Target.
func writeScript(t *testing.T, dir, name, body string) Target {
	t.Helper()
	base := "test_" + name + ".sh"
	p := filepath.Join(dir, base)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatalf("write script %s: %v", base, err)
	}
	return Target{Path: p, Base: base, Ceiling: DefaultCeiling, Mode: ModeParallel}
}

func bashPath(t *testing.T) string {
	t.Helper()
	b, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash not available: %v", err)
	}
	return b
}

func TestSandboxIsolation(t *testing.T) {
	scripts := t.TempDir()
	work := t.TempDir()
	tgt := writeScript(t, scripts, "iso", strings.Join([]string{
		`printf 'HOME=%s\n' "$HOME"`,
		`printf 'TMPDIR=%s\n' "$TMPDIR"`,
		`printf 'EMAIL=%s\n' "$(git config user.email)"`,
		`echo 'ok - iso'`,
	}, "\n")+"\n")

	reg := newProcRegistry()
	res, err := ExecuteTarget(context.Background(), bashPath(t), tgt, work, reg, nil)
	if err != nil {
		t.Fatalf("ExecuteTarget: %v", err)
	}
	if res.RC != 0 {
		t.Fatalf("rc = %d, want 0", res.RC)
	}

	logBytes, err := os.ReadFile(filepath.Join(work, "logs", "test_iso.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	log := string(logBytes)

	// The child's HOME/TMPDIR are built with filepath.Join in Sandbox/ExecuteTarget,
	// which runs filepath.Clean and collapses any redundant slash. work comes from
	// t.TempDir(), which inherits $TMPDIR verbatim — and macOS's $TMPDIR carries a
	// trailing slash, so under the Bash oracle's launch() nesting work can hold a
	// "T//run-tests" double slash the child path never shows. Compare against the
	// cleaned form so the isolation assertion tracks what Sandbox actually produces.
	cleanWork := filepath.Clean(work)
	if !strings.Contains(log, "HOME="+cleanWork) {
		t.Fatalf("child HOME not under work dir; log:\n%s", log)
	}
	if !strings.Contains(log, "TMPDIR="+cleanWork) {
		t.Fatalf("child TMPDIR not under work dir; log:\n%s", log)
	}
	if !strings.Contains(log, "EMAIL=test@docket.invalid") {
		t.Fatalf("synthetic git identity not applied; log:\n%s", log)
	}
	if realHome := os.Getenv("HOME"); realHome != "" && strings.Contains(log, "HOME="+realHome+"\n") {
		t.Fatalf("real HOME %q leaked into child; log:\n%s", realHome, log)
	}
}

func TestExecuteWritesDurableResultAtomically(t *testing.T) {
	scripts := t.TempDir()
	work := t.TempDir()
	tgt := writeScript(t, scripts, "p", "echo 'ok - one'\necho 'ok - two'\n")

	reg := newProcRegistry()
	res, err := ExecuteTarget(context.Background(), bashPath(t), tgt, work, reg, nil)
	if err != nil {
		t.Fatalf("ExecuteTarget: %v", err)
	}
	if res.RC != 0 || res.OK != 2 || res.NotOK != 0 {
		t.Fatalf("observed result = %+v, want rc=0 ok=2 notok=0", res)
	}

	statDir := filepath.Join(work, "stat")
	entries, err := os.ReadDir(statDir)
	if err != nil {
		t.Fatalf("read stat dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "test_p.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("stat dir = %v, want exactly [test_p.json] (no temp leftovers)", names)
	}

	durable, err := ReadResult(filepath.Join(statDir, "test_p.json"))
	if err != nil {
		t.Fatalf("ReadResult: %v", err)
	}
	if durable.RC != 0 || durable.OK != 2 || durable.NotOK != 0 || durable.Target != "test_p.sh" {
		t.Fatalf("durable = %+v, want rc=0 ok=2 notok=0 target=test_p.sh", durable)
	}
}

func TestExecuteRecordsFailure(t *testing.T) {
	scripts := t.TempDir()
	work := t.TempDir()
	tgt := writeScript(t, scripts, "fail", "echo 'NOT OK - broke'\nexit 1\n")

	reg := newProcRegistry()
	res, err := ExecuteTarget(context.Background(), bashPath(t), tgt, work, reg, nil)
	if err != nil {
		t.Fatalf("ExecuteTarget: %v", err)
	}
	if res.RC != 1 || res.NotOK != 1 {
		t.Fatalf("observed = %+v, want rc=1 notok=1", res)
	}
	logBytes, err := os.ReadFile(filepath.Join(work, "logs", "test_fail.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(logBytes), "NOT OK - broke") {
		t.Fatalf("log missing marker line:\n%s", logBytes)
	}
}

func TestExecuteChildGetsOwnProcessGroup(t *testing.T) {
	scripts := t.TempDir()
	work := t.TempDir()
	gate := t.TempDir()
	started := filepath.Join(gate, "started")
	proceed := filepath.Join(gate, "proceed")

	// The child announces its own pgid, signals that it is live, then blocks on
	// a gate file so the test can observe registration deterministically — no
	// fixed sleep stands in as the verdict.
	body := strings.Join([]string{
		`ps -o pgid= -p $$ | tr -d ' \n' > "` + filepath.Join(gate, "childpgid") + `"`,
		`: > "` + started + `"`,
		`for _ in $(seq 1 500); do [ -e "` + proceed + `" ] && break; sleep 0.01; done`,
		`echo 'ok - grouped'`,
	}, "\n") + "\n"
	tgt := writeScript(t, scripts, "pgid", body)

	reg := newProcRegistry()
	done := make(chan Result, 1)
	go func() {
		r, err := ExecuteTarget(context.Background(), bashPath(t), tgt, work, reg, nil)
		if err != nil {
			t.Errorf("ExecuteTarget: %v", err)
		}
		done <- r
	}()

	// Deadline-poll for the child being live AND registered.
	deadline := time.Now().Add(5 * time.Second)
	var snap []int
	for time.Now().Before(deadline) {
		if _, err := os.Stat(started); err == nil {
			if snap = reg.Snapshot(); len(snap) == 1 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(snap) != 1 {
		t.Fatalf("registry snapshot during run = %v, want exactly one live pgid", snap)
	}

	// Release the child and let it finish.
	if err := os.WriteFile(proceed, nil, 0o644); err != nil {
		t.Fatalf("release gate: %v", err)
	}
	res := <-done
	if res.RC != 0 {
		t.Fatalf("rc = %d, want 0", res.RC)
	}

	if after := reg.Snapshot(); len(after) != 0 {
		t.Fatalf("registry snapshot after completion = %v, want empty (Unregister)", after)
	}

	childPgidBytes, err := os.ReadFile(filepath.Join(gate, "childpgid"))
	if err != nil {
		t.Fatalf("read child pgid: %v", err)
	}
	childPgid, err := strconv.Atoi(strings.TrimSpace(string(childPgidBytes)))
	if err != nil {
		t.Fatalf("parse child pgid %q: %v", childPgidBytes, err)
	}
	selfPgid, _ := syscall.Getpgid(0)
	if childPgid == selfPgid {
		t.Fatalf("child pgid %d equals runner pgid %d — child was not placed in its own group", childPgid, selfPgid)
	}
	if snap[0] != childPgid {
		t.Fatalf("registered pgid %d != child's own pgid %d", snap[0], childPgid)
	}
}
