package repoguard

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
)

// Enforces change 0359's migration invariant: after Tasks 8/9/10 every test-intent
// command on a workflow fixture routes through the native gate DRIVER (`docket gate
// drive …`); a DIRECT test execution injected into a workflow fixture is a defect.
// The guard classifies workflow-shaped test-execution SITES by syntactic SHAPE,
// never by a hand-listed filename or a third-party runner spelling — the two things
// the repo's byte-pattern-guard rules forbid. Both closed identifiers it keys on are
// DERIVED at test time, never typed:
//
//   - (a) docket's OWN suite channel: the argv of the capability id `development.test`,
//     joined into `docket development test` and, with the one documented source-entry
//     prefix, `go run ./cmd/docket development test` — both read from the live catalog
//     (`docket capabilities --json`), so a later argv change tracks automatically.
//   - (b) a RESOLVED test-command identity interpolated into a shell invocation: the
//     exported-variable form of every config.Effective struct field whose json tag is
//     `test_command` (reflected — today build.test_command + finalize.test_command,
//     yielding $BUILD_TEST_COMMAND / $FINALIZE_TEST_COMMAND). Deriving the identity from
//     the CONSUMER (the config struct), never a spelling list, is the derive-from-the-
//     consumer rule.
//
// A site of either shape is a violation UNLESS it routes through `gate drive` — for
// (a) anywhere in the same fenced block (the sanctioned task-owner recipe carries the
// suite argv AFTER `-- ` on a `gate drive start` line), for (b) on the same line.
//
// Residual risk, recorded not hidden (mirroring gatedriver_test.go's recorded
// residual, per the byte-pattern-guard learning): the markdown detector only reads
// FENCED runnable recipes, so an author who writes the same suite spelling or
// `$BUILD_TEST_COMMAND` in INLINE prose back-ticks rather than a fence dodges it. The
// fence is the mechanical shape signal the house convention uses for every runnable
// recipe; this limitation is asserted in the red_injection subtest (the inline case
// must NOT flag).

// jsonTagName returns the bare json tag name of a struct field ("" if none).
func jsonTagName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return ""
	}
	if i := strings.IndexByte(tag, ','); i >= 0 {
		tag = tag[:i]
	}
	if tag == "-" {
		return ""
	}
	return tag
}

// testCommandExportedVars reflects over config.Effective and returns the exported-
// variable form (UPPER, dots→underscores) of every nested field whose json tag is
// `test_command`. This is the CONSUMER-derived identity set for violation shape (b);
// no test-command spelling and no config-key string is hand-listed.
func testCommandExportedVars(t *testing.T) []string {
	t.Helper()
	eff := reflect.TypeOf(config.Effective{})
	var vars []string
	seen := map[string]bool{}
	for i := 0; i < eff.NumField(); i++ {
		parent := eff.Field(i)
		parentTag := jsonTagName(parent)
		if parentTag == "" || parent.Type.Kind() != reflect.Struct {
			continue
		}
		sub := parent.Type
		for j := 0; j < sub.NumField(); j++ {
			if jsonTagName(sub.Field(j)) != "test_command" {
				continue
			}
			dotted := parentTag + ".test_command"
			exp := strings.ToUpper(strings.ReplaceAll(dotted, ".", "_"))
			if !seen[exp] {
				seen[exp] = true
				vars = append(vars, exp)
			}
		}
	}
	return vars
}

