//go:build integration

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// These are the durable gate-record store tests (change 0334, Task 1). Each
// builds a real temporary git repository via git init (reusing the package's
// runGit/gitIdentity/requireRealGit fixture helpers) so the store's git
// common-dir rooting, cross-repo refusal, and linked-worktree resolution are
// exercised against real git, not a mock. The store is the generalization of
// scripts/lib/docket-dispatch-dir.sh's durable-dir conventions.
// The fixtures untagged test files share (newGateRepo, sampleGateRecord,
// mintGateWithHash) live in rungate_store_helpers_test.go (change 0465).

// repeat returns s repeated n times (a tiny local helper so the malformed-key
// case can build an over-long key without importing strings just for this).
func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

// --- claim-binding primitives and schema v3 (change 0407) ---

// mintPlainGate mints a minimal armed record for store-primitive tests.
func mintPlainGate(t *testing.T, repoDir string) string {
	t.Helper()
	key, err := MintGateRecord(repoDir, GateRecord{Target: "docket-implement-next", Retry: RetryUnused, Disposition: "gate-armed", AttemptLimit: 2})
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	return key
}

// TestIntegrationGateEpochGateSchemaV2RecordFailsClosed: a hand-written schema-2 record (the pre-0407
// shape whose AttributedID may be an inferred guess) must fail closed on load as
// corrupt-record — never a silent migration that blesses an old guessed id.
func TestIntegrationGateEpochGateSchemaV2RecordFailsClosed(t *testing.T) {
	repo := newGateRepo(t)
	key, err := MintGateRecord(repo, GateRecord{Target: "docket-implement-next", Retry: RetryUnused, AttemptLimit: 2})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	rec, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	rec.Schema = 2 // bypass SaveGateRecord's authoritative stamp: write the file directly
	common, _ := gateGitCommonDir(repo)
	buf, _ := json.Marshal(rec)
	if err := os.WriteFile(filepath.Join(common, "docket", runTrackerDirName, key, gateRecordFileName), buf, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, lerr := LoadGateRecord(repo, key)
	gse, ok := AsGateStoreError(lerr)
	if !ok || gse.Kind != ErrGateCorruptRecord {
		t.Fatalf("want corrupt-record for schema 2, got %v", lerr)
	}
}

// TestIntegrationGateEpochReserveGateClaimIsBindOnce: the first reservation wins; a different
// (change, request) under the same key is refused binding-conflict; an
// identical replay is a no-op.
func TestIntegrationGateEpochReserveGateClaimIsBindOnce(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	if err := ReserveGateClaim(repo, key, 3, "claim-3-aaa"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := ReserveGateClaim(repo, key, 3, "claim-3-aaa"); err != nil {
		t.Fatalf("replay reserve: %v", err)
	}
	err := ReserveGateClaim(repo, key, 4, "claim-4-bbb")
	gse, ok := AsGateStoreError(err)
	if !ok || gse.Kind != ErrGateBindingConflict {
		t.Fatalf("want binding-conflict, got %v", err)
	}
}

// TestIntegrationGateEpochConfirmGateClaimMirrorsRecord: confirm finalizes the binding and mirrors
// AttributedID/BoundRequestID/BoundRevision onto the record; a mismatched
// confirm is binding-conflict; a re-confirm is idempotent.
func TestIntegrationGateEpochConfirmGateClaimMirrorsRecord(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	if err := ReserveGateClaim(repo, key, 3, "claim-3-aaa"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := ConfirmGateClaim(repo, key, 3, "claim-3-aaa", "deadbeef", ""); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := ConfirmGateClaim(repo, key, 3, "claim-3-aaa", "deadbeef", ""); err != nil {
		t.Fatalf("re-confirm: %v", err)
	}
	if err := ConfirmGateClaim(repo, key, 4, "claim-4-bbb", "cafe", ""); err == nil {
		t.Fatalf("mismatched confirm must fail")
	}
	b, ok, err := LoadGateClaimBinding(repo, key)
	if err != nil || !ok || !b.Confirmed || b.ChangeID != 3 || b.Revision != "deadbeef" {
		t.Fatalf("binding = %+v ok=%v err=%v", b, ok, err)
	}
	rec, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rec.AttributedID != 3 || rec.BoundRequestID != "claim-3-aaa" || rec.BoundRevision != "deadbeef" {
		t.Fatalf("record mirror = %+v", rec)
	}
}

// TestIntegrationGateEpochConfirmWithoutReservationFails: a failed or absent reservation can never
// become a confirmed binding (spec: "Failed claims never become confirmed
// bindings").
func TestIntegrationGateEpochConfirmWithoutReservationFails(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	if err := ConfirmGateClaim(repo, key, 3, "claim-3-aaa", "deadbeef", ""); err == nil {
		t.Fatalf("confirm without reservation must fail")
	}
}

// TestIntegrationGateEpochLoadGateClaimBindingCorruptFailsClosed: unparseable binding bytes are a
// typed corrupt-record error, never (ok=false, nil).
func TestIntegrationGateEpochLoadGateClaimBindingCorruptFailsClosed(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	common, _ := gateGitCommonDir(repo)
	if err := os.WriteFile(filepath.Join(common, "docket", runTrackerDirName, key, gateClaimBindingName), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _, err := LoadGateClaimBinding(repo, key)
	gse, ok := AsGateStoreError(err)
	if !ok || gse.Kind != ErrGateCorruptRecord {
		t.Fatalf("want corrupt-record, got %v", err)
	}
}

// TestIntegrationGateEpochFindGateRecordByContextHash: exactly-one non-terminal match resolves;
// zero is not-found; two armed gates sharing a hash is context-ambiguous;
// a terminal record does not match.
func TestIntegrationGateEpochFindGateRecordByContextHash(t *testing.T) {
	repo := newGateRepo(t)
	keyA := mintGateWithHash(t, repo, "ha", false)
	_ = mintGateWithHash(t, repo, "hb", false)
	_ = mintGateWithHash(t, repo, "ht", true) // terminal
	k, rec, err := FindGateRecordByContextHash(repo, "ha")
	if err != nil || k != keyA || rec.ChildContextHash != "ha" {
		t.Fatalf("k=%q rec=%+v err=%v", k, rec, err)
	}
	if _, _, err := FindGateRecordByContextHash(repo, "ht"); err == nil {
		t.Fatalf("terminal record must not match")
	}
	if _, _, err := FindGateRecordByContextHash(repo, "nope"); err == nil {
		t.Fatalf("zero matches must error")
	}
	_ = mintGateWithHash(t, repo, "hb", false)
	_, _, err = FindGateRecordByContextHash(repo, "hb")
	gse, ok := AsGateStoreError(err)
	if !ok || gse.Kind != ErrGateContextAmbiguous {
		t.Fatalf("want context-ambiguous, got %v", err)
	}
}

// --- counted per-attempt retry budget and schema v4 (change 0421) ---

// TestIntegrationGateEpochConsumeGateRetryPerAttemptCAS: with limit 3, distinct attempts each grant
// their own marker exactly once, a repeat of a spent attempt refuses, and an
// attempt at or above the limit refuses WITHOUT creating a marker.
func TestIntegrationGateEpochConsumeGateRetryPerAttemptCAS(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)

	// attempt 1 grants; a second call for attempt 1 refuses (marker exists).
	if ok, err := ConsumeGateRetry(repo, key, 1, 3); err != nil || !ok {
		t.Fatalf("ConsumeGateRetry(1,3) = %v,%v; want true,nil", ok, err)
	}
	if ok, err := ConsumeGateRetry(repo, key, 1, 3); err != nil || ok {
		t.Fatalf("re-consume attempt 1 = %v,%v; want false,nil (marker exists)", ok, err)
	}
	// attempt 2 grants a distinct marker.
	if ok, err := ConsumeGateRetry(repo, key, 2, 3); err != nil || !ok {
		t.Fatalf("ConsumeGateRetry(2,3) = %v,%v; want true,nil", ok, err)
	}
	// attempt 3 == limit: refuse, and create NO marker.
	if ok, err := ConsumeGateRetry(repo, key, 3, 3); err != nil || ok {
		t.Fatalf("ConsumeGateRetry(3,3) = %v,%v; want false,nil (attempt >= limit)", ok, err)
	}
	common, _ := gateGitCommonDir(repo)
	dir := filepath.Join(common, "docket", runTrackerDirName, key)
	if _, serr := os.Stat(filepath.Join(dir, gateRetryMarkerFor(3))); !os.IsNotExist(serr) {
		t.Fatalf("marker for the refused over-limit attempt must not exist (stat err=%v)", serr)
	}
	if used, err := GateRetryUsage(repo, key); err != nil || used != 2 {
		t.Fatalf("GateRetryUsage = %d,%v; want 2,nil", used, err)
	}
}

// TestIntegrationGateEpochConsumeGateRetryLimitOne: a limit of 1 disables retries — attempt 1 is
// already at the limit, so nothing is granted and no marker is created.
func TestIntegrationGateEpochConsumeGateRetryLimitOne(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	if ok, err := ConsumeGateRetry(repo, key, 1, 1); err != nil || ok {
		t.Fatalf("ConsumeGateRetry(1,1) = %v,%v; want false,nil (limit 1 disables retries)", ok, err)
	}
	if used, err := GateRetryUsage(repo, key); err != nil || used != 0 {
		t.Fatalf("GateRetryUsage = %d,%v; want 0,nil (no marker created)", used, err)
	}
}

// TestIntegrationGateEpochGateRetryUsageCountsLegacyMarker: a bare legacy `retry-consumed` marker
// (schema v3's single-permit name) counts as one consumed marker and is read as
// the attempt-1 marker, so an already-consumed legacy permit can never be
// re-granted — an older consumed marker must never read as unused budget.
func TestIntegrationGateEpochGateRetryUsageCountsLegacyMarker(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	common, _ := gateGitCommonDir(repo)
	dir := filepath.Join(common, "docket", runTrackerDirName, key)
	if err := os.WriteFile(filepath.Join(dir, gateRetryMarkerName), nil, 0o644); err != nil {
		t.Fatalf("plant legacy marker: %v", err)
	}
	if used, err := GateRetryUsage(repo, key); err != nil || used != 1 {
		t.Fatalf("GateRetryUsage = %d,%v; want 1,nil (legacy marker counts)", used, err)
	}
	if ok, err := ConsumeGateRetry(repo, key, 1, 2); err != nil || ok {
		t.Fatalf("ConsumeGateRetry(1,2) over a legacy marker = %v,%v; want false,nil (attempt 1 already spent)", ok, err)
	}
}

// TestIntegrationGateEpochLoadGateRecordRefusesV3: a v3-shaped record fails closed on load with the
// schema-mismatch diagnostic — the v4 store never silently migrates an older
// record whose consumed state could be reinterpreted as unused budget.
func TestIntegrationGateEpochLoadGateRecordRefusesV3(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	rec, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	rec.Schema = 3 // write a v3-shaped record directly, bypassing the authoritative stamp
	common, _ := gateGitCommonDir(repo)
	buf, _ := json.Marshal(rec)
	if err := os.WriteFile(filepath.Join(common, "docket", runTrackerDirName, key, gateRecordFileName), buf, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, lerr := LoadGateRecord(repo, key)
	gse, ok := AsGateStoreError(lerr)
	if !ok || gse.Kind != ErrGateCorruptRecord {
		t.Fatalf("want corrupt-record for schema 3, got %v", lerr)
	}
}

// TestIntegrationGateEpochSaveGateRecordRefusesUnstampedLimit: a v4 record whose AttemptLimit is
// below the floor is a corrupt/unstamped record and must fail closed on the write
// boundary, exactly like a partial continuation triple or claim-binding pair.
func TestIntegrationGateEpochSaveGateRecordRefusesUnstampedLimit(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	rec, err := LoadGateRecord(repo, key)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	rec.AttemptLimit = 0 // corrupt / unstamped
	serr := SaveGateRecord(repo, key, rec)
	gse, ok := AsGateStoreError(serr)
	if !ok || gse.Kind != ErrGateCorruptRecord {
		t.Fatalf("SaveGateRecord with AttemptLimit 0 = %v, want corrupt-record", serr)
	}
}
