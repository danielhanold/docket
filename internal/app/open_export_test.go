package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// retitle gives a fixture record a real title in place of openChange's
// "Change N", which title cleaning would itself treat as a change reference.
func retitle(b StatusBlob, title string) StatusBlob {
	b.Data = regexp.MustCompile(`(?m)^title: .*$`).ReplaceAll(b.Data, []byte("title: "+title))
	return b
}

// exportCorpus is openCorpus with change 3 and change 7 carrying real titles.
func exportCorpus() []StatusBlob {
	blobs := openCorpus()
	for i, b := range blobs {
		switch {
		case strings.Contains(b.Path, "0003-old"):
			blobs[i] = retitle(b, "Old gadget")
		case strings.Contains(b.Path, "0007-widget"):
			blobs[i] = retitle(b, "Widget export")
		}
	}
	return blobs
}

const exportSpecBody = "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
	"> ↩ **[Change 0007 — Widget export](../../changes/active/0007-widget.md)**\n" +
	"<!-- docket:backlink:end -->\n\n" +
	"# Widget design\n\nBuilt after change 0003, see [the notes](https://example.test/n).\n"

const exportSpecClean = "# Widget design\n\nBuilt after \"Old gadget\", see the notes.\n"

// exportDeps is the fixture's deps plus the export seams: home is the home
// directory, GOOS the platform, and the working directory is cwd.
func (f *openFixture) exportDeps(home, goos, cwd string) OpenDeps {
	d := f.deps()
	d.HomeDir = func() (string, error) { return home, nil }
	d.GOOS = goos
	d.Getwd = func() (string, error) { return cwd, nil }
	return d
}

func (f *openFixture) export(t *testing.T, deps OpenDeps, what string, id int, mut func(*OpenOptions)) OpenResult {
	t.Helper()
	o := OpenOptions{RepoDir: f.repoDir, What: what, ID: id, Export: true}
	if mut != nil {
		mut(&o)
	}
	return Open(context.Background(), deps, o)
}

func readExport(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestOpenExportWritesCleanCopy(t *testing.T) {
	f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
	f.writeBody(t, widgetSpec, exportSpecBody)
	out := testsupport.TempDir(t)
	res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "spec", 7, func(o *OpenOptions) {
		o.Print, o.OutputPath = true, out
	})
	want := filepath.Join(out, "2026-01-02-spec-widget.md")
	if res.Result != ResultApplied || res.Launched || res.ExportedPath != want || res.Target != want ||
		res.TargetKind != OpenTargetKindFile || len(f.opener.targets) != 0 {
		t.Fatalf("Open = (%s, launched %v, exported %q, target %q, %s, %q)", res.Result, res.Launched, res.ExportedPath, res.Target, res.TargetKind, res.Message)
	}
	if got := readExport(t, want); got != exportSpecClean {
		t.Errorf("exported bytes = %q, want %q", got, exportSpecClean)
	}
	info, err := os.Stat(want)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v (%v), want 0644", info, err)
	}
	if res.Replaced || len(res.Leftovers) != 0 || res.Mentions == nil || res.Mentions.Count != 0 {
		t.Errorf("replaced %v leftovers %v mentions %+v", res.Replaced, res.Leftovers, res.Mentions)
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != 1 {
		t.Errorf("destination holds %d entries, want only the export (no temp file left)", len(entries))
	}
}

func TestOpenExportOpensTheCopy(t *testing.T) {
	f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
	f.writeBody(t, widgetSpec, exportSpecBody)
	out := testsupport.TempDir(t)
	res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "spec", 7, func(o *OpenOptions) { o.OutputPath = out })
	want := filepath.Join(out, "2026-01-02-spec-widget.md")
	if res.Result != ResultApplied || !res.Launched || !reflect.DeepEqual(f.opener.targets, []string{want}) {
		t.Errorf("(%s, launched %v, opener %v), want the exported copy opened", res.Result, res.Launched, f.opener.targets)
	}
}

