package app

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
)

// codeConstants parses dir's non-test .go files (minus skip) and returns
// value → "file:Name" for every string const whose name starts with Code.
func codeConstants(t *testing.T, dir string, skip map[string]bool) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || skip[name] {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, id := range vs.Names {
					if !strings.HasPrefix(id.Name, "Code") || i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					v, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatal(err)
					}
					out[v] = name + ":" + id.Name
				}
			}
		}
	}
	return out
}

// validationCodeSet is the spec's coverage set: every Code constant in
// internal/domain and internal/repository, minus repository/evolution.go,
// whose codes only the transaction engine emits.
func validationCodeSet(t *testing.T) map[string]string {
	t.Helper()
	root := appPackageDir(t)
	set := codeConstants(t, filepath.Join(root, "..", "domain"), nil)
	for v, where := range codeConstants(t, filepath.Join(root, "..", "repository"), map[string]bool{"evolution.go": true}) {
		set[v] = where
	}
	return set
}

type validationFixture struct {
	label   string
	finding domain.Finding
}

func fxChange(id int, slug string) domain.EntityRef {
	return domain.EntityRef{Kind: domain.EntityChange, ID: id, Slug: slug, Path: fmt.Sprintf("docs/changes/active/%04d-%s.md", id, slug)}
}

func fxArchived(id int, slug string) domain.EntityRef {
	return domain.EntityRef{Kind: domain.EntityChange, ID: id, Slug: slug, Path: fmt.Sprintf("docs/changes/archive/2026-05-28-%04d-%s.md", id, slug)}
}

func fxADR(id int, slug string) domain.EntityRef {
	return domain.EntityRef{Kind: domain.EntityADR, ID: id, Slug: slug, Path: fmt.Sprintf("docs/adrs/%04d-%s.md", id, slug)}
}

func fxLearning(slug string) domain.EntityRef {
	return domain.EntityRef{Kind: domain.EntityLearning, Slug: slug, Path: "docs/changes/learnings/" + slug + ".md"}
}

// fxDecoded is the subject a decoder attributes its findings to: kind and
// path only, before identity is known.
func fxDecoded(kind domain.EntityKind, recordPath string) domain.EntityRef {
	return domain.EntityRef{Kind: kind, Path: recordPath}
}

func fxGraph(code string, sev domain.Severity, field string, related []domain.EntityRef, detail map[string]string) domain.Finding {
	d := map[string]string{"path": "docs/adrs/0012-x.md"}
	for k, v := range detail {
		d[k] = v
	}
	return domain.Finding{Code: code, Severity: sev, Entity: domain.EntityRef{Kind: domain.EntityADR, ID: 12}, Field: field, Related: related, Detail: d}
}

func changeTarget(id int) []domain.EntityRef {
	return []domain.EntityRef{{Kind: domain.EntityChange, ID: id}}
}

func adrTarget(id int) []domain.EntityRef {
	return []domain.EntityRef{{Kind: domain.EntityADR, ID: id}}
}

func cycleFixture(code, field string, ids ...int) domain.Finding {
	var related []domain.EntityRef
	var rendered []string
	for _, id := range ids {
		related = append(related, domain.EntityRef{Kind: domain.EntityChange, ID: id})
		rendered = append(rendered, fmt.Sprintf("%04d", id))
	}
	return domain.Finding{Code: code, Severity: domain.SeverityError, Entity: related[0], Field: field,
		Related: related, Detail: map[string]string{"members": strings.Join(rendered, ",")}}
}

func danglingDep() domain.Finding {
	return domain.Finding{
		Code:     repository.CodeChangeReferenceDangling,
		Severity: domain.SeverityError,
		Entity:   domain.EntityRef{Kind: domain.EntityChange, ID: 263, Slug: "x", Path: "docs/changes/active/0263-x.md"},
		Field:    "depends_on",
		Related:  changeTarget(172),
		Detail:   map[string]string{"lookup": "absent", "target": "172"},
	}
}

