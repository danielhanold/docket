// Package suiterunner owns the Go-native whole-suite test runner that backs
// `docket development test` (change 0318, contracted to the final topology by
// change 0370). It discovers the tests/test_*.sh corpus, applies the per-file
// wall-clock budget policy, isolates and schedules each target, and maps the
// aggregate to the runner's exit contract.
//
// This file owns deterministic, CATEGORY-DECLARED discovery and whole-input-set
// target validation. Discovery lists tests/test_*.sh at maxdepth 1, sorted by
// byte value (C collation), and REQUIRES each file to declare its suite category
// in a `# docket-suite:` header within its first 10 lines. A file with a missing,
// malformed, unknown, below-line-10, or trailing-text declaration is a discovery
// ERROR (fail closed) naming the file — never skipped, and never assigned a
// generic/legacy category (change 0370, acceptance 12: no dormant compatibility
// execution branch). Validation follows the learning
// validate-the-whole-input-set-first — every path is checked and every violation
// reported together, never stopping at the first.
package suiterunner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Category is a declared suite category. The three constants below are the WHOLE
// vocabulary the surviving runner admits; there is no default and no fallback.
type Category string

const (
	CategoryGo         Category = "go"               // a test_go_*.sh wrapper that runs `go test`
	CategoryInstall    Category = "posix-install"    // the retained install.sh POSIX product suite
	CategoryDownloader Category = "posix-downloader" // the retained release-downloader POSIX product suite
)

// suiteHeaderRe matches a category declaration EXACTLY: the whole line is
// `# docket-suite: ` followed by exactly one of the three category tokens and
// nothing else. Anchoring both ends is what rejects trailing text; the closed
// alternation is what rejects an unknown token. The header contract is fixed
// identically in tests/README.md and Task 5's suite files.
var suiteHeaderRe = regexp.MustCompile(`^# docket-suite: (go|posix-install|posix-downloader)$`)

// suiteHeaderScanLines bounds how far into a file a declaration may sit. A header
// on line 11 or later is not found, so the file fails closed — a bounded parse
// with no lenient fallback (change 0370, acceptance 12).
const suiteHeaderScanLines = 10

// DiscoveredTarget is one discovered suite file paired with its declared
// category, the output of Discover before the budget join.
type DiscoveredTarget struct {
	Path     string   // as given (absolute or repo-relative)
	Category Category // declared in the file's `# docket-suite:` header
}

// Target is one scheduled suite member. Base is the stable identity join key
// (basename), mirroring the "Basename is the join key" rule.
type Target struct {
	Path     string // as given (absolute or repo-relative)
	Base     string // "test_x.sh"
	Ceiling  int    // seconds
	Mode     Mode
	Category Category // carried from discovery
}

// parseCategory reads the first suiteHeaderScanLines lines of path and returns
// the declared category. It fails closed: an unreadable file, or a file with no
// exactly-matching declaration in that window, is an error naming the file. The
// error text never suggests a fallback category — an undeclared file is refused,
// not defaulted.
func parseCategory(path string) (Category, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("%s: cannot read for its suite-category declaration: %w", path, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for i := 0; i < suiteHeaderScanLines && sc.Scan(); i++ {
		if m := suiteHeaderRe.FindStringSubmatch(sc.Text()); m != nil {
			return Category(m[1]), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("%s: cannot read for its suite-category declaration: %w", path, err)
	}
	return "", fmt.Errorf("%s: missing or malformed suite-category declaration — the first %d lines must contain exactly one `# docket-suite: <go|posix-install|posix-downloader>` line", path, suiteHeaderScanLines)
}

// Discover returns tests/test_*.sh under dir (maxdepth 1), sorted by byte value
// (C collation), each paired with its declared category. Fail-closed on every
// axis: an unreadable dir, an empty result, or ANY file whose category
// declaration is missing/malformed/unknown/below-line-10/trailing-text is an
// error, never an empty or lenient pass. Undeclared files are reported together
// (validate-the-whole-input-set-first).
func Discover(dir string) ([]DiscoveredTarget, error) {
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

	targets := make([]DiscoveredTarget, 0, len(names))
	var problems []string
	for _, name := range names {
		p := filepath.Join(dir, name)
		cat, err := parseCategory(p)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		targets = append(targets, DiscoveredTarget{Path: p, Category: cat})
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("suiterunner: undeclared or malformed suite categories (fail closed):\n  %s", strings.Join(problems, "\n  "))
	}
	return targets, nil
}

// ResolveTargets joins discovered targets with the budget table and validates the
// whole input set before anything runs (learning
// validate-the-whole-input-set-first): every path must exist, and no two targets
// may share a basename. Each target's declared category is carried through. All
// violations are reported together.
func ResolveTargets(discovered []DiscoveredTarget, budgets map[string]budgetRow) ([]Target, error) {
	targets := make([]Target, 0, len(discovered))
	var problems []string

	// First pass: existence and budget join, in input order.
	byBase := make(map[string][]string)
	for _, d := range discovered {
		p := d.Path
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
			Path:     p,
			Base:     base,
			Ceiling:  row.Ceiling,
			Mode:     row.Mode,
			Category: d.Category,
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
