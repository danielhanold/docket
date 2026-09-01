package evidence

import (
	"strings"
	"testing"
	"time"
)

func TestNewSkippedRecord(t *testing.T) {
	rec, err := NewSkippedRecord("AB12"+strings.Repeat("cd", 18), time.Date(2026, 8, 31, 12, 0, 0, 500e6, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if rec.Result != ResultSkipped || rec.Reason != ReasonBuildGateOff {
		t.Errorf("result/reason = %q/%q, want skipped/build-gate-off", rec.Result, rec.Reason)
	}
	if rec.Command != "" {
		t.Errorf("a skipped record carries no command, got %q", rec.Command)
	}
	if rec.Head != "ab12"+strings.Repeat("cd", 18) {
		t.Errorf("head not normalized: %q", rec.Head)
	}
}

func TestSkippedRoundTrip(t *testing.T) {
	rec, _ := NewSkippedRecord(strings.Repeat("ab", 20), time.Now())
	block := Render(rec)
	if strings.Contains(block, "command") {
		t.Errorf("skipped block must carry no command line:\n%s", block)
	}
	if !strings.Contains(block, "reason:") || !strings.Contains(block, "build-gate-off") {
		t.Errorf("skipped block must carry the reason line:\n%s", block)
	}
	got, err := Extract([]byte(block))
	if err != nil || got != rec {
		t.Errorf("round trip: got %+v err %v, want %+v", got, err, rec)
	}
}

func TestVerifySkipped(t *testing.T) {
	head := strings.Repeat("ab", 20)
	rec, _ := NewSkippedRecord(head, time.Now())
	if v := Verify([]byte(Render(rec)), head); v != VerdictSkipped {
		t.Errorf("exact-head skipped = %v, want VerdictSkipped", v)
	}
	if v := Verify([]byte(Render(rec)), strings.Repeat("cd", 20)); v != VerdictStale {
		t.Errorf("wrong-head skipped = %v, want VerdictStale", v)
	}
}

func TestGreenRenderUnchanged(t *testing.T) {
	// Legacy compatibility pin: a green record's rendered bytes carry exactly
	// command/result/head_sha/ran_at — no reason line.
	rec, _ := NewRecord("go test ./...", strings.Repeat("ab", 20), time.Now())
	if strings.Contains(Render(rec), "reason") {
		t.Errorf("green block grew a reason line:\n%s", Render(rec))
	}
}
