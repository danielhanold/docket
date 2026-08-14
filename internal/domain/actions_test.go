package domain

import (
	"testing"
	"time"
)

// actNow is the injected clock reading every action test uses.
var actNow = time.Date(2026, 8, 14, 9, 30, 15, 0, time.UTC)

// actSpec is a compact description of one change for the action tests.
type actSpec struct {
	id         ChangeID
	slug       string
	status     Status
	branch     string
	claimedAt  string
	blockedBy  string
	pr         string
	plan       string
	reconciled bool
	parent     OptionalInt
	runHalted  bool
	dependsOn  []ChangeID
}

// build turns an actSpec into an immutable Change.
func (as actSpec) build() Change {
	cs := ChangeSpec{
		ID:           as.id,
		Slug:         as.slug,
		Status:       as.status,
		RawStatus:    string(as.status),
		Reconciled:   as.reconciled,
		StackedOn:    as.parent,
		DependsOn:    as.dependsOn,
		HasRunHalted: as.runHalted,
	}
	if as.slug == "" {
		cs.Slug = "a-slug"
	}
	if as.branch != "" {
		cs.Branch = OptionalString{State: FieldPresent, Value: as.branch}
	}
	if as.claimedAt != "" {
		stamp, err := time.Parse(time.RFC3339, as.claimedAt)
		if err != nil {
			panic(err)
		}
		cs.ClaimedAt = OptionalTime{State: FieldPresent, Value: stamp, Raw: as.claimedAt}
	}
	if as.blockedBy != "" {
		cs.BlockedBy = OptionalString{State: FieldPresent, Value: as.blockedBy}
	}
	if as.pr != "" {
		cs.PR = OptionalString{State: FieldPresent, Value: as.pr}
	}
	if as.plan != "" {
		cs.Plan = OptionalString{State: FieldPresent, Value: as.plan}
	}
	return NewChange(cs)
}

// actSnapshot builds a snapshot with "main" as the integration branch.
func actSnapshot(specs ...actSpec) Snapshot {
	changes := make([]Change, 0, len(specs))
	for _, as := range specs {
		changes = append(changes, as.build())
	}
	return NewSnapshot(SnapshotSpec{
		Policy:  RepositoryPolicy{IntegrationBranch: "main"},
		Changes: changes,
	})
}

// allStatuses is the closed status set, used to drive the source-state matrix.
var allStatuses = []Status{
	StatusProposed, StatusInProgress, StatusBlocked, StatusDeferred,
	StatusImplemented, StatusStackedMerged, StatusDone, StatusKilled,
}

// fingerprint renders every field an action could touch, so a test can prove
// the input Change was not mutated.
func fingerprint(c Change) string {
	return string(c.Status()) + "|" + c.RawStatus() + "|" +
		c.Branch().Value + "|" + string(rune('0'+int(c.Branch().State))) + "|" +
		c.ClaimedAt().Raw + "|" + string(rune('0'+int(c.ClaimedAt().State))) + "|" +
		c.BlockedBy().Value + "|" + string(rune('0'+int(c.BlockedBy().State))) + "|" +
		c.PR().Value + "|" + c.Plan().Value + "|" + c.Updated().Raw + "|" +
		map[bool]string{true: "T", false: "F"}[c.Reconciled()]
}

// changedFields returns the Field names of a Changed set, in order.
func changedFields(r ActionResult) []string {
	fields := make([]string, 0, len(r.Changed))
	for _, fc := range r.Changed {
		fields = append(fields, fc.Field)
	}
	return fields
}

// hasField reports whether the Changed set carries a field.
func hasField(r ActionResult, field string) bool {
	for _, fc := range r.Changed {
		if fc.Field == field {
			return true
		}
	}
	return false
}

// fieldTo returns the To value recorded for field, and whether it was present.
func fieldTo(r ActionResult, field string) (string, bool) {
	for _, fc := range r.Changed {
		if fc.Field == field {
			return fc.To, true
		}
	}
	return "", false
}

