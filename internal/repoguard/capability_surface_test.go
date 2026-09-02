package repoguard

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// This guard enforces change 0394's central invariant on the MAINTAINED workflow
// surfaces (skills/, agents/, cursor-rules/): agent-executed markdown is code
// (agent-executed-markdown-is-code), and the ONLY executable docket CLI spelling
// it may hard-code is the capability bootstrap `docket capabilities --json`. Every
// other executable invocation must be resolved from the catalog at runtime, so a
// hard-coded `docket <argv>` spelling on these surfaces is a defect — except the
// small, asserted set of human-typed bootstrap remedies that run before any
// catalog exists to resolve against.
//
// ---------------------------------------------------------------------------
// KNOWN IMPRECISION — what this SHAPE catches, and the residue it cannot
// ---------------------------------------------------------------------------
//
// SCOPE IS EXECUTABLE POSITION, NOT THE BARE PRODUCT NAME. A naive `docket
// <word>` scan also strikes ~69 legitimate PROSE occurrences on these surfaces:
// product-name English ("docket metadata", "docket backlog", "docket tracks",
// "docket state", "docket board") and bare command-GROUP nouns in backticks
// (`docket gate` used as a noun for the gate subsystem). The exemption set must
// NOT be allowed to absorb those — that is exemption laundering. So the shape is
// narrowed to executable position:
//
//	docket <FAMILY> <flag-or-subcommand-token>
//
// i.e. `docket`, then a real catalog command FAMILY, then a THIRD token that is a
// flag (`--x`) or a subcommand/verb word (`[a-z][a-z-]*`). This is what makes it
// an invocation rather than a noun phrase:
//   - "docket metadata" / "docket backlog" never match — the second word is not a
//     catalog family.
//   - A bare `docket gate` used as an English noun never matches — a GROUP family
//     with no following subcommand/flag token is not a command.
//   - `docket change create ...`, `docket repository migrate` DO match — a GROUP
//     family followed by its verb is an invocation.
//
// THE FAMILY VOCABULARY IS DERIVED, NEVER HAND-LISTED (enumerated-floor): the
// family set comes from the binary's own `docket capabilities --json` at test
// time (catalogFamilies), so a family added to the tree later is covered
// automatically and no spelling this file failed to anticipate can slip through
// as its own house idiom.
//
// RESIDUE this line-based shape deliberately does not see (byte-pattern-guard-
// matches-a-spelling): a bare, COMPLETE leaf command sitting inside a pure inline-
// code span with NO flag and NO subcommand token (e.g. a lone `` `docket status` ``)
// is below the shape — telling it apart from prose would require a markdown parser,
// and the migration removed every such executable spelling; a re-introduced
// invocation on these surfaces essentially always carries a flag or subcommand and
// is caught. Prose that merely NAMES a command in backticks is byte-identical to an
// instruction to run it; this guard bans the executable SPELLING, not the intent,
// and the asserted human-remedy exemption set is the sanctioned residue.
//
// EXCLUSIONS. The population is drawn from maintainedPop, which already prunes the
// categorical corpora (docs/ immutable history, any testdata/, tests/fixtures,
// .worktrees, internal/install/legacydata). On top of that this guard scans ONLY
// skills/**/*.md, agents/**/*.md, and every file under cursor-rules/. Excluded by
// that scoping, and called out here because the exclusion is load-bearing:
//   - scripts/*.md are frozen script-layer CONTRACTS (documentation of the shell
//     layer), not agent-executed workflow surfaces — out of scope.
//   - internal/assets/embedded/** is the stale EMBEDDED copy regenerated from the
//     authored trees; this guard scans the AUTHORED surfaces, never the generated
//     mirror (it is not under skills/agents/cursor-rules, so it is out already).

// capabilityExemptions pins the bucket-2 human-typed bootstrap remedies MEASURED
// at HEAD after the skill migration — spellings a HUMAN runs before any catalog
// exists to resolve against, never an agent runtime invocation. Each value is the
// exact occurrence count on the maintained workflow surfaces; a changed count is a
// laundering tripwire, not a number to reconcile (guard-remedy-must-not-teach-the-
// evasion — the failure message leads with the migration remedy, below).
var capabilityExemptions = map[string]int{
	"docket repository migrate":         9,
	"docket repository init":            3,
	"docket repository configure-tests": 3,
	// `docket change create` is exempt ONLY as HUMAN-ACTION prose: the convention/
	// fix-loop/implement-next surfaces describe what a *human* does to capture
	// discovered work ("a human captures reported work deliberately with `docket
	// change create`"), never an agent runtime invocation. The agent-executed sites
	// in docket-new-change were migrated to the `change.create` catalog operation, so
	// this pin certifies human prose only — a rise here means an executable spelling
	// was laundered back in, not that the number is stale.
	"docket change create": 5,
}

