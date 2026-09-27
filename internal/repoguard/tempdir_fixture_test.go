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
// SCOPE (see realProcFloors below): since change 0462 the scan population is
// derived from the WHOLE repository through the shared MaintainedFiles
// walker, filtered to _test.go files — there is no guard-local list of roots
// left to shrink. Narrowing coverage now means editing the categorical
// exclusions in repoguard.go that every repoguard guard depends on.
// realProcFloors keeps a population floor in each known test root
// (internal/ from change 0373, cmd/ from change 0398), so a rotted
// exec.Command shape or an exclusion that swallows a root fails loudly
// instead of passing vacuously.

import (
	"bytes"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// realProcFloors are module-relative (slash-separated) packages the
// whole-repo derivation must always find. An empty or rotted derivation, or
// a shared exclusion that swallows internal/ or cmd/, then fails loudly
// instead of passing vacuously (marker-scoped guards need a population
// floor).
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

// exemptMarkerLines maps each 1-based line that carries a justified
// tempdir-exempt marker to whether that marker is standalone — the first
// non-whitespace token on its line — rather than trailing code. It lexes the
// RAW source with go/scanner (the masked view blanks comments), so only a real
// // comment token counts: marker text inside a string literal or a /* */
// block never exempts.
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
			off := f.Offset(pos)
			lineStart := f.Offset(f.LineStart(f.Line(pos)))
			lines[f.Line(pos)] = len(bytes.TrimSpace(src[lineStart:off])) == 0
		}
	}
	return lines
}

// mkdirTempViolations finds every executable <ident>.MkdirTemp( call in src
// (comments and string/char literals masked) and splits them by line into
// violations and exempt calls. A call is exempt only when its own line carries
// a justified marker, or the line immediately above carries a standalone one
// (exemptMarkerLines): a marker trailing one call never also covers the next.
func mkdirTempViolations(src []byte) (violations, exempt []int) {
	markers := exemptMarkerLines(src)
	callView := maskProse(src, true)
	for _, loc := range mkdirTempCallRe.FindAllIndex(callView, -1) {
		// maskProse blanks in place and preserves newlines, so offsets in the
		// masked view map to the same line numbers as the raw source.
		line := 1 + bytes.Count(callView[:loc[0]], []byte("\n"))
		_, ownLine := markers[line]
		if ownLine || markers[line-1] {
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
	files, err := MaintainedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	// Module-relative slash dir -> its module-relative slash _test.go files.
	pkgs := map[string][]string{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			dir := path.Dir(f)
			pkgs[dir] = append(pkgs[dir], f)
		}
	}
	const fixtureDir = "internal/testsupport"
	read := func(rel string) []byte {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	var realProc []string
	for dir, pkgFiles := range pkgs {
		if dir == fixtureDir {
			continue // the fixture itself is exempt by construction
		}
		for _, f := range pkgFiles {
			if execCallRe.Match(read(f)) {
				realProc = append(realProc, dir)
				break
			}
		}
	}
	sort.Strings(realProc)
	// Population floors (marker-scoped guards need one): the whole-repo
	// derivation must find each floor package — internal/process, whose
	// supervisor tests motivated the fixture, and cmd/docket, whose built-binary
	// gate tests spawn the real supervisor. A missing floor means the
	// exec.Command shape rotted or a shared MaintainedFiles exclusion swallowed
	// a test root, and the guard would otherwise pass vacuously over it.
	for _, floor := range realProcFloors {
		if !slices.Contains(realProc, floor) {
			t.Fatalf("derivation lost %s — real-process set: %v", floor, realProc)
		}
	}
	var violations []string
	for _, dir := range realProc {
		for _, f := range pkgs[dir] {
			b := read(f)
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
		{"trailing marker does not cover next line", "package p\nfunc f() {\n\ta, _ := os.MkdirTemp(\"\", \"a\") // tempdir-exempt: first only\n\tb, _ := os.MkdirTemp(\"\", \"b\")\n\t_, _ = a, b\n}\n", []int{4}, []int{3}},
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