// actionCase describes one action for the full source-state matrix: the legal
// source statuses mapped to the status the action lands on.
type actionCase struct {
	name  string
	apply func(Change) (ActionResult, *PolicyFailure)
	legal map[Status]Status
}

// matrixActions supplies every action with facts that satisfy its
// preconditions, so an illegal row can only fail on the source state.
var matrixActions = []actionCase{
	{
		name:  "Claim",
		apply: func(c Change) (ActionResult, *PolicyFailure) { return Claim(c, actNow) },
		legal: map[Status]Status{StatusProposed: StatusInProgress},
	},
	{
		name:  "RefreshClaim",
		apply: func(c Change) (ActionResult, *PolicyFailure) { return RefreshClaim(c, actNow) },
		legal: map[Status]Status{StatusInProgress: StatusInProgress},
	},
	{
		name:  "Block",
		apply: func(c Change) (ActionResult, *PolicyFailure) { return Block(c, "waiting on review") },
		legal: map[Status]Status{StatusInProgress: StatusBlocked},
	},
	{
		name:  "Unblock",
		apply: Unblock,
		legal: map[Status]Status{StatusBlocked: StatusInProgress},
	},
	{
		name:  "Defer",
		apply: Defer,
		legal: map[Status]Status{StatusProposed: StatusDeferred, StatusInProgress: StatusDeferred},
	},
	{
		name:  "Revive",
		apply: Revive,
		legal: map[Status]Status{StatusDeferred: StatusProposed},
	},
	{
		name:  "Kill",
		apply: Kill,
		legal: map[Status]Status{StatusProposed: StatusKilled, StatusInProgress: StatusKilled},
	},
	{
		name: "MarkImplemented",
		apply: func(c Change) (ActionResult, *PolicyFailure) {
			return MarkImplemented(c, ImplementedFacts{PR: "#42", Plan: "docs/plan.md", Now: actNow})
		},
		legal: map[Status]Status{StatusInProgress: StatusImplemented},
	},
	{
		name: "MarkStackedMerged",
		apply: func(c Change) (ActionResult, *PolicyFailure) {
			return MarkStackedMerged(c, "feat/parent", MergeFacts{VerifiedDestination: "feat/parent"})
		},
		legal: map[Status]Status{StatusImplemented: StatusStackedMerged},
	},
	{
		name: "MarkDone",
		apply: func(c Change) (ActionResult, *PolicyFailure) {
			return MarkDone(c, DoneFacts{ReachableFromIntegration: true})
		},
		legal: map[Status]Status{StatusImplemented: StatusDone, StatusStackedMerged: StatusDone},
	},
}

// TestActionSourceStateMatrix drives every action from every one of the eight
// statuses: legal rows land on the expected status, illegal rows return
// FailInvalidState naming the source state, and no input is ever mutated.
func TestActionSourceStateMatrix(t *testing.T) {
	for _, ac := range matrixActions {
		for _, from := range allStatuses {
			t.Run(ac.name+"/"+string(from), func(t *testing.T) {
				input := actSpec{
					id:         7,
					slug:       "a-slug",
					status:     from,
					branch:     "feat/a-slug",
					claimedAt:  "2026-08-10T00:00:00Z",
					pr:         "#7",
					plan:       "docs/plan.md",
					reconciled: true,
				}.build()
				before := fingerprint(input)

				got, fail := ac.apply(input)

				if after := fingerprint(input); after != before {
					t.Fatalf("input mutated: %q -> %q", before, after)
				}
				want, legal := ac.legal[from]
				switch {
				case legal && fail != nil:
					t.Fatalf("legal transition failed: %v", fail)
				case legal:
					if got.Change.Status() != want {
						t.Fatalf("status = %q, want %q", got.Change.Status(), want)
					}
					if !hasField(got, "status") && from != want {
						t.Fatalf("Changed set omits status: %v", changedFields(got))
					}
				case fail == nil:
					t.Fatalf("illegal transition from %q succeeded: %q", from, got.Change.Status())
				default:
					if fail.Kind != FailInvalidState {
						t.Fatalf("Kind = %q, want %q", fail.Kind, FailInvalidState)
					}
					if fail.State != from {
						t.Fatalf("State = %q, want %q", fail.State, from)
					}
					if fail.Change != 7 {
						t.Fatalf("Change = %d, want 7", fail.Change)
					}
					if fail.Reason == "" {
						t.Fatal("Reason is empty")
					}
					if fail.Error() == "" {
						t.Fatal("Error() is empty")
					}
				}
			})
		}
	}
}

