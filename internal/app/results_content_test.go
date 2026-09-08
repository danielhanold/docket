package app

import (
	"sort"
	"strings"
	"testing"
)

// reasonsOf collects the Reason strings of a finding slice, sorted, for
// order-independent multiset comparison in the table below.
func reasonsOf(fs []ResultsContentFinding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Reason)
	}
	sort.Strings(out)
	return out
}

func sameReasons(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	w := append([]string(nil), want...)
	sort.Strings(w)
	for i := range got {
		if got[i] != w[i] {
			return false
		}
	}
	return true
}

// The two explicit cases the plan pins verbatim.

func TestValidateResultsContentOutcomeOnlyFinal(t *testing.T) {
	src := []byte("<!-- docket:backlink:start (generated — do not hand-edit) -->\n> home\n<!-- docket:backlink:end -->\n\n# Change title — Results\n\n## Outcome\n\nDelivered the thing; behavior X now refuses Y.\n")
	if fs := ValidateResultsContent(src, ResultsPhaseFinal); len(fs) != 0 {
		t.Fatalf("want valid, got %v", fs)
	}
}

func TestValidateResultsContentFillerSectionRefusedFinal(t *testing.T) {
	src := []byte("# T — Results\n\n## Outcome\n\nReal outcome prose.\n\n## Findings and limitations\n\nNone.\n")
	fs := ValidateResultsContent(src, ResultsPhaseFinal)
	if len(fs) != 1 || fs[0].Reason != "results-filler-section" {
		t.Fatalf("want one results-filler-section, got %v", fs)
	}
}

func TestValidateResultsContentTable(t *testing.T) {
	const backlink = "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> home\n<!-- docket:backlink:end -->\n\n"

	cases := []struct {
		name  string
		src   string
		phase ResultsPhase
		want  []string
	}{
		{
			name:  "valid outcome-only final",
			src:   "# T — Results\n\n## Outcome\n\nDelivered the thing; behavior X now refuses Y.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "valid in-progress checkpoint (outcome only)",
			src:   "# T — Results\n\n## Outcome\n\nBuild landed; review pending.\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "missing outcome refused final",
			src:   "# T — Results\n\n## Human testing\n\n### Scenario\n\nDo X and observe Y.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-outcome-missing"},
		},
		{
			name:  "missing outcome accepted checkpoint",
			src:   "# T — Results\n\n## Human testing\n\n### Scenario\n\nDo X and observe Y.\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "empty outcome body refused final",
			src:   "# T — Results\n\n## Outcome\n\n## Notes\n\nSome real notes.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-outcome-empty"},
		},
		{
			name:  "empty subsection under filled parent refused final",
			src:   "# T — Results\n\n## Outcome\n\nReal outcome.\n\n## Human testing\n\nSetup paragraph here.\n\n### Empty scenario\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-empty-section"},
		},
		{
			name:  "filled sub under bodyless parent accepted final",
			src:   "# T — Results\n\n## Outcome\n\nReal outcome.\n\n## Human testing\n\n### Scenario\n\nDo X and observe Y.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "None in prose is legal final",
			src:   "# T — Results\n\n## Outcome\n\nNone of the flags are read at startup, so the change is inert.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "placeholder token TODO refused checkpoint",
			src:   "# T — Results\n\n## Outcome\n\nTODO finish this.\n",
			phase: ResultsPhaseCheckpoint,
			want:  []string{"results-placeholder"},
		},
		{
			name:  "raw template scaffold refused checkpoint",
			src:   "# <Change title> — Results\n\n## Outcome\n\n<What was delivered and how the behavior changed.>\n",
			phase: ResultsPhaseCheckpoint,
			want:  []string{"results-placeholder"},
		},
		{
			name:  "outcome-looking heading in fence ignored, real outcome validates",
			src:   "# T — Results\n\n## Outcome\n\nReal outcome prose.\n\n## Notes\n\n```\n## Outcome\n```\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "only H1 inside fence is title-missing checkpoint",
			src:   "```\n# Title\n```\n\n## Outcome\n\nReal outcome.\n",
			phase: ResultsPhaseCheckpoint,
			want:  []string{"results-title-missing"},
		},
		{
			name:  "malformed managed markers",
			src:   "<!-- docket:backlink:start -->\nno end marker here\n\n# T — Results\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-doc-malformed"},
		},
		{
			name:  "backlink block at top does not count as body or title",
			src:   backlink + "# T — Results\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "N/A whole-section filler refused final",
			src:   "# T — Results\n\n## Outcome\n\nReal outcome.\n\n## Follow-ups\n\nN/A\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-filler-section"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := reasonsOf(ValidateResultsContent([]byte(tc.src), tc.phase))
			if !sameReasons(got, tc.want) {
				t.Fatalf("phase=%v\nsrc=%q\ngot reasons %v\nwant %v", tc.phase, tc.src, got, tc.want)
			}
		})
	}
}

// TestValidateResultsContentScaffoldFinalContainsPlaceholder pins that the raw
// angle-bracket scaffold is rejected in the final phase too (it carries other
// section problems as well, so this asserts containment, not the exact set).
func TestValidateResultsContentScaffoldFinalContainsPlaceholder(t *testing.T) {
	src := []byte("# <Change title> — Results\n\n## Outcome\n\n<What was delivered.>\n\n## Findings and limitations\n\n### <Finding>\n\n<What was observed.>\n")
	fs := ValidateResultsContent(src, ResultsPhaseFinal)
	var found bool
	for _, f := range fs {
		if f.Reason == "results-placeholder" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want results-placeholder among findings, got %v", fs)
	}
}

// TestValidateResultsContentEmptyResultIsSlice guards that a valid document
// returns an empty (non-nil is not required, but len 0) slice, and that Message
// is populated on a refusal for operator diagnostics.
func TestValidateResultsContentMessagePopulated(t *testing.T) {
	src := []byte("# T — Results\n\n## Outcome\n")
	fs := ValidateResultsContent(src, ResultsPhaseFinal)
	if len(fs) == 0 {
		t.Fatalf("want a refusal for an empty Outcome body")
	}
	for _, f := range fs {
		if strings.TrimSpace(f.Message) == "" {
			t.Fatalf("finding %q carries no message", f.Reason)
		}
	}
}
