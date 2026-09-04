package render

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/danielhanold/docket/internal/domain"
)

// Board — the inline BOARD.md surface contract (change 0367).
//
// Board() renders the backlog as six configurable presentation groups whose
// order and per-section sort come from the caller's BoardPresentation (built
// from resolved configuration; the renderer invents no defaults and refuses an
// invalid presentation — see BoardPresentation.validate). Each active change is
// classified into exactly one rendered group by boardClassify — a mapping that
// is deliberately NOT one-to-one with lifecycle status (finalize-blocked
// implemented changes render Blocked; spec-backed build-ready proposals render
// Groomed). testdata/board/board.golden is the byte-drift guard: it is the
// reviewed canonical render of testdata/board/corpus/ under the DEFAULT
// presentation (see testdata/board/PROVENANCE.md), re-founded at change 0367
// when the default output changed by design. A diff there is a real change to
// the board surface and must be re-blessed deliberately.
//
// Structure, in emission order:
//
//  1. "# Backlog\n\n".
//  2. Counts line: "**<total> changes** — <seg>\n", where <total> is every
//     change record (active + archive) and <seg> joins "<emoji> <n> <label>"
//     with " · ", iterating the configured section order over the six rendered
//     groups (count = classified membership) then the terminal done/killed
//     archive counts, skipping any group with a zero count. Group labels:
//     in-progress→"in progress", built→"built", blocked→"blocked",
//     groomed→"groomed", proposed→"proposed", deferred→"deferred";
//     terminal labels are the stored "done"/"killed" tokens.
//  3. Active sections, one per rendered group in the configured section order,
//     each emitted only when non-empty:
//     "\n## <emoji> <Title> (<n>)\n\n" then a group-specific table. Rows sort
//     by the group's BoardSort (sortBoardSection). There is no heading suffix;
//     the Built State column carries the awaiting-merge wording. Emoji/title
//     per group: 🟢 In progress, 🔵 Built, 🔴 Blocked, 🟣 Groomed,
//     🟡 Proposed, ⚪ Deferred. Column layouts per group:
//     in-progress: # | Title | Priority | Type | Spec | Branch | Readiness
//     built:       # | Title | Priority | Type | PR | State
//     blocked:     # | Title | Priority | Type | PR | Reason
//     groomed:     # | Title | Priority | Type | Spec
//     proposed:    # | Title | Priority | Type | Readiness
//     deferred:    # | Title | Priority | Type
//     Priority renders the stored spelling, Type the stored token or "untyped"
//     when absent, both backtick-quoted. The Spec cell links "../"+the spec
//     path with its leading "docs/" stripped. The Built State cell is
//     "awaiting merge" for an implemented change and "merged into #<parent>"
//     (zero-padded, "—" when no usable parent edge) for a stacked-merged
//     change. The Blocked Reason cell is the stored blocked_by text for a
//     lifecycle-blocked change and "finalize blocked — needs you" for a
//     finalize-blocked implemented change; its PR cell comes from boardPRCell
//     (empty when no pr:).
//     The In progress Readiness cell is "run halted — needs you" when the
//     change carries the bare "## Run halted" body section (HasRunHalted),
//     else empty.
//     The Proposed Readiness cell:
//     build-ready                                    → "build-ready" / "build-ready (trivial)"
//     needs-brainstorm                               → "needs-brainstorm"
//     auto-groom-blocked                             → "auto-groom blocked — needs you"
//     waiting-dependency                             → "⏳ waiting on #<dep> — not yet built"
//     / "… — needs your merge"
//     stack-base-unresolved                          → "⏳ waiting on #<parent> — stack base not built"
//     The waiting cell names the representative unmet dependency by its bare
//     id; the stack cell names the change's immediate stacked_on parent,
//     zero-padded. A build-ready proposal that reaches Proposed is necessarily
//     trivial-without-spec (spec-backed build-ready is Groomed) and renders
//     "build-ready (trivial)".
//  4. A mermaid graph: "\n```mermaid\ngraph TD\n", then every active change in
//     ascending id order — one "  <dep> --> <id>" edge per depends_on entry
//     (padded), or a bare "  <id>" node when it has none — then every archived
//     `done` change referenced by some active change's depends_on styled
//     "  <id>:::done" (ascending id), and a "  classDef done …" line only when
//     at least one such node was emitted, closed by "```\n". The mermaid graph
//     is outside section order/sorting: it never reads the presentation.
//  5. The archive <details> block, rendered only when at least one archived
//     terminal record exists: a summary line concatenating the present
//     terminal emoji and joining their labels with " + ", then a
//     "# | Title | Merged" table of the archive rows sorted date-descending
//     then id-descending. Every killed row renders verbatim; `done` rows past
//     the 15 most recent collapse into a trailing per-YYYY-MM "Older done
//     (collapsed)" digest. The Merged date is the archive filename's leading
//     YYYY-MM-DD. The archive is a fixed footer, outside section order/sorting.
//
// Links are repo-relative from docs/changes/ (active/<file>, archive/<file>);
// the board carries no web-URL variant.
const boardArchiveRecent = 15