// validationFixtures holds one realistic finding per validator code, plus one
// per branch the table's text switches on. Each mirrors its producer's shape
// (repository validate.go, decode.go, build.go; domain adr.go).
func validationFixtures() map[string][]validationFixture {
	E, W := domain.SeverityError, domain.SeverityWarning
	f := func(code string, sev domain.Severity, e domain.EntityRef, field string, detail map[string]string) domain.Finding {
		return domain.Finding{Code: code, Severity: sev, Entity: e, Field: field, Detail: detail}
	}
	ref := func(code string, sev domain.Severity, e domain.EntityRef, field string, target []domain.EntityRef, lookup string) domain.Finding {
		x := f(code, sev, e, field, map[string]string{"lookup": lookup, "target": strconv.Itoa(target[0].ID)})
		x.Related = target
		return x
	}
	changePath := "docs/changes/active/0263-x.md"
	adrPath := "docs/adrs/0012-x.md"
	learnPath := "docs/changes/learnings/shell-pipe.md"
	return map[string][]validationFixture{
		repository.CodeRecordUndecodable: {
			{"undecodable", f(repository.CodeRecordUndecodable, E, fxDecoded(domain.EntityChange, changePath), "",
				map[string]string{"error": "yaml: line 3: mapping values are not allowed in this context"})},
		},
		repository.CodeFieldMalformed: {
			{"error", f(repository.CodeFieldMalformed, E, fxDecoded(domain.EntityChange, changePath), "stacked_on", map[string]string{"raw": "abc"})},
			{"warning", f(repository.CodeFieldMalformed, W, fxDecoded(domain.EntityChange, changePath), "updated", map[string]string{"raw": "yesterday"})},
			{"raw empty", f(repository.CodeFieldMalformed, E, fxDecoded(domain.EntityChange, changePath), "depends_on", map[string]string{"raw": ""})},
		},
		repository.CodeListItemMalformed: {
			{"item", f(repository.CodeListItemMalformed, E, fxDecoded(domain.EntityChange, changePath), "depends_on", map[string]string{"raw": "#12", "index": "1"})},
			{"raw empty", f(repository.CodeListItemMalformed, E, fxDecoded(domain.EntityChange, changePath), "related", map[string]string{"raw": "", "index": "0"})},
		},
		repository.CodeChangeTypeInvalid: {
			{"type", f(repository.CodeChangeTypeInvalid, E, fxDecoded(domain.EntityChange, changePath), "type", map[string]string{"raw": "Big Feature"})},
		},
		repository.CodeADRStatusUnparseable: {
			{"status", f(repository.CodeADRStatusUnparseable, E, fxDecoded(domain.EntityADR, adrPath), "status", map[string]string{"raw": "Approved"})},
			{"raw empty", f(repository.CodeADRStatusUnparseable, E, fxDecoded(domain.EntityADR, adrPath), "status", map[string]string{"raw": ""})},
		},
		repository.CodeLearningPromotionUnknown: {
			{"state", f(repository.CodeLearningPromotionUnknown, E, fxDecoded(domain.EntityLearning, learnPath), "promotion_state", map[string]string{"raw": "graduated"})},
		},
		repository.CodeChangeIDInvalid: {
			{"zero", f(repository.CodeChangeIDInvalid, E, domain.EntityRef{Kind: domain.EntityChange, Slug: "x", Path: "docs/changes/active/0000-x.md"}, "id", map[string]string{"raw": "0"})},
			{"negative", f(repository.CodeChangeIDInvalid, E, domain.EntityRef{Kind: domain.EntityChange, Slug: "x", Path: "docs/changes/active/0000-x.md"}, "id", map[string]string{"raw": "-3"})},
		},
		repository.CodeADRIDInvalid: {
			{"zero", f(repository.CodeADRIDInvalid, E, domain.EntityRef{Kind: domain.EntityADR, Slug: "x", Path: "docs/adrs/0000-x.md"}, "id", map[string]string{"raw": "0"})},
		},
		repository.CodeChangeSlugInvalid: {
			{"bad", f(repository.CodeChangeSlugInvalid, E, fxChange(263, "Bad_Slug"), "slug", map[string]string{"raw": "Bad_Slug"})},
			{"raw empty", f(repository.CodeChangeSlugInvalid, E, domain.EntityRef{Kind: domain.EntityChange, ID: 263, Path: "docs/changes/active/0263-x.md"}, "slug", map[string]string{"raw": ""})},
		},
		repository.CodeADRSlugInvalid: {
			{"bad", f(repository.CodeADRSlugInvalid, E, fxADR(12, "Bad--slug"), "slug", map[string]string{"raw": "Bad--slug"})},
		},
		repository.CodeLearningSlugInvalid: {
			{"bad", f(repository.CodeLearningSlugInvalid, E, fxLearning("Shell_Pipe"), "slug", map[string]string{"raw": "Shell_Pipe"})},
		},
		repository.CodeChangeStatusUnknown: {
			{"unknown", f(repository.CodeChangeStatusUnknown, E, fxChange(263, "x"), "status", map[string]string{"raw": "wip"})},
			{"raw empty", f(repository.CodeChangeStatusUnknown, E, fxChange(263, "x"), "status", map[string]string{"raw": ""})},
		},
		repository.CodeChangePriorityUnknown: {
			{"unknown", f(repository.CodeChangePriorityUnknown, W, fxChange(263, "x"), "priority", map[string]string{"raw": "urgent", "status": "in-progress"})},
			{"raw empty", f(repository.CodeChangePriorityUnknown, W, fxChange(263, "x"), "priority", map[string]string{"raw": "", "status": "in-progress"})},
			{"proposed", f(repository.CodeChangePriorityUnknown, W, fxChange(263, "x"), "priority", map[string]string{"raw": "urgent", "status": "proposed"})},
		},
		repository.CodeChangeDeliverableUnknown: {
			{"unknown", f(repository.CodeChangeDeliverableUnknown, W, fxChange(263, "x"), "deliverable", map[string]string{"raw": "research", "status": "in-progress"})},
			{"proposed", f(repository.CodeChangeDeliverableUnknown, W, fxChange(263, "x"), "deliverable", map[string]string{"raw": "research", "status": "proposed"})},
		},
		repository.CodeChangeFilenameMismatch: {
			{"mismatch", f(repository.CodeChangeFilenameMismatch, E, domain.EntityRef{Kind: domain.EntityChange, ID: 263, Slug: "x", Path: "docs/changes/active/0263-y.md"}, "path",
				map[string]string{"base": "0263-y.md", "slug": "x", "id": "0263"})},
		},
		repository.CodeADRFilenameMismatch: {
			{"mismatch", f(repository.CodeADRFilenameMismatch, E, domain.EntityRef{Kind: domain.EntityADR, ID: 12, Slug: "x", Path: "docs/adrs/0013-x.md"}, "path",
				map[string]string{"base": "0013-x.md", "slug": "x", "id": "0012"})},
		},
		repository.CodeChangeIDDuplicate: {
			{"two", f(repository.CodeChangeIDDuplicate, E, fxChange(263, "x"), "id", map[string]string{"count": "2"})},
			{"three", f(repository.CodeChangeIDDuplicate, E, fxChange(263, "x"), "id", map[string]string{"count": "3"})},
		},
		repository.CodeChangeSlugDuplicate: {
			{"two", f(repository.CodeChangeSlugDuplicate, E, fxChange(263, "x"), "slug", map[string]string{"count": "2"})},
			{"three", f(repository.CodeChangeSlugDuplicate, E, fxChange(263, "x"), "slug", map[string]string{"count": "3"})},
		},
		repository.CodeADRIDDuplicate: {
			{"two", f(repository.CodeADRIDDuplicate, E, fxADR(12, "x"), "id", map[string]string{"count": "2"})},
		},
		repository.CodeLearningSlugDuplicate: {
			{"two", f(repository.CodeLearningSlugDuplicate, E, fxLearning("shell-pipe"), "slug", map[string]string{"count": "2"})},
		},
		repository.CodeRecordPathDuplicate: {
			{"dup", f(repository.CodeRecordPathDuplicate, E, domain.EntityRef{Kind: domain.EntityChange, Path: changePath}, "path", map[string]string{"count": "2"})},
		},
		repository.CodeChangePlacementInvalid: {
			{"active", f(repository.CodeChangePlacementInvalid, E, fxChange(263, "x"), "status", map[string]string{"placement": "active", "status": "done"})},
			{"archive", f(repository.CodeChangePlacementInvalid, E, fxArchived(263, "x"), "status", map[string]string{"placement": "archive", "status": "proposed"})},
		},
		repository.CodeChangeArchiveDateInvalid: {
			{"date", f(repository.CodeChangeArchiveDateInvalid, E, domain.EntityRef{Kind: domain.EntityChange, ID: 263, Slug: "x", Path: "docs/changes/archive/0263-x.md"}, "path",
				map[string]string{"base": "0263-x.md"})},
		},
		repository.CodeChangeStateIncoherent: {
			{"missing", f(repository.CodeChangeStateIncoherent, E, fxChange(263, "x"), "branch", map[string]string{"status": "in-progress"})},
		},
		repository.CodeChangeReferenceDangling: {
			{"depends_on", danglingDep()},
			{"stacked_on", ref(repository.CodeChangeReferenceDangling, E, fxChange(263, "x"), "stacked_on", changeTarget(250), "absent")},
			{"other field", ref(repository.CodeChangeReferenceDangling, W, fxChange(263, "x"), "related", changeTarget(99), "absent")},
			{"adrs", ref(repository.CodeChangeReferenceDangling, W, fxChange(263, "x"), "adrs", adrTarget(71), "absent")},
			{"ambiguous", ref(repository.CodeChangeReferenceDangling, E, fxChange(263, "x"), "depends_on", changeTarget(172), "ambiguous")},
		},
		repository.CodeChangeDependencyCycle: {
			{"two", cycleFixture(repository.CodeChangeDependencyCycle, "depends_on", 12, 13)},
			{"three", cycleFixture(repository.CodeChangeDependencyCycle, "depends_on", 12, 13, 14)},
			{"self", cycleFixture(repository.CodeChangeDependencyCycle, "depends_on", 12)},
		},
		repository.CodeChangeStackCycle: {
			{"two", cycleFixture(repository.CodeChangeStackCycle, "stacked_on", 12, 13)},
			{"self", cycleFixture(repository.CodeChangeStackCycle, "stacked_on", 12)},
		},
		repository.CodeArtifactReferenceKindMismatch: {
			{"plan", f(repository.CodeArtifactReferenceKindMismatch, E, fxChange(263, "x"), "plan",
				map[string]string{"path": "docs/changes/active/0250-y.md", "supplied-as": "change"})},
		},
		repository.CodeLearningFilenameMismatch: {
			{"mismatch", f(repository.CodeLearningFilenameMismatch, E, domain.EntityRef{Kind: domain.EntityLearning, Slug: "shell-pipe", Path: "docs/changes/learnings/pipe.md"}, "path",
				map[string]string{"base": "pipe.md", "slug": "shell-pipe"})},
		},
		repository.CodeLearningTopicInvalid: {
			{"topic", f(repository.CodeLearningTopicInvalid, E, fxLearning("shell-pipe"), "topics", map[string]string{"raw": "Shell", "index": "0"})},
			{"raw empty", f(repository.CodeLearningTopicInvalid, E, fxLearning("shell-pipe"), "topics", map[string]string{"raw": "", "index": "1"})},
		},
		repository.CodeLearningPromotionDestination: {
			{"error", f(repository.CodeLearningPromotionDestination, E, fxLearning("shell-pipe"), "promoted_to", map[string]string{"promotion_state": "promoted"})},
			{"warning", f(repository.CodeLearningPromotionDestination, W, fxLearning("shell-pipe"), "promoted_to",
				map[string]string{"promotion_state": "candidate", "raw": "AGENTS.md"})},
		},
		repository.CodeLearningReferenceDangling: {
			{"absent", ref(repository.CodeLearningReferenceDangling, W, fxLearning("shell-pipe"), "changes", changeTarget(172), "absent")},
		},
		repository.CodeRecordUnaccounted: {
			{"unaccounted", domain.Finding{Code: repository.CodeRecordUnaccounted, Severity: E,
				Entity: domain.EntityRef{Kind: domain.EntityRepo, Path: "docs/changes/active/notes.md"}}},
		},
		domain.CodeADRBackpointerMismatch: {
			{"edge", fxGraph(domain.CodeADRBackpointerMismatch, E, "supersedes", adrTarget(5),
				map[string]string{"expected": "Superseded by ADR-0012", "actual": "Accepted"})},
			{"status", fxGraph(domain.CodeADRBackpointerMismatch, E, "status", adrTarget(13),
				map[string]string{"status": "Superseded by ADR-0013", "missing-edge": "supersedes", "expected-on": "0013", "expected-edge": "0012"})},
		},
		domain.CodeADRDanglingReference: {
			{"error", fxGraph(domain.CodeADRDanglingReference, E, "supersedes", adrTarget(99), map[string]string{"lookup": "absent"})},
			{"warning", fxGraph(domain.CodeADRDanglingReference, W, "relates_to", adrTarget(99), map[string]string{"lookup": "ambiguous"})},
			{"status", fxGraph(domain.CodeADRDanglingReference, E, "status", adrTarget(99), map[string]string{"lookup": "absent"})},
			{"change", fxGraph(domain.CodeADRDanglingReference, W, "change", changeTarget(172), map[string]string{"lookup": "absent"})},
		},
		domain.CodeADRIDGap: {
			{"gap", domain.Finding{Code: domain.CodeADRIDGap, Severity: W, Entity: domain.EntityRef{Kind: domain.EntityADR, ID: 7},
				Detail: map[string]string{"highest": "0012"}}},
		},
		domain.CodeADRSelfReference: {
			{"edge", fxGraph(domain.CodeADRSelfReference, E, "supersedes", nil, nil)},
			{"status", fxGraph(domain.CodeADRSelfReference, E, "status", nil, nil)},
		},
	}
}

