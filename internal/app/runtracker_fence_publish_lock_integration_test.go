//go:build integration

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/workspace"
)

// lockStateDuringCall records, from inside the adapter call, the journal entry's
// status, its token, and its lock probe.
type lockStateDuringCall struct {
	status, token string
	probe         process.LockProbe
	seen          bool
}

func captureLockState(t *testing.T, repo, key string, out *lockStateDuringCall) func() {
	return func() {
		ep, _, err := LoadRunRecord(repo, key)
		if err != nil || len(ep.AdmittedMutations) == 0 {
			t.Errorf("load inside adapter: %v (entries %d)", err, len(ep.AdmittedMutations))
			return
		}
		m := ep.AdmittedMutations[len(ep.AdmittedMutations)-1]
		out.status, out.token, out.seen = m.Status, m.LockToken, true
		if dir, derr := runKeyDir(repo, key, "test"); derr == nil {
			if p, ok := publishLockPath(dir, m.LockToken); ok {
				out.probe = process.ProbeLock(p)
			}
		}
	}
}

func assertLockAcrossCall(t *testing.T, repo, key string, during lockStateDuringCall) {
	t.Helper()
	if !during.seen || during.status != mutationStatusAdmitted || !validPublishLockToken(during.token) {
		t.Fatalf("during the remote call: %+v, want an admitted entry with a valid lock_token", during)
	}
	if during.probe != process.LockProbeHeld {
		t.Fatalf("lock probed %v during the remote call, want held", during.probe)
	}
	m := journalEntry(t, repo, key, 0)
	if m.Status != mutationStatusCompleted || m.LockToken != during.token {
		t.Fatalf("after the call: %+v, want completed with the same token", m)
	}
	path := publishLockPathFor(t, repo, key, during.token)
	if got := process.ProbeLock(path); got != process.LockProbeFree {
		t.Fatalf("lock probed %v after the callback, want free", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file must never be deleted: %v", err)
	}
}

// TestIntegrationRunFencePRPublishHoldsPublishLockAcrossRemoteCall (spec test 7).
func TestIntegrationRunFencePRPublishHoldsPublishLockAcrossRemoteCall(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	key := mintFenceRun(t, repoDir, repoDir, RunActive)
	var during lockStateDuringCall
	gh := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("verified")}}
	gh.onEnsure = captureLockState(t, repoDir, key, &during)
	res := PRPublish(context.Background(), workspaceDepsFor(t, prReader(t)), WorkspaceDeps{Service: readyService(prHead)},
		GitHubDeps{Service: gh}, repoDir, PRPublishRequest{ID: 7, Head: prHead, Title: "t", Body: "b\n", EvidenceRecord: prEvidenceBytes(t, prHead)})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q (reason %q), want applied", res.Result, res.Reason)
	}
	assertLockAcrossCall(t, repoDir, key, during)
}

// TestIntegrationRunFenceWorkspacePublishHoldsPublishLockAcrossRemoteCall (spec test 7).
func TestIntegrationRunFenceWorkspacePublishHoldsPublishLockAcrossRemoteCall(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	key := mintFenceRun(t, repoDir, repoDir, RunActive)
	const head = "abcdef0000000000000000000000000000000000"
	var during lockStateDuringCall
	svc := &fakeWorkspaceService{
		inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)},
		publishRes: workspace.PublishResult{Disposition: workspace.PublishPublished, Head: gitcli.ObjectID(head)},
	}
	svc.onPublish = captureLockState(t, repoDir, key, &during)
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")}}
	res := WorkspacePublish(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
		repoDir, WorkspacePublishRequest{ID: 7, Head: head})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q (reason %q), want applied", res.Result, res.Reason)
	}
	assertLockAcrossCall(t, repoDir, key, during)
}

// TestIntegrationRunFencePublishLockFailureStillPublishes (spec test 7): a lock that
// cannot be taken never refuses the publish. The entry is journaled with no token
// and resolves exactly as before change 0494.
func TestIntegrationRunFencePublishLockFailureStillPublishes(t *testing.T) {
	const head = "abcdef0000000000000000000000000000000000"
	cases := map[string]func(t *testing.T){
		"acquire error": func(t *testing.T) {
			overrideSeam(t, &publishLockAcquire, func(string) (*os.File, bool, error) { return nil, false, errors.New("EIO") })
		},
		"busy on a fresh token": func(t *testing.T) {
			overrideSeam(t, &publishLockAcquire, func(string) (*os.File, bool, error) { return nil, true, nil })
		},
		"token mint failure": func(t *testing.T) {
			overrideSeam(t, &publishLockMintToken, func() (string, error) { return "", errors.New("no entropy") })
		},
	}
	for name, arm := range cases {
		t.Run(name, func(t *testing.T) {
			arm(t)
			repoDir := newWorkingRepo(t, nil).invocation
			key := mintFenceRun(t, repoDir, repoDir, RunActive)
			svc := &fakeWorkspaceService{
				inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)},
				publishRes: workspace.PublishResult{Disposition: workspace.PublishPublished, Head: gitcli.ObjectID(head)},
			}
			reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")}}
			res := WorkspacePublish(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
				repoDir, WorkspacePublishRequest{ID: 7, Head: head})
			if res.Result != ResultApplied || len(svc.publishCalls) != 1 {
				t.Fatalf("result = %q, publish calls = %d; a lock failure must never refuse the publish", res.Result, len(svc.publishCalls))
			}
			if m := journalEntry(t, repoDir, key, 0); m.LockToken != "" || m.Status != mutationStatusCompleted {
				t.Fatalf("entry = %+v, want completed with no lock_token", m)
			}
		})
	}
}

