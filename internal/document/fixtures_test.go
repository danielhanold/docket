package document

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// frozenDir holds byte-exact real records captured at pinned commits. They are
// historical snapshots and deliberately do NOT track their live originals; see
// testdata/repositories/v0.9.2/PROVENANCE.md for the pins and testdata/README.md
// for that immutability statement.
const frozenDir = "../../testdata/repositories/v0.9.2/documents"

func frozenCorpus(t *testing.T) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(frozenDir)
	if err != nil {
		t.Fatalf("frozen corpus missing: %v", err)
	}
	out := map[string][]byte{}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(frozenDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = b
	}
	if len(out) < 5 {
		t.Fatalf("corpus has %d files, want the 5 captured records", len(out))
	}
	return out
}

// commonPrefix returns the number of leading bytes a and b share.
func commonPrefix(a, b []byte) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// commonSuffix returns the number of trailing bytes a and b share, bounded so
// it can never overlap the common prefix.
func commonSuffix(a, b []byte) int {
	limit := len(a) - commonPrefix(a, b)
	if other := len(b) - commonPrefix(a, b); other < limit {
		limit = other
	}
	n := 0
	for n < limit && a[len(a)-1-n] == b[len(b)-1-n] {
		n++
	}
	return n
}

func TestFrozenCorpusParsesAndDecodesWithoutNormalizing(t *testing.T) {
	for name, src := range frozenCorpus(t) {
		d, err := Parse(src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := d.Source(); !bytes.Equal(got, src) {
			t.Fatalf("%s: Source() normalized bytes", name)
		}
		if d.HasFrontmatter() {
			var meta struct {
				ID   int    `yaml:"id"`
				Slug string `yaml:"slug"`
			}
			if err := d.DecodeFrontmatter(&meta); err != nil {
				t.Fatalf("%s: decode: %v", name, err)
			}
		}
	}
}

func TestEmptyPatchSetByteIdenticalAcrossCorpus(t *testing.T) {
	for name, src := range frozenCorpus(t) {
		d, err := Parse(src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		out, err := d.Apply(PatchSet{})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(out, src) {
			t.Fatalf("%s: empty PatchSet must be byte-identical", name)
		}
	}
}

func TestSingleFieldPatchChangesOnlyDeclaredSpan(t *testing.T) {
	src := frozenCorpus(t)["active-change-0158.md"]
	d, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	f, ok := d.Field("status")
	if !ok {
		t.Fatal("fixture must carry status:")
	}
	var p PatchSet
	p.SetField("status", String("in-progress"))
	out, err := d.Apply(p)
	if err != nil {
		t.Fatal(err)
	}
	pre, suf := commonPrefix(src, out), commonSuffix(src, out)
	// Every differing byte must sit inside the declared field spans.
	if pre < f.Value.Start || len(src)-suf > f.Entry.End {
		t.Fatalf("difference [%d, %d) escapes the declared field spans [%d, %d)",
			pre, len(out)-suf, f.Value.Start, f.Entry.End)
	}
}

func TestBatchPatchOnFrozenActiveChange(t *testing.T) {
	src := frozenCorpus(t)["active-change-0158.md"]
	d, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	fs, ok := d.Field("status")
	if !ok {
		t.Fatal("fixture must carry status:")
	}
	fb, ok := d.Field("branch")
	if !ok {
		t.Fatal("fixture must carry branch:")
	}
	var p PatchSet
	p.SetField("status", String("in-progress"))
	p.SetField("branch", String("feat/batch-mode"))
	out, err := d.Apply(p)
	if err != nil {
		t.Fatal(err)
	}
	// The changed-range oracle binds a batch exactly as it binds one edit:
	// every differing byte sits inside the hull of the two declared entry
	// spans. A hand-listed set of surviving substrings is not that oracle.
	lo, hi := fs.Entry.Start, fb.Entry.End
	if fb.Entry.Start < lo {
		lo = fb.Entry.Start
	}
	if fs.Entry.End > hi {
		hi = fs.Entry.End
	}
	pre, suf := commonPrefix(src, out), commonSuffix(src, out)
	if pre < lo || len(src)-suf > hi {
		t.Fatalf("difference [%d, %d) escapes the declared entry spans [%d, %d)",
			pre, len(src)-suf, lo, hi)
	}
	rt, err := Parse(out)
	if err != nil {
		t.Fatalf("patched frozen record must reparse: %v", err)
	}
	var meta struct {
		Status string `yaml:"status"`
		Branch string `yaml:"branch"`
	}
	if err := rt.DecodeFrontmatter(&meta); err != nil {
		t.Fatal(err)
	}
	if meta.Status != "in-progress" || meta.Branch != "feat/batch-mode" {
		t.Fatalf("%+v", meta)
	}
	// Unknown-to-this-struct fields, comments, blocks, body survive.
	for _, invariant := range []string{"docket:artifacts:start", "## Why"} {
		if !strings.Contains(string(out), invariant) {
			t.Fatalf("lost %q", invariant)
		}
	}
}

func TestBlockReplaceOnFrozenPlanBacklink(t *testing.T) {
	src := frozenCorpus(t)["plan-with-backlink.md"]
	d, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := d.Block("backlink")
	if !ok {
		t.Fatal("plan fixture must carry its backlink block")
	}
	var p PatchSet
	p.ReplaceBlock("backlink", "> ↩ re-rendered home link")
	out, err := d.Apply(p)
	if err != nil {
		t.Fatal(err)
	}
	// Marker lines and everything outside the interior are untouched.
	if !bytes.Equal(out[:b.Interior.Start], src[:b.Interior.Start]) {
		t.Fatal("bytes before the block interior changed")
	}
	tailSrc, tailOut := src[b.Interior.End:], out[len(out)-(len(src)-b.Interior.End):]
	if !bytes.Equal(tailSrc, tailOut) {
		t.Fatal("bytes after the block interior changed")
	}
}

func TestPackageLocalFixturesParseByNamingRule(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".golden.md") {
			continue
		}
		src, err := os.ReadFile(filepath.Join("testdata", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		_, perr := Parse(src)
		if strings.HasPrefix(e.Name(), "invalid-") {
			if perr == nil {
				t.Errorf("%s: want Parse failure", e.Name())
			}
		} else if perr != nil {
			t.Errorf("%s: %v", e.Name(), perr)
		}
	}
}

func TestRefusalsReturnNoBytesAcrossCorpus(t *testing.T) {
	for name, src := range frozenCorpus(t) {
		d, err := Parse(src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !d.HasFrontmatter() {
			continue
		}
		var p PatchSet
		p.SetField("field_that_never_exists_anywhere", Int(1))
		out, err := d.Apply(p)
		if err == nil || out != nil {
			t.Errorf("%s: refusal must return (nil, error); got (%v, %v)", name, out, err)
		}
	}
}
