//go:build integration

package app

import (
	"errors"
	"os"
	"testing"
)

// These are the death-guardian tests (change 0375 Task 13). They are ADAPTER-LEVEL
// REAL-PROCESS tests per the internal/process conventions: SpawnAgentGuardian
// re-execs THIS test binary, whose TestMain (gate_test.go) routes the guardian role
// via GuardianRequested, so a killed owner and real descriptor inheritance are
// exercised — the signal-handler unit test alone never proves the death path.
//
// Each test spawns a guardian, then either simulates an abrupt owner death (closing
// the pipe with no marker) or a clean end (Complete writes the marker first). Both
// reap the guardian synchronously (cmd.Wait), so the run state is settled and the
// assertions are deterministic — no polling.

// guardianExecutable returns this test binary's path (the guardian re-exec target).
func guardianExecutable(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	return exe
}

// spawnTestGuardian mints an active run and spawns a guardian for it, returning
// the handle, the run, and the run key.
func spawnTestGuardian(t *testing.T) (*GuardianHandle, RunRecord, string, string) {
	t.Helper()
	repo := newRunTrackerRepo(t)
	key := mintTestRunKey(t, repo)
	ep, err := MintRunRecord(repo, key, "375")
	if err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	marker, err := AgentGuardianMarkerPath(repo, key)
	if err != nil {
		t.Fatalf("AgentGuardianMarkerPath: %v", err)
	}
	handle, err := SpawnAgentGuardian(guardianExecutable(t), repo, key, ep.RunID, marker)
	if err != nil {
		t.Fatalf("SpawnAgentGuardian: %v", err)
	}
	return handle, ep, key, repo
}

// TestIntegrationGateLifecycleGuardianEOFFencesRun proves an abrupt owner death — the pipe closes with no
// completion marker — makes the guardian fence the run to cancelling, the
// durable exclusion that admits no replacement.
func TestIntegrationGateLifecycleGuardianEOFFencesRun(t *testing.T) {
	handle, _, key, repo := spawnTestGuardian(t)

	// Simulate the owner dying: drop the pipe write end WITHOUT writing the marker.
	if err := handle.pipeW.Close(); err != nil {
		t.Fatalf("closing owner pipe: %v", err)
	}
	// The guardian does all its work before exiting; Wait blocks until then.
	if err := handle.cmd.Wait(); err != nil {
		t.Fatalf("guardian exited nonzero: %v", err)
	}

	ep, _, err := LoadRunRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.State != RunCancelling {
		t.Fatalf("run state = %s, want cancelling after abrupt owner death", ep.State)
	}
}

// TestIntegrationGateLifecycleGuardianCompletionMarkerPreventsCancel proves a clean end — the owner writes
// the durable marker before closing the pipe — makes the guardian exit WITHOUT
// fencing, so a normal return or a handoff is never mistaken for a Stop. This is the
// mutation-evidence target: suppress the marker write in Complete and this reddens.
func TestIntegrationGateLifecycleGuardianCompletionMarkerPreventsCancel(t *testing.T) {
	handle, _, key, repo := spawnTestGuardian(t)

	// Complete writes the marker, closes the pipe, and reaps the guardian.
	handle.Complete()

	ep, _, err := LoadRunRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.State != RunActive {
		t.Fatalf("run state = %s, want active (a completion marker must prevent a fence)", ep.State)
	}
}

// TestIntegrationGateLifecycleGuardianStaleMarkerDoesNotSuppressFence proves SpawnAgentGuardian clears any
// pre-existing completion marker before it starts the guardian, so only a marker
// this owner writes during THIS lifetime can suppress the fence. A stale marker
// left in a reused run-key directory (from a prior clean completion) must NOT
// pre-suppress a fresh guardian's fence: an abrupt owner death still finds no
// marker and fences the run — the exact abrupt-death case the guardian exists to
// catch. Defense in depth: run keys are unique today, but the guardian must not
// depend on that for its safety property.
func TestIntegrationGateLifecycleGuardianStaleMarkerDoesNotSuppressFence(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := mintTestRunKey(t, repo)
	ep, err := MintRunRecord(repo, key, "375")
	if err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	marker, err := AgentGuardianMarkerPath(repo, key)
	if err != nil {
		t.Fatalf("AgentGuardianMarkerPath: %v", err)
	}
	// Plant a STALE marker in the run-key directory before the guardian spawns —
	// as if a prior clean completion had left one behind.
	if err := os.WriteFile(marker, []byte("stale\n"), 0o600); err != nil {
		t.Fatalf("planting stale marker: %v", err)
	}

	handle, err := SpawnAgentGuardian(guardianExecutable(t), repo, key, ep.RunID, marker)
	if err != nil {
		t.Fatalf("SpawnAgentGuardian: %v", err)
	}
	// Simulate an abrupt owner death: drop the pipe write end WITHOUT writing a
	// fresh marker. If the spawn had left the stale marker in place, the guardian
	// would Stat it and exit without fencing.
	if err := handle.pipeW.Close(); err != nil {
		t.Fatalf("closing owner pipe: %v", err)
	}
	if err := handle.cmd.Wait(); err != nil {
		t.Fatalf("guardian exited nonzero: %v", err)
	}

	got, _, err := LoadRunRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if got.State != RunCancelling {
		t.Fatalf("run state = %s, want cancelling: a stale pre-spawn marker must not suppress the fence", got.State)
	}
}

