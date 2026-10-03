package process

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
)

// The leftover check (change 0492). ProbeLeftover answers, read-only, whether a
// run whose supervisor has exited still has members in its recorded process
// group. Real-process tests pin the leftover, none, and zombie answers; seam
// tests drive every unclear branch deterministically on every platform. The
// seam-swapping tests never run in parallel: the seams are package globals.

// withLeftoverProbes swaps the group and leader probes for one test and
// restores them at cleanup.
func withLeftoverProbes(t *testing.T, group, leader probeAnswer) {
	t.Helper()
	prevGroup, prevLeader := leftoverGroupProbe, leftoverLeaderProbe
	leftoverGroupProbe = func(int) probeAnswer { return group }
	leftoverLeaderProbe = func(int) probeAnswer { return leader }
	t.Cleanup(func() { leftoverGroupProbe, leftoverLeaderProbe = prevGroup, prevLeader })
}

// quiescedRun launches a fast-exit run and waits until its supervisor is gone,
// returning a valid run dir with a free live.lock whose manifest records
// pgid == supervisor_pid — the shape ProbeLeftover's callers hand it.
func quiescedRun(t *testing.T) (*Service, string, manifestRecord) {
	t.Helper()
	svc := newTestService(t)
	out := launchHelper(t, svc, testsupport.TempDir(t), "exit", "0")
	waitTerminalState(t, out.RunDir, false)
	quiesceRun(t, out.RunDir)
	m, err := readManifest(out.RunDir)
	if err != nil || m == nil || m.PGID <= 1 || m.PGID != m.SupervisorPID {
		t.Fatalf("precondition: an established manifest with pgid == supervisor_pid, got %+v (err %v)", m, err)
	}
	return svc, out.RunDir, *m
}

// writeManifestCopy writes m (a copy of the run's manifest, mutated by the
// caller) back over the run's manifest.json.
func writeManifestCopy(t *testing.T, runDir string, m manifestRecord) {
	t.Helper()
	if err := writeAtomicJSON(filepath.Join(runDir, manifestFile), &m); err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}
}

func mustProbeLeftover(t *testing.T, svc *Service, runDir string) Leftover {
	t.Helper()
	got, err := svc.ProbeLeftover(runDir)
	if err != nil {
		t.Fatalf("ProbeLeftover: %v", err)
	}
	return got
}

// TestProbeLeftoverReportsSuiteOfKilledSupervisor: a supervisor SIGKILLed alone,
// and reaped (launchHelper's reapSupervisor plays init), leaves its command
// running in the supervisor's group: leftover, naming that group. Once the
// command is gone too, the group is empty: none.
func TestProbeLeftoverReportsSuiteOfKilledSupervisor(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, testsupport.TempDir(t), "sleep")
	m, err := readManifest(out.RunDir)
	if err != nil || m == nil || m.SupervisorPID <= 1 || m.PGID != m.SupervisorPID {
		t.Fatalf("manifest: %+v (err %v)", m, err)
	}
	t.Cleanup(func() { _ = signalGroup(m.PGID, syscall.SIGKILL) })
	if err := syscall.Kill(m.SupervisorPID, syscall.SIGKILL); err != nil {
		t.Fatalf("kill the supervisor: %v", err)
	}
	waitFor(t, "the killed supervisor to be reaped", 30*time.Second, func() bool {
		return processAlive(m.SupervisorPID) == probeAbsent
	})

	if got := mustProbeLeftover(t, svc, out.RunDir); got.Answer != LeftoverPresent || got.PGID != m.PGID {
		t.Fatalf("a reaped supervisor's still-running command: got %+v, want leftover with pgid %d", got, m.PGID)
	}

	if err := signalGroup(m.PGID, syscall.SIGKILL); err != nil {
		t.Fatalf("end the orphaned command: %v", err)
	}
	waitFor(t, "the orphaned command to be reaped", 30*time.Second, func() bool {
		return groupAlive(m.PGID) == probeAbsent
	})
	if got := mustProbeLeftover(t, svc, out.RunDir); got.Answer != LeftoverNone {
		t.Fatalf("an empty group: got %+v, want none", got)
	}
}

// TestProbeLeftoverZombieSupervisorIsUnclear pins spec fact 8: a dead supervisor
// whose launching process (this test) has not reaped it is a zombie whose pid
// still answers kill(pid, 0), so the check cannot prove the leader gone and
// answers unclear — never leftover. The group probe may read live (the command)
// or unknown (EPERM); both must land on unclear.
func TestProbeLeftoverZombieSupervisorIsUnclear(t *testing.T) {
	svc := newTestService(t)
	// svc.Launch directly, NOT launchHelper: no reapSupervisor, so the killed
	// supervisor stays a zombie child of this process.
	out, err := svc.Launch(LaunchRequest{Root: testsupport.TempDir(t), Cwd: testsupport.TempDir(t), Argv: helperArgv(t, "sleep")})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	m, err := readManifest(out.RunDir)
	if err != nil || m == nil || m.SupervisorPID <= 1 {
		t.Fatalf("manifest: %+v (err %v)", m, err)
	}
	pid := m.SupervisorPID
	t.Cleanup(func() {
		_ = signalGroup(m.PGID, syscall.SIGKILL)
		// Reap exactly this zombie (never -1, so no other test's child is stolen).
		var ws syscall.WaitStatus
		for {
			wpid, werr := syscall.Wait4(pid, &ws, 0, nil)
			if werr != syscall.EINTR || wpid == pid {
				return
			}
		}
	})
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatalf("kill the supervisor: %v", err)
	}
	// The kernel closes a dying process's descriptors before it becomes a zombie,
	// so a free live.lock proves the supervisor has exited.
	waitFor(t, "live lock release", 30*time.Second, func() bool {
		held, ans := probeFlock(filepath.Join(out.RunDir, liveLockFile))
		return !held && ans == probeAbsent
	})
	if got := processAlive(pid); got == probeAbsent {
		t.Fatalf("precondition (spec fact 8): an unreaped supervisor must still answer kill(pid, 0), got %v", got)
	}

	if got := mustProbeLeftover(t, svc, out.RunDir); got.Answer != LeftoverUnclear {
		t.Fatalf("a zombie supervisor: got %+v, want unclear", got)
	}
}

