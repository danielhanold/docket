package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/evidence"
)

const esHead = "0123456789abcdef0123456789abcdef01234567"

func esRecord() string {
	return "---\nid: 7\nslug: x\n---\n\n## Artifacts\n\n<!-- docket:artifacts:start (generated — do not hand-edit) -->\n<!-- docket:artifacts:end -->\n\n## Why\n\nBecause.\n"
}

func esGreen(t *testing.T) evidence.Record {
	t.Helper()
	r, err := evidence.NewRecord("go test ./...", esHead, time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestUpsertRecordEvidenceAppendsSection(t *testing.T) {
	rec := esGreen(t)
	out, err := UpsertRecordEvidence([]byte(esRecord()), rec)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	want := esRecord() + "\n## Build evidence\n\n" + evidence.Render(rec) + "\n"
	if string(out) != want {
		t.Fatalf("upsert bytes:\n%q\nwant\n%q", out, want)
	}
	got, err := ReadRecordEvidence(out)
	if err != nil || got != rec {
		t.Fatalf("read back = %+v, %v; want %+v", got, err, rec)
	}
}

func TestUpsertRecordEvidenceReplacesInPlace(t *testing.T) {
	first := esGreen(t)
	src := esRecord() + "\n## Build evidence\n\n" + evidence.Render(first) + "\n\n## Reconcile log\n\n### 2026-10-05\n\nok\n"
	second, _ := evidence.NewSkippedRecord(esHead, time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC))
	out, err := UpsertRecordEvidence([]byte(src), second)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if !strings.Contains(string(out), "## Build evidence\n\n"+evidence.Render(second)+"\n\n## Reconcile log") {
		t.Fatalf("section not replaced in place:\n%s", out)
	}
	if strings.Count(string(out), "## Build evidence") != 1 {
		t.Fatalf("duplicate section:\n%s", out)
	}
	if !strings.HasSuffix(string(out), "## Reconcile log\n\n### 2026-10-05\n\nok\n") {
		t.Fatalf("bytes after the section changed:\n%s", out)
	}
}

func TestUpsertRecordEvidenceKeepsCRLF(t *testing.T) {
	crlf := strings.ReplaceAll(esRecord(), "\n", "\r\n")
	out, err := UpsertRecordEvidence([]byte(crlf), esGreen(t))
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if strings.Contains(strings.ReplaceAll(string(out), "\r\n", ""), "\n") {
		t.Fatalf("mixed line endings in a CRLF record:\n%q", out)
	}
}

func TestReadRecordEvidenceMissingAndFenced(t *testing.T) {
	if _, err := ReadRecordEvidence([]byte(esRecord())); !errors.Is(err, evidence.ErrMissing) {
		t.Fatalf("absent section err = %v, want ErrMissing", err)
	}
}

func TestReadRecordEvidenceIgnoresFencedHeading(t *testing.T) {
	src := esRecord() + "\n```\n## Build evidence\n\n" + evidence.Render(esGreen(t)) + "\n```\n"
	if _, err := ReadRecordEvidence([]byte(src)); !errors.Is(err, evidence.ErrMissing) {
		t.Fatalf("fenced heading read as the section: err = %v", err)
	}
	if v := VerifyRecordEvidence([]byte(src), esHead); v != evidence.VerdictMissing {
		t.Fatalf("verify fenced = %q, want missing", v)
	}
}

func TestVerifyRecordEvidenceVerdicts(t *testing.T) {
	out, _ := UpsertRecordEvidence([]byte(esRecord()), esGreen(t))
	if v := VerifyRecordEvidence(out, esHead); v != evidence.VerdictVerified {
		t.Fatalf("exact head = %q", v)
	}
	if v := VerifyRecordEvidence(out, strings.Repeat("f", 40)); v != evidence.VerdictStale {
		t.Fatalf("other head = %q", v)
	}
	dup := string(out) + "\n## Build evidence\n\nx\n"
	if v := VerifyRecordEvidence([]byte(dup), esHead); v != evidence.VerdictMalformed {
		t.Fatalf("duplicated section = %q, want malformed", v)
	}
}

func TestRecordEvidenceFacts(t *testing.T) {
	out, _ := UpsertRecordEvidence([]byte(esRecord()), esGreen(t))
	h, cmd, green := recordEvidenceFacts(out)
	if h != esHead || cmd != "go test ./..." || !green {
		t.Fatalf("facts = %q %q %v", h, cmd, green)
	}
	if h, cmd, green := recordEvidenceFacts([]byte(esRecord())); h != "" || cmd != "" || green {
		t.Fatalf("absent facts = %q %q %v", h, cmd, green)
	}
}
