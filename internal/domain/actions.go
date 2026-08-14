package domain

import (
	"fmt"
	"time"
)

// PolicyFailureKind classifies why a lifecycle action was refused. The three
// kinds map onto the protocol's invalid-state, blocked, and invalid-input
// outcomes without any caller parsing prose.
type PolicyFailureKind string

// The closed set of policy failure kinds.
const (
	FailInvalidState PolicyFailureKind = "invalid-state" // illegal source status
	FailBlocked      PolicyFailureKind = "blocked"       // unmet precondition
	FailInvalidInput PolicyFailureKind = "invalid-input" // malformed supplied fact
)

// PolicyFailure is the typed refusal an action returns instead of a next
// state. Reason is a stable token — "missing-pr", "empty-block-reason" — never
// prose a caller might match on, and Detail carries the operands a message
// renderer needs. It implements error only so callers can plumb it through
// error-shaped APIs; policy decisions switch on Kind.
type PolicyFailure struct {
	Kind   PolicyFailureKind
	Change ChangeID
	State  Status
	Reason string
	Detail map[string]string
}

// Error renders the failure for plumbing. Callers switch on Kind and Reason.
func (f PolicyFailure) Error() string {
	return fmt.Sprintf("change %d: %s: %s (state %q)", int(f.Change), f.Kind, f.Reason, string(f.State))
}

// FieldChange names one frontmatter field an action rewrote, with its rendered
// before and after values. A cleared field renders as the empty string.
type FieldChange struct {
	Field string
	From  string
	To    string
}

// ActionResult is a successful action's outcome: the next semantic Change plus
// a typed description of what moved. Changed carries only fields whose
// rendered value actually differs, so a semantic no-op reports nothing.
type ActionResult struct {
	Change  Change
	Changed []FieldChange
	// OwnedRemovals names body sections the persisting layer must remove
	// (e.g. "## Run halted" on claim). Never empty strings.
	OwnedRemovals []string
}

// RunHaltedSection is the body section a claim takes ownership of removing.
// The domain only identifies it; the document layer performs the removal.
const RunHaltedSection = "## Run halted"

// StackParentKilledReason is the retained blocked_by reason every live
// descendant of a killed stack parent receives.
const StackParentKilledReason = "stack parent killed — re-scope, re-parent, or kill"

// stampLayout is the second-precision UTC layout for claimed_at.
const stampLayout = time.RFC3339

// dateLayout is the date-only layout for created:/updated:.
const dateLayout = "2006-01-02"

// ImplementedFacts carries the verified postconditions of opening a PR.
type ImplementedFacts struct {
	PR   string
	Plan string
	Now  time.Time
}

// MergeFacts carries the branch a merge verifiably landed on.
type MergeFacts struct {
	VerifiedDestination string
}

// DoneFacts carries the verified reachability of a change's commits from the
// integration branch.
type DoneFacts struct {
	ReachableFromIntegration bool
}

// Claim moves a proposed change to in-progress, recording the deterministic
// feat/<slug> branch, the injected timestamp as claimed_at, and reconciled:
// false. A historical "## Run halted" section is reported as an owned removal
// — the domain identifies the marker; the document layer removes it.
func Claim(c Change, now time.Time) (ActionResult, *PolicyFailure) {
	if fail := requireStatus(c, "claim", StatusProposed); fail != nil {
		return ActionResult{}, fail
	}
	b := newChangeBuilder(c)
	b.setStatus(StatusInProgress)
	b.setBranch(BranchForSlug(c.Slug()))
	b.setClaimedAt(now)
	b.setReconciled(false)
	if c.HasRunHalted() {
		b.own(RunHaltedSection)
	}
	return b.result(), nil
}

// RefreshClaim re-stamps an in-progress change's lease without changing any
// other field.
func RefreshClaim(c Change, now time.Time) (ActionResult, *PolicyFailure) {
	if fail := requireStatus(c, "refresh-claim", StatusInProgress); fail != nil {
		return ActionResult{}, fail
	}
	b := newChangeBuilder(c)
	b.setClaimedAt(now)
	return b.result(), nil
}

// Block moves an in-progress change to blocked, recording a non-empty reason.
// An empty reason is a malformed supplied fact, not an illegal transition.
func Block(c Change, reason string) (ActionResult, *PolicyFailure) {
	if fail := requireStatus(c, "block", StatusInProgress); fail != nil {
		return ActionResult{}, fail
	}
	if reason == "" {
		return ActionResult{}, newFailure(c, FailInvalidInput, "empty-block-reason", nil)
	}
	return blockedResult(c, reason), nil
}

