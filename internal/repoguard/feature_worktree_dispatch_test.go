package repoguard

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/harness"
)

// A feature-scoped Codex role enters through agent.enter, whose startup guard
// reads this raw line from the unchanged request file. A marker-delimited
// dispatch instruction makes the executable payload boundary explicit rather
// than treating surrounding explanatory Markdown as an instruction.
const (
	featureWorktreeDispatchLine = "Feature worktree: <absolute canonical feature-worktree root>"
	featureDispatchEnd          = "<!-- docket:feature-dispatch:end -->"
)

var (
	featureDispatchStart  = regexp.MustCompile(`^<!-- docket:feature-dispatch:start targets=([a-z0-9-]+(?:,[a-z0-9-]+)*) -->$`)
	directFeatureDispatch = regexp.MustCompile("^Dispatch `(" + `docket-[a-z0-9-]+` + ")`")
	orderedMarkdownList   = regexp.MustCompile(`^[0-9]+\.\s`)
)

type featureDispatchSite struct {
	rel     string
	start   int
	end     int
	targets []string
	lines   []string
}

type directFeatureDispatchSite struct {
	rel    string
	line   int
	target string
}

// featureDispatchTargets reads the closed role scope from ParseInventory; a
// feature-role addition cannot evade this guard by omitting a hand-maintained
// target list.
func featureDispatchTargets(t *testing.T) map[string]bool {
	t.Helper()
	catalog, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sources, err := harness.ParseInventory(catalog)
	if err != nil {
		t.Fatal(err)
	}
	targets := map[string]bool{}
	for _, source := range sources {
		if source.WorktreeScope == harness.WorktreeScopeFeature {
			targets[source.Name] = true
		}
	}
	if len(targets) == 0 {
		t.Fatal("feature-role inventory is empty")
	}
	return targets
}

// parseFeatureDispatches recognizes only the maintained, marker-delimited
// executable dispatch shape. It validates marker balance and order before
// returning a bounded instruction, so prose and malformed marker ranges cannot
// accidentally satisfy the worktree contract.
func parseFeatureDispatches(rel, content string, targets map[string]bool) ([]featureDispatchSite, []string) {
	var sites []featureDispatchSite
	var problems []string
	var open *featureDispatchSite
	for i, line := range strings.Split(content, "\n") {
		lineNo := i + 1
		if strings.HasPrefix(line, "<!-- docket:feature-dispatch:start") {
			match := featureDispatchStart.FindStringSubmatch(line)
			if match == nil {
				problems = append(problems, fmt.Sprintf("%s:%d malformed feature-dispatch start marker", rel, lineNo))
				continue
			}
			if open != nil {
				problems = append(problems, fmt.Sprintf("%s:%d nested feature-dispatch start before block from line %d closes", rel, lineNo, open.start))
				continue
			}
			seen := map[string]bool{}
			var markerTargets []string
			for _, target := range strings.Split(match[1], ",") {
				if !targets[target] {
					problems = append(problems, fmt.Sprintf("%s:%d marker target %q is not feature-scoped", rel, lineNo, target))
				}
				if seen[target] {
					problems = append(problems, fmt.Sprintf("%s:%d marker target %q is repeated", rel, lineNo, target))
				}
				seen[target] = true
				markerTargets = append(markerTargets, target)
			}
			open = &featureDispatchSite{rel: rel, start: lineNo, targets: markerTargets}
			continue
		}
		if strings.HasPrefix(line, "<!-- docket:feature-dispatch:end") {
			if line != featureDispatchEnd {
				problems = append(problems, fmt.Sprintf("%s:%d malformed feature-dispatch end marker", rel, lineNo))
				continue
			}
			if open == nil {
				problems = append(problems, fmt.Sprintf("%s:%d feature-dispatch end has no start", rel, lineNo))
				continue
			}
			open.end = lineNo
			sites = append(sites, *open)
			open = nil
			continue
		}
		if open != nil {
			open.lines = append(open.lines, line)
		}
	}
	if open != nil {
		problems = append(problems, fmt.Sprintf("%s:%d feature-dispatch start has no end", rel, open.start))
	}
	return sites, problems
}

