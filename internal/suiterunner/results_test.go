package suiterunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteResultRoundTripsAtomically(t *testing.T) {
	dir := t.TempDir()
	want := Result{Schema: 1, Target: "test_x.sh", RC: 0, Seconds: 3, OK: 4, NotOK: 0}
	if err := WriteResult(dir, want); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	// The file lands under the .sh-stripped stem, and no temp scratch survives.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("stat dir has %d entries %v, want exactly 1 (no temp leftovers)", len(entries), names)
	}
	if entries[0].Name() != "test_x.json" {
		t.Fatalf("result filename = %q, want test_x.json", entries[0].Name())
	}

	got, err := ReadResult(filepath.Join(dir, "test_x.json"))
	if err != nil {
		t.Fatalf("ReadResult: %v", err)
	}
	if got != want {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestReadResultFailureModes(t *testing.T) {
	dir := t.TempDir()

	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
		return p
	}

	trunc := write("test_trunc.json", `{"schema":1,"target":"test_trunc.sh"`)
	badSchema := write("test_schema.json", `{"schema":2,"target":"test_schema.sh"}`)
	emptyTarget := write("test_empty.json", `{"schema":1,"target":""}`)
	missing := filepath.Join(dir, "test_absent.json")

	cases := []struct {
		name string
		path string
		want string // substring; "" means only "must error"
	}{
		{"truncated json", trunc, "malformed"},
		{"wrong schema", badSchema, "unsupported schema"},
		{"empty target", emptyTarget, "missing target identity"},
		{"unreadable file", missing, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ReadResult(c.path)
			if err == nil {
				t.Fatalf("ReadResult(%s) = nil error, want failure", c.name)
			}
			if c.want != "" && !strings.Contains(err.Error(), c.want) {
				t.Fatalf("ReadResult(%s) error %q, want substring %q", c.name, err.Error(), c.want)
			}
		})
	}
}
