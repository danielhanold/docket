package workspace

import (
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

func resolvedBase(branch string) domain.EffectiveBase {
	return domain.EffectiveBase{Kind: domain.BaseResolved, Branch: branch}
}

func TestNewTargetAcceptsValidAndDerives(t *testing.T) {
	tgt, err := NewTarget(7, "fix-the-thing", resolvedBase("main"), "feat/fix-the-thing")
	if err != nil {
		t.Fatalf("NewTarget valid = %v; want nil", err)
	}
	if tgt.ChangeID != 7 {
		t.Errorf("ChangeID = %d; want 7", tgt.ChangeID)
	}
	if tgt.Slug != "fix-the-thing" {
		t.Errorf("Slug = %q; want fix-the-thing", tgt.Slug)
	}
	if want := gitcli.RefName("refs/heads/feat/fix-the-thing"); tgt.FeatureRef != want {
		t.Errorf("FeatureRef = %q; want %q", tgt.FeatureRef, want)
	}
	if want := gitcli.RefName("refs/heads/main"); tgt.BaseRef != want {
		t.Errorf("BaseRef = %q; want %q", tgt.BaseRef, want)
	}
	if tgt.Base.Kind != domain.BaseResolved || tgt.Base.Branch != "main" {
		t.Errorf("Base = %+v; want resolved/main", tgt.Base)
	}
}

// TestNewTargetHonorsRecordedFeatureBranch: an explicit recorded branch distinct
// from any slug-derivation is the one FeatureRef carries — proving the feature
// branch is the caller-supplied input, never re-derived from the slug. Mutation:
// derive FeatureRef from the slug and this reddens (it would be feat/my-slug).
func TestNewTargetHonorsRecordedFeatureBranch(t *testing.T) {
	tgt, err := NewTarget(1, "my-slug", resolvedBase("main"), "feature/other-name")
	if err != nil {
		t.Fatalf("NewTarget valid = %v; want nil", err)
	}
	if want := gitcli.RefName("refs/heads/feature/other-name"); tgt.FeatureRef != want {
		t.Errorf("FeatureRef = %q; want %q (the recorded branch, not a slug derivation)", tgt.FeatureRef, want)
	}
	if got := tgt.FeatureBranch(); got != "feature/other-name" {
		t.Errorf("FeatureBranch() = %q; want feature/other-name", got)
	}
}

func TestNewTargetRejects(t *testing.T) {
	// fb defaults to a valid feature branch so the id/slug/base rejection cases
	// still fire on their own field; the featureBranch cases set fb explicitly.
	const okFB = "feat/ok-slug"
	tests := []struct {
		name string
		id   domain.ChangeID
		slug string
		base domain.EffectiveBase
		fb   string
	}{
		{"id zero", 0, "ok-slug", resolvedBase("main"), okFB},
		{"id negative", -1, "ok-slug", resolvedBase("main"), okFB},
		{"slug empty", 7, "", resolvedBase("main"), okFB},
		{"slug uppercase", 7, "Fix", resolvedBase("main"), okFB},
		{"slug underscore", 7, "a_b", resolvedBase("main"), okFB},
		{"slug space", 7, "a b", resolvedBase("main"), okFB},
		{"slug slash", 7, "feat/x", resolvedBase("main"), okFB},
		{"slug unicode", 7, "café", resolvedBase("main"), okFB},
		{"slug leading hyphen", 7, "-x", resolvedBase("main"), okFB},
		{"slug trailing hyphen", 7, "x-", resolvedBase("main"), okFB},
		{"slug doubled hyphen", 7, "a--b", resolvedBase("main"), okFB},
		{"base empty branch", 7, "ok-slug", resolvedBase(""), okFB},
		{"base already qualified", 7, "ok-slug", resolvedBase("refs/heads/main"), okFB},
		{"base dotdot", 7, "ok-slug", resolvedBase("ma..in"), okFB},
		{"base space", 7, "ok-slug", resolvedBase("ma in"), okFB},
		{"feature branch empty", 7, "ok-slug", resolvedBase("main"), ""},
		{"feature branch already qualified", 7, "ok-slug", resolvedBase("main"), "refs/heads/feature/x"},
		{"feature branch leading hyphen", 7, "ok-slug", resolvedBase("main"), "-lead"},
		{"feature branch dotdot", 7, "ok-slug", resolvedBase("main"), "a..b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tgt, err := NewTarget(tc.id, tc.slug, tc.base, tc.fb)
			if err == nil {
				t.Fatalf("NewTarget(%d, %q, %+v, %q) = nil error; want rejection", tc.id, tc.slug, tc.base, tc.fb)
			}
			if tgt != (Target{}) {
				t.Errorf("rejected NewTarget returned non-zero Target %+v; want zero", tgt)
			}
			if f, ok := AsFailure(err); !ok {
				t.Errorf("error %v is not a *Failure", err)
			} else if f.Kind != KindInvalidInput {
				t.Errorf("Kind = %q; want %q", f.Kind, KindInvalidInput)
			}
		})
	}
}

