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
	// Scope, when non-nil, narrows a match to lines (or, for Go literals, to
	// files) where the retired spelling is the retired flag rather than a kept
	// namesake: row 12 retires `change claim --gate-context` only; the gate
	// drive's own --gate-context is not an ADR-0129 row and stays.
	Scope func(rel, line string) bool
}

var (
	changeClaimLineRe = regexp.MustCompile(`change[ .]claim`)
	gateDriveLineRe   = regexp.MustCompile(`gate[ .-]drive`)
)

// changeClaimScope confines row 12 to change-claim sites. A Go literal is one
// only in internal/cli/change.go. A text line is one when it names `change
// claim` and no gate drive; when it names both, a --gate-context occurrence
// belongs to whichever command noun sits nearest it. The generated dispatch
// clause says "labeled for `change.claim --run-context` and gate-drive
// `--gate-context`": that flag is the gate drive's kept one, and a line-wide
// `change claim` test would flag it.
func changeClaimScope(rel, line string) bool {
	if rel == "internal/cli/change.go" {
		return true
	}
	claims := changeClaimLineRe.FindAllStringIndex(line, -1)
	if len(claims) == 0 {
		return false
	}
	drives := gateDriveLineRe.FindAllStringIndex(line, -1)
	if len(drives) == 0 {
		return true
	}
	for _, flag := range tokenRe("--gate-context").FindAllStringIndex(line, -1) {
		if spanDistance(claims, flag) < spanDistance(drives, flag) {
			return true
		}
	}
	return false
}

// spanDistance is the byte gap between span and the nearest of spans (0 when
// they overlap).
func spanDistance(spans [][]int, span []int) int {
	best := -1
	for _, s := range spans {
		d := 0
		switch {
		case s[1] <= span[0]:
			d = span[0] - s[1]
		case s[0] >= span[1]:
			d = s[0] - span[1]
		}
		if best < 0 || d < best {
			best = d
		}
	}
	return best
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
	{Row: "12", Kind: kindToken, Old: "--gate-context", New: "change claim --run-context", Scope: changeClaimScope},
	{Row: "12", Kind: kindGoFlag, Old: "gate-context", New: "run-context", Scope: changeClaimScope},
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
// kindToken row.
func scanTextLine(rel string, lineNo int, line string) []retiredHit {
	var hits []retiredHit
	for _, r := range retiredVocabulary {
		if r.Kind != kindToken {
			continue
		}
		if tokenRe(r.Old).MatchString(line) && (r.Scope == nil || r.Scope(rel, line)) {
			hits = append(hits, retiredHit{rel, lineNo, r, strings.TrimSpace(line)})
		}
	}
	return hits
}

// scanTextContent scans a markdown/shell/config file. Shell and config lines
// have their `#` comment stripped (stripHashComment, absence_test.go); markdown
// is scanned whole, prose included.
func scanTextContent(rel, content string) []retiredHit {
	var hits []retiredHit
	md := isMarkdownSurface(rel)
	for i, raw := range strings.Split(content, "\n") {
		line := raw
		if !md {
			line = stripHashComment(raw)
		}
		hits = append(hits, scanTextLine(rel, i+1, line)...)
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
		if r.Scope != nil && !r.Scope(rel, "") {
			continue
		}
		hit := false
		switch r.Kind {
		case kindToken:
			hit = tokenRe(r.Old).MatchString(val)
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
			goRel := "internal/p/p.go"
			if r.Scope != nil {
				line = "docket change claim " + r.Old + " x"
				goRel = "internal/cli/change.go"
			}
			hits = append(hits, scanTextContent("skills/x/SKILL.md", line)...)
			hits = append(hits, scanTextContent("tests/test_x.sh", "docket "+line)...)
			hits = append(hits, goHits(goRel, "package p\nvar v = "+strconv.Quote("x "+r.Old+" y")+"\n")...)
			want = 3
		case kindGoFlag, kindGoPrefix:
			rel := "internal/cli/run.go"
			if r.Scope != nil {
				rel = "internal/cli/change.go"
			}
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
	// caught next to a kept gate-drive one (the nearest-noun rule in
	// changeClaimScope, whose kept side negative_controls pins).
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
		{"internal/cli/gate.go", "package p\nvar f = \"gate-context\"\n"},
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
		for i, line := range strings.Split(string(b), "\n") {
			for _, h := range scanTextLine(rel, i+1, line) {
				violations = append(violations, "[generator-output] "+h.String())
			}
		}
	}
	slices.Sort(violations)
	if len(violations) != 0 {
		t.Errorf("retired ADR-0129 vocabulary in GENERATOR OUTPUT (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}
}