// TestValidationTextCoversEveryValidatorCode keeps the table, its fixtures,
// and the Code constants in step in both directions.
func TestValidationTextCoversEveryValidatorCode(t *testing.T) {
	set := validationCodeSet(t)
	if len(set) < 37 {
		t.Fatalf("coverage set has %d codes, want at least 37: %v", len(set), set)
	}
	repoDir := filepath.Join(appPackageDir(t), "..", "repository")
	all := codeConstants(t, repoDir, nil)
	kept := codeConstants(t, repoDir, map[string]bool{"evolution.go": true})
	if len(all)-len(kept) < 1 {
		t.Fatalf("the evolution.go exclusion removed no code (all=%d, kept=%d)", len(all), len(kept))
	}
	fixtures := validationFixtures()
	loc := reposetup.RecordLocator{Checkout: layout.SharedWorktreeDir}
	codes := make([]string, 0, len(set))
	for code := range set {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		if _, ok := validationTexts[code]; !ok {
			t.Errorf("%s (%s) has no validationTexts entry", code, set[code])
		}
		fx, ok := fixtures[code]
		if !ok || len(fx) == 0 {
			t.Errorf("%s (%s) has no validationFixtures entry", code, set[code])
			continue
		}
		for _, x := range fx {
			if x.finding.Code != code {
				t.Errorf("%s/%s: fixture carries code %q", code, x.label, x.finding.Code)
			}
			got := explainValidation(x.finding, loc)
			if got.Message == "" || got.Impact == "" || got.Remedy == "" {
				t.Errorf("%s/%s: empty part in %+v", code, x.label, got)
			}
		}
	}
	for code := range validationTexts {
		if _, ok := set[code]; !ok {
			t.Errorf("validationTexts has %q, which is no validator Code constant", code)
		}
	}
}

