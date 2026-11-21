package app

// export.go holds `docket export`: its argument grammar, its result document,
// the formats it writes, the destination rule, the atomic write of the clean
// copy and of its PDF, and the warnings the self-check reports, filtered by
// --warnings.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/export"
	"github.com/danielhanold/docket/internal/exportcheck"
	"github.com/danielhanold/docket/internal/pdfrender"
)

// OperationExport is the operation id of `docket export`.
const OperationExport = "export"

// ExportTargets lists what `docket export` can export, in usage order. Each
// takes an optional change id.
var ExportTargets = []string{"change", "spec", "plan", "results"}

// ExportOptions is one `docket export` request.
type ExportOptions struct {
	RepoDir    string
	What       string
	ID         int // 0 = infer from the checked-out branch
	Print      bool
	OutputPath string // "" = the default destination
	Warnings   string // "" = WarningsAll
	// Formats is "" for both formats, or a comma-separated list
	// ParseExportFormats accepts. A string keeps ExportOptions comparable.
	Formats string
}

// The --format values: which files an export writes.
const (
	ExportFormatMarkdown = "markdown"
	ExportFormatPDF      = "pdf"
)

// ExportFormatSet is the set of formats an export writes.
type ExportFormatSet struct{ Markdown, PDF bool }

// ParseExportFormats parses --format values: each is split on commas, and
// every element must be exactly markdown or pdf, in any order, duplicates
// allowed. An empty list or element, or an unknown value, is a one-line error.
func ParseExportFormats(vals []string) (ExportFormatSet, error) {
	var set ExportFormatSet
	needs := errors.New("--format needs a value — choose markdown, pdf, or both")
	if len(vals) == 0 {
		return set, needs
	}
	for _, v := range vals {
		for _, e := range strings.Split(v, ",") {
			switch e {
			case ExportFormatMarkdown:
				set.Markdown = true
			case ExportFormatPDF:
				set.PDF = true
			case "":
				return ExportFormatSet{}, needs
			default:
				return ExportFormatSet{}, fmt.Errorf("invalid --format value %q — choose markdown, pdf, or both", e)
			}
		}
	}
	return set, nil
}

// exportFormats resolves ExportOptions.Formats: "" is both formats.
func exportFormats(formats string) (ExportFormatSet, error) {
	if formats == "" {
		return ExportFormatSet{Markdown: true, PDF: true}, nil
	}
	return ParseExportFormats([]string{formats})
}

// ExportFile is one file an export wrote: its format, its path, and whether it
// replaced an existing file.
type ExportFile struct {
	Format   string `json:"format"`
	Path     string `json:"path"`
	Replaced bool   `json:"replaced"`
}

// ExportResult is the document `docket export` emits for every outcome.
// ChangeID is omitted until the change is resolved; Files lists every file
// written, Markdown first, and is always a list, as is Notes; Launched means
// the opened file was launched; Leftovers and Mentions follow --warnings.
type ExportResult struct {
	Envelope
	What      string           `json:"what"`
	ChangeID  int              `json:"change_id,omitempty"`
	Files     []ExportFile     `json:"files"`
	Launched  bool             `json:"launched"`
	Notes     []string         `json:"notes"`
	Leftovers []ExportLeftover `json:"leftovers,omitempty"`
	Mentions  *ExportMentions  `json:"mentions,omitempty"`
	Reason    string           `json:"reason,omitempty"`
	Message   string           `json:"message,omitempty"`
}

// Path returns the path of the file written in format, or "" when none was.
func (r ExportResult) Path(format string) string {
	for _, f := range r.Files {
		if f.Format == format {
			return f.Path
		}
	}
	return ""
}

// HumanText renders the result for a terminal: one path per line for every
// file written (so a single-format export composes in command substitution),
// then the failure message on its own line when there is one.
func (r ExportResult) HumanText() string {
	lines := []string{}
	for _, f := range r.Files {
		lines = append(lines, f.Path)
	}
	if r.Message != "" {
		lines = append(lines, r.Message)
	}
	return strings.Join(lines, "\n")
}

// HumanNotes returns the notes a human renderer writes apart from HumanText.
func (r ExportResult) HumanNotes() []string { return r.Notes }

