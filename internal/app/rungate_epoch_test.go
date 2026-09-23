package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// These are the run-epoch registry tests (change 0375 Task 9). The epoch lives
// beside the gate record under the same gate-key directory (rungate_epoch.go); the
// gate key locates both. A fresh gate-before arm mints an active epoch; claim
// confirmation binds its change; participant registration is gated on the active
// state and fails closed on an unknown schema.

// isEpochKind reports whether err carries an *EpochError of the given kind.
func isEpochKind(err error, kind EpochErrorKind) bool {
	ee, ok := AsEpochError(err)
	return ok && ee.Kind == kind
}

// mintTestGateKey mints a minimal valid gate record and returns its key, so an
// epoch test has a real key directory (the epoch store requires one) without
// arming the whole gate. AttemptLimit is floored at 1 so the v4 write guard
// accepts it.
func mintTestGateKey(t *testing.T, repo string) string {
	t.Helper()
	key, err := MintGateRecord(repo, GateRecord{
		Target:       gateBeforeStoredTarget,
		AttemptLimit: 1,
		Retry:        RetryUnused,
		Disposition:  "gate-armed",
	})
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	return key
}

// TestEpochRecordCRUD proves mint → load → register round-trips: mint yields an
// active record with a non-empty public EpochID keyed by the gate key, load
// returns a generation, a second mint is refused bind-once, a participant is
// appended with a stamped RegisteredAt under a rotated generation, and a stale
// expected-epoch locator confers no registration authority.
func TestEpochRecordCRUD(t *testing.T) {
	repo := newGateRepo(t)
	key := mintTestGateKey(t, repo)

	rec, err := MintEpochRecord(repo, key, "375")
	if err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	if rec.State != EpochActive {
		t.Fatalf("a fresh epoch must be active, got %q", rec.State)
	}
	if rec.EpochID == "" {
		t.Fatalf("mint must assign a public EpochID locator")
	}
	if rec.GateKey != key {
		t.Fatalf("epoch must record its gate key %q, got %q", key, rec.GateKey)
	}

	// Bind-once: a second mint for the same key never clobbers the first.
	if _, err := MintEpochRecord(repo, key, "375"); !isEpochKind(err, ErrEpochExists) {
		t.Fatalf("a second mint must be refused ErrEpochExists, got %v", err)
	}

	got, gen, err := LoadEpochRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if gen == "" {
		t.Fatalf("load must return a physical generation")
	}
	if got.EpochID != rec.EpochID || got.ChangeID != "375" || got.State != EpochActive {
		t.Fatalf("load round-trip mismatch: %+v", got)
	}

	p := EpochParticipant{Kind: "coordinator", NativeHandle: "handle-1"}
	if err := RegisterEpochParticipant(repo, key, rec.EpochID, p); err != nil {
		t.Fatalf("RegisterEpochParticipant: %v", err)
	}
	got, gen2, err := LoadEpochRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadEpochRecord after register: %v", err)
	}
	if len(got.Participants) != 1 {
		t.Fatalf("participant not appended: %+v", got.Participants)
	}
	if got.Participants[0].Kind != "coordinator" || got.Participants[0].NativeHandle != "handle-1" {
		t.Fatalf("participant fields not persisted: %+v", got.Participants[0])
	}
	if got.Participants[0].RegisteredAt == "" {
		t.Fatalf("register must stamp RegisteredAt")
	}
	if gen2 == gen {
		t.Fatalf("a CAS write must rotate the physical generation")
	}

	// A stale expected-epoch locator is refused: registration binds to the exact epoch.
	if err := RegisterEpochParticipant(repo, key, "not-the-epoch", p); !isEpochKind(err, ErrEpochMismatch) {
		t.Fatalf("a stale expected-epoch must be refused ErrEpochMismatch, got %v", err)
	}
}

