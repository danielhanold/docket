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

var featureDispatchStart = regexp.MustCompile(`^<!-- docket:feature-dispatch:start targets=([a-z0-9-]+(?:,[a-z0-9-]+)*) -->$`)

type featureDispatchSite struct {
	rel     string
	start   int
	end     int
	targets []string
	lines   []string
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

func TestFeatureDispatchPayloadsCarryCanonicalWorktree(t *testing.T) {
	root := guardRoot(t)
	targets := featureDispatchTargets(t)
	var sites []featureDispatchSite
	var problems []string
	for _, rel := range maintainedPop(t, root) {
		if !strings.HasPrefix(rel, "skills/") || !strings.HasSuffix(rel, ".md") {
			continue
		}
		found, parseProblems := parseFeatureDispatches(rel, readMaintained(t, root, rel), targets)
		sites = append(sites, found...)
		problems = append(problems, parseProblems...)
	}
	problems = append(problems, featureDispatchProblems(sites, targets)...)
	if len(sites) == 0 {
		t.Error("feature-dispatch population is empty")
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
	})
}
