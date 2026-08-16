package render

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
)

// Board — the inline BOARD.md surface contract.
//
// This renderer reproduces scripts/render-board.sh's `--format markdown`
// projection byte-for-byte. The script is the byte authority; the contract
// below is inventoried from it (change 0022; status vocabulary change 0104;
// stacked-changes readiness change 0298), and testdata/board/board.golden is a
// frozen historical snapshot of the script's output over
// testdata/board/corpus/ (see testdata/board/PROVENANCE.md). The golden is the
// drift guard: the script dies at the 0316+ cutover, and this renderer must
// keep emitting the same bytes afterward.
//
// Structure, in emission order:
//
//  1. "# Backlog\n\n".
//  2. Counts line: "**<total> changes** — <seg>\n", where <total> is every
//     change record (active + archive) and <seg> joins "<emoji> <n> <label>"
//     with " · " over the lifecycle statuses in display order, skipping any
//     with a zero count. Active-status counts come from active records; the
//     two terminal statuses (done, killed) count archive records. The count
//     label spells in-progress as "in progress" and every other status as its
//     stored token.
//  3. Active sections, one per active status in the fixed display order
//     (in-progress, proposed, blocked, deferred, implemented, stacked-merged),
//     each rendered only when non-empty:
//     "\n## <emoji> <Title><suffix> (<n>)\n\n" then a status-specific table.
//     The suffix is " — awaiting merge" for implemented, empty otherwise.
//     Rows sort by ascending numeric id. Column layouts per status:
//     in-progress:   # | Title | Priority | Type | Spec | Branch
//     proposed:      # | Title | Priority | Type | Readiness
//     blocked:       # | Title | Priority | Type | Blocked by
//     deferred:      # | Title | Priority | Type
//     implemented:   # | Title | Priority | Type | PR | Readiness
//     stacked-merged:# | Title | Priority | Type | PR | Stack
//     Priority renders the stored spelling, Type the stored token or "untyped"
//     when absent, both backtick-quoted. The Spec cell links "../"+the spec
//     path with its leading "docs/" stripped. The Readiness cell (proposed):
//     build-ready                                    → "build-ready"
//     needs-brainstorm                               → "needs-brainstorm"
//     auto-groom-blocked                             → "auto-groom blocked — needs you"
//     waiting-dependency                             → "⏳ waiting on #<dep> — not yet built"
//     / "… — needs your merge"
//     stack-base-unresolved                          → "⏳ waiting on #<parent> — stack base not built"
//     The waiting cell names the representative unmet dependency by its bare
//     id; the stack cell names the change's immediate stacked_on parent,
//     zero-padded. The implemented Readiness cell is "finalize blocked —
//     needs you" when the record carries a "## Finalize blocked" section,
//     empty otherwise.
//  4. A mermaid graph: "\n```mermaid\ngraph TD\n", then every active change in
//     ascending id order — one "  <dep> --> <id>" edge per depends_on entry
//     (padded), or a bare "  <id>" node when it has none — then every archived
//     `done` change referenced by some active change's depends_on styled
//     "  <id>:::done" (ascending id), and a "  classDef done …" line only when
//     at least one such node was emitted, closed by "```\n".
//  5. The archive <details> block, rendered only when at least one archived
//     terminal record exists: a summary line concatenating the present
//     terminal emoji and joining their labels with " + ", then a
//     "# | Title | Merged" table of the archive rows sorted date-descending
//     then id-descending. Every killed row renders verbatim; `done` rows past
//     the 15 most recent collapse into a trailing per-YYYY-MM "Older done
//     (collapsed)" digest. The Merged date is the archive filename's leading
//     YYYY-MM-DD.
//
// Links are repo-relative from docs/changes/ (active/<file>, archive/<file>),
// exactly as the script emits them; the board carries no web-URL variant.
const boardArchiveRecent = 15

// BoardInput is the complete input to the board renderer: the candidate
// snapshot and the branch-existence facts stack-base resolution consults. Facts
// is the zero value when unknown — a zero BranchFacts knows no branches, so a
// stacked change with no resolvable base reports stack-base-unresolved.
type BoardInput struct {
	Snapshot domain.Snapshot
	Facts    domain.BranchFacts
}

// boardActiveStatuses is the active lifecycle group in the board's display
// order (mirrors DOCKET_STATUSES_ACTIVE in scripts/lib/docket-frontmatter.sh).
var boardActiveStatuses = []domain.Status{
	domain.StatusInProgress, domain.StatusProposed, domain.StatusBlocked,
	domain.StatusDeferred, domain.StatusImplemented, domain.StatusStackedMerged,
}

