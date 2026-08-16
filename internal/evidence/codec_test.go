package evidence

import (
	"errors"
	"strings"
	"testing"
)

// mustRecord builds a valid record for tests.
func mustRecord(t *testing.T, command, head string) Record {
	t.Helper()
	r, err := NewRecord(command, head, fixedTime())
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	return r
}

func TestRenderExtractRoundTrip(t *testing.T) {
	for _, head := range []string{head40, head64} {
		r := mustRecord(t, "go test ./...", head)
		block := Render(r)
		got, err := Extract([]byte(block))
		if err != nil {
			t.Fatalf("Extract(%q): %v", head, err)
		}
		if got != r {
			t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, r)
		}
	}
}

func TestRenderIsCanonicalLF(t *testing.T) {
	r := mustRecord(t, "go test ./...", head40)
	block := Render(r)
	if strings.Contains(block, "\r") {
		t.Errorf("canonical render contains CR: %q", block)
	}
	if !strings.HasPrefix(block, "<!-- docket:build-evidence:start -->\n") {
		t.Errorf("render start = %q", block)
	}
	if !strings.HasSuffix(block, "\n<!-- docket:build-evidence:end -->") {
		t.Errorf("render end = %q", block)
	}
}

func TestExtractCommandWithColons(t *testing.T) {
	r := mustRecord(t, "env A=1 go test: ./... -run X:Y", head40)
	got, err := Extract([]byte(Render(r)))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got.Command != r.Command {
		t.Errorf("command = %q, want %q", got.Command, r.Command)
	}
}

func TestExtractAcceptsCRLF(t *testing.T) {
	r := mustRecord(t, "go test ./...", head40)
	crlf := strings.ReplaceAll(Render(r), "\n", "\r\n")
	got, err := Extract([]byte(crlf))
	if err != nil {
		t.Fatalf("Extract CRLF: %v", err)
	}
	if got != r {
		t.Errorf("CRLF round trip mismatch: got %+v want %+v", got, r)
	}
}

func TestExtractInBiggerBody(t *testing.T) {
	r := mustRecord(t, "go test ./...", head40)
	body := "# PR title\n\nSome prose describing the change.\n\n" +
		Render(r) + "\n\nMore prose after.\n"
	got, err := Extract([]byte(body))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got != r {
		t.Errorf("mismatch: got %+v want %+v", got, r)
	}
}

func TestExtractMissing(t *testing.T) {
	body := []byte("# just prose\n\nno evidence here.\n")
	_, err := Extract(body)
	if !errors.Is(err, ErrMissing) {
		t.Fatalf("err = %v, want ErrMissing", err)
	}
}

func TestExtractFenceExampleIgnored(t *testing.T) {
	// A fenced code block that merely SHOWS the marker pair must not be read as
	// a real block: fenced content is example text.
	body := "Here is what an evidence block looks like:\n\n" +
		"```\n" +
		"<!-- docket:build-evidence:start -->\n" +
		"command:  go test ./...\n" +
		"<!-- docket:build-evidence:end -->\n" +
		"```\n"
	_, err := Extract([]byte(body))
	if !errors.Is(err, ErrMissing) {
		t.Fatalf("fenced example should be ErrMissing, got %v", err)
	}
}

func TestExtractFenceExamplePlusRealBlock(t *testing.T) {
	r := mustRecord(t, "go test ./...", head64)
	body := "Example:\n\n```\n<!-- docket:build-evidence:start -->\nx\n<!-- docket:build-evidence:end -->\n```\n\n" +
		Render(r) + "\n"
	got, err := Extract([]byte(body))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got != r {
		t.Errorf("mismatch: got %+v want %+v", got, r)
	}
}

// malformedBody returns a body carrying a single build-evidence block whose
// interior lines are the given lines (LF-joined), for the interior-shape matrix.
func evidenceWith(lines ...string) []byte {
	return []byte("<!-- docket:build-evidence:start -->\n" +
		strings.Join(lines, "\n") + "\n" +
		"<!-- docket:build-evidence:end -->\n")
}

func TestExtractMalformedInterior(t *testing.T) {
	good := []string{
		"command:  go test ./...",
		"result:   green",
		"head_sha: " + head40,
		"ran_at:   2026-08-16T12:00:00Z",
	}
	cases := map[string][]string{
		"duplicate key": {
			"command:  a", "command:  b", "result:   green",
			"head_sha: " + head40, "ran_at:   2026-08-16T12:00:00Z",
		},
		"missing key": {
			"command:  a", "result:   green", "head_sha: " + head40,
		},
		"unknown key":         append(append([]string{}, good...), "extra:    x"),
		"nonblank stray line": append(append([]string{}, good...), "just some words"),
		"result red": {
			"command:  a", "result:   red",
			"head_sha: " + head40, "ran_at:   2026-08-16T12:00:00Z",
		},
		"short head": {
			"command:  a", "result:   green",
			"head_sha: " + head40[:39], "ran_at:   2026-08-16T12:00:00Z",
		},
		"bad timestamp": {
			"command:  a", "result:   green",
			"head_sha: " + head40, "ran_at:   not-a-time",
		},
	}
	for name, lines := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Extract(evidenceWith(lines...))
			if err == nil {
				t.Fatalf("expected malformed error")
			}
			if errors.Is(err, ErrMissing) {
				t.Fatalf("malformed must not be ErrMissing: %v", err)
			}
		})
	}
	// sanity: the good interior parses
	if _, err := Extract(evidenceWith(good...)); err != nil {
		t.Fatalf("good interior should parse: %v", err)
	}
}

func TestExtractMalformedMarkers(t *testing.T) {
	r := mustRecord(t, "go test ./...", head40)
	block := Render(r)
	cases := map[string][]byte{
		"two blocks":     []byte(block + "\n\n" + block + "\n"),
		"dangling start": []byte("<!-- docket:build-evidence:start -->\ncommand:  a\n"),
		"dangling end":   []byte("command:  a\n<!-- docket:build-evidence:end -->\n"),
		"out of order": []byte("<!-- docket:build-evidence:end -->\n" +
			"<!-- docket:build-evidence:start -->\n"),
		"nested foreign marker": []byte("<!-- docket:build-evidence:start -->\n" +
			"<!-- docket:backlink:start -->\ncommand:  a\n" +
			"<!-- docket:build-evidence:end -->\n"),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Extract(body)
			if err == nil {
				t.Fatalf("expected error")
			}
			if errors.Is(err, ErrMissing) {
				t.Fatalf("marker imbalance must not be ErrMissing: %v", err)
			}
		})
	}
}
