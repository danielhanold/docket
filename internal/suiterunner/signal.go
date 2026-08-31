// This file owns the interruption lifecycle: catching SIGINT/SIGTERM, cancelling
// the run so nothing new launches, forwarding the SAME signal to every live child
// process GROUP, escalating survivors to SIGKILL after a bounded grace period,
// and recording which signal fired so the run exits 130 (INT) / 143 (TERM) and
// NEVER 0.
//
// Why groups, not pids: ExecuteTarget starts each target with Setpgid, so a
// target and every process it forks share one process group. Forwarding via
// kill(-pgid) (procRegistry.Signal) reaches grandchildren a pid-by-pid handler
// could not, preventing the orphaned test processes writing into a
// being-deleted scratch dir that the former Bash oracle's on_signal handler
// documented as "a data-destroying operation".
package suiterunner

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// defaultKillAfter is the grace period between forwarding the first interrupt
// signal to the child groups and escalating survivors to an untrappable SIGKILL.
const defaultKillAfter = 5 * time.Second

// InstallSignalHandling subscribes to SIGINT/SIGTERM. On the first signal it:
// (1) cancels ctx so no further target launches; (2) forwards the SAME signal to
// every registered process GROUP (kill(-pgid, sig)); (3) starts a bounded
// escalation timer (default 5s, override via killAfter for tests) after which
// surviving groups get SIGKILL; (4) records which signal fired so the exit code
// is 130 (INT) or 143 (TERM). A second signal is absorbed (the handler never
// re-enters). Returns fired(), which the run entrypoint reads after the lanes
// drain, and stop(), which unsubscribes and cancels a pending escalation.
func InstallSignalHandling(cancel context.CancelFunc, reg *procRegistry, killAfter time.Duration) (fired func() (os.Signal, bool), stop func()) {
	if killAfter <= 0 {
		killAfter = defaultKillAfter
	}
	// Buffered so a second signal arriving while the first is being handled is
	// queued (then absorbed) rather than reverting to the default terminate
	// disposition and killing the runner mid-reap.
	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var mu sync.Mutex
	var firedSig os.Signal
	var didFire bool
	var killTimer *time.Timer

	done := make(chan struct{})
	go func() {
		for {
			select {
			case s := <-sigCh:
				mu.Lock()
				if didFire {
					// Absorb the second (and any later) signal: the interruption is
					// already underway; re-entering would re-cancel, re-forward, and
					// re-arm escalation. Drain and ignore.
					mu.Unlock()
					continue
				}
				didFire = true
				firedSig = s
				// (1) Stop launching new targets.
				cancel()
				// (2) Forward the SAME signal to every live child GROUP.
				if sysSig, ok := s.(syscall.Signal); ok {
					reg.Signal(sysSig)
				}
				// (3) Bounded escalation: a group that trapped or ignored the first
				// signal gets an untrappable SIGKILL once the grace period elapses.
				killTimer = time.AfterFunc(killAfter, func() {
					reg.Signal(syscall.SIGKILL)
				})
				mu.Unlock()
			case <-done:
				return
			}
		}
	}()

	fired = func() (os.Signal, bool) {
		mu.Lock()
		defer mu.Unlock()
		return firedSig, didFire
	}
	var stopOnce sync.Once
	stop = func() {
		stopOnce.Do(func() {
			signal.Stop(sigCh)
			close(done)
			mu.Lock()
			if killTimer != nil {
				killTimer.Stop()
			}
			mu.Unlock()
		})
	}
	return fired, stop
}

// InterruptExitCode maps a fired interrupt to its shell-conventional exit code
// (128+signum): SIGINT -> 130, SIGTERM -> 143. When fired is false the run was
// not interrupted and this returns 0, letting the tally-derived code stand. When
// fired is true it always returns a non-zero code — the invariant that an
// interrupted run can NEVER exit 0, encoded where it is guardable. A fired signal
// other than INT/TERM cannot occur through InstallSignalHandling's subscription,
// but is mapped to 143 (non-zero, fail-closed) rather than a silent 0.
func InterruptExitCode(sig os.Signal, fired bool) int {
	if !fired {
		return 0
	}
	if sig == syscall.SIGINT {
		return 130
	}
	return 143
}
