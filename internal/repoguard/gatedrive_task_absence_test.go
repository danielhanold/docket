package repoguard

// Change 0488 replaces change 0416's scoped-start identity guard (its subject,
// a build-task worker's task-owned drive, is gone) with that guard's inverse.
// Build-task workers run every test directly in the foreground under
// `timeout --kill-after=10s 10m` and call no gate operation; the gate driver
// serves only the full-suite gates. Three parts:
//   (1) no paragraph of the maintained workflow-markdown corpus (isWorkflowMD:
//       skills/ and agents/ plus their embedded mirrors) or the dispatch-rule
//       surface (cursor-rules/ plus its embedded mirror) instructs a
//       task-owned drive or its recovery-scope protocol: `--owner task`, the
//       prepare-scope / acknowledge / takeover operations (dotted catalog id
//       or spaced CLI spelling), the predecessor-receipt flags, or the scope
//       credentials --scope-id / --child-cap;
//   (2) no paragraph pairs WAITING with a worker outcome (COMPLETE,
//       NEEDS_ESCALATION, BLOCKED): gatedriver_test.go's retired detector D
//       shape (waitFwd/waitRev), inverted from "must name a handoff" to "must
//       not occur";
//   (3) a non-vacuity companion through the SAME extractor (the same corpus
//       walk and paragraphs split): the build controller's build-owned
//       gate.drive.start site and docket-build-task's outcome list must still
//       be found, so a dead extractor or a moved corpus reddens instead of
//       passing.
// Sites are discovered by scanning the whole corpus, never a per-file list;
// the forbidden token set is the asserted property.
// Residual risk, recorded not hidden (byte-pattern-guard-matches-a-spelling):
// a retired operation named without its gate.drive / gate drive prefix (a
// bare `takeover`) is not matched — the bare words are ordinary English, and
// change 0489 deleted these operations from the Go catalog.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

const (
	buildSkillRel     = "skills/docket-build/SKILL.md"
	buildTaskSkillRel = "skills/docket-build-task/SKILL.md"
	runTrackerRuleRel = "cursor-rules/run-tracker.md"
	// workerOutcomeList is docket-build-task's return-schema outcome line.
	workerOutcomeList = "OUTCOME: COMPLETE | NEEDS_ESCALATION | BLOCKED"
)

var (
	// startOpRe also serves gatedrive_run_id_thread_test.go's build-owned sites.
	startOpRe = regexp.MustCompile(`gate\.drive\.start`)

	// taskDriveTokenRe: one alternative per forbidden class, each right-bounded
	// so a longer token (--owner taskforce, --scope-ids) never matches.
	taskDriveTokenRe = regexp.MustCompile(`--owner[ =]task(?:[^a-z-]|$)` +
		`|gate[. ]drive[. ](?:prepare-scope|acknowledge|takeover)(?:[^a-z-]|$)` +
		`|--predecessor-(?:drive-id|owner-gen)(?:[^a-z-]|$)` +
		`|--(?:scope-id|child-cap)(?:[^a-z-]|$)`)

	// waitFwd / waitRev: WAITING within 45 characters of a worker outcome, in
	// either order (retired detector D's shape; waitRev also matches a
	// paragraph-final WAITING).
	waitFwd = regexp.MustCompile(`(^|[^-A-Za-z])WAITING([^-A-Za-z]).{0,45}(COMPLETE|BLOCKED|NEEDS_ESCALATION)`)
	waitRev = regexp.MustCompile(`(COMPLETE|BLOCKED|NEEDS_ESCALATION)([^-A-Za-z]).{0,45}([^-A-Za-z])WAITING([^-A-Za-z]|$)`)
)

// isTaskDriveCorpus: the workflow-markdown corpus plus the dispatch-rule
// surface the run-tracker block renders from (cursor-rules/ and its mirror).
func isTaskDriveCorpus(rel string) bool {
	return isWorkflowMD(rel) ||
		(hasExt(rel, ".md") && underDir(rel, "cursor-rules", "internal/assets/embedded/tree/cursor-rules"))
}

// taskDriveViolations is the whole detector over one file's paragraphs,
// exposed so non_vacuity exercises it directly.
func taskDriveViolations(rel string, paras []string) []string {
	var v []string
	for _, p := range paras {
		if m := taskDriveTokenRe.FindString(p); m != "" {
			v = append(v, fmt.Sprintf("%s: instructs a task-owned drive or its recovery scope (%q): %.160s", rel, strings.TrimSpace(m), p))
		}
		flat := strings.ReplaceAll(p, "`", "")
		if waitFwd.MatchString(flat) || waitRev.MatchString(flat) {
			v = append(v, fmt.Sprintf("%s: pairs WAITING with a worker outcome (workers return COMPLETE, NEEDS_ESCALATION, or BLOCKED): %.160s", rel, p))
		}
	}
	return v
}