// BoardInput is the complete input to the board renderer: the candidate
// snapshot and the branch-existence facts stack-base resolution consults. Facts
// is the zero value when unknown — a zero BranchFacts knows no branches, so a
// stacked change with no resolvable base reports stack-base-unresolved.
type BoardInput struct {
	Snapshot domain.Snapshot
	Facts    domain.BranchFacts
	// Presentation is the typed section-order-and-sort policy the resolved
	// config produces (change 0367). Board() iterates it: the section order
	// drives the counts line and section emission, and each section's BoardSort
	// drives its row order. A caller building options fills it from config; the
	// renderer invents no defaults and refuses an invalid presentation.
	Presentation BoardPresentation
}

// BoardSection is one rendered board group — a closed vocabulary deliberately
// distinct from the lifecycle statuses a change carries (change 0367). A change
// is classified into exactly one section; the mapping is not one-to-one with
// status (finalize-blocked implemented changes render Blocked, spec-backed
// build-ready proposals render Groomed).
type BoardSection string

const (
	BoardSectionInProgress BoardSection = "in-progress"
	BoardSectionBuilt      BoardSection = "built"
	BoardSectionBlocked    BoardSection = "blocked"
	BoardSectionGroomed    BoardSection = "groomed"
	BoardSectionProposed   BoardSection = "proposed"
	BoardSectionDeferred   BoardSection = "deferred"
)

// boardSectionOrderDefault is the built-in permutation, in display order.
var boardSectionOrderDefault = []BoardSection{
	BoardSectionInProgress, BoardSectionBuilt, BoardSectionBlocked,
	BoardSectionGroomed, BoardSectionProposed, BoardSectionDeferred,
}

// BoardSortKey names the field a section sorts on: "id" | "updated" | "created".
type BoardSortKey string

const (
	BoardSortKeyID      BoardSortKey = "id"
	BoardSortKeyUpdated BoardSortKey = "updated"
	BoardSortKeyCreated BoardSortKey = "created"
)

// BoardDirection names a sort direction: "asc" | "desc".
type BoardDirection string

const (
	BoardDirectionAsc  BoardDirection = "asc"
	BoardDirectionDesc BoardDirection = "desc"
)

// BoardSort is one section's sort policy.
type BoardSort struct {
	By        BoardSortKey
	Direction BoardDirection
}

// BoardPresentation is the typed presentation policy the resolved config
// produces. Board() refuses an incomplete or invalid presentation — the
// renderer never fills defaults, so a caller that forgot to build options
// fails loudly instead of silently rendering the built-in view.
type BoardPresentation struct {
	SectionOrder []BoardSection
	Sorting      map[BoardSection]BoardSort
}