func TestClaimSetsBranchStampAndReconciled(t *testing.T) {
	input := actSpec{id: 12, slug: "widget-work", status: StatusProposed, reconciled: true}.build()

	got, fail := Claim(input, actNow)
	if fail != nil {
		t.Fatalf("Claim failed: %v", fail)
	}
	if got.Change.Status() != StatusInProgress {
		t.Fatalf("status = %q", got.Change.Status())
	}
	if b := got.Change.Branch(); b.State != FieldPresent || b.Value != BranchForSlug("widget-work") {
		t.Fatalf("branch = %+v, want %q", b, BranchForSlug("widget-work"))
	}
	stamp := got.Change.ClaimedAt()
	if stamp.State != FieldPresent || stamp.Raw != "2026-08-14T09:30:15Z" {
		t.Fatalf("claimed_at = %+v, want the injected now", stamp)
	}
	if !stamp.Value.Equal(actNow) {
		t.Fatalf("claimed_at value = %v, want %v", stamp.Value, actNow)
	}
	if got.Change.Reconciled() {
		t.Fatal("reconciled should be cleared by a claim")
	}
	for _, field := range []string{"status", "branch", "claimed_at", "reconciled"} {
		if !hasField(got, field) {
			t.Fatalf("Changed set omits %q: %v", field, changedFields(got))
		}
	}
	if len(got.OwnedRemovals) != 0 {
		t.Fatalf("OwnedRemovals = %v, want none without a run-halted section", got.OwnedRemovals)
	}
}

func TestClaimOwnsRunHaltedRemovalOnlyWhenPresent(t *testing.T) {
	halted := actSpec{id: 3, slug: "s", status: StatusProposed, runHalted: true}.build()
	got, fail := Claim(halted, actNow)
	if fail != nil {
		t.Fatalf("Claim failed: %v", fail)
	}
	if len(got.OwnedRemovals) != 1 || got.OwnedRemovals[0] != RunHaltedSection {
		t.Fatalf("OwnedRemovals = %v, want [%q]", got.OwnedRemovals, RunHaltedSection)
	}
	for _, name := range got.OwnedRemovals {
		if name == "" {
			t.Fatal("OwnedRemovals carries an empty string")
		}
	}
}

func TestRefreshClaimRestampsOnly(t *testing.T) {
	input := actSpec{
		id: 5, slug: "s", status: StatusInProgress,
		branch: "feat/s", claimedAt: "2026-08-01T00:00:00Z",
	}.build()

	got, fail := RefreshClaim(input, actNow)
	if fail != nil {
		t.Fatalf("RefreshClaim failed: %v", fail)
	}
	if got.Change.Status() != StatusInProgress {
		t.Fatalf("status = %q", got.Change.Status())
	}
	if fields := changedFields(got); len(fields) != 1 || fields[0] != "claimed_at" {
		t.Fatalf("Changed = %v, want only claimed_at", fields)
	}
	if to, _ := fieldTo(got, "claimed_at"); to != "2026-08-14T09:30:15Z" {
		t.Fatalf("claimed_at To = %q", to)
	}
	if got.Change.Branch().Value != "feat/s" {
		t.Fatal("refresh must not touch the branch")
	}
}

