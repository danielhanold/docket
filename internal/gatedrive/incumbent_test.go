package gatedrive

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// Change 0446 spec §3: a worktree slot whose incumbent execution has provably
// finished — but whose release was interrupted before its owner persisted it — is
// settled by the NEXT normal admission instead of refusing that admission. These
// tests pin the decision table: a supervisor-committed PASSED/FAILED drive, and a
// raw or HALTED execution with positive process teardown proof, settle; a live,
// nonterminal, or unprovable incumbent, and a failed release write, still refuse;
// admission never sends a stop signal to make room.

// incumbentSeam is fakeProc with a scriptable ClassifyRun, the process-recovery
// predicate reconciliation consults for a raw or HALTED incumbent.
type incumbentSeam struct {
	fakeProc
	classify  func(runDir string, mark bool) (process.RecoveryEntry, error)
	classifyN int
	classDirs []string
}

func (p *incumbentSeam) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	p.classifyN++
	p.classDirs = append(p.classDirs, runDir)
	if p.classify == nil {
		return process.RecoveryEntry{Disposition: "invalid"}, nil
	}
	return p.classify(runDir, mark)
}

func classifyAs(disposition string) func(string, bool) (process.RecoveryEntry, error) {
	return func(runDir string, _ bool) (process.RecoveryEntry, error) {
		return process.RecoveryEntry{RunDir: runDir, Disposition: disposition}, nil
	}
}

// newIncumbentDriver wires a driver over seam with the same short-slice test
// plumbing newTestDriver uses.
func newIncumbentDriver(t *testing.T, seam ProcessSeam) (*Driver, *Store) {
	t.Helper()
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	d := NewDriver(store, clk, seam, stableGit())
	d.slice = 4 * pollTick
	d.pollInterval = pollTick
	d.sleep = func(dur time.Duration) { clk.advance(dur) }
	return d, store
}

// incumbentStart is sampleStart on a worktree private to the test.
func incumbentStart(t *testing.T) StartRequest {
	t.Helper()
	req := sampleStart()
	req.Worktree = mkWorktree(t)
	return req
}

// forceSlotState rewrites the slot's state under the real CAS, keeping its token:
// the durable shape an interrupted release (or a HALTED drive whose release never
// ran) leaves behind.
func forceSlotState(t *testing.T, store *Store, worktree string, st admissionState) {
	t.Helper()
	if err := store.admissionCAS(worktree, func(rec *admissionRecord) error {
		rec.State = st
		return nil
	}); err != nil {
		t.Fatalf("force slot state %s: %v", st, err)
	}
}

