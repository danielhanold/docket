package domain

import (
	"testing"
	"time"
)

// leaseNow is the injected clock reading every lease test evaluates against.
var leaseNow = time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)

// leaseTTL is the TTL every lease test uses unless it says otherwise.
const leaseTTL = 4

// leaseChange builds an in-progress change whose claim stamp is supplied as a
// ready-made OptionalTime, so a test can express absent, empty, malformed, and
// present stamps without going through a parser.
func leaseChange(status Status, stamp OptionalTime, branch string) Change {
	cs := ChangeSpec{
		ID:        7,
		Slug:      "lease-slug",
		Type:      "feat",
		Status:    status,
		RawStatus: string(status),
		ClaimedAt: stamp,
	}
	if branch != "" {
		cs.Branch = OptionalString{State: FieldPresent, Value: branch}
	}
	return NewChange(cs)
}

// leaseStamp renders a present claim stamp offset from leaseNow: a negative
// offset is a claim taken in the past.
func leaseStamp(offset time.Duration) OptionalTime {
	at := leaseNow.Add(offset)
	return OptionalTime{State: FieldPresent, Value: at, Raw: at.Format(time.RFC3339)}
}

// leaseBranches is the branch-existence fact set for the reclaim tests.
func leaseBranches(names ...string) BranchFacts {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return NewBranchFacts(set)
}

