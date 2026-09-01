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
	keyReason  = "reason"
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
// replace path so both produce identical block contents. A green record renders
// command/result/head_sha/ran_at — byte-identical to ADR-0066's original schema
// so legacy blocks parse unchanged; a skipped record substitutes reason for
// command, rendering result/reason/head_sha/ran_at.
func interior(r Record) string {
	if r.Result == ResultSkipped {
		return field(keyResult, string(r.Result)) + "\n" +
			field(keyReason, r.Reason) + "\n" +
			field(keyHead, r.Head) + "\n" +
			field(keyRanAt, r.RanAt.UTC().Format(time.RFC3339))
	}
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
// The required field set is keyed on the result value: a green block requires
// command/result/head_sha/ran_at and forbids reason; a skipped block requires
// result/reason/head_sha/ran_at and forbids command. Any other result value —
// or a field mix that violates its result's shape — is a malformed error.
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
	result, ok := values[keyResult]
	if !ok {
		return Record{}, fmt.Errorf("evidence: missing key %q", keyResult)
	}
	switch Result(result) {
	case ResultGreen:
		return parseGreenInterior(values)
	case ResultSkipped:
		return parseSkippedInterior(values)
	default:
		return Record{}, fmt.Errorf("evidence: result is %q, only %q or %q is valid", result, ResultGreen, ResultSkipped)
	}
}

// parseGreenInterior parses the values of a result: green block: exactly
// command/result/head_sha/ran_at, with no reason line.
func parseGreenInterior(values map[string]string) (Record, error) {
	if _, present := values[keyReason]; present {
		return Record{}, fmt.Errorf("evidence: a green record carries no %q line", keyReason)
	}
	for _, key := range []string{keyCommand, keyHead, keyRanAt} {
		if _, ok := values[key]; !ok {
			return Record{}, fmt.Errorf("evidence: missing key %q", key)
		}
	}
	ranAt, err := time.Parse(time.RFC3339, values[keyRanAt])
	if err != nil {
		return Record{}, fmt.Errorf("evidence: ran_at is not RFC3339: %w", err)
	}
	return NewRecord(values[keyCommand], values[keyHead], ranAt)
}

// parseSkippedInterior parses the values of a result: skipped block: exactly
// result/reason/head_sha/ran_at, with no command line and reason pinned to
// ReasonBuildGateOff.
func parseSkippedInterior(values map[string]string) (Record, error) {
	if _, present := values[keyCommand]; present {
		return Record{}, fmt.Errorf("evidence: a skipped record carries no %q line", keyCommand)
	}
	for _, key := range []string{keyReason, keyHead, keyRanAt} {
		if _, ok := values[key]; !ok {
			return Record{}, fmt.Errorf("evidence: missing key %q", key)
		}
	}
	if values[keyReason] != ReasonBuildGateOff {
		return Record{}, fmt.Errorf("evidence: reason is %q, only %q is valid", values[keyReason], ReasonBuildGateOff)
	}
	ranAt, err := time.Parse(time.RFC3339, values[keyRanAt])
	if err != nil {
		return Record{}, fmt.Errorf("evidence: ran_at is not RFC3339: %w", err)
	}
	return NewSkippedRecord(values[keyHead], ranAt)
}

// knownKey reports whether key is one of the five schema keys (reason belongs to
// skipped records; command to green records — parseInterior enforces which set a
// given result value may carry).
func knownKey(key string) bool {
	switch key {
	case keyCommand, keyResult, keyReason, keyHead, keyRanAt:
		return true
	default:
		return false
	}
}
