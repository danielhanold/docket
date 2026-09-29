//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
)

// These are the `docket run start` (start the run tracker) tests (change 0334,
// Task 2). run start re-syncs the metadata worktree to fresh origin, reads the
// in-progress claim set, captures a dispatch time AFTER that read, and mints a
// durable gate record — printing `run-started <key>` on success and
// `run-untracked <reason-token>` on any failure, exiting 0 either way (the report
// line is the contract). Only `implement-next` is an accepted target; anything
// else is a usage error that exits non-zero.
//
// The in-progress read reuses the same PinContext/ReadCorpus plumbing the claim
// path uses (a fresh-origin fetch inside PinContext, one change-file parse), so
// these tests inject the scriptable fakeReader for the corpus and a real temp
// git repo (newRunTrackerRepo) for the store's git-common-dir rooting.

// runStartCorpus is a scriptable in-progress claim set: ids 3 and 7 are
// in-progress (the before-set), id 5 is proposed and id 8 is blocked (neither is
// an in-progress claim), so a correct before-read collects exactly {3, 7}.
func runStartCorpus() []StatusBlob {
	blob := func(id int, slug, status string) StatusBlob {
		return StatusBlob{
			Kind:     repository.KindChange,
			Location: repository.LocationActive,
			Path:     groomPath(id, slug),
			Version:  miVersion,
			Data:     []byte(lifecycleChange(id, slug, status)),
		}
	}
	return []StatusBlob{
		blob(3, "alpha", "in-progress"),
		blob(7, "bravo", "in-progress"),
		blob(5, "charlie", "proposed"),
		blob(8, "delta", "blocked"),
	}
}

// runStartReader is the fakeReader wired with a mainPin and the given corpus /
// injected errors.
func runStartReader(t *testing.T, corpus []StatusBlob, pinErr, corpusErr error) *fakeReader {
	t.Helper()
	return &fakeReader{pin: mainPin(t), corpus: corpus, pinErr: pinErr, corpusErr: corpusErr}
}

// --- outer-scope preparation fake (change 0359) ---------------------------

// scopeParentCap / scopeChildCap / scopeID are the fake grant's distinct tokens:
// distinct so a redaction assert cannot pass by confusing one for another, and
// child != parent so the printed run context is provably the child alone.
const (
	scopeGrantID     = "scope-abc123"
	scopeGrantChild  = "childctx-1111"
	scopeGrantParent = "parentcap-secret-9999"
)

// sampleScopeGrant is the canned grant the fake scope-prep returns.
func sampleScopeGrant() gatedrive.ScopeGrant {
	return gatedrive.ScopeGrant{
		ScopeID:          scopeGrantID,
		ChildCapability:  scopeGrantChild,
		ParentCapability: scopeGrantParent,
	}
}

// fakeScopePrep is a scriptable RunTrackerScopeDeps.Prepare: it records the request it
// was handed and returns a canned grant (or a canned error), so a test can prove
// the scope was prepared with the right ChangeID/Branch/Worktree and was NOT
// prepared on a pre-scope refusal.
type fakeScopePrep struct {
	grant gatedrive.ScopeGrant
	err   error
	calls int
	req   gatedrive.ScopeRequest
}

func (f *fakeScopePrep) deps() RunTrackerScopeDeps {
	return RunTrackerScopeDeps{
		Prepare: func(req gatedrive.ScopeRequest) (gatedrive.ScopeGrant, error) {
			f.calls++
			f.req = req
			if f.err != nil {
				return gatedrive.ScopeGrant{}, f.err
			}
			return f.grant, nil
		},
		// A permissive cancellation seam so every resume test that reaches the
		// RunCancelled/RunSuperseded branches sees a quiescent old run (change
		// 0435): accounted launches, no worktree slot. Slot-bearing tests override
		// CancelSeams with a real store. A nil store makes validateResumeQuiescence's
		// slot leg vacuous, which is correct for fixtures that bind no worktree slot.
		CancelSeams: func(string) cancelSeams {
			return cancelSeams{launches: okLaunchReconciler()}
		},
	}
}

