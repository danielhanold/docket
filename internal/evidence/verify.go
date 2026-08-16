package evidence

import (
	"errors"
	"strings"
)

// Verdict is the narrow, prose-free outcome of exact-head verification.
type Verdict string

const (
	VerdictVerified  Verdict = "verified"  // parsed, green, and head matches exactly
	VerdictMissing   Verdict = "missing"   // no build-evidence block present
	VerdictMalformed Verdict = "malformed" // a block exists but does not parse
	VerdictStale     Verdict = "stale"     // parsed and green, but head does not match
)

// Verify extracts the build-evidence record from body and checks it against the
// exact branch HEAD the caller obtained authoritatively through Git (never a PR
// title or body claim). It returns verified only when the record parses, is
// green, and its full head_sha equals head after case normalization. The
// comparison is full-length equality — NEVER a prefix test — so a 40-hex record
// and a 64-hex head sharing a prefix, or vice versa, is stale rather than
// verified. A body with no block is missing; a block that does not parse is
// malformed.
func Verify(body []byte, head string) Verdict {
	record, err := Extract(body)
	switch {
	case errors.Is(err, ErrMissing):
		return VerdictMissing
	case err != nil:
		return VerdictMalformed
	}
	// Extract guarantees record.Result is green (interior parsing rejects any
	// other value), so an equal, case-normalized head is the only open question.
	if record.Head == strings.ToLower(head) {
		return VerdictVerified
	}
	return VerdictStale
}