// TestOpenExportIgnoresGitHubRouting: a shared repository with a GitHub origin
// and the default open.artifacts still exports the local metadata-checkout file.
func TestOpenExportIgnoresGitHubRouting(t *testing.T) {
	f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
	f.writeBody(t, widgetSpec, exportSpecBody)
	out := testsupport.TempDir(t)
	res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "spec", 7, func(o *OpenOptions) { o.OutputPath, o.Print = out, true })
	if res.Result != ResultApplied || res.TargetKind != OpenTargetKindFile || !strings.HasPrefix(res.Target, out) || len(f.prepared) != 1 {
		t.Errorf("(%s, %s, %q) prepared %v", res.Result, res.TargetKind, res.Target, f.prepared)
	}
	if !reflect.DeepEqual(res.Notes, []string{}) {
		t.Errorf("notes %q, want none (no not-GitHub note on the export path)", res.Notes)
	}
}

func TestOpenExportKeepsStalenessNote(t *testing.T) {
	f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
	f.refusePrepare("metadata-worktree-dirty")
	f.writeBody(t, widgetSpec, exportSpecBody)
	out := testsupport.TempDir(t)
	res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "spec", 7, func(o *OpenOptions) { o.OutputPath, o.Print = out, true })
	if res.Result != ResultApplied || !reflect.DeepEqual(res.Notes, []string{openNoteStale("metadata-worktree-dirty")}) {
		t.Errorf("(%s, %q)", res.Result, res.Notes)
	}
}

func TestOpenExportDefaultDestination(t *testing.T) {
	name := "2026-01-02-spec-widget.md"
	newF := func() *openFixture {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		f.writeBody(t, widgetSpec, exportSpecBody)
		return f
	}
	print := func(o *OpenOptions) { o.Print = true }

	home := testsupport.TempDir(t)
	if err := os.Mkdir(filepath.Join(home, "Downloads"), 0o755); err != nil {
		t.Fatal(err)
	}
	f := newF()
	if res := f.export(t, f.exportDeps(home, "darwin", "/nowhere"), "spec", 7, print); res.Result != ResultApplied ||
		res.ExportedPath != filepath.Join(home, "Downloads", name) {
		t.Errorf("darwin: (%s, %q, %q)", res.Result, res.ExportedPath, res.Message)
	}

	bare := testsupport.TempDir(t)
	f = newF()
	res := f.export(t, f.exportDeps(bare, "darwin", "/nowhere"), "spec", 7, print)
	if res.Result != ResultInvalidState || res.Reason != ReasonOpenOutputPathInvalid || !strings.Contains(res.Message, "--output-path") ||
		strings.Contains(res.Message, "\n") || res.ExportedPath != "" {
		t.Errorf("darwin without Downloads: (%s, %s, %q)", res.Result, res.Reason, res.Message)
	}
	if entries, _ := os.ReadDir(bare); len(entries) != 0 {
		t.Errorf("refusal wrote %d entries into home", len(entries))
	}

	linux := testsupport.TempDir(t)
	f = newF()
	if res := f.export(t, f.exportDeps(linux, "linux", "/nowhere"), "spec", 7, print); res.Result != ResultApplied ||
		res.ExportedPath != filepath.Join(linux, name) {
		t.Errorf("linux: (%s, %q, %q)", res.Result, res.ExportedPath, res.Message)
	}

	f = newF()
	d := f.exportDeps(linux, "linux", "/nowhere")
	d.HomeDir = func() (string, error) { return "", errors.New("no home") }
	if res := f.export(t, d, "spec", 7, print); res.Result == ResultApplied || res.ExportedPath != "" {
		t.Errorf("home error: (%s, %s, %q)", res.Result, res.Reason, res.Message)
	}
}

