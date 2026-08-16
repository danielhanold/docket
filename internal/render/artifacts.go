package render

import (
	"fmt"
	"path"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
)

// Backlink marker lines. The spelling is the canonical docket:backlink pair
// (mirrors internal/document/markers.go's startMarkerLine/endMarkerLine for the
// "backlink" block name and the "(generated — do not hand-edit)" annotation);
// it is reproduced verbatim here because those helpers are unexported and this
// package must not read the filesystem.
const (
	backlinkStartMarker = "<!-- docket:backlink:start (generated — do not hand-edit) -->"
	backlinkEndMarker   = "<!-- docket:backlink:end -->"
)

// ArtifactBlockContent renders the body of the managed docket:artifacts block —
// the table BETWEEN the markers, no marker lines — for one change, from its
// typed fields: Spec, Plan, Results paths and the ADRs list. Rows appear in the
// fixed order Spec, Plan, Results, ADRs; a row is omitted when its field is
// unset/empty. Empty content ("") means "no artifacts" — the caller still
// writes the (empty) block via document.ReplaceBlock, which supplies the
// markers this function omits.
//
// snap resolves the change's ADR ids to their canonical repo-relative paths;
// an id snap cannot resolve is a caller error (the app layer validates ADR
// references against the candidate snapshot before rendering), so it surfaces
// as an error rather than a degraded link.
//
// This reproduces scripts/render-change-links.sh's block body for the four
// typed rows. The Bash renderer's PR row and derived "Stacked children" row are
// out of scope for the v1 typed renderer (PR is a later slice; stacked children
// is a render-time directory scan) and are not emitted here. The v1 LinkContext
// carries a single MetadataBranch, so every link resolves onto that branch;
// the Bash renderer's lifecycle-pinned Plan/Results branch is a later concern.
func ArtifactBlockContent(c domain.Change, snap domain.Snapshot, link LinkContext) (string, error) {
	var rows []string

	if p := c.Spec().Value; p != "" {
		rows = append(rows, pathRow("Spec", p, link))
	}
	if p := c.Plan().Value; p != "" {
		rows = append(rows, pathRow("Plan", p, link))
	}
	if p := c.Results().Value; p != "" {
		rows = append(rows, pathRow("Results", p, link))
	}

	if adrs := c.ADRs(); len(adrs) > 0 {
		cell, err := adrCell(adrs, snap, link)
		if err != nil {
			return "", err
		}
		rows = append(rows, "| ADRs | "+cell+" |")
	}

	if len(rows) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString("| Artifact | Link |\n|---|---|\n")
	for _, r := range rows {
		b.WriteString(r)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// pathRow renders a Spec/Plan/Results row. In GitHub mode the link text is the
// path basename and the URL is the full repo-relative path on the metadata
// branch; in repo-relative mode the cell is the backtick-quoted path.
func pathRow(label, repoRelPath string, link LinkContext) string {
	if url := link.BlobURL(repoRelPath); url != "" {
		return fmt.Sprintf("| %s | [%s](%s) |", label, path.Base(repoRelPath), url)
	}
	return fmt.Sprintf("| %s | `%s` |", label, repoRelPath)
}

// adrCell renders the comma-separated ADR cell. In GitHub mode each entry is
// [ADR-NNNN](blob-url-to-the-resolved-file); in repo-relative mode the ADR-NNNN
// label is dropped and the entry is the backtick-quoted repo-relative path
// (matching scripts/render-change-links.sh).
func adrCell(adrs []domain.ADRID, snap domain.Snapshot, link LinkContext) (string, error) {
	entries := make([]string, len(adrs))
	for i, id := range adrs {
		adr, outcome := snap.ADR(id)
		if outcome != domain.LookupFound {
			return "", fmt.Errorf("render: cannot resolve ADR-%04d to a record path (lookup outcome %d)", int(id), outcome)
		}
		relPath := adr.Path()
		if url := link.BlobURL(relPath); url != "" {
			entries[i] = fmt.Sprintf("[ADR-%04d](%s)", int(id), url)
		} else {
			entries[i] = fmt.Sprintf("`%s`", relPath)
		}
	}
	return strings.Join(entries, ", "), nil
}

// BacklinkContent renders the full docket:backlink block for a spec/plan/results
// file — the two marker lines plus the "> ↩ **[Change NNNN — Title](url)**"
// line — targeting the change's CURRENT canonical metadata path (c.Path()). In
// repo-relative mode (empty RepoWebURL) the link becomes
// "> ↩ **Change NNNN — Title** — `relpath`", mirroring
// scripts/render-artifact-backlink.sh. The result ends with a trailing newline.
func BacklinkContent(c domain.Change, link LinkContext) (string, error) {
	padded := fmt.Sprintf("%04d", int(c.ID()))
	relPath := c.Path()

	var line string
	if url := link.BlobURL(relPath); url != "" {
		line = fmt.Sprintf("> ↩ **[Change %s — %s](%s)**", padded, c.Title(), url)
	} else {
		line = fmt.Sprintf("> ↩ **Change %s — %s** — `%s`", padded, c.Title(), relPath)
	}

	return backlinkStartMarker + "\n" + line + "\n" + backlinkEndMarker + "\n", nil
}