// capabilitySurfaceRemedy is the substantive check the guard's failures lead with,
// before any exemption count is named, so a reader is steered to migrate the site,
// not to launder it into the exemption pin.
const capabilitySurfaceRemedy = "a new `docket <argv>` spelling on a workflow surface must be migrated to a catalog-resolved semantic operation — see docket-convention's Step-0 preamble"

// capabilitySurfaceCorpus is the maintained workflow surface: every *.md under
// skills/ and agents/, and every file under cursor-rules/ (agent-executed markdown
// is code). Drawn from maintainedPop so the categorical exclusions and fail-closed
// read discipline apply.
func capabilitySurfaceCorpus(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, rel := range maintainedPop(t, root) {
		switch {
		case underDir(rel, "cursor-rules"):
			out = append(out, rel)
		case (underDir(rel, "skills") || underDir(rel, "agents")) && hasExt(rel, ".md"):
			out = append(out, rel)
		}
	}
	return out
}

// catalogFamilies derives the catalog command-family vocabulary from the binary's
// own read-only bootstrap — the authoritative source, never a hand-maintained list
// (enumerated-floor). It fails the test closed on any exec/parse error: a guard
// that cannot establish its vocabulary must not silently pass vacuously.
func catalogFamilies(t *testing.T, root string) []string {
	t.Helper()
	cmd := exec.Command("go", "run", "./cmd/docket", "capabilities", "--json")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("derive catalog families via `go run ./cmd/docket capabilities --json`: %v (fail closed)", err)
	}
	var doc struct {
		Commands []struct {
			Argv []string `json:"argv"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("parse capabilities catalog JSON: %v (fail closed)", err)
	}
	set := map[string]bool{}
	for _, c := range doc.Commands {
		if len(c.Argv) >= 2 {
			set[c.Argv[1]] = true
		}
	}
	fams := make([]string, 0, len(set))
	for f := range set {
		fams = append(fams, f)
	}
	sort.Strings(fams)
	return fams
}

// buildCapabilitySurfaceRe compiles the executable-position shape for the given
// derived family vocabulary. Bounded on the left by (^|[^[:alnum:]_./-]) so
// `dev/docket`, `.docket/`, and `docket-status` never match; bounded on the right
// by requiring a THIRD token (flag or subcommand/verb) so a bare group noun and
// product-name English cannot match (byte-pattern-guard-matches-a-spelling).
func buildCapabilitySurfaceRe(families []string) *regexp.Regexp {
	alt := strings.Join(families, "|")
	return regexp.MustCompile(`(^|[^[:alnum:]_./-])docket[[:space:]]+(` + alt + `)[[:space:]]+(--[a-z][a-z-]*|[a-z][a-z-]*)`)
}

// scanCapabilitySurface returns the non-exempt violations on one file and the
// per-spelling exemption hits it contributed. `docket capabilities ...` is the
// permitted bootstrap and is neither a violation nor an exemption; a spelling in
// capabilityExemptions is counted, not flagged; everything else is a violation.
func scanCapabilitySurface(re *regexp.Regexp, rel, content string) (violations []string, exemptHits map[string]int) {
	exemptHits = map[string]int{}
	for i, line := range strings.Split(content, "\n") {
		for _, m := range re.FindAllStringSubmatch(line, -1) {
			fam, third := m[2], m[3]
			if fam == "capabilities" {
				continue // the one permitted hard-coded bootstrap
			}
			spelling := "docket " + fam + " " + third
			if _, ok := capabilityExemptions[spelling]; ok {
				exemptHits[spelling]++
				continue
			}
			violations = append(violations, fmt.Sprintf("%s:%d: %s", rel, i+1, spelling))
		}
	}
	return violations, exemptHits
}

func TestCapabilitySurface(t *testing.T) {
	root := guardRoot(t)

	families := catalogFamilies(t, root)
	if len(families) < 10 {
		t.Fatalf("vocabulary floor: derived only %d catalog families (expected >= 10) — the shape would be near-vacuous", len(families))
	}
	re := buildCapabilitySurfaceRe(families)

	corpus := capabilitySurfaceCorpus(t, root)
	if len(corpus) < 40 {
		t.Fatalf("population floor: capability-surface corpus collapsed to %d files (expected >= 40)", len(corpus))
	}

	var violations []string
	exemptTotals := map[string]int{}
	for _, rel := range corpus {
		v, e := scanCapabilitySurface(re, rel, readMaintained(t, root, rel))
		violations = append(violations, v...)
		for k, n := range e {
			exemptTotals[k] += n
		}
	}

	if len(violations) != 0 {
		t.Errorf("%s\n%s", capabilitySurfaceRemedy, strings.Join(violations, "\n"))
	}

	// Exact per-spelling exemption counts: the pin is the laundering tripwire.
	// Any drift — a launderer adding a spelling into an agent-executed block, or a
	// remedy moved/removed — reddens here. Sorted for a deterministic report.
	spellings := make([]string, 0, len(capabilityExemptions))
	for s := range capabilityExemptions {
		spellings = append(spellings, s)
	}
	sort.Strings(spellings)
	for _, spelling := range spellings {
		want := capabilityExemptions[spelling]
		if got := exemptTotals[spelling]; got != want {
			t.Errorf("%s.\nThe human-typed remedy %q is pinned at %d occurrence(s); the surface now carries %d — a changed count means an argv spelling was added or moved, not that the pin is stale. Migrate the spelling; the pin exists to catch laundering, it is not a number to reconcile.",
				capabilitySurfaceRemedy, spelling, want, got)
		}
	}

	// Bootstrap-presence floor (marker-scoped-guard-needs-a-population-floor): the
	// convention's Step-0 preamble must spell the one permitted bootstrap at least
	// once, or the allow-rule guards nothing.
	conv := readMaintained(t, root, "skills/docket-convention/SKILL.md")
	if !strings.Contains(conv, "docket capabilities --json") {
		t.Errorf("bootstrap-presence floor: docket-convention's Step-0 preamble must spell `docket capabilities --json` at least once — the single permitted hard-coded bootstrap; without it the allow-rule and this whole guard are vacuous")
	}

	t.Run("non_vacuity", func(t *testing.T) {
		// The scanner fires on an executable invocation and stays silent on the
		// noun/prose forms the KNOWN IMPRECISION header carves out. Uses a synthetic
		// family set so the shape, not the live vocabulary, is under test.
		synth := buildCapabilitySurfaceRe([]string{"capabilities", "change", "gate", "repository", "status"})

		flags := []string{
			"run `docket status --json` before proceeding",  // leaf + flag
			"then `docket gate drive advance --run-dir $d`", // group + subcommand
			"execute docket change reconcile --repo-dir .",  // unfenced invocation
		}
		for _, s := range flags {
			if v, _ := scanCapabilitySurface(synth, "x.md", s); len(v) == 0 {
				t.Errorf("scanner missed an executable invocation: %q", s)
			}
		}

		quiet := []string{
			"the `docket gate` facade owns the run",         // bare GROUP noun in backticks
			"synced the docket metadata worktree",           // product-name English
			"drains the docket backlog into build-ready",    // product-name English
			"how docket tracks a change through its states", // product-name English
			"see dev/docket and the docket-status wrapper",  // boundary: path + hyphenated name
		}
		for _, s := range quiet {
			if v, _ := scanCapabilitySurface(synth, "x.md", s); len(v) != 0 {
				t.Errorf("scanner wrongly flagged a non-invocation %q: %v", s, v)
			}
		}

		// The bootstrap is allowed; the human remedies are counted, not flagged.
		if v, _ := scanCapabilitySurface(synth, "x.md", "run `docket capabilities --json`"); len(v) != 0 {
			t.Errorf("scanner wrongly flagged the permitted bootstrap: %v", v)
		}
		if v, e := scanCapabilitySurface(synth, "x.md", "the human runs `docket repository migrate`"); len(v) != 0 || e["docket repository migrate"] != 1 {
			t.Errorf("exemption not counted as exempt: viol=%v hits=%v", v, e)
		}
	})
}