// TestValidationTextUsesNoInternalVocabulary keeps the table's text in the
// reader's words.
func TestValidationTextUsesNoInternalVocabulary(t *testing.T) {
	banned := []string{"canonical render", "roster", "scalar", "corpus", "lease", "snapshot", "postcondition", "final", "``"}
	for _, loc := range []reposetup.RecordLocator{{Checkout: ".docket"}, {Label: "your private metadata checkout"}, {}} {
		for code, fx := range validationFixtures() {
			for _, x := range fx {
				got := explainValidation(x.finding, loc)
				for _, part := range []string{got.Message, got.Impact, got.Remedy} {
					lower := strings.ToLower(part)
					for _, word := range banned {
						if strings.Contains(lower, word) {
							t.Errorf("%s/%s: %q contains %q", code, x.label, part, word)
						}
					}
				}
			}
		}
	}
}

// TestExplainValidationTypeRuleMatchesValidator pins that the type finding
// describes domain.ValidTypeToken's rule (a leading letter, then letters,
// digits, or hyphens), not the slug rule, and that each example the text
// gives passes the validator.
func TestExplainValidationTypeRuleMatchesValidator(t *testing.T) {
	f := domain.Finding{Code: repository.CodeChangeTypeInvalid, Severity: domain.SeverityError,
		Entity: fxChange(263, "x"), Field: "type", Detail: map[string]string{"raw": "1fix"}}
	got := explainValidation(f, reposetup.RecordLocator{Checkout: ".docket"})
	for _, part := range []string{got.Message, got.Remedy} {
		if strings.Contains(part, slugRule) || !strings.Contains(part, typeRule) {
			t.Errorf("%q should describe the type rule %q, not the slug rule", part, typeRule)
		}
	}
	for _, ok := range []string{"feature", "fix"} {
		if !domain.ValidTypeToken(ok) || !strings.Contains(got.Remedy, "`"+ok+"`") {
			t.Errorf("example %q: valid=%v, in remedy=%v", ok, domain.ValidTypeToken(ok), strings.Contains(got.Remedy, "`"+ok+"`"))
		}
	}
}

