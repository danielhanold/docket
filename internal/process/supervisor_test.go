package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnvWithout(t *testing.T) {
	in := []string{"A=1", supervisorRunDirEnv + "=/x", "B=2", supervisorArgvEnv + `=["y"]`}
	out := envWithout(in, supervisorRunDirEnv, supervisorArgvEnv)
	if len(out) != 2 || out[0] != "A=1" || out[1] != "B=2" {
		t.Fatalf("envWithout = %v", out)
	}
}

func TestSupervisorRequested(t *testing.T) {
	if SupervisorRequested() {
		t.Fatal("requested with env unset")
	}
	t.Setenv(supervisorRunDirEnv, "/somewhere")
	if !SupervisorRequested() {
		t.Fatal("not requested with env set")
	}
}

// TestSupervisorEstablishesBeforeRunning proves the ordered state machine: the
// group is published as addressable ("established") strictly before the
// supervised command is started ("running"). The transitions are traced in
// supervisor.log, so the order is externally observable. This is the guard for
// the establishment-ordering mutation (skipping the established publish): with
// the established step removed, supervisor.log never records it before running
// and this test reddens.
func TestSupervisorEstablishesBeforeRunning(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "sleep")
	defer killRun(t, out.RunDir)

	logPath := filepath.Join(out.RunDir, supervisorLogFile)
	var estIdx, runIdx int
	waitFor(t, "established+running trace", 30*time.Second, func() bool {
		raw, err := os.ReadFile(logPath)
		if err != nil {
			return false
		}
		log := string(raw)
		estIdx = strings.Index(log, "phase: established")
		runIdx = strings.Index(log, "phase: running")
		return estIdx >= 0 && runIdx >= 0
	})
	if estIdx < 0 {
		t.Fatalf("supervisor never published the established phase before running")
	}
	if !(estIdx < runIdx) {
		t.Fatalf("established (%d) did not precede running (%d) in supervisor.log", estIdx, runIdx)
	}
}
