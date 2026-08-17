//go:build darwin

package process

import "syscall"

// platformSupported gates Launch: darwin has a validated SYS_GETSID spelling.
const platformSupported = true

// getSID reads the session id of pid. There is no os/syscall wrapper for
// getsid(2), so it is issued raw: errno 0 -> live; ESRCH -> absent; any other
// errno -> unknown (never absence — an unprobeable session is not a gone one).
func getSID(pid int) (int, probeAnswer) {
	sid, _, errno := syscall.Syscall(syscall.SYS_GETSID, uintptr(pid), 0, 0)
	switch errno {
	case 0:
		return int(sid), probeLive
	case syscall.ESRCH:
		return 0, probeAbsent
	default:
		return 0, probeUnknown
	}
}
