package app

import (
	"fmt"
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
	src := []byte("<!-- docket:backlink:start (generated — do not hand-edit) -->\n> home\n<!-- docket:backlink:end -->\n\n# Change title — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nDelivered the thing; behavior X now refuses Y.\n")
	if fs := ValidateResultsContent(src, ResultsPhaseFinal); len(fs) != 0 {
		t.Fatalf("want valid, got %v", fs)
	}
}

func TestValidateResultsContentFillerSectionRefusedFinal(t *testing.T) {
	src := []byte("# T — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nReal outcome prose.\n\n## Findings and limitations\n\nNone.\n")
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
			src:   "# T — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nDelivered the thing; behavior X now refuses Y.\n",
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
			src:   "# T — Results\n\n**Human action:** No required action.\n\n## Human testing\n\n### Scenario\n\nDo X and observe Y.\n",
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
			src:   "# T — Results\n\n**Human action:** No required action.\n\n## Outcome\n\n## Notes\n\nSome real notes.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-outcome-empty"},
		},
		{
			name:  "empty subsection under filled parent refused final",
			src:   "# T — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nReal outcome.\n\n## Human testing\n\nSetup paragraph here.\n\n### Empty scenario\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-empty-section"},
		},
		{
			name:  "filled sub under bodyless parent accepted final",
			src:   "# T — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nReal outcome.\n\n## Human testing\n\n### Scenario\n\nDo X and observe Y.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "None in prose is legal final",
			src:   "# T — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nNone of the flags are read at startup, so the change is inert.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			// Finding 1: a bare content word (TODO/FIXME/TBD/XXX/TKTK/PLACEHOLDER)
			// in results prose is NOT unfilled scaffolding — results legitimately
			// discuss these tokens — so it is accepted, not refused as a placeholder.
			name:  "content word TODO in results prose accepted checkpoint",
			src:   "# T — Results\n\n## Outcome\n\nTODO finish this.\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "raw template scaffold refused checkpoint",
			src:   "# <Change title> — Results\n\n## Outcome\n\n<What was delivered and how the behavior changed.>\n",
			phase: ResultsPhaseCheckpoint,
			want:  []string{"results-placeholder"},
		},
		{
			name:  "outcome-looking heading in fence ignored, real outcome validates",
			src:   "# T — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nReal outcome prose.\n\n## Notes\n\n```\n## Outcome\n```\n",
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
			src:   backlink + "# T — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "N/A whole-section filler refused final",
			src:   "# T — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nReal outcome.\n\n## Follow-ups\n\nN/A\n",
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

// TestResultsPlaceholderRedesign covers placeholder detection across two axes
// that the retired capitalization heuristic got wrong, plus the change-0414
// derived-prompt matcher (firstTemplatePrompt):
//
//	Finding 1 — bare content words TODO/FIXME/TBD/XXX/TKTK/PLACEHOLDER in results
//	prose are legitimate discussion (especially in ## Findings and limitations /
//	## Follow-ups) and must never refuse.
//
//	Finding 2 — legitimate inline HTML (<details>, <summary>, <br>, <sub>, and
//	their UPPERCASE spellings) and autolinks (<mailto:>, <tel:>, <HTTPS://…>) are
//	not emitted template prompts and are accepted regardless of case.
//
// Detection now matches only the reserved authoring prompts DERIVED from the
// canonical results template (skills/docket-implement-next/results-template.md),
// so a lowercase prompt like `<short name of the action>` is caught while
// uppercase markup passes.
func TestResultsPlaceholderRedesign(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		phase ResultsPhase
		want  []string
	}{
		// Finding 1: content words in real results prose are accepted, in both
		// the sections most prone to them and both phases.
		{
			name:  "FIXME discussed in findings accepted final",
			src:   "# T — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nDelivered the retry fix.\n\n## Findings and limitations\n\n### Retry logic\n\nWe still need to address the FIXME in retry logic; it is out of scope here.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "TODO deferred to a change accepted final",
			src:   "# T — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nShipped the parser.\n\n## Follow-ups\n\n### Deferred cleanup\n\nThe TODO is deferred to change 0NNN because it needs a schema change.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "TBD mentioned in outcome prose accepted checkpoint",
			src:   "# T — Results\n\n## Outcome\n\nThe exact rollout date is TBD pending a downstream review.\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "XXX and PLACEHOLDER mentioned in prose accepted final",
			src:   "# T — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nWe replaced the literal XXX marker and the PLACEHOLDER token in the fixture with real values.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		// Finding 2: legitimate inline HTML and non-http autolinks are accepted.
		{
			name:  "inline HTML details/summary accepted final",
			src:   "# T — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nAdded a collapsible block:\n\n<details>\n<summary>Show detail</summary>\nBody text.\n</details>\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "inline HTML br and sub accepted checkpoint",
			src:   "# T — Results\n\n## Outcome\n\nLine one.\n<br>\nH<sub>2</sub>O reference.\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "mailto autolink accepted final",
			src:   "# T — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nContact the owner:\n\n<mailto:owner@example.com>\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "tel autolink accepted checkpoint",
			src:   "# T — Results\n\n## Outcome\n\nSupport line:\n\n<tel:+15550000000>\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		// Rejection: ACTUAL emitted template prompts, in every slot shape the
		// template uses (change 0414 — derived prompts, not capitalization).
		{
			name:  "unfilled H1 title prompt refused checkpoint",
			src:   "# <Change title> — Results\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseCheckpoint,
			want:  []string{"results-placeholder"},
		},
		{
			name:  "unfilled inline heading prompt refused checkpoint (lowercase prompt)",
			src:   "# T — Results\n\n## Human actions and testing\n\n### Important — <short name of the action>\n\nReal body.\n",
			phase: ResultsPhaseCheckpoint,
			want:  []string{"results-placeholder"},
		},
		{
			name:  "unfilled inline Expected prompt refused checkpoint",
			src:   "# T — Results\n\n## Outcome\n\nReal.\n\n## Human actions and testing\n\n1. Run the tool.\n   Expected: <Observable result.>\n",
			phase: ResultsPhaseCheckpoint,
			want:  []string{"results-placeholder"},
		},
		{
			name:  "wrapped prompt reflowed across different line breaks still refused",
			src:   "# T — Results\n\n## Outcome\n\n<The original problem,\nthe delivered behavior, and any material departure from the agreed design — lead with observable effects. Explain unfamiliar Docket concepts when necessary; include method names, stored fields, or internal identifiers\nonly when they help the reader understand a consequence or take action.>\n",
			phase: ResultsPhaseCheckpoint,
			want:  []string{"results-placeholder"},
		},

		// Deliberate consequences (spec "Consequences are deliberate"):
		{
			name:  "uppercase HTML tags accepted checkpoint",
			src:   "# T — Results\n\n## Outcome\n\nAdded markup:\n\n<BR>\n<DETAILS><SUMMARY>More</SUMMARY>Body</DETAILS>\n<DIV CLASS=\"x\">attribute-bearing</DIV>\n<CUSTOM-TAG>custom</CUSTOM-TAG>\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "uppercase URI schemes accepted final",
			src:   "# T — Results\n\n**Human action:** <MAILTO:person@example.com> is the contact; see <HTTPS://example.com>.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "custom placeholder-looking prose is not guessed",
			src:   "# T — Results\n\n## Outcome\n\n<What was delivered and how the behavior changed.>\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "prompt quoted in inline code is a literal example",
			src:   "# T — Results\n\n## Outcome\n\nThe validator now rejects `<Change title>` when left unfilled.\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "prompt inside a fenced example is a literal example",
			src:   "# T — Results\n\n## Outcome\n\nReal.\n\n```\n# <Change title> — Results\n```\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "escaped opening bracket is a literal example",
			src:   "# T — Results\n\n## Outcome\n\nAuthors must replace \\<Change title> with the real title.\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "unescaped emitted prompt amid substantive content still refused",
			src:   "# T — Results\n\n## Outcome\n\nWe shipped the fix and verified it end to end.\n<Concrete step.>\nMore real prose after.\n",
			phase: ResultsPhaseCheckpoint,
			want:  []string{"results-placeholder"},
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

// Change 0440: the final boundary requires a substantive "Human action:"
// statement between the H1 title and the first H2. Detection is shape-keyed
// (label + colon, emphasis tolerated), never an enumerated statement list.
func TestValidateResultsContentActionStatement(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		phase ResultsPhase
		want  []string
	}{
		{
			name:  "bold statement accepted final",
			src:   "# T — Results\n\n**Human action:** No required action or additional functional testing identified.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "plain unbolded statement accepted final (shape, not spelling)",
			src:   "# T — Results\n\nhuman action: Important verification remains; follow the scenario below.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "colon-outside-bold form accepted final",
			src:   "# T — Results\n\n**Human action**: No required action.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "wrapped statement paragraph accepted final",
			src:   "# T — Results\n\n**Human action:**\nImportant verification remains before relying on this feature.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "missing statement refused final",
			src:   "# T — Results\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-action-statement-missing"},
		},
		{
			name:  "empty statement refused final",
			src:   "# T — Results\n\n**Human action:**\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-action-statement-empty"},
		},
		{
			name:  "filler statement refused final",
			src:   "# T — Results\n\n**Human action:** None.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-action-statement-empty"},
		},
		{
			name:  "statement still the template prompt refused final",
			src:   "# T — Results\n\n**Human action:** <Whether human action is needed, in one or two sentences, consistent\nwith the sections below. During implementation this may say the assessment is pending;\nfinal results give a settled assessment.>\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-placeholder", "results-action-statement-empty"},
		},
		{
			name:  "statement beginning with legitimate uppercase markup accepted final",
			src:   "# T — Results\n\n**Human action:** <DETAILS> rendering must be checked by eye in the PR preview.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
		{
			name:  "statement below first H2 does not count",
			src:   "# T — Results\n\n## Outcome\n\n**Human action:** No required action.\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-action-statement-missing"},
		},
		{
			name:  "statement only inside fence does not count",
			src:   "# T — Results\n\n```\n**Human action:** No required action.\n```\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  []string{"results-action-statement-missing"},
		},
		{
			name:  "missing statement accepted checkpoint",
			src:   "# T — Results\n\n## Outcome\n\nBuild landed; review pending.\n",
			phase: ResultsPhaseCheckpoint,
			want:  nil,
		},
		{
			name:  "pending assessment prose accepted final (settledness is coordinator judgment)",
			src:   "# T — Results\n\n**Human action:** Assessment pending until review completes.\n\n## Outcome\n\nReal outcome prose.\n",
			phase: ResultsPhaseFinal,
			want:  nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := reasonsOf(ValidateResultsContent([]byte(tc.src), tc.phase))
			if !sameReasons(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestResultsPlaceholderScaffoldMessageIsScaffold pins that when the placeholder
// finding fires it is the placeholder reason with a message naming the specific
// unfilled template prompt (change 0414) — the only trigger now is a derived
// template prompt, so the diagnostic can never misdescribe a content-word match
// (Finding 1's diagnostic complaint).
func TestResultsPlaceholderScaffoldMessageIsScaffold(t *testing.T) {
	src := []byte("# <Change title> — Results\n\n## Outcome\n\nReal outcome prose.\n")
	fs := ValidateResultsContent(src, ResultsPhaseCheckpoint)
	if len(fs) != 1 || fs[0].Reason != "results-placeholder" {
		t.Fatalf("want one results-placeholder, got %v", fs)
	}
	if !strings.Contains(fs[0].Message, "unfilled template prompt") {
		t.Fatalf("want a message naming the unfilled template prompt, got %q", fs[0].Message)
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

// A missing/unusable prompt source is a validator SETUP failure: a stable
// results-template-invalid finding whose message blames the template, never a
// pass and never an accusation against the author's document.
func TestValidateResultsContentTemplateSetupFailure(t *testing.T) {
	orig := resultsTemplatePrompts
	defer func() { resultsTemplatePrompts = orig }()
	resultsTemplatePrompts = func() ([]string, error) {
		return nil, fmt.Errorf("boom: no template")
	}
	fs := ValidateResultsContent([]byte("# T — Results\n\n## Outcome\n\nReal prose.\n"), ResultsPhaseCheckpoint)
	if len(fs) != 1 || fs[0].Reason != "results-template-invalid" {
		t.Fatalf("findings = %#v, want exactly one results-template-invalid", fs)
	}
	if !strings.Contains(fs[0].Message, "validator setup failure") {
		t.Fatalf("message must name the failure as the validator's, got %q", fs[0].Message)
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
