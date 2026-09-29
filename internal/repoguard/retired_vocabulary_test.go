package repoguard

import (
	"fmt"
	"go/scanner"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/claude"
	"github.com/danielhanold/docket/internal/harness/codex"
	"github.com/danielhanold/docket/internal/harness/cursor"
	"github.com/danielhanold/docket/internal/harness/opencode"
	"github.com/danielhanold/docket/internal/install"
)

// This file is the retired-vocabulary table (ADR-0129 Decision 10) and the
// absence seal it drives. Change 0471 (family (a), the run tracker) created it;
// each later ADR-0129 family change APPENDS its rows to retiredVocabulary and
// never creates a second table. Every seal failure names the replacement.
//
// SCOPE, stated where the population is defined (learning
// byte-pattern-guard-matches-a-spelling): the seal scans the maintained
// executable surface (repoguard.ExecutableSurface: shell, exec-bit files,
// scripts/ and skills/ markdown), the always-loaded agent-instruction surface
// (repoguard.AlwaysLoadedSurface: AGENTS.md, CLAUDE.md, agents/**,
// cursor-rules/**), the STRING LITERALS of non-test Go source, and generator
// output. Markdown and shell are scanned line by line, with a `#` comment
// stripped from shell/config lines; markdown prose IS scanned, because a skill's
// prose names catalog operations an agent resolves and runs. Go comments and
// *_test.go files are NOT scanned: a comment is a passing mention, and a test
// may legitimately assert that an old spelling is refused. docs/, testdata/,
// tests/fixtures/ and internal/install/legacydata/ are excluded by
// MaintainedFiles itself (point-in-time records and frozen corpora).
//
// LIMITATION: the match is a bounded spelling, not the property. A retired
// token re-assembled at runtime from fragments (e.g. "gate-" + "armed") escapes
// it; that is accepted, as for every repoguard spelling seal.

// retiredKind selects which sites a retired row matches.
type retiredKind int

const (
	// kindToken: the bounded token anywhere in a scanned line or Go string
	// literal. Leading boundary: start or a non-[A-Za-z0-9_] byte (so a flag's
	// leading "--" and an op id's "run." prefix still match); trailing boundary:
	// end or a non-[A-Za-z0-9_-] byte (so gate-stop never matches gate-stopped,
	// gate_context never matches the kept gate_context_hash, and a hyphenated
	// continuation such as the frozen ADR filename 0074-build-gate-verdict-is-…
	// is a different word, not the retired gate-verdict).
	kindToken retiredKind = iota
	// kindGoFlag: a non-test Go string literal whose whole content equals Old —
	// a flag-definition or flag-lookup name ("run-epoch", "epoch").
	kindGoFlag
	// kindJSONKey: a non-test Go struct tag naming Old as its json key.
	kindJSONKey
	// kindGoPrefix: a non-test Go string literal that starts with Old — the
	// retired error-text prefix.
	kindGoPrefix
)

// retiredToken is one row of the retired-vocabulary table.
type retiredToken struct {
	Row  string      // ADR-0129 rename-table row, e.g. "7", "38c"
	Kind retiredKind // which sites match
	Old  string      // the retired spelling
	New  string      // its replacement, named in every failure
	// Kept, when non-nil, INVERTS the row onto a kept namesake: every
	// occurrence is retired unless Kept reports it syntactically bound to the
	// namesake. text is the scanned block (a markdown paragraph, one shell or
	// config line, or one Go literal's content) and text[lo:hi] the occurrence.
	// Row 12 retires `change claim --gate-context`; the gate drive's own
	// --gate-context is not an ADR-0129 row and stays, so the row is keyed on
	// what binds a flag to the gate drive, never on how prose names the claim
	// (review finding 3 of change 0471: "the Step-2 claim's --gate-context"
	// named no `change claim` and evaded a claim-keyed scope).
	Kept func(rel, text string, lo, hi int) bool
}

