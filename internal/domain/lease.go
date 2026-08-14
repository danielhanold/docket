package domain

import "time"

// LeaseState is a claim lease's evaluated condition. It is a closed vocabulary
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

// The stable reasons a reclaim is refused.
const (
	reasonLeaseNotExpired = "lease-not-expired"
	reasonMissingStamp    = "missing-claim-stamp"
	reasonMalformedStamp  = "malformed-claim-stamp"
	reasonBranchExists    = "branch-still-exists"
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

// ReclaimVerdict is the evaluated reclaim predicate: whether the claim may be
// taken back, the lease condition that decided it, and the branch that blocked
// it when one did. BlockingBranch is the empty string when no branch blocked.
type ReclaimVerdict struct {
	Eligible       bool
	Lease          LeaseState
	BlockingBranch string
}

// EvaluateReclaim reports whether c's claim may be reclaimed. Three conjuncts
// must all hold: the record is in-progress, its lease is strictly expired, and
// neither the branch it recorded nor the conventional feat/<slug> branch exists
// among the supplied facts. A live branch is unfinished work whoever left it,
// so it blocks the reclaim independently of the lease age — BlockingBranch is
// populated even for a fresh lease, naming the recorded branch in preference to
// the conventional one.
func EvaluateReclaim(c Change, now time.Time, ttlHours int, facts BranchFacts) ReclaimVerdict {
	lease := EvaluateLease(c, now, ttlHours)
	blocking := blockingBranch(c, facts)
	return ReclaimVerdict{
		Eligible:       lease == LeaseExpired && blocking == "",
		Lease:          lease,
		BlockingBranch: blocking,
	}
}

// blockingBranch returns the name of the live branch that stands in the way of
// reclaiming c, or the empty string when neither candidate exists.
func blockingBranch(c Change, facts BranchFacts) string {
	if recorded := c.Branch(); recorded.State == FieldPresent && recorded.Value != "" {
		if facts.HasBranch(recorded.Value) {
			return recorded.Value
		}
	}
	if conventional := BranchForSlug(c.Slug()); facts.HasBranch(conventional) {
		return conventional
	}
	return ""
}

// Reclaim returns an in-progress change whose claim aged out to proposed,
// clearing the branch it recorded and its claim stamp and marking it
// unreconciled: the next claimant inherits no facts from the abandoned run.
//
// A record that is not in-progress is an illegal source state; every other
// refusal is a blocked failure carrying the verdict's stable reason. A live
// branch is reported ahead of the lease condition because it is the actionable
// one — a fresh lease resolves itself, an unmerged branch does not.
func Reclaim(c Change, now time.Time, ttlHours int, facts BranchFacts) (ActionResult, *PolicyFailure) {
	if fail := requireStatus(c, "reclaim", StatusInProgress); fail != nil {
		return ActionResult{}, fail
	}
	verdict := EvaluateReclaim(c, now, ttlHours, facts)
	if !verdict.Eligible {
		if verdict.BlockingBranch != "" {
			return ActionResult{}, newFailure(c, FailBlocked, reasonBranchExists, map[string]string{
				"branch": verdict.BlockingBranch,
			})
		}
		return ActionResult{}, newFailure(c, FailBlocked, leaseRefusalReason(verdict.Lease), nil)
	}

	b := newChangeBuilder(c)
	b.setStatus(StatusProposed)
	b.setBranch("")
	b.clearClaimedAt()
	b.setReconciled(false)
	return b.result(), nil
}

// leaseRefusalReason maps a non-expired lease onto its stable refusal reason.
func leaseRefusalReason(lease LeaseState) string {
	switch lease {
	case LeaseMissing:
		return reasonMissingStamp
	case LeaseMalformed:
		return reasonMalformedStamp
	default:
		return reasonLeaseNotExpired
	}
}
