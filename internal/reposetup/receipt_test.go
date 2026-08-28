package reposetup

import (
	"reflect"
	"testing"
)

// TestReceiptRoundTrip proves that a fully-populated Receipt encodes to
// trailers and parses back to an equal Receipt, for every operation value.
func TestReceiptRoundTrip(t *testing.T) {
	for _, op := range []string{OpInitRoot, OpMigrateSeed, OpMigratePrune} {
		r := Receipt{
			Operation:        op,
			SourceRevision:   "1111111111111111111111111111111111111111",
			MetadataRevision: "2222222222222222222222222222222222222222",
			CopyDigest:       "copydigesthex",
			RepairDigest:     "repairdigesthex",
		}
		got, ok := ParseReceipt(r.Trailers())
		if !ok {
			t.Fatalf("op %q: ParseReceipt returned ok=false for a receipt it encoded", op)
		}
		if !reflect.DeepEqual(got, r) {
			t.Fatalf("op %q: round-trip mismatch\n got %+v\nwant %+v", op, got, r)
		}
	}
}

// TestReceiptTrailersOperationFirst proves the operation trailer leads the
// encoded block and carries the operation value.
func TestReceiptTrailersOperationFirst(t *testing.T) {
	tr := Receipt{Operation: OpInitRoot}.Trailers()
	if len(tr) == 0 {
		t.Fatal("Trailers() returned no trailers")
	}
	if tr[0].Key != TrailerOperation || tr[0].Value != OpInitRoot {
		t.Fatalf("first trailer = %+v, want %s: %s", tr[0], TrailerOperation, OpInitRoot)
	}
}

// TestReceiptTrailersOmitsEmptyOptionalFields proves an init-root receipt (only
// the operation set) does not emit empty optional trailers.
func TestReceiptTrailersOmitsEmptyOptionalFields(t *testing.T) {
	tr := Receipt{Operation: OpInitRoot}.Trailers()
	if len(tr) != 1 {
		t.Fatalf("Trailers() = %d trailers, want exactly 1 (operation only): %+v", len(tr), tr)
	}
}

// TestParseReceiptUnknownOperation proves an unrecognized operation value is
// rejected.
func TestParseReceiptUnknownOperation(t *testing.T) {
	if r, ok := ParseReceipt([]Trailer{{Key: TrailerOperation, Value: "repository-frobnicate/v9"}}); ok {
		t.Fatalf("ParseReceipt accepted an unknown operation, got %+v", r)
	}
}

// TestParseReceiptMissingOperation proves trailers without an operation are
// rejected.
func TestParseReceiptMissingOperation(t *testing.T) {
	if _, ok := ParseReceipt([]Trailer{{Key: TrailerSourceRevision, Value: "abc"}}); ok {
		t.Fatal("ParseReceipt accepted trailers without an operation")
	}
}

// TestParseReceiptRejectsControlBytes proves a trailer value carrying a control
// byte (a newline or other C0/DEL byte) is rejected — defense in depth mirroring
// gitcli's validateTrailer, since reposetup stays gitcli-free.
func TestParseReceiptRejectsControlBytes(t *testing.T) {
	cases := map[string]string{
		"newline":         "abc\ndef",
		"nul":             "abc\x00def",
		"carriage-return": "abc\rdef",
		"del":             "abc\x7fdef",
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			trailers := []Trailer{
				{Key: TrailerOperation, Value: OpMigrateSeed},
				{Key: TrailerSourceRevision, Value: bad},
			}
			if r, ok := ParseReceipt(trailers); ok {
				t.Fatalf("ParseReceipt accepted a control-byte value, got %+v", r)
			}
		})
	}
}
