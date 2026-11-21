package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
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

func (f *openFixture) export(t *testing.T, deps OpenDeps, what string, id int, mut func(*ExportOptions)) ExportResult {
	t.Helper()
	o := ExportOptions{RepoDir: f.repoDir, What: what, ID: id}
	if mut != nil {
		mut(&o)
	}
	return Export(context.Background(), deps, o)
}

func readExport(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// stubRenderPDF replaces the PDF renderer for one test.
func stubRenderPDF(t *testing.T, fn func(md []byte, title string) ([]byte, int, error)) {
	t.Helper()
	prev := renderPDF
	t.Cleanup(func() { renderPDF = prev })
	renderPDF = fn
}

// entryNames lists dir's entries by name.
func entryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

const (
	exportMDName  = "2026-01-02-spec-widget.md"
	exportPDFName = "2026-01-02-spec-widget.pdf"
)

func TestExportWritesCleanCopy(t *testing.T) {
	f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
	f.writeBody(t, widgetSpec, exportSpecBody)
	out := testsupport.TempDir(t)
	res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "spec", 7, func(o *ExportOptions) {
		o.Print, o.OutputPath = true, out
	})
	md, pdf := filepath.Join(out, exportMDName), filepath.Join(out, exportPDFName)
	if res.Result != ResultApplied || res.Launched || res.HumanText() != md+"\n"+pdf ||
		res.Operation != OperationExport || len(f.opener.targets) != 0 {
		t.Fatalf("Export = (%s, %s, launched %v, files %+v, human %q, %q)", res.Operation, res.Result, res.Launched, res.Files, res.HumanText(), res.Message)
	}
	wantFiles := []ExportFile{{ExportFormatMarkdown, md, false}, {ExportFormatPDF, pdf, false}}
	if !reflect.DeepEqual(res.Files, wantFiles) {
		t.Errorf("files = %+v, want %+v", res.Files, wantFiles)
	}
	if got := readExport(t, md); got != exportSpecClean {
		t.Errorf("exported bytes = %q, want %q", got, exportSpecClean)
	}
	if got := readExport(t, pdf); !strings.HasPrefix(got, "%PDF-") {
		t.Errorf("the PDF starts %q, want %%PDF-", got[:min(len(got), 16)])
	}
	for _, p := range []string{md, pdf} {
		info, err := os.Stat(p)
		if err != nil || info.Mode().Perm() != 0o644 {
			t.Errorf("%s mode = %v (%v), want 0644", p, info, err)
		}
	}
	if len(res.Leftovers) != 0 || res.Mentions == nil || res.Mentions.Count != 0 {
		t.Errorf("leftovers %v mentions %+v", res.Leftovers, res.Mentions)
	}
	if got := entryNames(t, out); !reflect.DeepEqual(got, []string{exportMDName, exportPDFName}) {
		t.Errorf("destination holds %q, want only the two exports (no temp file left)", got)
	}
}

// TestExportLaunchesOnlyAtTheDefaultDestination: only an export with neither
// --output-path nor --print hands the copy to the opener.
func TestExportLaunchesOnlyAtTheDefaultDestination(t *testing.T) {
	for _, tc := range []struct{ output, print bool }{{false, false}, {true, false}, {true, true}, {false, true}} {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		f.writeBody(t, widgetSpec, exportSpecBody)
		home := testsupport.TempDir(t)
		out := testsupport.TempDir(t)
		res := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, func(o *ExportOptions) {
			o.Print = tc.print
			if tc.output {
				o.OutputPath = out
			}
		})
		if res.Result != ResultApplied {
			t.Errorf("%+v: (%s, %s, %q)", tc, res.Result, res.Reason, res.Message)
			continue
		}
		if !tc.output && !tc.print {
			want := []string{filepath.Join(home, exportPDFName)}
			if !res.Launched || !reflect.DeepEqual(f.opener.targets, want) {
				t.Errorf("%+v: launched %v opener %v, want %v opened", tc, res.Launched, f.opener.targets, want)
			}
			continue
		}
		if res.Launched || len(f.opener.targets) != 0 || res.HumanText() != res.Path(ExportFormatMarkdown)+"\n"+res.Path(ExportFormatPDF) {
			t.Errorf("%+v: launched %v opener %v human %q, want nothing opened and the path printed", tc, res.Launched, f.opener.targets, res.HumanText())
		}
	}
}

