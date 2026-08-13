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

// markerAdversarialSeeds are hand-written body slices aimed at the marker
// surface specifically: balance, nesting, fence masking, annotation edges, line
// terminators, and marker-shaped prose. They are seeds, not assertions — each
// one either parses (and is then held to the structural properties below) or is
// a legal refusal.
var markerAdversarialSeeds = []string{
	"<!-- docket:a:start -->\nbody\n<!-- docket:a:end -->\n",
	"<!-- docket:a:start -->\nbody\n<!-- docket:a:end -->", // unterminated last line
	"<!-- docket:a:start -->\n<!-- docket:a:end -->\n",     // empty interior
	"<!-- docket:a:start -->\r\nbody\r\n<!-- docket:a:end -->\r\n",
	"<!-- docket:a:start (annotated) -->\nx\n<!-- docket:a:end -->\n",
	"<!-- docket:a:start () -->\nx\n<!-- docket:a:end -->\n", // empty annotation
	"<!-- docket:a:start -->\n<!-- docket:a:end -->\n<!-- docket:b:start -->\n<!-- docket:b:end -->\n",
	"<!-- docket:a:start -->\n<!-- docket:b:start -->\n<!-- docket:b:end -->\n<!-- docket:a:end -->\n", // nesting
	"<!-- docket:a:start -->\n",                                                                        // dangling start
	"<!-- docket:a:end -->\n",                                                                          // unmatched end
	"<!-- docket:a:end (why) -->\n",                                                                    // annotation on an end marker
	"<!-- docket:a:start -->\nx\n<!-- docket:b:end -->\n",                                              // crossed names
	"<!-- docket:a:start -->\nx\n<!-- docket:a:end -->\n<!-- docket:a:start -->\ny\n<!-- docket:a:end -->\n",
	"```\n<!-- docket:a:start -->\n```\n",        // fence-masked marker
	"~~~info\n<!-- docket:a:start -->\n~~~\n",    // tilde fence with info string
	"````\n```\n<!-- docket:a:start -->\n````\n", // longer fence run
	"```\n<!-- docket:a:start -->\n",             // unclosed fence swallows the marker
	"   <!-- docket:a:start -->\n",               // indented: prose, not a marker
	"<!-- docket:A:start -->\n",                  // malformed name
	"<!-- docket:a:start -->trailing\n",          // malformed tail
	"<!-- docket: -->\n",                         // prefix without a grammar match
	"<!-- docket:a:start (nested (paren)) -->\nx\n<!-- docket:a:end -->\n",
	"---\nid: 1\n---\n<!-- docket:a:start -->\nx\n<!-- docket:a:end -->\n",
	"---\n<!-- docket:a:start -->\n---\nbody\n", // marker-shaped text inside frontmatter
}

// seedMarkerBodies seeds the marker surface with body-oriented slices: the
// post-frontmatter body of every package-local fixture and every frozen corpus
// file (the frontmatter's bytes are never marker territory, so the body alone
// is the input this surface actually consumes), plus the adversarial fixtures
// above. A fixture that does not parse is seeded whole — its bytes are still
// useful mutation fuel.
func seedMarkerBodies(f *testing.F) {
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
			if d, perr := Parse(b); perr == nil && d.hasFM {
				b = b[d.fmClose.End:]
			}
			f.Add(b)
		}
	}
	for _, s := range markerAdversarialSeeds {
		f.Add([]byte(s))
	}
}

// trimTerminator drops a line's terminator, so what remains is the text
// markerRE is anchored against.
func trimTerminator(line []byte) []byte {
	line = bytes.TrimSuffix(line, []byte("\n"))
	return bytes.TrimSuffix(line, []byte("\r"))
}

