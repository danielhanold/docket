package repoguard

// TestNoIntegrationPushOutsidePRMerge pins that no operation writes the
// integration branch outside a PR merge. Two syntactic shapes are checked over
// every non-test Go file under internal/app:
//
//  1. Every transaction.Request composite literal's TargetRef expression must
//     mention MetadataBranchName — the engine may only commit to the metadata
//     branch.
//  2. Every direct PushLease / PushCreateLease call must sit in a function on
//     pushAllowlist. Each entry names why it may push; only migrateExecute
//     touches the integration branch, for the one-time human-run legacy
//     migration prune. A new direct pusher reddens this test and must argue its
//     way onto the list.
//
// Mutation-tested: re-adding a TargetRef of branchRefPrefix + integrationBranch
// reddens shape 1; adding a PushLease call to a new function reddens shape 2.
//
// RESIDUAL RISK, recorded not hidden: shape 1 reads composite literals only, so
// a Request built field by field (req.TargetRef = ...) escapes it; no such site
// exists, and the population floor keeps the literal detector honest. Shape 2
// keys on the selector name, not the receiver's resolved type, which errs safe.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// pushAllowlist names every function under internal/app that may call
// PushLease or PushCreateLease directly, and why.
var pushAllowlist = map[string]string{
	"publishOrAdoptMetadataRoot": "repository init seeds the metadata branch",
	"executeRepositoryRepair":    "repository repair publishes one metadata commit",
	"reconcileResumeSeed":        "repository migrate resumes the metadata seed",
	"publishSeed":                "repository migrate publishes the metadata seed",
	"migrateExecute":             "repository migrate's one-time, human-run legacy prune of the integration branch",
}

// integrationPushReport is what the walk over internal/app found.
type integrationPushReport struct {
	requestLiterals int
	pushers         map[string]bool
	violations      []string
}

// analyzeIntegrationPush walks the parsed files for both shapes.
func analyzeIntegrationPush(fset *token.FileSet, files map[string]*ast.File) integrationPushReport {
	rep := integrationPushReport{pushers: map[string]bool{}}
	rels := make([]string, 0, len(files))
	for rel := range files {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		file := files[rel]
		// Shape 1: transaction.Request literals target the metadata branch.
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || !isTransactionRequest(lit.Type) {
				return true
			}
			rep.requestLiterals++
			var target ast.Expr
			for _, el := range lit.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "TargetRef" {
					target = kv.Value
				}
			}
			pos := fset.Position(lit.Pos())
			if target == nil {
				rep.violations = append(rep.violations, rel+": a transaction.Request literal at line "+strconv.Itoa(pos.Line)+" sets no TargetRef, so its target cannot be proven to be the metadata branch")
				return true
			}
			var buf bytes.Buffer
			if err := printer.Fprint(&buf, fset, target); err != nil || !strings.Contains(buf.String(), "MetadataBranchName") {
				rep.violations = append(rep.violations, rel+": a transaction.Request targets "+buf.String()+", not the metadata branch (MetadataBranchName)")
			}
			return true
		})
		// Shape 2: direct pushes sit only in allowlisted functions.
		for _, decl := range file.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			ast.Inspect(decl, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (sel.Sel.Name != "PushLease" && sel.Sel.Name != "PushCreateLease") {
					return true
				}
				if !isFunc {
					rep.violations = append(rep.violations, rel+": a package-level "+sel.Sel.Name+" call sits outside any function")
					return true
				}
				name := fn.Name.Name
				rep.pushers[name] = true
				if _, ok := pushAllowlist[name]; !ok {
					rep.violations = append(rep.violations, rel+": "+name+" calls "+sel.Sel.Name+" directly but is not on pushAllowlist")
				}
				return true
			})
		}
	}
	return rep
}

// isTransactionRequest reports whether a composite literal's type is the
// selector transaction.Request.
func isTransactionRequest(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Request" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "transaction"
}

func TestNoIntegrationPushOutsidePRMerge(t *testing.T) {
	root := guardRoot(t)
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, rel := range maintainedPop(t, root) {
		if !hasExt(rel, ".go") || strings.HasSuffix(rel, "_test.go") || dirOf(rel) != "internal/app" {
			continue
		}
		file, err := parser.ParseFile(fset, rel, readMaintained(t, root, rel), 0)
		if err != nil {
			// Fail closed: an unparseable file is a guard failure, never a clean miss.
			t.Fatalf("parse %s: %v", rel, err)
		}
		files[rel] = file
	}

	rep := analyzeIntegrationPush(fset, files)

	// Population floors first: an empty enumeration passes every negative.
	if rep.requestLiterals < 15 {
		t.Fatalf("population floor: found %d transaction.Request literals under internal/app (want >= 15); the literal detector drifted", rep.requestLiterals)
	}
	for name := range pushAllowlist {
		if !rep.pushers[name] {
			t.Errorf("pushAllowlist entry %q calls no PushLease/PushCreateLease; the entry is stale — delete it", name)
		}
	}
	for _, v := range rep.violations {
		t.Error(v)
	}
}

// TestNoIntegrationPushOutsidePRMergeDetects is the guard's own negative
// control: a planted integration-branch TargetRef and an unlisted pusher each
// produce a violation, and the metadata-branch shape produces none.
func TestNoIntegrationPushOutsidePRMergeDetects(t *testing.T) {
	src := `package app

func good() {
	_ = transaction.Request{TargetRef: gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName)}
}

func bad() {
	_ = transaction.Request{TargetRef: gitcli.RefName(branchRefPrefix + cc.integrationBranch)}
}

func sneakyPush() {
	git.PushLease(ctx, repo, remote, ref, commit, old)
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "internal/app/x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	rep := analyzeIntegrationPush(fset, map[string]*ast.File{"internal/app/x.go": f})
	if rep.requestLiterals != 2 {
		t.Errorf("requestLiterals = %d, want 2", rep.requestLiterals)
	}
	joined := strings.Join(rep.violations, "\n")
	if len(rep.violations) != 2 || !strings.Contains(joined, "cc.integrationBranch") || !strings.Contains(joined, "sneakyPush") {
		t.Errorf("violations = %v, want exactly the integration TargetRef and sneakyPush", rep.violations)
	}
}
