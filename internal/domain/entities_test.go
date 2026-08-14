package domain

import (
	"slices"
	"testing"
	"time"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

// fullChangeSpec returns a ChangeSpec with every field set to a distinctive
// value, so a missing or mis-wired accessor cannot pass by coincidence.
func fullChangeSpec(t *testing.T) ChangeSpec {
	t.Helper()
	return ChangeSpec{
		ID:             307,
		Slug:           "domain-snapshot",
		Title:          "Domain snapshot",
		Status:         StatusInProgress,
		RawStatus:      "in-progress",
		Priority:       PriorityHigh,
		RawPriority:    "high",
		Type:           "feat",
		Created:        OptionalTime{State: FieldPresent, Value: mustDate(t, "2026-08-13"), Raw: "2026-08-13"},
		Updated:        OptionalTime{State: FieldMalformed, Raw: "not-a-date"},
		DependsOn:      []ChangeID{301, 302},
		StackedOn:      OptionalInt{State: FieldPresent, Value: 300, Raw: "300"},
		Related:        []ChangeID{305},
		DiscoveredFrom: []ChangeID{234, 235},
		ADRs:           []ADRID{71, 54},
		Spec:           OptionalString{State: FieldPresent, Value: "docs/specs/x.md"},
		Plan:           OptionalString{State: FieldEmpty},
		Results:        OptionalString{State: FieldAbsent},
		Trivial:        true,
		Branch:         OptionalString{State: FieldPresent, Value: "feat/domain-snapshot"},
		ClaimedAt:      OptionalTime{State: FieldPresent, Value: time.Date(2026, 8, 13, 4, 5, 6, 0, time.UTC), Raw: "2026-08-13T04:05:06Z"},
		PR:             OptionalString{State: FieldPresent, Value: "https://example.test/pr/1"},
		Issue:          OptionalString{State: FieldMalformed, Value: "#"},
		BlockedBy:      OptionalString{State: FieldEmpty},
		Reconciled:     true,
		Location:       LocationActive,
		Path:           "docs/changes/active/0307-domain-snapshot.md",
		ArchiveDate:    OptionalTime{State: FieldAbsent},

		HasRunHalted:        true,
		HasAutoGroomBlocked: true,
		HasFinalizeBlocked:  true,
		HasPublishDeferred:  true,
	}
}

func TestChangeAccessorsRoundTrip(t *testing.T) {
	spec := fullChangeSpec(t)
	c := NewChange(spec)

	if got := c.ID(); got != spec.ID {
		t.Errorf("ID() = %d; want %d", got, spec.ID)
	}
	if got := c.Slug(); got != spec.Slug {
		t.Errorf("Slug() = %q; want %q", got, spec.Slug)
	}
	if got := c.Title(); got != spec.Title {
		t.Errorf("Title() = %q; want %q", got, spec.Title)
	}
	if got := c.Status(); got != spec.Status {
		t.Errorf("Status() = %q; want %q", got, spec.Status)
	}
	if got := c.RawStatus(); got != spec.RawStatus {
		t.Errorf("RawStatus() = %q; want %q", got, spec.RawStatus)
	}
	if got := c.Priority(); got != spec.Priority {
		t.Errorf("Priority() = %q; want %q", got, spec.Priority)
	}
	if got := c.RawPriority(); got != spec.RawPriority {
		t.Errorf("RawPriority() = %q; want %q", got, spec.RawPriority)
	}
	if got := c.Type(); got != spec.Type {
		t.Errorf("Type() = %q; want %q", got, spec.Type)
	}
	if got := c.Created(); got != spec.Created {
		t.Errorf("Created() = %+v; want %+v", got, spec.Created)
	}
	if got := c.Updated(); got != spec.Updated {
		t.Errorf("Updated() = %+v; want %+v", got, spec.Updated)
	}
	if got := c.DependsOn(); !slices.Equal(got, spec.DependsOn) {
		t.Errorf("DependsOn() = %v; want %v", got, spec.DependsOn)
	}
	if got := c.StackedOn(); got != spec.StackedOn {
		t.Errorf("StackedOn() = %+v; want %+v", got, spec.StackedOn)
	}
	if got := c.Related(); !slices.Equal(got, spec.Related) {
		t.Errorf("Related() = %v; want %v", got, spec.Related)
	}
	if got := c.DiscoveredFrom(); !slices.Equal(got, spec.DiscoveredFrom) {
		t.Errorf("DiscoveredFrom() = %v; want %v", got, spec.DiscoveredFrom)
	}
	if got := c.ADRs(); !slices.Equal(got, spec.ADRs) {
		t.Errorf("ADRs() = %v; want %v", got, spec.ADRs)
	}
	if got := c.Trivial(); got != spec.Trivial {
		t.Errorf("Trivial() = %v; want %v", got, spec.Trivial)
	}
	if got := c.Branch(); got != spec.Branch {
		t.Errorf("Branch() = %+v; want %+v", got, spec.Branch)
	}
	if got := c.ClaimedAt(); got != spec.ClaimedAt {
		t.Errorf("ClaimedAt() = %+v; want %+v", got, spec.ClaimedAt)
	}
	if got := c.PR(); got != spec.PR {
		t.Errorf("PR() = %+v; want %+v", got, spec.PR)
	}
	if got := c.Issue(); got != spec.Issue {
		t.Errorf("Issue() = %+v; want %+v", got, spec.Issue)
	}
	if got := c.BlockedBy(); got != spec.BlockedBy {
		t.Errorf("BlockedBy() = %+v; want %+v", got, spec.BlockedBy)
	}
	if got := c.Reconciled(); got != spec.Reconciled {
		t.Errorf("Reconciled() = %v; want %v", got, spec.Reconciled)
	}
	if got := c.Location(); got != spec.Location {
		t.Errorf("Location() = %q; want %q", got, spec.Location)
	}
	if got := c.Path(); got != spec.Path {
		t.Errorf("Path() = %q; want %q", got, spec.Path)
	}
	if got := c.ArchiveDate(); got != spec.ArchiveDate {
		t.Errorf("ArchiveDate() = %+v; want %+v", got, spec.ArchiveDate)
	}
	if got := c.HasRunHalted(); got != spec.HasRunHalted {
		t.Errorf("HasRunHalted() = %v; want %v", got, spec.HasRunHalted)
	}
	if got := c.HasAutoGroomBlocked(); got != spec.HasAutoGroomBlocked {
		t.Errorf("HasAutoGroomBlocked() = %v; want %v", got, spec.HasAutoGroomBlocked)
	}
	if got := c.HasFinalizeBlocked(); got != spec.HasFinalizeBlocked {
		t.Errorf("HasFinalizeBlocked() = %v; want %v", got, spec.HasFinalizeBlocked)
	}
	if got := c.HasPublishDeferred(); got != spec.HasPublishDeferred {
		t.Errorf("HasPublishDeferred() = %v; want %v", got, spec.HasPublishDeferred)
	}
}

// TestChangeOptionalFieldStates walks all four FieldStates through the
// optional accessors the decoder relies on most.
func TestChangeOptionalFieldStates(t *testing.T) {
	claimRaw := "2026-08-13T04:05:06Z"
	claimVal := time.Date(2026, 8, 13, 4, 5, 6, 0, time.UTC)
	tests := []struct {
		name  string
		spec  OptionalString
		claim OptionalTime
	}{
		{"absent", OptionalString{State: FieldAbsent}, OptionalTime{State: FieldAbsent}},
		{"empty", OptionalString{State: FieldEmpty}, OptionalTime{State: FieldEmpty, Raw: ""}},
		{"malformed", OptionalString{State: FieldMalformed, Value: "  "}, OptionalTime{State: FieldMalformed, Raw: "yesterday"}},
		{"present", OptionalString{State: FieldPresent, Value: "docs/spec.md"}, OptionalTime{State: FieldPresent, Value: claimVal, Raw: claimRaw}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewChange(ChangeSpec{Spec: tc.spec, ClaimedAt: tc.claim})
			if got := c.Spec(); got != tc.spec {
				t.Errorf("Spec() = %+v; want %+v", got, tc.spec)
			}
			if got := c.ClaimedAt(); got != tc.claim {
				t.Errorf("ClaimedAt() = %+v; want %+v", got, tc.claim)
			}
		})
	}
	// FieldAbsent is the zero value, so an unset optional reads as absent.
	c := NewChange(ChangeSpec{})
	if got := c.Plan(); got.State != FieldAbsent {
		t.Errorf("unset Plan().State = %v; want FieldAbsent", got.State)
	}
	if got := c.Results(); got.State != FieldAbsent {
		t.Errorf("unset Results().State = %v; want FieldAbsent", got.State)
	}
	if got := c.StackedOn(); got.State != FieldAbsent {
		t.Errorf("unset StackedOn().State = %v; want FieldAbsent", got.State)
	}
}

