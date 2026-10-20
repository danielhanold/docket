package domain

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
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

// ReasonBuiltStackDescendants is the policy reason KillInSnapshot gives for a
// change whose built stacked descendants still depend on its branch.
const ReasonBuiltStackDescendants = "built-stack-descendants"

// ReasonNotDeferrable is the policy reason Defer gives for a change being built.
const ReasonNotDeferrable = "not-deferrable"

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

// ClaimEligibility reports whether c may be claimed against s, or the typed
// refusal naming why not. It is the OTHER half of the claim precondition —
// build-readiness plus an unambiguous current snapshot — and is deliberately
// separate from Claim, which takes no Snapshot: Claim is the pure status
// transition, this is the policy condition the workflow layer evaluates FIRST.
// A caller that claims without calling it claims a change with unmet
// dependencies, no design, or an unresolved stack base.
//
// A held change (on-hold) is eligible: the hold excludes it from autonomous
// selection only, and a claim that names it overrides that.
//
// The refusal keys on EvaluateReadiness: every other non-build-ready outcome becomes
// the stable reason token "not-ready-<kind>", blocked for an unmet precondition
// the backlog can still satisfy and invalid-state for a record that cannot be
// claimed as it stands (not proposed, or an id more than one record claims). A
// subject the snapshot does not hold at all is a bad supplied fact rather than
// a readiness outcome, and is refused as "unknown-change".
func ClaimEligibility(s Snapshot, c Change, facts BranchFacts) *PolicyFailure {
	if _, out := s.Change(c.ID()); out == LookupAbsent {
		return newFailure(c, FailInvalidInput, "unknown-change", nil)
	}
	readiness := EvaluateReadiness(s, c, facts)
	// A hold is a selection-only exclusion: a single-id act names the change
	// deliberately and overrides it, so on-hold is as claimable as build-ready.
	if readiness.Kind == ReadyBuildReady || readiness.Kind == ReadyOnHold {
		return nil
	}
	kind := FailBlocked
	if readiness.Kind == ReadyNotProposed || readiness.Kind == ReadyInvalid {
		kind = FailInvalidState
	}
	return newFailure(c, kind, "not-ready-"+string(readiness.Kind), map[string]string{
		"readiness": string(readiness.Kind),
	})
}

