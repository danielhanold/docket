package gatedrive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestDriveDocCarriesProtocolAndOutcome pins the emitted outcome document's
// protocol-v1 shape: the protocol version, the surfaced outcome, and the raw
// run dir that only a PASSED doc exposes.
func TestDriveDocCarriesProtocolAndOutcome(t *testing.T) {
	d := DriveDoc{ProtocolVersion: 1, DriveID: "d1", Generation: "g1", Attempt: 1,
		Outcome: PASSED, RawRunDir: "/runs/abc"}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["protocol_version"].(float64) != 1 {
		t.Fatalf("missing protocol_version")
	}
	if m["outcome"] != "PASSED" {
		t.Fatalf("outcome not surfaced")
	}
	if _, ok := m["raw_run_dir"]; !ok {
		t.Fatalf("passed doc must expose raw run dir")
	}
}

// TestDriveDocRedactsSecrets proves the diagnostic document never carries a
// launch argv, environment value, worktree diff, or ownership credential —
// only bounded identity and a typed cause.
func TestDriveDocRedactsSecrets(t *testing.T) {
	// launch argv, env values, worktree diff, credential must never appear in the doc.
	d := DriveDoc{ProtocolVersion: 1, Outcome: HALTED, Cause: "identity-mismatch"}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"argv", "env", "diff", "credential", "token"} {
		if strings.Contains(strings.ToLower(string(b)), banned) {
			t.Fatalf("doc leaked %q", banned)
		}
	}
}

// TestDriveDocOutcomeConstants pins the four typed outcome spellings the whole
// driver protocol keys on.
func TestDriveDocOutcomeConstants(t *testing.T) {
	cases := map[Outcome]string{
		WAITING: "WAITING",
		PASSED:  "PASSED",
		FAILED:  "FAILED",
		HALTED:  "HALTED",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Fatalf("outcome constant = %q, want %q", string(got), want)
		}
	}
}

// TestNonPassedDocOmitsRawRunDir proves the raw run dir is populated on PASSED
// only — a WAITING/FAILED/HALTED doc must not expose it.
func TestNonPassedDocOmitsRawRunDir(t *testing.T) {
	d := DriveDoc{ProtocolVersion: 1, Outcome: WAITING}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["raw_run_dir"]; ok {
		t.Fatalf("non-passed doc must not expose raw run dir")
	}
}

// TestDriveRecordCarriesSchemaVersion proves the persisted schema struct stamps
// an explicit schema version so the store (Task 4) refuses an unknown one
// rather than best-effort migrating it.
func TestDriveRecordCarriesSchemaVersion(t *testing.T) {
	r := driveRecord{SchemaVersion: driveSchemaVersion}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["schema_version"]; !ok {
		t.Fatalf("persisted record must carry schema_version")
	}
	if driveSchemaVersion < 1 {
		t.Fatalf("schema version must be a positive generation, got %d", driveSchemaVersion)
	}
}

// TestDriveSchemaV1FailsClosedUnderV2 proves the immediately-prior schema
// generation is not migrated: a persisted v1 record read by the v2 store fails
// closed with the typed unknown-schema error rather than being silently upgraded.
func TestDriveSchemaV1FailsClosedUnderV2(t *testing.T) {
	if driveSchemaVersion != 2 {
		t.Fatalf("this fail-closed assertion is pinned to schema v2, got v%d", driveSchemaVersion)
	}
	s := OpenStore(testsupport.TempDir(t))
	id, _, err := s.NewDrive(sampleRecord())
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	// Overwrite the record with an explicit v1 schema version — the prior schema.
	rec := sampleRecord()
	rec.SchemaVersion = 1
	buf, err := json.Marshal(storedRecord{Generation: "x", Record: rec})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.root, id, recordFileName), buf, 0o600); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	_, err = s.Load(id)
	if se, ok := AsStoreError(err); !ok || se.Kind != ErrUnknownSchema {
		t.Fatalf("a v1 record must fail closed as ErrUnknownSchema under v2, got %v", err)
	}
}
