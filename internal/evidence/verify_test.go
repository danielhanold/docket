package evidence

import (
	"strings"
	"testing"
)

func bodyFor(t *testing.T, command, head string) []byte {
	t.Helper()
	return []byte(Render(mustRecord(t, command, head)))
}

func TestVerifyVerified(t *testing.T) {
	body := bodyFor(t, "go test ./...", head40)
	if v := Verify(body, head40); v != VerdictVerified {
		t.Errorf("verdict = %q, want verified", v)
	}
}

func TestVerifyNormalizesSuppliedHeadCase(t *testing.T) {
	body := bodyFor(t, "go test ./...", head40)
	if v := Verify(body, strings.ToUpper(head40)); v != VerdictVerified {
		t.Errorf("verdict = %q, want verified (uppercase supplied head normalized)", v)
	}
}

func TestVerifyStaleDifferentHead(t *testing.T) {
	body := bodyFor(t, "go test ./...", head40)
	other := strings.Repeat("f", 40)
	if v := Verify(body, other); v != VerdictStale {
		t.Errorf("verdict = %q, want stale", v)
	}
}

func TestVerifyStaleWidthMismatch(t *testing.T) {
	// record is 40-hex; supplied is 64-hex whose first 40 chars equal it.
	body := bodyFor(t, "go test ./...", head40)
	if head64[:40] != head40 {
		t.Fatalf("test fixtures broken")
	}
	if v := Verify(body, head64); v != VerdictStale {
		t.Errorf("verdict = %q, want stale (width mismatch, not prefix match)", v)
	}
}

func TestVerifyStalePrefixOnly(t *testing.T) {
	// record is 64-hex; supplied is the 40-hex prefix of it. A prefix test
	// would say verified; full-length equality says stale.
	body := bodyFor(t, "go test ./...", head64)
	if v := Verify(body, head40); v != VerdictStale {
		t.Errorf("verdict = %q, want stale (prefix must not verify)", v)
	}
}

func TestVerifyMissing(t *testing.T) {
	body := []byte("# just prose\n\nnothing here.\n")
	if v := Verify(body, head40); v != VerdictMissing {
		t.Errorf("verdict = %q, want missing", v)
	}
}

func TestVerifyMalformed(t *testing.T) {
	body := []byte("<!-- docket:backlink:start -->\ndangling marker\n")
	if v := Verify(body, head40); v != VerdictMalformed {
		t.Errorf("verdict = %q, want malformed", v)
	}
}

func TestVerifyMalformedInterior(t *testing.T) {
	body := evidenceWith("command:  a", "result:   red",
		"head_sha: "+head40, "ran_at:   2026-08-16T12:00:00Z")
	if v := Verify(body, head40); v != VerdictMalformed {
		t.Errorf("verdict = %q, want malformed", v)
	}
}
