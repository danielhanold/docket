package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
)

// stubOpen replaces the open operation and its dependency wiring for one test,
// recording every OpenOptions the command passes and returning res.
func stubOpen(t *testing.T, res app.OpenResult) *[]app.OpenOptions {
	t.Helper()
	var got []app.OpenOptions
	prevRunner, prevDeps := openRunner, openDepsFor
	t.Cleanup(func() { openRunner, openDepsFor = prevRunner, prevDeps })
	openDepsFor = func() (app.OpenDeps, error) { return app.OpenDeps{}, nil }
	openRunner = func(_ context.Context, _ app.OpenDeps, o app.OpenOptions) app.OperationResult {
		got = append(got, o)
		return res
	}
	return &got
}

// openOK is a successful spec open of target carrying notes.
func openOK(target string, notes ...string) app.OpenResult {
	if notes == nil {
		notes = []string{}
	}
	return app.OpenResult{Envelope: app.NewEnvelope(app.OperationOpen, app.ResultApplied), What: "spec", ChangeID: 541,
		Target: target, TargetKind: app.OpenTargetKindURL, Notes: notes}
}

func TestOpenCLIPassesParsedOptions(t *testing.T) {
	got := stubOpen(t, openOK("https://example.test/s"))
	if _, errS, code := runCLI(t, "open", "spec", "0541", "--print", "--repo-dir", "/tmp/repo"); code != 0 {
		t.Fatalf("code %d stderr %q", code, errS)
	}
	if _, _, code := runCLI(t, "open", "plan", "--repo-dir", "/tmp/repo"); code != 0 {
		t.Fatalf("inference code %d", code)
	}
	want := []app.OpenOptions{{RepoDir: "/tmp/repo", What: "spec", ID: 541, Print: true}, {RepoDir: "/tmp/repo", What: "plan"}}
	if len(*got) != 2 || (*got)[0] != want[0] || (*got)[1] != want[1] {
		t.Fatalf("options = %+v, want %+v", *got, want)
	}
}

// TestOpenCLIPrintNotesGoToStderr pins that `--print` output composes in
// command substitution: stdout is exactly the target, notes go to stderr.
func TestOpenCLIPrintNotesGoToStderr(t *testing.T) {
	stubOpen(t, openOK("/m/docs/spec.md", "origin is not a GitHub remote — opened the local file instead"))
	out, errS, code := runCLI(t, "open", "spec", "7", "--print", "--repo-dir", "/tmp/repo")
	if code != 0 || out != "/m/docs/spec.md\n" || errS != "note: origin is not a GitHub remote — opened the local file instead\n" {
		t.Fatalf("code %d stdout %q stderr %q", code, out, errS)
	}
}

func TestOpenCLIUsageErrors(t *testing.T) {
	got := stubOpen(t, openOK("x"))
	for _, args := range [][]string{{"open"}, {"open", "bogus"}, {"open", "board", "5"}, {"open", "spec", "#541"}, {"open", "spec", "0"}, {"open", "spec", "1", "2"}} {
		if _, errS, code := runCLI(t, args...); code != 2 || strings.Contains(strings.TrimRight(errS, "\n"), "\n") {
			t.Errorf("%q: code %d stderr %q, want exit 2 and one line", args, code, errS)
		}
	}
	_, errS, _ := runCLI(t, "open", "bogus")
	for _, w := range app.OpenTargets {
		if !strings.Contains(errS, w) {
			t.Errorf("unknown-target error %q does not list %q", errS, w)
		}
	}
	if _, errS, _ := runCLI(t, "open", "spec", "#541"); !strings.Contains(errS, "invalid change id #541") {
		t.Errorf("stderr = %q", errS)
	}
	if len(*got) != 0 {
		t.Errorf("usage errors reached the operation: %+v", *got)
	}
}
