package codexcontract

import (
	"strings"
	"testing"
)

func TestParseReceiptUsesNestedDriveAndPreservesNonzeroOutput(t *testing.T) {
	stdout := []byte(`{"protocol_version":1,"operation":"gate.drive.start","result":"gate-failed","drive":{"protocol_version":1,"drive_id":"drive-1","generation":"gen-1","deadline":"2026-09-14T12:00:00Z","outcome":"FAILED","cause":"tests","run_root":"/private/runs"}}`)
	r, err := ParseReceipt("gate.drive.start", stdout, []byte("diagnostic"), 1, Assignment{RunRoot: "/private/runs"})
	if err != nil {
		t.Fatalf("ParseReceipt: %v", err)
	}
	if r.Drive == nil || r.Drive.DriveID != "drive-1" || r.Drive.Outcome != "FAILED" {
		t.Fatalf("nested drive lost: %+v", r)
	}
	if string(r.Stdout) != string(stdout) || string(r.Stderr) != "diagnostic" || r.ExitCode != 1 {
		t.Fatalf("original process receipt lost: %+v", r)
	}

	old := []byte(`{"protocol_version":1,"operation":"gate.drive.start","result":"applied","drive_id":"drive-1","owner_generation":"gen-1"}`)
	if _, err := ParseReceipt("gate.drive.start", old, nil, 0, Assignment{}); err == nil {
		t.Fatal("accepted obsolete flattened drive shape")
	}
}

func TestParseReceiptValidatesWaitingAndTerminalRunRoots(t *testing.T) {
	waiting := []byte(`{"protocol_version":1,"operation":"gate.drive.start","result":"applied","drive":{"protocol_version":1,"drive_id":"d","generation":"g","deadline":"2026-09-14T12:00:00Z","outcome":"WAITING"}}`)
	if _, err := ParseReceipt("gate.drive.start", waiting, nil, 0, Assignment{RunRoot: "/private/runs"}); err != nil {
		t.Fatalf("WAITING without run_root: %v", err)
	}
	bad := []byte(strings.Replace(string(waiting), `"outcome":"WAITING"`, `"outcome":"PASSED","run_root":"relative"`, 1))
	if _, err := ParseReceipt("gate.drive.start", bad, nil, 0, Assignment{RunRoot: "/private/runs"}); err == nil {
		t.Fatal("accepted terminal relative run_root")
	}
}

func TestParseReceiptReadsScopeTopLevelShape(t *testing.T) {
	b := []byte(`{"protocol_version":1,"operation":"gate.drive.prepare-scope","result":"applied","scope_id":"s","child_capability":"c","parent_capability":"p"}`)
	r, err := ParseReceipt("gate.drive.prepare-scope", b, nil, 0, Assignment{})
	if err != nil || r.ScopeID != "s" || r.ChildCapability != "c" || r.ParentCapability != "p" {
		t.Fatalf("scope receipt = %+v, %v", r, err)
	}
}

func TestParseReceiptTransferOmitsTerminalRunRoot(t *testing.T) {
	b := []byte(`{"protocol_version":1,"operation":"gate.drive.claim","result":"applied","drive":{"protocol_version":1,"drive_id":"d","generation":"g","deadline":"2026-09-14T12:00:00Z","outcome":"PASSED"}}`)
	if _, err := ParseReceipt("gate.drive.claim", b, nil, 0, Assignment{RunRoot: "/private/runs"}); err != nil {
		t.Fatalf("terminal transfer without run_root: %v", err)
	}
}

func TestParseReceiptPassedRawRunDirStaysWithinRunRoot(t *testing.T) {
	b := []byte(`{"protocol_version":1,"operation":"gate.drive.start","result":"applied","drive":{"protocol_version":1,"drive_id":"d","generation":"g","deadline":"2026-09-14T12:00:00Z","outcome":"PASSED","run_root":"/private/runs","raw_run_dir":"/elsewhere/run"}}`)
	if _, err := ParseReceipt("gate.drive.start", b, nil, 0, Assignment{RunRoot: "/private/runs"}); err == nil {
		t.Fatal("accepted PASSED raw_run_dir outside assigned run root")
	}
}

func TestParseReceiptClassifiesDiagnosticHaltWithoutOwnership(t *testing.T) {
	b := []byte(`{"protocol_version":1,"operation":"gate.drive.start","result":"invalid-state","reason":"run-cancelled","message":"use current epoch"}`)
	r, err := ParseReceipt("gate.drive.start", b, []byte("diagnostic"), 1, Assignment{RunRoot: "/private/runs"})
	if err != nil {
		t.Fatalf("diagnostic halt: %v", err)
	}
	if r.Classification != "halt" || r.Reason != "run-cancelled" || r.Drive != nil || r.ExitCode != 1 {
		t.Fatalf("halt receipt = %+v", r)
	}
}