// Claim moves a proposed change to in-progress, recording the minted
// <type>/<slug> branch, or <branch_prefix>/<slug> when the optional per-change
// override is present, the injected timestamp as claimed_at, and reconciled:
// false. A historical "## Run halted" section is reported as an owned removal
// — the domain identifies the marker; the document layer removes it.
//
// Claim guards only what it can see from the record itself: a legal source
// status, a slug that is a usable branch component, and a mint component (the
// override, else the type) that is itself a usable branch component.
// Build-readiness — met dependencies, existing design, a resolved stack base,
// an unambiguous id — is NOT checked here, because it needs the Snapshot this
// signature deliberately does not take; ClaimEligibility is that condition and
// the workflow layer calls it first.
func Claim(c Change, now time.Time) (ActionResult, *PolicyFailure) {
	if fail := requireStatus(c, "claim", StatusProposed); fail != nil {
		return ActionResult{}, fail
	}
	if !ValidSlugToken(c.Slug()) {
		return ActionResult{}, newFailure(c, FailInvalidInput, "invalid-slug", map[string]string{
			"slug": c.Slug(),
		})
	}
	component := c.Type()
	if p := c.BranchPrefix(); p.State == FieldPresent && p.Value != "" {
		component = p.Value
	}
	if !ValidBranchComponent(component) {
		return ActionResult{}, newFailure(c, FailInvalidInput, "invalid-branch-component", map[string]string{
			"component": component,
		})
	}
	b := newChangeBuilder(c)
	b.setStatus(StatusInProgress)
	b.setBranch(MintBranch(c.Type(), c.BranchPrefix(), c.Slug()))
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
// recorded reason.
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

// Defer parks a proposed change and touches status only. A change being built
// is refused with ReasonNotDeferrable: deferring it would carry its branch,
// claim stamp, plan, and results onto a record that revive later returns to
// proposed. It is blocked (to pause, keeping its branch) or killed (to drop)
// instead.
func Defer(c Change) (ActionResult, *PolicyFailure) {
	if c.Status() == StatusInProgress {
		return ActionResult{}, newFailure(c, FailInvalidState, ReasonNotDeferrable,
			map[string]string{"action": "defer", "from": "in-progress"})
	}
	if fail := requireStatus(c, "defer", StatusProposed); fail != nil {
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

// killedResult is the shared kill transition.
func killedResult(c Change) ActionResult {
	b := newChangeBuilder(c)
	b.setStatus(StatusKilled)
	b.clearClaimedAt()
	b.setBranch("")
	return b.result()
}

// KillInSnapshot is Kill with the stack rule the snapshot makes visible: it
// refuses, with ReasonBuiltStackDescendants, to kill a change that still has
// built stacked descendants (implemented or stacked-merged), because their
// code exists only on the change's branch and a kill would strand them with
// no way to finish or kill them. Kill's own source-status check runs first and
// its failure is returned unchanged. A kill never touches a descendant record.
//
// The descendant walk (BuiltStackDescendants) stops at a final (done or
// killed) link: that subtree already resolves to the integration branch or
// to parent-killed, and this kill changes nothing for it, so counting a built
// change beneath it would make the change permanently unkillable.
func KillInSnapshot(s Snapshot, c Change) (ActionResult, *PolicyFailure) {
	result, fail := Kill(c)
	if fail != nil {
		return ActionResult{}, fail
	}
	built := BuiltStackDescendants(s, c.ID())
	if len(built) == 0 {
		return result, nil
	}
	ids := make([]string, len(built))
	for i, id := range built {
		ids[i] = strconv.Itoa(int(id))
	}
	return ActionResult{}, newFailure(c, FailBlocked, ReasonBuiltStackDescendants, map[string]string{
		"action":      "kill",
		"descendants": strings.Join(ids, ","),
	})
}

// BuiltStackDescendants returns the built (implemented or stacked-merged)
// changes stacked transitively on id, in ascending id order, reached only
// through non-final links: a done or killed child is skipped and its subtree
// is never entered. The walk keeps a visited set seeded with id, so a cycle in
// stacked_on terminates. It deliberately does not reuse
// StackDescendantsParentFirst, which walks straight through final links.
func BuiltStackDescendants(s Snapshot, id ChangeID) []ChangeID {
	var built []ChangeID
	visited := map[ChangeID]bool{id: true}
	pending := StackChildren(s, id)
	for len(pending) > 0 {
		next := pending[0]
		pending = pending[1:]
		if visited[next] {
			continue
		}
		visited[next] = true
		child, out := s.Change(next)
		if out != LookupFound || child.Status().Final() {
			continue
		}
		if st := child.Status(); st == StatusImplemented || st == StatusStackedMerged {
			built = append(built, next)
		}
		pending = append(slices.Clone(StackChildren(s, next)), pending...)
	}
	slices.Sort(built)
	return built
}

// MarkImplemented moves an in-progress change to implemented once its PR
// postconditions are supplied: a PR reference, a plan reference, and a
// reconciled record. Each unmet precondition is a blocked failure with its own
// stable reason. The injected time stamps updated: as a date.
func MarkImplemented(c Change, f ImplementedFacts) (ActionResult, *PolicyFailure) {
	if fail := requireStatus(c, "mark-implemented", StatusInProgress); fail != nil {
		return ActionResult{}, fail
	}
	// The implemented transition is the pr ending: a results-artifact change, or
	// one whose stored deliverable is unknown, never takes it.
	if c.Deliverable() != DeliverablePR {
		return ActionResult{}, newFailure(c, FailInvalidState, "not-pr-deliverable", nil)
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
// claim stamp is cleared: an archived record holds no lease.
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

// Complete is the results-artifact ending: it terminates a proposed or
// in-progress change whose deliverable is results-artifact, carries attached
// results, and records no PR, moving it straight to done without a PR. The
// claim stamp is cleared (an archived record holds no lease); the recorded
// branch is kept, so finalize cleanup can find and delete it. A historical
// "## Run halted" section is an owned removal. The deliverable check keys on
// the effective value, so an absent field (pr) and an unknown stored value are
// both refused rather than guessed. updated: is left to the persisting layer.
func Complete(c Change) (ActionResult, *PolicyFailure) {
	if fail := requireStatus(c, "complete", StatusProposed, StatusInProgress); fail != nil {
		return ActionResult{}, fail
	}
	if c.Deliverable() != DeliverableResultsArtifact {
		return ActionResult{}, newFailure(c, FailInvalidState, "not-results-artifact", map[string]string{
			"deliverable": c.RawDeliverable(),
		})
	}
	if c.PR().Value != "" {
		return ActionResult{}, newFailure(c, FailInvalidState, "pr-recorded", map[string]string{
			"pr": c.PR().Value,
		})
	}
	if c.Results().Value == "" {
		return ActionResult{}, newFailure(c, FailBlocked, "results-missing", nil)
	}
	b := newChangeBuilder(c)
	b.setStatus(StatusDone)
	b.clearClaimedAt()
	if c.HasRunHalted() {
		b.own(RunHaltedSection)
	}
	return b.result(), nil
}

// Reclaim is declared in lease.go, alongside the EvaluateReclaim predicate it
// evaluates: the transition is trivial, and the policy that gates it is the
// lease vocabulary.

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
		Deliverable:    c.Deliverable(),
		RawDeliverable: c.RawDeliverable(),
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
		AutoGroomable:  c.AutoGroomable(),
		Hold:           c.Hold(),
		BranchPrefix:   c.BranchPrefix(),
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
	u := now.UTC()
	day := time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
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
