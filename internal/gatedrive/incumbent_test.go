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

// incumbentDriveID resolves the one drive whose admission token is the slot's
// current token — the same link reconciliation follows.
func incumbentDriveID(t *testing.T, store *Store, token string) string {
	t.Helper()
	id, _, f := store.findIncumbentDrive(token)
	if f != "" {
		t.Fatalf("seed incumbent drive not resolvable: %s", f)
	}
	return id
}

// TestReconcileSafeRefusalRows pins every remaining safe-refusal row of the
// finished-incumbent decision table (proveIncumbentFinished / proveDriveFinished /
// reconcileFinishedIncumbent): each refuses with its exact bounded finding, leaves
// the slot's token and state untouched, and sends no stop. Every row is built so
// that removing its guard would otherwise SETTLE (or change the finding), so each
// guard is mutation-visible.
func TestReconcileSafeRefusalRows(t *testing.T) {
	type seeded struct {
		token string
		state admissionState
		// reconcileEpoch, when set, reconciles directly with this requesting epoch
		// instead of going through Admit (the epoch fence is only reachable when the
		// slot is foreign to the caller, which the reserve fences before any
		// incumbent refusal exists).
		direct         bool
		reconcileEpoch string
	}
	cases := []struct {
		name    string
		seam    *incumbentSeam
		seed    func(t *testing.T, d *Driver, store *Store, seam *incumbentSeam, req StartRequest) seeded
		finding string
	}{
		{
			// A HALTED drive with full teardown proof would settle — but its launch
			// claim is held by another claimant, who may be relaunching it.
			name:    "claim busy",
			seam:    &incumbentSeam{classify: classifyAs("terminal")},
			finding: "incumbent-claim-busy",
			seed: func(t *testing.T, d *Driver, store *Store, _ *incumbentSeam, req StartRequest) seeded {
				token := haltedDriveIncumbent(t, d, store, req)
				claim, busy, err := store.tryRelaunchClaim(incumbentDriveID(t, store, token))
				if err != nil || busy {
					t.Fatalf("hold seed claim: busy=%v err=%v", busy, err)
				}
				t.Cleanup(claim.close)
				return seeded{token: token, state: admissionExecuting}
			},
		},
		{
			// A PASSED drive would settle on its own record — but an unattached
			// relaunch reservation may still launch or be live.
			name:    "relaunch pending",
			seam:    &incumbentSeam{classify: classifyAs("terminal")},
			finding: "incumbent-relaunch-pending",
			seed: func(t *testing.T, d *Driver, store *Store, seam *incumbentSeam, req StartRequest) seeded {
				token := finishedDriveIncumbent(t, d, store, seam, req)
				if err := store.ownerCAS(incumbentDriveID(t, store, token), func(r *driveRecord) error {
					r.RelaunchReserved = true
					return nil
				}); err != nil {
					t.Fatalf("reserve relaunch: %v", err)
				}
				return seeded{token: token, state: admissionExecuting}
			},
		},
		{
			// A raw reservation with no confirmed run dir: its launcher may sit
			// between reserve and launch, even though the proof seam would call any
			// run terminal.
			name:    "raw reservation pending",
			seam:    &incumbentSeam{classify: classifyAs("terminal")},
			finding: "incumbent-reservation-pending",
			seed: func(t *testing.T, _ *Driver, store *Store, _ *incumbentSeam, req StartRequest) seeded {
				token, err := store.ReserveRawWorktreeExecution("repo-x", req.Worktree, nil)
				if err != nil {
					t.Fatalf("raw reserve: %v", err)
				}
				return seeded{token: token, state: admissionReserved}
			},
		},
		{
			// Two readable drives carry the slot's token: the incumbent is not
			// uniquely identified, so neither is trusted — even though both PASSED.
			name:    "drive ambiguous",
			seam:    &incumbentSeam{},
			finding: "incumbent-drive-ambiguous",
			seed: func(t *testing.T, d *Driver, store *Store, seam *incumbentSeam, req StartRequest) seeded {
				other := incumbentStart(t)
				otherToken := finishedDriveIncumbent(t, d, store, seam, other)
				otherID := incumbentDriveID(t, store, otherToken)
				token := finishedDriveIncumbent(t, d, store, seam, req)
				if err := store.ownerCAS(otherID, func(r *driveRecord) error {
					r.AdmissionToken = token
					return nil
				}); err != nil {
					t.Fatalf("alias drive token: %v", err)
				}
				return seeded{token: token, state: admissionExecuting}
			},
		},
		{
			// A PASSED drive a DIFFERENT run epoch owns: the requester never touches
			// it, however provably finished it is.
			name:    "epoch fenced",
			seam:    &incumbentSeam{},
			finding: "incumbent-epoch-fenced",
			seed: func(t *testing.T, d *Driver, store *Store, seam *incumbentSeam, req StartRequest) seeded {
				req.RunEpochID = "E-owner"
				token := finishedDriveIncumbent(t, d, store, seam, req)
				if got := mustSlot(t, store, req.Worktree).RunEpochID; got != "E-owner" {
					t.Fatalf("seed slot epoch = %q, want E-owner", got)
				}
				return seeded{token: token, state: admissionExecuting, direct: true, reconcileEpoch: "E-other"}
			},
		},
		{
			// A HALTED drive that never attached a run: its exact admission
			// reservation resolves unresolved (not never-launched), so there is no
			// teardown proof.
			name: "halted never attached, reservation unresolved",
			seam: &incumbentSeam{
				classify: classifyAs("terminal"),
				fakeProc: fakeProc{resolve: func(string, string) (*process.ReservationResolution, error) {
					return &process.ReservationResolution{Disposition: "unresolved"}, nil
				}},
			},
			finding: "incumbent-halted-unproven",
			seed: func(t *testing.T, d *Driver, store *Store, _ *incumbentSeam, req StartRequest) seeded {
				token := haltedNeverAttached(t, d, store, req)
				return seeded{token: token, state: admissionExecuting}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seam := tc.seam
			d, store := newIncumbentDriver(t, seam)
			req := incumbentStart(t)
			sd := tc.seed(t, d, store, seam, req)
			stops := seam.stopN

			if sd.direct {
				settled, finding, err := store.reconcileFinishedIncumbent(req.Worktree, sd.reconcileEpoch, seam)
				if settled || err != nil {
					t.Fatalf("reconcile settled=%v err=%v, want a clean refusal", settled, err)
				}
				if finding != tc.finding {
					t.Fatalf("finding = %q, want %s", finding, tc.finding)
				}
				slot := mustSlot(t, store, req.Worktree)
				if slot.ReservationToken != sd.token || slot.State != sd.state {
					t.Fatalf("slot changed under a refusal: token match=%v state=%s", slot.ReservationToken == sd.token, slot.State)
				}
			} else {
				ticket, err := d.Admit(req)
				if ticket != nil {
					t.Fatalf("an unproven incumbent must not admit")
				}
				oe := requireIncumbentRefusal(t, err, store, req.Worktree, sd.token, sd.state)
				if oe.Reconciliation != tc.finding {
					t.Fatalf("finding = %q, want %s", oe.Reconciliation, tc.finding)
				}
			}
			if seam.stopN != stops {
				t.Fatalf("reconciliation sent %d stop signal(s)", seam.stopN-stops)
			}
		})
	}
}