var (
	// gateDriveHeadRe: a command segment led by a gate-drive command and at most
	// one subcommand word — "docket gate drive start", "gate.drive.prepare-scope".
	gateDriveHeadRe = regexp.MustCompile(`^(?:\$\s+)?(?:docket\s+)?gate[ .-]drive(?:[ .][a-z][a-z-]*)?(?:\s|$)`)
	// gateDriveQualifierRe: the text before a flag (or before its flag-only code
	// span) ends in a gate-drive qualifier — "gate-drive `--gate-context`", "the
	// gate drive's --gate-context".
	gateDriveQualifierRe = regexp.MustCompile(`gate[ .-]drive(?:'s|’s)?\s*$`)
	// gateDriveOpSpanRe: the house argv idiom "the `gate.drive.start` operation
	// with `--flags …`" — a gate-drive command span, then nothing but the words
	// "operation"/"command" and "with", then the flag-only span.
	gateDriveOpSpanRe = regexp.MustCompile("`(?:docket\\s+)?gate[ .-]drive[^`]*`\\s*(?:(?:operation|command)\\s*)?(?:with\\s*)?$")
)

// gateDriveBound is row 12's Kept: a --gate-context occurrence is the gate
// drive's kept flag only when it is syntactically bound to a gate-drive
// command —
//   - inside a code span whose command segment (after the last shell
//     separator) is a gate-drive command followed only by argv;
//   - inside a flag-only code span introduced by a gate-drive qualifier or by
//     a gate-drive command span plus "operation with";
//   - outside any code span, right after a gate-drive qualifier, or in a line
//     segment that is a gate-drive command followed only by argv.
//
// Everything else is the retired change-claim flag, however the prose names
// its owner.
func gateDriveBound(_, text string, lo, _ int) bool {
	if s, ok := enclosingCodeSpan(text, lo); ok {
		seg := afterShellSeparator(text[s:lo])
		if bound, decided := gateDriveCommand(seg); decided {
			return bound
		}
		pre := text[:s-1] // before the span's opening backtick
		return argvShaped(seg) && (gateDriveQualifierRe.MatchString(pre) || gateDriveOpSpanRe.MatchString(pre))
	}
	if gateDriveQualifierRe.MatchString(text[:lo]) {
		return true
	}
	line := text[strings.LastIndex(text[:lo], "\n")+1 : lo]
	bound, _ := gateDriveCommand(afterShellSeparator(line))
	return bound
}

// gateDriveFlagFile is row 12's Kept for the Go flag-name literal: the gate
// drive's --gate-context is registered and read only in its command file, so
// the bare name "gate-context" anywhere else is the retired change-claim flag.
func gateDriveFlagFile(rel, _ string, _, _ int) bool {
	return rel == "internal/cli/gate.go"
}

// gateDriveCommand classifies a command segment ending at a flag: decided is
// false when the segment is flag-only argv (its binding lies before it), and
// bound is true when it is a gate-drive command followed only by argv.
func gateDriveCommand(seg string) (bound, decided bool) {
	t := strings.TrimLeft(seg, " \t\n[")
	if m := gateDriveHeadRe.FindStringIndex(t); m != nil {
		return argvShaped(t[m[1]:]), true
	}
	if t == "" || strings.HasPrefix(t, "-") {
		return false, false
	}
	return false, true
}

// argvShaped reports whether s is only flags, each followed by at most one
// value word — never prose. Brackets of optional flags and a trailing
// line-continuation backslash are ignored.
func argvShaped(s string) bool {
	afterFlag := false
	for _, f := range strings.Fields(s) {
		f = strings.Trim(f, "[]")
		switch {
		case f == "" || f == "\\":
		case strings.HasPrefix(f, "-"):
			afterFlag = true
		case afterFlag:
			afterFlag = false
		default:
			return false
		}
	}
	return true
}

// afterShellSeparator returns the part of s after its last command separator
// (&&, ||, ;, | or an opening parenthesis), or s when it has none.
func afterShellSeparator(s string) string {
	cut := 0
	for _, sep := range []string{"&&", "||", ";", "|", "("} {
		if i := strings.LastIndex(s, sep); i >= 0 && i+len(sep) > cut {
			cut = i + len(sep)
		}
	}
	return s[cut:]
}

// enclosingCodeSpan returns the start of the content of the backtick code span
// holding text[lo] (backticks paired in order; an unpaired last one opens
// nothing).
func enclosingCodeSpan(text string, lo int) (int, bool) {
	open := -1
	for i := 0; i < len(text); i++ {
		if text[i] != '`' {
			continue
		}
		if open < 0 {
			open = i
			continue
		}
		if open < lo && lo < i {
			return open + 1, true
		}
		open = -1
	}
	return 0, false
}