// resumeInspectService is a fakeWorkspaceService that inspects a resume target to
// StateReady at a known worktree path, so WorkspaceInspect returns ResultApplied.
func resumeInspectService(worktree string) *fakeWorkspaceService {
	return &fakeWorkspaceService{
		inspection: workspace.Inspection{
			Kind:       workspace.StateReady,
			Path:       worktree,
			HeadCommit: gitcli.ObjectID(evidenceHead),
		},
	}
}

// TestIntegrationRunStartPreparesOuterScope: a non-resume start prepares the outer scope,
// carries the scope binding in the record, prints the run context on the
// started line, and NEVER leaks the parent capability into the result JSON or the
// human text (it lives only in the 0600 record).
func TestIntegrationRunStartPreparesOuterScope(t *testing.T) {
	repo := newRunTrackerRepo(t)
	deps := PlanningDeps{Reader: runStartReader(t, runStartCorpus(), nil, nil), Clock: testClock()}
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunStart(context.Background(), deps, WorkspaceDeps{}, sp.deps(), repo, "implement-next", 0)
	if !res.Started || res.Key == "" {
		t.Fatalf("Started=%v Key=%q, want started", res.Started, res.Key)
	}
	if sp.calls != 1 {
		t.Fatalf("PrepareScope called %d times, want 1", sp.calls)
	}
	// A fresh (non-resume) outer scope binds no change id yet and carries no
	// resumed identity.
	if sp.req.ChangeID != "" || sp.req.Branch != "" || sp.req.Worktree != "" {
		t.Errorf("fresh scope request carried identity: %+v", sp.req)
	}
	// Started line: run-started <key> <run-id> <run-context>, followed by the
	// honest owner-lifecycle caveat (change 0375 Task 13). The run id is minted
	// beside the run-tracker record and surfaced so the Stop path is followable.
	ep, _, err := LoadRunRecord(repo, res.Key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if got, want := res.HumanText(), "run-started "+res.Key+" "+ep.RunID+" "+scopeGrantChild+"\n"+ReasonOwnerLifecycleUnavailable; got != want {
		t.Errorf("HumanText = %q, want %q", got, want)
	}
	if res.RunContext != scopeGrantChild {
		t.Errorf("RunContext = %q, want %q", res.RunContext, scopeGrantChild)
	}

	rec, err := LoadRunTrackerRecord(repo, res.Key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	if rec.ScopeID != scopeGrantID {
		t.Errorf("ScopeID = %q, want %q", rec.ScopeID, scopeGrantID)
	}
	if rec.ParentCap != scopeGrantParent {
		t.Errorf("ParentCap = %q, want the raw parent cap persisted in the 0600 record", rec.ParentCap)
	}
	if want := runTrackerHashToken(scopeGrantChild); rec.ChildContextHash != want {
		t.Errorf("ChildContextHash = %q, want sha256 of the run context %q", rec.ChildContextHash, want)
	}

	// Redaction: the parent capability must never appear in the marshalled result
	// JSON or in the human text.
	blob, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(blob), scopeGrantParent) {
		t.Errorf("result JSON leaks the parent capability:\n%s", blob)
	}
	if strings.Contains(res.HumanText(), scopeGrantParent) {
		t.Errorf("HumanText leaks the parent capability: %q", res.HumanText())
	}
}

// TestIntegrationRunStartFreshStartSurfacesRunID: a fresh (non-resume) start surfaces the
// minted run's public id in the result (Run) and in the human report line
// — the documented `run.cancel --run-id <id>` / `--run-id` value the operator and
// the dispatcher thread through. Without it the primary human-Stop path names an
// run the start never gave (change 0375). The surfaced id must equal the id the
// bound run record actually carries — the same value run.cancel cross-checks.
func TestIntegrationRunStartFreshStartSurfacesRunID(t *testing.T) {
	repo := newRunTrackerRepo(t)
	deps := PlanningDeps{Reader: runStartReader(t, runStartCorpus(), nil, nil), Clock: testClock()}
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunStart(context.Background(), deps, WorkspaceDeps{}, sp.deps(), repo, "implement-next", 0)
	if !res.Started || res.Key == "" {
		t.Fatalf("Started=%v Key=%q, want started", res.Started, res.Key)
	}

	// The start minted a run beside the run-tracker record; its id is what run.cancel and
	// every --run-id flag consume, so the start must hand it back.
	ep, _, err := LoadRunRecord(repo, res.Key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.RunID == "" {
		t.Fatalf("minted run has no id")
	}
	if res.RunID != ep.RunID {
		t.Errorf("result Run = %q, want the minted run id %q", res.RunID, ep.RunID)
	}

	// Human report line: run-started <key> <run-id> <run-context>, then the
	// owner-lifecycle caveat.
	if got, want := res.HumanText(), "run-started "+res.Key+" "+ep.RunID+" "+scopeGrantChild+"\n"+ReasonOwnerLifecycleUnavailable; got != want {
		t.Errorf("HumanText = %q, want %q", got, want)
	}
}

// TestIntegrationRunStartResumeBindsOnlyVerifiedInProgress: a --resume id pre-binds
// attribution ONLY when the id is genuinely in-progress AND WorkspaceInspect
// applies; a proposed id or a failed inspect is resume-unverified and mints no
// record (and never prepares a scope).
func TestIntegrationRunStartResumeBindsOnlyVerifiedInProgress(t *testing.T) {
	t.Run("in-progress with valid inspect binds", func(t *testing.T) {
		repoDir := newWorkingRepo(t, nil).invocation
		reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(5, "epsilon", "v5", "")}}
		deps := workspaceDepsFor(t, reader)
		wdeps := WorkspaceDeps{Service: resumeInspectService("/tmp/wt/epsilon")}
		sp := &fakeScopePrep{grant: sampleScopeGrant()}

		res := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
		if !res.Started {
			t.Fatalf("resume did not start: %q", res.HumanText())
		}
		if sp.calls != 1 {
			t.Fatalf("PrepareScope called %d times, want 1", sp.calls)
		}
		// The scope's ChangeID is pre-bound to the verified id, and its
		// Branch/Worktree carry the resumed change's identity.
		if sp.req.ChangeID != "5" {
			t.Errorf("scope ChangeID = %q, want \"5\" (pre-bound)", sp.req.ChangeID)
		}
		if sp.req.Branch != "refs/heads/feat/epsilon" {
			t.Errorf("scope Branch = %q, want refs/heads/feat/epsilon", sp.req.Branch)
		}
		if sp.req.Worktree != "/tmp/wt/epsilon" {
			t.Errorf("scope Worktree = %q, want /tmp/wt/epsilon", sp.req.Worktree)
		}
		rec, err := LoadRunTrackerRecord(repoDir, res.Key)
		if err != nil {
			t.Fatalf("LoadRunTrackerRecord: %v", err)
		}
		if rec.AttributedID != 5 {
			t.Errorf("AttributedID = %d, want 5 (pre-bound)", rec.AttributedID)
		}
	})

	t.Run("proposed id is resume-unverified", func(t *testing.T) {
		repoDir := newWorkingRepo(t, nil).invocation
		reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{proposedChangeBlob(5, "epsilon", "v5")}}
		deps := workspaceDepsFor(t, reader)
		wdeps := WorkspaceDeps{Service: resumeInspectService("/tmp/wt/epsilon")}
		sp := &fakeScopePrep{grant: sampleScopeGrant()}

		res := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
		if res.Started {
			t.Fatalf("started for a non-in-progress resume id")
		}
		if res.Reason != ReasonRunResumeUnverified {
			t.Errorf("Reason = %q, want %q", res.Reason, ReasonRunResumeUnverified)
		}
		if res.Key != "" {
			t.Errorf("minted a record %q for an unverified resume", res.Key)
		}
		if sp.calls != 0 {
			t.Errorf("prepared a scope (%d calls) for an unverified resume", sp.calls)
		}
	})

	t.Run("failed inspect is resume-unverified", func(t *testing.T) {
		repoDir := newWorkingRepo(t, nil).invocation
		reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(5, "epsilon", "v5", "")}}
		deps := workspaceDepsFor(t, reader)
		svc := resumeInspectService("/tmp/wt/epsilon")
		svc.inspectErr = errInspectProbe
		wdeps := WorkspaceDeps{Service: svc}
		sp := &fakeScopePrep{grant: sampleScopeGrant()}

		res := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
		if res.Started {
			t.Fatalf("started despite a failed inspect")
		}
		if res.Reason != ReasonRunResumeUnverified {
			t.Errorf("Reason = %q, want %q", res.Reason, ReasonRunResumeUnverified)
		}
		if res.Key != "" || sp.calls != 0 {
			t.Errorf("minted/prepared on a failed inspect: key=%q calls=%d", res.Key, sp.calls)
		}
	})
}

