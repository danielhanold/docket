package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// TestRepositorySyncResultShape proves the constructor stamps the operation
// envelope and that the SyncOutcome fields marshal under their protocol keys.
func TestRepositorySyncResultShape(t *testing.T) {
	r := newRepositorySyncResult(ResultApplied, RepositorySyncResult{SyncOutcome: SyncOutcome{
		Disposition: SyncDispAdvanced, IntegrationBranch: "main",
		PrimaryPath: "/repo", BeforeOID: "a", TargetOID: "b", AfterOID: "b",
	}})
	if r.Operation != OperationRepositorySyncIntegration || r.Result != ResultApplied {
		t.Fatalf("envelope = %s/%s", r.Operation, r.Result)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"disposition":"advanced"`, `"integration_branch":"main"`, `"before_oid":"a"`, `"target_oid":"b"`, `"after_oid":"b"`} {
		if !strings.Contains(string(b), key) {
			t.Fatalf("marshal missing %s in %s", key, b)
		}
	}
}

// TestRepositorySyncDirtyMessageNamesUntrackedFiles pins the one dirty-skip
// message every path shares: it must name non-ignored untracked files as a
// blocker and tell the user to inspect and resolve the dirt.
func TestRepositorySyncDirtyMessageNamesUntrackedFiles(t *testing.T) {
	msg := syncDirtyMessage() // the one constructor every dirty-skip path uses
	for _, phrase := range []string{"untracked", "inspect"} {
		if !strings.Contains(msg, phrase) {
			t.Fatalf("dirty message %q must mention %q", msg, phrase)
		}
	}
}

// ---- Task 3: the safety ladder over injectable seams ----

const (
	ladderPrimary = "/repo"
	ladderBranch  = "main"
	ladderRef     = "refs/heads/main"
	ladderHead    = "aaaaaaaaaaaa1111"
	ladderTarget  = "bbbbbbbbbbbb2222"
)

// stateReturn is one scripted answer from the fake state seam.
type stateReturn struct {
	st  gitcli.CheckoutState
	err error
}

// ancReturn is one scripted answer from the fake isAncestor seam.
type ancReturn struct {
	ok  bool
	err error
}

// fakeSeams records call counts and replays scripted sequences so a whole
// ladder path is proved without a repository. The state and isAncestor
// sequences are consumed in call order (the recheck reads the second state).
type fakeSeams struct {
	loadCtx syncContext
	loadErr error

	states     []stateReturn
	stateCalls int

	dirtyVal bool
	dirtyErr error

	ancestor      []ancReturn
	ancestorCalls int

	ffErr   error
	ffCalls int
}

func (f *fakeSeams) seams() syncSeams {
	return syncSeams{
		load: func(ctx context.Context) (syncContext, error) {
			return f.loadCtx, f.loadErr
		},
		state: func(ctx context.Context, dir string) (gitcli.CheckoutState, error) {
			i := f.stateCalls
			f.stateCalls++
			switch {
			case i < len(f.states):
				return f.states[i].st, f.states[i].err
			case len(f.states) > 0:
				last := f.states[len(f.states)-1]
				return last.st, last.err
			default:
				return gitcli.CheckoutState{}, nil
			}
		},
		dirty: func(ctx context.Context, dir string) (bool, error) {
			return f.dirtyVal, f.dirtyErr
		},
		isAncestor: func(ctx context.Context, ancestor, descendant string) (bool, error) {
			i := f.ancestorCalls
			f.ancestorCalls++
			if i < len(f.ancestor) {
				return f.ancestor[i].ok, f.ancestor[i].err
			}
			return false, nil
		},
		fastForward: func(ctx context.Context, dir, target string) (bool, error) {
			f.ffCalls++
			return f.ffErr == nil, f.ffErr
		},
	}
}

// cleanState is the attached-clean-on-main observation the happy path reads.
func cleanState(head string) gitcli.CheckoutState {
	return gitcli.CheckoutState{Branch: ladderRef, Head: gitcli.ObjectID(head)}
}

func TestRepositorySyncLadder(t *testing.T) {
	ctx := context.Background()
	loadedCtx := syncContext{
		primaryWorktree:     ladderPrimary,
		integrationBranch:   ladderBranch,
		integrationRevision: ladderTarget,
	}
	probeErr := errors.New("boom")

	tests := []struct {
		name string
		f    *fakeSeams

		wantDisp   string
		wantReason string
		wantResult Result

		msgContains string
		wantFFCalls int // -1 = do not assert
		afterEmpty  bool
		afterEqual  string // require AfterOID == this when non-empty
		beforeEqual string // require BeforeOID == this when non-empty
	}{
		{
			name:        "load discovery error",
			f:           &fakeSeams{loadErr: fmt.Errorf("%w: discovery blew up", ErrStatusExternal)},
			wantDisp:    SyncDispFailed,
			wantReason:  ReasonStatusExternal,
			wantResult:  ResultExternalFailed,
			wantFFCalls: -1,
		},
		{
			name:        "load invalid config",
			f:           &fakeSeams{loadErr: fmt.Errorf("%w: bad config", ErrStatusInvalidInput)},
			wantDisp:    SyncDispRefused,
			wantReason:  ReasonStatusInvalidInput,
			wantResult:  ResultInvalidInput,
			wantFFCalls: -1,
		},
		{
			name:        "state probe error",
			f:           &fakeSeams{loadCtx: loadedCtx, states: []stateReturn{{err: probeErr}}},
			wantDisp:    SyncDispFailed,
			wantReason:  ReasonSyncStateProbeFailed,
			wantResult:  ResultExternalFailed,
			wantFFCalls: 0,
		},
		{
			name:        "detached",
			f:           &fakeSeams{loadCtx: loadedCtx, states: []stateReturn{{st: gitcli.CheckoutState{Detached: true}}}},
			wantDisp:    SyncDispSkipped,
			wantReason:  ReasonSyncDetachedHead,
			wantResult:  ResultNoOp,
			wantFFCalls: 0,
		},
		{
			name:        "other branch",
			f:           &fakeSeams{loadCtx: loadedCtx, states: []stateReturn{{st: gitcli.CheckoutState{Branch: "refs/heads/feature-x", Head: ladderHead}}}},
			wantDisp:    SyncDispSkipped,
			wantReason:  ReasonSyncOtherBranch,
			wantResult:  ResultNoOp,
			wantFFCalls: 0,
		},
		{
			name:        "merge in progress",
			f:           &fakeSeams{loadCtx: loadedCtx, states: []stateReturn{{st: gitcli.CheckoutState{Branch: ladderRef, Head: ladderHead, OperationInProgress: true}}}},
			wantDisp:    SyncDispSkipped,
			wantReason:  ReasonSyncOperationInProgress,
			wantResult:  ResultNoOp,
			wantFFCalls: 0,
		},
		{
			name:        "dirty",
			f:           &fakeSeams{loadCtx: loadedCtx, states: []stateReturn{{st: cleanState(ladderHead)}}, dirtyVal: true},
			wantDisp:    SyncDispSkipped,
			wantReason:  ReasonSyncDirtyWorktree,
			wantResult:  ResultNoOp,
			msgContains: "untracked",
			wantFFCalls: 0,
		},
		{
			name:        "dirty probe error",
			f:           &fakeSeams{loadCtx: loadedCtx, states: []stateReturn{{st: cleanState(ladderHead)}}, dirtyErr: probeErr},
			wantDisp:    SyncDispFailed,
			wantReason:  ReasonSyncStateProbeFailed,
			wantResult:  ResultExternalFailed,
			wantFFCalls: 0,
		},
		{
			name:        "already current",
			f:           &fakeSeams{loadCtx: loadedCtx, states: []stateReturn{{st: cleanState(ladderTarget)}}},
			wantDisp:    SyncDispAlreadyCurrent,
			wantResult:  ResultNoOp,
			wantFFCalls: 0,
			afterEqual:  ladderTarget,
		},
		{
			name: "local ahead",
			f: &fakeSeams{loadCtx: loadedCtx, states: []stateReturn{{st: cleanState(ladderHead)}},
				ancestor: []ancReturn{{ok: false}, {ok: true}}},
			wantDisp:    SyncDispSkipped,
			wantReason:  ReasonSyncLocalAhead,
			wantResult:  ResultNoOp,
			wantFFCalls: 0,
		},
		{
			name: "diverged",
			f: &fakeSeams{loadCtx: loadedCtx, states: []stateReturn{{st: cleanState(ladderHead)}},
				ancestor: []ancReturn{{ok: false}, {ok: false}}},
			wantDisp:    SyncDispSkipped,
			wantReason:  ReasonSyncDiverged,
			wantResult:  ResultNoOp,
			wantFFCalls: 0,
		},
		{
			name: "ancestry error",
			f: &fakeSeams{loadCtx: loadedCtx, states: []stateReturn{{st: cleanState(ladderHead)}},
				ancestor: []ancReturn{{err: probeErr}}},
			wantDisp:    SyncDispFailed,
			wantReason:  ReasonSyncAncestryProbeFailed,
			wantResult:  ResultExternalFailed,
			wantFFCalls: 0,
		},
		{
			name: "second ancestry probe error",
			f: &fakeSeams{loadCtx: loadedCtx, states: []stateReturn{{st: cleanState(ladderHead)}},
				ancestor: []ancReturn{{ok: false}, {err: probeErr}}},
			wantDisp:    SyncDispFailed,
			wantReason:  ReasonSyncAncestryProbeFailed,
			wantResult:  ResultExternalFailed,
			wantFFCalls: 0,
		},
		{
			name: "checkout changed at recheck (branch)",
			f: &fakeSeams{loadCtx: loadedCtx,
				states:   []stateReturn{{st: cleanState(ladderHead)}, {st: gitcli.CheckoutState{Branch: "refs/heads/other", Head: ladderHead}}},
				ancestor: []ancReturn{{ok: true}}},
			wantDisp:    SyncDispSkipped,
			wantReason:  ReasonSyncCheckoutChanged,
			wantResult:  ResultNoOp,
			wantFFCalls: 0,
		},
		{
			name: "head moved at recheck",
			f: &fakeSeams{loadCtx: loadedCtx,
				states:   []stateReturn{{st: cleanState(ladderHead)}, {st: cleanState("cccccccccccc3333")}},
				ancestor: []ancReturn{{ok: true}}},
			wantDisp:    SyncDispSkipped,
			wantReason:  ReasonSyncCheckoutChanged,
			wantResult:  ResultNoOp,
			wantFFCalls: 0,
		},
		{
			name: "update failed",
			f: &fakeSeams{loadCtx: loadedCtx,
				states:   []stateReturn{{st: cleanState(ladderHead)}, {st: cleanState(ladderHead)}},
				ancestor: []ancReturn{{ok: true}}, ffErr: probeErr},
			wantDisp:    SyncDispFailed,
			wantReason:  ReasonSyncUpdateFailed,
			wantResult:  ResultExternalFailed,
			wantFFCalls: 1,
		},
		{
			name: "post-check probe error",
			f: &fakeSeams{loadCtx: loadedCtx,
				states:   []stateReturn{{st: cleanState(ladderHead)}, {st: cleanState(ladderHead)}, {err: probeErr}},
				ancestor: []ancReturn{{ok: true}}},
			wantDisp:    SyncDispFailed,
			wantReason:  ReasonSyncPostcheckUnverified,
			wantResult:  ResultExternalFailed,
			wantFFCalls: 1,
			afterEmpty:  true,
		},
		{
			name: "post-check head mismatch",
			f: &fakeSeams{loadCtx: loadedCtx,
				states:   []stateReturn{{st: cleanState(ladderHead)}, {st: cleanState(ladderHead)}, {st: cleanState(ladderHead)}},
				ancestor: []ancReturn{{ok: true}}},
			wantDisp:    SyncDispFailed,
			wantReason:  ReasonSyncPostcheckUnverified,
			wantResult:  ResultExternalFailed,
			wantFFCalls: 1,
			afterEmpty:  true,
		},
		{
			name: "clean advance",
			f: &fakeSeams{loadCtx: loadedCtx,
				states:   []stateReturn{{st: cleanState(ladderHead)}, {st: cleanState(ladderHead)}, {st: cleanState(ladderTarget)}},
				ancestor: []ancReturn{{ok: true}}},
			wantDisp:    SyncDispAdvanced,
			wantResult:  ResultApplied,
			wantFFCalls: 1,
			beforeEqual: ladderHead,
			afterEqual:  ladderTarget,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := repositorySyncIntegration(ctx, tc.f.seams())

			if r.Disposition != tc.wantDisp {
				t.Fatalf("disposition = %q, want %q", r.Disposition, tc.wantDisp)
			}
			if tc.wantReason != "" && r.Reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", r.Reason, tc.wantReason)
			}
			if r.Result != tc.wantResult {
				t.Fatalf("envelope result = %q, want %q", r.Result, tc.wantResult)
			}
			if r.Operation != OperationRepositorySyncIntegration {
				t.Fatalf("operation = %q", r.Operation)
			}
			if tc.msgContains != "" && !strings.Contains(r.Message, tc.msgContains) {
				t.Fatalf("message %q missing %q", r.Message, tc.msgContains)
			}
			if tc.wantFFCalls >= 0 && tc.f.ffCalls != tc.wantFFCalls {
				t.Fatalf("fastForward calls = %d, want %d", tc.f.ffCalls, tc.wantFFCalls)
			}
			if tc.afterEmpty && r.AfterOID != "" {
				t.Fatalf("AfterOID = %q, want empty", r.AfterOID)
			}
			if tc.afterEqual != "" && r.AfterOID != tc.afterEqual {
				t.Fatalf("AfterOID = %q, want %q", r.AfterOID, tc.afterEqual)
			}
			if tc.beforeEqual != "" && r.BeforeOID != tc.beforeEqual {
				t.Fatalf("BeforeOID = %q, want %q", r.BeforeOID, tc.beforeEqual)
			}
			// Once load succeeds, the primary/branch/target must ride every outcome.
			if tc.f.loadErr == nil {
				if r.PrimaryPath != ladderPrimary || r.IntegrationBranch != ladderBranch {
					t.Fatalf("primary/branch = %q/%q, want %q/%q", r.PrimaryPath, r.IntegrationBranch, ladderPrimary, ladderBranch)
				}
				if r.TargetOID != ladderTarget {
					t.Fatalf("TargetOID = %q, want %q", r.TargetOID, ladderTarget)
				}
			}
		})
	}
}
