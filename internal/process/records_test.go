package process

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

func TestManifestRoundTrip(t *testing.T) {
	dir := testsupport.TempDir(t)
	in := &manifestRecord{Schema: recordSchema, RunID: "aa", Token: "bb", Root: "/r",
		RunDir: dir, SupervisorPID: 42, PGID: 42, SID: 42, Phase: "allocated",
		Cwd: "/w", Argv0: "true", Argc: 1, CreatedAt: "2026-08-16T00:00:00Z", UpdatedAt: "2026-08-16T00:00:00Z"}
	if err := writeAtomicJSON(filepath.Join(dir, manifestFile), in); err != nil {
		t.Fatal(err)
	}
	out, err := readManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if *out != *in {
		t.Fatalf("round trip mismatch:\n in=%+v\nout=%+v", in, out)
	}
}

func TestTerminalRoundTrip(t *testing.T) {
	dir := testsupport.TempDir(t)
	in := &terminalRecord{Schema: recordSchema, RunID: "aa", Kind: "signal", Signal: 15, RecordedAt: "x"}
	if err := writeAtomicJSON(filepath.Join(dir, terminalFile), in); err != nil {
		t.Fatal(err)
	}
	out, err := readTerminal(dir)
	if err != nil || out.Kind != "signal" || out.Signal != 15 {
		t.Fatalf("got %+v, %v", out, err)
	}
}

func TestStopIntentRoundTrip(t *testing.T) {
	dir := testsupport.TempDir(t)
	in := &stopIntentRecord{Schema: recordSchema, RunID: "aa", Reason: "operator stop", RecordedAt: "x"}
	if err := writeAtomicJSON(filepath.Join(dir, stopIntentFile), in); err != nil {
		t.Fatal(err)
	}
	out, err := readStopIntent(dir)
	if err != nil || out.Reason != "operator stop" {
		t.Fatalf("got %+v, %v", out, err)
	}
}

func TestStoppedRoundTrip(t *testing.T) {
	dir := testsupport.TempDir(t)
	in := &stoppedRecord{Schema: recordSchema, RunID: "aa", VerifiedAt: "x"}
	if err := writeAtomicJSON(filepath.Join(dir, stoppedFile), in); err != nil {
		t.Fatal(err)
	}
	out, err := readStopped(dir)
	if err != nil || out.RunID != "aa" || out.VerifiedAt != "x" {
		t.Fatalf("got %+v, %v", out, err)
	}
}

func TestAbandonedRoundTrip(t *testing.T) {
	dir := testsupport.TempDir(t)
	in := &abandonedRecord{Schema: recordSchema, RunID: "aa", Cause: "vanished", RecordedAt: "x"}
	if err := writeAtomicJSON(filepath.Join(dir, abandonedFile), in); err != nil {
		t.Fatal(err)
	}
	out, err := readAbandoned(dir)
	if err != nil || out.Cause != "vanished" {
		t.Fatalf("got %+v, %v", out, err)
	}
}

func TestFailureRecordRoundTrip(t *testing.T) {
	dir := testsupport.TempDir(t)
	in := &failureRecord{Schema: recordSchema, RunID: "aa", Stage: "start", Reason: "exec failed", RecordedAt: "x"}
	if err := writeAtomicJSON(filepath.Join(dir, failureFile), in); err != nil {
		t.Fatal(err)
	}
	out, err := readFailureRecord(dir)
	if err != nil || out.Stage != "start" || out.Reason != "exec failed" {
		t.Fatalf("got %+v, %v", out, err)
	}
}

// TestReadersThreeWayContract — absent is (nil, nil); malformed JSON and a
// wrong schema are FailInvalidState; the three answers never collapse.
func TestReadersThreeWayContract(t *testing.T) {
	dir := testsupport.TempDir(t)
	if rec, err := readTerminal(dir); rec != nil || err != nil {
		t.Fatalf("clean absence must be (nil, nil), got %v, %v", rec, err)
	}
	if err := os.WriteFile(filepath.Join(dir, terminalFile), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readTerminal(dir); err == nil {
		t.Fatal("malformed record must error")
	} else if f, ok := AsFailure(err); !ok || f.Class != FailInvalidState {
		t.Fatalf("malformed record class = %v", err)
	}
	if err := writeAtomicJSON(filepath.Join(dir, terminalFile),
		map[string]any{"schema": 99, "run_id": "aa", "kind": "exit"}); err != nil {
		t.Fatal(err)
	}
	if _, err := readTerminal(dir); err == nil {
		t.Fatal("unknown schema must be refused, not guessed")
	} else if f, ok := AsFailure(err); !ok || f.Class != FailInvalidState {
		t.Fatalf("unknown schema class = %v", err)
	}
}
