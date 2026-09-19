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

// Change 0437 Task 8: a syntactic, computed guard that binds every process
// launch in this package to the epoch-authorization boundary. Spec AC7:
// "Derive any launch-site guard from syntactic executable call sites, not a
// hand-maintained spelling list."
//
// The guard is derived entirely from the AST of the package's non-test source:
//
//   1. Population — every call expression of the shape <recv>.<procField>.Launch(…)
//      where <procField> is the name of the Driver's ProcessSeam field, itself
//      read from the struct declaration (never a regex or a written-down spelling
//      of "proc" — a byte-pattern guard matches a spelling, learning
//      byte-pattern-guard-matches-a-spelling).
//   2. Boundary set — the functions whose body calls the single epoch-authorization
//      helper epochGated (the ONE helper every epoch-backed reservation/launch
//      authorization flows through, by design of Task 1). Computed from the AST,
//      not enumerated.
//   3. Reachability — the package-internal receiver call graph. Every function
//      that contains a launch site must be adjacent to the boundary: it must
//      either call a boundary-set member itself, or be invoked ONLY by functions
//      that call a boundary-set member. A launch reachable through a caller that
//      does not pass through the boundary — or a launch with no caller at all — is
//      a violation. Adjacency (rather than mere transitive reach-a-boundary) is
//      what makes the guard falsifiable: StartAdmitted and Advance each cross a
//      boundary before their FIRST launch, so a purely transitive model would keep
//      a later, separately-authorized launch (the automatic relaunch, guarded only
//      by authorizeRelaunch) green after its own boundary was stripped.
//
// The population is COMPUTED and asserted non-empty: a guard that finds zero
// launch sites is broken, not green (learning marker-scoped-guard-needs-a-population-floor).
// The guard is mutation-tested by TestLaunchSiteGuardIsFalsifiable.

// launchGuardResult is the computed accounting one scan produces.
type launchGuardResult struct {
	procField     string   // ProcessSeam field name, derived from the struct decl (AST)
	launchSites   int      // total <recv>.<procField>.Launch(…) call expressions
	boundaryFuncs []string // functions whose body calls epochGated (sorted)
	launchFuncs   []string // functions whose body contains a launch site (sorted)
	violations    []string
	visitedFiles  int
}

// funcInfo is the per-function-name AST-derived facts the call graph needs.
type funcInfo struct {
	callsEpochGated bool
	launchCount     int
	callees         map[string]bool // receiver-method calls d.<name>(…)
}

// analyzeGatedriveLaunchSites parses every non-test .go file under root and
// computes the launch-site guard accounting. Facts are merged by function name
// (conservative: a name is a boundary member if ANY declaration of that name
// calls epochGated; a launch func if ANY launches; callees are unioned) so a
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
							res.launchSites++
							return true
						}
					}
					// A receiver-method call: d.<name>(…) where the receiver is a bare
					// identifier. Records both the epochGated boundary call and every
					// intra-package call edge.
					if _, ok := sel.X.(*ast.Ident); ok {
						if sel.Sel.Name == "epochGated" {
							fi.callsEpochGated = true
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
		if fi.callsEpochGated {
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
	}
	sort.Strings(res.launchFuncs)

	for _, name := range res.launchFuncs {
		if directlyGuarded(name) {
			continue
		}
		callers := callersOf(name)
		if len(callers) == 0 {
			res.violations = append(res.violations,
				name+" launches but crosses no epoch boundary and has no caller to cross one")
			continue
		}
		for _, c := range callers {
			if !directlyGuarded(c) {
				res.violations = append(res.violations,
					"launch in "+name+" is reachable via caller "+c+" which does not cross the epoch boundary")
			}
		}
	}

	return res, nil
}

// TestLaunchSitesBoundToEpochGate is the run-level guard: every launch site in
// the finished package is bound, syntactically, to the epoch-authorization
// boundary, and the computed population is non-empty.
func TestLaunchSitesBoundToEpochGate(t *testing.T) {
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
		t.Fatalf("no epoch-boundary functions found (none call epochGated) — guard is vacuous")
	}
	// Population floor (computed and reported): a guard that finds no launch sites
	// is broken, not green.
	if res.launchSites == 0 {
		t.Fatalf("population floor: found zero %s.Launch call sites — guard is broken, not green", res.procField)
	}
	for _, v := range res.violations {
		t.Errorf("unguarded launch site: %s", v)
	}
	t.Logf("launch-site guard: %d launch call sites in %d files; procField=%q; boundary funcs=%v; launch funcs=%v",
		res.launchSites, res.visitedFiles, res.procField, res.boundaryFuncs, res.launchFuncs)
}

