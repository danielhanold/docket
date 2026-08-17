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
//
// A pgid <= 1 is not a real supervised group and must never be probed: with
// pgid == 0, -0 == 0 and kill(0, 0) addresses the CALLER'S OWN process group,
// which would return nil and falsely resolve probeLive; pgid 1 is init's group.
// Both fail closed to probeUnknown — never probeLive/probeAbsent — so no
// caller can mistake a non-real group id for a provably-live or provably-absent
// answer.
func groupAlive(pgid int) probeAnswer {
	if pgid <= 1 {
		return probeUnknown
	}
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
//
// It fails closed to probeUnknown on any non-real group id — a pid <= 1 (0 is
// "self" to Getpgid, 1 is init) or a resolved pgid <= 1 — rather than handing
// back a live/absent answer paired with a group id that groupAlive/signalGroup
// would refuse anyway.
func getPGID(pid int) (int, probeAnswer) {
	if pid <= 1 {
		return 0, probeUnknown
	}
	switch pgid, err := syscall.Getpgid(pid); err {
	case nil:
		if pgid <= 1 {
			return 0, probeUnknown
		}
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
//
// It refuses a pgid <= 1 outright: kill(-0, sig) == kill(0, sig) would signal
// the caller's OWN process group, and pgid 1 is init. Neither is a real
// supervised group, so it returns FailInvalidState and delivers nothing rather
// than issuing the kill.
func signalGroup(pgid int, sig syscall.Signal) error {
	if pgid <= 1 {
		return failf(FailInvalidState, "signal-group", "refusing to signal non-real group %d", pgid)
	}
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