// TestExportOpenerFailureKeepsThePath: an opener failure after the copy was
// written keeps the copy and its path, and reports the failure after the path.
func TestExportOpenerFailureKeepsThePath(t *testing.T) {
	for _, tc := range []struct {
		err    error
		reason string
	}{{&NoOpenerError{GOOS: "plan9"}, ReasonOpenNoOpener}, {errors.New("exit status 4"), ReasonOpenOpenerFailed}} {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		f.writeBody(t, widgetSpec, exportSpecBody)
		f.opener.err = tc.err
		res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "spec", 7, nil)
		md, pdf := res.Path(ExportFormatMarkdown), res.Path(ExportFormatPDF)
		if res.Result != ResultExternalFailed || res.Reason != tc.reason || md == "" || pdf == "" || res.Launched ||
			!strings.HasPrefix(res.HumanText(), md+"\n"+pdf+"\n") {
			t.Errorf("%s: (%s, %s, %+v, launched %v, human %q)", tc.reason, res.Result, res.Reason, res.Files, res.Launched, res.HumanText())
			continue
		}
		for _, p := range []string{md, pdf} {
			if _, err := os.Stat(p); err != nil {
				t.Errorf("%s: the copy is gone: %v", tc.reason, err)
			}
		}
	}
}

// TestExportRefusesInvalidRequest: a type outside the four, or a bad
// --warnings or formats value, refuses before any read and opens nothing.
func TestExportRefusesInvalidRequest(t *testing.T) {
	for _, o := range []ExportOptions{{What: "board"}, {What: "pr", ID: 7}, {What: "bogus"}, {What: "spec", ID: 7, Warnings: "bogus"},
		{What: "spec", ID: 7, Formats: "html"}, {What: "spec", ID: 7, Formats: "markdown,"}} {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		o.RepoDir = f.repoDir
		res := Export(context.Background(), f.deps(), o)
		if res.Result != ResultInvalidInput || res.Reason != ReasonOpenInvalidTarget || res.Operation != OperationExport ||
			res.Message == "" || strings.Contains(res.Message, "\n") || f.reader.pinCount != 0 || len(f.opener.targets) != 0 {
			t.Errorf("%+v: (%s, %s, %s, %q) pins %d opener %v", o, res.Operation, res.Result, res.Reason, res.Message, f.reader.pinCount, f.opener.targets)
		}
	}
}

func TestExportResultJSON(t *testing.T) {
	raw, _ := json.Marshal(ExportResult{What: "spec", Notes: []string{}, Files: []ExportFile{{ExportFormatMarkdown, "/d/x.md", true}},
		Leftovers: []ExportLeftover{{SourceLine: 5, Text: "m", Rule: ExportRuleStrayMarker}, {Line: 2, Text: "t", Rule: "link"}},
		Mentions:  &ExportMentions{Count: 1, Lines: []int{4}}})
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"files": []any{map[string]any{"format": "markdown", "path": "/d/x.md", "replaced": true}},
		"leftovers": []any{
			map[string]any{"source_line": float64(5), "text": "m", "rule": "stray-marker"},
			map[string]any{"line": float64(2), "text": "t", "rule": "link"},
		},
		"mentions": map[string]any{"count": float64(1), "lines": []any{float64(4)}},
		"launched": false,
		"notes":    []any{},
	}
	for k, w := range want {
		if !reflect.DeepEqual(doc[k], w) {
			t.Errorf("%s = %#v, want %#v (doc %s)", k, doc[k], w, raw)
		}
	}
	for _, k := range []string{"target", "target_kind", "exported_path", "replaced"} {
		if _, ok := doc[k]; ok {
			t.Errorf("export document carries %s: %s", k, raw)
		}
	}
}