// tokenSpans returns every bounded occurrence of old in text, with the same
// boundaries as tokenRe.
func tokenSpans(text, old string) [][2]int {
	var out [][2]int
	for i := 0; i < len(text); {
		j := strings.Index(text[i:], old)
		if j < 0 {
			break
		}
		lo, hi := i+j, i+j+len(old)
		if (lo == 0 || !isIdentByte(text[lo-1])) && (hi == len(text) || !(isIdentByte(text[hi]) || text[hi] == '-')) {
			out = append(out, [2]int{lo, hi})
		}
		i = lo + 1
	}
	return out
}

func isIdentByte(b byte) bool {
	return b == '_' || '0' <= b && b <= '9' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z'
}

// retiredVocabulary is the table. Append-only across ADR-0129 families.
var retiredVocabulary = []retiredToken{
	// Family (a) — run tracker (change 0471): rows 7-37, 38 (storage names), 38a-38d.
	{Row: "7", Kind: kindToken, Old: "gate-before", New: "run.start / docket run start"},
	{Row: "8", Kind: kindToken, Old: "gate-verdict", New: "run.verdict / docket run verdict"},
	{Row: "9", Kind: kindToken, Old: "gate-claim", New: "run.continue / docket run continue"},
	{Row: "10", Kind: kindToken, Old: "--run-epoch", New: "--run-id"},
	{Row: "10", Kind: kindGoFlag, Old: "run-epoch", New: "run-id"},
	{Row: "11", Kind: kindToken, Old: "--epoch", New: "run cancel --run-id"},
	{Row: "11", Kind: kindGoFlag, Old: "epoch", New: "run-id"},
	{Row: "12", Kind: kindToken, Old: "--gate-context", New: "change claim --run-context", Kept: gateDriveBound},
	{Row: "12", Kind: kindGoFlag, Old: "gate-context", New: "run-context", Kept: gateDriveFlagFile},
	{Row: "13", Kind: kindToken, Old: "gate-armed", New: "run-started"},
	{Row: "13", Kind: kindToken, Old: "gate-unarmed", New: "run-untracked"},
	{Row: "13", Kind: kindJSONKey, Old: "armed", New: "started"},
	{Row: "14", Kind: kindToken, Old: "gate-retry-once", New: "run-retry-once"},
	{Row: "14", Kind: kindToken, Old: "gate-continue", New: "run-continue"},
	{Row: "14", Kind: kindToken, Old: "gate-done", New: "run-done"},
	{Row: "14", Kind: kindToken, Old: "gate-stop", New: "run-stop"},
	{Row: "14", Kind: kindToken, Old: "gate-observe", New: "run-observe"},
	{Row: "15", Kind: kindToken, Old: "gate-claimed", New: "run-continued"},
	{Row: "16", Kind: kindToken, Old: "epoch-not-found", New: "run-not-found"},
	{Row: "17", Kind: kindToken, Old: "epoch-corrupt", New: "run-record-corrupt"},
	{Row: "18", Kind: kindToken, Old: "epoch-exists", New: "run-exists"},
	{Row: "19", Kind: kindToken, Old: "epoch-not-active", New: "run-not-active"},
	{Row: "20", Kind: kindToken, Old: "epoch-mismatch", New: "run-id-mismatch"},
	{Row: "21", Kind: kindToken, Old: "epoch-not-cancelled", New: "run-not-cancelled"},
	{Row: "22", Kind: kindToken, Old: "epoch-ambiguous", New: "run-ambiguous"},
	{Row: "23", Kind: kindToken, Old: "epoch-owner-ambiguous", New: "run-owner-ambiguous"},
	{Row: "24", Kind: kindToken, Old: "epoch-owner-unresolved", New: "run-owner-unresolved"},
	{Row: "25", Kind: kindToken, Old: "epoch-io", New: "run-record-io"},
	{Row: "26", Kind: kindToken, Old: "epoch-participant-unknown", New: "run-participant-unknown"},
	{Row: "27", Kind: kindToken, Old: "epoch-state-unknown", New: "run-state-unknown"},
	{Row: "28", Kind: kindToken, Old: "epoch-unreadable", New: "run-record-unreadable"},
	{Row: "29", Kind: kindToken, Old: "incumbent-epoch-fenced", New: "incumbent-run-fenced"},
	{Row: "30", Kind: kindToken, Old: "resume-epoch-unreadable", New: "resume-run-record-unreadable"},
	{Row: "31", Kind: kindToken, Old: "stale-run-epoch", New: "stale-run-id"},
	{Row: "32", Kind: kindToken, Old: "unknown-run-epoch", New: "unknown-run-id"},
	{Row: "33", Kind: kindToken, Old: "gate-unavailable", New: "run-tracker-unavailable"},
	{Row: "34", Kind: kindToken, Old: "gate-context-invalid", New: "run-context-invalid"},
	{Row: "34", Kind: kindToken, Old: "gate-context-conflict", New: "run-context-conflict"},
	{Row: "35", Kind: kindToken, Old: "mint-epoch", New: "mint-run"},
	{Row: "35", Kind: kindToken, Old: "find-epoch", New: "find-run"},
	{Row: "35", Kind: kindToken, Old: "complete-epoch", New: "complete-run"},
	{Row: "35", Kind: kindToken, Old: "supersede-epoch", New: "supersede-run"},
	{Row: "36", Kind: kindToken, Old: "write-epoch", New: "write-run-record"},
	{Row: "36", Kind: kindToken, Old: "load-epoch", New: "load-run-record"},
	{Row: "36", Kind: kindToken, Old: "epoch-cas", New: "run-record-cas"},
	{Row: "37", Kind: kindToken, Old: "bind-epoch-change", New: "bind-run-change"},
	{Row: "37", Kind: kindToken, Old: "bind-epoch-worktree", New: "bind-run-worktree"},
	{Row: "37", Kind: kindToken, Old: "epoch-launch-gate", New: "run-launch-gate"},
	{Row: "37", Kind: kindToken, Old: "reserve-worktree-execution-epoch", New: "reserve-worktree-execution-run"},
	{Row: "37", Kind: kindToken, Old: "retire-worktree-execution-epoch", New: "retire-worktree-execution-run"},
	{Row: "37", Kind: kindGoPrefix, Old: "run epoch ", New: "error-text prefix \"run \""},
	{Row: "38", Kind: kindToken, Old: "rungate", New: "run-tracker (storage root)"},
	{Row: "38", Kind: kindToken, Old: "rungate-resume", New: "run-tracker-resume (storage root)"},
	{Row: "38", Kind: kindToken, Old: "epoch.json", New: "run.json"},
	{Row: "38", Kind: kindToken, Old: "epoch.lock", New: "run.lock"},
	{Row: "38a", Kind: kindToken, Old: "--run-gate-key", New: "--run-key"},
	{Row: "38a", Kind: kindGoFlag, Old: "run-gate-key", New: "run-key"},
	{Row: "38b", Kind: kindJSONKey, Old: "epoch", New: "run_id"},
	{Row: "38c", Kind: kindToken, Old: "gate_context", New: "run_context"},
	{Row: "38d", Kind: kindToken, Old: "DOCKET_AGENT_GUARDIAN_EPOCH", New: "DOCKET_AGENT_GUARDIAN_RUN_ID"},
}

