package process

import (
	"path/filepath"
	"syscall"
	"time"
)

// StopOutcome is the verdict Stop returns. Performed is false for the
// already-terminal no-op that preserves the child's own verdict, and true when
// stop drove the bounded TERM->KILL termination sequence. Terminal is non-nil
// exactly when a durable terminal record decided the state.
type StopOutcome struct {
	RunID     string
	RunDir    string
	State     State
	Terminal  *Terminal
	Performed bool
}

// Stop terminates an ownership-proven run's process group with a bounded
// escalation and records the outcome durably. The order is load-bearing
// (spec: Stop):
//
//  1. Validate the run path, manifest, and run-ID agreement (as observe). A
//     terminal record already present is an already-terminal no-op that
//     preserves the child's own verdict — no signal, no intent written.
//  2. With no terminal record, require the full ownership conjunction
//     immediately before signalling. A free lock or any unprovable clause
//     re-reads terminal (a race the supervisor may just have won) and, failing
//     that, is blocked — Stop never signals a group it cannot prove it owns.
//  3. Record stop intent BEFORE the first signal, then SIGTERM the group.
//  4. Poll up to stopTermWait for a durable terminal record and a torn-down
//     group.
//  5. Not yet down: re-prove ownership immediately before escalating. Still
//     unprovable is blocked with no escalation; provable escalates to SIGKILL
//     and polls up to stopKillWait for the group to vanish.
//  6. Assemble the verdict from the durable record, or — when KILL took the
//     supervisor before it could record — from verified teardown, writing a
//     stopped marker. Stop never writes terminal.json.
//
// Stop takes no context: once the intent is written and the first signal sent,
// completing the bounded sequence is the contract.
func (s *Service) Stop(runDir, reason string) (*StopOutcome, error) {
	// (1) Validate run path + manifest + run-ID agreement, exactly as observe:
	// the manifest supplies the recorded root, so containment is proven against
	// what the run claims rather than against the run dir's own parent.
	m, err := readManifest(runDir)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, failf(FailInvalidState, "stop", "run directory has no manifest")
	}
	_, dirID, err := resolveRunDir(m.Root, runDir)
	if err != nil {
		return nil, err
	}
	if m.RunID != dirID {
		return nil, failf(FailInvalidState, "stop", "manifest run id disagrees with the run directory")
	}

	// (1 cont.) Terminal record first: a present verdict is the child's own and
	// outranks any liveness guess. Stop is a no-op that never signals.
	if out, err := s.terminalNoOp(m, runDir); err != nil || out != nil {
		return out, err
	}

	// (2) No terminal: require the full ownership conjunction immediately before
	// signalling. A free lock or an unprovable clause never authorizes a signal;
	// re-read terminal in case the supervisor just recorded its verdict, else
	// this is blocked.
	self, _ := syscall.Getpgid(0)
	if err := identityConjunction(m, self); err != nil {
		if out, rerr := s.terminalNoOp(m, runDir); rerr != nil || out != nil {
			return out, rerr
		}
		return nil, err
	}

	// (3) Record stop intent BEFORE the first signal, then SIGTERM the group.
	// The intent is what later distinguishes a requested stop from a raw signal
	// death; it must be durable before any process can die from our signal.
	if werr := writeAtomicJSON(filepath.Join(runDir, stopIntentFile), &stopIntentRecord{
		Schema: recordSchema, RunID: m.RunID, Reason: boundReason(reason), RecordedAt: supervisorStamp(),
	}); werr != nil {
		return nil, werr
	}
	if serr := signalGroup(m.PGID, syscall.SIGTERM); serr != nil {
		return nil, serr
	}

	// (4) Poll up to stopTermWait for a durable terminal record AND a torn-down
	// group. A graceful child dies by TERM, the supervisor records and releases,
	// and the group vanishes.
	if s.awaitTeardown(runDir, m.PGID, s.stopTermWait, true) {
		return s.stopResult(m, runDir)
	}

	// (5) Still up: re-prove ownership immediately before escalating. A lock
	// that freed (or any unprovable clause) means the group is no longer
	// provably ours — blocked, never a blind SIGKILL, and no escalation.
	if err := identityConjunction(m, self); err != nil {
		return nil, err
	}
	if serr := signalGroup(m.PGID, syscall.SIGKILL); serr != nil {
		return nil, serr
	}
	// KILL may take the supervisor before it can record a terminal, so verify by
	// group teardown alone.
	s.awaitTeardown(runDir, m.PGID, s.stopKillWait, false)

	// (6) Assemble the verdict from the durable record or verified teardown.
	return s.stopResult(m, runDir)
}

