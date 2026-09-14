package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
	if err := ConfirmGateClaim(repo, res.Key, 42, "req-1", "revabc123"); err != nil {
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
	if err := ConfirmGateClaim(repo, key, 7, "req-x", "rev-x"); err != nil {
		t.Fatalf("ConfirmGateClaim over a no-epoch dispatch must succeed: %v", err)
	}
	if _, _, err := LoadEpochRecord(repo, key); !isEpochKind(err, ErrEpochNotFound) {
		t.Fatalf("no epoch must be fabricated by the claim, got %v", err)
	}
}