// HumanWarnings returns the warning lines a human renderer writes after the
// notes.
func (r ExportResult) HumanWarnings() []string { return exportWarningLines(r.Leftovers, r.Mentions) }

// ParseExportArgs parses `export <type> [id]` into the type and the change id,
// 0 when none was given. Every error is a one-line usage message.
func ParseExportArgs(args []string) (string, int, error) {
	types := strings.Join(ExportTargets, ", ")
	switch {
	case len(args) == 0:
		return "", 0, fmt.Errorf("missing type — choose one of %s", types)
	case len(args) > 2:
		return "", 0, fmt.Errorf("too many arguments — usage: export <type> [id], where <type> is one of %s", types)
	}
	what := args[0]
	if !slices.Contains(ExportTargets, what) {
		return "", 0, fmt.Errorf("unknown type %q — choose one of %s", what, types)
	}
	if len(args) == 1 {
		return what, 0, nil
	}
	n, err := parseChangeIDArg(args[1])
	if err != nil {
		return "", 0, err
	}
	return what, n, nil
}

// ValidateExportWarnings accepts the --warnings values: all, leftovers,
// mentions, or "" (all).
func ValidateExportWarnings(w string) error {
	switch w {
	case "", WarningsAll, WarningsLeftovers, WarningsMentions:
		return nil
	}
	return fmt.Errorf("invalid --warnings value %q — choose all, leftovers, or mentions", w)
}

// Export resolves what (and the change id, or the change the checked-out branch
// records when the id is 0) to an artifact in the local metadata checkout,
// whatever the repository's open routing, and writes its clean copy in the
// requested formats. Only an export to the default destination without Print
// opens anything, and only when every file was written: the PDF when it was
// requested, else the Markdown.
func Export(ctx context.Context, deps OpenDeps, o ExportOptions) ExportResult {
	res := ExportResult{What: o.What, ChangeID: o.ID, Files: []ExportFile{}, Notes: []string{}}
	fail := func(f openFailure) ExportResult {
		res.Envelope = NewEnvelope(OperationExport, f.result)
		res.Reason, res.Message = f.reason, f.message
		return res
	}
	if !slices.Contains(ExportTargets, o.What) {
		args := []string{o.What}
		if o.ID != 0 {
			args = append(args, strconv.Itoa(o.ID))
		}
		_, _, err := ParseExportArgs(args)
		return fail(openFailure{ResultInvalidInput, ReasonOpenInvalidTarget, err.Error()})
	}
	// Reusing invalid-target here is deliberate: only direct app-layer callers
	// reach this path, because the CLI rejects a bad --warnings value as a usage
	// error (exit 2) before the operation runs, and the spec removed the old
	// invalid-option reason.
	if err := ValidateExportWarnings(o.Warnings); err != nil {
		return fail(openFailure{ResultInvalidInput, ReasonOpenInvalidTarget, err.Error()})
	}
	formats, err := exportFormats(o.Formats)
	if err != nil {
		return fail(openFailure{ResultInvalidInput, ReasonOpenInvalidTarget, err.Error()})
	}
	dir, branch, pin, f := openPrelude(ctx, deps, o.RepoDir, o.What, o.ID)
	if f != nil {
		return fail(*f)
	}
	c, snap, f := resolveOpenChange(ctx, deps.Reader, pin, o.ID, branch)
	if f != nil {
		return fail(*f)
	}
	res.ChangeID = int(c.ID())
	rel, f := changeArtifactPath(c, o.What)
	if f != nil {
		return fail(*f)
	}
	if f := checkArtifactPath(o.What, rel); f != nil {
		return fail(*f)
	}
	cp, f := writeCleanCopy(ctx, deps, o.What, o.OutputPath, o.Warnings, dir, pin, c, snap, rel, formats)
	// What was written is reported on every outcome, so a PDF failure still
	// lists the Markdown copy it kept and that copy's warnings.
	res.Notes = append(res.Notes, cp.Notes...)
	res.Files = append(res.Files, cp.Files...)
	res.Leftovers, res.Mentions = cp.Leftovers, cp.Mentions
	if f != nil {
		return fail(*f)
	}
	if !o.Print && o.OutputPath == "" {
		if f := runOpener(ctx, deps.Opener, cp.Open); f != nil {
			return fail(*f)
		}
		res.Launched = true
	}
	res.Envelope = NewEnvelope(OperationExport, ResultApplied)
	return res
}

