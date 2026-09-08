package repoguard

// Change 0376: any maintained workflow instruction that invokes a
// credential-returning gate.drive operation with concrete flags must pass
// --json and capture the first response — a drive id gleaned from human text
// never authorizes advancement or handoff. Two prongs:
//   (1) the shared caller contract states the capture requirement and the
//       caller-contract-failure rule (phrase bound to claim, bounded gaps);
//   (2) every flag-bearing invocation paragraph of a credential-returning op
//       (start|handoff|claim|takeover|prepare-scope — advance returns no new
//       credentials) in the workflow-markdown corpus carries --json.
// Keyed on syntactic shape (op reference + flag token in one paragraph),
// never a per-file allowlist. Residual risk, recorded not hidden: an author
// who names an op without any --flag in the same paragraph is classified as
// prose mention, not invocation; the shared contract carries that case.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

const sharedContractRel = "skills/docket-build/references/gate-caller-loop.md"

var (
	credOpRe   = regexp.MustCompile(`gate\.drive\.(start|handoff|claim|takeover|prepare-scope)`)
	flagRe     = regexp.MustCompile(`--[a-z][a-z-]+`)
	jsonFlagRe = regexp.MustCompile(`--json`)

	// Contract-clause asserts: one bounded gap each (two stacked gaps
	// backtrack catastrophically on non-matching input).
	reqMustJSON  = regexp.MustCompile(`(?i)MUST pass[^.]{0,40}--json`)
	reqCapture   = regexp.MustCompile(`(?i)--json[^.]{0,200}capture`)
	reqValidate  = regexp.MustCompile(`(?i)validate[^.]{0,120}before acting`)
	reqFailure   = regexp.MustCompile(`(?i)missing, malformed, or incomplete[^.]{0,160}caller-contract failure`)
	reqNoRerun   = regexp.MustCompile(`(?i)not[^.]{0,20}permission to rerun`)
	reqBlocked   = regexp.MustCompile("(?i)returns `BLOCKED` with the[^.]{0,60}reason")
	reqParentCap = regexp.MustCompile(`(?i)parent capability[^.]{0,80}stays with the parent`)
)

// paragraphs splits markdown into blank-line-delimited blocks with all runs
// of whitespace collapsed to single spaces, so wrapped prose matches
// single-line patterns (phrase-grep-over-wrapped-prose).
func paragraphs(content string) []string {
	var out []string
	for _, block := range regexp.MustCompile(`\n[ \t]*\n`).Split(content, -1) {
		out = append(out, strings.Join(strings.Fields(block), " "))
	}
	return out
}

// isJSONCaptureSite: a paragraph is a SITE when it references a
// credential-returning op AND carries at least one concrete --flag token; a
// site VIOLATES when no --json appears in the same paragraph.
func isJSONCaptureSite(p string) bool {
	return credOpRe.MatchString(p) && flagRe.MatchString(p)
}

func TestGateDriveJSONCapture(t *testing.T) {
	root := guardRoot(t)

	// Prong 1 — the shared contract carries the requirement, phrase bound to
	// claim. The embedded mirror is byte-identical by the assets DiffTree
	// check, so the source copy is the one asserted here.
	contract := strings.Join(strings.Fields(readMaintained(t, root, sharedContractRel)), " ")
	for name, re := range map[string]*regexp.Regexp{
		"must-pass-json":        reqMustJSON,
		"json-bound-to-capture": reqCapture,
		"validate-before-use":   reqValidate,
		"contract-failure":      reqFailure,
		"no-rerun-recovery":     reqNoRerun,
		"maps-to-blocked":       reqBlocked,
		"parent-cap-stays":      reqParentCap,
	} {
		if !re.MatchString(contract) {
			t.Errorf("shared contract %s lost its %s clause (pattern %v)", sharedContractRel, name, re)
		}
	}

	// Prong 2 — corpus scan. The shared contract file (and its embedded
	// mirror) is the requirement's definition, not a caller: excluded.
	var sites, violations []string
	perFile := map[string]int{}
	for _, rel := range maintainedPop(t, root) {
		if !isWorkflowMD(rel) || strings.HasSuffix(rel, "docket-build/references/gate-caller-loop.md") {
			continue
		}
		for _, p := range paragraphs(readMaintained(t, root, rel)) {
			if !isJSONCaptureSite(p) {
				continue
			}
			sites = append(sites, rel)
			perFile[rel]++
			if !jsonFlagRe.MatchString(p) {
				violations = append(violations, fmt.Sprintf("%s: credential gate.drive invocation without --json: %.160s", rel, p))
			}
		}
	}

	// Population floors FIRST (a vacuous scan passes every negative): the
	// three caller skills each contribute, and the corpus (source + embedded
	// mirrors) stays above a global floor.
	if len(sites) < 8 {
		t.Fatalf("population floor: only %d credential gate.drive invocation sites discovered (expected >= 8)", len(sites))
	}
	for _, rel := range []string{
		"skills/docket-build-task/SKILL.md",
		"skills/docket-build/SKILL.md",
		"skills/docket-implement-next/SKILL.md",
	} {
		if perFile[rel] == 0 {
			t.Errorf("coverage floor: %s contributes no invocation site (scan or corpus drifted)", rel)
		}
	}
	if len(violations) != 0 {
		t.Errorf("gate.drive JSON-capture violations (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		bad := "run the `gate.drive.start` operation with `--owner task --scope-id <id>` and read the drive id"
		good := bad + " from the `--json` response"
		mention := "performed an explicit `gate.drive.claim` operation on the named handoff"
		advOnly := "the `gate.drive.advance` operation with `--drive-id <id> --owner-gen <gen>`"
		if !isJSONCaptureSite(bad) || jsonFlagRe.MatchString(bad) {
			t.Errorf("a flag-bearing credential invocation without --json was not classified as a violating site")
		}
		if !isJSONCaptureSite(good) || !jsonFlagRe.MatchString(good) {
			t.Errorf("a --json-carrying invocation was not classified as a compliant site")
		}
		if isJSONCaptureSite(mention) {
			t.Errorf("a flagless prose mention was wrongly classified as an invocation site")
		}
		if isJSONCaptureSite(advOnly) {
			t.Errorf("an advance-only paragraph was wrongly classified as a credential site")
		}
		wrapped := "run the `gate.drive.handoff` operation\nwith `--drive-id <id>\n--owner-gen <gen>`"
		if got := paragraphs(wrapped); len(got) != 1 || !isJSONCaptureSite(got[0]) {
			t.Errorf("whitespace collapse failed: a wrapped invocation was not one matchable paragraph")
		}
		if reqCapture.MatchString("the --json flag exists. capture is discussed elsewhere") {
			t.Errorf("bounded gap failed: --json and capture in separate sentences must not satisfy the clause bind")
		}
	})
}
