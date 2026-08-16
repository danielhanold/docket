package render

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
)

// ADRIndex — the docs/adrs/README.md ADR-index surface contract.
//
// This renderer reproduces scripts/render-adr-index.sh's output byte-for-byte.
// The script is the byte authority; the contract below is inventoried from it
// (change 0030), and testdata/adrindex/index.golden is a frozen historical
// snapshot of the script's output over testdata/adrindex/adrs/ (see
// testdata/adrindex/PROVENANCE.md). The golden is the drift guard: the script
// dies at the 0316+ cutover, and this renderer must keep emitting the same
// bytes afterward.
//
// Structure, in emission order:
//
//  1. "# Architecture Decision Records\n\n".
//  2. The fixed prose line, verbatim, then "\n".
//  3. Three groups in fixed order — "Active", "Superseded / Reversed",
//     "Deprecated" — each emitted as "\n## <Header>\n\n" then either its rows or
//     "_None._\n" when the group is empty. Grouping is on the ADR's stored raw
//     status string, exactly as the script's case: a status beginning
//     "Superseded by" or "Reversed by" goes to the second group; a status
//     exactly "Deprecated" goes to the third; everything else (Accepted,
//     Proposed, draft, unknown) is Active. Rows within a group sort by ascending
//     numeric id.
//
// Each row is the script's row() output:
//
//	"- [ADR-<pad>](<file>) — <title> (<rawstatus>)" then, appended only when
//	present and in this fixed order:
//	  " ← change #<change>"        when the ADR names a producing change,
//	  " → supersedes <adr-list>"   when supersedes is non-empty,
//	  " → reverses <adr-list>"     when reverses is non-empty,
//	  " · relates to <adr-list>"   when relates_to is non-empty,
//	where <adr-list> joins the ids with ", " as "ADR-<pad>" each. Ids are
//	zero-padded to four digits. The file cell is the ADR's basename (the script
//	reads a flat --adrs-dir and emits basename links).
func ADRIndex(snap domain.Snapshot) ([]byte, error) {
	var active, supRev, deprecated []domain.ADR
	for _, a := range snap.ADRs() {
		switch adrIndexGroup(a.RawStatus()) {
		case adrGroupSupRev:
			supRev = append(supRev, a)
		case adrGroupDeprecated:
			deprecated = append(deprecated, a)
		default:
			active = append(active, a)
		}
	}

	var b strings.Builder
	b.WriteString("# Architecture Decision Records\n\n")
	b.WriteString("Immutable, numbered record of *why*. ADRs are never archived or rewritten; once `Accepted`, only the `status:` line changes (on supersession/reversal). This index is generated — do not hand-edit.\n")
	adrEmitGroup(&b, "Active", active)
	adrEmitGroup(&b, "Superseded / Reversed", supRev)
	adrEmitGroup(&b, "Deprecated", deprecated)
	return []byte(b.String()), nil
}

// adrGroup names the three index groups in emission order.
type adrGroup int

const (
	adrGroupActive adrGroup = iota
	adrGroupSupRev
	adrGroupDeprecated
)

// adrIndexGroup classifies an ADR by its stored raw status string, matching the
// script's case on syntactic prefix (not on the parsed status kind): the raw
// text is what the script buckets on, so grouping must too.
func adrIndexGroup(rawStatus string) adrGroup {
	switch {
	case strings.HasPrefix(rawStatus, "Superseded by"), strings.HasPrefix(rawStatus, "Reversed by"):
		return adrGroupSupRev
	case rawStatus == "Deprecated":
		return adrGroupDeprecated
	default:
		return adrGroupActive
	}
}

// adrEmitGroup writes one group's "\n## <header>\n\n" heading then its rows in
// ascending id order, or "_None._\n" when the group is empty.
func adrEmitGroup(b *strings.Builder, header string, adrs []domain.ADR) {
	fmt.Fprintf(b, "\n## %s\n\n", header)
	if len(adrs) == 0 {
		b.WriteString("_None._\n")
		return
	}
	sort.SliceStable(adrs, func(i, j int) bool { return adrs[i].ID() < adrs[j].ID() })
	for _, a := range adrs {
		b.WriteString(adrRow(a))
	}
}

// adrRow renders one ADR's index row, mirroring the script's row().
func adrRow(a domain.ADR) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- [ADR-%04d](%s) — %s (%s)", int(a.ID()), path.Base(a.Path()), a.Title(), a.RawStatus())
	if ch := a.Change(); ch.State == domain.FieldPresent {
		fmt.Fprintf(&b, " ← change #%d", ch.Value)
	}
	if sup := a.Supersedes(); len(sup) > 0 {
		fmt.Fprintf(&b, " → supersedes %s", adrIDList(sup))
	}
	if rev := a.Reverses(); len(rev) > 0 {
		fmt.Fprintf(&b, " → reverses %s", adrIDList(rev))
	}
	if rel := a.RelatesTo(); len(rel) > 0 {
		fmt.Fprintf(&b, " · relates to %s", adrIDList(rel))
	}
	b.WriteString("\n")
	return b.String()
}

// adrIDList joins ADR ids as "ADR-<pad>, ..." in supplied order, mirroring the
// script's adr_list().
func adrIDList(ids []domain.ADRID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("ADR-%04d", int(id))
	}
	return strings.Join(parts, ", ")
}