func TestEvaluateLeaseStates(t *testing.T) {
	cases := []struct {
		name   string
		status Status
		stamp  OptionalTime
		want   LeaseState
	}{
		{"well under ttl", StatusInProgress, leaseStamp(-time.Hour), LeaseFresh},
		{"one second under ttl", StatusInProgress, leaseStamp(-(leaseTTL*time.Hour - time.Second)), LeaseFresh},
		{"exactly at ttl is fresh", StatusInProgress, leaseStamp(-leaseTTL * time.Hour), LeaseFresh},
		{"one second past ttl", StatusInProgress, leaseStamp(-(leaseTTL*time.Hour + time.Second)), LeaseExpired},
		{"far past ttl", StatusInProgress, leaseStamp(-100 * time.Hour), LeaseExpired},
		{"stamp in the future", StatusInProgress, leaseStamp(2 * time.Hour), LeaseFresh},
		{"absent stamp", StatusInProgress, OptionalTime{}, LeaseMissing},
		{"empty stamp", StatusInProgress, OptionalTime{State: FieldEmpty}, LeaseMissing},
		{"malformed stamp", StatusInProgress, OptionalTime{State: FieldMalformed, Raw: "yesterday"}, LeaseMalformed},
		{"proposed change", StatusProposed, leaseStamp(-100 * time.Hour), LeaseNotInProgress},
		{"blocked change", StatusBlocked, leaseStamp(-100 * time.Hour), LeaseNotInProgress},
		{"done change", StatusDone, OptionalTime{}, LeaseNotInProgress},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateLease(leaseChange(tc.status, tc.stamp, ""), leaseNow, leaseTTL)
			if got != tc.want {
				t.Fatalf("EvaluateLease = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEvaluateLeaseStatusOutranksStamp(t *testing.T) {
	// A terminal record with a malformed stamp is still reported by status:
	// the stamp of a change holding no lease is not a lease diagnosis.
	got := EvaluateLease(leaseChange(StatusKilled, OptionalTime{State: FieldMalformed, Raw: "??"}, ""), leaseNow, leaseTTL)
	if got != LeaseNotInProgress {
		t.Fatalf("EvaluateLease = %q, want %q", got, LeaseNotInProgress)
	}
}

func TestEvaluateReclaimConjuncts(t *testing.T) {
	expired := leaseStamp(-10 * time.Hour)
	fresh := leaseStamp(-time.Hour)

	cases := []struct {
		name         string
		status       Status
		stamp        OptionalTime
		branch       string
		branches     []string
		wantEligible bool
		wantLease    LeaseState
		wantBlocking string
	}{
		{
			name: "all three conjuncts met", status: StatusInProgress, stamp: expired,
			branch: "feat/lease-slug", branches: nil,
			wantEligible: true, wantLease: LeaseExpired,
		},
		{
			name: "no recorded branch and no conventional branch", status: StatusInProgress, stamp: expired,
			branches:     []string{"feat/someone-else"},
			wantEligible: true, wantLease: LeaseExpired,
		},
		{
			name: "recorded branch still exists", status: StatusInProgress, stamp: expired,
			branch: "feat/renamed-branch", branches: []string{"feat/renamed-branch"},
			wantEligible: false, wantLease: LeaseExpired, wantBlocking: "feat/renamed-branch",
		},
		{
			name: "recorded branch blocks regardless of lease age", status: StatusInProgress, stamp: fresh,
			branch: "feat/renamed-branch", branches: []string{"feat/renamed-branch"},
			wantEligible: false, wantLease: LeaseFresh, wantBlocking: "feat/renamed-branch",
		},
		{
			name: "recorded branch absent but conventional branch exists", status: StatusInProgress, stamp: expired,
			branch: "feat/renamed-branch", branches: []string{"feat/lease-slug"},
			wantEligible: false, wantLease: LeaseExpired, wantBlocking: "feat/lease-slug",
		},
		{
			name: "conventional branch exists with no recorded branch", status: StatusInProgress, stamp: expired,
			branches:     []string{"feat/lease-slug"},
			wantEligible: false, wantLease: LeaseExpired, wantBlocking: "feat/lease-slug",
		},
		{
			name: "fresh lease is never eligible", status: StatusInProgress, stamp: fresh,
			wantEligible: false, wantLease: LeaseFresh,
		},
		{
			name: "exact ttl boundary is never eligible", status: StatusInProgress, stamp: leaseStamp(-leaseTTL * time.Hour),
			wantEligible: false, wantLease: LeaseFresh,
		},
		{
			name: "missing stamp is not evidence of expiry", status: StatusInProgress, stamp: OptionalTime{},
			wantEligible: false, wantLease: LeaseMissing,
		},
		{
			name: "malformed stamp is not evidence of expiry", status: StatusInProgress,
			stamp:        OptionalTime{State: FieldMalformed, Raw: "yesterday"},
			wantEligible: false, wantLease: LeaseMalformed,
		},
		{
			name: "a change holding no lease is not reclaimable", status: StatusProposed, stamp: expired,
			wantEligible: false, wantLease: LeaseNotInProgress,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := leaseChange(tc.status, tc.stamp, tc.branch)
			got := EvaluateReclaim(c, leaseNow, leaseTTL, leaseBranches(tc.branches...))
			if got.Eligible != tc.wantEligible {
				t.Errorf("Eligible = %v, want %v", got.Eligible, tc.wantEligible)
			}
			if got.Lease != tc.wantLease {
				t.Errorf("Lease = %q, want %q", got.Lease, tc.wantLease)
			}
			if got.BlockingBranch != tc.wantBlocking {
				t.Errorf("BlockingBranch = %q, want %q", got.BlockingBranch, tc.wantBlocking)
			}
		})
	}
}

func TestReclaimEligibleReturnsProposed(t *testing.T) {
	c := leaseChange(StatusInProgress, leaseStamp(-10*time.Hour), "feat/lease-slug")
	before := fingerprint(c)

	got, fail := Reclaim(c, leaseNow, leaseTTL, leaseBranches())
	if fail != nil {
		t.Fatalf("Reclaim failed: %v", fail)
	}
	if got.Change.Status() != StatusProposed {
		t.Errorf("status = %q, want %q", got.Change.Status(), StatusProposed)
	}
	if b := got.Change.Branch(); b.State != FieldAbsent || b.Value != "" {
		t.Errorf("branch = %+v, want cleared", b)
	}
	if s := got.Change.ClaimedAt(); s.State != FieldAbsent || s.Raw != "" {
		t.Errorf("claimed_at = %+v, want cleared", s)
	}
	if got.Change.Reconciled() {
		t.Error("reconciled = true, want false")
	}
	for _, field := range []string{"status", "branch", "claimed_at"} {
		if !hasField(got, field) {
			t.Errorf("Changed set missing %q: %v", field, changedFields(got))
		}
	}
	if to, ok := fieldTo(got, "branch"); !ok || to != "" {
		t.Errorf("branch change To = %q (present %v), want cleared", to, ok)
	}
	if after := fingerprint(c); after != before {
		t.Errorf("Reclaim mutated its input: %q -> %q", before, after)
	}
}

func TestReclaimRefusals(t *testing.T) {
	cases := []struct {
		name       string
		status     Status
		stamp      OptionalTime
		branch     string
		branches   []string
		wantKind   PolicyFailureKind
		wantReason string
	}{
		{
			name: "fresh lease", status: StatusInProgress, stamp: leaseStamp(-time.Hour),
			wantKind: FailBlocked, wantReason: "lease-not-expired",
		},
		{
			name: "exact ttl boundary", status: StatusInProgress, stamp: leaseStamp(-leaseTTL * time.Hour),
			wantKind: FailBlocked, wantReason: "lease-not-expired",
		},
		{
			name: "missing stamp", status: StatusInProgress, stamp: OptionalTime{},
			wantKind: FailBlocked, wantReason: "missing-claim-stamp",
		},
		{
			name: "malformed stamp", status: StatusInProgress, stamp: OptionalTime{State: FieldMalformed, Raw: "??"},
			wantKind: FailBlocked, wantReason: "malformed-claim-stamp",
		},
		{
			name: "recorded branch still exists", status: StatusInProgress, stamp: leaseStamp(-10 * time.Hour),
			branch: "feat/renamed-branch", branches: []string{"feat/renamed-branch"},
			wantKind: FailBlocked, wantReason: "branch-still-exists",
		},
		{
			name: "conventional branch still exists", status: StatusInProgress, stamp: leaseStamp(-10 * time.Hour),
			branches: []string{"feat/lease-slug"},
			wantKind: FailBlocked, wantReason: "branch-still-exists",
		},
		{
			name: "not in progress", status: StatusProposed, stamp: leaseStamp(-10 * time.Hour),
			wantKind: FailInvalidState, wantReason: "illegal-source-status",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := leaseChange(tc.status, tc.stamp, tc.branch)
			got, fail := Reclaim(c, leaseNow, leaseTTL, leaseBranches(tc.branches...))
			if fail == nil {
				t.Fatalf("Reclaim succeeded, want %s/%s", tc.wantKind, tc.wantReason)
			}
			if fail.Kind != tc.wantKind || fail.Reason != tc.wantReason {
				t.Fatalf("failure = %s/%s, want %s/%s", fail.Kind, fail.Reason, tc.wantKind, tc.wantReason)
			}
			if fail.Change != c.ID() || fail.State != c.Status() {
				t.Errorf("failure identity = %d/%q, want %d/%q", fail.Change, fail.State, c.ID(), c.Status())
			}
			if len(got.Changed) != 0 || len(got.OwnedRemovals) != 0 {
				t.Errorf("refused Reclaim returned an outcome: %+v", got)
			}
		})
	}
}

func TestReclaimBranchFailureNamesTheBranch(t *testing.T) {
	c := leaseChange(StatusInProgress, leaseStamp(-10*time.Hour), "feat/renamed-branch")
	_, fail := Reclaim(c, leaseNow, leaseTTL, leaseBranches("feat/renamed-branch"))
	if fail == nil {
		t.Fatal("Reclaim succeeded, want a blocked failure")
	}
	if fail.Detail["branch"] != "feat/renamed-branch" {
		t.Fatalf("detail branch = %q, want %q", fail.Detail["branch"], "feat/renamed-branch")
	}
}

// mintCandidateChange builds an in-progress, branchless, strictly-expired change
// whose fresh-mint candidate is minted from its type and optional branch_prefix
// — the branch a fresh claim would take, which blockingBranch must probe.
func mintCandidateChange(changeType string, prefix OptionalString) Change {
	return NewChange(ChangeSpec{
		ID:           7,
		Slug:         "lease-slug",
		Type:         changeType,
		Status:       StatusInProgress,
		RawStatus:    string(StatusInProgress),
		ClaimedAt:    leaseStamp(-10 * time.Hour),
		BranchPrefix: prefix,
	})
}

func TestBlockingBranchProbesMintCandidate(t *testing.T) {
	// blockingBranch probes the branch a fresh claim would MINT from
	// type/branch_prefix/slug, not the fixed feat/<slug>: a type-fix change is
	// blocked by a live fix/<slug>; a branch_prefix override moves the candidate;
	// and a live feat/<slug> alone does not block a fix change.
	cases := []struct {
		name         string
		changeType   string
		prefix       OptionalString
		branches     []string
		wantBlocking string
	}{
		{
			name: "type mints the candidate", changeType: "fix",
			branches: []string{"fix/lease-slug"}, wantBlocking: "fix/lease-slug",
		},
		{
			name: "branch_prefix overrides the type", changeType: "fix",
			prefix:   OptionalString{State: FieldPresent, Value: "hotfix"},
			branches: []string{"hotfix/lease-slug"}, wantBlocking: "hotfix/lease-slug",
		},
		{
			name: "feat/<slug> does not block a fix change", changeType: "fix",
			branches: []string{"feat/lease-slug"}, wantBlocking: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := mintCandidateChange(tc.changeType, tc.prefix)
			got := EvaluateReclaim(c, leaseNow, leaseTTL, leaseBranches(tc.branches...))
			if got.BlockingBranch != tc.wantBlocking {
				t.Fatalf("BlockingBranch = %q, want %q", got.BlockingBranch, tc.wantBlocking)
			}
		})
	}
}

