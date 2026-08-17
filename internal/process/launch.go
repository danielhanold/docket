package process

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// LaunchOutcome is the handle Launch returns once a run is durably published.
// State is the run's state at the moment Launch returned: StateRunning for a
// still-running gate, or a terminal state when a fast command already
// finished. Terminal is non-nil exactly when State is terminal.
type LaunchOutcome struct {
	RunID     string
	RunDir    string
	StdoutLog string
	StderrLog string
	State     State
	Terminal  *Terminal
}

// terminalState maps a durable terminal record (plus whether a stop was
// intended) to the run-state vocabulary. It is the shared helper observe and
// stop reuse: a clean exit passes, a nonzero exit fails, a signal death is a
// stop when stop intent was recorded and a raw signal otherwise.
func terminalState(term *terminalRecord, stopIntentPresent bool) State {
	switch term.Kind {
	case "exit":
		if term.ExitCode == 0 {
			return StatePassed
		}
		return StateFailed
	case "signal":
		if stopIntentPresent {
			return StateStopped
		}
		return StateSignaled
	}
	// Unreachable for a supervisor-written record (kind is only exit/signal);
	// never treat an unrecognized terminal as a pass.
	return StateFailed
}

// Launch runs the ordered launch state machine: validate, allocate a run
// directory holding a live lock under the registry lock, re-exec this binary
// as a Setsid session-leader supervisor with the live lock and a handshake
// pipe, and wait — bounded by establishTimeout — for the supervisor to publish
// an addressable running group before returning a usable handle. The run
// survives this process's exit: the supervisor is detached into its own
// session and this function never waits on it.
func (s *Service) Launch(req LaunchRequest) (*LaunchOutcome, error) {
	// (1) All validation precedes any filesystem create.
	if err := validateLaunchRequest(req); err != nil {
		return nil, err
	}
	if !platformSupported {
		return nil, failf(FailExternal, "launch", "unsupported operating system")
	}

	// (2) Allocate under the registry lock so no manifest is ever visible
	// before its live lock is held. The registry lock guards only allocation;
	// it is released before the supervisor spawns and is never held for the
	// gate's lifetime.
	if err := ensurePrivateDir(req.Root); err != nil {
		return nil, err
	}
	regLock, err := acquireFlock(filepath.Join(req.Root, registryLockFile))
	if err != nil {
		return nil, err
	}
	runID, token, err := NewRunIdentity()
	if err != nil {
		regLock.Close()
		return nil, err
	}
	runDir := filepath.Join(req.Root, runID)
	if err := ensurePrivateDir(runDir); err != nil {
		regLock.Close()
		return nil, err
	}
	lockFile, err := acquireFlock(filepath.Join(runDir, liveLockFile))
	if err != nil {
		regLock.Close()
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	m := &manifestRecord{
		Schema: recordSchema, RunID: runID, Token: token, Root: req.Root, RunDir: runDir,
		SupervisorPID: 0, PGID: 0, SID: 0, Phase: "allocated",
		Cwd: req.Cwd, Argv0: filepath.Base(req.Argv[0]), Argc: len(req.Argv),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := writeAtomicJSON(filepath.Join(runDir, manifestFile), m); err != nil {
		lockFile.Close()
		regLock.Close()
		return nil, err
	}
	// Allocation is complete: release the registry lock. The live lock stays
	// held; it is handed to the supervisor next.
	regLock.Close()

	// (3) Re-exec this binary as the supervisor. The child inherits the live
	// lock (fd 3) and the handshake pipe's write end (fd 4) via ExtraFiles, and
	// carries the run dir plus JSON-encoded argv in private env vars.
	handle, err := s.spawnSupervisor(req, runDir, lockFile)
	if err != nil {
		lockFile.Close()
		return nil, err
	}
	// The supervisor now owns the run. The launcher's copies of the lock and
	// pipe-write descriptors are surplus: the flock survives on the
	// supervisor's inherited descriptor, and closing pipeW lets the launcher's
	// reader observe EOF when the supervisor is gone.
	lockFile.Close()
	handle.pipeW.Close()
	defer handle.pipeR.Close()

	// (4) Await the establishment handshake, bounded by establishTimeout. The
	// pipe lines are wake-ups; the atomic records are the truth.
	return s.awaitEstablishment(runDir, runID, handle.pipeR)
}

// supervisorHandle bundles the launcher-side pipe ends of a spawned supervisor.
type supervisorHandle struct {
	pipeR *os.File
	pipeW *os.File
}

// spawnSupervisor builds and starts the re-exec'd supervisor. On any error
// before or during Start it closes every descriptor it opened (never the
// caller's lockFile) so the caller's cleanup is single-owner.
func (s *Service) spawnSupervisor(req LaunchRequest, runDir string, lockFile *os.File) (*supervisorHandle, error) {
	argvJSON, err := json.Marshal(req.Argv)
	if err != nil {
		return nil, failf(FailExternal, "launch-spawn", "encoding supervised argv: %v", err)
	}
	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return nil, failf(FailExternal, "launch-spawn", "creating handshake pipe: %v", err)
	}
	supLog, err := openSupervisorFile(filepath.Join(runDir, supervisorLogFile), os.O_APPEND)
	if err != nil {
		pipeR.Close()
		pipeW.Close()
		return nil, failf(FailExternal, "launch-spawn", "opening supervisor log: %v", err)
	}
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		pipeR.Close()
		pipeW.Close()
		supLog.Close()
		return nil, failf(FailExternal, "launch-spawn", "opening /dev/null: %v", err)
	}
	defer supLog.Close()
	defer devNull.Close()

	cmd := exec.Command(s.executable)
	cmd.Env = append(os.Environ(),
		supervisorRunDirEnv+"="+runDir,
		supervisorArgvEnv+"="+string(argvJSON))
	cmd.SysProcAttr = sessionAttrs()
	cmd.Stdin = devNull
	cmd.Stdout = supLog
	cmd.Stderr = supLog
	// ExtraFiles slot 0 -> fd 3 (live lock), slot 1 -> fd 4 (handshake write).
	cmd.ExtraFiles = []*os.File{lockFile, pipeW}
	if err := cmd.Start(); err != nil {
		pipeR.Close()
		pipeW.Close()
		return nil, failf(FailExternal, "launch-spawn", "starting the supervisor failed")
	}
	return &supervisorHandle{pipeR: pipeR, pipeW: pipeW}, nil
}