func hasExactFeatureWorktreeLine(lines []string) bool {
	for _, line := range lines {
		if line == featureWorktreeDispatchLine {
			return true
		}
	}
	return false
}

// discoverDirectFeatureDispatches recognizes docket's direct-dispatch grammar
// only in ordinary Markdown paragraph lines. Headings, tables, lists,
// blockquotes, indented/fenced code, and HTML-marker lines are structurally
// non-executable, so explanatory material cannot be mistaken for a dispatch.
// Indirect selection has no direct target token; its marker targets remain the
// authoritative, exhaustively checked declaration.
func discoverDirectFeatureDispatches(rel, content string, targets map[string]bool) []directFeatureDispatchSite {
	var sites []directFeatureDispatchSite
	inFence := false
	inComment := false
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if inComment {
			if strings.Contains(line, "-->") {
				inComment = false
			}
			continue
		}
		if strings.HasPrefix(trimmed, "<!--") {
			if !strings.Contains(trimmed, "-->") {
				inComment = true
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || !markdownParagraphLine(line) {
			continue
		}
		match := directFeatureDispatch.FindStringSubmatch(line)
		if match != nil && targets[match[1]] {
			sites = append(sites, directFeatureDispatchSite{rel: rel, line: i + 1, target: match[1]})
		}
	}
	return sites
}

func markdownParagraphLine(line string) bool {
	if line == "" || strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
		return false
	}
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "<!--") || strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, ">") || strings.HasPrefix(trimmed, "|") ||
		strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") ||
		strings.HasPrefix(trimmed, "+ ") {
		return false
	}
	return !orderedMarkdownList.MatchString(trimmed)
}

func featureDispatchProblems(sites []featureDispatchSite, targets map[string]bool) []string {
	seen := map[string]bool{}
	var problems []string
	for _, site := range sites {
		if !hasExactFeatureWorktreeLine(site.lines) {
			problems = append(problems, fmt.Sprintf("%s:%d-%d feature-dispatch payload lacks raw exact line %q", site.rel, site.start, site.end, featureWorktreeDispatchLine))
		}
		for _, target := range site.targets {
			seen[target] = true
		}
	}
	var missing []string
	for target := range targets {
		if !seen[target] {
			missing = append(missing, target)
		}
	}
	sort.Strings(missing)
	if len(missing) != 0 {
		problems = append(problems, "feature roles without a marker-delimited dispatch instruction: "+strings.Join(missing, ", "))
	}
	return problems
}

func directFeatureDispatchProblems(direct []directFeatureDispatchSite, bounded []featureDispatchSite) []string {
	var problems []string
	for _, dispatch := range direct {
		matches := 0
		declared := false
		for _, block := range bounded {
			if dispatch.rel != block.rel || dispatch.line <= block.start || dispatch.line >= block.end {
				continue
			}
			matches++
			for _, target := range block.targets {
				if target == dispatch.target {
					declared = true
				}
			}
		}
		if matches != 1 || !declared {
			problems = append(problems, fmt.Sprintf("%s:%d direct feature dispatch %q is not inside exactly one marker-delimited block naming that target", dispatch.rel, dispatch.line, dispatch.target))
		}
	}
	return problems
}

