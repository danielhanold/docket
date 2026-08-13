package document

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// seedAll adds every package-local fixture and every frozen corpus file as a
// seed. frozenDir is the same pinned snapshot directory fixtures_test.go reads.
func seedAll(f *testing.F) {
	f.Helper()
	for _, dir := range []string{"testdata", frozenDir} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			f.Fatalf("seed dir %s: %v", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				f.Fatal(err)
			}
			f.Add(b)
		}
	}
}

// FuzzParse — fence + field-location discovery must never panic or loop, and
// a successful parse must round-trip its bytes and locate self-consistent spans.
func FuzzParse(f *testing.F) {
	seedAll(f)
	f.Fuzz(func(t *testing.T, src []byte) {
		d, err := Parse(src)
		if err != nil {
			return
		}
		if !bytes.Equal(d.Source(), src) {
			t.Fatal("Source() must equal input bytes")
		}
		for _, fld := range d.Fields() {
			if fld.Value.Start < fld.Entry.Start || fld.Value.End > fld.Entry.End ||
				fld.Entry.End > len(src) {
				t.Fatalf("field %s spans out of bounds: %+v", fld.Name, fld)
			}
		}
		for _, b := range d.Blocks() {
			if b.Interior.Start < b.Start.End || b.Interior.End > b.End.Start {
				t.Fatalf("block %s spans inconsistent: %+v", b.Name, b)
			}
		}
	})
}

// FuzzMarkers — marker discovery/balance over arbitrary body bytes.
func FuzzMarkers(f *testing.F) {
	seedAll(f)
	f.Fuzz(func(t *testing.T, body []byte) {
		d, err := Parse(body)
		if err != nil {
			return
		}
		names := map[string]bool{}
		for _, b := range d.Blocks() {
			if names[b.Name] {
				t.Fatalf("duplicate block name %q survived validation", b.Name)
			}
			names[b.Name] = true
		}
	})
}

// FuzzValueRoundTrip — serializer output must decode back to the same value.
func FuzzValueRoundTrip(f *testing.F) {
	f.Add("plain", int64(0), false)
	f.Add("it's # tricky: yes", int64(-7), true)
	f.Add("", int64(9223372036854775807), false)
	f.Fuzz(func(t *testing.T, s string, n int64, b bool) {
		v := String(s)
		if v.validate() != nil {
			return // control chars etc. are legal refusals
		}
		doc, err := New([]FieldSpec{{Name: "s", Value: v}, {Name: "n", Value: Int(n)}, {Name: "b", Value: Bool(b)}}, "x\n")
		if err != nil {
			t.Fatalf("builder refused validated values: %v", err)
		}
		d, err := Parse(doc)
		if err != nil {
			t.Fatalf("canonical output failed reparse: %v", err)
		}
		var out struct {
			S string `yaml:"s"`
			N int64  `yaml:"n"`
			B bool   `yaml:"b"`
		}
		if err := d.DecodeFrontmatter(&out); err != nil {
			t.Fatal(err)
		}
		if out.S != s || out.N != n || out.B != b {
			t.Fatalf("round trip: got %+v want (%q, %d, %v)", out, s, n, b)
		}
	})
}

// FuzzApply — batch patching over corpus documents with fuzzed values.
func FuzzApply(f *testing.F) {
	f.Add([]byte("---\nid: 1\nstatus: proposed\n---\nbody\n"), "done")
	f.Fuzz(func(t *testing.T, src []byte, val string) {
		d, err := Parse(src)
		if err != nil {
			return
		}
		var p PatchSet
		p.SetField("status", String(val))
		out, aerr := d.Apply(p)
		if aerr != nil {
			if out != nil {
				t.Fatal("error with non-nil bytes")
			}
			return
		}
		if _, err := Parse(out); err != nil {
			t.Fatalf("successful patch must reparse: %v", err)
		}
		// Idempotence: applying again yields identical bytes.
		d2, err := Parse(out)
		if err != nil {
			t.Fatalf("reparse of patched bytes refused: %v", err)
		}
		out2, err := d2.Apply(p)
		if err != nil {
			t.Fatalf("idempotent reapply refused: %v", err)
		}
		if !bytes.Equal(out, out2) {
			t.Fatal("reapply not byte-idempotent")
		}
	})
}