func TestNoTaskOwnedDriveInstructions(t *testing.T) {
	root := guardRoot(t)
	var violations []string
	scanned := map[string]bool{}
	buildStartSeen, outcomeListSeen := false, false
	for _, rel := range maintainedPop(t, root) {
		if !isTaskDriveCorpus(rel) {
			continue
		}
		scanned[rel] = true
		paras := paragraphs(readMaintained(t, root, rel))
		violations = append(violations, taskDriveViolations(rel, paras)...)
		for _, p := range paras {
			if rel == buildSkillRel && startOpRe.MatchString(p) && ownerBuildRe.MatchString(p) {
				buildStartSeen = true
			}
			if rel == buildTaskSkillRel && strings.Contains(p, workerOutcomeList) {
				outcomeListSeen = true
			}
		}
	}

	// Population and non-vacuity FIRST (a vacuous scan passes every negative).
	if len(scanned) < 20 {
		t.Fatalf("population floor: only %d corpus files scanned (expected >= 20)", len(scanned))
	}
	for _, rel := range []string{
		buildSkillRel, buildTaskSkillRel, runTrackerRuleRel,
		"internal/assets/embedded/tree/" + buildTaskSkillRel,
		"internal/assets/embedded/tree/" + runTrackerRuleRel,
	} {
		if !scanned[rel] {
			t.Errorf("coverage floor: %s was not scanned (corpus predicate or walk drifted)", rel)
		}
	}
	if !buildStartSeen {
		t.Errorf("non-vacuity: the extractor found no build-owned gate.drive.start paragraph in %s", buildSkillRel)
	}
	if !outcomeListSeen {
		t.Errorf("non-vacuity: the extractor found no %q paragraph in %s", workerOutcomeList, buildTaskSkillRel)
	}
	if len(violations) != 0 {
		t.Errorf("task-owned drive instructions (%d) — workers run tests directly under timeout and call no gate operation:\n%s",
			len(violations), strings.Join(violations, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		flagged := []string{
			"run the `gate.drive.start` operation with `--owner task --json -- <cmd>`",
			"start it with `--owner=task`",
			"run the `gate.drive.prepare-scope` operation with `--change-id <id>`",
			"run `docket gate drive prepare-scope --change-id 1`",
			"perform the `gate.drive.acknowledge` operation",
			"run `docket gate drive acknowledge --drive-id <id>`",
			"run the `gate.drive.takeover` operation",
			"pass `--predecessor-drive-id <id>`",
			"pass `--predecessor-owner-gen <gen>`",
			"pass `--scope-id <id>` through",
			"hand over `--child-cap <token>`",
			"Return `COMPLETE`, `WAITING`, or `BLOCKED`.",
			"OUTCOME: COMPLETE | NEEDS_ESCALATION | BLOCKED | WAITING",
			"`WAITING` — then `NEEDS_ESCALATION` follows",
		}
		for _, s := range flagged {
			if got := taskDriveViolations("skills/x/SKILL.md", paragraphs(s)); len(got) == 0 {
				t.Errorf("missed a forbidden instruction: %q", s)
			}
		}
		clean := []string{
			"the `gate.drive.start` operation with `--owner build --change-id <id> --json`",
			"`WAITING` is the only nonterminal disposition and the only one that advances again",
			workerOutcomeList,
			"the `--owner taskforce` flag, `--scope-ids`, and `--child-capacity`",
			"run the `maintenance.sweep` operation with `--scope full --json`",
			"a run-waiting verdict is never COMPLETE",
			"the catalog's takeover and acknowledge operations",
		}
		for _, s := range clean {
			if got := taskDriveViolations("skills/x/SKILL.md", paragraphs(s)); len(got) != 0 {
				t.Errorf("wrongly flagged a permitted paragraph %q: %v", s, got)
			}
		}
		// Whitespace collapse: a wrapped token is still one matchable paragraph.
		for _, wrapped := range []string{
			"the `gate.drive.prepare-scope`\noperation",
			"run `docket gate drive\nacknowledge`",
			"Return COMPLETE or\nWAITING",
		} {
			if got := taskDriveViolations("skills/x/SKILL.md", paragraphs(wrapped)); len(got) == 0 {
				t.Errorf("whitespace collapse failed: %q", wrapped)
			}
		}
		// Paragraph scoping: WAITING and an outcome in different paragraphs are no pair.
		if got := taskDriveViolations("skills/x/SKILL.md", paragraphs("`WAITING` ends a slice.\n\nReturn COMPLETE.")); len(got) != 0 {
			t.Errorf("a WAITING/outcome pair across paragraphs was wrongly flagged: %v", got)
		}
		if !isTaskDriveCorpus(runTrackerRuleRel) ||
			!isTaskDriveCorpus("internal/assets/embedded/tree/"+runTrackerRuleRel) ||
			isTaskDriveCorpus("docs/reference/glossary.md") {
			t.Errorf("corpus predicate misclassified the dispatch-rule surface or docs/")
		}
	})
}
