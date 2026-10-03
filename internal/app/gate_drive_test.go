package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/process"
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
	startCount  int
	// admitErr, when set, makes Admit refuse (the launch/StartAdmitted half is never
	// reached) — e.g. a worktree-busy lock — so a test can prove the charge stays
	// AFTER Admit.
	admitErr error
	// startAdmittedCount and abandonCount count the two launch-half calls so a test
	// can prove the build owner charged BETWEEN admission and launch (and abandoned
	// on a post-admission charge failure). startCount counts Start OR Admit — the
	// number of admissions reached — so the existing budget assertions carry over
	// unchanged whichever half the owner used.
	startAdmittedCount int
	abandonCount       int
}

func (f *fakeDriveEngine) recordStart(r gatedrive.StartRequest) {
	f.lastStart = r
	f.startCalled = true
	f.startCount++
}

func (f *fakeDriveEngine) Start(r gatedrive.StartRequest) (gatedrive.DriveDoc, error) {
	f.recordStart(r)
	return f.doc, f.err
}

func (f *fakeDriveEngine) Admit(r gatedrive.StartRequest) (*gatedrive.AdmissionTicket, error) {
	f.recordStart(r)
	if f.admitErr != nil {
		return nil, f.admitErr
	}
	return &gatedrive.AdmissionTicket{}, nil
}

func (f *fakeDriveEngine) StartAdmitted(*gatedrive.AdmissionTicket) (gatedrive.DriveDoc, error) {
	f.startAdmittedCount++
	return f.doc, f.err
}