// boardTerminalStatuses is the terminal group in display order (done, killed).
var boardTerminalStatuses = []domain.Status{domain.StatusDone, domain.StatusKilled}

// Board renders docs/changes/BOARD.md byte-for-byte per the contract above.
func Board(in BoardInput) ([]byte, error) {
	activeByStatus := map[domain.Status][]domain.Change{}
	var activeAll []domain.Change
	var archiveChanges []domain.Change
	archiveCount := map[domain.Status]int{}

	for _, c := range in.Snapshot.Changes() {
		switch c.Location() {
		case domain.LocationActive:
			activeByStatus[c.Status()] = append(activeByStatus[c.Status()], c)
			activeAll = append(activeAll, c)
		case domain.LocationArchive:
			archiveChanges = append(archiveChanges, c)
			archiveCount[c.Status()]++
		}
	}
	for s := range activeByStatus {
		sortByID(activeByStatus[s])
	}
	sortByID(activeAll)

	total := len(activeAll) + len(archiveChanges)

	var b strings.Builder
	b.WriteString("# Backlog\n\n")

	// --- counts line ---
	var seg []string
	for _, s := range append(append([]domain.Status{}, boardActiveStatuses...), boardTerminalStatuses...) {
		n := len(activeByStatus[s])
		if isTerminalStatus(s) {
			n = archiveCount[s]
		}
		if n == 0 {
			continue
		}
		seg = append(seg, fmt.Sprintf("%s %d %s", boardEmoji(s), n, boardCountLabel(s)))
	}
	fmt.Fprintf(&b, "**%d changes** — %s\n", total, strings.Join(seg, " · "))

	// --- active sections ---
	for _, s := range boardActiveStatuses {
		rows := activeByStatus[s]
		if len(rows) == 0 {
			continue
		}
		suffix := ""
		if s == domain.StatusImplemented {
			suffix = " — awaiting merge"
		}
		fmt.Fprintf(&b, "\n## %s %s%s (%d)\n\n", boardEmoji(s), boardSectionTitle(s), suffix, len(rows))
		b.WriteString(boardTableHeader(s))
		for _, c := range rows {
			line, err := boardRow(in, s, c)
			if err != nil {
				return nil, err
			}
			b.WriteString(line)
		}
	}

	// --- mermaid ---
	b.WriteString("\n```mermaid\ngraph TD\n")
	referenced := map[int]bool{}
	for _, c := range activeAll {
		deps := c.DependsOn()
		if len(deps) == 0 {
			fmt.Fprintf(&b, "  %04d\n", int(c.ID()))
			continue
		}
		for _, dep := range deps {
			referenced[int(dep)] = true
			fmt.Fprintf(&b, "  %04d --> %04d\n", int(dep), int(c.ID()))
		}
	}
	doneNodes := make([]domain.Change, 0, len(archiveChanges))
	for _, c := range archiveChanges {
		if c.Status() == domain.StatusDone {
			doneNodes = append(doneNodes, c)
		}
	}
	sortByID(doneNodes)
	doneShown := false
	for _, c := range doneNodes {
		if !referenced[int(c.ID())] {
			continue
		}
		fmt.Fprintf(&b, "  %04d:::done\n", int(c.ID()))
		doneShown = true
	}
	if doneShown {
		b.WriteString("  classDef done fill:#d3f9d8;\n")
	}
	b.WriteString("```\n")

	// --- archive ---
	archiveTotal := 0
	for _, s := range boardTerminalStatuses {
		archiveTotal += archiveCount[s]
	}
	if archiveTotal > 0 {
		em, lbl := "", ""
		for _, s := range boardTerminalStatuses {
			if archiveCount[s] == 0 {
				continue
			}
			em += boardEmoji(s)
			if lbl != "" {
				lbl += " + " + string(s)
			} else {
				lbl = string(s)
			}
		}
		fmt.Fprintf(&b, "\n<details><summary>%s Archive — %s (%d)</summary>\n\n", em, lbl, archiveTotal)
		b.WriteString("| # | Title | Merged |\n|---|-------|--------|\n")

		type arcRow struct {
			date, base, title string
			id                int
			status            domain.Status
		}
		rows := make([]arcRow, 0, len(archiveChanges))
		for _, c := range archiveChanges {
			base := path.Base(c.Path())
			date := base
			if len(base) >= 10 {
				date = base[:10]
			}
			rows = append(rows, arcRow{date: date, base: base, title: c.Title(), id: int(c.ID()), status: c.Status()})
		}
		// date descending, then id descending (sort -k1,1r -k2,2nr).
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].date != rows[j].date {
				return rows[i].date > rows[j].date
			}
			return rows[i].id > rows[j].id
		})

		doneSeen := 0
		monthDone := map[string]int{}
		var monthOrder []string
		for _, r := range rows {
			if r.status == domain.StatusDone {
				doneSeen++
				if doneSeen > boardArchiveRecent {
					ym := r.date
					if len(r.date) >= 7 {
						ym = r.date[:7]
					}
					if _, seen := monthDone[ym]; !seen {
						monthOrder = append(monthOrder, ym)
					}
					monthDone[ym]++
					continue
				}
			}
			fmt.Fprintf(&b, "| [%04d](archive/%s) | %s | %s |\n", r.id, r.base, r.title, r.date)
		}
		if len(monthOrder) > 0 {
			b.WriteString("\n**Older done (collapsed)**\n\n")
			b.WriteString("| Month | Done |\n|-------|------|\n")
			for _, ym := range monthOrder {
				fmt.Fprintf(&b, "| [%s](archive/) | %d done |\n", ym, monthDone[ym])
			}
		}
		b.WriteString("\n</details>\n")
	}

	return []byte(b.String()), nil
}

