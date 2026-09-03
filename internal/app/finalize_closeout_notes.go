package app

// finalize_closeout_notes.go — the optional authored closeout-notes payload:
// its normalized shape, its validation, the digest that binds it into the
// closeout receipt, and the splice that lands it as the terminal record's
// final authored body section. The renderer (render.CloseoutNotesBody) owns
// every Markdown byte; nothing here concatenates caller text into structure.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/render"
)

// CloseoutNotes is the normalized optional payload `finalize closeout`
// accepts: exactly two ordered lists. Nil and empty lists are canonically
// identical (both normalize to nil), so "no input" and "explicitly empty
// input" key the same promise.
type CloseoutNotes struct {
	VerificationOutcomes []string
	LateFindings         []string
}

// Empty reports whether the notes carry no entries.
func (n CloseoutNotes) Empty() bool {
	return len(n.VerificationOutcomes) == 0 && len(n.LateFindings) == 0
}

// normalizeCloseoutNotes trims each entry and validates the whole input set
// before anything acts on any of it (learnings: validate-the-whole-input-set-
// first). An entry empty after trimming is invalid, never silently dropped;
// entries carrying C0 control characters other than '\n' and '\t' (or DEL),
// or reserved managed-marker text, are rejected; each entry and the rendered
// whole are bounded by the shared authored-input bound. Order is preserved.
func normalizeCloseoutNotes(n CloseoutNotes) (CloseoutNotes, []StatusFinding) {
	var findings []StatusFinding
	norm := func(label string, entries []string) []string {
		if len(entries) == 0 {
			return nil // canonical: empty and absent are the same request
		}
		out := make([]string, 0, len(entries))
		for i, e := range entries {
			t := strings.TrimSpace(e)
			if t == "" {
				findings = append(findings, lifecycleFinding(FCEmptyNoteEntry,
					fmt.Sprintf("%s[%d] is empty after trimming; drop the entry or write one", label, i)))
				continue
			}
			if reason := invalidNoteText(t); reason != "" {
				findings = append(findings, lifecycleFinding(FCInvalidNoteEntry,
					fmt.Sprintf("%s[%d] %s", label, i, reason)))
				continue
			}
			boundAuthored(&findings, fmt.Sprintf("%s[%d]", label, i), t)
			out = append(out, t)
		}
		return out
	}
	res := CloseoutNotes{
		VerificationOutcomes: norm("verification_outcomes", n.VerificationOutcomes),
		LateFindings:         norm("late_findings", n.LateFindings),
	}
	boundAuthored(&findings, "closeout notes",
		render.CloseoutNotesBody(res.VerificationOutcomes, res.LateFindings))
	return res, findings
}

// invalidNoteText names why an entry cannot be rendered safely, or "" when it
// can. '\n' is legal (a multiline bullet; the renderer indents continuations)
// and '\t' is legal content; every other control character — including '\r',
// which would smuggle a CR into an LF document — corrupts the record.
// Managed-marker text is rejected so an entry can never open or close a
// generated block.
func invalidNoteText(t string) string {
	for _, r := range t {
		if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f {
			return "carries a control character that could corrupt the record"
		}
	}
	if strings.Contains(t, "<!-- docket:") {
		return "carries reserved managed-marker text"
	}
	return ""
}

// closeoutNotesDigest keys the promise being made: the exact rendered section
// body. Empty notes digest to "" so a no-notes receipt is byte-identical to a
// pre-notes-era receipt (learnings: idempotency-keying — key on the promised
// state, never a proxy).
func closeoutNotesDigest(n CloseoutNotes) string {
	if n.Empty() {
		return ""
	}
	sum := sha256.Sum256([]byte(render.CloseoutNotesBody(n.VerificationOutcomes, n.LateFindings)))
	return hex.EncodeToString(sum[:])
}

// closeoutNotesHeadingSet is the closed owned-heading set the notes splice may
// touch.
var closeoutNotesHeadingSet = []string{render.CloseoutNotesHeading}

// spliceCloseoutNotes lands the rendered section as the record's final
// authored body section via the marker/fence-aware section editor (append-at-
// EOF when absent, replace when present). Empty notes return src unchanged —
// today's closeout stays byte-for-byte identical.
func spliceCloseoutNotes(src []byte, n CloseoutNotes) ([]byte, error) {
	if n.Empty() {
		return src, nil
	}
	return render.ApplySectionEdits(src, closeoutNotesHeadingSet, []render.SectionEdit{{
		Heading:  render.CloseoutNotesHeading,
		Intent:   render.SectionReplace,
		Markdown: render.CloseoutNotesBody(n.VerificationOutcomes, n.LateFindings),
	}})
}
