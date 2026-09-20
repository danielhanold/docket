package reposetup

import (
	"reflect"
	"testing"
)

// TestUnmetHealthConditionsHealthyIsEmpty pins the equivalence anchor: the
// healthy fixture yields no unmet condition.
func TestUnmetHealthConditionsHealthyIsEmpty(t *testing.T) {
	if got := UnmetHealthConditions(healthyFacts()); len(got) != 0 {
		t.Fatalf("UnmetHealthConditions(healthy) = %v, want empty", got)
	}
}

// TestUnmetHealthConditionsEachConjunct varies every conjunct of the healthy
// conjunction through a non-satisfying state and asserts exactly that
// condition is reported (spec Verification item 1). Unknown and Absent are
// both non-satisfying for Presence-valued conjuncts.
func TestUnmetHealthConditionsEachConjunct(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Facts)
		want   HealthCondition
	}{
		{"metadata absent", func(f *Facts) { f.RemoteMetadata.Presence = PresenceAbsent }, CondMetadataBranchPresent},
		{"metadata unknown", func(f *Facts) { f.RemoteMetadata.Presence = PresenceUnknown }, CondMetadataBranchPresent},
		{"root unknown", func(f *Facts) { f.MetadataRoot = RootUnknown }, CondMetadataRootVerified},
		{"root foreign", func(f *Facts) { f.MetadataRoot = RootForeign }, CondMetadataRootVerified},
		{"local metadata absent", func(f *Facts) { f.LocalMetadata.Presence = PresenceAbsent }, CondLocalMetadataPresent},
		{"local metadata unknown", func(f *Facts) { f.LocalMetadata.Presence = PresenceUnknown }, CondLocalMetadataPresent},
		{"worktree absent", func(f *Facts) { f.DocketWorktree.Presence = PresenceAbsent }, CondWorktreePresent},
		{"worktree unregistered", func(f *Facts) { f.DocketWorktree.Registered = PresenceAbsent }, CondWorktreeRegistered},
		{"worktree foreign", func(f *Facts) { f.DocketWorktree.Foreign = true }, CondWorktreeNotForeign},
		{"worktree dirty", func(f *Facts) { f.DocketWorktree.Clean = PresenceAbsent }, CondWorktreeClean},
		{"worktree unsynchronized", func(f *Facts) { f.DocketWorktree.Synchronized = PresenceAbsent }, CondWorktreeSynchronized},
		{"hooks enabled", func(f *Facts) { f.DocketWorktree.HooksOff = PresenceAbsent }, CondWorktreeHooksOff},
		{"hooks unknown", func(f *Facts) { f.DocketWorktree.HooksOff = PresenceUnknown }, CondWorktreeHooksOff},
		{"committed ignore invalid", func(f *Facts) { f.CommittedIgnoreBlock = PresenceAbsent }, CondCommittedIgnoreValid},
		{"committed ignore unknown", func(f *Facts) { f.CommittedIgnoreBlock = PresenceUnknown }, CondCommittedIgnoreValid},
		{"live surface present", func(f *Facts) { f.LiveSurface = PresencePresent }, CondLiveSurfaceAbsent},
		{"legacy key present", func(f *Facts) { f.LegacyConfigKey = PresencePresent }, CondLegacyConfigKeyAbsent},
		{"primary dirty", func(f *Facts) { f.PrimaryClean = PresenceAbsent }, CondPrimaryClean},
		{"primary off integration", func(f *Facts) { f.PrimaryOnIntegration = PresenceAbsent }, CondPrimaryOnIntegration},
		{"primary behind tip", func(f *Facts) { f.PrimaryAtRemoteTip = PresenceAbsent }, CondPrimaryAtRemoteTip},
		{"surfaces drift", func(f *Facts) { f.SurfacesAgree = PresenceAbsent }, CondSurfacesAgree},
		{"pending review paths", func(f *Facts) { f.PendingReviewPaths = []string{".gitignore"} }, CondNoPendingReviewPaths},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := healthyFacts()
			tc.mutate(&f)
			got := UnmetHealthConditions(f)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("UnmetHealthConditions = %v, want exactly [%s]", got, tc.want)
			}
		})
	}
}

// TestUnmetHealthConditionsSurfacesUnauthorized: an unauthorized surface
// declaration satisfies the surfaces conjunct regardless of SurfacesAgree,
// exactly like the classifier's `(!f.SurfacesAuthorized || ...)` disjunct.
func TestUnmetHealthConditionsSurfacesUnauthorized(t *testing.T) {
	f := healthyFacts()
	f.SurfacesAuthorized = false
	f.SurfacesAgree = PresenceAbsent
	if got := UnmetHealthConditions(f); len(got) != 0 {
		t.Fatalf("unauthorized surfaces must not be unmet, got %v", got)
	}
}

// TestUnmetHealthConditionsMultipleInFixedOrder: simultaneous failures are
// all reported, in the fixed declaration order.
func TestUnmetHealthConditionsMultipleInFixedOrder(t *testing.T) {
	f := healthyFacts()
	f.DocketWorktree.HooksOff = PresenceAbsent
	f.CommittedIgnoreBlock = PresenceAbsent
	f.PrimaryClean = PresenceAbsent
	want := []HealthCondition{CondWorktreeHooksOff, CondCommittedIgnoreValid, CondPrimaryClean}
	if got := UnmetHealthConditions(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("UnmetHealthConditions = %v, want %v", got, want)
	}
}

// TestClassifyHealthyIffNoUnmetConditions is the drift guard tying the
// classifier's healthy verdict to the helper (learning
// duplicated-gate-copies-the-whole-predicate): for the healthy fixture and
// every single-conjunct mutation above, Classify says healthy exactly when
// UnmetHealthConditions is empty.
func TestClassifyHealthyIffNoUnmetConditions(t *testing.T) {
	probe := func(f Facts) {
		t.Helper()
		healthy := Classify(f).State == StateHealthy
		empty := len(UnmetHealthConditions(f)) == 0
		if healthy != empty {
			t.Fatalf("Classify healthy=%v but unmet-empty=%v for %+v", healthy, empty, f)
		}
	}
	probe(healthyFacts())
	f := healthyFacts()
	f.DocketWorktree.HooksOff = PresenceAbsent
	probe(f)
	g := healthyFacts()
	g.CommittedIgnoreBlock = PresenceUnknown
	probe(g)
}