// TestIntegrationGateLifecycleGuardianCannotMutate proves the guardian holds cancel/observe authority only:
// a registered guardian participant confers nothing (a workflow mutation is still
// admitted while the run is active), and the ONLY effect a guardian can have on
// mutation admission is to FENCE the run on death — after which no workflow
// mutation is admitted. The guardian can block, never enable.
func TestIntegrationGateLifecycleGuardianCannotMutate(t *testing.T) {
	repo := newRunTrackerRepo(t)
	key := mintTestRunKey(t, repo)
	ep, err := MintRunRecord(repo, key, "375")
	if err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	// Bind the worktree so the mutation fence resolves this run by worktree.
	canon, err := canonicalWorktree(repo)
	if err != nil {
		t.Fatalf("canonicalWorktree: %v", err)
	}
	if err := runRecordCAS(repo, key, func(r *RunRecord) error { r.Worktree = canon; return nil }); err != nil {
		t.Fatalf("bind worktree: %v", err)
	}
	// Register a guardian participant: a registered-but-passive guardian must not by
	// itself fence the run.
	if err := RegisterRunParticipant(repo, key, ep.RunID, RunParticipant{Kind: "guardian", NativeHandle: "g1"}); err != nil {
		t.Fatalf("RegisterRunParticipant: %v", err)
	}
	// A registered (even a would-be-dead) guardian participant leaves the run
	// ACTIVE — the durable exclusion that still blocks a replacement resume until an
	// explicit recovery (Task 12's active-run refusal). The guardian's registration
	// confers no fence.
	if got, _, err := LoadRunRecord(repo, key); err != nil || got.State != RunActive {
		t.Fatalf("run after guardian registration = %v (err=%v), want active", got.State, err)
	}

	// While active, a workflow mutation is admitted — the guardian participant
	// confers no block.
	done, err := admitWorkflowMutation(repo, "test.mutation.pre", nil)
	if err != nil {
		t.Fatalf("mutation refused while active: %v", err)
	}
	done(mutationStatusCompleted, false)

	// Now the owner dies abruptly and the guardian fences the run.
	marker, err := AgentGuardianMarkerPath(repo, key)
	if err != nil {
		t.Fatalf("AgentGuardianMarkerPath: %v", err)
	}
	handle, err := SpawnAgentGuardian(guardianExecutable(t), repo, key, ep.RunID, marker)
	if err != nil {
		t.Fatalf("SpawnAgentGuardian: %v", err)
	}
	if err := handle.pipeW.Close(); err != nil {
		t.Fatalf("closing owner pipe: %v", err)
	}
	if err := handle.cmd.Wait(); err != nil {
		t.Fatalf("guardian exited nonzero: %v", err)
	}

	// Under the guardian's fence, no workflow mutation is admitted.
	if _, err := admitWorkflowMutation(repo, "test.mutation.post", nil); !errors.Is(err, ErrRunCancelled) {
		t.Fatalf("mutation after guardian fence = %v, want ErrRunCancelled", err)
	}
}

// TestIntegrationGateLifecycleGuardianLeavesCompletingRunForReplay pins the death guardian's completing
// behavior (change 0441): the guardian fences ONLY an active run (guardianFenceAndReap
// flips active→cancelling and reaps only when the fence lands), so an abrupt owner
// death over a COMPLETING run — a keyed verdict verified run-complete and durably
// fenced the run mid-closeout — leaves it exactly completing, untouched and
// unreaped, so a keyed-verdict replay resumes the closeout. Success is never encoded
// as cancellation. This is the mutation-evidence target: make the guardian CAS also
// flip completing→cancelling and this reddens.
func TestIntegrationGateLifecycleGuardianLeavesCompletingRunForReplay(t *testing.T) {
	handle, _, key, repo := spawnTestGuardian(t)

	// The keyed verdict fenced the run to completing (successful closeout in
	// flight). Force it BEFORE the owner dies so the guardian observes completing.
	forceRunState(t, repo, key, RunCompleting)

	// Simulate the owner dying: drop the pipe write end WITHOUT writing the marker.
	if err := handle.pipeW.Close(); err != nil {
		t.Fatalf("closing owner pipe: %v", err)
	}
	if err := handle.cmd.Wait(); err != nil {
		t.Fatalf("guardian exited nonzero: %v", err)
	}

	ep, _, err := LoadRunRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.State != RunCompleting {
		t.Fatalf("run state = %s, want completing left untouched (the guardian fences only active; a keyed-verdict replay resumes closeout)", ep.State)
	}
}
