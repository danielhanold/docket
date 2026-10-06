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
// fixed order Spec, Spec (merged), Plan, Results, ADRs; a row is omitted when
// its field is unset/empty. Empty content ("") means "no artifacts" — the caller
// still writes the (empty) block via document.ReplaceBlock, which supplies the
// markers this function omits.
//
// snap resolves the change's ADR ids to their canonical repo-relative paths;
// an id snap cannot resolve is a caller error (the app layer validates ADR
// references against the candidate snapshot before rendering), so it surfaces
// as an error rather than a degraded link.
//
// The spec, plan, results, and ADRs all live on the metadata branch with the
// record, so their rows link relatively from the record file (RelativeLink).
// docs/changes/active and docs/changes/archive are siblings, so the block is
// byte-identical before and after archiving, and it renders the same with or
// without a web URL. Some rows are absolute on another branch, because their
// files are there:
//
//   - Spec (merged): the spec copy that merged with the PR, on the integration
//     branch, for a done change that carries the "## Build evidence" section
//     (built on the metadata-branch flow).
//   - Plan and Results of a legacy record (see legacyArtifactBranch), whose
//     files stayed on the integration or feature branch because the change
//     closed out before plan and results moved to the metadata branch.
func ArtifactBlockContent(c domain.Change, snap domain.Snapshot, link LinkContext) (string, error) {
	var rows []string
	recPath := c.Path()
	legacyBranch, legacy := legacyArtifactBranch(c, link)

	if p := c.Spec().Value; p != "" {
		rows = append(rows, relativeRow("Spec", recPath, p))
		if c.Status() == domain.StatusDone && c.HasBuildEvidence() {
			rows = append(rows, absoluteRow("Spec (merged)", p, link.IntegrationBranch, link))
		}
	}
	for _, a := range []struct{ label, path string }{
		{"Plan", c.Plan().Value},
		{"Results", c.Results().Value},
	} {
		switch {
		case a.path == "":
		case legacy:
			rows = append(rows, absoluteRow(a.label, a.path, legacyBranch, link))
		default:
			rows = append(rows, relativeRow(a.label, recPath, a.path))
		}
	}

	if adrs := c.ADRs(); len(adrs) > 0 {
		cell, err := adrCell(adrs, snap, recPath)
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

// legacyArtifactBranch reports whether a change's plan and results live off
// the metadata branch, and on which branch. A closed change WITHOUT the
// "## Build evidence" section closed out before plan and results moved to the
// metadata branch, so its files never move again: a done change's merged with
// its PR to the integration branch; a killed change's stayed on its feature
// branch (an unset branch: yields "", which BlobURLOnBranch resolves to the
// metadata branch, the rendering such a record has always had). Every other
// change's plan and results are on the metadata branch.
func legacyArtifactBranch(c domain.Change, link LinkContext) (string, bool) {
	if c.HasBuildEvidence() {
		return "", false
	}
	switch c.Status() {
	case domain.StatusDone:
		return link.IntegrationBranch, true
	case domain.StatusKilled:
		return c.Branch().Value, true
	}
	return "", false
}

// relativeRow renders a same-branch Spec/Plan/Results row: the link text is the
// path basename and the target is the path relative to the record file.
func relativeRow(label, recPath, target string) string {
	return fmt.Sprintf("| %s | [%s](%s) |", label, path.Base(target), RelativeLink(recPath, target))
}

// absoluteRow renders a row for a file on another branch. With a web URL the
// link text is the path basename and the URL is the full repo-relative path on
// branch ("" falls back to the metadata branch at BlobURLOnBranch); without
// one the cell is the backtick-quoted path.
func absoluteRow(label, target, branch string, link LinkContext) string {
	if url := link.BlobURLOnBranch(target, branch); url != "" {
		return fmt.Sprintf("| %s | [%s](%s) |", label, path.Base(target), url)
	}
	return fmt.Sprintf("| %s | `%s` |", label, target)
}

// adrCell renders the comma-separated ADR cell: each entry is
// [ADR-NNNN](path-relative-to-the-record) for the resolved ADR file.
func adrCell(adrs []domain.ADRID, snap domain.Snapshot, recPath string) (string, error) {
	entries := make([]string, len(adrs))
	for i, id := range adrs {
		adr, outcome := snap.ADR(id)
		if outcome != domain.LookupFound {
			return "", fmt.Errorf("render: cannot resolve ADR-%04d to a record path (lookup outcome %d)", int(id), outcome)
		}
		entries[i] = fmt.Sprintf("[ADR-%04d](%s)", int(id), RelativeLink(recPath, adr.Path()))
	}
	return strings.Join(entries, ", "), nil
}

// ArtifactBacklinkContent renders the full docket:backlink block for a spec,
// plan, or results file on the metadata branch — the two marker lines plus
// "> ↩ **[Change NNNN — Title](<relative path to the record>)**" — targeting
// the change's CURRENT canonical record path (c.Path()) relative to
// artifactPath, the file's own repo-relative path. The link is the same with
// or without a web URL. The result ends with a trailing newline.
func ArtifactBacklinkContent(c domain.Change, artifactPath string) string {
	line := fmt.Sprintf("> ↩ **[Change %04d — %s](%s)**", int(c.ID()), c.Title(), RelativeLink(artifactPath, c.Path()))
	return backlinkStartMarker + "\n" + line + "\n" + backlinkEndMarker + "\n"
}

// PRArtifactLinksContent renders the interior of the PR description's
// docket:artifacts block: absolute metadata-branch links to the plan and
// results ("- Plan: [<base>](<url>)", "- Results: [<base>](<url>)"), since a
// PR body has no branch to be relative to. "" when RepoWebURL is empty or
// neither field is set.
func PRArtifactLinksContent(c domain.Change, link LinkContext) string {
	var b strings.Builder
	for _, a := range []struct{ label, path string }{
		{"Plan", c.Plan().Value},
		{"Results", c.Results().Value},
	} {
		if a.path == "" {
			continue
		}
		url := link.BlobURL(a.path)
		if url == "" {
			return ""
		}
		fmt.Fprintf(&b, "- %s: [%s](%s)\n", a.label, path.Base(a.path), url)
	}
	return b.String()
}

// BacklinkContent renders the full docket:backlink block for a PR description —
// the two marker lines plus the "> ↩ **[Change NNNN — Title](url)**" line —
// targeting the change's CURRENT canonical metadata path (c.Path()) with an
// absolute metadata-branch URL, since a PR body has no branch to be relative
// to. (A file on the metadata branch uses ArtifactBacklinkContent.) In
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