func TestBlockRequiresNonEmptyReason(t *testing.T) {
	input := actSpec{id: 9, slug: "s", status: StatusInProgress}.build()

	_, fail := Block(input, "")
	if fail == nil {
		t.Fatal("Block with an empty reason succeeded")
	}
	if fail.Kind != FailInvalidInput {
		t.Fatalf("Kind = %q, want %q", fail.Kind, FailInvalidInput)
	}
	if fail.Reason != "empty-block-reason" {
		t.Fatalf("Reason = %q", fail.Reason)
	}

	got, fail := Block(input, "waiting on 0306")
	if fail != nil {
		t.Fatalf("Block failed: %v", fail)
	}
	if got.Change.Status() != StatusBlocked {
		t.Fatalf("status = %q", got.Change.Status())
	}
	if b := got.Change.BlockedBy(); b.State != FieldPresent || b.Value != "waiting on 0306" {
		t.Fatalf("blocked_by = %+v", b)
	}
	if !hasField(got, "blocked_by") {
		t.Fatalf("Changed omits blocked_by: %v", changedFields(got))
	}
}

func TestUnblockClearsReason(t *testing.T) {
	input := actSpec{id: 9, slug: "s", status: StatusBlocked, blockedBy: "waiting"}.build()

	got, fail := Unblock(input)
	if fail != nil {
		t.Fatalf("Unblock failed: %v", fail)
	}
	if b := got.Change.BlockedBy(); b.State != FieldAbsent || b.Value != "" {
		t.Fatalf("blocked_by = %+v, want cleared", b)
	}
	to, ok := fieldTo(got, "blocked_by")
	if !ok || to != "" {
		t.Fatalf("Changed blocked_by To = %q, ok=%v", to, ok)
	}
}

func TestKillClearsClaimAndBranch(t *testing.T) {
	input := actSpec{
		id: 4, slug: "s", status: StatusInProgress,
		branch: "feat/s", claimedAt: "2026-08-01T00:00:00Z",
	}.build()

	got, fail := Kill(input)
	if fail != nil {
		t.Fatalf("Kill failed: %v", fail)
	}
	if got.Change.Status() != StatusKilled {
		t.Fatalf("status = %q", got.Change.Status())
	}
	if got.Change.ClaimedAt().State != FieldAbsent || got.Change.Branch().State != FieldAbsent {
		t.Fatalf("kill must clear claimed_at and branch: %+v %+v",
			got.Change.ClaimedAt(), got.Change.Branch())
	}
	for _, field := range []string{"claimed_at", "branch"} {
		to, ok := fieldTo(got, field)
		if !ok || to != "" {
			t.Fatalf("Changed %q To = %q, ok=%v", field, to, ok)
		}
	}
}