func TestParseExportFormats(t *testing.T) {
	ok := []struct {
		vals []string
		want ExportFormatSet
	}{
		{[]string{"markdown"}, ExportFormatSet{Markdown: true}},
		{[]string{"pdf"}, ExportFormatSet{PDF: true}},
		{[]string{"markdown,pdf"}, ExportFormatSet{Markdown: true, PDF: true}},
		{[]string{"pdf", "markdown"}, ExportFormatSet{Markdown: true, PDF: true}},
		{[]string{"pdf", "pdf"}, ExportFormatSet{PDF: true}},
	}
	for _, tc := range ok {
		got, err := ParseExportFormats(tc.vals)
		if err != nil || got != tc.want {
			t.Errorf("ParseExportFormats(%q) = (%+v, %v), want %+v", tc.vals, got, err, tc.want)
		}
	}
	for _, vals := range [][]string{nil, {}, {""}, {"markdown,"}, {"html"}, {"PDF"}} {
		_, err := ParseExportFormats(vals)
		if err == nil || strings.Contains(err.Error(), "\n") || !strings.Contains(err.Error(), "markdown, pdf") {
			t.Errorf("ParseExportFormats(%q) = %v, want a one-line error naming markdown, pdf", vals, err)
		}
	}
	if _, err := ParseExportFormats([]string{"html"}); err == nil || err.Error() != `invalid --format value "html" — choose markdown, pdf, or both` {
		t.Errorf("ParseExportFormats(html) = %v", err)
	}
	if _, err := ParseExportFormats(nil); err == nil || err.Error() != "--format needs a value — choose markdown, pdf, or both" {
		t.Errorf("ParseExportFormats(nil) = %v", err)
	}
}

func TestParseExportArgs(t *testing.T) {
	ok := []struct {
		args []string
		what string
		id   int
	}{
		{[]string{"spec"}, "spec", 0}, {[]string{"change", "541"}, "change", 541},
		{[]string{"plan", "0541"}, "plan", 541}, {[]string{"results", "000012"}, "results", 12},
	}
	for _, tc := range ok {
		what, id, err := ParseExportArgs(tc.args)
		if err != nil || what != tc.what || id != tc.id {
			t.Errorf("ParseExportArgs(%q) = (%q, %d, %v), want (%q, %d)", tc.args, what, id, err, tc.what, tc.id)
		}
	}
	bad := [][]string{nil, {"board"}, {"pr", "7"}, {"bogus"}, {"spec", "1", "2"}, {"spec", "#541"}, {"spec", "0"}, {"spec", "-5"}}
	for _, args := range bad {
		_, _, err := ParseExportArgs(args)
		if err == nil || strings.Contains(err.Error(), "\n") {
			t.Errorf("ParseExportArgs(%q) = %v, want a one-line error", args, err)
			continue
		}
		if i := strings.Index(err.Error(), "one of "); i >= 0 {
			if got := err.Error()[i+len("one of "):]; got != "change, spec, plan, results" {
				t.Errorf("ParseExportArgs(%q) offers %q", args, got)
			}
		}
	}
	if _, _, err := ParseExportArgs([]string{"spec", "#541"}); err == nil || err.Error() != "invalid change id #541" {
		t.Errorf("ParseExportArgs(spec #541) = %v", err)
	}
	for _, args := range [][]string{nil, {"board"}, {"pr", "7"}, {"bogus"}, {"spec", "1", "2"}} {
		if _, _, err := ParseExportArgs(args); err == nil || !strings.Contains(err.Error(), "one of ") {
			t.Errorf("ParseExportArgs(%q) = %v, want the four types listed", args, err)
		}
	}
}

// TestExportIgnoresGitHubRouting: a shared repository with a GitHub origin
// and the default open.artifacts still exports the local metadata-checkout file.
func TestExportIgnoresGitHubRouting(t *testing.T) {
	f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
	f.writeBody(t, widgetSpec, exportSpecBody)
	out := testsupport.TempDir(t)
	res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "spec", 7, func(o *ExportOptions) {
		o.OutputPath, o.Print, o.Formats = out, true, ExportFormatMarkdown
	})
	if res.Result != ResultApplied || !strings.HasPrefix(res.Path(ExportFormatMarkdown), out) || len(f.prepared) != 1 {
		t.Errorf("(%s, %+v) prepared %v", res.Result, res.Files, f.prepared)
	}
	if !reflect.DeepEqual(res.Notes, []string{}) {
		t.Errorf("notes %q, want none (no not-GitHub note on the export path)", res.Notes)
	}
}

func TestExportKeepsStalenessNote(t *testing.T) {
	f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
	f.refusePrepare("metadata-worktree-dirty")
	f.writeBody(t, widgetSpec, exportSpecBody)
	out := testsupport.TempDir(t)
	res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "spec", 7, func(o *ExportOptions) {
		o.OutputPath, o.Print, o.Formats = out, true, ExportFormatMarkdown
	})
	if res.Result != ResultApplied || !reflect.DeepEqual(res.Notes, []string{openNoteStale("metadata-worktree-dirty")}) {
		t.Errorf("(%s, %q)", res.Result, res.Notes)
	}
}

