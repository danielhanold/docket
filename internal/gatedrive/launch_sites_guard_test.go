package gatedrive

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// Change 0437 Task 8, narrowed by change 0490: a syntactic, computed guard that
// binds every process launch in this package to an admission boundary. Spec AC7:
// "Derive any launch-site guard from syntactic executable call sites, not a
// hand-maintained spelling list."
//
// The package has two launch sites, admitted differently:
//
//   - The first launch (launchScopeless, reached through StartAdmitted) must
//     still cross the RUN launch gate — kept for change 0491, which retires it.
//   - The single automatic relaunch (driveSlice) crosses no run gate: no
//     production drive both carries a run and can relaunch (only finalize's
//     run-less local gate is idempotent — change 0490 spec, Problem fact 6), so
//     its admission is the WORKTREE LOCK it re-takes before launching.
//
// The guard is derived entirely from the AST of the package's non-test source:
//
//   1. Population — every call expression of the shape <recv>.<procField>.Launch(…)
//      where <procField> is the name of the Driver's ProcessSeam field, itself
//      read from the struct declaration (never a regex or a written-down spelling
//      of "proc" — a byte-pattern guard matches a spelling, learning
//      byte-pattern-guard-matches-a-spelling).
//   2. Boundary set — the functions whose body calls the single run-authorization
//      helper runLaunchGated (the ONE helper every run-backed reservation/launch
//      authorization flows through, by design of change 0437 Task 1). Computed
//      from the AST, not enumerated.
//   3. Lock guard — a function whose body calls the worktree-lock helper
//      lockWorktree at a source position BEFORE each of its launch sites is
//      guarded by itself: the launch it makes is admitted by the lock it took.
//   4. Reachability — the package-internal receiver call graph. Every function
//      that contains a launch site must be lock-guarded, or adjacent to the run
//      boundary: it must either call a boundary-set member itself, or be invoked
//      ONLY by functions that call a boundary-set member. A launch reachable
//      through a caller that does not pass through a boundary — or a launch with
//      no caller at all — is a violation. Adjacency (rather than mere transitive
//      reach-a-boundary) is what makes the guard falsifiable: StartAdmitted
//      crosses the run boundary before its launch, so a purely transitive model
//      would keep a separately-admitted launch green after its own guard was
//      stripped.
//
// The population is COMPUTED and pinned at the package's two launch sites: a
// guard that finds zero launch sites is broken, not green (learning
// marker-scoped-guard-needs-a-population-floor), and a third site must be
// admitted deliberately. The guard is mutation-tested by
// TestLaunchSiteGuardIsFalsifiable.

// launchGuardResult is the computed accounting one scan produces.
type launchGuardResult struct {
	procField        string   // ProcessSeam field name, derived from the struct decl (AST)
	launchSites      int      // total <recv>.<procField>.Launch(…) call expressions
	boundaryFuncs    []string // functions whose body calls runLaunchGated (sorted)
	lockGuardedFuncs []string // functions that call lockWorktree before every launch they make (sorted)
	launchFuncs      []string // functions whose body contains a launch site (sorted)
	violations       []string
	visitedFiles     int
}

// funcInfo is the per-function-name AST-derived facts the call graph needs.
type funcInfo struct {
	callsRunLaunchGated bool
	launchCount         int
	launchPos           []token.Pos     // source position of each launch site
	lockPos             []token.Pos     // source position of each lockWorktree call
	callees             map[string]bool // receiver-method calls d.<name>(…)
}

// lockGuarded reports whether every launch site in the function is preceded,
// in source order within the same function, by a lockWorktree call.
func (fi *funcInfo) lockGuarded() bool {
	if fi == nil || len(fi.launchPos) == 0 || len(fi.lockPos) == 0 {
		return false
	}
	first := fi.lockPos[0]
	for _, p := range fi.lockPos {
		if p < first {
			first = p
		}
	}
	for _, p := range fi.launchPos {
		if p < first {
			return false
		}
	}
	return true
}

