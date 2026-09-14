// The detached agent death-guardian (change 0375 Task 13). Codex `agent.enter`
// integrates a signal-connected cancellation for a catchable Stop (SIGTERM/SIGINT
// handled in internal/codexentry). This guardian covers the case a signal handler
// cannot: an UNCATCHABLE owner death — SIGKILL, a crashed terminal, a lost SSH
// session — where the owner never runs another instruction.
//
// MECHANISM. Before entering a run's turn the owner re-execs THIS binary as a
// detached guardian (SpawnAgentGuardian): a new session leader (Setsid) that
// carries the run's locators in private env and inherits ONE descriptor — the read
// end of a pipe whose write end the owner keeps. The guardian blocks reading that
// pipe. The pipe has exactly two ways to reach EOF:
//
//   - The owner finishes (a normal return, an explicit handoff, or a handled
//     signal-cancellation): it first writes a durable completion MARKER file, then
//     closes the pipe. The guardian sees EOF, finds the marker, and exits without
//     fencing — a completed turn, a tool-call timeout, and a normal return are
//     never mistaken for a Stop.
//   - The owner dies abruptly: the kernel closes every descriptor it held,
//     including the pipe write end, and NO marker was written. The guardian sees
//     EOF, finds no marker, and fences the run epoch (active→cancelling) then reaps
//     the run — so an abandoned run stops blocking a replacement automatically.
//
// AUTHORITY. The guardian holds CANCEL/OBSERVE authority only. It carries the gate
// key and the public epoch id (locators, not credentials) and NO child capability,
// so it is fenced out of mutation admission exactly like any non-writer (the
// run-epoch mutation fence keys on the epoch state the guardian drives, never on the
// guardian's identity). It reaps the run's registered participants and worktree slot
// through the SAME accounting run.cancel uses (reconcileEpochTeardown), but WITHOUT
// the authority conjunction — the guardian is a trusted re-exec the already-authorized
// owner spawned, located to exactly one epoch, which it verifies before fencing.
package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// The private environment that activates and locates a re-exec'd guardian. Only
// SpawnAgentGuardian sets these; ordinary `docket` invocations never do, so
// GuardianRequested is false for every user-facing command.
const (
	guardianRepoDirEnv = "DOCKET_AGENT_GUARDIAN_REPO_DIR"
	guardianGateKeyEnv = "DOCKET_AGENT_GUARDIAN_GATE_KEY"
	guardianEpochEnv   = "DOCKET_AGENT_GUARDIAN_EPOCH"
	guardianMarkerEnv  = "DOCKET_AGENT_GUARDIAN_MARKER"
	// guardianPipeFD is the inherited pipe read end (the first and only ExtraFiles
	// slot), mirroring the supervisor's inherited-descriptor contract.
	guardianPipeFD = 3
)

// errGuardianEpochMismatch aborts the fence CAS without a write when the located
// epoch's public id does not match the id the guardian was spawned for — a stale
// re-exec can never fence an unrelated successor epoch. It is internal to the fence.
var errGuardianEpochMismatch = errors.New("guardian epoch id does not match the located epoch")

// GuardianRequested reports whether this process was re-executed as an agent death
// guardian. It is the whole predicate cli.Run's pre-Cobra hook keys on: true iff
// the private repo-dir env var is set, which only SpawnAgentGuardian does.
func GuardianRequested() bool { return os.Getenv(guardianRepoDirEnv) != "" }

// MaybeRunAgentGuardian runs the death guardian when this process was re-executed
// as one and reports ok=true. cli.Run calls it before Cobra parses anything: the
// guardian is not a public command and must never touch the protocol streams.
func MaybeRunAgentGuardian() (int, bool) {
	if !GuardianRequested() {
		return 0, false
	}
	return RunAgentGuardianFromEnv(), true
}

// RunAgentGuardianFromEnv runs the whole guardian lifetime and returns the process
// exit code (always 0 for the guardian path — a guardian that cannot act fails
// SAFE: the un-fenced epoch's durable exclusion still blocks a replacement until an
// explicit human recovery). It never calls os.Exit; cmd/docket/main.go owns the
// single exit site.
func RunAgentGuardianFromEnv() int {
	repoDir := os.Getenv(guardianRepoDirEnv)
	gateKey := os.Getenv(guardianGateKeyEnv)
	epochID := os.Getenv(guardianEpochEnv)
	marker := os.Getenv(guardianMarkerEnv)

	// Adopt the inherited pipe read end and mark it close-on-exec: it must never
	// leak into any process the guardian might start, and it is the guardian's sole
	// liveness signal for the owner.
	pipe := os.NewFile(uintptr(guardianPipeFD), "agent-guardian-pipe")
	syscall.CloseOnExec(guardianPipeFD)
	if pipe == nil {
		return 0
	}
	defer pipe.Close()

	// Block until the owner's pipe write end closes. The bytes are irrelevant — the
	// owner writes nothing meaningful; the pipe's closure is the whole signal — so
	// drain to EOF (or any read error, which is also "the owner is gone").
	buf := make([]byte, 64)
	for {
		if _, err := pipe.Read(buf); err != nil {
			break
		}
	}

	// A durable completion/handoff marker proves a CLEAN end: the owner wrote it
	// before closing the pipe. Only a marker that provably EXISTS suppresses the
	// fence — a normal return, a handoff, and a handled signal-cancel all leave it.
	if marker != "" {
		if _, err := os.Stat(marker); err == nil {
			return 0
		}
	}

	// Abrupt owner death, no marker: fence the epoch and reap the run.
	guardianFenceAndReap(repoDir, gateKey, epochID)
	return 0
}

