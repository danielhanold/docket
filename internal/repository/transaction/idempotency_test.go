package transaction

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// This file proves the request-ID idempotency contract end to end through
// Execute against real Git topologies: the engine's five-trailer block on keyed
// commits (three on unkeyed), lost-response replay returning the ORIGINAL receipt
// with no new commit, request-id reuse detection by digest, and the invalid-state
// verdicts for duplicate/malformed/contradictory history. Hand-crafted history is
// built through the writer clone with git's own commit machinery, so the scan is
// exercised against genuine trailer blocks, not fixtures the engine authored.

// keyReq is the standard idempotency key the keyed tests reuse.
func keyReq() *IdempotencyKey {
	return &IdempotencyKey{RequestID: "req-abc-00000001", Digest: validDigest()}
}

// otherDigest is a well-formed sha256 digest distinct from validDigest, for the
// request-id-reuse case (same ID, different digest).
func otherDigest() RequestDigest {
	return RequestDigest("sha256:" + strings.Repeat("b", 64))
}

// b64 is the unpadded base64url encoding the Docket-Result trailer uses.
func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// trailerKeys returns the ordered keys of the parsed trailer block of a commit,
// read with git's own trailer interpretation (the same parser
// `git interpret-trailers --parse` uses), so a body-prose line never appears.
func trailerKeys(t *testing.T, dir string, commit gitcli.ObjectID) []string {
	t.Helper()
	block := hgitOut(t, dir, "log", "-1", "--format=%(trailers:only,unfold)", string(commit))
	var keys []string
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, ":"); idx >= 0 {
			keys = append(keys, strings.TrimSpace(line[:idx]))
		}
	}
	return keys
}

// plantCommit makes one empty commit on the target branch in the writer clone with
// exactly message as its full commit message (subject plus a hand-authored trailer
// block), pushes it to origin, and returns the new commit id. The writer must be
// current with origin on the target branch (true in a freshly built main-mode repo
// before the engine has applied anything).
func plantCommit(t *testing.T, r *testRepos, message string) gitcli.ObjectID {
	t.Helper()
	branch := r.short()
	hgitOut(t, r.Writer, "checkout", "-q", branch)
	hgitOut(t, r.Writer, "commit", "-q", "--allow-empty", "-m", message)
	hgitOut(t, r.Writer, "push", "-q", "origin", branch)
	return gitcli.ObjectID(hgitOut(t, r.Writer, "rev-parse", "HEAD"))
}

// engineBlockMessage renders a full commit message with a subject and the engine's
// five-trailer block as its final paragraph.
func engineBlockMessage(subject, txnID, op, reqID, digest, resultB64 string) string {
	var b strings.Builder
	b.WriteString(subject)
	b.WriteString("\n\n")
	b.WriteString("Docket-Transaction-ID: " + txnID + "\n")
	b.WriteString("Docket-Operation: " + op + "\n")
	b.WriteString("Docket-Request-ID: " + reqID + "\n")
	b.WriteString("Docket-Request-Digest: " + digest + "\n")
	b.WriteString("Docket-Result: " + resultB64 + "\n")
	return b.String()
}