// The --warnings values: which warning lists an export reports.
const (
	WarningsAll       = "all"
	WarningsLeftovers = "leftovers"
	WarningsMentions  = "mentions"
)

// ExportRuleStrayMarker is the leftover rule of an unpaired or malformed marker
// line the transform removed from the source; it carries a SourceLine, not a Line.
const ExportRuleStrayMarker = "stray-marker"

// ExportLeftover is one piece of docket machinery the self-check found in the
// exported copy, located by its output Line, or a damaged marker the transform
// removed from the source, located by its SourceLine. Exactly one is set.
type ExportLeftover struct {
	Line       int    `json:"line,omitempty"`
	SourceLine int    `json:"source_line,omitempty"`
	Text       string `json:"text"`
	Rule       string `json:"rule"`
}

// ExportMentions lists the exported lines, code included, that name docket.
type ExportMentions struct {
	Count int   `json:"count"`
	Lines []int `json:"lines"`
}

// exportWarningLines renders an export's warnings: one line per leftover, then
// one for the docket mentions when there are any.
func exportWarningLines(leftovers []ExportLeftover, mentions *ExportMentions) []string {
	out := []string{}
	for _, l := range leftovers {
		if l.Rule == ExportRuleStrayMarker {
			out = append(out, fmt.Sprintf("damaged marker removed at source line %d: %s — the source has an unpaired marker; the exported file is clean", l.SourceLine, l.Text))
			continue
		}
		out = append(out, fmt.Sprintf("leftover line %d (%s): %s — docket machinery the export did not remove; edit it out before sharing", l.Line, l.Rule, l.Text))
	}
	if m := mentions; m != nil && m.Count > 0 {
		nums := make([]string, len(m.Lines))
		for i, n := range m.Lines {
			nums[i] = strconv.Itoa(n)
		}
		if m.Count == 1 {
			out = append(out, fmt.Sprintf("1 mention of docket on line %s — kept as written; review it before sharing", strings.Join(nums, ", ")))
		} else {
			out = append(out, fmt.Sprintf("%d mentions of docket on lines %s — kept as written; review them before sharing", m.Count, strings.Join(nums, ", ")))
		}
	}
	return out
}

// exportKinds maps an exportable target to its export kind.
var exportKinds = map[string]export.Kind{
	"change": export.KindTicket, "spec": export.KindSpec, "plan": export.KindPlan, "results": export.KindResults,
}

// exportCopy is what writeCleanCopy wrote: every file, Markdown first; the
// path to open; the notes to attach; and the warnings --warnings keeps.
type exportCopy struct {
	Files     []ExportFile
	Open      string
	Notes     []string
	Leftovers []ExportLeftover
	Mentions  *ExportMentions
}

// renderPDF renders the clean copy's Markdown to a PDF; tests replace it.
var renderPDF = pdfrender.Render

