package gatedrive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// sampleScopeReq builds a ScopeRequest with a value in every field and a
// non-empty GateContext, so a persisted-record inspection proves the raw
// GateContext is never stored (only its hash). ChangeID is empty so the
// bind-once change path has an unbound field to bind.
func sampleScopeReq() ScopeRequest {
	return ScopeRequest{
		RepoIdentity: "repo-x",
		ChangeID:     "",
		TaskID:       "task-1",
		Phase:        "build",
		Branch:       "feat/x",
		Worktree:     "/wt/x",
		GateContext:  "outer-child-context-token",
	}
}

// isHex32 reports whether s is exactly 32 lowercase hex characters — the shape
// randomToken(16) mints.
func isHex32(s string) bool {
	if len(s) != 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// isOwnershipKind reports whether err is an OwnershipError of the given kind.
func isOwnershipKind(err error, kind OwnershipErrorKind) bool {
	oe, ok := AsOwnershipError(err)
	return ok && oe.Kind == kind
}

// isStoreKind reports whether err is a StoreError of the given kind.
func isStoreKind(err error, kind StoreErrorKind) bool {
	se, ok := AsStoreError(err)
	return ok && se.Kind == kind
}

// TestPrepareScopeMintsSeparatedCapabilities proves PrepareScope returns a
// scope id and two SEPARATE opaque capabilities — non-empty, pairwise distinct,
// 32 lowercase hex — and that the persisted record stores only sha256 hashes of
// the two capabilities and of GateContext, never their raw values.
func TestPrepareScopeMintsSeparatedCapabilities(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	req := sampleScopeReq()
	grant, err := s.PrepareScope(req)
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	for name, v := range map[string]string{
		"ScopeID":          grant.ScopeID,
		"ChildCapability":  grant.ChildCapability,
		"ParentCapability": grant.ParentCapability,
	} {
		if !isHex32(v) {
			t.Fatalf("%s = %q must be 32 lowercase hex chars", name, v)
		}
	}
	if grant.ScopeID == grant.ChildCapability ||
		grant.ScopeID == grant.ParentCapability ||
		grant.ChildCapability == grant.ParentCapability {
		t.Fatalf("scope id and both capabilities must be pairwise distinct: %+v", grant)
	}

	// The persisted record must never contain any raw token — only hashes.
	raw, err := os.ReadFile(filepath.Join(s.scopeRoot, grant.ScopeID, recordFileName))
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	body := string(raw)
	for name, secret := range map[string]string{
		"ChildCapability":  grant.ChildCapability,
		"ParentCapability": grant.ParentCapability,
		"GateContext":      req.GateContext,
	} {
		if strings.Contains(body, secret) {
			t.Fatalf("record leaked raw %s %q", name, secret)
		}
	}
	for name, want := range map[string]string{
		"child cap hash":    capHash(grant.ChildCapability),
		"parent cap hash":   capHash(grant.ParentCapability),
		"gate context hash": capHash(req.GateContext),
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("record must persist the %s %q", name, want)
		}
	}

	// A scope prepared with an empty GateContext persists no gate-context hash.
	noCtx := req
	noCtx.GateContext = ""
	g2, err := s.PrepareScope(noCtx)
	if err != nil {
		t.Fatalf("PrepareScope no-ctx: %v", err)
	}
	rec, err := s.LoadScope(g2.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if rec.GateContextHash != "" {
		t.Fatalf("empty GateContext must persist no hash, got %q", rec.GateContextHash)
	}
	if rec.ChildCapHash != capHash(g2.ChildCapability) || rec.ParentCapHash != capHash(g2.ParentCapability) {
		t.Fatalf("record cap hashes do not match the grant")
	}
}

// drive id fixtures for the slot-lifecycle tests: 32 hex chars, distinct.
const (
	scopeDriveA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	scopeDriveB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	scopeDriveC = "cccccccccccccccccccccccccccccccc"
)

// readScopeBytes returns the raw record.json bytes for a scope, so a rejection
// test can prove the persisted record is untouched with a before/after compare.
func readScopeBytes(t *testing.T, s *Store, id string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(s.scopeRoot, id, recordFileName))
	if err != nil {
		t.Fatalf("read scope bytes: %v", err)
	}
	return b
}