// TestRegisterParticipantRejectsNonActive proves a fenced (non-active) epoch admits
// no new participant: after the epoch flips active→cancelling, registration fails
// closed ErrEpochNotActive.
func TestRegisterParticipantRejectsNonActive(t *testing.T) {
	repo := newGateRepo(t)
	key := mintTestGateKey(t, repo)
	rec, err := MintEpochRecord(repo, key, "")
	if err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	// Fence the epoch through the CAS (the durable active→cancelling transition Task 10
	// drives; here it stands in so the state gate is exercised).
	if err := epochCAS(repo, key, func(r *EpochRecord) error {
		r.State = EpochCancelling
		return nil
	}); err != nil {
		t.Fatalf("epochCAS fence: %v", err)
	}
	err = RegisterEpochParticipant(repo, key, rec.EpochID, EpochParticipant{Kind: "task"})
	if !isEpochKind(err, ErrEpochNotActive) {
		t.Fatalf("register on a cancelling epoch must be refused ErrEpochNotActive, got %v", err)
	}
	// The rejected registration wrote nothing: the participant list stays empty.
	got, _, err := LoadEpochRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if len(got.Participants) != 0 {
		t.Fatalf("a rejected registration must write nothing, got %+v", got.Participants)
	}
}

// TestLoadEpochNotFound proves a load before any mint fails closed ErrEpochNotFound
// rather than fabricating a live epoch.
func TestLoadEpochNotFound(t *testing.T) {
	repo := newGateRepo(t)
	key := mintTestGateKey(t, repo)
	if _, _, err := LoadEpochRecord(repo, key); !isEpochKind(err, ErrEpochNotFound) {
		t.Fatalf("load with no epoch must be ErrEpochNotFound, got %v", err)
	}
}

// TestEpochUnknownSchemaFailsClosed proves a record carrying an unknown schema
// version fails closed ErrEpochCorrupt on load — a record the store cannot read is
// never a live epoch (premium: fail-closed schema versioning).
func TestEpochUnknownSchemaFailsClosed(t *testing.T) {
	repo := newGateRepo(t)
	key := mintTestGateKey(t, repo)
	if _, err := MintEpochRecord(repo, key, "375"); err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	common, err := gateGitCommonDir(repo)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	path := filepath.Join(common, "docket", "rungate", key, epochRecordFileName)
	bad := `{"generation":"g","record":{"schema_version":99,"gate_key":"` + key + `","state":"active","epoch_id":"e"}}`
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatalf("seed bad schema: %v", err)
	}
	if _, _, err := LoadEpochRecord(repo, key); !isEpochKind(err, ErrEpochCorrupt) {
		t.Fatalf("unknown schema must be ErrEpochCorrupt, got %v", err)
	}
}

// TestGateBeforeMintsEpoch proves a fresh (non-resume) arm binds a new run epoch
// beside the gate record, keyed by the gate key: active, with a public EpochID and
// no change bound yet.
func TestGateBeforeMintsEpoch(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, gateBeforeCorpus(), nil, nil), Clock: testClock()}
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunGateBefore(context.Background(), deps, WorkspaceDeps{}, sp.deps(), repo, "implement-next", 0)
	if !res.Armed || res.Key == "" {
		t.Fatalf("Armed=%v Key=%q, want armed", res.Armed, res.Key)
	}
	ep, _, err := LoadEpochRecord(repo, res.Key)
	if err != nil {
		t.Fatalf("a fresh arm must mint an epoch keyed by the gate key: %v", err)
	}
	if ep.State != EpochActive {
		t.Fatalf("minted epoch must be active, got %q", ep.State)
	}
	if ep.EpochID == "" {
		t.Fatalf("minted epoch must carry a public EpochID")
	}
	if ep.GateKey != res.Key {
		t.Fatalf("epoch gate key = %q, want %q", ep.GateKey, res.Key)
	}
	// A fresh arm has not yet chosen a change, so the epoch binds none until claim.
	if ep.ChangeID != "" {
		t.Fatalf("a fresh arm must bind no change yet, got %q", ep.ChangeID)
	}
}

