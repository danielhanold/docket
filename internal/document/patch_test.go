package document

import (
	"strings"
	"testing"
)

const fixtureDoc = "---\nid: 306\nstatus: proposed   # claim flips this\nadrs: []\npr:\nunknown_key: kept\n---\n\nBody stays.\n"

func applyOne(t *testing.T, src string, build func(*PatchSet)) string {
	t.Helper()
	d := mustParse(t, src)
	var p PatchSet
	build(&p)
	out, err := d.Apply(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestEmptyPatchSetIsByteIdentical(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	out, err := d.Apply(PatchSet{})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != fixtureDoc {
		t.Fatalf("no-op must be byte-identical:\n%q", out)
	}
}

func TestSetFieldPreservesEverythingElse(t *testing.T) {
	got := applyOne(t, fixtureDoc, func(p *PatchSet) {
		p.SetField("status", String("in-progress"))
	})
	want := "---\nid: 306\nstatus: 'in-progress'   # claim flips this\nadrs: []\npr:\nunknown_key: kept\n---\n\nBody stays.\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestSetFieldOnEmptyValue(t *testing.T) {
	got := applyOne(t, fixtureDoc, func(p *PatchSet) {
		p.SetField("pr", Int(211))
	})
	if !strings.Contains(got, "\npr: 211\n") {
		t.Fatalf("empty value must accept a value without eating the newline:\n%q", got)
	}
}

// TestSetFieldOnEmptyValueWithInlineComment pins the one insertion point whose
// naive payload rule corrupts the line: an empty value carrying an inline
// comment puts the zero-width span at the '#', so a bare payload would splice
// the value and the comment into the single scalar "211# set when the PR is
// opened".
func TestSetFieldOnEmptyValueWithInlineComment(t *testing.T) {
	src := "---\nid: 306\npr:   # set when the PR is opened\n---\n"
	got := applyOne(t, src, func(p *PatchSet) { p.SetField("pr", Int(211)) })
	if !strings.Contains(got, "# set when the PR is opened") {
		t.Fatalf("inline comment must survive:\n%q", got)
	}
	d := mustParse(t, got)
	var decoded struct {
		PR *int `yaml:"pr"`
	}
	if err := d.DecodeFrontmatter(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.PR == nil || *decoded.PR != 211 {
		t.Fatalf("patched line must reparse as the typed value 211, got %v in:\n%q", decoded.PR, got)
	}
}

func TestSetFieldToNullDropsValueToken(t *testing.T) {
	got := applyOne(t, "---\nbranch: 'feat/x'\n---\n", func(p *PatchSet) {
		p.SetField("branch", Null())
	})
	if got != "---\nbranch:\n---\n" {
		t.Fatalf("null must render the bare key: form, got %q", got)
	}
}

// TestSetFieldToNullKeepsSpacingBeforeAnInlineComment guards the exception to
// the rule above: swallowing the spacing here would produce "key:# comment",
// which is not a comment at all.
func TestSetFieldToNullKeepsSpacingBeforeAnInlineComment(t *testing.T) {
	got := applyOne(t, "---\nstatus: proposed   # claim flips this\n---\n", func(p *PatchSet) {
		p.SetField("status", Null())
	})
	if !strings.Contains(got, "# claim flips this") {
		t.Fatalf("inline comment must survive:\n%q", got)
	}
	f, ok := mustParse(t, got).Field("status")
	if !ok || f.Shape != ShapeEmpty {
		t.Fatalf("patched field must reparse as an empty value, got shape %v in:\n%q", f.Shape, got)
	}
}

func TestSetFieldNullOnAlreadyEmptyFieldIsByteIdentical(t *testing.T) {
	got := applyOne(t, fixtureDoc, func(p *PatchSet) { p.SetField("pr", Null()) })
	if got != fixtureDoc {
		t.Fatalf("null onto an empty field must be byte-identical:\n%q", got)
	}
}

func TestBatchAppliesAllEdits(t *testing.T) {
	got := applyOne(t, fixtureDoc, func(p *PatchSet) {
		p.SetField("status", String("done"))
		p.SetField("adrs", Seq(Int(93)))
		p.SetField("id", Int(306)) // same value — still a valid edit
	})
	for _, want := range []string{"status: 'done'", "adrs: [93]", "id: 306"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%q", want, got)
		}
	}
}

func TestPatchIsIdempotent(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	var p PatchSet
	p.SetField("status", String("done"))
	once, err := d.Apply(p)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := Parse(once)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := d2.Apply(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(once) != string(twice) {
		t.Fatal("re-applying the same semantic patch must be byte-idempotent")
	}
}

func TestMissingTargetFailsWholeBatch(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	var p PatchSet
	p.SetField("status", String("done"))     // valid
	p.SetField("no_such_field", String("x")) // defect in a LATER item
	out, err := d.Apply(p)
	if !IsKind(err, KindMissingPatchTarget) {
		t.Fatalf("got %v", err)
	}
	if out != nil {
		t.Fatal("on error Apply must return nil bytes")
	}
}

func TestUnsupportedShapeRefused(t *testing.T) {
	d := mustParse(t, "---\nnotes: |\n  text\n---\n")
	var p PatchSet
	p.SetField("notes", String("flat"))
	if _, err := d.Apply(p); !IsKind(err, KindUnsupportedPatchShape) {
		t.Fatalf("got %v", err)
	}
}

func TestUnsupportedShapeOnUnrelatedFieldIsFine(t *testing.T) {
	src := "---\nnotes: |\n  text\nstatus: proposed\n---\n"
	got := applyOne(t, src, func(p *PatchSet) { p.SetField("status", String("done")) })
	if !strings.Contains(got, "notes: |\n  text\n") {
		t.Fatalf("unrelated block scalar must stay byte-identical:\n%q", got)
	}
}

func TestDuplicateEditRejected(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	var p PatchSet
	p.SetField("status", String("a"))
	p.SetField("status", String("b"))
	if _, err := d.Apply(p); !IsKind(err, KindDuplicateEdit) {
		t.Fatalf("got %v", err)
	}
}

func TestInvalidValueRejectedBeforeAnyWork(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	var p PatchSet
	p.SetField("status", String("bad\x00nul"))
	if _, err := d.Apply(p); !IsKind(err, KindInvalidValue) {
		t.Fatalf("got %v", err)
	}
}

func TestInvalidKeyRejected(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	var p PatchSet
	p.SetField("Status", String("done"))
	if _, err := d.Apply(p); !IsKind(err, KindInvalidValue) {
		t.Fatalf("got %v", err)
	}
}

func TestApplyDoesNotMutateInputDocument(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	var p PatchSet
	p.SetField("status", String("done"))
	if _, err := d.Apply(p); err != nil {
		t.Fatal(err)
	}
	if string(d.Source()) != fixtureDoc {
		t.Fatal("Apply mutated the parsed document")
	}
}

func TestPatchedBytesDoNotAliasDocument(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	out, err := d.Apply(PatchSet{})
	if err != nil {
		t.Fatal(err)
	}
	out[0] = 'X'
	if d.Source()[0] == 'X' {
		t.Fatal("returned bytes alias the document")
	}
}

func TestPatchOnNoFrontmatterDocRejected(t *testing.T) {
	d := mustParse(t, "just body\n")
	var p PatchSet
	p.SetField("id", Int(1))
	if _, err := d.Apply(p); !IsKind(err, KindMissingFrontmatter) {
		t.Fatalf("got %v", err)
	}
}

// TestPatchPreservesCRLF checks the splice never rewrites a line terminator it
// does not own.
func TestPatchPreservesCRLF(t *testing.T) {
	src := "---\r\nid: 306\r\nstatus: proposed\r\n---\r\n\r\nBody.\r\n"
	got := applyOne(t, src, func(p *PatchSet) { p.SetField("status", String("done")) })
	want := "---\r\nid: 306\r\nstatus: 'done'\r\n---\r\n\r\nBody.\r\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// TestReparseGateCatchesACorruptingEdit is the control run for the phase-3
// reparse gate: no value reachable through the closed typed model can corrupt a
// candidate (control characters are rejected and strings are single-quoted), so
// the gate is mutation-checked by defeating the serializer's quote rule rather
// than by constructing a hostile Value here. The adversarial-bytes half of this
// invariant belongs to the batch-patch fuzz target.
func TestReparseGateCatchesACorruptingEdit(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\n")
	var p PatchSet
	p.SetField("id", Int(2))
	out, err := d.Apply(p)
	if err != nil || out == nil {
		t.Fatalf("control run failed: %v", err)
	}
}

func TestInsertFieldLandsBeforeClosingFence(t *testing.T) {
	got := applyOne(t, "---\nid: 1\n---\nbody\n", func(p *PatchSet) {
		p.InsertField("claimed_at", String("2026-08-13T17:51:28Z"))
	})
	want := "---\nid: 1\nclaimed_at: '2026-08-13T17:51:28Z'\n---\nbody\n"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestInsertFieldUsesDocumentLineEnding(t *testing.T) {
	got := applyOne(t, "---\r\nid: 1\r\n---\r\n", func(p *PatchSet) {
		p.InsertField("pr", Int(7))
	})
	if !strings.Contains(got, "pr: 7\r\n") {
		t.Fatalf("inserted line must use CRLF here: %q", got)
	}
}

// TestInsertFieldNullRendersBareKey pins the one asymmetry the shared
// serializer forces on insertion: a null value has no token at all, so the
// inserted line must not carry the trailing space that every other value gets.
func TestInsertFieldNullRendersBareKey(t *testing.T) {
	got := applyOne(t, "---\nid: 1\n---\n", func(p *PatchSet) { p.InsertField("pr", Null()) })
	if got != "---\nid: 1\npr:\n---\n" {
		t.Fatalf("got %q", got)
	}
}

func TestInsertExistingFieldRejected(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\n")
	var p PatchSet
	p.InsertField("id", Int(2))
	if _, err := d.Apply(p); !IsKind(err, KindDuplicateEdit) {
		t.Fatalf("inserting a present field: got %v", err)
	}
}

func TestSetAndInsertSameNameRejected(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\n")
	var p PatchSet
	p.SetField("id", Int(2))
	p.InsertField("id", Int(3))
	if _, err := d.Apply(p); !IsKind(err, KindDuplicateEdit) {
		t.Fatalf("got %v", err)
	}
}

func TestInsertFieldOnNoFrontmatterDocRejected(t *testing.T) {
	d := mustParse(t, "just body\n")
	var p PatchSet
	p.InsertField("id", Int(1))
	if _, err := d.Apply(p); !IsKind(err, KindMissingFrontmatter) {
		t.Fatalf("got %v", err)
	}
}

// TestTwoFieldInsertionsRefusedAsOverlapping pins a deliberate refusal rather
// than a capability: two insertions competing for the same zero-width point have
// no order this change defines, so they are refused instead of silently ordered.
// A later change that needs a multi-field insert coalesces them into one edit.
func TestTwoFieldInsertionsRefusedAsOverlapping(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\n")
	var p PatchSet
	p.InsertField("pr", Int(7))
	p.InsertField("branch", String("feat/x"))
	if _, err := d.Apply(p); !IsKind(err, KindOverlappingEdit) {
		t.Fatalf("got %v", err)
	}
}

func TestReplaceBlockPreservesMarkerLines(t *testing.T) {
	src := "---\nid: 1\n---\n\n" + artifactsBlock + "\n## Why\n"
	got := applyOne(t, src, func(p *PatchSet) {
		p.ReplaceBlock("artifacts", "| new | row |")
	})
	want := "---\nid: 1\n---\n\n<!-- docket:artifacts:start (generated — do not hand-edit) -->\n| new | row |\n<!-- docket:artifacts:end -->\n\n## Why\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestReplaceBlockEmitsBlockLineEnding(t *testing.T) {
	src := "<!-- docket:backlink:start -->\r\nold\r\n<!-- docket:backlink:end -->\r\n"
	got := applyOne(t, src, func(p *PatchSet) {
		p.ReplaceBlock("backlink", "line one\nline two")
	})
	if !strings.Contains(got, "line one\r\nline two\r\n") {
		t.Fatalf("logical LF content must be emitted with the block's CRLF: %q", got)
	}
}

// TestReplaceBlockAcceptsATrailingNewline checks the two spellings of the same
// logical content agree: callers that terminate their last line and callers that
// do not must produce identical bytes, or block patches stop being idempotent
// across the two rendering styles.
func TestReplaceBlockAcceptsATrailingNewline(t *testing.T) {
	src := "<!-- docket:x:start -->\nold\n<!-- docket:x:end -->\n"
	bare := applyOne(t, src, func(p *PatchSet) { p.ReplaceBlock("x", "a\nb") })
	terminated := applyOne(t, src, func(p *PatchSet) { p.ReplaceBlock("x", "a\nb\n") })
	if bare != terminated {
		t.Fatalf("trailing-newline spelling changed the result:\n%q\n%q", bare, terminated)
	}
	if !strings.Contains(bare, "start -->\na\nb\n<!-- docket:x:end") {
		t.Fatalf("got %q", bare)
	}
}

func TestReplaceBlockWithEmptyContentEmptiesInterior(t *testing.T) {
	got := applyOne(t, "<!-- docket:x:start -->\nold\n<!-- docket:x:end -->\n",
		func(p *PatchSet) { p.ReplaceBlock("x", "") })
	if got != "<!-- docket:x:start -->\n<!-- docket:x:end -->\n" {
		t.Fatalf("got %q", got)
	}
}

func TestReplaceMissingBlockRejected(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\n")
	var p PatchSet
	p.ReplaceBlock("artifacts", "x")
	if _, err := d.Apply(p); !IsKind(err, KindMissingPatchTarget) {
		t.Fatalf("got %v", err)
	}
}

func TestReplaceBlockRejectsInvalidContent(t *testing.T) {
	d := mustParse(t, "<!-- docket:x:start -->\nold\n<!-- docket:x:end -->\n")
	for _, bad := range []string{"has\x00nul", "carriage\r\nreturn"} {
		var p PatchSet
		p.ReplaceBlock("x", bad)
		if _, err := d.Apply(p); !IsKind(err, KindInvalidValue) {
			t.Errorf("ReplaceBlock(%q): got %v", bad, err)
		}
	}
}

func TestDuplicateBlockEditRejected(t *testing.T) {
	d := mustParse(t, "<!-- docket:x:start -->\nold\n<!-- docket:x:end -->\n")
	var p PatchSet
	p.ReplaceBlock("x", "a")
	p.ReplaceBlock("x", "b")
	if _, err := d.Apply(p); !IsKind(err, KindDuplicateEdit) {
		t.Fatalf("got %v", err)
	}
}

// TestBlockAndFieldNamespacesAreSeparate checks duplicate detection keys on the
// (kind, name) pair: a block and a field may legitimately share a name.
func TestBlockAndFieldNamespacesAreSeparate(t *testing.T) {
	src := "---\nartifacts: []\n---\n\n" + artifactsBlock
	got := applyOne(t, src, func(p *PatchSet) {
		p.SetField("artifacts", Seq(Int(1)))
		p.ReplaceBlock("artifacts", "| row |")
	})
	for _, want := range []string{"artifacts: [1]", "\n| row |\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%q", want, got)
		}
	}
}

func TestInsertBlockAtDocumentStart(t *testing.T) {
	got := applyOne(t, "# Spec\n\nprose\n", func(p *PatchSet) {
		p.InsertBlock("backlink", "generated — do not hand-edit", "> home", AtDocumentStart)
	})
	want := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> home\n" +
		"<!-- docket:backlink:end -->\n# Spec\n\nprose\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestInsertBlockWithoutAnnotation(t *testing.T) {
	got := applyOne(t, "prose\n", func(p *PatchSet) {
		p.InsertBlock("backlink", "", "> home", AtDocumentStart)
	})
	if !strings.HasPrefix(got, "<!-- docket:backlink:start -->\n") {
		t.Fatalf("an empty annotation must not render empty parentheses: %q", got)
	}
}

func TestInsertBlockAfterFrontmatter(t *testing.T) {
	got := applyOne(t, "---\nid: 1\n---\nbody\n", func(p *PatchSet) {
		p.InsertBlock("backlink", "", "> home", AfterFrontmatter)
	})
	want := "---\nid: 1\n---\n<!-- docket:backlink:start -->\n> home\n<!-- docket:backlink:end -->\nbody\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestInsertBlockUsesDocumentLineEnding(t *testing.T) {
	got := applyOne(t, "---\r\nid: 1\r\n---\r\nbody\r\n", func(p *PatchSet) {
		p.InsertBlock("backlink", "", "> home", AfterFrontmatter)
	})
	if !strings.Contains(got, "<!-- docket:backlink:start -->\r\n> home\r\n<!-- docket:backlink:end -->\r\n") {
		t.Fatalf("inserted block must use the document's CRLF: %q", got)
	}
}

// TestInsertBlockAtDocumentStartRefusedOnAFrontmatterDocument guards a
// corruption the reparse gate cannot see: bytes in front of the opening fence
// demote real frontmatter to ordinary prose, and the result still parses — as a
// different, frontmatterless document.
func TestInsertBlockAtDocumentStartRefusedOnAFrontmatterDocument(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\nbody\n")
	var p PatchSet
	p.InsertBlock("backlink", "", "> home", AtDocumentStart)
	if _, err := d.Apply(p); !IsKind(err, KindUnsupportedPatchShape) {
		t.Fatalf("got %v", err)
	}
}

func TestInsertBlockAfterFrontmatterRequiresFrontmatter(t *testing.T) {
	d := mustParse(t, "prose\n")
	var p PatchSet
	p.InsertBlock("backlink", "", "> home", AfterFrontmatter)
	if _, err := d.Apply(p); !IsKind(err, KindMissingFrontmatter) {
		t.Fatalf("got %v", err)
	}
}

func TestInsertPresentBlockRejected(t *testing.T) {
	d := mustParse(t, artifactsBlock)
	var p PatchSet
	p.InsertBlock("artifacts", "", "x", AtDocumentStart)
	if _, err := d.Apply(p); !IsKind(err, KindDuplicateEdit) {
		t.Fatalf("got %v", err)
	}
}

func TestInsertBlockRejectsBadNameOrAnnotation(t *testing.T) {
	d := mustParse(t, "prose\n")
	for _, c := range []struct{ name, annotation string }{
		{"BadName", ""},
		{"with_underscore", ""},
		{"", ""},
		{"ok", "has ) paren"},
		{"ok", "has\nnewline"},
	} {
		var p PatchSet
		p.InsertBlock(c.name, c.annotation, "x", AtDocumentStart)
		if _, err := d.Apply(p); !IsKind(err, KindInvalidValue) {
			t.Errorf("InsertBlock(%q, %q): got %v", c.name, c.annotation, err)
		}
	}
}

// TestInsertBlockContentCarryingMarkersTripsReparseGate is the reachable half of
// the phase-3 gate: block content is free-form Markdown, so unlike a typed field
// value it CAN carry marker-shaped lines, and only the candidate reparse catches
// the imbalance they create.
func TestInsertBlockContentCarryingMarkersTripsReparseGate(t *testing.T) {
	d := mustParse(t, "prose\n")
	var p PatchSet
	p.InsertBlock("outer", "", "<!-- docket:inner:start -->", AtDocumentStart)
	if _, err := d.Apply(p); !IsKind(err, KindReparseFailed) {
		t.Fatalf("got %v", err)
	}
}

func TestBlockPatchesAreIdempotent(t *testing.T) {
	src := "---\nid: 1\n---\n\n" + artifactsBlock
	d := mustParse(t, src)
	var p PatchSet
	p.ReplaceBlock("artifacts", "| new |")
	once, err := d.Apply(p)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := Parse(once)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := d2.Apply(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(once) != string(twice) {
		t.Fatalf("re-applying the same block patch must be byte-idempotent:\n%q\n%q", once, twice)
	}
}

// TestBatchMixesFieldAndBlockOps exercises the whole plan through one Apply: a
// field set, an absent-field insert, a block replace, and a block insert all
// land, and the splice order does not disturb any of them.
func TestBatchMixesFieldAndBlockOps(t *testing.T) {
	src := "---\nid: 1\nstatus: proposed\n---\n\n" + artifactsBlock
	got := applyOne(t, src, func(p *PatchSet) {
		p.SetField("status", String("done"))
		p.InsertField("pr", Int(7))
		p.ReplaceBlock("artifacts", "| new |")
		p.InsertBlock("backlink", "generated", "> home", AfterFrontmatter)
	})
	want := "---\nid: 1\nstatus: 'done'\npr: 7\n---\n" +
		"<!-- docket:backlink:start (generated) -->\n> home\n<!-- docket:backlink:end -->\n" +
		"\n<!-- docket:artifacts:start (generated — do not hand-edit) -->\n| new |\n" +
		"<!-- docket:artifacts:end -->\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// TestReparseGateRejectsACorruptingPayload drives the gate directly through the
// internal edit list: a payload the public constructors cannot produce proves
// the candidate reparse, not the value validator, is what refuses it.
func TestReparseGateRejectsACorruptingPayload(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\nbody\n")
	f, ok := d.Field("id")
	if !ok {
		t.Fatal("fixture field missing")
	}
	out, err := d.applyResolved([]resolvedEdit{{span: f.Value, payload: []byte("2\nid: 1")}})
	if !IsKind(err, KindReparseFailed) {
		t.Fatalf("hand-built corrupting payload must trip the reparse gate, got %v", err)
	}
	if out != nil {
		t.Fatal("a failed reparse must return nil bytes")
	}
}
