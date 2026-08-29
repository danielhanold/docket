// Package suiterunner owns the Go-native whole-suite test runner that backs
// `docket development test` (change 0318). It discovers the same tests/test_*.sh
// corpus the frozen Bash oracle (scripts/run-tests.sh) runs, applies the same
// per-file wall-clock budget policy, isolates and schedules each target, and
// maps the aggregate to the oracle's exit contract.
//
// This file owns deterministic discovery and whole-input-set target validation.
// Discovery mirrors the oracle's rule: tests/test_*.sh at maxdepth 1, sorted by
// byte value (C collation). Validation follows the learning
// validate-the-whole-input-set-first — every path is checked and every
// violation reported together, never stopping at the first.
package suiterunner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Target is one scheduled suite member. Base is the stable identity join key
// (basename), mirroring the Bash oracle's "Basename is the join key" rule.
type Target struct {
	Path    string // as given (absolute or repo-relative)
	Base    string // "test_x.sh"
	Ceiling int    // seconds
	Mode    Mode
}

// Discover returns tests/test_*.sh under dir (maxdepth 1), sorted by byte
// value (C collation). Fail-closed: an unreadable dir or an empty result is
// an error, never an empty pass.
func Discover(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("suiterunner: cannot read tests dir %q: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue // maxdepth 1: subdir members are excluded
		}
		name := e.Name()
		if strings.HasPrefix(name, "test_") && strings.HasSuffix(name, ".sh") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("suiterunner: no test files (test_*.sh) under %q", dir)
	}
	// Byte sort of the basenames == C collation; every path shares the dir
	// prefix, so sorting names then joining preserves that order.
	sort.Strings(names)
	paths := make([]string, len(names))
	for i, name := range names {
		paths[i] = filepath.Join(dir, name)
	}
	return paths, nil
}

// ResolveTargets joins discovered/explicit paths with the budget table and
// validates the whole input set before anything runs (learning
// validate-the-whole-input-set-first): every path must exist, and no two
// targets may share a basename. All violations are reported together.
func ResolveTargets(paths []string, budgets map[string]budgetRow) ([]Target, error) {
	targets := make([]Target, 0, len(paths))
	var problems []string

	// First pass: existence and budget join, in input order.
	byBase := make(map[string][]string)
	for _, p := range paths {
		base := filepath.Base(p)
		byBase[base] = append(byBase[base], p)
		if _, err := os.Stat(p); err != nil {
			problems = append(problems, fmt.Sprintf("missing target: %s", p))
			continue
		}
		row, ok := budgets[base]
		if !ok {
			row = budgetRow{Ceiling: DefaultCeiling, Mode: ModeParallel}
		}
		targets = append(targets, Target{
			Path:    p,
			Base:    base,
			Ceiling: row.Ceiling,
			Mode:    row.Mode,
		})
	}

	// Second pass: duplicate-basename detection across the whole set. Report the
	// colliding paths together, in the deterministic order of their basenames.
	dupBases := make([]string, 0)
	for base, ps := range byBase {
		if len(ps) > 1 {
			dupBases = append(dupBases, base)
		}
	}
	sort.Strings(dupBases)
	for _, base := range dupBases {
		problems = append(problems, fmt.Sprintf("duplicate basename %q shared by: %s", base, strings.Join(byBase[base], ", ")))
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("suiterunner: invalid target set:\n  %s", strings.Join(problems, "\n  "))
	}
	return targets, nil
}
