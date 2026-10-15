package reposetup

// findingtext.go — the shared wording for record-level hand-edit remedies.
//
// A record's path on the metadata branch is not where a human edits it: in a
// shared repository the record lives in the `.docket` checkout, and in a
// private one it lives in a checkout outside the clone. A remedy that names
// only the branch path sends the reader looking in the wrong place. And
// `docket status` must never print a host-absolute path (see the
// StatusResult.HumanText doc comment), so a private checkout is sometimes
// named in words rather than by path. RecordLocator carries whichever of the
// two the caller may print, and builds the two hand-edit remedy shapes from it.

import (
	"errors"
	"fmt"
	"path/filepath"
	"unicode"
	"unicode/utf8"

	"github.com/danielhanold/docket/internal/document"
)

// RecordLocator names where a metadata record is really edited.
//
// Checkout is the checkout directory to name by path: `.docket` in a shared
// repository, or the private checkout path. Label names the checkout in words
// (for example "your private metadata checkout") and is used only when
// Checkout is empty — the form for output that must not print a host-absolute
// path. The zero value names the path "on the metadata branch".
type RecordLocator struct {
	Checkout string
	Label    string
}

// where renders the file reference for rel, or "" when rel is empty (an edit
// that spans several records names no single file).
func (l RecordLocator) where(rel string) string {
	switch {
	case rel == "":
		return ""
	case l.Checkout != "":
		return "`" + filepath.Join(l.Checkout, filepath.FromSlash(rel)) + "`"
	case l.Label != "":
		return "`" + rel + "` in " + l.Label
	default:
		return "`" + rel + "` on the metadata branch"
	}
}

// commitClause tells the reader to commit and push the edit. It is never
// optional: check and status read the pushed metadata branch, so an
// uncommitted edit in the checkout fixes nothing.
func (l RecordLocator) commitClause() string {
	switch {
	case l.Checkout != "":
		return "commit and push it from `" + l.Checkout + "`"
	case l.Label != "":
		return "commit and push it from " + l.Label
	default:
		return "commit and push it"
	}
}

// HandEditThenRepair is the remedy for a problem only a hand edit can fix:
// it names the edit, the file it goes in, the commit, and the trailing
// `docket repository repair` that brings the generated views back in line.
// instruction is a lowercase imperative clause ("remove `172` from
// `depends_on`"); rel is the record's metadata-branch path, or "" when the
// edit spans several records.
func (l RecordLocator) HandEditThenRepair(instruction, rel string) string {
	s := capitalizeFirst(instruction)
	if w := l.where(rel); w != "" {
		s += " in " + w
	}
	return s + ", " + l.commitClause() + ", then run `docket repository repair` so the board and link blocks catch up."
}

// RepairOrHandEdit is the remedy for a problem `docket repository repair` can
// fix itself: it offers the repair first (action says what it does, as a
// lowercase clause) and the equivalent hand edit second. instruction and rel
// are as for HandEditThenRepair.
func (l RecordLocator) RepairOrHandEdit(action, instruction, rel string) string {
	s := "Run `docket repository repair` to " + action + " (it shows the edit first), or " + instruction
	if w := l.where(rel); w != "" {
		s += " in " + w
	}
	return s + " by hand and " + l.commitClause() + "."
}

// PlainDocumentError renders a document-parse error for a finding message:
// the problem, the field or block it names, and the line — without the
// internal error-kind prefix that document.Error.Error carries. Any other
// error renders as its own text.
func PlainDocumentError(err error) string {
	var de *document.Error
	if !errors.As(err, &de) {
		return err.Error()
	}
	s := de.Msg
	if de.Name != "" {
		s += " (`" + de.Name + "`)"
	}
	if de.Line > 0 {
		s += fmt.Sprintf(" on line %d", de.Line)
	}
	return s
}

// capitalizeFirst upper-cases the first rune of s so a lowercase instruction
// clause can open a sentence.
func capitalizeFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}