func mustSlot(t *testing.T, store *Store, worktree string) admissionRecord {
	t.Helper()
	slot, _, err := store.LoadWorktreeExecution(worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	return slot
}

// finishedDriveIncumbent drives a scopeless start to PASSED, then rewrites its
// released slot back to executing under the same token — a proven-finished drive
// whose release was interrupted. It returns the incumbent token.
func finishedDriveIncumbent(t *testing.T, d *Driver, store *Store, seam *incumbentSeam, req StartRequest) string {
	t.Helper()
	seam.observe = func(runDir string) (*process.Observation, error) { return obs(process.StatePassed, runDir), nil }
	doc, err := d.Start(req)
	if err != nil || doc.Outcome != PASSED {
		t.Fatalf("seed start: outcome=%s err=%v", doc.Outcome, err)
	}
	forceSlotState(t, store, req.Worktree, admissionExecuting)
	return mustSlot(t, store, req.Worktree).ReservationToken
}

// haltedDriveIncumbent starts a scopeless drive that stays WAITING (slot executing),
// then settles the drive record HALTED without releasing its slot: a HALTED label
// with no teardown proof of its own.
func haltedDriveIncumbent(t *testing.T, d *Driver, store *Store, req StartRequest) (token string) {
	t.Helper()
	doc, err := d.Start(req)
	if err != nil || doc.Outcome != WAITING {
		t.Fatalf("seed start: outcome=%s err=%v", doc.Outcome, err)
	}
	if err := store.ownerCAS(doc.DriveID, func(r *driveRecord) error {
		r.LastOutcome = HALTED
		r.LastCause = "stopped-not-initiated"
		return nil
	}); err != nil {
		t.Fatalf("halt seed drive: %v", err)
	}
	slot := mustSlot(t, store, req.Worktree)
	if slot.State != admissionExecuting {
		t.Fatalf("seed slot state = %s, want executing", slot.State)
	}
	return slot.ReservationToken
}

// rawIncumbent reserves and confirms a raw execution with a recorded run dir.
func rawIncumbent(t *testing.T, store *Store, worktree string) (token, runDir string) {
	t.Helper()
	token, err := store.ReserveRawWorktreeExecution("repo-x", worktree, nil)
	if err != nil {
		t.Fatalf("raw reserve: %v", err)
	}
	const runID = "0446cccccccccccccccccccccccccc01"
	runDir = "/runs/" + runID
	if err := store.ConfirmWorktreeExecution(worktree, token, runID, runDir); err != nil {
		t.Fatalf("raw confirm: %v", err)
	}
	return token, runDir
}

// requireIncumbentRefusal asserts err is a worktree-admission refusal carrying a
// reconciliation finding, and that the slot is byte-for-byte the incumbent's.
func requireIncumbentRefusal(t *testing.T, err error, store *Store, worktree, token string, st admissionState) *OwnershipError {
	t.Helper()
	oe, ok := AsOwnershipError(err)
	if !ok || (oe.Kind != ErrWorktreeBusy && oe.Kind != ErrUnresolvedExecution) {
		t.Fatalf("admit err = %v, want a worktree-busy/unresolved refusal", err)
	}
	if oe.Reconciliation == "" {
		t.Fatalf("refusal carries no reconciliation finding: %+v", oe)
	}
	slot := mustSlot(t, store, worktree)
	if slot.ReservationToken != token || slot.State != st {
		t.Fatalf("slot changed under a refusal: token match=%v state=%s, want state %s",
			slot.ReservationToken == token, slot.State, st)
	}
	return oe
}

// TestAdmitSettlesFinishedIncumbent: a PASSED drive whose release was interrupted,
// and a raw run with positive process teardown proof, are both settled by the next
// normal Admit, which then admits and launches exactly once — with no stop signal
// sent to make room.
func TestAdmitSettlesFinishedIncumbent(t *testing.T) {
	t.Run("passed drive incumbent", func(t *testing.T) {
		seam := &incumbentSeam{}
		d, store := newIncumbentDriver(t, seam)
		req := incumbentStart(t)
		old := finishedDriveIncumbent(t, d, store, seam, req)

		stops, launches := seam.stopN, seam.launchN
		ticket, err := d.Admit(req)
		if err != nil {
			t.Fatalf("Admit over a proven-finished incumbent: %v", err)
		}
		if seam.stopN != stops {
			t.Fatalf("admission sent %d stop signal(s) to make room", seam.stopN-stops)
		}
		slot := mustSlot(t, store, req.Worktree)
		if slot.State != admissionReserved || slot.ReservationToken != ticket.token || slot.ReservationToken == old {
			t.Fatalf("slot after settle+admit: state=%s fresh-token=%v", slot.State, slot.ReservationToken != old)
		}
		if _, err := d.StartAdmitted(ticket); err != nil {
			t.Fatalf("StartAdmitted: %v", err)
		}
		if seam.launchN != launches+1 {
			t.Fatalf("launches = %d, want exactly one new launch", seam.launchN-launches)
		}
	})

	t.Run("raw incumbent with teardown proof", func(t *testing.T) {
		seam := &incumbentSeam{classify: classifyAs("terminal")}
		d, store := newIncumbentDriver(t, seam)
		req := incumbentStart(t)
		_, runDir := rawIncumbent(t, store, req.Worktree)

		ticket, err := d.Admit(req)
		if err != nil {
			t.Fatalf("Admit over a proven-finished raw incumbent: %v", err)
		}
		if ticket == nil || seam.stopN != 0 {
			t.Fatalf("ticket=%v stops=%d, want a ticket and no stop", ticket, seam.stopN)
		}
		if len(seam.classDirs) == 0 || seam.classDirs[0] != runDir {
			t.Fatalf("teardown proof consulted %v, want the incumbent run dir %q", seam.classDirs, runDir)
		}
	})

	t.Run("halted drive incumbent with teardown proof", func(t *testing.T) {
		seam := &incumbentSeam{classify: classifyAs("terminal")}
		d, store := newIncumbentDriver(t, seam)
		req := incumbentStart(t)
		haltedDriveIncumbent(t, d, store, req)

		stops := seam.stopN
		if _, err := d.Admit(req); err != nil {
			t.Fatalf("Admit over a HALTED incumbent with positive teardown proof: %v", err)
		}
		if seam.stopN != stops {
			t.Fatalf("admission sent a stop signal to make room")
		}
	})
}

// TestAdmitRefusesLiveIncumbent: a raw run the process predicate reports live, and
// a nonterminal (WAITING) drive, remain safe refusals — the slot is untouched.
func TestAdmitRefusesLiveIncumbent(t *testing.T) {
	t.Run("live raw run", func(t *testing.T) {
		seam := &incumbentSeam{classify: classifyAs("live")}
		d, store := newIncumbentDriver(t, seam)
		req := incumbentStart(t)
		token, _ := rawIncumbent(t, store, req.Worktree)

		ticket, err := d.Admit(req)
		if ticket != nil {
			t.Fatalf("a live incumbent must not admit")
		}
		requireIncumbentRefusal(t, err, store, req.Worktree, token, admissionExecuting)
		if seam.stopN != 0 {
			t.Fatalf("admission stopped a live incumbent")
		}
	})

	t.Run("nonterminal drive", func(t *testing.T) {
		seam := &incumbentSeam{classify: classifyAs("terminal")}
		d, store := newIncumbentDriver(t, seam)
		req := incumbentStart(t)
		if doc, err := d.Start(req); err != nil || doc.Outcome != WAITING {
			t.Fatalf("seed start: %s %v", doc.Outcome, err)
		}
		token := mustSlot(t, store, req.Worktree).ReservationToken

		_, err := d.Admit(req)
		oe := requireIncumbentRefusal(t, err, store, req.Worktree, token, admissionExecuting)
		if oe.Reconciliation != "incumbent-nonterminal" {
			t.Fatalf("finding = %q, want incumbent-nonterminal", oe.Reconciliation)
		}
	})
}

// TestAdmitHaltedNoProofNeverReleases: a HALTED drive label is never slot-release
// proof. With the process predicate erroring (no positive teardown proof), the
// admission refuses, the slot keeps its state and token, and no stop is sent.
func TestAdmitHaltedNoProofNeverReleases(t *testing.T) {
	seam := &incumbentSeam{classify: func(string, bool) (process.RecoveryEntry, error) {
		return process.RecoveryEntry{}, errors.New("probe failed")
	}}
	d, store := newIncumbentDriver(t, seam)
	req := incumbentStart(t)
	token := haltedDriveIncumbent(t, d, store, req)

	stops := seam.stopN
	ticket, err := d.Admit(req)
	if ticket != nil {
		t.Fatalf("a HALTED incumbent without teardown proof must not admit")
	}
	oe := requireIncumbentRefusal(t, err, store, req.Worktree, token, admissionExecuting)
	if oe.Reconciliation != "incumbent-halted-unproven" {
		t.Fatalf("finding = %q, want incumbent-halted-unproven", oe.Reconciliation)
	}
	if seam.stopN != stops {
		t.Fatalf("admission sent a stop signal")
	}
}

// TestAdmitFailedReleaseWriteRefuses: when the release write itself fails, the
// admission is refused — a failed write is never reported as a successful
// admission, and the slot stays occupied.
func TestAdmitFailedReleaseWriteRefuses(t *testing.T) {
	seam := &incumbentSeam{}
	d, store := newIncumbentDriver(t, seam)
	req := incumbentStart(t)
	token := finishedDriveIncumbent(t, d, store, seam, req)

	slotDir := filepath.Dir(admissionRecordPath(t, store, req.Worktree))
	t.Cleanup(func() { _ = os.Chmod(slotDir, 0o700) })
	incumbentApplyHook = func() {
		if err := os.Chmod(slotDir, 0o500); err != nil {
			t.Errorf("chmod slot dir: %v", err)
		}
	}
	t.Cleanup(func() { incumbentApplyHook = nil })

	ticket, err := d.Admit(req)
	if ticket != nil {
		t.Fatalf("a failed release write must never admit")
	}
	oe := requireIncumbentRefusal(t, err, store, req.Worktree, token, admissionExecuting)
	if oe.Reconciliation != "release-write-failed" {
		t.Fatalf("finding = %q, want release-write-failed", oe.Reconciliation)
	}
}
