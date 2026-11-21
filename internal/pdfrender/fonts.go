package pdfrender

import (
	"embed"
	"fmt"
	"sync"

	"golang.org/x/image/font/sfnt"
)

// Placeholder is drawn in place of any character no embedded font can draw,
// including every character above U+FFFF.
const Placeholder = '□'

// fontFS holds the embedded fonts and their licence texts; fonts/README.md
// records where each file came from and its SHA-256.
//
//go:embed fonts/*.ttf fonts/*.txt
var fontFS embed.FS

// face is one embedded font file.
type face int

const (
	faceSans face = iota
	faceSansBold
	faceSansOblique
	faceSansBoldOblique
	faceMono
	faceMonoBold
	faceEmoji
)

// allFaces lists every face, in declaration order.
var allFaces = []face{faceSans, faceSansBold, faceSansOblique, faceSansBoldOblique, faceMono, faceMonoBold, faceEmoji}

type faceInfo struct {
	file, family, style string
}

var faceInfos = [...]faceInfo{
	faceSans:            {"DejaVuSans.ttf", "dejavu", ""},
	faceSansBold:        {"DejaVuSans-Bold.ttf", "dejavu", "B"},
	faceSansOblique:     {"DejaVuSans-Oblique.ttf", "dejavu", "I"},
	faceSansBoldOblique: {"DejaVuSans-BoldOblique.ttf", "dejavu", "BI"},
	faceMono:            {"DejaVuSansMono.ttf", "dejavumono", ""},
	faceMonoBold:        {"DejaVuSansMono-Bold.ttf", "dejavumono", "B"},
	faceEmoji:           {"NotoEmoji-Regular.ttf", "notoemoji", ""},
}

// family is the fpdf font family name of the face.
func (f face) family() string { return faceInfos[f].family }

// style is the fpdf style string of the face ("", "B", "I", or "BI").
func (f face) style() string { return faceInfos[f].style }

// bytes returns the embedded TTF of the face.
func (f face) bytes() []byte {
	b, err := fontFS.ReadFile("fonts/" + faceInfos[f].file)
	if err != nil {
		panic(fmt.Sprintf("pdfrender: embedded font %s missing: %v", faceInfos[f].file, err))
	}
	return b
}

var (
	parseOnce   sync.Once
	parsedFaces [len(faceInfos)]*sfnt.Font
	parseErr    error
)

func parsedFont(f face) (*sfnt.Font, error) {
	parseOnce.Do(func() {
		for _, g := range allFaces {
			font, err := sfnt.Parse(g.bytes())
			if err != nil {
				parseErr = fmt.Errorf("pdfrender: parsing %s: %w", faceInfos[g].file, err)
				return
			}
			parsedFaces[g] = font
		}
	})
	return parsedFaces[f], parseErr
}

// covers reports whether the face can draw r: r is in the Basic Multilingual
// Plane and the face's cmap maps it to a real glyph. Characters above U+FFFF
// are never covered, even when the cmap has them, because fpdf cannot embed
// them. Safe for concurrent use.
func (f face) covers(r rune) bool {
	if r < 0 || r > 0xFFFF {
		return false
	}
	font, err := parsedFont(f)
	if err != nil {
		return false
	}
	var buf sfnt.Buffer
	g, err := font.GlyphIndex(&buf, r)
	return err == nil && g != 0
}