func TestBlockingBranchPrefersRecorded(t *testing.T) {
	// A live recorded branch is named ahead of the mint candidate even when the
	// mint candidate (feature/lease-slug) is also live.
	c := NewChange(ChangeSpec{
		ID:        7,
		Slug:      "lease-slug",
		Type:      "feature",
		Status:    StatusInProgress,
		RawStatus: string(StatusInProgress),
		ClaimedAt: leaseStamp(-10 * time.Hour),
		Branch:    OptionalString{State: FieldPresent, Value: "feature/other-name"},
	})
	got := EvaluateReclaim(c, leaseNow, leaseTTL, leaseBranches("feature/other-name", "feature/lease-slug"))
	if got.BlockingBranch != "feature/other-name" {
		t.Fatalf("BlockingBranch = %q, want recorded %q", got.BlockingBranch, "feature/other-name")
	}
}

func TestReclaimPreservesBranchPrefix(t *testing.T) {
	// Reclaiming a branchless expired claim clears the branch but leaves
	// branch_prefix exactly as recorded — the next claimant re-mints from it.
	c := mintCandidateChange("fix", OptionalString{State: FieldPresent, Value: "hotfix"})
	got, fail := Reclaim(c, leaseNow, leaseTTL, leaseBranches())
	if fail != nil {
		t.Fatalf("Reclaim failed: %v", fail)
	}
	if b := got.Change.Branch(); b.State != FieldAbsent || b.Value != "" {
		t.Errorf("branch = %+v, want cleared", b)
	}
	if bp := got.Change.BranchPrefix(); bp.State != FieldPresent || bp.Value != "hotfix" {
		t.Errorf("branch_prefix = %+v, want preserved {Present hotfix}", bp)
	}
}
