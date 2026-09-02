package gatedrive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
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
	s := OpenStore(t.TempDir())
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

// TestScopeBindOnce proves bindScopeDrive binds exactly one live drive per
// scope: the exact child capability succeeds once and is idempotent for the same
// drive id; a different drive id, a wrong or empty capability, and a closed
// scope each fail with a distinct typed error.
func TestScopeBindOnce(t *testing.T) {
	s := OpenStore(t.TempDir())
	grant, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	const d1 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	if err := s.bindScopeDrive(grant.ScopeID, grant.ChildCapability, d1); err != nil {
		t.Fatalf("first bind: %v", err)
	}
	rec, err := s.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if rec.BoundDriveID != d1 {
		t.Fatalf("BoundDriveID = %q, want %q", rec.BoundDriveID, d1)
	}
	// Idempotent re-bind of the same drive id is a no-op.
	if err := s.bindScopeDrive(grant.ScopeID, grant.ChildCapability, d1); err != nil {
		t.Fatalf("idempotent re-bind: %v", err)
	}
	// A second, different drive id is refused.
	if err := s.bindScopeDrive(grant.ScopeID, grant.ChildCapability, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"); !isOwnershipKind(err, ErrScopeSecondDrive) {
		t.Fatalf("second live drive must fail ErrScopeSecondDrive, got %v", err)
	}
	// A wrong capability is refused.
	if err := s.bindScopeDrive(grant.ScopeID, "wrong-capability", d1); !isOwnershipKind(err, ErrScopeCapabilityMismatch) {
		t.Fatalf("wrong capability must fail ErrScopeCapabilityMismatch, got %v", err)
	}
	// An empty capability is refused.
	if err := s.bindScopeDrive(grant.ScopeID, "", d1); !isOwnershipKind(err, ErrScopeCapabilityMismatch) {
		t.Fatalf("empty capability must fail ErrScopeCapabilityMismatch, got %v", err)
	}

	// A closed scope refuses any bind.
	closed, err := s.PrepareScope(sampleScopeReq())
	if err != nil {
		t.Fatalf("PrepareScope closed: %v", err)
	}
	if err := s.closeScope(closed.ScopeID); err != nil {
		t.Fatalf("closeScope: %v", err)
	}
	if err := s.bindScopeDrive(closed.ScopeID, closed.ChildCapability, d1); !isOwnershipKind(err, ErrScopeClosed) {
		t.Fatalf("bind on a closed scope must fail ErrScopeClosed, got %v", err)
	}
}

// TestScopeIdentityFailClosed proves LoadScope fails closed on an unknown schema
// version and on a corrupt record (reusing the StoreError kinds), and that a
// traversal-shaped scope id is rejected before any path is built.
func TestScopeIdentityFailClosed(t *testing.T) {
	s := OpenStore(t.TempDir())

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
	s := OpenStore(t.TempDir())
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
// many goroutines race to bind a live drive: the winner binds, every loser gets
// the typed ErrScopeSecondDrive rejection, never a physical-contention leak. Run
// under -race.
func TestScopeCASConcurrent(t *testing.T) {
	s := OpenStore(t.TempDir())
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
			results[idx] = s.bindScopeDrive(grant.ScopeID, grant.ChildCapability, driveID)
		}(i)
	}
	wg.Wait()

	winners := 0
	for _, e := range results {
		if e == nil {
			winners++
			continue
		}
		if !isOwnershipKind(e, ErrScopeSecondDrive) {
			t.Fatalf("loser must fail ErrScopeSecondDrive, got %v", e)
		}
	}
	if winners != 1 {
		t.Fatalf("exactly one bind must win, got %d", winners)
	}
}
