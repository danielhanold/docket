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
//   Request-shape validator mints (change 0399, review): the change-create,
//   adr.record, learning.record, change-groom, change-reconcile, and finalize
//   block/clear-block validators mint through addShape closures / adrFinding /
//   learningFinding, each now fed a registry FindingCode constant (never a
//   literal). ctorLit lists those constructor names, so a literal-first argument
//   reddens the guard; their concrete codes — including the enumerated
//   empty-<field> expansions — are registered FindingCode constants folded into
//   AllFindingCodes.
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
// The constructor pattern enumerates the finding-constructor NAMES (a closed set
// this package owns): lifecycleFinding, refuseLifecycle, the *Refusal helpers,
// and the request-shape validators' addShape closures / adrFinding /
// learningFinding all take a FindingCode / message pair whose first argument must
// be a registry constant, never a literal. A NEW finding-constructor with a
// literal-first code argument MUST be added to this pattern. Two shape backstops
// catch a constructor this list misses: the Code: pattern (a new constructor's
// body must still build a Finding/StatusFinding with a Code: field somewhere) and
// the FindingCode("…") pattern (converting a string literal to the registry's own
// type outside the registry is itself a rogue mint). Together they redden a
// future addShape("rogue-code", …) or StatusFinding{Code: "rogue-code"} planted
// in any non-registry, non-test file.
func TestNoInlineFindingCodeLiterals(t *testing.T) {
	codeLit := regexp.MustCompile(`Code:\s*"[^"]*"`)
	convLit := regexp.MustCompile(`FindingCode\(\s*"`)
	ctorLit := regexp.MustCompile(`(?:lifecycleFinding|refuseLifecycle|attachRefusal|haltRefusal|implementedRefusal|reclaimSkip|repairRefusal|closeoutRefusal|mergeRefusal|maintenanceRefusal|prRefusal|backlinkRefusal|addShape|adrFinding|learningFinding)\(\s*"`)
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
		matches := append(codeLit.FindAllString(string(b), -1), ctorLit.FindAllString(string(b), -1)...)
		matches = append(matches, convLit.FindAllString(string(b), -1)...)
		for _, m := range matches {
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

// TestInvalidIDCodeByKeyCoversCallSiteKeys proves the id-shape lookup is closed
// over the idKeys the shape validators actually pass. It scans the app package
// source for validateLifecycleShape("<key>", …) calls, extracts the first string
// literal from each, and asserts every one is a registered key of
// invalidIDCodeByKey — so a future call site adding a new key without a map entry
// reddens here instead of silently emitting the FCInvalidID fallback. It also
// pins the fail-closed helper: known keys resolve to their concrete codes and an
// absent key falls back to FCInvalidID (never Code:"").
func TestInvalidIDCodeByKeyCoversCallSiteKeys(t *testing.T) {
	call := regexp.MustCompile(`validateLifecycleShape\(\s*"([^"]*)"`)
	root := appPackageDir(t)
	keys := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range call.FindAllStringSubmatch(string(b), -1) {
			keys[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) == 0 {
		t.Fatal("found no validateLifecycleShape call sites to scan")
	}
	for k := range keys {
		if _, ok := invalidIDCodeByKey[k]; !ok {
			t.Errorf("validateLifecycleShape call site passes idKey %q with no invalidIDCodeByKey entry — the id-code lookup is not closed over it", k)
		}
	}

	// Fail-closed helper: known keys resolve to concrete codes; an unknown key
	// falls back to FCInvalidID rather than emitting an empty Code.
	if got := invalidIDCode("id"); got != FCInvalidID {
		t.Errorf(`invalidIDCode("id") = %q, want %q`, got, FCInvalidID)
	}
	if got := invalidIDCode("change_id"); got != FCInvalidChangeID {
		t.Errorf(`invalidIDCode("change_id") = %q, want %q`, got, FCInvalidChangeID)
	}
	if got := invalidIDCode("no_such_key"); got != FCInvalidID {
		t.Errorf(`invalidIDCode(unknown) = %q, want fail-closed fallback %q`, got, FCInvalidID)
	}
	if invalidIDCode("no_such_key") == "" {
		t.Error("invalidIDCode(unknown) returned an empty FindingCode — the lookup is not fail-closed")
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

// TestShapeValidatorCodesAreRegistered drives the request-shape validators with
// empty/invalid requests and asserts every finding code they emit is a member of
// AllFindingCodes — the closed finding_codes vocabulary the schema publishes.
// This exercises the real addShape/adrFinding/learningFinding mint paths (change
// 0399, review): a code minted but never registered would surface here, not only
// in the syntactic guard. It also asserts a floor set of the newly-registered
// concrete codes is actually reached, so a regression that stops emitting one is
// visible.
func TestShapeValidatorCodesAreRegistered(t *testing.T) {
	registered := map[string]bool{}
	for _, c := range AllFindingCodes {
		registered[string(c)] = true
	}

	zero := 0
	badChange := ADRProducingChange{ID: 0, Path: "", Version: ""}
	adrContent := ADRRecordRequest{Change: &badChange} // authored fields empty + a bad producing change

	var emitted []StatusFinding
	emitted = append(emitted, validateChangeCreateShape(ChangeCreateRequest{StackedOn: &zero})...)
	emitted = append(emitted, validateADRRecordShape(adrContent)...)
	emitted = append(emitted, validateADRReplaceShape(ADRReplaceRequest{Target: ADRTarget{ID: 0}, Successor: adrContent})...)
	emitted = append(emitted, validateLearningRecordShape(LearningRecordRequest{Topics: []string{""}})...)
	emitted = append(emitted, validateChangeGroomShape(ChangeGroomRequest{Outcome: GroomSpec})...)
	emitted = append(emitted, validateChangeGroomShape(ChangeGroomRequest{Outcome: GroomTrivial})...)
	emitted = append(emitted, validateChangeGroomShape(ChangeGroomRequest{Outcome: GroomOutcome("bogus")})...)
	emitted = append(emitted, validateChangeReconcileShape(ChangeReconcileRequest{
		Sections:     map[string]string{"## Not Owned": "x"},
		SpecSections: map[string]string{"not a heading": "x"},
	})...)
	emitted = append(emitted, validateBlockShape(BlockRequest{})...)
	emitted = append(emitted, validateClearBlockShape(ClearBlockRequest{})...)

	seen := map[string]bool{}
	for _, f := range emitted {
		seen[f.Code] = true
		if !registered[f.Code] {
			t.Errorf("shape validator emitted finding code %q that is absent from AllFindingCodes — the finding_codes vocabulary is not closed over it", f.Code)
		}
	}

	// Floor set: concrete codes registered by this fix that these requests must
	// reach. A miss means the mint path drifted from the registered constant.
	floor := []FindingCode{
		FCInvalidRequestID, FCInvalidStackedOn,
		FCEmptyTitle, FCEmptyWhy, FCEmptyWhatChanges, FCEmptyOutOfScope,
		FCEmptyContext, FCEmptyDecision, FCEmptyConsequences, FCEmptyAlternatives,
		FCInvalidChangeDotID, FCEmptyChangePath, FCEmptyChangeVersion,
		FCInvalidTargetID, FCEmptyTargetPath, FCEmptyTargetVersion,
		FCEmptyHook, FCEmptyApply, FCEmptyWarStory, FCInvalidTopics,
		FCEmptySpecMarkdown, FCMissingRationale, FCInvalidOutcome,
		FCInvalidSpecSectionHeading, FCEmptyReconcileLogEntry,
		FCInvalidPRNumber, FCInvalidAttempt, FCEmptyHead,
	}
	for _, c := range floor {
		if !seen[string(c)] {
			t.Errorf("expected shape validators to emit registered code %q, but none did", string(c))
		}
	}
}