func TestMarkImplementedPreconditions(t *testing.T) {
	base := actSpec{id: 8, slug: "s", status: StatusInProgress, branch: "feat/s", reconciled: true}

	tests := []struct {
		name       string
		reconciled bool
		facts      ImplementedFacts
		wantReason string
	}{
		{
			name: "missing pr", reconciled: true,
			facts:      ImplementedFacts{Plan: "docs/plan.md", Now: actNow},
			wantReason: "missing-pr",
		},
		{
			name: "missing plan", reconciled: true,
			facts:      ImplementedFacts{PR: "#8", Now: actNow},
			wantReason: "missing-plan",
		},
		{
			name: "not reconciled", reconciled: false,
			facts:      ImplementedFacts{PR: "#8", Plan: "docs/plan.md", Now: actNow},
			wantReason: "not-reconciled",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := base
			spec.reconciled = tc.reconciled
			_, fail := MarkImplemented(spec.build(), tc.facts)
			if fail == nil {
				t.Fatal("expected a policy failure")
			}
			if fail.Kind != FailBlocked {
				t.Fatalf("Kind = %q, want %q", fail.Kind, FailBlocked)
			}
			if fail.Reason != tc.wantReason {
				t.Fatalf("Reason = %q, want %q", fail.Reason, tc.wantReason)
			}
		})
	}

	got, fail := MarkImplemented(base.build(), ImplementedFacts{PR: "#8", Plan: "docs/plan.md", Now: actNow})
	if fail != nil {
		t.Fatalf("MarkImplemented failed: %v", fail)
	}
	if got.Change.Status() != StatusImplemented {
		t.Fatalf("status = %q", got.Change.Status())
	}
	if got.Change.PR().Value != "#8" || got.Change.Plan().Value != "docs/plan.md" {
		t.Fatalf("pr/plan not recorded: %+v %+v", got.Change.PR(), got.Change.Plan())
	}
	if u := got.Change.Updated(); u.State != FieldPresent || u.Raw != "2026-08-14" {
		t.Fatalf("updated = %+v, want the injected date", u)
	}
	// Value must round-trip with Raw: the date-only field parses to midnight UTC.
	if v := got.Change.Updated().Value; !v.Equal(time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("updated value = %v, want midnight UTC of the injected date", v)
	}
}

func TestMarkStackedMergedRequiresParentDestination(t *testing.T) {
	input := actSpec{id: 11, slug: "s", status: StatusImplemented, branch: "feat/s"}.build()

	_, fail := MarkStackedMerged(input, "feat/parent", MergeFacts{VerifiedDestination: "main"})
	if fail == nil {
		t.Fatal("mismatched destination succeeded")
	}
	if fail.Kind != FailBlocked || fail.Reason != "destination-mismatch" {
		t.Fatalf("failure = %+v", fail)
	}
	if fail.Detail["expected"] != "feat/parent" || fail.Detail["actual"] != "main" {
		t.Fatalf("Detail = %v", fail.Detail)
	}

	_, fail = MarkStackedMerged(input, "", MergeFacts{VerifiedDestination: "main"})
	if fail == nil || fail.Kind != FailInvalidInput {
		t.Fatalf("empty parent branch: %+v", fail)
	}
	_, fail = MarkStackedMerged(input, "feat/parent", MergeFacts{})
	if fail == nil || fail.Kind != FailInvalidInput {
		t.Fatalf("empty destination: %+v", fail)
	}

	got, fail := MarkStackedMerged(input, "feat/parent", MergeFacts{VerifiedDestination: "feat/parent"})
	if fail != nil {
		t.Fatalf("MarkStackedMerged failed: %v", fail)
	}
	if got.Change.Status() != StatusStackedMerged {
		t.Fatalf("status = %q", got.Change.Status())
	}
}

func TestMarkDoneRequiresReachability(t *testing.T) {
	for _, from := range []Status{StatusImplemented, StatusStackedMerged} {
		input := actSpec{
			id: 2, slug: "s", status: from,
			branch: "feat/s", claimedAt: "2026-08-01T00:00:00Z",
		}.build()

		if _, fail := MarkDone(input, DoneFacts{}); fail == nil {
			t.Fatalf("%s: unreachable change marked done", from)
		} else if fail.Kind != FailBlocked || fail.Reason != "not-reachable-from-integration" {
			t.Fatalf("%s: failure = %+v", from, fail)
		}

		got, fail := MarkDone(input, DoneFacts{ReachableFromIntegration: true})
		if fail != nil {
			t.Fatalf("%s: MarkDone failed: %v", from, fail)
		}
		if got.Change.Status() != StatusDone {
			t.Fatalf("%s: status = %q", from, got.Change.Status())
		}
		if got.Change.ClaimedAt().State != FieldAbsent {
			t.Fatal("a done record must carry no claim stamp")
		}
	}
}