// TestExplainValidationIDInvalidNeverQuotesPlaceholder pins that an id the
// validator reports as 0 (absent, zero, or not a number) is not quoted as a
// stored `id: 0`, while a stored negative id is quoted as written.
func TestExplainValidationIDInvalidNeverQuotesPlaceholder(t *testing.T) {
	for _, code := range []string{repository.CodeChangeIDInvalid, repository.CodeADRIDInvalid} {
		zero := explainValidation(domain.Finding{Code: code, Severity: domain.SeverityError,
			Entity: domain.EntityRef{Path: "docs/adrs/0000-x.md"}, Field: "id", Detail: map[string]string{"raw": "0"}}, reposetup.RecordLocator{})
		if strings.Contains(zero.Message, "`id: 0`") || !strings.Contains(zero.Message, "has no valid `id:`") {
			t.Errorf("%s zero: Message = %q", code, zero.Message)
		}
		neg := explainValidation(domain.Finding{Code: code, Severity: domain.SeverityError,
			Entity: domain.EntityRef{Path: "docs/adrs/0000-x.md"}, Field: "id", Detail: map[string]string{"raw": "-3"}}, reposetup.RecordLocator{})
		if !strings.Contains(neg.Message, "`id: -3`") {
			t.Errorf("%s negative: Message = %q", code, neg.Message)
		}
	}
}