// retiredHit is one seal violation.
type retiredHit struct {
	rel  string
	line int
	row  retiredToken
	text string
}

func (h retiredHit) String() string {
	return fmt.Sprintf("%s:%d: ADR-0129 row %s: retired %q — use %s: %s", h.rel, h.line, h.row.Row, h.row.Old, h.row.New, h.text)
}

var tokenReCache = map[string]*regexp.Regexp{}

// tokenRe is the bounded matcher for a kindToken row (see kindToken).
func tokenRe(old string) *regexp.Regexp {
	if re, ok := tokenReCache[old]; ok {
		return re
	}
	re := regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(old) + `([^A-Za-z0-9_-]|$)`)
	tokenReCache[old] = re
	return re
}

// scanTextLine checks one markdown/shell/generated line against every
// unscoped kindToken row. A row with Kept is scanned per block instead
// (scanKeptRows), because its binding can span lines.
func scanTextLine(rel string, lineNo int, line string) []retiredHit {
	var hits []retiredHit
	for _, r := range retiredVocabulary {
		if r.Kind != kindToken || r.Kept != nil {
			continue
		}
		if tokenRe(r.Old).MatchString(line) {
			hits = append(hits, retiredHit{rel, lineNo, r, strings.TrimSpace(line)})
		}
	}
	return hits
}