func TestKillStackParentBlocksLiveDescendants(t *testing.T) {
	parentEdgeOn := func(id int) OptionalInt {
		return OptionalInt{State: FieldPresent, Value: id, Raw: "0" + string(rune('0'+id))}
	}
	snap := actSnapshot(
		actSpec{id: 1, slug: "root", status: StatusInProgress, branch: "feat/root"},
		actSpec{id: 2, slug: "kid-proposed", status: StatusProposed, parent: parentEdgeOn(1)},
		actSpec{id: 3, slug: "kid-blocked", status: StatusBlocked, blockedBy: "already", parent: parentEdgeOn(1)},
		actSpec{id: 4, slug: "kid-done", status: StatusDone, parent: parentEdgeOn(1)},
		actSpec{id: 5, slug: "grandkid", status: StatusInProgress, parent: parentEdgeOn(2)},
	)

	got, fail := KillStackParent(snap, 1)
	if fail != nil {
		t.Fatalf("KillStackParent failed: %v", fail)
	}
	if got.Parent.Change.Status() != StatusKilled {
		t.Fatalf("parent status = %q", got.Parent.Change.Status())
	}

	byID := map[ChangeID]DescendantOutcome{}
	for _, d := range got.Descendants {
		byID[d.ID] = d
	}
	if _, touched := byID[4]; touched {
		t.Fatalf("done descendant was touched: %+v", got.Descendants)
	}
	if len(byID) != 3 {
		t.Fatalf("descendants = %+v, want 2, 3, and 5", got.Descendants)
	}

	for _, id := range []ChangeID{2, 5} {
		d := byID[id]
		if d.NoOp {
			t.Fatalf("%d: live descendant reported as a no-op", id)
		}
		if d.Result.Change.Status() != StatusBlocked {
			t.Fatalf("%d: status = %q", id, d.Result.Change.Status())
		}
		if d.Result.Change.BlockedBy().Value != StackParentKilledReason {
			t.Fatalf("%d: blocked_by = %q, want %q",
				id, d.Result.Change.BlockedBy().Value, StackParentKilledReason)
		}
	}

	already := byID[3]
	if !already.NoOp {
		t.Fatal("already-blocked descendant should be a no-op")
	}
	if already.Result.Changed != nil {
		t.Fatalf("no-op carries Changed = %v", already.Result.Changed)
	}
	if already.Result.Change.Status() != StatusBlocked || already.Result.Change.BlockedBy().Value != "already" {
		t.Fatalf("no-op rewrote the descendant: %+v", already.Result.Change)
	}
}

func TestKillStackParentLookupFailures(t *testing.T) {
	dup := actSnapshot(
		actSpec{id: 1, slug: "a", status: StatusProposed},
		actSpec{id: 1, slug: "b", status: StatusProposed},
	)
	_, fail := KillStackParent(dup, 1)
	if fail == nil || fail.Kind != FailInvalidInput || fail.Reason != "ambiguous-change" {
		t.Fatalf("ambiguous parent: %+v", fail)
	}

	empty := actSnapshot()
	_, fail = KillStackParent(empty, 1)
	if fail == nil || fail.Kind != FailInvalidInput || fail.Reason != "unknown-change" {
		t.Fatalf("unknown parent: %+v", fail)
	}
}

func TestKillStackParentRejectsIllegalParentState(t *testing.T) {
	snap := actSnapshot(actSpec{id: 1, slug: "root", status: StatusDone})
	_, fail := KillStackParent(snap, 1)
	if fail == nil || fail.Kind != FailInvalidState {
		t.Fatalf("done parent: %+v", fail)
	}
}

