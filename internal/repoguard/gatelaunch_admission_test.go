package repoguard

// Computed launch-site admission guard (change 0375 Task 14). One canonical
// worktree carries at most one reserved-or-running top-level gate execution, so
// EVERY top-level gate launch must first reserve the worktree execution slot
// (internal/gatedrive/admission.go). This guard proves that admission is composed
// wherever a launch happens, and that whatever reserves the slot also releases it.
//
// DERIVATION. The covered population is DERIVED, never remembered — the same rule
// anchored on admitWorkflowMutation (internal/app/rungate_fence.go, "do not rely on
// a remembered list"): this guard WALKS the maintained non-test Go surface under
// internal/, collects launch call sites by SYNTACTIC SHAPE (a `.Launch(` selector,
// a spawnSupervisor call, or an exec.Command that shells `docket ... gate`), keyed
// on shape rather than an enumerated file allowlist, and subtracts the two
// SANCTIONED interiors by PATH shape:
//
//   - internal/process/ — the lowest-level OS launcher (spawnSupervisor); it is the
//     primitive the whole slot mechanism sits on.
//   - internal/gatedrive/ — the admission-HOLDING driver: its ProcessSeam.Launch
//     calls run only after the driver itself reserved the slot (Tasks 3–5), so they
//     carry the ticket and must not re-reserve.
//
// Every REMAINING launch site must sit in a package that reaches the slot's reserve
// API (AST-level: the package contains a call to a reserve-admission symbol). The
// reverse correspondence — a guard runs both ways — is that every package which
// RESERVES the slot also confirms-or-releases it, so a slot is never taken and
// abandoned.
//
// RESIDUAL RISK, recorded not hidden: the launch detector keys on the `.Launch(`
// selector name rather than the receiver's resolved type (no go/types pass), so an
// unrelated future `.Launch(` in a non-interior package would be flagged too. That
// errs safe (it demands admission of a thing that may not need it) and today the
// only `.Launch(` selectors in the tree are the process launches this guard is
// about.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"testing"
)

var (
	// Admission symbol SHAPES — keyed on shape, never a spelling list (the spelling
	// you omit is the one that bites). reserveAdmissionSym matches the reserve family
	// (ReserveWorktreeExecution, ReserveRawWorktreeExecution, reserveWorktreeExecution);
	// confirmReleaseSym matches the confirm/release/mark family
	// (ConfirmWorktreeExecution, ReleaseWorktreeExecution,
	// MarkWorktreeExecutionUnresolved/Stopping).
	reserveAdmissionSym = regexp.MustCompile(`^[Rr]eserve[A-Za-z]*WorktreeExecution$`)
	confirmReleaseSym   = regexp.MustCompile(`^(Confirm|Release|Mark)[A-Za-z]*WorktreeExecution[A-Za-z]*$`)

	// The two sanctioned launch interiors, by PATH shape.
	sanctionedLaunchLayer = regexp.MustCompile(`^internal/(process|gatedrive)/`)
)

// launchCallShape reports whether call is a top-level gate launch by syntactic
// shape: a `.Launch(` selector, a spawnSupervisor(...) call, or an exec.Command
// that shells docket's own `gate` verb.
func launchCallShape(call *ast.CallExpr) bool {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		// A method/selector call: `x.Launch(` (the process/ProcessSeam launch) or
		// `s.spawnSupervisor(` (the process package's own re-exec).
		if fn.Sel.Name == "Launch" || fn.Sel.Name == "spawnSupervisor" {
			return true
		}
		if x, ok := fn.X.(*ast.Ident); ok && x.Name == "exec" && fn.Sel.Name == "Command" {
			return execCommandDocketGate(call)
		}
	case *ast.Ident:
		// A package-local call: `spawnSupervisor(`.
		if fn.Name == "spawnSupervisor" {
			return true
		}
	}
	return false
}

// execCommandDocketGate reports whether an exec.Command call names docket's own
// binary AND a `gate` subcommand across its string-literal arguments — the shape a
// re-exec'd `docket ... gate ...` launch would take.
func execCommandDocketGate(call *ast.CallExpr) bool {
	var docket, gate bool
	for _, arg := range call.Args {
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		v := strings.ToLower(lit.Value)
		if strings.Contains(v, "docket") {
			docket = true
		}
		if strings.Contains(v, "gate") {
			gate = true
		}
	}
	return docket && gate
}

// symCallShape reports whether call invokes an identifier (selector or bare)
// matching re.
func symCallShape(call *ast.CallExpr, re *regexp.Regexp) bool {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		return re.MatchString(fn.Sel.Name)
	case *ast.Ident:
		return re.MatchString(fn.Name)
	}
	return false
}

// dirOf returns the slash-directory of a slash-relative path.
func dirOf(rel string) string {
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[:i]
	}
	return "."
}

type launchSite struct {
	rel string
	dir string
}