// boardRow renders one active change's table row for its section's layout.
func boardRow(in BoardInput, s domain.Status, c domain.Change) (string, error) {
	id := int(c.ID())
	base := path.Base(c.Path())
	title := c.Title()
	priority := c.RawPriority()
	ctype := boardTypeCell(c)

	switch s {
	case domain.StatusInProgress:
		return fmt.Sprintf("| [%04d](active/%s) | %s | `%s` | `%s` | [spec](%s) | `%s` |\n",
			id, base, title, priority, ctype, boardSpecLink(c.Spec().Value), c.Branch().Value), nil
	case domain.StatusProposed:
		cell, err := boardReadinessCell(in, c)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("| [%04d](active/%s) | %s | `%s` | `%s` | %s |\n",
			id, base, title, priority, ctype, cell), nil
	case domain.StatusBlocked:
		return fmt.Sprintf("| [%04d](active/%s) | %s | `%s` | `%s` | %s |\n",
			id, base, title, priority, ctype, c.BlockedBy().Value), nil
	case domain.StatusDeferred:
		return fmt.Sprintf("| [%04d](active/%s) | %s | `%s` | `%s` |\n",
			id, base, title, priority, ctype), nil
	case domain.StatusImplemented:
		return fmt.Sprintf("| [%04d](active/%s) | %s | `%s` | `%s` | %s | %s |\n",
			id, base, title, priority, ctype, boardPRCell(c), boardImplementedCell(c)), nil
	case domain.StatusStackedMerged:
		return fmt.Sprintf("| [%04d](active/%s) | %s | `%s` | `%s` | %s | %s |\n",
			id, base, title, priority, ctype, boardPRCell(c), boardStackCell(c)), nil
	}
	return "", fmt.Errorf("render: board: change %04d has non-active status %q for row rendering", id, s)
}

// boardReadinessCell renders the proposed-section Readiness cell from the
// domain readiness evaluation (the same policy internal/app/status.go composes).
func boardReadinessCell(in BoardInput, c domain.Change) (string, error) {
	r := domain.EvaluateReadiness(in.Snapshot, c, in.Facts)
	switch r.Kind {
	case domain.ReadyBuildReady:
		return "build-ready", nil
	case domain.ReadyNeedsBrainstorm:
		return "needs-brainstorm", nil
	case domain.ReadyAutoGroomBlocked:
		return "auto-groom blocked — needs you", nil
	case domain.ReadyWaitingDependency:
		return fmt.Sprintf("⏳ waiting on #%d — %s",
			int(r.Dependency.Representative), boardDepReason(r.Dependency.Summary)), nil
	case domain.ReadyStackBaseUnresolved:
		return fmt.Sprintf("⏳ waiting on #%04d — stack base not built", int(c.StackedOn().Value)), nil
	default:
		// A proposed row never reaches here for a well-formed corpus: readiness
		// reports not-proposed only for a non-proposed change, and invalid only
		// for a duplicate/negative id or an unusable slug — both of which the
		// app layer's whole-repository validation rejects before rendering.
		return "", fmt.Errorf("render: board: proposed change %04d has unexpected readiness %q", int(c.ID()), r.Kind)
	}
}

// boardDepReason maps an unmet-dependency summary to the board's cell wording.
func boardDepReason(reason domain.DependencyReason) string {
	if reason == domain.DepNeedsMerge {
		return "needs your merge"
	}
	return "not yet built"
}