// TestConfirmGateClaimBindsEpochChange proves the claim confirmation binds the
// epoch to the confirmed change instance — the readable locator a later
// resume/cancel resolves the run by.
// TestNoAdapterReportsLifecycleUnavailable proves an armed gate reports the honest
// owner-lifecycle limitation (change 0375 Task 13): the default dispatch route has
// no automatic Stop/owner-death cancellation, so a Stop is the explicit run.cancel
// operation. The field is a standing caveat, never a refusal — the gate still arms.
func TestNoAdapterReportsLifecycleUnavailable(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, gateBeforeCorpus(), nil, nil), Clock: testClock()}
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunGateBefore(context.Background(), deps, WorkspaceDeps{}, sp.deps(), repo, "implement-next", 0)
	if !res.Armed {
		t.Fatalf("gate must arm; got Armed=%v Reason=%q", res.Armed, res.Reason)
	}
	if res.OwnerLifecycle != ReasonOwnerLifecycleUnavailable {
		t.Fatalf("OwnerLifecycle = %q, want %q", res.OwnerLifecycle, ReasonOwnerLifecycleUnavailable)
	}
	if !strings.Contains(res.HumanText(), ReasonOwnerLifecycleUnavailable) {
		t.Fatalf("human text omits the owner-lifecycle caveat: %q", res.HumanText())
	}
}

func TestConfirmGateClaimBindsEpochChange(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, gateBeforeCorpus(), nil, nil), Clock: testClock()}
	sp := &fakeScopePrep{grant: sampleScopeGrant()}
	res := RunGateBefore(context.Background(), deps, WorkspaceDeps{}, sp.deps(), repo, "implement-next", 0)
	if !res.Armed {
		t.Fatalf("arm failed: %+v", res)
	}
	if err := ReserveGateClaim(repo, res.Key, 42, "req-1"); err != nil {
		t.Fatalf("ReserveGateClaim: %v", err)
	}
	if err := ConfirmGateClaim(repo, res.Key, 42, "req-1", "revabc123", ""); err != nil {
		t.Fatalf("ConfirmGateClaim: %v", err)
	}
	ep, _, err := LoadEpochRecord(repo, res.Key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if ep.ChangeID != "42" {
		t.Fatalf("claim confirmation must bind the epoch change id, got %q", ep.ChangeID)
	}
}

// TestConfirmGateClaimNoEpochIsNoop proves a claim over a dispatch with NO epoch
// (a standalone gate record) is unaffected: the confirm succeeds and no epoch is
// fabricated. This guards the existing claim path against the epoch bind.
func TestConfirmGateClaimNoEpochIsNoop(t *testing.T) {
	repo := newGateRepo(t)
	key := mintTestGateKey(t, repo) // a gate record with no epoch minted beside it
	if err := ReserveGateClaim(repo, key, 7, "req-x"); err != nil {
		t.Fatalf("ReserveGateClaim: %v", err)
	}
	if err := ConfirmGateClaim(repo, key, 7, "req-x", "rev-x", ""); err != nil {
		t.Fatalf("ConfirmGateClaim over a no-epoch dispatch must succeed: %v", err)
	}
	if _, _, err := LoadEpochRecord(repo, key); !isEpochKind(err, ErrEpochNotFound) {
		t.Fatalf("no epoch must be fabricated by the claim, got %v", err)
	}
}

// mintEpochFixture mints a gate-key directory and an active epoch beside it
// (change 0441), returning the repo and the gate key the completion-lifecycle
// tests drive. changeID "441" mirrors the change under test.
func mintEpochFixture(t *testing.T) (repo, key string) {
	t.Helper()
	repo = newGateRepo(t)
	key = mintTestGateKey(t, repo)
	if _, err := MintEpochRecord(repo, key, "441"); err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	return repo, key
}

// forceEpochState drives the epoch record to state s through the CAS, standing in
// for the durable transitions other tasks own so a lifecycle guard can be exercised
// against an arbitrary state.
func forceEpochState(t *testing.T, repo, key string, s epochState) {
	t.Helper()
	if err := epochCAS(repo, key, func(r *EpochRecord) error {
		r.State = s
		return nil
	}); err != nil {
		t.Fatalf("forceEpochState %q: %v", s, err)
	}
}