// TestNewTargetRejectsEveryNonResolvedBaseKind iterates the REAL tagged
// constants from internal/domain/stack.go (EffectiveBaseKind) rather than a
// hand-listed set, per the derive-don't-enumerate rule, and asserts each
// non-resolved kind is rejected.
func TestNewTargetRejectsEveryNonResolvedBaseKind(t *testing.T) {
	allKinds := []domain.EffectiveBaseKind{
		domain.BaseResolved,
		domain.BaseParentKilled,
		domain.BaseMissingParent,
		domain.BaseCycle,
		domain.BaseMalformedEdge,
		domain.BaseBranchAbsent,
	}
	for _, kind := range allKinds {
		if kind == domain.BaseResolved {
			continue
		}
		t.Run(string(kind), func(t *testing.T) {
			// A non-resolved outcome carries no meaningful branch.
			base := domain.EffectiveBase{Kind: kind, Cause: 3}
			_, err := NewTarget(7, "ok-slug", base, "feat/ok-slug")
			if err == nil {
				t.Fatalf("NewTarget with base kind %q = nil error; want rejection", kind)
			}
			if f, ok := AsFailure(err); !ok || f.Kind != KindInvalidInput {
				t.Errorf("error for kind %q = %v; want *Failure invalid-input", kind, err)
			}
		})
	}
}

// TestNewTargetSpendsResolverBranch wires REAL domain.ResolveEffectiveBase
// outcomes — unstacked, live-parent stack, done-parent, and recursively
// stacked-merged — into the constructor, proving workspace spends the
// resolver's branch rather than shadowing the resolution rules (spec
// §"Feature target").
func TestNewTargetSpendsResolverBranch(t *testing.T) {
	tests := []struct {
		name       string
		specs      []domain.ChangeSpec
		branches   []string
		subject    domain.ChangeID
		wantBranch string
	}{
		{
			name:       "unstacked resolves to integration branch",
			specs:      []domain.ChangeSpec{{ID: 7, Status: domain.StatusProposed}},
			subject:    7,
			wantBranch: "main",
		},
		{
			name: "live parent resolves to parent branch",
			specs: []domain.ChangeSpec{
				{ID: 5, Status: domain.StatusInProgress, Branch: present("feat/five")},
				{ID: 7, Status: domain.StatusProposed, StackedOn: parentOf(5)},
			},
			branches:   []string{"feat/five"},
			subject:    7,
			wantBranch: "feat/five",
		},
		{
			name: "done parent resolves to integration branch",
			specs: []domain.ChangeSpec{
				{ID: 5, Status: domain.StatusDone, Branch: present("feat/five")},
				{ID: 7, Status: domain.StatusProposed, StackedOn: parentOf(5)},
			},
			branches:   []string{"feat/five"},
			subject:    7,
			wantBranch: "main",
		},
		{
			name: "recursively stacked-merged parent recurses to grandparent branch",
			specs: []domain.ChangeSpec{
				{ID: 4, Status: domain.StatusInProgress, Branch: present("feat/four")},
				{ID: 5, Status: domain.StatusStackedMerged, StackedOn: parentOf(4)},
				{ID: 7, Status: domain.StatusProposed, StackedOn: parentOf(5)},
			},
			branches:   []string{"feat/four"},
			subject:    7,
			wantBranch: "feat/four",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			changes := make([]domain.Change, 0, len(tc.specs))
			for _, sp := range tc.specs {
				changes = append(changes, domain.NewChange(sp))
			}
			snap := domain.NewSnapshot(domain.SnapshotSpec{
				Policy:  domain.RepositoryPolicy{IntegrationBranch: "main"},
				Changes: changes,
			})
			set := make(map[string]bool, len(tc.branches))
			for _, b := range tc.branches {
				set[b] = true
			}
			facts := domain.NewBranchFacts(set)
			c, out := snap.Change(tc.subject)
			if out != domain.LookupFound {
				t.Fatalf("Change(%d) outcome = %v; want found", tc.subject, out)
			}
			base := domain.ResolveEffectiveBase(snap, c, facts)
			if base.Kind != domain.BaseResolved {
				t.Fatalf("resolver base kind = %q; want resolved (fixture bug)", base.Kind)
			}

			tgt, err := NewTarget(tc.subject, "ok-slug", base, "feat/ok-slug")
			if err != nil {
				t.Fatalf("NewTarget = %v; want nil", err)
			}
			if want := gitcli.RefName("refs/heads/" + tc.wantBranch); tgt.BaseRef != want {
				t.Errorf("BaseRef = %q; want %q", tgt.BaseRef, want)
			}
			if tgt.Base.Branch != tc.wantBranch {
				t.Errorf("Base.Branch = %q; want %q", tgt.Base.Branch, tc.wantBranch)
			}
		})
	}
}

func present(v string) domain.OptionalString {
	return domain.OptionalString{State: domain.FieldPresent, Value: v}
}

func parentOf(id domain.ChangeID) domain.OptionalInt {
	return domain.OptionalInt{State: domain.FieldPresent, Value: int(id)}
}
