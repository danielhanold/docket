package githubcli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestEveryNetworkSiteIsReadWriteClassified is the one-way correspondence guard
// for Task 11 Step 1, the githubcli half of the network-site classification. Every
// runRequest in this package's source that sets `network: true` MUST be accounted
// for below with an explicit read/write disposition (a remote MUTATION — a gh
// merge/create/edit/comment — sets `write: true` and draws the write budget; every
// list/view/probe/verify read draws the read budget). The guard runs ONE WAY —
// source INTO the classification: a NEW network op whose enclosing function is not
// listed, or one whose source disposition disagrees, reddens; a stale entry is
// tolerated. It complements the prose enumeration in client_test.go by making the
// correspondence mechanical rather than a comment that can drift silently.
//
// Keyed on AST SHAPE (the runRequest literal's network/write fields), not on a gh
// argv spelling.
func TestEveryNetworkSiteIsReadWriteClassified(t *testing.T) {
	// enclosing function -> isWrite. false = read budget, true = write budget.
	classified := map[string]bool{
		"probeByHead":                false, // pr list --all
		"verifyListOpen":             false, // pr list --state open
		"verifyViewByNumber":         false, // pr view <n>
		"createRequest":              true,  // pr create
		"editRequest":                true,  // pr edit
		"FindComment":                false, // pr view --comments
		"EnsureComment":              true,  // pr comment
		"probeRepoMergeMethods":      false, // repo/api read
		"probeBranchMergeRules":      false, // api read
		"MergePullRequest":           true,  // pr merge
		"probeMergeSnapshot":         false, // pr view (verify/reprobe)
		"ViewPullRequestsBatch":      false, // api graphql (batched read)
		"ViewPullRequest":            false, // pr view
		"FindOpenPullRequestsByHead": false, // pr list --head
		"RetargetPullRequest":        true,  // pr edit --base
		"DiscoverRepository":         false, // repo view
	}
	assertNetworkSitesClassified(t, classified)
}

// assertNetworkSitesClassified walks this package's non-test source, finds every
// runRequest{…, network: true, …} literal and its enclosing function, and asserts
// each is present in classified with a matching read/write disposition. It fails
// if it finds no sites at all, so a broken walk cannot pass vacuously.
func assertNetworkSitesClassified(t *testing.T, classified map[string]bool) {
	t.Helper()
	found := networkSitesInPackageSource(t)
	if len(found) == 0 {
		t.Fatal("found no network:true runRequest sites — the AST walk is broken; the guard would pass vacuously")
	}
	for fn, isWrite := range found {
		want, ok := classified[fn]
		if !ok {
			t.Errorf("network:true site in %q() is UNCLASSIFIED — decide its read vs write budget and add it to the classification (a write left as a read draws the shorter read budget)", fn)
			continue
		}
		if want != isWrite {
			t.Errorf("network site %q(): source write=%v but classification says write=%v", fn, isWrite, want)
		}
	}
}

// networkSitesInPackageSource returns enclosing-function-name -> isWrite for every
// runRequest composite literal in the current package's non-test .go files that
// sets network: true. Each such function carries exactly one network site by
// convention.
func networkSitesInPackageSource(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package source: %v", err)
	}
	found := map[string]bool{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					lit, ok := n.(*ast.CompositeLit)
					if !ok {
						return true
					}
					id, ok := lit.Type.(*ast.Ident)
					if !ok || id.Name != "runRequest" {
						return true
					}
					if litBool(lit, "network") {
						found[fn.Name.Name] = litBool(lit, "write")
					}
					return true
				})
			}
		}
	}
	return found
}

// litBool reports whether the composite literal sets field to the identifier
// true. A field absent, or set to anything but the bare `true` identifier, reads
// as false — matching Go's zero value for the runRequest bool fields.
func litBool(lit *ast.CompositeLit, field string) bool {
	for _, el := range lit.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != field {
			continue
		}
		val, ok := kv.Value.(*ast.Ident)
		return ok && val.Name == "true"
	}
	return false
}
