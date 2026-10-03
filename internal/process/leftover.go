package process

import "path/filepath"

// LeftoverAnswer is ProbeLeftover's three-way verdict on a run whose supervisor
// has exited (change 0492).
type LeftoverAnswer string

const (
	// LeftoverNone: the run's recorded process group is provably empty, so
	// nothing that stayed in the supervisor's group is still running.
	LeftoverNone LeftoverAnswer = "none"
	// LeftoverPresent: the recorded group still has members while its leader
	// (the supervisor's own pid) is provably gone — part of the suite outlived
	// its supervisor.
	LeftoverPresent LeftoverAnswer = "leftover"
	// LeftoverUnclear: anything the check cannot prove — a supervisor still
	// holding live.lock, an unprovable probe, an unaddressable group, or a
	// leader pid that still answers (a zombie supervisor, or a reused number).
	LeftoverUnclear LeftoverAnswer = "unclear"
)

// Leftover is ProbeLeftover's result: the answer and the recorded group id it
// is about (zero only when validation failed).
type Leftover struct {
	Answer LeftoverAnswer
	PGID   int
}

// leftoverGroupProbe and leftoverLeaderProbe are ProbeLeftover's liveness
// probes. Production is groupAlive and processAlive; a test overrides them to
// drive every unclear branch deterministically on every platform, in the style
// of recoverGroupProbe.
var (
	leftoverGroupProbe  = groupAlive
	leftoverLeaderProbe = processAlive
)

// ProbeLeftover answers, for a run whose supervisor has exited, whether any of
// its suite is still running in the supervisor's process group. It is
// read-only: it never signals and never writes.
//
//   - none: the recorded group is provably absent.
//   - leftover: the group still has members and its leader pid is provably
//     gone. Process-group ids are not reused while the group exists (Linux and
//     the BSD-derived kernels, macOS included, never hand out a pid still in
//     use as a process-group id), so a populated group whose leader is gone is
//     the run's own group.
//   - unclear: the supervisor still holds live.lock or the lock is
//     unprovable; the recorded pgid is <= 1 or differs from supervisor_pid; the
//     group probe is unknown; or the leader pid still answers (a zombie
//     supervisor whose launcher has not reaped it, or the number reused by an
//     unrelated process) or is unknown.
//
// Validation is Observe's (validatedManifest); a validation failure is an
// error, which callers treat as unclear. Callers act only on leftover — none
// and unclear both keep today's behavior.
//
// It sees only what stayed in the supervisor's group (go run, the suite
// runner). Test targets lead their own groups; while the runner lives the
// supervisor's group is non-empty, so that is enough. Targets whose runner was
// itself killed are invisible here (an accepted loss).
func (s *Service) ProbeLeftover(runDir string) (Leftover, error) {
	m, err := validatedManifest("probe-leftover", runDir)
	if err != nil {
		return Leftover{Answer: LeftoverUnclear}, err
	}
	out := Leftover{Answer: LeftoverUnclear, PGID: m.PGID}

	// The check means something only once the supervisor is gone: a held lock
	// or an unprovable probe is unclear, never absence.
	held, ans := probeFlock(filepath.Join(runDir, liveLockFile))
	if ans == probeUnknown || held {
		return out, nil
	}

	// Only the group the supervisor led is the run's: pgid <= 1 is never a real
	// group, and a pgid that is not the supervisor's own pid was never its.
	if m.PGID <= 1 || m.PGID != m.SupervisorPID {
		return out, nil
	}

	switch leftoverGroupProbe(m.PGID) {
	case probeAbsent:
		out.Answer = LeftoverNone
	case probeLive:
		// A live group alone is not enough: a zombie supervisor (or an unrelated
		// process reusing the number) still answers as its leader.
		if leftoverLeaderProbe(m.PGID) == probeAbsent {
			out.Answer = LeftoverPresent
		}
	}
	return out, nil
}
