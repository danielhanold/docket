package cli

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
)

// stubExport replaces the export operation and its dependency wiring for one
// test, recording every ExportOptions the command passes and returning res.
func stubExport(t *testing.T, res app.ExportResult) *[]app.ExportOptions {
	t.Helper()
	var got []app.ExportOptions
	prevRunner, prevDeps := exportRunner, openDepsFor
	t.Cleanup(func() { exportRunner, openDepsFor = prevRunner, prevDeps })
	openDepsFor = func() (app.OpenDeps, error) { return app.OpenDeps{}, nil }
	exportRunner = func(_ context.Context, _ app.OpenDeps, o app.ExportOptions) app.OperationResult {
		got = append(got, o)
		return res
	}
	return &got
}

// exportOK is a successful spec export to path carrying notes.
func exportOK(path string, notes ...string) app.ExportResult {
	if notes == nil {
		notes = []string{}
	}
	return app.ExportResult{Envelope: app.NewEnvelope(app.OperationExport, app.ResultApplied), What: "spec", ChangeID: 7,
		ExportedPath: path, Notes: notes}
}

func TestExportCLIPassesParsedOptions(t *testing.T) {
	got := stubExport(t, exportOK("/tmp/x/2026-01-02-spec-widget.md"))
	for _, args := range [][]string{
		{"export", "spec", "541", "--output-path", "/tmp/x", "--warnings", "leftovers", "--print", "--repo-dir", "/tmp/repo"},
		{"export", "spec", "0541", "--repo-dir", "/tmp/repo"},
		{"export", "plan", "--repo-dir", "/tmp/repo"},
	} {
		if _, errS, code := runCLI(t, args...); code != 0 {
			t.Fatalf("%q: code %d stderr %q", args, code, errS)
		}
	}
	want := []app.ExportOptions{
		{RepoDir: "/tmp/repo", What: "spec", ID: 541, Print: true, OutputPath: "/tmp/x", Warnings: "leftovers"},
		{RepoDir: "/tmp/repo", What: "spec", ID: 541},
		{RepoDir: "/tmp/repo", What: "plan"},
	}
	if len(*got) != len(want) {
		t.Fatalf("options = %+v, want %+v", *got, want)
	}
	for i := range want {
		if (*got)[i] != want[i] {
			t.Errorf("options[%d] = %+v, want %+v", i, (*got)[i], want[i])
		}
	}
}

// TestExportCLIUsageErrors: every usage mistake is a CLI usage error caught
// before the operation runs — never an export operation document.
func TestExportCLIUsageErrors(t *testing.T) {
	got := stubExport(t, exportOK("x"))
	for _, args := range [][]string{
		{"export"}, {"export", "board"}, {"export", "pr", "5"}, {"export", "bogus"},
		{"export", "spec", "#541"}, {"export", "spec", "0"}, {"export", "spec", "1", "2"},
		{"export", "spec", "5", "--warnings", "bogus"},
		{"export", "spec", "5", "--output-path", ""},
	} {
		if _, errS, code := runCLI(t, args...); code != 2 || errS == "" || strings.Contains(strings.TrimRight(errS, "\n"), "\n") {
			t.Errorf("%q: code %d stderr %q, want exit 2 and one line", args, code, errS)
		}
	}
	out, _, code := runCLI(t, "--json", "export", "board")
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil || code != 2 {
		t.Fatalf("--json export board: code %d stdout %q (%v)", code, out, err)
	}
	if doc["reason"] != app.ReasonInvalidArguments || doc["operation"] == app.OperationExport {
		t.Errorf("--json export board = %s, want the CLI's %s document", out, app.ReasonInvalidArguments)
	}
	if len(*got) != 0 {
		t.Errorf("usage errors reached the operation: %+v", *got)
	}
}

// TestExportCLIPrintsThePath: stdout is exactly the exported path, so it
// composes in command substitution; notes go to stderr.
func TestExportCLIPrintsThePath(t *testing.T) {
	stubExport(t, exportOK("/d/2026-01-02-spec-widget.md", "replaced /d/2026-01-02-spec-widget.md"))
	out, errS, code := runCLI(t, "export", "spec", "7", "--print", "--repo-dir", "/tmp/repo")
	if code != 0 || out != "/d/2026-01-02-spec-widget.md\n" || errS != "note: replaced /d/2026-01-02-spec-widget.md\n" {
		t.Fatalf("code %d stdout %q stderr %q", code, out, errS)
	}
}

// TestExportCLIDepsWiring pins the production wiring the export needs: the app
// layer refuses an export when any of these is unset.
func TestExportCLIDepsWiring(t *testing.T) {
	d, err := openDepsFor()
	if err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	if d.GOOS != runtime.GOOS || d.HomeDir == nil || d.Getwd == nil {
		t.Fatalf("GOOS %q (want %q), HomeDir set %v, Getwd set %v", d.GOOS, runtime.GOOS, d.HomeDir != nil, d.Getwd != nil)
	}
}

// TestExportCLIHelpNamesTicketKind pins that the copy's kind list names the
// ticket kind, never the retired issue kind.
func TestExportCLIHelpNamesTicketKind(t *testing.T) {
	out, errS, code := runCLI(t, "export", "--help")
	if code != 0 {
		t.Fatalf("export --help exit %d, stderr %q", code, errS)
	}
	if !strings.Contains(out, "kind one of ticket, spec, plan, or") {
		t.Errorf("export --help must name the ticket kind:\n%s", out)
	}
	if strings.Contains(out, "one of issue") {
		t.Errorf("export --help still names the retired issue kind:\n%s", out)
	}
}