func TestFieldStateConstantsOrdered(t *testing.T) {
	states := []FieldState{FieldAbsent, FieldEmpty, FieldMalformed, FieldPresent}
	for i, s := range states {
		if int(s) != i {
			t.Errorf("state %d = %d; want %d", i, int(s), i)
		}
	}
}

func TestNewChangeDefensiveCopies(t *testing.T) {
	depends := []ChangeID{301, 302}
	related := []ChangeID{305}
	discovered := []ChangeID{234}
	adrs := []ADRID{71, 54}
	c := NewChange(ChangeSpec{
		DependsOn:      depends,
		Related:        related,
		DiscoveredFrom: discovered,
		ADRs:           adrs,
	})

	// Mutating the caller's input slices must not reach the entity.
	depends[0] = 999
	related[0] = 999
	discovered[0] = 999
	adrs[0] = 999

	if got := c.DependsOn(); !slices.Equal(got, []ChangeID{301, 302}) {
		t.Errorf("DependsOn() = %v after input mutation; want [301 302]", got)
	}
	if got := c.Related(); !slices.Equal(got, []ChangeID{305}) {
		t.Errorf("Related() = %v after input mutation; want [305]", got)
	}
	if got := c.DiscoveredFrom(); !slices.Equal(got, []ChangeID{234}) {
		t.Errorf("DiscoveredFrom() = %v after input mutation; want [234]", got)
	}
	if got := c.ADRs(); !slices.Equal(got, []ADRID{71, 54}) {
		t.Errorf("ADRs() = %v after input mutation; want [71 54]", got)
	}

	// Mutating an accessor's result must not reach the entity either.
	c.DependsOn()[0] = 888
	c.Related()[0] = 888
	c.DiscoveredFrom()[0] = 888
	c.ADRs()[0] = 888

	if got := c.DependsOn(); !slices.Equal(got, []ChangeID{301, 302}) {
		t.Errorf("DependsOn() = %v after result mutation; want [301 302]", got)
	}
	if got := c.Related(); !slices.Equal(got, []ChangeID{305}) {
		t.Errorf("Related() = %v after result mutation; want [305]", got)
	}
	if got := c.DiscoveredFrom(); !slices.Equal(got, []ChangeID{234}) {
		t.Errorf("DiscoveredFrom() = %v after result mutation; want [234]", got)
	}
	if got := c.ADRs(); !slices.Equal(got, []ADRID{71, 54}) {
		t.Errorf("ADRs() = %v after result mutation; want [71 54]", got)
	}
}

