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

// validSourceRev and validCopyDigest are well-formed 40-hex values reused across
// the seed-verdict cases. Their exact bytes carry no meaning here — the verdict
// decides on trailer shape, not on any comparison to a real object.
const (
	validSourceRev  = "1111111111111111111111111111111111111111"
	validCopyDigest = "2222222222222222222222222222222222222222"
)

// TestEvaluateSeedTrailers is the table-driven decision matrix for the pure
// seed-receipt verdict over a ROOT commit's raw trailer scan. It is stricter
// than ParseReceipt on purpose: duplicated recognized fields, a prune receipt on
// the root, an unsupported operation version, and operation-inappropriate fields
// are all invalid here even where the last-wins ParseReceipt reader tolerates
// them.
func TestEvaluateSeedTrailers(t *testing.T) {
	cases := []struct {
		name     string
		trailers []Trailer
		want     SeedVerdict
		// check runs only when want is SeedInit or SeedMigrate, to assert the
		// returned Receipt carries exactly the fields the verdict promises.
		check func(t *testing.T, r Receipt)
	}{
		{
			name:     "no recognized docket trailer at all",
			trailers: []Trailer{{Key: "Signed-off-by", Value: "someone"}},
			want:     SeedAbsent,
		},
		{
			name:     "empty trailer set",
			trailers: nil,
			want:     SeedAbsent,
		},
		{
			name: "recognized trailer but no operation",
			trailers: []Trailer{
				{Key: TrailerCopyDigest, Value: validCopyDigest},
			},
			want: SeedInvalid,
		},
		{
			name: "duplicated operation",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: OpInitRoot},
				{Key: TrailerOperation, Value: OpInitRoot},
			},
			want: SeedInvalid,
		},
		{
			name: "duplicated source revision",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: OpMigrateSeed},
				{Key: TrailerSourceRevision, Value: validSourceRev},
				{Key: TrailerSourceRevision, Value: validSourceRev},
				{Key: TrailerCopyDigest, Value: validCopyDigest},
				{Key: TrailerRepairDigest, Value: "deadbeef"},
			},
			want: SeedInvalid,
		},
		{
			name: "prune receipt on the root",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: OpMigratePrune},
				{Key: TrailerMetadataRev, Value: validSourceRev},
			},
			want: SeedInvalid,
		},
		{
			name: "unknown operation",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: "repository-frobnicate/v9"},
			},
			want: SeedInvalid,
		},
		{
			name: "unsupported version",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: "repository-init-root/v2"},
			},
			want: SeedInvalid,
		},
		{
			name: "control byte in a recognized value",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: OpMigrateSeed},
				{Key: TrailerSourceRevision, Value: "abc\ndef"},
				{Key: TrailerCopyDigest, Value: validCopyDigest},
				{Key: TrailerRepairDigest, Value: "deadbeef"},
			},
			want: SeedInvalid,
		},
		{
			name: "valid init root, operation only",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: OpInitRoot},
			},
			want: SeedInit,
			check: func(t *testing.T, r Receipt) {
				if r.Operation != OpInitRoot {
					t.Fatalf("operation = %q, want %q", r.Operation, OpInitRoot)
				}
				if r.SourceRevision != "" || r.MetadataRevision != "" || r.CopyDigest != "" || r.RepairDigest != "" {
					t.Fatalf("init receipt carried migrate-only fields: %+v", r)
				}
			},
		},
		{
			name: "init root carrying a migrate-only field",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: OpInitRoot},
				{Key: TrailerSourceRevision, Value: validSourceRev},
			},
			want: SeedInvalid,
		},
		{
			name: "init root carrying a copy digest",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: OpInitRoot},
				{Key: TrailerCopyDigest, Value: validCopyDigest},
			},
			want: SeedInvalid,
		},
		{
			name: "init root carrying a repair digest",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: OpInitRoot},
				{Key: TrailerRepairDigest, Value: "deadbeef"},
			},
			want: SeedInvalid,
		},
		{
			name: "init root carrying a metadata revision",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: OpInitRoot},
				{Key: TrailerMetadataRev, Value: validSourceRev},
			},
			want: SeedInvalid,
		},
		{
			name: "valid migrate seed with all three required fields",
			trailers: []Trailer{
				{Key: "Signed-off-by", Value: "someone"},
				{Key: TrailerOperation, Value: OpMigrateSeed},
				{Key: TrailerSourceRevision, Value: validSourceRev},
				{Key: TrailerCopyDigest, Value: validCopyDigest},
				{Key: TrailerRepairDigest, Value: "deadbeef"},
			},
			want: SeedMigrate,
			check: func(t *testing.T, r Receipt) {
				if r.SourceRevision != validSourceRev || r.CopyDigest != validCopyDigest {
					t.Fatalf("migrate receipt fields not preserved: %+v", r)
				}
				if r.RepairDigest != "deadbeef" {
					t.Fatalf("repair digest not carried through untouched: %q", r.RepairDigest)
				}
				if r.MetadataRevision != "" {
					t.Fatalf("migrate seed unexpectedly carried a metadata revision: %q", r.MetadataRevision)
				}
			},
		},
		{
			name: "migrate seed missing source revision",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: OpMigrateSeed},
				{Key: TrailerCopyDigest, Value: validCopyDigest},
				{Key: TrailerRepairDigest, Value: "deadbeef"},
			},
			want: SeedInvalid,
		},
		{
			name: "migrate seed missing copy digest",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: OpMigrateSeed},
				{Key: TrailerSourceRevision, Value: validSourceRev},
				{Key: TrailerRepairDigest, Value: "deadbeef"},
			},
			want: SeedInvalid,
		},
		{
			name: "migrate seed missing repair digest",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: OpMigrateSeed},
				{Key: TrailerSourceRevision, Value: validSourceRev},
				{Key: TrailerCopyDigest, Value: validCopyDigest},
			},
			want: SeedInvalid,
		},
		{
			name: "migrate seed carrying a metadata revision",
			trailers: []Trailer{
				{Key: TrailerOperation, Value: OpMigrateSeed},
				{Key: TrailerSourceRevision, Value: validSourceRev},
				{Key: TrailerCopyDigest, Value: validCopyDigest},
				{Key: TrailerRepairDigest, Value: "deadbeef"},
				{Key: TrailerMetadataRev, Value: validSourceRev},
			},
			want: SeedInvalid,
		},
		{
			name: "unrecognized non-docket trailers are ignored around a valid init",
			trailers: []Trailer{
				{Key: "Signed-off-by", Value: "someone"},
				{Key: "Reviewed-by", Value: "another"},
				{Key: TrailerOperation, Value: OpInitRoot},
			},
			want: SeedInit,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, v := EvaluateSeedTrailers(tc.trailers)
			if v != tc.want {
				t.Fatalf("verdict = %v, want %v", v, tc.want)
			}
			if v != SeedInit && v != SeedMigrate {
				// Invalid/absent verdicts must return the zero Receipt.
				if (rec != Receipt{}) {
					t.Fatalf("non-seed verdict returned a non-zero receipt: %+v", rec)
				}
				return
			}
			if tc.check != nil {
				tc.check(t, rec)
			}
		})
	}
}

// TestEvaluateSeedTrailersMigrateValid mirrors the plan's worked example: a
// valid OpMigrateSeed receipt with unrelated trailers mixed in yields
// SeedMigrate with the source, copy, and repair fields preserved verbatim.
func TestEvaluateSeedTrailersMigrateValid(t *testing.T) {
	rec, v := EvaluateSeedTrailers([]Trailer{
		{Key: "Signed-off-by", Value: "someone"},
		{Key: TrailerOperation, Value: OpMigrateSeed},
		{Key: TrailerSourceRevision, Value: validSourceRev},
		{Key: TrailerCopyDigest, Value: validCopyDigest},
		{Key: TrailerRepairDigest, Value: "deadbeef"},
	})
	if v != SeedMigrate {
		t.Fatalf("verdict = %v, want SeedMigrate", v)
	}
	if rec.SourceRevision == "" || rec.CopyDigest == "" || rec.RepairDigest != "deadbeef" {
		t.Fatalf("receipt fields not preserved: %+v", rec)
	}
}