// blockedResult is the shared block transition: blocked status plus the
// recorded reason. KillStackParent reuses it for descendants of any
// non-terminal status, which is why it is separate from Block's own guard.
func blockedResult(c Change, reason string) ActionResult {
	b := newChangeBuilder(c)
	b.setStatus(StatusBlocked)
	b.setBlockedBy(reason)
	return b.result()
}

// Unblock returns a blocked change to in-progress and clears blocked_by.
func Unblock(c Change) (ActionResult, *PolicyFailure) {
	if fail := requireStatus(c, "unblock", StatusBlocked); fail != nil {
		return ActionResult{}, fail
	}
	b := newChangeBuilder(c)
	b.setStatus(StatusInProgress)
	b.setBlockedBy("")
	return b.result(), nil
}

// Defer parks a proposed or in-progress change. It touches status only: a
// deferred change is not terminal, so its recorded branch and claim stamp stay
// readable for whoever revives it.
func Defer(c Change) (ActionResult, *PolicyFailure) {
	if fail := requireStatus(c, "defer", StatusProposed, StatusInProgress); fail != nil {
		return ActionResult{}, fail
	}
	b := newChangeBuilder(c)
	b.setStatus(StatusDeferred)
	return b.result(), nil
}

// Revive returns a deferred change to proposed.
func Revive(c Change) (ActionResult, *PolicyFailure) {
	if fail := requireStatus(c, "revive", StatusDeferred); fail != nil {
		return ActionResult{}, fail
	}
	b := newChangeBuilder(c)
	b.setStatus(StatusProposed)
	return b.result(), nil
}

// Kill terminates a proposed or in-progress change, clearing its claim stamp
// and recorded branch: a killed record holds no lease and owns no branch.
func Kill(c Change) (ActionResult, *PolicyFailure) {
	if fail := requireStatus(c, "kill", StatusProposed, StatusInProgress); fail != nil {
		return ActionResult{}, fail
	}
	return killedResult(c), nil
}

// killedResult is the shared kill transition, reused by KillStackParent.
func killedResult(c Change) ActionResult {
	b := newChangeBuilder(c)
	b.setStatus(StatusKilled)
	b.clearClaimedAt()
	b.setBranch("")
	return b.result()
}

// MarkImplemented moves an in-progress change to implemented once its PR
// postconditions are supplied: a PR reference, a plan reference, and a
// reconciled record. Each unmet precondition is a blocked failure with its own
// stable reason. The injected time stamps updated: as a date.
func MarkImplemented(c Change, f ImplementedFacts) (ActionResult, *PolicyFailure) {
	if fail := requireStatus(c, "mark-implemented", StatusInProgress); fail != nil {
		return ActionResult{}, fail
	}
	switch {
	case f.PR == "":
		return ActionResult{}, newFailure(c, FailBlocked, "missing-pr", nil)
	case f.Plan == "":
		return ActionResult{}, newFailure(c, FailBlocked, "missing-plan", nil)
	case !c.Reconciled():
		return ActionResult{}, newFailure(c, FailBlocked, "not-reconciled", nil)
	}
	b := newChangeBuilder(c)
	b.setStatus(StatusImplemented)
	b.setPR(f.PR)
	b.setPlan(f.Plan)
	b.setUpdated(f.Now)
	return b.result(), nil
}

// MarkStackedMerged moves an implemented change to stacked-merged once the
// merge is verified to have landed on its stack parent's branch. A destination
// that is not the parent branch is an unmet precondition, not a bad fact; an
// unnamed parent branch or destination is a bad fact.
func MarkStackedMerged(c Change, parentBranch string, f MergeFacts) (ActionResult, *PolicyFailure) {
	if fail := requireStatus(c, "mark-stacked-merged", StatusImplemented); fail != nil {
		return ActionResult{}, fail
	}
	switch {
	case parentBranch == "":
		return ActionResult{}, newFailure(c, FailInvalidInput, "empty-parent-branch", nil)
	case f.VerifiedDestination == "":
		return ActionResult{}, newFailure(c, FailInvalidInput, "empty-merge-destination", nil)
	case f.VerifiedDestination != parentBranch:
		return ActionResult{}, newFailure(c, FailBlocked, "destination-mismatch", map[string]string{
			"expected": parentBranch,
			"actual":   f.VerifiedDestination,
		})
	}
	b := newChangeBuilder(c)
	b.setStatus(StatusStackedMerged)
	return b.result(), nil
}