func TestNewChangeNilSlicesStayNil(t *testing.T) {
	c := NewChange(ChangeSpec{})
	if got := c.DependsOn(); got != nil {
		t.Errorf("DependsOn() = %v; want nil", got)
	}
	if got := c.ADRs(); got != nil {
		t.Errorf("ADRs() = %v; want nil", got)
	}
}

func TestNewADRAccessorsAndCopies(t *testing.T) {
	spec := ADRSpec{
		ID:         71,
		Slug:       "quote-frontmatter-scalars",
		Title:      "Quote frontmatter scalars",
		Status:     ADRStatus{Kind: ADRSupersededBy, Ref: 88},
		RawStatus:  "Superseded by ADR-0088",
		Date:       OptionalTime{State: FieldPresent, Value: mustDate(t, "2026-07-01"), Raw: "2026-07-01"},
		Supersedes: []ADRID{40, 41},
		Reverses:   []ADRID{42},
		RelatesTo:  []ADRID{54},
		Change:     OptionalInt{State: FieldPresent, Value: 307, Raw: "307"},
		Path:       "docs/adrs/0071-quote-frontmatter-scalars.md",
		ContentID:  "sha:abc123",
	}
	a := NewADR(spec)

	if a.ID() != spec.ID || a.Slug() != spec.Slug || a.Title() != spec.Title {
		t.Errorf("identity accessors = %d/%q/%q; want %d/%q/%q", a.ID(), a.Slug(), a.Title(), spec.ID, spec.Slug, spec.Title)
	}
	if a.Status() != spec.Status {
		t.Errorf("Status() = %+v; want %+v", a.Status(), spec.Status)
	}
	if a.RawStatus() != spec.RawStatus {
		t.Errorf("RawStatus() = %q; want %q", a.RawStatus(), spec.RawStatus)
	}
	if a.Date() != spec.Date {
		t.Errorf("Date() = %+v; want %+v", a.Date(), spec.Date)
	}
	if a.Change() != spec.Change {
		t.Errorf("Change() = %+v; want %+v", a.Change(), spec.Change)
	}
	if a.Path() != spec.Path || a.ContentID() != spec.ContentID {
		t.Errorf("Path()/ContentID() = %q/%q; want %q/%q", a.Path(), a.ContentID(), spec.Path, spec.ContentID)
	}
	if !slices.Equal(a.Supersedes(), []ADRID{40, 41}) ||
		!slices.Equal(a.Reverses(), []ADRID{42}) ||
		!slices.Equal(a.RelatesTo(), []ADRID{54}) {
		t.Fatalf("relation accessors = %v/%v/%v", a.Supersedes(), a.Reverses(), a.RelatesTo())
	}

	spec.Supersedes[0] = 999
	spec.Reverses[0] = 999
	spec.RelatesTo[0] = 999
	a.Supersedes()[1] = 888

	if !slices.Equal(a.Supersedes(), []ADRID{40, 41}) {
		t.Errorf("Supersedes() = %v after mutations; want [40 41]", a.Supersedes())
	}
	if !slices.Equal(a.Reverses(), []ADRID{42}) {
		t.Errorf("Reverses() = %v after mutations; want [42]", a.Reverses())
	}
	if !slices.Equal(a.RelatesTo(), []ADRID{54}) {
		t.Errorf("RelatesTo() = %v after mutations; want [54]", a.RelatesTo())
	}
}