// analyzeGatedriveLaunchSites parses every non-test .go file under root and
// computes the launch-site guard accounting. Facts are merged by function name
// (conservative: a name is a boundary member if ANY declaration of that name
// calls runLaunchGated; a launch func if ANY launches; callees are unioned) so a
// name collision never silently drops a call edge.
func analyzeGatedriveLaunchSites(root string) (launchGuardResult, error) {
	var res launchGuardResult
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, root, func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		return res, err
	}

	// 1. Derive the ProcessSeam field name from the struct declaration.
	procField := ""
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				st, ok := n.(*ast.StructType)
				if !ok || st.Fields == nil {
					return true
				}
				for _, f := range st.Fields.List {
					id, ok := f.Type.(*ast.Ident)
					if !ok || id.Name != "ProcessSeam" || len(f.Names) == 0 {
						continue
					}
					procField = f.Names[0].Name
				}
				return true
			})
		}
	}
	res.procField = procField

	// 2. Per-function facts.
	infos := map[string]*funcInfo{}
	get := func(name string) *funcInfo {
		fi := infos[name]
		if fi == nil {
			fi = &funcInfo{callees: map[string]bool{}}
			infos[name] = fi
		}
		return fi
	}

	for _, pkg := range pkgs {
		for name := range pkg.Files {
			res.visitedFiles++
			_ = name
		}
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				fi := get(fn.Name.Name)
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					// A launch: <recv>.<procField>.Launch(…).
					if procField != "" && sel.Sel.Name == "Launch" {
						if inner, ok := sel.X.(*ast.SelectorExpr); ok && inner.Sel.Name == procField {
							fi.launchCount++
							fi.launchPos = append(fi.launchPos, call.Pos())
							res.launchSites++
							return true
						}
					}
					// A receiver-method call: d.<name>(…) where the receiver is a bare
					// identifier. Records the runLaunchGated boundary call, the
					// lockWorktree guard call and its position, and every intra-package
					// call edge.
					if _, ok := sel.X.(*ast.Ident); ok {
						switch sel.Sel.Name {
						case "runLaunchGated":
							fi.callsRunLaunchGated = true
						case "lockWorktree":
							fi.lockPos = append(fi.lockPos, call.Pos())
						}
						fi.callees[sel.Sel.Name] = true
					}
					return true
				})
			}
		}
	}

	// 3. Boundary set and reachability.
	boundary := map[string]bool{}
	for name, fi := range infos {
		if fi.callsRunLaunchGated {
			boundary[name] = true
			res.boundaryFuncs = append(res.boundaryFuncs, name)
		}
	}
	sort.Strings(res.boundaryFuncs)

	directlyGuarded := func(name string) bool {
		if boundary[name] {
			return true
		}
		fi := infos[name]
		if fi == nil {
			return false
		}
		for callee := range fi.callees {
			if boundary[callee] {
				return true
			}
		}
		return false
	}

	callersOf := func(target string) []string {
		var callers []string
		for name, fi := range infos {
			if fi.callees[target] {
				callers = append(callers, name)
			}
		}
		sort.Strings(callers)
		return callers
	}

	for name, fi := range infos {
		if fi.launchCount == 0 {
			continue
		}
		res.launchFuncs = append(res.launchFuncs, name)
		if fi.lockGuarded() {
			res.lockGuardedFuncs = append(res.lockGuardedFuncs, name)
		}
	}
	sort.Strings(res.launchFuncs)
	sort.Strings(res.lockGuardedFuncs)

	for _, name := range res.launchFuncs {
		if directlyGuarded(name) || infos[name].lockGuarded() {
			continue
		}
		callers := callersOf(name)
		if len(callers) == 0 {
			res.violations = append(res.violations,
				name+" launches but takes no worktree lock before it, crosses no run boundary, and has no caller to cross one")
			continue
		}
		for _, c := range callers {
			if !directlyGuarded(c) {
				res.violations = append(res.violations,
					"launch in "+name+" is reachable via caller "+c+" which does not cross the run boundary")
			}
		}
	}

	return res, nil
}

// TestLaunchSitesBoundToRunLaunchGate is the run-level guard: every launch site in
// the finished package is bound, syntactically, to an admission boundary — the
// run-authorization boundary or a worktree lock taken before the launch — and
// the computed population is the package's two launch sites.
func TestLaunchSitesBoundToRunLaunchGate(t *testing.T) {
	res, err := analyzeGatedriveLaunchSites(".")
	if err != nil {
		t.Fatalf("analyze gatedrive package: %v", err)
	}
	if res.visitedFiles == 0 {
		t.Fatalf("launch-site guard visited zero files — an unfalsifiable walker is decoration")
	}
	if res.procField == "" {
		t.Fatalf("could not derive the ProcessSeam field name from the package — guard cannot bind launches")
	}
	if len(res.boundaryFuncs) == 0 {
		t.Fatalf("no run-boundary functions found (none call runLaunchGated) — guard is vacuous")
	}
	// Population (computed and reported): a guard that finds no launch sites is
	// broken, not green; the package has exactly the first launch and the single
	// relaunch, so a third site must be admitted deliberately.
	if res.launchSites != 2 {
		t.Fatalf("population: found %d %s.Launch call sites, want 2 (the first launch and the single relaunch)", res.launchSites, res.procField)
	}
	if len(res.lockGuardedFuncs) == 0 {
		t.Fatalf("no launch function takes the worktree lock before launching — the relaunch's guard is gone")
	}
	for _, v := range res.violations {
		t.Errorf("unguarded launch site: %s", v)
	}
	t.Logf("launch-site guard: %d launch call sites in %d files; procField=%q; boundary funcs=%v; lock-guarded funcs=%v; launch funcs=%v",
		res.launchSites, res.visitedFiles, res.procField, res.boundaryFuncs, res.lockGuardedFuncs, res.launchFuncs)
}

