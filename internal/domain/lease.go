package domain

import "time"

// LeaseState is a claim lease's evaluated condition. It takes a fixed set of allowed values
// rather than a Boolean "stale" because the three non-fresh conditions are not
// interchangeable: an expired lease is positive evidence that a claim aged out,
// while a missing or malformed stamp is evidence of nothing at all — a record
// whose stamp cannot be read must never be treated as having aged out.
type LeaseState string

// The closed set of lease states.
const (
	LeaseFresh         LeaseState = "fresh"           // claimed within the TTL
	LeaseExpired       LeaseState = "expired"         // claimed strictly longer ago than the TTL
	LeaseMissing       LeaseState = "missing"         // no claim stamp recorded
	LeaseMalformed     LeaseState = "malformed"       // stamp present but unparseable
	LeaseNotInProgress LeaseState = "not-in-progress" // the record holds no lease
)

// EvaluateLease reports the condition of c's claim lease against the injected
// now and a TTL in hours.
//
// Status outranks the stamp: only an in-progress record holds a lease, so any
// other status reports not-in-progress whatever its stamp says. A stamp that is
// absent or present-but-empty is missing; one that is present but unparseable
// is malformed. Neither is expiry.
//
// The TTL boundary is strict: elapsed time must exceed the TTL to expire, so a
// lease evaluated at exactly its deadline is still fresh. A stamp in the future
// is fresh for the same reason — its elapsed time does not exceed the TTL.
func EvaluateLease(c Change, now time.Time, ttlHours int) LeaseState {
	if c.Status() != StatusInProgress {
		return LeaseNotInProgress
	}
	stamp := c.ClaimedAt()
	switch stamp.State {
	case FieldMalformed:
		return LeaseMalformed
	case FieldPresent:
		if now.Sub(stamp.Value) > time.Duration(ttlHours)*time.Hour {
			return LeaseExpired
		}
		return LeaseFresh
	default: // FieldAbsent, FieldEmpty — no stamp to read
		return LeaseMissing
	}
}