// DefaultBoardPresentation returns the built-in presentation: the default
// permutation, updated desc everywhere.
func DefaultBoardPresentation() BoardPresentation {
	order := append([]BoardSection(nil), boardSectionOrderDefault...)
	sorting := make(map[BoardSection]BoardSort, len(order))
	for _, s := range order {
		sorting[s] = BoardSort{By: BoardSortKeyUpdated, Direction: BoardDirectionDesc}
	}
	return BoardPresentation{SectionOrder: order, Sorting: sorting}
}

// validate reports whether p is a well-formed presentation: SectionOrder is a
// complete permutation of the six rendered sections (each exactly once, none
// unknown, none missing) and Sorting carries a valid sort (a known key and a
// known direction) for every section. Config owns user-facing fallback; an
// invalid presentation reaching the renderer is a docket wiring bug, reported
// as an error so a caller that forgot to build options fails loudly instead of
// silently rendering the built-in view. Every message names "presentation".
func (p BoardPresentation) validate() error {
	if len(p.SectionOrder) != len(boardSectionOrderDefault) {
		return fmt.Errorf("render: board: presentation section order has %d entries, want %d",
			len(p.SectionOrder), len(boardSectionOrderDefault))
	}
	seen := make(map[BoardSection]bool, len(boardSectionOrderDefault))
	for _, s := range p.SectionOrder {
		if !boardSectionKnown(s) {
			return fmt.Errorf("render: board: presentation section order names unknown section %q", s)
		}
		if seen[s] {
			return fmt.Errorf("render: board: presentation section order lists %q more than once", s)
		}
		seen[s] = true
	}
	for _, s := range boardSectionOrderDefault {
		if !seen[s] {
			return fmt.Errorf("render: board: presentation section order is missing section %q", s)
		}
	}
	for _, s := range boardSectionOrderDefault {
		srt, ok := p.Sorting[s]
		if !ok {
			return fmt.Errorf("render: board: presentation is missing a sort for section %q", s)
		}
		if !boardSortKeyValid(srt.By) {
			return fmt.Errorf("render: board: presentation section %q has invalid sort key %q", s, srt.By)
		}
		if !boardDirectionValid(srt.Direction) {
			return fmt.Errorf("render: board: presentation section %q has invalid sort direction %q", s, srt.Direction)
		}
	}
	return nil
}

// boardSectionKnown reports whether s is one of the six rendered sections.
func boardSectionKnown(s BoardSection) bool {
	switch s {
	case BoardSectionInProgress, BoardSectionBuilt, BoardSectionBlocked,
		BoardSectionGroomed, BoardSectionProposed, BoardSectionDeferred:
		return true
	}
	return false
}

// boardSortKeyValid reports whether k is one of the three sort fields.
func boardSortKeyValid(k BoardSortKey) bool {
	return k == BoardSortKeyID || k == BoardSortKeyUpdated || k == BoardSortKeyCreated
}

// boardDirectionValid reports whether d is one of the two sort directions.
func boardDirectionValid(d BoardDirection) bool {
	return d == BoardDirectionAsc || d == BoardDirectionDesc
}

// boardClassify maps one ACTIVE change to exactly one rendered section, per the
// spec's precedence: Blocked → In progress → Built → Groomed → Proposed →
// Deferred. Pure; mutates nothing. A non-active (terminal) status is a caller
// error — the board classifies only active records.
func boardClassify(in BoardInput, c domain.Change) (BoardSection, error) {
	switch c.Status() {
	case domain.StatusBlocked:
		return BoardSectionBlocked, nil
	case domain.StatusInProgress:
		return BoardSectionInProgress, nil
	case domain.StatusImplemented:
		if c.HasFinalizeBlocked() {
			return BoardSectionBlocked, nil
		}
		return BoardSectionBuilt, nil
	case domain.StatusStackedMerged:
		return BoardSectionBuilt, nil
	case domain.StatusProposed:
		// Groomed means build-ready AND spec-backed: a trivial build-ready
		// change with an empty spec stays Proposed.
		if c.Spec().Value != "" &&
			domain.EvaluateReadiness(in.Snapshot, c, in.Facts).Kind == domain.ReadyBuildReady {
			return BoardSectionGroomed, nil
		}
		return BoardSectionProposed, nil
	case domain.StatusDeferred:
		return BoardSectionDeferred, nil
	}
	return "", fmt.Errorf("render: board: change %04d has non-active status %q", int(c.ID()), c.Status())
}