func (f *fakeDriveEngine) AbandonAdmission(*gatedrive.AdmissionTicket) error {
	f.abandonCount++
	return nil
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
			svc := newGateDriveService(eng, 30*time.Minute, "go test ./...", "prov")

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
	svc := newGateDriveService(eng, 42*time.Minute, "go test ./...", "prov-token")

	got := svc.Start(GateDriveStartRequest{
		RepoDir:  "/repo",
		Worktree: "/repo",
		ChangeID: "0342",
		TaskID:   "task-9",
		Phase:    "build",
	})
	if got.Result != ResultApplied {
		t.Fatalf("a WAITING start is an applied operation, got %s", got.Result)
	}
	if !eng.startCalled {
		t.Fatalf("Start must reach the engine")
	}
	wantArgv := []string{"/bin/sh", "-c", "go test ./..."}
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
	if eng.lastStart.ChangeID != "0342" {
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

// TestFinalizeConstructorResolvesConfig proves the finalize owner constructor
// resolves the config-provenanced observation budget (minutes) and suite command
// from the effective configuration and roots the store at the given Git common
// dir without shelling out.
func TestFinalizeConstructorResolvesConfig(t *testing.T) {
	eff := config.Effective{
		GateObservation: config.Value[int]{Value: 30, Provenance: config.Provenance{Layer: config.LayerRepository}},
	}
	eff.Finalize.TestCommand = config.Value[string]{Value: "go test ./...", Provenance: config.Provenance{Layer: config.LayerRepository}}

	svc, res, reason := NewFinalizeGateDriveService(testsupport.TempDir(t), "/usr/bin/true", eff)
	if svc == nil {
		t.Fatalf("production constructor must build a service: %s %s", res, reason)
	}
	if svc.budget != 30*time.Minute {
		t.Fatalf("budget must resolve from gate_observation_budget minutes, got %v", svc.budget)
	}
	if svc.command != "go test ./..." {
		t.Fatalf("command must resolve from finalize.test_command, got %q", svc.command)
	}
	if svc.provenance == "" {
		t.Fatalf("the service must record a config provenance")
	}
}

// TestOwnerConstructorsReadOnlyTheirOwnCommand is the divergent-command fixture
// the spec's Testing section requires: build and finalize test commands DIFFER,
// so a service reading the wrong key cannot pass. Each owner constructor must
// read ONLY its own test_command and name its own owning path in the persisted
// provenance.
func TestOwnerConstructorsReadOnlyTheirOwnCommand(t *testing.T) {
	eff := config.Effective{}
	eff.GateObservation = config.Value[int]{Value: 5, Provenance: config.Provenance{Layer: config.LayerRepository}}
	eff.Build.TestCommand = config.Value[string]{Value: "go test ./build-only",
		Provenance: config.Provenance{Layer: config.LayerRepository}}
	eff.Finalize.TestCommand = config.Value[string]{Value: "make finalize-only",
		Provenance: config.Provenance{Layer: config.LayerGlobal}}

	b, _, _ := NewBuildGateDriveService(testsupport.TempDir(t), "/bin/true", eff)
	if b.command != "go test ./build-only" {
		t.Errorf("build service command = %q; it must read only build.test_command", b.command)
	}
	if want := "gate_observation_budget=repository;build.test_command=repository"; !strings.HasSuffix(b.provenance, "build.test_command=repository") {
		t.Errorf("build provenance = %q, want it to name build.test_command (e.g. %q)", b.provenance, want)
	}

	f, _, _ := NewFinalizeGateDriveService(testsupport.TempDir(t), "/bin/true", eff)
	if f.command != "make finalize-only" {
		t.Errorf("finalize service command = %q; it must read only finalize.test_command", f.command)
	}
	if !strings.Contains(f.provenance, "finalize.test_command=") {
		t.Errorf("finalize provenance = %q, want it to name finalize.test_command", f.provenance)
	}
}

// TestOwnerConstructorUnresolvedCommandNamesRemedy proves that an owner whose
// own test_command is unconfigured fails Start closed with the stable
// unresolved-command reason token AND a human message naming the owner and the
// setup remedy — never a fabricated verdict, and never reaching the engine.
func TestOwnerConstructorUnresolvedCommandNamesRemedy(t *testing.T) {
	eff := config.Effective{}
	eff.GateObservation = config.Value[int]{Value: 5, Provenance: config.Provenance{Layer: config.LayerRepository}}
	// Build command left unconfigured; finalize is set to prove the build owner
	// does not fall back to it.
	eff.Finalize.TestCommand = config.Value[string]{Value: "make finalize-only",
		Provenance: config.Provenance{Layer: config.LayerGlobal}}

	b, _, _ := NewBuildGateDriveService(testsupport.TempDir(t), "/bin/true", eff)
	got := b.Start(GateDriveStartRequest{RepoDir: "/repo", Worktree: "/repo"})
	if got.Result == ResultApplied || got.Drive != nil {
		t.Fatalf("an unresolved build command must be a command failure, got result=%s", got.Result)
	}
	if got.Reason != "unresolved-command" {
		t.Errorf("reason token = %q, want the stable unresolved-command token", got.Reason)
	}
	if !strings.Contains(got.Message, "build") || !strings.Contains(got.Message, "docket repository configure-tests") {
		t.Errorf("message %q must name the owner and the setup remedy", got.Message)
	}
}

// --- FIX #4: run-root cleanup at the terminal (temp-dir leak) --------------

// runRootFixture makes a real directory that stands in for a drive's private
// run root, so a test can assert whether mapDriveOutcome removed it.
func runRootFixture(t *testing.T) string {
	t.Helper()
	return testsupport.TempDir(t)
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

// TestFinalizeCleanupReportsWithheldRunRoot (review fix): a terminal document that
// carries NO RunRoot (a defensive leg — the driver exposes it on every terminal)
// leaves the root the Start minted on disk. That retention must surface as the
// bounded TeardownFinding, never silently.
func TestFinalizeCleanupReportsWithheldRunRoot(t *testing.T) {
	g := &processFinalizeGate{}
	doc := gatedrive.DriveDoc{Outcome: gatedrive.HALTED, Cause: gatedrive.CauseUnknownObservation}
	res := g.mapDriveOutcome(context.Background(), LocalGateRequest{}, GateDriveResult{Drive: &doc})
	if res.Outcome != FinalizeGateHalted {
		t.Fatalf("outcome = %q, want halted", res.Outcome)
	}
	if res.TeardownFinding != teardownFindingRunRootRetainedUnsettled {
		t.Fatalf("TeardownFinding = %q, want %q", res.TeardownFinding, teardownFindingRunRootRetainedUnsettled)
	}
	// A root the document does expose is removed and reports no retention.
	root := runRootFixture(t)
	doc.RunRoot = root
	if res := g.mapDriveOutcome(context.Background(), LocalGateRequest{}, GateDriveResult{Drive: &doc}); res.TeardownFinding != "" {
		t.Fatalf("an exposed, removed root reported TeardownFinding %q", res.TeardownFinding)
	}
}

// runRootObserver answers Observe from a map keyed by run dir; a missing dir is
// an observation error.
type runRootObserver map[string]process.State

func (m runRootObserver) Observe(runDir string) (*process.Observation, error) {
	st, ok := m[runDir]
	if !ok {
		return nil, errors.New("unobservable")
	}
	return &process.Observation{RunDir: runDir, State: st}, nil
}

// TestMapDriveOutcomeRetainsHaltedRunRootWithUnexitedRun: a HALTED drive whose
// recorded run's supervisor may still be live (a deadline expiry whose stop was
// unproven) must NOT have its run root deleted — that would destroy the live
// gate's manifest and logs and leave busy refusals saying "holder unknown". The
// root is kept and the retention surfaces as the bounded TeardownFinding. A run
// that observes exited still has its root removed.
func TestMapDriveOutcomeRetainsHaltedRunRootWithUnexitedRun(t *testing.T) {
	for _, tc := range []struct {
		name   string
		state  process.State // "" = unobservable
		retain bool
	}{
		{"running", process.StateRunning, true},
		{"unobservable", "", true},
		{"stopped", process.StateStopped, false},
		{"vanished", process.StateVanished, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := runRootFixture(t)
			runDir := filepath.Join(root, "0123456789abcdef0123456789abcdef")
			if err := os.MkdirAll(runDir, 0o700); err != nil {
				t.Fatal(err)
			}
			obs := runRootObserver{}
			if tc.state != "" {
				obs[runDir] = tc.state
			}
			g := &processFinalizeGate{observer: obs}
			doc := gatedrive.DriveDoc{Outcome: gatedrive.HALTED, Cause: "deadline-expired-stop-unproven", RunRoot: root}
			res := g.mapDriveOutcome(context.Background(), LocalGateRequest{}, GateDriveResult{Drive: &doc})
			if res.Outcome != FinalizeGateHalted {
				t.Fatalf("outcome = %q, want halted", res.Outcome)
			}
			if got := dirExists(t, runDir); got != tc.retain {
				t.Fatalf("run dir retained = %v, want %v", got, tc.retain)
			}
			wantFinding := ""
			if tc.retain {
				wantFinding = teardownFindingRunRootRetainedUnsettled
			}
			if res.TeardownFinding != wantFinding {
				t.Fatalf("TeardownFinding = %q, want %q", res.TeardownFinding, wantFinding)
			}
		})
	}
}

// TestFailedStartRetainsRootHoldingLaunchEvidence (change 0446 spec §5 audit): a
// Start that fails with no drive document removes the minted run root ONLY when
// nothing was launched under it. A root holding launch evidence (a run dir the
// process backend created before the failure) is retained and reported; an empty
// root (a pre-launch admission refusal) is still removed so it does not leak.
func TestFailedStartRetainsRootHoldingLaunchEvidence(t *testing.T) {
	empty := runRootFixture(t)
	if retained := removeUnlaunchedGateRunRoot(empty); retained || dirExists(t, empty) {
		t.Fatalf("an empty run root (nothing launched) must be removed; retained=%v", retained)
	}
	launched := runRootFixture(t)
	if err := os.MkdirAll(filepath.Join(launched, "run-1"), 0o700); err != nil {
		t.Fatalf("seed launch evidence: %v", err)
	}
	if retained := removeUnlaunchedGateRunRoot(launched); !retained || !dirExists(t, filepath.Join(launched, "run-1")) {
		t.Fatalf("a run root holding launch evidence must be retained; retained=%v", retained)
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
		// Change 0481's token is deliberately not distinguished: finalize reads it
		// as an unavailable gate.
		string(gatedrive.ErrScopeIdentityMismatch): GateHaltUnavailable,
		// Change 0493: a supervisor death halts the drive (never relaunched);
		// finalize reads it as an unavailable gate and a human re-runs finalize.
		gatedrive.CauseSupervisorDied: GateHaltUnavailable,
		// Any cause the mapping does not distinguish falls through to unavailable.
		"owner-superseded": GateHaltUnavailable,
	}
	for cause, want := range cases {
		if got := mapDriveHaltCause(cause); got != want {
			t.Fatalf("mapDriveHaltCause(%q) = %q, want %q", cause, got, want)
		}
	}
}

// TestMapDriveOutcomeSupervisorDiedIsGateHalted (change 0493): a finalize or
// recertify drive whose supervisor died is a terminal HALTED gate in the
// unavailable class (finalize reports it as gate-halted), and its exited run
// root is removed like any other terminal halt's.
func TestMapDriveOutcomeSupervisorDiedIsGateHalted(t *testing.T) {
	g := &processFinalizeGate{}
	root := runRootFixture(t)
	doc := gatedrive.DriveDoc{Outcome: gatedrive.HALTED, Cause: gatedrive.CauseSupervisorDied, RunRoot: root}
	res := g.mapDriveOutcome(context.Background(), LocalGateRequest{}, GateDriveResult{Drive: &doc})
	if res.Outcome != FinalizeGateHalted || res.HaltCause != GateHaltUnavailable {
		t.Fatalf("supervisor-died = %s/%s, want %s/%s", res.Outcome, res.HaltCause, FinalizeGateHalted, GateHaltUnavailable)
	}
	if dirExists(t, root) {
		t.Fatalf("a supervisor-died terminal must remove its exited run root %q", root)
	}
}

// TestStartForwardsRunFields proves Start carries the run-linkage field
// (RunContext) through to the engine unchanged. No run id is carried (change
// 0491).
func TestStartForwardsRunFields(t *testing.T) {
	eng := &fakeDriveEngine{doc: gatedrive.DriveDoc{Outcome: gatedrive.WAITING}}
	svc := newGateDriveService(eng, 5*time.Minute, "go test ./...", "prov")
	got := svc.Start(GateDriveStartRequest{
		RepoDir:    "/repo",
		Worktree:   "/repo",
		RunContext: "ctx-token",
	})
	if got.Result != ResultApplied {
		t.Fatalf("result = %s, want applied", got.Result)
	}
	if eng.lastStart.RunContext != "ctx-token" {
		t.Fatalf("Start must forward the run context, got %+v", eng.lastStart)
	}
}

// TestStartRequestCarriesOwner proves every start names its owning policy to the
// driver (change 0490): the holder note records it so a busy refusal can name
// the right remedy. The build owner's budgeted path and the finalize owner's
// thin Start both forward it.
func TestStartRequestCarriesOwner(t *testing.T) {
	build, beng, _ := newBudgetTestBuildService(t, 4)
	if got := build.Start(buildStartReq("0490")); got.Result != ResultApplied {
		t.Fatalf("build start = %s (%s), want applied", got.Result, got.Reason)
	}
	if beng.lastStart.Owner != "build" {
		t.Fatalf("a build-owned start must carry owner build, got %q", beng.lastStart.Owner)
	}
	feng := &fakeDriveEngine{doc: gatedrive.DriveDoc{Outcome: gatedrive.WAITING}}
	finalize := newGateDriveService(feng, time.Minute, "go test ./...", "prov")
	finalize.owner = "finalize"
	if got := finalize.Start(GateDriveStartRequest{RepoDir: "/repo", Worktree: "/repo", ChangeID: "0490"}); got.Result != ResultApplied {
		t.Fatalf("finalize start = %s (%s), want applied", got.Result, got.Reason)
	}
	if feng.lastStart.Owner != "finalize" {
		t.Fatalf("a finalize-owned start must carry owner finalize, got %q", feng.lastStart.Owner)
	}
}

// --- Task 6: build-owned drive starts reserve the phase suite-attempt budget ---

// buildEffWithMaxAttempts is the fixture the build-budget tests share: a
// build-owned effective config carrying a resolved build.test_command (so Start
// clears the unresolved-command guard), the observation budget, and the
// build.max_attempts snapshot the reservation enforces.
func buildEffWithMaxAttempts(command string, maxAttempts int) config.Effective {
	eff := config.Effective{}
	eff.GateObservation = config.Value[int]{Value: 30, Provenance: config.Provenance{Layer: config.LayerRepository}}
	eff.Build.TestCommand = config.Value[string]{Value: command, Provenance: config.Provenance{Layer: config.LayerRepository}}
	eff.Build.MaxAttempts = config.Value[int]{Value: maxAttempts, Provenance: config.Provenance{Layer: config.LayerRepository}}
	return eff
}

// newBudgetTestBuildService builds a real BUILD-owned service rooted at a fresh
// temp store dir and swaps in a scriptable fake engine, so a start's budget
// reservation runs against the real durable store while the drive itself never
// launches. It returns the service, the fake engine, and the store dir the test
// re-opens to read usage.
func newBudgetTestBuildService(t *testing.T, maxAttempts int) (*GateDriveService, *fakeDriveEngine, string) {
	t.Helper()
	dir := testsupport.TempDir(t)
	svc, res, reason := NewBuildGateDriveService(dir, "/bin/true", buildEffWithMaxAttempts("go test ./...", maxAttempts))
	if svc == nil {
		t.Fatalf("build constructor must build a service: %s %s", res, reason)
	}
	eng := &fakeDriveEngine{doc: gatedrive.DriveDoc{Outcome: gatedrive.WAITING}}
	svc.engine = eng
	return svc, eng, dir
}

// buildStartReq deliberately carries a req.Phase that is NOT the literal "build"
// (here empty). The reservation must key the budget on the literal "build" phase,
// never req.Phase, so suiteUsage (which queries the "build" phase) sees the charge
// only when the reservation ignores req.Phase — this is what the phase-literal
// mutation probe reddens.
func buildStartReq(changeID string) GateDriveStartRequest {
	return GateDriveStartRequest{
		RepoDir:  "/repo",
		Worktree: "/repo",
		ChangeID: changeID,
		Phase:    "",
	}
}

func suiteUsage(t *testing.T, dir, changeID string) (used, limit int) {
	t.Helper()
	store := gatedrive.OpenStore(dir)
	key := gatedrive.SuiteBudgetKey{RepoIdentity: "/repo", ChangeID: changeID, Phase: "build"}
	u, l, err := store.SuiteBudgetUsage(key)
	if err != nil {
		t.Fatalf("SuiteBudgetUsage(%q): %v", changeID, err)
	}
	return u, l
}

// TestBuildOwnedStartReservesSuiteAttempt proves a build-owned change-scoped start
// reserves one logical full-suite attempt per call up to the snapshotted limit,
// and that the first over-limit start is REFUSED with the exhausted reason without
// ever reaching the engine (no fifth NewDrive) or mutating the spent budget.
func TestBuildOwnedStartReservesSuiteAttempt(t *testing.T) {
	svc, eng, dir := newBudgetTestBuildService(t, 4)
	req := buildStartReq("0421")

	for i := 1; i <= 4; i++ {
		got := svc.Start(req)
		if got.Result != ResultApplied {
			t.Fatalf("start %d: result = %s, want applied (reason=%q)", i, got.Result, got.Reason)
		}
		if used, limit := suiteUsage(t, dir, "0421"); used != i || limit != 4 {
			t.Fatalf("after start %d: usage = (%d,%d), want (%d,4)", i, used, limit, i)
		}
	}
	if eng.startCount != 4 {
		t.Fatalf("four admitted starts must each reach the engine, got %d", eng.startCount)
	}

	// The fifth start is over the budget: refused, no engine call, budget untouched.
	eng.startCount = 0
	got := svc.Start(req)
	if got.Result == ResultApplied || got.Drive != nil {
		t.Fatalf("the fifth start must be refused, got result=%s drive=%v", got.Result, got.Drive)
	}
	if got.Reason != "suite-attempts-exhausted" {
		t.Fatalf("refused reason = %q, want suite-attempts-exhausted", got.Reason)
	}
	if eng.startCount != 0 {
		t.Fatalf("a refused start must not reach the engine (no fifth NewDrive), got %d engine starts", eng.startCount)
	}
	if used, limit := suiteUsage(t, dir, "0421"); used != 4 || limit != 4 {
		t.Fatalf("a refused start must not change the budget, got (%d,%d)", used, limit)
	}
}

// TestBuildStartLimitOne proves build.max_attempts: 1 admits exactly the initial
// run and refuses the second — no repair cycle.
func TestBuildStartLimitOne(t *testing.T) {
	svc, eng, _ := newBudgetTestBuildService(t, 1)
	req := buildStartReq("0421")

	if got := svc.Start(req); got.Result != ResultApplied {
		t.Fatalf("first start must succeed at limit 1, got %s (%s)", got.Result, got.Reason)
	}
	eng.startCount = 0
	got := svc.Start(req)
	if got.Result == ResultApplied || got.Reason != "suite-attempts-exhausted" {
		t.Fatalf("second start at limit 1 must be refused as exhausted, got result=%s reason=%q", got.Result, got.Reason)
	}
	if eng.startCount != 0 {
		t.Fatalf("the refused second start must not reach the engine, got %d", eng.startCount)
	}
}

// TestScopelessBuildStartNotBudgeted proves a build-owned start with NO ChangeID
// (a scopeless ad-hoc drive) is unbudgeted: it succeeds regardless of a spent
// budget for some change, because it is not keyed to any phase budget.
func TestScopelessBuildStartNotBudgeted(t *testing.T) {
	svc, _, dir := newBudgetTestBuildService(t, 1)

	// Spend the whole budget for change 0421.
	if got := svc.Start(buildStartReq("0421")); got.Result != ResultApplied {
		t.Fatalf("first change-scoped start must succeed, got %s", got.Result)
	}
	if got := svc.Start(buildStartReq("0421")); got.Reason != "suite-attempts-exhausted" {
		t.Fatalf("the change budget must be spent, got reason %q", got.Reason)
	}

	// A build-owned start with no ChangeID is never charged: it keeps succeeding.
	for i := 0; i < 3; i++ {
		got := svc.Start(GateDriveStartRequest{RepoDir: "/repo", Worktree: "/repo", Phase: "build"})
		if got.Result != ResultApplied {
			t.Fatalf("scopeless build start %d must be unbudgeted, got %s (%s)", i, got.Result, got.Reason)
		}
	}
	// The scopeless starts created no record for the empty change id.
	if used, limit := suiteUsage(t, dir, ""); used != 0 || limit != 0 {
		t.Fatalf("a scopeless start must reserve nothing, got usage (%d,%d)", used, limit)
	}
}

// TestAdvanceRecoverTakeoverDoNotCharge proves the non-start drive operations —
// advance (observation) and handoff/claim (continuation and ownership transfer) —
// never charge the budget: after one budgeted start, usage stays at exactly 1 no
// matter how many of these resume/transfer calls run.
func TestAdvanceRecoverTakeoverDoNotCharge(t *testing.T) {
	svc, _, dir := newBudgetTestBuildService(t, 4)

	if got := svc.Start(buildStartReq("0421")); got.Result != ResultApplied {
		t.Fatalf("the initial start must be applied, got %s", got.Result)
	}
	if used, _ := suiteUsage(t, dir, "0421"); used != 1 {
		t.Fatalf("one start reserves exactly one attempt, got used=%d", used)
	}

	// None of these consult or charge the budget.
	svc.Advance("d1", "gen")
	svc.Advance("d1", "gen")
	svc.Handoff("d1", "gen")
	svc.Claim("d1", "handoff")

	if used, limit := suiteUsage(t, dir, "0421"); used != 1 || limit != 4 {
		t.Fatalf("advance/handoff/claim must charge nothing, got usage (%d,%d)", used, limit)
	}
}

// TestExhaustionDiagnosticNamesKnob proves the exhaustion refusal's human text
// names the knob (build.max_attempts) and the used/limit fraction so a halting
// worker can report the actionable diagnostic.
func TestExhaustionDiagnosticNamesKnob(t *testing.T) {
	svc, _, _ := newBudgetTestBuildService(t, 4)
	req := buildStartReq("0421")
	for i := 0; i < 4; i++ {
		if got := svc.Start(req); got.Result != ResultApplied {
			t.Fatalf("start %d must be applied, got %s", i, got.Result)
		}
	}
	got := svc.Start(req)
	human := got.HumanText()
	if !strings.Contains(human, "build.max_attempts") {
		t.Fatalf("exhaustion human text must name build.max_attempts, got %q", human)
	}
	if !strings.Contains(human, "4/4") {
		t.Fatalf("exhaustion human text must carry the used/limit fraction 4/4, got %q", human)
	}
}

// --- Task 8: worktree admission precedes full-suite attempt charging (ADR-0116) ---

// TestBusyRefusalChargesNoSuiteAttempt proves a build-owned start refused by a
// busy worktree lock charges NO suite attempt and surfaces the full refusal: the
// reason, the worktree-admission stage, and — with no live holder named — an
// empty locator and the holder-unknown remedy. The refusal is the store's own
// (another gate holds the lock in the SAME durable store the service charges
// against); the lock is taken before the charge, so moving the charge before
// Admit reddens this test.
func TestBusyRefusalChargesNoSuiteAttempt(t *testing.T) {
	svc, eng, dir := newBudgetTestBuildService(t, 4)
	store := gatedrive.OpenStore(dir)
	root := testsupport.TempDir(t)
	held, err := store.TryWorktreeLock(root, nil)
	if err != nil {
		t.Fatalf("hold the worktree lock: %v", err)
	}
	defer held.Release()
	_, busy := store.TryWorktreeLock(root, nil)
	if oe, ok := gatedrive.AsOwnershipError(busy); !ok || oe.Kind != gatedrive.ErrWorktreeBusy {
		t.Fatalf("a second try of a held lock = %v, want worktree-busy", busy)
	}
	eng.admitErr = busy

	got := svc.Start(GateDriveStartRequest{RepoDir: "/repo", Worktree: root, Cwd: root, ChangeID: "0421"})
	if got.Result == ResultApplied || got.Drive != nil {
		t.Fatalf("a busy worktree must refuse the build start, got result=%s drive=%v", got.Result, got.Drive)
	}
	if got.Reason != string(gatedrive.ErrWorktreeBusy) || got.Stage != stageWorktreeAdmission || got.Locator != "" {
		t.Fatalf("busy refusal = reason %q stage %q locator %q, want worktree-busy / %s / empty", got.Reason, got.Stage, got.Locator, stageWorktreeAdmission)
	}
	if !strings.Contains(got.Message, "holder unknown") {
		t.Fatalf("a holder-less busy refusal must say the holder is unknown, got %q", got.Message)
	}
	if used, limit := suiteUsage(t, dir, "0421"); used != 0 || limit != 0 {
		t.Fatalf("a busy-worktree refusal must charge no suite attempt, got usage (%d,%d)", used, limit)
	}
	if eng.startCount != 1 || eng.startAdmittedCount != 0 {
		t.Fatalf("admit=%d launch=%d, want the refusal at admission and no launch", eng.startCount, eng.startAdmittedCount)
	}
}

// TestAdmitRefusalChargesNoSuiteAttempt proves the ordering: when Admit refuses
// worktree-busy, the start is refused with no suite attempt charged, because the
// charge sits AFTER Admit; Admit is reached exactly once (there is no advisory
// busy pre-check before it) and the launch half is never reached. Moving the
// charge before Admit reddens this test.
func TestAdmitRefusalChargesNoSuiteAttempt(t *testing.T) {
	svc, eng, dir := newBudgetTestBuildService(t, 4)
	eng.admitErr = &gatedrive.OwnershipError{Kind: gatedrive.ErrWorktreeBusy, Op: "worktree-admission"}

	got := svc.Start(buildStartReq("0421"))
	if got.Result == ResultApplied || got.Drive != nil {
		t.Fatalf("an Admit worktree-busy refusal must refuse the start, got result=%s", got.Result)
	}
	if got.Reason != string(gatedrive.ErrWorktreeBusy) {
		t.Fatalf("refusal reason = %q, want %q", got.Reason, string(gatedrive.ErrWorktreeBusy))
	}
	if eng.startCount != 1 {
		t.Fatalf("Admit must be reached exactly once, got %d", eng.startCount)
	}
	if eng.startAdmittedCount != 0 {
		t.Fatalf("a refused admission must never launch, got %d StartAdmitted calls", eng.startAdmittedCount)
	}
	if used, limit := suiteUsage(t, dir, "0421"); used != 0 || limit != 0 {
		t.Fatalf("an Admit refusal must charge no suite attempt, got usage (%d,%d)", used, limit)
	}
}

// TestAdmittedLaunchFailureStillCharges proves the other half of ADR-0116: once an
// admitted start has charged its attempt, a launch/persistence failure in the
// launch half is NOT refunded. Admission succeeds, the charge lands, StartAdmitted
// returns a launch error, and the usage stays at one — no abandon, no refund.
func TestAdmittedLaunchFailureStillCharges(t *testing.T) {
	svc, eng, dir := newBudgetTestBuildService(t, 4)
	eng.doc = gatedrive.DriveDoc{}
	eng.err = errors.New("gatedrive: start launch: boom")

	got := svc.Start(buildStartReq("0421"))
	if got.Result == ResultApplied {
		t.Fatalf("a launch failure is a command failure, got applied")
	}
	if used, limit := suiteUsage(t, dir, "0421"); used != 1 || limit != 4 {
		t.Fatalf("an admitted-then-failed launch keeps its charge (no refund), got usage (%d,%d)", used, limit)
	}
	if eng.startCount != 1 || eng.startAdmittedCount != 1 {
		t.Fatalf("the start must admit then launch exactly once, got admit=%d launch=%d", eng.startCount, eng.startAdmittedCount)
	}
	if eng.abandonCount != 0 {
		t.Fatalf("a charged, admitted start must not abandon its admission, got %d", eng.abandonCount)
	}
}

// TestCancellationDoesNotCharge is the Task-10 placeholder: observation,
// continuation, and transfer never pass through admission, so they never charge the
// suite budget. Cancellation (Task 10) rests on this — it drives these paths and
// must reset no accounting.
func TestCancellationDoesNotCharge(t *testing.T) {
	svc, _, dir := newBudgetTestBuildService(t, 4)

	svc.Advance("d1", "gen")
	svc.Advance("d1", "gen")
	svc.Handoff("d1", "gen")
	svc.Claim("d1", "h")

	if used, limit := suiteUsage(t, dir, "0421"); used != 0 || limit != 0 {
		t.Fatalf("advance/handoff/claim must charge nothing, got usage (%d,%d)", used, limit)
	}
}

// TestMapDriveFailureOwnershipKinds proves mapDriveFailure surfaces every known
// OwnershipError as its typed reason token — never collapsing to the generic
// invalid-request — while an unrecognized error still falls through to
// invalid-request. It also proves the reason is the bounded kind token alone: the
// wrapped error's free text (a stand-in for argv/env/path) never leaks into the
// reason.
func TestMapDriveFailureOwnershipKinds(t *testing.T) {
	kinds := []gatedrive.OwnershipErrorKind{
		gatedrive.ErrScopeIdentityMismatch,
		gatedrive.ErrScopeCapabilityMismatch,
		gatedrive.ErrScopeClosed,
		gatedrive.ErrHandoffOutstanding,
		gatedrive.ErrUnresolvedLaunchTransition,
		// The worktree-admission ownership kind (change 0490's worktree lock).
		gatedrive.ErrWorktreeBusy,
		// A launch cwd outside any git worktree: refused, never internal.
		gatedrive.ErrWorktreeUnresolved,
	}
	const secret = "SECRET-ARGV"
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			// Wrap the ownership error in free text that must NOT reach the reason.
			wrapped := fmt.Errorf("launch failed with %s: %w", secret, &gatedrive.OwnershipError{Kind: kind, Op: "start"})
			res, reason := mapDriveFailure(wrapped)
			if res != ResultInvalidInput {
				t.Fatalf("ownership error result = %s, want invalid-input", res)
			}
			if reason != string(kind) {
				t.Fatalf("reason = %q, want the typed kind token %q (never collapsed to invalid-request)", reason, string(kind))
			}
			if strings.Contains(reason, secret) {
				t.Fatalf("reason must not leak the wrapped error text %q, got %q", secret, reason)
			}
		})
	}

	// A plain unrecognized error still maps to the generic invalid-request.
	res, reason := mapDriveFailure(errors.New("boom"))
	if res != ResultInvalidInput || reason != "invalid-request" {
		t.Fatalf("unrecognized error must map to invalid-request, got result=%s reason=%q", res, reason)
	}
}

