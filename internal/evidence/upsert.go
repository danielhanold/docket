package evidence

import (
	"fmt"

	"github.com/danielhanold/docket/internal/document"
)

// Upsert writes r into body without disturbing any byte outside the owned
// block. When a build-evidence block is present, only its interior is replaced
// via the document patch API, leaving both marker lines and every other body
// byte identical. When absent, one canonical LF block is appended after a
// deterministic blank-line boundary, preserving every pre-existing byte as a
// prefix. Either way the original whole-population marker validation runs first,
// and the candidate is reparsed and re-extracted before any bytes are returned:
// if either parse fails, Upsert returns no bytes.
func Upsert(body []byte, r Record) ([]byte, error) {
	// r must be a valid green record. Re-normalize through NewRecord so a caller
	// that hand-built the struct (or tampered its Result) cannot smuggle an
	// unvalidated or non-green record into a PR body.
	if r.Result != ResultGreen {
		return nil, fmt.Errorf("evidence: refusing to upsert a non-green record (result %q)", r.Result)
	}
	norm, err := NewRecord(r.Command, r.Head, r.RanAt)
	if err != nil {
		return nil, err
	}

	doc, err := document.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("evidence: malformed body: %w", err)
	}

	var candidate []byte
	if _, ok := doc.Block(blockName); ok {
		var patch document.PatchSet
		patch.ReplaceBlock(blockName, interior(norm))
		candidate, err = doc.Apply(patch)
		if err != nil {
			return nil, fmt.Errorf("evidence: replace failed: %w", err)
		}
	} else {
		candidate = appendBlock(body, norm)
	}

	// Reparse gate: the candidate must read back as exactly the record we wrote.
	// Extract runs document.Parse internally, so a candidate that broke the
	// marker population fails here rather than reaching the caller.
	got, err := Extract(candidate)
	if err != nil {
		return nil, fmt.Errorf("evidence: candidate failed reparse: %w", err)
	}
	if got != norm {
		return nil, fmt.Errorf("evidence: candidate reparsed to a different record")
	}
	return candidate, nil
}

// appendBlock returns body with one canonical LF block appended after a
// deterministic blank-line boundary. Every original byte is preserved as a
// prefix: the boundary only ADDS newlines so that at least one blank line
// separates prior content from the block, and a trailing newline follows the
// block. An empty body receives the block with no leading blanks.
func appendBlock(body []byte, r Record) []byte {
	block := Render(r) + "\n"
	if len(body) == 0 {
		return []byte(block)
	}
	out := append([]byte(nil), body...)
	for trailingNewlines(out) < 2 {
		out = append(out, '\n')
	}
	return append(out, block...)
}

// trailingNewlines counts the '\n' bytes at the end of b. A CRLF terminator
// ends in '\n' too, so a body written with CRLF still reports its line count
// here; the count only ever drives how many '\n' the boundary must add.
func trailingNewlines(b []byte) int {
	n := 0
	for i := len(b) - 1; i >= 0 && b[i] == '\n'; i-- {
		n++
	}
	return n
}
