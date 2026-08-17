package process

import (
	"path/filepath"
	"syscall"
)

// Observation is the read-only verdict Observe returns for a run. State is
// always one of the run-state vocabulary; Terminal is non-nil exactly when a
// durable terminal record decided the state. Cause carries a bounded safe
// disappearance reason for a vanished run when a marker supplies one. StdoutLog
// and StderrLog are paths only — Observe never reads their contents.
type Observation struct {
	RunID     string
	RunDir    string
	State     State
	Terminal  *Terminal
	Cause     string
	StdoutLog string
	StderrLog string
}

// observePostProbeHook is a package-private test seam fired once in the cleanly
// free-lock branch, between the live-lock probe and the re-read of
// terminal.json. Production leaves it nil; a race test sets it to write a
// terminal record at exactly that instant, proving the re-read is load-bearing.
var observePostProbeHook func()

// Observe reports a run's state through a fixed, read-only, ordered decision.
// The order is load-bearing (spec: Observation): a valid run always yields a
// state, never a guessed one, and no unprovable read is ever resolved into
// running, vanished, or permission to signal.
//
//  1. Validate the run path, manifest, and run-ID agreement between the
//     manifest and the directory name.
//  2. Read terminal.json first. A valid terminal record decides the state
//     (stop intent distinguishes a signalled death from a requested stop).
//  3. With no terminal record, probe the live lock and — when held — require
//     the full identity conjunction to decide running; an unprovable probe or
//     conjunction is blocked, never running.
//  4. A cleanly free lock means the supervisor is gone. Re-read terminal.json
//     so a terminal write racing the first read wins rather than being
//     misreported as disappearance.
//  5. Still no terminal: a completed stop decides stopped; otherwise the run
//     is vanished, with an abandoned/failure marker supplying the cause when
//     present.
func (s *Service) Observe(runDir string) (*Observation, error) {
	// (1) Validate run path + manifest + run-ID agreement. The manifest supplies
	// the recorded root, so containment is proven against what the run claims
	// rather than against the run dir's own parent.
	m, err := readManifest(runDir)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, failf(FailInvalidState, "observe", "run directory has no manifest")
	}
	_, dirID, err := resolveRunDir(m.Root, runDir)
	if err != nil {
		return nil, err
	}
	if m.RunID != dirID {
		return nil, failf(FailInvalidState, "observe", "manifest run id disagrees with the run directory")
	}

	obs := &Observation{
		RunID:     m.RunID,
		RunDir:    runDir,
		StdoutLog: filepath.Join(runDir, stdoutLogFile),
		StderrLog: filepath.Join(runDir, stderrLogFile),
	}

	// (2) Terminal record first — the child's own verdict outranks any liveness
	// guess. A malformed record is invalid-state, never a fabricated pass.
	stopIntent, err := readStopIntent(runDir)
	if err != nil {
		return nil, err
	}
	if term, err := readTerminal(runDir); err != nil {
		return nil, err
	} else if term != nil {
		return s.observed(obs, terminalState(term, stopIntent != nil), term), nil
	}

	// (3) No terminal record: probe the live lock. An unprovable probe is
	// blocked, never absence.
	held, ans := probeFlock(filepath.Join(runDir, liveLockFile))
	if ans == probeUnknown {
		return nil, failf(FailBlocked, "observe", "live lock unprovable")
	}
	if held {
		self, _ := syscall.Getpgid(0)
		if err := identityConjunction(m, self); err != nil {
			return nil, err // FailBlocked — a held lock without proven identity is not running
		}
		obs.State = StateRunning
		return obs, nil
	}

	// (4) Cleanly free lock: the supervisor is gone. Re-read terminal.json so a
	// terminal write racing the first read wins rather than being misreported as
	// disappearance.
	if observePostProbeHook != nil {
		observePostProbeHook()
	}
	stopIntent, err = readStopIntent(runDir)
	if err != nil {
		return nil, err
	}
	if term, err := readTerminal(runDir); err != nil {
		return nil, err
	} else if term != nil {
		return s.observed(obs, terminalState(term, stopIntent != nil), term), nil
	}

	// (5) Supervisor gone, still no terminal: a completed stop decides stopped;
	// otherwise the run vanished, with a marker supplying the cause when present.
	if stopped, err := readStopped(runDir); err != nil {
		return nil, err
	} else if stopped != nil {
		obs.State = StateStopped
		return obs, nil
	}
	obs.State = StateVanished
	if ab, err := readAbandoned(runDir); err != nil {
		return nil, err
	} else if ab != nil {
		obs.Cause = boundReason(ab.Cause)
	} else if fr, err := readFailureRecord(runDir); err != nil {
		return nil, err
	} else if fr != nil {
		obs.Cause = boundReason(fr.Reason)
	}
	return obs, nil
}

// observed stamps a terminal verdict onto obs from a terminal record.
func (s *Service) observed(obs *Observation, state State, term *terminalRecord) *Observation {
	obs.State = state
	obs.Terminal = &Terminal{Kind: term.Kind, ExitCode: term.ExitCode, Signal: term.Signal}
	return obs
}
