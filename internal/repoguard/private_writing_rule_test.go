package repoguard

import (
	"regexp"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/harness"
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

// visibilityDispatchLine is the payload line that hands a feature-branch writer
// the repository's visibility.
const visibilityDispatchLine = "Visibility: <shared|private>"

// TestPrivateWritingRuleReachesEveryFeatureWriter derives the writer set rather
// than listing it: every feature-scoped role whose description does not declare
// it read-only writes text that ships through the feature branch. Each such role
// must state the rule and the `Visibility:` payload field in one source of its
// own instructions (its body, or one preloaded skill — docket-convention alone
// defines the rule but never tells a role to read its payload), and every
// marker-delimited dispatch naming it must carry the raw visibility line.
func TestPrivateWritingRuleReachesEveryFeatureWriter(t *testing.T) {
	catalog, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sources, err := harness.ParseInventory(catalog)
	if err != nil {
		t.Fatal(err)
	}
	writers := map[string]bool{}
	scoped := map[string]bool{}
	for _, src := range sources {
		if src.WorktreeScope != harness.WorktreeScopeFeature {
			continue
		}
		scoped[src.Name] = true
		if strings.Contains(strings.ToLower(src.Description), "read-only") {
			continue
		}
		writers[src.Name] = true
		ownTexts := []string{readRepoFile(t, "agents/"+src.Name+".md")}
		for _, skill := range src.Skills {
			ownTexts = append(ownTexts, readRepoFile(t, "skills/"+skill+"/SKILL.md"))
		}
		if !namesRuleAndVisibility(ownTexts) {
			t.Errorf("%s writes on the feature branch but no single source of its instructions names both %q and the %s", src.Name, "Visibility:", privateRulePhrase)
		}
	}
	if len(writers) < 4 {
		t.Fatalf("derived feature-writer set shrank to %d roles: %v", len(writers), writers)
	}

	root := guardRoot(t)
	dispatched := map[string]bool{}
	for _, rel := range maintainedPop(t, root) {
		if !strings.HasPrefix(rel, "skills/") || !strings.HasSuffix(rel, ".md") {
			continue
		}
		sites, _ := parseFeatureDispatches(rel, readMaintained(t, root, rel), scoped)
		for _, site := range sites {
			for _, target := range site.targets {
				if !writers[target] {
					continue
				}
				dispatched[target] = true
				if !hasExactLine(site.lines, visibilityDispatchLine) {
					t.Errorf("%s:%d-%d dispatch of %s lacks raw line %q", site.rel, site.start, site.end, target, visibilityDispatchLine)
				}
			}
		}
	}
	for w := range writers {
		if !dispatched[w] {
			t.Errorf("feature writer %s has no marker-delimited dispatch to check", w)
		}
	}

	t.Run("non_vacuity", func(t *testing.T) {
		if namesRuleAndVisibility([]string{"Visibility: private", "the " + privateRulePhrase}) {
			t.Error("rule and payload field split across sources was accepted")
		}
		if !namesRuleAndVisibility([]string{"Payload says `Visibility: private`; follow the Private-Repository\nwriting rule."}) {
			t.Error("hard-wrapped rule beside the payload field was rejected")
		}
	})
}

func namesRuleAndVisibility(texts []string) bool {
	for _, text := range texts {
		if strings.Contains(text, "Visibility:") && strings.Contains(normalizeProse(text), privateRulePhrase) {
			return true
		}
	}
	return false
}

func hasExactLine(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}