// TestMapDriveFailureOwnershipNextAction proves a command failure classified as an
// ownership error carries a non-empty valid-next-action message the caller can act
// on, distinct per kind, while the reason stays the bounded kind token. A plain
// store failure carries no such next-action message.
func TestMapDriveFailureOwnershipNextAction(t *testing.T) {
	const secret = "SECRET-TOKEN-deadbeefdeadbeef"
	seen := map[string]string{}
	for _, kind := range []gatedrive.OwnershipErrorKind{
		gatedrive.ErrHandoffOutstanding,
		gatedrive.ErrUnresolvedLaunchTransition,
		// The worktree-admission ownership kind — it MUST carry its own distinct
		// next-action message.
		gatedrive.ErrWorktreeBusy,
		gatedrive.ErrWorktreeUnresolved,
	} {
		// Wrap the ownership error in credential-shaped free text (a stand-in for a
		// reservation token / argv) that must reach NEITHER the reason NOR the message.
		wrapped := fmt.Errorf("launch failed carrying %s: %w", secret, &gatedrive.OwnershipError{Kind: kind, Op: "start"})
		eng := &fakeDriveEngine{err: wrapped}
		svc := newGateDriveService(eng, 0, "", "")
		got := svc.Advance("d1", "owner")
		if got.Result != ResultInvalidInput {
			t.Fatalf("kind %v result = %s, want invalid-input", kind, got.Result)
		}
		if got.Reason != string(kind) {
			t.Fatalf("kind %v reason = %q, want %q", kind, got.Reason, string(kind))
		}
		if got.Message == "" {
			t.Fatalf("kind %v must carry a valid-next-action message", kind)
		}
		// Redaction: neither the reason token nor the next-action message may leak the
		// wrapped credential (Global Constraint: diagnostics never include a token).
		if strings.Contains(got.Message, secret) || strings.Contains(got.Reason, secret) {
			t.Fatalf("kind %v leaked the wrapped credential; reason=%q message=%q", kind, got.Reason, got.Message)
		}
		if prev, ok := seen[got.Message]; ok {
			t.Fatalf("next-action message %q is shared by kinds %v and %v; each state gets its own action", got.Message, prev, kind)
		}
		seen[got.Message] = string(kind)
		// The scopeless caller has no parent, no identity bundle, and no takeover
		// path (change 0489): a message may not send it to any of them.
		for _, retired := range []string{"parent", "identity bundle", "dispatch prompt", "take over", "taking over"} {
			if strings.Contains(strings.ToLower(got.Message), retired) {
				t.Fatalf("kind %v next-action message names the retired %q recovery path: %q", kind, retired, got.Message)
			}
		}
	}

	// A store command failure is not an ownership error: no next-action message.
	eng := &fakeDriveEngine{err: &gatedrive.StoreError{Kind: gatedrive.ErrNotFound, Op: "resolve"}}
	svc := newGateDriveService(eng, 0, "", "")
	if got := svc.Advance("d1", "owner"); got.Message != "" {
		t.Fatalf("a store failure must carry no ownership next-action message, got %q", got.Message)
	}
}

