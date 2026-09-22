package app

import (
	"reflect"
	"strings"
	"testing"
)

// Change 0414: reserved authoring prompts are DERIVED from the results
// template's angle-bracket instruction spans — comments and code excluded,
// wrapped prompts collapsed, whitespace normalized, case and punctuation kept
// exact, first-appearance order, deduplicated.
func TestExtractTemplatePrompts(t *testing.T) {
	tmpl := "<!-- authoring notes: <not a prompt> -->\n" +
		"# <Change title> — Results\n\n" +
		"**Human action:** <Whether action is needed, in one or two\nsentences, wrapped across lines.>\n\n" +
		"## Steps\n\n" +
		"1. <Concrete step.>\n   Expected: <Observable result.>\n" +
		"2. <Concrete step.>\n" +
		"### Important — <short name of the action>\n\n" +
		"```\n<Fenced example, not a prompt>\n```\n\n" +
		"Literal `<inline code, not a prompt>` here.\n"
	got, err := extractTemplatePrompts([]byte(tmpl))
	if err != nil {
		t.Fatalf("extractTemplatePrompts: %v", err)
	}
	want := []string{
		"<Change title>",
		"<Whether action is needed, in one or two sentences, wrapped across lines.>",
		"<Concrete step.>",
		"<Observable result.>",
		"<short name of the action>",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prompts = %#v\nwant %#v", got, want)
	}
}

func TestExtractTemplatePromptsCRLFAndReflowIdentity(t *testing.T) {
	lf := "# <Change title> — Results\n\n<A wrapped\nprompt.>\n"
	crlf := strings.ReplaceAll("# <Change title> — Results\n\n<A wrapped prompt.>\n", "\n", "\r\n")
	a, err := extractTemplatePrompts([]byte(lf))
	if err != nil {
		t.Fatalf("lf: %v", err)
	}
	b, err := extractTemplatePrompts([]byte(crlf))
	if err != nil {
		t.Fatalf("crlf: %v", err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("reflow/CRLF changed prompt identity: %#v vs %#v", a, b)
	}
}

// A missing, malformed, or empty prompt source is a setup FAILURE — never a
// silently-empty prompt set that would validate everything as filled.
func TestExtractTemplatePromptsFailsClosed(t *testing.T) {
	for name, tmpl := range map[string]string{
		"no prompts at all":      "# Title — Results\n\nProse only.\n",
		"only commented prompts": "<!-- <Hidden> -->\n# Title\n\nProse.\n",
		"unterminated span":      "# Title\n\n<Never closed\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := extractTemplatePrompts([]byte(tmpl)); err == nil {
				t.Fatalf("want error, got prompts %#v", got)
			}
		})
	}
}

// The production path: prompts derived from the EMBEDDED canonical template
// must succeed, be nonempty, and cover the known headline slots — including
// the lowercase "<short name of the action>" the retired uppercase heuristic
// could not see.
func TestResultsTemplatePromptsEmbedded(t *testing.T) {
	prompts, err := resultsTemplatePrompts()
	if err != nil {
		t.Fatalf("resultsTemplatePrompts (embedded): %v", err)
	}
	if len(prompts) == 0 {
		t.Fatal("embedded template yielded zero prompts")
	}
	wantContains := []string{"<Change title>", "<short name of the action>", "<Concrete step.>", "<Observable result.>"}
	for _, w := range wantContains {
		found := false
		for _, p := range prompts {
			if p == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("embedded prompts missing %q; got %#v", w, prompts)
		}
	}
}

func TestMaskInlineLiterals(t *testing.T) {
	cases := map[string]struct{ in, mustNotContain string }{
		"html comment masked":       {"a <!-- <P> --> b", "<P>"},
		"unclosed comment masks on": {"a <!-- <P> and <Q>", "<P>"},
		"inline code masked":        {"a `<P>` b", "<P>"},
		"double-backtick span":      {"a ``x `<P>` y`` b", "<P>"},
		"escaped bracket masked":    {`a \<P> b`, "<P"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := maskInlineLiterals(tc.in); strings.Contains(got, tc.mustNotContain) {
				t.Fatalf("maskInlineLiterals(%q) = %q still contains %q", tc.in, got, tc.mustNotContain)
			}
		})
	}
	// An UNCLOSED backtick run is literal text, not a mask-to-the-end.
	if got := maskInlineLiterals("a ` b <P>"); !strings.Contains(got, "<P>") {
		t.Fatalf("unclosed backtick swallowed following text: %q", got)
	}
	// Masking must not SPLICE fragments into a prompt: the sentinel is
	// non-whitespace, so text around a masked span stays non-contiguous.
	spliced := normalizeWS(maskInlineLiterals("<Change `x` title>"))
	if strings.Contains(spliced, "<Change title>") {
		t.Fatalf("mask spliced fragments into a prompt: %q", spliced)
	}
}
