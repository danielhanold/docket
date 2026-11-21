package pdfrender

import (
	"math"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// style is how a run of inline text is drawn.
type style struct {
	bold, italic, mono, strike, tint bool
	size                             float64
}

// piece is text drawn in one face and style; w is its width in points, filled
// in by the layout engine.
type piece struct {
	text string
	f    face
	st   style
	w    float64
}

// primary is the face the style draws its text in. Mono has no italic face,
// so mono italic draws in mono regular (or mono bold).
func (st style) primary() face {
	switch {
	case st.mono && st.bold:
		return faceMonoBold
	case st.mono:
		return faceMono
	case st.bold && st.italic:
		return faceSansBoldOblique
	case st.bold:
		return faceSansBold
	case st.italic:
		return faceSansOblique
	}
	return faceSans
}

// sans is the style's upright DejaVu Sans face, which draws Placeholder and
// backs up the mono faces.
func (st style) sans() face {
	if st.bold {
		return faceSansBold
	}
	return faceSans
}

// dropped reports whether r is an invisible that is removed silently and not
// counted: variation selectors, zero-width joiners and spaces, the word
// joiner, the byte-order mark, tag characters, and control characters other
// than tab and newline.
func dropped(r rune) bool {
	switch {
	case r == '\t' || r == '\n':
		return false
	case r < 0x20, r == 0x7F, r >= 0x80 && r <= 0x9F:
		return true
	case r >= 0xFE00 && r <= 0xFE0F, r >= 0x200B && r <= 0x200D, r == 0x2060, r == 0xFEFF:
		return true
	case r >= 0xE0020 && r <= 0xE007F, r >= 0xE0100 && r <= 0xE01EF:
		return true
	}
	return false
}

// faceFor picks the first face that can draw r in st: the style's primary
// face, then (for mono) the matching DejaVu Sans face, then Noto Emoji.
// Every face it returns covers r, and covers is false above U+FFFF, so no
// rune above U+FFFF is ever given a face.
func faceFor(r rune, st style) (face, bool) {
	if f := st.primary(); f.covers(r) {
		return f, true
	}
	if st.mono {
		if f := st.sans(); f.covers(r) {
			return f, true
		}
	}
	if faceEmoji.covers(r) {
		return faceEmoji, true
	}
	return 0, false
}

// resolve screens s for drawing in st and splits it into pieces, one per run
// of adjacent runes drawn in the same face. Invisibles (see dropped) vanish.
// Tab and newline become a space: no face draws them, and callers that need
// tabs expanded do it first. Any other rune no face can draw becomes
// Placeholder, drawn in the style's sans face, and is counted. Invalid UTF-8
// decodes to U+FFFD and is resolved like any other rune.
//
// This is the only path text takes to the canvas, so every piece is valid
// UTF-8 holding only BMP runes that its face covers.
func resolve(s string, st style) (pieces []piece, placeholders int) {
	var b strings.Builder
	cur := face(-1)
	flush := func() {
		if b.Len() > 0 {
			pieces = append(pieces, piece{text: b.String(), f: cur, st: st})
			b.Reset()
		}
	}
	for _, r := range s {
		if dropped(r) {
			continue
		}
		if r == '\t' || r == '\n' {
			r = ' '
		}
		f, ok := faceFor(r, st)
		if !ok {
			f, r = st.sans(), Placeholder
			placeholders++
		}
		if f != cur {
			flush()
			cur = f
		}
		b.WriteRune(r)
	}
	flush()
	return pieces, placeholders
}

type advanceKey struct {
	f face
	r rune
}

// advances caches advance widths in thousandths of an em, keyed by face and
// rune. It is shared by every render and safe for concurrent use.
var advances sync.Map

// advance is r's advance width in f, in thousandths of an em, rounded the way
// fpdf rounds its own width table, so measure matches what fpdf draws.
func (f face) advance(r rune) int {
	key := advanceKey{f, r}
	if v, ok := advances.Load(key); ok {
		return v.(int)
	}
	w := 0
	if fnt, err := parsedFont(f); err == nil {
		var buf sfnt.Buffer
		upem := fnt.UnitsPerEm()
		g, err := fnt.GlyphIndex(&buf, r)
		if err == nil {
			adv, err := fnt.GlyphAdvance(&buf, g, fixed.Int26_6(upem)<<6, font.HintingNone)
			if err == nil {
				w = int(math.Round(1000 * float64(adv) / 64 / float64(upem)))
			}
		}
	}
	advances.Store(key, w)
	return w
}
