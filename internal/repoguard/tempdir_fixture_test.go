package repoguard

// Change 0373's fail-closed fixture guard. A real-process package is one
// whose _test.go files spawn subprocesses (shape: exec.Command — derived
// here at test time, never hand-listed). In those packages every temp dir
// must come from internal/testsupport, whose cleanup drains detached
// writers and retries removal; a bare <ident>.TempDir() call reintroduces
// the "directory not empty" teardown race this change closes.
//
// PROSE vs EXECUTABLE (AGENTS.md/CLAUDE.md, "sort them into prose vs
// executable — only the executable ones can violate a gate, and a
// docs-shaped reading skips right past them"): the ban keys on the
// receiver-call SHAPE, but a raw-byte scan would also fire on the same
// spelling sitting in a comment or a string literal (this file's own error
// text, or a doc comment discussing os.TempDir). So the two violation
// regexes run over a go/scanner rendering with prose MASKED — comments for
// the alias check, comments and string/char literals for the TempDir call
// check — leaving only executable tokens. A real X.TempDir() call is never
// inside a comment or a string, so masking can only remove false positives,
// never hide a genuine violation.
//
// The real-process population is still derived by the raw-byte exec.Command
// shape — the SAME grep shape Task 6 uses to enumerate the adopting packages
// (over-inclusion from a prose mention only widens the scanned set; it can
// never hide a violation).
//
// LIMITATION (asserted, per the byte-pattern-guard learning): the ban
// matches the receiver-call shapes `<ident>.TempDir(` and, since change
// 0462, `<ident>.MkdirTemp(`. It cannot see a call through an interface
// value, a function value, or a helper that shadows the name, and
// os.CreateTemp (temp files) is out of scope. The aliased-import check below
// closes the one cheap evasion (import testsupport under another name and
// the receiver test goes vacuous).
//
// MkdirTemp EXEMPTIONS: a few real-process sites need a lifetime the
// per-test fixture deliberately does not provide — a binary built in
// TestMain (no t), a process-lifetime dir created under sync.Once, failure
// evidence that must survive, a mandated /tmp parent. Such a call is exempt
// only when a line comment on its own line or the line immediately above
// carries the tempdir-exempt marker with a non-empty reason (see
// tempdirExemptRe). The marker is lexed from the RAW source, so marker text
// inside a string literal or a block comment never exempts.
//
// SCOPE (explicit — see scanRoots and realProcFloors below): this guard
// enforces the fixture rule for the real-process test packages under the
// module's two Go test roots, `internal/` (change 0373) and `cmd/` (change
// 0398). The walk roots are a visible property of the test via the named
// scanRoots list, not an accident of a buried literal, so any later change
// of coverage is a deliberate edit to that list. Each root carries a
// population floor in realProcFloors, kept as a SEPARATE list so that
// dropping a root from the walk reddens its floor instead of silently
// taking the floor with it.