// TestIntegrationRunStartNoTimestampGames: the resume path never plays a timestamp game.
// The resumed change stays in the fresh BeforeIDs and DispatchedAt stays
// post-read — attribution is bound by verified identity, not by excluding the id
// from the before-set.
func TestIntegrationRunStartNoTimestampGames(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(5, "epsilon", "v5", "")}}
	deps := workspaceDepsFor(t, reader)
	wdeps := WorkspaceDeps{Service: resumeInspectService("/tmp/wt/epsilon")}
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if !res.Started {
		t.Fatalf("resume did not start: %q", res.HumanText())
	}
	rec, err := LoadRunTrackerRecord(repoDir, res.Key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}
	// The resumed change is present in the fresh before-set (not excluded).
	found := false
	for _, id := range rec.BeforeIDs {
		if id == 5 {
			found = true
		}
	}
	if !found {
		t.Errorf("resumed id 5 not in BeforeIDs %v — attribution must not lean on excluding it", rec.BeforeIDs)
	}
	// DispatchedAt is still captured post-read (>= CreatedAt), unchanged by resume.
	if rec.DispatchedAt < rec.CreatedAt {
		t.Errorf("DispatchedAt %d < CreatedAt %d — resume must not rewind the run", rec.DispatchedAt, rec.CreatedAt)
	}
	if rec.AttributedID != 5 {
		t.Errorf("AttributedID = %d, want 5", rec.AttributedID)
	}
}

