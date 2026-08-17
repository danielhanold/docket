//go:build !darwin && !linux

package process

// platformSupported is false off darwin/linux: no SYS_GETSID spelling has been
// validated. Launch (Task 6) refuses outright with FailExternal rather than
// running in a weaker, un-detached mode.
const platformSupported = false

// getSID cannot be issued portably here, so it reports unknown — never a
// clean absence a caller might mistake for a gone session.
func getSID(pid int) (int, probeAnswer) {
	return 0, probeUnknown
}