// sortBoardSection orders rows in place per s. The comparator is total and
// deterministic: the primary key is s.By in s.Direction; equal date values
// tie-break on numeric ID in the SAME direction; rows with a missing, empty, or
// malformed date sort after every valid-dated row regardless of direction, and
// among themselves order by ID in the configured direction. Arrival order never
// decides — the comparator is a strict weak ordering over a total key, so a
// stable sort's fallback to input position is unreachable.
func sortBoardSection(rows []domain.Change, s BoardSort) {
	desc := s.Direction == BoardDirectionDesc
	idLess := func(a, b domain.Change) bool {
		if desc {
			return a.ID() > b.ID()
		}
		return a.ID() < b.ID()
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if s.By == BoardSortKeyID {
			return idLess(a, b)
		}
		at, aok := boardSortDate(a, s.By)
		bt, bok := boardSortDate(b, s.By)
		switch {
		case aok && !bok:
			return true // valid dates before unknown, regardless of direction
		case !aok && bok:
			return false
		case !aok && !bok:
			return idLess(a, b)
		case at.Equal(bt):
			return idLess(a, b)
		case desc:
			return at.After(bt)
		default:
			return at.Before(bt)
		}
	})
}

// boardSortDate extracts the change's date for key (updated or created); ok is
// false for a missing, empty, or malformed field (State != FieldPresent), which
// the comparator sorts after every valid date. A key that is neither updated
// nor created (i.e. id, handled by the caller before this point) yields ok
// false, so a mis-keyed call degrades to the unknown-date band rather than
// silently reading a wrong field.
func boardSortDate(c domain.Change, key BoardSortKey) (time.Time, bool) {
	var ot domain.OptionalTime
	switch key {
	case BoardSortKeyUpdated:
		ot = c.Updated()
	case BoardSortKeyCreated:
		ot = c.Created()
	default:
		return time.Time{}, false
	}
	if ot.State != domain.FieldPresent {
		return time.Time{}, false
	}
	return ot.Value, true
}

// boardTerminalStatuses is the terminal group in display order (done, killed).
var boardTerminalStatuses = []domain.Status{domain.StatusDone, domain.StatusKilled}