// scanTextContent scans a markdown/shell/config file. Shell and config lines
// have their `#` comment stripped (stripHashComment, absence_test.go); markdown
// is scanned whole, prose included.
func scanTextContent(rel, content string) []retiredHit {
	return scanText(rel, content, isMarkdownSurface(rel))
}

// scanText scans content line by line for the unscoped rows and block by block
// (a markdown paragraph, or one line otherwise) for the rows with Kept.
func scanText(rel, content string, md bool) []retiredHit {
	lines := strings.Split(content, "\n")
	if !md {
		for i := range lines {
			lines[i] = stripHashComment(lines[i])
		}
	}
	var hits []retiredHit
	for i, line := range lines {
		hits = append(hits, scanTextLine(rel, i+1, line)...)
	}
	for a := 0; a < len(lines); {
		b := a + 1
		if md && !isParagraphBreak(lines[a]) {
			for b < len(lines) && !isParagraphBreak(lines[b]) {
				b++
			}
		}
		hits = append(hits, scanKeptRows(rel, lines, a, b)...)
		a = b
	}
	return hits
}

// isParagraphBreak: a blank line or a code-fence line ends a markdown block (a
// fence's backticks must not pair with an inline code span's).
func isParagraphBreak(line string) bool {
	t := strings.TrimSpace(line)
	return t == "" || strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}

// scanKeptRows checks lines[a:b], joined as one block, against every kindToken
// row with Kept: each occurrence Kept does not bind is a hit on its line.
func scanKeptRows(rel string, lines []string, a, b int) []retiredHit {
	text := strings.Join(lines[a:b], "\n")
	var hits []retiredHit
	for _, r := range retiredVocabulary {
		if r.Kind != kindToken || r.Kept == nil {
			continue
		}
		last := -1
		for _, sp := range tokenSpans(text, r.Old) {
			if r.Kept(rel, text, sp[0], sp[1]) {
				continue
			}
			if ln := a + strings.Count(text[:sp[0]], "\n"); ln != last {
				last = ln
				hits = append(hits, retiredHit{rel, ln + 1, r, strings.TrimSpace(lines[ln])})
			}
		}
	}
	return hits
}

var jsonTagKeyRe = regexp.MustCompile(`json:"([^",]*)`)

// scanGoLiteral checks one Go string literal's content against every row.
func scanGoLiteral(rel string, lineNo int, lit string) []retiredHit {
	val := lit
	if unq, err := strconv.Unquote(lit); err == nil {
		val = unq
	}
	var hits []retiredHit
	for _, r := range retiredVocabulary {
		hit := false
		switch r.Kind {
		case kindToken:
			for _, sp := range tokenSpans(val, r.Old) {
				if r.Kept == nil || !r.Kept(rel, val, sp[0], sp[1]) {
					hit = true
				}
			}
		case kindGoFlag:
			hit = val == r.Old && (r.Kept == nil || !r.Kept(rel, val, 0, len(val)))
		case kindGoPrefix:
			hit = strings.HasPrefix(val, r.Old)
		case kindJSONKey:
			for _, m := range jsonTagKeyRe.FindAllStringSubmatch(val, -1) {
				if m[1] == r.Old {
					hit = true
				}
			}
		}
		if hit {
			hits = append(hits, retiredHit{rel, lineNo, r, lit})
		}
	}
	return hits
}

// scanGoSource tokenizes one non-test Go file and scans its string literals
// (interpreted and raw, struct tags included). Comments are not scanned.
func scanGoSource(rel string, src []byte) ([]retiredHit, error) {
	fset := token.NewFileSet()
	file := fset.AddFile(rel, fset.Base(), len(src))
	var s scanner.Scanner
	var scanErr error
	s.Init(file, src, func(pos token.Position, msg string) { scanErr = fmt.Errorf("%s: %s", pos, msg) }, 0)
	var hits []retiredHit
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.STRING {
			hits = append(hits, scanGoLiteral(rel, fset.Position(pos).Line, lit)...)
		}
	}
	return hits, scanErr
}

// goSourcePop returns the maintained non-test Go files.
func goSourcePop(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, rel := range maintainedPop(t, root) {
		if strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") {
			out = append(out, rel)
		}
	}
	return out
}

