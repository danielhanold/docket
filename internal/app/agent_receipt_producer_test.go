package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/codexcontract"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

type receiptClock struct{ now time.Time }

func (c receiptClock) Now() time.Time                  { return c.now }
func (c receiptClock) Since(t time.Time) time.Duration { return c.now.Sub(t) }

type receiptGit struct{}

func (receiptGit) HeadOID(string) (string, error)       { return "head", nil }
func (receiptGit) IndexEntries(string) ([]byte, error)  { return nil, nil }
func (receiptGit) Status(string) ([]byte, error)        { return nil, nil }
func (receiptGit) WorktreePaths(string) ([]byte, error) { return nil, nil }

type receiptProcess struct{ runDir string }

func (p receiptProcess) Launch(process.LaunchRequest) (*process.LaunchOutcome, error) {
	return &process.LaunchOutcome{RunID: "run", RunDir: p.runDir, State: process.StateRunning}, nil
}
func (p receiptProcess) Observe(string) (*process.Observation, error) {
	return &process.Observation{RunID: "run", RunDir: p.runDir, State: process.StatePassed}, nil
}
func (p receiptProcess) Stop(string, string) (*process.StopOutcome, error) {
	return &process.StopOutcome{RunID: "run", RunDir: p.runDir, State: process.StatePassed}, nil
}
func (receiptProcess) ResolveReservation(string, string) (*process.ReservationResolution, error) {
	return &process.ReservationResolution{Disposition: "never-launched"}, nil
}

func parseProducedDriveReceipt(t *testing.T, result GateDriveResult, exitCode int, runRoot string) (codexcontract.Receipt, error) {
	t.Helper()
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return codexcontract.ParseReceipt(result.Operation, body, []byte("captured diagnostic"), exitCode, codexcontract.Assignment{RunRoot: runRoot})
}

func TestAgentReceiptAcceptsProductionTransferAndDiagnosticHaltShapes(t *testing.T) {
	root := testsupport.TempDir(t)
	worktree := filepath.Join(root, "worktree")
	runRoot := filepath.Join(root, "runs")
	if err := mkdirAll(worktree, runRoot); err != nil {
		t.Fatal(err)
	}
	rawRunDir := filepath.Join(runRoot, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	store := gatedrive.OpenStore(filepath.Join(root, "common"))
	grant, err := store.PrepareScope(gatedrive.ScopeRequest{RepoIdentity: root, Worktree: worktree, ChangeID: "425", TaskID: "1", Phase: "build", Branch: "codex/review"})
	if err != nil {
		t.Fatal(err)
	}
	driver := gatedrive.NewDriver(store, receiptClock{now: time.Unix(1_000_000, 0)}, receiptProcess{runDir: rawRunDir}, receiptGit{})
	service := newGateDriveService(driver, time.Minute, "true", "test")
	started := service.Start(GateDriveStartRequest{RepoDir: root, Worktree: worktree, ChangeID: "425", TaskID: "1", Phase: "build", Branch: "codex/review", Ref: "refs/heads/codex/review", Cwd: worktree, RunRoot: runRoot, ScopeID: grant.ScopeID, ChildCapability: grant.ChildCapability})
	if started.Drive == nil || started.Drive.Outcome != gatedrive.PASSED {
		t.Fatalf("start did not produce PASSED: %+v", started)
	}

	handoff := service.Handoff(started.Drive.DriveID, started.Drive.Generation)
	if handoff.Drive == nil || handoff.Drive.RawRunDir != rawRunDir || handoff.Drive.RunRoot != "" {
		t.Fatalf("production handoff shape changed: %+v", handoff)
	}
	parsed, err := parseProducedDriveReceipt(t, handoff, 0, runRoot)
	if err != nil {
		t.Fatalf("production PASSED transfer rejected: %v", err)
	}
	if parsed.Classification != string(gatedrive.PASSED) {
		t.Fatalf("transfer classification = %q", parsed.Classification)
	}
	if _, err := parseProducedDriveReceipt(t, handoff, 1, runRoot); err == nil {
		t.Fatal("accepted PASSED transfer with a nonzero exit status")
	}

	malformed := handoff
	doc := *handoff.Drive
	doc.RawRunDir = filepath.Join(root, "elsewhere", "run")
	malformed.Drive = &doc
	if _, err := parseProducedDriveReceipt(t, malformed, 0, runRoot); err == nil {
		t.Fatal("accepted PASSED transfer raw path outside the assigned run root")
	}

	claimed := service.Claim(started.Drive.DriveID, handoff.Drive.Generation)
	if claimed.Drive == nil || claimed.Drive.Outcome != gatedrive.PASSED {
		t.Fatalf("claim did not close the passing scope: %+v", claimed)
	}
	halted := service.Takeover(grant.ScopeID, grant.ParentCapability, started.Drive.DriveID)
	if halted.Drive == nil || halted.Drive.Outcome != gatedrive.HALTED || !halted.Drive.Deadline.IsZero() || halted.Drive.Generation != "" || halted.Drive.RunRoot != "" {
		t.Fatalf("production diagnostic halt shape changed: %+v", halted)
	}
	parsed, err = parseProducedDriveReceipt(t, halted, 1, runRoot)
	if err != nil {
		t.Fatalf("production diagnostic HALTED document rejected: %v", err)
	}
	if parsed.Classification != "halt" {
		t.Fatalf("diagnostic halt classification = %q", parsed.Classification)
	}
	if _, err := parseProducedDriveReceipt(t, halted, 0, runRoot); err == nil {
		t.Fatal("accepted diagnostic HALTED document with a zero exit status")
	}
}

func mkdirAll(paths ...string) error {
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	return nil
}
