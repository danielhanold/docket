package repoguard

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Ports two CORRESPONDENCE guards off the retired Bash suite, each written in
// BOTH directions and anchored on the consuming artifact rather than an
// allowlist (correspondence-guard-runs-one-way):
//
//   tests/test_script_contracts_coverage.sh -> TestScriptContractsCoverage
//   tests/test_runtime_budgets.sh (registry<->files half only) -> TestRuntimeBudgetsCorrespondence
//
// NOTE on the runtime-budgets split: only the registry<->files correspondence is
// ported here. The budget CEILING / sum / serial-pin / merge-gate-report
// mechanics moved to internal/suiterunner and are re-proven by that package's own
// tests (change 0370, Task 6); this guard owns the population correspondence.

// ---------------------------------------------------------------------------
// scripts/<name>.sh <-> scripts/<name>.md coverage (top level and runners/)
// ---------------------------------------------------------------------------

var (
	scriptTop     = regexp.MustCompile(`^scripts/([^/]+)\.(sh|md)$`)
	scriptRunners = regexp.MustCompile(`^scripts/runners/([^/]+)\.(sh|md)$`)
)

// pairCoverage returns the orphans in a name->{sh,md} presence map: a base with
// a .sh but no .md, or a .md but no .sh.
func pairCoverage(dir string, present map[string]map[string]bool) []string {
	var orphans []string
	names := make([]string, 0, len(present))
	for n := range present {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		exts := present[n]
		if exts["sh"] && !exts["md"] {
			orphans = append(orphans, fmt.Sprintf("%s/%s.sh has no %s/%s.md contract", dir, n, dir, n))
		}
		if exts["md"] && !exts["sh"] {
			orphans = append(orphans, fmt.Sprintf("orphaned %s/%s.md (no %s/%s.sh)", dir, n, dir, n))
		}
	}
	return orphans
}

func TestScriptContractsCoverage(t *testing.T) {
	root := guardRoot(t)
	top := map[string]map[string]bool{}
	runners := map[string]map[string]bool{}
	scanned := 0
	for _, rel := range maintainedPop(t, root) {
		if m := scriptTop.FindStringSubmatch(rel); m != nil {
			if top[m[1]] == nil {
				top[m[1]] = map[string]bool{}
			}
			top[m[1]][m[2]] = true
			scanned++
			continue
		}
		if m := scriptRunners.FindStringSubmatch(rel); m != nil {
			if runners[m[1]] == nil {
				runners[m[1]] = map[string]bool{}
			}
			runners[m[1]][m[2]] = true
			scanned++
		}
	}
	// Population floor: the runners generator products survive change 0370, so at
	// least a few pairs are always present; a collapse to near-zero means the scan
	// stopped reaching scripts/.
	if scanned < 4 {
		t.Fatalf("population floor: only %d scripts/*.{sh,md} scanned (expected >= 4)", scanned)
	}
	var orphans []string
	orphans = append(orphans, pairCoverage("scripts", top)...)
	orphans = append(orphans, pairCoverage("scripts/runners", runners)...)
	if len(orphans) != 0 {
		t.Errorf("script<->contract coverage gaps:\n%s", strings.Join(orphans, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		fixture := map[string]map[string]bool{
			"paired": {"sh": true, "md": true},
			"orphsh": {"sh": true},
			"orphmd": {"md": true},
		}
		got := pairCoverage("scripts", fixture)
		if len(got) != 2 {
			t.Errorf("pair-coverage scanner should have found 2 orphans, got %d: %v", len(got), got)
		}
	})
}

// ---------------------------------------------------------------------------
// runtime-budgets registry <-> tests/test_*.sh correspondence
// ---------------------------------------------------------------------------

var testFileRe = regexp.MustCompile(`^tests/test_[^/]+\.sh$`)

// budgetRows parses the registry, returning the ordered col-1 paths of the
// data rows and any malformed/duplicate diagnostics.
func budgetRows(content string) (paths []string, problems []string) {
	seen := map[string]bool{}
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			problems = append(problems, fmt.Sprintf("row %d is not <path>TAB<secs>TAB<mode>: %q", i+1, line))
			continue
		}
		p := fields[0]
		if seen[p] {
			problems = append(problems, fmt.Sprintf("row %d duplicates path %q", i+1, p))
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths, problems
}

func TestRuntimeBudgetsCorrespondence(t *testing.T) {
	root := guardRoot(t)
	registry := readMaintained(t, root, "tests/runtime-budgets.tsv")
	rowPaths, problems := budgetRows(registry)
	if len(problems) != 0 {
		t.Errorf("malformed budget registry rows:\n%s", strings.Join(problems, "\n"))
	}
	if len(rowPaths) < 20 {
		t.Fatalf("population floor: only %d budget rows (expected >= 20)", len(rowPaths))
	}

	rowSet := make(map[string]bool, len(rowPaths))
	for _, p := range rowPaths {
		rowSet[p] = true
	}
	fileSet := map[string]bool{}
	for _, rel := range maintainedPop(t, root) {
		if testFileRe.MatchString(rel) {
			fileSet[rel] = true
		}
	}
	if len(fileSet) < 20 {
		t.Fatalf("population floor: only %d tests/test_*.sh files found (expected >= 20)", len(fileSet))
	}

	// Forward: every live test file carries a budget row.
	var missingRow []string
	for f := range fileSet {
		if !rowSet[f] {
			missingRow = append(missingRow, f)
		}
	}
	// Reverse (anchored on the registry, the consuming artifact): every budget
	// row names a live test file — a pruned/renamed file must be a conscious
	// registry edit, not a silent drift.
	var deadRow []string
	for _, p := range rowPaths {
		if !fileSet[p] {
			deadRow = append(deadRow, p)
		}
	}
	if len(missingRow) != 0 {
		slices.Sort(missingRow)
		t.Errorf("tests/test_*.sh files with no budget row:\n%s", strings.Join(missingRow, "\n"))
	}
	if len(deadRow) != 0 {
		slices.Sort(deadRow)
		t.Errorf("budget rows naming a non-existent test file:\n%s", strings.Join(deadRow, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		// A row without a file, and a file without a row, are both detected.
		paths, probs := budgetRows("tests/test_gone.sh\t10\tparallel\ntests/test_here.sh\t10\tparallel\n")
		if len(probs) != 0 {
			t.Fatalf("unexpected parse problems: %v", probs)
		}
		if !slices.Contains(paths, "tests/test_gone.sh") {
			t.Errorf("parser dropped a valid row")
		}
		// Malformed row (2 fields) and a duplicate are flagged.
		_, probs2 := budgetRows("tests/test_x.sh\t10\ntests/test_y.sh\t10\tparallel\ntests/test_y.sh\t9\tserial\n")
		if len(probs2) < 2 {
			t.Errorf("parser should flag a 2-field row AND a duplicate, got %v", probs2)
		}
	})
}