func TestActionsNeverMutateSnapshotChanges(t *testing.T) {
	parent := OptionalInt{State: FieldPresent, Value: 1, Raw: "1"}
	snap := actSnapshot(
		actSpec{id: 1, slug: "root", status: StatusInProgress},
		actSpec{id: 2, slug: "kid", status: StatusProposed, parent: parent},
	)
	before := map[ChangeID]string{}
	for _, c := range snap.Changes() {
		before[c.ID()] = fingerprint(c)
	}

	if _, fail := KillStackParent(snap, 1); fail != nil {
		t.Fatalf("KillStackParent failed: %v", fail)
	}
	for _, c := range snap.Changes() {
		if got := fingerprint(c); got != before[c.ID()] {
			t.Fatalf("change %d mutated: %q -> %q", c.ID(), before[c.ID()], got)
		}
	}
}

func TestPolicyFailureIsAnError(t *testing.T) {
	var err error = &PolicyFailure{
		Kind: FailBlocked, Change: 3, State: StatusInProgress, Reason: "missing-pr",
	}
	if err.Error() == "" {
		t.Fatal("Error() is empty")
	}
}

func TestActionsPreserveUnrelatedFields(t *testing.T) {
	input := actSpec{
		id: 6, slug: "s", status: StatusProposed,
		dependsOn: []ChangeID{1, 2},
	}.build()

	got, fail := Claim(input, actNow)
	if fail != nil {
		t.Fatalf("Claim failed: %v", fail)
	}
	deps := got.Change.DependsOn()
	if len(deps) != 2 || deps[0] != 1 || deps[1] != 2 {
		t.Fatalf("depends_on = %v, want [1 2]", deps)
	}
	deps[0] = 99
	if again := got.Change.DependsOn(); again[0] != 1 {
		t.Fatal("the next-state Change shares its dependency slice with a caller")
	}
	if again := input.DependsOn(); again[0] != 1 {
		t.Fatal("the input Change shares its dependency slice with the result")
	}
}

// eligibilityCase is one row of the claim-eligibility matrix: the snapshot's
// records, the remote branches, the subject, and the refusal (or its absence)
// ClaimEligibility must report.
type eligibilityCase struct {
	name       string
	specs      []readySpec
	branches   []string
	subject    ChangeID
	absent     bool // evaluate a subject the snapshot does not hold
	wantKind   PolicyFailureKind
	wantReason string
}

func TestClaimEligibility(t *testing.T) {
	tests := []eligibilityCase{
		{
			name: "a build-ready change is eligible",
			specs: []readySpec{
				{id: 2, status: StatusProposed, spec: specRef("s.md")},
			},
			subject: 2,
		},
		{
			name: "an unmet dependency refuses the claim",
			specs: []readySpec{
				{id: 1, status: StatusProposed},
				{id: 2, status: StatusProposed, spec: specRef("s.md"), dependsOn: []ChangeID{1}},
			},
			subject:    2,
			wantKind:   FailBlocked,
			wantReason: "not-ready-waiting-dependency",
		},
		{
			name: "a change still needing design refuses the claim",
			specs: []readySpec{
				{id: 2, status: StatusProposed},
			},
			subject:    2,
			wantKind:   FailBlocked,
			wantReason: "not-ready-needs-brainstorm",
		},
		{
			name: "an auto-groom-blocked change refuses the claim under its own token",
			specs: []readySpec{
				{id: 2, status: StatusProposed, groomBlocked: true},
			},
			subject:    2,
			wantKind:   FailBlocked,
			wantReason: "not-ready-auto-groom-blocked",
		},
		{
			name: "an unresolved stack base refuses the claim",
			specs: []readySpec{
				{id: 1, status: StatusKilled, branch: "feat/one"},
				{id: 2, status: StatusProposed, spec: specRef("s.md"), parent: parentEdge(1)},
			},
			branches:   []string{"feat/one"},
			subject:    2,
			wantKind:   FailBlocked,
			wantReason: "not-ready-stack-base-unresolved",
		},
		{
			name: "an ambiguous id refuses the claim",
			specs: []readySpec{
				{id: 2, status: StatusProposed, spec: specRef("s.md")},
				{id: 2, status: StatusProposed, spec: specRef("s.md")},
			},
			subject:    2,
			wantKind:   FailInvalidState,
			wantReason: "not-ready-invalid",
		},
		{
			name: "a non-proposed change refuses the claim",
			specs: []readySpec{
				{id: 2, status: StatusInProgress, spec: specRef("s.md")},
			},
			subject:    2,
			wantKind:   FailInvalidState,
			wantReason: "not-ready-not-proposed",
		},
		{
			name: "a subject the snapshot does not hold refuses the claim",
			specs: []readySpec{
				{id: 2, status: StatusProposed, spec: specRef("s.md")},
			},
			subject:    7,
			absent:     true,
			wantKind:   FailInvalidInput,
			wantReason: "unknown-change",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := readySnapshot(tt.specs...)
			c := readySpec{id: tt.subject, status: StatusProposed, spec: specRef("s.md")}.build()
			if !tt.absent {
				for _, rp := range tt.specs {
					if rp.id == tt.subject {
						c = rp.build()
						break
					}
				}
			}

			fail := ClaimEligibility(s, c, remotes(tt.branches...))
			if tt.wantReason == "" {
				if fail != nil {
					t.Fatalf("ClaimEligibility = %v; want eligible", fail)
				}
				return
			}
			if fail == nil {
				t.Fatalf("ClaimEligibility = nil; want %s/%s", tt.wantKind, tt.wantReason)
			}
			if fail.Kind != tt.wantKind || fail.Reason != tt.wantReason {
				t.Fatalf("ClaimEligibility = %s/%s; want %s/%s",
					fail.Kind, fail.Reason, tt.wantKind, tt.wantReason)
			}
			if fail.Change != tt.subject {
				t.Fatalf("failure Change = %d; want %d", fail.Change, tt.subject)
			}
		})
	}
}