// TestExplainValidationPriorityNamesTypedEditWhenProposed pins that a
// proposed change's priority remedy offers the typed groom revise first and
// keeps the hand edit as the fallback, while any other status (which the
// revise refuses) gets the hand edit alone.
func TestExplainValidationPriorityNamesTypedEditWhenProposed(t *testing.T) {
	mk := func(status string) findingText {
		return explainValidation(domain.Finding{Code: repository.CodeChangePriorityUnknown, Severity: domain.SeverityWarning,
			Entity: fxChange(263, "x"), Field: "priority", Detail: map[string]string{"raw": "urgent", "status": status}},
			reposetup.RecordLocator{Checkout: ".docket"})
	}
	proposed := mk("proposed").Remedy
	typed := strings.Index(proposed, "`docket change groom`")
	hand := strings.Index(proposed, "`.docket/docs/changes/active/0263-x.md`")
	if typed < 0 || hand < 0 || typed > hand || !strings.Contains(proposed, "docket-groom-next") {
		t.Errorf("proposed Remedy should name the typed revise before the hand edit: %q", proposed)
	}
	if other := mk("in-progress").Remedy; strings.Contains(other, "docket change groom") || !strings.Contains(other, "`.docket/docs/changes/active/0263-x.md`") {
		t.Errorf("in-progress Remedy should be the hand edit alone: %q", other)
	}
}