func TestFeatureDispatchPayloadsCarryCanonicalWorktree(t *testing.T) {
	root := guardRoot(t)
	targets := featureDispatchTargets(t)
	var sites []featureDispatchSite
	var direct []directFeatureDispatchSite
	var problems []string
	for _, rel := range maintainedPop(t, root) {
		if !strings.HasPrefix(rel, "skills/") || !strings.HasSuffix(rel, ".md") {
			continue
		}
		found, parseProblems := parseFeatureDispatches(rel, readMaintained(t, root, rel), targets)
		sites = append(sites, found...)
		direct = append(direct, discoverDirectFeatureDispatches(rel, readMaintained(t, root, rel), targets)...)
		problems = append(problems, parseProblems...)
	}
	problems = append(problems, featureDispatchProblems(sites, targets)...)
	problems = append(problems, directFeatureDispatchProblems(direct, sites)...)
	if len(sites) == 0 {
		t.Error("feature-dispatch population is empty")
	}
	if len(direct) == 0 {
		t.Error("direct feature-dispatch population is empty")
	}
	if len(problems) != 0 {
		t.Errorf("feature worktree dispatch contract violations (%d):\n%s", len(problems), strings.Join(problems, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		const target = "docket-plan-writer"
		start := "<!-- docket:feature-dispatch:start targets=" + target + " -->"
		alternativeWording := start + "\nUse an entirely different dispatch sentence.\n" + featureWorktreeDispatchLine + "\n" + featureDispatchEnd
		sites, problems := parseFeatureDispatches("fixture.md", alternativeWording, map[string]bool{target: true})
		if len(problems) != 0 || len(sites) != 1 || !hasExactFeatureWorktreeLine(sites[0].lines) {
			t.Errorf("marker shape depended on dispatch prose: sites=%+v problems=%v", sites, problems)
		}

		secondMissingLine := start + "\n" + featureWorktreeDispatchLine + "\n" + featureDispatchEnd + "\n" + start + "\nSecond dispatch uses different wording.\n" + featureDispatchEnd
		sites, problems = parseFeatureDispatches("fixture.md", secondMissingLine, map[string]bool{target: true})
		problems = append(problems, featureDispatchProblems(sites, map[string]bool{target: true})...)
		if len(problems) == 0 {
			t.Error("second marker-delimited dispatch without worktree line was accepted")
		}

		explanatory := "`" + target + "` is discussed here.\n" + featureWorktreeDispatchLine
		sites, problems = parseFeatureDispatches("fixture.md", explanatory, map[string]bool{target: true})
		if len(sites) != 0 || len(problems) != 0 {
			t.Errorf("unmarked explanatory prose was treated as a dispatch: sites=%+v problems=%v", sites, problems)
		}

		indented := start + "\n    " + featureWorktreeDispatchLine + "\n" + featureDispatchEnd
		sites, problems = parseFeatureDispatches("fixture.md", indented, map[string]bool{target: true})
		problems = append(problems, featureDispatchProblems(sites, map[string]bool{target: true})...)
		if len(problems) == 0 {
			t.Error("indented worktree line was accepted as raw payload input")
		}

		markedAndUnmarked := start + "\nDispatch `" + target + "` foreground.\n" + featureWorktreeDispatchLine + "\n" + featureDispatchEnd + "\nDispatch `" + target + "` foreground again."
		sites, problems = parseFeatureDispatches("fixture.md", markedAndUnmarked, map[string]bool{target: true})
		direct := discoverDirectFeatureDispatches("fixture.md", markedAndUnmarked, map[string]bool{target: true})
		problems = append(problems, directFeatureDispatchProblems(direct, sites)...)
		if len(problems) == 0 {
			t.Error("unmarked direct dispatch for an already-covered feature role was accepted")
		}

		explanatoryStructure := "# Dispatch `" + target + "`\n\n| instruction | target |\n|---|---|\n| Dispatch | `" + target + "` |\n\n```text\nDispatch `" + target + "` foreground.\n```"
		if got := discoverDirectFeatureDispatches("fixture.md", explanatoryStructure, map[string]bool{target: true}); len(got) != 0 {
			t.Errorf("structurally explanatory Markdown was treated as direct dispatch: %+v", got)
		}

		commentedExplanation := "<!-- explanatory dispatch discussion\nDispatch `" + target + "` foreground.\n-->"
		if got := discoverDirectFeatureDispatches("fixture.md", commentedExplanation, map[string]bool{target: true}); len(got) != 0 {
			t.Errorf("bounded explanatory comment was treated as direct dispatch: %+v", got)
		}
	})
}