// must fails the test immediately when err is non-nil, so a fixture setup step
// whose failure is not the assertion under test reads as one line.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFenceEpochCompletingFromActive(t *testing.T) {
	repo, key := mintEpochFixture(t) // reuse/extract the file's existing mint helper; changeID "441"
	st, err := FenceEpochCompleting(repo, key, "")
	if err != nil || st != EpochCompleting {
		t.Fatalf("fence: state %q err %v", st, err)
	}
	rec, _, _ := LoadEpochRecord(repo, key)
	if rec.State != EpochCompleting {
		t.Fatalf("persisted state %q", rec.State)
	}
	// Idempotent replay resumes the same closeout.
	if st, err = FenceEpochCompleting(repo, key, ""); err != nil || st != EpochCompleting {
		t.Fatalf("replay: state %q err %v", st, err)
	}
}

func TestFenceEpochCompletingNeverRelabelsTerminalStates(t *testing.T) {
	for _, s := range []epochState{EpochCancelling, EpochCancelled, EpochSuperseded, epochState("garbage")} {
		repo, key := mintEpochFixture(t)
		forceEpochState(t, repo, key, s) // helper: epochCAS setting rec.State = s
		st, err := FenceEpochCompleting(repo, key, "")
		ee, ok := AsEpochError(err)
		if !ok || ee.Kind != ErrEpochNotActive || st != s {
			t.Fatalf("state %q: got st %q err %v", s, st, err)
		}
		rec, _, _ := LoadEpochRecord(repo, key)
		if rec.State != s {
			t.Fatalf("state %q was rewritten to %q", s, rec.State)
		}
	}
}

func TestFenceEpochCompletingRejectsStaleLocator(t *testing.T) {
	repo, key := mintEpochFixture(t)
	_, err := FenceEpochCompleting(repo, key, "not-the-epoch-id")
	if ee, ok := AsEpochError(err); !ok || ee.Kind != ErrEpochMismatch {
		t.Fatalf("err %v", err)
	}
}

func TestCompleteEpochOnlyFromCompleting(t *testing.T) {
	repo, key := mintEpochFixture(t)
	if err := CompleteEpoch(repo, key); err == nil {
		t.Fatal("completed from active") // never a shortcut past the fence
	}
	_, _ = FenceEpochCompleting(repo, key, "")
	if err := CompleteEpoch(repo, key); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if err := CompleteEpoch(repo, key); err != nil {
		t.Fatalf("idempotent replay: %v", err) // completed receipt replay is safe
	}
	// Cancellation that won from completing makes completion lose.
	repo2, key2 := mintEpochFixture(t)
	_, _ = FenceEpochCompleting(repo2, key2, "")
	forceEpochState(t, repo2, key2, EpochCancelling)
	if err := CompleteEpoch(repo2, key2); err == nil {
		t.Fatal("completion must lose to a cancellation that won")
	}
}

func TestRegisterEpochParticipantRejectedOnCompletingAndCompleted(t *testing.T) {
	for _, s := range []epochState{EpochCompleting, EpochCompleted} {
		repo, key := mintEpochFixture(t)
		forceEpochState(t, repo, key, s)
		err := RegisterEpochParticipant(repo, key, "", EpochParticipant{Kind: "task", NativeHandle: "h"})
		if ee, ok := AsEpochError(err); !ok || ee.Kind != ErrEpochNotActive {
			t.Fatalf("state %q admitted a registration: %v", s, err)
		}
	}
}

func TestSupersedeRefusesCompletingAndCompleted(t *testing.T) {
	for _, s := range []epochState{EpochCompleting, EpochCompleted} {
		repo, key := mintEpochFixture(t)
		forceEpochState(t, repo, key, s)
		err := SupersedeCancelledEpoch(repo, key, "replacement-key")
		if ee, ok := AsEpochError(err); !ok || ee.Kind != ErrEpochNotCancelled {
			t.Fatalf("state %q superseded: %v", s, err)
		}
	}
}