// runTrackerPinWithRunMaxAttempts builds a StatusPin whose resolved config carries an
// explicit repository-layer run.max_attempts, so a run start through it
// snapshots that value into the record's AttemptLimit.
func runTrackerPinWithRunMaxAttempts(t *testing.T, n int) StatusPin {
	t.Helper()
	snap, _, err := config.Resolve([]config.Source{{
		Layer: config.LayerRepository,
		Name:  ".docket.yml",
		Data:  []byte(fmt.Sprintf("run:\n  max_attempts: %d\n", n)),
	}}, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("resolve config run.max_attempts=%d: %v", n, err)
	}
	p := mainPin(t)
	p.Config = *snap
	return p
}

// TestIntegrationRunStartMintSnapshotsRunMaxAttempts: run start snapshots the authoritative
// run.max_attempts into the record's AttemptLimit at mint (change 0421). A repo
// configured run.max_attempts: 3 yields AttemptLimit == 3; the default yields 2;
// and a later config change never rewrites an already-minted record's limit (the
// snapshot rule — the load never re-reads config).
func TestIntegrationRunStartMintSnapshotsRunMaxAttempts(t *testing.T) {
	t.Run("configured value is snapshotted", func(t *testing.T) {
		repo := newRunTrackerRepo(t)
		reader := &fakeReader{pin: runTrackerPinWithRunMaxAttempts(t, 3), corpus: runStartCorpus()}
		deps := PlanningDeps{Reader: reader, Clock: testClock()}
		sp := &fakeScopePrep{grant: sampleScopeGrant()}

		res := RunStart(context.Background(), deps, WorkspaceDeps{}, sp.deps(), repo, "implement-next", 0)
		if !res.Started {
			t.Fatalf("did not start: %q", res.HumanText())
		}
		rec, err := LoadRunTrackerRecord(repo, res.Key)
		if err != nil {
			t.Fatalf("LoadRunTrackerRecord: %v", err)
		}
		if rec.AttemptLimit != 3 {
			t.Errorf("AttemptLimit = %d, want 3 (snapshotted run.max_attempts)", rec.AttemptLimit)
		}
		// The snapshot is immutable: a re-load never re-reads config, so the limit
		// stays 3 regardless of any later configuration change.
		rec2, err := LoadRunTrackerRecord(repo, res.Key)
		if err != nil {
			t.Fatalf("LoadRunTrackerRecord (reload): %v", err)
		}
		if rec2.AttemptLimit != 3 {
			t.Errorf("reloaded AttemptLimit = %d, want 3 (snapshot immutable)", rec2.AttemptLimit)
		}
	})

	t.Run("default is 2", func(t *testing.T) {
		repo := newRunTrackerRepo(t)
		deps := PlanningDeps{Reader: runStartReader(t, runStartCorpus(), nil, nil), Clock: testClock()}
		sp := &fakeScopePrep{grant: sampleScopeGrant()}

		res := RunStart(context.Background(), deps, WorkspaceDeps{}, sp.deps(), repo, "implement-next", 0)
		if !res.Started {
			t.Fatalf("did not start: %q", res.HumanText())
		}
		rec, err := LoadRunTrackerRecord(repo, res.Key)
		if err != nil {
			t.Fatalf("LoadRunTrackerRecord: %v", err)
		}
		if rec.AttemptLimit != 2 {
			t.Errorf("AttemptLimit = %d, want the built-in default 2", rec.AttemptLimit)
		}
	})
}