import (
	"bytes"
	"fmt"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// scanRoots names the guard's deliberate coverage boundary: the module
// subtrees walked for real-process test packages (see the SCOPE note in the
// file header). Widening or narrowing coverage is an explicit edit to this
// list, never an accident of a buried walk-root literal.
var scanRoots = []string{"internal", "cmd"}

// realProcFloors are module-relative (slash-separated) packages the
// derivation must always find — one per scan root. An empty or rotted
// derivation, or a root silently dropped from scanRoots, then fails loudly
// instead of passing vacuously (marker-scoped guards need a population
// floor). Deliberately separate from scanRoots: see the SCOPE note.
var realProcFloors = []string{"internal/process", "cmd/docket"}

var execCallRe = regexp.MustCompile(`\bexec\.Command`)
var tempDirCallRe = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.TempDir\(`)
var testsupportAliasRe = regexp.MustCompile(`(?m)^\s*(?:([A-Za-z_][A-Za-z0-9_]*)\s+)?"[^"]*internal/testsupport"`)
var mkdirTempCallRe = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.MkdirTemp\(`)

// tempdirExemptRe matches the text of a justified exemption marker: a line
// comment reading "tempdir-exempt:" followed by a non-blank reason.
var tempdirExemptRe = regexp.MustCompile(`^//[ \t]*tempdir-exempt:[ \t]*\S`)

// maskProse returns src with every comment token blanked to spaces (newlines
// preserved so line-anchored regexes keep their line structure), and, when
// maskStrings is set, every string/char literal blanked too. It lexes with
// go/scanner so the classification is Go's own, not a heuristic. This is how
// the shape regexes see only executable tokens (prose-vs-executable rule).
func maskProse(src []byte, maskStrings bool) []byte {
	out := make([]byte, len(src))
	copy(out, src)
	fset := token.NewFileSet()
	f := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(f, src, nil, scanner.ScanComments)
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		blank := tok == token.COMMENT || (maskStrings && (tok == token.STRING || tok == token.CHAR))
		if !blank || lit == "" {
			continue
		}
		off := f.Offset(pos)
		for i := 0; i < len(lit) && off+i < len(out); i++ {
			if out[off+i] != '\n' {
				out[off+i] = ' '
			}
		}
	}
	return out
}

// exemptMarkerLines returns the 1-based lines that carry a justified
// tempdir-exempt marker. It lexes the RAW source with go/scanner (the masked
// view blanks comments), so only a real // comment token counts: marker text
// inside a string literal or a /* */ block never exempts.
func exemptMarkerLines(src []byte) map[int]bool {
	lines := map[int]bool{}
	fset := token.NewFileSet()
	f := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(f, src, nil, scanner.ScanComments)
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT && tempdirExemptRe.MatchString(lit) {
			lines[f.Line(pos)] = true
		}
	}
	return lines
}

// mkdirTempViolations finds every executable <ident>.MkdirTemp( call in src
// (comments and string/char literals masked) and splits them by line into
// violations and exempt calls. A call is exempt only when its own line or the
// line immediately above carries a justified marker (exemptMarkerLines).
func mkdirTempViolations(src []byte) (violations, exempt []int) {
	markers := exemptMarkerLines(src)
	callView := maskProse(src, true)
	for _, loc := range mkdirTempCallRe.FindAllIndex(callView, -1) {
		// maskProse blanks in place and preserves newlines, so offsets in the
		// masked view map to the same line numbers as the raw source.
		line := 1 + bytes.Count(callView[:loc[0]], []byte("\n"))
		if markers[line] || markers[line-1] {
			exempt = append(exempt, line)
		} else {
			violations = append(violations, line)
		}
	}
	return violations, exempt
}

