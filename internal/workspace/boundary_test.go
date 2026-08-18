package workspace

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// This file is the workspace package's import-boundary pin plus the no-forbidden-
// git-verbs shape guard owed here by Task 2.
//
// Dependency direction (spec §"Package and dependency boundaries"): the workspace
// package sits above gitcli and beside domain. It may import ONLY the standard
// library, internal/domain (EffectiveBase/ChangeID/slug value semantics), and
// internal/gitcli (the sole process starter). It must never reach config,
// document, a repository snapshot, GitHub state, or the application layers — and
// on the inward side, only the application layer (internal/app and internal/cli)
// may reach INTO it: 0315's composition wires workspace there and maps its closed
// outcomes. Every OTHER landed package must stay clear of it. Both directions are
// pinned mechanically here so the boundary is a checked fact, not a convention.
//
// The scans cover production (non-_test) files only: `_test.go` files legitimately
// reach for fixtures and helpers, so pinning them would make the guard unusable.

const modulePrefix = "github.com/danielhanold/docket/"

// workspaceAllowedInModule is the exact set of in-module packages a workspace
// production file may import. Derived from the spec's dependency direction; any
// in-module import outside it fails.
var workspaceAllowedInModule = []string{
	"internal/domain",
	"internal/gitcli",
}

// moduleRoot walks up from the package directory to the directory holding go.mod.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs cwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod found walking up from package directory")
		}
		dir = parent
	}
}

// productionGoFiles returns the non-test .go files directly in dir, sorted.
func productionGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package dir %s: %v", dir, err)
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	slices.Sort(files)
	return files
}

// fileImports parses path in ImportsOnly mode and returns its import paths.
func fileImports(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse imports of %s: %v", path, err)
	}
	var paths []string
	for _, spec := range f.Imports {
		unquoted, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("unquote import path %s in %s: %v", spec.Path.Value, path, err)
		}
		paths = append(paths, unquoted)
	}
	return paths
}

// allProductionGoFilesUnder walks root recursively and returns every non-test
// .go file, so the reverse-direction scan derives its file set from the tree
// rather than a hand list.
func allProductionGoFilesUnder(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	slices.Sort(files)
	return files
}

// TestWorkspaceImportsOnlyDomainAndGitcli pins the OUTWARD boundary: every
// in-module import a workspace production file declares must be internal/domain or
// internal/gitcli. Adding an import of config/document/repository/etc. reddens.
func TestWorkspaceImportsOnlyDomainAndGitcli(t *testing.T) {
	files := productionGoFiles(t, ".")
	if len(files) == 0 {
		t.Fatal("no workspace production .go files found — the boundary guard would pass vacuously")
	}
	for _, file := range files {
		for _, imp := range fileImports(t, file) {
			if !strings.HasPrefix(imp, modulePrefix) {
				continue // stdlib or external — the forward rule pins in-module imports
			}
			rel := strings.TrimPrefix(imp, modulePrefix)
			if !slices.Contains(workspaceAllowedInModule, rel) {
				t.Errorf("%s imports %q: workspace may import only %v in-module",
					file, imp, workspaceAllowedInModule)
			}
		}
	}
}

// TestNoLandedPackageImportsWorkspace pins the INWARD boundary: no production file
// anywhere under internal/ (outside the workspace package itself, and outside the
// application layer) may import internal/workspace. The application layer —
// internal/app and internal/cli — is the ONLY allowed importer (0315 wires
// workspace there); every other package must stay clear. The importer set is
// DERIVED by walking internal/ — never enumerated — so a future non-application
// package that reaches up into workspace reddens.
func TestNoLandedPackageImportsWorkspace(t *testing.T) {
	root := moduleRoot(t)
	selfDir := filepath.Join(root, "internal", "workspace")
	// The application layer is allowed to import workspace. A file is exempt only
	// when it lives directly in one of these package directories (not merely under
	// them), so a hypothetical internal/app/foo subpackage is still guarded.
	allowedImporterDirs := []string{
		filepath.Join(root, "internal", "app"),
		filepath.Join(root, "internal", "cli"),
	}
	target := modulePrefix + "internal/workspace"
	files := allProductionGoFilesUnder(t, filepath.Join(root, "internal"))
	if len(files) == 0 {
		t.Fatal("no internal/ production files found — the reverse guard would pass vacuously")
	}
	scanned := 0
	for _, file := range files {
		dir := filepath.Dir(file)
		if dir == selfDir || slices.Contains(allowedImporterDirs, dir) {
			continue // the package may refer to itself; the application layer may import it
		}
		scanned++
		for _, imp := range fileImports(t, file) {
			if imp == target {
				t.Errorf("%s imports %q: only the application layer (internal/app, internal/cli) may depend on internal/workspace", file, imp)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("reverse guard scanned no files outside the workspace package — would pass vacuously")
	}
}

// funcStringLiterals parses the Go file at path and returns, per top-level
// function declaration, every string literal appearing anywhere in its body. It
// is the mechanism behind the argv shape guard: the forbidden/required flags are
// read out of the exact functions that build git's argv.
func funcStringLiterals(t *testing.T, path string) map[string][]string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	out := map[string][]string{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		var lits []string
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if bl, ok := n.(*ast.BasicLit); ok && bl.Kind == token.STRING {
				if s, err := strconv.Unquote(bl.Value); err == nil {
					lits = append(lits, s)
				}
			}
			return true
		})
		out[fn.Name.Name] = lits
	}
	return out
}

// TestGitcliWorktreeVerbsAreNonForcing is the no-forbidden-git-verbs guard owed to
// Task 12 by Task 2. The three non-destructive/attach operations must never build
// a `worktree add -B` (branch reset) or a `worktree remove --force`; the
// deliberately forced RemoveWorktree keeps its documented --force. Anchored on the
// argv literals inside the NAMED functions, so a mutation adding -B/--force to a
// non-forcing verb — or dropping --force from the forced one — reddens.
func TestGitcliWorktreeVerbsAreNonForcing(t *testing.T) {
	worktreeGo := filepath.Join(moduleRoot(t), "internal", "gitcli", "worktree.go")
	lits := funcStringLiterals(t, worktreeGo)

	nonForcing := []string{"AddBranchWorktree", "AttachBranchWorktree", "RemoveWorktreeClean"}
	for _, name := range nonForcing {
		argv, ok := lits[name]
		if !ok {
			t.Fatalf("function %s not found in %s — the shape guard would pass vacuously", name, worktreeGo)
		}
		if slices.Contains(argv, "-B") {
			t.Errorf("%s builds a `-B` (branch reset) argv: %v — non-forcing worktree verbs must never reset a branch", name, argv)
		}
		if slices.Contains(argv, "--force") {
			t.Errorf("%s builds a `--force` argv: %v — this operation must let git recheck at the destructive boundary", name, argv)
		}
	}

	// The intentionally forced removal is the counter-case: it MUST keep --force,
	// so this guard cannot be satisfied by simply neutering every worktree verb.
	forced, ok := lits["RemoveWorktree"]
	if !ok {
		t.Fatalf("function RemoveWorktree not found in %s — the counter-case guard would pass vacuously", worktreeGo)
	}
	if !slices.Contains(forced, "--force") {
		t.Errorf("RemoveWorktree no longer builds `--force`: %v — the documented forced removal must retain it", forced)
	}
}
