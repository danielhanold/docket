package repoguard

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/harness"
)

// A feature-scoped Codex role enters through agent.enter, whose startup guard
// reads this line from the unchanged request file. Every dispatch owner must
// therefore put the same structured input in its payload, rather than relying
// on a retired runner facade to carry a worktree flag out of band.
const featureWorktreeDispatchLine = "Feature worktree: <absolute canonical feature-worktree root>"

type featureDispatchSite struct {
	rel    string
	block  int
	target string
	text   string
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

// discoverFeatureDispatchSites finds maintained workflow payloads by shape:
// markdown dispatch payload blocks that name a typed feature role. The whole
// skills tree is the population; generated mirrors and immutable history are
// categorically excluded by maintainedPop, not named here.
func discoverFeatureDispatchSites(t *testing.T, root string, targets map[string]bool) []featureDispatchSite {
	t.Helper()
	var sites []featureDispatchSite
	for _, rel := range maintainedPop(t, root) {
		if !strings.HasPrefix(rel, "skills/") || !strings.HasSuffix(rel, ".md") {
			continue
		}
		for i, block := range strings.Split(readMaintained(t, root, rel), "\n\n") {
			if !strings.Contains(strings.ToLower(block), "dispatch payload") {
				continue
			}
			for target := range targets {
				if strings.Contains(block, target) {
					sites = append(sites, featureDispatchSite{rel: rel, block: i + 1, target: target, text: block})
				}
			}
		}
	}
	return sites
}

func hasExactFeatureWorktreeLine(block string) bool {
	for _, line := range strings.Split(block, "\n") {
		if strings.TrimSpace(line) == featureWorktreeDispatchLine {
			return true
		}
	}
	return false
}

func TestFeatureDispatchPayloadsCarryCanonicalWorktree(t *testing.T) {
	root := guardRoot(t)
	targets := featureDispatchTargets(t)
	sites := discoverFeatureDispatchSites(t, root, targets)
	seen := map[string]bool{}
	var violations []string
	for _, site := range sites {
		seen[site.target] = true
		if !hasExactFeatureWorktreeLine(site.text) {
			violations = append(violations, fmt.Sprintf("%s:block-%d dispatch payload for %s lacks exact line %q", site.rel, site.block, site.target, featureWorktreeDispatchLine))
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
		violations = append(violations, "feature roles without a maintained dispatch payload site: "+strings.Join(missing, ", "))
	}
	if len(sites) < len(targets) {
		t.Errorf("feature-dispatch population is incomplete: sites=%d targets=%d", len(sites), len(targets))
	}
	if len(violations) != 0 {
		t.Errorf("feature worktree dispatch contract violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		if hasExactFeatureWorktreeLine("Feature worktree: relative/path") {
			t.Error("relative worktree fixture satisfied exact-line matcher")
		}
		if hasExactFeatureWorktreeLine("payload\nFeature worktree: <absolute canonical feature-worktree root>\n") == false {
			t.Error("exact worktree fixture did not satisfy matcher")
		}
	})
}
