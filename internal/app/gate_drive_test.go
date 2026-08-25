package app

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gatedrive"
)

// fakeDriveEngine is a scriptable driveEngine: every method returns the same
// canned document/error and records the last StartRequest, so the service's
// outcome mapping and its authoritative-config injection are tested without a
// real driver, store, process supervisor, or repository.
type fakeDriveEngine struct {
	doc         gatedrive.DriveDoc
	err         error
	lastStart   gatedrive.StartRequest
	startCalled bool
}

func (f *fakeDriveEngine) Start(r gatedrive.StartRequest) (gatedrive.DriveDoc, error) {
	f.lastStart = r
	f.startCalled = true
	return f.doc, f.err
}
func (f *fakeDriveEngine) Advance(id, ownerGen string) (gatedrive.DriveDoc, error) {
	return f.doc, f.err
}
func (f *fakeDriveEngine) Handoff(id, ownerGen string) (gatedrive.DriveDoc, error) {
	return f.doc, f.err
}
func (f *fakeDriveEngine) Claim(id, handoffID string) (gatedrive.DriveDoc, error) {
	return f.doc, f.err
}

// TestServiceMapsEveryOutcomeIntoProtocolDoc proves each of the four driver
// verdicts maps to a successful (applied) operation carrying the shared DriveDoc
// verbatim, and that only PASSED exposes the raw run dir.
func TestServiceMapsEveryOutcomeIntoProtocolDoc(t *testing.T) {
	cases := []struct {
		outcome    gatedrive.Outcome
		rawRunDir  string
		wantRawDir bool
	}{
		{gatedrive.WAITING, "", false},
		{gatedrive.PASSED, "/runs/abc", true},
		{gatedrive.FAILED, "", false},
		{gatedrive.HALTED, "", false},
	}
	for _, tc := range cases {
		t.Run(string(tc.outcome), func(t *testing.T) {
			eng := &fakeDriveEngine{doc: gatedrive.DriveDoc{
				ProtocolVersion: gatedrive.ProtocolVersion,
				DriveID:         "d1",
				Outcome:         tc.outcome,
				RawRunDir:       tc.rawRunDir,
			}}
			svc := newGateDriveService(eng, 30*time.Minute, "scripts/run-tests.sh", "prov")

			got := svc.Advance("d1", "owner")
			if got.Result != ResultApplied {
				t.Fatalf("a produced verdict is an applied operation, got %s", got.Result)
			}
			if got.Drive == nil {
				t.Fatalf("an applied drive operation must carry the shared document")
			}
			if got.Drive.Outcome != tc.outcome {
				t.Fatalf("outcome not surfaced: got %s want %s", got.Drive.Outcome, tc.outcome)
			}
			if tc.wantRawDir && got.Drive.RawRunDir == "" {
				t.Fatalf("PASSED must expose the raw run dir")
			}
			if !tc.wantRawDir && got.Drive.RawRunDir != "" {
				t.Fatalf("%s must not expose a raw run dir, got %q", tc.outcome, got.Drive.RawRunDir)
			}
			if got.Reason != "" {
				t.Fatalf("a successful operation carries no failure reason, got %q", got.Reason)
			}
		})
	}
}

// TestServiceCommandFailureIsDistinctFromWorkflowResult proves a command failure
// (an unrecognized drive, an unparseable request) is a NON-applied result that
// omits the drive document — distinct from a recognized FAILED/HALTED verdict,
// which is applied and carries the document.
func TestServiceCommandFailureIsDistinctFromWorkflowResult(t *testing.T) {
	// An unrecognized drive: a store not-found is a command failure (invalid input).
	notFound := &fakeDriveEngine{err: &gatedrive.StoreError{Kind: gatedrive.ErrNotFound, Op: "resolve"}}
	svc := newGateDriveService(notFound, 30*time.Minute, "cmd", "prov")
	got := svc.Advance("00000000000000000000000000000000", "owner")
	if got.Result != ResultInvalidInput {
		t.Fatalf("an unrecognized drive must be a command failure (invalid-input), got %s", got.Result)
	}
	if got.Drive != nil {
		t.Fatalf("a command failure must omit the drive document")
	}
	if got.Reason == "" {
		t.Fatalf("a command failure must carry a bounded reason")
	}

	// A generic driver error is also a command failure, never a fabricated verdict.
	generic := &fakeDriveEngine{err: errors.New("boom")}
	svc2 := newGateDriveService(generic, 30*time.Minute, "cmd", "prov")
	got2 := svc2.Claim("d1", "handoff")
	if got2.Result == ResultApplied || got2.Drive != nil {
		t.Fatalf("a generic driver error must be a command failure, got result=%s drive=%v", got2.Result, got2.Drive)
	}

	// A recognized FAILED verdict is NOT a command failure: it is applied and
	// carries the document.
	failed := &fakeDriveEngine{doc: gatedrive.DriveDoc{Outcome: gatedrive.FAILED}}
	svc3 := newGateDriveService(failed, 30*time.Minute, "cmd", "prov")
	got3 := svc3.Advance("d1", "owner")
	if got3.Result != ResultApplied || got3.Drive == nil || got3.Drive.Outcome != gatedrive.FAILED {
		t.Fatalf("a FAILED verdict must be an applied workflow result carrying the document, got %s", got3.Result)
	}
}

