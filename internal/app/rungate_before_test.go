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

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
)

// These are the `docket run gate-before` (arm the gate) tests (change 0334,
// Task 2). gate-before re-syncs the metadata worktree to fresh origin, reads the
// in-progress claim set, captures a dispatch epoch AFTER that read, and mints a
// durable gate record — printing `gate-armed <key>` on success and
// `gate-unarmed <reason-token>` on any failure, exiting 0 either way (the report
// line is the contract). Only `implement-next` is an accepted target; anything
// else is a usage error that exits non-zero.
//
// The in-progress read reuses the same PinContext/ReadCorpus plumbing the claim
// path uses (a fresh-origin fetch inside PinContext, one change-file parse), so
// these tests inject the scriptable fakeReader for the corpus and a real temp
// git repo (newGateRepo) for the store's git-common-dir rooting.

// gateBeforeCorpus is a scriptable in-progress claim set: ids 3 and 7 are
// in-progress (the before-set), id 5 is proposed and id 8 is blocked (neither is
// an in-progress claim), so a correct before-read collects exactly {3, 7}.
func gateBeforeCorpus() []StatusBlob {
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

// gateBeforeReader is the fakeReader wired with a mainPin and the given corpus /
// injected errors.
func gateBeforeReader(t *testing.T, corpus []StatusBlob, pinErr, corpusErr error) *fakeReader {
	t.Helper()
	return &fakeReader{pin: mainPin(t), corpus: corpus, pinErr: pinErr, corpusErr: corpusErr}
}

// --- outer-scope preparation fake (change 0359) ---------------------------

// scopeParentCap / scopeChildCap / scopeID are the fake grant's distinct tokens:
// distinct so a redaction assert cannot pass by confusing one for another, and
// child != parent so the printed dispatch context is provably the child alone.
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

// fakeScopePrep is a scriptable GateScopeDeps.Prepare: it records the request it
// was handed and returns a canned grant (or a canned error), so a test can prove
// the scope was prepared with the right ChangeID/Branch/Worktree and was NOT
// prepared on a pre-scope refusal.
type fakeScopePrep struct {
	grant gatedrive.ScopeGrant
	err   error
	calls int
	req   gatedrive.ScopeRequest
}

func (f *fakeScopePrep) deps() GateScopeDeps {
	return GateScopeDeps{Prepare: func(req gatedrive.ScopeRequest) (gatedrive.ScopeGrant, error) {
		f.calls++
		f.req = req
		if f.err != nil {
			return gatedrive.ScopeGrant{}, f.err
		}
		return f.grant, nil
	}}
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

// TestGateBeforePreparesOuterScope: a non-resume arm prepares the outer scope,
// carries the scope binding in the record, prints the dispatch context on the
// armed line, and NEVER leaks the parent capability into the result JSON or the
// human text (it lives only in the 0600 record).
func TestGateBeforePreparesOuterScope(t *testing.T) {
	repo := newGateRepo(t)
	deps := PlanningDeps{Reader: gateBeforeReader(t, gateBeforeCorpus(), nil, nil), Clock: testClock()}
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunGateBefore(context.Background(), deps, WorkspaceDeps{}, sp.deps(), repo, "implement-next", 0)
	if !res.Armed || res.Key == "" {
		t.Fatalf("Armed=%v Key=%q, want armed", res.Armed, res.Key)
	}
	if sp.calls != 1 {
		t.Fatalf("PrepareScope called %d times, want 1", sp.calls)
	}
	// A fresh (non-resume) outer scope binds no change id yet and carries no
	// resumed identity.
	if sp.req.ChangeID != "" || sp.req.Branch != "" || sp.req.Worktree != "" {
		t.Errorf("fresh scope request carried identity: %+v", sp.req)
	}
	// Armed line: gate-armed <key> <dispatch-context>.
	if got, want := res.HumanText(), "gate-armed "+res.Key+" "+scopeGrantChild; got != want {
		t.Errorf("HumanText = %q, want %q", got, want)
	}
	if res.DispatchContext != scopeGrantChild {
		t.Errorf("DispatchContext = %q, want %q", res.DispatchContext, scopeGrantChild)
	}

	rec, err := LoadGateRecord(repo, res.Key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
	}
	if rec.ScopeID != scopeGrantID {
		t.Errorf("ScopeID = %q, want %q", rec.ScopeID, scopeGrantID)
	}
	if rec.ParentCap != scopeGrantParent {
		t.Errorf("ParentCap = %q, want the raw parent cap persisted in the 0600 record", rec.ParentCap)
	}
	if want := gateHashToken(scopeGrantChild); rec.ChildContextHash != want {
		t.Errorf("ChildContextHash = %q, want sha256 of the dispatch context %q", rec.ChildContextHash, want)
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

// TestGateBeforeResumeBindsOnlyVerifiedInProgress: a --resume id pre-binds
// attribution ONLY when the id is genuinely in-progress AND WorkspaceInspect
// applies; a proposed id or a failed inspect is resume-unverified and mints no
// record (and never prepares a scope).
func TestGateBeforeResumeBindsOnlyVerifiedInProgress(t *testing.T) {
	t.Run("in-progress with valid inspect binds", func(t *testing.T) {
		repoDir := newWorkingRepo(t, nil).invocation
		reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(5, "epsilon", "v5", "")}}
		deps := workspaceDepsFor(t, reader)
		wdeps := WorkspaceDeps{Service: resumeInspectService("/tmp/wt/epsilon")}
		sp := &fakeScopePrep{grant: sampleScopeGrant()}

		res := RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
		if !res.Armed {
			t.Fatalf("resume did not arm: %q", res.HumanText())
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
		rec, err := LoadGateRecord(repoDir, res.Key)
		if err != nil {
			t.Fatalf("LoadGateRecord: %v", err)
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

		res := RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
		if res.Armed {
			t.Fatalf("armed for a non-in-progress resume id")
		}
		if res.Reason != ReasonGateResumeUnverified {
			t.Errorf("Reason = %q, want %q", res.Reason, ReasonGateResumeUnverified)
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

		res := RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
		if res.Armed {
			t.Fatalf("armed despite a failed inspect")
		}
		if res.Reason != ReasonGateResumeUnverified {
			t.Errorf("Reason = %q, want %q", res.Reason, ReasonGateResumeUnverified)
		}
		if res.Key != "" || sp.calls != 0 {
			t.Errorf("minted/prepared on a failed inspect: key=%q calls=%d", res.Key, sp.calls)
		}
	})
}

// TestGateBeforeNoTimestampGames: the resume path never plays a timestamp game.
// The resumed change stays in the fresh BeforeIDs and DispatchEpoch stays
// post-read — attribution is bound by verified identity, not by excluding the id
// from the before-set.
func TestGateBeforeNoTimestampGames(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(5, "epsilon", "v5", "")}}
	deps := workspaceDepsFor(t, reader)
	wdeps := WorkspaceDeps{Service: resumeInspectService("/tmp/wt/epsilon")}
	sp := &fakeScopePrep{grant: sampleScopeGrant()}

	res := RunGateBefore(context.Background(), deps, wdeps, sp.deps(), repoDir, "implement-next", 5)
	if !res.Armed {
		t.Fatalf("resume did not arm: %q", res.HumanText())
	}
	rec, err := LoadGateRecord(repoDir, res.Key)
	if err != nil {
		t.Fatalf("LoadGateRecord: %v", err)
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
	// DispatchEpoch is still captured post-read (>= CreatedAt), unchanged by resume.
	if rec.DispatchEpoch < rec.CreatedAt {
		t.Errorf("DispatchEpoch %d < CreatedAt %d — resume must not rewind the epoch", rec.DispatchEpoch, rec.CreatedAt)
	}
	if rec.AttributedID != 5 {
		t.Errorf("AttributedID = %d, want 5", rec.AttributedID)
	}
}

// TestGateRecordContinuationTripleRule: the store rejects a partial continuation
// triple on BOTH the write and the read boundary as a corrupt record.
func TestGateRecordContinuationTripleRule(t *testing.T) {
	repo := newGateRepo(t)

	// Write boundary: minting/saving a partial triple fails closed.
	partial := sampleGateRecord()
	partial.ContinuationID = "cont-1"
	// ContinuationDrive / ContinuationHandoff deliberately left empty.
	_, err := MintGateRecord(repo, partial)
	if gse, ok := AsGateStoreError(err); !ok || gse.Kind != ErrGateCorruptRecord {
		t.Errorf("mint of a partial triple = %v, want ErrGateCorruptRecord", err)
	}

	// A full triple writes and reads back cleanly.
	full := sampleGateRecord()
	full.ContinuationID = "cont-1"
	full.ContinuationDrive = "drive-1"
	full.ContinuationHandoff = "handoff-1"
	key, err := MintGateRecord(repo, full)
	if err != nil {
		t.Fatalf("mint of a full triple: %v", err)
	}
	if _, err := LoadGateRecord(repo, key); err != nil {
		t.Fatalf("load of a full triple: %v", err)
	}

	// Read boundary: a partial triple planted on disk fails closed on load.
	root, err := gateRoot(repo)
	if err != nil {
		t.Fatalf("gateRoot: %v", err)
	}
	writeRawGateRecord(t, root, key, `{"schema":2,"repo":%q,"target":"docket-implement-next","retry":"unused","continuation_id":"x","continuation_drive":"y"}`)
	_, err = LoadGateRecord(repo, key)
	if gse, ok := AsGateStoreError(err); !ok || gse.Kind != ErrGateCorruptRecord {
		t.Errorf("load of a planted partial triple = %v, want ErrGateCorruptRecord", err)
	}
}

// TestGateRecordSchema1FailsClosed: a schema-1 record fails closed as a corrupt
// record — the v2 store never migrates a pre-upgrade record.
func TestGateRecordSchema1FailsClosed(t *testing.T) {
	repo := newGateRepo(t)
	key, err := MintGateRecord(repo, sampleGateRecord())
	if err != nil {
		t.Fatalf("MintGateRecord: %v", err)
	}
	root, err := gateRoot(repo)
	if err != nil {
		t.Fatalf("gateRoot: %v", err)
	}
	writeRawGateRecord(t, root, key, `{"schema":1,"repo":%q,"target":"docket-implement-next","retry":"unused"}`)
	_, err = LoadGateRecord(repo, key)
	if gse, ok := AsGateStoreError(err); !ok || gse.Kind != ErrGateCorruptRecord {
		t.Errorf("schema 1 load = %v, want ErrGateCorruptRecord", err)
	}
}

// errInspectProbe is a canned WorkspaceService.Inspect failure so a resume test
// can drive WorkspaceInspect to a non-applied result.
var errInspectProbe = errors.New("inspect probe failed")

// writeRawGateRecord overwrites the record.json at root/key with tmpl formatted
// against the record's own canonical repo value (read from the existing record so
// the wrong-repo guard is not the thing that fires). It is how a test plants a
// deliberately malformed on-disk record to prove a fail-closed load.
func writeRawGateRecord(t *testing.T, root, key, tmpl string) {
	t.Helper()
	path := filepath.Join(root, key, gateRecordFileName)
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
