package domain

import (
	"strings"
	"testing"
)

func TestParseStatus(t *testing.T) {
	tests := []struct {
		in   string
		want Status
		ok   bool
	}{
		{"proposed", StatusProposed, true},
		{"in-progress", StatusInProgress, true},
		{"blocked", StatusBlocked, true},
		{"deferred", StatusDeferred, true},
		{"implemented", StatusImplemented, true},
		{"stacked-merged", StatusStackedMerged, true},
		{"done", StatusDone, true},
		{"killed", StatusKilled, true},
		{"Proposed", "", false},
		{"", "", false},
		{"open", "", false},
	}
	for _, tc := range tests {
		got, ok := ParseStatus(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("ParseStatus(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestStatusFinal(t *testing.T) {
	tests := []struct {
		in   Status
		want bool
	}{
		{StatusDone, true},
		{StatusKilled, true},
		{StatusProposed, false},
		{StatusInProgress, false},
		{StatusBlocked, false},
		{StatusDeferred, false},
		{StatusImplemented, false},
		{StatusStackedMerged, false},
	}
	for _, tc := range tests {
		if got := tc.in.Final(); got != tc.want {
			t.Errorf("%q.Final() = %v; want %v", tc.in, got, tc.want)
		}
	}
}

func TestParsePriority(t *testing.T) {
	tests := []struct {
		in   string
		want Priority
		ok   bool
	}{
		{"critical", PriorityCritical, true},
		{"high", PriorityHigh, true},
		{"medium", PriorityMedium, true},
		{"low", PriorityLow, true},
		{"Critical", "", false},
		{"", "", false},
		{"urgent", "", false},
	}
	for _, tc := range tests {
		got, ok := ParsePriority(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("ParsePriority(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestPriorityRank(t *testing.T) {
	tests := []struct {
		in   Priority
		want int
	}{
		{PriorityCritical, 0},
		{PriorityHigh, 1},
		{PriorityMedium, 2},
		{PriorityLow, 3},
		{Priority("urgent"), 2},
		{Priority(""), 2},
	}
	for _, tc := range tests {
		if got := priorityRank(tc.in); got != tc.want {
			t.Errorf("priorityRank(%q) = %d; want %d", tc.in, got, tc.want)
		}
	}
}

func TestParseADRStatus(t *testing.T) {
	tests := []struct {
		in   string
		want ADRStatus
		ok   bool
	}{
		{"Accepted", ADRStatus{Kind: ADRAccepted}, true},
		{"Deprecated", ADRStatus{Kind: ADRDeprecated}, true},
		{"Superseded by ADR-0071", ADRStatus{Kind: ADRSupersededBy, Ref: 71}, true},
		{"Reversed by ADR-0042", ADRStatus{Kind: ADRReversedBy, Ref: 42}, true},
		{"superseded by ADR-1", ADRStatus{}, false},
		{"Superseded by ADR-", ADRStatus{}, false},
		{"Superseded by 71", ADRStatus{}, false},
		{"Accepted ", ADRStatus{}, false},
		{"Superseded by ADR-0", ADRStatus{}, false},
		{"", ADRStatus{}, false},
		{"Superseded by ADR-00x1", ADRStatus{}, false},
	}
	for _, tc := range tests {
		got, ok := ParseADRStatus(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("ParseADRStatus(%q) = %+v, %v; want %+v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestADRStatusString(t *testing.T) {
	tests := []struct {
		in   ADRStatus
		want string
	}{
		{ADRStatus{Kind: ADRAccepted}, "Accepted"},
		{ADRStatus{Kind: ADRDeprecated}, "Deprecated"},
		{ADRStatus{Kind: ADRSupersededBy, Ref: 71}, "Superseded by ADR-0071"},
		{ADRStatus{Kind: ADRReversedBy, Ref: 42}, "Reversed by ADR-0042"},
	}
	for _, tc := range tests {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("%+v.String() = %q; want %q", tc.in, got, tc.want)
		}
		round, ok := ParseADRStatus(tc.want)
		if !ok || round != tc.in {
			t.Errorf("ParseADRStatus(%q) = %+v, %v; want round-trip %+v", tc.want, round, ok, tc.in)
		}
	}
}

func TestParsePromotionState(t *testing.T) {
	tests := []struct {
		in   string
		want PromotionState
		ok   bool
	}{
		{"retained", PromotionRetained, true},
		{"candidate", PromotionCandidate, true},
		{"promoted", PromotionPromoted, true},
		{"", PromotionRetained, true},
		{"graduated", "", false},
		{"Retained", "", false},
	}
	for _, tc := range tests {
		got, ok := ParsePromotionState(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("ParsePromotionState(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestValidTypeToken(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"feat", true},
		{"fix2", true},
		{"a-b", true},
		{"a", true},
		{"", false},
		{"Feat", false},
		{"2fix", false},
		{"-a", false},
		{"a_b", false},
		{"a b", false},
	}
	for _, tc := range tests {
		if got := ValidTypeToken(tc.in); got != tc.want {
			t.Errorf("ValidTypeToken(%q) = %v; want %v", tc.in, got, tc.want)
		}
	}
}

func TestValidBranchComponent(t *testing.T) {
	valid := []string{"feat", "fix", "chore", "hotfix", "feature", "a", "release-2"}
	invalid := []string{
		"", "feat/", "refs", "refs/heads", "a/b", "-lead", ".lead",
		"has space", "has\ttab", "a..b", "a@{b", "a~b", "a^b", "a:b",
		"a?b", "a*b", "a[b", "a\\z", "end.lock", "end.",
	}
	for _, s := range valid {
		if !ValidBranchComponent(s) {
			t.Errorf("ValidBranchComponent(%q) = false, want true", s)
		}
	}
	for _, s := range invalid {
		if ValidBranchComponent(s) {
			t.Errorf("ValidBranchComponent(%q) = true, want false", s)
		}
	}
}

func TestMintBranch(t *testing.T) {
	if got := MintBranch("fix", OptionalString{}, "my-slug"); got != "fix/my-slug" {
		t.Fatalf("type mint = %q, want fix/my-slug", got)
	}
	if got := MintBranch("fix", OptionalString{State: FieldPresent, Value: "hotfix"}, "my-slug"); got != "hotfix/my-slug" {
		t.Fatalf("prefix mint = %q, want hotfix/my-slug", got)
	}
	if got := MintBranch("chore", OptionalString{State: FieldPresent, Value: ""}, "s"); got != "chore/s" {
		t.Fatalf("empty prefix mint = %q, want chore/s", got)
	}
}

func TestNormalizeBranchPrefix(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		ok   bool
	}{
		{"hotfix", "hotfix", true},
		{"  hotfix  ", "hotfix", true},
		{"hotfix/", "hotfix", true},
		{"Hotfix", "hotfix", true},
		{"HOTFIX/", "hotfix", true},
		{" Hotfix/ ", "hotfix", true},
		{"hotfix//", "", false},
		{"/hotfix", "", false},
		{"team/hotfix", "", false},
		{"refs", "", false},
		{"REFS", "", false},
		{"refs/heads/x", "", false},
		{"-x", "", false},
		{"x.lock", "", false},
		{"hot fix", "", false},
		{"", "", true},
		{"   ", "", true},
		{"/", "", true},
	}
	for _, c := range cases {
		got, ok := NormalizeBranchPrefix(c.raw)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizeBranchPrefix(%q) = (%q, %v), want (%q, %v)", c.raw, got, ok, c.want, c.ok)
		}
	}
}

// FuzzNormalizeBranchPrefix ties the normalizer to the reader it feeds: every
// accepted non-empty output must pass claim-time ValidBranchComponent, be
// lowercase, and be a fixed point — so a stored prefix can never fail at claim
// and a retry of the stored value replays.
func FuzzNormalizeBranchPrefix(f *testing.F) {
	for _, s := range []string{"hotfix", " Hotfix/ ", "a/b", "refs", "", "/", "x.lock", "HOTFIX//"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		got, ok := NormalizeBranchPrefix(raw)
		if !ok {
			if got != "" {
				t.Fatalf("refused %q but returned %q", raw, got)
			}
			return
		}
		if got == "" {
			return
		}
		if !ValidBranchComponent(got) {
			t.Fatalf("normalized %q -> %q fails claim-time ValidBranchComponent", raw, got)
		}
		if got != strings.ToLower(got) {
			t.Fatalf("normalized %q -> %q is not lowercase", raw, got)
		}
		if again, ok2 := NormalizeBranchPrefix(got); !ok2 || again != got {
			t.Fatalf("not idempotent: %q -> %q -> (%q, %v)", raw, got, again, ok2)
		}
	})
}

func TestParseDeliverable(t *testing.T) {
	for _, s := range []string{"pr", "results-artifact"} {
		if d, ok := ParseDeliverable(s); !ok || string(d) != s {
			t.Errorf("ParseDeliverable(%q) = %q,%v", s, d, ok)
		}
	}
	for _, s := range []string{"", "PR", "results_artifact", " pr"} {
		if _, ok := ParseDeliverable(s); ok {
			t.Errorf("ParseDeliverable(%q) accepted", s)
		}
	}
}

// TestChangeDeliverableEffectiveValue pins the accessor default: an absent
// key reads as pr, an unknown stored value reads as "".
func TestChangeDeliverableEffectiveValue(t *testing.T) {
	if got := NewChange(ChangeSpec{}).Deliverable(); got != DeliverablePR {
		t.Errorf("absent deliverable = %q, want pr", got)
	}
	unknown := NewChange(ChangeSpec{RawDeliverable: "research"})
	if got := unknown.Deliverable(); got != "" {
		t.Errorf("unknown deliverable = %q, want empty", got)
	}
	if got := unknown.RawDeliverable(); got != "research" {
		t.Errorf("RawDeliverable = %q, want research", got)
	}
	ra := NewChange(ChangeSpec{Deliverable: DeliverableResultsArtifact, RawDeliverable: "results-artifact"})
	if got := ra.Deliverable(); got != DeliverableResultsArtifact {
		t.Errorf("results-artifact deliverable = %q", got)
	}
}

// TestLifecycleActionCarriesDeliverable pins newChangeBuilder's copy: a
// lifecycle write must keep the stored deliverable, known or unknown.
func TestLifecycleActionCarriesDeliverable(t *testing.T) {
	for _, spec := range []ChangeSpec{
		{ID: 1, Slug: "s", Status: StatusProposed, Deliverable: DeliverableResultsArtifact, RawDeliverable: "results-artifact"},
		{ID: 1, Slug: "s", Status: StatusProposed, RawDeliverable: "research"},
	} {
		res, fail := Defer(NewChange(spec))
		if fail != nil {
			t.Fatalf("Defer: %v", fail)
		}
		if got := res.Change.Deliverable(); got != spec.Deliverable {
			t.Errorf("Deliverable after Defer = %q, want %q", got, spec.Deliverable)
		}
		if got := res.Change.RawDeliverable(); got != spec.RawDeliverable {
			t.Errorf("RawDeliverable after Defer = %q, want %q", got, spec.RawDeliverable)
		}
	}
}

// TestLifecycleActionCarriesHold pins newChangeBuilder's copy of the hold: a
// claim keeps the stored hold rather than dropping it.
func TestLifecycleActionCarriesHold(t *testing.T) {
	c := NewChange(ChangeSpec{ID: 1, Slug: "s", Status: StatusProposed, Type: "feat", Spec: specRef("docs/specs/a.md"), Hold: holdTrue})
	res, fail := Claim(c, actNow)
	if fail != nil {
		t.Fatalf("Claim: %v", fail)
	}
	if !res.Change.Held() {
		t.Error("Held after Claim = false, want true")
	}
	if got := res.Change.Hold(); got != holdTrue {
		t.Errorf("Hold after Claim = %+v, want %+v", got, holdTrue)
	}
}