func TestExportDefaultDestination(t *testing.T) {
	name := exportMDName
	newF := func() *openFixture {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		f.writeBody(t, widgetSpec, exportSpecBody)
		return f
	}
	print := func(o *ExportOptions) { o.Print = true }

	home := testsupport.TempDir(t)
	if err := os.Mkdir(filepath.Join(home, "Downloads"), 0o755); err != nil {
		t.Fatal(err)
	}
	f := newF()
	if res := f.export(t, f.exportDeps(home, "darwin", "/nowhere"), "spec", 7, print); res.Result != ResultApplied ||
		res.Path(ExportFormatMarkdown) != filepath.Join(home, "Downloads", name) || res.Path(ExportFormatPDF) != filepath.Join(home, "Downloads", exportPDFName) {
		t.Errorf("darwin: (%s, %+v, %q)", res.Result, res.Files, res.Message)
	}

	bare := testsupport.TempDir(t)
	f = newF()
	res := f.export(t, f.exportDeps(bare, "darwin", "/nowhere"), "spec", 7, print)
	if res.Result != ResultInvalidState || res.Reason != ReasonOpenOutputPathInvalid || !strings.Contains(res.Message, "--output-path") ||
		strings.Contains(res.Message, "\n") || res.Files == nil || len(res.Files) != 0 {
		t.Errorf("darwin without Downloads: (%s, %s, %q)", res.Result, res.Reason, res.Message)
	}
	if entries, _ := os.ReadDir(bare); len(entries) != 0 {
		t.Errorf("refusal wrote %d entries into home", len(entries))
	}

	linux := testsupport.TempDir(t)
	f = newF()
	if res := f.export(t, f.exportDeps(linux, "linux", "/nowhere"), "spec", 7, print); res.Result != ResultApplied ||
		res.Path(ExportFormatMarkdown) != filepath.Join(linux, name) {
		t.Errorf("linux: (%s, %+v, %q)", res.Result, res.Files, res.Message)
	}

	f = newF()
	d := f.exportDeps(linux, "linux", "/nowhere")
	d.HomeDir = func() (string, error) { return "", errors.New("no home") }
	if res := f.export(t, d, "spec", 7, print); res.Result == ResultApplied || len(res.Files) != 0 {
		t.Errorf("home error: (%s, %s, %q)", res.Result, res.Reason, res.Message)
	}
}

func TestExportOutputPath(t *testing.T) {
	newF := func() *openFixture {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		f.writeBody(t, widgetSpec, exportSpecBody)
		return f
	}
	home := testsupport.TempDir(t)
	dir := testsupport.TempDir(t)
	at := func(p string) func(*ExportOptions) { return func(o *ExportOptions) { o.OutputPath, o.Print = p, true } }
	mdAt := func(p string) func(*ExportOptions) {
		return func(o *ExportOptions) { o.OutputPath, o.Print, o.Formats = p, true, ExportFormatMarkdown }
	}

	f := newF()
	if res := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, at(dir)); res.Path(ExportFormatMarkdown) != filepath.Join(dir, exportMDName) ||
		res.Path(ExportFormatPDF) != filepath.Join(dir, exportPDFName) {
		t.Errorf("dir: (%s, %+v, %q)", res.Result, res.Files, res.Message)
	}

	exact := filepath.Join(dir, "shared.md")
	f = newF()
	if res := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, mdAt(exact)); res.Path(ExportFormatMarkdown) != exact || readExport(t, exact) != exportSpecClean {
		t.Errorf("file: (%s, %+v, %q)", res.Result, res.Files, res.Message)
	}

	cwd := testsupport.TempDir(t)
	f = newF()
	if res := f.export(t, f.exportDeps(home, "linux", cwd), "spec", 7, mdAt("out.md")); res.Path(ExportFormatMarkdown) != filepath.Join(cwd, "out.md") {
		t.Errorf("relative: (%s, %+v, %q)", res.Result, res.Files, res.Message)
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
	first := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, mdAt(again))
	second := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, mdAt(again))
	if len(first.Files) != 1 || first.Files[0].Replaced || len(second.Files) != 1 || !second.Files[0].Replaced || second.Result != ResultApplied ||
		!reflect.DeepEqual(second.Notes, []string{"replaced " + second.Path(ExportFormatMarkdown)}) {
		t.Errorf("re-export: first %+v, second (%s, %+v, %q)", first.Files, second.Result, second.Files, second.Notes)
	}
}

