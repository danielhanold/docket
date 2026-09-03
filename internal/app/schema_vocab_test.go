package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"
)

// assertVocabMembers checks that the named vocabulary's Members equal want in
// exact order (the emitted vocabularies derive from ordered source lists).
func assertVocabMembers(t *testing.T, v map[string]Vocabulary, name string, want []string) {
	t.Helper()
	got, ok := v[name]
	if !ok {
		t.Errorf("vocabulary %q missing", name)
		return
	}
	if !reflect.DeepEqual(got.Members, want) {
		t.Errorf("vocabulary %q members = %v, want %v", name, got.Members, want)
	}
}

// TestSchemaVocabulariesCore pins the core named vocabularies to their sources.
func TestSchemaVocabulariesCore(t *testing.T) {
	effects := []string{"effect-alpha", "effect-beta"}
	v := SchemaVocabularies(effects)

	wantFC := make([]string, len(AllFindingCodes))
	for i, c := range AllFindingCodes {
		wantFC[i] = string(c)
	}
	assertVocabMembers(t, v, "finding_codes", wantFC)

	wantResults := make([]string, len(AllResults))
	for i, r := range AllResults {
		wantResults[i] = string(r)
	}
	assertVocabMembers(t, v, "results", wantResults)

	assertVocabMembers(t, v, "priorities", []string{"critical", "high", "medium", "low"})
	assertVocabMembers(t, v, "section_intents", []string{"preserve", "replace", "remove"})
	assertVocabMembers(t, v, "groom_outcomes", []string{"spec", "trivial"})
	assertVocabMembers(t, v, "statuses", []string{
		"proposed", "in-progress", "blocked", "deferred",
		"implemented", "stacked-merged", "done", "killed",
	})
	assertVocabMembers(t, v, "effects", effects)

	ct, ok := v["change_types"]
	if !ok {
		t.Fatal("vocabulary change_types missing")
	}
	if ct.Pattern == "" {
		t.Error("change_types.Pattern is empty; want the ValidTypeToken shape")
	}
	if len(ct.Members) != 0 {
		t.Errorf("change_types.Members = %v, want empty (change types are shape-closed, not a list)", ct.Members)
	}
}

// stringConst is one string-valued constant collected from source: its
// declared type name ("" when untyped), its identifier name, and its value.
type stringConst struct {
	typeName string
	name     string
	value    string
}