func TestOpenExportOutputPath(t *testing.T) {
	newF := func() *openFixture {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		f.writeBody(t, widgetSpec, exportSpecBody)
		return f
	}
	home := testsupport.TempDir(t)
	dir := testsupport.TempDir(t)
	at := func(p string) func(*OpenOptions) { return func(o *OpenOptions) { o.OutputPath, o.Print = p, true } }

	f := newF()
	if res := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, at(dir)); res.ExportedPath != filepath.Join(dir, "2026-01-02-spec-widget.md") {
		t.Errorf("dir: (%s, %q, %q)", res.Result, res.ExportedPath, res.Message)
	}

	exact := filepath.Join(dir, "shared.md")
	f = newF()
	if res := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, at(exact)); res.ExportedPath != exact || readExport(t, exact) != exportSpecClean {
		t.Errorf("file: (%s, %q, %q)", res.Result, res.ExportedPath, res.Message)
	}

	cwd := testsupport.TempDir(t)
	f = newF()
	if res := f.export(t, f.exportDeps(home, "linux", cwd), "spec", 7, at("out.md")); res.ExportedPath != filepath.Join(cwd, "out.md") {
		t.Errorf("relative: (%s, %q, %q)", res.Result, res.ExportedPath, res.Message)
	}

	missing := filepath.Join(dir, "no-such-dir", "x.md")
	f = newF()
	res := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, at(missing))
	if res.Result != ResultInvalidState || res.Reason != ReasonOpenOutputPathInvalid || strings.Contains(res.Message, "\n") {
		t.Errorf("missing parent: (%s, %s, %q)", res.Result, res.Reason, res.Message)
	}
	if _, err := os.Stat(filepath.Dir(missing)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing parent was created: %v", err)
	}

	again := testsupport.TempDir(t)
	f = newF()
	first := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, at(again))
	second := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, at(again))
	if first.Replaced || !second.Replaced || second.Result != ResultApplied ||
		!reflect.DeepEqual(second.Notes, []string{"replaced " + second.ExportedPath}) {
		t.Errorf("re-export: first replaced %v, second (%s, %v, %q)", first.Replaced, second.Result, second.Replaced, second.Notes)
	}
}

// TestOpenExportRefusesProtectedDestination: --output-path never overwrites the
// source artifact and never writes into the metadata checkout, however the path
// is spelled — exactly, relatively, as a directory, or through a symlinked
// parent. The refusal writes nothing and opens nothing.
func TestOpenExportRefusesProtectedDestination(t *testing.T) {
	home := testsupport.TempDir(t)
	cases := []struct {
		name string
		cwd  func(src string) string
		path func(src string) string
	}{
		{"source file", nil, func(src string) string { return src }},
		{"source file, unclean", nil, func(src string) string {
			return filepath.Join(filepath.Dir(src), "..", filepath.Base(filepath.Dir(src)), filepath.Base(src))
		}},
		{"source file, relative", func(src string) string { return filepath.Dir(src) }, func(src string) string { return filepath.Base(src) }},
		{"directory in the checkout", nil, func(src string) string { return filepath.Dir(src) }},
		{"new file in the checkout", nil, func(src string) string { return filepath.Join(filepath.Dir(src), "copy.md") }},
		{"symlinked parent into the checkout", nil, func(src string) string {
			link := filepath.Join(testsupport.TempDir(t), "meta-link")
			if err := os.Symlink(filepath.Dir(src), link); err != nil {
				t.Fatal(err)
			}
			return filepath.Join(link, filepath.Base(src))
		}},
	}
	for _, tc := range cases {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		src := f.writeBody(t, widgetSpec, exportSpecBody)
		cwd := "/nowhere"
		if tc.cwd != nil {
			cwd = tc.cwd(src)
		}
		res := f.export(t, f.exportDeps(home, "linux", cwd), "spec", 7, func(o *OpenOptions) { o.OutputPath = tc.path(src) })
		if res.Result != ResultInvalidState || res.Reason != ReasonOpenOutputPathProtected || res.ExportedPath != "" ||
			strings.Contains(res.Message, "\n") || len(f.opener.targets) != 0 {
			t.Errorf("%s: (%s, %s, %q, exported %q, opened %v)", tc.name, res.Result, res.Reason, res.Message, res.ExportedPath, f.opener.targets)
		}
		if got := readExport(t, src); got != exportSpecBody {
			t.Errorf("%s: source artifact changed to %q", tc.name, got)
		}
		entries, _ := os.ReadDir(filepath.Dir(src))
		if len(entries) != 1 {
			t.Errorf("%s: the checkout directory holds %d entries, want only the source", tc.name, len(entries))
		}
	}
}

