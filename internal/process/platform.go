package process

import "syscall"

// probeAnswer is the three-way vocabulary every liveness question in this
// package returns. The distinction between probeAbsent ("it is provably
// gone") and probeUnknown ("I could not tell") is load-bearing: a caller
// that signals or deletes on absence must never collapse probeUnknown into
// probeAbsent, or a permission-denied read becomes permission to act.
type probeAnswer int

const (
	probeLive probeAnswer = iota
	probeAbsent
	probeUnknown
)

func (a probeAnswer) String() string {
	switch a {
	case probeLive:
		return "live"
	case probeAbsent:
		return "absent"
	default:
		return "unknown"
	}
}

// processAlive answers whether pid names a live process without disturbing
// it. syscall.Kill with signal 0 performs the existence/permission check and
// delivers nothing: nil -> live; ESRCH -> truly gone; anything else (most
// importantly EPERM, a process we exist but may not signal) -> unknown, never
// absence.
func processAlive(pid int) probeAnswer {
	switch err := syscall.Kill(pid, 0); err {
	case nil:
		return probeLive
	case syscall.ESRCH:
		return probeAbsent
	default:
		return probeUnknown
	}
}

// groupAlive answers whether any member of process group pgid is live,
// using the negative-pid convention of kill(2). Same three-way mapping as
// processAlive.
func groupAlive(pgid int) probeAnswer {
	switch err := syscall.Kill(-pgid, 0); err {
	case nil:
		return probeLive
	case syscall.ESRCH:
		return probeAbsent
	default:
		return probeUnknown
	}
}

// getPGID reads the process-group id of pid. nil -> live; ESRCH -> absent;
// anything else -> unknown.
func getPGID(pid int) (int, probeAnswer) {
	switch pgid, err := syscall.Getpgid(pid); err {
	case nil:
		return pgid, probeLive
	case syscall.ESRCH:
		return 0, probeAbsent
	default:
		return 0, probeUnknown
	}
}

// signalGroup delivers sig to the whole process group pgid via the
// negative-pid convention of kill(2). The caller must have proven ownership
// first — this function does not itself gate on identity.
func signalGroup(pgid int, sig syscall.Signal) error {
	if err := syscall.Kill(-pgid, sig); err != nil {
		return failf(FailExternal, "signal-group", "signalling group %d: %v", pgid, err)
	}
	return nil
}

// sessionAttrs returns the SysProcAttr that makes an os/exec child a new
// session (and thus process-group) leader, so the supervisor detaches the
// run into its own signalable group.
func sessionAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
