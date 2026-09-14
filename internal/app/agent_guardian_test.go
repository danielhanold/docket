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
// reap the guardian synchronously (cmd.Wait), so the epoch state is settled and the
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

// spawnTestGuardian mints an active epoch and spawns a guardian for it, returning
// the handle, the epoch, and the gate key.
func spawnTestGuardian(t *testing.T) (*GuardianHandle, EpochRecord, string, string) {
	t.Helper()
	repo := newGateRepo(t)
	key := mintTestGateKey(t, repo)
	ep, err := MintEpochRecord(repo, key, "375")
	if err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	marker, err := AgentGuardianMarkerPath(repo, key)
	if err != nil {
		t.Fatalf("AgentGuardianMarkerPath: %v", err)
	}
	handle, err := SpawnAgentGuardian(guardianExecutable(t), repo, key, ep.EpochID, marker)
	if err != nil {
		t.Fatalf("SpawnAgentGuardian: %v", err)
	}
	return handle, ep, key, repo
}

// TestGuardianEOFFencesEpoch proves an abrupt owner death — the pipe closes with no
// completion marker — makes the guardian fence the run epoch to cancelling, the
// durable exclusion that admits no replacement.
func TestGuardianEOFFencesEpoch(t *testing.T) {
	handle, _, key, repo := spawnTestGuardian(t)

	// Simulate the owner dying: drop the pipe write end WITHOUT writing the marker.
	if err := handle.pipeW.Close(); err != nil {
		t.Fatalf("closing owner pipe: %v", err)
	}
	// The guardian does all its work before exiting; Wait blocks until then.
	if err := handle.cmd.Wait(); err != nil {
		t.Fatalf("guardian exited nonzero: %v", err)
	}

	ep, _, err := LoadEpochRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if ep.State != EpochCancelling {
		t.Fatalf("epoch state = %s, want cancelling after abrupt owner death", ep.State)
	}
}

// TestGuardianCompletionMarkerPreventsCancel proves a clean end — the owner writes
// the durable marker before closing the pipe — makes the guardian exit WITHOUT
// fencing, so a normal return or a handoff is never mistaken for a Stop. This is the
// mutation-evidence target: suppress the marker write in Complete and this reddens.
func TestGuardianCompletionMarkerPreventsCancel(t *testing.T) {
	handle, _, key, repo := spawnTestGuardian(t)

	// Complete writes the marker, closes the pipe, and reaps the guardian.
	handle.Complete()

	ep, _, err := LoadEpochRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if ep.State != EpochActive {
		t.Fatalf("epoch state = %s, want active (a completion marker must prevent a fence)", ep.State)
	}
}

// TestGuardianStaleMarkerDoesNotSuppressFence proves SpawnAgentGuardian clears any
// pre-existing completion marker before it starts the guardian, so only a marker
// this owner writes during THIS lifetime can suppress the fence. A stale marker
// left in a reused gate-key directory (from a prior clean completion) must NOT
// pre-suppress a fresh guardian's fence: an abrupt owner death still finds no
// marker and fences the epoch — the exact abrupt-death case the guardian exists to
// catch. Defense in depth: gate keys are unique today, but the guardian must not
// depend on that for its safety property.
func TestGuardianStaleMarkerDoesNotSuppressFence(t *testing.T) {
	repo := newGateRepo(t)
	key := mintTestGateKey(t, repo)
	ep, err := MintEpochRecord(repo, key, "375")
	if err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	marker, err := AgentGuardianMarkerPath(repo, key)
	if err != nil {
		t.Fatalf("AgentGuardianMarkerPath: %v", err)
	}
	// Plant a STALE marker in the gate-key directory before the guardian spawns —
	// as if a prior clean completion had left one behind.
	if err := os.WriteFile(marker, []byte("stale\n"), 0o600); err != nil {
		t.Fatalf("planting stale marker: %v", err)
	}

	handle, err := SpawnAgentGuardian(guardianExecutable(t), repo, key, ep.EpochID, marker)
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

	got, _, err := LoadEpochRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if got.State != EpochCancelling {
		t.Fatalf("epoch state = %s, want cancelling: a stale pre-spawn marker must not suppress the fence", got.State)
	}
}

// TestGuardianCannotMutate proves the guardian holds cancel/observe authority only:
// a registered guardian participant confers nothing (a workflow mutation is still
// admitted while the epoch is active), and the ONLY effect a guardian can have on
// mutation admission is to FENCE the epoch on death — after which no workflow
// mutation is admitted. The guardian can block, never enable.
func TestGuardianCannotMutate(t *testing.T) {
	repo := newGateRepo(t)
	key := mintTestGateKey(t, repo)
	ep, err := MintEpochRecord(repo, key, "375")
	if err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	// Bind the worktree so the mutation fence resolves this epoch by worktree.
	canon, err := canonicalWorktree(repo)
	if err != nil {
		t.Fatalf("canonicalWorktree: %v", err)
	}
	if err := epochCAS(repo, key, func(r *EpochRecord) error { r.Worktree = canon; return nil }); err != nil {
		t.Fatalf("bind worktree: %v", err)
	}
	// Register a guardian participant: a registered-but-passive guardian must not by
	// itself fence the run.
	if err := RegisterEpochParticipant(repo, key, ep.EpochID, EpochParticipant{Kind: "guardian", NativeHandle: "g1"}); err != nil {
		t.Fatalf("RegisterEpochParticipant: %v", err)
	}
	// A registered (even a would-be-dead) guardian participant leaves the epoch
	// ACTIVE — the durable exclusion that still blocks a replacement resume until an
	// explicit recovery (Task 12's active-epoch refusal). The guardian's registration
	// confers no fence.
	if got, _, err := LoadEpochRecord(repo, key); err != nil || got.State != EpochActive {
		t.Fatalf("epoch after guardian registration = %v (err=%v), want active", got.State, err)
	}

	// While active, a workflow mutation is admitted — the guardian participant
	// confers no block.
	done, err := admitWorkflowMutation(repo, "test.mutation.pre")
	if err != nil {
		t.Fatalf("mutation refused while active: %v", err)
	}
	done(mutationStatusCompleted)

	// Now the owner dies abruptly and the guardian fences the epoch.
	marker, err := AgentGuardianMarkerPath(repo, key)
	if err != nil {
		t.Fatalf("AgentGuardianMarkerPath: %v", err)
	}
	handle, err := SpawnAgentGuardian(guardianExecutable(t), repo, key, ep.EpochID, marker)
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
	if _, err := admitWorkflowMutation(repo, "test.mutation.post"); !errors.Is(err, ErrRunCancelled) {
		t.Fatalf("mutation after guardian fence = %v, want ErrRunCancelled", err)
	}
}
