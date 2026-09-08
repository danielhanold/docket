package app

import (
	"context"
	"fmt"
	"strconv"

	"github.com/danielhanold/docket/internal/gitcli"
)

// This file is the `finalize.resolver-reserve` operation (change 0349): durable
// reserve-before-dispatch admission for the conflict resolver. On a `conflicted`
// rebase the finalize skill asks this operation for permission before every
// native resolver dispatch. Under the per-workspace operation lock it reloads the
// owned receipt, refuses a receipt it does not own or cannot budget, echoes an
// outstanding reservation as `pending`, refuses a spent budget as `exhausted`,
// and otherwise durably increments the used count and records the reservation
// token bound to the live stopped commit — returning `reserved` ONLY after that
// receipt lands (its crash-safe round-trip is the verification). It launches
// nothing, stages nothing, and touches no Git or metadata: it is a receipt-only
// admission decision. A reserved opportunity is never refunded, so a lost dispatch
// or lost response can only cost an opportunity, never overrun the configured
// bound (ADR context: reserve-before-dispatch in Go because a continue-time check
// cannot bound dispatches).

// OperationFinalizeResolverReserve is the operation key this operation records.
const OperationFinalizeResolverReserve = "finalize.resolver-reserve"

// The closed reservation dispositions (protocol-v1 vocabulary, change 0349). Only
// this operation's result uses `exhausted`; the rebase disposition set does not
// grow (post-continuation exhaustion is rebase `blocked` in Task 7).
const (
	ReserveReserved  = "reserved"  // one dispatch was durably admitted
	ReservePending   = "pending"   // a reservation is already outstanding; admit nothing new
	ReserveExhausted = "exhausted" // the stored budget is spent (reason resolver-budget-exhausted)
	ReserveBlocked   = "blocked"   // a retained precondition refusal; a human is needed
	ReserveContended = "contended" // a lost race the caller resolves by re-reading context
)

// The stable machine reasons the resolver budget introduces (change 0349). Task 7
// reuses ReasonResolverBudgetExhausted as the rebase `blocked` reason a last-
// permitted continuation's next conflict carries, and ReasonResolverBudgetUnavailable
// wherever a budgeted operation refuses a legacy (pre-budget) receipt.
const (
	// ReasonResolverBudgetExhausted marks a reservation refused because the stored
	// per-attempt budget is fully spent.
	ReasonResolverBudgetExhausted = "resolver-budget-exhausted"
	// ReasonResolverBudgetUnavailable marks a budgeted operation refused on a
	// legacy receipt (the whole budget group absent): abort stays available, but a
	// resolver dispatch cannot be reserved.
	ReasonResolverBudgetUnavailable = "resolver-budget-unavailable"
)

// FinalizeReserveGit is the narrow read-only Git seam finalize.resolver-reserve
// probes the live rebase through: the conflict-state probe its non-conflicted
// refusal keys on, and the stopped-commit probe whose full object id the
// reservation records. *gitcli.Client satisfies it; a unit test injects a fake
// that faults exactly one probe to prove the operation refuses (blocked) and never
// increments the used count on an unprovable probe (probe-error-is-not-clean-
// absence). It is nil in production wiring; reserve falls back to the concrete
// Planning.Client via reserveGit.
type FinalizeReserveGit interface {
	RebaseState(ctx context.Context, worktreeDir string) (gitcli.RebaseStatus, error)
	StoppedRebaseCommit(ctx context.Context, worktreeDir string) (gitcli.ObjectID, error)
}

// FinalizeReserveResult is the protocol-v1 document finalize.resolver-reserve
// returns. It mirrors FinalizeRebaseResult's conventions (identity, disposition,
// reason, message, attempt) and adds the reservation token, the stopped commit it
// binds to, and the resolver-budget counts. It holds no authored bytes.
type FinalizeReserveResult struct {
	Envelope
	ID                int    `json:"id,omitempty"`
	Disposition       string `json:"disposition,omitempty"`
	Attempt           string `json:"attempt,omitempty"`
	Reservation       string `json:"reservation,omitempty"`        // opaque reservation token
	StoppedCommit     string `json:"stopped_commit,omitempty"`     // full object id the reservation binds to
	ResolverLimit     int    `json:"resolver_limit,omitempty"`     // the stored per-attempt cap
	ResolverUsed      int    `json:"resolver_used,omitempty"`      // reservations spent so far
	ResolverRemaining int    `json:"resolver_remaining,omitempty"` // limit - used
	Reason            string `json:"reason,omitempty"`
	Message           string `json:"message,omitempty"`
}