// Board renders docs/changes/BOARD.md per the contract above.
func Board(in BoardInput) ([]byte, error) {
	if err := in.Presentation.validate(); err != nil {
		return nil, err
	}

	bySection := map[BoardSection][]domain.Change{}
	var activeAll []domain.Change
	var archiveChanges []domain.Change
	archiveCount := map[domain.Status]int{}

	for _, c := range in.Snapshot.Changes() {
		switch c.Location() {
		case domain.LocationActive:
			sec, err := boardClassify(in, c)
			if err != nil {
				return nil, err
			}
			bySection[sec] = append(bySection[sec], c)
			activeAll = append(activeAll, c)
		case domain.LocationArchive:
			archiveChanges = append(archiveChanges, c)
			archiveCount[c.Status()]++
		}
	}
	sortByID(activeAll)

	total := len(activeAll) + len(archiveChanges)

	var b strings.Builder
	b.WriteString("# Backlog\n\n")

	// --- counts line: rendered groups in configured order, then archive ---
	var seg []string
	for _, s := range in.Presentation.SectionOrder {
		n := len(bySection[s])
		if n == 0 {
			continue
		}
		seg = append(seg, fmt.Sprintf("%s %d %s", boardSectionEmoji(s), n, boardSectionCountLabel(s)))
	}
	for _, s := range boardTerminalStatuses {
		n := archiveCount[s]
		if n == 0 {
			continue
		}
		seg = append(seg, fmt.Sprintf("%s %d %s", boardEmoji(s), n, string(s)))
	}
	fmt.Fprintf(&b, "**%d changes** — %s\n", total, strings.Join(seg, " · "))

	// --- active sections in configured order ---
	for _, s := range in.Presentation.SectionOrder {
		rows := bySection[s]
		if len(rows) == 0 {
			continue
		}
		sortBoardSection(rows, in.Presentation.Sorting[s])
		fmt.Fprintf(&b, "\n## %s %s (%d)\n\n", boardSectionEmoji(s), boardSectionHeading(s), len(rows))
		b.WriteString(boardSectionTableHeader(s))
		for _, c := range rows {
			line, err := boardSectionRow(in, s, c)
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

// boardSectionRow renders one active change's table row for its rendered
// section's layout. The change has already been classified into s by
// boardClassify, so the section — not the raw lifecycle status — selects the
// column shape and cell wording.
func boardSectionRow(in BoardInput, s BoardSection, c domain.Change) (string, error) {
	id := int(c.ID())
	base := path.Base(c.Path())
	title := c.Title()
	priority := c.RawPriority()
	ctype := boardTypeCell(c)

	switch s {
	case BoardSectionInProgress:
		return fmt.Sprintf("| [%04d](active/%s) | %s | `%s` | `%s` | [spec](%s) | `%s` | %s |\n",
			id, base, title, priority, ctype, boardSpecLink(c.Spec().Value), c.Branch().Value,
			boardInProgressReadinessCell(c)), nil
	case BoardSectionBuilt:
		return fmt.Sprintf("| [%04d](active/%s) | %s | `%s` | `%s` | %s | %s |\n",
			id, base, title, priority, ctype, boardPRCell(c), boardBuiltStateCell(c)), nil
	case BoardSectionBlocked:
		return fmt.Sprintf("| [%04d](active/%s) | %s | `%s` | `%s` | %s | %s |\n",
			id, base, title, priority, ctype, boardPRCell(c), boardBlockedReasonCell(c)), nil
	case BoardSectionGroomed:
		return fmt.Sprintf("| [%04d](active/%s) | %s | `%s` | `%s` | [spec](%s) |\n",
			id, base, title, priority, ctype, boardSpecLink(c.Spec().Value)), nil
	case BoardSectionProposed:
		cell, err := boardReadinessCell(in, c)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("| [%04d](active/%s) | %s | `%s` | `%s` | %s |\n",
			id, base, title, priority, ctype, cell), nil
	case BoardSectionDeferred:
		return fmt.Sprintf("| [%04d](active/%s) | %s | `%s` | `%s` |\n",
			id, base, title, priority, ctype), nil
	}
	return "", fmt.Errorf("render: board: change %04d has unknown section %q for row rendering", id, s)
}

// boardBuiltStateCell renders the Built State cell: a stacked-merged change
// carries its "merged into #<parent>" edge (reusing boardStackCell); every
// other Built row is a healthy implemented change awaiting merge (finalize-
// blocked implemented changes classify Blocked, not Built).
func boardBuiltStateCell(c domain.Change) string {
	if c.Status() == domain.StatusStackedMerged {
		return boardStackCell(c)
	}
	return "awaiting merge"
}

// boardBlockedReasonCell renders the Blocked Reason cell: a finalize-blocked
// implemented change surfaces the call to action, and a lifecycle-blocked
// change surfaces its stored blocked_by text. (The only implemented change that
// reaches the Blocked section is finalize-blocked — boardClassify's precedence.)
func boardBlockedReasonCell(c domain.Change) string {
	if c.Status() == domain.StatusImplemented {
		return "finalize blocked — needs you"
	}
	return c.BlockedBy().Value
}

// boardInProgressReadinessCell renders the In progress Readiness cell: an
// in-progress change carrying the bare "## Run halted" body section (0237's
// stop + surface disposition, decoded as HasRunHalted) surfaces the call to
// action in the family idiom of boardBlockedReasonCell's
// "finalize blocked — needs you"; every other row renders an empty cell.
func boardInProgressReadinessCell(c domain.Change) string {
	if c.HasRunHalted() {
		return "run halted — needs you"
	}
	return ""
}

// boardReadinessCell renders the proposed-section Readiness cell from the
// domain readiness evaluation (the same policy internal/app/status.go composes).
func boardReadinessCell(in BoardInput, c domain.Change) (string, error) {
	r := domain.EvaluateReadiness(in.Snapshot, c, in.Facts)
	switch r.Kind {
	case domain.ReadyBuildReady:
		// The only build-ready row that reaches Proposed is trivial-without-spec
		// (spec-backed build-ready is Groomed); mark it so the reader sees why it
		// skipped grooming.
		if c.Trivial() {
			return "build-ready (trivial)", nil
		}
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

// boardSectionTableHeader is the two-line Markdown table header for a rendered
// section's layout.
func boardSectionTableHeader(s BoardSection) string {
	switch s {
	case BoardSectionInProgress:
		return "| # | Title | Priority | Type | Spec | Branch | Readiness |\n|---|-------|----------|------|------|--------|-----------|\n"
	case BoardSectionBuilt:
		return "| # | Title | Priority | Type | PR | State |\n|---|-------|----------|------|----|-------|\n"
	case BoardSectionBlocked:
		return "| # | Title | Priority | Type | PR | Reason |\n|---|-------|----------|------|----|--------|\n"
	case BoardSectionGroomed:
		return "| # | Title | Priority | Type | Spec |\n|---|-------|----------|------|------|\n"
	case BoardSectionProposed:
		return "| # | Title | Priority | Type | Readiness |\n|---|-------|----------|------|-----------|\n"
	case BoardSectionDeferred:
		return "| # | Title | Priority | Type |\n|---|-------|----------|------|\n"
	}
	return ""
}

// boardSectionEmoji is the rendered section's board emoji.
func boardSectionEmoji(s BoardSection) string {
	switch s {
	case BoardSectionInProgress:
		return "🟢"
	case BoardSectionBuilt:
		return "🔵"
	case BoardSectionBlocked:
		return "🔴"
	case BoardSectionGroomed:
		return "🟣"
	case BoardSectionProposed:
		return "🟡"
	case BoardSectionDeferred:
		return "⚪"
	}
	return ""
}

// boardSectionCountLabel is the rendered section's label in the counts line.
func boardSectionCountLabel(s BoardSection) string {
	switch s {
	case BoardSectionInProgress:
		return "in progress"
	case BoardSectionBuilt:
		return "built"
	case BoardSectionBlocked:
		return "blocked"
	case BoardSectionGroomed:
		return "groomed"
	case BoardSectionProposed:
		return "proposed"
	case BoardSectionDeferred:
		return "deferred"
	}
	return ""
}

// boardSectionHeading is the rendered section's heading title.
func boardSectionHeading(s BoardSection) string {
	switch s {
	case BoardSectionInProgress:
		return "In progress"
	case BoardSectionBuilt:
		return "Built"
	case BoardSectionBlocked:
		return "Blocked"
	case BoardSectionGroomed:
		return "Groomed"
	case BoardSectionProposed:
		return "Proposed"
	case BoardSectionDeferred:
		return "Deferred"
	}
	return ""
}

// boardEmoji is the terminal (archive) status's board emoji, used by the counts
// line and the archive summary; the active groups use boardSectionEmoji.
func boardEmoji(s domain.Status) string {
	switch s {
	case domain.StatusDone:
		return "✅"
	case domain.StatusKilled:
		return "🗑️"
	}
	return ""
}

// sortByID sorts changes in ascending numeric id order in place.
func sortByID(cs []domain.Change) {
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].ID() < cs[j].ID() })
}
