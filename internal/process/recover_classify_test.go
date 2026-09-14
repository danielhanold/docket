package process

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// makeAbandonedCandidateSlot builds one run slot in the shape classifyRun's
// final group-probe arm sees: a self-agreeing manifest past establishment
// (a supervisor pid and a real recorded group), no live.lock (so the live-lock
// probe reads free — a missing lock file is clean absence), and no terminal,
// stopped, or abandoned record. With recoverGroupProbe overridden to
// probeAbsent, such a slot is exactly the arm that would earn a fresh abandoned
// marker under an applied recovery. Adapted from the fixture construction the
// abandoned-marker Recover tests use (recover_test.go).
func makeAbandonedCandidateSlot(t *testing.T, root string) (runDir, name string) {
	t.Helper()
	name = "0123456789abcdef0123456789abcdef"
	runDir = filepath.Join(root, name)
	if err := os.Mkdir(runDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomicJSON(filepath.Join(runDir, manifestFile), &manifestRecord{
		Schema: recordSchema, RunID: name, Root: root, RunDir: runDir,
		SupervisorPID: 4242, PGID: 4242, SID: 4242, Phase: "running",
	}); err != nil {
		t.Fatal(err)
	}
	return runDir, name
}

// TestClassifyRunAssessDoesNotMark proves the only behavioral fork between
// assess (mark=false) and apply (mark=true): a provably-absent recorded group
// is reported "abandonable" and writes NO marker in assess mode, and is
// "abandoned-marked" and DOES write the marker in apply mode.
func TestClassifyRunAssessDoesNotMark(t *testing.T) {
	prev := recoverGroupProbe
	recoverGroupProbe = func(int) probeAnswer { return probeAbsent }
	defer func() { recoverGroupProbe = prev }()

	svc := newTestService(t)

	// Assess: reports abandonable, writes nothing.
	rootAssess := testsupport.TempDir(t)
	assessDir, _ := makeAbandonedCandidateSlot(t, rootAssess)
	entry, err := svc.ClassifyRun(assessDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Disposition != "abandonable" {
		t.Fatalf("assess disposition = %q, want abandonable (reason %q)", entry.Disposition, entry.Reason)
	}
	if rec, _ := readAbandoned(assessDir); rec != nil {
		t.Fatal("assess mode wrote abandoned.json; must never write in mark=false")
	}

	// Apply: marks, writes the marker.
	rootApply := testsupport.TempDir(t)
	applyDir, _ := makeAbandonedCandidateSlot(t, rootApply)
	entry, err = svc.ClassifyRun(applyDir, true)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Disposition != "abandoned-marked" {
		t.Fatalf("apply disposition = %q, want abandoned-marked (reason %q)", entry.Disposition, entry.Reason)
	}
	if rec, _ := readAbandoned(applyDir); rec == nil {
		t.Fatal("apply mode did not write abandoned.json")
	}
}

// TestClassifyRunUnprovableProbeIsNeedsInspectionBothModes proves the
// non-forking dispositions are identical in both modes: an unprovable recorded
// group is left for inspection with no marker regardless of mark.
func TestClassifyRunUnprovableProbeIsNeedsInspectionBothModes(t *testing.T) {
	prev := recoverGroupProbe
	recoverGroupProbe = func(int) probeAnswer { return probeUnknown }
	defer func() { recoverGroupProbe = prev }()

	svc := newTestService(t)
	for _, mark := range []bool{false, true} {
		root := testsupport.TempDir(t)
		runDir, _ := makeAbandonedCandidateSlot(t, root)
		entry, err := svc.ClassifyRun(runDir, mark)
		if err != nil {
			t.Fatalf("mark=%v: %v", mark, err)
		}
		if entry.Disposition != "needs-inspection" {
			t.Fatalf("mark=%v disposition = %q, want needs-inspection", mark, entry.Disposition)
		}
		if rec, _ := readAbandoned(runDir); rec != nil {
			t.Fatalf("mark=%v wrote abandoned.json for an unprovable group", mark)
		}
	}
}

// TestClassifyRunRejectsRelativeAndNonSlotPaths proves the exported wrapper's
// two boundary rules: a non-absolute runDir is an input error, and an absolute
// path whose basename is not a run id is reported "foreign" with no error.
func TestClassifyRunRejectsRelativeAndNonSlotPaths(t *testing.T) {
	svc := newTestService(t)

	if _, err := svc.ClassifyRun("relative/run/dir", false); err == nil {
		t.Fatal("relative runDir accepted; want an input error")
	}

	root := testsupport.TempDir(t)
	nonSlot := filepath.Join(root, "not-a-run-id")
	if err := os.Mkdir(nonSlot, 0o700); err != nil {
		t.Fatal(err)
	}
	entry, err := svc.ClassifyRun(nonSlot, false)
	if err != nil {
		t.Fatalf("non-slot path errored: %v", err)
	}
	if entry.Disposition != "foreign" {
		t.Fatalf("non-slot disposition = %q, want foreign", entry.Disposition)
	}
}