// TestIntegrationRunStartRunTrackerRecordContinuationTripleRule: the store rejects a partial continuation
// triple on BOTH the write and the read boundary as a corrupt record.
func TestIntegrationRunStartRunTrackerRecordContinuationTripleRule(t *testing.T) {
	repo := newRunTrackerRepo(t)

	// Write boundary: minting/saving a partial triple fails closed.
	partial := sampleRunTrackerRecord()
	partial.ContinuationID = "cont-1"
	// ContinuationDrive / ContinuationHandoff deliberately left empty.
	_, err := MintRunTrackerRecord(repo, partial)
	if gse, ok := AsRunTrackerStoreError(err); !ok || gse.Kind != ErrRunTrackerCorruptRecord {
		t.Errorf("mint of a partial triple = %v, want ErrRunTrackerCorruptRecord", err)
	}

	// A full triple writes and reads back cleanly.
	full := sampleRunTrackerRecord()
	full.ContinuationID = "cont-1"
	full.ContinuationDrive = "drive-1"
	full.ContinuationHandoff = "handoff-1"
	key, err := MintRunTrackerRecord(repo, full)
	if err != nil {
		t.Fatalf("mint of a full triple: %v", err)
	}
	if _, err := LoadRunTrackerRecord(repo, key); err != nil {
		t.Fatalf("load of a full triple: %v", err)
	}

	// Read boundary: a partial triple planted on disk fails closed on load.
	root, err := runTrackerRoot(repo)
	if err != nil {
		t.Fatalf("runTrackerRoot: %v", err)
	}
	writeRawRunTrackerRecord(t, root, key, `{"schema":2,"repo":%q,"target":"docket-implement-next","retry":"unused","continuation_id":"x","continuation_drive":"y"}`)
	_, err = LoadRunTrackerRecord(repo, key)
	if gse, ok := AsRunTrackerStoreError(err); !ok || gse.Kind != ErrRunTrackerCorruptRecord {
		t.Errorf("load of a planted partial triple = %v, want ErrRunTrackerCorruptRecord", err)
	}
}

