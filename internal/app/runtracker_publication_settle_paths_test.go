package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// These tests pin change 0444 acceptance 3 at the real entry points: settlement of a
// retry-proven uncertain publication happens through the attributed keyed verdict
// (RunGateVerdict → gateCompleteRun → completeSuccessfulRun) and cancellation
// teardown ONLY; the unattributed observe verdict and RunVerify over the very same
// settleable pair write nothing. TestSettleUncertainPublicationsAuthorizedCallers
// pins the write to its two authorized callers by deriving every reference from
// source. The two real-entry-point tests run real git, so they live behind the
// integration tag in runtracker_publication_settle_paths_integration_test.go (change
// 0465); only the source-derived shape guard stays in the default corpus.

// TestSettleUncertainPublicationsAuthorizedCallers is change 0444's shape guard: the
// settlement WRITE may be reached only from cancellation teardown
// (reconcileEpochTeardown) and the attributed successful closeout
// (completeSuccessfulRun). It derives every reference to the identifier
// settleUncertainPublications from the package's production source via the AST —
// any use (a call, or the function taken as a value) counts, keyed on the
// identifier, not a spelling list — and resolves the enclosing top-level
// declaration. A reference anywhere else, or either authorized caller losing its
// reference, reddens.
//
// Mutation probes: (a) add a call inside RunVerify -> "unauthorized referrer";
// (b) delete the call in completeSuccessfulRun -> the referrer set shrinks.
func TestSettleUncertainPublicationsAuthorizedCallers(t *testing.T) {
	const target = "settleUncertainPublications"
	want := []string{"completeSuccessfulRun", "reconcileEpochTeardown"}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	referrers := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			owner := "<package-level declaration in " + name + ">"
			var skip *ast.Ident
			if fd, ok := decl.(*ast.FuncDecl); ok {
				owner = fd.Name.Name
				if fd.Recv == nil && fd.Name.Name == target {
					skip = fd.Name // the definition itself is not a reference
				}
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok || id == skip || id.Name != target {
					return true
				}
				referrers[owner] = true
				return true
			})
		}
	}
	got := make([]string, 0, len(referrers))
	for r := range referrers {
		got = append(got, r)
	}
	sort.Strings(got)
	allowed := map[string]bool{}
	for _, w := range want {
		allowed[w] = true
	}
	for _, r := range got {
		if !allowed[r] {
			t.Errorf("unauthorized referrer of %s: %s — settlement is a write reachable only from %v",
				target, r, want)
		}
	}
	for _, w := range want {
		if !referrers[w] {
			t.Errorf("authorized caller %s no longer references %s (got %v)", w, target, got)
		}
	}
}
