package evidence

import (
	"bytes"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/document"
)

// blockEnvelope returns the bytes up to and including the block's start marker
// line, and the bytes from the block's end marker line to EOF. Two bodies with
// byte-identical envelopes differ only inside the owned interior.
func blockEnvelope(t *testing.T, body []byte) (prefix, suffix []byte) {
	t.Helper()
	doc, err := document.Parse(body)
	if err != nil {
		t.Fatalf("document.Parse: %v", err)
	}
	b, ok := doc.Block("build-evidence")
	if !ok {
		t.Fatalf("no build-evidence block")
	}
	src := doc.Source()
	return src[:b.Start.End], src[b.End.Start:]
}

func TestUpsertReplacePreservesEverythingElse(t *testing.T) {
	// A stale head that is NOT a prefix of the fresh head, so the "old head is
	// gone" assertion is not fooled by head64 beginning with head40.
	staleHead := strings.Repeat("a", 40)
	stale := mustRecord(t, "go test ./...", staleHead)
	body := []byte("<!-- docket:backlink:start -->\n> backlink text\n<!-- docket:backlink:end -->\n\n" +
		"## Summary\n\nProse describing the change in detail.\n\n" +
		Render(stale) + "\n\n" +
		"## Findings\n\n| id | note |\n|----|------|\n| 1  | none |\n")

	fresh := mustRecord(t, "make verify", head64)
	out, err := Upsert(body, fresh)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	prefIn, sufIn := blockEnvelope(t, body)
	prefOut, sufOut := blockEnvelope(t, out)
	if !bytes.Equal(prefIn, prefOut) {
		t.Errorf("prefix changed:\n in: %q\nout: %q", prefIn, prefOut)
	}
	if !bytes.Equal(sufIn, sufOut) {
		t.Errorf("suffix changed:\n in: %q\nout: %q", sufIn, sufOut)
	}
	got, err := Extract(out)
	if err != nil {
		t.Fatalf("Extract(out): %v", err)
	}
	if got != fresh {
		t.Errorf("extracted %+v, want %+v", got, fresh)
	}
	// The old head must be gone, the new head present, prose preserved.
	if bytes.Contains(out, []byte(staleHead)) {
		t.Errorf("stale head still present")
	}
	if !bytes.Contains(out, []byte(head64)) {
		t.Errorf("fresh head missing")
	}
	if !bytes.Contains(out, []byte("Prose describing the change")) {
		t.Errorf("prose lost")
	}
	if !bytes.Contains(out, []byte("| 1  | none |")) {
		t.Errorf("findings table lost")
	}
}

func TestUpsertAppendWhenAbsent(t *testing.T) {
	body := []byte("<!-- docket:backlink:start -->\n> backlink\n<!-- docket:backlink:end -->\n\n## Summary\n\nProse.\n")
	r := mustRecord(t, "go test ./...", head40)
	out, err := Upsert(body, r)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if !bytes.HasPrefix(out, body) {
		t.Fatalf("original body is not a byte prefix of output")
	}
	// exactly one blank-line boundary: body ends with one \n, so the boundary
	// bytes between it and the start marker are a single extra \n.
	tail := out[len(body):]
	if !bytes.HasPrefix(tail, []byte("\n<!-- docket:build-evidence:start -->")) {
		t.Errorf("boundary = %q, want single blank line then start marker", tail)
	}
	got, err := Extract(out)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got != r {
		t.Errorf("extracted %+v, want %+v", got, r)
	}
}

func TestUpsertIdempotent(t *testing.T) {
	body := []byte("## Summary\n\nProse.\n")
	r := mustRecord(t, "go test ./...", head64)
	out1, err := Upsert(body, r)
	if err != nil {
		t.Fatalf("Upsert 1: %v", err)
	}
	out2, err := Upsert(out1, r)
	if err != nil {
		t.Fatalf("Upsert 2: %v", err)
	}
	if !bytes.Equal(out1, out2) {
		t.Errorf("not idempotent:\n1: %q\n2: %q", out1, out2)
	}
}

func TestUpsertMalformedPopulationRefused(t *testing.T) {
	body := []byte("## Summary\n\n<!-- docket:backlink:start -->\ndangling\n")
	orig := append([]byte(nil), body...)
	r := mustRecord(t, "go test ./...", head40)
	out, err := Upsert(body, r)
	if err == nil {
		t.Fatalf("expected error on malformed population")
	}
	if out != nil {
		t.Errorf("expected nil bytes, got %q", out)
	}
	if !bytes.Equal(body, orig) {
		t.Errorf("input mutated")
	}
}

func TestUpsertExtractRoundTripProperty(t *testing.T) {
	commands := []string{
		"go test ./...",
		"env A=1 go test: ./... -run X:Y",
		"make check && go vet ./...",
	}
	bodies := [][]byte{
		[]byte("## Summary\n\nProse.\n"),
		[]byte("Prose with no trailing newline"),
		[]byte("line1\r\nline2\r\n"), // CRLF original prose
		[]byte(""),
	}
	for _, cmd := range commands {
		for _, head := range []string{head40, head64} {
			r := mustRecord(t, cmd, head)
			for _, body := range bodies {
				out, err := Upsert(body, r)
				if err != nil {
					t.Fatalf("Upsert(cmd=%q body=%q): %v", cmd, body, err)
				}
				got, err := Extract(out)
				if err != nil {
					t.Fatalf("Extract: %v", err)
				}
				if got != r {
					t.Errorf("cmd=%q body=%q: got %+v want %+v", cmd, body, got, r)
				}
			}
		}
	}
}

func TestUpsertRejectsNonGreenRecord(t *testing.T) {
	body := []byte("## Summary\n")
	r := mustRecord(t, "go test ./...", head40)
	r.Result = "red" // tamper the immutable-by-convention value
	if _, err := Upsert(body, r); err == nil {
		t.Fatalf("expected Upsert to reject a non-green record")
	}
}

// TestUpsertReplaceKeepsBlockLineEnding proves the replacement reuses the
// existing block's line ending rather than forcing LF into a CRLF block.
func TestUpsertReplaceCRLFBlock(t *testing.T) {
	stale := mustRecord(t, "old", head40)
	crlfBlock := strings.ReplaceAll(Render(stale), "\n", "\r\n")
	body := []byte("## Summary\r\n\r\n" + crlfBlock + "\r\n")
	fresh := mustRecord(t, "new", head64)
	out, err := Upsert(body, fresh)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := Extract(out)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got != fresh {
		t.Errorf("got %+v want %+v", got, fresh)
	}
}
