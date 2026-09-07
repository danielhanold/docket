package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// These are the durable gate-record store tests (change 0334, Task 1). Each
// builds a real temporary git repository via git init (reusing the package's
// runGit/gitIdentity/requireRealGit fixture helpers) so the store's git
// common-dir rooting, cross-repo refusal, and linked-worktree resolution are
// exercised against real git, not a mock. The store is the generalization of
// scripts/lib/docket-dispatch-dir.sh's durable-dir conventions.

// newGateRepo initializes a temp git repo with a deterministic identity and one
// seed commit (a commit is required before `git worktree add` can attach a
// linked worktree). It returns the repo's working-tree path.
func newGateRepo(t *testing.T) string {
	t.Helper()
	requireRealGit(t)
	dir := testsupport.TempDir(t)
	runGit(t, dir, "init")
	gitIdentity(t, dir)
	writeRepoFile(t, dir, "seed.txt", "seed\n")
	runGit(t, dir, "add", "seed.txt")
	runGit(t, dir, "commit", "-m", "seed")
	return dir
}

// sampleGateRecord is a fully-populated non-authoritative record (Schema and
// Repo are stamped by the store, so they are left zero here).
func sampleGateRecord() GateRecord {
	return GateRecord{
		Target:        "docket-implement-next",
		CreatedAt:     1700000000,
		DispatchEpoch: 1700000005,
		BeforeIDs:     []int{12, 34, 56},
		AttributedID:  0,
		Retry:         RetryUnused,
		Disposition:   "gate-armed",
		Terminal:      false,
	}
}

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
	key, err := MintGateRecord(repoDir, GateRecord{Target: "docket-implement-next", Retry: RetryUnused, Disposition: "gate-armed"})
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	return key
}

// mintGateWithHash mints a record carrying ChildContextHash, optionally terminal.
func mintGateWithHash(t *testing.T, repoDir, hash string, terminal bool) string {
	t.Helper()
	key, err := MintGateRecord(repoDir, GateRecord{Target: "docket-implement-next", Retry: RetryUnused, ChildContextHash: hash, Terminal: terminal})
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	return key
}

// TestGateSchemaV2RecordFailsClosed: a hand-written schema-2 record (the pre-0407
// shape whose AttributedID may be an inferred guess) must fail closed on load as
// corrupt-record — never a silent migration that blesses an old guessed id.
func TestGateSchemaV2RecordFailsClosed(t *testing.T) {
	repo := newGateRepo(t)
	key, err := MintGateRecord(repo, GateRecord{Target: "docket-implement-next", Retry: RetryUnused})
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
	if err := os.WriteFile(filepath.Join(common, "docket", "rungate", key, gateRecordFileName), buf, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, lerr := LoadGateRecord(repo, key)
	gse, ok := AsGateStoreError(lerr)
	if !ok || gse.Kind != ErrGateCorruptRecord {
		t.Fatalf("want corrupt-record for schema 2, got %v", lerr)
	}
}

// TestReserveGateClaimIsBindOnce: the first reservation wins; a different
// (change, request) under the same key is refused binding-conflict; an
// identical replay is a no-op.
func TestReserveGateClaimIsBindOnce(t *testing.T) {
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

// TestConfirmGateClaimMirrorsRecord: confirm finalizes the binding and mirrors
// AttributedID/BoundRequestID/BoundRevision onto the record; a mismatched
// confirm is binding-conflict; a re-confirm is idempotent.
func TestConfirmGateClaimMirrorsRecord(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	if err := ReserveGateClaim(repo, key, 3, "claim-3-aaa"); err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if err := ConfirmGateClaim(repo, key, 3, "claim-3-aaa", "deadbeef"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := ConfirmGateClaim(repo, key, 3, "claim-3-aaa", "deadbeef"); err != nil {
		t.Fatalf("re-confirm: %v", err)
	}
	if err := ConfirmGateClaim(repo, key, 4, "claim-4-bbb", "cafe"); err == nil {
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

// TestConfirmWithoutReservationFails: a failed or absent reservation can never
// become a confirmed binding (spec: "Failed claims never become confirmed
// bindings").
func TestConfirmWithoutReservationFails(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	if err := ConfirmGateClaim(repo, key, 3, "claim-3-aaa", "deadbeef"); err == nil {
		t.Fatalf("confirm without reservation must fail")
	}
}

// TestLoadGateClaimBindingCorruptFailsClosed: unparseable binding bytes are a
// typed corrupt-record error, never (ok=false, nil).
func TestLoadGateClaimBindingCorruptFailsClosed(t *testing.T) {
	repo := newGateRepo(t)
	key := mintPlainGate(t, repo)
	common, _ := gateGitCommonDir(repo)
	if err := os.WriteFile(filepath.Join(common, "docket", "rungate", key, gateClaimBindingName), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _, err := LoadGateClaimBinding(repo, key)
	gse, ok := AsGateStoreError(err)
	if !ok || gse.Kind != ErrGateCorruptRecord {
		t.Fatalf("want corrupt-record, got %v", err)
	}
}

// TestFindGateRecordByContextHash: exactly-one non-terminal match resolves;
// zero is not-found; two armed gates sharing a hash is context-ambiguous;
// a terminal record does not match.
func TestFindGateRecordByContextHash(t *testing.T) {
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