// writeCleanCopy reads rel from the local metadata checkout (whatever the
// repository's open routing), runs the self-check, and writes the clean copy
// in each requested format to the destination outputPath names, keeping the
// warnings the warnings value asks for. Warnings never block. Every
// destination is resolved and checked before anything is written. The
// Markdown copy is written first; a PDF failure keeps it and is reported as
// pdf-failed. Notes, Files, and the warnings carry what was done even when a
// failure is returned.
func writeCleanCopy(ctx context.Context, deps OpenDeps, what, outputPath, warnings, dir string, pin StatusPin, c domain.Change, snap domain.Snapshot, rel string, formats ExportFormatSet) (exportCopy, *openFailure) {
	var cp exportCopy
	if deps.HomeDir == nil || deps.Getwd == nil || deps.GOOS == "" {
		return cp, &openFailure{ResultInternalError, ReasonOpenExportFailed, "export is not wired: home directory, working directory, and platform are required"}
	}
	src, root, notes, f := localArtifact(ctx, deps, dir, pin, rel)
	cp.Notes = append(cp.Notes, notes...)
	if f != nil {
		return cp, f
	}
	body, err := os.ReadFile(src)
	if err != nil {
		return cp, &openFailure{ResultExternalFailed, ReasonOpenExportFailed, fmt.Sprintf("could not read %s: %v", src, err)}
	}

	kind := exportKinds[what]
	created := ""
	if cr := c.Created(); cr.State == domain.FieldPresent {
		created = cr.Value.Format("2006-01-02")
	}
	name := export.FileName(kind, rel, created, c.Slug())
	stem := strings.TrimSuffix(name, ".md")
	dest, exact, f := exportDestination(deps, outputPath, name)
	if f != nil {
		return cp, f
	}
	mdDest, pdfDest := dest, dest
	switch {
	case !exact:
		pdfDest = filepath.Join(filepath.Dir(dest), stem+".pdf")
	case formats.Markdown && formats.PDF:
		mdDest, pdfDest = withExportExt(dest, ".md"), withExportExt(dest, ".pdf")
	}
	var dests []string
	if formats.Markdown {
		dests = append(dests, mdDest)
	}
	if formats.PDF {
		dests = append(dests, pdfDest)
	}
	for _, d := range dests {
		if f := protectedDestination(d, src, root); f != nil {
			return cp, f
		}
	}

	titles := exportTitles(snap)
	out := export.Transform(export.Input{Kind: kind, Source: body, Title: c.Title(), Titles: titles})
	report := exportcheck.Check(out.Markdown, func(id int) bool { _, ok := titles[id]; return ok })
	leftovers, mentions := exportWarnings(warnings, out, report)

	if formats.Markdown {
		replaced, err := writeExport(mdDest, out.Markdown)
		if err != nil {
			return cp, &openFailure{ResultExternalFailed, ReasonOpenExportFailed, fmt.Sprintf("could not write %s: %v", mdDest, err)}
		}
		if replaced {
			cp.Notes = append(cp.Notes, "replaced "+mdDest)
		}
		cp.Files = append(cp.Files, ExportFile{Format: ExportFormatMarkdown, Path: mdDest, Replaced: replaced})
		cp.Open, cp.Leftovers, cp.Mentions = mdDest, leftovers, mentions
	}
	if formats.PDF {
		pdfFailed := func(msg string) (exportCopy, *openFailure) {
			if formats.Markdown {
				msg += " — the Markdown copy is at " + mdDest
			}
			return cp, &openFailure{ResultExternalFailed, ReasonOpenPDFFailed, msg}
		}
		pdf, placeholders, err := renderPDF(out.Markdown, stem)
		if err != nil {
			return pdfFailed("could not render the PDF: " + oneLine(err.Error()))
		}
		replaced, err := writeExport(pdfDest, pdf)
		if err != nil {
			return pdfFailed(fmt.Sprintf("could not write %s: %s", pdfDest, oneLine(err.Error())))
		}
		if replaced {
			cp.Notes = append(cp.Notes, "replaced "+pdfDest)
		}
		switch {
		case placeholders == 1:
			cp.Notes = append(cp.Notes, fmt.Sprintf("1 character the PDF fonts cannot draw was replaced with %c", pdfrender.Placeholder))
		case placeholders > 1:
			cp.Notes = append(cp.Notes, fmt.Sprintf("%d characters the PDF fonts cannot draw were replaced with %c", placeholders, pdfrender.Placeholder))
		}
		cp.Files = append(cp.Files, ExportFile{Format: ExportFormatPDF, Path: pdfDest, Replaced: replaced})
		cp.Open, cp.Leftovers, cp.Mentions = pdfDest, leftovers, mentions
	}
	return cp, nil
}

// exportWarnings builds the warnings the warnings value keeps from the
// transform's stray markers and the self-check report on the Markdown copy.
func exportWarnings(warnings string, out export.Output, report exportcheck.Report) ([]ExportLeftover, *ExportMentions) {
	var leftovers []ExportLeftover
	var mentions *ExportMentions
	if warnings == "" {
		warnings = WarningsAll
	}
	if warnings != WarningsMentions {
		for _, s := range out.Stray {
			leftovers = append(leftovers, ExportLeftover{SourceLine: s.SourceLine, Text: s.Text, Rule: ExportRuleStrayMarker})
		}
		for _, l := range report.Leftovers {
			leftovers = append(leftovers, ExportLeftover{Line: l.Line, Text: l.Text, Rule: l.Rule})
		}
	}
	if warnings != WarningsLeftovers {
		mentions = &ExportMentions{Count: report.Mentions.Count, Lines: append([]int{}, report.Mentions.Lines...)}
	}
	return leftovers, mentions
}

