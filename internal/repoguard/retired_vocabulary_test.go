package repoguard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/scanner"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/cli"
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
// Every row retires its spelling everywhere in the scanned surface. Change 0471
// carried a kept-namesake exception for row 12 (the gate drive's own
// --gate-context); change 0477 renamed that flag to --run-context (ADR-0129 row
// 38e) and deleted the exception, since a mechanism with no users cannot be
// mutation-tested (Decision 10). A later family that needs one brings it back
// with its own mutation tests.
//
// Family (b), revision (change 0472), appends rows 40, 40a, 41-44 and 44a.
// It brings the first two kinds whose match is not a bare spelling:
// kindBoundFlag depends on CONTEXT (a --version is retired only where it is
// bound to a change, finalize or workspace command, derived from the live
// capability catalog), and kindSchemaKey depends on the WIRE CONTRACT (a
// request/result key found by walking the schema registry, with an exact kept
// set of software and format versions).
//
// Family (d), re-enable / final / shared-setting guard (change 0474), appends
// rows 54-59 and 59a, all kindToken: the change.groom outcome rearm and its
// nothing-to-rearm refusal, the config guard's fenced-setting-ignored warning,
// the seven lifecycle codes spelled with "terminal", and the renamed close-out
// reference file (row 59a, the first path row). rearm also matches inside
// nothing-to-rearm; the non-vacuity check counts only the planted row's own hits.
//
// Family (e), readability renames (change 0469), appends rows 67-73, the wire
// rows: the readiness needs-brainstorm, the gate-drive halt causes
// identity-mismatch and unresolved-execution, the change.repair-identity
// operation and its repaired-branch / repaired-pr results, finalize's
// pr-identity-mismatch and recertify's identity-drift. Rows 74-84 are prose and
// rows 85-86 retire names with no Go sites, which the seal does not scan (the
// results file records the closing grep). Row 68 is the first kindWord row:
// scope-identity-mismatch (kept) and pr-identity-mismatch (row 72) end in its
// spelling. LIMITATION: a composite such as halted-identity-mismatch is not a
// kindWord hit; the cause literal it would be composed from is sealed.
//
// Change 0491 retires the run id (rows 3, 10, 11, 20, 31, 32, 38b, 38d): it
// re-points the rows whose replacements were run-id spellings and adds
// kindSchemaPath, the first exact-path kind, because the run.start result's
// run_id shares its key name with the gate supervisor's run_id.
//
// LIMITATION (byte-pattern-guard-matches-a-spelling): a foreign tool's
// --version whose nearest preceding operation reference in the same block is a
// change, finalize or workspace operation reads as bound. No maintained surface
// carries that shape; the negative controls pin the shapes that exist.
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
	// kindBoundFlag: the bare flag Old (row 40's --version), retired where it is
	// BOUND to a change, finalize or workspace command (see scanBoundFlags for
	// markdown/shell and scanGoBoundFlag for Go) and, in markdown, everywhere a
	// foreign software-version tool does not bind it. A software version stays:
	// docket version, the release tools, foreign CLIs.
	kindBoundFlag
	// kindSchemaKey: a request/result key whose name ends in Old, found by
	// walking the schema registry (the wire contract), never by grepping tags
	// (see schemaVersionKeyHits). The exact kept set is schemaKeptVersionKeys.
	kindSchemaKey
	// kindWord: like kindToken, but the LEADING boundary also excludes '-', so a
	// hyphenated compound that merely ends in Old is a different word. Family (e)
	// row 68 needs it: identity-mismatch is retired, while the kept
	// scope-identity-mismatch and row 72's own pr-identity-mismatch are other
	// words (ADR-0129 "Kept in family (e)").
	kindWord
	// kindSchemaPath: an exact "<operation> <REQ|RES> <key path>" that must be
	// absent from the live schema registry (change 0491, row 38b: the run.start
	// result's run_id). It is exact, never a suffix, so the gate supervisor's own
	// run_id keys — gate.launch, gate.observe, gate.recover, and gate.stop results —
	// can never match.
	kindSchemaPath
)

// retiredToken is one row of the retired-vocabulary table.
type retiredToken struct {
	Row  string      // ADR-0129 rename-table row, e.g. "7", "38c"
	Kind retiredKind // which sites match
	Old  string      // the retired spelling
	New  string      // its replacement, named in every failure
}