// suiteChannelSpellings derives the two direct spellings of docket's own suite
// channel from the live capability catalog — the authoritative argv, never a
// hand-maintained constant. Fails closed on any exec/parse/lookup error.
func suiteChannelSpellings(t *testing.T, root string) []string {
	t.Helper()
	cmd := exec.Command("go", "run", "./cmd/docket", "capabilities", "--json")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("derive development.test argv via `go run ./cmd/docket capabilities --json`: %v (fail closed)", err)
	}
	var doc struct {
		Commands []struct {
			ID   string   `json:"id"`
			Argv []string `json:"argv"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("parse capabilities catalog JSON: %v (fail closed)", err)
	}
	var argv []string
	for _, c := range doc.Commands {
		if c.ID == "development.test" {
			argv = c.Argv
			break
		}
	}
	if len(argv) < 2 {
		t.Fatalf("catalog has no usable development.test argv (got %v) — cannot derive the suite-channel spelling (fail closed)", argv)
	}
	// Join the catalog argv: the invoked form and the one documented source-entry
	// form (the `go run ./cmd/docket` prefix over the argv tail).
	direct := strings.Join(argv, " ")
	source := "go run ./cmd/docket " + strings.Join(argv[1:], " ")
	return []string{direct, source}
}

// spellingPattern turns a space-separated command spelling into a whitespace-
// tolerant, metacharacter-safe regexp fragment.
func spellingPattern(spelling string) string {
	toks := strings.Fields(spelling)
	for i, tk := range toks {
		toks[i] = regexp.QuoteMeta(tk)
	}
	return strings.Join(toks, `[[:space:]]+`)
}

// buildSuiteRe compiles the direct-suite-channel shape (violation a) from the
// derived spellings.
func buildSuiteRe(spellings []string) *regexp.Regexp {
	frags := make([]string, len(spellings))
	for i, s := range spellings {
		frags[i] = spellingPattern(s)
	}
	return regexp.MustCompile("(" + strings.Join(frags, "|") + ")")
}

// buildIdentityRe compiles the resolved-identity interpolation shape (violation b):
// a shell expansion ($NAME or ${NAME}) of a derived exported test-command variable.
func buildIdentityRe(exported []string) *regexp.Regexp {
	quoted := make([]string, len(exported))
	for i, e := range exported {
		quoted[i] = regexp.QuoteMeta(e)
	}
	return regexp.MustCompile(`\$\{?(` + strings.Join(quoted, "|") + `)\}?`)
}

var gateDriveRe = regexp.MustCompile(`gate[[:space:]]+drive`)

// scanWorkflowMD (a+b): fenced runnable recipes only. (a) is excused when the
// enclosing fenced block routes through `gate drive` anywhere (the task-owner recipe
// carries the suite argv after `-- ` on the driver line); (b) is excused on a
// `gate drive` line.
func scanWorkflowMD(rel, content string, suiteRe, identityRe *regexp.Regexp) []string {
	var v []string
	lines := strings.Split(content, "\n")
	inFence := false
	var block []int
	flush := func() {
		if len(block) == 0 {
			return
		}
		blockHasDrive := false
		for _, i := range block {
			if gateDriveRe.MatchString(lines[i]) {
				blockHasDrive = true
				break
			}
		}
		for _, i := range block {
			line := lines[i]
			if suiteRe.MatchString(line) && !blockHasDrive {
				v = append(v, fmt.Sprintf("a\t%s:%d: direct docket suite channel in a fenced recipe not routed through gate drive", rel, i+1))
			}
			if identityRe.MatchString(line) && !gateDriveRe.MatchString(line) {
				v = append(v, fmt.Sprintf("b\t%s:%d: resolved test-command identity invoked directly outside gate drive", rel, i+1))
			}
		}
		block = block[:0]
	}
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inFence {
				flush()
			}
			inFence = !inFence
			continue
		}
		if inFence {
			block = append(block, i)
		}
	}
	flush() // unterminated fence: scan what was collected
	return v
}

// scanWorkflowSH (a+b): non-comment command lines of a workflow shell script. Both
// shapes are excused on a `gate drive` line.
func scanWorkflowSH(rel, content string, suiteRe, identityRe *regexp.Regexp) []string {
	var v []string
	for i, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if gateDriveRe.MatchString(line) {
			continue
		}
		if suiteRe.MatchString(line) {
			v = append(v, fmt.Sprintf("a\t%s:%d: direct docket suite channel on a shell command line not routed through gate drive", rel, i+1))
		}
		if identityRe.MatchString(line) {
			v = append(v, fmt.Sprintf("b\t%s:%d: resolved test-command identity invoked directly outside gate drive", rel, i+1))
		}
	}
	return v
}

func TestExecBoundary(t *testing.T) {
	root := guardRoot(t)
	pop := maintainedPop(t, root)

	// Both classifiers derive their closed identifiers; assert the derivation is
	// non-vacuous BEFORE any "zero violations" negative can pass by default.
	spellings := suiteChannelSpellings(t, root)
	if len(spellings) < 2 {
		t.Fatalf("derivation floor: expected the direct + source suite spellings, got %v", spellings)
	}
	exported := testCommandExportedVars(t)
	if len(exported) == 0 {
		t.Fatalf("derivation floor: config.Effective exposes no `test_command` field — the identity classifier would be vacuous")
	}

	suiteRe := buildSuiteRe(spellings)
	identityRe := buildIdentityRe(exported)

	var wfMD, wfSH []string
	for _, rel := range pop {
		switch {
		case isWorkflowMD(rel):
			wfMD = append(wfMD, rel)
		case isWorkflowSH(rel):
			wfSH = append(wfSH, rel)
		}
	}
	if len(wfMD) < 20 {
		t.Fatalf("population floor: only %d workflow-contract markdown files (expected >= 20)", len(wfMD))
	}

	var violations []string
	for _, rel := range wfMD {
		violations = append(violations, scanWorkflowMD(rel, readMaintained(t, root, rel), suiteRe, identityRe)...)
	}
	for _, rel := range wfSH {
		violations = append(violations, scanWorkflowSH(rel, readMaintained(t, root, rel), suiteRe, identityRe)...)
	}
	if len(violations) != 0 {
		t.Errorf("direct test-execution in workflow fixtures:\n%s", strings.Join(violations, "\n"))
	}

	// Red-injection proof (change 0359 spec requirement): mutate the POPULATION with
	// synthetic fixture content and confirm the classifier reddens on each violation
	// shape and stays green on the sanctioned driver forms.
	t.Run("red_injection", func(t *testing.T) {
		// (a) fenced direct suite channel, no gate drive → violation.
		a := "```bash\ndocket development test\n```\n"
		if got := scanWorkflowMD("skills/x/SKILL.md", a, suiteRe, identityRe); len(got) == 0 {
			t.Errorf("a: missed a fenced `docket development test` recipe")
		}
		// (a) the source-entry spelling too.
		aSrc := "```bash\ngo run ./cmd/docket development test\n```\n"
		if got := scanWorkflowMD("skills/x/SKILL.md", aSrc, suiteRe, identityRe); len(got) == 0 {
			t.Errorf("a: missed a fenced `go run ./cmd/docket development test` recipe")
		}
		// (b) fenced resolved-identity shell invocation → violation.
		b := "```bash\n\"$BUILD_TEST_COMMAND\"\n```\n"
		if got := scanWorkflowMD("skills/x/SKILL.md", b, suiteRe, identityRe); len(got) == 0 {
			t.Errorf("b: missed a fenced `\"$BUILD_TEST_COMMAND\"` invocation")
		}
		// sanctioned build-owner driver recipe → not a violation.
		buildOwner := "```bash\ndocket gate drive start --owner build --run-root .scratch\n```\n"
		if got := scanWorkflowMD("skills/x/SKILL.md", buildOwner, suiteRe, identityRe); len(got) != 0 {
			t.Errorf("sanctioned `gate drive start --owner build` wrongly flagged: %v", got)
		}
		// sanctioned task-owner recipe carrying the suite argv AFTER `-- ` on the
		// driver line → routed through the driver, not a violation.
		taskOwner := "```bash\ndocket gate drive start --owner task --scope-id S --child-cap C --run-root .scratch -- go run ./cmd/docket development test\n```\n"
		if got := scanWorkflowMD("skills/x/SKILL.md", taskOwner, suiteRe, identityRe); len(got) != 0 {
			t.Errorf("task-owner driver recipe carrying suite argv wrongly flagged: %v", got)
		}
		// Recorded residual: the SAME suite spelling in INLINE prose back-ticks (no
		// fence) is NOT scanned — the documented evasion, asserted so the limitation
		// is visible and honest.
		inline := "Never run `docket development test` directly; route it through the driver.\n"
		if got := scanWorkflowMD("skills/x/SKILL.md", inline, suiteRe, identityRe); len(got) != 0 {
			t.Errorf("residual mischaracterized: an inline prose mention must NOT be flagged: %v", got)
		}
		// Shell: command line vs comment vs driver line.
		if got := scanWorkflowSH("scripts/x.sh", "docket development test\n", suiteRe, identityRe); len(got) == 0 {
			t.Errorf("a: missed a `docket development test` shell command line")
		}
		if got := scanWorkflowSH("scripts/x.sh", "# docket development test is forbidden here\n", suiteRe, identityRe); len(got) != 0 {
			t.Errorf("a: wrongly flagged a shell comment: %v", got)
		}
		if got := scanWorkflowSH("scripts/x.sh", "docket gate drive start --owner task -- \"$FINALIZE_TEST_COMMAND\"\n", suiteRe, identityRe); len(got) != 0 {
			t.Errorf("a driver line carrying an identity argv wrongly flagged: %v", got)
		}
	})
}
