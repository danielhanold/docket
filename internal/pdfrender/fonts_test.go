package pdfrender

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"testing"

	"golang.org/x/image/font/sfnt"
)

// TestFontsParse proves every embedded face is a font sfnt can read, so a
// corrupted or truncated file fails here rather than inside a render.
func TestFontsParse(t *testing.T) {
	for _, f := range allFaces {
		if _, err := sfnt.Parse(f.bytes()); err != nil {
			t.Errorf("face %s/%q does not parse: %v", f.family(), f.style(), err)
		}
	}
	if !faceSans.covers('A') {
		t.Errorf("faceSans does not cover 'A'")
	}
}

// TestFaceCoverage pins which face draws which character the corpus uses,
// and that characters above U+FFFF are covered by no face even when a
// font's cmap has them: fpdf cannot embed them, so they must become the
// placeholder.
func TestFaceCoverage(t *testing.T) {
	for _, r := range "—→…§–·×“”↩─│└├═⇒≤≥≠≈∈⊆⊕∧¬①②⟨⟩" {
		if !faceSans.covers(r) {
			t.Errorf("faceSans does not cover %q (U+%04X)", r, r)
		}
	}
	for _, r := range "✅❌⏳⚠⛔⚪⭐⏺" {
		if !faceEmoji.covers(r) {
			t.Errorf("faceEmoji does not cover %q (U+%04X)", r, r)
		}
	}
	for _, r := range []rune{Placeholder, '☐', '☑'} {
		for _, f := range []face{faceSans, faceSansBold} {
			if !f.covers(r) {
				t.Errorf("face %s/%q does not cover %q (U+%04X)", f.family(), f.style(), r, r)
			}
		}
	}
	for _, r := range []rune{'🟡', '🗑', '🟢', '中'} {
		for _, f := range allFaces {
			if f.covers(r) {
				t.Errorf("face %s/%q covers %q (U+%04X); want no face to", f.family(), f.style(), r, r)
			}
		}
	}
}

var shaRow = regexp.MustCompile("^\\|\\s*`?([^|`]+?)`?\\s*\\|\\s*`?([0-9a-f]{64})`?\\s*\\|\\s*$")

// TestFontProvenance checks the fonts README's sha256 table against the
// embedded files in both directions: every embedded file is listed with its
// true digest, and every listed file is embedded.
func TestFontProvenance(t *testing.T) {
	readme, err := os.ReadFile("fonts/README.md")
	if err != nil {
		t.Fatalf("reading fonts/README.md: %v", err)
	}
	listed := map[string]string{}
	for _, line := range strings.Split(string(readme), "\n") {
		if m := shaRow.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			if _, dup := listed[m[1]]; dup {
				t.Errorf("README lists %s twice", m[1])
			}
			listed[m[1]] = m[2]
		}
	}
	if len(listed) == 0 {
		t.Fatalf("fonts/README.md has no | file | sha256 | rows")
	}

	embedded := map[string]bool{}
	err = fs.WalkDir(fontFS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := path.Base(p)
		embedded[name] = true
		data, err := fontFS.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		want, ok := listed[name]
		switch {
		case !ok:
			t.Errorf("embedded file %s is not listed in fonts/README.md", name)
		case want != hex.EncodeToString(sum[:]):
			t.Errorf("%s: README sha256 %s, embedded file has %s", name, want, hex.EncodeToString(sum[:]))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking embedded fonts: %v", err)
	}

	var missing []string
	for name := range listed {
		if !embedded[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	for _, name := range missing {
		t.Errorf("fonts/README.md lists %s, which is not embedded", name)
	}
}