// retiredVocabulary is the table. Append-only across ADR-0129 families.
var retiredVocabulary = []retiredToken{
	// Family (a) — run tracker (change 0471): rows 7-37, 38 (storage names), 38a-38d;
	// change 0477 finished rows 38e-38h.
	{Row: "7", Kind: kindToken, Old: "gate-before", New: "run.start / docket run start"},
	{Row: "8", Kind: kindToken, Old: "gate-verdict", New: "run.verdict / docket run verdict"},
	{Row: "9", Kind: kindToken, Old: "gate-claim", New: "run.continue / docket run continue"},
	{Row: "10", Kind: kindToken, Old: "--run-epoch", New: "no flag — the run key is the run tracker's only handle (change 0491)"},
	{Row: "10", Kind: kindGoFlag, Old: "run-epoch", New: "run-key"},
	{Row: "11", Kind: kindToken, Old: "--epoch", New: "run cancel --key"},
	{Row: "11", Kind: kindGoFlag, Old: "epoch", New: "key"},
	{Row: "12, 38e", Kind: kindToken, Old: "--gate-context", New: "--run-context"},
	{Row: "12, 38e", Kind: kindGoFlag, Old: "gate-context", New: "run-context"},
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
	{Row: "20", Kind: kindToken, Old: "epoch-mismatch", New: "run-record-conflict"},
	{Row: "21", Kind: kindToken, Old: "epoch-not-cancelled", New: "run-not-cancelled"},
	{Row: "22", Kind: kindToken, Old: "epoch-ambiguous", New: "run-ambiguous"},
	{Row: "23", Kind: kindToken, Old: "epoch-owner-ambiguous", New: "run-owner-ambiguous"},
	// Row 24's replacement was itself retired by change 0490 (the slot reader that
	// raised it was deleted); the row stays to seal the epoch-era spelling and to
	// mirror ADR-0129's table.
	{Row: "24", Kind: kindToken, Old: "epoch-owner-unresolved", New: "run-owner-unresolved"},
	{Row: "25", Kind: kindToken, Old: "epoch-io", New: "run-record-io"},
	{Row: "26", Kind: kindToken, Old: "epoch-participant-unknown", New: "run-participant-unknown"},
	{Row: "27", Kind: kindToken, Old: "epoch-state-unknown", New: "run-state-unknown"},
	{Row: "28", Kind: kindToken, Old: "epoch-unreadable", New: "run-record-unreadable"},
	{Row: "29", Kind: kindToken, Old: "incumbent-epoch-fenced", New: "incumbent-run-fenced"},
	{Row: "30", Kind: kindToken, Old: "resume-epoch-unreadable", New: "resume-run-record-unreadable"},
	{Row: "31", Kind: kindToken, Old: "stale-run-epoch", New: "run-superseded"},
	{Row: "32", Kind: kindToken, Old: "unknown-run-epoch", New: "run-not-found"},
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
	{Row: "38b", Kind: kindJSONKey, Old: "epoch", New: "none — run.start reports key and run_context (change 0491)"},
	{Row: "38c", Kind: kindToken, Old: "gate_context", New: "run_context"},
	{Row: "38d", Kind: kindToken, Old: "DOCKET_AGENT_GUARDIAN_EPOCH", New: "DOCKET_AGENT_GUARDIAN_RUN_KEY"},
	// Change 0477 — rows 38e-38h: 38e shares row 12's entries above (the gate
	// drive's flag now takes the same --run-context as change claim, so no kept
	// namesake remains); 38h (the `rungate store` error-text prefix) is already
	// sealed by row 38's rungate token and needs no row of its own.
	{Row: "38f", Kind: kindToken, Old: "DOCKET_AGENT_GUARDIAN_GATE_KEY", New: "DOCKET_AGENT_GUARDIAN_RUN_KEY"},
	{Row: "38g", Kind: kindJSONKey, Old: "dispatch_context", New: "run_context"},
	// 38g's token entry seals the key's consumers (a `jq -r .dispatch_context`
	// read, a Go map-key literal), which the struct-tag entry above cannot see.
	// No maintained namesake carries the spelling (change 0477 review Finding 3).
	{Row: "38g", Kind: kindToken, Old: "dispatch_context", New: "run_context"},
	// Family (b) — revision (change 0472): rows 40, 40a, 41-44, 44a. Row 39 is the
	// concept itself; row 45 (resolver_budget_version) is kept.
	{Row: "40", Kind: kindBoundFlag, Old: "--version", New: "--revision"},
	{Row: "40a", Kind: kindToken, Old: "expect-version", New: "--expect-revision / expect-revision"},
	{Row: "41-44", Kind: kindSchemaKey, Old: "version", New: "revision"},
	{Row: "42", Kind: kindToken, Old: "spec_version", New: "spec_revision"},
	{Row: "44", Kind: kindToken, Old: "pr_version", New: "pr_revision"},
	{Row: "44a", Kind: kindToken, Old: "version-mismatch", New: "revision-mismatch"},
	{Row: "44a", Kind: kindToken, Old: "version-drift", New: "revision-drift"},
	{Row: "44a", Kind: kindToken, Old: "spec-version-mismatch", New: "spec-revision-mismatch"},
	{Row: "44a", Kind: kindToken, Old: "reclaim-version-missing", New: "reclaim-revision-missing"},
	{Row: "44a", Kind: kindToken, Old: "empty-version", New: "empty-revision"},
	{Row: "44a", Kind: kindToken, Old: "empty-change-version", New: "empty-change-revision"},
	{Row: "44a", Kind: kindToken, Old: "empty-target-version", New: "empty-target-revision"},
	{Row: "44a", Kind: kindToken, Old: "empty-spec_version", New: "empty-spec_revision"},
	{Row: "44a", Kind: kindToken, Old: "invalid-spec_version", New: "invalid-spec_revision"},
	{Row: "44a", Kind: kindToken, Old: "empty-child_pr_version", New: "empty-child_pr_revision"},
	// Family (d) — re-enable, final, shared-setting guard (change 0474): rows
	// 54-59 and 59a. Row 53 is the concept; rows 59b and 59c are Go names and
	// prose, which the seal does not scan (a guard keyed on passing mentions
	// would violate the guard rule; the results file records the closing grep).
	{Row: "54", Kind: kindToken, Old: "rearm", New: "re-enable"},
	{Row: "55", Kind: kindToken, Old: "nothing-to-rearm", New: "nothing-to-re-enable"},
	{Row: "56", Kind: kindToken, Old: "fenced-setting-ignored", New: "shared-setting-ignored"},
	{Row: "57", Kind: kindToken, Old: "terminal-backlink-pending", New: "final-backlink-pending"},
	{Row: "57", Kind: kindToken, Old: "terminal-notes-frozen", New: "final-notes-frozen"},
	{Row: "58", Kind: kindToken, Old: "change-terminal-claim-stamp", New: "change-final-claim-stamp"},
	{Row: "58", Kind: kindToken, Old: "drop-terminal-claimed-at", New: "drop-final-claimed-at"},
	{Row: "59", Kind: kindToken, Old: "not-terminal", New: "not-final"},
	{Row: "59", Kind: kindToken, Old: "skipped-terminal", New: "skipped-final"},
	{Row: "59", Kind: kindToken, Old: "adr-update-after-terminal", New: "adr-update-after-final"},
	{Row: "59a", Kind: kindToken, Old: "terminal-close-out.md", New: "close-out.md (skills/docket-convention/references/)"},
	// Family (e) — readability renames (change 0469): rows 67-73, the wire rows.
	// Rows 74-86 are prose or retire names with no Go sites.
	{Row: "67", Kind: kindToken, Old: "needs-brainstorm", New: "needs-grooming"},
	{Row: "68", Kind: kindWord, Old: "identity-mismatch", New: "worktree-changed"},
	{Row: "69", Kind: kindToken, Old: "unresolved-execution", New: "launch-unconfirmed"},
	{Row: "70", Kind: kindToken, Old: "repair-identity", New: "change.relink / docket change relink"},
	{Row: "71", Kind: kindToken, Old: "repaired-branch", New: "relinked-branch"},
	{Row: "71", Kind: kindToken, Old: "repaired-pr", New: "relinked-pr"},
	{Row: "72", Kind: kindToken, Old: "pr-identity-mismatch", New: "pr-link-mismatch"},
	{Row: "73", Kind: kindToken, Old: "identity-drift", New: "certified-input-changed"},
	// Change 0491 — the run id is retired; the run key is the run tracker's only
	// handle (ADR-0129 rows 3, 10, 11, 20, 31, 32, 38b, 38d amended in place). No row
	// may match the gate supervisor's own run id (internal/process, gate.* results'
	// run_id, incumbent-run:<id>): the negative controls pin that.
	{Row: "10, 11", Kind: kindToken, Old: "--run-id", New: "no flag — the run key is the handle (run cancel --key)"},
	{Row: "10, 11", Kind: kindGoFlag, Old: "run-id", New: "run-key / key"},
	{Row: "3", Kind: kindToken, Old: "<run-id>", New: "<key> (the run key)"},
	{Row: "20", Kind: kindToken, Old: "run-id-mismatch", New: "run-record-conflict"},
	{Row: "31", Kind: kindToken, Old: "stale-run-id", New: "run-superseded"},
	{Row: "32", Kind: kindToken, Old: "unknown-run-id", New: "run-not-found"},
	{Row: "38b", Kind: kindSchemaPath, Old: "run.start RES run_id", New: "none — run.start reports key and run_context"},
	{Row: "38d", Kind: kindToken, Old: "DOCKET_AGENT_GUARDIAN_RUN_ID", New: "DOCKET_AGENT_GUARDIAN_RUN_KEY (the run id env is retired)"},
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

var wordReCache = map[string]*regexp.Regexp{}

// wordRe is the bounded matcher for a kindWord row (see kindWord).
func wordRe(old string) *regexp.Regexp {
	if re, ok := wordReCache[old]; ok {
		return re
	}
	re := regexp.MustCompile(`(^|[^A-Za-z0-9_-])` + regexp.QuoteMeta(old) + `([^A-Za-z0-9_-]|$)`)
	wordReCache[old] = re
	return re
}

// textMatcher returns the line matcher for a row scanned as text (kindToken,
// kindWord), or nil for a kind scanned another way.
func textMatcher(r retiredToken) *regexp.Regexp {
	switch r.Kind {
	case kindToken:
		return tokenRe(r.Old)
	case kindWord:
		return wordRe(r.Old)
	}
	return nil
}

// scanTextLine checks one markdown/shell/generated line against every
// kindToken and kindWord row.
func scanTextLine(rel string, lineNo int, line string) []retiredHit {
	var hits []retiredHit
	for _, r := range retiredVocabulary {
		// Pre-filter (change 0487): every text matcher embeds
		// regexp.QuoteMeta(r.Old) literally, so a line without r.Old cannot
		// match. Skipping the regexp is detection-neutral and avoids a
		// byte-by-byte regexp walk per row per line; testRetiredPrefilterEquivalence
		// pins the equivalence.
		if !strings.Contains(line, r.Old) {
			continue
		}
		re := textMatcher(r)
		if re == nil {
			continue
		}
		if re.MatchString(line) {
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

// scanText scans content line by line. Shell and config lines (md false)
// have their `#` comment stripped first.
func scanText(rel, content string, md bool) []retiredHit {
	lines := strings.Split(content, "\n")
	var hits []retiredHit
	for i, line := range lines {
		if !md {
			line = stripHashComment(line)
		}
		lines[i] = line
		hits = append(hits, scanTextLine(rel, i+1, line)...)
	}
	return append(hits, scanBoundFlags(rel, lines, md)...)
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
			// Pre-filter (change 0487), on the UNQUOTED value: see scanTextLine.
			hit = strings.Contains(val, r.Old) && tokenRe(r.Old).MatchString(val)
		case kindWord:
			hit = strings.Contains(val, r.Old) && wordRe(r.Old).MatchString(val)
		case kindGoFlag:
			hit = val == r.Old
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
	// groups is the open-bracket stack: each entry is the method name when the
	// '(' opened a method call (x.M(…)), else "", plus its top-level argument
	// index, so a literal that is a whole argument of a method call is
	// recognisable by position. prev1/prev2 (prevLit) are the tokens before.
	type group struct {
		method string
		arg    int
	}
	var groups []group
	var prev1, prev2 token.Token
	var prevLit string
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		switch tok {
		case token.STRING:
			hits = append(hits, scanGoLiteral(rel, fset.Position(pos).Line, lit)...)
			if n := len(groups); n > 0 && groups[n-1].method != "" && strings.HasPrefix(rel, "internal/cli/") &&
				(prev1 == token.LPAREN || prev1 == token.COMMA) && isFlagNameArg(groups[n-1].method, groups[n-1].arg) {
				hits = append(hits, scanGoBoundFlag(rel, fset.Position(pos).Line, lit)...)
			}
		case token.LPAREN:
			m := ""
			if prev1 == token.IDENT && prev2 == token.PERIOD {
				m = prevLit
			}
			groups = append(groups, group{method: m})
		case token.LBRACE, token.LBRACK:
			groups = append(groups, group{})
		case token.RPAREN, token.RBRACE, token.RBRACK:
			if len(groups) > 0 {
				groups = groups[:len(groups)-1]
			}
		case token.COMMA:
			if len(groups) > 0 {
				groups[len(groups)-1].arg++
			}
		}
		prev2, prev1, prevLit = prev1, tok, lit
	}
	return hits, scanErr
}

// isFlagNameArg reports whether argument arg of method names a flag: the FIRST
// argument of any method (Flags().String("version", …), GetString,
// MarkFlagRequired), or the SECOND of a pflag *Var / *VarP definition, whose
// first argument is the destination pointer (StringVar(&v, "version", …)).
func isFlagNameArg(method string, arg int) bool {
	return arg == 0 || (arg == 1 && (strings.HasSuffix(method, "Var") || strings.HasSuffix(method, "VarP")))
}

// boundFlagFamilies are the command families whose bare --version pinned the
// record revision (ADR-0129 row 40).
var boundFlagFamilies = map[string]bool{"change": true, "finalize": true, "workspace": true}

var (
	opRefOnce sync.Once
	opRefRe   *regexp.Regexp
	opRefIDs  int
	opRefErr  error
)

// catalogOpRefs compiles the operation-reference matcher from the live
// capability catalog, read in-process through cli.Run, never hand-listed
// (enumerated-floor): every dotted op id (change.claim) and every multi-word
// command form (change claim, gate drive start). Single-word commands (status,
// version, schema) are deliberately not references: they name no family a
// --version binds to, and as English words they would shadow a real binding.
func catalogOpRefs() (*regexp.Regexp, int, error) {
	opRefOnce.Do(func() {
		var out, errb bytes.Buffer
		if code := cli.Run([]string{"capabilities", "--json"}, strings.NewReader(""), &out, &errb, buildinfo.Info{}, buildinfo.RuntimeFacts{}); code != 0 {
			opRefErr = fmt.Errorf("capabilities exited %d: %s", code, errb.String())
			return
		}
		var doc struct {
			Commands []struct {
				ID   string   `json:"id"`
				Argv []string `json:"argv"`
			} `json:"commands"`
		}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			opRefErr = fmt.Errorf("parse capabilities: %w", err)
			return
		}
		var alts []string
		for _, c := range doc.Commands {
			if strings.Contains(c.ID, ".") {
				alts = append(alts, regexp.QuoteMeta(c.ID))
				opRefIDs++
			}
			if len(c.Argv) >= 3 {
				words := make([]string, 0, len(c.Argv)-1)
				for _, w := range c.Argv[1:] {
					words = append(words, regexp.QuoteMeta(w))
				}
				alts = append(alts, strings.Join(words, `[ \t]+`))
			}
		}
		if len(alts) == 0 {
			opRefErr = fmt.Errorf("capabilities listed no operation references")
			return
		}
		// Longest first: Go's alternation is leftmost-first, so change.refresh-claim
		// must be tried before any shorter reference it contains.
		sort.Slice(alts, func(i, j int) bool { return len(alts[i]) > len(alts[j]) })
		opRefRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.-])(` + strings.Join(alts, "|") + `)(?:[^A-Za-z0-9_-]|$)`)
	})
	return opRefRe, opRefIDs, opRefErr
}

// refFamily returns an operation reference's family: its text up to the first
// '.' or blank.
func refFamily(ref string) string {
	if i := strings.IndexAny(ref, ". \t"); i >= 0 {
		return ref[:i]
	}
	return ref
}

// foreignVersionRe matches a foreign software-version context: a tool whose own
// --version names a software version, never a docket record revision — the
// release tools (releasepkg, release-smoke, install.sh), `docket version`, and
// the harness CLIs. It is an allowlist that fails CLOSED: a tool missing from
// it reads as a retired --version (a visible false positive), never a silent
// pass.
var foreignVersionRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.-])(releasepkg|release-smoke|install\.sh|docket[ \t]+version|codex|opencode|cursor-agent|claude)(?:[^A-Za-z0-9_-]|$)`)

// blockMayBindFlag reports whether block contains some kindBoundFlag row's Old
// literally. A block without one cannot produce a bound-flag hit (each row is
// matched by tokenRe, which embeds regexp.QuoteMeta(r.Old)), so scanBoundFlags
// skips it before running the catalog operation-reference alternation over it
// (change 0487).
func blockMayBindFlag(block string) bool {
	for _, r := range retiredVocabulary {
		if r.Kind == kindBoundFlag && strings.Contains(block, r.Old) {
			return true
		}
	}
	return false
}

// scanBoundFlags reports each kindBoundFlag row's retired flag. A block is a
// maximal run of non-blank lines, so a flag wrapped onto the line after its
// operation still binds, and a blank line ends the binding. The flag's binding
// is the NEAREST anchor that precedes it: a catalog operation reference in its
// block, or a foreign software-version context (foreignVersionRe) on its own
// line. A markdown document whose title (its first `# ` heading) names a
// foreign tool documents that tool (scripts/release-smoke.md), so the title
// binds every --version in it.
//
//   - Shell/config (md false): retired only when bound to a change, finalize or
//     workspace operation, so the release tools' --version and a foreign CLI's
//     (`codex --version`) stay.
//   - Markdown (md true: skills, agents, cursor rules, always-loaded files,
//     scripts docs, generator output): retired unless a foreign context is the binding. No
//     markdown surface carries a docket software --version, so an UNBOUND
//     mention (`Scalar identities (--id, --version, …)`) or one bound to another
//     family is a record revision and is retired too.
//
// LIMITATION (byte-pattern-guard-matches-a-spelling): in shell, a foreign
// tool's --version whose nearest preceding reference in the same block is a
// change, finalize or workspace operation reads as bound. No maintained surface
// carries that shape; the negative controls pin the shapes that exist.
func scanBoundFlags(rel string, lines []string, md bool) []retiredHit {
	re, _, err := catalogOpRefs()
	if err != nil {
		panic(fmt.Sprintf("retired-vocabulary seal: catalog operation references unavailable (fail closed): %v", err))
	}
	docForeign := false
	if md {
		for _, l := range lines {
			if strings.HasPrefix(l, "# ") {
				docForeign = foreignVersionRe.MatchString(l)
				break
			}
		}
	}
	var hits []retiredHit
	for start := 0; start < len(lines); {
		if strings.TrimSpace(lines[start]) == "" {
			start++
			continue
		}
		end := start
		for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
			end++
		}
		block := strings.Join(lines[start:end], "\n")
		if !blockMayBindFlag(block) {
			start = end
			continue
		}
		refs := re.FindAllStringSubmatchIndex(block, -1)
		for _, r := range retiredVocabulary {
			if r.Kind != kindBoundFlag {
				continue
			}
			for _, m := range tokenRe(r.Old).FindAllStringIndex(block, -1) {
				pos := m[0] + strings.Index(block[m[0]:m[1]], r.Old)
				family, refStart := "", -1
				for _, ref := range refs {
					if ref[2] >= pos {
						break
					}
					family, refStart = refFamily(block[ref[2]:ref[3]]), ref[2]
				}
				retired := boundFlagFamilies[family]
				if md && !retired && !docForeign {
					lineStart := strings.LastIndex(block[:pos], "\n") + 1
					foreignStart := -1
					for _, f := range foreignVersionRe.FindAllStringSubmatchIndex(block[lineStart:pos], -1) {
						foreignStart = lineStart + f[2]
					}
					retired = foreignStart < 0 || foreignStart < refStart
				}
				if retired {
					lineNo := start + 1 + strings.Count(block[:pos], "\n")
					hits = append(hits, retiredHit{rel, lineNo, r, strings.TrimSpace(lines[lineNo-1])})
				}
			}
		}
		start = end
	}
	return hits
}

// scanGoBoundFlag: a kindBoundFlag row's flag name (Old without its leading
// "--") passed as a method call's flag-name argument (isFlagNameArg) in
// non-test internal/cli source, which is a flag definition or lookup
// (Flags().String("version", …), StringVar(&v, "version", …),
// GetString("version"), MarkFlagRequired("version")). internal/cli builds every
// change, finalize and workspace command, and none of its commands takes a
// software --version (docket version is a subcommand with no such flag); the
// release tools' --version lives outside it (cmd/releasepkg, the shell
// downloader). A plain function call (capability("version", …)) and a struct
// field (Use: "version") are not method calls and never bind.
func scanGoBoundFlag(rel string, lineNo int, lit string) []retiredHit {
	val, err := strconv.Unquote(lit)
	if err != nil {
		return nil
	}
	var hits []retiredHit
	for _, r := range retiredVocabulary {
		if r.Kind == kindBoundFlag && val == strings.TrimPrefix(r.Old, "--") {
			hits = append(hits, retiredHit{rel, lineNo, r, lit})
		}
	}
	return hits
}

// schemaKeptVersionKeys is the bounded kept set of spec Decision 4 as it appears
// in the schema: software and format versions, the version operation's own key,
// and capabilities' binary.version. Each entry is an exact "<scope> <path>", never
// a bare name, so a record revision cannot hide behind a kept spelling elsewhere.
// The claim digest payload is not a registered shape and is outside the walk.
var schemaKeptVersionKeys = map[string]bool{
	"envelope protocol_version":           true,
	"capabilities RES capability_version": true,
	"capabilities RES binary.version":     true,
	"diagnostic.runtime RES go_version":   true,
	"version RES version":                 true,
}

// schemaVersionKeyHits walks every key of doc (the envelope, then each
// operation's request and result, recursively) and reports each key whose name
// ends, case-insensitively, in a kindSchemaKey row's Old and is not in
// schemaKeptVersionKeys, naming the replacement path. revisionKeys counts the keys
// ending in "revision", the live population floor a truncated walk would miss.
func schemaVersionKeyHits(doc app.SchemaResult) (hits []string, revisionKeys int, seen map[string]bool) {
	seen = map[string]bool{}
	var walk func(scope, prefix string, fs []app.FieldDescriptor)
	walk = func(scope, prefix string, fs []app.FieldDescriptor) {
		for _, f := range fs {
			path := prefix + f.Key
			key := scope + " " + path
			seen[key] = true
			lower := strings.ToLower(f.Key)
			if strings.HasSuffix(lower, "revision") {
				revisionKeys++
			}
			for _, r := range retiredVocabulary {
				if r.Kind != kindSchemaKey || !strings.HasSuffix(lower, r.Old) || schemaKeptVersionKeys[key] {
					continue
				}
				n := len(f.Key) - len(r.Old)
				repl := f.Key[:n] + r.New
				if f.Key[n] == 'V' {
					repl = f.Key[:n] + strings.ToUpper(r.New[:1]) + r.New[1:]
				}
				hits = append(hits, fmt.Sprintf("schema %s: ADR-0129 rows %s: retired %q — use %s", key, r.Row, f.Key, prefix+repl))
			}
			walk(scope, path+".", f.Fields)
		}
	}
	walk("envelope", "", doc.EnvelopeShape.Fields)
	for _, op := range doc.Operations {
		if op.Request != nil {
			walk(op.ID+" REQ", "", op.Request.Fields)
		}
		walk(op.ID+" RES", "", op.Result.Fields)
	}
	sort.Strings(hits)
	return hits, revisionKeys, seen
}

// schemaPathHits reports every kindSchemaPath row whose exact path the walked
// schema carries (seen, from schemaVersionKeyHits), naming the replacement.
func schemaPathHits(seen map[string]bool) []string {
	var hits []string
	for _, r := range retiredVocabulary {
		if r.Kind == kindSchemaPath && seen[r.Old] {
			hits = append(hits, fmt.Sprintf("schema %s: ADR-0129 rows %s: retired — use %s", r.Old, r.Row, r.New))
		}
	}
	sort.Strings(hits)
	return hits
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
	t.Run("catalog_vocabulary", testRetiredCatalogVocabulary)
	t.Run("table_integrity", testRetiredTableIntegrity)
	t.Run("non_vacuity", testRetiredNonVacuity)
	t.Run("negative_controls", testRetiredNegativeControls)
	t.Run("maintained_surfaces", testRetiredMaintainedSurfaces)
	t.Run("generator_output", testRetiredGeneratorOutput)
	t.Run("schema_walk", testRetiredSchemaWalk)
	t.Run("prefilter_equivalence", testRetiredPrefilterEquivalence)
}

// testRetiredPrefilterEquivalence pins the scan pre-filters (change 0487) to
// the matchers they guard. For every text row, on lines and Go literals that
// do and do not carry the row's spelling, the filtered scan reports a hit
// attributed to that row exactly when the row's own regexp matches. A future
// row whose matcher stops containing its Old literally (a case-insensitive
// kind, say) reddens here instead of silently scanning nothing. Go literals
// are also planted with every '-' escaped as \x2d, so the literal filter must
// run on the unquoted value. A bound flag wrapped onto the line after its
// operation reference must still bind, so the block filter must read the
// whole block.
func testRetiredPrefilterEquivalence(t *testing.T) {
	ownHits := func(hits []retiredHit, r retiredToken) int {
		n := 0
		for _, h := range hits {
			if h.row.Kind == r.Kind && h.row.Old == r.Old {
				n++
			}
		}
		return n
	}
	checked := 0
	for _, r := range retiredVocabulary {
		re := textMatcher(r)
		if re == nil {
			continue
		}
		upper := strings.ToUpper(r.Old)
		lines := []string{
			r.Old,
			"run " + r.Old + " now",
			"x" + r.Old,
			r.Old + "-x",
			"(" + r.Old + ")",
			upper,
			"run " + upper + " now",
		}
		for _, line := range lines {
			want := 0
			if re.MatchString(line) {
				want = 1
			}
			if got := ownHits(scanTextLine("skills/x/SKILL.md", 1, line), r); got != want {
				t.Errorf("row %s: text line %q: filtered scan reports %d hit(s), its matcher says %d", r.Row, line, got, want)
			}
			lit := strconv.Quote(line)
			for _, l := range []string{lit, strings.ReplaceAll(lit, "-", `\x2d`)} {
				val, err := strconv.Unquote(l)
				if err != nil {
					t.Fatalf("row %s: planted literal %s does not unquote: %v", r.Row, l, err)
				}
				want := 0
				if re.MatchString(val) {
					want = 1
				}
				if got := ownHits(scanGoLiteral("internal/p/p.go", 1, l), r); got != want {
					t.Errorf("row %s: Go literal %s: filtered scan reports %d hit(s), its matcher says %d", r.Row, l, got, want)
				}
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatalf("population floor: no text row was checked")
	}
	bound := 0
	for _, r := range retiredVocabulary {
		if r.Kind != kindBoundFlag {
			continue
		}
		bound++
		block := "the `change.claim` operation\nwith `--id <id> " + r.Old + " <v>`"
		if got := ownHits(scanBoundFlags("skills/x/SKILL.md", strings.Split(block, "\n"), true), r); got != 1 {
			t.Errorf("row %s: %q wrapped onto the line after its operation reference: %d hit(s), want 1", r.Row, r.Old, got)
		}
	}
	if bound == 0 {
		t.Fatalf("population floor: no kindBoundFlag row was checked")
	}
}

// testRetiredCatalogVocabulary derives the operation references the bound-flag
// scan needs and fails closed if the catalog cannot be read or has collapsed.
func testRetiredCatalogVocabulary(t *testing.T) {
	re, ids, err := catalogOpRefs()
	if err != nil {
		t.Fatalf("catalog operation references: %v (fail closed)", err)
	}
	if ids < 70 {
		t.Fatalf("population floor: %d dotted operation ids (expected >= 70)", ids)
	}
	for _, fam := range []string{"change.claim", "finalize.merge", "workspace.prepare", "docket change claim"} {
		if !re.MatchString(" " + fam + " ") {
			t.Errorf("operation reference matcher does not recognise %q", fam)
		}
	}
}

// testRetiredSchemaWalk seals the bare record-revision key over the live wire
// contract.
func testRetiredSchemaWalk(t *testing.T) {
	doc, err := app.Schema(nil)
	if err != nil {
		t.Fatalf("app.Schema: %v (fail closed)", err)
	}
	if len(doc.Operations) < 75 {
		t.Fatalf("population floor: schema walk saw %d operations (expected >= 75)", len(doc.Operations))
	}
	hits, revs, seen := schemaVersionKeyHits(doc)
	if revs < 60 {
		t.Fatalf("population floor: schema walk saw %d *revision keys (expected >= 60)", revs)
	}
	// A kept entry the live schema no longer carries is a stale exemption.
	for k := range schemaKeptVersionKeys {
		if !seen[k] {
			t.Errorf("kept schema key %q is absent from the live schema: delete it from schemaKeptVersionKeys", k)
		}
	}
	if ph := schemaPathHits(seen); len(ph) != 0 {
		t.Errorf("retired ADR-0129 schema paths in the live schema (%d):\n%s", len(ph), strings.Join(ph, "\n"))
	}
	// Non-vacuity of the path spelling: the scoped row must name a path shape the
	// walk produces. run.start's kept run_context key proves the scope spelling.
	if !seen["run.start RES run_context"] {
		t.Errorf("the schema walk no longer yields %q; the run.start RES scope spelling drifted", "run.start RES run_context")
	}
	if len(hits) != 0 {
		t.Errorf("retired ADR-0129 record-revision keys in the schema (%d):\n%s", len(hits), strings.Join(hits, "\n"))
	}
}

// testRetiredTableIntegrity: a malformed row would seal nothing or name no
// replacement. The floor stops a truncated table from passing vacuously.
func testRetiredTableIntegrity(t *testing.T) {
	const floor = 105
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
		case kindToken, kindWord:
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
		case kindBoundFlag:
			line := "the `change.claim` operation with `--id <id> " + r.Old + " <v>`"
			hits = append(hits, scanTextContent("skills/x/SKILL.md", line)...)
			hits = append(hits, scanTextContent("tests/test_x.sh", "docket change claim --id 1 "+r.Old+" v")...)
			hits = append(hits, goHits("internal/cli/change.go", "package p\nfunc f(c *C) { c.Flags().String("+strconv.Quote(strings.TrimPrefix(r.Old, "--"))+", \"\", \"x\") }\n")...)
			want = 3
		case kindSchemaKey:
			doc := app.SchemaResult{Operations: []app.OperationSchema{{ID: "change.x", Request: &app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: r.Old}}}}}}
			sh, _, _ := schemaVersionKeyHits(doc)
			if len(sh) != 1 || !strings.Contains(sh[0], r.New) {
				t.Errorf("row %s: a planted %q key was not detected naming %q: %v", r.Row, r.Old, r.New, sh)
			}
			continue
		case kindSchemaPath:
			op, rest, _ := strings.Cut(r.Old, " ")
			_, key, _ := strings.Cut(rest, " ")
			doc := app.SchemaResult{Operations: []app.OperationSchema{{ID: op, Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: key}}}}}}
			_, _, seen := schemaVersionKeyHits(doc)
			if ph := schemaPathHits(seen); len(ph) != 1 || !strings.Contains(ph[0], r.New) {
				t.Errorf("row %s: a planted %q was not detected naming %q: %v", r.Row, r.Old, r.New, ph)
			}
			continue
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
	// Rows 12 and 38e (change 0477): the gate drive's flag is now --run-context,
	// so every --gate-context is retired, including the spellings change 0471 kept
	// as the gate drive's namesake. Each line below was a CLEAN negative control
	// before 0477; each must now hit the plain row.
	for _, line := range []string{
		"docket gate drive prepare-scope --gate-context <ctx>",
		"docket gate drive start --gate-context <ctx>",
		"include the run-context token, labeled for `change.claim --run-context` and gate-drive `--gate-context`",
		"plus gate-drive `--gate-context <token>` if dispatched with one",
		"| `prepare-scope` | `gate.drive.prepare-scope --change-id <id> [--gate-context <token>]`: mint",
		"pass it into `gate.drive.start` and into the Step-2 claim's `--gate-context`",
	} {
		for _, rel := range []string{"skills/x/SKILL.md", "tests/test_x.sh"} {
			if !hasRetiredRow(scanTextContent(rel, line), "12, 38e") {
				t.Errorf("rows 12/38e: a --gate-context in %s was not detected: %q", rel, line)
			}
		}
	}
	for _, c := range []struct{ rel, src string }{
		{"internal/cli/gate.go", "package p\nvar f = \"gate-context\"\n"},
		{"internal/app/x.go", "package p\nvar m = \"(the <run-context> goes to the gate drive's --gate-context)\"\n"},
	} {
		if !hasRetiredRow(goHits(c.rel, c.src), "12, 38e") {
			t.Errorf("rows 12/38e: the former kept Go spelling in %s was not detected: %q", c.rel, c.src)
		}
	}
	// Row 38g (change 0477 review Finding 3): the retired run.start JSON key is
	// sealed at its CONSUMERS too, not only at the producer's struct tag — a
	// skill/shell line reading it and a Go map-key literal must both hit.
	for _, line := range []string{
		"ctx=$(docket run start implement-next --json | jq -r .dispatch_context)",
		"read `.dispatch_context` from the run.start JSON result",
	} {
		for _, rel := range []string{"skills/x/SKILL.md", "tests/test_x.sh"} {
			if !hasRetiredRow(scanTextContent(rel, line), "38g") {
				t.Errorf("row 38g: a dispatch_context read in %s was not detected: %q", rel, line)
			}
		}
	}
	if !hasRetiredRow(goHits("internal/app/x.go", "package p\nvar c = m[\"dispatch_context\"]\n"), "38g") {
		t.Errorf("row 38g: a dispatch_context Go map-key literal was not detected")
	}
	// Row 40 (change 0472): a --version bound to a change, finalize or workspace
	// command, in each binding shape the tree carried.
	for _, c := range []struct{ rel, text string }{
		{"skills/x/SKILL.md", "docket finalize merge --id 1 --version v --head h"},
		{"skills/x/SKILL.md", "workspace.prepare  --id <id> --version <v>   # resolve argv from the capability catalog"},
		// The flag wrapped onto the line after its operation still binds (edge-paths shape).
		{"skills/x/SKILL.md", "resume through the `change.resume-halted` operation with `--id <id>\n--version <v> --acknowledge-quiescent`"},
		{"tests/test_x.sh", "docket change relink --id 1 --version v   # a bare --version on relink is bound too"},
	} {
		if !hasRetiredRow(scanTextContent(c.rel, c.text), "40") {
			t.Errorf("row 40: a bound --version in %s was not detected: %q", c.rel, c.text)
		}
	}
	// Row 40 (change 0472 review finding): in maintained markdown every --version
	// is retired unless a foreign software-version tool binds it on its own line —
	// an UNBOUND mention (no catalog operation before it in its block) and one
	// whose nearest operation is another family both hit. This is the
	// docket-finalize-change paragraph as it read before the rename.
	for _, c := range []struct{ rel, text string }{
		{"skills/x/SKILL.md", "Each effect is one named operation — argv resolved from the capability catalog.\nScalar identities (`--id`, `--version`, …) ride on flags; the exact `--version` is always the opaque entity version"},
		{"agents/x.md", "pass the record's `--version` unchanged"},
		{"skills/x/SKILL.md", "then `docket run start implement-next` with `--version <v>`"},
		// A foreign tool on an EARLIER line does not bind a --version below it.
		{"skills/x/SKILL.md", "record `codex --version` alongside any finding,\nthen pass `--version <v>` to the claim"},
		// Only a TITLE naming a foreign tool binds the whole document.
		{"skills/x/SKILL.md", "# docket-finalize-change\n\nsee release-smoke.sh\n\n| `--version <v>` | yes | the record |"},
	} {
		if !hasRetiredRow(scanTextContent(c.rel, c.text), "40") {
			t.Errorf("row 40: an unbound markdown --version in %s was not detected: %q", c.rel, c.text)
		}
	}
	if hits := scanText("dispatch/x.md", "Scalar identities (`--id`, `--version`, …) ride on flags", true); !hasRetiredRow(hits, "40") {
		t.Errorf("row 40: an unbound --version in generator markdown output was not detected")
	}
	for _, src := range []string{
		"package p\nfunc f(c *C) { v, _ := c.Flags().GetString(\"version\"); _ = v }\n",
		"package p\nfunc f(c *C) { _ = c.MarkFlagRequired(\"version\") }\n",
		// Review finding: the *Var / *VarP forms name the flag in the SECOND argument.
		"package p\nfunc f(c *C) { c.Flags().StringVar(&v, \"version\", \"\", \"x\") }\n",
		"package p\nfunc f(c *C) { c.Flags().StringVarP(&o.v, \"version\", \"v\", \"\", \"x\") }\n",
	} {
		if !hasRetiredRow(goHits("internal/cli/finalize.go", src), "40") {
			t.Errorf("row 40: a bound Go flag call was not detected: %q", src)
		}
	}
	// Family (e) (change 0469): row 68 is a kindWord row. The bare halt cause hits
	// it in Go and markdown; the two hyphenated compounds ending in its spelling
	// are other words — scope-identity-mismatch hits nothing (a negative control
	// below), and pr-identity-mismatch hits row 72, never row 68. Row 67's
	// composed refusal reason not-ready-needs-brainstorm hits row 67.
	if !hasRetiredRow(goHits("internal/gatedrive/driver.go", "package p\nfunc f() { halt(&res, \"identity-mismatch\") }\n"), "68") {
		t.Errorf("row 68: a bare identity-mismatch halt literal was not detected")
	}
	if !hasRetiredRow(scanTextContent("skills/x/SKILL.md", "the drive halts `identity-mismatch`"), "68") {
		t.Errorf("row 68: a bare identity-mismatch in markdown was not detected")
	}
	if hits := scanTextContent("skills/x/SKILL.md", "finalize refuses `pr-identity-mismatch`"); hasRetiredRow(hits, "68") || !hasRetiredRow(hits, "72") {
		t.Errorf("pr-identity-mismatch must hit row 72 and never row 68: %v", hits)
	}
	if !hasRetiredRow(goHits("internal/domain/actions.go", "package p\nvar r = \"not-ready-needs-brainstorm\"\n"), "67") {
		t.Errorf("row 67: the composed not-ready-needs-brainstorm reason was not detected")
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
		"run.start then run.verdict then run.continue",
		// Change 0477's new spellings, and the committed receipt key row 38c must
		// never reach (Decision 3).
		"docket gate drive start --run-context <ctx>",
		"pass it, always as `--run-context`, into every `gate.drive.prepare-scope` / `gate.drive.start`",
		"DOCKET_AGENT_GUARDIAN_RUN_KEY is set on the guardian",
		"the run.start result carries run_context and the gate drive stores run_context_hash",
		"the claim receipt's gate_context_hash is committed state",
		// Change 0472 — kept namesakes and unbound --version (spec §3 shape boundaries).
		"docket version --json",
		"run `releasepkg --source <dir> --version v1.2.3 --commit <sha> --source-epoch 1 --out <dir>`",
		"sh install.sh --version v1.2.3 --harness claude",
		"`opencode run --version` prints the version, while `opencode run -- --version` sends it as the message",
		"record `codex --version` alongside any finding",
		// Nearest binding wins: run.start follows change.claim, so codex's --version is unbound.
		"the `change.claim` operation, then `docket run start implement-next`, then record `codex --version`",
		// A blank line ends the block, so the op cannot bind a --version below it.
		"the `change.claim` operation with `--id <id> --revision <v>`\n\nthen record `cursor-agent --version`",
		"the `change.claim` operation with `--id <id> --revision <v>`",
		"docket change relink --id 1 --expect-revision <v> --adopt-pr-head",
		// A document titled by a foreign tool documents that tool (scripts/release-smoke.md).
		"# release-smoke.sh — native per-tuple smoke driver\n\n| Argument | Required |\n|---|---|\n| `--version <v>` | yes |",
		"the rebase receipt keeps resolver_budget_version",
		"every mutation result carries committed_revision, metadata_revision and base_branch_revision",
		"the claim digest payload keeps its version key",
		// Change 0474 — the new spellings and the kept namesakes (spec §D).
		"apply `change.groom` with `outcome: re-enable`; a `nothing-to-re-enable` refusal writes nothing",
		"`final-backlink-pending`, `final-notes-frozen`, `not-final`, `skipped-final`, `adr-update-after-final`",
		"`change-final-claim-stamp` and `drop-final-claimed-at`; the warning is `shared-setting-ignored`",
		"read `../docket-convention/references/close-out.md` now — blocking",
		"terminal_publish: false stays parseable; terminal publication is deferred from Go v1",
		"a run-continue is nonterminal; incumbent-nonterminal; the run reached a terminal disposition",
		"the frozen fixture testdata/repositories/v0.9.2/fenced-machine-keys/ keeps its name",
		"a signal re-arm escalation is drained and ignored",
		"`skipped-not-open` for a non-final child with no open PR",
		// Change 0469 — family (e): the new spellings and the kept namesakes
		// (ADR-0129 "Kept in family (e)").
		"readiness `needs-grooming`; the refusal reason `not-ready-needs-grooming`",
		"refused `scope-identity-mismatch`: the scope pins another change",
		"the drive halts `worktree-changed` or `launch-unconfirmed`; a `launch-unresolved` halt is different",
		"the slot state `unresolved` and the stage `mark-worktree-execution-unresolved` stay",
		"`relinked-branch` / `relinked-pr` / `stale-evidence` / `workspace-conflict` / `candidate-branch-absent` / `pr-unknown` / `invalid-request`",
		"finalize refuses `pr-link-mismatch`; recertify refuses `certified-input-changed`",
		"`results-identity-broken`, `identity-reused`, `identity-mutated`, `fingerprint-mismatch`",
		"an identity-mismatched repository definition is refused",
		// Change 0491 — the gate supervisor's own run id is a different identity
		// and must never match a run-id row.
		"the `gate.observe` operation reports `run_id: 0790b760e26444866ef2e156ba383326` for the raw run",
		"a worktree-busy refusal names `incumbent-run:<id>` while that raw run holds the lock",
		"the gate supervisor's raw run id is the base name of its run dir",
		"agent.enter --run-key <key>; run cancel --key <key> --reason <why>; run-started <key> <run-context>",
		"refused `run-superseded`, `run-record-conflict`, or `run-not-found`; DOCKET_AGENT_GUARDIAN_RUN_KEY is set",
	}
	for _, line := range cleanText {
		for _, rel := range []string{"skills/x/SKILL.md", "tests/test_x.sh"} {
			if hits := scanTextContent(rel, line); len(hits) != 0 {
				t.Errorf("negative control %q in %s matched: %v", line, rel, hits)
			}
		}
	}
	cleanGo := []struct{ rel, src string }{
		{"internal/app/change_claim.go", "package p\ntype R struct {\n\tH string `json:\"gate_context_hash\"`\n}\n"},
		{"cmd/releasepkg/main.go", "package p\nvar f = \"source-epoch\"\n"},
		{"internal/cli/gate.go", "package p\nvar f = \"run-context\"\n"},
		{"internal/app/agent_guardian.go", "package p\nconst e = \"DOCKET_AGENT_GUARDIAN_RUN_KEY\"\n"},
		{"internal/app/runtracker_start.go", "package p\ntype R struct {\n\tC string `json:\"run_context,omitempty\"`\n}\n"},
		{"internal/app/evidence_ops.go", "package p\nvar r = \"gate-stopped\"\n"},
		{"internal/p/p.go", "package p\n// run epoch gate-armed in a comment is a passing mention\nvar x = 1\n"},
		{"cmd/releasepkg/main.go", "package p\nvar v = fs.String(\"version\", \"\", \"safe release version\")\n"},
		{"internal/cli/root.go", "package p\nvar c = capability(\"version\", EffectRead)\n"},
		{"internal/cli/root.go", "package p\nvar c = &C{Use: \"version\"}\n"},
		{"internal/cli/install.go", "package p\nvar m = map[string]bool{\"version\": true}\n"},
		{"internal/cli/change.go", "package p\nfunc f(c *C) { c.Flags().String(\"revision\", \"\", \"x\") }\n"},
		{"internal/cli/agent.go", "package p\nfunc f(c *C) { c.Flags().StringVar(&v, \"revision\", \"\", \"version\") }\n"},
		{"internal/cli/agent.go", "package p\nfunc f(c *C) { c.Flags().StringVarP(&v, \"revision\", \"r\", \"version\", \"x\") }\n"},
		{"cmd/releasepkg/main.go", "package p\nfunc f() { fs.StringVar(&v, \"version\", \"\", \"safe release version\") }\n"},
		{"internal/cli/root.go", "package p\nfunc f() { x.Log(msg, \"version\") }\n"},
		{"internal/app/change_claim.go", "package p\ntype P struct {\n\tRevision string `json:\"version\"`\n}\n"},
		{"internal/workspace/rebasereceipt.go", "package p\ntype R struct {\n\tB string `json:\"resolver_budget_version,omitempty\"`\n}\n"},
		{"internal/app/change_groom.go", "package p\nconst o = \"re-enable\"\n"},
		{"internal/config/config.go", "package p\nconst c = \"shared-setting-ignored\"\n"},
		{"internal/app/finalize_retarget.go", "package p\nconst c = \"skipped-final\"\n"},
		{"internal/config/fixtures_test.go", "package p\nvar d = \"fenced-machine-keys\"\n"},
		{"internal/gatedrive/ownership.go", "package p\nconst k = \"scope-identity-mismatch\"\n"},
		{"internal/gatedrive/ownership.go", "package p\nconst k = \"launch-unconfirmed\"\n"},
		{"internal/gatedrive/driver.go", "package p\nvar c = \"worktree-changed\"\n"},
		{"internal/app/change_relink.go", "package p\nconst o = \"change.relink\"\n"},
		{"internal/domain/finalize.go", "package p\nconst c = \"pr-link-mismatch\"\n"},
		// Change 0491 — the gate supervisor's run_id (struct tag and report line)
		// and the run tracker's surviving key spellings.
		{"internal/app/gate.go", "package p\ntype R struct {\n\tRunID string `json:\"run_id\"`\n}\n"},
		{"internal/app/gate.go", "package p\nfunc f() { lines = append(lines, \"run_id: \"+r.RunID) }\n"},
		{"internal/cli/run.go", "package p\nfunc f(c *C) { c.Flags().String(\"key\", \"\", \"x\"); c.Flags().String(\"run-key\", \"\", \"x\") }\n"},
		{"internal/app/runtracker_fence.go", "package p\nconst a, b = \"run-superseded\", \"run-record-conflict\"\n"},
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
	// Kept schema keys are exact (scope, path) pairs: clean where kept, retired
	// anywhere else.
	kept := app.SchemaResult{
		EnvelopeShape: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "protocol_version"}}},
		Operations: []app.OperationSchema{
			{ID: "capabilities", Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "capability_version"}, {Key: "binary", Fields: []app.FieldDescriptor{{Key: "version"}}}}}},
			{ID: "diagnostic.runtime", Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "go_version"}}}},
			{ID: "version", Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "version"}}}},
			{ID: "change.claim", Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "committed_revision"}}}},
		},
	}
	if hits, _, _ := schemaVersionKeyHits(kept); len(hits) != 0 {
		t.Errorf("kept schema keys matched: %v", hits)
	}
	// Change 0491: kindSchemaPath is exact, so the gate supervisor's run_id in
	// every gate.* result, and run.start's kept keys, never match row 38b.
	sup := app.SchemaResult{Operations: []app.OperationSchema{
		{ID: "gate.launch", Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "run_id"}}}},
		{ID: "gate.observe", Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "run_id"}}}},
		{ID: "gate.recover", Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "entries", Fields: []app.FieldDescriptor{{Key: "run_id"}}}}}},
		{ID: "gate.stop", Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "run_id"}}}},
		{ID: "run.start", Request: &app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "run_id"}}}, Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "key"}, {Key: "run_context"}, {Key: "result", Fields: []app.FieldDescriptor{{Key: "run_id"}}}}}},
	}}
	if _, _, seen := schemaVersionKeyHits(sup); len(schemaPathHits(seen)) != 0 {
		t.Errorf("the gate supervisor's run_id (or a non-exact run.start path) matched a schema-path row: %v", schemaPathHits(seen))
	}
	moved := app.SchemaResult{Operations: []app.OperationSchema{
		{ID: "status", Result: app.TypeDescriptor{Fields: []app.FieldDescriptor{{Key: "binary", Fields: []app.FieldDescriptor{{Key: "version"}}}}}},
	}}
	if hits, _, _ := schemaVersionKeyHits(moved); len(hits) != 1 || !strings.Contains(hits[0], "binary.revision") {
		t.Errorf("a kept spelling at an unkept path was not detected naming binary.revision: %v", hits)
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