// launchOne drives a scope's slot to a LAUNCHED current drive: reserve with an
// empty receipt (a first start), then confirm the launch.
func launchOne(t *testing.T, s *Store, grant ScopeGrant, driveID string) {
	t.Helper()
	if err := s.reserveScopeDrive(grant.ScopeID, grant.ChildCapability, driveID, predecessorReceipt{}); err != nil {
		t.Fatalf("reserve %s: %v", driveID, err)
	}
	if err := s.confirmScopeLaunch(grant.ScopeID, driveID); err != nil {
		t.Fatalf("confirm %s: %v", driveID, err)
	}
}

// TestScopeV2RoundTripZeroSlot proves a freshly prepared scope round-trips
// through LoadScope as a v2 zero-slot record: no current drive, no reservation,
// and a zero drive count.
func TestScopeV2RoundTripZeroSlot(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	grant, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	rec, err := s.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if rec.SchemaVersion != scopeSchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", rec.SchemaVersion, scopeSchemaVersion)
	}
	if rec.CurrentDriveID != "" || rec.CurrentDriveState != "" || rec.PriorDriveID != "" {
		t.Fatalf("zero-slot scope must have empty slot, got current=%q state=%q prior=%q",
			rec.CurrentDriveID, rec.CurrentDriveState, rec.PriorDriveID)
	}
	if rec.DriveCount != 0 {
		t.Fatalf("DriveCount = %d, want 0", rec.DriveCount)
	}
	if rec.PendingAckDriveID != "" || rec.PendingAckOwnerGen != "" {
		t.Fatalf("zero-slot scope must have no pending ack, got %q/%q", rec.PendingAckDriveID, rec.PendingAckOwnerGen)
	}
}

// TestScopeSchemaV1FailClosed proves a persisted v1 scope record (the old
// bound_drive_id shape) fails closed with ErrUnknownSchema — the store never
// silently reinterprets an in-flight one-drive record as a reusable v2 scope.
func TestScopeSchemaV1FailClosed(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	g, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	// Hand-write a v1 envelope: schema_version 1, with the retired bound_drive_id
	// field and no v2 slot fields.
	v1 := `{"generation":"x","record":{"schema_version":1,"repo_identity":"repo-x","child_cap_hash":"` +
		capHash(g.ChildCapability) + `","parent_cap_hash":"` + capHash(g.ParentCapability) +
		`","bound_drive_id":"` + scopeDriveA + `","closed":false}}`
	if err := os.WriteFile(filepath.Join(s.scopeRoot, g.ScopeID, recordFileName), []byte(v1), 0o600); err != nil {
		t.Fatalf("write v1: %v", err)
	}
	if _, err := s.LoadScope(g.ScopeID); !isStoreKind(err, ErrUnknownSchema) {
		t.Fatalf("a v1 scope record must fail closed ErrUnknownSchema, got %v", err)
	}
}