// TestLaunchSiteGuardIsFalsifiable mutation-tests the guard both ways against
// synthetic source trees that reproduce the package's launch/boundary shape:
// (a) a bare unguarded launch helper is detected, (b) stripping epochGated from
// the boundary a launch depends on is detected, plus a clean control and the
// population/visited floors. A guard that cannot redden is decoration (AGENTS.md).
func TestLaunchSiteGuardIsFalsifiable(t *testing.T) {
	// A guarded baseline mirroring the real shape: driveSlice launches and is
	// directly guarded (calls the boundary authorizeRelaunch); launchScopeless
	// launches and is caller-guarded (its only caller StartAdmitted crosses the
	// boundary revalidate); the relaunch launch is reached through the recovery
	// path (Advance -> driveAndPersistClaim -> driveSlice).
	baseline := "" +
		"package p\n" +
		"type ProcessSeam interface{ Launch(x int) (int, error) }\n" +
		"type Driver struct{ proc ProcessSeam }\n" +
		"func (d *Driver) epochGated(f func() error) error { return f() }\n" +
		"func (d *Driver) authorizeRelaunch() error { return d.epochGated(func() error { return nil }) }\n" +
		"func (d *Driver) revalidate() error { return d.epochGated(func() error { return nil }) }\n" +
		"func (d *Driver) recovery() error { return d.epochGated(func() error { return nil }) }\n" +
		"func (d *Driver) driveSlice() { _ = d.authorizeRelaunch(); d.proc.Launch(1) }\n" +
		"func (d *Driver) driveAndPersistClaim() { d.driveSlice() }\n" +
		"func (d *Driver) launchScopeless() { d.proc.Launch(2) }\n" +
		"func (d *Driver) StartAdmitted() { _ = d.revalidate(); d.launchScopeless() }\n" +
		"func (d *Driver) Advance() { _ = d.recovery(); d.driveAndPersistClaim() }\n"

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
		if len(res.violations) != 0 {
			t.Errorf("baseline flagged violations: %v", res.violations)
		}
	}

	// Mutation (a): a bare unguarded launch helper is detected.
	dirA := testsupport.TempDir(t)
	writeGuardGoFile(t, dirA, "p.go", baseline+
		"func (d *Driver) sneaky() { d.proc.Launch(3) }\n")
	if res, err := analyzeGatedriveLaunchSites(dirA); err != nil {
		t.Fatalf("analyze mutation (a): %v", err)
	} else if len(res.violations) == 0 {
		t.Errorf("mutation (a): guard did not detect a bare unguarded launch helper")
	}

	// Mutation (b): stripping epochGated from authorizeRelaunch (the boundary the
	// driveSlice launch depends on) is detected — the launch is now reachable only
	// via driveAndPersistClaim, which does not cross the boundary.
	dirB := testsupport.TempDir(t)
	mutB := strings.Replace(baseline,
		"func (d *Driver) authorizeRelaunch() error { return d.epochGated(func() error { return nil }) }",
		"func (d *Driver) authorizeRelaunch() error { return nil }",
		1)
	if mutB == baseline {
		t.Fatalf("mutation (b) fixture did not alter the baseline source")
	}
	writeGuardGoFile(t, dirB, "p.go", mutB)
	if res, err := analyzeGatedriveLaunchSites(dirB); err != nil {
		t.Fatalf("analyze mutation (b): %v", err)
	} else if len(res.violations) == 0 {
		t.Errorf("mutation (b): guard did not detect the stripped epochGated boundary")
	}

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
