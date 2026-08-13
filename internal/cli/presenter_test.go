package cli

import (
	"bytes"
	"testing"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/buildinfo"
)

func TestPresentJSONWritesOneCompactDocument(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := Presenter{Stdout: &out, Stderr: &errBuf, JSON: true}
	code := p.Present(app.Version(buildinfo.Info{Version: "development", Commit: "unknown", BuildDate: "unknown"}))
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	want := `{"protocol_version":1,"operation":"version","result":"applied","version":"development","commit":"unknown","build_date":"unknown"}` + "\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("stderr must be empty, got %q", errBuf.String())
	}
}

func TestPresentHumanTextLine(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := Presenter{Stdout: &out, Stderr: &errBuf, JSON: false}
	code := p.Present(app.Version(buildinfo.Info{Version: "development", Commit: "unknown", BuildDate: "unknown"}))
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out.String() != "docket development (commit unknown, built unknown)\n" {
		t.Fatalf("stdout = %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("stderr must be empty, got %q", errBuf.String())
	}
}

func TestPresentJSONCLIErrorGoesToStdoutOnly(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := Presenter{Stdout: &out, Stderr: &errBuf, JSON: true}
	code := p.Present(app.CLIError(app.ReasonInvalidArguments, "unknown flag: --bogus"))
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	want := `{"protocol_version":1,"operation":"cli","result":"invalid-input","reason":"invalid-arguments","message":"unknown flag: --bogus"}` + "\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("a handled JSON result must not duplicate on stderr, got %q", errBuf.String())
	}
}

func TestPresentHumanErrorGoesToStderrOnly(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := Presenter{Stdout: &out, Stderr: &errBuf, JSON: false}
	code := p.PresentHumanError(app.CLIError(app.ReasonInvalidArguments, "unknown flag: --bogus"))
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout must be empty on human parse failure, got %q", out.String())
	}
	if errBuf.String() != "docket: unknown flag: --bogus\n" {
		t.Fatalf("stderr = %q", errBuf.String())
	}
}