// TestScopeReserveEmptyScope proves reserveScopeDrive on an empty slot succeeds
// only with an empty receipt (a first start) and fills the slot as reserved with
// drive count 1; a non-empty receipt on an empty scope is ErrStalePredecessor.
func TestScopeReserveEmptyScope(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	grant, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	// A non-empty receipt on an empty scope is refused with no write.
	before := readScopeBytes(t, s, grant.ScopeID)
	if err := s.reserveScopeDrive(grant.ScopeID, grant.ChildCapability, scopeDriveA, predecessorReceipt{DriveID: scopeDriveB, OwnerGen: "gen-b"}); !isOwnershipKind(err, ErrStalePredecessor) {
		t.Fatalf("a receipt on an empty scope must fail ErrStalePredecessor, got %v", err)
	}
	if after := readScopeBytes(t, s, grant.ScopeID); string(after) != string(before) {
		t.Fatalf("a rejected reserve must not write: bytes changed")
	}

	// An empty receipt succeeds and fills the slot as reserved.
	if err := s.reserveScopeDrive(grant.ScopeID, grant.ChildCapability, scopeDriveA, predecessorReceipt{}); err != nil {
		t.Fatalf("first reserve: %v", err)
	}
	rec, err := s.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if rec.CurrentDriveID != scopeDriveA || rec.CurrentDriveState != scopeStateReserved {
		t.Fatalf("after reserve want current=%q reserved, got current=%q state=%q", scopeDriveA, rec.CurrentDriveID, rec.CurrentDriveState)
	}
	if rec.DriveCount != 1 {
		t.Fatalf("DriveCount = %d, want 1", rec.DriveCount)
	}
	if rec.PendingAckDriveID != "" || rec.PendingAckOwnerGen != "" || rec.PriorDriveID != "" {
		t.Fatalf("first reserve must leave no pending ack or prior, got pending=%q/%q prior=%q", rec.PendingAckDriveID, rec.PendingAckOwnerGen, rec.PriorDriveID)
	}
}

// TestScopeReserveOccupiedScope proves the successor handshake against an
// occupied slot: an empty receipt is a second-drive error; a receipt naming a
// non-current drive is stale; a reserved (unconfirmed) slot is busy; a journaled
// pending ack is an unresolved transition; and the correct receipt against a
// LAUNCHED current drive reserves the successor and journals the ack.
func TestScopeReserveOccupiedScope(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	grant, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	launchOne(t, s, grant, scopeDriveA) // current = A, launched

	// Empty receipt against an occupied scope: a second live drive is refused.
	if err := s.reserveScopeDrive(grant.ScopeID, grant.ChildCapability, scopeDriveB, predecessorReceipt{}); !isOwnershipKind(err, ErrScopeSecondDrive) {
		t.Fatalf("empty receipt on occupied scope must fail ErrScopeSecondDrive, got %v", err)
	}
	// Receipt naming a non-current drive: stale.
	if err := s.reserveScopeDrive(grant.ScopeID, grant.ChildCapability, scopeDriveB, predecessorReceipt{DriveID: scopeDriveC, OwnerGen: "gen-c"}); !isOwnershipKind(err, ErrStalePredecessor) {
		t.Fatalf("receipt naming a non-current drive must fail ErrStalePredecessor, got %v", err)
	}

	// Correct receipt against a LAUNCHED current drive reserves the successor.
	if err := s.reserveScopeDrive(grant.ScopeID, grant.ChildCapability, scopeDriveB, predecessorReceipt{DriveID: scopeDriveA, OwnerGen: "gen-a"}); err != nil {
		t.Fatalf("successor reserve: %v", err)
	}
	rec, err := s.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if rec.CurrentDriveID != scopeDriveB || rec.CurrentDriveState != scopeStateReserved {
		t.Fatalf("successor reserve want current=%q reserved, got current=%q state=%q", scopeDriveB, rec.CurrentDriveID, rec.CurrentDriveState)
	}
	if rec.PriorDriveID != scopeDriveA {
		t.Fatalf("PriorDriveID = %q, want %q", rec.PriorDriveID, scopeDriveA)
	}
	if rec.PendingAckDriveID != scopeDriveA || rec.PendingAckOwnerGen != "gen-a" {
		t.Fatalf("pending ack = %q/%q, want %q/gen-a", rec.PendingAckDriveID, rec.PendingAckOwnerGen, scopeDriveA)
	}
	if rec.DriveCount != 2 {
		t.Fatalf("DriveCount = %d, want 2", rec.DriveCount)
	}

	// While a pending ack is journaled (state now launched via confirm), a further
	// reserve is an unresolved launch transition.
	if err := s.confirmScopeLaunch(grant.ScopeID, scopeDriveB); err != nil {
		t.Fatalf("confirm B: %v", err)
	}
	if err := s.reserveScopeDrive(grant.ScopeID, grant.ChildCapability, scopeDriveC, predecessorReceipt{DriveID: scopeDriveB, OwnerGen: "gen-b"}); !isOwnershipKind(err, ErrUnresolvedLaunchTransition) {
		t.Fatalf("reserve while a pending ack is journaled must fail ErrUnresolvedLaunchTransition, got %v", err)
	}

	// A reserved (unconfirmed) slot is busy. Fresh scope: reserve without confirm.
	busy, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope busy: %v", err)
	}
	if err := s.reserveScopeDrive(busy.ScopeID, busy.ChildCapability, scopeDriveA, predecessorReceipt{}); err != nil {
		t.Fatalf("reserve A (unconfirmed): %v", err)
	}
	if err := s.reserveScopeDrive(busy.ScopeID, busy.ChildCapability, scopeDriveB, predecessorReceipt{DriveID: scopeDriveA, OwnerGen: "gen-a"}); !isOwnershipKind(err, ErrScopeBusy) {
		t.Fatalf("reserve while the slot is reserved must fail ErrScopeBusy, got %v", err)
	}
}