// TestRetiredVocabularySeal is the ADR-0129 absence seal.
func TestRetiredVocabularySeal(t *testing.T) {
	t.Run("table_integrity", testRetiredTableIntegrity)
	t.Run("non_vacuity", testRetiredNonVacuity)
	t.Run("negative_controls", testRetiredNegativeControls)
	t.Run("maintained_surfaces", testRetiredMaintainedSurfaces)
	t.Run("generator_output", testRetiredGeneratorOutput)
}

// testRetiredTableIntegrity: a malformed row would seal nothing or name no
// replacement. The floor stops a truncated table from passing vacuously.
func testRetiredTableIntegrity(t *testing.T) {
	const floor = 60
	if len(retiredVocabulary) < floor {
		t.Fatalf("retired-vocabulary table has %d rows, expected >= %d", len(retiredVocabulary), floor)
	}
	seen := map[string]bool{}
	for _, r := range retiredVocabulary {
		if r.Row == "" || r.Old == "" || r.New == "" {
			t.Errorf("incomplete row %+v", r)
		}
		key := fmt.Sprintf("%d|%s", r.Kind, r.Old)
		if seen[key] {
			t.Errorf("duplicate row for %q", r.Old)
		}
		seen[key] = true
	}
}

// testRetiredNonVacuity plants every row in each site of its kind and demands
// a hit ATTRIBUTED TO THAT ROW that names its replacement: a detector that
// stopped matching reddens here. (A token nested in a longer retired token, e.g.
// epoch-unreadable inside resume-epoch-unreadable, also hits its own shorter
// row; only the planted row's own hits are counted.)
func testRetiredNonVacuity(t *testing.T) {
	goHits := func(rel, src string) []retiredHit {
		t.Helper()
		hits, err := scanGoSource(rel, []byte(src))
		if err != nil {
			t.Fatalf("tokenize planted source: %v", err)
		}
		return hits
	}
	for _, r := range retiredVocabulary {
		var hits []retiredHit
		want := 0
		switch r.Kind {
		case kindToken:
			line := "run `" + r.Old + "` now"
			hits = append(hits, scanTextContent("skills/x/SKILL.md", line)...)
			hits = append(hits, scanTextContent("tests/test_x.sh", "docket "+line)...)
			hits = append(hits, goHits("internal/p/p.go", "package p\nvar v = "+strconv.Quote("x "+r.Old+" y")+"\n")...)
			want = 3
		case kindGoFlag, kindGoPrefix:
			rel := "internal/cli/run.go"
			lit := r.Old
			if r.Kind == kindGoPrefix {
				lit = r.Old + "%s"
			}
			hits = goHits(rel, "package p\nvar v = "+strconv.Quote(lit)+"\n")
			want = 1
		case kindJSONKey:
			hits = goHits("internal/p/p.go", "package p\ntype T struct {\n\tF string `json:\""+r.Old+",omitempty\"`\n}\n")
			want = 1
		}
		own := 0
		for _, h := range hits {
			if h.row.Kind != r.Kind || h.row.Old != r.Old {
				continue
			}
			own++
			if !strings.Contains(h.String(), r.New) {
				t.Errorf("row %s: failure %q does not name the replacement %q", r.Row, h, r.New)
			}
		}
		if own < want {
			t.Errorf("row %s: %q detected in %d of %d planted sites", r.Row, r.Old, own, want)
		}
	}
	// Row 12 on a line naming both nouns: the change-claim flag must still be
	// caught next to a kept gate-drive one (gateDriveBound, whose kept side
	// negative_controls pins).
	mixed := "`gate drive start --gate-context <ctx>`, then `change claim --gate-context <ctx>`"
	row12 := 0
	for _, h := range scanTextContent("skills/x/SKILL.md", mixed) {
		if h.row.Row == "12" {
			row12++
		}
	}
	if row12 == 0 {
		t.Errorf("row 12: change claim --gate-context not detected on a line that also names gate drive: %q", mixed)
	}
	// Row 12 is inverted (gateDriveBound): a --gate-context NOT syntactically
	// bound to a gate-drive command is retired, whatever noun the prose uses for
	// its owner. The first case is the pre-0471 implement-next "Verify the run"
	// clause, which names gate.drive four times and change.claim never, yet its
	// last flag is the claim's (review finding 3 of change 0471).
	for _, line := range []string{
		"pass it into every `gate.drive.prepare-scope` / `gate.drive.start --gate-context` this run performs — each invoked with `--json` per the shared capture requirement — and into the Step-2 claim's --gate-context.",
		"pass it into `gate.drive.start` and into the Step-2 claim's `--gate-context`",
		"gate drive start takes the token, and the Step-2 claim's --gate-context",
		"`gate drive start --owner task && change claim --gate-context <ctx>`",
		"the `gate.drive.start` operation, then the claim with `--gate-context <ctx>`",
	} {
		if !hasRetiredRow(scanTextContent("skills/x/SKILL.md", line), "12") {
			t.Errorf("row 12: a --gate-context not bound to a gate-drive command was not detected: %q", line)
		}
	}
	unbound := "package p\nvar m = " + strconv.Quote("(the <run-context> goes to --gate-context)") + "\n"
	if !hasRetiredRow(goHits("internal/app/x.go", unbound), "12") {
		t.Errorf("row 12: an unbound --gate-context in a Go literal was not detected")
	}
}

