package repoguard

// Change 0420: a resumed build worker executed `status=$?` after capturing a
// gate.drive.start JSON response; in zsh `status` is a read-only special
// parameter, so the shell aborted before the drive id and owner generation
// were parsed. Two prongs:
//   (1) the shared caller contract (gate-caller-loop.md) names the shell-safe
//       capture variables (`gate_reply` for the response, `gate_rc` for the
//       exit code), requires them to work in both zsh and bash, and forbids
//       assigning zsh read-only special parameters — phrase bound to claim,
//       one bounded gap per assert (stacked gaps backtrack catastrophically);
//   (2) no paragraph of the workflow-markdown corpus (source skills/ and
//       agents/ plus their embedded mirrors) carries a reserved-parameter
//       ASSIGNMENT shape, and the shell-safe names actually appear at the
//       gate-capture sites (population floors, so deleting the instruction
//       or displacing the scan reddens rather than passing vacuously).
// Keyed on syntactic shape — a reserved name immediately followed by `=`,
// bounded on the left — never on an enumerated list of RHS spellings:
// status=$?, status="$?", and status=$(...) all match because the RHS is
// unconstrained.
// LIMITATION (byte-pattern-guard-matches-a-spelling): the reserved-name set
// is the zsh read-only special parameters a capture site plausibly reaches
// for (status, pipestatus, signals, ARGC), derived from zshparam(1), not
// from repo spellings; a TOML-style `status = "x"` with spaces around `=` is
// out of shape and out of scope. Maintained prose that must NAME a reserved
// parameter writes it without a trailing `=` (backticked `status`), which is
// also what keeps this guard's own contract clauses out of its violation set.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var (
	// An assignment to a zsh read-only special parameter: the name abutting
	// `=`, left-bounded so `--status=`, `pipestatus` inside a longer word,
	// and `gate_rc=` never match. `-` in the class exempts flag spellings;
	// `$` exempts dereferences.
	reservedAssignRe = regexp.MustCompile(`(^|[^[:alnum:]_$-])(status|pipestatus|signals|ARGC)=`)

	// A gate-capture site mention: any gate.drive operation reference.
	gateOpMentionRe = regexp.MustCompile(`gate\.drive\.(start|advance|handoff|claim|takeover|prepare-scope)`)

	// The shell-safe capture vocabulary the contract mints.
	shellSafeNameRe = regexp.MustCompile(`gate_reply|gate_rc`)

	// Contract-clause asserts: one bounded gap each.
	reqReplyName   = regexp.MustCompile(`(?i)capture[^.]{0,120}gate_reply`)
	reqRcName      = regexp.MustCompile(`(?i)exit code[^.]{0,80}gate_rc`)
	reqBothShells  = regexp.MustCompile(`(?i)zsh and bash`)
	reqNeverAssign = regexp.MustCompile(`(?i)never assign[^.]{0,60}read-only special parameter`)
	reqAbortReason = regexp.MustCompile(`(?i)read-only special parameter[^.]{0,160}aborts`)
)

func TestGateCaptureReservedShellParams(t *testing.T) {
	root := guardRoot(t)

	// Prong 1 — the shared contract carries the shell-safe capture clauses,
	// phrase bound to claim. The embedded mirror is byte-identical by the
	// assets drift check, so the source copy is the one asserted here.
	contract := strings.Join(strings.Fields(readMaintained(t, root, sharedContractRel)), " ")
	for name, re := range map[string]*regexp.Regexp{
		"reply-capture-name":  reqReplyName,
		"rc-capture-name":     reqRcName,
		"works-in-both":       reqBothShells,
		"never-assign":        reqNeverAssign,
		"abort-consequence":   reqAbortReason,
	} {
		if !re.MatchString(contract) {
			t.Errorf("shared contract %s lost its %s clause (pattern %v)", sharedContractRel, name, re)
		}
	}

	// Prong 2 — corpus scan over the workflow markdown population (source and
	// embedded mirrors both; docs/ and testdata are excluded categorically by
	// MaintainedFiles). Violations and floors are collected in one pass.
	var violations []string
	sites := 0
	perFile := map[string]int{}
	for _, rel := range maintainedPop(t, root) {
		if !isWorkflowMD(rel) {
			continue
		}
		for _, p := range paragraphs(readMaintained(t, root, rel)) {
			if reservedAssignRe.MatchString(p) {
				violations = append(violations, fmt.Sprintf("%s: assignment to a zsh read-only special parameter: %.160s", rel, p))
			}
			if gateOpMentionRe.MatchString(p) && shellSafeNameRe.MatchString(p) {
				sites++
				perFile[rel]++
			}
		}
	}

	// Population floors FIRST (a vacuous scan passes every negative): each
	// caller skill carries the shell-safe names at a gate.drive paragraph,
	// and the corpus (source + embedded mirrors) stays above a global floor.
	if sites < 6 {
		t.Fatalf("population floor: only %d gate.drive paragraphs carry the shell-safe capture names (expected >= 6: three caller skills plus embedded mirrors)", sites)
	}
	for _, rel := range []string{
		"skills/docket-build-task/SKILL.md",
		"skills/docket-build/SKILL.md",
		"skills/docket-implement-next/SKILL.md",
	} {
		if perFile[rel] == 0 {
			t.Errorf("coverage floor: %s names no shell-safe capture variable at a gate.drive site (instruction deleted, or scan drifted)", rel)
		}
	}
	if len(violations) != 0 {
		t.Errorf("reserved zsh parameter assignments (%d):\n%s", len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		if !reservedAssignRe.MatchString("then run status=$? to record the exit code") {
			t.Errorf("a space-preceded status=$? assignment was not classified as a violation")
		}
		if !reservedAssignRe.MatchString(`reply="$(op --json)"; status=$?`) {
			t.Errorf("a semicolon-preceded status=$? assignment was not classified as a violation")
		}
		if !reservedAssignRe.MatchString("pipestatus=(0 1)") {
			t.Errorf("a line-leading pipestatus= assignment was not classified as a violation")
		}
		if !reservedAssignRe.MatchString(`status="$?"`) {
			t.Errorf("a quoted-RHS status= assignment was not classified as a violation (shape must be RHS-independent)")
		}
		if reservedAssignRe.MatchString("gate_rc=$? captures the exit code") {
			t.Errorf("the shell-safe gate_rc= assignment was wrongly flagged")
		}
		if reservedAssignRe.MatchString("--status=green is a flag, not an assignment") {
			t.Errorf("a --status= flag spelling was wrongly flagged")
		}
		if reservedAssignRe.MatchString("read the exit status of the command") {
			t.Errorf("prose mentioning the words exit status was wrongly flagged")
		}
		if reservedAssignRe.MatchString(`never assign `+"`status`"+` or `+"`pipestatus`") {
			t.Errorf("a backticked parameter NAME without `=` was wrongly flagged — the contract prose itself must stay legal")
		}
		if reqRcName.MatchString("the exit code matters. gate_rc is discussed elsewhere") {
			t.Errorf("bounded gap failed: exit code and gate_rc in separate sentences must not satisfy the clause bind")
		}
	})
}
