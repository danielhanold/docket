package gitcli

import (
	"bytes"
	"strings"
	"testing"
)

// zero40 is git's all-zero SHA-1 sentinel for an absent side of a raw diff
// record; oidA/oidB are two distinct, valid full-hex ids the fixtures reuse.
const (
	zero40 = "0000000000000000000000000000000000000000"
	oidA   = "1111111111111111111111111111111111111111"
	oidB   = "2222222222222222222222222222222222222222"
)

// rawRecord builds one NUL-terminated header/path pair as
// `diff-tree -r -z --no-renames --full-index` emits it:
// ":<oldmode> <newmode> <oldoid> <newoid> <status>\x00<path>\x00".
func rawRecord(oldMode, newMode, oldOID, newOID, status, path string) []byte {
	var b bytes.Buffer
	b.WriteByte(':')
	b.WriteString(oldMode + " " + newMode + " " + oldOID + " " + newOID + " " + status)
	b.WriteByte(0)
	b.WriteString(path)
	b.WriteByte(0)
	return b.Bytes()
}

// TestParseDiffTreeRawZ pins parseDiffTreeRawZ's record discipline against
// literal NUL-delimited streams: the four --no-renames statuses (including a
// mode-only change and a delete carrying the all-zero new side), a hostile
// path (embedded newline + non-UTF8 byte), an empty stream, and every
// malformed shape returning an error with zero entries.
func TestParseDiffTreeRawZ(t *testing.T) {
	t.Run("empty-stream", func(t *testing.T) {
		got, err := parseDiffTreeRawZ(nil)
		if err != nil {
			t.Fatalf("empty stream: err = %v, want nil", err)
		}
		if got != nil {
			t.Fatalf("empty stream: entries = %v, want nil", got)
		}
	})

	t.Run("add", func(t *testing.T) {
		out := rawRecord("000000", "100644", zero40, oidA, "A", "new.txt")
		got, err := parseDiffTreeRawZ(out)
		if err != nil {
			t.Fatalf("add: %v", err)
		}
		want := deltaEntry{Path: "new.txt", Status: 'A', NewMode: "100644", NewOID: oidA}
		if len(got) != 1 || got[0] != want {
			t.Fatalf("add: got %+v, want [%+v]", got, want)
		}
	})

	t.Run("modify", func(t *testing.T) {
		out := rawRecord("100644", "100644", oidA, oidB, "M", "mod.txt")
		got, err := parseDiffTreeRawZ(out)
		if err != nil {
			t.Fatalf("modify: %v", err)
		}
		want := deltaEntry{Path: "mod.txt", Status: 'M', NewMode: "100644", NewOID: oidB}
		if len(got) != 1 || got[0] != want {
			t.Fatalf("modify: got %+v, want [%+v]", got, want)
		}
	})

	t.Run("delete-all-zero-new-side", func(t *testing.T) {
		out := rawRecord("100644", "000000", oidA, zero40, "D", "gone.txt")
		got, err := parseDiffTreeRawZ(out)
		if err != nil {
			t.Fatalf("delete: %v", err)
		}
		want := deltaEntry{Path: "gone.txt", Status: 'D', NewMode: "000000", NewOID: zero40}
		if len(got) != 1 || got[0] != want {
			t.Fatalf("delete: got %+v, want [%+v]", got, want)
		}
	})

	t.Run("mode-only-change-same-oid", func(t *testing.T) {
		out := rawRecord("100644", "100755", oidA, oidA, "M", "exe.sh")
		got, err := parseDiffTreeRawZ(out)
		if err != nil {
			t.Fatalf("mode change: %v", err)
		}
		want := deltaEntry{Path: "exe.sh", Status: 'M', NewMode: "100755", NewOID: oidA}
		if len(got) != 1 || got[0] != want {
			t.Fatalf("mode change: got %+v, want [%+v]", got, want)
		}
	})

	t.Run("type-change", func(t *testing.T) {
		out := rawRecord("100644", "120000", oidA, oidB, "T", "link")
		got, err := parseDiffTreeRawZ(out)
		if err != nil {
			t.Fatalf("type change: %v", err)
		}
		want := deltaEntry{Path: "link", Status: 'T', NewMode: "120000", NewOID: oidB}
		if len(got) != 1 || got[0] != want {
			t.Fatalf("type change: got %+v, want [%+v]", got, want)
		}
	})

	t.Run("hostile-path-newline-and-non-utf8", func(t *testing.T) {
		hostile := "dir/we\nird\xff\x80name"
		out := rawRecord("000000", "100644", zero40, oidA, "A", hostile)
		got, err := parseDiffTreeRawZ(out)
		if err != nil {
			t.Fatalf("hostile path: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("hostile path: got %d entries, want 1", len(got))
		}
		if string(got[0].Path) != hostile {
			t.Fatalf("hostile path: Path = %q, want %q (verbatim bytes)", got[0].Path, hostile)
		}
	})

	t.Run("multiple-records-in-order", func(t *testing.T) {
		out := bytes.Join([][]byte{
			rawRecord("000000", "100644", zero40, oidA, "A", "a"),
			rawRecord("100644", "000000", oidB, zero40, "D", "b"),
		}, nil)
		got, err := parseDiffTreeRawZ(out)
		if err != nil {
			t.Fatalf("multi: %v", err)
		}
		if len(got) != 2 || got[0].Path != "a" || got[1].Path != "b" {
			t.Fatalf("multi: got %+v, want records for a then b", got)
		}
	})

	// Every malformed shape must error with zero entries.
	malformed := []struct {
		name string
		out  []byte
	}{
		{
			name: "unterminated-trailing-record-missing-nul",
			out:  []byte(":100644 100644 " + oidA + " " + oidB + " M\x00mod.txt"),
		},
		{
			name: "short-header-three-fields",
			out:  []byte(":100644 100644 M\x00mod.txt\x00"),
		},
		{
			name: "bad-oid-non-hex",
			out:  rawRecord("100644", "100644", oidA, "not-a-valid-object-id-zzzz", "M", "mod.txt"),
		},
		{
			name: "header-without-path-odd-records",
			out:  []byte(":100644 100644 " + oidA + " " + oidB + " M\x00"),
		},
		{
			name: "all-zero-new-oid-on-non-deletion",
			out:  rawRecord("000000", "100644", zero40, zero40, "A", "new.txt"),
		},
		{
			name: "status-outside-no-rename-set",
			out:  rawRecord("100644", "100644", oidA, oidB, "R", "moved.txt"),
		},
		{
			name: "header-missing-leading-colon",
			out:  []byte("100644 100644 " + oidA + " " + oidB + " M\x00mod.txt\x00"),
		},
		{
			name: "empty-path",
			out:  []byte(":100644 100644 " + oidA + " " + oidB + " M\x00\x00"),
		},
	}
	for _, tc := range malformed {
		t.Run("malformed/"+tc.name, func(t *testing.T) {
			got, err := parseDiffTreeRawZ(tc.out)
			if err == nil {
				t.Fatalf("%s: err = nil, want an error", tc.name)
			}
			if got != nil {
				t.Fatalf("%s: entries = %+v, want nil on error", tc.name, got)
			}
		})
	}
}

// TestParseDiffTreeRawZDetail keeps a tiny guard that a plausible-looking but
// invalid stream is not silently swallowed: a truncated header field count is
// reported, never coerced into a partial entry.
func TestParseDiffTreeRawZDetail(t *testing.T) {
	out := []byte(":100644 " + oidA + " " + oidB + " M\x00mod.txt\x00")
	_, err := parseDiffTreeRawZ(out)
	if err == nil || !strings.Contains(err.Error(), "five fields") {
		t.Fatalf("four-field header: err = %v, want a five-fields error", err)
	}
}