func TestExplainValidationFallbackKeepsCodeAndDetail(t *testing.T) {
	got := explainValidation(domain.Finding{Code: "evolution-something", Detail: map[string]string{"b": "2", "a": "1"}}, reposetup.RecordLocator{})
	want := findingText{Message: "evolution-something (a=1, b=2)"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestExplainValidationDanglingDependency(t *testing.T) {
	got := explainValidation(danglingDep(), reposetup.RecordLocator{Checkout: ".docket"})
	if want := "Change 0263 lists change 0172 under `depends_on`, but no change 0172 exists."; got.Message != want {
		t.Errorf("Message = %q, want %q", got.Message, want)
	}
	if !strings.Contains(got.Impact, "never becomes ready") {
		t.Errorf("Impact = %q", got.Impact)
	}
	if !strings.Contains(got.Remedy, "`.docket/docs/changes/active/0263-x.md`") {
		t.Errorf("Remedy = %q", got.Remedy)
	}
}

func TestExplainValidationADRGraphNamesFile(t *testing.T) {
	f := domain.Finding{Code: domain.CodeADRSelfReference, Severity: domain.SeverityError,
		Entity: domain.EntityRef{Kind: domain.EntityADR, ID: 12}, Field: "supersedes",
		Detail: map[string]string{"path": "docs/adrs/0012-x.md"}}
	got := explainValidation(f, reposetup.RecordLocator{Checkout: ".docket"})
	if !strings.Contains(got.Remedy, "`.docket/docs/adrs/0012-x.md`") {
		t.Fatalf("Remedy = %q", got.Remedy)
	}
}

func TestExplainValidationCycleNamesMembersNotEmptyFile(t *testing.T) {
	f := domain.Finding{Code: repository.CodeChangeDependencyCycle, Severity: domain.SeverityError,
		Entity: domain.EntityRef{Kind: domain.EntityChange, ID: 12}, Field: "depends_on",
		Related: []domain.EntityRef{{Kind: domain.EntityChange, ID: 12}, {Kind: domain.EntityChange, ID: 13}},
		Detail:  map[string]string{"members": "0012,0013"}}
	got := explainValidation(f, reposetup.RecordLocator{Checkout: ".docket"})
	if !strings.Contains(got.Remedy, "0012") || !strings.Contains(got.Remedy, "0013") || strings.Contains(got.Remedy, "``") {
		t.Fatalf("Remedy = %q", got.Remedy)
	}
}

func TestStatusValidationTextNeverPrintsPrivateCheckoutPath(t *testing.T) {
	lay := layout.PrivateLayout("/repo/.git", "/repo", "/home/u/.local/share", "o/r")
	sf := validationFinding(danglingDep(), statusRecordLocator(lay))
	parts := []string{sf.Code, sf.Severity, sf.Entity, sf.Identity, sf.Field, sf.Path, sf.Message, sf.Impact, sf.Remedy}
	parts = append(parts, sf.Related...)
	for _, p := range parts {
		if strings.Contains(p, "/home/u") || (lay.MetadataWorktree != "" && strings.Contains(p, lay.MetadataWorktree)) {
			t.Errorf("status finding part %q names a host-absolute path", p)
		}
	}
	if !strings.Contains(sf.Remedy, "private metadata checkout") {
		t.Errorf("private Remedy = %q", sf.Remedy)
	}
	shared := validationFinding(danglingDep(), statusRecordLocator(layout.SharedLayout("/repo/.git", "/repo")))
	if !strings.Contains(shared.Remedy, "`.docket/docs/changes/active/0263-x.md`") {
		t.Errorf("shared Remedy = %q", shared.Remedy)
	}
}

// TestFindingsToStatusOmitsRemedy pins that a typed operation's findings carry
// the plain-language message and impact but no remedy. Those findings can come
// from the transaction engine's after-gate, which validates a candidate state
// that was never committed, so the table's hand-edit remedy would send the
// reader to fix a file that does not exist or does not have the problem.
//
// Mutation probe: route findingsToStatus through validationFinding unchanged —
// this test must redden.
func TestFindingsToStatusOmitsRemedy(t *testing.T) {
	if explainValidation(danglingDep(), reposetup.RecordLocator{}).Remedy == "" {
		t.Fatal("fixture precondition: the table must offer a remedy for this finding")
	}
	got := findingsToStatus([]domain.Finding{danglingDep()})
	if len(got) != 1 {
		t.Fatalf("findingsToStatus returned %d findings, want 1", len(got))
	}
	if got[0].Remedy != "" {
		t.Errorf("Remedy = %q, want empty for a typed operation's finding", got[0].Remedy)
	}
	if got[0].Message == "" || got[0].Impact == "" {
		t.Errorf("Message = %q, Impact = %q, want both kept", got[0].Message, got[0].Impact)
	}
	if empty := findingsToStatus(nil); empty == nil {
		t.Error("findingsToStatus(nil) = nil, want a non-nil empty slice")
	}
}
