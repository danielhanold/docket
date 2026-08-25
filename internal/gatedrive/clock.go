// Clock, deadline, and backward-jump logic for the gate driver.
//
// Time enters the driver through one injectable seam so tests can drive
// slices and deadlines deterministically without sleeping for production
// durations. Two independent notions of time are bound here, matching the
// spec's "Deadline semantics":
//
//   - While a single driver process is alive, a slice is bounded using Go's
//     monotonic clock (systemClock reads it; time.Since carries the monotonic
//     reading), so a wall-clock adjustment cannot shorten or lengthen a slice
//     that is already in flight.
//   - Across separate invocations there is no shared monotonic reading, so the
//     persisted UTC deadline (fixed once at Start) and the last-accepted
//     wall-clock value bind elapsed time. A forward jump past the deadline
//     expires the drive; a backward jump below the last-accepted clock could
//     lengthen the effective budget, so it is flagged for the caller to HALT
//     rather than silently granting more time.
package gatedrive

import "time"

// Clock is the injectable time seam. Now reports the current instant carrying a
// monotonic reading where the platform supports one; Since measures elapsed
// time from an earlier Now, using that monotonic reading when present. Tests
// substitute a deterministic implementation.
type Clock interface {
	Now() time.Time
	Since(time.Time) time.Duration
}

// systemClock is the production Clock backed by the real monotonic wall clock.
type systemClock struct{}

// Now returns the current instant, including a monotonic reading on platforms
// that provide one.
func (systemClock) Now() time.Time { return time.Now() }

// Since returns the time elapsed since t, using the monotonic reading captured
// by an earlier Now when both instants carry one.
func (systemClock) Since(t time.Time) time.Duration { return time.Since(t) }

// computeDeadline fixes the absolute deadline exactly once from the resolved
// observation budget: start + budget. It is a pure function of its inputs, so a
// later clock reading can never move the returned deadline. A budget of zero
// yields a deadline equal to start, preserving the "one observation then
// stop-and-halt" contract for a zero budget. The result is normalized to UTC so
// the persisted deadline is comparable across invocations that read the wall
// clock in different zones.
func computeDeadline(start time.Time, budget time.Duration) time.Time {
	return start.Add(budget).UTC()
}

// deadlineState reports how the fixed deadline stands against a freshly observed
// wall-clock instant now, given the last-accepted clock value persisted on the
// record.
//
//   - backwardJump is true when now precedes the last-accepted clock: the wall
//     clock ran backward, which could lengthen the effective budget, so the
//     caller must HALT rather than trust the reading.
//   - expired is true when now is at or after the fixed Deadline. The at-or-after
//     boundary makes a zero-budget drive (Deadline == start) expired at the very
//     first observation, matching the stop-and-halt contract, while a normal
//     budget leaves the first in-window slice unexpired.
//
// The deadline itself is never mutated here; only the fixed Deadline and the
// caller-supplied now decide the answer.
func (r *driveRecord) deadlineState(now time.Time) (expired bool, backwardJump bool) {
	if now.Before(r.LastClock) {
		backwardJump = true
	}
	if !now.Before(r.Deadline) {
		expired = true
	}
	return expired, backwardJump
}