// TestServiceStartInjectsAuthoritativeConfig proves Start supplies the resolved
// suite command, budget, and provenance from configuration — never from the
// caller — and shells the resolved command exactly as the finalize gate does.
func TestServiceStartInjectsAuthoritativeConfig(t *testing.T) {
	eng := &fakeDriveEngine{doc: gatedrive.DriveDoc{Outcome: gatedrive.WAITING}}
	svc := newGateDriveService(eng, 42*time.Minute, "scripts/run-tests.sh", "prov-token")

	got := svc.Start(GateDriveStartRequest{
		RepoDir:             "/repo",
		Worktree:            "/repo",
		ChangeID:            "0342",
		TaskID:              "task-9",
		Phase:               "build",
		IdempotentSuiteGate: true,
	})
	if got.Result != ResultApplied {
		t.Fatalf("a WAITING start is an applied operation, got %s", got.Result)
	}
	if !eng.startCalled {
		t.Fatalf("Start must reach the engine")
	}
	wantArgv := []string{"/bin/sh", "-c", "scripts/run-tests.sh"}
	if len(eng.lastStart.Command) != len(wantArgv) {
		t.Fatalf("Start must shell the resolved command, got %v", eng.lastStart.Command)
	}
	for i := range wantArgv {
		if eng.lastStart.Command[i] != wantArgv[i] {
			t.Fatalf("resolved command argv[%d] = %q, want %q", i, eng.lastStart.Command[i], wantArgv[i])
		}
	}
	if eng.lastStart.Budget != 42*time.Minute {
		t.Fatalf("Start must inject the resolved budget, got %v", eng.lastStart.Budget)
	}
	if eng.lastStart.ConfigProvenance != "prov-token" {
		t.Fatalf("Start must inject the config provenance, got %q", eng.lastStart.ConfigProvenance)
	}
	if eng.lastStart.ChangeID != "0342" || !eng.lastStart.IdempotentSuiteGate {
		t.Fatalf("Start must carry the caller identity through, got %+v", eng.lastStart)
	}
}

// TestServiceStartUnresolvedCommandIsCommandFailure proves that an unresolved
// suite command (config resolved to unset) fails closed as a command failure
// before touching the engine — never a fabricated verdict.
func TestServiceStartUnresolvedCommandIsCommandFailure(t *testing.T) {
	eng := &fakeDriveEngine{}
	svc := newGateDriveService(eng, 30*time.Minute, "", "prov")
	got := svc.Start(GateDriveStartRequest{RepoDir: "/repo", Worktree: "/repo"})
	if got.Result == ResultApplied || got.Drive != nil {
		t.Fatalf("an unresolved command must be a command failure, got result=%s", got.Result)
	}
	if got.Reason == "" {
		t.Fatalf("an unresolved command must carry a reason")
	}
	if eng.startCalled {
		t.Fatalf("an unresolved command must not reach the engine")
	}
}

// TestNewGateDriveServiceResolvesConfig proves the production constructor resolves
// the config-provenanced observation budget (minutes) and suite command from the
// effective configuration and roots the store at the given Git common dir without
// shelling out.
func TestNewGateDriveServiceResolvesConfig(t *testing.T) {
	eff := config.Effective{
		GateObservation: config.Value[int]{Value: 30, Provenance: config.Provenance{Layer: config.LayerRepository}},
	}
	eff.Finalize.TestCommand = config.Value[string]{Value: "scripts/run-tests.sh", Provenance: config.Provenance{Layer: config.LayerRepository}}

	svc, res, reason := NewGateDriveService(t.TempDir(), "/usr/bin/true", eff)
	if svc == nil {
		t.Fatalf("production constructor must build a service: %s %s", res, reason)
	}
	if svc.budget != 30*time.Minute {
		t.Fatalf("budget must resolve from gate_observation_budget minutes, got %v", svc.budget)
	}
	if svc.command != "scripts/run-tests.sh" {
		t.Fatalf("command must resolve from finalize.test_command, got %q", svc.command)
	}
	if svc.provenance == "" {
		t.Fatalf("the service must record a config provenance")
	}
}