// guardianFenceAndReap flips the located epoch active→cancelling (idempotent; a
// concurrent or prior run.cancel that already fenced it leaves it cancelling) and,
// only when the fence holds, reaps the run's participants and worktree slot. It
// verifies the epoch id before writing so a stale guardian cannot fence a successor
// epoch, and it never revives a terminal (cancelled/superseded) epoch.
//
// The guardian FENCES and reaps best-effort but deliberately does NOT finalize the
// epoch to cancelled: reporting a run fully cancelled requires the authority
// conjunction and the full accounting run.cancel owns. The guardian holds
// cancel/observe authority only, so it leaves the epoch CANCELLING — a durable
// exclusion that blocks any replacement until a human `run.cancel` re-runs the
// accounting under authority and confirms cancellation. That is the authority split:
// the guardian (no authority) fences on death; the human (authority) confirms.
func guardianFenceAndReap(repoDir, gateKey, epochID string) {
	ferr := epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
		if epochID != "" && rec.EpochID != epochID {
			return errGuardianEpochMismatch // abort with no write
		}
		if rec.State == EpochActive {
			rec.State = EpochCancelling
		}
		// A cancelling/cancelled/superseded epoch is left exactly as found.
		return nil
	})
	if ferr != nil {
		return
	}
	ep, _, err := LoadEpochRecord(repoDir, gateKey)
	if err != nil || ep.State != EpochCancelling {
		// Nothing to reap: the epoch is missing, unreadable, or already terminal /
		// not the one we fenced.
		return
	}
	// The guardian has no native adapter (a dead owner has no live app-server thread
	// to interrupt) — native participants surface as findings, discarded here — and
	// no capability. The teardown accounting is identical to run.cancel's; its
	// verdict is discarded because the guardian never finalizes the epoch.
	_, _, _ = reconcileEpochTeardown(productionCancelSeams(repoDir), repoDir, gateKey, ep)
}

// GuardianHandle is the owner-side handle to a spawned death guardian: the pipe
// write end whose closure the guardian watches, the durable marker path the owner
// writes on a clean end, and the spawned process for reaping.
type GuardianHandle struct {
	pipeW  *os.File
	marker string
	cmd    *exec.Cmd
}

// SpawnAgentGuardian re-execs executable as a detached death guardian for the run
// epoch located by (gateKey, epochID) under repoDir, watching the returned handle's
// pipe. markerPath names the durable completion marker the owner writes (via
// Complete) on a clean end so the guardian never mistakes it for a Stop. The
// guardian is a new session leader (Setsid), so it survives the owner's terminal or
// session ending; it inherits ONLY the pipe read end (fd 3) and none of the owner's
// standard streams (all routed to /dev/null), carrying no capability. On any error
// before Start it closes every descriptor it opened.
func SpawnAgentGuardian(executable, repoDir, gateKey, epochID, markerPath string) (*GuardianHandle, error) {
	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		pipeR.Close()
		pipeW.Close()
		return nil, err
	}
	defer devNull.Close()

	cmd := exec.Command(executable)
	cmd.Env = append(os.Environ(),
		guardianRepoDirEnv+"="+repoDir,
		guardianGateKeyEnv+"="+gateKey,
		guardianEpochEnv+"="+epochID,
		guardianMarkerEnv+"="+markerPath)
	// Setsid detaches the guardian into its own session so a SIGHUP to the owner's
	// session, or the owner's death, does not take the guardian with it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	// ExtraFiles slot 0 -> fd 3: the pipe READ end. The write end is a Go pipe fd,
	// which the runtime marks close-on-exec, so it never leaks into the guardian —
	// the guardian must observe EOF the moment the owner (and only the owner) lets
	// its write end go.
	cmd.ExtraFiles = []*os.File{pipeR}
	if err := cmd.Start(); err != nil {
		pipeR.Close()
		pipeW.Close()
		return nil, err
	}
	// The guardian now owns the read end. The owner keeps only the write end.
	pipeR.Close()
	return &GuardianHandle{pipeW: pipeW, marker: markerPath, cmd: cmd}, nil
}

// Complete signals a CLEAN end to the guardian: it writes the durable completion
// marker, then closes the pipe so the guardian observes EOF WITH the marker and
// exits without fencing, and reaps the guardian process. It is called on a normal
// return, an explicit handoff, or after a handled signal-cancellation — every path
// where the owner is still executing and the run must NOT be mistaken for an abrupt
// death. A nil handle is a no-op (no guardian was spawned).
func (h *GuardianHandle) Complete() {
	if h == nil {
		return
	}
	if h.marker != "" {
		// Write the marker BEFORE closing the pipe: the guardian may race to Stat it
		// the instant the pipe closes.
		_ = os.WriteFile(h.marker, []byte("complete\n"), 0o600)
	}
	if h.pipeW != nil {
		h.pipeW.Close()
	}
	if h.cmd != nil && h.cmd.Process != nil {
		// The guardian exits promptly on EOF+marker; reap it so it leaves no zombie.
		_ = h.cmd.Wait()
	}
}

// markerPathFor returns the durable completion-marker path for a run, beside the
// run's gate-key directory so it shares that record's lifetime and 0700 privacy.
func markerPathFor(gitCommonDir, gateKey string) string {
	return filepath.Join(gitCommonDir, "docket", "rungate", gateKey, "owner-complete.marker")
}

// AgentGuardianMarkerPath resolves the completion-marker path a guardian for
// (repoDir, gateKey) watches, beside the run's gate-key directory. The CLI passes
// it to SpawnAgentGuardian so the owner and the guardian agree on the one file
// whose presence distinguishes a clean end from an abrupt death.
func AgentGuardianMarkerPath(repoDir, gateKey string) (string, error) {
	common, err := gateGitCommonDir(repoDir)
	if err != nil {
		return "", err
	}
	return markerPathFor(common, gateKey), nil
}
