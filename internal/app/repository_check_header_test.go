package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// stateConstants returns the value of every const declared with type State in
// reposetup/classify.go. It keys on the declaration's shape (a typed const of
// State), never a hand-written list, so a state added there reaches
// TestCheckStatePhrasesCoverEveryState without anyone remembering to list it.
func stateConstants(t *testing.T) []string {
	t.Helper()
	path := filepath.Join(appPackageDir(t), "..", "reposetup", "classify.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			ident, ok := vs.Type.(*ast.Ident)
			if !ok || ident.Name != "State" {
				continue
			}
			for _, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("State const value is not a string literal: %#v", v)
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("unquote %s: %v", lit.Value, err)
				}
				out = append(out, s)
			}
		}
	}
	if len(out) < 7 {
		t.Fatalf("found %d State constants in %s, want at least 7: %v", len(out), path, out)
	}
	return out
}

// Both directions: every declared state has a phrase, and every phrase belongs
// to a declared state (a phrase for a removed state is dead text).
func TestCheckStatePhrasesCoverEveryState(t *testing.T) {
	states := stateConstants(t)
	declared := map[string]bool{}
	for _, s := range states {
		declared[s] = true
		if checkStatePhrases[reposetup.State(s)] == "" {
			t.Errorf("state %q has no checkStatePhrases entry", s)
		}
	}
	for k := range checkStatePhrases {
		if !declared[string(k)] {
			t.Errorf("checkStatePhrases key %q is not a declared reposetup.State", k)
		}
	}
}

func TestCheckHeaderForms(t *testing.T) {
	warn := reposetup.Finding{Code: "w", Severity: reposetup.SeverityWarning}
	errF := reposetup.Finding{Code: "e", Severity: reposetup.SeverityError}
	cases := []struct {
		state    reposetup.State
		findings []reposetup.Finding
		want     string
	}{
		{reposetup.StateHealthy, nil, "repository check: setup healthy, no problems found"},
		{reposetup.StateHealthy, []reposetup.Finding{warn}, "repository check: setup healthy, 1 warning (read-only, nothing was changed)"},
		{reposetup.StateLegacy, []reposetup.Finding{warn, errF, warn}, "repository check: old single-branch layout, 1 error, 2 warnings (read-only, nothing was changed)"},
		{reposetup.StateFresh, nil, "repository check: not set up yet (read-only, nothing was changed)"},
	}
	for _, c := range cases {
		if got := checkHeader(string(c.state), c.findings); got != c.want {
			t.Errorf("checkHeader(%q, %d findings) = %q, want %q", c.state, len(c.findings), got, c.want)
		}
	}
}

// A blank or unrecognised state must never print "repository check:  ()"; it
// falls back to the unknown phrase.
func TestCheckHeaderUnknownAndBlankState(t *testing.T) {
	findings := []reposetup.Finding{{Code: "e", Severity: reposetup.SeverityError}}
	const want = "repository check: setup state unknown, 1 error (read-only, nothing was changed)"
	for _, state := range []string{"", "bogus", "unknown"} {
		if got := checkHeader(state, findings); got != want {
			t.Errorf("checkHeader(%q) = %q, want %q", state, got, want)
		}
	}
}

func TestFindingCountsOtherSeverity(t *testing.T) {
	findings := []reposetup.Finding{
		{Severity: reposetup.Severity("info")},
		{Severity: reposetup.SeverityError},
		{Severity: reposetup.Severity("note")},
	}
	if got, want := findingCounts(findings), "1 error, 2 other findings"; got != want {
		t.Errorf("findingCounts = %q, want %q", got, want)
	}
	if got := findingCounts(nil); got != "" {
		t.Errorf("findingCounts(nil) = %q, want empty", got)
	}
}