func TestRecordEpochParticipantTerminal(t *testing.T) {
	repo, key := mintEpochFixture(t)
	must(t, RegisterEpochParticipant(repo, key, "", EpochParticipant{Kind: "coordinator", NativeHandle: "thread-1"}))
	must(t, RecordEpochParticipantTerminal(repo, key, "", "thread-1", "turn-9", ParticipantTerminalCompleted))
	rec, _, _ := LoadEpochRecord(repo, key)
	p := rec.Participants[0]
	if p.TerminalStatus != ParticipantTerminalCompleted || p.TerminalTurn != "turn-9" || p.TerminalObservedAt == "" {
		t.Fatalf("evidence not persisted: %+v", p)
	}
	// Idempotent identical replay; conflicting evidence fails closed.
	must(t, RecordEpochParticipantTerminal(repo, key, "", "thread-1", "turn-9", ParticipantTerminalCompleted))
	if err := RecordEpochParticipantTerminal(repo, key, "", "thread-1", "turn-9", ParticipantTerminalFailed); err == nil {
		t.Fatal("conflicting terminal status accepted")
	}
	if err := RecordEpochParticipantTerminal(repo, key, "", "thread-1", "other-turn", ParticipantTerminalCompleted); err == nil {
		t.Fatal("mismatched turn accepted") // AC4: mismatched turn cannot satisfy
	}
}

func TestRecordEpochParticipantTerminalUnknownHandleAndBadInput(t *testing.T) {
	repo, key := mintEpochFixture(t)
	err := RecordEpochParticipantTerminal(repo, key, "", "ghost", "t", ParticipantTerminalCompleted)
	if ee, ok := AsEpochError(err); !ok || ee.Kind != ErrEpochParticipantUnknown {
		t.Fatalf("err %v", err)
	}
	for _, bad := range [][3]string{{"", "t", "completed"}, {"h", "", "completed"}, {"h", "t", ""}, {"h", "t", "yielded"}} {
		if RecordEpochParticipantTerminal(repo, key, "", bad[0], bad[1], bad[2]) == nil {
			t.Fatalf("malformed evidence %v accepted", bad)
		}
	}
}

func TestRecordEpochParticipantTerminalAllowedAfterFence(t *testing.T) {
	// "Completion of an existing participant is allowed after the completing
	// fence; registering or reopening work is not."
	for _, s := range []epochState{EpochCompleting, EpochCancelling} {
		repo, key := mintEpochFixture(t)
		must(t, RegisterEpochParticipant(repo, key, "", EpochParticipant{Kind: "task", NativeHandle: "h1"}))
		forceEpochState(t, repo, key, s)
		must(t, RecordEpochParticipantTerminal(repo, key, "", "h1", "turn-1", ParticipantTerminalFailed))
	}
}

// TestEpochSettledResolverStates (change 0446): the admission settlement read
// reports settled only for an epoch whose record is terminal with its accounting
// done — completed, cancelled, superseded. Active, cancelling, and completing
// epochs still own their worktree, and an unknown epoch id is an unresolved owner,
// never settlement.
func TestEpochSettledResolverStates(t *testing.T) {
	cases := []struct {
		state   epochState
		settled bool
	}{
		{EpochActive, false},
		{EpochCancelling, false},
		{EpochCompleting, false},
		{EpochCancelled, true},
		{EpochSuperseded, true},
		{EpochCompleted, true},
	}
	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			repo, common, key, epochID, _ := epochGateFixture(t)
			if tc.state != EpochActive {
				fenceEpoch(t, repo, key, tc.state)
			}
			settled, err := epochSettledResolver(common)(epochID)
			if err != nil {
				t.Fatalf("resolver err: %v", err)
			}
			if settled != tc.settled {
				t.Fatalf("state %q settled = %v, want %v", tc.state, settled, tc.settled)
			}
		})
	}
	t.Run("unknown-epoch", func(t *testing.T) {
		_, common, _, _, _ := epochGateFixture(t)
		settled, err := epochSettledResolver(common)("0123456789abcdef0123456789abcdef")
		if !errors.Is(err, gatedrive.ErrEpochUnresolved) || settled {
			t.Fatalf("unknown epoch = (%v, %v), want (false, ErrEpochUnresolved)", settled, err)
		}
	})
}