// TestExportRefusesProtectedDestination: --output-path never overwrites the
// source artifact and never writes into the metadata checkout, however the path
// is spelled — exactly, relatively, as a directory, or through a symlinked
// parent. The refusal writes nothing and opens nothing.
func TestExportRefusesProtectedDestination(t *testing.T) {
	home := testsupport.TempDir(t)
	cases := []struct {
		name    string
		cwd     func(src string) string
		path    func(src string) string
		formats string
	}{
		{"source file", nil, func(src string) string { return src }, ""},
		{"source file, unclean", nil, func(src string) string {
			return filepath.Join(filepath.Dir(src), "..", filepath.Base(filepath.Dir(src)), filepath.Base(src))
		}, ""},
		{"source file, relative", func(src string) string { return filepath.Dir(src) }, func(src string) string { return filepath.Base(src) }, ""},
		{"directory in the checkout", nil, func(src string) string { return filepath.Dir(src) }, ""},
		{"new file in the checkout", nil, func(src string) string { return filepath.Join(filepath.Dir(src), "copy.md") }, ""},
		{"symlinked parent into the checkout", nil, func(src string) string {
			link := filepath.Join(testsupport.TempDir(t), "meta-link")
			if err := os.Symlink(filepath.Dir(src), link); err != nil {
				t.Fatal(err)
			}
			return filepath.Join(link, filepath.Base(src))
		}, ""},
		{"source file, PDF only", nil, func(src string) string { return src }, ExportFormatPDF},
	}
	for _, tc := range cases {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		src := f.writeBody(t, widgetSpec, exportSpecBody)
		cwd := "/nowhere"
		if tc.cwd != nil {
			cwd = tc.cwd(src)
		}
		res := f.export(t, f.exportDeps(home, "linux", cwd), "spec", 7, func(o *ExportOptions) { o.OutputPath, o.Formats = tc.path(src), tc.formats })
		if res.Result != ResultInvalidState || res.Reason != ReasonOpenOutputPathProtected || len(res.Files) != 0 ||
			strings.Contains(res.Message, "\n") || len(f.opener.targets) != 0 {
			t.Errorf("%s: (%s, %s, %q, files %+v, opened %v)", tc.name, res.Result, res.Reason, res.Message, res.Files, f.opener.targets)
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

func TestExportTicketNameAndH1(t *testing.T) {
	f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
	f.writeBody(t, "docs/changes/active/0007-widget.md",
		"---\nid: 7\ntitle: Widget export\n---\n\n# Widget export\n\n## Why\n\nBecause change 0003 broke.\n\n## Reconcile log\n\nnoise\n")
	out := testsupport.TempDir(t)
	res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "change", 7, func(o *ExportOptions) {
		o.OutputPath, o.Print, o.Formats = out, true, ExportFormatMarkdown
	})
	want := filepath.Join(out, "2026-01-02-ticket-widget.md")
	if res.Result != ResultApplied || res.Path(ExportFormatMarkdown) != want {
		t.Fatalf("(%s, %+v, %q)", res.Result, res.Files, res.Message)
	}
	if got, w := readExport(t, want), "# Widget export\n\n## Why\n\nBecause \"Old gadget\" broke.\n"; got != w {
		t.Errorf("ticket = %q, want %q", got, w)
	}
}

func TestExportWarningsFilter(t *testing.T) {
	body := "# Widget design\n\n<!-- docket:artifacts:start -->\nTalk about docket here.\n"
	for _, tc := range []struct {
		warnings            string
		leftovers, mentions bool
		formats             string
	}{
		{"", true, true, ExportFormatMarkdown}, {WarningsAll, true, true, ExportFormatMarkdown},
		{WarningsLeftovers, true, false, ExportFormatMarkdown}, {WarningsMentions, false, true, ExportFormatMarkdown},
		// PDF alone: the warnings still describe the Markdown the PDF was rendered from.
		{"", true, true, ExportFormatPDF},
	} {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		f.writeBody(t, widgetSpec, body)
		out := testsupport.TempDir(t)
		res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "spec", 7, func(o *ExportOptions) {
			o.OutputPath, o.Print, o.Warnings, o.Formats = out, true, tc.warnings, tc.formats
		})
		if res.Result != ResultApplied || len(res.Files) != 1 || res.Files[0].Format != tc.formats {
			t.Errorf("%q/%s: (%s, %+v, %q)", tc.warnings, tc.formats, res.Result, res.Files, res.Message)
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

func TestExportResultHumanWarningsWording(t *testing.T) {
	r := ExportResult{
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
	if got := (ExportResult{Mentions: &ExportMentions{Lines: []int{}}}).HumanWarnings(); len(got) != 0 {
		t.Errorf("zero mentions render %q, want nothing", got)
	}
}

// TestExportAmbiguousIDLeftAsWritten (Review Focus 4): an id two records
// claim is unknown to the export, never given either record's title.
func TestExportAmbiguousIDLeftAsWritten(t *testing.T) {
	blobs := append(exportCorpus(), retitle(openChange(3, "old-copy", false, "branch: feat/old-copy\n"), "Copy gadget"))
	f := newOpenFixture(t, "", false, openWeb, blobs...)
	f.writeBody(t, widgetSpec, exportSpecBody)
	out := testsupport.TempDir(t)
	res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "spec", 7, func(o *ExportOptions) {
		o.OutputPath, o.Print, o.Formats = out, true, ExportFormatMarkdown
	})
	if res.Result != ResultApplied {
		t.Fatalf("(%s, %q)", res.Result, res.Message)
	}
	if got := readExport(t, res.Path(ExportFormatMarkdown)); !strings.Contains(got, "Built after change 0003, see the notes.") {
		t.Errorf("ambiguous id converted: %q", got)
	}
}

// TestExportFormatsWriteAndOpen: every format set writes exactly the requested
// files at the default destination, lists them Markdown first, and opens one:
// the PDF when it was requested, else the Markdown.
func TestExportFormatsWriteAndOpen(t *testing.T) {
	for _, tc := range []struct {
		formats string
		want    []string // formats written, in order
	}{
		{"", []string{ExportFormatMarkdown, ExportFormatPDF}},
		{"markdown", []string{ExportFormatMarkdown}},
		{"pdf", []string{ExportFormatPDF}},
		{"markdown,pdf", []string{ExportFormatMarkdown, ExportFormatPDF}},
		{"pdf,markdown", []string{ExportFormatMarkdown, ExportFormatPDF}},
	} {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		f.writeBody(t, widgetSpec, exportSpecBody)
		home := testsupport.TempDir(t)
		res := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, func(o *ExportOptions) { o.Formats = tc.formats })
		if res.Result != ResultApplied || !res.Launched {
			t.Errorf("%q: (%s, %s, %q, launched %v)", tc.formats, res.Result, res.Reason, res.Message, res.Launched)
			continue
		}
		got, names := []string{}, []string{}
		for _, fl := range res.Files {
			got = append(got, fl.Format)
			names = append(names, filepath.Base(fl.Path))
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: files %+v, want formats %q", tc.formats, res.Files, tc.want)
		}
		if disk := entryNames(t, home); !reflect.DeepEqual(disk, names) {
			t.Errorf("%q: home holds %q, want exactly %q", tc.formats, disk, names)
		}
		open := filepath.Join(home, exportMDName)
		if slices.Contains(tc.want, ExportFormatPDF) {
			open = filepath.Join(home, exportPDFName)
		}
		if !reflect.DeepEqual(f.opener.targets, []string{open}) {
			t.Errorf("%q: opened %q, want %q", tc.formats, f.opener.targets, open)
		}
	}
}

// TestExportOutputPathFormats: an exact --output-path with both formats gives
// each file the path with its own extension; with one format the path is used
// exactly as given; re-exporting both reports each replacement.
func TestExportOutputPathFormats(t *testing.T) {
	newF := func() *openFixture {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		f.writeBody(t, widgetSpec, exportSpecBody)
		return f
	}
	home := testsupport.TempDir(t)
	for _, base := range []string{"report.md", "report", "report.txt"} {
		dir := testsupport.TempDir(t)
		f := newF()
		res := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, func(o *ExportOptions) { o.OutputPath = filepath.Join(dir, base) })
		want := []ExportFile{{ExportFormatMarkdown, filepath.Join(dir, "report.md"), false}, {ExportFormatPDF, filepath.Join(dir, "report.pdf"), false}}
		if res.Result != ResultApplied || !reflect.DeepEqual(res.Files, want) || res.Launched {
			t.Errorf("%s: (%s, %+v, %q, launched %v), want %+v", base, res.Result, res.Files, res.Message, res.Launched, want)
		}
		if disk := entryNames(t, dir); !reflect.DeepEqual(disk, []string{"report.md", "report.pdf"}) {
			t.Errorf("%s: dir holds %q", base, disk)
		}
	}

	dir := testsupport.TempDir(t)
	f := newF()
	exact := filepath.Join(dir, "x.md")
	res := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, func(o *ExportOptions) { o.OutputPath, o.Formats = exact, ExportFormatPDF })
	if res.Result != ResultApplied || !reflect.DeepEqual(res.Files, []ExportFile{{ExportFormatPDF, exact, false}}) ||
		!strings.HasPrefix(readExport(t, exact), "%PDF-") {
		t.Errorf("pdf at x.md: (%s, %+v, %q)", res.Result, res.Files, res.Message)
	}
	txt := filepath.Join(dir, "notes.txt")
	f = newF()
	res = f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, func(o *ExportOptions) { o.OutputPath, o.Formats = txt, ExportFormatMarkdown })
	if res.Result != ResultApplied || res.Path(ExportFormatMarkdown) != txt || readExport(t, txt) != exportSpecClean {
		t.Errorf("markdown at notes.txt: (%s, %+v, %q)", res.Result, res.Files, res.Message)
	}

	again := testsupport.TempDir(t)
	f = newF()
	at := func(o *ExportOptions) { o.OutputPath = again }
	first := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, at)
	second := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, at)
	md, pdf := filepath.Join(again, exportMDName), filepath.Join(again, exportPDFName)
	if !reflect.DeepEqual(first.Files, []ExportFile{{ExportFormatMarkdown, md, false}, {ExportFormatPDF, pdf, false}}) {
		t.Errorf("first export: %+v", first.Files)
	}
	if second.Result != ResultApplied || !reflect.DeepEqual(second.Files, []ExportFile{{ExportFormatMarkdown, md, true}, {ExportFormatPDF, pdf, true}}) ||
		!reflect.DeepEqual(second.Notes, []string{"replaced " + md, "replaced " + pdf}) {
		t.Errorf("re-export: (%s, %+v, %q)", second.Result, second.Files, second.Notes)
	}
}

