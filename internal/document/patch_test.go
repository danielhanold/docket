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
