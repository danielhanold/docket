package evidence

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/danielhanold/docket/internal/document"
)

// blockName is the managed-block name that bounds the evidence record.
const blockName = "build-evidence"

// Marker lines the codec renders. document.Parse recognizes and validates these
// as a whole population; the codec never scans for them itself.
const (
	startMarker = "<!-- docket:" + blockName + ":start -->"
	endMarker   = "<!-- docket:" + blockName + ":end -->"
)

// Field keys, in canonical order. Each value column is aligned to a common
// offset so the rendered block reads as a table.
const (
	keyCommand = "command"
	keyResult  = "result"
	keyHead    = "head_sha"
	keyRanAt   = "ran_at"
)

// ErrMissing reports that a body carries no build-evidence block at all. It is
// distinct from a malformed-block error so a caller can tell "never recorded"
// from "recorded but unreadable".
var ErrMissing = errors.New("evidence: no build-evidence block")

// Render returns the one canonical complete block with LF line endings and no
// trailing newline. Values are aligned to a common column.
func Render(r Record) string {
	return startMarker + "\n" + interior(r) + "\n" + endMarker
}

// interior renders the four field lines (LF-joined, no trailing newline) that
// sit between the markers. It is shared by Render and the loss-preserving
// replace path so both produce identical block contents.
func interior(r Record) string {
	return field(keyCommand, r.Command) + "\n" +
		field(keyResult, string(r.Result)) + "\n" +
		field(keyHead, r.Head) + "\n" +
		field(keyRanAt, r.RanAt.UTC().Format(time.RFC3339))
}

// field renders one "key:  value" line with the value aligned to valueColumn.
func field(key, value string) string {
	pad := valueColumn - len(key) - 1 // -1 for the colon
	if pad < 1 {
		pad = 1
	}
	return key + ":" + strings.Repeat(" ", pad) + value
}

// valueColumn is the byte column the value starts on: len("head_sha: "), the
// widest key, so every value aligns.
const valueColumn = len("head_sha: ")

// Extract strictly parses an existing body and returns its build-evidence
// record. It requires document.Parse to succeed over the WHOLE body (every
// docket marker balanced, fence-aware), exactly one build-evidence block (the
// document parser rejects duplicate pairs), and an interior with exactly one
// line per known key, no unknown keys, and no nonblank extra lines. Keys split
// at the first colon only, so a command may contain later colons; CRLF and LF
// are both accepted. A body with no block returns ErrMissing; every other
// failure returns a distinct malformed error.
func Extract(body []byte) (Record, error) {
	doc, err := document.Parse(body)
	if err != nil {
		return Record{}, fmt.Errorf("evidence: malformed body: %w", err)
	}
	block, ok := doc.Block(blockName)
	if !ok {
		return Record{}, ErrMissing
	}
	src := doc.Source()
	return parseInterior(src[block.Interior.Start:block.Interior.End])
}

// parseInterior reads the bytes strictly between the two markers into a Record.
func parseInterior(interior []byte) (Record, error) {
	values := map[string]string{}
	for _, raw := range strings.Split(string(interior), "\n") {
		line := strings.TrimSuffix(raw, "\r")
		if line == "" {
			continue // blank lines are permitted; nonblank extras are not
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			return Record{}, fmt.Errorf("evidence: malformed line (no key separator): %q", line)
		}
		key := line[:colon]
		value := strings.TrimLeft(line[colon+1:], " \t")
		if !knownKey(key) {
			return Record{}, fmt.Errorf("evidence: unknown key %q", key)
		}
		if _, dup := values[key]; dup {
			return Record{}, fmt.Errorf("evidence: duplicate key %q", key)
		}
		values[key] = value
	}
	for _, key := range []string{keyCommand, keyResult, keyHead, keyRanAt} {
		if _, ok := values[key]; !ok {
			return Record{}, fmt.Errorf("evidence: missing key %q", key)
		}
	}
	if values[keyResult] != string(ResultGreen) {
		return Record{}, fmt.Errorf("evidence: result is %q, only %q is valid", values[keyResult], ResultGreen)
	}
	ranAt, err := time.Parse(time.RFC3339, values[keyRanAt])
	if err != nil {
		return Record{}, fmt.Errorf("evidence: ran_at is not RFC3339: %w", err)
	}
	return NewRecord(values[keyCommand], values[keyHead], ranAt)
}

// knownKey reports whether key is one of the four schema keys.
func knownKey(key string) bool {
	switch key {
	case keyCommand, keyResult, keyHead, keyRanAt:
		return true
	default:
		return false
	}
}
