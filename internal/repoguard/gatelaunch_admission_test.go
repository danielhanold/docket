package repoguard

// Computed launch-site lock hand-over guard (change 0490, rewriting change 0375's
// slot-admission guard). One canonical worktree admits at most one live top-level
// gate, and "busy" means "a live supervisor holds the worktree lock": every launch
// site takes the per-worktree lock (gatedrive.Store.TryWorktreeLock) and hands it
// to the supervisor through process.LaunchRequest.WorktreeLock, which holds it for
// its whole life. A launch that builds its request WITHOUT a WorktreeLock spawns a
// gate no other launch can see — exactly the double-suite this guard exists to
// stop. So EVERY process launch outside internal/process must hand one over.
//
// DERIVATION. The covered population is DERIVED, never remembered: this guard WALKS
// the maintained non-test Go surface under internal/ and collects launch sites by
// SYNTACTIC SHAPE — a call whose selector is `Launch` with exactly one argument —
// subtracting only the launcher's own package (internal/process, by PATH shape).
// Each site's single argument must be either
//
//	(a) a process.LaunchRequest (or package-local LaunchRequest) composite literal
//	    that sets the WorktreeLock key, or
//	(b) a call to a function or method declared in the SAME package (resolved by
//	    the callee's name among that package's FuncDecls) whose body builds such a
//	    literal — and builds no LaunchRequest literal without it.
//
// Any other argument shape (a variable, a field, a cross-package helper) cannot be
// proven and is a violation: fail closed.
//
// Today's population: app's raw GateLaunch (form a) and the gate driver's first
// launch and single relaunch, both through driveRecord.launchRequest (form b).
//
// RESIDUAL RISK, recorded not hidden: the detector keys on the `Launch` selector
// name rather than the receiver's resolved type (no go/types pass), so an unrelated
// future one-argument `.Launch(` would be flagged too — that errs safe. A helper is
// resolved by name only, so two same-named helpers in one package (a function and
// a method) must BOTH carry the lock. The guard proves the lock is handed over, not
// that the handed file is a held lock; TryWorktreeLock is the only producer of one.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// launcherLayer is the launcher's own package, by PATH shape: process.Service.Launch
// is the primitive that receives the lock, never a site that must hand one over.
var launcherLayer = regexp.MustCompile(`^internal/process/`)

// launchSiteShape reports whether call is a process launch by syntactic shape: a
// `.Launch(` selector with exactly one argument (the LaunchRequest).
func launchSiteShape(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Launch" && len(call.Args) == 1
}

// launchRequestLiteral reports whether e (parens and & stripped) is a LaunchRequest
// composite literal, and whether it sets the WorktreeLock key.
func launchRequestLiteral(e ast.Expr) (isLit, hasLock bool) {
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
			continue
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				e = x.X
				continue
			}
		}
		break
	}
	lit, ok := e.(*ast.CompositeLit)
	if !ok {
		return false, false
	}
	switch ty := lit.Type.(type) {
	case *ast.SelectorExpr:
		if ty.Sel.Name != "LaunchRequest" {
			return false, false
		}
	case *ast.Ident:
		if ty.Name != "LaunchRequest" {
			return false, false
		}
	default:
		return false, false
	}
	for _, el := range lit.Elts {
		if kv, ok := el.(*ast.KeyValueExpr); ok {
			if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "WorktreeLock" {
				return true, true
			}
		}
	}
	return true, false
}

// helperBuildsLockedRequest reports whether fn's body builds at least one
// LaunchRequest literal and every such literal sets WorktreeLock.
func helperBuildsLockedRequest(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}
	lits, locked := 0, 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		e, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		if isLit, hasLock := launchRequestLiteral(e); isLit {
			lits++
			if hasLock {
				locked++
			}
			return false
		}
		return true
	})
	return lits > 0 && lits == locked
}

// calleeName returns the bare name a call invokes (`f(` or `x.f(`), or "".
func calleeName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

// lockSiteReport is the analysis of one parsed population.
type lockSiteReport struct {
	sites, literalSites, helperSites int
	violations                       []string
}

// analyzeLaunchLockHandover checks every launch site in files (rel path → parsed
// file; a file's package is its slash directory) for a WorktreeLock hand-over.
func analyzeLaunchLockHandover(files map[string]*ast.File) lockSiteReport {
	funcs := map[string]map[string][]*ast.FuncDecl{} // dir → name → decls
	rels := make([]string, 0, len(files))
	for rel, f := range files {
		rels = append(rels, rel)
		dir := dirOf(rel)
		if funcs[dir] == nil {
			funcs[dir] = map[string][]*ast.FuncDecl{}
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				funcs[dir][fd.Name.Name] = append(funcs[dir][fd.Name.Name], fd)
			}
		}
	}
	sort.Strings(rels)

	var rep lockSiteReport
	for _, rel := range rels {
		if launcherLayer.MatchString(rel) {
			continue
		}
		dir := dirOf(rel)
		ast.Inspect(files[rel], func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !launchSiteShape(call) {
				return true
			}
			rep.sites++
			arg := call.Args[0]
			if isLit, hasLock := launchRequestLiteral(arg); isLit {
				rep.literalSites++
				if !hasLock {
					rep.violations = append(rep.violations, "launch site in "+rel+" builds a LaunchRequest literal without a WorktreeLock")
				}
				return true
			}
			if inner, ok := arg.(*ast.CallExpr); ok {
				name := calleeName(inner)
				decls := funcs[dir][name]
				if name != "" && len(decls) > 0 {
					rep.helperSites++
					for _, fd := range decls {
						if !helperBuildsLockedRequest(fd) {
							rep.violations = append(rep.violations, "launch site in "+rel+" builds its request through "+name+", which does not set WorktreeLock on every LaunchRequest it builds")
							break
						}
					}
					return true
				}
			}
			rep.violations = append(rep.violations, "launch site in "+rel+" passes a request whose WorktreeLock hand-over cannot be proven (neither a LaunchRequest literal nor a same-package helper building one)")
			return true
		})
	}
	return rep
}