// haltedNeverAttached seeds a HALTED drive with no recorded run dir (current or
// prior): the shape of a drive that halted before its launch attached, whose only
// possible teardown proof is ResolveReservation on its exact admission token.
func haltedNeverAttached(t *testing.T, d *Driver, store *Store, req StartRequest) string {
	t.Helper()
	token := haltedDriveIncumbent(t, d, store, req)
	if err := store.ownerCAS(incumbentDriveID(t, store, token), func(r *driveRecord) error {
		r.RawRunDir = ""
		r.PriorRawRunDir = ""
		return nil
	}); err != nil {
		t.Fatalf("clear run dirs: %v", err)
	}
	return token
}

// TestAdmitSettlesHaltedNeverLaunched: a HALTED drive that never attached a run is
// settled only on ResolveReservation's never-launched verdict for its EXACT
// admission token — and that verdict is consulted, not the process classifier.
func TestAdmitSettlesHaltedNeverLaunched(t *testing.T) {
	var gotTokens []string
	seam := &incumbentSeam{fakeProc: fakeProc{resolve: func(_, token string) (*process.ReservationResolution, error) {
		gotTokens = append(gotTokens, token)
		return &process.ReservationResolution{Disposition: "never-launched"}, nil
	}}}
	d, store := newIncumbentDriver(t, seam)
	req := incumbentStart(t)
	token := haltedNeverAttached(t, d, store, req)

	stops, resolves := seam.stopN, seam.resolveN
	ticket, err := d.Admit(req)
	if err != nil || ticket == nil {
		t.Fatalf("Admit over a proven never-launched HALTED incumbent: ticket=%v err=%v", ticket, err)
	}
	if seam.resolveN == resolves || gotTokens[len(gotTokens)-1] != token {
		t.Fatalf("never-launched proof not resolved for the exact incumbent token (calls=%d)", seam.resolveN-resolves)
	}
	if seam.classifyN != 0 {
		t.Fatalf("a never-attached drive has no run to classify, got %d classify call(s)", seam.classifyN)
	}
	if seam.stopN != stops {
		t.Fatalf("admission sent a stop signal to make room")
	}
}
