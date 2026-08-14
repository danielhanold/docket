package domain

import (
	"slices"
	"testing"
)

// learningRec builds a learning with a promotion state and topics.
func learningRec(slug string, promotion PromotionState, topics ...string) Learning {
	return NewLearning(LearningSpec{Slug: slug, Promotion: promotion, Topics: topics})
}

// learningSnapshot builds a snapshot with learnings enabled or disabled.
func learningSnapshot(enabled bool, learnings ...Learning) Snapshot {
	return NewSnapshot(SnapshotSpec{
		Policy:    RepositoryPolicy{LearningsEnabled: enabled},
		Learnings: learnings,
	})
}

// learningSlugs renders a catalog's findings as their slugs, in order.
func learningSlugs(c LearningCatalog) []string {
	out := make([]string, 0, len(c.Findings))
	for _, l := range c.Findings {
		out = append(out, l.Slug())
	}
	return out
}

func TestLearningCandidatesDisabledPolicy(t *testing.T) {
	s := learningSnapshot(false,
		learningRec("a", PromotionRetained, "shell"),
		learningRec("b", PromotionCandidate, "go"),
	)

	got := LearningCandidates(s)

	if !got.Disabled {
		t.Fatalf("Disabled = false; want true with learnings disabled")
	}
	if got.Findings != nil {
		t.Fatalf("Findings = %v; want nil when disabled", learningSlugs(got))
	}
	if filtered := FilterLearnings(s, []string{"shell"}, nil); !filtered.Disabled || filtered.Findings != nil {
		t.Fatalf("FilterLearnings = %+v; want disabled with no findings", filtered)
	}
}

func TestLearningCandidatesExcludesPromoted(t *testing.T) {
	s := learningSnapshot(true,
		learningRec("retained-one", PromotionRetained, "shell"),
		learningRec("promoted-one", PromotionPromoted, "shell"),
		learningRec("candidate-one", PromotionCandidate, "go"),
	)

	got := LearningCandidates(s)

	if got.Disabled {
		t.Fatalf("Disabled = true; want false with learnings enabled")
	}
	want := []string{"retained-one", "candidate-one"}
	if !slices.Equal(learningSlugs(got), want) {
		t.Fatalf("Findings = %v; want %v in authored order", learningSlugs(got), want)
	}
}

func TestFilterLearnings(t *testing.T) {
	s := learningSnapshot(true,
		learningRec("shell-pipe", PromotionRetained, "shell", "pipefail"),
		learningRec("go-tests", PromotionCandidate, "go"),
		learningRec("yaml-quoting", PromotionRetained, "yaml", "shell"),
		learningRec("promoted-shell", PromotionPromoted, "shell"),
	)

	cases := []struct {
		name   string
		topics []string
		slugs  []string
		want   []string
	}{
		{"no filters returns the active catalog", nil, nil, []string{"shell-pipe", "go-tests", "yaml-quoting"}},
		{"by topic", []string{"shell"}, nil, []string{"shell-pipe", "yaml-quoting"}},
		{"topics OR within the dimension", []string{"go", "yaml"}, nil, []string{"go-tests", "yaml-quoting"}},
		{"by slug", nil, []string{"go-tests"}, []string{"go-tests"}},
		{"slugs OR within the dimension", nil, []string{"go-tests", "shell-pipe"}, []string{"shell-pipe", "go-tests"}},
		{"AND across dimensions", []string{"shell"}, []string{"yaml-quoting", "go-tests"}, []string{"yaml-quoting"}},
		{"promoted never matches", []string{"shell"}, []string{"promoted-shell"}, nil},
		{"unknown topic matches nothing", []string{"nope"}, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterLearnings(s, tc.topics, tc.slugs)

			if got.Disabled {
				t.Fatalf("Disabled = true; want false")
			}
			if !slices.Equal(learningSlugs(got), tc.want) {
				t.Fatalf("Findings = %v; want %v", learningSlugs(got), tc.want)
			}
		})
	}
}

func TestFilterLearningsMatchesTopicsExactly(t *testing.T) {
	s := learningSnapshot(true, learningRec("one", PromotionRetained, "Shell"))

	if got := FilterLearnings(s, []string{"shell"}, nil); len(got.Findings) != 0 {
		t.Fatalf("case-folded topic matched: %v", learningSlugs(got))
	}
}

func TestLearningQueriesDoNotRetainOrAliasCallerState(t *testing.T) {
	s := learningSnapshot(true, learningRec("one", PromotionRetained, "shell"))

	topics := []string{"shell"}
	slugs := []string{"one"}
	got := FilterLearnings(s, topics, slugs)
	if len(got.Findings) != 1 {
		t.Fatalf("Findings = %v; want the single match", learningSlugs(got))
	}
	topics[0] = "mutated"
	slugs[0] = "mutated"
	if again := FilterLearnings(s, []string{"shell"}, []string{"one"}); len(again.Findings) != 1 {
		t.Fatalf("filter inputs were retained: %v", learningSlugs(again))
	}

	got.Findings[0] = learningRec("tamper", PromotionRetained)
	if fresh := LearningCandidates(s); fresh.Findings[0].Slug() != "one" {
		t.Fatalf("catalog aliased stored state: %q", fresh.Findings[0].Slug())
	}
}