func TestRealProcessPackagesUseFixtureTempDir(t *testing.T) {
	root, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	pkgs := map[string][]string{}
	for _, sr := range scanRoots {
		err = filepath.WalkDir(filepath.Join(root, sr), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, "_test.go") {
				return err
			}
			dir := filepath.Dir(p)
			pkgs[dir] = append(pkgs[dir], p)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	fixtureDir := filepath.Join(root, "internal", "testsupport")
	var realProc []string
	for dir, files := range pkgs {
		if dir == fixtureDir {
			continue // the fixture itself is exempt by construction
		}
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if execCallRe.Match(b) {
				realProc = append(realProc, dir)
				break
			}
		}
	}
	sort.Strings(realProc)
	// Population floors (marker-scoped guards need one): the derivation must
	// find each floor package — internal/process, whose supervisor tests
	// motivated the fixture, and cmd/docket, whose built-binary gate tests
	// spawn the real supervisor. A missing floor means the grep shape rotted
	// or a root was dropped from scanRoots, and the guard would otherwise pass
	// vacuously over that root.
	for _, floor := range realProcFloors {
		if !slices.Contains(realProc, filepath.Join(root, filepath.FromSlash(floor))) {
			t.Fatalf("derivation lost %s — real-process set: %v", floor, realProc)
		}
	}
	var violations []string
	for _, dir := range realProc {
		for _, f := range pkgs[dir] {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			// Comments masked, string literals kept: import paths survive so an
			// aliased testsupport import is still visible.
			aliasView := maskProse(b, false)
			for _, m := range testsupportAliasRe.FindAllStringSubmatch(string(aliasView), -1) {
				if m[1] != "" && m[1] != "testsupport" && m[1] != "_" {
					violations = append(violations, fmt.Sprintf("%s: testsupport imported under alias %q", f, m[1]))
				}
			}
			// Comments AND string/char literals masked: a bare TempDir receiver
			// call is executable code, never prose.
			callView := maskProse(b, true)
			for _, m := range tempDirCallRe.FindAllStringSubmatch(string(callView), -1) {
				if m[1] != "testsupport" {
					violations = append(violations, fmt.Sprintf("%s: bare %s.TempDir() — use testsupport.TempDir(t)", f, m[1]))
				}
			}
			// Change 0462: an executable MkdirTemp call is a hand-rolled temp dir
			// that skips the fixture's drain-then-retry cleanup, unless the site
			// needs a lifetime the per-test fixture cannot provide and says so.
			bad, ok := mkdirTempViolations(b)
			for _, line := range bad {
				violations = append(violations, fmt.Sprintf("%s:%d: MkdirTemp call without a justified tempdir-exempt marker — use testsupport.TempDir(t); only a dir the per-test fixture cannot serve (no t in TestMain, process lifetime under sync.Once, failure evidence that must survive, a mandated parent) may carry an adjacent \"// tempdir-exempt: <reason>\"", f, line))
			}
			for _, line := range ok {
				t.Logf("exempt MkdirTemp: %s:%d", f, line)
			}
		}
	}
	if len(violations) > 0 {
		t.Fatalf("real-process packages must use the testsupport fixture:\n%s", strings.Join(violations, "\n"))
	}
}

func TestMkdirTempViolations(t *testing.T) {
	cases := []struct {
		name          string
		src           string
		wantViolation []int
		wantExempt    []int
	}{
		{"bare call", "package p\nfunc f() {\n\td, _ := os.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", []int{3}, nil},
		{"marker above", "package p\nfunc f() {\n\t// tempdir-exempt: built once in TestMain, no t\n\td, _ := os.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", nil, []int{4}},
		{"same-line marker", "package p\nfunc f() {\n\td, _ := os.MkdirTemp(\"\", \"x\") // tempdir-exempt: process-lifetime dir\n\t_ = d\n}\n", nil, []int{3}},
		{"marker two lines above", "package p\nfunc f() {\n\t// tempdir-exempt: too far away\n\n\td, _ := os.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", []int{5}, nil},
		{"empty reason", "package p\nfunc f() {\n\t// tempdir-exempt:\n\td, _ := os.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", []int{4}, nil},
		{"whitespace reason", "package p\nfunc f() {\n\t// tempdir-exempt:   \t\n\td, _ := os.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", []int{4}, nil},
		{"spelling in comment and string", "package p\n// os.MkdirTemp(\"\", \"x\")\nvar s = \"os.MkdirTemp(\"\n", nil, nil},
		{"marker in string", "package p\nfunc f() {\n\t_ = \"// tempdir-exempt: not a comment\"\n\td, _ := os.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", []int{4}, nil},
		{"block comment marker", "package p\nfunc f() {\n\t/* tempdir-exempt: block comments do not count */\n\td, _ := os.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", []int{4}, nil},
		{"consecutive calls", "package p\nfunc f() {\n\t// tempdir-exempt: first only\n\ta, _ := os.MkdirTemp(\"\", \"a\")\n\tb, _ := os.MkdirTemp(\"\", \"b\")\n\t_, _ = a, b\n}\n", []int{5}, []int{4}},
		{"non-os receiver", "package p\nfunc f() {\n\td, _ := afs.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", []int{3}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotV, gotE := mkdirTempViolations([]byte(c.src))
			if !slices.Equal(gotV, c.wantViolation) || !slices.Equal(gotE, c.wantExempt) {
				t.Fatalf("violations=%v exempt=%v, want violations=%v exempt=%v", gotV, gotE, c.wantViolation, c.wantExempt)
			}
		})
	}
}