// MarkDone terminates an implemented or stacked-merged change once
// reachability from the integration branch is supplied as a verified fact. The
// claim stamp is cleared: a terminal record holds no lease.
func MarkDone(c Change, f DoneFacts) (ActionResult, *PolicyFailure) {
	if fail := requireStatus(c, "mark-done", StatusImplemented, StatusStackedMerged); fail != nil {
		return ActionResult{}, fail
	}
	if !f.ReachableFromIntegration {
		return ActionResult{}, newFailure(c, FailBlocked, "not-reachable-from-integration", nil)
	}
	b := newChangeBuilder(c)
	b.setStatus(StatusDone)
	b.clearClaimedAt()
	return b.result(), nil
}

// DescendantOutcome is one descendant's share of a stack kill. NoOp marks a
// descendant that was already blocked: it is reported so the caller can
// account for it, with its unchanged record and an empty Changed set.
type DescendantOutcome struct {
	ID     ChangeID
	Result ActionResult
	NoOp   bool
}

// StackKillResult is the outcome of killing a stack parent: the parent's kill
// plus one outcome per live descendant, parent-first.
type StackKillResult struct {
	Parent      ActionResult
	Descendants []DescendantOutcome
}

// KillStackParent kills a change and blocks every non-terminal descendant with
// the retained re-scope, re-parent, or kill reason. It is a distinct graph
// action rather than a widening of Block: a descendant is blocked from any
// non-terminal status, an already-blocked one is a semantic no-op, and a
// terminal (done or killed) descendant is not touched and not reported.
func KillStackParent(s Snapshot, id ChangeID) (StackKillResult, *PolicyFailure) {
	parent, out := s.Change(id)
	switch out {
	case LookupAmbiguous:
		return StackKillResult{}, &PolicyFailure{
			Kind: FailInvalidInput, Change: id, Reason: "ambiguous-change",
		}
	case LookupAbsent:
		return StackKillResult{}, &PolicyFailure{
			Kind: FailInvalidInput, Change: id, Reason: "unknown-change",
		}
	}
	killed, fail := Kill(parent)
	if fail != nil {
		return StackKillResult{}, fail
	}

	result := StackKillResult{Parent: killed}
	for _, descendantID := range StackDescendantsParentFirst(s, id) {
		descendant, out := s.Change(descendantID)
		if out != LookupFound || descendant.Status().Terminal() {
			continue
		}
		if descendant.Status() == StatusBlocked {
			result.Descendants = append(result.Descendants, DescendantOutcome{
				ID: descendantID, Result: ActionResult{Change: descendant}, NoOp: true,
			})
			continue
		}
		result.Descendants = append(result.Descendants, DescendantOutcome{
			ID: descendantID, Result: blockedResult(descendant, StackParentKilledReason),
		})
	}
	return result, nil
}

// Reclaim is declared by Task 9 alongside the reclaim predicate it evaluates.

// requireStatus refuses c unless its status is one of the legal source states,
// naming the attempted action in the failure detail.
func requireStatus(c Change, action string, legal ...Status) *PolicyFailure {
	for _, s := range legal {
		if c.Status() == s {
			return nil
		}
	}
	return newFailure(c, FailInvalidState, "illegal-source-status", map[string]string{
		"action": action,
		"from":   string(c.Status()),
	})
}

// newFailure builds a failure carrying the subject's identity and state.
func newFailure(c Change, kind PolicyFailureKind, reason string, detail map[string]string) *PolicyFailure {
	return &PolicyFailure{
		Kind:   kind,
		Change: c.ID(),
		State:  c.Status(),
		Reason: reason,
		Detail: detail,
	}
}

// changeBuilder accumulates a next-state ChangeSpec and the typed record of
// what moved. It is seeded from the input's accessors, which hand back fresh
// collections, so neither the input Change nor the result can reach the other.
type changeBuilder struct {
	spec     ChangeSpec
	changed  []FieldChange
	removals []string
}

