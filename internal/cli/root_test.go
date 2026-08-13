package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/buildinfo"
)

func devInfo() buildinfo.Info {
	return buildinfo.Info{Version: "development", Commit: "unknown", BuildDate: "unknown"}
}

func hostFacts() buildinfo.RuntimeFacts {
	return buildinfo.RuntimeFacts{GoVersion: "go1.26.5", GOOS: "darwin", GOARCH: "arm64"}
}

func runCLI(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = Run(args, strings.NewReader(""), &out, &errBuf, devInfo(), hostFacts())
	return out.String(), errBuf.String(), code
}

func TestVersionHuman(t *testing.T) {
	out, errS, code := runCLI(t, "version")
	if code != 0 || errS != "" || out != "docket development (commit unknown, built unknown)\n" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
}

func TestVersionJSONFlagPositions(t *testing.T) {
	want := `{"protocol_version":1,"operation":"version","result":"applied","version":"development","commit":"unknown","build_date":"unknown"}` + "\n"
	for _, args := range [][]string{{"--json", "version"}, {"version", "--json"}} {
		out, errS, code := runCLI(t, args...)
		if code != 0 || errS != "" || out != want {
			t.Fatalf("args=%v out=%q err=%q code=%d", args, out, errS, code)
		}
	}
}

func TestVersionJSONFalseIsHuman(t *testing.T) {
	out, errS, code := runCLI(t, "version", "--json=false")
	if code != 0 || errS != "" || out != "docket development (commit unknown, built unknown)\n" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
}

func TestDiagnosticRuntimeJSON(t *testing.T) {
	out, errS, code := runCLI(t, "diagnostic", "runtime", "--json")
	want := `{"protocol_version":1,"operation":"diagnostic.runtime","result":"applied","go_version":"go1.26.5","go_os":"darwin","go_arch":"arm64","supported_target":true}` + "\n"
	if code != 0 || errS != "" || out != want {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
}

func TestHostileParseOrderStillJSON(t *testing.T) {
	// --json AFTER the failing token: the transport scan, not Cobra, must
	// select JSON mode, or this emits a human error.
	out, errS, code := runCLI(t, "version", "--bogus", "--json")
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if errS != "" {
		t.Fatalf("stderr must be empty in JSON mode, got %q", errS)
	}
	if !strings.HasPrefix(out, `{"protocol_version":1,"operation":"cli","result":"invalid-input","reason":"invalid-arguments",`) {
		t.Fatalf("stdout = %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

func TestHumanParseErrorStdoutEmpty(t *testing.T) {
	out, errS, code := runCLI(t, "version", "--bogus")
	if code != 2 || out != "" {
		t.Fatalf("out=%q code=%d", out, code)
	}
	if !strings.HasPrefix(errS, "docket: ") {
		t.Fatalf("stderr = %q", errS)
	}
}

func TestJSONHelpConflictThreeForms(t *testing.T) {
	for _, args := range [][]string{
		{"--json", "--help"},
		{"--json", "-h"},
		{"--json", "help"},
	} {
		out, errS, code := runCLI(t, args...)
		if code != 2 {
			t.Fatalf("args=%v code=%d, want 2", args, code)
		}
		if errS != "" {
			t.Fatalf("args=%v stderr=%q, want empty", args, errS)
		}
		if !strings.Contains(out, `"reason":"json-help-conflict"`) {
			t.Fatalf("args=%v stdout=%q", args, out)
		}
		if strings.Contains(out, "Usage") {
			t.Fatalf("args=%v help text leaked into protocol stream: %q", args, out)
		}
	}
}

func TestHumanHelpIsolation(t *testing.T) {
	out, errS, code := runCLI(t, "--help")
	if code != 0 || errS != "" {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	if !strings.Contains(out, "Usage") {
		t.Fatalf("human help must render Cobra text, got %q", out)
	}
}

func TestMissingCommand(t *testing.T) {
	out, errS, code := runCLI(t, "--json")
	if code != 2 || errS != "" {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	if !strings.Contains(out, `"result":"invalid-input"`) {
		t.Fatalf("stdout = %q", out)
	}
}

func TestExtraArgumentRejected(t *testing.T) {
	_, _, code := runCLI(t, "version", "extra")
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	out, errS, code := runCLI(t, "version", "extra", "--json")
	if code != 2 || errS != "" || !strings.Contains(out, `"result":"invalid-input"`) {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
}

func TestUnknownCommandJSON(t *testing.T) {
	out, errS, code := runCLI(t, "--json", "bogus")
	if code != 2 || errS != "" || !strings.Contains(out, `"result":"invalid-input"`) {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
}

func TestNoCompletionCommand(t *testing.T) {
	_, _, code := runCLI(t, "completion")
	if code == 0 {
		t.Fatal("default completion command must be disabled")
	}
	out, _, _ := runCLI(t, "--help")
	if strings.Contains(out, "completion") {
		t.Fatalf("help must not advertise a completion command: %q", out)
	}
}
