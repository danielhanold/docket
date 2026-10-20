package app

// open_export.go holds the --export branch of `docket open`: its usage check,
// the destination rule, the atomic write of the clean copy, and the warnings
// the self-check reports, filtered by --warnings.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/export"
	"github.com/danielhanold/docket/internal/exportcheck"
)

// The --warnings values: which warning lists an export reports.
const (
	WarningsAll       = "all"
	WarningsLeftovers = "leftovers"
	WarningsMentions  = "mentions"
)

// ExportRuleStrayMarker is the leftover rule of an unpaired or malformed marker
// line the transform removed from the source; its Line is a source line.
const ExportRuleStrayMarker = "stray-marker"

// ExportLeftover is one piece of docket machinery the self-check found in the
// exported copy, or a damaged marker the transform removed from the source.
type ExportLeftover struct {
	Line int    `json:"line"`
	Text string `json:"text"`
	Rule string `json:"rule"`
}

// ExportMentions lists the exported lines, code included, that name docket.
type ExportMentions struct {
	Count int   `json:"count"`
	Lines []int `json:"lines"`
}

// ValidateOpenExport is the one usage check of the export flags, shared by the
// CLI and Open: --export takes no board or pr, --output-path and --warnings
// need --export, and --warnings takes all, leftovers, or mentions.
func ValidateOpenExport(o OpenOptions) error {
	if !o.Export {
		switch {
		case o.OutputPath != "":
			return errors.New("--output-path needs --export")
		case o.Warnings != "":
			return errors.New("--warnings needs --export")
		}
		return nil
	}
	if o.What == "board" || o.What == "pr" {
		return fmt.Errorf("--export works on change, spec, plan, or results, not %s", o.What)
	}
	switch o.Warnings {
	case "", WarningsAll, WarningsLeftovers, WarningsMentions:
		return nil
	}
	return fmt.Errorf("invalid --warnings value %q — choose all, leftovers, or mentions", o.Warnings)
}

// HumanWarnings returns the warning lines a human renderer writes after the
// notes: one per leftover, then one for the docket mentions when there are any.
func (r OpenResult) HumanWarnings() []string {
	out := []string{}
	for _, l := range r.Leftovers {
		if l.Rule == ExportRuleStrayMarker {
			out = append(out, fmt.Sprintf("damaged marker removed at source line %d: %s — the source has an unpaired marker; the exported file is clean", l.Line, l.Text))
			continue
		}
		out = append(out, fmt.Sprintf("leftover line %d (%s): %s — docket machinery the export did not remove; edit it out before sharing", l.Line, l.Rule, l.Text))
	}
	if m := r.Mentions; m != nil && m.Count > 0 {
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
	"change": export.KindIssue, "spec": export.KindSpec, "plan": export.KindPlan, "results": export.KindResults,
}

// openExport reads rel from the local metadata checkout (whatever the
// repository's open routing), writes its clean copy to the destination, runs
// the self-check, and opens or prints the copy. Warnings never block.
func openExport(ctx context.Context, deps OpenDeps, o OpenOptions, dir string, pin StatusPin, c domain.Change, snap domain.Snapshot, rel string, res OpenResult) OpenResult {
	fail := func(f openFailure) OpenResult {
		res.Envelope = NewEnvelope(OperationOpen, f.result)
		res.Reason, res.Message = f.reason, f.message
		return res
	}
	if deps.HomeDir == nil || deps.Getwd == nil || deps.GOOS == "" {
		return fail(openFailure{ResultInternalError, ReasonOpenExportFailed, "export is not wired: home directory, working directory, and platform are required"})
	}
	src, notes, f := localArtifact(ctx, deps, dir, pin, rel)
	res.Notes = append(res.Notes, notes...)
	if f != nil {
		return fail(*f)
	}
	body, err := os.ReadFile(src)
	if err != nil {
		return fail(openFailure{ResultExternalFailed, ReasonOpenExportFailed, fmt.Sprintf("could not read %s: %v", src, err)})
	}

	kind := exportKinds[o.What]
	created := ""
	if cr := c.Created(); cr.State == domain.FieldPresent {
		created = cr.Value.Format("2006-01-02")
	}
	dest, f := exportDestination(deps, o.OutputPath, export.FileName(kind, rel, created, c.Slug()))
	if f != nil {
		return fail(*f)
	}

	titles := exportTitles(snap)
	out := export.Transform(export.Input{Kind: kind, Source: body, Title: c.Title(), Titles: titles})
	report := exportcheck.Check(out.Markdown, func(id int) bool { _, ok := titles[id]; return ok })

	replaced, err := writeExport(dest, out.Markdown)
	if err != nil {
		return fail(openFailure{ResultExternalFailed, ReasonOpenExportFailed, fmt.Sprintf("could not write %s: %v", dest, err)})
	}
	if replaced {
		res.Notes = append(res.Notes, "replaced "+dest)
	}
	res.ExportedPath, res.Replaced = dest, replaced

	warnings := o.Warnings
	if warnings == "" {
		warnings = WarningsAll
	}
	if warnings != WarningsMentions {
		for _, s := range out.Stray {
			res.Leftovers = append(res.Leftovers, ExportLeftover{Line: s.SourceLine, Text: s.Text, Rule: ExportRuleStrayMarker})
		}
		for _, l := range report.Leftovers {
			res.Leftovers = append(res.Leftovers, ExportLeftover{Line: l.Line, Text: l.Text, Rule: l.Rule})
		}
	}
	if warnings != WarningsLeftovers {
		res.Mentions = &ExportMentions{Count: report.Mentions.Count, Lines: append([]int{}, report.Mentions.Lines...)}
	}
	res.Target, res.TargetKind = dest, OpenTargetKindFile
	return finishOpen(ctx, deps, o, res)
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
// and its parent directory must exist.
func exportDestination(deps OpenDeps, outputPath, name string) (string, *openFailure) {
	invalid := func(msg string) (string, *openFailure) {
		return "", &openFailure{ResultInvalidState, ReasonOpenOutputPathInvalid, msg}
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
		return filepath.Join(dir, name), nil
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
		return filepath.Join(path, name), nil
	}
	parent := filepath.Dir(path)
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return invalid(fmt.Sprintf("%s is not an existing directory — create it or choose another --output-path", parent))
	}
	return path, nil
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