// HumanText renders a one-line summary naming identity, disposition, and the
// resolver counts. It never echoes a report body.
func (r FinalizeReserveResult) HumanText() string {
	var s string
	switch r.Result {
	case ResultApplied, ResultNoOp:
		s = fmt.Sprintf("%s: change %04d %s", r.Operation, r.ID, r.Disposition)
	default:
		if r.Reason != "" {
			s = fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
		} else {
			s = fmt.Sprintf("%s: %s", r.Operation, r.Result)
		}
	}
	if r.ResolverLimit > 0 {
		s += fmt.Sprintf(" [resolver %d/%d used, %d remaining]", r.ResolverUsed, r.ResolverLimit, r.ResolverRemaining)
	}
	return s
}

// newReserveResult stamps the envelope for the reserve operation.
func newReserveResult(result Result, out FinalizeReserveResult) FinalizeReserveResult {
	out.Envelope = NewEnvelope(OperationFinalizeResolverReserve, result)
	return out
}

// reserveOutcome builds a reserve result carrying result/disposition/reason/message.
func reserveOutcome(result Result, disposition, reason, message string, id int) FinalizeReserveResult {
	return newReserveResult(result, FinalizeReserveResult{
		ID: id, Disposition: disposition, Reason: reason, Message: message,
	})
}

// withReserveCounts stamps the resolver-budget counts (remaining = limit - used).
func withReserveCounts(out FinalizeReserveResult, limit, used int) FinalizeReserveResult {
	out.ResolverLimit = limit
	out.ResolverUsed = used
	out.ResolverRemaining = limit - used
	return out
}

// reserveFromRebaseRefusal converts a shared rebase refusal (from loadRebaseContext
// or requireOwnedAttempt) into a reserve result, preserving the protocol Result,
// reason, and message and mapping the disposition onto the closed reserve set:
// contended stays contended, everything else is blocked. The operation key is the
// reserve op's, never the rebase op's.
func reserveFromRebaseRefusal(r FinalizeRebaseResult) FinalizeReserveResult {
	disp := ReserveBlocked
	if r.Disposition == RebaseDispContended {
		disp = ReserveContended
	}
	return reserveOutcome(r.Result, disp, r.Reason, r.Message, r.ID)
}

// reserveGit returns the injected reserve Git seam, or the concrete Planning.Client
// when none is wired (production leaves ReserveGit unset).
func reserveGit(deps FinalizeDeps) FinalizeReserveGit {
	if deps.ReserveGit != nil {
		return deps.ReserveGit
	}
	return deps.Planning.Client
}

// newReservationToken mints an opaque, non-empty reservation token from the
// injected clock and the stopped commit, mirroring newRebaseAttempt's shape
// (clock stamp + a short prefix of the bound object id) so a test can drive it
// from the fake clock. The stopped commit distinguishes one conflicting commit's
// reservation from the next.
func newReservationToken(deps FinalizeDeps, stopped gitcli.ObjectID) string {
	stamp := deps.Planning.Clock.Now().UTC().Format("20060102T150405Z")
	short := string(stopped)
	if len(short) > 12 {
		short = short[:12]
	}
	return stamp + "-" + short
}