// collectStringConsts parses every non-test .go file under each package dir and
// returns every string-valued constant it declares. A constant carries its
// spec's explicit type name (nil → ""); Go does not propagate a type onto a
// spec that supplies its own value, so per-spec .Type is authoritative for the
// families this guard covers.
func collectStringConsts(t *testing.T, dirs ...string) []stringConst {
	t.Helper()
	var out []stringConst
	for _, dir := range dirs {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool {
			return !hasSuffix(fi.Name(), "_test.go")
		}, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", dir, err)
		}
		for _, pkg := range pkgs {
			for _, file := range pkg.Files {
				for _, decl := range file.Decls {
					gd, ok := decl.(*ast.GenDecl)
					if !ok || gd.Tok != token.CONST {
						continue
					}
					for _, spec := range gd.Specs {
						vs, ok := spec.(*ast.ValueSpec)
						if !ok || len(vs.Values) == 0 {
							continue
						}
						typeName := ""
						if id, ok := vs.Type.(*ast.Ident); ok {
							typeName = id.Name
						}
						for i, nameIdent := range vs.Names {
							if i >= len(vs.Values) {
								break
							}
							lit, ok := vs.Values[i].(*ast.BasicLit)
							if !ok || lit.Kind != token.STRING {
								continue
							}
							val, err := strconv.Unquote(lit.Value)
							if err != nil {
								continue
							}
							out = append(out, stringConst{typeName: typeName, name: nameIdent.Name, value: val})
						}
					}
				}
			}
		}
	}
	return out
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// TestVocabularyConstCompleteness ties each emitted Members vocabulary to the
// const group it derives from — located by declared type name (Priority,
// Status, GroomOutcome, SectionIntent, Result, FindingCode) or, for the untyped
// disposition families, by const-name prefix. Every non-subset vocabulary is
// held to set-equality in BOTH directions (an added constant that never reaches
// the surface reddens; so does a phantom member with no constant), per the
// correspondence rule. finding_codes is subset-only: AllFindingCodes folds in
// non-const literal tokens (ReasonStatus*, domain policy-reason tokens) that
// legitimately have no FindingCode constant, so the guard requires only that
// every declared FindingCode constant is listed — the residual Task 3 Step 5
// flagged (deleting an FC constant from AllFindingCodes while keeping the
// constant now reddens here).
func TestVocabularyConstCompleteness(t *testing.T) {
	v := SchemaVocabularies([]string{"noop"})

	appDir := appPackageDir(t)
	domainDir := filepath.Join(appDir, "..", "domain")
	renderDir := filepath.Join(appDir, "..", "render")
	consts := collectStringConsts(t, appDir, domainDir, renderDir)

	type spec struct {
		vocab    string
		byType   string // locate by declared type name
		byPrefix string // locate by const-name prefix (untyped families)
		subset   bool   // consts ⊆ members only (finding_codes)
	}
	specs := []spec{
		{vocab: "finding_codes", byType: "FindingCode", subset: true},
		{vocab: "results", byType: "Result"},
		{vocab: "priorities", byType: "Priority"},
		{vocab: "statuses", byType: "Status"},
		{vocab: "section_intents", byType: "SectionIntent"},
		{vocab: "groom_outcomes", byType: "GroomOutcome"},
		{vocab: "claim_dispositions", byPrefix: "ClaimDisposition"},
		{vocab: "halt_dispositions", byPrefix: "HaltDisp"},
		{vocab: "reclaim_dispositions", byPrefix: "ReclaimDisp"},
		{vocab: "reconcile_dispositions", byPrefix: "ReconcileDisposition"},
		{vocab: "block_dispositions", byPrefix: "BlockDisp"},
		{vocab: "cleanup_dispositions", byPrefix: "CleanupDisp"},
		{vocab: "closeout_dispositions", byPrefix: "CloseoutDisp"},
		{vocab: "merge_dispositions", byPrefix: "MergeDisp"},
		{vocab: "publish_dispositions", byPrefix: "PublishDisp"},
	}

	for _, s := range specs {
		vocab, ok := v[s.vocab]
		if !ok {
			t.Errorf("vocabulary %q not emitted", s.vocab)
			continue
		}
		memberSet := map[string]bool{}
		for _, m := range vocab.Members {
			memberSet[m] = true
		}
		constSet := map[string]bool{}
		for _, c := range consts {
			match := false
			if s.byType != "" && c.typeName == s.byType {
				match = true
			}
			if s.byPrefix != "" && hasPrefix(c.name, s.byPrefix) {
				match = true
			}
			if match {
				constSet[c.value] = true
			}
		}
		if len(constSet) == 0 {
			t.Errorf("vocabulary %q: located no constants (byType=%q byPrefix=%q); the locator drifted from source", s.vocab, s.byType, s.byPrefix)
			continue
		}
		// Forward: every constant reaches the surface.
		for val := range constSet {
			if !memberSet[val] {
				t.Errorf("vocabulary %q: constant value %q is declared but not emitted", s.vocab, val)
			}
		}
		if s.subset {
			continue
		}
		// Reverse: every member is backed by a constant.
		for _, m := range vocab.Members {
			if !constSet[m] {
				t.Errorf("vocabulary %q: member %q has no backing constant", s.vocab, m)
			}
		}
		if len(memberSet) != len(constSet) {
			t.Errorf("vocabulary %q: %d members vs %d constants (set mismatch)", s.vocab, len(memberSet), len(constSet))
		}
	}
	// Guard hygiene: the located finding-code constants must not be empty and
	// must all appear in sorted order in the vocabulary; a trivial belt-and-
	// braces check that collectStringConsts found the FindingCode family.
	var fcConsts []string
	for _, c := range consts {
		if c.typeName == "FindingCode" {
			fcConsts = append(fcConsts, c.value)
		}
	}
	if len(fcConsts) == 0 {
		t.Fatal("collectStringConsts located no FindingCode constants; the AST walk is broken")
	}
	sort.Strings(fcConsts)
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