func TestClaimEligibilityDoesNotTransition(t *testing.T) {
	c := NewChange(ChangeSpec{ID: 2, Slug: "a-slug", Status: StatusProposed})
	s := NewSnapshot(SnapshotSpec{
		Policy:  RepositoryPolicy{IntegrationBranch: "main"},
		Changes: []Change{c},
	})

	if fail := ClaimEligibility(s, c, remotes()); fail == nil {
		t.Fatal("ClaimEligibility accepted a change that needs design")
	}
	// Claim itself stays a pure status transition: the eligibility conjunct is
	// the workflow layer's to call, so Claim still succeeds here.
	if _, fail := Claim(c, actNow); fail != nil {
		t.Fatalf("Claim failed: %v", fail)
	}
}

func TestClaimRefusesUnusableSlug(t *testing.T) {
	bad := []string{"", "Upper", "with space", "-leading", "trailing-", "dou--ble", "under_score"}
	for _, slug := range bad {
		t.Run("slug="+slug, func(t *testing.T) {
			c := NewChange(ChangeSpec{ID: 4, Slug: slug, Status: StatusProposed})
			_, fail := Claim(c, actNow)
			if fail == nil {
				t.Fatalf("Claim accepted slug %q", slug)
			}
			if fail.Kind != FailInvalidInput || fail.Reason != "invalid-slug" {
				t.Fatalf("Claim = %s/%s; want %s/invalid-slug", fail.Kind, fail.Reason, FailInvalidInput)
			}
		})
	}

	for _, slug := range []string{"a-slug", "2fa-support", "x"} {
		t.Run("ok="+slug, func(t *testing.T) {
			c := NewChange(ChangeSpec{ID: 4, Slug: slug, Status: StatusProposed})
			got, fail := Claim(c, actNow)
			if fail != nil {
				t.Fatalf("Claim(%q) failed: %v", slug, fail)
			}
			if branch := got.Change.Branch().Value; branch != "feat/"+slug {
				t.Fatalf("branch = %q; want %q", branch, "feat/"+slug)
			}
		})
	}
}