// FinalizeResolverReserve durably admits at most one conflict-resolver dispatch
// for the owned rebase named by (id, attempt). It runs the whole reload-check-write
// under the per-workspace operation lock so concurrent reservations serialize and
// never double-admit. It launches nothing, stages nothing, and touches no Git or
// metadata beyond the receipt.
func FinalizeResolverReserve(ctx context.Context, deps FinalizeDeps, repoDir string, id int, attempt string) FinalizeReserveResult {
	op := OperationFinalizeResolverReserve

	rc, refusal := loadRebaseContext(ctx, deps, repoDir, op, id)
	if refusal != nil {
		return reserveFromRebaseRefusal(*refusal)
	}
	cid := int(rc.change.ID())

	// Serialize the entire receipt reload-check-write under the per-workspace
	// operation lock: two racing resolver dispatches must admit at most one
	// opportunity (the spec's durable admission clause). The lock is released
	// before returning via defer.
	release, err := deps.Workspace.AcquireOperationLock(rc.metaDir)
	if err != nil {
		return reserveOutcome(ResultExternalFailed, ReserveBlocked, ReasonRebaseWorkspaceProbe,
			"could not acquire the workspace operation lock: "+err.Error(), cid)
	}
	defer release()

	// Reload the receipt under the lock and prove ownership (foreign attempt,
	// missing receipt, and corrupt receipt are the shared owned-attempt refusals).
	rec, rRefusal := requireOwnedAttempt(ctx, deps, op, rc, attempt)
	if rRefusal != nil {
		return reserveFromRebaseRefusal(*rRefusal)
	}

	// Legacy receipt (whole budget group absent): a pre-budget attempt cannot be
	// budgeted retroactively — refuse the reservation; abort stays available
	// elsewhere (prohibition-needs-a-return-value).
	if !rec.HasResolverBudget() {
		return reserveOutcome(ResultBlocked, ReserveBlocked, ReasonResolverBudgetUnavailable,
			"this owned rebase predates the resolver budget; a resolver dispatch cannot be reserved (abort remains available)", cid)
	}
	limit, used, err := rec.ResolverBudget()
	if err != nil {
		return reserveOutcome(ResultBlocked, ReserveBlocked, ReasonResolverBudgetUnavailable,
			"the resolver budget on the owned receipt is unreadable: "+err.Error(), cid)
	}

	// An outstanding reservation means a resolver dispatch is already out: echo it
	// unchanged as `pending` and admit nothing new. The token is the authority
	// (presence-encoded state), so no Git probe is needed to answer pending.
	if rec.ResolverReservationToken != "" {
		out := reserveOutcome(ResultNoOp, ReservePending, "",
			"a resolver dispatch is already reserved for this attempt; feed its report to finalize.rebase-continue", cid)
		out.Attempt = rec.Attempt
		out.Reservation = rec.ResolverReservationToken
		out.StoppedCommit = rec.ResolverReservationStopped
		return withReserveCounts(out, limit, used)
	}

	// A reservation only makes sense against a live conflict. A probe error is
	// never a clean "not stopped": refuse, never adopt a fabricated state.
	git := reserveGit(deps)
	state, err := git.RebaseState(ctx, rc.wsDir)
	if err != nil {
		return reserveOutcome(ResultExternalFailed, ReserveBlocked, ReasonRebaseWorkspaceProbe, err.Error(), cid)
	}
	if state.Disposition != gitcli.RebaseConflicted {
		out := reserveOutcome(ResultBlocked, ReserveBlocked, ReasonRebaseNoConflict,
			fmt.Sprintf("no resolvable conflict is in progress (state %q); there is nothing to reserve", state.Disposition), cid)
		out.Attempt = rec.Attempt
		return withReserveCounts(out, limit, used)
	}

	// Capacity guard BEFORE any increment: a spent budget admits nothing and leaves
	// Git and the receipt untouched. This is the admission cap — no refunds, so a
	// lost dispatch can never overrun the configured bound.
	if used == limit {
		out := reserveOutcome(ResultBlocked, ReserveExhausted, ReasonResolverBudgetExhausted,
			fmt.Sprintf("the resolver budget is spent (%d/%d used); raising finalize.resolver_max_attempts takes effect on the next explicit finalize attempt, not this receipt", used, limit), cid)
		out.Attempt = rec.Attempt
		return withReserveCounts(out, limit, used)
	}

	// The full object id the live rebase is stopped on binds the reservation to a
	// specific conflicting commit (Task 7 verifies it at continue time). A probe
	// error refuses WITHOUT incrementing.
	stopped, err := git.StoppedRebaseCommit(ctx, rc.wsDir)
	if err != nil {
		return reserveOutcome(ResultExternalFailed, ReserveBlocked, ReasonRebaseWorkspaceProbe, err.Error(), cid)
	}

	// Durability BEFORE permission: build the incremented receipt and land it
	// first. WriteRebaseReceipt's crash-safe round-trip guard is the verification;
	// only a receipt that reads back byte-equal authorizes the dispatch. A write
	// failure returns blocked with the used count untouched — no `reserved`.
	token := newReservationToken(deps, stopped)
	updated := rec
	updated.ResolverUsed = strconv.Itoa(used + 1)
	updated.ResolverReservationToken = token
	updated.ResolverReservationStopped = string(stopped)
	// ResolverContinuationStarted stays "": a reservation is not yet a continuation.
	if err := deps.Workspace.WriteRebaseReceipt(ctx, rc.metaDir, updated); err != nil {
		return reserveOutcome(ResultExternalFailed, ReserveBlocked, ReasonRebaseReceiptWrite, err.Error(), cid)
	}

	out := reserveOutcome(ResultApplied, ReserveReserved, "",
		"reserved one resolver dispatch; include the reservation token in the dispatch payload", cid)
	out.Attempt = rec.Attempt
	out.Reservation = token
	out.StoppedCommit = string(stopped)
	return withReserveCounts(out, limit, used+1)
}