// TestLaunchSiteGuardIsFalsifiable mutation-tests the guard against synthetic
// source trees that reproduce the package's launch/boundary shape: (a) a bare
// unguarded launch helper is detected, (b) stripping runLaunchGated from the
// boundary the first launch depends on is detected, (c) stripping the
// lockWorktree call from the relaunch is detected, (d) a lockWorktree call that
// comes only AFTER the launch is detected, plus a clean control and the
// population/visited floors. A guard that cannot redden is decoration
// (AGENTS.md).
func TestLaunchSiteGuardIsFalsifiable(t *testing.T) {
	// A guarded baseline mirroring the real shape: driveSlice takes the worktree
	// lock and then launches (lock-guarded, reached through Advance ->
	// driveAndPersistClaim, which crosses no run boundary); launchScopeless
	// launches and is caller-guarded (its only caller StartAdmitted crosses the
	// run boundary revalidate).
	baseline := "" +
		"package p\n" +
		"type ProcessSeam interface{ Launch(x int) (int, error) }\n" +
		"type Driver struct{ proc ProcessSeam }\n" +
		"func (d *Driver) runLaunchGated(f func() error) error { return f() }\n" +
		"func (d *Driver) lockWorktree() error { return nil }\n" +
		"func (d *Driver) revalidate() error { return d.runLaunchGated(func() error { return nil }) }\n" +
		"func (d *Driver) driveSlice() { _ = d.lockWorktree(); d.proc.Launch(1) }\n" +
		"func (d *Driver) driveAndPersistClaim() { d.driveSlice() }\n" +
		"func (d *Driver) launchScopeless() { d.proc.Launch(2) }\n" +
		"func (d *Driver) StartAdmitted() { _ = d.revalidate(); d.launchScopeless() }\n" +
		"func (d *Driver) Advance() { d.driveAndPersistClaim() }\n"

	// Control: the baseline is clean and the population is computed.
	dirClean := testsupport.TempDir(t)
	writeGuardGoFile(t, dirClean, "p.go", baseline)
	if res, err := analyzeGatedriveLaunchSites(dirClean); err != nil {
		t.Fatalf("analyze baseline: %v", err)
	} else {
		if res.procField != "proc" {
			t.Errorf("baseline procField = %q, want %q", res.procField, "proc")
		}
		if res.launchSites != 2 {
			t.Errorf("baseline launchSites = %d, want 2", res.launchSites)
		}
		if len(res.lockGuardedFuncs) != 1 || res.lockGuardedFuncs[0] != "driveSlice" {
			t.Errorf("baseline lock-guarded funcs = %v, want [driveSlice]", res.lockGuardedFuncs)
		}
		if len(res.violations) != 0 {
			t.Errorf("baseline flagged violations: %v", res.violations)
		}
	}

	mutate := func(t *testing.T, name, from, to string) {
		t.Helper()
		mut := strings.Replace(baseline, from, to, 1)
		if mut == baseline {
			t.Fatalf("mutation (%s) fixture did not alter the baseline source", name)
		}
		dir := testsupport.TempDir(t)
		writeGuardGoFile(t, dir, "p.go", mut)
		res, err := analyzeGatedriveLaunchSites(dir)
		if err != nil {
			t.Fatalf("analyze mutation (%s): %v", name, err)
		}
		if len(res.violations) == 0 {
			t.Errorf("mutation (%s): guard did not detect it", name)
		}
	}

	// Mutation (a): a bare unguarded launch helper is detected.
	mutate(t, "a: bare unguarded launch helper",
		"func (d *Driver) Advance()",
		"func (d *Driver) sneaky() { d.proc.Launch(3) }\nfunc (d *Driver) Advance()")

	// Mutation (b): stripping runLaunchGated from revalidate (the boundary the first
	// launch depends on) is detected — launchScopeless is then reachable only via
	// StartAdmitted, which no longer crosses the boundary.
	mutate(t, "b: stripped run boundary",
		"func (d *Driver) revalidate() error { return d.runLaunchGated(func() error { return nil }) }",
		"func (d *Driver) revalidate() error { return nil }")

	// Mutation (c): stripping the relaunch's lockWorktree call is detected — the
	// relaunch is reachable only via driveAndPersistClaim, which crosses no boundary.
	mutate(t, "c: stripped worktree lock",
		"func (d *Driver) driveSlice() { _ = d.lockWorktree(); d.proc.Launch(1) }",
		"func (d *Driver) driveSlice() { d.proc.Launch(1) }")

	// Mutation (d): a lock taken only AFTER the launch admits nothing.
	mutate(t, "d: worktree lock after the launch",
		"func (d *Driver) driveSlice() { _ = d.lockWorktree(); d.proc.Launch(1) }",
		"func (d *Driver) driveSlice() { d.proc.Launch(1); _ = d.lockWorktree() }")

	// Floor: an empty tree computes zero launch sites and visits zero files — the
	// run-level guard fails on both, proving the walker is falsifiable.
	dirEmpty := testsupport.TempDir(t)
	if res, err := analyzeGatedriveLaunchSites(dirEmpty); err != nil {
		t.Fatalf("analyze empty: %v", err)
	} else {
		if res.launchSites != 0 {
			t.Errorf("empty tree reported %d launch sites, want 0", res.launchSites)
		}
		if res.visitedFiles != 0 {
			t.Errorf("empty tree visited %d files, want 0", res.visitedFiles)
		}
	}
}

func writeGuardGoFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