func TestOpenExportIssueNameAndH1(t *testing.T) {
	f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
	f.writeBody(t, "docs/changes/active/0007-widget.md",
		"---\nid: 7\ntitle: Widget export\n---\n\n# Widget export\n\n## Why\n\nBecause change 0003 broke.\n\n## Reconcile log\n\nnoise\n")
	out := testsupport.TempDir(t)
	res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "change", 7, func(o *OpenOptions) { o.OutputPath, o.Print = out, true })
	want := filepath.Join(out, "2026-01-02-issue-widget.md")
	if res.Result != ResultApplied || res.ExportedPath != want {
		t.Fatalf("(%s, %q, %q)", res.Result, res.ExportedPath, res.Message)
	}
	if got, w := readExport(t, want), "# Widget export\n\n## Why\n\nBecause \"Old gadget\" broke.\n"; got != w {
		t.Errorf("issue = %q, want %q", got, w)
	}
}

func TestOpenExportWarningsFilter(t *testing.T) {
	body := "# Widget design\n\n<!-- docket:artifacts:start -->\nTalk about docket here.\n"
	for _, tc := range []struct {
		warnings            string
		leftovers, mentions bool
	}{{"", true, true}, {WarningsAll, true, true}, {WarningsLeftovers, true, false}, {WarningsMentions, false, true}} {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		f.writeBody(t, widgetSpec, body)
		out := testsupport.TempDir(t)
		res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "spec", 7, func(o *OpenOptions) {
			o.OutputPath, o.Print, o.Warnings = out, true, tc.warnings
		})
		if res.Result != ResultApplied || res.ExportedPath == "" {
			t.Errorf("%q: (%s, %q)", tc.warnings, res.Result, res.Message)
			continue
		}
		wantLeft := []ExportLeftover{{SourceLine: 3, Text: "<!-- docket:artifacts:start -->", Rule: ExportRuleStrayMarker}}
		if !tc.leftovers {
			wantLeft = nil
		}
		if !reflect.DeepEqual(res.Leftovers, wantLeft) {
			t.Errorf("%q: leftovers %+v, want %+v", tc.warnings, res.Leftovers, wantLeft)
		}
		var wantMent *ExportMentions
		if tc.mentions {
			wantMent = &ExportMentions{Count: 1, Lines: []int{3}}
		}
		if !reflect.DeepEqual(res.Mentions, wantMent) {
			t.Errorf("%q: mentions %+v, want %+v", tc.warnings, res.Mentions, wantMent)
		}
		wantHuman := []string{}
		if tc.leftovers {
			wantHuman = append(wantHuman, "damaged marker removed at source line 3: <!-- docket:artifacts:start --> — the source has an unpaired marker; the exported file is clean")
		}
		if tc.mentions {
			wantHuman = append(wantHuman, "1 mention of docket on line 3 — kept as written; review it before sharing")
		}
		if got := res.HumanWarnings(); !reflect.DeepEqual(got, wantHuman) {
			t.Errorf("%q: human warnings %q, want %q", tc.warnings, got, wantHuman)
		}
	}
}

func TestOpenResultHumanWarningsWording(t *testing.T) {
	r := OpenResult{
		Leftovers: []ExportLeftover{{Line: 12, Text: "[x](y)", Rule: "link"}},
		Mentions:  &ExportMentions{Count: 3, Lines: []int{2, 5, 9}},
	}
	want := []string{
		"leftover line 12 (link): [x](y) — docket machinery the export did not remove; edit it out before sharing",
		"3 mentions of docket on lines 2, 5, 9 — kept as written; review them before sharing",
	}
	if got := r.HumanWarnings(); !reflect.DeepEqual(got, want) {
		t.Errorf("HumanWarnings = %q, want %q", got, want)
	}
	if got := (OpenResult{Mentions: &ExportMentions{Lines: []int{}}}).HumanWarnings(); len(got) != 0 {
		t.Errorf("zero mentions render %q, want nothing", got)
	}
}

