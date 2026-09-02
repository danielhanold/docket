package app

import (
	"context"
	"errors"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"strings"
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
	doc          gatedrive.DriveDoc
	err          error
	lastStart    gatedrive.StartRequest
	startCalled  bool
	grant        gatedrive.ScopeGrant
	grantErr     error
	lastScopeReq gatedrive.ScopeRequest
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
func (f *fakeDriveEngine) Takeover(scopeID, parentCap, driveID string) (gatedrive.DriveDoc, error) {
	return f.doc, f.err
}
func (f *fakeDriveEngine) PrepareScope(r gatedrive.ScopeRequest) (gatedrive.ScopeGrant, error) {
	f.lastScopeReq = r
	return f.grant, f.grantErr
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
	d, err := os.MkdirTemp(testsupport.TempDir(t), "runroot-*")
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

// --- Task 3: prepare-scope, takeover, and the task-intent owner ------------

// TestPrepareScopeHumanTextRedactsCapabilities proves the grant travels in the
// JSON document (all three fields) while the human text carries ONLY the scope
// id — never either capability token.
func TestPrepareScopeHumanTextRedactsCapabilities(t *testing.T) {
	eng := &fakeDriveEngine{grant: gatedrive.ScopeGrant{
		ScopeID:          "scope-123",
		ChildCapability:  "childcapsecret",
		ParentCapability: "parentcapsecret",
	}}
	svc := newGateDriveService(eng, 0, "", "")
	got := svc.PrepareScope(gatedrive.ScopeRequest{ChangeID: "0359"})
	if got.Result != ResultApplied {
		t.Fatalf("prepare-scope result = %s, want applied", got.Result)
	}
	if got.Operation != OperationGateDrivePrepareScope {
		t.Fatalf("operation = %q, want %q", got.Operation, OperationGateDrivePrepareScope)
	}
	if got.ScopeID != "scope-123" || got.ChildCapability != "childcapsecret" || got.ParentCapability != "parentcapsecret" {
		t.Fatalf("JSON document must carry all three grant fields, got %+v", got)
	}
	if eng.lastScopeReq.ChangeID != "0359" {
		t.Fatalf("PrepareScope must forward the request, got %+v", eng.lastScopeReq)
	}
	human := got.HumanText()
	if !strings.Contains(human, "scope-123") {
		t.Fatalf("human text must name the scope id, got %q", human)
	}
	if strings.Contains(human, "childcapsecret") || strings.Contains(human, "parentcapsecret") {
		t.Fatalf("human text must NOT carry any capability, got %q", human)
	}
}

// TestTaskServiceForcesNonIdempotent proves the task-intent owner forces
// IdempotentSuiteGate false regardless of the request, passes the agent-supplied
// argv through VERBATIM (no /bin/sh -c wrapping), and records the fixed task
// provenance.
func TestTaskServiceForcesNonIdempotent(t *testing.T) {
	argv := []string{"go", "test", "-run", "Focus", "./internal/app/"}
	eff := config.Effective{GateObservation: config.Value[int]{Value: 30, Provenance: config.Provenance{Layer: config.LayerRepository}}}
	svc, res, reason := NewTaskGateDriveService(t.TempDir(), "/bin/true", eff, argv)
	if svc == nil {
		t.Fatalf("task constructor must build a service: %s %s", res, reason)
	}
	eng := &fakeDriveEngine{doc: gatedrive.DriveDoc{Outcome: gatedrive.WAITING}}
	svc.engine = eng
	got := svc.Start(GateDriveStartRequest{RepoDir: "/repo", Worktree: "/repo", IdempotentSuiteGate: true})
	if got.Result != ResultApplied {
		t.Fatalf("a WAITING start is an applied operation, got %s", got.Result)
	}
	if eng.lastStart.IdempotentSuiteGate {
		t.Fatalf("task-intent Start must force IdempotentSuiteGate false")
	}
	if len(eng.lastStart.Command) != len(argv) {
		t.Fatalf("task argv must pass through verbatim, got %v", eng.lastStart.Command)
	}
	for i := range argv {
		if eng.lastStart.Command[i] != argv[i] {
			t.Fatalf("argv[%d] = %q, want %q", i, eng.lastStart.Command[i], argv[i])
		}
	}
	if eng.lastStart.ConfigProvenance != "task.argv=agent-supplied" {
		t.Fatalf("task provenance = %q, want task.argv=agent-supplied", eng.lastStart.ConfigProvenance)
	}
}

// TestTaskServiceRequiresArgv proves an empty argv fails closed at construction
// with ResultInvalidInput and the stable missing-argv reason — never a service
// that could Start an empty command.
func TestTaskServiceRequiresArgv(t *testing.T) {
	eff := config.Effective{GateObservation: config.Value[int]{Value: 30, Provenance: config.Provenance{Layer: config.LayerRepository}}}
	svc, res, reason := NewTaskGateDriveService(t.TempDir(), "/bin/true", eff, nil)
	if svc != nil {
		t.Fatalf("empty argv must not build a service")
	}
	if res != ResultInvalidInput {
		t.Fatalf("empty argv result = %s, want invalid-input", res)
	}
	if reason != "missing-argv" {
		t.Fatalf("reason = %q, want missing-argv", reason)
	}
}

// TestTaskServiceResolvesObservationBudget is the regression for the Task-12
// defect: the task-intent constructor USED to hardcode a zero observation budget,
// which fixed the drive's deadline at start so any focused test caught running
// even once HALTed deadline-expired instead of WAITING for its result. The budget
// must instead resolve from authoritative config (gate_observation_budget,
// minutes) exactly as the build/finalize owner constructors do, so a running child
// stays in-window. A zero budget would still yield an at-start deadline; a resolved
// 30-minute default yields a deadline strictly after start, and Start must inject
// that resolved budget into the engine request.
func TestTaskServiceResolvesObservationBudget(t *testing.T) {
	argv := []string{"go", "test", "-run", "Focus", "./internal/app/"}
	eff := config.Effective{GateObservation: config.Value[int]{Value: 30, Provenance: config.Provenance{Layer: config.LayerRepository}}}
	svc, res, reason := NewTaskGateDriveService(t.TempDir(), "/bin/true", eff, argv)
	if svc == nil {
		t.Fatalf("task constructor must build a service: %s %s", res, reason)
	}
	if svc.budget != 30*time.Minute {
		t.Fatalf("task budget must resolve from gate_observation_budget minutes, got %v (a zero budget HALTs a running focused test)", svc.budget)
	}
	// The resolved budget must actually reach the engine's Start request — a budget
	// held on the service but not injected would leave the drive at a zero deadline.
	eng := &fakeDriveEngine{doc: gatedrive.DriveDoc{Outcome: gatedrive.WAITING}}
	svc.engine = eng
	if got := svc.Start(GateDriveStartRequest{RepoDir: "/repo", Worktree: "/repo"}); got.Result != ResultApplied {
		t.Fatalf("a WAITING start is an applied operation, got %s", got.Result)
	}
	if eng.lastStart.Budget != 30*time.Minute {
		t.Fatalf("task Start must inject the resolved non-zero budget, got %v", eng.lastStart.Budget)
	}
}

// TestStartForwardsScopeFields proves Start carries the scope-binding fields
// (ScopeID, ChildCapability, GateContext) through to the engine unchanged.
func TestStartForwardsScopeFields(t *testing.T) {
	eng := &fakeDriveEngine{doc: gatedrive.DriveDoc{Outcome: gatedrive.WAITING}}
	svc := newGateDriveService(eng, 5*time.Minute, "go test ./...", "prov")
	got := svc.Start(GateDriveStartRequest{
		RepoDir:         "/repo",
		Worktree:        "/repo",
		ScopeID:         "sc-1",
		ChildCapability: "childcap",
		GateContext:     "ctx-token",
	})
	if got.Result != ResultApplied {
		t.Fatalf("result = %s, want applied", got.Result)
	}
	if eng.lastStart.ScopeID != "sc-1" || eng.lastStart.ChildCapability != "childcap" || eng.lastStart.GateContext != "ctx-token" {
		t.Fatalf("Start must forward the scope fields, got %+v", eng.lastStart)
	}
}

// TestTakeoverMapsDoc proves Takeover delegates to the engine, carries a
// successful document verbatim under the takeover operation name, and maps a
// command failure through the shared mapDriveFailure classifier.
func TestTakeoverMapsDoc(t *testing.T) {
	eng := &fakeDriveEngine{doc: gatedrive.DriveDoc{Outcome: gatedrive.PASSED, DriveID: "d1", Generation: "freshgen"}}
	svc := newGateDriveService(eng, 0, "", "")
	got := svc.Takeover("sc-1", "parentcap", "d1")
	if got.Result != ResultApplied || got.Drive == nil || got.Drive.Outcome != gatedrive.PASSED {
		t.Fatalf("takeover success must be an applied result carrying the doc, got %s", got.Result)
	}
	if got.Operation != OperationGateDriveTakeover {
		t.Fatalf("operation = %q, want %q", got.Operation, OperationGateDriveTakeover)
	}

	bad := &fakeDriveEngine{err: &gatedrive.StoreError{Kind: gatedrive.ErrNotFound, Op: "resolve"}}
	svc2 := newGateDriveService(bad, 0, "", "")
	got2 := svc2.Takeover("sc-x", "parentcap", "dx")
	if got2.Result != ResultInvalidInput || got2.Drive != nil || got2.Reason == "" {
		t.Fatalf("takeover command failure must map like the other ops, got result=%s reason=%q", got2.Result, got2.Reason)
	}
}