// TestMapDriveFailureFenceReasons proves the run mutation-fence refusal
// (MutationFenceError) is classified through the SAME shared mapDriveFailure
// classifier into (invalid-input, <its bounded, stable token>) — run-cancelled /
// run-superseded — and that a wrapped credential leaks into neither the reason nor
// the message. It is the fail-safe path for a fenced-run error that ever chains
// through the gate-drive seam (no gate-drive path raises one since change 0491).
func TestMapDriveFailureFenceReasons(t *testing.T) {
	const secret = "SECRET-TOKEN-cafebabecafebabe"
	for _, tc := range []struct {
		name string
		err  *MutationFenceError
	}{
		{"run-cancelled", ErrRunCancelled},
		{"run-superseded", ErrRunSuperseded},
	} {
		wrapped := fmt.Errorf("mutation refused carrying %s: %w", secret, tc.err)
		res, reason := mapDriveFailure(wrapped)
		if res != ResultInvalidInput {
			t.Fatalf("%s result = %s, want invalid-input", tc.name, res)
		}
		if reason != tc.name {
			t.Fatalf("%s reason = %q, want the stable fence token %q", tc.name, reason, tc.name)
		}
		if strings.Contains(reason, secret) {
			t.Fatalf("%s reason leaked the wrapped credential: %q", tc.name, reason)
		}
		// The service surfaces the same bounded reason through the same seam.
		eng := &fakeDriveEngine{err: wrapped}
		got := newGateDriveService(eng, 0, "", "").Advance("d1", "owner")
		if got.Reason != tc.name {
			t.Fatalf("%s service reason = %q, want %q", tc.name, got.Reason, tc.name)
		}
		if strings.Contains(got.Message, secret) || strings.Contains(got.HumanText(), secret) {
			t.Fatalf("%s leaked the wrapped credential: message=%q human=%q", tc.name, got.Message, got.HumanText())
		}
	}
}