// withExportExt gives an exact --output-path file its format's extension: the
// base's extension is replaced when there is a stem before it, and the
// extension is appended otherwise (report, report.txt, report.md → report.md).
func withExportExt(p, ext string) string {
	base := filepath.Base(p)
	if e := filepath.Ext(base); e != "" && e != base {
		return strings.TrimSuffix(p, e) + ext
	}
	return p + ext
}

// oneLine folds an error text onto one line, so a failure message stays one
// line whatever the error says.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// exportTitles maps every change id exactly one record claims to its title; an
// ambiguous id is left out, so the export treats it as unknown.
func exportTitles(snap domain.Snapshot) export.Titles {
	count := map[int]int{}
	for _, c := range snap.Changes() {
		count[int(c.ID())]++
	}
	titles := export.Titles{}
	for _, c := range snap.Changes() {
		if id := int(c.ID()); count[id] == 1 {
			titles[id] = c.Title()
		}
	}
	return titles
}

// exportDestination resolves where the copy named name is written. With no
// output path it goes to ~/Downloads on darwin (refused when missing) and the
// home directory elsewhere. An existing directory takes the copy under name;
// any other output path is the exact file, relative to the working directory,
// and its parent directory must exist. exact reports that last case: the
// output path named a file rather than a directory or the default.
func exportDestination(deps OpenDeps, outputPath, name string) (dest string, exact bool, f *openFailure) {
	invalid := func(msg string) (string, bool, *openFailure) {
		return "", false, &openFailure{ResultInvalidState, ReasonOpenOutputPathInvalid, msg}
	}
	if outputPath == "" {
		home, err := deps.HomeDir()
		if err != nil || home == "" {
			return invalid(fmt.Sprintf("cannot find the home directory (%v) — pass --output-path", err))
		}
		dir := home
		if deps.GOOS == "darwin" {
			dir = filepath.Join(home, "Downloads")
			if info, err := os.Stat(dir); err != nil || !info.IsDir() {
				return invalid(fmt.Sprintf("%s does not exist — pass --output-path to choose where the export goes", dir))
			}
		}
		return filepath.Join(dir, name), false, nil
	}
	path := outputPath
	if !filepath.IsAbs(path) {
		wd, err := deps.Getwd()
		if err != nil {
			return invalid(fmt.Sprintf("cannot resolve %s: %v", outputPath, err))
		}
		path = filepath.Join(wd, path)
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return filepath.Join(path, name), false, nil
	}
	parent := filepath.Dir(path)
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return invalid(fmt.Sprintf("%s is not an existing directory — create it or choose another --output-path", parent))
	}
	return path, true, nil
}

// protectedDestination refuses a destination that is the source artifact or
// lies inside the metadata checkout, compared after cleaning the path and
// resolving the symlinks of its parent directory (and of the source and the
// checkout root), so no spelling of the path slips past.
func protectedDestination(dest, src, root string) *openFailure {
	refuse := func(msg string) *openFailure {
		return &openFailure{ResultInvalidState, ReasonOpenOutputPathProtected, msg}
	}
	d := filepath.Join(resolveSymlinks(filepath.Dir(filepath.Clean(dest))), filepath.Base(dest))
	if d == resolveSymlinks(src) {
		return refuse(fmt.Sprintf("%s is the source artifact — choose another --output-path", dest))
	}
	if rel, err := filepath.Rel(resolveSymlinks(root), d); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return refuse(fmt.Sprintf("%s is inside the metadata checkout — choose another --output-path", dest))
	}
	return nil
}

// resolveSymlinks returns p cleaned and with its symlinks resolved, or cleaned
// alone when it cannot be resolved.
func resolveSymlinks(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// writeExport writes data to dest atomically (a temp file beside it, mode 0644,
// then a rename) and reports whether dest existed before.
func writeExport(dest string, data []byte) (bool, error) {
	_, statErr := os.Lstat(dest)
	replaced := statErr == nil
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return false, statErr
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+filepath.Base(dest)+".*.tmp")
	if err != nil {
		return false, err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }() // a no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return false, err
	}
	if err := os.Rename(name, dest); err != nil {
		return false, err
	}
	return replaced, nil
}
