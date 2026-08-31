package gitcli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestEveryNetworkSiteIsReadWriteClassified is the one-way correspondence guard
// for Task 11 Step 1: every runRequest in this package's source that sets
// `network: true` MUST be accounted for below with an explicit read/write
// disposition (a remote MUTATION sets `write: true` and draws the write budget;
// every other network op is a read and draws the read budget). The guard runs ONE
// WAY — source INTO the classification: a NEW network op whose enclosing function
// is not listed here, or one whose source disposition disagrees with the list,
// reddens. That forces a deliberate read-vs-write decision, because a network
// write accidentally left as a read would silently draw the shorter read budget.
// A stale entry (a function later removed from source) is tolerated, never a false
// failure — the reverse direction is not asserted.
//
// The guard keys on AST SHAPE (a runRequest composite literal's network/write
// fields), never on an argv spelling or a grep of a prose comment, so the house
// idiom the next author reaches for cannot slip past it.
func TestEveryNetworkSiteIsReadWriteClassified(t *testing.T) {
	// enclosing function -> isWrite, derived from this package's `network: true`
	// sites. false = read budget, true = write budget.
	classified := map[string]bool{
		"RemoteDefaultBranch":  false, // ls-remote --symref … HEAD
		"FetchBranch":          false, // fetch
		"classifyFetchFailure": false, // ls-remote failure-classification probe
		"ProbeRemoteBranch":    false, // ls-remote <ref>
		"ListRemoteHeads":      false, // ls-remote --heads
		"PushLease":            true,  // push --force-with-lease
		"PushCreateLease":      true,  // push (create)
		"DeleteRemoteRefLease": true,  // push --delete (lease)
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
// convention; a second with a different disposition would overwrite, which the
// classification's disposition check would then surface.
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
					network, write := litBool(lit, "network"), litBool(lit, "write")
					if network {
						found[fn.Name.Name] = write
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