// FuzzMarkers — marker discovery, balance, and block geometry over arbitrary
// body bytes. Parse must never panic or loop; and on success the located block
// population must be structurally sound: ascending and non-overlapping in
// source order, each block's interior exactly the bytes between its two marker
// lines and never reaching into either of them, and each marker line's own
// bytes re-matching the marker grammar with the fields the Block reports.
func FuzzMarkers(f *testing.F) {
	seedMarkerBodies(f)
	f.Fuzz(func(t *testing.T, body []byte) {
		d, err := Parse(body)
		if err != nil {
			return
		}
		src := d.Source()
		prevEnd := 0
		for i, b := range d.Blocks() {
			// Every span is in bounds and internally ordered.
			for _, s := range []struct {
				name string
				span Span
			}{{"Start", b.Start}, {"Interior", b.Interior}, {"End", b.End}} {
				if s.span.Start < 0 || s.span.Start > s.span.End || s.span.End > len(src) {
					t.Fatalf("block %q %s span %+v out of bounds for %d source bytes",
						b.Name, s.name, s.span, len(src))
				}
			}
			// Ascending and non-overlapping: this block begins at or after the
			// previous block's end marker finished.
			if b.Start.Start < prevEnd {
				t.Fatalf("block %d (%q) starts at %d, inside the preceding block ending at %d",
					i, b.Name, b.Start.Start, prevEnd)
			}
			prevEnd = b.End.End
			// The interior is exactly the bytes between the marker lines: it
			// never crosses into either marker line, and never leaves a gap.
			if b.Interior.Start != b.Start.End || b.Interior.End != b.End.Start {
				t.Fatalf("block %q interior %+v is not the span between its markers [%d, %d)",
					b.Name, b.Interior, b.Start.End, b.End.Start)
			}
			// Each marker line's own bytes re-match the marker grammar, and
			// agree with what the Block reports about them.
			startText := trimTerminator(src[b.Start.Start:b.Start.End])
			m := markerRE.FindSubmatch(startText)
			if m == nil {
				t.Fatalf("block %q start line %q does not match the marker grammar", b.Name, startText)
			}
			if string(m[1]) != b.Name || string(m[2]) != "start" || string(m[3]) != b.Annotation {
				t.Fatalf("block %q start line %q reports (%q, %q, %q), want (%q, \"start\", %q)",
					b.Name, startText, m[1], m[2], m[3], b.Name, b.Annotation)
			}
			endText := trimTerminator(src[b.End.Start:b.End.End])
			m = markerRE.FindSubmatch(endText)
			if m == nil {
				t.Fatalf("block %q end line %q does not match the marker grammar", b.Name, endText)
			}
			if string(m[1]) != b.Name || string(m[2]) != "end" || string(m[3]) != "" {
				t.Fatalf("block %q end line %q reports (%q, %q, %q), want (%q, \"end\", \"\")",
					b.Name, endText, m[1], m[2], m[3], b.Name)
			}
			// The canonical renderers agree with source whenever the source
			// line is already canonical — the end marker has exactly one
			// spelling, so it always is.
			if got := endMarkerLine(b.Name); got != string(endText) {
				t.Fatalf("endMarkerLine(%q) = %q, but the located line is %q", b.Name, got, endText)
			}
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
		// The only edit is on "status", so its Entry span bounds every byte
		// Apply is permitted to touch — SetField resolves inside the entry
		// line, reaching at most back over the spacing in front of the value.
		before := d.Source()
		target, targeted := d.Field("status")
		var p PatchSet
		p.SetField("status", String(val))
		out, aerr := d.Apply(p)
		if aerr != nil {
			if out != nil {
				t.Fatal("error with non-nil bytes")
			}
			return
		}
		if !targeted {
			t.Fatal("successful SetField on an unlocated field")
		}
		// Byte identity outside the reported span, in both directions.
		if !bytes.Equal(out[:target.Entry.Start], before[:target.Entry.Start]) {
			t.Fatalf("bytes before the edited entry [0, %d) changed", target.Entry.Start)
		}
		tailAt := target.Entry.End + (len(out) - len(before))
		if tailAt < 0 || tailAt > len(out) {
			t.Fatalf("edit resized past its own entry: tail offset %d of %d", tailAt, len(out))
		}
		if !bytes.Equal(out[tailAt:], before[target.Entry.End:]) {
			t.Fatalf("bytes after the edited entry (from %d) changed", target.Entry.End)
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
		// Non-aliasing: every returned buffer is the caller's alone. The two
		// places a returned slice could plausibly share memory with a
		// document's own are the empty set, which splices nothing, and Parse
		// re-reading patched bytes. Done last — both deliberately corrupt the
		// buffer they mutate.
		if idle, ierr := d.Apply(PatchSet{}); ierr == nil && len(idle) > 0 {
			idle[0] ^= 0xFF
			if !bytes.Equal(d.Source(), before) {
				t.Fatal("mutating an empty patch's result reached the document")
			}
		}
		if len(out) > 0 {
			out[0] ^= 0xFF
			if !bytes.Equal(d2.Source(), out2) {
				t.Fatal("mutating Apply's result reached the document reparsed from it")
			}
		}
	})
}