// newChangeBuilder seeds a builder from c's accessors.
func newChangeBuilder(c Change) *changeBuilder {
	return &changeBuilder{spec: ChangeSpec{
		ID:             c.ID(),
		Slug:           c.Slug(),
		Title:          c.Title(),
		Status:         c.Status(),
		RawStatus:      c.RawStatus(),
		Priority:       c.Priority(),
		RawPriority:    c.RawPriority(),
		Type:           c.Type(),
		Created:        c.Created(),
		Updated:        c.Updated(),
		DependsOn:      c.DependsOn(),
		StackedOn:      c.StackedOn(),
		Related:        c.Related(),
		DiscoveredFrom: c.DiscoveredFrom(),
		ADRs:           c.ADRs(),
		Spec:           c.Spec(),
		Plan:           c.Plan(),
		Results:        c.Results(),
		Trivial:        c.Trivial(),
		Branch:         c.Branch(),
		ClaimedAt:      c.ClaimedAt(),
		PR:             c.PR(),
		Issue:          c.Issue(),
		BlockedBy:      c.BlockedBy(),
		Reconciled:     c.Reconciled(),
		Location:       c.Location(),
		Path:           c.Path(),
		ArchiveDate:    c.ArchiveDate(),

		HasRunHalted:        c.HasRunHalted(),
		HasAutoGroomBlocked: c.HasAutoGroomBlocked(),
		HasFinalizeBlocked:  c.HasFinalizeBlocked(),
		HasPublishDeferred:  c.HasPublishDeferred(),
	}}
}

// result finalizes the builder into an immutable next-state ActionResult.
func (b *changeBuilder) result() ActionResult {
	return ActionResult{
		Change:        NewChange(b.spec),
		Changed:       b.changed,
		OwnedRemovals: b.removals,
	}
}

// record appends a field change, skipping a rewrite that moved nothing.
func (b *changeBuilder) record(field, from, to string) {
	if from == to {
		return
	}
	b.changed = append(b.changed, FieldChange{Field: field, From: from, To: to})
}

// own registers a body section the persisting layer must remove.
func (b *changeBuilder) own(section string) {
	if section == "" {
		return
	}
	b.removals = append(b.removals, section)
}

func (b *changeBuilder) setStatus(next Status) {
	b.record("status", string(b.spec.Status), string(next))
	b.spec.Status = next
	b.spec.RawStatus = string(next)
}

func (b *changeBuilder) setBranch(name string) { b.setOptionalString("branch", &b.spec.Branch, name) }
func (b *changeBuilder) setPR(ref string)      { b.setOptionalString("pr", &b.spec.PR, ref) }
func (b *changeBuilder) setPlan(ref string)    { b.setOptionalString("plan", &b.spec.Plan, ref) }
func (b *changeBuilder) setBlockedBy(r string) {
	b.setOptionalString("blocked_by", &b.spec.BlockedBy, r)
}

// setOptionalString writes value into an optional text field, clearing the
// field to absent when value is empty.
func (b *changeBuilder) setOptionalString(field string, target *OptionalString, value string) {
	next := OptionalString{State: FieldPresent, Value: value}
	if value == "" {
		next = OptionalString{}
	}
	b.record(field, target.Value, next.Value)
	*target = next
}

// setClaimedAt stamps the lease at second-precision UTC.
func (b *changeBuilder) setClaimedAt(now time.Time) {
	stamp := now.UTC().Truncate(time.Second)
	next := OptionalTime{State: FieldPresent, Value: stamp, Raw: stamp.Format(stampLayout)}
	b.record("claimed_at", b.spec.ClaimedAt.Raw, next.Raw)
	b.spec.ClaimedAt = next
}

// clearClaimedAt drops the lease stamp entirely.
func (b *changeBuilder) clearClaimedAt() {
	b.record("claimed_at", b.spec.ClaimedAt.Raw, "")
	b.spec.ClaimedAt = OptionalTime{}
}

// setUpdated stamps the date-only updated: field from the injected time.
func (b *changeBuilder) setUpdated(now time.Time) {
	day := now.UTC().Truncate(time.Second)
	next := OptionalTime{State: FieldPresent, Value: day, Raw: day.Format(dateLayout)}
	b.record("updated", b.spec.Updated.Raw, next.Raw)
	b.spec.Updated = next
}

// setReconciled writes the reconciled flag.
func (b *changeBuilder) setReconciled(v bool) {
	b.record("reconciled", boolText(b.spec.Reconciled), boolText(v))
	b.spec.Reconciled = v
}

// boolText renders a boolean frontmatter value.
func boolText(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