// boardImplementedCell renders the implemented Readiness cell: a finalize-blocked
// marker surfaces as a call to action, otherwise the cell is empty.
func boardImplementedCell(c domain.Change) string {
	if c.HasFinalizeBlocked() {
		return "finalize blocked — needs you"
	}
	return ""
}

// boardStackCell renders the stacked-merged Stack cell: the parent whose branch
// this change merged into, or an em dash when no usable parent edge is present.
func boardStackCell(c domain.Change) string {
	parent := c.StackedOn()
	if parent.State != domain.FieldPresent {
		return "—"
	}
	return fmt.Sprintf("merged into #%04d", parent.Value)
}

// boardPRCell renders a PR cell from a stored pr: reference. A full URL becomes
// "[#<n>](<url>)"; a bare value degrades to "#<n>" (the board carries no repo
// to synthesize a URL); an absent pr: is the empty cell.
func boardPRCell(c domain.Change) string {
	pr := c.PR().Value
	if pr == "" {
		return ""
	}
	num := pr
	if i := strings.LastIndex(pr, "/"); i >= 0 {
		num = pr[i+1:]
	}
	if strings.HasPrefix(pr, "http") {
		return fmt.Sprintf("[#%s](%s)", num, pr)
	}
	return fmt.Sprintf("#%s", num)
}

// boardSpecLink renders the in-progress Spec cell target: "../" + the spec path
// with its leading "docs/" stripped (mirrors render-board.sh's spec_link).
func boardSpecLink(spec string) string {
	return "../" + strings.TrimPrefix(spec, "docs/")
}

// boardTypeCell renders the stored change type verbatim, or "untyped" when the
// record carries none — configuration governs creation, never readability.
func boardTypeCell(c domain.Change) string {
	if t := c.Type(); t != "" {
		return t
	}
	return "untyped"
}

// boardTableHeader is the two-line Markdown table header for a status's section.
func boardTableHeader(s domain.Status) string {
	switch s {
	case domain.StatusInProgress:
		return "| # | Title | Priority | Type | Spec | Branch |\n|---|-------|----------|------|------|--------|\n"
	case domain.StatusProposed:
		return "| # | Title | Priority | Type | Readiness |\n|---|-------|----------|------|-----------|\n"
	case domain.StatusBlocked:
		return "| # | Title | Priority | Type | Blocked by |\n|---|-------|----------|------|------------|\n"
	case domain.StatusDeferred:
		return "| # | Title | Priority | Type |\n|---|-------|----------|------|\n"
	case domain.StatusImplemented:
		return "| # | Title | Priority | Type | PR | Readiness |\n|---|-------|----------|------|----|-----------|\n"
	case domain.StatusStackedMerged:
		return "| # | Title | Priority | Type | PR | Stack |\n|---|-------|----------|------|----|-------|\n"
	}
	return ""
}

// boardEmoji is the lifecycle status's board emoji.
func boardEmoji(s domain.Status) string {
	switch s {
	case domain.StatusInProgress:
		return "🟢"
	case domain.StatusProposed:
		return "🟡"
	case domain.StatusBlocked:
		return "🔴"
	case domain.StatusDeferred:
		return "⚪"
	case domain.StatusImplemented:
		return "🔵"
	case domain.StatusStackedMerged:
		return "🪆"
	case domain.StatusDone:
		return "✅"
	case domain.StatusKilled:
		return "🗑️"
	}
	return ""
}

// boardCountLabel is the status's label in the counts line: in-progress spells
// as "in progress", every other status as its stored token.
func boardCountLabel(s domain.Status) string {
	if s == domain.StatusInProgress {
		return "in progress"
	}
	return string(s)
}

// boardSectionTitle is the status's section heading title.
func boardSectionTitle(s domain.Status) string {
	switch s {
	case domain.StatusInProgress:
		return "In progress"
	case domain.StatusProposed:
		return "Proposed"
	case domain.StatusBlocked:
		return "Blocked"
	case domain.StatusDeferred:
		return "Deferred"
	case domain.StatusImplemented:
		return "Implemented"
	case domain.StatusStackedMerged:
		return "Stacked-merged"
	}
	return ""
}

// isTerminalStatus reports whether s is an archive-side terminal status.
func isTerminalStatus(s domain.Status) bool {
	return s == domain.StatusDone || s == domain.StatusKilled
}

// sortByID sorts changes in ascending numeric id order in place.
func sortByID(cs []domain.Change) {
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].ID() < cs[j].ID() })
}
