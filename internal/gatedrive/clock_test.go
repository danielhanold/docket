package gatedrive

import (
	"testing"
	"time"
)

// TestDeadlineFixedOnce proves the deadline is computed once from the budget:
// it is start+budget and nothing observed later can move it.
func TestDeadlineFixedOnce(t *testing.T) {
	start := time.Unix(1000, 0).UTC()
	dl := computeDeadline(start, 30*time.Minute)
	if !dl.Equal(start.Add(30 * time.Minute)) {
		t.Fatalf("deadline not fixed from budget: got %v want %v", dl, start.Add(30*time.Minute))
	}
}

// TestDeadlineUnchangedByLaterClock proves the fixed deadline persisted on the
// record is not recomputed or extended by advancing the injected clock: later
// Now() values only decide expiry, never move the deadline.
func TestDeadlineUnchangedByLaterClock(t *testing.T) {
	start := time.Unix(1000, 0).UTC()
	budget := 30 * time.Minute
	r := &driveRecord{Deadline: computeDeadline(start, budget), LastClock: start}

	fixed := r.Deadline
	// Simulate several slices advancing an injected clock. The deadline must
	// never change; only expiry flips once the clock passes it.
	for _, delta := range []time.Duration{time.Second, time.Minute, 29 * time.Minute} {
		now := start.Add(delta)
		expired, backward := r.deadlineState(now)
		if backward {
			t.Fatalf("forward advance flagged as backward jump at %v", now)
		}
		if expired {
			t.Fatalf("expired before deadline at %v (deadline %v)", now, r.Deadline)
		}
		if !r.Deadline.Equal(fixed) {
			t.Fatalf("deadline moved: got %v want %v", r.Deadline, fixed)
		}
	}
}

// TestDeadlineForwardJumpExpires proves a forward clock jump past the deadline
// reports expired.
func TestDeadlineForwardJumpExpires(t *testing.T) {
	start := time.Unix(1000, 0).UTC()
	r := &driveRecord{Deadline: computeDeadline(start, 30*time.Minute), LastClock: start.Add(time.Minute)}

	expired, backward := r.deadlineState(start.Add(31 * time.Minute))
	if backward {
		t.Fatalf("forward jump past deadline must not be a backward jump")
	}
	if !expired {
		t.Fatalf("forward jump past deadline must expire")
	}
}

// TestBackwardClockJumpHalts proves a backward clock jump that could lengthen
// the budget is flagged so the caller HALTs rather than silently gaining time.
func TestBackwardClockJumpHalts(t *testing.T) {
	r := &driveRecord{Deadline: time.Unix(2000, 0).UTC(), LastClock: time.Unix(1500, 0).UTC()}
	_, backward := r.deadlineState(time.Unix(1400, 0).UTC())
	if !backward {
		t.Fatalf("backward jump must be flagged")
	}
}

// TestZeroBudgetDeadlineEqualsStart proves a zero budget fixes the deadline at
// the start instant, so the drive takes one observation and then stops and
// halts rather than inventing more time. At start==deadline the state is
// already expired (at-or-after the deadline).
func TestZeroBudgetDeadlineEqualsStart(t *testing.T) {
	start := time.Unix(1000, 0).UTC()
	dl := computeDeadline(start, 0)
	if !dl.Equal(start) {
		t.Fatalf("zero budget deadline must equal start: got %v want %v", dl, start)
	}

	r := &driveRecord{Deadline: dl, LastClock: start}
	expired, backward := r.deadlineState(start)
	if backward {
		t.Fatalf("start observation must not be a backward jump")
	}
	if !expired {
		t.Fatalf("zero-budget deadline must be expired at the start instant")
	}
}

// TestClockInterfaceSatisfiedByMonotonicClock proves the real clock satisfies
// the injectable Clock seam and reports monotonic-consistent elapsed time.
func TestClockInterfaceSatisfiedByMonotonicClock(t *testing.T) {
	var c Clock = systemClock{}
	before := c.Now()
	if c.Since(before) < 0 {
		t.Fatalf("Since must be non-negative for a past instant")
	}
}
