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
	"strings"
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
//
// SetupBlocked marks a repository whose setup is not healthy: its checkout may
// not exist yet and `docket repository repair` refuses until the setup is
// fixed, so every remedy names the record on the metadata branch (Checkout and
// Label are ignored) and sends the reader to the setup finding before any
// repair.
type RecordLocator struct {
	Checkout     string
	Label        string
	SetupBlocked bool
}

// setupResolved is the condition a SetupBlocked remedy puts before any repair.
const setupResolved = "once the setup finding above is resolved"

// where renders the file reference for rel, or "" when rel is empty (an edit
// that spans several records names no single file).
func (l RecordLocator) where(rel string) string {
	switch {
	case rel == "":
		return ""
	case l.SetupBlocked:
		return "`" + rel + "` on the metadata branch"
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
// uncommitted edit in the checkout fixes nothing. rel is as for where: an
// edit that names no single file commits "the edited records", so the clause
// never points an "it" at nothing.
func (l RecordLocator) commitClause(rel string) string {
	what := "it"
	if rel == "" {
		what = "the edited records"
	}
	switch {
	case l.SetupBlocked:
		return "commit and push " + what
	case l.Checkout != "":
		return "commit and push " + what + " from `" + l.Checkout + "`"
	case l.Label != "":
		return "commit and push " + what + " from " + l.Label
	default:
		return "commit and push " + what
	}
}

// edit renders the hand edit and its commit as one lowercase clause, the file
// first ("in `<file>`, <instruction>; then commit and push it ..."). The file
// leads so it can never attach to the instruction's last example or
// alternative.
func (l RecordLocator) edit(instruction, rel string) string {
	s := instruction
	if w := l.where(rel); w != "" {
		s = "in " + w + ", " + instruction
	}
	return s + "; then " + l.commitClause(rel)
}

// HandEditThenRepair is the remedy for a problem only a hand edit can fix:
// it names the file, the edit, the commit, and the trailing
// `docket repository repair` that brings the generated views back in line.
// instruction is a lowercase imperative clause ("remove `172` from
// `depends_on`"); rel is the record's metadata-branch path, or "" when the
// edit spans several records.
func (l RecordLocator) HandEditThenRepair(instruction, rel string) string {
	s := capitalizeFirst(l.edit(instruction, rel))
	if l.SetupBlocked {
		return s + ". " + capitalizeFirst(setupResolved) + ", run `docket repository repair` so the board and link blocks catch up."
	}
	return s + ", and run `docket repository repair` so the board and link blocks catch up."
}

// RepairOrHandEdit is the remedy for a problem `docket repository repair` can
// fix itself: it offers the repair first (action says what it does, as a
// lowercase clause) and the equivalent hand edit second. When the setup is
// blocked the repair refuses, so the hand edit comes first and the repair
// waits on the setup finding. instruction and rel are as for
// HandEditThenRepair.
func (l RecordLocator) RepairOrHandEdit(action, instruction, rel string) string {
	repair := "run `docket repository repair` to " + action + " (it shows the edit first)"
	if l.SetupBlocked {
		return "Edit it by hand: " + l.edit(instruction, rel) + ". Or, " + setupResolved + ", " + repair + "."
	}
	return capitalizeFirst(repair) + ". Or edit it by hand: " + l.edit(instruction, rel) + "."
}

// RunRepair is the remedy for a generated view only `docket repository repair`
// rewrites: purpose is what follows the command (" to regenerate it (it shows
// the change first)", or ""). When the setup is blocked the repair refuses, so
// the setup step comes first.
func (l RecordLocator) RunRepair(purpose string) string {
	if l.SetupBlocked {
		return capitalizeFirst(setupResolved) + ", run `docket repository repair`" + purpose + "."
	}
	return "Run `docket repository repair`" + purpose + "."
}

// PlainDocumentError renders a document-parse error for a finding message:
// the problem, the field or block it names, and the line — without the
// internal error-kind prefix that document.Error.Error carries. The name is
// left out when the message already quotes it, and otherwise leads the
// clause ("for `id` on line 3, ..."), so the text never stacks a second
// parenthetical or repeats the name. Any other error renders as its own text.
func PlainDocumentError(err error) string {
	var de *document.Error
	if !errors.As(err, &de) {
		return err.Error()
	}
	line := ""
	if de.Line > 0 {
		line = fmt.Sprintf(" on line %d", de.Line)
	}
	if de.Name == "" || strings.Contains(de.Msg, "'"+de.Name+"'") || strings.Contains(de.Msg, "`"+de.Name+"`") {
		return de.Msg + line
	}
	return "for `" + de.Name + "`" + line + ", " + de.Msg
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