func TestNewLearningAccessorsAndCopies(t *testing.T) {
	spec := LearningSpec{
		Slug:       "cached-runner-serves-a-mutated-tree",
		Hook:       "go test caches passes",
		Topics:     []string{"go", "testing"},
		Changes:    []ChangeID{307, 301},
		Created:    OptionalTime{State: FieldPresent, Value: mustDate(t, "2026-08-01"), Raw: "2026-08-01"},
		Updated:    OptionalTime{State: FieldAbsent},
		Promotion:  PromotionCandidate,
		PromotedTo: OptionalString{State: FieldEmpty},
		Content:    "always pass -count=1",
		Path:       "docs/changes/learnings/cached-runner.md",
	}
	l := NewLearning(spec)

	if l.Slug() != spec.Slug || l.Hook() != spec.Hook || l.Content() != spec.Content || l.Path() != spec.Path {
		t.Errorf("scalar accessors mismatch: %q/%q/%q/%q", l.Slug(), l.Hook(), l.Content(), l.Path())
	}
	if l.Created() != spec.Created || l.Updated() != spec.Updated {
		t.Errorf("date accessors = %+v/%+v; want %+v/%+v", l.Created(), l.Updated(), spec.Created, spec.Updated)
	}
	if l.Promotion() != spec.Promotion || l.PromotedTo() != spec.PromotedTo {
		t.Errorf("promotion accessors = %q/%+v; want %q/%+v", l.Promotion(), l.PromotedTo(), spec.Promotion, spec.PromotedTo)
	}
	if !slices.Equal(l.Topics(), []string{"go", "testing"}) || !slices.Equal(l.Changes(), []ChangeID{307, 301}) {
		t.Fatalf("collection accessors = %v/%v", l.Topics(), l.Changes())
	}

	spec.Topics[0] = "mutated"
	spec.Changes[0] = 999
	l.Topics()[1] = "mutated"
	l.Changes()[1] = 999

	if !slices.Equal(l.Topics(), []string{"go", "testing"}) {
		t.Errorf("Topics() = %v after mutations; want [go testing]", l.Topics())
	}
	if !slices.Equal(l.Changes(), []ChangeID{307, 301}) {
		t.Errorf("Changes() = %v after mutations; want [307 301]", l.Changes())
	}
}

