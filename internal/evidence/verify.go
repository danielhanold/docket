package evidence

import (
	"errors"
	"strings"
)

// Verdict is the narrow, prose-free outcome of exact-head verification.
type Verdict string

const (
	VerdictVerified  Verdict = "verified"  // parsed, green, and head matches exactly
	VerdictSkipped   Verdict = "skipped"   // parsed skipped (build-gate-off), and head matches exactly
	VerdictMissing   Verdict = "missing"   // no build-evidence block present
	VerdictMalformed Verdict = "malformed" // a block exists but does not parse
	VerdictStale     Verdict = "stale"     // parsed (green or skipped), but head does not match
)

// Verify extracts the build-evidence record from body and checks it against the
// exact branch HEAD the caller obtained authoritatively through Git (never a PR
// title or body claim). It returns verified when a GREEN record's full head_sha
// equals head after case normalization, and skipped when a SKIPPED
// (build-gate-off) record's head equals it — a caller distinguishes a green run
// from an explicitly disabled gate. The comparison is full-length equality —
// NEVER a prefix test — so a 40-hex record and a 64-hex head sharing a prefix,
// or vice versa, is stale rather than verified. A body with no block is missing;
// a block that does not parse is malformed.
func Verify(body []byte, head string) Verdict {
	record, err := Extract(body)
	switch {
	case errors.Is(err, ErrMissing):
		return VerdictMissing
	case err != nil:
		return VerdictMalformed
	}
	// Extract guarantees record.Result is green or skipped; a mismatched head is
	// stale for either, so resolve the head first.
	if record.Head != strings.ToLower(head) {
		return VerdictStale
	}
	if record.Result == ResultSkipped {
		return VerdictSkipped
	}
	return VerdictVerified
}