// TestScopeReserveReceiptShape proves a half-filled receipt (one field set, the
// other empty) is rejected ErrStalePredecessor with no write, even when the set
// field names the scope's current drive.
func TestScopeReserveReceiptShape(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	grant, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	launchOne(t, s, grant, scopeDriveA) // current = A, launched

	for _, r := range []predecessorReceipt{
		{DriveID: scopeDriveA, OwnerGen: ""}, // id without generation
		{DriveID: "", OwnerGen: "gen-a"},     // generation without id
	} {
		before := readScopeBytes(t, s, grant.ScopeID)
		if err := s.reserveScopeDrive(grant.ScopeID, grant.ChildCapability, scopeDriveB, r); !isOwnershipKind(err, ErrStalePredecessor) {
			t.Fatalf("half-filled receipt %+v must fail ErrStalePredecessor, got %v", r, err)
		}
		if after := readScopeBytes(t, s, grant.ScopeID); string(after) != string(before) {
			t.Fatalf("a rejected half-filled receipt %+v must not write: bytes changed", r)
		}
	}
}

// TestScopeReserveCapabilityAndClosed proves reserveScopeDrive refuses a wrong or
// empty capability (ErrScopeCapabilityMismatch) and a closed scope
// (ErrScopeClosed), and that each rejection leaves the persisted bytes unchanged.
func TestScopeReserveCapabilityAndClosed(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	grant, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	for _, cap := range []string{"wrong-capability", ""} {
		before := readScopeBytes(t, s, grant.ScopeID)
		if err := s.reserveScopeDrive(grant.ScopeID, cap, scopeDriveA, predecessorReceipt{}); !isOwnershipKind(err, ErrScopeCapabilityMismatch) {
			t.Fatalf("capability %q must fail ErrScopeCapabilityMismatch, got %v", cap, err)
		}
		if after := readScopeBytes(t, s, grant.ScopeID); string(after) != string(before) {
			t.Fatalf("a rejected capability %q must not write: bytes changed", cap)
		}
	}

	closed, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope closed: %v", err)
	}
	if err := s.closeScope(closed.ScopeID); err != nil {
		t.Fatalf("closeScope: %v", err)
	}
	before := readScopeBytes(t, s, closed.ScopeID)
	if err := s.reserveScopeDrive(closed.ScopeID, closed.ChildCapability, scopeDriveA, predecessorReceipt{}); !isOwnershipKind(err, ErrScopeClosed) {
		t.Fatalf("reserve on a closed scope must fail ErrScopeClosed, got %v", err)
	}
	if after := readScopeBytes(t, s, closed.ScopeID); string(after) != string(before) {
		t.Fatalf("a rejected reserve on a closed scope must not write: bytes changed")
	}
}