// awaitEstablishment reads handshake wake-ups from pipeR under establishTimeout
// and resolves each against the durable records. It returns a usable handle
// once the supervisor publishes an addressable running group (or the already
// terminal state of a fast command), and never hands back a usable handle
// while an unaddressable command might still be starting.
func (s *Service) awaitEstablishment(runDir, runID string, pipeR *os.File) (*LaunchOutcome, error) {
	lines := make(chan string, 8)
	go func() {
		sc := bufio.NewScanner(pipeR)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()

	deadline := time.After(s.establishTimeout)
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				// EOF before "running": the supervisor is gone. The command may
				// still have run to completion, or failed to establish.
				if term, err := readTerminal(runDir); err != nil {
					return nil, err
				} else if term != nil {
					return s.outcome(runDir, runID, terminalState(term, false), term), nil
				}
				if fr, err := readFailureRecord(runDir); err != nil {
					return nil, err
				} else if fr != nil {
					return nil, failf(FailExternal, fr.Stage, "%s", boundReason(fr.Reason))
				}
				return nil, failf(FailExternal, "launch-establish", "supervisor exited before establishment")
			}
			switch line {
			case "established":
				// The session is addressable but the command has not started.
				// Keep waiting for "running".
				continue
			case "running", "terminal":
				// The command is up. A fast command may already be terminal.
				if term, err := readTerminal(runDir); err != nil {
					return nil, err
				} else if term != nil {
					return s.outcome(runDir, runID, terminalState(term, false), term), nil
				}
				return s.outcome(runDir, runID, StateRunning, nil), nil
			case "failed":
				if fr, err := readFailureRecord(runDir); err != nil {
					return nil, err
				} else if fr != nil {
					return nil, failf(FailExternal, fr.Stage, "%s", boundReason(fr.Reason))
				}
				return nil, failf(FailExternal, "launch-establish", "supervisor reported failure")
			default:
				continue
			}
		case <-deadline:
			return nil, s.tearDownStalledLaunch(runDir)
		}
	}
}

// tearDownStalledLaunch handles an establishment timeout with an ownership
// check. If the recorded run is provably ours (identity conjunction holds), it
// SIGKILLs the group and waits, bounded by stopKillWait, for it to vanish, then
// reports FailExternal. If ownership is not provable, it signals nothing and
// reports FailBlocked — never a usable handle while an unaddressable command
// might still start.
func (s *Service) tearDownStalledLaunch(runDir string) error {
	m, err := readManifest(runDir)
	if err == nil && m != nil {
		self, _ := syscall.Getpgid(0)
		if identityConjunction(m, self) == nil {
			_ = signalGroup(m.PGID, syscall.SIGKILL)
			killDeadline := time.Now().Add(s.stopKillWait)
			for groupAlive(m.PGID) != probeAbsent {
				if time.Now().After(killDeadline) {
					break
				}
				time.Sleep(s.pollInterval)
			}
			return failf(FailExternal, "launch-establish", "establishment timed out")
		}
	}
	return failf(FailBlocked, "launch-establish", "establishment timed out with an unaddressable run")
}

// outcome assembles a LaunchOutcome for runDir at the given state. term is
// non-nil exactly when state is terminal.
func (s *Service) outcome(runDir, runID string, state State, term *terminalRecord) *LaunchOutcome {
	o := &LaunchOutcome{
		RunID:     runID,
		RunDir:    runDir,
		StdoutLog: filepath.Join(runDir, stdoutLogFile),
		StderrLog: filepath.Join(runDir, stderrLogFile),
		State:     state,
	}
	if term != nil {
		o.Terminal = &Terminal{Kind: term.Kind, ExitCode: term.ExitCode, Signal: term.Signal}
	}
	return o
}
