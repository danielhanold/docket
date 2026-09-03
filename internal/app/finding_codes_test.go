package app

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Plan-time census (change 0399, Task 3). Derived from a whole-repo grep, not
// hand-listed; the guard below — not this comment — is the enforcement. The
// registry-minted vocabulary is:
//
//   Code:-field literals (StatusFinding / domain.Finding / reposetup.Finding
//   built in package app):
//     artifact-missing, dangling-reference, docket-dir-foreign,
//     docket-worktree-ambiguous-registration, local-metadata-ahead,
//     local-metadata-diverged, metadata-root-foreign, metadata-root-unresolved,
//     metadata-worktree-dirty, migration-incomplete, prepare-local-state-unknown,
//     prepare-topology-unresolved, repository-fresh, repository-legacy,
//     sweep-pr-facts-unresolved, invalid-section-heading, invalid-section-intent,
//     invalid-section-markdown, invalid-slug, unknown-priority, unknown-type,
//     and the collection composites invalid-{depends_on,related,discovered_from,
//     adrs} / duplicate-{depends_on,related,discovered_from,adrs}.
//
//   Finding-constructor literal first args (lifecycleFinding / refuseLifecycle):
//     authored-input-too-large, duplicate-child_id, empty-attempt,
//     empty-child_pr_version, empty-commit, empty-evidence, empty-note-entry,
//     empty-path, empty-pr, empty-reason, empty-report, empty-version,
//     empty-why_deferred, empty-why_killed, invalid-change_id, invalid-child_id,
//     invalid-child_pr_number, invalid-head, invalid-id, invalid-note-entry,
//     workspace-service-unavailable, artifact-render-failed, not-found,
//     path-mismatch, section-edit-failed.
//
//   Non-literal mints that already reference a constant and so are not scanned:
//     parse-failed (migrated to string(FCParseFailed)), the ReasonStatus* tokens,
//     and the domain policy-reason tokens forwarded through fail.Reason.
//
// Scope note: the scan roots at the internal/app package directory, not all of
// internal/. The registry is a package-app value (FindingCode, with
// FindingCode(ReasonStatusInternalError) folded in), and internal/app already
// imports internal/reposetup — so reposetup cannot reference this registry
// without an import cycle. The registry governs package app's minting; a
// sibling package that also mints codes is out of this guard's reach by
// construction, not by omission.

// appPackageDir returns the absolute path of the internal/app package directory
// (this test file's own directory), so the tree walk is hermetic regardless of
// the process working directory.
func appPackageDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed to locate this test file")
	}
	return filepath.Dir(thisFile)
}

// TestNoInlineFindingCodeLiterals is the repo-wide minting guard: every
// production finding code is minted from the FindingCode registry in
// finding_codes.go. The scan keys on syntactic shape — a string literal in a
// Code: field position or as the first argument of a finding-constructor call —
// never on an enumerated spelling list. _test.go files are excluded (they
// assert codes; they do not mint them), and the exclusion is bounded to that
// suffix alone.
//
// The constructor pattern enumerates the constructor NAMES (a closed set this
// package owns): lifecycleFinding and refuseLifecycle take a FindingCode /
// message pair whose first argument was historically a literal. A NEW
// finding-constructor with a literal-first code argument MUST be added to this
// pattern. The Code: shape pattern is the backstop that catches a constructor
// this list misses, because a new constructor's body must still build a
// Finding/StatusFinding with a Code: field somewhere.
func TestNoInlineFindingCodeLiterals(t *testing.T) {
	codeLit := regexp.MustCompile(`Code:\s*"[^"]*"`)
	ctorLit := regexp.MustCompile(`(?:lifecycleFinding|refuseLifecycle|attachRefusal|haltRefusal|implementedRefusal|reclaimSkip|repairRefusal|closeoutRefusal|mergeRefusal|maintenanceRefusal|prRefusal|backlinkRefusal)\(\s*"`)
	root := appPackageDir(t)
	var violations []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		if filepath.Base(path) == "finding_codes.go" {
			return nil // the registry is the one sanctioned literal site
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range append(codeLit.FindAllString(string(b), -1), ctorLit.FindAllString(string(b), -1)...) {
			violations = append(violations, filepath.Base(path)+": "+m)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("finding codes minted outside the registry — mint from a FindingCode constant in finding_codes.go:\n%s",
			strings.Join(violations, "\n"))
	}
}

// TestFindingCodeRegistryIntegrity holds the AllFindingCodes vocabulary to its
// invariants: non-empty, strictly ascending by token value (so a hand-added
// entry lands in order), free of duplicates, and every token in the canonical
// finding-code shape ^[a-z][a-z0-9_-]*$ (the underscore admits keys such as
// invalid-change_id and empty-why_deferred). It does NOT assert that every
// declared FindingCode constant appears in the list — that constant↔list
// correspondence is Task 6's AST completeness guard (change 0399).
func TestFindingCodeRegistryIntegrity(t *testing.T) {
	if len(AllFindingCodes) == 0 {
		t.Fatal("AllFindingCodes is empty")
	}
	shape := regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	for i, c := range AllFindingCodes {
		if !shape.MatchString(string(c)) {
			t.Errorf("AllFindingCodes[%d] = %q does not match the finding-code shape %s", i, c, shape.String())
		}
		if i == 0 {
			continue
		}
		prev := AllFindingCodes[i-1]
		if string(c) == string(prev) {
			t.Errorf("AllFindingCodes has a duplicate entry %q at index %d", c, i)
		}
		if string(c) < string(prev) {
			t.Errorf("AllFindingCodes is not sorted: %q (index %d) precedes %q (index %d)", prev, i-1, c, i)
		}
	}
}