// TestIntegrationRunFenceRefusedPublishReleasesItsLock (spec test 7): a fence
// refusal from the admission CAS closes the lock taken before it. No entry is
// journaled, the lock reads free, and the file is never deleted.
func TestIntegrationRunFenceRefusedPublishReleasesItsLock(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	key := mintFenceRun(t, repoDir, repoDir, RunCancelling)
	desc := lockTestWSDesc()
	done, err := admitWorkflowMutation(repoDir, OperationWorkspacePublish, &desc)
	if !errors.Is(err, ErrRunCancelled) || done != nil {
		t.Fatalf("admit = (%v, %v), want (nil, ErrRunCancelled)", done != nil, err)
	}
	dir, derr := runKeyDir(repoDir, key, "test")
	if derr != nil {
		t.Fatal(derr)
	}
	locks, _ := filepath.Glob(filepath.Join(dir, "publish-*.lock"))
	if len(locks) != 1 {
		t.Fatalf("publish locks = %v, want exactly the one taken before the CAS", locks)
	}
	if got := process.ProbeLock(locks[0]); got != process.LockProbeFree {
		t.Fatalf("refused publish left its lock %v, want free", got)
	}
	f, busy, terr := process.TryExclusiveLock(locks[0])
	if terr != nil || busy {
		t.Fatalf("TryExclusiveLock after refusal: busy=%v err=%v", busy, terr)
	}
	f.Close()
	if ep, _, _ := LoadRunRecord(repoDir, key); len(ep.AdmittedMutations) != 0 {
		t.Fatalf("a refused admission journaled %+v", ep.AdmittedMutations)
	}
}

// TestIntegrationRunFenceOutcomeWriteLandsBeforePublishLockRelease (spec tests 7 and 9):
// while the outcome write waits on run.lock, the publish lock stays held, and a
// settler's probe reads it busy. A settler running under a live publisher's lock
// leaves the entry admitted. Neither ordering deadlocks.
func TestIntegrationRunFenceOutcomeWriteLandsBeforePublishLockRelease(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	key := mintFenceRun(t, repoDir, repoDir, RunActive)
	dir, err := runKeyDir(repoDir, key, "test")
	if err != nil {
		t.Fatal(err)
	}
	desc := lockTestWSDesc()

	// (a) A settler under a live publisher's lock: it probes busy, leaves the entry
	// admitted, and releases run.lock; then the publisher's outcome write completes.
	done1, err := admitWorkflowMutation(repoDir, OperationWorkspacePublish, &desc)
	if err != nil {
		t.Fatal(err)
	}
	finishedA := make(chan struct{})
	go func() {
		defer close(finishedA)
		settled, findings := settleUncertainPublications(repoDir, key)
		if len(settled) != 0 || len(findings) != 0 {
			t.Errorf("settler under a live publisher: settled=%v findings=%v, want none", settled, findings)
		}
	}()
	select {
	case <-finishedA:
	case <-time.After(10 * time.Second):
		t.Fatal("settler deadlocked against a live publisher's lock")
	}
	if m := journalEntry(t, repoDir, key, 0); m.Status != mutationStatusAdmitted {
		t.Fatalf("settler rewrote a live publisher's entry to %q", m.Status)
	}
	done1(mutationStatusCompleted, true)
	if m := journalEntry(t, repoDir, key, 0); m.Status != mutationStatusCompleted || !m.Verified {
		t.Fatalf("publisher outcome = %+v, want completed+verified", m)
	}

	// (b) Ordering: hold run.lock (a settler mid-CAS); the publisher's callback blocks
	// on it for its outcome write, and its publish lock must stay held throughout.
	done2, err := admitWorkflowMutation(repoDir, OperationWorkspacePublish, &desc)
	if err != nil {
		t.Fatal(err)
	}
	entry := journalEntry(t, repoDir, key, 1)
	path, ok := publishLockPath(dir, entry.LockToken)
	if !ok {
		t.Fatalf("entry carries no valid lock_token: %+v", entry)
	}
	runLock, err := acquireRunLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	finishedB := make(chan struct{})
	go func() {
		defer close(finishedB)
		done2(mutationStatusCompleted, true)
	}()
	for i := 0; i < 50; i++ { // ~500ms: a release-before-write would surface here
		if got := process.ProbeLock(path); got != process.LockProbeHeld {
			runLock.Close()
			t.Fatalf("publish lock %v while the outcome write is still blocked: released before the outcome was written", got)
		}
		if publisherGone(dir, entry) {
			runLock.Close()
			t.Fatal("a settler under run.lock read the live publisher as gone")
		}
		time.Sleep(10 * time.Millisecond)
	}
	runLock.Close()
	select {
	case <-finishedB:
	case <-time.After(10 * time.Second):
		t.Fatal("outcome write deadlocked after run.lock was released")
	}
	if m := journalEntry(t, repoDir, key, 1); m.Status != mutationStatusCompleted {
		t.Fatalf("outcome = %q, want completed", m.Status)
	}
	if got := process.ProbeLock(path); got != process.LockProbeFree {
		t.Fatalf("lock %v after the callback returned, want free", got)
	}
}