// dirOf returns the slash-directory of a slash-relative path.
func dirOf(rel string) string {
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[:i]
	}
	return "."
}

func TestGateLaunchAdmissionCoverage(t *testing.T) {
	root := guardRoot(t)
	pop := maintainedPop(t, root)

	files := map[string]*ast.File{}
	for _, rel := range pop {
		if !hasExt(rel, ".go") || strings.HasSuffix(rel, "_test.go") || !underDir(rel, "internal") {
			continue
		}
		content := readMaintained(t, root, rel)
		file, err := parser.ParseFile(token.NewFileSet(), rel, content, 0)
		if err != nil {
			// Fail closed: an unparseable maintained Go file is a guard failure, not a
			// silent clean miss (probe-error-is-not-clean-absence).
			t.Fatalf("parse %s: %v", rel, err)
		}
		files[rel] = file
	}

	rep := analyzeLaunchLockHandover(files)

	// Population floors FIRST — an empty enumeration passes every "no violations"
	// negative by default. A refactor that renames Launch or LaunchRequest drops the
	// population below a floor and reddens here rather than going vacuous.
	if rep.sites < 3 {
		t.Fatalf("population floor: found %d launch sites (want >= 3: app GateLaunch and the gate driver's first launch and relaunch); the launch-shape detector drifted", rep.sites)
	}
	if rep.literalSites < 1 || rep.helperSites < 1 {
		t.Fatalf("population floor: found %d literal-form and %d helper-form sites (want >= 1 of each); the request-shape detector drifted", rep.literalSites, rep.helperSites)
	}
	if len(rep.violations) != 0 {
		t.Errorf("gate-launch worktree-lock hand-over violations:\n%s", strings.Join(rep.violations, "\n"))
	}

	t.Run("detectors_non_vacuity", func(t *testing.T) {
		parse := func(src string) map[string]*ast.File {
			t.Helper()
			f, err := parser.ParseFile(token.NewFileSet(), "internal/x/x.go", src, 0)
			if err != nil {
				t.Fatalf("parse synthetic source: %v", err)
			}
			return map[string]*ast.File{"internal/x/x.go": f}
		}
		cases := []struct {
			name string
			src  string
			ok   bool
		}{
			{"literal with WorktreeLock passes", `package x
func f() { svc.Launch(process.LaunchRequest{Root: r, WorktreeLock: l.TakeFile()}) }`, true},
			{"literal without WorktreeLock fails", `package x
func f() { svc.Launch(process.LaunchRequest{Root: r, Cwd: c}) }`, false},
			{"helper lacking WorktreeLock fails", `package x
func (r *rec) req(tok string) process.LaunchRequest { return process.LaunchRequest{Root: r.root, ReservationToken: tok} }
func f() { d.proc.Launch(r.req(tok)) }`, false},
			{"helper with WorktreeLock passes", `package x
func (r *rec) req(tok string, l *os.File) process.LaunchRequest { return process.LaunchRequest{Root: r.root, WorktreeLock: l} }
func f() { d.proc.Launch(r.req(tok, lock.TakeFile())) }`, true},
			{"unprovable variable argument fails", `package x
func f(req process.LaunchRequest) { svc.Launch(req) }`, false},
		}
		for _, tc := range cases {
			rep := analyzeLaunchLockHandover(parse(tc.src))
			if rep.sites != 1 {
				t.Errorf("%s: found %d launch sites, want 1", tc.name, rep.sites)
				continue
			}
			if got := len(rep.violations) == 0; got != tc.ok {
				t.Errorf("%s: clean=%v, want %v (violations %v)", tc.name, got, tc.ok, rep.violations)
			}
		}
		// The launcher's own package is never a site; a non-Launch call is never a site.
		f, err := parser.ParseFile(token.NewFileSet(), "internal/process/p.go", `package process
func f() { s.Launch(LaunchRequest{}) }`, 0)
		if err != nil {
			t.Fatal(err)
		}
		if rep := analyzeLaunchLockHandover(map[string]*ast.File{"internal/process/p.go": f}); rep.sites != 0 {
			t.Errorf("internal/process launch counted as a site")
		}
		if rep := analyzeLaunchLockHandover(parse(`package x
func f() { svc.Observe(run) }`)); rep.sites != 0 {
			t.Errorf("non-Launch call counted as a site")
		}
	})
}
