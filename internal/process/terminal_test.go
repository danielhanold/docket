package process

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func itoa(n int) string { return strconv.Itoa(n) }

func launchAndWaitTerminal(t *testing.T, mode string, extra ...string) (*Service, *LaunchOutcome, *terminalRecord) {
	t.Helper()
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), mode, extra...)
	var term *terminalRecord
	waitFor(t, "terminal record", 30*time.Second, func() bool {
		rec, err := readTerminal(out.RunDir)
		if err != nil {
			t.Fatalf("readTerminal: %v", err)
		}
		term = rec
		return rec != nil
	})
	return svc, out, term
}

func TestExactExitCodes(t *testing.T) {
	for _, code := range []int{0, 7, 143} {
		_, _, term := launchAndWaitTerminal(t, "exit", itoa(code))
		if term.Kind != "exit" || term.ExitCode != code {
			t.Fatalf("exit %d recorded as %+v", code, term)
		}
	}
}

// TestSignalTermIsExactlySignal15 — the spec's headline: a genuine `exit 143`
// stays kind=exit code=143 (proven by TestExactExitCodes); SIGTERM death stays
// kind=signal signal=15. No 128+signal heuristic anywhere.
func TestSignalTermIsExactlySignal15(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "sleep")
	m, err := readManifest(out.RunDir)
	if err != nil {
		t.Fatal(err)
	}
	// TERM the GROUP the way an external killer would; the supervisor
	// catches it and stays alive to record, the child dies by it.
	if err := signalGroup(m.PGID, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	var term *terminalRecord
	waitFor(t, "signal terminal record", 30*time.Second, func() bool {
		rec, rerr := readTerminal(out.RunDir)
		if rerr != nil {
			t.Fatalf("readTerminal: %v", rerr)
		}
		term = rec
		return rec != nil
	})
	if term.Kind != "signal" || term.Signal != 15 {
		t.Fatalf("SIGTERM recorded as %+v (128+signal fabrication?)", term)
	}
}

// TestStartFailureIsDistinct — an unstartable command produces a
// failure.json, never a fabricated terminal record.
func TestStartFailureIsDistinct(t *testing.T) {
	svc := newTestService(t)
	root := t.TempDir()
	missing := filepath.Join(t.TempDir(), "no-such-binary")
	_, err := svc.Launch(LaunchRequest{Root: root, Cwd: t.TempDir(), Argv: []string{missing}})
	if err == nil {
		t.Fatal("unstartable command reported a usable handle")
	}
	if f, ok := AsFailure(err); !ok || f.Class != FailExternal {
		t.Fatalf("start failure class = %v", err)
	}
	// The one run dir under root must hold failure.json and NO terminal.json.
	entries, _ := os.ReadDir(root)
	var runDir string
	for _, e := range entries {
		if e.IsDir() {
			runDir = filepath.Join(root, e.Name())
		}
	}
	if runDir == "" {
		t.Fatal("no run dir retained for diagnosis")
	}
	if rec, _ := readTerminal(runDir); rec != nil {
		t.Fatalf("fabricated terminal record: %+v", rec)
	}
	fr, err := readFailureRecord(runDir)
	if err != nil || fr == nil || fr.Stage != "start-command" {
		t.Fatalf("failure record: %+v, %v", fr, err)
	}
}