// TestScopeConfirmLaunch proves confirmScopeLaunch flips reserved→launched only
// for the matching current drive id; a mismatched id is an unresolved launch
// transition; an already-launched matching id is an idempotent no-op (nil).
func TestScopeConfirmLaunch(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	grant, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	if err := s.reserveScopeDrive(grant.ScopeID, grant.ChildCapability, scopeDriveA, predecessorReceipt{}); err != nil {
		t.Fatalf("reserve A: %v", err)
	}
	// A mismatched id cannot confirm the launch.
	if err := s.confirmScopeLaunch(grant.ScopeID, scopeDriveB); !isOwnershipKind(err, ErrUnresolvedLaunchTransition) {
		t.Fatalf("confirm of a non-current id must fail ErrUnresolvedLaunchTransition, got %v", err)
	}
	// The matching id flips reserved→launched.
	if err := s.confirmScopeLaunch(grant.ScopeID, scopeDriveA); err != nil {
		t.Fatalf("confirm A: %v", err)
	}
	rec, err := s.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if rec.CurrentDriveState != scopeStateLaunched {
		t.Fatalf("CurrentDriveState = %q, want launched", rec.CurrentDriveState)
	}
	// An already-launched matching id is an idempotent no-op.
	if err := s.confirmScopeLaunch(grant.ScopeID, scopeDriveA); err != nil {
		t.Fatalf("idempotent confirm A: %v", err)
	}
}

// TestScopeClearPendingAck proves clearPendingAck clears only a matching journal
// entry; a mismatched predecessor id is an unresolved launch transition.
func TestScopeClearPendingAck(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	grant, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	launchOne(t, s, grant, scopeDriveA) // current = A, launched
	if err := s.reserveScopeDrive(grant.ScopeID, grant.ChildCapability, scopeDriveB, predecessorReceipt{DriveID: scopeDriveA, OwnerGen: "gen-a"}); err != nil {
		t.Fatalf("successor reserve: %v", err) // journals pending ack A
	}
	// A mismatched predecessor id does not clear the journal.
	if err := s.clearPendingAck(grant.ScopeID, scopeDriveC); !isOwnershipKind(err, ErrUnresolvedLaunchTransition) {
		t.Fatalf("clear of a non-journaled id must fail ErrUnresolvedLaunchTransition, got %v", err)
	}
	// The matching predecessor id clears it.
	if err := s.clearPendingAck(grant.ScopeID, scopeDriveA); err != nil {
		t.Fatalf("clear A: %v", err)
	}
	rec, err := s.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if rec.PendingAckDriveID != "" || rec.PendingAckOwnerGen != "" {
		t.Fatalf("pending ack must be cleared, got %q/%q", rec.PendingAckDriveID, rec.PendingAckOwnerGen)
	}
}

// TestScopeReserveSerialization proves that two goroutines presenting the SAME
// valid successor receipt against a launched slot have exactly one nil-error
// winner; the loser gets a typed ErrScopeBusy or ErrStalePredecessor, and the
// persisted record names exactly one winner. Run under -race.
func TestScopeReserveSerialization(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	grant, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	launchOne(t, s, grant, scopeDriveA) // current = A, launched

	const n = 8
	var wg sync.WaitGroup
	results := make([]error, n)
	newIDs := make([]string, n)
	for i := 0; i < n; i++ {
		newIDs[i] = fmt.Sprintf("%032d", i)
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = s.reserveScopeDrive(grant.ScopeID, grant.ChildCapability, newIDs[idx], predecessorReceipt{DriveID: scopeDriveA, OwnerGen: "gen-a"})
		}(i)
	}
	wg.Wait()

	winners := 0
	winIdx := -1
	for i, e := range results {
		if e == nil {
			winners++
			winIdx = i
			continue
		}
		if !isOwnershipKind(e, ErrScopeBusy) && !isOwnershipKind(e, ErrStalePredecessor) {
			t.Fatalf("loser must fail ErrScopeBusy or ErrStalePredecessor, got %v", e)
		}
	}
	if winners != 1 {
		t.Fatalf("exactly one reserve must win, got %d", winners)
	}
	rec, err := s.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if rec.CurrentDriveID != newIDs[winIdx] {
		t.Fatalf("persisted record must name the sole winner %q, got %q", newIDs[winIdx], rec.CurrentDriveID)
	}
}