// TestProbeLeftoverGroupAndLeaderAnswers drives the group and leader probes
// through every combination. Only a live group whose leader is provably absent
// is leftover; only an absent group is none; everything else is unclear.
func TestProbeLeftoverGroupAndLeaderAnswers(t *testing.T) {
	svc, runDir, m := quiescedRun(t)
	for _, tc := range []struct {
		name          string
		group, leader probeAnswer
		want          LeftoverAnswer
	}{
		{"group-absent", probeAbsent, probeLive, LeftoverNone},
		{"group-unknown", probeUnknown, probeAbsent, LeftoverUnclear},
		{"leader-absent", probeLive, probeAbsent, LeftoverPresent},
		{"leader-live", probeLive, probeLive, LeftoverUnclear},
		{"leader-unknown", probeLive, probeUnknown, LeftoverUnclear},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withLeftoverProbes(t, tc.group, tc.leader)
			got := mustProbeLeftover(t, svc, runDir)
			if got.Answer != tc.want || got.PGID != m.PGID {
				t.Fatalf("group %v, leader %v: got %+v, want %s with pgid %d", tc.group, tc.leader, got, tc.want, m.PGID)
			}
		})
	}
}

// TestProbeLeftoverRefusesUnaddressableGroup: a recorded pgid <= 1, or one that
// is not the supervisor's own pid, names no group the run owned — unclear, even
// when the probes would otherwise read leftover.
func TestProbeLeftoverRefusesUnaddressableGroup(t *testing.T) {
	svc, runDir, orig := quiescedRun(t)
	withLeftoverProbes(t, probeLive, probeAbsent) // would be leftover
	for _, tc := range []struct {
		name string
		pgid int
	}{
		{"pgid-zero", 0},
		{"pgid-one", 1},
		{"pgid-not-supervisor", orig.SupervisorPID + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := orig
			m.PGID = tc.pgid
			writeManifestCopy(t, runDir, m)
			if got := mustProbeLeftover(t, svc, runDir); got.Answer != LeftoverUnclear {
				t.Fatalf("pgid %d (supervisor_pid %d): got %+v, want unclear", tc.pgid, m.SupervisorPID, got)
			}
		})
	}
}

// TestProbeLeftoverNeedsAFreeLiveLock: the check means something only once the
// supervisor is gone. A held live.lock and an unprovable one are unclear; a
// missing live.lock reads as free, as Observe reads it, so the group probe
// decides.
func TestProbeLeftoverNeedsAFreeLiveLock(t *testing.T) {
	svc, runDir, _ := quiescedRun(t)
	withLeftoverProbes(t, probeLive, probeAbsent) // would be leftover
	lockPath := filepath.Join(runDir, liveLockFile)

	t.Run("held", func(t *testing.T) {
		f, busy, err := TryExclusiveLock(lockPath)
		if err != nil || busy {
			t.Fatalf("take the live lock: busy=%v err=%v", busy, err)
		}
		defer f.Close()
		if got := mustProbeLeftover(t, svc, runDir); got.Answer != LeftoverUnclear {
			t.Fatalf("a held live lock: got %+v, want unclear", got)
		}
	})
	t.Run("unprovable", func(t *testing.T) {
		if err := os.Remove(lockPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(lockPath, 0o700); err != nil { // opening a directory O_RDWR fails: probeUnknown
			t.Fatal(err)
		}
		defer func() {
			_ = os.Remove(lockPath)
			_ = os.WriteFile(lockPath, nil, 0o600)
		}()
		if got := mustProbeLeftover(t, svc, runDir); got.Answer != LeftoverUnclear {
			t.Fatalf("an unprovable live lock: got %+v, want unclear", got)
		}
	})
	t.Run("missing", func(t *testing.T) {
		if err := os.Remove(lockPath); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.WriteFile(lockPath, nil, 0o600) }()
		if got := mustProbeLeftover(t, svc, runDir); got.Answer != LeftoverPresent {
			t.Fatalf("a missing live lock reads as free: got %+v, want leftover", got)
		}
	})
}

// TestProbeLeftoverValidationFailureIsAnError: Observe's validation applies —
// a manifest whose run id disagrees with its directory, or no manifest at all,
// is an error (callers treat it as unclear), never an answer.
func TestProbeLeftoverValidationFailureIsAnError(t *testing.T) {
	svc, runDir, orig := quiescedRun(t)
	withLeftoverProbes(t, probeLive, probeAbsent) // would be leftover

	m := orig
	m.RunID = "ffffffffffffffffffffffffffffffff"
	writeManifestCopy(t, runDir, m)
	if got, err := svc.ProbeLeftover(runDir); err == nil || got.Answer != LeftoverUnclear {
		t.Fatalf("a disagreeing manifest: got %+v err %v, want an error with an unclear answer", got, err)
	}

	if err := os.Remove(filepath.Join(runDir, manifestFile)); err != nil {
		t.Fatal(err)
	}
	if got, err := svc.ProbeLeftover(runDir); err == nil || got.Answer != LeftoverUnclear {
		t.Fatalf("no manifest: got %+v err %v, want an error with an unclear answer", got, err)
	}
}