func TestNewArtifactAndDerivedView(t *testing.T) {
	art := NewArtifact(ArtifactSpec{
		Path:              "docs/superpowers/plans/p.md",
		Kind:              ArtifactPlan,
		ContentID:         "sha:def",
		HasBacklinkMarker: true,
	})
	if art.Path() != "docs/superpowers/plans/p.md" || art.Kind() != ArtifactPlan ||
		art.ContentID() != "sha:def" || !art.HasBacklinkMarker() {
		t.Errorf("artifact accessors = %q/%q/%q/%v", art.Path(), art.Kind(), art.ContentID(), art.HasBacklinkMarker())
	}

	view := NewDerivedView(DerivedViewSpec{Path: "docs/changes/BOARD.md", Kind: DerivedBoard})
	if view.Path() != "docs/changes/BOARD.md" || view.Kind() != DerivedBoard {
		t.Errorf("derived view accessors = %q/%q", view.Path(), view.Kind())
	}
}

func TestArtifactAndDerivedKindVocabulary(t *testing.T) {
	kinds := map[ArtifactKind]string{
		ArtifactSpecKind: "spec",
		ArtifactPlan:     "plan",
		ArtifactResults:  "results",
		ArtifactOther:    "other",
	}
	for k, want := range kinds {
		if string(k) != want {
			t.Errorf("artifact kind = %q; want %q", string(k), want)
		}
	}
	views := map[DerivedViewKind]string{
		DerivedBoard:          "board",
		DerivedADRIndex:       "adr-index",
		DerivedLearningsIndex: "learnings-index",
		DerivedOther:          "other",
	}
	for k, want := range views {
		if string(k) != want {
			t.Errorf("derived view kind = %q; want %q", string(k), want)
		}
	}
}

func TestRecordLocationVocabulary(t *testing.T) {
	locations := map[RecordLocation]string{
		LocationActive:   "active",
		LocationArchive:  "archive",
		LocationLedger:   "ledger",
		LocationArtifact: "artifact",
		LocationDerived:  "derived",
	}
	for l, want := range locations {
		if string(l) != want {
			t.Errorf("location = %q; want %q", string(l), want)
		}
	}
}

func TestNewRepositoryPolicyDefensiveCopies(t *testing.T) {
	in := RepositoryPolicy{
		IntegrationBranch: "docket",
		ChangeTypes:       []string{"feat", "fix"},
		ReclaimTTLHours:   24,
		LearningsEnabled:  true,
	}
	p := NewRepositoryPolicy(in)

	if p.IntegrationBranch != in.IntegrationBranch || p.ReclaimTTLHours != in.ReclaimTTLHours ||
		p.LearningsEnabled != in.LearningsEnabled {
		t.Errorf("scalar fields = %q/%d/%v; want %q/%d/%v",
			p.IntegrationBranch, p.ReclaimTTLHours, p.LearningsEnabled,
			in.IntegrationBranch, in.ReclaimTTLHours, in.LearningsEnabled)
	}
	if !slices.Equal(p.ChangeTypes, []string{"feat", "fix"}) {
		t.Fatalf("ChangeTypes = %v; want [feat fix]", p.ChangeTypes)
	}

	in.ChangeTypes[0] = "mutated"
	if !slices.Equal(p.ChangeTypes, []string{"feat", "fix"}) {
		t.Errorf("ChangeTypes = %v after input mutation; want [feat fix]", p.ChangeTypes)
	}

	p.ChangeTypes[0] = "mutated"
	if !slices.Equal(NewRepositoryPolicy(p).ChangeTypes, []string{"mutated", "fix"}) {
		t.Errorf("re-copy did not observe the copy's own slice")
	}
	if in.ChangeTypes[1] != "fix" {
		t.Errorf("copy mutation leaked back into the input: %v", in.ChangeTypes)
	}
}
