package document

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNewRendersEveryValueKind(t *testing.T) {
	got, err := New([]FieldSpec{
		{"id", Int(306)},
		{"slug", String("loss-preserving-document-layer")},
		{"title", String("It's a 'title': tricky # yes")},
		{"trivial", Bool(false)},
		{"depends_on", Seq(Int(304))},
		{"adrs", Seq()},
		{"pr", Null()},
	}, "## Why\n\nBecause.\n")
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "new-doc-all-kinds.golden.md")
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestNewOutputReparsesAndDecodes(t *testing.T) {
	got, err := New([]FieldSpec{{"id", Int(1)}, {"note", String("it's")}}, "body\n")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Parse(got)
	if err != nil {
		t.Fatalf("builder output must reparse: %v", err)
	}
	var out struct {
		ID   int    `yaml:"id"`
		Note string `yaml:"note"`
	}
	if err := d.DecodeFrontmatter(&out); err != nil {
		t.Fatal(err)
	}
	if out.ID != 1 || out.Note != "it's" {
		t.Fatalf("%+v", out)
	}
}

func TestNewRejectsDuplicateKeys(t *testing.T) {
	_, err := New([]FieldSpec{{"id", Int(1)}, {"id", Int(2)}}, "")
	if !IsKind(err, KindDuplicateField) {
		t.Fatalf("got %v", err)
	}
}

func TestNewRejectsBadKeyOrValueBeforeRenderingAnything(t *testing.T) {
	_, err := New([]FieldSpec{{"id", Int(1)}, {"Bad-Key", Int(2)}}, "")
	if !IsKind(err, KindInvalidValue) {
		t.Fatalf("got %v", err)
	}
	_, err = New([]FieldSpec{{"a", Int(1)}, {"b", String("x\x00")}}, "")
	if !IsKind(err, KindInvalidValue) {
		t.Fatalf("got %v", err)
	}
}

func TestNewNormalizesFinalNewline(t *testing.T) {
	for _, body := range []string{"body", "body\n", "body\n\n\n"} {
		got, err := New([]FieldSpec{{"id", Int(1)}}, body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasSuffix(got, []byte("body\n")) || bytes.HasSuffix(got, []byte("\n\n")) {
			t.Fatalf("body %q → %q; want exactly one final newline", body, got)
		}
	}
}

func TestNewRequiresAtLeastOneField(t *testing.T) {
	if _, err := New(nil, "body\n"); !IsKind(err, KindInvalidValue) {
		t.Fatalf("a frontmatter builder with zero fields: got %v", err)
	}
}

func TestNewRejectsCRLFBody(t *testing.T) {
	if _, err := New([]FieldSpec{{"id", Int(1)}}, "a\r\nb\n"); !IsKind(err, KindInvalidValue) {
		t.Fatalf("canonical documents are LF-only: got %v", err)
	}
}

func TestNewWithEmptyBodyReparses(t *testing.T) {
	got, err := New([]FieldSpec{{"id", Int(1)}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(got, []byte("---\n")) {
		t.Fatalf("empty body → %q; want fences plus exactly one final newline", got)
	}
	if _, err := Parse(got); err != nil {
		t.Fatalf("empty-body output must reparse: %v", err)
	}
}