// TestIntegrationRunStartRunTrackerRecordSchema1FailsClosed: a schema-1 record fails closed as a corrupt
// record — the v2 store never migrates a pre-upgrade record.
func TestIntegrationRunStartRunTrackerRecordSchema1FailsClosed(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key, err := MintRunTrackerRecord(repo, sampleRunTrackerRecord())
	if err != nil {
		t.Fatalf("MintRunTrackerRecord: %v", err)
	}
	root, err := runTrackerRoot(repo)
	if err != nil {
		t.Fatalf("runTrackerRoot: %v", err)
	}
	writeRawRunTrackerRecord(t, root, key, `{"schema":1,"repo":%q,"target":"docket-implement-next","retry":"unused"}`)
	_, err = LoadRunTrackerRecord(repo, key)
	if gse, ok := AsRunTrackerStoreError(err); !ok || gse.Kind != ErrRunTrackerCorruptRecord {
		t.Errorf("schema 1 load = %v, want ErrRunTrackerCorruptRecord", err)
	}
}

// errInspectProbe is a canned WorkspaceService.Inspect failure so a resume test
// can drive WorkspaceInspect to a non-applied result.
var errInspectProbe = errors.New("inspect probe failed")

// writeRawRunTrackerRecord overwrites the record.json at root/key with tmpl formatted
// against the record's own canonical repo value (read from the existing record so
// the wrong-repo guard is not the thing that fires). It is how a test plants a
// deliberately malformed on-disk record to prove a fail-closed load.
func writeRawRunTrackerRecord(t *testing.T, root, key, tmpl string) {
	t.Helper()
	path := filepath.Join(root, key, runTrackerRecordFileName)
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read existing record: %v", err)
	}
	var existing struct {
		Repo string `json:"repo"`
	}
	if err := json.Unmarshal(buf, &existing); err != nil {
		t.Fatalf("parse existing record: %v", err)
	}
	if err := os.WriteFile(path, []byte(fmt.Sprintf(tmpl, existing.Repo)), 0o600); err != nil {
		t.Fatalf("write raw record: %v", err)
	}
}

// TestIntegrationRunStartStartedLineIsAlwaysThreeTokens (change 0463): every started result a real start
// produces (fresh, no-run-record resume, cancelled-replacement resume) prints a first
// line of exactly four space-separated fields. Field 3 is the run and field 4 is
// the run context, so a positional parser can never read the run context
// as the run.
func TestIntegrationRunStartStartedLineIsAlwaysThreeTokens(t *testing.T) {
	check := func(t *testing.T, res RunStartResult) {
		t.Helper()
		if !res.Started {
			t.Fatalf("did not start: %q", res.HumanText())
		}
		first := strings.SplitN(res.HumanText(), "\n", 2)[0]
		fields := strings.Fields(first)
		if len(fields) != 4 || fields[0] != "run-started" {
			t.Fatalf("started line %q: want exactly `run-started <key> <run-id> <run-context>`", first)
		}
		if fields[1] != res.Key || fields[2] != res.RunID || fields[3] != res.RunContext {
			t.Fatalf("started line %q: fields (%q,%q,%q), want (key %q, run %q, run context %q)",
				first, fields[1], fields[2], fields[3], res.Key, res.RunID, res.RunContext)
		}
		if res.RunID == "" || res.RunID == res.RunContext {
			t.Fatalf("run %q must be a distinct non-empty token from the run context %q", res.RunID, res.RunContext)
		}
	}
	t.Run("fresh start", func(t *testing.T) {
		repo := newRunTrackerRepo(t)
		deps := PlanningDeps{Reader: runStartReader(t, runStartCorpus(), nil, nil), Clock: testClock()}
		sp := &fakeScopePrep{grant: sampleScopeGrant()}
		check(t, RunStart(context.Background(), deps, WorkspaceDeps{}, sp.deps(), repo, "implement-next", 0))
	})
	t.Run("no-run-record resume", func(t *testing.T) {
		repoDir := newWorkingRepo(t, nil).invocation
		deps, wdeps := resumeRunDeps(t)
		sp := &fakeScopePrep{grant: sampleScopeGrant()}
		check(t, RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5))
	})
	t.Run("cancelled-replacement resume", func(t *testing.T) {
		repoDir := newWorkingRepo(t, nil).invocation
		seedPriorRun(t, repoDir, RunCancelled)
		deps, wdeps := resumeRunDeps(t)
		sp := &fakeScopePrep{grant: sampleScopeGrant()}
		check(t, RunStart(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5))
	})
}