func TestGateLaunchAdmissionCoverage(t *testing.T) {
	root := guardRoot(t)
	pop := maintainedPop(t, root)

	var (
		launchSites []launchSite
		reserveDirs = map[string]bool{}
		crDirs      = map[string]bool{}
		reserveN    int
	)

	for _, rel := range pop {
		if !hasExt(rel, ".go") || strings.HasSuffix(rel, "_test.go") || !underDir(rel, "internal") {
			continue
		}
		content := readMaintained(t, root, rel)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, rel, content, 0)
		if err != nil {
			// Fail closed: an unparseable maintained Go file is a guard failure, not a
			// silent clean miss (probe-error-is-not-clean-absence).
			t.Fatalf("parse %s: %v", rel, err)
		}
		dir := dirOf(rel)
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if launchCallShape(call) {
				launchSites = append(launchSites, launchSite{rel: rel, dir: dir})
			}
			if symCallShape(call, reserveAdmissionSym) {
				reserveDirs[dir] = true
				reserveN++
			}
			if symCallShape(call, confirmReleaseSym) {
				crDirs[dir] = true
			}
			return true
		})
	}

	// Split launch sites into the two sanctioned interiors vs the remaining sites.
	interiorDirs := map[string]bool{}
	var remaining []launchSite
	for _, s := range launchSites {
		if sanctionedLaunchLayer.MatchString(s.rel) {
			interiorDirs[s.dir] = true
			continue
		}
		remaining = append(remaining, s)
	}

	// Population floors FIRST — an empty enumeration passes every "no violations"
	// negative by default. A refactor that renames the launch or admission symbols
	// drops the population below a floor and reddens here rather than going vacuous.
	if len(interiorDirs) < 2 {
		t.Fatalf("population floor: found %d sanctioned launch interiors (want >= 2: internal/process and internal/gatedrive); the launch-shape detector or the interior classification drifted", len(interiorDirs))
	}
	if reserveN < 1 {
		t.Fatalf("population floor: found %d worktree-slot reserve calls (want >= 1); the reserve-admission symbol shape drifted from source", reserveN)
	}
	if len(remaining) < 1 {
		t.Fatalf("population floor: found %d non-interior launch sites (want >= 1, e.g. internal/app raw gate.launch); detector D has no live input for the coverage assert", len(remaining))
	}

	var violations []string

	// Forward: every remaining launch site sits in a package that reaches the reserve
	// API — admission is composed wherever a top-level launch happens.
	for _, s := range remaining {
		if !reserveDirs[s.dir] {
			violations = append(violations, "launch site "+s.rel+" is not in a package that reserves the worktree execution slot")
		}
	}

	// Reverse (both-ways correspondence): every package that reserves the slot also
	// confirms-or-releases it — a slot is never taken and abandoned.
	for dir := range reserveDirs {
		if !crDirs[dir] {
			violations = append(violations, "package "+dir+" reserves the worktree execution slot but never confirms or releases it")
		}
	}

	if len(violations) != 0 {
		t.Errorf("gate-launch admission violations:\n%s", strings.Join(violations, "\n"))
	}

	t.Run("detectors_non_vacuity", func(t *testing.T) {
		// A launch call in a package with no reserve reads as an uncovered site.
		if !exprIsLaunch(t, "svc.Launch(req)") {
			t.Errorf("launch detector missed a `.Launch(` selector call")
		}
		if !exprIsLaunch(t, "spawnSupervisor(req, dir, lock)") {
			t.Errorf("launch detector missed a bare spawnSupervisor call")
		}
		if !exprIsLaunch(t, "s.spawnSupervisor(req, dir, lock)") {
			t.Errorf("launch detector missed a selector spawnSupervisor call")
		}
		if !exprIsLaunch(t, `exec.Command(self, "gate", "launch", "--docket")`) {
			t.Errorf("launch detector missed an exec.Command docket-gate shell")
		}
		if exprIsLaunch(t, "svc.Observe(run)") {
			t.Errorf("launch detector wrongly flagged an unrelated method call")
		}
		if exprIsLaunch(t, `exec.Command("git", "rev-parse")`) {
			t.Errorf("launch detector wrongly flagged an unrelated exec.Command")
		}
		// Reserve-admission shape: the whole family, and nothing adjacent.
		for _, sym := range []string{"ReserveWorktreeExecution", "ReserveRawWorktreeExecution", "reserveWorktreeExecution"} {
			if !exprCallMatches(t, "store."+sym+"(rec)", reserveAdmissionSym) {
				t.Errorf("reserve shape missed %q", sym)
			}
		}
		if exprCallMatches(t, "store.ReleaseWorktreeExecution(w, tok)", reserveAdmissionSym) {
			t.Errorf("reserve shape wrongly matched a release call")
		}
		// Confirm/release shape: the whole family, and not a reserve.
		for _, sym := range []string{"ConfirmWorktreeExecution", "ReleaseWorktreeExecution", "MarkWorktreeExecutionUnresolved", "MarkWorktreeExecutionStopping"} {
			if !exprCallMatches(t, "store."+sym+"(w, tok)", confirmReleaseSym) {
				t.Errorf("confirm/release shape missed %q", sym)
			}
		}
		if exprCallMatches(t, "store.ReserveWorktreeExecution(rec)", confirmReleaseSym) {
			t.Errorf("confirm/release shape wrongly matched a reserve call")
		}
	})
}

// exprIsLaunch parses a single call expression and reports launchCallShape.
func exprIsLaunch(t *testing.T, src string) bool {
	t.Helper()
	return exprCall(t, src, launchCallShape)
}

// exprCallMatches parses a single call expression and reports symCallShape against
// re.
func exprCallMatches(t *testing.T, src string, re *regexp.Regexp) bool {
	t.Helper()
	return exprCall(t, src, func(c *ast.CallExpr) bool { return symCallShape(c, re) })
}

func exprCall(t *testing.T, src string, pred func(*ast.CallExpr) bool) bool {
	t.Helper()
	expr, err := parser.ParseExpr(src)
	if err != nil {
		t.Fatalf("parse expr %q: %v", src, err)
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expr %q is not a call", src)
	}
	return pred(call)
}
