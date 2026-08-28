package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// proceedableFacts is a facts value that classifies fresh and passes every
// supported-contract preflight, so a test can perturb one field to exercise a
// single refusal in isolation.
func proceedableFacts() reposetup.Facts {
	return reposetup.Facts{
		RemoteConfigured:    reposetup.PresencePresent,
		RemoteDefaultBranch: reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: "a"},
		RemoteIntegration:   reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: "b"},
		RemoteMetadata:      reposetup.BranchFact{Presence: reposetup.PresenceAbsent},
		LiveSurface:         reposetup.PresenceAbsent,
		PrimaryClean:        reposetup.PresencePresent,
		PrimaryAtRemoteTip:  reposetup.PresencePresent,
	}
}

// TestInitGuardLegacyRefusesToMigrate proves a legacy live surface refuses init
// with invalid-state and a remedy that names migrate, never check.
func TestInitGuardLegacyRefusesToMigrate(t *testing.T) {
	f := proceedableFacts()
	f.LiveSurface = reposetup.PresencePresent // no remote metadata + live surface = legacy

	_, refusal := initGuard(f)
	if refusal == nil {
		t.Fatal("legacy facts must refuse init")
	}
	if refusal.Result != ResultInvalidState {
		t.Errorf("Result = %q, want invalid-state", refusal.Result)
	}
	if refusal.RepositoryState != string(reposetup.StateLegacy) {
		t.Errorf("RepositoryState = %q, want legacy", refusal.RepositoryState)
	}
	h := refusal.HumanText()
	if !strings.Contains(h, "migrate") {
		t.Errorf("remedy %q must name migrate", h)
	}
	if strings.Contains(h, "check") {
		t.Errorf("legacy remedy %q must not name check", h)
	}
}

// TestInitGuardUnknownRefusesToCheck proves an unresolved probe refuses init with
// invalid-state and a remedy that names check.
func TestInitGuardUnknownRefusesToCheck(t *testing.T) {
	f := proceedableFacts()
	f.RemoteMetadata = reposetup.BranchFact{Presence: reposetup.PresenceUnknown}

	_, refusal := initGuard(f)
	if refusal == nil {
		t.Fatal("unknown facts must refuse init")
	}
	if refusal.Result != ResultInvalidState {
		t.Errorf("Result = %q, want invalid-state", refusal.Result)
	}
	if refusal.RepositoryState != string(reposetup.StateUnknown) {
		t.Errorf("RepositoryState = %q, want unknown", refusal.RepositoryState)
	}
	if !strings.Contains(refusal.HumanText(), "check") {
		t.Errorf("unknown remedy %q must name check", refusal.HumanText())
	}
}

// TestInitGuardDirtyPrimaryRefuses proves a dirty primary worktree refuses the
// supported-contract preflight.
func TestInitGuardDirtyPrimaryRefuses(t *testing.T) {
	f := proceedableFacts()
	f.PrimaryClean = reposetup.PresenceAbsent

	_, refusal := initGuard(f)
	if refusal == nil || refusal.Result != ResultInvalidState {
		t.Fatalf("dirty primary must refuse with invalid-state, got %+v", refusal)
	}
}

// TestInitGuardFreshProceeds proves a fresh, clean repository is not refused.
func TestInitGuardFreshProceeds(t *testing.T) {
	if _, refusal := initGuard(proceedableFacts()); refusal != nil {
		t.Fatalf("fresh facts must proceed, got refusal %+v", refusal)
	}
}

// TestRepositoryOpResultJSONFieldNames pins the protocol-v1 field names the
// result document carries.
func TestRepositoryOpResultJSONFieldNames(t *testing.T) {
	out := newRepositoryOpResult(OperationRepositoryInit, ResultApplied, RepositoryOpResult{
		RepositoryState: string(reposetup.StateNeedsReview),
		PendingPaths:    []string{".gitignore"},
		MetadataTip:     "deadbeef",
		SourceRevision:  "cafebabe",
	})
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"protocol_version", "operation", "result", "repository_state", "pending_paths", "metadata_revision", "source_revision"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("result JSON missing field %q: %s", key, raw)
		}
	}
	if decoded["operation"] != OperationRepositoryInit {
		t.Errorf("operation = %v, want %q", decoded["operation"], OperationRepositoryInit)
	}
}

// TestRepositoryOpResultHumanTextNamesPendingPaths proves the default human text
// names every pending review path.
func TestRepositoryOpResultHumanTextNamesPendingPaths(t *testing.T) {
	out := newRepositoryOpResult(OperationRepositoryInit, ResultApplied, RepositoryOpResult{
		RepositoryState: string(reposetup.StateNeedsReview),
		PendingPaths:    []string{".gitignore", "CLAUDE.md"},
	})
	h := out.HumanText()
	for _, p := range []string{".gitignore", "CLAUDE.md"} {
		if !strings.Contains(h, p) {
			t.Errorf("HumanText %q must name pending path %q", h, p)
		}
	}
}
