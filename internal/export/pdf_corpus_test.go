package export

import (
	"bytes"
	"path"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/pdfrender"
)

// countAboveBMP counts the runes above U+FFFF in md that the PDF renderer must
// draw as its placeholder: every one except the invisible tag characters and
// supplementary variation selectors, which it drops uncounted. It is written
// apart from the renderer's own screening so the two can disagree.
func countAboveBMP(md []byte) int {
	n := 0
	for _, r := range string(md) {
		switch {
		case r <= 0xFFFF:
		case r >= 0xE0020 && r <= 0xE007F, r >= 0xE0100 && r <= 0xE01EF:
		default:
			n++
		}
	}
	return n
}

// TestCorpusRendersToPDF renders every corpus file's clean copy to PDF. Every
// BMP character in the corpus is one the fonts draw, so the placeholder count
// must equal the characters above U+FFFF exactly: a font-coverage regression
// that turns a covered character into the placeholder reddens it.
func TestCorpusRendersToPDF(t *testing.T) {
	const knownFile = "specs/2026-07-20-frontmatter-field-domain-guard-design.md"
	files := loadCorpus(t)
	total, sawKnown := 0, false
	for _, f := range files {
		t.Run(f.rel, func(t *testing.T) {
			out := Transform(f.in)
			pdf, n, err := pdfrender.Render(out.Markdown, strings.TrimSuffix(path.Base(f.rel), ".md"))
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
				t.Fatalf("output does not start with %%PDF-: %q", pdf[:min(len(pdf), 16)])
			}
			if want := countAboveBMP(out.Markdown); n != want {
				t.Errorf("placeholders = %d, want %d (the characters above U+FFFF)", n, want)
			}
			total += n
			if f.rel == knownFile {
				sawKnown = true
				// Its 🟡 sits in a code span the transform keeps.
				if n != 1 {
					t.Errorf("placeholders = %d, want 1", n)
				}
			}
		})
	}
	if !sawKnown {
		t.Errorf("corpus no longer holds %s", knownFile)
	}
	if total < 1 {
		t.Errorf("the corpus drew %d placeholders in total, want at least 1", total)
	}
}