// --- FIX #4: run-root cleanup at the terminal (temp-dir leak) --------------

// runRootFixture makes a real directory that stands in for a drive's private
// run root, so a test can assert whether mapDriveOutcome removed it.
func runRootFixture(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp(t.TempDir(), "runroot-*")
	if err != nil {
		t.Fatalf("mktemp: %v", err)
	}
	return d
}

func dirExists(t *testing.T, p string) bool {
	t.Helper()
	_, err := os.Stat(p)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	t.Fatalf("stat %q: %v", p, err)
	return false
}

// TestMapDriveOutcomeRemovesRunRootOnTerminal proves the per-drive run root is
// removed once the drive reaches a terminal FAILED/HALTED outcome — including
// when that terminal is reached on a resume slice, where the root is recovered
// from the terminal document's RunRoot rather than from a local variable.
func TestMapDriveOutcomeRemovesRunRootOnTerminal(t *testing.T) {
	g := &processFinalizeGate{}
	for _, tc := range []struct {
		name string
		doc  gatedrive.DriveDoc
	}{
		{"failed", gatedrive.DriveDoc{Outcome: gatedrive.FAILED}},
		{"halted", gatedrive.DriveDoc{Outcome: gatedrive.HALTED, Cause: gatedrive.CauseUnknownObservation}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := runRootFixture(t)
			doc := tc.doc
			doc.RunRoot = root
			g.mapDriveOutcome(context.Background(), LocalGateRequest{}, GateDriveResult{Drive: &doc})
			if dirExists(t, root) {
				t.Fatalf("terminal %s outcome must remove the run root %q", tc.name, root)
			}
		})
	}
}

// TestMapDriveOutcomeRetainsRunRootWhileWaiting proves a WAITING slice never
// removes the run root — the run is still live and may relaunch under it. This
// is the guard that keeps the terminal-only removal from deleting a live drive's
// root; stripping the outcome check reddens it.
func TestMapDriveOutcomeRetainsRunRootWhileWaiting(t *testing.T) {
	g := &processFinalizeGate{}
	root := runRootFixture(t)
	doc := gatedrive.DriveDoc{Outcome: gatedrive.WAITING, DriveID: "d1", Generation: "g1", RunRoot: root}
	res := g.mapDriveOutcome(context.Background(), LocalGateRequest{}, GateDriveResult{Drive: &doc})
	if res.Outcome != FinalizeGateWaiting {
		t.Fatalf("outcome = %q, want waiting", res.Outcome)
	}
	if !dirExists(t, root) {
		t.Fatalf("a live WAITING drive must retain its run root %q", root)
	}
}

// --- FIX #5: halt-cause mapping keyed on exported gatedrive constants -------

// TestMapDriveHaltCauseKeysOnGatedriveConstants pins each gatedrive cause
// constant to its intended finalize halt classification, referencing the
// EXPORTED constants (never literal spellings) so a rename of a token in
// gatedrive is a compile error at this mapping rather than a silent
// reclassification.
func TestMapDriveHaltCauseKeysOnGatedriveConstants(t *testing.T) {
	cases := map[string]string{
		gatedrive.CauseDeadlineExpired:       GateHaltRunningAtBudget,
		gatedrive.CauseSchemaMismatch:        GateHaltMalformed,
		gatedrive.CauseObservationUnreadable: GateHaltMalformed,
		gatedrive.CauseUnknownObservation:    GateHaltMalformed,
		// A deadline-expired variant is matched as a prefix of the constant.
		gatedrive.CauseDeadlineExpired + "-stop-unproven": GateHaltRunningAtBudget,
		// Any cause the mapping does not distinguish falls through to unavailable.
		"owner-superseded": GateHaltUnavailable,
	}
	for cause, want := range cases {
		if got := mapDriveHaltCause(cause); got != want {
			t.Fatalf("mapDriveHaltCause(%q) = %q, want %q", cause, got, want)
		}
	}
}
