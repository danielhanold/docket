package pdfrender

import (
	"time"

	"codeberg.org/go-pdf/fpdf"
)

// canvas is what the layout engine draws on. Coordinates are in points with
// the origin at the top left of the page; y is the top of the line box, not
// the baseline. Tests substitute a recorder that wraps the real canvas.
type canvas interface {
	measure(f face, size float64, s string) float64
	text(x, y float64, f face, size float64, s string)
	fillRect(x, y, w, h float64, gray int)
	line(x1, y1, x2, y2, width float64, gray int)
	newPage()
	pageNo() int
}

// fixedDate is the creation and modification date of every PDF, so the same
// input always gives the same bytes.
var fixedDate = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// baselineFactor places the baseline inside a line box of height
// size*lineFactor: half the leading above the em box, then the ascent.
const baselineFactor = (lineFactor-1)/2 + 0.8

// fpdfCanvas draws on a US Letter fpdf document measured in points, with
// every face registered and automatic page breaks off: the layout engine
// breaks pages itself.
type fpdfCanvas struct {
	pdf     *fpdf.Fpdf
	curFace face
	curSize float64
	fontSet bool
}

func newFpdfCanvas() *fpdfCanvas {
	pdf := fpdf.New("P", "pt", "Letter", "")
	// Set per instance, never through fpdf's package-level defaults, so
	// concurrent renders share no state.
	pdf.SetCreationDate(fixedDate)
	pdf.SetModificationDate(fixedDate)
	pdf.SetCatalogSort(true)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetMargins(0, 0, 0)
	for _, f := range allFaces {
		pdf.AddUTF8FontFromBytes(f.family(), f.style(), f.bytes())
	}
	return &fpdfCanvas{pdf: pdf}
}

// measure is the width of s in f at size, from the same rounded advance
// widths fpdf uses, without selecting a font on the document.
func (c *fpdfCanvas) measure(f face, size float64, s string) float64 {
	total := 0
	for _, r := range s {
		total += f.advance(r)
	}
	return float64(total) * size / 1000
}

func (c *fpdfCanvas) setFont(f face, size float64) {
	if c.fontSet && c.curFace == f && c.curSize == size {
		return
	}
	c.pdf.SetFont(f.family(), f.style(), size)
	c.curFace, c.curSize, c.fontSet = f, size, true
}

func (c *fpdfCanvas) text(x, y float64, f face, size float64, s string) {
	c.setFont(f, size)
	c.pdf.Text(x, y+size*baselineFactor, s)
}

func (c *fpdfCanvas) fillRect(x, y, w, h float64, gray int) {
	c.pdf.SetFillColor(gray, gray, gray)
	c.pdf.Rect(x, y, w, h, "F")
}

func (c *fpdfCanvas) line(x1, y1, x2, y2, width float64, gray int) {
	c.pdf.SetLineWidth(width)
	c.pdf.SetDrawColor(gray, gray, gray)
	c.pdf.Line(x1, y1, x2, y2)
}

func (c *fpdfCanvas) newPage() { c.pdf.AddPage() }

func (c *fpdfCanvas) pageNo() int { return c.pdf.PageNo() }