// validDriveIDForTest returns a literal that satisfies gatedrive.ValidDriveID
// (32 lowercase hex chars), matching the shape the legacy-inventory tests use.
func validDriveIDForTest(t *testing.T) string {
	t.Helper()
	id := strings.Repeat("a", 32)
	if !gatedrive.ValidDriveID(id) {
		t.Fatalf("fixture drive id %q does not validate", id)
	}
	return id
}

// ownershipErrWith crafts a worktree-admission OwnershipError carrying (or not)
// an incumbent snapshot, so the mapping tests exercise mapDriveResult's
// admission-refusal branch directly.
func ownershipErrWith(kind gatedrive.OwnershipErrorKind, inc *gatedrive.IncumbentSnapshot) *gatedrive.OwnershipError {
	return &gatedrive.OwnershipError{Kind: kind, Op: "reserve-worktree-execution", Incumbent: inc}
}

// TestMapDriveResultWorktreeAdmissionRefusal proves a worktree-busy refusal
// always carries the worktree-admission stage, a safe locator for a live
// holder, and the remedy for that holder's kind (change 0490): run.cancel for a
// build drive's owning run, waiting for finalize's gate, gate stop for a raw
// launch, and "holder unknown" (lsof on busy.lock) when no running holder could
// be named. Identities render only after they validate, and a non-busy kind
// keeps its own next-action message with no stage.
func TestMapDriveResultWorktreeAdmissionRefusal(t *testing.T) {
	driveID := validDriveIDForTest(t)
	const runID = "0123456789abcdef0123456789abcdef"
	buildInc := &gatedrive.IncumbentSnapshot{Kind: "drive", DriveID: driveID, RawRunID: runID,
		RawRunDir: "/runs/" + runID, ChangeID: "0490", Owner: "build"}
	finalizeInc := &gatedrive.IncumbentSnapshot{Kind: "drive", DriveID: driveID, RawRunID: runID,
		RawRunDir: "/runs/" + runID, ChangeID: "0490", Owner: "finalize"}
	rawInc := &gatedrive.IncumbentSnapshot{Kind: "raw", RawRunID: runID, RawRunDir: "/runs/" + runID, Owner: "raw"}
	rawBadInc := &gatedrive.IncumbentSnapshot{Kind: "raw", RawRunID: "NOT-HEX", RawRunDir: "/runs/NOT-HEX", Owner: "raw"}
	driveBadInc := &gatedrive.IncumbentSnapshot{Kind: "drive", DriveID: "../escape", ChangeID: "0490; rm -rf /", Owner: "build"}

	cases := []struct {
		name      string
		err       *gatedrive.OwnershipError
		wantStage string
		wantLoc   string
		msgHas    []string
		msgLacks  []string
	}{
		{"busy with a build drive holder", ownershipErrWith(gatedrive.ErrWorktreeBusy, buildInc),
			"worktree-admission", "incumbent-drive:" + driveID,
			[]string{"change 0490", "drive " + driveID, "run.cancel", "--key <key> --reason <why>"},
			[]string{"gate stop", "holder unknown", "--run-id"}},
		{"busy with a finalize holder", ownershipErrWith(gatedrive.ErrWorktreeBusy, finalizeInc),
			"worktree-admission", "incumbent-drive:" + driveID,
			[]string{"finalize", "change 0490", "wait"},
			[]string{"run.cancel", "gate stop", "holder unknown"}},
		{"busy with a raw holder", ownershipErrWith(gatedrive.ErrWorktreeBusy, rawInc),
			"worktree-admission", "incumbent-run:" + runID,
			[]string{"gate stop '/runs/" + runID + "'", "raw gate run"},
			[]string{"run.cancel", "holder unknown"}},
		{"busy with no holder", ownershipErrWith(gatedrive.ErrWorktreeBusy, nil),
			"worktree-admission", "",
			[]string{"holder unknown", "lsof", "busy.lock"},
			[]string{"run.cancel", "gate stop"}},
		{"busy with a raw holder whose run id does not validate", ownershipErrWith(gatedrive.ErrWorktreeBusy, rawBadInc),
			"worktree-admission", "",
			[]string{"raw gate run", "did not validate"},
			[]string{"gate stop", "NOT-HEX"}},
		{"busy with a drive holder whose ids do not validate", ownershipErrWith(gatedrive.ErrWorktreeBusy, driveBadInc),
			"worktree-admission", "",
			[]string{"another change", "run.cancel"},
			[]string{"../escape", "rm -rf", "(drive"}},
		{"a non-busy kind keeps its next action", ownershipErrWith(gatedrive.ErrHandoffOutstanding, buildInc),
			"", "", []string{ownershipNextAction(gatedrive.ErrHandoffOutstanding)}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapDriveResult(OperationGateDriveStart, gatedrive.DriveDoc{}, tc.err)
			if got.Reason != string(tc.err.Kind) {
				t.Fatalf("reason = %q, want %q", got.Reason, tc.err.Kind)
			}
			if got.Stage != tc.wantStage || got.Locator != tc.wantLoc {
				t.Fatalf("stage/locator = %q/%q, want %q/%q", got.Stage, got.Locator, tc.wantStage, tc.wantLoc)
			}
			for _, s := range tc.msgHas {
				if !strings.Contains(got.Message, s) {
					t.Fatalf("message %q lacks %q", got.Message, s)
				}
			}
			for _, s := range tc.msgLacks {
				if strings.Contains(got.Message, s) {
					t.Fatalf("message %q must not contain %q", got.Message, s)
				}
			}
		})
	}
}

// TestIncumbentRefusalLocatorValidatesIDs proves an invalid drive/run id
// collapses to "" rather than rendering arbitrary bytes into the locator.
func TestIncumbentRefusalLocatorValidatesIDs(t *testing.T) {
	bad := &gatedrive.IncumbentSnapshot{DriveID: "../escape"}
	if got := incumbentRefusalLocator(bad); got != "" {
		t.Fatalf("invalid drive id rendered locator %q", got)
	}
	badRun := &gatedrive.IncumbentSnapshot{RawRunID: "NOT-HEX"}
	if got := incumbentRefusalLocator(badRun); got != "" {
		t.Fatalf("invalid run id rendered locator %q", got)
	}
}

// TestQuoteOperand pins the shell-safe quoting of incumbent paths in guidance.
func TestQuoteOperand(t *testing.T) {
	if got := quoteOperand(`/tmp/o'brien`); got != `'/tmp/o'\''brien'` {
		t.Fatalf("quoteOperand = %q", got)
	}
}