// TestExportPDFFailureKeepsMarkdown: a PDF that cannot be rendered or written
// keeps the Markdown copy already written, reports the failure naming it, and
// opens nothing; with the PDF alone, nothing is written.
func TestExportPDFFailureKeepsMarkdown(t *testing.T) {
	newF := func() *openFixture {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		f.writeBody(t, widgetSpec, exportSpecBody)
		return f
	}
	failing := func([]byte, string) ([]byte, int, error) { return nil, 0, errors.New("boom") }

	t.Run("render fails", func(t *testing.T) {
		stubRenderPDF(t, failing)
		f, home := newF(), testsupport.TempDir(t)
		res := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, nil)
		md := filepath.Join(home, exportMDName)
		if res.Result != ResultExternalFailed || res.Reason != ReasonOpenPDFFailed || res.Operation != OperationExport {
			t.Fatalf("(%s, %s, %s, %q)", res.Operation, res.Result, res.Reason, res.Message)
		}
		if !reflect.DeepEqual(res.Files, []ExportFile{{ExportFormatMarkdown, md, false}}) {
			t.Errorf("files %+v, want the Markdown copy alone", res.Files)
		}
		if got := readExport(t, md); got != exportSpecClean {
			t.Errorf("markdown bytes %q", got)
		}
		if disk := entryNames(t, home); !reflect.DeepEqual(disk, []string{exportMDName}) {
			t.Errorf("home holds %q, want the Markdown copy alone (no PDF, no temp file)", disk)
		}
		if res.Launched || len(f.opener.targets) != 0 {
			t.Errorf("launched %v opener %v, want nothing opened", res.Launched, f.opener.targets)
		}
		if strings.Contains(res.Message, "\n") || !strings.Contains(res.Message, md) || !strings.Contains(res.Message, "boom") {
			t.Errorf("message %q, want one line naming the failure and %s", res.Message, md)
		}
		if res.HumanText() != md+"\n"+res.Message {
			t.Errorf("human %q", res.HumanText())
		}
		if res.Mentions == nil {
			t.Errorf("the Markdown copy's warnings were dropped")
		}
	})

	t.Run("pdf alone", func(t *testing.T) {
		stubRenderPDF(t, failing)
		f, home := newF(), testsupport.TempDir(t)
		res := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, func(o *ExportOptions) { o.Formats = ExportFormatPDF })
		if res.Result != ResultExternalFailed || res.Reason != ReasonOpenPDFFailed || res.Files == nil || len(res.Files) != 0 {
			t.Fatalf("(%s, %s, %+v, %q)", res.Result, res.Reason, res.Files, res.Message)
		}
		if disk := entryNames(t, home); len(disk) != 0 {
			t.Errorf("home holds %q, want nothing written", disk)
		}
		if strings.Contains(res.Message, "Markdown") || strings.Contains(res.Message, ".md") || res.Launched || len(f.opener.targets) != 0 {
			t.Errorf("message %q launched %v opener %v", res.Message, res.Launched, f.opener.targets)
		}
	})

	t.Run("write fails", func(t *testing.T) {
		f, home := newF(), testsupport.TempDir(t)
		if err := os.Mkdir(filepath.Join(home, exportPDFName), 0o755); err != nil {
			t.Fatal(err)
		}
		res := f.export(t, f.exportDeps(home, "linux", "/nowhere"), "spec", 7, nil)
		md := filepath.Join(home, exportMDName)
		if res.Result != ResultExternalFailed || res.Reason != ReasonOpenPDFFailed ||
			!reflect.DeepEqual(res.Files, []ExportFile{{ExportFormatMarkdown, md, false}}) {
			t.Fatalf("(%s, %s, %+v, %q)", res.Result, res.Reason, res.Files, res.Message)
		}
		if strings.Contains(res.Message, "\n") || !strings.Contains(res.Message, md) || readExport(t, md) != exportSpecClean {
			t.Errorf("message %q", res.Message)
		}
		if disk := entryNames(t, home); !reflect.DeepEqual(disk, []string{exportMDName, exportPDFName}) {
			t.Errorf("home holds %q, want the Markdown copy and the blocking directory (no temp file)", disk)
		}
		if res.Launched || len(f.opener.targets) != 0 {
			t.Errorf("launched %v opener %v", res.Launched, f.opener.targets)
		}
	})
}

