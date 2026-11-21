package pdfrender

import (
	"strings"
	"testing"
	"time"
)

// drawn is one recorded text operation.
type drawn struct {
	page int
	x, y float64
	f    face
	size float64
	s    string
}

// rectOp is one recorded fillRect operation.
type rectOp struct {
	page       int
	x, y, w, h float64
	gray       int
	corners    string // rounded corners; "" for fillRect
}

// lineOp is one recorded line operation.
type lineOp struct {
	page                  int
	x1, y1, x2, y2, width float64
	gray                  int
}

// recorder wraps a real fpdfCanvas, so measure is the real one, and records
// every text, fillRect, and line call with the page it was drawn on.
type recorder struct {
	*fpdfCanvas
	ops   []drawn
	rects []rectOp
	lines []lineOp
}

func newRecorder() *recorder { return &recorder{fpdfCanvas: newFpdfCanvas()} }

func (r *recorder) text(x, y float64, f face, size float64, s string) {
	r.ops = append(r.ops, drawn{page: r.pageNo(), x: x, y: y, f: f, size: size, s: s})
	r.fpdfCanvas.text(x, y, f, size, s)
}

func (r *recorder) fillRect(x, y, w, h float64, gray int) {
	r.rects = append(r.rects, rectOp{page: r.pageNo(), x: x, y: y, w: w, h: h, gray: gray})
	r.fpdfCanvas.fillRect(x, y, w, h, gray)
}

func (r *recorder) fillRoundRect(x, y, w, h, rad float64, corners string, gray int) {
	r.rects = append(r.rects, rectOp{page: r.pageNo(), x: x, y: y, w: w, h: h, gray: gray, corners: corners})
	r.fpdfCanvas.fillRoundRect(x, y, w, h, rad, corners, gray)
}

func (r *recorder) line(x1, y1, x2, y2, width float64, gray int) {
	r.lines = append(r.lines, lineOp{page: r.pageNo(), x1: x1, y1: y1, x2: x2, y2: y2, width: width, gray: gray})
	r.fpdfCanvas.line(x1, y1, x2, y2, width, gray)
}

// renderRec runs render over md with a fresh recorder.
func renderRec(t *testing.T, md string) (*recorder, string, int) {
	t.Helper()
	r := newRecorder()
	title, n := render([]byte(md), "fallback-title", r)
	return r, title, n
}

// record runs render over md with a fresh recorder and touches no
// *testing.T, so it is safe inside within.
func record(md string) *recorder {
	r := newRecorder()
	render([]byte(md), "fallback-title", r)
	return r
}

// texts is all drawn text, footers included, concatenated in drawing order.
func texts(r *recorder) string {
	var b strings.Builder
	for _, d := range r.ops {
		b.WriteString(d.s)
	}
	return b.String()
}

// body is every text operation except the page footers.
func body(r *recorder) []drawn {
	var out []drawn
	for _, d := range r.ops {
		if d.y != footerY {
			out = append(out, d)
		}
	}
	return out
}

// bodyText is the body's drawn text concatenated in drawing order.
func bodyText(r *recorder) string {
	var b strings.Builder
	for _, d := range body(r) {
		b.WriteString(d.s)
	}
	return b.String()
}

// find returns every text operation whose text is exactly s.
func find(r *recorder, s string) []drawn {
	var out []drawn
	for _, d := range r.ops {
		if d.s == s {
			out = append(out, d)
		}
	}
	return out
}

// findOne returns the single text operation whose text is exactly s.
func findOne(t *testing.T, r *recorder, s string) drawn {
	t.Helper()
	got := find(r, s)
	if len(got) != 1 {
		t.Fatalf("want exactly one op drawing %q, got %d (all ops: %+v)", s, len(got), r.ops)
	}
	return got[0]
}

// within runs fn on another goroutine and fails the test if it has not
// returned after d, so a layout loop that never ends fails cleanly instead of
// hanging the binary. fn must not call t: assert after within returns.
func within(t *testing.T, d time.Duration, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("did not finish within %s", d)
	}
}
