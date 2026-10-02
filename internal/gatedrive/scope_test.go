package gatedrive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// sampleScopeReq builds the outer ScopeRequest run.start prepares, with a value
// in every field but ChangeID, which is empty so the bind-once change path has an
// unbound field to bind.
func sampleScopeReq() ScopeRequest {
	return ScopeRequest{
		RepoIdentity: "repo-x",
		ChangeID:     "",
		Branch:       "feat/x",
		Worktree:     "/wt/x",
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
// the two capabilities, never their raw values, beside the request's identity.
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
	} {
		if strings.Contains(body, secret) {
			t.Fatalf("record leaked raw %s %q", name, secret)
		}
	}
	rec, err := s.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if rec.ChildCapHash != capHash(grant.ChildCapability) || rec.ParentCapHash != capHash(grant.ParentCapability) {
		t.Fatalf("record cap hashes do not match the grant")
	}
	want := scopeRecord{
		SchemaVersion: scopeSchemaVersion, RepoIdentity: req.RepoIdentity, ChangeID: req.ChangeID,
		Branch: req.Branch, Worktree: req.Worktree,
		ChildCapHash: rec.ChildCapHash, ParentCapHash: rec.ParentCapHash,
	}
	if rec != want {
		t.Fatalf("prepared scope = %+v, want %+v", rec, want)
	}
}

// TestScopeSchemaPriorVersionsFailClosed proves the reader accepts schema 3 only:
// a v1 record (the retired bound_drive_id shape) and a v2 record (the pre-0375
// slot shape, no longer tolerated since change 0489) both fail closed with
// ErrUnknownSchema — never silently reinterpreted.
func TestScopeSchemaPriorVersionsFailClosed(t *testing.T) {
	s := OpenStore(testsupport.TempDir(t))
	for _, tc := range []struct {
		name   string
		record map[string]any
	}{
		{"v1-bound-drive", map[string]any{
			"schema_version": 1, "repo_identity": "repo-x", "child_cap_hash": capHash("c"), "parent_cap_hash": capHash("p"),
			"bound_drive_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "closed": false,
		}},
		{"v2-slot", map[string]any{
			"schema_version": 2, "repo_identity": "repo-x", "child_cap_hash": capHash("c"), "parent_cap_hash": capHash("p"),
			"current_drive_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "current_drive_state": "launched", "drive_count": 1, "closed": false,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, err := s.PrepareScope(sampleScopeReq())
			if err != nil {
				t.Fatalf("PrepareScope: %v", err)
			}
			buf, err := json.Marshal(map[string]any{"generation": "x", "record": tc.record})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if err := os.WriteFile(filepath.Join(s.scopeRoot, g.ScopeID, recordFileName), buf, 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			if _, err := s.LoadScope(g.ScopeID); !isStoreKind(err, ErrUnknownSchema) {
				t.Fatalf("a %s scope record must fail closed ErrUnknownSchema, got %v", tc.name, err)
			}
		})
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
	if err := s.claimScopeForTakeover(closed.ScopeID); err != nil {
		t.Fatalf("claimScopeForTakeover: %v", err)
	}
	if err := s.bindScopeChange(closed.ScopeID, "0359"); !isOwnershipKind(err, ErrScopeClosed) {
		t.Fatalf("bindScopeChange on a closed scope must fail ErrScopeClosed, got %v", err)
	}
}

// TestScopeCASKeptTransitionsSerialize (change 0489) replaces TestScopeCASConcurrent:
// the scope store's conflict-checked write is exercised through the two transitions
// that survive — run.verdict's change bind racing Takeover's single-use close. No
// write is lost: the close always lands, and a bind that reported success is
// always visible. Run under -race.
func TestScopeCASKeptTransitionsSerialize(t *testing.T) {
	for i := 0; i < 50; i++ {
		store := OpenStore(testsupport.TempDir(t))
		grant, err := store.PrepareScope(ScopeRequest{RepoIdentity: "/repo", Branch: "feat/x", Worktree: "/repo"})
		if err != nil {
			t.Fatalf("PrepareScope: %v", err)
		}
		const binders = 8
		var wg sync.WaitGroup
		bindErrs := make([]error, binders)
		var closeErr error
		wg.Add(binders + 1)
		for b := 0; b < binders; b++ {
			go func(b int) { defer wg.Done(); bindErrs[b] = store.bindScopeChange(grant.ScopeID, "0342") }(b)
		}
		go func() { defer wg.Done(); closeErr = store.claimScopeForTakeover(grant.ScopeID) }()
		wg.Wait()
		if closeErr != nil {
			t.Fatalf("iteration %d: the single takeover close must land: %v", i, closeErr)
		}
		got, err := store.LoadScope(grant.ScopeID)
		if err != nil {
			t.Fatalf("LoadScope: %v", err)
		}
		if !got.Closed {
			t.Fatalf("iteration %d: a bind overwrote the takeover close (lost write)", i)
		}
		for b, berr := range bindErrs {
			switch {
			case berr == nil && got.ChangeID != "0342":
				t.Fatalf("iteration %d: bind %d reported success but its write was lost", i, b)
			case berr != nil && !isOwnershipKind(berr, ErrScopeClosed):
				t.Fatalf("iteration %d: bind %d failed with %v, want nil or scope-closed", i, b, berr)
			}
		}
	}
}