// TestScopeIdentityFailClosed proves LoadScope fails closed on an unknown schema
// version and on a corrupt record (reusing the StoreError kinds), and that a
// traversal-shaped scope id is rejected before any path is built.
func TestScopeIdentityFailClosed(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))

	// Unknown schema version.
	unk, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	bad := scopeRecord{SchemaVersion: scopeSchemaVersion + 999}
	buf, err := json.Marshal(storedScope{Generation: "x", Record: bad})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.scopeRoot, unk.ScopeID, recordFileName), buf, 0o600); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if _, err := s.LoadScope(unk.ScopeID); !isStoreKind(err, ErrUnknownSchema) {
		t.Fatalf("unknown schema must return ErrUnknownSchema, got %v", err)
	}

	// Corrupt record.
	corrupt, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.scopeRoot, corrupt.ScopeID, recordFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	if _, err := s.LoadScope(corrupt.ScopeID); !isStoreKind(err, ErrCorruptRecord) {
		t.Fatalf("corrupt record must return ErrCorruptRecord, got %v", err)
	}

	// A traversal-shaped id is rejected before any path is built.
	for _, id := range []string{"../../etc/passwd", "a/b", "", "foo.json"} {
		if _, err := s.LoadScope(id); !isStoreKind(err, ErrInvalidID) {
			t.Fatalf("LoadScope(%q) must reject with ErrInvalidID, got %v", id, err)
		}
	}
}

// TestBindScopeChangeOnce proves bindScopeChange sets an empty ChangeID exactly
// once: rebinding the same id is a no-op, a different id fails closed, and a
// closed scope refuses the bind.
func TestBindScopeChangeOnce(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	grant, err := s.PrepareScope(sampleScopeReq()) // ChangeID is empty
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	if err := s.bindScopeChange(grant.ScopeID, "0359"); err != nil {
		t.Fatalf("first bindScopeChange: %v", err)
	}
	rec, err := s.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if rec.ChangeID != "0359" {
		t.Fatalf("ChangeID = %q, want 0359", rec.ChangeID)
	}
	// Rebinding the same id is a no-op.
	if err := s.bindScopeChange(grant.ScopeID, "0359"); err != nil {
		t.Fatalf("idempotent rebind: %v", err)
	}
	// A different id fails closed.
	if err := s.bindScopeChange(grant.ScopeID, "0400"); !isOwnershipKind(err, ErrScopeIdentityMismatch) {
		t.Fatalf("rebinding a different change must fail ErrScopeIdentityMismatch, got %v", err)
	}

	// A closed scope refuses the bind.
	closed, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope closed: %v", err)
	}
	if err := s.closeScope(closed.ScopeID); err != nil {
		t.Fatalf("closeScope: %v", err)
	}
	if err := s.bindScopeChange(closed.ScopeID, "0359"); !isOwnershipKind(err, ErrScopeClosed) {
		t.Fatalf("bindScopeChange on a closed scope must fail ErrScopeClosed, got %v", err)
	}
}

// TestScopeCASConcurrent proves the per-scope CAS grants exactly one winner when
// many goroutines race to reserve the slot of an EMPTY scope (racing first
// starts): the winner reserves, every loser gets the typed ErrScopeBusy
// rejection (the winner's reservation is in flight), never a physical-contention
// leak. Run under -race.
func TestScopeCASConcurrent(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	grant, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	const n = 8
	var wg sync.WaitGroup
	results := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			driveID := fmt.Sprintf("%032d", idx)
			results[idx] = s.reserveScopeDrive(grant.ScopeID, grant.ChildCapability, driveID, predecessorReceipt{})
		}(i)
	}
	wg.Wait()

	winners := 0
	for _, e := range results {
		if e == nil {
			winners++
			continue
		}
		if !isOwnershipKind(e, ErrScopeBusy) {
			t.Fatalf("loser must fail ErrScopeBusy, got %v", e)
		}
	}
	if winners != 1 {
		t.Fatalf("exactly one reserve must win, got %d", winners)
	}
}
