package domain

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"
)

// actNow is the injected clock reading every action test uses.
var actNow = time.Date(2026, 8, 14, 9, 30, 15, 0, time.UTC)

// actSpec is a compact description of one change for the action tests.
type actSpec struct {
	id           ChangeID
	slug         string
	status       Status
	typ          string // change type token; build defaults an empty value to "feat"
	branchPrefix string // per-change branch_prefix override, present when non-empty
	branch       string
	claimedAt    string
	blockedBy    string
	pr           string
	plan         string
	reconciled   bool
	parent       OptionalInt
	runHalted    bool
	dependsOn    []ChangeID
	deliverable  string // stored deliverable text; a non-member is kept raw as unknown
	results      string // attached results path, present when non-empty
}

// build turns an actSpec into an immutable Change.
func (as actSpec) build() Change {
	cs := ChangeSpec{
		ID:           as.id,
		Slug:         as.slug,
		Status:       as.status,
		RawStatus:    string(as.status),
		Type:         as.typ,
		Reconciled:   as.reconciled,
		StackedOn:    as.parent,
		DependsOn:    as.dependsOn,
		HasRunHalted: as.runHalted,
	}
	if as.slug == "" {
		cs.Slug = "a-slug"
	}
	if as.typ == "" {
		cs.Type = "feat"
	}
	if as.branchPrefix != "" {
		cs.BranchPrefix = OptionalString{State: FieldPresent, Value: as.branchPrefix}
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
	if as.results != "" {
		cs.Results = OptionalString{State: FieldPresent, Value: as.results}
	}
	if as.deliverable != "" {
		cs.RawDeliverable = as.deliverable
		cs.Deliverable, _ = ParseDeliverable(as.deliverable)
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
		legal: map[Status]Status{StatusProposed: StatusDeferred},
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
	wantBranch := MintBranch(got.Change.Type(), got.Change.BranchPrefix(), "widget-work")
	if b := got.Change.Branch(); b.State != FieldPresent || b.Value != wantBranch {
		t.Fatalf("branch = %+v, want %q", b, wantBranch)
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

// TestMarkImplementedRefusesNonPRDeliverable: the implemented transition is the
// pr ending, so a results-artifact change or one whose stored deliverable is not
// a known value refuses as invalid state rather than guessing an ending.
func TestMarkImplementedRefusesNonPRDeliverable(t *testing.T) {
	facts := ImplementedFacts{PR: "#8", Plan: "docs/plan.md", Now: actNow}
	for _, d := range []string{string(DeliverableResultsArtifact), "research"} {
		t.Run(d, func(t *testing.T) {
			spec := actSpec{id: 8, slug: "s", status: StatusInProgress, branch: "feat/s", reconciled: true, deliverable: d}
			_, fail := MarkImplemented(spec.build(), facts)
			if fail == nil {
				t.Fatal("expected a policy failure")
			}
			if fail.Kind != FailInvalidState || fail.Reason != "not-pr-deliverable" {
				t.Fatalf("failure = %q/%q, want %q/not-pr-deliverable", fail.Kind, fail.Reason, FailInvalidState)
			}
		})
	}
	for _, d := range []string{"", string(DeliverablePR)} {
		spec := actSpec{id: 8, slug: "s", status: StatusInProgress, branch: "feat/s", reconciled: true, deliverable: d}
		if _, fail := MarkImplemented(spec.build(), facts); fail != nil {
			t.Fatalf("deliverable %q refused: %v", d, fail)
		}
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

// edge is a present stacked_on edge naming id.
func edge(id int) OptionalInt {
	return OptionalInt{State: FieldPresent, Value: id, Raw: strconv.Itoa(id)}
}

// TestKillInSnapshotRefusesOnlyBuiltDescendants: a direct child refuses the
// kill exactly when it is built (implemented or stacked-merged); every other
// child status lets the kill through.
func TestKillInSnapshotRefusesOnlyBuiltDescendants(t *testing.T) {
	for _, st := range allStatuses {
		t.Run(string(st), func(t *testing.T) {
			parent := actSpec{id: 1, slug: "root", status: StatusInProgress, branch: "feat/root"}
			snap := actSnapshot(parent, actSpec{id: 2, slug: "kid", status: st, parent: edge(1)})
			result, fail := KillInSnapshot(snap, parent.build())
			built := st == StatusImplemented || st == StatusStackedMerged
			if built {
				if fail == nil {
					t.Fatal("kill accepted with a built child")
				}
				if fail.Kind != FailBlocked || fail.Reason != ReasonBuiltStackDescendants ||
					fail.Detail["descendants"] != "2" || fail.Detail["action"] != "kill" {
					t.Fatalf("failure = %+v", fail)
				}
				return
			}
			if fail != nil {
				t.Fatalf("kill refused with a %s child: %+v", st, fail)
			}
			if result.Change.Status() != StatusKilled {
				t.Fatalf("status = %q, want killed", result.Change.Status())
			}
		})
	}
}

func TestKillInSnapshotRefusesBuiltGrandchild(t *testing.T) {
	parent := actSpec{id: 1, slug: "root", status: StatusInProgress, branch: "feat/root"}
	snap := actSnapshot(parent,
		actSpec{id: 2, slug: "kid", status: StatusProposed, parent: edge(1)},
		actSpec{id: 3, slug: "grandkid", status: StatusImplemented, parent: edge(2)},
	)
	_, fail := KillInSnapshot(snap, parent.build())
	if fail == nil || fail.Reason != ReasonBuiltStackDescendants || fail.Detail["descendants"] != "3" {
		t.Fatalf("failure = %+v, want built-stack-descendants naming 3", fail)
	}
}

// TestKillInSnapshotPrunesFinalLinks: a built grandchild below a done or
// killed child does not depend on the parent's branch, so it never refuses
// the kill (else the parent would be permanently unkillable).
func TestKillInSnapshotPrunesFinalLinks(t *testing.T) {
	cases := []struct{ child, grandchild Status }{
		{StatusDone, StatusImplemented},
		{StatusKilled, StatusStackedMerged},
	}
	for _, tc := range cases {
		t.Run(string(tc.child), func(t *testing.T) {
			parent := actSpec{id: 1, slug: "root", status: StatusInProgress, branch: "feat/root"}
			snap := actSnapshot(parent,
				actSpec{id: 2, slug: "kid", status: tc.child, parent: edge(1)},
				actSpec{id: 3, slug: "grandkid", status: tc.grandchild, parent: edge(2)},
			)
			if _, fail := KillInSnapshot(snap, parent.build()); fail != nil {
				t.Fatalf("kill refused through a %s link: %+v", tc.child, fail)
			}
		})
	}
}

func TestKillInSnapshotNamesEveryBuiltDescendant(t *testing.T) {
	parent := actSpec{id: 1, slug: "root", status: StatusInProgress, branch: "feat/root"}
	snap := actSnapshot(parent,
		actSpec{id: 4, slug: "kid-merged", status: StatusStackedMerged, parent: edge(1)},
		actSpec{id: 2, slug: "kid-impl", status: StatusImplemented, parent: edge(1)},
		actSpec{id: 5, slug: "kid-proposed", status: StatusProposed, parent: edge(1)},
		actSpec{id: 6, slug: "grandkid", status: StatusImplemented, parent: edge(5)},
	)
	_, fail := KillInSnapshot(snap, parent.build())
	if fail == nil || fail.Detail["descendants"] != "2,4,6" {
		t.Fatalf("failure = %+v, want descendants 2,4,6", fail)
	}
	if got := BuiltStackDescendants(snap, 1); !reflect.DeepEqual(got, []ChangeID{2, 4, 6}) {
		t.Fatalf("BuiltStackDescendants = %v, want [2 4 6]", got)
	}
}

func TestKillInSnapshotProposedParentWithBranch(t *testing.T) {
	parent := actSpec{id: 1, slug: "legacy", status: StatusProposed, branch: "feat/legacy"}
	snap := actSnapshot(parent, actSpec{id: 2, slug: "kid", status: StatusImplemented, parent: edge(1)})
	_, fail := KillInSnapshot(snap, parent.build())
	if fail == nil || fail.Reason != ReasonBuiltStackDescendants || fail.State != StatusProposed {
		t.Fatalf("failure = %+v, want built-stack-descendants from proposed", fail)
	}
}

func TestKillInSnapshotSourceStatusFirst(t *testing.T) {
	parent := actSpec{id: 1, slug: "root", status: StatusImplemented, branch: "feat/root"}
	snap := actSnapshot(parent, actSpec{id: 2, slug: "kid", status: StatusImplemented, parent: edge(1)})
	_, fail := KillInSnapshot(snap, parent.build())
	if fail == nil || fail.Reason != "illegal-source-status" {
		t.Fatalf("failure = %+v, want illegal-source-status", fail)
	}
}

func TestKillInSnapshotTerminatesOnCycle(t *testing.T) {
	parent := actSpec{id: 1, slug: "a", status: StatusInProgress, branch: "feat/a", parent: edge(2)}
	snap := actSnapshot(parent, actSpec{id: 2, slug: "b", status: StatusProposed, parent: edge(1)})
	if _, fail := KillInSnapshot(snap, parent.build()); fail != nil {
		t.Fatalf("kill refused on a cycle with no built descendants: %+v", fail)
	}
}

func TestKillInSnapshotLeavesDescendantsUntouched(t *testing.T) {
	parent := actSpec{id: 1, slug: "root", status: StatusInProgress, branch: "feat/root",
		claimedAt: "2026-08-01T00:00:00Z"}
	snap := actSnapshot(parent, actSpec{id: 2, slug: "kid", status: StatusProposed, parent: edge(1)})
	before := map[ChangeID]string{}
	for _, c := range snap.Changes() {
		before[c.ID()] = fingerprint(c)
	}
	result, fail := KillInSnapshot(snap, parent.build())
	if fail != nil {
		t.Fatalf("KillInSnapshot failed: %v", fail)
	}
	for _, c := range snap.Changes() {
		if got := fingerprint(c); got != before[c.ID()] {
			t.Fatalf("change %d mutated: %q -> %q", c.ID(), before[c.ID()], got)
		}
	}
	if result.Change.ID() != 1 {
		t.Fatalf("result names change %d, want the parent", result.Change.ID())
	}
	for _, f := range changedFields(result) {
		switch f {
		case "status", "claimed_at", "branch":
		default:
			t.Fatalf("Changed names %q; a kill rewrites only the parent's own fields", f)
		}
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

	root, _ := snap.Change(1)
	if _, fail := KillInSnapshot(snap, root); fail != nil {
		t.Fatalf("KillInSnapshot failed: %v", fail)
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
			wantReason: "not-ready-needs-grooming",
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

// A hold excludes a change from selection only; a single-id claim still builds
// it, while any earlier readiness blocker still refuses.
func TestClaimEligibilityAcceptsOnHold(t *testing.T) {
	tests := []eligibilityCase{
		{
			name:    "a held spec-backed change is eligible",
			specs:   []readySpec{{id: 2, status: StatusProposed, spec: specRef("s.md"), hold: holdTrue}},
			subject: 2,
		},
		{
			name: "a held change on an unresolved stack base refuses the claim",
			specs: []readySpec{
				{id: 1, status: StatusProposed, spec: specRef("p.md")},
				{id: 2, status: StatusProposed, spec: specRef("s.md"), parent: parentEdge(1), hold: holdTrue},
			},
			subject:    2,
			wantKind:   FailBlocked,
			wantReason: "not-ready-stack-base-unresolved",
		},
		{
			name: "a held change with an unmet dependency refuses the claim",
			specs: []readySpec{
				{id: 1, status: StatusProposed},
				{id: 2, status: StatusProposed, spec: specRef("s.md"), dependsOn: []ChangeID{1}, hold: holdTrue},
			},
			subject:    2,
			wantKind:   FailBlocked,
			wantReason: "not-ready-waiting-dependency",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := readySnapshot(tt.specs...)
			c, _ := s.Change(tt.subject)
			fail := ClaimEligibility(s, c, remotes(tt.branches...))
			if tt.wantReason == "" {
				if fail != nil {
					t.Fatalf("ClaimEligibility = %v; want eligible", fail)
				}
				return
			}
			if fail == nil || fail.Kind != tt.wantKind || fail.Reason != tt.wantReason {
				t.Fatalf("ClaimEligibility = %v; want %s/%s", fail, tt.wantKind, tt.wantReason)
			}
		})
	}
}

func TestClaimEligibilityDoesNotTransition(t *testing.T) {
	c := NewChange(ChangeSpec{ID: 2, Slug: "a-slug", Status: StatusProposed, Type: "feat"})
	s := NewSnapshot(SnapshotSpec{
		Policy:  RepositoryPolicy{IntegrationBranch: "main"},
		Changes: []Change{c},
	})

	if fail := ClaimEligibility(s, c, remotes()); fail == nil {
		t.Fatal("ClaimEligibility accepted a change that needs design")
	}
	// Claim itself stays a pure status transition: the eligibility condition is
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
			c := NewChange(ChangeSpec{ID: 4, Slug: slug, Status: StatusProposed, Type: "feat"})
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

func TestClaimMintsTypePrefixedBranch(t *testing.T) {
	// A claim mints <type>/<slug>; a non-built-in configured type behaves
	// identically because the mint keys on the token's shape, not a set.
	for _, tc := range []struct{ typ, want string }{
		{"fix", "fix/widget"},
		{"chore", "chore/widget"},
		{"spike", "spike/widget"},
	} {
		t.Run(tc.typ, func(t *testing.T) {
			c := actSpec{id: 20, slug: "widget", status: StatusProposed, typ: tc.typ}.build()
			got, fail := Claim(c, actNow)
			if fail != nil {
				t.Fatalf("Claim failed: %v", fail)
			}
			if b := got.Change.Branch(); b.State != FieldPresent || b.Value != tc.want {
				t.Fatalf("branch = %+v, want %q", b, tc.want)
			}
		})
	}
}

func TestClaimHonorsBranchPrefixOverride(t *testing.T) {
	c := actSpec{
		id: 21, slug: "widget", status: StatusProposed, typ: "fix", branchPrefix: "hotfix",
	}.build()
	got, fail := Claim(c, actNow)
	if fail != nil {
		t.Fatalf("Claim failed: %v", fail)
	}
	if b := got.Change.Branch(); b.State != FieldPresent || b.Value != "hotfix/widget" {
		t.Fatalf("branch = %+v, want hotfix/widget", b)
	}
}

func TestClaimRejectsInvalidBranchComponent(t *testing.T) {
	cases := []struct {
		name string
		spec actSpec
	}{
		{"prefix", actSpec{id: 22, slug: "widget", status: StatusProposed, typ: "fix", branchPrefix: "a/b"}},
		{"type", actSpec{id: 22, slug: "widget", status: StatusProposed, typ: "bad type"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := tc.spec.build()
			before := fingerprint(input)
			_, fail := Claim(input, actNow)
			if fail == nil {
				t.Fatal("Claim accepted an invalid branch component")
			}
			if fail.Kind != FailInvalidInput || fail.Reason != "invalid-branch-component" {
				t.Fatalf("Claim = %s/%s; want %s/invalid-branch-component", fail.Kind, fail.Reason, FailInvalidInput)
			}
			if after := fingerprint(input); after != before {
				t.Fatalf("input mutated: %q -> %q", before, after)
			}
		})
	}
}

// TestCompleteMatrix pins the results-artifact ending: a proposed or
// in-progress results-artifact change carrying results and no PR completes to
// done with its claim stamp cleared and its branch kept; every other source
// status, deliverable, or missing precondition refuses with its stable reason.
func TestCompleteMatrix(t *testing.T) {
	const ra = string(DeliverableResultsArtifact)
	const res = "docs/results/r.md"

	t.Run("proposed completes", func(t *testing.T) {
		got, fail := Complete(actSpec{id: 9, slug: "s", status: StatusProposed, deliverable: ra, results: res}.build())
		if fail != nil {
			t.Fatalf("Complete failed: %v", fail)
		}
		if got.Change.Status() != StatusDone {
			t.Fatalf("status = %q, want done", got.Change.Status())
		}
		if len(got.OwnedRemovals) != 0 {
			t.Fatalf("OwnedRemovals = %v, want none without a run-halted section", got.OwnedRemovals)
		}
		if _, ok := fieldTo(got, "updated"); ok {
			t.Fatalf("Complete must not touch updated: (the app upserts it): %+v", got.Changed)
		}
	})

	for _, halted := range []bool{false, true} {
		t.Run(fmt.Sprintf("in-progress completes (run halted %v)", halted), func(t *testing.T) {
			got, fail := Complete(actSpec{
				id: 9, slug: "s", status: StatusInProgress, branch: "feat/s",
				claimedAt: "2026-08-01T00:00:00Z", deliverable: ra, results: res, runHalted: halted,
			}.build())
			if fail != nil {
				t.Fatalf("Complete failed: %v", fail)
			}
			if got.Change.Status() != StatusDone {
				t.Fatalf("status = %q, want done", got.Change.Status())
			}
			if got.Change.ClaimedAt().State != FieldAbsent {
				t.Fatalf("claimed_at not cleared: %+v", got.Change.ClaimedAt())
			}
			if to, ok := fieldTo(got, "claimed_at"); !ok || to != "" {
				t.Fatalf("Changed claimed_at To = %q, ok=%v", to, ok)
			}
			if got.Change.Branch().Value != "feat/s" {
				t.Fatalf("branch = %+v, want kept", got.Change.Branch())
			}
			if _, ok := fieldTo(got, "branch"); ok {
				t.Fatalf("branch must not be rewritten: %+v", got.Changed)
			}
			want := 0
			if halted {
				want = 1
			}
			if len(got.OwnedRemovals) != want || (halted && got.OwnedRemovals[0] != RunHaltedSection) {
				t.Fatalf("OwnedRemovals = %v, want run-halted only when halted", got.OwnedRemovals)
			}
		})
	}

	for _, st := range []Status{StatusBlocked, StatusDeferred, StatusImplemented, StatusStackedMerged, StatusDone, StatusKilled} {
		t.Run("from "+string(st), func(t *testing.T) {
			_, fail := Complete(actSpec{id: 9, slug: "s", status: st, branch: "feat/s", deliverable: ra, results: res}.build())
			if fail == nil || fail.Kind != FailInvalidState || fail.Reason != "illegal-source-status" {
				t.Fatalf("failure = %+v, want invalid-state/illegal-source-status", fail)
			}
		})
	}

	refusals := []struct {
		name   string
		spec   actSpec
		kind   PolicyFailureKind
		reason string
	}{
		{"pr deliverable", actSpec{id: 9, status: StatusProposed, deliverable: string(DeliverablePR), results: res}, FailInvalidState, "not-results-artifact"},
		{"absent deliverable reads as pr", actSpec{id: 9, status: StatusProposed, results: res}, FailInvalidState, "not-results-artifact"},
		{"unknown deliverable", actSpec{id: 9, status: StatusProposed, deliverable: "research", results: res}, FailInvalidState, "not-results-artifact"},
		{"pr recorded", actSpec{id: 9, status: StatusInProgress, branch: "feat/s", deliverable: ra, results: res, pr: "https://x/pull/1"}, FailInvalidState, "pr-recorded"},
		{"no results", actSpec{id: 9, status: StatusProposed, deliverable: ra}, FailBlocked, "results-missing"},
	}
	for _, r := range refusals {
		t.Run(r.name, func(t *testing.T) {
			_, fail := Complete(r.spec.build())
			if fail == nil || fail.Kind != r.kind || fail.Reason != r.reason {
				t.Fatalf("failure = %+v, want %s/%s", fail, r.kind, r.reason)
			}
		})
	}
}

// TestDeferRefusesInProgressAsNotDeferrable: a change being built cannot be
// deferred; every other non-proposed source keeps illegal-source-status.
func TestDeferRefusesInProgressAsNotDeferrable(t *testing.T) {
	in := actSpec{id: 7, slug: "a-slug", status: StatusInProgress, branch: "feat/a-slug",
		claimedAt: "2026-08-10T00:00:00Z", plan: "docs/plan.md", reconciled: true}.build()
	before := fingerprint(in)
	_, fail := Defer(in)
	if fingerprint(in) != before {
		t.Fatal("input mutated")
	}
	if fail == nil || fail.Kind != FailInvalidState || fail.Reason != ReasonNotDeferrable ||
		fail.State != StatusInProgress || fail.Change != 7 {
		t.Fatalf("failure = %+v, want invalid-state/%s from in-progress", fail, ReasonNotDeferrable)
	}
	for _, st := range []Status{StatusBlocked, StatusDeferred, StatusImplemented, StatusStackedMerged, StatusDone, StatusKilled} {
		if _, f := Defer(actSpec{id: 7, slug: "a-slug", status: st}.build()); f == nil || f.Reason != "illegal-source-status" {
			t.Errorf("defer from %q: failure = %+v, want illegal-source-status", st, f)
		}
	}
}