// hasRetiredRow reports whether hits include one attributed to row.
func hasRetiredRow(hits []retiredHit, row string) bool {
	for _, h := range hits {
		if h.row.Row == row {
			return true
		}
	}
	return false
}

// testRetiredNegativeControls: kept namesakes must never match (spec §4 shape
// boundaries). Each is a real spelling the tree legitimately carries.
func testRetiredNegativeControls(t *testing.T) {
	cleanText := []string{
		"the claim receipt keeps gate_context_hash",
		"releasepkg --source-epoch 1700000000",
		"SOURCE_DATE_EPOCH=1700000000",
		"the suite gate-failed; a gate-scope participant",
		"gatedrive-owned drives and idempotent-suite-gate",
		"evidence reason gate-stopped",
		// Frozen ADR filenames (docs/adrs/**) that concept pages link to: the
		// trailing hyphen exclusion keeps "gate-verdict-is" from matching row 8.
		"see docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md",
		"see docs/adrs/0075-run-gate-attributes-a-claim-conservatively-and-reports-a-halt-with-its-own-exit-code.md",
		"docket gate drive prepare-scope --gate-context <ctx>",
		"docket gate drive start --gate-context <ctx> --run-id <id>",
		"run.start then run.verdict then run.continue",
		// Row 12's scope: the generated dispatch clause names both nouns, and its
		// --gate-context is the gate drive's kept flag.
		"include the run-context token, labeled for `change.claim --run-context` and gate-drive `--gate-context`, and the run id",
		// The other gate-drive bindings gateDriveBound accepts: the same code span
		// as a gate-drive command, and a gate-drive qualifier before the flag.
		"pass it into every `gate.drive.prepare-scope` / `gate.drive.start --gate-context` this run performs",
		"plus gate-drive `--gate-context <token>` if dispatched with one",
		"(the <run-context> goes to the gate drive's --gate-context)",
		"| `prepare-scope` | `gate.drive.prepare-scope --change-id <id> [--gate-context <token>] [--run-id <id>]`: mint",
	}
	for _, line := range cleanText {
		for _, rel := range []string{"skills/x/SKILL.md", "tests/test_x.sh"} {
			if hits := scanTextContent(rel, line); len(hits) != 0 {
				t.Errorf("negative control %q in %s matched: %v", line, rel, hits)
			}
		}
	}
	// Markdown only: the house argv idiom "the `<gate-drive op>` operation with
	// `<flags>`", whose flag span wraps across lines of one paragraph.
	cleanMarkdown := []string{
		"the `gate.drive.start` operation with `--owner task\n--repo-dir <w> --scope-id <id> --child-cap <token> --gate-context <token> --run-root\n<dir> --json -- <the test command>`. Every identity value comes in your dispatch\nprompt — pass the bundle through unchanged, omitting gate-drive `--gate-context` only when no dispatch\ncontext reached you;",
		"run the `gate.drive.prepare-scope`\noperation with `--change-id <id> --worktree\n<worktree> --gate-context <run-context> --run-id <run-id> --json` (the run context",
	}
	for _, md := range cleanMarkdown {
		if hits := scanTextContent("skills/x/SKILL.md", md); len(hits) != 0 {
			t.Errorf("markdown negative control %q matched: %v", md, hits)
		}
	}
	cleanGo := []struct{ rel, src string }{
		{"internal/app/change_claim.go", "package p\ntype R struct {\n\tH string `json:\"gate_context_hash\"`\n}\n"},
		{"cmd/releasepkg/main.go", "package p\nvar f = \"source-epoch\"\n"},
		{"internal/cli/gate.go", "package p\nvar f = \"gate-context\"\n"},
		{"internal/app/x.go", "package p\nvar m = \"(the <run-context> goes to the gate drive's --gate-context)\"\n"},
		{"internal/app/evidence_ops.go", "package p\nvar r = \"gate-stopped\"\n"},
		{"internal/p/p.go", "package p\n// run epoch gate-armed in a comment is a passing mention\nvar x = 1\n"},
	}
	for _, c := range cleanGo {
		hits, err := scanGoSource(c.rel, []byte(c.src))
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 0 {
			t.Errorf("negative control %s matched: %v", c.rel, hits)
		}
	}
}

