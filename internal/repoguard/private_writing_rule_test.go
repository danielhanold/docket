package repoguard

import (
	"regexp"
	"strings"
	"testing"
)

// TestPrivateWritingRuleReachesEveryWriter guards the private-repository writing
// rule's prose. The rule is stated once, in docket-convention's *Branch model*,
// and every skill or agent that writes something that ships through the feature
// branch or the PR (commit messages, code, specs, PR text) names it. The two
// producer paragraphs that act on a `leak-detected` refusal must also say how the
// run records it: implement-next through `change.halt`, finalize through
// `finalize.block`.
//
// Matching is case-insensitive with whitespace runs collapsed, so a hard wrap
// inside the phrase still matches.

const (
	privateRuleAnchor = "**Private-repository writing rule.**"
	privateRulePhrase = "private-repository writing rule"
	privateRuleWindow = 600
)

// privateRuleWriters are the files that hand work to something that writes
// shipped text. Each must name the rule.
var privateRuleWriters = []string{
	"skills/docket-build-task/SKILL.md",
	"skills/docket-build/SKILL.md",
	"skills/docket-implement-next/SKILL.md",
	"skills/docket-implement-next/references/fix-pass.md",
	"skills/docket-implement-next/references/edge-paths.md",
	"skills/docket-new-change/SKILL.md",
	"skills/docket-groom-next/SKILL.md",
	"skills/docket-auto-groom/SKILL.md",
	"skills/docket-brainstorm/SKILL.md",
	"agents/docket-plan-writer.md",
}

// privateRuleProducers are the paragraphs that turn a leak-detected refusal into
// a recorded stop. Each needs one blank-line paragraph holding every token.
var privateRuleProducers = []struct {
	file   string
	tokens []string
}{
	{"skills/docket-implement-next/SKILL.md", []string{"leak-detected", "change.halt"}},
	{"skills/docket-finalize-change/SKILL.md", []string{"leak-detected", "finalize.block"}},
}

var blankLineSplit = regexp.MustCompile(`\n[ \t]*\n`)

func normalizeProse(s string) string {
	return strings.ToLower(wsRun.ReplaceAllString(s, " "))
}

// paragraphWithAll reports whether some blank-line-separated paragraph of
// content contains every token verbatim.
func paragraphWithAll(content string, tokens []string) bool {
	for _, para := range blankLineSplit.Split(content, -1) {
		all := true
		for _, tok := range tokens {
			if !strings.Contains(para, tok) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

func TestPrivateWritingRuleReachesEveryWriter(t *testing.T) {
	conv := readRepoFile(t, "skills/docket-convention/SKILL.md")
	if n := strings.Count(conv, privateRuleAnchor); n != 1 {
		t.Errorf("docket-convention: %q appears %d times, want exactly 1", privateRuleAnchor, n)
	} else {
		i := strings.Index(conv, privateRuleAnchor)
		end := min(i+len(privateRuleAnchor)+privateRuleWindow, len(conv))
		window := normalizeProse(conv[i:end])
		for _, claim := range []string{"change ids", "metadata branch"} {
			if !strings.Contains(window, claim) {
				t.Errorf("docket-convention: the writing rule's first %d bytes do not name %q", privateRuleWindow, claim)
			}
		}
	}

	if len(privateRuleWriters) < 10 {
		t.Fatalf("writer table shrank to %d rows, floor is 10", len(privateRuleWriters))
	}
	for _, rel := range privateRuleWriters {
		if !strings.Contains(normalizeProse(readRepoFile(t, rel)), privateRulePhrase) {
			t.Errorf("%s does not name the %s", rel, privateRulePhrase)
		}
	}

	for _, p := range privateRuleProducers {
		if !paragraphWithAll(readRepoFile(t, p.file), p.tokens) {
			t.Errorf("%s: no paragraph holds all of %q", p.file, p.tokens)
		}
	}
}
