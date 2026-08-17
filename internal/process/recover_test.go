package process

import (
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"testing"
	"time"
)

func TestRecoverMarksCleanlyAbandonedOwnedRun(t *testing.T) {
	svc := newTestService(t)
	root := t.TempDir()
	out := launchHelper(t, svc, root, "sleep")
	m, _ := readManifest(out.RunDir)
	// KILL the whole group: supervisor dies without a terminal record —
	// the abandoned shape.
	signalGroup(m.PGID, syscall.SIGKILL)
	waitFor(t, "lock release", 30*time.Second, func() bool {
		held, _ := probeFlock(filepath.Join(out.RunDir, liveLockFile))
		return !held
	})
	waitFor(t, "group gone", 30*time.Second, func() bool {
		return groupAlive(m.PGID) == probeAbsent
	})
	res, err := svc.Recover(root)
	if err != nil {
		t.Fatal(err)
	}
	if res.Marked != 1 || len(res.Entries) != 1 || res.Entries[0].Disposition != "abandoned-marked" {
		t.Fatalf("recover: %+v", res)
	}
	if rec, _ := readAbandoned(out.RunDir); rec == nil {
		t.Fatalf("abandoned.json not written")
	}
	// Observation now carries the stable cause.
	obs, _ := svc.Observe(out.RunDir)
	if obs.State != StateVanished || obs.Cause == "" {
		t.Fatalf("post-recovery observe: %+v", obs)
	}
	// Idempotent: second pass marks nothing.
	res2, _ := svc.Recover(root)
	if res2.Marked != 0 || res2.Entries[0].Disposition != "already-abandoned" {
		t.Fatalf("second recover: %+v", res2)
	}
}

// TestRecoverDoesNotProbeLiveForZeroPGID covers the leaked-slot shape the
// finding names: spawnSupervisor failed after the allocated manifest was
// written (PGID 0, phase "allocated") and the launcher released the live lock.
// Recover sees a free lock, no terminal/stopped/abandoned record, and probes
// the recorded group via the default recoverGroupProbe (groupAlive). PGID 0
// must NOT resolve probeLive — that would address the caller's own group and
// wedge the slot at needs-inspection-via-live-group forever. The fail-closed
// guard routes it to probeUnknown instead, still leaving it for inspection but
// never falsely live.
func TestRecoverDoesNotProbeLiveForZeroPGID(t *testing.T) {
	svc := newTestService(t)
	root := t.TempDir()
	id := "abababababababababababababababab"
	runDir := filepath.Join(root, id)
	if err := os.Mkdir(runDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomicJSON(filepath.Join(runDir, manifestFile), &manifestRecord{
		Schema: recordSchema, RunID: id, RunDir: runDir, PGID: 0, Phase: "allocated",
	}); err != nil {
		t.Fatal(err)
	}
	// The load-bearing assertion: the recorded group 0 the recover probe sees is
	// never live. groupAlive is recoverGroupProbe's production value.
	if got := groupAlive(0); got == probeLive {
		t.Fatal("group 0 probed live — addresses the caller's own process group")
	}
	res, err := svc.Recover(root)
	if err != nil {
		t.Fatal(err)
	}
	if res.Marked != 0 || len(res.Entries) != 1 {
		t.Fatalf("recover: %+v", res)
	}
	if d := res.Entries[0].Disposition; d != "needs-inspection" {
		t.Fatalf("PGID:0 slot disposition = %q, want needs-inspection", d)
	}
	if rec, _ := readAbandoned(runDir); rec != nil {
		t.Fatal("abandoned.json written for a non-real recorded group")
	}
}

func TestRecoverRetainsLiveForeignAndInvalid(t *testing.T) {
	svc := newTestService(t)
	root := t.TempDir()
	live := launchHelper(t, svc, root, "sleep")
	// Foreign: a directory that is not run-id-shaped, with content.
	foreign := filepath.Join(root, "not-docket")
	os.Mkdir(foreign, 0o755)
	os.WriteFile(filepath.Join(foreign, "keep.txt"), []byte("bytes"), 0o644)
	// Invalid: run-id-shaped but malformed manifest.
	badID := "ffffffffffffffffffffffffffffffff"
	bad := filepath.Join(root, badID)
	os.Mkdir(bad, 0o700)
	os.WriteFile(filepath.Join(bad, manifestFile), []byte("{broken"), 0o600)
	// Symlink at a run slot.
	linkID := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	os.Symlink(bad, filepath.Join(root, linkID))

	res, err := svc.Recover(root)
	if err != nil {
		t.Fatal(err)
	}
	if res.Marked != 0 {
		t.Fatalf("marked %d, want 0", res.Marked)
	}
	byID := map[string]string{}
	for _, e := range res.Entries {
		byID[e.RunID] = e.Disposition
	}
	if byID[live.RunID] != "live" || byID[badID] != "invalid" {
		t.Fatalf("dispositions: %v", byID)
	}
	// Foreign and invalid state byte-untouched.
	if b, _ := os.ReadFile(filepath.Join(foreign, "keep.txt")); string(b) != "bytes" {
		t.Fatalf("foreign content touched")
	}
	if b, _ := os.ReadFile(filepath.Join(bad, manifestFile)); string(b) != "{broken" {
		t.Fatalf("invalid manifest rewritten")
	}
	// Deterministic order.
	ids := make([]string, len(res.Entries))
	for i, e := range res.Entries {
		ids[i] = e.RunID
	}
	if !sort.StringsAreSorted(ids) {
		t.Fatalf("entries not sorted by run id: %v", ids)
	}
	svc.Stop(live.RunDir, "teardown")
}

// TestRecoverLeavesUnprovableGroupForInspection is the destructive-branch
// guard's proof (plan Task 10, Step 5). A manifest with PGID/SupervisorPID 1 is
// inert for a different reason than a real EPERM read: groupAlive fails closed
// on any pgid <= 1, so groupAlive(1) returns probeUnknown without ever issuing
// kill(-1, 0) — the same disposition (needs-inspection, no marker) a genuine
// probe error yields, but reached by the guard rather than by the probe. So a
// PGID=1 case cannot exercise the real probeUnknown-from-EPERM path the mutation
// targets. This case instead forces the recorded-group probe to be permanently
// unprovable —
// the shape a real EPERM read (e.g. an unreaped zombie group leader, or another
// user's group) produces — and asserts the run is left for inspection with NO
// marker. Routing probeUnknown into the clean-absence arm reddens it on every
// platform.
func TestRecoverLeavesUnprovableGroupForInspection(t *testing.T) {
	svc := newTestService(t)
	root := t.TempDir()
	out := launchHelper(t, svc, root, "sleep")
	m, _ := readManifest(out.RunDir)
	signalGroup(m.PGID, syscall.SIGKILL)
	waitFor(t, "lock release", 30*time.Second, func() bool {
		held, _ := probeFlock(filepath.Join(out.RunDir, liveLockFile))
		return !held
	})
	// A probe error is not clean absence: the recorded group is unprovable, so
	// the run must be left for inspection and never marked abandoned.
	prev := recoverGroupProbe
	recoverGroupProbe = func(int) probeAnswer { return probeUnknown }
	defer func() { recoverGroupProbe = prev }()

	res, err := svc.Recover(root)
	if err != nil {
		t.Fatal(err)
	}
	if res.Marked != 0 || len(res.Entries) != 1 || res.Entries[0].Disposition != "needs-inspection" {
		t.Fatalf("recover: %+v", res)
	}
	if rec, _ := readAbandoned(out.RunDir); rec != nil {
		t.Fatalf("abandoned.json written for an unprovable group")
	}
}