// testRetiredMaintainedSurfaces scans the committed tree.
func testRetiredMaintainedSurfaces(t *testing.T) {
	root := guardRoot(t)
	text := execPop(t, root)
	for _, rel := range alwaysLoadedPop(t, root) {
		if !slices.Contains(text, rel) {
			text = append(text, rel)
		}
	}
	goPop := goSourcePop(t, root)
	// Population floors FIRST: a broken walk passes every absence assert vacuously.
	if len(text) < 45 {
		t.Fatalf("population floor: text surface collapsed to %d files (expected >= 45)", len(text))
	}
	if len(goPop) < 150 {
		t.Fatalf("population floor: non-test Go surface collapsed to %d files (expected >= 150)", len(goPop))
	}
	var violations []string
	for _, rel := range text {
		if strings.HasSuffix(rel, ".go") {
			continue // Go files are scanned by literal below, never as text
		}
		for _, h := range scanTextContent(rel, readMaintained(t, root, rel)) {
			violations = append(violations, h.String())
		}
	}
	for _, rel := range goPop {
		hits, err := scanGoSource(rel, []byte(readMaintained(t, root, rel)))
		if err != nil {
			t.Fatalf("tokenize %s: %v (fail closed)", rel, err)
		}
		for _, h := range hits {
			violations = append(violations, h.String())
		}
	}
	if len(violations) != 0 {
		t.Errorf("retired ADR-0129 vocabulary survives in maintained executable surfaces (%d):\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

// testRetiredGeneratorOutput renders every dispatch surface docket writes into
// consumer repos — each adapter's targets, the shared dispatch interior (Codex
// variant, a superset), and the cursor rule — and scans every emitted line,
// prose included.
func testRetiredGeneratorOutput(t *testing.T) {
	cat, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatalf("EmbeddedCatalog: %v", err)
	}
	in := harness.PlanInput{
		Assets:    cat,
		Mode:      harness.ModeRelease,
		AssetsDir: "/data/versions/sha256-x/assets",
		Roots: install.UserRoots{
			Home:       "/home/u",
			DataRoot:   "/home/u/.local/share/docket",
			ConfigHome: "/home/u/.config",
			BinDir:     "/home/u/.local/bin",
		},
		Agents: crossAgentsTable(),
	}
	outputs := map[string][]byte{}
	adapters := map[string]harness.Adapter{
		"claude": claude.New(), "codex": codex.New(), "cursor": cursor.New(), "opencode": opencode.New(),
	}
	for _, name := range harness.Order {
		targets, err := adapters[name].Plan(in)
		if err != nil {
			t.Fatalf("%s Plan: %v", name, err)
		}
		for i, tg := range targets {
			outputs[fmt.Sprintf("%s/%03d-%s", name, i, filepath.Base(tg.Path))] = tg.Content
		}
	}
	payload, err := harness.RunTracker(cat)
	if err != nil {
		t.Fatalf("RunTracker: %v", err)
	}
	outputs["dispatch/codex-interior.md"] = []byte(harness.CodexDispatchInterior(payload))
	outputs["dispatch/cursor-rule.mdc"] = cursor.DispatchRuleContent(payload)
	if len(outputs) < 20 {
		t.Fatalf("generator population floor: %d outputs (expected >= 20)", len(outputs))
	}
	var violations []string
	for rel, b := range outputs {
		for _, h := range scanText(rel, string(b), true) {
			violations = append(violations, "[generator-output] "+h.String())
		}
	}
	slices.Sort(violations)
	if len(violations) != 0 {
		t.Errorf("retired ADR-0129 vocabulary in GENERATOR OUTPUT (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}
}