// TestOpenExportAmbiguousIDLeftAsWritten (Review Focus 4): an id two records
// claim is unknown to the export, never given either record's title.
func TestOpenExportAmbiguousIDLeftAsWritten(t *testing.T) {
	blobs := append(exportCorpus(), retitle(openChange(3, "old-copy", false, "branch: feat/old-copy\n"), "Copy gadget"))
	f := newOpenFixture(t, "", false, openWeb, blobs...)
	f.writeBody(t, widgetSpec, exportSpecBody)
	out := testsupport.TempDir(t)
	res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "spec", 7, func(o *OpenOptions) { o.OutputPath, o.Print = out, true })
	if res.Result != ResultApplied {
		t.Fatalf("(%s, %q)", res.Result, res.Message)
	}
	if got := readExport(t, res.ExportedPath); !strings.Contains(got, "Built after change 0003, see the notes.") {
		t.Errorf("ambiguous id converted: %q", got)
	}
}

func TestValidateOpenExport(t *testing.T) {
	bad := []OpenOptions{
		{What: "board", Export: true}, {What: "pr", ID: 7, Export: true},
		{What: "spec", OutputPath: "x.md"}, {What: "spec", Warnings: WarningsAll},
		{What: "spec", Export: true, Warnings: "bogus"},
	}
	for _, o := range bad {
		if err := ValidateOpenExport(o); err == nil || strings.Contains(err.Error(), "\n") {
			t.Errorf("ValidateOpenExport(%+v) = %v, want a one-line error", o, err)
		}
	}
	good := []OpenOptions{
		{What: "spec"}, {What: "board"}, {What: "pr", ID: 7},
		{What: "spec", Export: true}, {What: "change", Export: true, OutputPath: "x", Warnings: WarningsLeftovers},
		{What: "plan", Export: true, Warnings: WarningsMentions}, {What: "results", Export: true, Warnings: WarningsAll},
	}
	for _, o := range good {
		if err := ValidateOpenExport(o); err != nil {
			t.Errorf("ValidateOpenExport(%+v) = %v, want nil", o, err)
		}
	}
	// Open refuses the same way, writing nothing and opening nothing.
	f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
	res := Open(context.Background(), f.deps(), OpenOptions{RepoDir: f.repoDir, What: "pr", ID: 7, Export: true})
	if res.Result != ResultInvalidInput || res.Reason != ReasonOpenInvalidOption || len(f.opener.targets) != 0 || f.reader.pinCount != 0 {
		t.Errorf("Open pr --export: (%s, %s, %q) opener %v", res.Result, res.Reason, res.Message, f.opener.targets)
	}
	res = Open(context.Background(), f.deps(), OpenOptions{RepoDir: f.repoDir, What: "spec", ID: 7, OutputPath: "x.md"})
	if res.Result != ResultInvalidInput || res.Reason != ReasonOpenInvalidOption {
		t.Errorf("Open spec --output-path without --export: (%s, %s, %q)", res.Result, res.Reason, res.Message)
	}
}

func TestOpenResultExportJSON(t *testing.T) {
	raw, _ := json.Marshal(OpenResult{What: "spec", Notes: []string{}, ExportedPath: "/d/x.md", Replaced: true,
		Leftovers: []ExportLeftover{{SourceLine: 5, Text: "m", Rule: ExportRuleStrayMarker}, {Line: 2, Text: "t", Rule: "link"}},
		Mentions:  &ExportMentions{Count: 1, Lines: []int{4}}})
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"exported_path": "/d/x.md", "replaced": true,
		"leftovers": []any{
			map[string]any{"source_line": float64(5), "text": "m", "rule": "stray-marker"},
			map[string]any{"line": float64(2), "text": "t", "rule": "link"},
		},
		"mentions": map[string]any{"count": float64(1), "lines": []any{float64(4)}},
	}
	for k, w := range want {
		if !reflect.DeepEqual(doc[k], w) {
			t.Errorf("%s = %#v, want %#v (doc %s)", k, doc[k], w, raw)
		}
	}
	plain, _ := json.Marshal(OpenResult{What: "spec", Notes: []string{}})
	for k := range want {
		if strings.Contains(string(plain), `"`+k+`"`) {
			t.Errorf("non-export document carries %s: %s", k, plain)
		}
	}
}
