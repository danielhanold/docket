package evidence

import (
	"strings"
	"testing"
	"time"
)

const (
	head40 = "0123456789abcdef0123456789abcdef01234567"
	head64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func fixedTime() time.Time { return time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC) }

func TestNewRecordAccepts40Hex(t *testing.T) {
	r, err := NewRecord("go test ./...", head40, fixedTime())
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	if r.Command != "go test ./..." {
		t.Errorf("command = %q", r.Command)
	}
	if r.Result != ResultGreen {
		t.Errorf("result = %q, want green", r.Result)
	}
	if r.Head != head40 {
		t.Errorf("head = %q, want %q", r.Head, head40)
	}
	if !r.RanAt.Equal(fixedTime()) {
		t.Errorf("ranAt = %v", r.RanAt)
	}
}

func TestNewRecordAccepts64Hex(t *testing.T) {
	r, err := NewRecord("make check", head64, fixedTime())
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	if r.Head != head64 {
		t.Errorf("head = %q, want %q", r.Head, head64)
	}
}

func TestNewRecordNormalizesHeadToLowercase(t *testing.T) {
	r, err := NewRecord("cmd", strings.ToUpper(head40), fixedTime())
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	if r.Head != head40 {
		t.Errorf("head = %q, want normalized %q", r.Head, head40)
	}
}

func TestNewRecordCommandWithColonAndUnicode(t *testing.T) {
	cmd := "go test: run é ./..."
	r, err := NewRecord(cmd, head40, fixedTime())
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	if r.Command != cmd {
		t.Errorf("command = %q, want %q", r.Command, cmd)
	}
}

func TestNewRecordNormalizesTimeToUTCSeconds(t *testing.T) {
	loc := time.FixedZone("PST", -8*3600)
	in := time.Date(2026, 8, 16, 4, 0, 0, 500_000_000, loc) // 12:00:00.5 UTC
	r, err := NewRecord("cmd", head40, in)
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	want := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	if !r.RanAt.Equal(want) {
		t.Errorf("ranAt = %v, want %v", r.RanAt, want)
	}
	if r.RanAt.Location() != time.UTC {
		t.Errorf("location = %v, want UTC", r.RanAt.Location())
	}
	if r.RanAt.Nanosecond() != 0 {
		t.Errorf("nanoseconds not truncated: %d", r.RanAt.Nanosecond())
	}
}

func TestNewRecordRejects(t *testing.T) {
	cases := []struct {
		name    string
		command string
		head    string
	}{
		{"empty command", "", head40},
		{"newline in command", "go\ntest", head40},
		{"tab in command", "go\ttest", head40},
		{"nul in command", "go\x00test", head40},
		{"leading space command", " cmd", head40},
		{"trailing space command", "cmd ", head40},
		{"39-hex head", "cmd", head40[:39]},
		{"41-hex head", "cmd", head40 + "a"},
		{"63-hex head", "cmd", head64[:63]},
		{"65-hex head", "cmd", head64 + "a"},
		{"non-hex head", "cmd", strings.Repeat("g", 40)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewRecord(tc.command, tc.head, fixedTime()); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestNewRecordRejectsInvalidUTF8Command(t *testing.T) {
	if _, err := NewRecord(string([]byte{0xff, 0xfe}), head40, fixedTime()); err == nil {
		t.Fatal("expected error for invalid UTF-8 command")
	}
}
