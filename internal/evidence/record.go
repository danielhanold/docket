// Package evidence reads and writes the build-evidence block that certifies an
// exact tested commit. It preserves ADR-0066's marker-bounded schema — a green
// record carries command, result, head_sha, ran_at; a skipped record carries
// result, reason, head_sha, ran_at — bounded by the docket:build-evidence
// markers, and reuses internal/document's whole-population, fence-aware marker
// validation and loss-preserving patch API rather than scanning markers of its
// own.
//
// The package is deliberately process-free and text-inference-free: it never
// runs a command, reads a gate artifact, infers "green" from output text, or
// accepts a 128+signal heuristic. A caller supplies either an already-decided
// terminal green outcome (an exact command, an authoritative full object ID,
// and a timestamp) or an explicit build.gate: off policy that mints a truthful
// skipped record; a red, interrupted, running, or unavailable gate produces no
// record at all.
package evidence

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Result is the certified gate outcome. Two values are valid: green, from a
// decided green gate run; and skipped, minted only when the repository
// explicitly set build.gate: off. A red, interrupted, running, or unavailable
// gate creates no record.
type Result string

const (
	// ResultGreen certifies a decided green gate run. red, interrupted,
	// running, and unavailable gates create no record.
	ResultGreen Result = "green"
	// ResultSkipped is the truthful record for a repository that explicitly set
	// build.gate: off — nothing ran, so there is no command, only the fixed
	// ReasonBuildGateOff.
	ResultSkipped Result = "skipped"
)

// ReasonBuildGateOff is the fixed Reason a skipped record carries: the
// repository explicitly disabled its build test gate (build.gate: off).
const ReasonBuildGateOff = "build-gate-off"

// Record is the immutable trusted build-evidence value. Every field is
// validated and normalized by NewRecord or NewSkippedRecord; the zero value is
// not a valid record.
type Record struct {
	Command string    // green: non-empty single line, valid UTF-8, no control bytes, no surrounding whitespace; skipped: empty
	Result  Result    // ResultGreen or ResultSkipped
	Reason  string    // skipped: ReasonBuildGateOff; green: empty
	Head    string    // normalized full lowercase 40- or 64-hex object ID
	RanAt   time.Time // UTC, second precision
}

// NewRecord validates command and head, normalizes head to lowercase and ranAt
// to UTC second precision, and returns the immutable green Record. The result
// is always green: a caller only reaches this constructor with a decided green
// gate, so there is no result parameter to get wrong.
func NewRecord(command, head string, ranAt time.Time) (Record, error) {
	if err := validCommand(command); err != nil {
		return Record{}, err
	}
	h := strings.ToLower(head)
	if !validHead(h) {
		return Record{}, fmt.Errorf("evidence: head is not a full lowercase 40- or 64-hex object ID: %q", head)
	}
	return Record{
		Command: command,
		Result:  ResultGreen,
		Head:    h,
		RanAt:   ranAt.UTC().Truncate(time.Second),
	}, nil
}

// NewSkippedRecord returns the truthful record for an explicitly disabled
// build gate (build.gate: off). It carries no command — the gate ran nothing —
// and the fixed reason build-gate-off. Only explicit gate-off policy reaches
// this constructor: an unconfigured command, launch failure, or ambiguous
// verdict produces no record of any kind.
func NewSkippedRecord(head string, ranAt time.Time) (Record, error) {
	h := strings.ToLower(head)
	if !validHead(h) {
		return Record{}, fmt.Errorf("evidence: head is not a full lowercase 40- or 64-hex object ID: %q", head)
	}
	return Record{
		Result: ResultSkipped,
		Reason: ReasonBuildGateOff,
		Head:   h,
		RanAt:  ranAt.UTC().Truncate(time.Second),
	}, nil
}

// validCommand enforces the command grammar: valid UTF-8, non-empty, a single
// line with no ASCII control bytes (newline and tab included), and no leading or
// trailing whitespace — the last so a rendered command round-trips exactly
// through the alignment padding that Render inserts and Extract strips.
func validCommand(command string) error {
	if !utf8.ValidString(command) {
		return errors.New("evidence: command is not valid UTF-8")
	}
	if command == "" {
		return errors.New("evidence: command is empty")
	}
	for i := 0; i < len(command); i++ {
		if b := command[i]; b < 0x20 || b == 0x7f {
			return fmt.Errorf("evidence: command contains control byte %#02x", b)
		}
	}
	if strings.TrimSpace(command) != command {
		return errors.New("evidence: command has leading or trailing whitespace")
	}
	return nil
}

// validHead reports whether s is a full lowercase hexadecimal object ID: exactly
// 40 (SHA-1) or 64 (SHA-256) characters, each 0-9 or a-f. A local check keeps
// the package process-free — it never shells out to git for OID validation.
func validHead(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