// TestExportMarkdownWriteFailureStops: a Markdown write failure is today's
// export-failed and the PDF is never rendered.
func TestExportMarkdownWriteFailureStops(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	calls := 0
	stubRenderPDF(t, func([]byte, string) ([]byte, int, error) { calls++; return []byte("%PDF-"), 0, nil })
	f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
	f.writeBody(t, widgetSpec, exportSpecBody)
	out := testsupport.TempDir(t)
	if err := os.Chmod(out, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(out, 0o755) })
	res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "spec", 7, func(o *ExportOptions) { o.OutputPath = out })
	if res.Result != ResultExternalFailed || res.Reason != ReasonOpenExportFailed || calls != 0 || len(res.Files) != 0 {
		t.Errorf("(%s, %s, %q) render calls %d files %+v", res.Result, res.Reason, res.Message, calls, res.Files)
	}
}

// TestExportPlaceholderNote: characters the PDF fonts cannot draw are counted
// in one note, singular or plural, only when a PDF was written.
func TestExportPlaceholderNote(t *testing.T) {
	for _, tc := range []struct {
		body, formats, want string
	}{
		{"# Widget design\n\n🟡 and 🗑\n", "", "2 characters the PDF fonts cannot draw were replaced with □"},
		{"# Widget design\n\n🟡 alone\n", "", "1 character the PDF fonts cannot draw was replaced with □"},
		{"# Widget design\n\n🟡 and 🗑\n", ExportFormatMarkdown, ""},
		{"# Widget design\n\nall drawable — →\n", "", ""},
	} {
		f := newOpenFixture(t, "", false, openWeb, exportCorpus()...)
		f.writeBody(t, widgetSpec, tc.body)
		out := testsupport.TempDir(t)
		res := f.export(t, f.exportDeps(testsupport.TempDir(t), "linux", "/nowhere"), "spec", 7, func(o *ExportOptions) { o.OutputPath, o.Formats = out, tc.formats })
		if res.Result != ResultApplied {
			t.Errorf("%q: (%s, %q)", tc.body, res.Result, res.Message)
			continue
		}
		var got []string
		for _, n := range res.Notes {
			if strings.Contains(n, "cannot draw") {
				got = append(got, n)
			}
		}
		want := []string(nil)
		if tc.want != "" {
			want = []string{tc.want}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q/%q: placeholder notes %q, want %q", tc.body, tc.formats, got, want)
		}
	}
}