// terminalNoOp returns the already-terminal no-op outcome when a terminal
// record is present, preserving the child's own verdict (a signal death after
// an earlier stop reads as stopped; a bare signal as signaled). It returns
// (nil, nil) when no terminal record exists yet.
func (s *Service) terminalNoOp(m *manifestRecord, runDir string) (*StopOutcome, error) {
	term, err := readTerminal(runDir)
	if err != nil {
		return nil, err
	}
	if term == nil {
		return nil, nil
	}
	stopIntent, err := readStopIntent(runDir)
	if err != nil {
		return nil, err
	}
	return &StopOutcome{
		RunID:     m.RunID,
		RunDir:    runDir,
		State:     terminalState(term, stopIntent != nil),
		Terminal:  &Terminal{Kind: term.Kind, ExitCode: term.ExitCode, Signal: term.Signal},
		Performed: false,
	}, nil
}

// awaitTeardown polls at pollInterval up to bound. When requireTerminal is
// true it waits for both a durable terminal record and a torn-down group (the
// TERM wait); when false it waits for the torn-down group alone (the KILL path,
// where the supervisor may die before recording). A torn-down group is one
// groupAlive reports as probeAbsent — provably gone. A probeUnknown
// (a zombie group leader awaiting reaping, or an unsignalable member) is never
// treated as absence: the poll keeps waiting until absence is proven or the
// bound elapses. It reports whether the condition held within the bound.
func (s *Service) awaitTeardown(runDir string, pgid int, bound time.Duration, requireTerminal bool) bool {
	deadline := time.Now().Add(bound)
	for {
		if s.teardownComplete(runDir, pgid, requireTerminal) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(s.pollInterval)
	}
}

// teardownComplete reports whether the run's recorded group is provably gone
// (groupAlive == probeAbsent), optionally also requiring a durable terminal
// record. Group absence is the spec's teardown signal (spec: Stop); it becomes
// observable once the supervisor exits and is reaped — by init in production,
// where the launcher has already exited and orphaned it.
func (s *Service) teardownComplete(runDir string, pgid int, requireTerminal bool) bool {
	if requireTerminal {
		term, err := readTerminal(runDir)
		if err != nil || term == nil {
			return false
		}
	}
	return groupAlive(pgid) == probeAbsent
}

// stopResult assembles the final verdict after verified teardown. A durable
// terminal record decides the state (a signal after our intent is stopped, an
// exit is passed/failed as recorded); with no terminal possible because KILL
// took the supervisor, verified group absence writes a stopped marker and the
// state is stopped. Stop never writes terminal.json.
func (s *Service) stopResult(m *manifestRecord, runDir string) (*StopOutcome, error) {
	out := &StopOutcome{RunID: m.RunID, RunDir: runDir, Performed: true}
	term, err := readTerminal(runDir)
	if err != nil {
		return nil, err
	}
	if term != nil {
		// The intent we wrote is present, so a signal death classifies as
		// stopped; an exit keeps its own passed/failed verdict.
		out.State = terminalState(term, true)
		out.Terminal = &Terminal{Kind: term.Kind, ExitCode: term.ExitCode, Signal: term.Signal}
		return out, nil
	}
	// No terminal record: the KILL took the supervisor before it could record.
	// A stopped marker is written only after group absence is verified — never
	// claim a teardown that is not provably complete (spec: stopped markers
	// appear only after verified group absence).
	if groupAlive(m.PGID) != probeAbsent {
		return nil, failf(FailBlocked, "stop", "teardown unverified: group still present after kill escalation")
	}
	if werr := writeAtomicJSON(filepath.Join(runDir, stoppedFile), &stoppedRecord{
		Schema: recordSchema, RunID: m.RunID, VerifiedAt: supervisorStamp(),
	}); werr != nil {
		return nil, werr
	}
	out.State = StateStopped
	return out, nil
}
